package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// TargetScanner 扫描用户显式指定的目录。
//
// 与预设扫描器不同，它不假设目录用途，只如实列出该目录下的
// 直接子项及其大小。用于"我只想看这个目录占了多少"的场景。
//
// 安全约束：本扫描器产出的项一律标记为只读（见 model.Item.ReadOnly），
// 界面据此禁用勾选；删除动作由 guard 白名单把关。
type TargetScanner struct {
	paths     []string
	threshold int64
}

// NewTargetScanner 创建指定目录扫描器。
//
// paths 中的 ~ 会被展开；不存在的目录会被跳过（不报错，
// 因为用户可能在扫描期间删掉了它）。
func NewTargetScanner(paths []string, threshold int64) *TargetScanner {
	return &TargetScanner{paths: paths, threshold: threshold}
}

// Name 实现 Scanner。
func (s *TargetScanner) Name() string { return "指定目录" }

// Scan 实现 Scanner。
func (s *TargetScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if len(s.paths) == 0 {
		return nil, nil
	}

	home, _ := os.UserHomeDir()
	var items []model.Item
	var firstErr error

	for _, raw := range s.paths {
		if cerr := checkpoint(ctx); cerr != nil {
			return items, cerr
		}

		p := expandHomePath(raw, home)
		abs, err := filepath.Abs(p)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("解析路径 %q: %w", raw, err)
			}
			continue
		}

		info, err := os.Stat(abs)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("访问 %s: %w", abs, err)
			}
			continue
		}

		if !info.IsDir() {
			// 直接指定单个文件：作为一个项返回。
			items = append(items, model.Item{
				Path:        abs,
				DisplayName: filepath.Base(abs),
				Category:    model.CategoryLargeFile,
				Risk:        model.RiskMedium,
				Size:        info.Size(),
				IsDir:       false,
				ReadOnly:    true,
				Note:        "指定目录扫描：仅可查看",
			})
			continue
		}

		// 目录：把它自身作为一个项（显示总大小），同时列出直接子项。
		size, _ := dirSize(ctx, abs)
		items = append(items, model.Item{
			Path:        abs,
			DisplayName: abs,
			Category:    model.CategoryLargeFile,
			Risk:        model.RiskMedium,
			Size:        size,
			IsDir:       true,
			ReadOnly:    true,
			Note:        "指定目录扫描：仅可查看",
		})

		children, err := NewChildScanner().Children(ctx, abs)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("列出 %s: %w", abs, err)
			}
			continue
		}
		for _, c := range children {
			c.Category = model.CategoryLargeFile
			c.Risk = model.RiskMedium
			c.ReadOnly = true
			c.Note = "指定目录扫描：仅可查看"
			items = append(items, c)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Size != items[j].Size {
			return items[i].Size > items[j].Size
		}
		return items[i].Path < items[j].Path
	})

	// 只在全部目录都失败时返回错误；部分成功仍返回结果。
	if len(items) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return items, nil
}

// expandHomePath 展开开头的 ~。
func expandHomePath(p, home string) string {
	if p == "~" {
		return home
	}
	if len(p) > 2 && p[0] == '~' && p[1] == filepath.Separator {
		return filepath.Join(home, p[2:])
	}
	return p
}
