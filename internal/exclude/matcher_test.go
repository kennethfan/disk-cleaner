package exclude

import (
	"path/filepath"
	"testing"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

const testHome = "/Users/tester"

func mustNew(t *testing.T, rules []string) *Matcher {
	t.Helper()
	m, err := New(rules, testHome)
	if err != nil {
		t.Fatalf("New(%v): %v", rules, err)
	}
	return m
}

// TestAbsPathRule 验证绝对路径规则会排除该目录及其全部子项。
func TestAbsPathRule(t *testing.T) {
	m := mustNew(t, []string{testHome + "/Library/Caches/JetBrains"})

	cases := []struct {
		path string
		want bool
	}{
		{testHome + "/Library/Caches/JetBrains", true},
		{testHome + "/Library/Caches/JetBrains/IntelliJIdea", true},
		{testHome + "/Library/Caches/JetBrains/a/b/c", true},
		{testHome + "/Library/Caches/Google", false},
		// 前缀相似但不属于该目录：不能被误排除。
		{testHome + "/Library/Caches/JetBrains2", false},
		{testHome + "/Library/Caches/JetBrains-old", false},
	}
	for _, c := range cases {
		if got := m.Matches(c.path); got != c.want {
			t.Errorf("Matches(%q) = %v, 期望 %v", c.path, got, c.want)
		}
	}
}

// TestTildeExpansion 验证 ~ 会被展开为主目录。
func TestTildeExpansion(t *testing.T) {
	m := mustNew(t, []string{"~/Library/Caches/JetBrains"})

	if !m.Matches(testHome + "/Library/Caches/JetBrains/x") {
		t.Error("~ 未正确展开")
	}
}

// TestSegmentRule 验证目录名规则匹配任意层级。
func TestSegmentRule(t *testing.T) {
	m := mustNew(t, []string{"node_modules"})

	cases := []struct {
		path string
		want bool
	}{
		{"/Users/tester/project/node_modules", true},
		{"/Users/tester/project/node_modules/react/index.js", true},
		{"/Users/tester/a/b/c/node_modules/x", true},
		{"/Users/tester/project/src", false},
		// 名字包含但不等于：不应命中。
		{"/Users/tester/project/my_node_modules", false},
		{"/Users/tester/project/node_modules_backup", false},
	}
	for _, c := range cases {
		if got := m.Matches(c.path); got != c.want {
			t.Errorf("Matches(%q) = %v, 期望 %v", c.path, got, c.want)
		}
	}
}

// TestGlobRule 验证通配符只匹配最后一段。
func TestGlobRule(t *testing.T) {
	m := mustNew(t, []string{"*.log"})

	cases := []struct {
		path string
		want bool
	}{
		{"/Users/tester/a/app.log", true},
		{"/Users/tester/a/b/c/trace.log", true},
		{"/Users/tester/a/app.txt", false},
		// 目录名本身是 xxx.log 也算匹配（最后一段）。
		{"/Users/tester/a/archive.log/file.txt", false},
	}
	for _, c := range cases {
		if got := m.Matches(c.path); got != c.want {
			t.Errorf("Matches(%q) = %v, 期望 %v", c.path, got, c.want)
		}
	}
}

// TestEmptyMatcher 验证空规则不排除任何内容。
func TestEmptyMatcher(t *testing.T) {
	m := mustNew(t, nil)
	if !m.Empty() {
		t.Error("空规则应报告 Empty")
	}
	if m.Matches("/any/path") {
		t.Error("空规则不应排除任何路径")
	}

	// nil Matcher 也必须安全可用。
	var nilM *Matcher
	if nilM.Matches("/any/path") {
		t.Error("nil Matcher 不应排除任何路径")
	}
	if !nilM.Empty() {
		t.Error("nil Matcher 应报告 Empty")
	}
}

// TestSkipsBlankAndComment 验证空白与注释行被忽略。
func TestSkipsBlankAndComment(t *testing.T) {
	m := mustNew(t, []string{"", "   ", "# 这是注释", "node_modules"})
	if got := len(m.Rules()); got != 1 {
		t.Errorf("规则数 = %d, 期望 1", got)
	}
}

// TestInvalidGlobRejected 验证错误通配符会报错而非静默忽略。
func TestInvalidGlobRejected(t *testing.T) {
	if _, err := New([]string{"[invalid"}, testHome); err == nil {
		t.Error("无效通配符应返回错误")
	}
}

// TestFilter 验证过滤返回正确的保留项与统计。
func TestFilter(t *testing.T) {
	m := mustNew(t, []string{"node_modules"})

	items := []model.Item{
		{Path: "/Users/tester/a/node_modules", Size: 100},
		{Path: "/Users/tester/a/src", Size: 200},
		{Path: "/Users/tester/b/node_modules", Size: 300},
		{Path: "/Users/tester/b/src", Size: 400},
	}

	kept, count, bytes := m.Filter(items)
	if count != 2 {
		t.Errorf("排除数 = %d, 期望 2", count)
	}
	if bytes != 400 {
		t.Errorf("排除字节 = %d, 期望 400", bytes)
	}
	if len(kept) != 2 {
		t.Fatalf("保留数 = %d, 期望 2", len(kept))
	}
	for _, it := range kept {
		if filepath.Base(it.Path) != "src" {
			t.Errorf("保留了不该保留的项: %s", it.Path)
		}
	}
}

// TestRulesReturnsOriginals 验证 Rules 返回用户原始输入。
func TestRulesReturnsOriginals(t *testing.T) {
	m := mustNew(t, []string{"~/a/b", "node_modules", "*.log"})
	got := m.Rules()
	want := []string{"~/a/b", "node_modules", "*.log"}
	if len(got) != len(want) {
		t.Fatalf("规则数 = %d, 期望 %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("规则[%d] = %q, 期望 %q", i, got[i], want[i])
		}
	}
}
