package clean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kennethfan/disk-cleaner/internal/guard"
	"github.com/kennethfan/disk-cleaner/internal/model"
	"github.com/kennethfan/disk-cleaner/internal/trash"
)

// newTestEngine 构造一个以 tmp 为家目录、废纸篓在 tmp/Trash 的引擎。
func newTestEngine(t *testing.T, home string) *Engine {
	t.Helper()
	tr, err := trash.New(filepath.Join(home, "Trash"))
	if err != nil {
		t.Fatalf("trash.New: %v", err)
	}
	g, err := guard.New(DefaultRoots(home), home)
	if err != nil {
		t.Fatalf("guard.New: %v", err)
	}
	e, err := NewEngine(g, tr)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

func TestRunMovesAllowedItem(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches", "demo")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "f.bin"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine(t, home)
	res := e.Run([]model.Item{{
		Path: cache, DisplayName: "demo", Size: 5, IsDir: true,
	}})

	if res.Succeeded() != 1 {
		t.Fatalf("应成功 1 项，实际 %d（失败 %d）", res.Succeeded(), res.Failed())
	}
	if res.FreedBytes != 5 {
		t.Errorf("FreedBytes = %d, 期望 5", res.FreedBytes)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Error("源目录应已被移走")
	}
}

// TestRunRejectsOutsideRoots 确保白名单外的路径不会被移动。
func TestRunRejectsOutsideRoots(t *testing.T) {
	home := t.TempDir()
	outside := filepath.Join(home, "Documents", "important.txt")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine(t, home)
	res := e.Run([]model.Item{{Path: outside, DisplayName: "important"}})

	if len(res.Outcomes) != 1 {
		t.Fatalf("期望 1 个结果, got %d", len(res.Outcomes))
	}
	if !res.Outcomes[0].Skipped {
		t.Error("Documents 下的文件应被跳过")
	}
	if res.Outcomes[0].Err == nil {
		t.Error("跳过时应附带原因")
	}
	// 文件必须原封不动。
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("受保护文件被移动了: %v", err)
	}
}

// TestRunRejectsPermanentItems 确保标记为永久删除的项不走废纸篓流程。
func TestRunRejectsPermanentItems(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, "Downloads", "x.zip")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine(t, home)
	res := e.Run([]model.Item{{Path: p, DisplayName: "x", Permanent: true}})

	if !res.Outcomes[0].Skipped {
		t.Error("Permanent 项应被跳过")
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("Permanent 项不应被移动: %v", err)
	}
}

// TestRunContinuesAfterFailure 确保单项失败不中断整批。
func TestRunContinuesAfterFailure(t *testing.T) {
	home := t.TempDir()
	good := filepath.Join(home, "Library", "Caches", "ok")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine(t, home)
	res := e.Run([]model.Item{
		{Path: filepath.Join(home, "Library", "Caches", "missing"), DisplayName: "missing"},
		{Path: good, DisplayName: "ok"},
	})

	if len(res.Outcomes) != 2 {
		t.Fatalf("应有两个结果, got %d", len(res.Outcomes))
	}
	if res.Failed() != 1 {
		t.Errorf("应有 1 项失败, got %d", res.Failed())
	}
	if res.Succeeded() != 1 {
		t.Errorf("应有 1 项成功, got %d", res.Succeeded())
	}
}
