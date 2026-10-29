package ui

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// makeText 构造自定义字号/颜色的 canvas.Text。
func makeText(txt string, size float32, col color.Color, bold bool, align fyne.TextAlign) *canvas.Text {
	t := canvas.NewText(txt, col)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	t.Alignment = align
	return t
}

// ==================== 圆角底块 ====================

// roundRect 是带圆角（或胶囊/圆形）造型的底色块。
//
// Fyne 的 canvas.Rectangle 没有 CSS 那样的 border-radius，与交互原型
// 的卡片样式（--r-card:12px / 药丸 99px）对不上。这里用 2x 超采样位图
// 绘制（含 1px 抗锯齿），尺寸或颜色变化时重新生成；
// radius >= 边长一半时自然退化为胶囊/圆形。
type roundRect struct {
	widget.BaseWidget

	col    color.Color
	radius float32

	// 可选描边（原型卡片的 border:1px solid var(--line)），nil 表示无边框。
	borderCol   color.Color
	borderWidth float32 // 描边价，位图单位（2x 超采样下半价=1 即 1 逻辑px）

	// img 为已生成位图；尺寸与颜色都没变时复用，避免无谓重绘。
	img *image.NRGBA
}

func newRoundRect(col color.Color, radius float32) *roundRect {
	r := &roundRect{col: col, radius: radius}
	r.ExtendBaseWidget(r)
	return r
}

// setColor 更换底色并触发重绘（清空位图缓存，避免复用旧颜色）。
func (r *roundRect) setColor(c color.Color) {
	r.col = c
	r.img = nil
	r.Refresh()
}

// CreateRenderer 实现 fyne.Widget。
func (r *roundRect) CreateRenderer() fyne.WidgetRenderer {
	img := &canvas.Image{FillMode: canvas.ImageFillStretch, ScaleMode: canvas.ImageScaleSmooth}
	return &roundRectRenderer{r: r, img: img}
}

// render 生成带圆角（含可选描边）的位图。透明底：圆角外不填充。
func (r *roundRect) render(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}
	hr, hg, hb, ha := r.col.RGBA()
	if ha>>8 == 0 && r.borderCol == nil {
		return img // 全透明：无需绘制
	}
	// 2x 超采样，尺寸、半径与描边价均按位图像素换算。
	fw, fh := float64(w), float64(h)
	rad := float64(r.radius) * 2
	var halfStroke float64
	if r.borderCol != nil {
		halfStroke = float64(r.borderWidth) * 2 / 2
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := roundedSDF(float64(x)+0.5, float64(y)+0.5, fw, fh, rad)

			// 先填底色（仅描边内侧），再叠加描边。
			var pr, pg, pb, pa uint8
			fillCov := clamp01(0.5 - d)
			if ha>>8 != 0 && fillCov > 0 {
				pr = uint8(float64(hr>>8) * fillCov)
				pg = uint8(float64(hg>>8) * fillCov)
				pb = uint8(float64(hb>>8) * fillCov)
				pa = uint8(float64(ha>>8) * fillCov)
			}
			if r.borderCol != nil {
				strokeCov := clamp01(halfStroke + 0.5 - math.Abs(d))
				if strokeCov > 0 {
					br, bg2, bb, _ := r.borderCol.RGBA()
					// 描边覆盖填充（over 合成）。
					outA := strokeCov + float64(pa)/255*(1-strokeCov)
					if outA > 0 {
						sr := float64(br>>8) * strokeCov
						sg := float64(bg2>>8) * strokeCov
						sb := float64(bb>>8) * strokeCov
						pr = uint8((sr + float64(pr)*(1-strokeCov)) / outA)
						pg = uint8((sg + float64(pg)*(1-strokeCov)) / outA)
						pb = uint8((sb + float64(pb)*(1-strokeCov)) / outA)
						pa = uint8(outA * 255)
					}
				}
			}
			if pa == 0 {
				continue
			}
			off := (y*img.Rect.Dx() + x) * 4
			img.Pix[off+0] = pr
			img.Pix[off+1] = pg
			img.Pix[off+2] = pb
			img.Pix[off+3] = pa
		}
	}
	return img
}

// clamp01 把值限制到 0..1。
func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// roundedSDF 计算圆角矩形的有符号距离（单位：位图像素）：
// 内部为负、圆角外为正。
func roundedSDF(px, py, w, h, radius float64) float64 {
	hw, hh := w/2, h/2
	rad := math.Min(radius, math.Min(hw, hh))
	qx := math.Abs(px-hw) - (hw - rad)
	qy := math.Abs(py-hh) - (hh - rad)
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) - rad
}

// regenerate 按当前尺寸生成位图（尺寸没变则复用旧图）。
func (r *roundRect) regenerate() {
	const scale = 2
	s := r.Size()
	w := int(s.Width * scale)
	h := int(s.Height * scale)
	needRebuild := r.img == nil || r.img.Bounds().Dx() != w || r.img.Bounds().Dy() != h
	if needRebuild {
		r.img = r.render(w, h)
	}
}

// roundRectRenderer 是 roundRect 的渲染器。
type roundRectRenderer struct {
	r   *roundRect
	img *canvas.Image
}

func (cr *roundRectRenderer) Layout(size fyne.Size) {
	cr.img.Resize(size)
	cr.r.regenerate()
	if cr.r.img != nil {
		cr.img.Image = cr.r.img
	}
	cr.img.Refresh()
}

func (cr *roundRectRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (cr *roundRectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{cr.img}
}

func (cr *roundRectRenderer) Refresh() {
	cr.r.regenerate()
	if cr.r.img != nil {
		cr.img.Image = cr.r.img
	}
	cr.img.Refresh()
}

func (cr *roundRectRenderer) Destroy() {}

// ==================== 组装小件 ====================

// makeCard 构造白底圆角卡片容器（原型 --r-card: 12px + 1px 浅描边，
// 描边让圆角在浅色窗体上可辨识）。
func makeCard(content fyne.CanvasObject) *fyne.Container {
	bg := newRoundRect(colCardBg, radiusCard)
	bg.borderCol = colLine
	bg.borderWidth = 1 // 2x 位图下半价 = 1 逻辑 px
	return container.NewStack(bg, content)
}

// makeColoredRect 构造小色块（分类标识，原型 swatch 圆角 3px）。
func makeColoredRect(col color.Color, w, h float32) fyne.CanvasObject {
	r := newRoundRect(col, radiusSwatch)
	return container.NewGridWrap(fyne.NewSize(w, h), r)
}

// makeBadge 构造属性标签胶囊（如「推荐清理」「需人工确认」）。
// 原型小药丸：10px 加粗文字、胶囊底（padding 1px 7px）。
func makeBadge(text string, col, tint color.Color) fyne.CanvasObject {
	bg := newRoundRect(tint, radiusPill)
	lbl := canvas.NewText(text, col)
	lbl.TextSize = tagTextSize
	lbl.TextStyle = fyne.TextStyle{Bold: true} // 原型 font-weight:700
	return container.NewStack(bg,
		container.NewPadded(container.NewCenter(lbl)))
}

// makePill 任意外包一层药丸底色（chip、导航徽标、toast 共用）。
func makePill(col color.Color, content fyne.CanvasObject) *fyne.Container {
	return container.NewStack(newRoundRect(col, radiusPill), content)
}

// ==================== 圆形勾选框 ====================

// circleCheck 圆形勾选框，对照原型 .cat-check：
// 20px 圆、未选中 1.5px #c7c7cc 描边，选中主色填充 + 白色对勾。
// 分类卡片头部用它替代 Fyne 自带的方形 Check，与原型一致。
type circleCheck struct {
	widget.BaseWidget

	bg        *roundRect
	tick      *canvas.Text
	checked   bool
	partial   bool
	OnChanged func(bool)
}

func newCircleCheck(onChanged func(bool)) *circleCheck {
	c := &circleCheck{
		bg:        newRoundRect(color.NRGBA{A: 0}, radiusPill),
		tick:      canvas.NewText("✓", color.NRGBA{A: 0}),
		OnChanged: onChanged,
	}
	c.bg.borderCol = colCheckRing
	c.bg.borderWidth = 1.5 // 2x 位图下半价 = 1.5 逻辑px，同原型 border:1.5px
	c.tick.TextSize = 12
	c.tick.TextStyle = fyne.TextStyle{Bold: true}
	c.ExtendBaseWidget(c)
	return c
}

// Tapped 点击切换选中态并回调。手动点击时退出三态。
func (c *circleCheck) Tapped(*fyne.PointEvent) {
	c.partial = false
	c.SetChecked(!c.checked)
	if c.OnChanged != nil {
		c.OnChanged(c.checked)
	}
}

// SetChecked 设置选中态（不触发 OnChanged，与 widget.Check 一致）。
// 由代码写入时清除三态；SetPartial 可在之后重新标定。
func (c *circleCheck) SetChecked(on bool) {
	if c.checked == on {
		return
	}
	c.checked = on
	c.partial = false
	c.applyStyle()
	c.Refresh()
}

// applyStyle 按选中态切换底色与对勾可见性。
// 三态：部分勾选时主色填充 + 白色短横（macOS 混选惯例）。
func (c *circleCheck) applyStyle() {
	switch {
	case c.checked || c.partial:
		c.bg.setColor(colAccent)
		c.tick.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
		if c.partial {
			c.tick.Text = "−"
		} else {
			c.tick.Text = "✓"
		}
	default:
		c.bg.setColor(color.NRGBA{A: 0})
		c.tick.Color = color.NRGBA{A: 0}
	}
}

// SetPartial 设置三态（部分勾选），显示短横而非对勾。
func (c *circleCheck) SetPartial(on bool) {
	if c.partial == on {
		return
	}
	c.partial = on
	c.applyStyle()
	c.Refresh()
}

// Checked 返回当前选中态。
func (c *circleCheck) Checked() bool { return c.checked }

// CreateRenderer 实现 fyne.Widget。
func (c *circleCheck) CreateRenderer() fyne.WidgetRenderer {
	return &simpleRenderer{inner: container.NewStack(
		c.bg,
		container.NewCenter(c.tick),
	)}
}

// MinSize 固定为原型的 20px 圆。
func (c *circleCheck) MinSize() fyne.Size {
	return fyne.NewSize(20, 20)
}

// ==================== 可点击包装 ====================

// clickableBox 让任意内容整体可点，用于分类瓦片等卡片的点击跳转。
type clickableBox struct {
	widget.BaseWidget
	inner fyne.CanvasObject
	onTap func()
}

func newClickable(content fyne.CanvasObject, onTap func()) *clickableBox {
	c := &clickableBox{inner: content, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *clickableBox) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *clickableBox) CreateRenderer() fyne.WidgetRenderer {
	return &simpleRenderer{inner: c.inner}
}

// simpleRenderer 是只包装单个子对象的渲染器。
type simpleRenderer struct {
	inner fyne.CanvasObject
}

func (r *simpleRenderer) Layout(size fyne.Size) {
	r.inner.Resize(size)
}

func (r *simpleRenderer) MinSize() fyne.Size {
	return r.inner.MinSize()
}

func (r *simpleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.inner}
}

func (r *simpleRenderer) Refresh() {
	r.inner.Refresh()
}

func (r *simpleRenderer) Destroy() {}
