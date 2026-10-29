package ui

import (
	"testing"

	"github.com/kennethfan/disk-cleaner/internal/gate"
)

// TestPauseToggleSwitchesLabel 验证暂停按钮文案在「暂停/继续」之间切换。
func TestPauseToggleSwitchesLabel(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	g := s.ctrl.NewGate()

	if s.paused {
		t.Error("初始不应处于暂停")
	}

	// 暂停
	s.onTogglePause()
	if !s.paused {
		t.Error("点击后应进入暂停")
	}
	if !g.Paused() {
		t.Error("Gate 应处于暂停")
	}
	if got := s.scannerV.pauseBtn.Text; got != "继续" {
		t.Errorf("按钮文案 = %q, 期望「继续」", got)
	}

	// 继续
	s.onTogglePause()
	if s.paused {
		t.Error("再次点击应退出暂停")
	}
	if g.Paused() {
		t.Error("Gate 不应处于暂停")
	}
	if got := s.scannerV.pauseBtn.Text; got != "暂停" {
		t.Errorf("按钮文案 = %q, 期望「暂停」", got)
	}
}

// TestCancelScanSetsGateCanceled 验证取消按钮会终止门闩。
func TestCancelScanSetsGateCanceled(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	g := s.ctrl.NewGate()
	g.Pause()

	s.onCancelScan()
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
	s, cleanup := newTestShell(t)
	defer cleanup()

	// 未调用 NewGate，CurrentGate 为 nil。
	s.onTogglePause()
	s.onCancelScan()
	// 不 panic 即通过。
}

// TestStopTickerIdempotent 验证重复停止不会 panic。
func TestStopTickerIdempotent(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	s.startTicker()
	s.stopTicker()
	s.stopTicker() // 第二次应安全
}

// TestScanProgressReflectsGate 验证扫描视图的百分比随 Gate 进度变化。
//
// 原先用无限循环动画，不反映真实进度。改为按扫描器完成比例驱动环形图。
func TestScanProgressReflectsGate(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	g := s.ctrl.NewGate()
	g.SetTotal(4)

	// 初始 0%。
	s.scannerV.tick()
	if got := s.scannerV.stat.Text; got != "0%" {
		t.Errorf("初始进度文案 = %q, 期望 0%%", got)
	}

	// 完成 2/4。
	g.MarkDone()
	g.MarkDone()
	s.scannerV.tick()
	if got := s.scannerV.stat.Text; got != "50%" {
		t.Errorf("进度文案 = %q, 期望 50%%", got)
	}

	// 全部完成。
	g.MarkDone()
	g.MarkDone()
	s.scannerV.tick()
	if got := s.scannerV.stat.Text; got != "100%" {
		t.Errorf("进度文案 = %q, 期望 100%%", got)
	}
}

// TestPauseDoesNotChangeProgressValue 验证暂停不影响进度数值，
// 只影响状态文案。
func TestPauseDoesNotChangeProgressValue(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	g := s.ctrl.NewGate()
	g.SetTotal(4)
	g.MarkDone()
	s.scannerV.tick()

	before := s.scannerV.stat.Text
	s.onTogglePause()
	after := s.scannerV.stat.Text

	if before != after {
		t.Errorf("暂停不应改变进度文案: %q -> %q", before, after)
	}
	if !g.Paused() {
		t.Error("应处于暂停状态")
	}
	// 提示行随暂停切换（正常由 tick 刷新，这里手动触发验证文案）。
	s.scannerV.refreshHint()
	if got := s.scannerV.hint.Text; got != "已暂停 · 点击「继续」恢复扫描" {
		t.Errorf("暂停提示 = %q", got)
	}
}

// TestCancelNoticeWhilePaused 验证取消会解除暂停（唤醒等待者）。
func TestCancelNoticeWhilePaused(t *testing.T) {
	s, cleanup := newTestShell(t)
	defer cleanup()

	g := s.ctrl.NewGate()
	g.Pause()
	s.paused = true

	s.onCancelScan()
	if g.Paused() {
		t.Error("取消应同时解除暂停")
	}

	var _ = gate.New // 引用包避免仅测试用导入被裁剪。
}
