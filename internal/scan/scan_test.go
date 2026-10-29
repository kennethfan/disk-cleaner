package scan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

func TestScanAllCollectsItems(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	cacheChild := filepath.Join(cache, "com.example.app")
	if err := os.MkdirAll(cacheChild, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheChild, "a.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Home: home, LargeFileThreshold: 1 << 20, Concurrency: 2}
	report := ScanAll(context.Background(), All(opts), opts)

	if len(report.Items) == 0 {
		t.Fatal("应至少发现用户缓存项")
	}
	var found bool
	for _, it := range report.Items {
		if it.Path == cacheChild {
			found = true
			if it.Size < 2048 {
				t.Errorf("缓存大小统计偏小: %d", it.Size)
			}
			if it.Category != model.CategoryUserCache {
				t.Errorf("类别应为用户缓存, got %s", it.Category)
			}
		}
		if it.Path == cache {
			t.Errorf("~/Library/Caches 不应再作为整条出现: %s", it.Path)
		}
	}
	if !found {
		t.Error("未找到用户缓存子目录项")
	}
	if report.TotalBytes <= 0 {
		t.Error("TotalBytes 应大于 0")
	}
}

// TestScanAllTolerantToBrokenScanner 确保单个扫描器出错不影响其他扫描器。
func TestScanAllTolerantToBrokenScanner(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches", "app")
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

// TestJunkScannerSkipsIOSBackup 验证 iOS 设备备份不再作为可清理项出现：
// 速查卡把它列为红线，垃圾文件扫描器不应提供删除入口。
func TestJunkScannerSkipsIOSBackup(t *testing.T) {
	home := t.TempDir()
	backup := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "Info.plist"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewJunkScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, it := range items {
		if strings.Contains(it.Path, "MobileSync") {
			t.Errorf("iOS 备份不应出现在可清理项中: %s", it.Path)
		}
	}
}

// TestDevCacheMarksDockerReadOnly 验证 Docker 虚拟磁盘只展示、不可勾选，
// 并在说明里指向 docker system prune。
func TestDevCacheMarksDockerReadOnly(t *testing.T) {
	home := t.TempDir()
	vms := filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "vms")
	if err := os.MkdirAll(vms, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vms, "data.vmdk"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewDevCacheScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var found bool
	for _, it := range items {
		if it.Path != vms {
			continue
		}
		found = true
		if !it.ReadOnly {
			t.Error("Docker 虚拟磁盘应标记 ReadOnly")
		}
		if !strings.Contains(it.Note, "docker system prune") {
			t.Errorf("说明应指向 docker system prune, got %q", it.Note)
		}
	}
	if !found {
		t.Error("未发现 Docker 虚拟磁盘项")
	}
}

// TestLargeFileScannerSkipsRedLineDirs 验证红线目录即使含超大文件也不列出，
// 同时不影响同一扫描根下正常的 Application Support 子目录。
func TestLargeFileScannerSkipsRedLineDirs(t *testing.T) {
	home := t.TempDir()
	appSupport := filepath.Join(home, "Library", "Application Support")
	googleDir := filepath.Join(appSupport, "Google", "Chrome", "Default")
	slackDir := filepath.Join(appSupport, "Slack", "Attachments")
	for _, dir := range []string{googleDir, slackDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(googleDir, "History"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slackDir, "big.bin"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLargeFileScanner(Options{Home: home, LargeFileThreshold: 2048})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var google, slack bool
	for _, it := range items {
		if strings.Contains(it.Path, "Application Support/Google") {
			google = true
		}
		if strings.Contains(it.Path, "Application Support/Slack") {
			slack = true
		}
	}
	if google {
		t.Error("红线目录 Application Support/Google 中的文件不应被列出")
	}
	if !slack {
		t.Error("非红线目录 Application Support/Slack 中的大文件应正常列出")
	}
}

// TestUserCacheScannerSplitsCaches 验证 ~/Library/Caches 按直接子目录拆分：
// 每个应用可单独勾选，且不再产出父目录条目（避免父子重复计数）。
func TestUserCacheScannerSplitsCaches(t *testing.T) {
	home := t.TempDir()
	caches := filepath.Join(home, "Library", "Caches")
	chrome := filepath.Join(caches, "Google")
	geo := filepath.Join(caches, "GeoServices")
	jetbrains := filepath.Join(caches, "JetBrains") // 开发缓存，不归本扫描器
	for _, dir := range []string{chrome, geo, jetbrains} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "blob"), make([]byte, 1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := NewUserCacheScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	paths := make(map[string]bool, len(items))
	for _, it := range items {
		paths[it.Path] = true
		if it.Path == caches {
			t.Error("不应再产出 ~/Library/Caches 整条")
		}
	}
	for _, want := range []string{chrome, geo} {
		if !paths[want] {
			t.Errorf("缺少子目录项: %s", want)
		}
	}
	if paths[jetbrains] {
		t.Error("开发缓存子目录应由开发缓存扫描器产出，不应混进用户缓存")
	}
}

// TestUserCacheScannerSplitsDotCache 验证 ~/.cache 归用户缓存（速查卡 A 类），
// 并按子目录拆分（~/.cache 本身是 guard 根，整条列出会变成只读项）。
func TestUserCacheScannerSplitsDotCache(t *testing.T) {
	home := t.TempDir()
	dotCache := filepath.Join(home, ".cache")
	opencode := filepath.Join(dotCache, "opencode")
	if err := os.MkdirAll(opencode, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opencode, "blob"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := NewUserCacheScanner(Options{Home: home}).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var found bool
	for _, it := range items {
		if it.Path == dotCache {
			t.Error("不应产出 ~/.cache 整条（guard 会拒绝根目录本身）")
		}
		if it.Path != opencode {
			continue
		}
		found = true
		if it.Category != model.CategoryUserCache {
			t.Errorf("~/.cache/opencode 应归用户缓存, got %s", it.Category)
		}
		if it.Risk != model.RiskLow {
			t.Errorf("~/.cache/opencode 风险应为低, got %v", it.Risk)
		}
	}
	if !found {
		t.Error("~/.cache 的子目录应作为用户缓存项出现")
	}

	devItems, err := NewDevCacheScanner(Options{Home: home}).Scan(context.Background())
	if err != nil {
		t.Fatalf("DevCache Scan: %v", err)
	}
	for _, it := range devItems {
		if it.Path == opencode || it.Path == dotCache {
			t.Error("~/.cache 不应再由开发缓存扫描器产出")
		}
	}
}

// TestSystemCacheScannerIncludesUserLogs 验证 ~/Library/Logs 归系统缓存与日志
// （速查卡 E 类），不再混在用户缓存里。
func TestSystemCacheScannerIncludesUserLogs(t *testing.T) {
	home := t.TempDir()
	logs := filepath.Join(home, "Library", "Logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "app.log"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := NewSystemCacheScanner(Options{Home: home}).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var found bool
	for _, it := range items {
		if it.Path == logs {
			found = true
			if it.Category != model.CategorySystemCache {
				t.Errorf("~/Library/Logs 应归系统缓存与日志, got %s", it.Category)
			}
			if it.Risk != model.RiskLow {
				t.Errorf("~/Library/Logs 风险应为低, got %v", it.Risk)
			}
		}
	}
	if !found {
		t.Error("~/Library/Logs 应作为系统缓存项出现")
	}

	userItems, err := NewUserCacheScanner(Options{Home: home}).Scan(context.Background())
	if err != nil {
		t.Fatalf("UserCache Scan: %v", err)
	}
	for _, it := range userItems {
		if it.Path == logs {
			t.Error("~/Library/Logs 不应再由用户缓存扫描器产出")
		}
	}
}

// TestContainerCacheScannerOnlyCaches 验证 C 类扫描：只清容器内的
// Data/Library/Caches，不提供整个容器目录。
func TestContainerCacheScannerOnlyCaches(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, "Library", "Containers", "com.tencent.xinWeChat", "Data", "Library")
	cache := filepath.Join(base, "Caches")
	db := filepath.Join(base, "Application Support", "com.tencent.xinWeChat", "db_storage")
	for _, dir := range []string{cache, db} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cache, "media.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(db, "message.db"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewContainerCacheScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("应只产出容器缓存 1 项, got %d: %+v", len(items), items)
	}
	it := items[0]
	if it.Path != cache {
		t.Errorf("路径 = %s, 期望 %s", it.Path, cache)
	}
	if it.Risk != model.RiskLow {
		t.Errorf("容器缓存风险应为低, got %v", it.Risk)
	}
	if !strings.Contains(it.DisplayName, "微信") {
		t.Errorf("应显示应用名微信, got %q", it.DisplayName)
	}
	if it.Size < 2048 {
		t.Errorf("大小统计偏小: %d", it.Size)
	}
}

// TestAppSupportCacheScannerMatchesCacheSubdirs 验证 D 类扫描：只匹配
// Application Support 下带 cache 的子目录，红线目录与普通数据目录都不碰。
func TestAppSupportCacheScannerMatchesCacheSubdirs(t *testing.T) {
	home := t.TempDir()
	appSupport := filepath.Join(home, "Library", "Application Support")
	slackCache := filepath.Join(appSupport, "Slack", "Cache")
	slackCode := filepath.Join(appSupport, "Slack", "Code Cache")
	slackPrefs := filepath.Join(appSupport, "Slack", "Preferences")
	chromeCache := filepath.Join(appSupport, "Google", "Chrome", "Default", "Cache")
	for _, dir := range []string{slackCache, slackCode, slackPrefs, chromeCache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{slackCache, slackCode, slackPrefs, chromeCache} {
		if err := os.WriteFile(filepath.Join(dir, "data.bin"), make([]byte, 1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := NewAppSupportCacheScanner(Options{Home: home})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := make(map[string]model.Item, len(items))
	for _, it := range items {
		got[it.Path] = it
	}
	for _, want := range []string{slackCache, slackCode} {
		it, ok := got[want]
		if !ok {
			t.Errorf("缺少缓存子目录: %s", want)
			continue
		}
		if it.Risk != model.RiskMedium {
			t.Errorf("%s 风险应为中（黄区）, got %v", want, it.Risk)
		}
	}
	if _, ok := got[slackPrefs]; ok {
		t.Error("非缓存子目录 Preferences 不应被扫描")
	}
	if _, ok := got[chromeCache]; ok {
		t.Error("红线目录 Application Support/Google 下的缓存不应出现")
	}
}

// TestLargeFileScannerSkipsAppSupportCacheDirs 验证 Application Support 下的
// cache 子目录不被大文件扫描重复列出（它们已作为整体缓存项出现）。
func TestLargeFileScannerSkipsAppSupportCacheDirs(t *testing.T) {
	home := t.TempDir()
	appSupport := filepath.Join(home, "Library", "Application Support")
	cacheDir := filepath.Join(appSupport, "Slack", "Cache")
	dataDir := filepath.Join(appSupport, "Slack", "Attachments")
	for _, dir := range []string{cacheDir, dataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "blob.bin"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "big.bin"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLargeFileScanner(Options{Home: home, LargeFileThreshold: 2048})
	items, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var underCache, underData bool
	for _, it := range items {
		if strings.Contains(it.Path, "Slack/Cache") {
			underCache = true
		}
		if strings.Contains(it.Path, "Slack/Attachments") {
			underData = true
		}
	}
	if underCache {
		t.Error("应用缓存子目录内的文件不应被大文件扫描重复列出")
	}
	if !underData {
		t.Error("非缓存目录里的大文件应正常列出")
	}
}

// TestScanAllClassifiesDevCachesUnderLibraryCaches 验证 ~/Library/Caches 下
// 的开发工具缓存（pip、go-build、JetBrains、Yarn、pnpm）归入开发缓存，
// 且每个路径在报表里只出现一次（拆分扫描器不再重复产出）。
func TestScanAllClassifiesDevCachesUnderLibraryCaches(t *testing.T) {
	names := []string{"pip", "go-build", "JetBrains", "Yarn", "pnpm"}
	home := t.TempDir()
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		dir := filepath.Join(home, "Library", "Caches", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "blob"), make([]byte, 1024), 0o644); err != nil {
			t.Fatal(err)
		}
		wanted[dir] = true
	}

	opts := Options{Home: home, Concurrency: 4}
	report := ScanAll(context.Background(), All(opts), opts)

	counts := make(map[string]int, len(names))
	for _, it := range report.Items {
		if !wanted[it.Path] {
			continue
		}
		counts[it.Path]++
		if it.Category != model.CategoryDevCache {
			t.Errorf("%s 应归开发缓存, got %s", it.Path, it.Category)
		}
	}
	for path := range wanted {
		if counts[path] != 1 {
			t.Errorf("%s 应只出现 1 次, got %d", path, counts[path])
		}
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
