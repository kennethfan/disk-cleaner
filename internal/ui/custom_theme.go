package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// appTheme 是应用的自定义主题。
//
// 对照交互原型的视觉基调做两件事：
//   - 强制浅色变体：原型是浅色 macOS 原生风（#f5f5f7 窗体 / 系统蓝主色），
//     深色模式下配色令牌与卡片对比度都会偏离设计，故整体锁浅色。
//   - 主色改为系统蓝 #0a84ff，与原型的 --accent 一致。
//
// 其余（字号、间距、圆角）保持有利于可读性的定制。
type appTheme struct {
	base fyne.Theme
}

// newAppTheme 创建主题。base 传 theme.DefaultTheme()。
func newAppTheme() fyne.Theme {
	return &appTheme{base: theme.DefaultTheme()}
}

// Color 覆盖需要定制的颜色，其余交给默认主题（强制浅色变体）。
func (t *appTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return colAccent // 系统蓝
	case theme.ColorNameHyperlink:
		return colAccent
	case theme.ColorNameBackground:
		return colWindowBg // #f5f5f7 窗体底
	case theme.ColorNameForeground:
		return colFg
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x0A, G: 0x84, B: 0xFF, A: 0x66}
	case theme.ColorNameSelection:
		return colAccentTint
	case theme.ColorNameSeparator:
		return colLine
	case theme.ColorNameSuccess:
		return colOK
	case theme.ColorNameWarning:
		return colWarn
	case theme.ColorNameError:
		return colBad
	}
	// 透传其余颜色，但固定用浅色变体，避免深色主题下卡片底变黑。
	return t.base.Color(name, theme.VariantLight)
}

// Font 全部交给默认主题：中文字形需要系统字体，
// 自带字体会破坏中文渲染。
func (t *appTheme) Font(s fyne.TextStyle) fyne.Resource {
	return t.base.Font(s)
}

// Icon 全部交给默认主题。
func (t *appTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(n)
}

// Size 定制尺寸：加大间距与圆角，让界面有呼吸感。
func (t *appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		// 默认 4 太紧，内容贴边。
		return 6
	case theme.SizeNameInnerPadding:
		// 控件内部留白，影响行高与按钮高度。
		return 8
	case theme.SizeNameText:
		return 14
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameHeadingText:
		// 原型 .main-head h1 为 19px。
		return 19
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameLineSpacing:
		return 5
	case theme.SizeNameButtonRadius:
		// macOS 观感更圆润。
		return 8
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameScrollBarRadius:
		return 6
	case theme.SizeNameCardRadius:
		return 10
	case theme.SizeNameDialogRadius:
		// 原型弹窗圆角 14px。
		return 14
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return t.base.Size(n)
}
