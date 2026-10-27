package ui

import (
	"fmt"
	"strings"
	"time"

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

	summary     *widget.Label
	statusLbl   *widget.Label
	progress    *widget.ProgressBarInfinite
	cleanBtn    *widget.Button
	rescanBtn   *widget.Button
	pickDirBtn  *widget.Button
	resetDirBtn *widget.Button
	excludeBtn  *widget.Button
	pauseBtn    *widget.Button
	cancelBtn   *widget.Button
	tree        *widget.Tree
	model       *treeModel

	// report 是最近一次扫描结果。
	report *model.Report
	// scanning 防止重复触发扫描。
	scanning bool
	// paused 记录当前是否处于暂停状态，用于按钮文案切换。
	paused bool
	// progressStop 用于停止进度轮询 goroutine。
	progressStop chan struct{}
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

	v.pickDirBtn = widget.NewButton("选择目录…", v.onPickDir)
	v.resetDirBtn = widget.NewButton("返回默认扫描", v.onResetDir)
	v.resetDirBtn.Hide()
	v.excludeBtn = widget.NewButton("排除规则…", v.onManageExclude)

	// 扫描期间才显示的暂停/取消按钮。
	v.pauseBtn = widget.NewButton("暂停", v.onTogglePause)
	v.pauseBtn.Hide()
	v.cancelBtn = widget.NewButton("取消", v.onCancelScan)
	v.cancelBtn.Importance = widget.DangerImportance
	v.cancelBtn.Hide()

	// 树模型与树组件。
	v.model = newTreeModel(v)
	v.tree = v.model.build()
	v.tree.OnBranchOpened = func(id treeNodeID) {
		// 只在首次展开时触发加载；已加载的节点由 loadChildren 内部拦截。
		v.model.loadChildren(id, v.tree)
	}

	// 工具栏：扫描范围 + 排除规则
	toolbar := container.NewHBox(
		v.pickDirBtn,
		v.resetDirBtn,
		layout.NewSpacer(),
		v.excludeBtn,
	)

	header := container.NewVBox(
		container.NewHBox(
			canvas.NewText("macOS 磁盘空间清理", themeColorForeground()),
			layout.NewSpacer(),
		),
		v.summary,
		v.statusLbl,
		v.progress,
		toolbar,
		widget.NewSeparator(),
	)

	bottom := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			v.rescanBtn,
			v.pauseBtn,
			v.cancelBtn,
			layout.NewSpacer(),
			// 清理按钮给足最小宽度，避免文案变化时被挤压。
			container.NewGridWrap(fyne.NewSize(160, 36), v.cleanBtn),
		),
	)

	return container.NewBorder(header, bottom, nil, nil, v.tree)
}

// onPickDir 弹出目录选择器，切换到指定目录扫描模式。
func (v *mainView) onPickDir() {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, v.win)
			return
		}
		if uri == nil {
			return // 用户取消
		}

		v.ctrl.SetTargetPaths([]string{uri.Path()})
		v.pickDirBtn.Hide()
		v.resetDirBtn.Show()
		v.onRescan()
	}, v.win)
	d.Show()
}

// onResetDir 恢复默认扫描范围。
func (v *mainView) onResetDir() {
	v.ctrl.SetTargetPaths(nil)
	v.resetDirBtn.Hide()
	v.pickDirBtn.Show()
	v.onRescan()
}

// onManageExclude 打开排除规则管理对话框。
func (v *mainView) onManageExclude() {
	// 规则列表：每行一条，带删除按钮。
	list := container.NewVBox()
	var refresh func()

	refresh = func() {
		list.Objects = nil
		rules := v.ctrl.ExcludeRules()
		if len(rules) == 0 {
			list.Add(widget.NewLabel("尚未设置排除规则"))
		}
		for _, r := range rules {
			rule := r
			del := widget.NewButton("删除", func() {
				if err := v.ctrl.RemoveExcludeRule(rule); err != nil {
					dialog.ShowError(err, v.win)
					return
				}
				refresh()
			})
			del.Importance = widget.LowImportance
			list.Add(container.NewHBox(
				widget.NewLabel(rule),
				layout.NewSpacer(),
				del,
			))
		}
		list.Refresh()
	}
	refresh()

	input := widget.NewEntry()
	input.SetPlaceHolder("输入路径或名称，如 ~/Library/Caches/JetBrains 或 node_modules 或 *.log")

	hint := widget.NewLabel("支持三种写法：绝对路径（含全部子项）、目录名（任意层级）、通配符（如 *.log）")
	hint.Wrapping = fyne.TextWrapWord
	hint.TextStyle = fyne.TextStyle{Italic: true}

	// addRule 是添加规则并刷新界面的公共逻辑，
	// 供文本输入与目录选择两条路径复用。
	addRule := func(rule string) bool {
		if err := v.ctrl.AddExcludeRule(rule); err != nil {
			dialog.ShowError(err, v.win)
			return false
		}
		refresh()
		v.onRescan()
		return true
	}

	// 目录选择：直接选文件夹加入排除，省去手打路径。
	pickBtn := widget.NewButton("选择目录…", func() {
		fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, v.win)
				return
			}
			if uri == nil {
				return // 用户取消
			}
			addRule(uri.Path())
		}, v.win)
		fd.Show()
	})
	// 按钮固定宽度，避免把输入框挤到只剩几个字符。
	pickBox := container.NewGridWrap(fyne.NewSize(110, 36), pickBtn)

	addBtn := widget.NewButton("添加并重新扫描", func() {
		// Fyne v2.8 的 Entry 用字段 Text 而非 Text() 方法。
		if addRule(input.Text) {
			input.SetText("")
		}
	})
	addBtn.Importance = widget.HighImportance

	pathLbl := widget.NewLabel("配置文件: " + v.ctrl.ConfigPath())
	pathLbl.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		widget.NewLabel("当前排除规则"),
		list,
		widget.NewSeparator(),
		widget.NewLabel("添加新规则"),
		container.NewBorder(nil, nil, nil, pickBox, input),
		hint,
		addBtn,
		widget.NewSeparator(),
		pathLbl,
	)

	d := dialog.NewCustom("排除规则", "关闭",
		container.NewVScroll(content), v.win)
	d.Resize(fyne.NewSize(620, 460))
	d.Show()
}

// onRescan 触发一次异步扫描。
func (v *mainView) onRescan() {
	if v.scanning {
		return
	}
	v.scanning = true
	v.paused = false

	// 每次扫描新建门闩：Gate 的取消是终止性的，复用旧实例
	// 会让新扫描一开始就被判为已取消。
	v.ctrl.NewGate()

	v.rescanBtn.Disable()
	v.cleanBtn.Disable()
	v.progress.Show()
	// 显式启动动画：上一次扫描若在暂停状态下结束，Stop 后
	// running 仍为 false，仅 Show 不会让它重新动起来。
	v.progress.Start()
	v.statusLbl.SetText("正在扫描……（大目录可能需要几十秒）")

	// 扫描中把「重新扫描」换成暂停/取消。
	v.pauseBtn.Show()
	v.cancelBtn.Show()
	v.pauseBtn.SetText("暂停")

	go func() {
		report := v.ctrl.Scan()

		// 所有 UI 更新必须回到 Fyne 主线程。
		fyne.Do(func() {
			v.scanning = false
			v.paused = false
			v.stopProgressTicker()
			v.rescanBtn.Enable()
			// 先 Stop 再 Hide：若在暂停状态下结束，动画仍在 running，
			// 仅 Hide 会让它在下次 Show 时立刻继续跑。
			v.progress.Stop()
			v.progress.Hide()
			v.pauseBtn.Hide()
			v.cancelBtn.Hide()

			// 被用户取消的扫描不展示结果，保留上一次的列表。
			if g := v.ctrl.CurrentGate(); g != nil && g.Canceled() {
				v.statusLbl.SetText("扫描已取消")
				return
			}

			v.report = report
			v.renderReport(report)
		})
	}()

	// 进度轮询：扫描期间每秒刷新一次进度文案。
	v.startProgressTicker()
}

// startProgressTicker 在扫描期间周期性刷新进度显示。
func (v *mainView) startProgressTicker() {
	stop := make(chan struct{})
	v.progressStop = stop

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fyne.Do(func() { v.refreshProgressText() })
			}
		}
	}()
}

// stopProgressTicker 停止进度轮询 goroutine。
func (v *mainView) stopProgressTicker() {
	if v.progressStop != nil {
		close(v.progressStop)
		v.progressStop = nil
	}
}

// refreshProgressText 更新状态栏中的扫描进度。
func (v *mainView) refreshProgressText() {
	if !v.scanning {
		return
	}
	g := v.ctrl.CurrentGate()
	if g == nil {
		return
	}

	done, total := g.Progress()
	state := "正在扫描"
	if g.Paused() {
		state = "已暂停"
	}
	v.statusLbl.SetText(fmt.Sprintf("%s…… 已完成 %d/%d 个扫描器", state, done, total))
}

// onTogglePause 在暂停与继续之间切换。
func (v *mainView) onTogglePause() {
	g := v.ctrl.CurrentGate()
	if g == nil {
		return
	}

	if g.Paused() {
		g.Resume()
		v.paused = false
		v.pauseBtn.SetText("暂停")
		// 恢复扫描时重启进度条动画。
		v.progress.Start()
	} else {
		g.Pause()
		v.paused = true
		v.pauseBtn.SetText("继续")
		// 暂停时停掉动画：ProgressBarInfinite 是与扫描进度无关的
		// 无限循环动画，不停止的话暂停后它仍在跑，视觉上会让人
		// 误以为扫描还在继续。
		v.progress.Stop()
	}
	v.refreshProgressText()
}

// onCancelScan 取消当前扫描。
func (v *mainView) onCancelScan() {
	g := v.ctrl.CurrentGate()
	if g == nil {
		return
	}
	// Cancel 同时会唤醒暂停中的等待者，无需先 Resume。
	g.Cancel()
	v.statusLbl.SetText("正在取消……")
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

	v.statusLbl.SetText(v.buildStatusMessage(r))
}

// buildStatusMessage 生成状态栏文案，汇总扫描范围、排除情况与错误。
func (v *mainView) buildStatusMessage(r *model.Report) string {
	var parts []string

	if len(r.TargetPaths) > 0 {
		parts = append(parts, fmt.Sprintf("指定目录扫描：%s（仅可查看，不可清理）",
			strings.Join(r.TargetPaths, ", ")))
	} else {
		parts = append(parts, fmt.Sprintf("扫描完成：%d 项，合计 %s",
			len(r.Items), model.HumanBytes(r.TotalBytes)))
		parts = append(parts, "点击带箭头的目录可展开查看明细")
	}

	// 明确告知排除规则生效了，避免用户误以为扫描漏掉了内容。
	if r.ExcludedCount > 0 {
		parts = append(parts, fmt.Sprintf("已排除 %d 项（%s）",
			r.ExcludedCount, model.HumanBytes(r.ExcludedBytes)))
	}

	if len(r.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个扫描器出错", len(r.Errors)))
	}

	msg := strings.Join(parts, " · ")
	if w := v.ctrl.ConfigWarning(); w != "" {
		msg += "　⚠ 配置文件读取失败：" + w
	}
	return msg
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
