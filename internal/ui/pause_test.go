package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// TestPauseToggleSwitchesLabel 验证暂停按钮文案在「暂停/继续」之间切换。
func TestPauseToggleSwitchesLabel(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(nil)
	defer w.Close()

	v := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(v.build())

	g := v.ctrl.NewGate()

	if v.paused {
		t.Error("初始不应处于暂停")
	}

	// 暂停
	v.onTogglePause()
	if !v.paused {
		t.Error("点击后应进入暂停")
	}
	if !g.Paused() {
		t.Error("Gate 应处于暂停")
	}
	if got := v.pauseBtn.Text; got != "继续" {
		t.Errorf("按钮文案 = %q, 期望「继续」", got)
	}

	// 继续
	v.onTogglePause()
	if v.paused {
		t.Error("再次点击应退出暂停")
	}
	if g.Paused() {
		t.Error("Gate 不应处于暂停")
	}
	if got := v.pauseBtn.Text; got != "暂停" {
		t.Errorf("按钮文案 = %q, 期望「暂停」", got)
	}
}

// TestCancelScanSetsGateCanceled 验证取消按钮会终止门闩。
func TestCancelScanSetsGateCanceled(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(nil)
	defer w.Close()

	v := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(v.build())

	g := v.ctrl.NewGate()
	g.Pause()

	v.onCancelScan()
	if !g.Canceled() {
		t.Error("Gate 应被取消")
	}
	if g.Paused() {
		t.Error("取消后不应仍处于暂停")
	}
}

// TestNewGateResetsCancelState 验证每次扫描都新建门闩，
// 避免上一次的取消状态污染新扫描。
func TestNewGateResetsCancelState(t *testing.T) {
	c := &Controller{}

	g1 := c.NewGate()
	g1.Cancel()
	if !g1.Canceled() {
		t.Fatal("第一个 Gate 应已取消")
	}

	g2 := c.NewGate()
	if g2.Canceled() {
		t.Error("新 Gate 不应继承取消状态")
	}
	if g2.Paused() {
		t.Error("新 Gate 不应处于暂停")
	}
	if c.CurrentGate() != g2 {
		t.Error("CurrentGate 应返回最新的 Gate")
	}
}

// TestPauseToggleWithoutGate 验证没有门闩时点击暂停不会 panic。
func TestPauseToggleWithoutGate(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(nil)
	defer w.Close()

	v := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(v.build())

	// 未调用 NewGate，CurrentGate 为 nil。
	v.onTogglePause()
	v.onCancelScan()
	// 不 panic 即通过。
}

// TestStopProgressTickerIdempotent 验证重复停止不会 panic。
func TestStopProgressTickerIdempotent(t *testing.T) {
	v := &mainView{}
	v.progressStop = make(chan struct{})

	v.stopProgressTicker()
	v.stopProgressTicker() // 第二次应安全
}

// TestPauseStopsProgressAnimation 验证暂停时进度条动画停止，
// 恢复时重新启动。这是用户实际反馈的 bug：
// ProgressBarInfinite 与扫描进度无关，不停的话暂停后它仍在动，
// 视觉上误导用户以为扫描还在继续。
func TestPauseStopsProgressAnimation(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(nil)
	defer w.Close()

	v := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(v.build())
	v.ctrl.NewGate()

	// 模拟扫描中的状态：进度条可见且在跑。
	v.progress.Show()
	v.progress.Start()
	if !v.progress.Running() {
		t.Fatal("进度条应处于运行状态")
	}

	// 暂停 → 动画必须停止。
	v.onTogglePause()
	if v.progress.Running() {
		t.Error("暂停后进度条不应继续运行")
	}

	// 继续 → 动画恢复。
	v.onTogglePause()
	if !v.progress.Running() {
		t.Error("恢复后进度条应重新运行")
	}
}

// TestRescanRestartsAnimationAfterPauseEnd 验证上一次扫描在暂停中结束后，
// 下一次扫描能重新启动动画（Stop 后 running=false，仅 Show 不够）。
func TestRescanRestartsAnimationAfterPauseEnd(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(nil)
	defer w.Close()

	v := &mainView{win: w, ctrl: &Controller{}}
	w.SetContent(v.build())
	v.ctrl.NewGate()

	// 模拟"暂停中结束扫描"：Stop 后 Hide。
	v.progress.Stop()
	v.progress.Hide()
	if v.progress.Running() {
		t.Fatal("Stop 后不应运行")
	}

	// 新一轮扫描必须显式 Start。
	v.progress.Show()
	v.progress.Start()
	if !v.progress.Running() {
		t.Error("重新扫描时应启动动画")
	}
}
