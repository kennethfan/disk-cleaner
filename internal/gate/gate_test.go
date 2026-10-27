package gate

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWaitPassesWhenNotPaused(t *testing.T) {
	g := New()
	if err := g.Wait(context.Background()); err != nil {
		t.Errorf("未暂停时应立即放行, got %v", err)
	}
}

func TestPauseBlocksThenResume(t *testing.T) {
	g := New()
	g.Pause()

	done := make(chan error, 1)
	go func() {
		done <- g.Wait(context.Background())
	}()

	// 暂停期间应阻塞。
	select {
	case <-done:
		t.Fatal("暂停时 Wait 不应返回")
	case <-time.After(80 * time.Millisecond):
	}

	g.Resume()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("恢复后应返回 nil, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("恢复后 Wait 未返回")
	}
}

func TestCancelUnblocksWait(t *testing.T) {
	g := New()
	g.Pause()

	done := make(chan error, 1)
	go func() {
		done <- g.Wait(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)
	g.Cancel()

	select {
	case err := <-done:
		if !errors.Is(err, ErrCanceled) {
			t.Errorf("取消后应返回 ErrCanceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("取消后 Wait 未返回")
	}

	// 后续 Wait 也应立即返回 ErrCanceled。
	if err := g.Wait(context.Background()); !errors.Is(err, ErrCanceled) {
		t.Errorf("已取消的 Gate 应返回 ErrCanceled, got %v", err)
	}
}

func TestContextCancelUnblocksWait(t *testing.T) {
	g := New()
	g.Pause()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- g.Wait(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("ctx 取消应返回 context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ctx 取消后 Wait 未返回")
	}
}

// TestNoGoroutineLeak 验证反复暂停/恢复不会累积 goroutine。
func TestNoGoroutineLeak(t *testing.T) {
	g := New()

	for i := 0; i < 50; i++ {
		g.Pause()
		done := make(chan error, 1)
		go func() {
			done <- g.Wait(context.Background())
		}()
		time.Sleep(time.Millisecond)
		g.Resume()
		if err := <-done; err != nil {
			t.Fatalf("第 %d 轮: %v", i, err)
		}
	}
}

// TestConcurrentWaiters 验证多个等待者能被同一次 Resume 全部唤醒。
func TestConcurrentWaiters(t *testing.T) {
	g := New()
	g.Pause()

	const n = 20
	var wg sync.WaitGroup
	results := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = g.Wait(context.Background())
		}(i)
	}

	time.Sleep(80 * time.Millisecond)
	g.Resume()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("部分等待者未被唤醒")
	}

	for i, err := range results {
		if err != nil {
			t.Errorf("等待者 %d 返回错误: %v", i, err)
		}
	}
}

// TestZeroValueGate 验证零值 Gate 可用（始终放行）。
func TestZeroValueGate(t *testing.T) {
	var g Gate
	if err := g.Wait(context.Background()); err != nil {
		t.Errorf("零值 Gate 应放行, got %v", err)
	}
	if g.Paused() {
		t.Error("零值 Gate 不应处于暂停")
	}
}

// TestNilGate 验证 nil Gate 安全可用。
func TestNilGate(t *testing.T) {
	var g *Gate
	if err := g.Wait(context.Background()); err != nil {
		t.Errorf("nil Gate 应放行, got %v", err)
	}
	g.Pause()
	g.Resume()
	g.Cancel()
	if g.Paused() || g.Canceled() {
		t.Error("nil Gate 状态查询应返回 false")
	}
}

func TestProgress(t *testing.T) {
	g := New()
	g.SetTotal(6)

	if done, total := g.Progress(); done != 0 || total != 6 {
		t.Errorf("初始进度 = %d/%d, 期望 0/6", done, total)
	}

	g.MarkDone()
	g.MarkDone()
	if done, total := g.Progress(); done != 2 || total != 6 {
		t.Errorf("进度 = %d/%d, 期望 2/6", done, total)
	}

	// SetTotal 应重置计数。
	g.SetTotal(3)
	if done, total := g.Progress(); done != 0 || total != 3 {
		t.Errorf("重置后进度 = %d/%d, 期望 0/3", done, total)
	}
}
