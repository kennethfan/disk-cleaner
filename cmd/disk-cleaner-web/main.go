// Command disk-cleaner-web 是 WebView 壳（Wails v3）形态的净盘。
//
// 与 Fyne 图形界面必须是两个二进制：Wails 与 Fyne 的依赖各带一个
// 同名 Objective-C 类（MenuItem），无法链接进同一个可执行文件。
// 两者共用 internal/controller 与同一套 scan / clean 引擎。
package main

import (
	"fmt"
	"os"

	"github.com/kennethfan/disk-cleaner/internal/webui"
)

func main() {
	// 无参数即启动界面：这是双击 .app 的启动路径。
	if err := webui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
