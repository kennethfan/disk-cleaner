package ui

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// adviceView 是清理建议页。
//
// 勾选状态以 item 路径为键存在本页，跨报告刷新（排除某文件后重渲染）
// 时自动剪掉已不存在的条目。
type adviceView struct {
	shell *appShell
	root  fyne.CanvasObject

	cardsBox *fyne.Container
	cards    []*categoryCard
	selected map[string]bool // path → 勾选

	// 底部汇总栏（有选中项时滑出显示）。
	barBox   fyne.CanvasObject
	pickAll  *widget.Check
	mainSize *canvas.Text
	mainSub  *canvas.Text
	clearBtn *widget.Button
	cleanBtn *widget.Button

	emptyLbl *widget.Label
}

// newAdviceView 构建页面骨架。
func newAdviceView(s *appShell) *adviceView {
	v := &adviceView{
		shell:    s,
		selected: make(map[string]bool),
	}
	v.cardsBox = container.NewVBox()
	v.emptyLbl = widget.NewLabel("没有发现可清理的内容 · 磁盘很干净")
	v.emptyLbl.Alignment = fyne.TextAlignCenter
	v.emptyLbl.Importance = widget.LowImportance
	v.emptyLbl.Hide()

	list := container.NewStack(canvas.NewRectangle(colWindowBg),
		container.NewVScroll(container.NewVBox(
			container.NewPadded(v.emptyLbl),
			container.NewPadded(v.cardsBox),
		)))

	v.barBox = v.buildActionBar()
	v.root = container.NewBorder(nil, v.barBox, nil, nil, list)
	v.barBox.Hide()
	return v
}

// buildActionBar 构造底部悬浮汇总栏。
func (v *adviceView) buildActionBar() fyne.CanvasObject {
	v.mainSize = makeText("0 B", 15, colFg, true, fyne.TextAlignLeading)
	v.mainSub = makeText("已选 0 个项目", 12, colFg2, false, fyne.TextAlignLeading)

	v.pickAll = widget.NewCheck("全选推荐项", func(checked bool) { v.selectAllRecommended(checked) })
	v.clearBtn = widget.NewButton("取消选择", func() { v.clearSelection() })
	v.clearBtn.Importance = widget.LowImportance
	v.cleanBtn = widget.NewButton("🗑 安全清理", func() {
		v.shell.confirmClean(v.selectedItems(), "确认清理这些文件？")
	})
	v.cleanBtn.Importance = widget.DangerImportance
	v.cleanBtn.Disable()

	barBg := canvas.NewRectangle(colCardBg)
	content := container.NewVBox(widget.NewSeparator(),
		container.NewPadded(container.NewHBox(
			v.pickAll,
			container.NewVBox(v.mainSize, v.mainSub),
			layout.NewSpacer(),
			v.clearBtn,
			v.cleanBtn,
		)))
	return container.NewStack(barBg, content)
}

// setReport 用扫描结果重建分类卡片。r 可为 nil（未扫描）。
func (v *adviceView) setReport(r *model.Report) {
	// 剪掉已不存在的勾选。
	if r != nil {
		live := map[string]bool{}
		for _, it := range r.Items {
			live[it.Path] = v.selected[it.Path]
		}
		v.selected = live
	} else {
		v.selected = map[string]bool{}
	}

	v.cards = nil
	v.cardsBox.Objects = nil

	if r == nil || len(r.Items) == 0 {
		v.showEmpty(r)
		v.cardsBox.Refresh()
		v.updateSelection()
		return
	}
	v.hideEmpty()

	groups := r.ByCategory()
	for _, cat := range scanSegmentOrder() {
		g := groups[cat]
		if len(g.Items) == 0 {
			continue
		}
		card := v.newCategoryCard(cat, g)
		v.cards = append(v.cards, card)
		v.cardsBox.Add(card.cardBox)
	}
	v.cardsBox.Refresh()
	v.syncAll()
}

// showEmpty / hideEmpty 管理空状态占位。
func (v *adviceView) showEmpty(r *model.Report) {
	if r != nil && r.ExcludedCount > 0 {
		v.emptyLbl.SetText(fmt.Sprintf("共 %d 项被排除规则过滤了 · 可到「排除列表」调整",
			r.ExcludedCount))
	} else if r != nil && len(r.TargetPaths) > 0 {
		v.emptyLbl.SetText("指定目录中没有可清理的内容")
	} else {
		v.emptyLbl.SetText("没有发现可清理的内容 · 磁盘很干净")
	}
	v.emptyLbl.Show()
}

func (v *adviceView) hideEmpty() { v.emptyLbl.Hide() }

// newCategoryCard 构造单个分类卡片（默认收起）。
func (v *adviceView) newCategoryCard(cat model.Category, g model.CategoryGroup) *categoryCard {
	card := &categoryCard{
		view:  v,
		cat:   cat,
		group: g,
		open:  false,
	}

	// 头部勾选框为圆形（原型 .cat-check），替代 Fyne 方形 Check。
	card.check = newCircleCheck(func(checked bool) {
		card.setChecked(checked)
	})

	swatch := makeColoredRect(categoryColor(cat), 10, 10)
	nameLbl := makeText(string(cat), 14, colFg, true, fyne.TextAlignLeading)

	tag, tagCol := categoryTag(g.Items)
	tagBg := colOKTint
	if tagCol == colWarn {
		tagBg = colWarnTint
	}
	badge := makeBadge(tag, tagCol, tagBg)

	descLbl := makeText(categoryDesc(cat), 11.5, colFg2, false, fyne.TextAlignLeading)

	sizeLbl := makeText(model.HumanBytes(g.Bytes), 16, colFg, true, fyne.TextAlignTrailing)              // 原型 .cat-size .v 16px
	countLbl := makeText(fmt.Sprintf("%d 项", len(g.Items)), 11.5, colFg2, false, fyne.TextAlignTrailing) // .c 11.5px

	card.arrow = widget.NewButton("›", func() { card.toggle() })
	card.arrow.Importance = widget.LowImportance

	head := container.NewPadded(container.NewBorder(nil, nil,
		container.NewHBox(card.check, swatch),
		container.NewHBox(container.NewVBox(sizeLbl, countLbl), card.arrow),
		container.NewVBox(container.NewHBox(nameLbl, badge), descLbl),
	))
	card.head = head

	// 明细行列表：固定构建，展开/收起只切换可见性。
	rowsBox := container.NewVBox()
	card.rows = make([]*fileRow, 0, len(g.Items))
	for _, it := range g.Items {
		row := card.newFileRow(it)
		card.rows = append(card.rows, row)
		rowsBox.Add(row.build())
	}
	card.body = container.NewStack(
		newRoundRect(colSubBg, radiusCard), // 与卡片同样圆角：模拟 overflow:hidden 裁切效果
		container.NewVBox(widget.NewSeparator(), rowsBox))
	card.body.Hide()

	card.cardBox = makeCard(container.NewVBox(head, card.body))
	return card
}

// === selection ===

// categoryCard 是一个分类的卡片。
type categoryCard struct {
	view  *adviceView
	cat   model.Category
	group model.CategoryGroup
	rows  []*fileRow
	open  bool

	check   *circleCheck
	arrow   *widget.Button
	head    fyne.CanvasObject
	body    *fyne.Container
	cardBox *fyne.Container
}

// toggle 展开/收起明细列表（箭头文案同步旋转反馈）。
func (c *categoryCard) toggle() {
	c.open = !c.open
	if c.open {
		c.body.Show()
		c.arrow.SetText("⌄")
	} else {
		c.body.Hide()
		c.arrow.SetText("›")
	}
	c.arrow.Refresh()
	c.body.Refresh()
}

// stats 返回该分类下的可勾选行数与已选行数。
func (c *categoryCard) stats() (selected, total int) {
	for _, r := range c.rows {
		if !r.settable() {
			continue
		}
		total++
		if c.view.selected[r.item.Path] {
			selected++
		}
	}
	return selected, total
}

// setChecked 全选/全不选该分类。
func (c *categoryCard) setChecked(checked bool) {
	for _, r := range c.rows {
		if !r.settable() {
			continue
		}
		c.view.selected[r.item.Path] = checked
		r.syncCheck()
	}
	c.updateHeader()
	c.view.updateSelection()
}

// updateHeader 依当前勾选刷新分类级复选框的三态。
func (c *categoryCard) updateHeader() {
	sel, total := c.stats()
	cb := c.check
	cb.OnChanged = nil
	cb.SetChecked(total > 0 && sel == total)
	cb.SetPartial(sel > 0 && sel < total)
	cb.OnChanged = func(checked bool) { c.setChecked(checked) }
}

// refreshRows 将行级复选框与选中状态同步。
func (c *categoryCard) refreshRows() {
	for _, r := range c.rows {
		r.syncCheck()
	}
}

// fileRow 是明细列表的一行。
type fileRow struct {
	card  *categoryCard
	item  model.Item
	check *widget.Check
}

// settable 返回该行是否可勾选清理。
func (r *fileRow) settable() bool {
	return !r.item.Permanent && !r.item.ReadOnly
}

// newFileRow 构造一行。
func (c *categoryCard) newFileRow(it model.Item) *fileRow {
	r := &fileRow{card: c, item: it}
	r.check = widget.NewCheck("", func(checked bool) {
		c.view.setItemChecked(it.Path, checked)
	})
	return r
}

// build 组装行布局。
func (r *fileRow) build() fyne.CanvasObject {
	icon := widget.NewIcon(categoryIcon(r.item.Category))
	iconBox := container.NewGridWrap(fyne.NewSize(18, 18), icon)

	nameLbl := widget.NewLabel(r.item.DisplayName)
	nameLbl.Truncation = fyne.TextTruncateEllipsis
	pathLbl := widget.NewLabel(r.item.Path)
	pathLbl.Truncation = fyne.TextTruncateEllipsis
	pathLbl.Importance = widget.LowImportance
	nameCol := container.NewVBox(
		container.NewHBox(iconBox, nameLbl),
		pathLbl,
	)

	var right []fyne.CanvasObject
	if r.item.Note != "" {
		note := makeText(r.item.Note, 11, colFg3, false, fyne.TextAlignTrailing)
		right = append(right, note)
	}
	if r.settable() {
		ex := widget.NewButton("🚫 排除", func() { r.card.view.excludeRow(r.item) })
		ex.Importance = widget.LowImportance
		right = append(right, ex)
	}
	size := makeText(model.HumanBytes(r.item.Size), 13, colFg, true, fyne.TextAlignTrailing)
	right = append(right, size)

	left := container.NewHBox(r.check, nameCol)
	if !r.settable() {
		left = nameCol // 只读项（指定目录扫描）不带复选框
	}
	return container.NewPadded(container.NewBorder(nil, nil, nil,
		container.NewHBox(right...), left))
}

// syncCheck 把行复选框的值同步为选中状态（不触发回调）。
func (r *fileRow) syncCheck() {
	if !r.settable() {
		return
	}
	cb := r.check
	cb.OnChanged = nil
	cb.SetChecked(r.card.view.selected[r.item.Path])
	cb.Refresh()
	cb.OnChanged = func(checked bool) {
		r.card.view.setItemChecked(r.item.Path, checked)
	}
}

// setItemChecked 勾选/取消单个文件，向上同步分类与汇总栏。
func (v *adviceView) setItemChecked(path string, checked bool) {
	v.selected[path] = checked
	for _, c := range v.cards {
		c.updateHeader()
	}
	v.updateSelection()
}

// syncAll 把全部行与分类勾选状态重算并刷新。
func (v *adviceView) syncAll() {
	for _, c := range v.cards {
		c.refreshRows()
		c.updateHeader()
	}
	v.updateSelection()
}

// clearSelection 清空全部勾选。
func (v *adviceView) clearSelection() {
	v.selected = map[string]bool{}
	v.syncAll()
}

// selectAllRecommended 勾选/取消全部「推荐清理」（低风险）项。
func (v *adviceView) selectAllRecommended(checked bool) {
	for _, c := range v.cards {
		for _, r := range c.rows {
			if !r.settable() || r.item.Risk != model.RiskLow {
				continue
			}
			v.selected[r.item.Path] = checked
		}
	}
	v.syncAll()
}

// selectedItems 返回全部已勾选项。
func (v *adviceView) selectedItems() []model.Item {
	var out []model.Item
	for _, c := range v.cards {
		for _, r := range c.rows {
			if v.selected[r.item.Path] {
				out = append(out, r.item)
			}
		}
	}
	return out
}

// excludeRow 把该文件所在目录加入排除列表，随后重筛报告并提示。
func (v *adviceView) excludeRow(it model.Item) {
	dir := filepath.Dir(it.Path)
	if err := v.shell.ctrl.AddExcludeRule(dir); err != nil {
		v.shell.toast("添加排除规则失败：" + err.Error())
		return
	}
	v.shell.report = v.shell.ctrl.RefilterReport(v.shell.report)
	v.shell.refreshAllViews()
	v.shell.toast("已排除目录 " + dir + "，不再扫描")
}

// updateSelection 刷新底部汇总栏与清理按钮状态。
// 「全选推荐项」同步为与推荐项（低风险）选中情况一致的三态。
func (v *adviceView) updateSelection() {
	items := v.selectedItems()
	v.syncPickAll()
	if len(items) == 0 {
		v.mainSize.Text = "0 B"
		v.mainSub.Text = "已选 0 个项目"
		v.mainSize.Refresh()
		v.mainSub.Refresh()
		v.barBox.Hide()
		v.cleanBtn.Disable()
		return
	}

	var total int64
	for _, it := range items {
		total += it.Size
	}
	v.mainSize.Text = model.HumanBytes(total)
	v.mainSub.Text = fmt.Sprintf("已选 %d 个项目", len(items))
	v.mainSize.Refresh()
	v.mainSub.Refresh()
	v.barBox.Show()
	v.barBox.Refresh()
	v.cleanBtn.Enable()
	v.cleanBtn.SetText(fmt.Sprintf("🗑 安全清理 (%d)", len(items)))
	v.cleanBtn.Refresh()
}

// syncPickAll 把「全选推荐项」复选框设为与低风险项选中情况一致的三态。
func (v *adviceView) syncPickAll() {
	var total, sel int
	for _, c := range v.cards {
		for _, r := range c.rows {
			if !r.settable() || r.item.Risk != model.RiskLow {
				continue
			}
			total++
			if v.selected[r.item.Path] {
				sel++
			}
		}
	}
	cb := v.pickAll
	cb.OnChanged = nil
	cb.SetChecked(total > 0 && sel == total)
	cb.Partial = sel > 0 && sel < total
	cb.Refresh()
	cb.OnChanged = func(checked bool) { v.selectAllRecommended(checked) }
}
