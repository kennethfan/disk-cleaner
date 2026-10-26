package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

func TestScanAllCollectsItems(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "a.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Home: home, LargeFileThreshold: 1 << 20, Concurrency: 2}
	report := ScanAll(context.Background(), All(opts), opts)

	if len(report.Items) == 0 {
		t.Fatal("应至少发现用户缓存项")
	}
	var found bool
	for _, it := range report.Items {
		if it.Path == cache {
			found = true
			if it.Size < 2048 {
				t.Errorf("缓存大小统计偏小: %d", it.Size)
			}
			if it.Category != model.CategoryUserCache {
				t.Errorf("类别应为用户缓存, got %s", it.Category)
			}
		}
	}
	if !found {
		t.Error("未找到用户缓存项")
	}
	if report.TotalBytes <= 0 {
		t.Error("TotalBytes 应大于 0")
	}
}

// TestScanAllTolerantToBrokenScanner 确保单个扫描器出错不影响其他扫描器。
func TestScanAllTolerantToBrokenScanner(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}

	opts := Options{Home: home, Concurrency: 2}
	scanners := []Scanner{
		NewUserCacheScanner(opts),
		brokenScanner{},
	}
	report := ScanAll(context.Background(), scanners, opts)

	if len(report.Errors) != 1 {
		t.Errorf("应收集到 1 个错误, got %d", len(report.Errors))
	}
	if len(report.Items) == 0 {
		t.Error("坏扫描器不应阻止好扫描器产出结果")
	}
}

type brokenScanner struct{}

func (brokenScanner) Name() string { return "坏扫描器" }
func (brokenScanner) Scan(context.Context) ([]model.Item, error) {
	return nil, os.ErrPermission
}

func TestUserCacheScannerSkipsMissing(t *testing.T) {
	home := t.TempDir() // 空目录，没有任何缓存
	s := NewUserCacheScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("不存在的目录不应产出项, got %d", len(items))
	}
}

func TestDevCacheScannerFindsKnownTarget(t *testing.T) {
	home := t.TempDir()
	npmCache := filepath.Join(home, ".npm", "_cacache")
	if err := os.MkdirAll(npmCache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npmCache, "x"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewDevCacheScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var found bool
	for _, it := range items {
		if it.Path == npmCache {
			found = true
		}
	}
	if !found {
		t.Error("未发现 npm 缓存")
	}
}

func TestLargeFileScannerThreshold(t *testing.T) {
	home := t.TempDir()
	dl := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatal(err)
	}
	// 小文件不应入选。
	if err := os.WriteFile(filepath.Join(dl, "small.bin"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	// 大文件应入选。
	if err := os.WriteFile(filepath.Join(dl, "big.bin"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLargeFileScanner(Options{Home: home, LargeFileThreshold: 2048})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("应只发现 1 个大文件, got %d", len(items))
	}
	if filepath.Base(items[0].Path) != "big.bin" {
		t.Errorf("发现的大文件不对: %s", items[0].Path)
	}
}

func TestJunkScannerMarksTrashPermanent(t *testing.T) {
	home := t.TempDir()
	trashDir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trashDir, "old.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewJunkScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var found bool
	for _, it := range items {
		if it.Path == trashDir {
			found = true
			if !it.Permanent {
				t.Error("废纸篓项必须标记为 Permanent")
			}
			if it.Risk != model.RiskHigh {
				t.Errorf("废纸篓风险应为高, got %v", it.Risk)
			}
		}
	}
	if !found {
		t.Error("未发现废纸篓项")
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := model.HumanBytes(c.in); got != c.want {
			t.Errorf("HumanBytes(%d) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// TestDedupePrefersSpecificCategory 验证同一路径重复时保留更具体的类别。
func TestDedupePrefersSpecificCategory(t *testing.T) {
	p := "/Users/x/Downloads/Camera.zip"
	items := []model.Item{
		{Path: p, Category: model.CategoryLargeFile, DisplayName: "大文件版", Size: 100},
		{Path: p, Category: model.CategoryJunk, DisplayName: "旧下载版", Size: 100},
	}
	got := dedupeByPath(items)
	if len(got) != 1 {
		t.Fatalf("应去重为 1 项, got %d", len(got))
	}
	if got[0].Category != model.CategoryJunk {
		t.Errorf("应保留垃圾文件类别（更具体）, got %s", got[0].Category)
	}
}

// TestDedupeIsOrderIndependent 验证去重结果与输入顺序无关。
func TestDedupeIsOrderIndependent(t *testing.T) {
	p := "/Users/x/Downloads/a.bin"
	a := model.Item{Path: p, Category: model.CategoryLargeFile, DisplayName: "A"}
	b := model.Item{Path: p, Category: model.CategoryJunk, DisplayName: "B"}

	first := dedupeByPath([]model.Item{a, b})
	second := dedupeByPath([]model.Item{b, a})

	if first[0].Category != second[0].Category {
		t.Errorf("结果依赖输入顺序: %s vs %s", first[0].Category, second[0].Category)
	}
}

// TestScanAllDedupesOverlap 验证总大小不会因重复路径被重复累加。
func TestScanAllDedupesOverlap(t *testing.T) {
	home := t.TempDir()
	dl := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatal(err)
	}
	// 一个又大又旧的文件，同时满足"大文件"和"旧下载"两个扫描器。
	f := filepath.Join(dl, "both.bin")
	if err := os.WriteFile(f, make([]byte, 8192), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -200)
	if err := os.Chtimes(f, old, old); err != nil {
		t.Fatal(err)
	}

	opts := Options{Home: home, LargeFileThreshold: 4096, Concurrency: 4}
	report := ScanAll(context.Background(), All(opts), opts)

	var count int
	for _, it := range report.Items {
		if it.Path == f {
			count++
		}
	}
	if count != 1 {
		t.Errorf("同一文件应只出现一次, got %d", count)
	}
}
