package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// viewID 标识主区的一个视图，与交互原型的信息架构一致。
type viewID int

const (
	viewOverview viewID = iota // 概览
	viewAdvice                 // 清理建议
	viewLarge                  // 大文件
	viewExclude                // 排除列表
	viewScanning               // 扫描中（流程态）
	viewCleaning               // 清理进度（流程态）
	viewDone                   // 完成（流程态）
)

// navItem 是侧边栏的一行导航项：按钮 + 右侧计数胶囊 + 激活底色（圆角 8px）。
type navItem struct {
	btn       *widget.Button
	badgeTxt  *canvas.Text
	badgeWrap fyne.CanvasObject
	bg        *roundRect
	inner     fyne.CanvasObject
}

// setActive 切换激活态：浅蓝圆角底。
func (n *navItem) setActive(on bool) {
	if on {
		n.bg.setColor(colAccentTint)
	} else {
		n.bg.setColor(color.NRGBA{A: 0})
	}
}

// appShell 是整个界面：侧边栏 + 主区。
// 它持有全局状态（扫描结果、进行中的流程）并负责视图切换与流程调度。
type appShell struct {
	win  fyne.Window
	ctrl *Controller

	// 侧边栏
	miniRing *ringWidget
	diskName *canvas.Text
	diskSub  *canvas.Text
	navBtns  map[viewID]*navItem

	// 顶部
	headTitle *widget.RichText
	headSub   *canvas.Text
	rescanBtn *widget.Button
	headBox   *fyne.Container

	// toast（顶部居中滑入的浅提示条）
	toastBox   *fyne.Container
	toastLbl   *canvas.Text
	toastMu    sync.Mutex
	toastTimer *time.Timer

	// 视图
	body      *fyne.Container
	views     map[viewID]fyne.CanvasObject
	overview  *overviewView
	adviceV   *adviceView
	largeV    *largeView
	excludeV  *excludeView
	scannerV  *scanningView
	cleaningV *cleaningView
	doneV     *doneView

	// 全局状态
	current viewID
	report  *model.Report
	// lastClean 记录最近一次清理结果，供「撤销本次清理」使用。
	lastClean CleanResult

	scanning bool
	paused   bool
	// tickerStop 用于停止进度轮询 goroutine。
	tickerStop chan struct{}
}

// newAppShell 构造外壳并装配全部视图。
func newAppShell(w fyne.Window, ctrl *Controller) *appShell {
	s := &appShell{
		win:     w,
		ctrl:    ctrl,
		navBtns: map[viewID]*navItem{},
		views:   map[viewID]fyne.CanvasObject{},
	}

	s.overview = newOverviewView(s)
	s.adviceV = newAdviceView(s)
	s.largeV = newLargeView(s)
	s.excludeV = newExcludeView(s)
	s.scannerV = newScanningView(s)
	s.cleaningV = newCleaningView(s)
	s.doneV = newDoneView(s)

	s.views[viewOverview] = s.overview.root
	s.views[viewAdvice] = s.adviceV.root
	s.views[viewLarge] = s.largeV.root
	s.views[viewExclude] = s.excludeV.root
	s.views[viewScanning] = s.scannerV.root
	s.views[viewCleaning] = s.cleaningV.root
	s.views[viewDone] = s.doneV.root

	s.body = container.NewStack(
		s.overview.root, s.adviceV.root, s.largeV.root, s.excludeV.root,
		s.scannerV.root, s.cleaningV.root, s.doneV.root,
	)

	head := s.buildHead()
	mainBox := container.NewBorder(head, nil, nil, nil, s.body)
	base := container.NewBorder(nil, nil, s.buildSidebar(), nil, mainBox)
	root := container.NewMax(base, s.buildToastLayer())
	w.SetContent(root)

	s.current = viewOverview
	s.switchView(viewOverview)
	s.refreshDiskChip()
	s.refreshNav()
	return s
}

// buildSidebar 组装侧边栏。
// 用绑定的 inner 布局，避免 navItem 重复构造行。
func (s *appShell) buildSidebar() fyne.CanvasObject {
	s.miniRing = newRingWidget(0.30, 38)
	s.diskName = makeText(s.volumeName(), 13, colFg, true, fyne.TextAlignLeading)
	s.diskSub = makeText("可用 —", 11.5, colFg2, false, fyne.TextAlignLeading)

	diskCard := makeCard(container.NewPadded(container.NewHBox(
		container.NewGridWrap(fyne.NewSize(38, 38), s.miniRing),
		container.NewVBox(s.diskName, s.diskSub),
	)))

	s.navBtns[viewOverview] = newNavItem("📊  概览", func() { s.switchView(viewOverview) })
	s.navBtns[viewAdvice] = newNavItem("🧹  清理建议", func() { s.switchView(viewAdvice) })
	s.navBtns[viewLarge] = newNavItem("🗂  大文件", func() { s.switchView(viewLarge) })
	s.navBtns[viewExclude] = newNavItem("🚫  排除列表", func() { s.switchView(viewExclude) })

	sidebar := container.NewVBox(
		container.NewPadded(diskCard),
		makeSectionLabel("空间管理"),
		s.navBtns[viewOverview].row(),
		s.navBtns[viewAdvice].row(),
		s.navBtns[viewLarge].row(),
		makeSectionLabel("安全"),
		s.navBtns[viewExclude].row(),
		container.NewPadded(makeSafetyFooter(s.ctrl)),
	)
	// 侧边栏底色固定为浅灰，与主区（白卡、浅蓝窗体）区分层级。
	return container.NewStack(canvas.NewRectangle(colSidebar), container.NewVScroll(sidebar))
}

// makeSectionLabel 构造侧边栏的分组标题。
func makeSectionLabel(txt string) fyne.CanvasObject {
	t := makeText(txt, 11, colFg3, true, fyne.TextAlignLeading)
	return container.NewPadded(container.NewHBox(t))
}

// makeSafetyFooter 构造侧边栏底部的安全承诺文案。
func makeSafetyFooter(ctrl *Controller) fyne.CanvasObject {
	lines := []string{
		"✔ 只读扫描 — 扫描不会修改任何文件",
		"✔ 废纸篓机制 — 清理内容移入废纸篓，随时可恢复",
		"✔ 二次确认 — 清理前列出完整清单",
	}
	if w := ctrl.ConfigWarning(); w != "" {
		lines = append(lines, "⚠ 配置文件读取失败："+w)
	}

	box := container.NewVBox()
	for _, ln := range lines {
		box.Add(makeText(ln, 10.5, colFg3, false, fyne.TextAlignLeading))
	}
	return box
}

// row 返回导航项的整行容器：底色矩形在底层，按钮+角标在内容层。
func (n *navItem) row() fyne.CanvasObject {
	return container.NewStack(n.bg, n.inner)
}

// newNavItem 创建一行导航项。
func newNavItem(label string, onTap func()) *navItem {
	btn := widget.NewButton(label, onTap)
	btn.Importance = widget.LowImportance
	bg := newRoundRect(color.NRGBA{A: 0}, radiusNavItem)

	// 角标：计数胶囊，默认隐藏（原型 .cnt：11px 字号 + bg rgba(0,0,0,.06) 药丸底）。
	// 胶囊贴合文字：Border 尾部取 minSize，内层用 Padded 给文字留 Geneva 四周空白。
	badgeTxt := makeText("", badgeTextSize, colFg2, true, fyne.TextAlignCenter)
	pill := makePill(colCnt, container.NewPadded(badgeTxt))
	pill.Hide()
	badgeWrap := pill

	inner := container.NewPadded(container.NewBorder(nil, nil, nil, badgeWrap, btn))
	return &navItem{btn: btn, badgeTxt: badgeTxt, badgeWrap: pill, bg: bg, inner: inner}
}

// setBadge 设置角标文案与可见性。
func (n *navItem) setBadge(count int) {
	if count <= 0 {
		n.badgeTxt.Text = ""
		n.badgeWrap.Hide()
	} else {
		n.badgeTxt.Text = fmt.Sprintf("%d", count)
		n.badgeWrap.Show()
	}
	n.badgeTxt.Refresh()
	n.badgeWrap.Refresh()
}

// buildHead 组装顶部标题栏。
// 标题用 RichText：文案变化时最小尺寸会自动重算，避免切换视图后被裁切。
func (s *appShell) buildHead() fyne.CanvasObject {
	s.headTitle = widget.NewRichText(
		&widget.TextSegment{Text: "磁盘概览", Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignLeading,
			ColorName: theme.ColorNameForeground,
			SizeName:  theme.SizeNameHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		}},
	)
	s.headSub = makeText("", 12.5, colFg2, false, fyne.TextAlignLeading) // 原型 .main-head .sub

	s.rescanBtn = widget.NewButton("🔄 重新扫描", func() { s.startScan() })
	s.rescanBtn.Importance = widget.LowImportance
	s.rescanBtn.Hide()

	left := container.NewVBox(s.headTitle, s.headSub)
	s.headBox = container.NewPadded(container.NewBorder(nil, nil, nil, s.rescanBtn, left))
	return s.headBox
}

// setHeadTitle 更新标题文案（RichText 单段）。
func (s *appShell) setHeadTitle(title string) {
	seg := s.headTitle.Segments[0].(*widget.TextSegment)
	seg.Text = title
	s.headTitle.Refresh()
}

// headTitleText 返回当前标题文案（测试与断言用）。
func (s *appShell) headTitleText() string {
	return s.headTitle.Segments[0].(*widget.TextSegment).Text
}

// buildToastLayer 组装覆盖在主区顶部的 toast 层（药丸造型，13px 文字）。
func (s *appShell) buildToastLayer() fyne.CanvasObject {
	s.toastLbl = makeText("", 13, colToastFg, false, fyne.TextAlignCenter)
	s.toastBox = makePill(colToastBg, container.NewPadded(s.toastLbl))
	s.toastBox.Hide()
	return container.NewBorder(container.NewPadded(container.NewCenter(s.toastBox)), nil, nil, nil)
}

// switchView 切换当前视图并更新标题与激活态。
func (s *appShell) switchView(id viewID) {
	s.current = id
	for vid, v := range s.views {
		if vid == id {
			v.Show()
		} else {
			v.Hide()
		}
	}
	s.body.Refresh()

	for vid, n := range s.navBtns {
		n.setActive(vid == id)
	}
	s.updateHead()
}

// updateHead 按当前视图刷新标题行。
func (s *appShell) updateHead() {
	title, sub := "", ""
	switch s.current {
	case viewOverview:
		title, sub = "磁盘概览", s.lastScanText()
	case viewAdvice:
		title, sub = "清理建议", "勾选要清理的项目 · 全部将移入废纸篓"
	case viewLarge:
		title, sub = "大文件", "体积超过阈值的单个文件 · 按大小排序"
	case viewExclude:
		title, sub = "排除列表", "以下规则在扫描时将被完全跳过 · 规则长期生效"
	case viewScanning:
		title, sub = "扫描中", "只读分析磁盘 · 可暂停或取消"
	case viewCleaning:
		title, sub = "正在安全清理…", "所有内容将移入废纸篓，未从磁盘删除"
	case viewDone:
		title, sub = "清理完成", "内容在废纸篓中 · 30 天后系统自动清除"
	}
	s.setHeadTitle(title)
	s.headSub.Text = sub
	s.headSub.Refresh()

	// 重新扫描按钮只在既有结果、且没有流程进行时出现。
	showRescan := s.report != nil && !s.scanning
	if showRescan {
		s.rescanBtn.Show()
	} else {
		s.rescanBtn.Hide()
	}
	s.headBox.Refresh()
}

// lastScanText 生成「上次扫描：…」文案，报告排除信息让用户知道规则生效。
func (s *appShell) lastScanText() string {
	if s.report == nil {
		return "尚未扫描 · 点击下方按钮开始"
	}
	scope := "全盘"
	if len(s.report.TargetPaths) > 0 {
		scope = strings.Join(s.report.TargetPaths, ", ")
	}
	msg := fmt.Sprintf("上次扫描 %s（%s）",
		s.report.ScannedAt.Format("15:04:05"), scope)
	if s.report.ExcludedCount > 0 {
		msg += fmt.Sprintf(" · 已排除 %d 项", s.report.ExcludedCount)
	}
	if len(s.report.Errors) > 0 {
		msg += fmt.Sprintf(" · %d 个扫描器出错", len(s.report.Errors))
	}
	return msg
}

// volumeName 返回家目录所在卷的展示名。
func (s *appShell) volumeName() string {
	return volumeLabel(s.ctrl.Home())
}

// refreshDiskChip 按真实磁盘用量刷新侧边栏迷你环与文字。
func (s *appShell) refreshDiskChip() {
	total, used, free, ok := diskUsage(s.ctrl.Home())
	if !ok || total <= 0 {
		s.miniRing.SetProgress(0, colRingIdle)
		s.diskSub.Text = "可用 —"
		return
	}
	s.miniRing.SetProgress(float32(used)/float32(total), colAccent)
	s.diskSub.Text = fmt.Sprintf("可用 %.1f GB / %.1f GB",
		float64(free)/(1<<30), float64(total)/(1<<30))
	s.diskSub.Refresh()
}

// refreshNav 刷新侧边栏角标：清理建议=分类数，大文件=文件数。
func (s *appShell) refreshNav() {
	if s.report != nil {
		groups := s.report.ByCategory()
		catCount := 0
		for _, g := range groups {
			if len(g.Items) > 0 {
				catCount++
			}
		}
		s.navBtns[viewAdvice].setBadge(catCount)
		s.navBtns[viewLarge].setBadge(len(groups[model.CategoryLargeFile].Items))
	} else {
		s.navBtns[viewAdvice].setBadge(0)
		s.navBtns[viewLarge].setBadge(0)
	}
	// 排除列表角标始终显示当前规则数。
	s.navBtns[viewExclude].setBadge(len(s.ctrl.ExcludeRules()))
}

// toast 显示全局提示条，2.6 秒后自动消失。
func (s *appShell) toast(msg string) {
	s.toastMu.Lock()
	defer s.toastMu.Unlock()

	s.toastLbl.Text = msg
	s.toastLbl.Refresh()
	s.toastBox.Show()
	if s.toastTimer != nil {
		s.toastTimer.Stop()
	}
	s.toastTimer = time.AfterFunc(2600*time.Millisecond, func() {
		fyne.Do(func() {
			s.toastMu.Lock()
			defer s.toastMu.Unlock()
			s.toastBox.Hide()
		})
	})
}

// refreshAllViews 用最近的报告刷新各内容视图。
func (s *appShell) refreshAllViews() {
	s.overview.refresh()
	s.adviceV.setReport(s.report)
	s.largeV.setReport(s.report)
	s.refreshNav()
}

// startScan 触发一次异步扫描。返回 false 表示已被忽略（例如重复点击）。
func (s *appShell) startScan() bool {
	if s.scanning {
		return false
	}
	s.scanning = true
	s.paused = false

	// 每次扫描新建门闩：Gate 的取消是终止性的，复用旧实例
	// 会让新扫描一开始就被判为已取消。
	s.ctrl.NewGate()

	s.scannerV.reset()
	s.switchView(viewScanning)
	s.updateHead()
	s.startTicker()

	go func() {
		report := s.ctrl.Scan()
		// 所有 UI 更新必须回到 Fyne 主线程。
		fyne.Do(func() { s.onScanDone(report) })
	}()
	return true
}

// onScanDone 扫描完成（或被取消）后的 UI 线程收尾。
func (s *appShell) onScanDone(r *model.Report) {
	s.stopTicker()
	s.scanning = false
	s.paused = false

	g := s.ctrl.CurrentGate()
	if g != nil && g.Canceled() {
		// 被取消：返回概览，保留上一次结果。
		s.toast("扫描已取消")
		s.switchView(viewOverview)
		return
	}

	s.report = r
	s.refreshAllViews()
	s.updateHead()

	if r != nil {
		extra := ""
		if r.ExcludedCount > 0 {
			extra = fmt.Sprintf(" · 已按排除列表跳过 %d 个目录", r.ExcludedCount)
		}
		s.toast(fmt.Sprintf("扫描完成，发现 %s 可清理空间（范围：%s）%s",
			model.HumanBytes(r.TotalBytes), s.scopeText(), extra))
		s.switchView(viewAdvice)
	}
}

// onTogglePause 在暂停与继续之间切换。
func (s *appShell) onTogglePause() {
	g := s.ctrl.CurrentGate()
	if g == nil {
		return
	}
	if g.Paused() {
		g.Resume()
		s.paused = false
	} else {
		g.Pause()
		s.paused = true
	}
	s.scannerV.refreshPauseLabel()
}

// onCancelScan 取消当前扫描。
func (s *appShell) onCancelScan() {
	g := s.ctrl.CurrentGate()
	if g == nil {
		return
	}
	// Cancel 同时会唤醒暂停中的等待者，无需先 Resume。
	g.Cancel()
	s.scannerV.refreshPauseLabel()
}

// startTicker 扫描期间周期性刷新进度视图。
func (s *appShell) startTicker() {
	stop := make(chan struct{})
	s.tickerStop = stop
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fyne.Do(func() {
					if s.scanning {
						s.scannerV.tick()
					}
				})
			}
		}
	}()
}

// stopTicker 停止进度轮询。重复停止安全。
func (s *appShell) stopTicker() {
	if s.tickerStop != nil {
		close(s.tickerStop)
		s.tickerStop = nil
	}
}

// scopeText 描述当前扫描范围，用于进度与 toast 文案。
func (s *appShell) scopeText() string {
	paths := s.ctrl.TargetPaths()
	if len(paths) == 0 {
		return "全盘"
	}
	return strings.Join(paths, "、")
}
