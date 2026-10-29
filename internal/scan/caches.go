package scan

import (
	"context"
	"os"
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
		{"Library/Saved Application State", "应用保存状态", "窗口位置等状态信息，会重新生成"},
	}
	// ~/Library/Caches 与 ~/.cache 都按直接子目录拆分，让用户能按应用
	// 单独清理。不产出父目录条目：父目录本身是 guard 的根，拒绝整目录删除，
	// 列出来也只会变成只读项。
	items := s.cacheChildren(ctx, "Library/Caches", devCacheRelPaths())
	items = append(items, s.cacheChildren(ctx, ".cache", nil)...)
	for _, t := range targets {
		p := filepath.Join(s.home, t.rel)
		if it, ok := makeItem(ctx, p, t.name, model.CategoryUserCache, model.RiskLow, t.note); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

// cacheChildren 把某个缓存根目录的直接子目录各自作为一个清理项。
//
// rel 是相对主目录的缓存根（Library/Caches 或 .cache）；skip 非空时跳过
// 其中由其它扫描器负责的子目录。只取目录（跳过散文件与符号链接），
// 解析失败当作无内容。
//
// 归哪个类别按子目录名判断：命中开发者工具的（AndroidStudio2.3、go-build、
// ~/.cache/zig 等）归开发缓存，其余仍归应用缓存。缓存根里本来就混着两类
// 内容，只按「在哪个目录下」归类会让开发缓存以应用缓存的身份出现。
func (s *UserCacheScanner) cacheChildren(ctx context.Context, rel string, skip map[string]struct{}) []model.Item {
	root := filepath.Join(s.home, rel)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var items []model.Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := skip[rel+"/"+e.Name()]; ok {
			continue
		}
		if cerr := checkpoint(ctx); cerr != nil {
			return items
		}
		p := filepath.Join(root, e.Name())
		cat, note := model.CategoryUserCache, "~/"+rel+" 下的应用缓存，应用会自动重建"
		if isDevCacheName(e.Name()) {
			cat, note = model.CategoryDevCache, "开发者工具缓存，重新构建或打开时会自动重建"
		}
		if it, ok := makeItem(ctx, p, e.Name()+" 缓存", cat, model.RiskLow, note); ok {
			items = append(items, it)
		}
	}
	return items
}

// SystemCacheScanner 扫描系统级缓存与日志目录。
// 系统路径需要管理员权限，无权限时会被跳过；~/Library/Logs 不需要 sudo，
// 但按速查卡归入 E 类（系统缓存与日志），不与应用缓存混在一起。
type SystemCacheScanner struct {
	home string
}

// NewSystemCacheScanner 创建系统缓存扫描器。
func NewSystemCacheScanner(opts Options) *SystemCacheScanner {
	return &SystemCacheScanner{home: opts.Home}
}

// Name 实现 Scanner。
func (s *SystemCacheScanner) Name() string { return "系统缓存" }

// systemCacheTarget 描述一个系统级缓存/日志目标。
type systemCacheTarget struct {
	rel  string // 相对主目录；为空表示 path 是绝对路径
	path string
	name string
	note string
	risk model.Risk
}

// systemCacheTargets 返回全部系统级缓存/日志目标，风险等级按速查卡定。
func systemCacheTargets() []systemCacheTarget {
	return []systemCacheTarget{
		{path: "/Library/Caches", name: "系统缓存 (/Library/Caches)", note: "系统级缓存，部分需管理员权限", risk: model.RiskMedium},
		{path: "/private/var/log/asl", name: "系统日志归档 (asl)", note: "旧版系统日志，需 sudo 清理，收益低", risk: model.RiskMedium},
		{rel: "Library/Logs", name: "用户日志 (~/Library/Logs)", note: "应用诊断日志，删除后自动重建", risk: model.RiskLow},
	}
}

// Scan 实现 Scanner。
func (s *SystemCacheScanner) Scan(ctx context.Context) ([]model.Item, error) {
	var items []model.Item
	for _, t := range systemCacheTargets() {
		p := t.path
		if t.rel != "" {
			if s.home == "" {
				continue
			}
			p = filepath.Join(s.home, t.rel)
		}
		if it, ok := makeItem(ctx, p, t.name, model.CategorySystemCache, t.risk, t.note); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
