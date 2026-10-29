package ui

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	controller "github.com/kennethfan/disk-cleaner/internal/controller"
)

// expandTilde 展开开头的 ~（树形界面与概览的目录输入用）。
func expandTilde(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// Controller / CleanResult 是向后兼容的别名：真正实现已迁移到
// internal/controller，避免 Fyne 壳与 WebView 壳因都得经由本包
// 引入 Fyne 而被捆绑在同一个二进制里（两者 macOS 目标文件中的
// Objective-C 同名类无法共同链接）。
type (
	Controller  = controller.Controller
	CleanResult = controller.CleanResult
)

// Run 启动图形界面。
func Run() error {
	ctrl, err := controller.NewController()
	if err != nil {
		return err
	}

	a := app.NewWithID("com.kennethfan.disk-cleaner")
	a.Settings().SetTheme(newAppTheme())

	w := a.NewWindow("净盘 CleanDisk")
	w.Resize(fyne.NewSize(1120, 740))
	w.CenterOnScreen()

	// 概览页默认给出「开始智能扫描」入口，用户点击后才扫描，
	// 与交互原型的初始状态一致；扫描范围从配置恢复。
	newAppShell(w, ctrl)

	w.ShowAndRun()
	return nil
}
