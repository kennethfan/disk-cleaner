// Package guard 是删除操作的最后一道安全闸门。
//
// 设计原则：默认拒绝。只有同时满足以下条件的目标才被允许操作：
//  1. 路径已清理为绝对路径，且不含 ".." 穿越成分；
//  2. 路径位于某个显式登记的白名单根目录之下（且不等于根目录本身）；
//  3. 路径不是受保护的系统/用户关键目录。
package guard

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrEmptyPath 路径为空。
	ErrEmptyPath = errors.New("路径为空")
	// ErrRelativePath 路径不是绝对路径。
	ErrRelativePath = errors.New("路径必须是绝对路径")
	// ErrTraversal 路径包含 .. 穿越成分。
	ErrTraversal = errors.New("路径包含 .. 穿越成分")
	// ErrOutsideRoots 路径不在任何允许的根目录之下。
	ErrOutsideRoots = errors.New("路径不在允许的清理范围内")
	// ErrRootItself 目标就是根目录本身，拒绝整目录删除。
	ErrRootItself = errors.New("拒绝操作根目录本身")
	// ErrProtected 目标属于受保护路径。
	ErrProtected = errors.New("目标属于受保护路径")
	// ErrHomeItself 目标是用户主目录本身。
	ErrHomeItself = errors.New("拒绝操作用户主目录本身")
)

// protectedExact 是完全禁止作为清理目标的路径（精确匹配）。
var protectedExact = []string{
	"/",
	"/System",
	"/Library",
	"/Applications",
	"/Users",
	"/private",
	"/bin",
	"/sbin",
	"/usr",
	"/etc",
	"/var",
	"/tmp",
	"/cores",
	"/opt",
	"/Volumes",
}

// protectedPrefixes 下的任何路径都禁止操作。
// 这些目录要么是系统运行必需，要么是用户不可替代的数据。
var protectedPrefixes = []string{
	"/System/",
	"/usr/",
	"/bin/",
	"/sbin/",
	"/private/var/db/",
	"/Library/Apple/",
	"/Library/Extensions/",
	"/Library/LaunchDaemons/",
	"/Library/LaunchAgents/",
}

// homeProtectedSuffixes 是主目录下禁止操作的相对路径（相对 ~）。
var homeProtectedSuffixes = []string{
	"Documents",
	"Desktop",
	"Pictures",
	"Movies",
	"Music",
	"Library/Mobile Documents",
	"Library/Keychains",
	"Library/Mail",
	"Library/Messages",
	"Library/Safari",
	// 浏览器用户数据（书签/密码/扩展）都在这里，删了等于重置浏览器。
	// Chrome 的纯缓存位于 ~/Library/Caches/Google，不受此条影响。
	"Library/Application Support/Google",
	// iOS 设备备份，删了不可恢复。
	"Library/Application Support/MobileSync",
	// Docker 虚拟磁盘，含全部镜像与容器；只能经 docker system prune 清理。
	"Library/Containers/com.docker.docker/Data/vms",
	".ssh",
	".gnupg",
}

// Guard 持有本次运行允许操作的根目录白名单。
type Guard struct {
	roots []string
	home  string
}

// New 创建 Guard。roots 会被清理、去重并展开为绝对路径。
// home 传空字符串时跳过主目录保护检查。
func New(roots []string, home string) (*Guard, error) {
	g := &Guard{home: filepath.Clean(home)}
	seen := make(map[string]struct{}, len(roots))
	for _, r := range roots {
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, fmt.Errorf("解析根目录 %q: %w", r, err)
		}
		abs = filepath.Clean(abs)
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		g.roots = append(g.roots, abs)
	}
	if len(g.roots) == 0 {
		return nil, errors.New("未配置任何允许的根目录")
	}
	return g, nil
}

// Roots 返回允许操作的根目录副本。
func (g *Guard) Roots() []string {
	out := make([]string, len(g.roots))
	copy(out, g.roots)
	return out
}

// Check 校验 target 是否允许被清理。返回 nil 表示放行。
func (g *Guard) Check(target string) error {
	if strings.TrimSpace(target) == "" {
		return ErrEmptyPath
	}
	if !filepath.IsAbs(target) {
		return fmt.Errorf("%w: %q", ErrRelativePath, target)
	}

	// 显式拒绝任何 ".." 成分，而不是依赖 Clean 后的结果。
	// 这样 /a/b/../../etc 这种输入在清理前就被拦下，避免逻辑歧义。
	for _, seg := range strings.Split(filepath.ToSlash(target), "/") {
		if seg == ".." {
			return fmt.Errorf("%w: %q", ErrTraversal, target)
		}
	}

	abs := filepath.Clean(target)

	// 受保护精确路径。
	for _, p := range protectedExact {
		if abs == p {
			return fmt.Errorf("%w: %s", ErrProtected, abs)
		}
	}
	// 受保护前缀。
	slashAbs := abs + "/"
	for _, p := range protectedPrefixes {
		if strings.HasPrefix(slashAbs, p) {
			return fmt.Errorf("%w: %s", ErrProtected, abs)
		}
	}
	// 主目录本身及主目录下的关键数据目录。
	if g.home != "" {
		if abs == g.home {
			return ErrHomeItself
		}
		for _, suffix := range homeProtectedSuffixes {
			full := filepath.Join(g.home, suffix)
			if abs == full || strings.HasPrefix(slashAbs, full+"/") {
				return fmt.Errorf("%w: %s", ErrProtected, abs)
			}
		}
	}

	// 必须落在某个白名单根目录之下，且不能是根目录本身。
	for _, root := range g.roots {
		if abs == root {
			return fmt.Errorf("%w: %s", ErrRootItself, abs)
		}
		if strings.HasPrefix(slashAbs, root+"/") {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrOutsideRoots, abs)
}

// Allowed 是 Check 的布尔包装，便于在 UI 中做批量预筛选。
func (g *Guard) Allowed(target string) bool {
	return g.Check(target) == nil
}
