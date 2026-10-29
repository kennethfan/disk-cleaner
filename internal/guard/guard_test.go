package guard

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestCheckRejectsTraversal(t *testing.T) {
	home := "/Users/tester"
	g, err := New([]string{filepath.Join(home, "Library")}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 路径字面包含 ..，即使 Clean 后落在合法根目录下也必须拒绝。
	cases := []string{
		home + "/Library/../Documents/secret.txt",
		home + "/Library/Caches/../../.ssh/id_rsa",
		"/Users/tester/../../../etc/passwd",
	}
	for _, c := range cases {
		if err := g.Check(c); !errors.Is(err, ErrTraversal) {
			t.Errorf("Check(%q) = %v, 期望 ErrTraversal", c, err)
		}
	}
}

func TestCheckRejectsProtectedPaths(t *testing.T) {
	home := "/Users/tester"
	g, err := New([]string{"/", home + "/Library"}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		path string
		want error
	}{
		{"/", ErrProtected},
		{"/System/Library/Caches", ErrProtected},
		{"/usr/bin/ls", ErrProtected},
		{home, ErrHomeItself},
		{home + "/Documents/report.pdf", ErrProtected},
		{home + "/.ssh/id_rsa", ErrProtected},
		{home + "/Library/Keychains/login.keychain-db", ErrProtected},
		{home + "/Library/Mail", ErrProtected},
		// 速查卡红线：浏览器用户数据、iOS 备份、Docker 虚拟磁盘。
		{home + "/Library/Application Support/Google", ErrProtected},
		{home + "/Library/Application Support/Google/Chrome/Default/History", ErrProtected},
		{home + "/Library/Application Support/MobileSync/Backup", ErrProtected},
		{home + "/Library/Containers/com.docker.docker/Data/vms", ErrProtected},
		{home + "/Library/Containers/com.docker.docker/Data/vms/0/data.vmdk", ErrProtected},
	}
	for _, c := range cases {
		if err := g.Check(c.path); !errors.Is(err, c.want) {
			t.Errorf("Check(%q) = %v, 期望 %v", c.path, err, c.want)
		}
	}
}

func TestCheckRejectsRootItself(t *testing.T) {
	home := "/Users/tester"
	root := filepath.Join(home, "Library")
	g, err := New([]string{root}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := g.Check(root); !errors.Is(err, ErrRootItself) {
		t.Errorf("Check(根目录) = %v, 期望 ErrRootItself", err)
	}
}

func TestCheckRejectsOutsideRoots(t *testing.T) {
	home := "/Users/tester"
	g, err := New([]string{filepath.Join(home, "Library")}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := g.Check(home + "/Downloads/a.zip"); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("Check(根目录外) = %v, 期望 ErrOutsideRoots", err)
	}
}

func TestCheckAllowsValidPaths(t *testing.T) {
	home := "/Users/tester"
	g, err := New([]string{
		filepath.Join(home, "Library"),
		filepath.Join(home, ".npm"),
	}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []string{
		home + "/Library/Caches/com.apple.Safari",
		home + "/Library/Developer/Xcode/DerivedData",
		// Chrome 的纯缓存与 Slack 的 cache 子目录不属于红线，应继续可清理。
		home + "/Library/Caches/Google",
		home + "/Library/Application Support/Slack/Cache",
		home + "/.npm/_cacache",
	}
	for _, c := range cases {
		if err := g.Check(c); err != nil {
			t.Errorf("Check(%q) = %v, 期望放行", c, err)
		}
	}
}

func TestCheckRejectsRelativeAndEmpty(t *testing.T) {
	home := "/Users/tester"
	g, err := New([]string{filepath.Join(home, "Library")}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := g.Check("Library/Caches"); !errors.Is(err, ErrRelativePath) {
		t.Errorf("相对路径应被拒绝, got %v", err)
	}
	if err := g.Check(""); !errors.Is(err, ErrEmptyPath) {
		t.Errorf("空路径应被拒绝, got %v", err)
	}
}

func TestNewRejectsEmptyRoots(t *testing.T) {
	if _, err := New(nil, "/Users/tester"); err == nil {
		t.Error("空根目录应报错")
	}
}

// TestPrefixConfusion 确保 /Users/tester/Library2 不会被误判为 /Users/tester/Library 的子路径。
func TestPrefixConfusion(t *testing.T) {
	home := "/Users/tester"
	g, err := New([]string{home + "/Library"}, home)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := g.Check(home + "/Library2/secret"); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("前缀相似路径应被拒绝, got %v", err)
	}
}
