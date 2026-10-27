package ui

import (
	"fmt"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// treeNodeID 是树节点的唯一标识。
//
// 编码规则：
//   - "cat:<类别>"          类别节点
//   - "item:<绝对路径>"      清理项节点
//   - "child:<绝对路径>"     清理项展开后的子目录节点
//
// 用路径作为 ID 可直接从节点反查文件系统位置，无需额外映射表。
type treeNodeID = widget.TreeNodeID

func catNodeID(c model.Category) treeNodeID { return "cat:" + string(c) }
func itemNodeID(path string) treeNodeID     { return "item:" + path }
func childNodeID(path string) treeNodeID    { return "child:" + path }

// pathFromNodeID 从节点 ID 中取出文件系统路径；类别节点返回空串。
func pathFromNodeID(id treeNodeID) string {
	switch {
	case strings.HasPrefix(id, "item:"):
		return strings.TrimPrefix(id, "item:")
	case strings.HasPrefix(id, "child:"):
		return strings.TrimPrefix(id, "child:")
	default:
		return ""
	}
}

// treeRow 是树节点的渲染状态。
type treeRow struct {
	item    model.Item
	checked bool
	check   *widget.Check
	// selectable 为 false 表示该节点不可勾选（类别节点、永久删除项）。
	selectable bool
}

// treeModel 持有树形列表的全部状态。
type treeModel struct {
	view *mainView

	// mu 保护以下 map：子项扫描在后台 goroutine 完成，
	// 而树的 CreateNode/UpdateNode 在 UI 线程调用。
	mu sync.RWMutex

	// rows 按节点 ID 保存渲染状态。
	rows map[treeNodeID]*treeRow
	// children 缓存已加载的子项，避免重复扫描。
	children map[treeNodeID][]treeNodeID
	// loading 标记正在加载的节点，防止重复触发。
	loading map[treeNodeID]bool
	// loaded 标记已加载过的节点（即使结果为空）。
	loaded map[treeNodeID]bool
	// categoryOf 记录每个节点所属类别，用于"全选本类"。
	categoryOf map[treeNodeID]model.Category
}

// newTreeModel 创建树模型。
func newTreeModel(v *mainView) *treeModel {
	return &treeModel{
		view:       v,
		rows:       make(map[treeNodeID]*treeRow),
		children:   make(map[treeNodeID][]treeNodeID),
		loading:    make(map[treeNodeID]bool),
		loaded:     make(map[treeNodeID]bool),
		categoryOf: make(map[treeNodeID]model.Category),
	}
}

// reset 清空全部状态，用于重新扫描。
func (m *treeModel) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = make(map[treeNodeID]*treeRow)
	m.children = make(map[treeNodeID][]treeNodeID)
	m.loading = make(map[treeNodeID]bool)
	m.loaded = make(map[treeNodeID]bool)
	m.categoryOf = make(map[treeNodeID]model.Category)
}

// build 构造 Fyne 树组件。
func (m *treeModel) build() *widget.Tree {
	tree := widget.NewTree(
		m.childUIDs,
		m.isBranch,
		m.createNode,
		m.updateNode,
	)
	tree.HideSeparators = false
	return tree
}

// childUIDs 返回某节点的子节点 ID。类别节点的子项在 reset 时预填。
func (m *treeModel) childUIDs(id treeNodeID) []treeNodeID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if c, ok := m.children[id]; ok {
		return c
	}
	return nil
}

// rootNodeID 是树的虚拟根节点。
//
// Fyne 的 Tree 只有一个 Root，且 walk() 只有在 isBranch(root) 为 true
// 时才会下钻子节点——若根节点不可展开，整棵树只会渲染根自身。
// 因此这里用 "root" 作为一个可展开的汇总节点，其子节点是各分类。
const rootNodeID treeNodeID = "root"

// isBranch 判断节点是否可展开。
//
// 只有"目录类清理项"和"目录类子项"才可展开；根节点与类别节点始终可展开。
func (m *treeModel) isBranch(id treeNodeID) bool {
	// 根节点必须可展开，否则整棵树不会下钻。
	if id == rootNodeID {
		return true
	}
	if strings.HasPrefix(id, "cat:") {
		return true
	}

	m.mu.RLock()
	row, ok := m.rows[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}

	// 只有目录才能下钻；文件没有子项。
	if !row.item.IsDir {
		return false
	}

	// 限制最多展开 2 层：类别 → 清理项 → 子项。
	// child: 前缀的节点不再继续展开，避免无限下钻误选深层文件。
	if strings.HasPrefix(id, "child:") {
		return false
	}
	return true
}

// createNode 创建节点控件模板。Fyne 会复用这些对象。
func (m *treeModel) createNode(branch bool) fyne.CanvasObject {
	chk := widget.NewCheck("", func(bool) {})
	lbl := widget.NewLabel("")
	risk := canvas.NewText("", themeColorForeground())
	risk.TextStyle = fyne.TextStyle{Bold: true}
	risk.Alignment = fyne.TextAlignLeading
	riskBox := container.NewGridWrap(fyne.NewSize(riskBadgeWidth, riskSizeHeight), risk)

	// 类别节点专用的批量操作按钮，普通节点隐藏。
	allBtn := widget.NewButton("全选", func() {})
	allBtn.Importance = widget.LowImportance
	allBtn.Hide()
	noneBtn := widget.NewButton("清空", func() {})
	noneBtn.Importance = widget.LowImportance
	noneBtn.Hide()
	actions := container.NewHBox(allBtn, noneBtn)

	// 布局说明：
	//
	// 树节点会被 Fyne 按层级缩进，右侧贴边内容容易被裁切（实测
	// 无论给多大 MinSize，"● 高" 末尾仍会缺一角）。因此把风险标记
	// 紧跟在名称之后放在左侧，右侧只留批量操作按钮，彻底避开裁切。
	return container.NewHBox(chk, lbl, riskBox, actions)
}

// nodeRightMinWidth 保留以兼容旧的尺寸常量引用。
const nodeRightMinWidth float32 = 200

// updateNode 用节点数据刷新控件。
func (m *treeModel) updateNode(id treeNodeID, branch bool, o fyne.CanvasObject) {
	// 结构：HBox(chk, lbl, riskBox, actions)
	box := o.(*fyne.Container)
	chk := box.Objects[0].(*widget.Check)
	lbl := box.Objects[1].(*widget.Label)
	riskBox := box.Objects[2].(*fyne.Container)
	risk := riskBox.Objects[0].(*canvas.Text)
	actions := box.Objects[3].(*fyne.Container)

	allBtn := actions.Objects[0].(*widget.Button)
	noneBtn := actions.Objects[1].(*widget.Button)

	m.mu.RLock()
	row, hasRow := m.rows[id]
	loading := m.loading[id]
	m.mu.RUnlock()

	// 根节点：显示全局汇总，不可勾选，提供"全选/清空全部"。
	if id == rootNodeID {
		m.renderRootNode(chk, lbl, risk)
		allBtn.OnTapped = func() { m.selectAll(true, m.view.tree) }
		noneBtn.OnTapped = func() { m.selectAll(false, m.view.tree) }
		allBtn.Show()
		noneBtn.Show()
		return
	}

	// 类别节点：显示类别名与合计，不可勾选，但提供批量操作。
	if strings.HasPrefix(id, "cat:") {
		m.renderCategoryNode(id, chk, lbl, risk)

		cat := model.Category(strings.TrimPrefix(id, "cat:"))
		allBtn.OnTapped = func() { m.selectCategory(cat, true, m.view.tree) }
		noneBtn.OnTapped = func() { m.selectCategory(cat, false, m.view.tree) }
		allBtn.Show()
		noneBtn.Show()
		return
	}

	allBtn.Hide()
	noneBtn.Hide()

	if !hasRow {
		lbl.SetText("…")
		chk.Hide()
		risk.Text = ""
		risk.Refresh()
		return
	}

	row.check = chk
	chk.Show()

	// 名称 + 体积
	name := row.item.DisplayName
	size := model.HumanBytes(row.item.Size)
	if loading {
		lbl.SetText(fmt.Sprintf("%s   %s   加载中…", name, size))
	} else {
		lbl.SetText(fmt.Sprintf("%s   %s", name, size))
	}
	lbl.TextStyle = fyne.TextStyle{}
	lbl.Refresh()

	// 风险标记
	risk.Text = riskBadge(row.item.Risk)
	risk.Color = riskColor(row.item.Risk)
	risk.Refresh()

	// 永久删除项与只读项都不可勾选。
	//
	// ReadOnly 用于"指定目录扫描"：用户可以浏览任意目录，
	// 但删除仍受 guard 白名单约束，界面提前置灰避免误导。
	if row.item.Permanent || row.item.ReadOnly {
		chk.OnChanged = nil
		chk.Disable()
		chk.SetChecked(false)
		row.checked = false
		return
	}

	chk.Enable()

	// 顺序很重要：必须先解除回调、设置勾选状态、再重新绑定回调。
	//
	// SetChecked 会触发 OnChanged；若先绑定回调再 SetChecked，
	// 控件从复用的旧状态切换时会回调 setChecked，把模型里的
	// 真实状态覆盖掉，导致"已选 2 项但界面只显示 1 个勾"。
	chk.OnChanged = nil
	chk.SetChecked(row.checked)
	chk.OnChanged = func(b bool) {
		m.setChecked(id, b)
	}

	// SetChecked 在值未变化时会提前 return 而不刷新渲染。
	// 树节点控件是复用的，视觉状态可能滞后于模型，这里强制刷新。
	chk.Refresh()
}

// renderRootNode 渲染虚拟根节点：全局汇总。
func (m *treeModel) renderRootNode(chk *widget.Check, lbl *widget.Label, risk *canvas.Text) {
	m.mu.RLock()
	children := m.children[rootNodeID]
	var total int64
	var count int
	for _, cid := range children {
		if sub, ok := m.children[cid]; ok {
			count += len(sub)
			for _, itemID := range sub {
				if r, ok := m.rows[itemID]; ok {
					total += r.item.Size
				}
			}
		}
	}
	m.mu.RUnlock()

	chk.Hide()
	lbl.SetText(fmt.Sprintf("全部可清理内容   ·   %s   (%d 项)", model.HumanBytes(total), count))
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.Refresh()

	risk.Text = ""
	risk.Refresh()
}

// selectAll 勾选或取消全部可选项。
func (m *treeModel) selectAll(checked bool, tree *widget.Tree) {
	m.mu.Lock()
	for _, row := range m.rows {
		if row.selectable {
			row.checked = checked
		}
	}
	m.mu.Unlock()

	tree.Refresh()
	m.view.updateSummary()
}

// renderCategoryNode 渲染类别节点。
func (m *treeModel) renderCategoryNode(id treeNodeID, chk *widget.Check, lbl *widget.Label, risk *canvas.Text) {
	cat := model.Category(strings.TrimPrefix(id, "cat:"))

	m.mu.RLock()
	children := m.children[id]
	m.mu.RUnlock()

	var total int64
	for _, child := range children {
		if r, ok := m.rows[child]; ok {
			total += r.item.Size
		}
	}

	chk.Hide()
	lbl.SetText(fmt.Sprintf("%s   ·   %s   (%d 项)", cat, model.HumanBytes(total), len(children)))
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.Refresh()

	risk.Text = ""
	risk.Refresh()
}

// setChecked 更新某节点的勾选状态并刷新汇总。
func (m *treeModel) setChecked(id treeNodeID, checked bool) {
	m.mu.Lock()
	if row, ok := m.rows[id]; ok {
		row.checked = checked
	}
	m.mu.Unlock()
	m.view.updateSummary()
}

// loadChildren 异步加载某节点的子项。
func (m *treeModel) loadChildren(id treeNodeID, tree *widget.Tree) {
	m.mu.Lock()
	if m.loaded[id] || m.loading[id] {
		m.mu.Unlock()
		return
	}
	m.loading[id] = true
	m.mu.Unlock()

	tree.Refresh()

	path := pathFromNodeID(id)
	if path == "" {
		m.finishLoad(id, tree, nil)
		return
	}

	go func() {
		items, err := m.view.ctrl.Children(path)

		fyne.Do(func() {
			if err != nil {
				m.finishLoad(id, tree, nil)
				return
			}

			// 子项继承父项的类别与风险，便于统计与着色。
			m.mu.RLock()
			parentRow := m.rows[id]
			m.mu.RUnlock()

			var ids []treeNodeID
			m.mu.Lock()
			for _, it := range items {
				cid := childNodeID(it.Path)
				it.Category = parentRow.item.Category
				it.Risk = parentRow.item.Risk
				m.rows[cid] = &treeRow{item: it, selectable: !it.Permanent && !it.ReadOnly}
				m.categoryOf[cid] = parentRow.item.Category
				ids = append(ids, cid)
			}
			m.mu.Unlock()

			m.finishLoad(id, tree, ids)
		})
	}()
}

// finishLoad 记录加载结果并刷新树。
func (m *treeModel) finishLoad(id treeNodeID, tree *widget.Tree, ids []treeNodeID) {
	m.mu.Lock()
	delete(m.loading, id)
	m.loaded[id] = true
	m.children[id] = ids
	m.mu.Unlock()

	tree.Refresh()
	m.view.updateSummary()
}

// setCategoryItems 为类别节点预填子项（在 reset 后调用）。
func (m *treeModel) setCategoryItems(cat model.Category, items []model.Item) {
	cid := catNodeID(cat)

	m.mu.Lock()
	defer m.mu.Unlock()

	var ids []treeNodeID
	for _, it := range items {
		id := itemNodeID(it.Path)
		m.rows[id] = &treeRow{item: it, selectable: !it.Permanent && !it.ReadOnly}
		m.categoryOf[id] = cat
		ids = append(ids, id)
	}
	m.children[cid] = ids
	m.loaded[cid] = true
}

// selectedItems 返回全部已勾选的项，按体积降序。
func (m *treeModel) selectedItems() []model.Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []model.Item
	for _, row := range m.rows {
		if row.checked {
			out = append(out, row.item)
		}
	}
	// 排序交给调用方，这里只保证集合正确。
	return out
}

// selectCategory 勾选或取消某类别下的全部可选项。
func (m *treeModel) selectCategory(cat model.Category, checked bool, tree *widget.Tree) {
	m.mu.Lock()
	for id, row := range m.rows {
		if m.categoryOf[id] != cat || !row.selectable {
			continue
		}
		row.checked = checked
	}
	m.mu.Unlock()

	tree.Refresh()
	m.view.updateSummary()
}
