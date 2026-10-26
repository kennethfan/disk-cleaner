package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// LargeFileScanner 扫描体积超过阈值的大文件。
//
// 与缓存类扫描不同，大文件通常是用户自己的数据，因此：
//   - 风险等级为 medium（删除前必须由人确认）
//   - 不递归进入系统目录与隐藏的 Library 深层，避免噪音与误判
type LargeFileScanner struct {
	home      string
	threshold int64
}

// NewLargeFileScanner 创建大文件扫描器。
func NewLargeFileScanner(opts Options) *LargeFileScanner {
	th := opts.LargeFileThreshold
	if th <= 0 {
		th = 500 << 20
	}
	return &LargeFileScanner{home: opts.Home, threshold: th}
}

// Name 实现 Scanner。
func (s *LargeFileScanner) Name() string { return "大文件" }

// largeFileScanRoots 是默认搜索大文件的顶层目录。
var largeFileScanRoots = []string{
	"Downloads",
	"Documents",
	"Desktop",
	"Movies",
	"Pictures",
	"Library/Application Support",
}

// maxLargeFiles 限制返回条数，避免报告过长拖垮界面。
const maxLargeFiles = 50

// Scan 实现 Scanner。
func (s *LargeFileScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if s.home == "" {
		return nil, nil
	}
	var items []model.Item
	for _, rel := range largeFileScanRoots {
		root := filepath.Join(s.home, rel)
		if !pathExists(root) {
			continue
		}
		select {
		case <-ctx.Done():
			return items, ctx.Err()
		default:
		}

		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if info.IsDir() {
				return nil
			}
			if !info.Mode().IsRegular() || info.Size() < s.threshold {
				return nil
			}
			items = append(items, model.Item{
				Path:        path,
				DisplayName: rel + "/" + mustRel(root, path),
				Category:    model.CategoryLargeFile,
				Risk:        model.RiskMedium,
				Size:        info.Size(),
				IsDir:       false,
				Note:        "超过 " + model.HumanBytes(s.threshold) + " 的大文件，请确认后再清理",
			})
			return nil
		})
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	if len(items) > maxLargeFiles {
		items = items[:maxLargeFiles]
	}
	return items, nil
}

// mustRel 返回 path 相对 base 的路径，失败时退回文件名。
func mustRel(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
}
