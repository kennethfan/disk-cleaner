package webui

import (
	"embed"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// assets 是前端资源。index.html 与原型 HTML/CSS 保持视觉一致，
// 数据层由下面的绑定 Service 提供（引擎的真实结果）。
//
//go:embed assets
var assets embed.FS

// noCache 让所有资产响应携带 Cache-Control: no-store。
//
// wails 的资产服务器不发任何缓存校验头（无 ETag/Last-Modified），
// WKWebView 会启发式缓存模块脚本并跨启动复用，导致再生 bindings
// 后页面仍跑到旧接口（验过的现场：service.js 无 SetScope 导出而页面
// 已在调用它）。no-store 保证每次启动都拉取最新资源。
type noStoreHandler struct{ inner http.Handler }

func (h noStoreHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Cache-Control", "no-store")
	h.inner.ServeHTTP(rw, req)
}

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
			Handler: noStoreHandler{application.BundledAssetFileServer(assets)},
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
