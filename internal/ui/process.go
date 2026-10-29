package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// ===================== 扫描中 =====================

// scanningView 是扫描进行中的流程态视图。
type scanningView struct {
	shell *appShell
	root  fyne.CanvasObject

	ring     *ringWidget
	stat     *canvas.Text
	hint     *canvas.Text
	pauseBtn *widget.Button
}

func newScanningView(s *appShell) *scanningView {
	v := &scanningView{shell: s}

	v.ring = newRingWidget(0.20, 92)
	v.stat = makeText("0%", 22, colFg, true, fyne.TextAlignCenter)
	v.hint = makeText("", 12, colFg3, false, fyne.TextAlignCenter)

	v.pauseBtn = widget.NewButton("暂停", func() { s.onTogglePause() })
	cancel := widget.NewButton("取消", func() { s.onCancelScan() })
	cancel.Importance = widget.DangerImportance

	content := makeCard(container.NewVBox(
		container.NewGridWrap(fyne.NewSize(92, 92), v.ring),
		v.stat,
		v.hint,
		container.NewHBox(v.pauseBtn, cancel),
	))

	// 让内容在页面里水平垂直居中。
	v.root = container.NewStack(
		canvas.NewRectangle(colWindowBg),
		container.NewScroll(container.NewCenter(
			container.NewPadded(container.NewGridWrap(
				fyne.NewSize(420, v.panelHeight()), content)),
		)),
	)
	return v
}

// panelHeight 是扫描面板的固定高度。
func (v *scanningView) panelHeight() float32 { return 280 }

// reset 重置进度显示并准备开始扫描。
func (v *scanningView) reset() {
	v.ring.SetProgress(0, colAccent)
	v.stat.Text = "0%"
	v.refreshHint()
	v.refreshPauseLabel()
	v.root.Refresh()
}

// tick 每个周期刷新一次进度。进度来自 Gate 的扫描器完成比例。
func (v *scanningView) tick() {
	g := v.shell.ctrl.CurrentGate()
	if g == nil {
		return
	}
	done, total := g.Progress()
	var ratio float32
	if total > 0 {
		ratio = float32(done) / float32(total)
	}
	v.ring.SetProgress(ratio, colAccent)
	v.stat.Text = fmt.Sprintf("%d%%", int(ratio*100))
	v.refreshHint()
	v.refreshPauseLabel()
}

// refreshHint 刷新底部提示行。
func (v *scanningView) refreshHint() {
	text := fmt.Sprintf("正在以只读方式分析磁盘 · 范围：%s", v.shell.scopeText())
	if v.shell.paused {
		text = "已暂停 · 点击「继续」恢复扫描"
	}
	v.hint.Text = text
	v.hint.Refresh()
}

// refreshPauseLabel 切换暂停/继续按钮文案。
func (v *scanningView) refreshPauseLabel() {
	if v.shell.paused {
		v.pauseBtn.SetText("继续")
	} else {
		v.pauseBtn.SetText("暂停")
	}
	v.pauseBtn.Refresh()
}

// ===================== 清理进度 =====================

// cleanRow 是清理清单里的一行。
type cleanRow struct {
	tick *canvas.Text
	name *widget.Label
}

// cleaningView 是清理执行中的流程态视图。
type cleaningView struct {
	shell *appShell
	root  fyne.CanvasObject

	bar     *widget.ProgressBar
	rowsBox *fyne.Container
	rows    []*cleanRow
	total   int
}

func newCleaningView(s *appShell) *cleaningView {
	v := &cleaningView{shell: s}

	v.bar = widget.NewProgressBar()
	v.rowsBox = container.NewVBox()

	content := makeCard(container.NewVBox(
		container.NewCenter(makeText("正在安全清理…", 18, colFg, true, fyne.TextAlignCenter)),
		v.bar,
		container.NewScroll(v.rowsBox),
	))
	v.root = container.NewStack(
		canvas.NewRectangle(colWindowBg),
		container.NewScroll(container.NewCenter(
			container.NewPadded(container.NewGridWrap(
				fyne.NewSize(460, 300), content))),
		),
	)
	return v
}

// rows 是按行序排列的行状态，与 shell.cleaningItems 一一对应。
func (v *cleaningView) rowsFrom(items []model.Item) {
	v.rows = make([]*cleanRow, 0, len(items))
	v.rowsBox.Objects = nil
	for _, it := range items {
		name := widget.NewLabel(it.DisplayName)
		name.Truncation = fyne.TextTruncateEllipsis
		name.Importance = widget.LowImportance
		tick := makeText("", 14, colOK, true, fyne.TextAlignLeading)
		v.rows = append(v.rows, &cleanRow{tick: tick, name: name})
		v.rowsBox.Add(container.NewHBox(tick, name))
	}
	v.rowsBox.Refresh()
}

// reset 清空清单并按 items 生成灰色行。
func (v *cleaningView) reset(items []model.Item) {
	v.total = len(items)
	v.bar.SetValue(0)
	v.rowsFrom(items)
	v.root.Refresh()
}

// rows 是按行序排列的行状态，与 items 顺序一致。

// mark 把第 idx 行按结果点亮：成功 ✓ 绿、失败/跳过 ✗ 红，并推进进度条。
func (v *cleaningView) mark(idx int, o clean.Outcome) {
	if idx < 0 || idx >= len(v.rows) {
		return
	}
	row := v.rows[idx]

	if o.Err != nil || o.Skipped {
		row.tick.Text = "✗"
		row.tick.Color = colBad
		row.name.Importance = widget.LowImportance
	} else {
		row.tick.Text = "✓"
		row.tick.Color = colOK
		row.name.Importance = widget.MediumImportance
	}
	row.tick.Refresh()
	row.name.Refresh()

	if v.total > 0 {
		v.bar.SetValue(float64(idx+1) / float64(v.total))
	}
}

// ===================== 完成 =====================

// doneView 是清理完成的流程态视图。
type doneView struct {
	shell *appShell
	root  fyne.CanvasObject

	big     *canvas.Text
	sub     *canvas.Text
	failLbl *canvas.Text
	undoBtn *widget.Button
}

func newDoneView(s *appShell) *doneView {
	v := &doneView{shell: s} // 完成态图标：圆形底（原型 .done-ico radius 50%）。
	ico := makeText("✓", 30, colOK, true, fyne.TextAlignCenter)
	icoBox := container.NewGridWrap(fyne.NewSize(64, 64),
		container.NewStack(
			newRoundRect(colOKTint, 32),
			container.NewCenter(ico),
		))

	v.big = makeText("", 34, colOK, true, fyne.TextAlignCenter)
	v.sub = makeText("", 12.5, colFg2, false, fyne.TextAlignCenter)
	v.failLbl = makeText("", 12, colBad, false, fyne.TextAlignCenter)
	v.failLbl.Hide()

	v.undoBtn = widget.NewButton("↩︎ 撤销本次清理", func() { s.undoClean() })
	overviewBtn := widget.NewButton("查看最新概览", func() { s.backToOverviewFromDone() })
	overviewBtn.Importance = widget.HighImportance

	content := makeCard(container.NewVBox(
		container.NewCenter(icoBox),
		container.NewCenter(makeText("清理完成", 18, colFg, true, fyne.TextAlignCenter)),
		container.NewCenter(v.big),
		container.NewCenter(v.sub),
		container.NewCenter(v.failLbl),
		container.NewHBox(v.undoBtn, overviewBtn),
	))

	v.root = container.NewStack(
		canvas.NewRectangle(colWindowBg),
		container.NewScroll(container.NewCenter(
			container.NewPadded(container.NewGridWrap(
				fyne.NewSize(460, 320), content))),
		),
	)
	return v
}

// show 展示本次清理结果。
func (v *doneView) show(res CleanResult) {
	v.big.Text = "+" + model.HumanBytes(res.FreedBytes)

	succeeded := res.Succeeded()
	v.sub.Text = fmt.Sprintf("%d 个项目已移入废纸篓 · 30 天内可随时恢复", succeeded)
	if res.Failed() > 0 {
		v.failLbl.Text = fmt.Sprintf("%d 项处理失败，未做任何改动", res.Failed())
		v.failLbl.Show()
	} else {
		v.failLbl.Text = ""
		v.failLbl.Hide()
	}
	v.root.Refresh()
}
