package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kennethfan/disk-cleaner/internal/exclude"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

func TestTargetScannerListsDir(t *testing.T) {
	root := t.TempDir()

	// 建一个子目录与一个文件。
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.bin"), make([]byte, 3000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "single.txt"), make([]byte, 500), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewTargetScanner([]string{root}, 1<<20)
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// 期望：根目录自身 + sub + single.txt
	if len(items) != 3 {
		t.Fatalf("项数 = %d, 期望 3", len(items))
	}

	// 全部必须是只读。
	for _, it := range items {
		if !it.ReadOnly {
			t.Errorf("指定目录扫描的项应标记 ReadOnly: %s", it.Path)
		}
		if it.Category != model.CategoryLargeFile {
			t.Errorf("类别应为大文件, got %s", it.Category)
		}
	}

	// 按体积降序：根目录(3500) > sub(3000) > single(500)
	if items[0].Path != root {
		t.Errorf("第一项应为根目录, got %s", items[0].Path)
	}
}

func TestTargetScannerSingleFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "one.bin")
	if err := os.WriteFile(f, make([]byte, 1234), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewTargetScanner([]string{f}, 1<<20)
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("项数 = %d, 期望 1", len(items))
	}
	if items[0].Size != 1234 {
		t.Errorf("大小 = %d, 期望 1234", items[0].Size)
	}
	if items[0].IsDir {
		t.Error("不应标记为目录")
	}
}

func TestTargetScannerMissingPath(t *testing.T) {
	s := NewTargetScanner([]string{"/nonexistent/xyz"}, 1<<20)
	_, err := s.Scan(context.Background())
	if err == nil {
		t.Error("全部路径都不存在时应返回错误")
	}
}

// TestTargetScannerPartialSuccess 验证部分路径存在时仍返回结果。
func TestTargetScannerPartialSuccess(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.bin"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewTargetScanner([]string{"/nonexistent/xyz", root}, 1<<20)
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("部分成功不应报错, got %v", err)
	}
	if len(items) == 0 {
		t.Error("应返回存在目录的结果")
	}
}

// TestAllUsesTargetScannerWhenPathsGiven 验证指定 --path 时跳过预设扫描器。
func TestAllUsesTargetScannerWhenPathsGiven(t *testing.T) {
	opts := DefaultOptions()
	opts.Paths = []string{"/tmp"}

	scanners := All(opts)
	if len(scanners) != 1 {
		t.Fatalf("扫描器数 = %d, 期望 1", len(scanners))
	}
	if scanners[0].Name() != "指定目录" {
		t.Errorf("应使用指定目录扫描器, got %s", scanners[0].Name())
	}
}

// TestScanAllAppliesExclude 验证排除规则在 ScanAll 出口生效并计入统计。
func TestScanAllAppliesExclude(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "x"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.Home = home
	opts.Concurrency = 2

	// 未排除时应有用户缓存项。
	report := ScanAll(context.Background(), All(opts), opts)
	var found bool
	for _, it := range report.Items {
		if it.Path == cache {
			found = true
		}
	}
	if !found {
		t.Fatal("未排除时应发现用户缓存项")
	}
	if report.ExcludedCount != 0 {
		t.Errorf("未配置排除时 ExcludedCount 应为 0, got %d", report.ExcludedCount)
	}
}

// TestScanAllRecordsTargetPaths 验证指定目录模式下 Report 记录了目标路径。
func TestScanAllRecordsTargetPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.bin"), make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.Paths = []string{root}

	report := ScanAll(context.Background(), All(opts), opts)
	if len(report.TargetPaths) != 1 || report.TargetPaths[0] != root {
		t.Errorf("TargetPaths = %v, 期望 [%s]", report.TargetPaths, root)
	}
}

// TestScanAllExcludeActuallyFilters 真正验证排除规则会过滤掉项目并计入统计。
func TestScanAllExcludeActuallyFilters(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "x"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := exclude.New([]string{cache}, home)
	if err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.Home = home
	opts.Concurrency = 2
	opts.Exclude = m

	report := ScanAll(context.Background(), All(opts), opts)

	// 用户缓存项应被排除。
	for _, it := range report.Items {
		if it.Path == cache {
			t.Error("被排除的路径不应出现在结果中")
		}
	}
	if report.ExcludedCount == 0 {
		t.Error("ExcludedCount 应大于 0")
	}
	if report.ExcludedBytes == 0 {
		t.Error("ExcludedBytes 应大于 0")
	}
}

// TestChildScannerAppliesExclude 验证展开子项时排除规则同样生效。
func TestChildScannerAppliesExclude(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep")
	drop := filepath.Join(root, "drop")
	for _, d := range []string{keep, drop} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "f"), make([]byte, 100), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m, err := exclude.New([]string{"drop"}, root)
	if err != nil {
		t.Fatal(err)
	}

	s := NewChildScannerWithExclude(m)
	items, err := s.Children(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	for _, it := range items {
		if filepath.Base(it.Path) == "drop" {
			t.Error("被排除的子项不应出现在展开结果中")
		}
	}
	if len(items) != 1 {
		t.Errorf("子项数 = %d, 期望 1（只有 keep）", len(items))
	}
}
