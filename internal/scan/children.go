package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/kennethfan/disk-cleaner/internal/exclude"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// ChildScanner 按需列出某个目录的直接子项，用于树形界面展开。
//
// 与 Scanner 的区别：Scanner 一次性产出全部可清理项；
// ChildScanner 只在用户展开某个节点时调用，避免启动时遍历
// 上百个子目录（实测 ~/Library/Caches 有 104 项、~/Downloads 163 项）。
type ChildScanner struct {
	// exclude 为 nil 时不排除任何子项。
	exclude *exclude.Matcher
}

// NewChildScanner 创建子项扫描器。
func NewChildScanner() *ChildScanner {
	return &ChildScanner{}
}

// NewChildScannerWithExclude 创建带排除规则的子项扫描器。
//
// 排除规则必须在子项展开时也生效，否则被排除的目录虽然不出现在
// 扫描结果里，展开父目录时仍会重新冒出来。
func NewChildScannerWithExclude(m *exclude.Matcher) *ChildScanner {
	return &ChildScanner{exclude: m}
}

// maxChildren 限制单次返回的子项数量，避免超大目录拖垮界面。
const maxChildren = 200

// Children 列出 dir 的直接子项，按体积降序返回。
//
// 目录项会递归统计总大小；文件项直接取 Size。
// 权限不足的子目录会被跳过而非中断。
func (s *ChildScanner) Children(ctx context.Context, dir string) ([]model.Item, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	items := make([]model.Item, 0, len(entries))

	// 并发统计各子项大小：串行遍历上百个子目录会明显卡顿。
	type sized struct {
		idx   int
		size  int64
		isDir bool
	}
	results := make(chan sized, len(entries))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // 限制并发，避免 IO 打满

	for i, e := range entries {
		if cerr := checkpoint(ctx); cerr != nil {
			return items, cerr
		}

		full := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}

		if !info.IsDir() {
			// 普通文件：大小已知，无需遍历。
			results <- sized{idx: i, size: info.Size(), isDir: false}
			continue
		}

		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sz, _ := dirSize(ctx, path)
			results <- sized{idx: idx, size: sz, isDir: true}
		}(i, full)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	sizeByIdx := make(map[int]sized, len(entries))
	for r := range results {
		sizeByIdx[r.idx] = r
	}

	for i, e := range entries {
		r, ok := sizeByIdx[i]
		if !ok {
			continue // 该子项读取失败，跳过
		}
		full := filepath.Join(dir, e.Name())

		// 排除规则在此处也要生效：否则被排除的目录虽不在扫描结果里，
		// 展开父目录时仍会重新冒出来。
		if s.exclude != nil && s.exclude.Matches(full) {
			continue
		}

		items = append(items, model.Item{
			Path:        full,
			DisplayName: e.Name(),
			// 子项继承父项的类别，由调用方覆盖。
			Risk:  model.RiskLow,
			Size:  r.size,
			IsDir: r.isDir,
		})
	}

	// 按体积降序，让占用最大的排在最前。
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Size != items[j].Size {
			return items[i].Size > items[j].Size
		}
		return items[i].DisplayName < items[j].DisplayName
	})

	if len(items) > maxChildren {
		items = items[:maxChildren]
	}
	return items, nil
}
