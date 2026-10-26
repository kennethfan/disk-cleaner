package ui

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// itemRow 是列表中的一行，持有勾选状态与对应数据。
type itemRow struct {
	item    model.Item
	checked bool
	check   *widget.Check
}

// mainView 是主界面。
type mainView struct {
	win  fyne.Window
	ctrl *Controller

	summary   *widget.Label
	statusLbl *widget.Label
	progress  *widget.ProgressBarInfinite
	scroll    *container.Scroll
	cleanBtn  *widget.Button
	rescanBtn *widget.Button

	// rows 按类别分组保存，便于"全选本类别"操作。
	rows []*itemRow
	// report 是最近一次扫描结果，供清理后刷新使用。
	report *model.Report
	// scanning 防止重复触发扫描。
	scanning bool
}

// build 组装主界面。
func (v *mainView) build() fyne.CanvasObject {
	v.summary = widget.NewLabel("尚未扫描")
	v.summary.TextStyle = fyne.TextStyle{Bold: true}

	v.statusLbl = widget.NewLabel("点击「重新扫描」开始")
	v.statusLbl.TextStyle = fyne.TextStyle{Italic: true}

	v.progress = widget.NewProgressBarInfinite()
	v.progress.Hide()

	v.rescanBtn = widget.NewButton("重新扫描", v.onRescan)
	v.cleanBtn = widget.NewButton("清理选中项", v.onClean)
	v.cleanBtn.Disable()

	// 空状态提示。
	v.scroll = container.NewVScroll(container.NewVBox(
		widget.NewLabel("点击「重新扫描」查找可清理的内容"),
	))

	header := container.NewVBox(
		container.NewHBox(
			canvas.NewText("macOS 磁盘空间清理", themeColorForeground()),
			layout.NewSpacer(),
		),
		v.summary,
		v.statusLbl,
		v.progress,
		widget.NewSeparator(),
	)

	bottom := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			v.rescanBtn,
			layout.NewSpacer(),
			// 清理按钮给足最小宽度，避免文案变化时被挤压。
			container.NewGridWrap(fyne.NewSize(160, 36), v.cleanBtn),
		),
	)

	return container.NewBorder(header, bottom, nil, nil, v.scroll)
}

// onRescan 触发一次异步扫描。
func (v *mainView) onRescan() {
	if v.scanning {
		return
	}
	v.scanning = true
	v.rescanBtn.Disable()
	v.cleanBtn.Disable()
	v.progress.Show()
	v.statusLbl.SetText("正在扫描……（大目录可能需要几十秒）")

	go func() {
		report := v.ctrl.Scan()

		// 所有 UI 更新必须回到 Fyne 主线程。
		fyne.Do(func() {
			v.report = report
			v.renderReport(report)
			v.scanning = false
			v.rescanBtn.Enable()
			v.progress.Hide()
		})
	}()
}

// renderReport 把扫描结果渲染到列表。
func (v *mainView) renderReport(r *model.Report) {
	v.rows = nil
	groups := r.ByCategory()

	order := []model.Category{
		model.CategoryDevCache,
		model.CategoryUserCache,
		model.CategorySystemCache,
		model.CategoryJunk,
		model.CategoryLargeFile,
		model.CategorySnapshot,
	}

	list := container.NewVBox()
	for _, cat := range order {
		g, ok := groups[cat]
		if !ok || len(g.Items) == 0 {
			continue
		}
		list.Add(v.buildCategoryHeader(cat, g))

		// 组内按大小降序，已在 Report.Finalize 中保证，这里直接使用。
		for i := range g.Items {
			list.Add(v.buildRow(g.Items[i]))
		}
		list.Add(widget.NewSeparator())
	}

	if len(v.rows) == 0 {
		list.Add(widget.NewLabel("未发现可清理的内容。"))
		v.cleanBtn.Disable()
	}

	// Scroll 没有 SetContent 方法，替换 Content 字段后需显式 Refresh。
	v.scroll.Content = list
	v.scroll.Refresh()
	v.updateSummary()

	msg := fmt.Sprintf("扫描完成：%d 项，合计 %s", len(r.Items), model.HumanBytes(r.TotalBytes))
	if len(r.Errors) > 0 {
		msg += fmt.Sprintf("（%d 个扫描器出错）", len(r.Errors))
	}
	v.statusLbl.SetText(msg)
}

// buildCategoryHeader 构造类别标题行，带"全选本类"按钮。
func (v *mainView) buildCategoryHeader(cat model.Category, g model.CategoryGroup) fyne.CanvasObject {
	icon := widget.NewIcon(categoryIcon(cat))
	title := canvas.NewText(
		fmt.Sprintf("%s  ·  %s", cat, model.HumanBytes(g.Bytes)),
		themeColorForeground(),
	)
	title.TextStyle = fyne.TextStyle{Bold: true}

	selectAll := widget.NewButton("全选本类", func() {
		for _, row := range v.rows {
			if row.item.Category == cat {
				row.check.SetChecked(true)
			}
		}
	})
	selectAll.Importance = widget.LowImportance

	selectNone := widget.NewButton("全不选", func() {
		for _, row := range v.rows {
			if row.item.Category == cat {
				row.check.SetChecked(false)
			}
		}
	})
	selectNone.Importance = widget.LowImportance

	return container.NewHBox(icon, title, layout.NewSpacer(), selectAll, selectNone)
}

// buildRow 构造单条清理项。
func (v *mainView) buildRow(it model.Item) fyne.CanvasObject {
	row := &itemRow{item: it}

	label := fmt.Sprintf("%s   %s", it.DisplayName, model.HumanBytes(it.Size))
	if it.Permanent {
		label += "   [需专用流程]"
	}

	row.check = widget.NewCheck(label, func(checked bool) {
		row.checked = checked
		v.updateSummary()
	})
	// 永久删除项不允许勾选：它们不走废纸篓流程，必须用专用方式处理。
	if it.Permanent {
		row.check.Disable()
	}

	// 风险标记用 canvas.Text 以保留颜色，但必须放进固定宽度的 GridWrap：
	// canvas.Text 的 MinSize 对中文字形宽度估算偏小，直接放在 Border 右槽
	// 会被压缩到末尾字符被裁切。固定宽度 + 左对齐可确保完整显示。
	risk := canvas.NewText(riskBadge(it.Risk), riskColor(it.Risk))
	risk.TextStyle = fyne.TextStyle{Bold: true}
	risk.Alignment = fyne.TextAlignLeading

	v.rows = append(v.rows, row)
	riskBox := container.NewGridWrap(fyne.NewSize(riskBadgeWidth, riskSizeHeight), risk)
	return container.NewBorder(nil, nil, nil, riskBox, row.check)
}

// 风险标记的固定尺寸：留足中文字形宽度，避免被裁切。
const (
	riskBadgeWidth float32 = 72
	riskSizeHeight float32 = 32
)

// updateSummary 根据勾选状态刷新顶部摘要与按钮可用性。
func (v *mainView) updateSummary() {
	var count int
	var bytes int64
	for _, row := range v.rows {
		if row.checked {
			count++
			bytes += row.item.Size
		}
	}

	if v.report != nil {
		v.summary.SetText(fmt.Sprintf("共 %d 项，合计 %s  ·  已选 %d 项（%s）",
			len(v.report.Items),
			model.HumanBytes(v.report.TotalBytes),
			count,
			model.HumanBytes(bytes),
		))
	}

	if count == 0 {
		v.cleanBtn.Disable()
		v.cleanBtn.SetText("清理选中项")
		return
	}
	v.cleanBtn.Enable()
	// 按钮文案保持简短，避免窗口较窄时被裁切；
	// 具体的项数与体积已在上方摘要中展示。
	v.cleanBtn.SetText(fmt.Sprintf("清理选中项 (%d)", count))
}

// selected 返回当前勾选的项。
func (v *mainView) selected() []model.Item {
	var out []model.Item
	for _, row := range v.rows {
		if row.checked {
			out = append(out, row.item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

// onClean 走二次确认后执行清理。
func (v *mainView) onClean() {
	items := v.selected()
	if len(items) == 0 {
		return
	}

	var total int64
	highRisk := 0
	for _, it := range items {
		total += it.Size
		if it.Risk == model.RiskHigh {
			highRisk++
		}
	}

	body := fmt.Sprintf("将把 %d 项（共 %s）移入废纸篓。\n\n"+
		"这些内容可以随时从废纸篓恢复。", len(items), model.HumanBytes(total))
	if highRisk > 0 {
		body += fmt.Sprintf("\n\n⚠ 其中 %d 项为高风险，删除后可能需要重新下载或重新生成。", highRisk)
	}

	d := dialog.NewConfirm("确认清理", body, func(ok bool) {
		if !ok {
			return
		}
		v.runClean(items)
	}, v.win)
	d.SetConfirmText("移入废纸篓")
	d.SetDismissText("取消")
	d.Show()
}

// runClean 异步执行清理并汇报结果。
func (v *mainView) runClean(items []model.Item) {
	v.cleanBtn.Disable()
	v.rescanBtn.Disable()
	v.progress.Show()
	v.statusLbl.SetText(fmt.Sprintf("正在清理 %d 项……", len(items)))

	go func() {
		res := v.ctrl.Clean(items)

		fyne.Do(func() {
			v.progress.Hide()
			v.rescanBtn.Enable()
			v.showResult(res)
			// 清理后重新扫描，让列表反映最新状态。
			v.onRescan()
		})
	}()
}

// showResult 弹出清理结果。
func (v *mainView) showResult(res CleanResult) {
	var body string
	if res.Failed() == 0 {
		body = fmt.Sprintf("成功清理 %d 项，释放 %s。\n\n内容已移入废纸篓，可随时恢复。",
			res.Succeeded(), model.HumanBytes(res.FreedBytes))
	} else {
		body = fmt.Sprintf("成功 %d 项，失败 %d 项，释放 %s。\n\n失败详情：\n",
			res.Succeeded(), res.Failed(), model.HumanBytes(res.FreedBytes))
		n := 0
		for _, o := range res.Outcomes {
			if o.Err == nil {
				continue
			}
			body += fmt.Sprintf("· %s\n  %v\n", o.Item.DisplayName, o.Err)
			n++
			if n >= 8 {
				body += "……\n"
				break
			}
		}
	}

	d := dialog.NewInformation("清理完成", body, v.win)
	d.Show()
}
