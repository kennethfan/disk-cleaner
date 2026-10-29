package scan

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// ContainerCacheScanner 扫描应用沙盒容器内的缓存（速查卡 C 类）。
//
// 安全约束：只清容器内的 Data/Library/Caches，绝不提供整个容器目录。
// 容器里还有应用数据与登录态（例如微信的 db_storage），整目录删除会丢数据。
type ContainerCacheScanner struct {
	home string
}

// NewContainerCacheScanner 创建容器缓存扫描器。
func NewContainerCacheScanner(opts Options) *ContainerCacheScanner {
	return &ContainerCacheScanner{home: opts.Home}
}

// Name 实现 Scanner。
func (s *ContainerCacheScanner) Name() string { return "容器缓存" }

// containerDisplayNames 把常见容器标识换成用户认得的名字。
var containerDisplayNames = map[string]string{
	"com.tencent.xinWeChat":  "微信",
	"com.tencent.meeting":    "腾讯会议",
	"com.docker.docker":      "Docker",
	"com.apple.Safari":       "Safari",
	"com.google.Chrome":      "Chrome",
	"com.microsoft.VSCode":   "VS Code",
	"com.jetbrains.intellij": "IntelliJ IDEA",
	"com.tencent.qq":         "QQ",
}

// Scan 实现 Scanner。
func (s *ContainerCacheScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if s.home == "" {
		return nil, nil
	}
	root := filepath.Join(s.home, "Library/Containers")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}

	var items []model.Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if cerr := checkpoint(ctx); cerr != nil {
			return items, cerr
		}
		cache := filepath.Join(root, e.Name(), "Data/Library/Caches")
		name := containerDisplayName(e.Name())
		it, ok := makeItem(ctx, cache, name+" 容器缓存", model.CategoryUserCache, model.RiskLow,
			"仅清容器内的 Caches，应用数据与登录态保留")
		if ok {
			items = append(items, it)
		}
	}
	return items, nil
}

// containerDisplayName 返回容器的展示名，未知标识原样返回。
func containerDisplayName(id string) string {
	if name, ok := containerDisplayNames[id]; ok {
		return name
	}
	return id
}

// AppSupportCacheScanner 扫描 ~/Library/Application Support 下明确带 cache 的
// 子目录（速查卡 D 类）。
//
// Application Support 是混合区：里既有数据库、扩展等用户数据，也有 Cache /
// Code Cache 这类可再生缓存。因此只按名字匹配 cache，且最多看两层
// （<app>/Cache、<app>/Code Cache），不做整棵遍历。
type AppSupportCacheScanner struct {
	home string
}

// NewAppSupportCacheScanner 创建应用支持数据缓存扫描器。
func NewAppSupportCacheScanner(opts Options) *AppSupportCacheScanner {
	return &AppSupportCacheScanner{home: opts.Home}
}

// Name 实现 Scanner。
func (s *AppSupportCacheScanner) Name() string { return "应用缓存子目录" }

// Scan 实现 Scanner。
func (s *AppSupportCacheScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if s.home == "" {
		return nil, nil
	}
	root := filepath.Join(s.home, "Library/Application Support")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}
	skips := homePrefixes(s.home, redLineHomeDirs)

	var items []model.Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if cerr := checkpoint(ctx); cerr != nil {
			return items, cerr
		}
		appPath := filepath.Join(root, e.Name())
		// 红线目录（Chrome / iOS 备份等）整棵跳过。
		if underAny(appPath, skips) {
			continue
		}

		children, err := os.ReadDir(appPath)
		if err != nil {
			continue
		}
		for _, c := range children {
			if !c.IsDir() || !isCacheDirName(c.Name()) {
				continue
			}
			if cerr := checkpoint(ctx); cerr != nil {
				return items, cerr
			}
			p := filepath.Join(appPath, c.Name())
			it, ok := makeItem(ctx, p, e.Name()+"/"+c.Name(), model.CategoryUserCache, model.RiskMedium,
				"仅清该应用的缓存子目录，其余数据保留")
			if ok {
				items = append(items, it)
			}
		}
	}
	return items, nil
}

// isCacheDirName 判断子目录名是否表示缓存（Cache、Caches、Code Cache、GPUCache 等）。
func isCacheDirName(name string) bool {
	return strings.Contains(strings.ToLower(name), "cache")
}
