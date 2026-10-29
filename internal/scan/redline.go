package scan

import (
	"path/filepath"
	"strings"
)

// redLineHomeDirs 是速查卡里的红线目录（相对用户主目录）。
//
// 这些目录一旦删除不可恢复：浏览器用户数据、iOS 备份、Docker 虚拟磁盘。
// 扫描阶段就整棵跳过，避免出现"列出来却删不掉"的条目。
var redLineHomeDirs = []string{
	"Library/Application Support/Google",
	"Library/Application Support/MobileSync",
	"Library/Containers/com.docker.docker/Data/vms",
}

// homePrefixes 把相对主目录的路径展开成绝对路径。
func homePrefixes(home string, rels []string) []string {
	out := make([]string, 0, len(rels))
	for _, rel := range rels {
		out = append(out, filepath.Join(home, rel))
	}
	return out
}

// underAny 判断 path 是否等于某个前缀目录或位于其下。
// 用路径分隔符边界比较，避免 /a/b 误匹配 /a/bc。
func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if path == p || strings.HasPrefix(path, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
