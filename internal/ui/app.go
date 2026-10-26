package ui

import (
	"context"
	"fmt"
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/guard"
	"github.com/kennethfan/disk-cleaner/internal/model"
	"github.com/kennethfan/disk-cleaner/internal/scan"
	"github.com/kennethfan/disk-cleaner/internal/trash"
)

// CleanResult 是清理结果，对上层屏蔽 clean.Result 的细节。
type CleanResult struct {
	Outcomes   []clean.Outcome
	FreedBytes int64
}

// Succeeded 返回成功清理的数量。
func (r CleanResult) Succeeded() int {
	n := 0
	for _, o := range r.Outcomes {
		if o.Err == nil && !o.Skipped {
			n++
		}
	}
	return n
}

// Failed 返回失败的数量。
func (r CleanResult) Failed() int {
	n := 0
	for _, o := range r.Outcomes {
		if o.Err != nil {
			n++
		}
	}
	return n
}

// Controller 持有引擎并向外提供扫描/清理动作。
//
// 它与 CLI 使用完全相同的 scan 与 clean 引擎，
// 因此安全约束（guard 白名单、移入废纸篓）在两种形态下一致。
type Controller struct {
	home string
	opts scan.Options
	g    *guard.Guard
	tr   *trash.Trasher
	eng  *clean.Engine
}

// NewController 构造 Controller。任何一环失败都会返回错误，
// 避免界面在"无安全闸门"的状态下启动。
func NewController() (*Controller, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录: %w", err)
	}

	trashDir, err := trash.DefaultDir()
	if err != nil {
		return nil, err
	}
	tr, err := trash.New(trashDir)
	if err != nil {
		return nil, err
	}
	g, err := guard.New(clean.DefaultRoots(home), home)
	if err != nil {
		return nil, err
	}
	eng, err := clean.NewEngine(g, tr)
	if err != nil {
		return nil, err
	}

	opts := scan.DefaultOptions()
	return &Controller{home: home, opts: opts, g: g, tr: tr, eng: eng}, nil
}

// Home 返回用户主目录。
func (c *Controller) Home() string { return c.home }

// Scan 执行一次完整扫描。可在后台 goroutine 中调用。
func (c *Controller) Scan() *model.Report {
	return scan.ScanAll(context.Background(), scan.All(c.opts), c.opts)
}

// Clean 执行清理。调用方必须已取得用户确认。
func (c *Controller) Clean(items []model.Item) CleanResult {
	res := c.eng.Run(items)
	return CleanResult{Outcomes: res.Outcomes, FreedBytes: res.FreedBytes}
}

// Run 启动图形界面。
func Run() error {
	ctrl, err := NewController()
	if err != nil {
		return err
	}

	a := app.NewWithID("com.kennethfan.disk-cleaner")
	w := a.NewWindow("macOS 磁盘空间清理")
	w.Resize(fyne.NewSize(820, 560))
	w.CenterOnScreen()

	view := &mainView{win: w, ctrl: ctrl}
	w.SetContent(view.build())

	// 启动即扫描一次，省去用户手动点击。
	view.onRescan()

	w.ShowAndRun()
	return nil
}

// themeColorForeground 返回主题前景色，供 canvas.Text 使用。
func themeColorForeground() color.Color {
	return theme.ForegroundColor()
}
