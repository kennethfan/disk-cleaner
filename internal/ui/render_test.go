package ui

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// TestRenderMainView 渲染主界面并输出截图，用于人工核对布局。
//
// 该测试不依赖真实扫描结果：用构造的 Report 填充列表，
// 避免在沙箱环境中访问用户目录。未设置 UI_SNAPSHOT 时自动跳过。
func TestRenderMainView(t *testing.T) {
	out := os.Getenv("UI_SNAPSHOT")
	if out == "" {
		t.Skip("未设置 UI_SNAPSHOT，跳过截图")
	}

	test.NewApp()
	w := test.NewWindow(nil)
	defer w.Close()
	w.Resize(fyne.NewSize(820, 560))

	view := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(view.build())

	view.report = sampleReport()
	view.renderReport(view.report)

	// 勾选两项，让摘要与按钮显示已选状态。
	if len(view.rows) > 1 {
		view.rows[0].check.SetChecked(true)
		view.rows[1].check.SetChecked(true)
	}

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

// sampleReport 构造用于截图的示例数据。
func sampleReport() *model.Report {
	r := &model.Report{}
	r.Items = []model.Item{
		{DisplayName: "Docker 虚拟机数据", Category: model.CategoryDevCache, Risk: model.RiskHigh, Size: 64 << 30, IsDir: true},
		{DisplayName: "Maven 本地仓库", Category: model.CategoryDevCache, Risk: model.RiskMedium, Size: 9<<30 + 100<<20, IsDir: true},
		{DisplayName: "Gradle 缓存", Category: model.CategoryDevCache, Risk: model.RiskLow, Size: 2<<30 + 400<<20, IsDir: true},
		{DisplayName: "npm 缓存 (~/.npm/_cacache)", Category: model.CategoryDevCache, Risk: model.RiskLow, Size: 683 << 20, IsDir: true},
		{DisplayName: "用户缓存目录 (~/Library/Caches)", Category: model.CategoryUserCache, Risk: model.RiskLow, Size: 6<<30 + 300<<20, IsDir: true},
		{DisplayName: "用户日志 (~/Library/Logs)", Category: model.CategoryUserCache, Risk: model.RiskLow, Size: 164 << 20, IsDir: true},
		{DisplayName: "废纸篓 (~/.Trash)", Category: model.CategoryJunk, Risk: model.RiskHigh, Size: 120 << 20, IsDir: true, Permanent: true},
		{DisplayName: "Downloads/Camera.zip", Category: model.CategoryLargeFile, Risk: model.RiskMedium, Size: 568 << 20},
	}
	r.Finalize()
	return r
}
