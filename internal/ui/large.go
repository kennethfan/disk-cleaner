package ui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// largeView 是大文件页。
type largeView struct {
	shell *appShell
	root  fyne.CanvasObject

	rowsBox  *fyne.Container
	rows     []*largeRow
	selected map[string]bool

	// 底部汇总栏。
	barBox         *fyne.Container
	sumSizeText    *canvas.Text
	sumCountText   *canvas.Text
	cleanLargeBtn  *widget.Button
	threshNoteText *canvas.Text
}

func newLargeView(s *appShell) *largeView {
	v := &largeView{
		shell:    s,
		selected: make(map[string]bool),
	}
	v.rowsBox = container.NewVBox()

	headCols := container.NewHBox(
		makeText("文件", 12, colFg2, true, fyne.TextAlignLeading),
		makeText("位置", 12, colFg2, true, fyne.TextAlignLeading),
		makeText("修改时间", 12, colFg2, true, fyne.TextAlignTrailing),
		makeText("大小", 12, colFg2, true, fyne.TextAlignTrailing),
	)

	list := container.NewStack(
		canvas.NewRectangle(colWindowBg),
		container.NewVScroll(container.NewVBox(
			container.NewPadded(headCols),
			container.NewPadded(v.rowsBox),
		)))

	v.sumSizeText = makeText("0 B", 15, colFg, true, fyne.TextAlignLeading)
	v.sumCountText = makeText("已选 0 个大文件", 12, colFg2, false, fyne.TextAlignLeading)
	v.cleanLargeBtn = widget.NewButton("🗑 移入废纸篓", func() {
		v.shell.confirmClean(v.selectedItems(), "确认移入废纸篓？")
	})
	v.cleanLargeBtn.Importance = widget.DangerImportance
	v.cleanLargeBtn.Disable()

	barBg := canvas.NewRectangle(colCardBg)
	v.barBox = container.NewStack(barBg,
		container.NewVBox(widget.NewSeparator(),
			container.NewPadded(container.NewHBox(
				container.NewVBox(v.sumSizeText, v.sumCountText),
				layout.NewSpacer(),
				v.cleanLargeBtn,
			))))
	v.barBox.Hide()

	v.root = container.NewBorder(nil, v.barBox, nil, nil, list)
	return v
}

// setReport 用大文件分类的结果重建表格。r 可为 nil。
func (v *largeView) setReport(r *model.Report) {
	if r == nil {
		v.rows = nil
		v.selected = make(map[string]bool)
		v.rowsBox.Objects = nil
		v.updateSelection()
		v.rowsBox.Refresh()
		return
	}

	items := r.ByCategory()[model.CategoryLargeFile].Items
	// 剪掉已不存在的勾选。
	live := make(map[string]bool)
	for _, it := range items {
		live[it.Path] = v.selected[it.Path]
	}
	v.selected = live

	v.rows = make([]*largeRow, 0, len(items))
	v.rowsBox.Objects = nil
	for _, it := range items {
		row := &largeRow{view: v, item: it}
		row.check = widget.NewCheck("", func(checked bool) {
			v.selected[it.Path] = checked
			v.updateSelection()
		})
		v.rows = append(v.rows, row)
		v.rowsBox.Add(row.build())
	}
	// 同步已勾选状态（重渲染时保留）。
	for _, row := range v.rows {
		row.syncCheck()
	}
	v.rowsBox.Refresh()
	v.updateSelection()
}

// selectedItems 返回已勾选的大文件。
func (v *largeView) selectedItems() []model.Item {
	var out []model.Item
	for _, row := range v.rows {
		if v.selected[row.item.Path] {
			out = append(out, row.item)
		}
	}
	return out
}

// updateSelection 刷新底部汇总栏。
func (v *largeView) updateSelection() {
	var items []model.Item
	var total int64
	for _, row := range v.rows {
		if v.selected[row.item.Path] {
			items = append(items, row.item)
			total += row.item.Size
		}
	}
	if len(items) == 0 {
		v.sumSizeText.Text = "0 B"
		v.sumCountText.Text = "已选 0 个大文件"
		v.barBox.Hide()
		v.cleanLargeBtn.Disable()
	} else {
		v.sumSizeText.Text = model.HumanBytes(total)
		v.sumCountText.Text = fmt.Sprintf("已选 %d 个大文件", len(items))
		v.barBox.Show()
		v.cleanLargeBtn.Enable()
	}
	v.sumSizeText.Refresh()
	v.sumCountText.Refresh()
	v.barBox.Refresh()
}

// largeRow 是大文件表的一行。
type largeRow struct {
	view  *largeView
	item  model.Item
	check *widget.Check
}

// syncCheck 把行复选框同步为选中状态（不触发回调）。
func (r *largeRow) syncCheck() {
	if r.item.Permanent || r.item.ReadOnly {
		return
	}
	r.check.OnChanged = nil
	r.check.SetChecked(r.view.selected[r.item.Path])
	r.check.Refresh()
	r.check.OnChanged = func(checked bool) {
		r.view.selected[r.item.Path] = checked
		r.view.updateSelection()
	}
}

// build 组装行布局。
func (r *largeRow) build() fyne.CanvasObject {
	nameLbl := widget.NewLabel(r.item.DisplayName)
	nameLbl.Truncation = fyne.TextTruncateEllipsis

	pathLbl := widget.NewLabel(r.item.Path)
	pathLbl.Truncation = fyne.TextTruncateEllipsis
	pathLbl.Importance = widget.LowImportance

	nameCol := container.NewVBox(nameLbl, pathLbl)

	ageLbl := makeText(fileAge(r.item.Path), 12, colFg3, false, fyne.TextAlignTrailing)
	sizeLbl := makeText(model.HumanBytes(r.item.Size), 13, colFg, true, fyne.TextAlignTrailing)

	left := container.NewHBox(r.check, nameCol)
	if r.item.Permanent || r.item.ReadOnly {
		left = nameCol
	}
	return container.NewPadded(container.NewBorder(nil, nil, nil,
		container.NewHBox(ageLbl, sizeLbl), left))
}

// fileAge 返回文件距今天数的描述。
func fileAge(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "—"
	}
	days := int(time.Since(info.ModTime()).Hours() / 24)
	if days <= 0 {
		return "今天"
	}
	return fmt.Sprintf("%d 天前", days)
}
