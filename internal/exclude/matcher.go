// Package exclude 判断某个路径是否被用户排除在扫描范围之外。
//
// 排除规则支持三种写法，按以下顺序判定：
//
//  1. 绝对路径（支持 ~ 展开）：该路径及其全部子项都排除
//  2. 目录/文件名模式：匹配路径中任意一层与模式同名的那一段
//  3. 通配符：按 filepath.Match 匹配路径最后一段（文件名或目录名）
//
// 设计要点：所有判断都在清理项产出后统一过滤，而不是让每个扫描器
// 各写一遍，避免规则不一致。
package exclude

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// ErrEmptyPattern 规则为空。
var ErrEmptyPattern = errors.New("排除规则为空")

// ruleKind 是规则的类型。
type ruleKind int

const (
	// kindPath 绝对路径规则：排除该路径及其子项。
	kindPath ruleKind = iota
	// kindSegment 目录/文件名模式：匹配任意一层。
	kindSegment
	// kindGlob 通配符：匹配最后一段。
	kindGlob
)

// rule 是一条编译后的排除规则。
type rule struct {
	kind ruleKind
	// original 是用户写的原始文本，用于界面展示与错误信息。
	original string
	// absPath 仅 kindPath 使用，已展开 ~ 且清理过。
	absPath string
	// pattern 仅 kindSegment / kindGlob 使用。
	pattern string
}

// Matcher 是一组排除规则的集合。
//
// 零值 Matcher 可用：不排除任何路径。这避免了调用方到处判空。
type Matcher struct {
	rules []rule
	home  string
}

// New 编译排除规则。home 用于展开 ~，传空串时使用 os.UserHomeDir。
//
// 空字符串与纯空白行会被忽略；# 开头视为注释。
// 任何一条规则无效都会返回错误，而不是静默跳过——排除规则失效
// 会导致用户以为已排除的目录仍被删除。
func New(patterns []string, home string) (*Matcher, error) {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		home = h
	}
	m := &Matcher{home: home}

	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}

		r, err := compileRule(p, home)
		if err != nil {
			return nil, err
		}
		m.rules = append(m.rules, r)
	}
	return m, nil
}

// compileRule 把一条用户输入编译成规则。
func compileRule(raw, home string) (rule, error) {
	expanded := expandHome(raw, home)

	// 含路径分隔符的视为路径规则；否则视为名称模式。
	if strings.ContainsRune(expanded, filepath.Separator) || filepath.IsAbs(expanded) {
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return rule{}, err
		}
		return rule{kind: kindPath, original: raw, absPath: filepath.Clean(abs)}, nil
	}

	// 含通配符的按 glob 处理。
	if strings.ContainsAny(expanded, "*?[") {
		// 先验证模式语法，避免运行期才发现写错。
		if _, err := filepath.Match(expanded, "probe"); err != nil {
			return rule{}, errors.New("通配符语法错误: " + raw)
		}
		return rule{kind: kindGlob, original: raw, pattern: expanded}, nil
	}

	return rule{kind: kindSegment, original: raw, pattern: expanded}, nil
}

// expandHome 把开头的 ~ 替换为主目录。
func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// Empty 报告是否没有任何规则。
func (m *Matcher) Empty() bool {
	return m == nil || len(m.rules) == 0
}

// Rules 返回原始规则文本，供界面展示。
func (m *Matcher) Rules() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.rules))
	for _, r := range m.rules {
		out = append(out, r.original)
	}
	return out
}

// Matches 判断 path 是否应被排除。
func (m *Matcher) Matches(path string) bool {
	if m == nil || len(m.rules) == 0 {
		return false
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)

	// 预先算好路径分段，供名称模式复用。
	segments := splitSegments(abs)
	base := filepath.Base(abs)

	for _, r := range m.rules {
		switch r.kind {
		case kindPath:
			// 该路径本身，或其任意子项。
			if abs == r.absPath || strings.HasPrefix(abs+string(filepath.Separator), r.absPath+string(filepath.Separator)) {
				return true
			}
		case kindSegment:
			for _, seg := range segments {
				if seg == r.pattern {
					return true
				}
			}
		case kindGlob:
			if ok, _ := filepath.Match(r.pattern, base); ok {
				return true
			}
		}
	}
	return false
}

// Filter 返回未被排除的项，并给出被排除的数量与字节数。
//
// 用新的切片承载结果，避免修改调用方传入的切片。
func (m *Matcher) Filter(items []model.Item) (kept []model.Item, excludedCount int, excludedBytes int64) {
	if m == nil || len(m.rules) == 0 {
		return items, 0, 0
	}

	kept = make([]model.Item, 0, len(items))
	for _, it := range items {
		if m.Matches(it.Path) {
			excludedCount++
			excludedBytes += it.Size
			continue
		}
		kept = append(kept, it)
	}
	return kept, excludedCount, excludedBytes
}

// splitSegments 把绝对路径拆成非空分段。
func splitSegments(abs string) []string {
	parts := strings.Split(filepath.ToSlash(abs), "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
