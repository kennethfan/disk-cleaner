package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	skips := homePrefixes(s.home, redLineHomeDirs)
	for _, rel := range largeFileScanRoots {
		root := filepath.Join(s.home, rel)
		if !pathExists(root) {
			continue
		}
		if cerr := checkpoint(ctx); cerr != nil {
			return items, cerr
		}

		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if cerr := checkpoint(ctx); cerr != nil {
				return cerr
			}
			if info.IsDir() {
				if underAny(path, skips) || s.isAppSupportCacheSubdir(path) {
					return filepath.SkipDir
				}
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

// isAppSupportCacheSubdir 判断 path 是否是 <app>/<cache> 形式的应用缓存子目录。
//
// 这类目录已由 AppSupportCacheScanner 整体作为一个清理项；大文件扫描再列出
// 其中的文件会造成父子重复计数（去重只按路径精确匹配）。
func (s *LargeFileScanner) isAppSupportCacheSubdir(path string) bool {
	root := filepath.Join(s.home, "Library", "Application Support")
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) == 2 && isCacheDirName(parts[1])
}

// mustRel 返回 path 相对 base 的路径，失败时退回文件名。
func mustRel(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
}
