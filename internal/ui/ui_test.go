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

// rowsOf 返回树模型中全部行。
func rowsOf(v *mainView) []*treeRow {
	v.model.mu.RLock()
	defer v.model.mu.RUnlock()
	var out []*treeRow
	for _, r := range v.model.rows {
		out = append(out, r)
	}
	return out
}

func TestRenderReportBuildsTree(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	// 树根下应有 4 个类别节点（开发缓存/用户缓存/垃圾文件/大文件）。
	v.model.mu.RLock()
	roots := v.model.children[rootNodeID]
	v.model.mu.RUnlock()

	if len(roots) != 4 {
		t.Fatalf("类别节点数 = %d, 期望 4", len(roots))
	}

	// 每个清理项都应有对应行。
	if got := len(rowsOf(v)); got != len(v.report.Items) {
		t.Errorf("行数 = %d, 期望 %d", got, len(v.report.Items))
	}
}

// TestPermanentItemNotSelectable 确保永久删除项无法被勾选。
func TestPermanentItemNotSelectable(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	var found bool
	for _, row := range rowsOf(v) {
		if !row.item.Permanent {
			continue
		}
		found = true
		if row.selectable {
			t.Error("Permanent 项不应可勾选")
		}
	}
	if !found {
		t.Fatal("示例数据应包含 Permanent 项")
	}
}

// TestCategoryNodesHaveNoCheckbox 验证类别节点不可勾选。
func TestCategoryNodesHaveNoCheckbox(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	v.model.mu.RLock()
	roots := v.model.children[rootNodeID]
	v.model.mu.RUnlock()

	for _, id := range roots {
		if _, ok := v.model.rows[id]; ok {
			t.Errorf("类别节点 %s 不应出现在 rows 中（它不可勾选）", id)
		}
	}
}

// TestSelectCategorySelectsAll 验证"全选本类"覆盖该类别全部可选项。
func TestSelectCategorySelectsAll(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	cat := model.CategoryDevCache
	v.model.selectCategory(cat, true, v.tree)

	selected := v.model.selectedItems()
	if len(selected) == 0 {
		t.Fatal("全选后应有选中项")
	}
	for _, it := range selected {
		if it.Category != cat {
			t.Errorf("选中项 %s 的类别为 %s, 期望 %s", it.DisplayName, it.Category, cat)
		}
	}

	// 清空后应无选中。
	v.model.selectCategory(cat, false, v.tree)
	if got := v.model.selectedItems(); len(got) != 0 {
		t.Errorf("清空后仍剩 %d 项", len(got))
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

	// 勾选一项。
	var target treeNodeID
	v.model.mu.Lock()
	for id, row := range v.model.rows {
		if row.selectable {
			row.checked = true
			target = id
			break
		}
	}
	v.model.mu.Unlock()

	if target == "" {
		t.Fatal("找不到可勾选的项")
	}

	v.updateSummary()
	if v.cleanBtn.Disabled() {
		t.Error("有选中时清理按钮应启用")
	}

	got := v.model.selectedItems()
	if len(got) != 1 {
		t.Fatalf("selectedItems 返回 %d 项, 期望 1", len(got))
	}
}

// TestEmptyReportShowsNothing 验证无结果时按钮禁用。
func TestEmptyReportShowsNothing(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = &model.Report{}
	v.renderReport(v.report)

	if got := len(rowsOf(v)); got != 0 {
		t.Errorf("空报告不应产生行, got %d", got)
	}
	if !v.cleanBtn.Disabled() {
		t.Error("空报告时清理按钮应禁用")
	}
}

// TestNodeIDRoundTrip 验证节点 ID 与路径的双向转换。
func TestNodeIDRoundTrip(t *testing.T) {
	path := "/Users/x/Library/Caches"

	if got := pathFromNodeID(itemNodeID(path)); got != path {
		t.Errorf("item 节点往返失败: %q", got)
	}
	if got := pathFromNodeID(childNodeID(path)); got != path {
		t.Errorf("child 节点往返失败: %q", got)
	}
	if got := pathFromNodeID(catNodeID(model.CategoryDevCache)); got != "" {
		t.Errorf("类别节点应返回空路径, got %q", got)
	}
}

// TestIsBranchLimitsDepth 验证只允许展开 2 层。
func TestIsBranchLimitsDepth(t *testing.T) {
	v, cleanup := newTestView(t)
	defer cleanup()

	v.report = sampleReport()
	v.renderReport(v.report)

	// 类别节点可展开。
	if !v.model.isBranch(catNodeID(model.CategoryDevCache)) {
		t.Error("类别节点应可展开")
	}

	// 目录类清理项可展开。
	var dirID treeNodeID
	v.model.mu.RLock()
	for id, row := range v.model.rows {
		if row.item.IsDir {
			dirID = id
			break
		}
	}
	v.model.mu.RUnlock()
	if dirID == "" {
		t.Fatal("示例数据应有目录项")
	}
	if !v.model.isBranch(dirID) {
		t.Errorf("目录项 %s 应可展开", dirID)
	}

	// child: 前缀的节点不可再展开（限制 2 层）。
	fakeChild := childNodeID("/Users/x/Library/Caches/Google")
	v.model.mu.Lock()
	v.model.rows[fakeChild] = &treeRow{item: model.Item{IsDir: true, Path: "/Users/x/Library/Caches/Google"}}
	v.model.mu.Unlock()

	if v.model.isBranch(fakeChild) {
		t.Error("第三层节点不应可展开")
	}

	// 文件不可展开。
	fileID := itemNodeID("/Users/x/Downloads/Camera.zip")
	v.model.mu.Lock()
	v.model.rows[fileID] = &treeRow{item: model.Item{IsDir: false}}
	v.model.mu.Unlock()
	if v.model.isBranch(fileID) {
		t.Error("文件不应可展开")
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
