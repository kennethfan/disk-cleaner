// Package gate 提供可暂停/恢复的扫描门闩。
//
// 与 context 取消的区别：
//   - 取消是终止性的，一旦触发不可恢复
//   - 暂停是可逆的，恢复后从原处继续，进度不丢失
//
// 用法：在每个遍历循环的检查点调用 Wait(ctx)。暂停期间各 goroutine
// 阻塞在此，CPU 占用降为 0；恢复时统一放行。
package gate

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// ErrCanceled 表示门闩已被显式取消。
var ErrCanceled = errors.New("扫描已取消")

// Gate 是并发安全的暂停门闩。零值可用（等价于始终放行）。
type Gate struct {
	mu       sync.Mutex
	cond     *sync.Cond
	paused   bool
	canceled bool

	// 进度计数：已完成 / 总数。用于界面上展示扫描进度。
	done  atomic.Int64
	total atomic.Int64
}

// New 创建一个放行状态的 Gate。
func New() *Gate {
	g := &Gate{}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// ensureCond 确保 cond 已初始化，使零值 Gate 也可用。
// 调用方必须持有 g.mu。
func (g *Gate) ensureCond() {
	if g.cond == nil {
		g.cond = sync.NewCond(&g.mu)
	}
}

// Pause 暂停。重复调用无副作用。
func (g *Gate) Pause() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.paused = true
	g.mu.Unlock()
}

// Resume 恢复。重复调用无副作用。
func (g *Gate) Resume() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.paused = false
	g.ensureCond()
	g.mu.Unlock()
	g.cond.Broadcast()
}

// Cancel 终止。所有等待中的 Wait 会返回 ErrCanceled，
// 后续 Wait 也不再阻塞。
func (g *Gate) Cancel() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.canceled = true
	g.paused = false
	g.ensureCond()
	g.mu.Unlock()
	g.cond.Broadcast()
}

// Paused 报告当前是否处于暂停状态。
func (g *Gate) Paused() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.paused
}

// Canceled 报告是否已被取消。
func (g *Gate) Canceled() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.canceled
}

// Wait 在暂停时阻塞，直到恢复、取消或 ctx 结束。
//
// 返回值：
//   - nil         可以继续
//   - ctx.Err()   ctx 被取消或超时
//   - ErrCanceled 调用了 Cancel
func (g *Gate) Wait(ctx context.Context) error {
	if g == nil {
		return nil
	}

	g.mu.Lock()
	// 快速路径：未暂停且未取消，直接放行，不创建任何 goroutine。
	if !g.paused && !g.canceled {
		g.mu.Unlock()
		return nil
	}
	if g.canceled {
		g.mu.Unlock()
		return ErrCanceled
	}
	g.ensureCond()

	// 慢路径：需要等待。sync.Cond 不感知 ctx，
	// 因此另起 goroutine 在 ctx 结束时广播唤醒。
	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		select {
		case <-ctx.Done():
			// 这里不能持有 g.mu：Broadcast 需要等待者重新竞争锁。
			g.cond.Broadcast()
		case <-stop:
		}
	}()

	for g.paused && !g.canceled {
		if err := ctx.Err(); err != nil {
			g.mu.Unlock()
			close(stop)
			<-finished
			return err
		}
		g.cond.Wait()
	}

	canceled := g.canceled
	g.mu.Unlock()

	// 无论走哪条路径都要通知监视者退出，避免 goroutine 泄漏。
	close(stop)
	<-finished

	if canceled {
		return ErrCanceled
	}
	return ctx.Err()
}

// SetTotal 设置本次扫描的扫描器总数，供进度展示使用。
func (g *Gate) SetTotal(n int) {
	if g == nil {
		return
	}
	g.total.Store(int64(n))
	g.done.Store(0)
}

// MarkDone 标记一个扫描器完成。
func (g *Gate) MarkDone() {
	if g == nil {
		return
	}
	g.done.Add(1)
}

// Progress 返回已完成数与总数。
func (g *Gate) Progress() (done, total int) {
	if g == nil {
		return 0, 0
	}
	return int(g.done.Load()), int(g.total.Load())
}
