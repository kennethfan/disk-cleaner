package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// overviewView 是概览页。
type overviewView struct {
	shell *appShell
	root  fyne.CanvasObject

	ring    *ringWidget
	ringBig *canvas.Text
	ringCap *canvas.Text

	diskTitle *canvas.Text
	diskSub   *canvas.Text
	barsBox   *fyne.Container

	scopeAllBtn    *widget.Button
	scopeCustomBtn *widget.Button
	customRow      *fyne.Container
	chipsBox       *fyne.Container
	dirInput       *widget.Entry

	scanBtn  *widget.Button
	scanNote *canvas.Text
	tilesBox *fyne.Container

	// customMode 为 true 时采用"指定目录"扫描范围。
	customMode bool
}

func newOverviewView(s *appShell) *overviewView {
	v := &overviewView{shell: s}

	// ---------- 环形图 ----------
	v.ring = newRingWidget(0.26, 176)
	v.ringBig = makeText("", 30, colFg, true, fyne.TextAlignCenter)
	v.ringCap = makeText("", 12, colFg2, false, fyne.TextAlignCenter)
	ringWrap := container.NewStack(
		container.NewGridWrap(fyne.NewSize(176, 176), v.ring),
		container.NewCenter(container.NewVBox(v.ringBig, v.ringCap)),
	)

	// ---------- 信息列 ----------
	v.diskTitle = makeText("", 16, colFg, true, fyne.TextAlignLeading)
	v.diskSub = makeText("", 12.5, colFg2, false, fyne.TextAlignLeading)
	v.barsBox = container.NewVBox()

	// ---------- 扫描范围 ----------
	v.scopeAllBtn = widget.NewButton("🌐 全盘扫描", func() { v.setScopeMode(false) })
	v.scopeCustomBtn = widget.NewButton("📁 指定目录", func() { v.setScopeMode(true) })
	v.chipsBox = container.NewHBox()
	v.dirInput = widget.NewEntry()
	v.dirInput.SetPlaceHolder("输入目录，如 ~/Downloads 或 ~/Library/Caches")
	addBtn := widget.NewButton("＋ 添加", func() { v.addScopeDir() })
	v.customRow = container.NewVBox(
		container.NewHBox(v.chipsBox, layout.NewSpacer()),
		container.NewBorder(nil, nil, nil, addBtn, v.dirInput),
	)
	// 按当前已保存的范围恢复模式。
	v.customMode = len(s.ctrl.TargetPaths()) > 0

	v.scanBtn = widget.NewButton("✨ 开始智能扫描", func() { s.startScan() })
	v.scanBtn.Importance = widget.HighImportance
	v.scanNote = makeText("只读扫描 · 不会改动任何文件 · 清理前有二次确认", 12, colFg2, false, fyne.TextAlignLeading)

	infoBox := container.NewVBox(
		v.diskTitle, v.diskSub,
		v.barsBox,
		widget.NewSeparator(),
		// 分段控件轨道：底部圆角 9px 轨道 + 激活白底按钮，模仿原型 .seg。
		container.NewStack(newRoundRect(colHover, radiusSeg),
			container.NewPadded(container.NewHBox(v.scopeAllBtn, v.scopeCustomBtn))),
		v.customRow,
		container.NewHBox(v.scanBtn, container.NewPadded(v.scanNote)),
	)

	hero := makeCard(container.NewPadded(container.NewBorder(nil, nil, ringWrap, nil, infoBox)))
	v.tilesBox = container.NewVBox() // tiles 由 refreshTiles 填充

	scroller := container.NewVScroll(container.NewVBox(
		container.NewPadded(hero),
		container.NewPadded(v.tilesBox),
	))
	v.root = container.NewStack(canvas.NewRectangle(colWindowBg), scroller)

	v.updateScope()
	v.refresh()
	return v
}

// refresh 用当前报告与磁盘状态刷新概览。
func (v *overviewView) refresh() {
	v.refreshDisk()
	v.refreshTiles()
	v.updateScope()
}

// refreshDisk 刷新环形图中心、标题与分类构成条。报告为 nil 表示从未扫描。
func (v *overviewView) refreshDisk() {
	total, used, free, ok := diskUsage(v.shell.ctrl.Home())
	if !ok || total <= 0 {
		v.diskTitle.Text = "磁盘状态不可用"
		v.ring.SetProgress(0, colRingIdle)
		return
	}
	usedRatio := float32(used) / float32(total)

	if v.shell.report != nil && len(v.shell.report.Items) > 0 {
		// 已扫描：环按"已用空间的分类构成"着色，剩余为灰。
		segs := v.categorySegments(usedRatio, used)
		v.ring.SetSegments(segs)
		v.barsBox.Objects = v.categoryBars()
	} else {
		v.ring.SetProgress(usedRatio, colRingIdle)
		v.barsBox.Objects = nil
	}
	v.barsBox.Refresh()

	v.diskTitle.Text = fmt.Sprintf("%s — %.1f GB", v.shell.volumeName(), float64(total)/(1<<30))
	v.diskSub.Text = fmt.Sprintf("已用 %.1f GB · 可用 %.1f GB（%.1f%%）",
		float64(used)/(1<<30), float64(free)/(1<<30), float64(free)*100/float64(total))

	v.ringBig.Text = fmt.Sprintf("%.1f%%", usedRatio*100)
	v.ringCap.Text = fmt.Sprintf("已使用 %.1f GB", float64(used)/(1<<30))

	v.diskTitle.Refresh()
	v.diskSub.Refresh()
	v.ringBig.Refresh()
	v.ringCap.Refresh()
}

// categorySegments 计算分类分段（占"已用空间"的比例），返回含灰色补段的完整环。
// usedRatio 是已用比例，used 是已用字节数。
func (v *overviewView) categorySegments(usedRatio float32, used int64) []ringSegment {
	groups := v.shell.report.ByCategory()
	var segs []ringSegment
	cum := float32(0)
	for _, cat := range scanSegmentOrder() {
		g := groups[cat]
		if len(g.Items) == 0 || used <= 0 {
			continue
		}
		frac := float32(float64(g.Bytes) / float64(used))
		if frac > usedRatio-cum {
			frac = usedRatio - cum
		}
		if frac <= 0 {
			continue
		}
		segs = append(segs, ringSegment{col: categoryColor(cat), from: cum, to: cum + frac})
		cum += frac
	}
	if usedRatio-cum > 0.001 {
		segs = append(segs, ringSegment{col: colRingIdle, from: cum, to: usedRatio})
	}
	return segs
}

// shellDiskTotalBytes 返回最近报告中分类合计大小。
func (v *overviewView) shellDiskTotalBytes() int64 {
	if v.shell.report == nil {
		return 0
	}
	return v.shell.report.TotalBytes
}

// categoryBars 构造分类构成条列表。
func (v *overviewView) categoryBars() []fyne.CanvasObject {
	groups := v.shell.report.ByCategory()
	cleanable := v.shell.report.TotalBytes
	if cleanable <= 0 {
		return nil
	}

	var bars []fyne.CanvasObject
	for _, cat := range scanSegmentOrder() {
		g := groups[cat]
		if len(g.Items) == 0 {
			continue
		}
		name := makeText(string(cat), 12.5, colFg2, false, fyne.TextAlignLeading)
		size := makeText(model.HumanBytes(g.Bytes), 12.5, colFg, true, fyne.TextAlignTrailing)
		pb := widget.NewProgressBar()
		pb.SetValue(float64(g.Bytes) / float64(cleanable))
		head := container.NewBorder(nil, nil, nil, size, name)
		bars = append(bars, container.NewVBox(head, pb))
	}
	return bars
}

// refreshTiles 重建分类瓦片。未扫描时给出开始扫描的占位提示。
func (v *overviewView) refreshTiles() {
	v.tilesBox.Objects = nil

	if v.shell.report == nil {
		hint := makeText("点击「开始智能扫描」以获取可清理项分析", 13, colFg3, false, fyne.TextAlignCenter)
		v.tilesBox.Add(container.NewCenter(container.NewPadded(hint)))
		v.tilesBox.Refresh()
		return
	}

	groups := v.shell.report.ByCategory()
	// 224x100：按原型 .cat-tile 的内边距与三行内容量高度估算，留足余量。
	grid := container.NewGridWrap(fyne.NewSize(224, 100))
	for _, cat := range scanSegmentOrder() {
		g := groups[cat]
		if len(g.Items) == 0 {
			continue
		}
		tile := v.newCategoryTile(cat, g)
		grid.Add(tile)
	}
	if len(grid.Objects) == 0 {
		hint := makeText("没有发现可清理的内容 · 磁盘很干净", 13, colFg3, false, fyne.TextAlignCenter)
		v.tilesBox.Add(container.NewCenter(container.NewPadded(hint)))
		v.tilesBox.Refresh()
		return
	}
	v.tilesBox.Add(grid)
	v.tilesBox.Refresh()
}

// newCategoryTile 构造单击可跳到清理建议页的分类瓦片。
func (v *overviewView) newCategoryTile(cat model.Category, g model.CategoryGroup) fyne.CanvasObject {
	swatch := makeColoredRect(categoryColor(cat), 9, 9)
	name := makeText(string(cat), 13.5, colFg, true, fyne.TextAlignLeading)

	tag, tagCol := categoryTag(g.Items)
	tagBg := colOKTint
	if tagCol == colWarn {
		tagBg = colWarnTint
	}
	badge := makeBadge(tag, tagCol, tagBg)

	value := makeText(model.HumanBytes(g.Bytes), 20, colFg, true, fyne.TextAlignLeading)
	count := makeText(fmt.Sprintf("%d 个项目", len(g.Items)), 11.5, colFg2, false, fyne.TextAlignLeading)

	content := makeCard(container.NewPadded(container.NewVBox(
		container.NewHBox(swatch, name, layout.NewSpacer(), badge),
		value,
		count,
	)))
	return newClickable(content, func() { v.shell.switchView(viewAdvice) })
}

// ==================== 扫描范围 ====================

// setScopeMode 切换扫描范围模式。切到全盘会清空已存目录并持久化；切到指定目录保留原选择。
func (v *overviewView) setScopeMode(custom bool) {
	if v.customMode == custom {
		v.updateScope()
		return
	}
	v.customMode = custom
	if !custom {
		v.setTargetPaths(nil)
	}
	v.updateScope()
}

// setTargetPaths 更新 Controller 的扫描范围（会持久化到配置）。
func (v *overviewView) setTargetPaths(paths []string) {
	v.shell.ctrl.SetTargetPaths(paths)
}

// updateScope 按模式刷新分段控件高亮、目录行可见性与扫描按钮状态。
func (v *overviewView) updateScope() {
	if v.customMode {
		v.scopeCustomBtn.Importance = widget.HighImportance
		v.scopeAllBtn.Importance = widget.LowImportance
		v.customRow.Show()
	} else {
		v.scopeAllBtn.Importance = widget.HighImportance
		v.scopeCustomBtn.Importance = widget.LowImportance
		v.customRow.Hide()
	}
	v.scopeAllBtn.Refresh()
	v.scopeCustomBtn.Refresh()
	v.refreshChips()

	// 守卫逻辑：指定目录模式下未添加任何目录时禁用扫描。
	blocked := v.customMode && len(v.shell.ctrl.TargetPaths()) == 0
	if blocked {
		v.scanBtn.Disable()
		v.scanBtn.SetText("⚠︎ 请先添加扫描目录")
	} else {
		v.scanBtn.Enable()
		v.scanBtn.SetText("✨ 开始智能扫描")
	}
	v.scanBtn.Refresh()
}

// refreshChips 重建目录 chip 列表。
func (v *overviewView) refreshChips() {
	v.chipsBox.Objects = nil
	paths := v.shell.ctrl.TargetPaths()
	if len(paths) == 0 {
		v.chipsBox.Objects = append(v.chipsBox.Objects,
			makeText("尚未添加目录，请先添加再扫描", 12, colWarn, true, fyne.TextAlignLeading))
	} else {
		for i, dir := range paths {
			v.chipsBox.Add(v.newChip(i, dir))
		}
	}
	v.chipsBox.Refresh()
}

// newChip 构造一个目录 chip：药丸底 + 路径 + × 移除（原型 .chip）。
func (v *overviewView) newChip(idx int, dir string) fyne.CanvasObject {
	lbl := makeText(dir, 12, colAccentDeep, true, fyne.TextAlignLeading)
	removeBtn := widget.NewButton("×", func() {
		paths := v.shell.ctrl.TargetPaths()
		kept := append(paths[:idx:idx], paths[idx+1:]...)
		v.setTargetPaths(kept)
		v.updateScope()
	})
	removeBtn.Importance = widget.LowImportance
	return makePill(colAccentTint, container.NewPadded(container.NewHBox(lbl, removeBtn)))
}

// addScopeDir 把输入框中的目录（支持 ~ 路径）加入扫描范围。
func (v *overviewView) addScopeDir() {
	raw := strings.TrimSpace(v.dirInput.Text)
	if raw == "" {
		v.shell.toast("请输入目录路径")
		return
	}
	dir := expandTilde(raw, v.shell.ctrl.Home())

	paths := v.shell.ctrl.TargetPaths()
	for _, existing := range paths {
		if existing == dir {
			v.shell.toast("该目录已在扫描范围内")
			return
		}
	}

	v.dirInput.SetText("")
	v.setTargetPaths(append(append([]string{}, paths...), dir))
	v.updateScope()
	v.shell.toast("已添加扫描目录 " + dir)
}
