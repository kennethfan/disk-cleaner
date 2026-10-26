// Package scan 负责发现磁盘上可清理的内容。
//
// 重要约束：本包只读。任何删除动作都在 trash 包中，且必须经 guard 校验。
package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// Scanner 是一个具体的扫描器实现。
type Scanner interface {
	// Name 返回扫描器名称，用于错误归因。
	Name() string
	// Scan 产出清理项。实现应遵守 ctx 取消，并在单项失败时继续。
	Scan(ctx context.Context) ([]model.Item, error)
}

// Options 控制扫描行为。
type Options struct {
	// LargeFileThreshold 是"大文件"的字节阈值，默认 500MiB。
	LargeFileThreshold int64
	// Home 是用户主目录。
	Home string
	// Concurrency 是并发扫描器数量，0 表示按 CPU 数自动取值。
	Concurrency int
}

// DefaultOptions 返回推荐默认值。
func DefaultOptions() Options {
	home, _ := os.UserHomeDir()
	return Options{
		LargeFileThreshold: 500 << 20,
		Home:               home,
		Concurrency:        0,
	}
}

// All 返回默认启用的全部扫描器。
// 各扫描器在运行时自行探测目标是否存在，不存在的目标不会产生任何项。
func All(opts Options) []Scanner {
	return []Scanner{
		NewUserCacheScanner(opts),
		NewSystemCacheScanner(opts),
		NewDevCacheScanner(opts),
		NewLargeFileScanner(opts),
		NewJunkScanner(opts),
		NewSnapshotScanner(opts),
	}
}

// ScanAll 并发执行全部扫描器并汇总结果。
// 单个扫描器返回的错误被收集到 Report.Errors，不会中断整体扫描。
func ScanAll(ctx context.Context, scanners []Scanner, opts Options) *model.Report {
	report := &model.Report{ScannedAt: time.Now()}
	if len(scanners) == 0 {
		return report
	}

	limit := opts.Concurrency
	if limit <= 0 {
		limit = runtime.NumCPU()
	}
	if limit > len(scanners) {
		limit = len(scanners)
	}

	type result struct {
		items []model.Item
		err   error
		sc    Scanner
	}

	sem := make(chan struct{}, limit)
	results := make(chan result, len(scanners))

	var wg sync.WaitGroup
	for _, sc := range scanners {
		wg.Add(1)
		go func(s Scanner) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			items, err := s.Scan(ctx)
			results <- result{items: items, err: err, sc: s}
		}(sc)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			report.Errors = append(report.Errors, model.ScanError{
				Scanner: r.sc.Name(),
				Err:     r.err,
			})
		}
		report.Items = append(report.Items, r.items...)
	}

	report.Items = dedupeByPath(report.Items)
	report.Finalize()
	return report
}

// dedupeByPath 去掉重复路径，每条路径只保留一个项。
//
// 不同扫描器可能命中同一目标（例如 Downloads 下的大文件同时被
// "大文件" 与 "垃圾文件" 扫到）。若不去重，总大小会被重复累加，
// 用户在界面上也会看到两条一样的记录。
//
// 保留哪一个必须确定：并发扫描器的完成顺序不稳定，若简单"保留首次出现"，
// 同一个文件会在不同次运行中显示为不同类别。因此按类别优先级选择，
// 并用 DisplayName 作为最终决胜条件，保证结果可复现。
func dedupeByPath(items []model.Item) []model.Item {
	chosen := make(map[string]model.Item, len(items))
	order := make([]string, 0, len(items))

	for _, it := range items {
		key := filepath.Clean(it.Path)
		prev, dup := chosen[key]
		if !dup {
			chosen[key] = it
			order = append(order, key)
			continue
		}
		if moreSpecific(it, prev) {
			chosen[key] = it
		}
	}

	out := make([]model.Item, 0, len(order))
	for _, key := range order {
		out = append(out, chosen[key])
	}
	return out
}

// categoryPriority 越小表示越"具体"，越应当被保留。
// 旧下载带有时间上下文，比笼统的"大文件"更有行动价值。
func categoryPriority(c model.Category) int {
	switch c {
	case model.CategorySnapshot:
		return 0
	case model.CategoryJunk:
		return 1
	case model.CategoryDevCache:
		return 2
	case model.CategoryUserCache:
		return 3
	case model.CategorySystemCache:
		return 4
	case model.CategoryLargeFile:
		return 5
	default:
		return 6
	}
}

// moreSpecific 判断 a 是否应当取代 b。
func moreSpecific(a, b model.Item) bool {
	pa, pb := categoryPriority(a.Category), categoryPriority(b.Category)
	if pa != pb {
		return pa < pb
	}
	return a.DisplayName < b.DisplayName
}

// dirSize 递归统计目录占用的字节数。
// 遇到无权限的子目录时跳过而非中断，保证在受保护目录上也能返回部分结果。
func dirSize(ctx context.Context, root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// 权限不足等错误：跳过该子树，继续统计。
			if os.IsPermission(err) {
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// pathExists 判断路径是否存在。
func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// makeItem 构造一个目录类清理项，并计算其大小。
func makeItem(ctx context.Context, path, name string, cat model.Category, risk model.Risk, note string) (model.Item, bool) {
	if !pathExists(path) {
		return model.Item{}, false
	}
	size, _ := dirSize(ctx, path)
	return model.Item{
		Path:        path,
		DisplayName: name,
		Category:    cat,
		Risk:        risk,
		Size:        size,
		IsDir:       true,
		Note:        note,
	}, true
}
