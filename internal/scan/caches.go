package scan

import (
	"context"
	"path/filepath"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// UserCacheScanner 扫描用户级缓存目录。
// 这些目录删除后会被对应程序自动重建，风险最低。
type UserCacheScanner struct {
	home string
}

// NewUserCacheScanner 创建用户缓存扫描器。
func NewUserCacheScanner(opts Options) *UserCacheScanner {
	return &UserCacheScanner{home: opts.Home}
}

// Name 实现 Scanner。
func (s *UserCacheScanner) Name() string { return "用户缓存" }

// Scan 实现 Scanner。
func (s *UserCacheScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if s.home == "" {
		return nil, nil
	}
	targets := []struct {
		rel  string
		name string
		note string
	}{
		{"Library/Caches", "用户缓存目录 (~/Library/Caches)", "各应用自动重建，可安全清理"},
		{"Library/Logs", "用户日志 (~/Library/Logs)", "应用诊断日志，不影响功能"},
		{"Library/Saved Application State", "应用保存状态", "窗口位置等状态信息，会重新生成"},
	}
	var items []model.Item
	for _, t := range targets {
		p := filepath.Join(s.home, t.rel)
		if it, ok := makeItem(ctx, p, t.name, model.CategoryUserCache, model.RiskLow, t.note); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

// SystemCacheScanner 扫描系统级缓存目录。
// 部分子目录需要管理员权限，无权限时会被跳过。
type SystemCacheScanner struct{}

// NewSystemCacheScanner 创建系统缓存扫描器。
func NewSystemCacheScanner(opts Options) *SystemCacheScanner {
	return &SystemCacheScanner{}
}

// Name 实现 Scanner。
func (s *SystemCacheScanner) Name() string { return "系统缓存" }

// Scan 实现 Scanner。
func (s *SystemCacheScanner) Scan(ctx context.Context) ([]model.Item, error) {
	targets := []struct {
		path string
		name string
		note string
	}{
		{"/Library/Caches", "系统缓存 (/Library/Caches)", "系统级缓存，部分需管理员权限"},
		{"/private/var/log/asl", "系统日志归档 (asl)", "旧版系统日志"},
	}
	var items []model.Item
	for _, t := range targets {
		if it, ok := makeItem(ctx, t.path, t.name, model.CategorySystemCache, model.RiskLow, t.note); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
