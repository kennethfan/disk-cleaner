package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	"github.com/kennethfan/disk-cleaner/internal/clean"
	"github.com/kennethfan/disk-cleaner/internal/config"
	"github.com/kennethfan/disk-cleaner/internal/exclude"
	"github.com/kennethfan/disk-cleaner/internal/gate"
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

	// cfg 是持久化配置，cfgPath 是它的位置（用于界面上展示）。
	cfg     *config.Config
	cfgPath string
	// exclude 由 cfg.Exclude 编译而来。
	exclude *exclude.Matcher
	// targetPaths 非空时处于"指定目录扫描"模式。
	targetPaths []string
	// configWarning 记录配置读取或解析失败的原因，供界面提示。
	configWarning string

	// gate 是当前扫描的暂停门闩。每次扫描新建一个，
	// 避免上一次的取消状态影响下一次。
	gate *gate.Gate
}

// NewGate 为一次新扫描创建门闩并挂到 opts 上。
//
// 每次扫描都必须新建：Gate 的取消是终止性的，
// 复用旧 Gate 会导致新扫描一开始就被判定为已取消。
func (c *Controller) NewGate() *gate.Gate {
	g := gate.New()
	c.gate = g
	c.opts.Gate = g
	return g
}

// CurrentGate 返回当前门闩，可能为 nil。
func (c *Controller) CurrentGate() *gate.Gate { return c.gate }

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

	// 配置读取失败不应阻止程序启动：排除列表只是便利功能，
	// 但要在界面上明确告知用户，避免误以为规则已生效。
	cfg, cfgPath, cfgErr := config.Load()
	if cfg == nil {
		cfg = &config.Config{}
	}
	matcher, matcherErr := exclude.New(cfg.Exclude, home)
	if matcherErr != nil {
		matcher = &exclude.Matcher{}
	}

	opts := scan.DefaultOptions()
	opts.Exclude = matcher

	c := &Controller{
		home:    home,
		opts:    opts,
		g:       g,
		tr:      tr,
		eng:     eng,
		cfg:     cfg,
		cfgPath: cfgPath,
		exclude: matcher,
	}
	if cfgErr != nil {
		c.configWarning = cfgErr.Error()
	} else if matcherErr != nil {
		c.configWarning = matcherErr.Error()
	}
	return c, nil
}

// Home 返回用户主目录。
func (c *Controller) Home() string { return c.home }

// Scan 执行一次完整扫描。可在后台 goroutine 中调用。
func (c *Controller) Scan() *model.Report {
	return scan.ScanAll(context.Background(), scan.All(c.opts), c.opts)
}

// Children 列出某个目录的直接子项，供树形界面展开时按需调用。
//
// 这里同样做安全检查：只有通过 guard 的目录才允许列子项，
// 防止树节点被构造出白名单外的路径。
// 指定目录模式下放宽为"只读浏览"：用户显式选择的目录即便不在
// 白名单内也允许查看，但这些项标记为 ReadOnly，不可删除。
func (c *Controller) Children(dir string) ([]model.Item, error) {
	checkErr := c.g.Check(dir)
	if checkErr != nil {
		allowed := errors.Is(checkErr, guard.ErrRootItself) || c.inTargetMode(dir)
		if !allowed {
			return nil, checkErr
		}
	}

	items, err := scan.NewChildScannerWithExclude(c.exclude).Children(context.Background(), dir)
	if err != nil {
		return nil, err
	}

	// 指定目录模式下的子项一律只读。
	if len(c.targetPaths) > 0 {
		for i := range items {
			items[i].ReadOnly = true
			items[i].Note = "指定目录扫描：仅可查看"
		}
	}
	return items, nil
}

// inTargetMode 判断 dir 是否位于某个已指定的目标目录之下。
func (c *Controller) inTargetMode(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	abs = filepath.Clean(abs)

	for _, t := range c.targetPaths {
		root, err := filepath.Abs(expandTilde(t, c.home))
		if err != nil {
			continue
		}
		root = filepath.Clean(root)
		if abs == root || strings.HasPrefix(abs+string(filepath.Separator), root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// SetTargetPaths 切换扫描范围。传空切片恢复默认扫描。
func (c *Controller) SetTargetPaths(paths []string) {
	c.targetPaths = paths
	c.opts.Paths = paths
}

// TargetPaths 返回当前指定的目录。
func (c *Controller) TargetPaths() []string { return c.targetPaths }

// InTargetMode 报告当前是否处于指定目录模式。
func (c *Controller) InTargetMode() bool { return len(c.targetPaths) > 0 }

// ExcludeRules 返回当前生效的排除规则。
func (c *Controller) ExcludeRules() []string {
	if c.exclude == nil {
		return nil
	}
	return c.exclude.Rules()
}

// ConfigPath 返回配置文件路径，供界面展示。
func (c *Controller) ConfigPath() string { return c.cfgPath }

// ConfigWarning 返回配置加载失败的原因，为空表示正常。
func (c *Controller) ConfigWarning() string { return c.configWarning }

// AddExcludeRule 追加一条排除规则并持久化，随后重建匹配器。
func (c *Controller) AddExcludeRule(rule string) error {
	added, err := c.cfg.AddExclude(rule)
	if err != nil {
		return err
	}
	if !added {
		return fmt.Errorf("规则已存在: %s", rule)
	}
	if err := c.persistConfig(); err != nil {
		return err
	}
	return c.rebuildExclude()
}

// RemoveExcludeRule 删除一条排除规则并持久化。
func (c *Controller) RemoveExcludeRule(rule string) error {
	if !c.cfg.RemoveExclude(rule) {
		return fmt.Errorf("规则不存在: %s", rule)
	}
	if err := c.persistConfig(); err != nil {
		return err
	}
	return c.rebuildExclude()
}

// persistConfig 保存配置到磁盘。
func (c *Controller) persistConfig() error {
	if c.cfgPath == "" {
		p, err := config.Save(c.cfg)
		if err != nil {
			return err
		}
		c.cfgPath = p
		return nil
	}
	return config.SaveTo(c.cfgPath, c.cfg)
}

// rebuildExclude 用最新规则重建匹配器，并同步到扫描选项。
func (c *Controller) rebuildExclude() error {
	m, err := exclude.New(c.cfg.Exclude, c.home)
	if err != nil {
		return fmt.Errorf("排除规则无效: %w", err)
	}
	c.exclude = m
	c.opts.Exclude = m
	return nil
}

// Clean 执行清理。调用方必须已取得用户确认。
func (c *Controller) Clean(items []model.Item) CleanResult {
	res := c.eng.Run(items)
	return CleanResult{Outcomes: res.Outcomes, FreedBytes: res.FreedBytes}
}

// expandTilde 展开开头的 ~。
func expandTilde(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
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
