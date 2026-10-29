package ui

import (
	"image/color"

	"github.com/kennethfan/disk-cleaner/internal/model"
)

// 设计令牌：取自交互原型的浅色 macOS 原生风配色。
// 界面强制浅色，保证与原型的视觉基调一致。
var (
	colWindowBg = color.NRGBA{R: 0xF5, G: 0xF5, B: 0xF7, A: 0xFF} // 窗体底
	colCardBg   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // 卡片底
	colSidebar  = color.NRGBA{R: 0xF2, G: 0xF2, B: 0xF7, A: 0xFF} // 侧边栏底
	colSubBg    = color.NRGBA{R: 0xF7, G: 0xF7, B: 0xF9, A: 0xFF} // 卡片内明细区底
	colTrackBg  = color.NRGBA{R: 0xE8, G: 0xE8, B: 0xED, A: 0xFF} // 进度条/环底色
	colRingIdle = color.NRGBA{R: 0xC7, G: 0xC7, B: 0xCC, A: 0xFF} // 未扫描环形图前段

	colFg  = color.NRGBA{R: 0x1D, G: 0x1D, B: 0x1F, A: 0xFF} // 主文字
	colFg2 = color.NRGBA{R: 0x6E, G: 0x6E, B: 0x73, A: 0xFF} // 次要文字
	colFg3 = color.NRGBA{R: 0xAE, G: 0xAE, B: 0xB2, A: 0xFF} // 辅助文字

	colAccent     = color.NRGBA{R: 0x0A, G: 0x84, B: 0xFF, A: 0xFF} // 系统蓝
	colAccentDeep = color.NRGBA{R: 0x00, G: 0x71, B: 0xE3, A: 0xFF}
	colAccentTint = color.NRGBA{R: 0x0A, G: 0x84, B: 0xFF, A: 0x1A} // 主色浅底
	colOK         = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF} // 绿：安全/完成
	colOKTint     = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0x24}
	colWarn       = color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF} // 橙：需人工确认
	colWarnTint   = color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0x24}
	colBad        = color.NRGBA{R: 0xFF, G: 0x3B, B: 0x30, A: 0xFF} // 红：危险操作
	colLine       = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x14} // 分隔线
	colHover      = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x09} // 悬停底
	colCnt        = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0F} // 导航计数胶囊底（原型 .cnt rgba(0,0,0,.06)）
	colCheckRing  = color.NRGBA{R: 0xC7, G: 0xC7, B: 0xCC, A: 0xFF} // 勾选框未选中描边 #c7c7cc
	colToastBg    = color.NRGBA{R: 0x1D, G: 0x1D, B: 0x1F, A: 0xEA} // toast 深底
	colToastFg    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // toast 文字
)

// 圆角令牌：与原型 CSS 的 border-radius 对应。
// radiusPill 超过任何边长的一半，绘制时自动退化为胶囊/圆形。
const (
	radiusCard    float32 = 12      // 卡片 --r-card
	radiusNavItem float32 = 8       // 侧边栏导航项
	radiusSeg     float32 = 9       // 全盘/指定目录分段控件轨道
	radiusSwatch  float32 = 3       // 分类色块
	radiusPill    float32 = 1 << 20 // 药丸（chip / 徽标 / toast）
	badgeTextSize float32 = 11      // 导航计数徽标字号（原型 .cnt 11px）
	tagTextSize   float32 = 10      // 卡片属性徽标字号（原型 var(--ok) 小药丸 10px）
)

// scanSegmentOrder 为分段环与概览图的分类展示顺序。
func scanSegmentOrder() []model.Category {
	return []model.Category{
		model.CategoryDevCache,
		model.CategoryUserCache,
		model.CategorySystemCache,
		model.CategoryLargeFile,
		model.CategoryJunk,
		model.CategorySnapshot,
	}
}
