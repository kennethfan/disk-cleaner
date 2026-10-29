package scan

import (
	"context"
	"path/filepath"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// DevCacheScanner 扫描开发者工具缓存。
// 实测这类目录常是磁盘占用大头（例如 ~/.npm 可达数 GB）。
type DevCacheScanner struct {
	home string
}

// NewDevCacheScanner 创建开发缓存扫描器。
func NewDevCacheScanner(opts Options) *DevCacheScanner {
	return &DevCacheScanner{home: opts.Home}
}

// Name 实现 Scanner。
func (s *DevCacheScanner) Name() string { return "开发缓存" }

// devTarget 描述一个开发缓存目标。
type devTarget struct {
	rel  string // 相对主目录
	name string
	risk model.Risk
	note string
	// dirOnly 为 true 时，只有目标确实是目录才计入（用于某些以文件形式存在的缓存）。
	dirOnly bool
}

// devTargets 是全部已知开发缓存位置。
// 不存在的目标会被 makeItem 自动跳过，因此列表可以放全，无需预先探测。
func devTargets() []devTarget {
	return []devTarget{
		// --- Xcode 相关 ---
		{"Library/Developer/Xcode/DerivedData", "Xcode DerivedData", model.RiskLow, "编译中间产物，重新编译即可生成", true},
		{"Library/Developer/Xcode/iOS DeviceSupport", "Xcode iOS DeviceSupport", model.RiskMedium, "真机调试符号，下次连接设备会重新生成", true},
		{"Library/Developer/CoreSimulator/Caches", "iOS 模拟器缓存", model.RiskLow, "模拟器运行缓存", true},
		{"Library/Developer/CoreSimulator/Devices", "iOS 模拟器镜像", model.RiskMedium, "删除后模拟器需重新下载创建", true},

		// --- JS / Node ---
		{".npm/_cacache", "npm 缓存 (~/.npm/_cacache)", model.RiskLow, "npm 包缓存，重新安装时按需下载", true},
		{".yarn/cache", "Yarn 缓存", model.RiskLow, "Yarn 包缓存", true},
		{"Library/Caches/Yarn", "Yarn 缓存 (~/Library/Caches/Yarn)", model.RiskLow, "Yarn 包缓存", true},
		{".pnpm-store", "pnpm 存储", model.RiskLow, "pnpm 内容寻址存储", true},
		{"Library/Caches/pnpm", "pnpm 缓存 (~/Library/Caches/pnpm)", model.RiskLow, "pnpm 包缓存 / store", true},
		{".bun/install/cache", "Bun 缓存", model.RiskLow, "Bun 包缓存", true},

		// --- JVM / Android ---
		{".gradle/caches", "Gradle 缓存", model.RiskLow, "依赖缓存，重新构建时下载", true},
		{".m2/repository", "Maven 本地仓库", model.RiskMedium, "已下载依赖，删除后需重新拉取", true},

		// --- 其他语言生态 ---
		{".cargo/registry", "Cargo registry 缓存", model.RiskLow, "Rust crate 缓存", true},
		{"go/pkg/mod/cache/download", "Go 模块下载缓存", model.RiskLow, "Go module 压缩包缓存", true},
		{"Library/Caches/go-build", "Go 编译缓存 (go-build)", model.RiskLow, "编译中间产物，重新编译即可生成", true},
		{"Library/Caches/JetBrains", "JetBrains IDE 索引缓存", model.RiskLow, "IDE 索引，重开会重建", true},
		{"Library/Caches/pip", "pip 缓存", model.RiskLow, "Python 包缓存", true},

		// --- 容器与虚拟化 ---
		{"Library/Containers/com.docker.docker/Data/vms", "Docker 虚拟机数据", model.RiskHigh,
			"包含全部镜像与容器，整目录删除即丢失；清镜像请用 docker system prune -a", true},
	}
}

// devCacheRelPaths 返回全部开发缓存目标的相对路径（相主目录）。
//
// ~/Library/Caches 拆分时用它跳过这些子目录：同一目录由开发缓存扫描器
// 负责，避免被用户缓存与开发缓存重复统计（也避免重复遍历大目录）。
func devCacheRelPaths() map[string]struct{} {
	targets := devTargets()
	paths := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		paths[t.rel] = struct{}{}
	}
	return paths
}

// readonlyDevTargets 是仅展示、不可清理的目标：整目录删除会丢用户数据。
var readonlyDevTargets = map[string]struct{}{
	"Library/Containers/com.docker.docker/Data/vms": {},
}

// Scan 实现 Scanner。
func (s *DevCacheScanner) Scan(ctx context.Context) ([]model.Item, error) {
	if s.home == "" {
		return nil, nil
	}
	var items []model.Item
	for _, t := range devTargets() {
		p := filepath.Join(s.home, t.rel)
		it, ok := makeItem(ctx, p, t.name, model.CategoryDevCache, t.risk, t.note)
		if !ok {
			continue
		}
		if t.dirOnly && !it.IsDir {
			continue
		}
		if _, ro := readonlyDevTargets[t.rel]; ro {
			it.ReadOnly = true
		}
		items = append(items, it)
	}
	return items, nil
}
