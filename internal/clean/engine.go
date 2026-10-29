// Package clean 把 guard 与 trash 组合成可执行的清理动作。
//
// 这是唯一真正改动文件系统的包。所有删除都必须经过 Guard.Check。
package clean

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kennethfan/disk-cleaner/internal/guard"
	"github.com/kennethfan/disk-cleaner/internal/model"
	"github.com/kennethfan/disk-cleaner/internal/trash"
)

// Outcome 是单个清理项的处理结果。
type Outcome struct {
	Item model.Item
	// Dest 是移入废纸篓后的落点；永久删除时为空。
	Dest string
	Err  error
	// Skipped 为 true 表示被 guard 拒绝，未做任何改动。
	Skipped bool
}

// Result 是一次批量清理的汇总。
type Result struct {
	Outcomes []Outcome
	// FreedBytes 是成功清理项的大小总和。
	FreedBytes int64
}

// Succeeded 返回成功的数量。
func (r Result) Succeeded() int {
	n := 0
	for _, o := range r.Outcomes {
		if o.Err == nil && !o.Skipped {
			n++
		}
	}
	return n
}

// Failed 返回失败的数量。
func (r Result) Failed() int {
	n := 0
	for _, o := range r.Outcomes {
		if o.Err != nil {
			n++
		}
	}
	return n
}

// Engine 执行清理动作。
type Engine struct {
	guard   *guard.Guard
	trasher *trash.Trasher
}

// NewEngine 创建清理引擎。
func NewEngine(g *guard.Guard, t *trash.Trasher) (*Engine, error) {
	if g == nil {
		return nil, errors.New("guard 不能为空")
	}
	if t == nil {
		return nil, errors.New("trasher 不能为空")
	}
	return &Engine{guard: g, trasher: t}, nil
}

// Run 逐个处理 items。遇到被拒绝的项只标记 Skipped，不中断整批。
//
// 注意：调用方必须先取得用户明确确认。本方法不做任何交互。
func (e *Engine) Run(items []model.Item) Result {
	var res Result
	for _, it := range items {
		out := Outcome{Item: it}

		// 第一道闸门：安全检查。
		if err := e.guard.Check(it.Path); err != nil {
			out.Skipped = true
			out.Err = err
			res.Outcomes = append(res.Outcomes, out)
			continue
		}

		// 第二道闸门：永久删除类项目需要调用方额外确认。
		// 这里只处理"移入废纸篓"路径，Permanent 项直接拒绝，
		// 强制由专用流程（如废纸篓清空、快照删除）处理。
		if it.Permanent {
			out.Skipped = true
			out.Err = fmt.Errorf("该项标记为永久删除，需走专用流程: %s", it.Path)
			res.Outcomes = append(res.Outcomes, out)
			continue
		}

		dest, err := e.trasher.Move(it.Path)
		if err != nil {
			out.Err = err
			res.Outcomes = append(res.Outcomes, out)
			continue
		}
		out.Dest = dest
		res.FreedBytes += it.Size
		res.Outcomes = append(res.Outcomes, out)
	}
	return res
}

// Restore 把「移入废纸篓」的内容放回原位置。
//
// Move 与恢复是同一卷上的 rename，通常都成功；
// 若原路径父目录已不存在则补建。只处理成功移入的项，
// 被拒绝或失败的项目本来就没有移动，无需恢复。
func (e *Engine) Restore(outcomes []Outcome) (restored, failed int) {
	for _, o := range outcomes {
		if o.Err != nil || o.Skipped || o.Dest == "" {
			continue
		}
		if err := restorePath(o.Dest, o.Item.Path); err != nil {
			failed++
			continue
		}
		restored++
	}
	return restored, failed
}

// restorePath 把废弃篊内的落点放回原路径。
func restorePath(dest, orig string) error {
	if dir := filepath.Dir(orig); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("还原父目录 %s: %w", dir, err)
		}
	}
	if err := os.Rename(dest, orig); err != nil {
		return fmt.Errorf("找回 %s: %w", dest, err)
	}
	return nil
}

// DefaultRoots 返回默认允许清理的根目录白名单。
// 只有这些目录下的内容才可能被 guard 放行。
func DefaultRoots(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Library"),
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".yarn"),
		filepath.Join(home, ".pnpm-store"),
		filepath.Join(home, ".bun"),
		filepath.Join(home, ".gradle"),
		filepath.Join(home, ".m2"),
		filepath.Join(home, ".cargo"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, "go"),
		filepath.Join(home, ".Trash"),
		filepath.Join(home, "Downloads"),
	}
}
