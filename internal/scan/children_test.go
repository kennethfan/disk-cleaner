package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestChildrenListsAndSorts(t *testing.T) {
	root := t.TempDir()

	// 小目录
	small := filepath.Join(root, "small")
	if err := os.MkdirAll(small, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(small, "a"), make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}

	// 大目录
	big := filepath.Join(root, "big")
	if err := os.MkdirAll(big, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(big, "b"), make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}

	// 单文件
	if err := os.WriteFile(filepath.Join(root, "single.txt"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewChildScanner()
	items, err := s.Children(context.Background(), root)
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("应返回 3 项, got %d", len(items))
	}

	// 必须按体积降序：big(5000) > single(100) > small(10)
	if items[0].DisplayName != "big" {
		t.Errorf("第一项应为 big, got %s", items[0].DisplayName)
	}
	if items[1].DisplayName != "single.txt" {
		t.Errorf("第二项应为 single.txt, got %s", items[1].DisplayName)
	}
	if items[2].DisplayName != "small" {
		t.Errorf("第三项应为 small, got %s", items[2].DisplayName)
	}

	// 目录标记正确
	if !items[0].IsDir {
		t.Error("big 应标记为目录")
	}
	if items[1].IsDir {
		t.Error("single.txt 不应标记为目录")
	}

	// 大小正确
	if items[0].Size != 5000 {
		t.Errorf("big 大小 = %d, 期望 5000", items[0].Size)
	}
}

func TestChildrenMissingDir(t *testing.T) {
	s := NewChildScanner()
	if _, err := s.Children(context.Background(), "/nonexistent/path/xyz"); err == nil {
		t.Error("不存在的目录应返回错误")
	}
}

func TestChildrenEmptyDir(t *testing.T) {
	root := t.TempDir()
	s := NewChildScanner()
	items, err := s.Children(context.Background(), root)
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("空目录应返回 0 项, got %d", len(items))
	}
}

// TestChildrenRespectsCancel 验证取消信号能中断扫描。
func TestChildrenRespectsCancel(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		d := filepath.Join(root, string(rune('a'+i)))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	s := NewChildScanner()
	_, err := s.Children(ctx, root)
	if err == nil {
		t.Error("已取消的上下文应返回错误")
	}
}
