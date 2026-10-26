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

// riskBadge 返回带方块的等级标记，比纯文字更容易扫读。
func riskBadge(r model.Risk) string {
	switch r {
	case model.RiskLow:
		return "● 低"
	case model.RiskMedium:
		return "● 中"
	case model.RiskHigh:
		return "● 高"
	default:
		return "● 未知"
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
