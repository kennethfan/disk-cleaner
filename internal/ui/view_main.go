package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// mainView 是主界面。
//
// 列表用树形结构呈现：类别 → 清理项 → 子目录。
// 子目录按需加载（展开时才扫描），因为实测 ~/Library/Caches
// 有 104 个条目、~/Downloads 有 163 个，一次性全展开会卡死界面。
type mainView struct {
	win  fyne.Window
	ctrl *Controller

	summary   *widget.Label
	statusLbl *widget.Label
	progress  *widget.ProgressBarInfinite
	cleanBtn  *widget.Button
	rescanBtn *widget.Button
	tree      *widget.Tree
	model     *treeModel

	// report 是最近一次扫描结果。
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

	// 树模型与树组件。
	v.model = newTreeModel(v)
	v.tree = v.model.build()
	v.tree.OnBranchOpened = func(id treeNodeID) {
		// 只在首次展开时触发加载；已加载的节点由 loadChildren 内部拦截。
		v.model.loadChildren(id, v.tree)
	}

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

	return container.NewBorder(header, bottom, nil, nil, v.tree)
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

// categoryOrder 是类别在树中的展示顺序，按可清理体积从大到小排列。
var categoryOrder = []model.Category{
	model.CategoryDevCache,
	model.CategoryUserCache,
	model.CategorySystemCache,
	model.CategoryJunk,
	model.CategoryLargeFile,
	model.CategorySnapshot,
}

// renderReport 把扫描结果填进树模型并刷新。
func (v *mainView) renderReport(r *model.Report) {
	v.model.reset()

	groups := r.ByCategory()
	var roots []treeNodeID
	for _, cat := range categoryOrder {
		g, ok := groups[cat]
		if !ok || len(g.Items) == 0 {
			continue
		}
		id := catNodeID(cat)
		v.model.setCategoryItems(cat, g.Items)
		roots = append(roots, id)
	}

	// 树必须有唯一根节点，这里用 rootNodeID 作为全局汇总节点。
	v.model.mu.Lock()
	v.model.children[rootNodeID] = roots
	v.model.loaded[rootNodeID] = true
	v.model.mu.Unlock()

	v.tree.Root = rootNodeID
	v.tree.OpenAllBranches()
	v.tree.Refresh()
	v.updateSummary()

	msg := fmt.Sprintf("扫描完成：%d 项，合计 %s。点击带箭头的目录可展开查看明细。",
		len(r.Items), model.HumanBytes(r.TotalBytes))
	if len(r.Errors) > 0 {
		msg += fmt.Sprintf("（%d 个扫描器出错）", len(r.Errors))
	}
	v.statusLbl.SetText(msg)
}

// updateSummary 根据勾选状态刷新顶部摘要与按钮可用性。
func (v *mainView) updateSummary() {
	items := v.model.selectedItems()

	var bytes int64
	for _, it := range items {
		bytes += it.Size
	}
	count := len(items)

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

// onClean 走二次确认后执行清理。
func (v *mainView) onClean() {
	items := v.model.selectedItems()
	if len(items) == 0 {
		return
	}

	var total int64
	highRisk := 0
	dirCount := 0
	for _, it := range items {
		total += it.Size
		if it.Risk == model.RiskHigh {
			highRisk++
		}
		if it.IsDir {
			dirCount++
		}
	}

	body := fmt.Sprintf("将把 %d 项（共 %s）移入废纸篓。\n\n"+
		"这些内容可以随时从废纸篓恢复。", len(items), model.HumanBytes(total))
	if dirCount > 0 {
		body += fmt.Sprintf("\n\n其中 %d 项是完整目录，会连同其全部内容一起移动。", dirCount)
	}
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
