package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/config"
	controller "github.com/kennethfan/disk-cleaner/internal/controller"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// newTestShell 构造一个不接触真实扫描流程的完整界面。	// Controller 用测试构造器创建（带空配置避免读写用户配置），
// 避免访问真实扫描与废纸篓。
func newTestShell(t *testing.T) (*appShell, func()) {
	t.Helper()
	test.NewApp()
	test.ApplyTheme(t, newAppTheme())
	w := test.NewWindow(nil)
	s := newAppShell(w, controller.NewForTest(&config.Config{}))
	return s, func() { w.Close() }
}

// TestAdviceViewBuildsCards 验证按报告构建分类卡片：
// 只为非空分类建卡，类别数与报告一致。
func TestAdviceViewBuildsCards(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	r := sampleReport()
	s.adviceV.setReport(r)
	s.refreshNav()

	wantCats := 0
	groups := r.ByCategory()
	for _, cat := range scanSegmentOrder() {
		if len(groups[cat].Items) > 0 {
			wantCats++
		}
	}
	if got := len(s.adviceV.cards); got != wantCats {
		t.Fatalf("卡片数 = %d, 期望 %d", got, wantCats)
	}
}

// TestPermanentItemNotSelectable 确保永久删除项无法被勾选。
func TestPermanentItemNotSelectable(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	s.adviceV.setReport(sampleReport())

	// 找到废纸篓所在的分类卡（报告里该项 Permanent: true）。
	var junkCard *categoryCard
	for _, c := range s.adviceV.cards {
		if c.cat == model.CategoryJunk {
			junkCard = c
		}
	}
	if junkCard == nil {
		t.Fatal("示例数据应包含 Junk 分类")
	}

	sel, total := junkCard.stats()
	if total != 0 || sel != 0 {
		t.Errorf("Permanent 项不应计入可勾选范围: sel=%d total=%d", sel, total)
	}
}

// TestAdviceSelectionFlow 验证勾选→汇总栏→清理按钮的联动。
func TestAdviceSelectionFlow(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	s.adviceV.setReport(sampleReport())
	if !s.adviceV.cleanBtn.Disabled() {
		t.Error("未选中时清理按钮应禁用")
	}

	// 勾选一项。
	var target *categoryCard
	var targetRow *fileRow
	for _, c := range s.adviceV.cards {
		for _, r := range c.rows {
			if r.settable() {
				target, targetRow = c, r
				break
			}
		}
	}
	if targetRow == nil {
		t.Fatal("找不到可勾选的项")
	}

	s.adviceV.setItemChecked(targetRow.item.Path, true)
	if s.adviceV.cleanBtn.Disabled() {
		t.Error("有选中时清理按钮应启用")
	}
	sel, total := target.stats()
	if sel != 1 || total < 1 {
		t.Errorf("分类勾选数 sel=%d total=%d，期望 sel=1", sel, total)
	}
	if got := len(s.adviceV.selectedItems()); got != 1 {
		t.Errorf("selectedItems 返回 %d 项, 期望 1", got)
	}

	// 清空后回禁用。
	s.adviceV.clearSelection()
	if !s.adviceV.cleanBtn.Disabled() {
		t.Error("清空后清理按钮应禁用")
	}
}

// TestSelectAllRecommendedOnlyPickLowRisk 验证「全选推荐项」只勾低风险。
func TestSelectAllRecommendedOnlyPickLowRisk(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	s.adviceV.setReport(sampleReport())
	s.adviceV.selectAllRecommended(true)

	for _, it := range s.adviceV.selectedItems() {
		if it.Risk != model.RiskLow {
			t.Errorf("选中项 %s 风险等级 %s，全选推荐只应勾低风险", it.DisplayName, it.Risk)
		}
	}
}

// TestAdviceSelectionSurvivesRefresh 验证重渲染时已勾选项在报告内保留。
func TestAdviceSelectionSurvivesRefresh(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	r := sampleReport()
	s.adviceV.setReport(r)

	// 勾选第一项再重渲染，勾选应保留。
	var path string
walk:
	for _, c := range s.adviceV.cards {
		for _, row := range c.rows {
			if row.settable() {
				path = row.item.Path
				break walk
			}
		}
	}
	s.adviceV.setItemChecked(path, true)

	s.adviceV.setReport(r)
	if !s.adviceV.selected[path] {
		t.Error("重渲染后勾选丢失")
	}
}

// TestLargeViewSelection 验证大文件页勾选与汇总。
func TestLargeViewSelection(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	s.largeV.setReport(sampleReport())

	items := s.largeV.selectedItems()
	if len(items) != 0 {
		t.Fatalf("初始应无选中，got %d", len(items))
	}
	if !s.largeV.cleanLargeBtn.Disabled() {
		t.Error("未选中时按钮应禁用")
	}

	// 勾选全部大文件。
	for _, row := range s.largeV.rows {
		s.largeV.selected[row.item.Path] = true
	}
	s.largeV.updateSelection()

	if len(s.largeV.selectedItems()) != len(s.largeV.rows) {
		t.Errorf("选中数 %d, 期望 %d", len(s.largeV.selectedItems()), len(s.largeV.rows))
	}
	if s.largeV.cleanLargeBtn.Disabled() {
		t.Error("有选中时按钮应启用")
	}

	// nil 报告清空状态。
	s.largeV.setReport(nil)
	if len(s.largeV.selectedItems()) != 0 || !s.largeV.cleanLargeBtn.Disabled() {
		t.Error("nil 报告应清空勾选并禁用按钮")
	}
}

// TestExcludeViewRules 验证排除页的增删联动角标。
func TestExcludeViewRules(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	if err := s.ctrl.AddExcludeRule("node_modules"); err != nil {
		t.Fatalf("添加规则失败: %v", err)
	}
	s.excludeV.refresh()
	s.refreshNav()

	if got := len(s.excludeV.rowsBox.Objects); got == 0 {
		t.Error("添加规则后列表不应为空")
	}
	if got := s.navBtns[viewExclude].badgeTxt.Text; got != "1" {
		t.Errorf("角标 = %q, 期望 1", got)
	}
}

// TestHeadShowsViewTitle 验证标题随视图切换。
func TestHeadShowsViewTitle(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	cases := map[viewID]string{
		viewOverview: "磁盘概览",
		viewAdvice:   "清理建议",
		viewLarge:    "大文件",
		viewExclude:  "排除列表",
	}
	for id, want := range cases {
		s.switchView(id)
		if s.headTitleText() != want {
			t.Errorf("视图 %d 标题 = %q, 期望 %q", id, s.headTitleText(), want)
		}
	}
}

// TestScopeModeGuardsScan 验证指定目录模式下未添加目录时禁用扫描。
func TestScopeModeGuardsScan(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	v := s.overview
	v.setScopeMode(true)
	if !v.scanBtn.Disabled() {
		t.Error("未添加目录时应禁用扫描")
	}
	if v.scanBtn.Text != "⚠︎ 请先添加扫描目录" {
		t.Errorf("按钮文案 = %q", v.scanBtn.Text)
	}

	s.ctrl.SetTargetPaths([]string{"/tmp/a"})
	v.updateScope()
	if v.scanBtn.Disabled() {
		t.Error("添加目录后应可扫描")
	}

	v.setScopeMode(false)
	if v.scanBtn.Disabled() || len(s.ctrl.TargetPaths()) != 0 {
		t.Error("切回全盘应清空目录并启用扫描")
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
