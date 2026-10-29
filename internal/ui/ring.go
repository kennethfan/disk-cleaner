package ui

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// ringSegment 是环形图上的一段扇带：从 from 到 to（0~1，以 12 点方向
// 顺时针计）的角度范围用 col 着色。
type ringSegment struct {
	from, to float32
	col      color.Color
}

// ringWidget 用像素级绘制实现环形图。
//
// Fyne 没有 SVG 弧线原语，用 canvas.Image + 自绘像素最稳：
// 每次 SetProgress/SetSegments 重新生成一张小图（直径最大 512px、
// 按设备像素缩放），开销可控且刷新语义明确。
type ringWidget struct {
	widget.BaseWidget

	segments []ringSegment
	// track 是未覆盖区域的底色。
	track color.Color
	// thickness 是圆环粗细占半径的比例（0.1~0.45）。
	thickness float32
	// minSize 是创建时给出的期望方边长。
	minSize fyne.Size

	// img 是已渲染的位图；置 nil 表示下次布局时重新生成。
	img *image.NRGBA
}

func newRingWidget(thickness float32, size float32) *ringWidget {
	r := &ringWidget{
		track:     color.NRGBA{R: 0xE5, G: 0xE5, B: 0xEA, A: 0xFF},
		thickness: thickness,
		minSize:   fyne.NewSize(size, size),
	}
	r.ExtendBaseWidget(r)
	return r
}

// SetProgress 显示单段进度环：从 12 点方向顺时针扫过 p 比例。
// p<=0 时只显示底色。
func (r *ringWidget) SetProgress(p float32, col color.Color) {
	if p <= 0 {
		r.segments = nil
	} else {
		if p > 1 {
			p = 1
		}
		r.segments = []ringSegment{{from: 0, to: p, col: col}}
	}
	r.refresh()
}

// SetSegments 显示按绝对比例（from/to 已是 0~1 的角度占比）分段的彩色环。
func (r *ringWidget) SetSegments(parts []ringSegment) {
	r.segments = append(r.segments[:0:0], parts...)
	r.refresh()
}

func (r *ringWidget) refresh() {
	r.img = nil
	r.Refresh()
}

// CreateRenderer 实现 fyne.Widget。
func (r *ringWidget) CreateRenderer() fyne.WidgetRenderer {
	img := &canvas.Image{FillMode: canvas.ImageFillContain, ScaleMode: canvas.ImageScaleSmooth}
	return &ringRenderer{r: r, img: img}
}

// render 把环形图画成一张 w×h 的位图。透明底：环外与环心均不覆盖。
func (r *ringWidget) render(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}

	cx := float64(w) / 2
	cy := float64(h) / 2
	outer := math.Min(cx, cy)
	inner := outer * (1 - float64(r.thickness))
	if inner >= outer {
		inner = outer * 0.5
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			rad := math.Hypot(dx, dy)
			if rad < inner || rad > outer {
				continue
			}

			// atan2 以 12 点方向为 0，顺时针递增，映射到 [0,1)。
			angle := math.Atan2(dx, -dy)
			if angle < 0 {
				angle += 2 * math.Pi
			}
			frac := angle / (2 * math.Pi)

			col := r.colorAt(frac)
			pr, pg, pb, pa := col.RGBA()
			off := (y*img.Rect.Dx() + x) * 4
			img.Pix[off+0] = uint8(pr >> 8)
			img.Pix[off+1] = uint8(pg >> 8)
			img.Pix[off+2] = uint8(pb >> 8)
			img.Pix[off+3] = uint8(pa >> 8)
		}
	}
	return img
}

// colorAt 返回某比例处的颜色；无覆盖段时返回底色。
func (r *ringWidget) colorAt(frac float64) color.Color {
	for _, s := range r.segments {
		if frac >= float64(s.from) && frac < float64(s.to) {
			return s.col
		}
	}
	return r.track
}

// regenerate 按需要重新生成位图（尺寸或内容变化时）。
func (r *ringWidget) regenerate() {
	// 用 2x 超采样生成位图，确保高分屏下也足够清晰。
	const scale = 2
	w := int(float32(r.Size().Width) * scale)
	h := int(float32(r.Size().Height) * scale)
	if w <= 0 {
		w = 180
	}
	if h <= 0 || h != w {
		h = w
	}
	if w > 512 { // 防御：异常尺寸下也不至于生成超大位图
		w, h = 512, 512
	}
	// 尺寸没变时复用旧图，避免无谓重绘。
	reuse := false
	if r.img != nil {
		got := r.img.Bounds()
		reuse = got.Dx() == w && got.Dy() == h
	}
	if !reuse {
		r.img = r.render(w, h)
	}
}

// ringRenderer 是 ringWidget 的渲染器。
type ringRenderer struct {
	r   *ringWidget
	img *canvas.Image
}

// sync 把 widget 上最新的位图喂给 canvas.Image（无图则跳过）。
func (cr *ringRenderer) sync() {
	if cr.r.img == nil {
		return
	}
	if cr.img.Image != cr.r.img {
		cr.img.Image = cr.r.img
	}
}

func (cr *ringRenderer) Layout(size fyne.Size) {
	cr.img.Resize(size)
	cr.r.regenerate()
	cr.sync()
	cr.img.Refresh()
}

func (cr *ringRenderer) MinSize() fyne.Size {
	return cr.r.minSize
}

func (cr *ringRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{cr.img}
}

func (cr *ringRenderer) Refresh() {
	cr.r.regenerate()
	cr.sync()
	cr.img.Refresh()
}

func (cr *ringRenderer) Destroy() {}
