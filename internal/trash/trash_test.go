package trash

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFile(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, "Trash")
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tr, err := New(trashDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dest, err := tr.Move(src)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("源文件应已被移走")
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("读取落点: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("内容不符: %q", data)
	}
}

func TestMoveDirectory(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, "Trash")
	src := filepath.Join(dir, "sub")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tr, _ := New(trashDir)
	dest, err := tr.Move(src)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "nested", "f.txt")); err != nil {
		t.Errorf("目录内容未保留: %v", err)
	}
}

// TestMoveAvoidsOverwrite 是核心安全测试：废纸篓已有同名条目时不得覆盖。
func TestMoveAvoidsOverwrite(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, "Trash")
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// 预置一个同名文件在废纸篓中。
	existing := filepath.Join(trashDir, "dup.txt")
	if err := os.WriteFile(existing, []byte("原有内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(dir, "dup.txt")
	if err := os.WriteFile(src, []byte("新内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	tr, _ := New(trashDir)
	dest, err := tr.Move(src)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if dest == existing {
		t.Fatal("落点与已有文件相同，说明发生了覆盖")
	}
	if filepath.Base(dest) != "dup 2.txt" {
		t.Errorf("落点命名应为 'dup 2.txt'，实际 %q", filepath.Base(dest))
	}

	// 原文件必须完好。
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "原有内容" {
		t.Errorf("废纸篓原有文件被破坏: %q", data)
	}
}

func TestMoveMissingSource(t *testing.T) {
	dir := t.TempDir()
	tr, _ := New(filepath.Join(dir, "Trash"))
	if _, err := tr.Move(filepath.Join(dir, "nope.txt")); !errors.Is(err, ErrSourceMissing) {
		t.Errorf("期望 ErrSourceMissing, got %v", err)
	}
}

func TestNewRejectsEmptyDir(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Error("空废纸篓目录应报错")
	}
}

// TestCopyDirPreservesStructure 直接测试跨卷回退路径所用的 copyDir。
func TestCopyDirPreservesStructure(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a/b/deep.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "a", "b", "deep.txt"))
	if err != nil {
		t.Fatalf("深层文件未复制: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("内容不符: %q", data)
	}

	// 符号链接应按链接本身复制，而不是解引用成普通文件。
	info, err := os.Lstat(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatalf("链接未复制: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("符号链接被解引用成了普通文件")
	}
}

// TestCopyFilePreservesContent 测试单文件复制。
func TestCopyFilePreservesContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "s.bin")
	payload := []byte("payload-12345")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "d.bin")
	if err := copyFile(src, dst, 0o600); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("内容不符: %q", got)
	}
}

// TestUniqueDestNumbering 验证连续冲突时的序号递增。
func TestUniqueDestNumbering(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, "Trash")
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// 预占 base、2、3 三个名字。
	for _, n := range []string{"f.txt", "f 2.txt", "f 3.txt"} {
		if err := os.WriteFile(filepath.Join(trashDir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tr, _ := New(trashDir)
	dest, err := tr.uniqueDest("f.txt")
	if err != nil {
		t.Fatalf("uniqueDest: %v", err)
	}
	if filepath.Base(dest) != "f 4.txt" {
		t.Errorf("期望 'f 4.txt', got %q", filepath.Base(dest))
	}
}
