package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// excludeView 是排除列表页。
type excludeView struct {
	shell *appShell
	root  fyne.CanvasObject

	rowsBox *fyne.Container
	input   *widget.Entry
}

func newExcludeView(s *appShell) *excludeView {
	v := &excludeView{shell: s}

	desc := widget.NewLabel("以下路径在扫描时将被完全跳过，不计入可清理空间。规则自动保存，长期生效，支持通配符 *。")
	desc.Wrapping = fyne.TextWrapWord

	v.input = widget.NewEntry()
	v.input.SetPlaceHolder("输入路径或名称，如 ~/Library/Caches/JetBrains 或 node_modules 或 *.log")
	v.input.OnSubmitted = func(string) { v.addRule() }

	addBtn := widget.NewButton("＋ 加入排除", func() { v.addRule() })
	addBtn.Importance = widget.HighImportance
	pickBtn := widget.NewButton("选择目录…", func() { v.pickDir() })
	pickBtn.Importance = widget.LowImportance

	addRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(pickBtn, addBtn), v.input)

	v.rowsBox = container.NewVBox()

	card := makeCard(container.NewVBox(
		widget.NewSeparator(),
		v.rowsBox,
	))

	content := container.NewVBox(
		container.NewPadded(desc),
		container.NewPadded(addRow),
		container.NewPadded(card),
	)
	v.root = container.NewStack(
		canvas.NewRectangle(colWindowBg),
		container.NewVScroll(content),
	)
	v.refresh()
	return v
}

// refresh 重建规则列表。
func (v *excludeView) refresh() {
	v.rowsBox.Objects = nil
	rules := v.shell.ctrl.ExcludeRules()
	if len(rules) == 0 {
		v.rowsBox.Add(makeText("暂无排除项 · 扫描将覆盖默认白名单内全部目录", 12.5, colFg3, false, fyne.TextAlignLeading))
		v.rowsBox.Refresh()
		return
	}
	for _, rule := range rules {
		r := rule
		lbl := widget.NewLabel(r)
		lbl.Truncation = fyne.TextTruncateEllipsis

		del := widget.NewButton("移除", func() {
			if err := v.shell.ctrl.RemoveExcludeRule(r); err != nil {
				dialog.ShowError(err, v.shell.win)
				return
			}
			v.refresh()
			v.shell.refreshNav()
			v.shell.toast("已移除排除规则，下次扫描生效")
		})
		del.Importance = widget.LowImportance

		left := container.NewHBox(
			makeText("🚫", 13, colFg2, false, fyne.TextAlignLeading),
			lbl)
		v.rowsBox.Add(container.NewBorder(nil, nil, nil, del, left))
	}
	v.rowsBox.Refresh()
}

// addRule 从输入框添加规则。
func (v *excludeView) addRule() {
	rule := v.input.Text
	if rule == "" {
		v.shell.toast("请输入排除路径")
		return
	}
	if err := v.shell.ctrl.AddExcludeRule(rule); err != nil {
		dialog.ShowError(err, v.shell.win)
		return
	}
	v.input.SetText("")
	v.refresh()
	v.shell.refreshNav()
	v.shell.toast("已加入排除列表，下次扫描自动跳过")
}

// pickDir 用目录选择器直接选一个目录加入排除。
func (v *excludeView) pickDir() {
	dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, v.shell.win)
			return
		}
		if uri == nil {
			return // 用户取消
		}
		if err := v.shell.ctrl.AddExcludeRule(uri.Path()); err != nil {
			dialog.ShowError(err, v.shell.win)
			return
		}
		v.refresh()
		v.shell.refreshNav()
		v.shell.toast("已加入排除列表，下次扫描自动跳过")
	}, v.shell.win).Show()
}
