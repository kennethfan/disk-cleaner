package webui

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// assets 是前端资源。index.html 与原型 HTML/CSS 保持视觉一致，
// 仅把 mock 数据层换成这里的绑定服务。
//
//go:embed assets
var assets embed.FS

// Run 启动 WebView 壳的图形界面。
func Run() error {
	svc, err := NewService()
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Services: []application.Service{
			// 绑定 Service 暴露给前端；Controller 本身不越出 Go 层。
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "净盘 CleanDisk",
		URL:       "/",
		Width:     1120,
		Height:    740,
		MinWidth:  960,
		MinHeight: 640,
	})

	return app.Run()
}
