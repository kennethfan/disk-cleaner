package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kennethfan/disk-cleaner/internal/gate"
	"github.com/kennethfan/disk-cleaner/internal/model"
)

// buildTree 在 root 下创建 depth 层、每层 width 个子目录，
// 用于让扫描持续足够长的时间以便观察暂停行为。
func buildTree(t *testing.T, root string, depth, width int) {
	t.Helper()
	if depth == 0 {
		if err := os.WriteFile(filepath.Join(root, "f.bin"), make([]byte, 1024), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for i := 0; i < width; i++ {
		d := filepath.Join(root, string(rune('a'+i)))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		buildTree(t, d, depth-1, width)
	}
}

// TestScanAllPausesAndResumes 验证暂停会真正阻塞扫描，恢复后能继续。
func TestScanAllPausesAndResumes(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	buildTree(t, cache, 4, 3)

	g := gate.New()
	opts := DefaultOptions()
	opts.Home = home
	opts.Concurrency = 4
	opts.Gate = g

	done := make(chan *model.Report, 1)
	go func() {
		done <- ScanAll(context.Background(), All(opts), opts)
	}()

	// 先暂停，再让扫描跑一会儿。
	g.Pause()
	time.Sleep(60 * time.Millisecond)

	select {
	case <-done:
		t.Fatal("暂停期间扫描不应完成")
	case <-time.After(120 * time.Millisecond):
		// 仍在阻塞，符合预期。
	}

	g.Resume()

	select {
	case r := <-done:
		if len(r.Items) == 0 {
			t.Error("恢复后应产出结果")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("恢复后扫描未完成")
	}
}

// TestScanAllCancelDuringPause 验证暂停中取消能立即终止扫描。
func TestScanAllCancelDuringPause(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	buildTree(t, cache, 4, 3)

	g := gate.New()
	g.Pause()

	opts := DefaultOptions()
	opts.Home = home
	opts.Concurrency = 4
	opts.Gate = g

	done := make(chan *model.Report, 1)
	go func() {
		done <- ScanAll(context.Background(), All(opts), opts)
	}()

	time.Sleep(60 * time.Millisecond)
	g.Cancel()

	select {
	case r := <-done:
		// 取消后应收集到错误，且不应崩溃。
		if len(r.Errors) == 0 {
			t.Error("取消后应记录错误")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后扫描未结束")
	}
}

// TestGateProgressTracking 验证进度计数随扫描器完成而递增。
func TestGateProgressTracking(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "Library", "Caches")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "x"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	g := gate.New()
	opts := DefaultOptions()
	opts.Home = home
	opts.Concurrency = 1
	opts.Gate = g

	scanners := All(opts)
	ScanAll(context.Background(), scanners, opts)

	done, total := g.Progress()
	if total != len(scanners) {
		t.Errorf("总数 = %d, 期望 %d", total, len(scanners))
	}
	if done != len(scanners) {
		t.Errorf("完成数 = %d, 期望 %d", done, len(scanners))
	}
}

// TestCheckpointWithoutGate 验证没有 Gate 时检查点不阻塞。
func TestCheckpointWithoutGate(t *testing.T) {
	if err := checkpoint(context.Background()); err != nil {
		t.Errorf("无 Gate 时应返回 nil, got %v", err)
	}
}

// TestCheckpointRespectsCanceledContext 验证 ctx 取消优先于门闩。
func TestCheckpointRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checkpoint(withGate(ctx, gate.New()))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("应返回 context.Canceled, got %v", err)
	}
}

// TestCheckpointBlocksWhenPaused 验证暂停时检查点会阻塞。
func TestCheckpointBlocksWhenPaused(t *testing.T) {
	g := gate.New()
	g.Pause()
	ctx := withGate(context.Background(), g)

	done := make(chan error, 1)
	go func() { done <- checkpoint(ctx) }()

	select {
	case <-done:
		t.Fatal("暂停时检查点不应返回")
	case <-time.After(80 * time.Millisecond):
	}

	g.Resume()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("恢复后应返回 nil, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("恢复后检查点未返回")
	}
}
