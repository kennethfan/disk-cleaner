package scan

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// JunkScanner 扫描废纸篓、旧下载与 iOS 备份。
type JunkScanner struct {
	home string
}

// NewJunkScanner 创建垃圾文件扫描器。
func NewJunkScanner(opts Options) *JunkScanner {
	return &JunkScanner{home: opts.Home}
}

// Name 实现 Scanner。
func (s *JunkScanner) Name() string { return "垃圾文件" }

// staleDownloadDays 是"旧下载"的天数阈值。
const staleDownloadDays = 90

// Scan 实现 Scanner。
func (s *JunkScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if s.home == "" {
		return nil, nil
	}
	var items []model.Item

	// 废纸篓：本身已在废纸篓，只能永久删除，因此 Permanent=true。
	trashPath := filepath.Join(s.home, ".Trash")
	if it, ok := makeItem(ctx, trashPath, "废纸篓 (~/.Trash)", model.CategoryJunk, model.RiskHigh,
		"永久删除，不可恢复"); ok {
		it.Permanent = true
		items = append(items, it)
	}

	// iOS 设备备份。
	backup := filepath.Join(s.home, "Library/Application Support/MobileSync/Backup")
	if it, ok := makeItem(ctx, backup, "iOS 设备备份", model.CategoryJunk, model.RiskHigh,
		"删除后需重新连接设备备份"); ok {
		items = append(items, it)
	}

	// 旧下载文件：按最后修改时间筛选。
	items = append(items, s.staleDownloads(ctx)...)

	return items, nil
}

// staleDownloads 找出下载目录中超过阈值的陈旧文件。
func (s *JunkScanner) staleDownloads(ctx context.Context) []model.Item {
	dir := filepath.Join(s.home, "Downloads")
	if !pathExists(dir) {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -staleDownloadDays)

	var items []model.Item
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		select {
		case <-ctx.Done():
			return items
		default:
		}
		info, err := e.Info()
		if err != nil || info.IsDir() {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		items = append(items, model.Item{
			Path:        filepath.Join(dir, e.Name()),
			DisplayName: "旧下载: " + e.Name(),
			Category:    model.CategoryJunk,
			Risk:        model.RiskMedium,
			Size:        info.Size(),
			IsDir:       false,
			Note:        "超过 " + strconv.Itoa(staleDownloadDays) + " 天未修改",
		})
	}
	return items
}
