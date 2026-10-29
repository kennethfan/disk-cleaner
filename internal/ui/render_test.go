package ui

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// TestRenderShellSnapshot 渲染整个界面并输出截图，用于人工核对布局。
//
// 该测试不依赖真实扫描结果：用构造的 Report 填充各页，
// 避免在沙箱环境中访问用户目录。未设置 UI_SNAPSHOT 时自动跳过。
func TestRenderShellSnapshot(t *testing.T) {
	out := os.Getenv("UI_SNAPSHOT")
	if out == "" {
		t.Skip("未设置 UI_SNAPSHOT，跳过截图")
	}

	test.NewApp()
	test.ApplyTheme(t, newAppTheme())
	w := test.NewWindow(nil)
	defer w.Close()
	w.Resize(fyne.NewSize(1120, 740))

	s := newAppShell(w, &Controller{})

	// 填充报告并切到清理建议页，勾选若干项展示汇总栏。
	s.report = sampleReport()
	s.refreshAllViews()
	s.adviceV.setReport(s.report)
	s.adviceV.selectAllRecommended(true)
	s.switchView(viewAdvice)

	img := w.Canvas().Capture()
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("创建截图文件: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码 PNG: %v", err)
	}
	t.Logf("截图已写入 %s", out)
}

// TestRenderOverviewSnapshot 渲染概览页截图（含环形图与分类瓦片）。
func TestRenderOverviewSnapshot(t *testing.T) {
	out := os.Getenv("UI_SNAPSHOT_OVERVIEW")
	if out == "" {
		t.Skip("未设置 UI_SNAPSHOT_OVERVIEW，跳过截图")
	}

	test.NewApp()
	test.ApplyTheme(t, newAppTheme())
	w := test.NewWindow(nil)
	defer w.Close()
	w.Resize(fyne.NewSize(1120, 740))

	s := newAppShell(w, &Controller{})
	s.report = sampleReport()
	s.refreshAllViews()
	s.switchView(viewOverview)

	img := w.Canvas().Capture()
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("创建截图文件: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码 PNG: %v", err)
	}
	t.Logf("截图已写入 %s", out)
}

// sampleReport 构造用于截图与测试的示例数据。
func sampleReport() *model.Report {
	r := &model.Report{}
	r.Items = []model.Item{
		{Path: "/h/Library/Developer/Docker", DisplayName: "Docker 虚拟机数据", Category: model.CategoryDevCache, Risk: model.RiskHigh, Size: 64 << 30, IsDir: true},
		{Path: "/h/.m2/repository", DisplayName: "Maven 本地仓库", Category: model.CategoryDevCache, Risk: model.RiskMedium, Size: 9<<30 + 100<<20, IsDir: true},
		{Path: "/h/.gradle/caches", DisplayName: "Gradle 缓存", Category: model.CategoryDevCache, Risk: model.RiskLow, Size: 2<<30 + 400<<20, IsDir: true},
		{Path: "/h/.npm/_cacache", DisplayName: "npm 缓存", Category: model.CategoryDevCache, Risk: model.RiskLow, Size: 683 << 20, IsDir: true},
		{Path: "/h/Library/Caches", DisplayName: "用户缓存目录", Category: model.CategoryUserCache, Risk: model.RiskLow, Size: 6<<30 + 300<<20, IsDir: true},
		{Path: "/h/Library/Logs", DisplayName: "用户日志", Category: model.CategoryUserCache, Risk: model.RiskLow, Size: 164 << 20, IsDir: true},
		{Path: "/h/.Trash", DisplayName: "废纸篓", Category: model.CategoryJunk, Risk: model.RiskHigh, Size: 120 << 20, IsDir: true, Permanent: true},
		{Path: "/h/Downloads/Camera.zip", DisplayName: "Downloads/Camera.zip", Category: model.CategoryLargeFile, Risk: model.RiskMedium, Size: 568 << 20},
	}
	r.Finalize()
	return r
}
