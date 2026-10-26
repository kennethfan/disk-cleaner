package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// newTestView 构造一个不接触真实文件系统的视图。
func newTestView(t *testing.T) (*mainView, func()) {
	t.Helper()
	test.NewApp()
	w := test.NewWindow(nil)
	v := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(v.build())
	return v, func() { w.Close() }
}

func TestRenderReportBuildsRows(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	if len(v.rows) != len(v.report.Items) {
		t.Errorf("行数 %d, 期望 %d", len(v.rows), len(v.report.Items))
	}
}

// TestPermanentItemDisabled 确保永久删除项无法被勾选。
func TestPermanentItemDisabled(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	var found bool
	for _, row := range v.rows {
		if !row.item.Permanent {
			continue
		}
		found = true
		if !row.check.Disabled() {
			t.Error("Permanent 项的复选框应被禁用")
		}
		// 即使强制勾选，也不应进入 selected()。
		row.check.SetChecked(true)
	}
	if !found {
		t.Fatal("示例数据应包含 Permanent 项")
	}
}

// TestUpdateSummaryCountsSelection 验证摘要统计随勾选变化。
func TestUpdateSummaryCountsSelection(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	// 初始无选中，按钮应禁用。
	if !v.cleanBtn.Disabled() {
		t.Error("未选中时清理按钮应禁用")
	}

	v.rows[0].check.SetChecked(true)
	if v.cleanBtn.Disabled() {
		t.Error("有选中时清理按钮应启用")
	}

	got := v.selected()
	if len(got) != 1 {
		t.Fatalf("selected 返回 %d 项, 期望 1", len(got))
	}
	if got[0].Path != v.rows[0].item.Path || got[0].DisplayName != v.rows[0].item.DisplayName {
		t.Error("selected 返回的项与勾选项不一致")
	}
}

// TestSelectedSortedBySize 验证选中项按体积降序。
func TestSelectedSortedBySize(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)
	for _, row := range v.rows {
		row.check.SetChecked(true)
	}

	got := v.selected()
	for i := 1; i < len(got); i++ {
		if got[i-1].Size < got[i].Size {
			t.Errorf("未按体积降序: %d 后跟 %d", got[i-1].Size, got[i].Size)
		}
	}
}

// TestRiskBadgeCoversAllLevels 确保每个风险等级都有可读标记。
func TestRiskBadgeCoversAllLevels(t *testing.T) {
	for _, r := range []model.Risk{model.RiskLow, model.RiskMedium, model.RiskHigh} {
		if riskBadge(r) == "" {
			t.Errorf("风险等级 %v 没有标记", r)
		}
		if riskColor(r) == nil {
			t.Errorf("风险等级 %v 没有颜色", r)
		}
	}
}

// TestEmptyReportShowsPlaceholder 验证无结果时不显示空白。
func TestEmptyReportShowsPlaceholder(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = &model.Report{}
	v.renderReport(v.report)

	if len(v.rows) != 0 {
		t.Errorf("空报告不应产生行, got %d", len(v.rows))
	}
	if !v.cleanBtn.Disabled() {
		t.Error("空报告时清理按钮应禁用")
	}
}

// TestCleanResultCounts 验证结果统计口径。
func TestCleanResultCounts(t *testing.T) {
	res := CleanResult{
		Outcomes: []clean.Outcome{
			{Err: nil, Skipped: false},
			{Err: nil, Skipped: true},
			{Err: errForTest, Skipped: false},
		},
	}
	if got := res.Succeeded(); got != 1 {
		t.Errorf("Succeeded = %d, 期望 1", got)
	}
	if got := res.Failed(); got != 1 {
		t.Errorf("Failed = %d, 期望 1", got)
	}
}

var errForTest = testError("boom")

type testError string

func (e testError) Error() string { return string(e) }
