// Package ui 是 disk-cleaner 的 Fyne 图形界面。
//
// 设计约束：界面层不直接操作文件系统。扫描走 scan.ScanAll，
// 清理走 clean.Engine.Run，与 CLI 共用同一套引擎，
// 保证两种形态的安全语义完全一致。
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// 风险等级配色。选用较深的色值，保证在浅色与深色主题下都有足够对比度。
var (
	colorRiskLow    = color.NRGBA{R: 0x2E, G: 0x7D, B: 0x32, A: 0xFF} // 绿
	colorRiskMedium = color.NRGBA{R: 0xEF, G: 0x6C, B: 0x00, A: 0xFF} // 橙
	colorRiskHigh   = color.NRGBA{R: 0xC6, G: 0x28, B: 0x28, A: 0xFF} // 红
	colorMuted      = color.NRGBA{R: 0x75, G: 0x75, B: 0x75, A: 0xFF} // 次要文字
)

// 风险标记的固定尺寸。
//
// 必须用固定宽度而非依赖 canvas.Text 的 MinSize：
// 后者对中文字形宽度估算偏小，放在 Border 右槽会被压缩到
// 末尾字符被裁切（"● 高" 只显示成 "●"）。
const (
	riskBadgeWidth float32 = 72
	riskSizeHeight float32 = 32
)

// riskColor 返回风险等级对应的颜色。
func riskColor(r model.Risk) color.Color {
	switch r {
	case model.RiskLow:
		return colorRiskLow
	case model.RiskMedium:
		return colorRiskMedium
	case model.RiskHigh:
		return colorRiskHigh
	default:
		return colorMuted
	}
}

// riskBadge 返回风险等级的短标签。
//
// 只返回单字：胶囊本身已有底色区分等级，加圆点前缀反而在
// 窄胶囊里挤不下（实测会被裁切）。
func riskBadge(r model.Risk) string {
	switch r {
	case model.RiskLow:
		return "低"
	case model.RiskMedium:
		return "中"
	case model.RiskHigh:
		return "高"
	default:
		return "?"
	}
}

// categoryColor 返回分类在瓦片/分段环中的标识色。
// 与交互原型的分类色一一对应。
func categoryColor(c model.Category) color.Color {
	switch c {
	case model.CategoryDevCache:
		return color.NRGBA{R: 0xAF, G: 0x52, B: 0xDE, A: 0xFF} // 弥
	case model.CategoryUserCache:
		return color.NRGBA{R: 0x0A, G: 0x84, B: 0xFF, A: 0xFF} // 蓝
	case model.CategorySystemCache:
		return color.NRGBA{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF} // 灰
	case model.CategoryLargeFile:
		return color.NRGBA{R: 0x30, G: 0xB0, B: 0xC7, A: 0xFF} // 青
	case model.CategoryJunk:
		return color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF} // 橙
	case model.CategorySnapshot:
		return color.NRGBA{R: 0x5A, G: 0xC8, B: 0xFA, A: 0xFF} // 天蓝
	default:
		return colorMuted
	}
}

// recommendBadge 返回分类的属性标签：全部低风险 → 推荐，否则需确认。
func categoryTag(items []model.Item) (text string, col color.Color) {
	for _, it := range items {
		if it.Risk != model.RiskLow {
			return "需人工确认", colWarn
		}
	}
	return "推荐清理", colOK
}

// categoryDesc 返回分类的一句话描述，与原型文案一致。
func categoryDesc(c model.Category) string {
	switch c {
	case model.CategoryDevCache:
		return "Xcode DerivedData、npm / pip / Gradle 等本地缓存，可重新生成"
	case model.CategoryUserCache:
		return "各 App 的 Library/Caches 目录，重建后无影响"
	case model.CategorySystemCache:
		return "系统日志与诊断报告，通常可安全清理"
	case model.CategoryLargeFile:
		return "体积超过阈值的单个文件，请确认不再需要"
	case model.CategoryJunk:
		return "废纸篓与残留文件"
	case model.CategorySnapshot:
		return "APFS 本地快照"
	default:
		return ""
	}
}

// categoryIcon 返回类别对应的图标。
func categoryIcon(c model.Category) fyne.Resource {
	switch c {
	case model.CategoryUserCache:
		return theme.FolderIcon()
	case model.CategorySystemCache:
		return theme.ComputerIcon()
	case model.CategoryDevCache:
		return theme.FileApplicationIcon()
	case model.CategoryLargeFile:
		return theme.DocumentIcon()
	case model.CategoryJunk:
		return theme.DeleteIcon()
	case model.CategorySnapshot:
		return theme.HistoryIcon()
	default:
		return theme.FileIcon()
	}
}

// themeColorForeground 返回主题前景色，供 canvas.Text 使用。
func themeColorForeground() color.Color {
	return theme.ForegroundColor()
}

// themeColorMuted 返回次要文字色：降低透明度，形成层级。
func themeColorMuted() color.Color {
	fg := theme.ForegroundColor()
	r, g, b, _ := fg.RGBA()
	// RGBA() 返回 16 位分量，右移 8 位取高字节。
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0x99}
}
