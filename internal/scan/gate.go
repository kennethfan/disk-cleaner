package scan

import (
	"context"

	"github.com/kennethfan/disk-cleaner/internal/gate"
)

// gateCtxKey 是 Gate 在 context 中的键。
//
// 把 Gate 放进 ctx 而不是给每个扫描函数加参数，原因是：
// dirSize / Children / Walk 回调等函数签名已经很固定，
// 逐个加参数会污染一大片调用点，而 Gate 是贯穿整个扫描的
// 横切关注点，正好适合 ctx 承载。
type gateCtxKey struct{}

// withGate 把 Gate 放入 ctx。gate 为 nil 时原样返回。
func withGate(ctx context.Context, g *gate.Gate) context.Context {
	if g == nil {
		return ctx
	}
	return context.WithValue(ctx, gateCtxKey{}, g)
}

// gateFrom 从 ctx 取出 Gate，没有则返回 nil。
func gateFrom(ctx context.Context) *gate.Gate {
	if g, ok := ctx.Value(gateCtxKey{}).(*gate.Gate); ok {
		return g
	}
	return nil
}

// checkpoint 是遍历循环中的暂停检查点。
//
// 返回非 nil 时调用方应立即中止当前遍历并向上传递该错误。
// 无 Gate 时开销极小（一次 ctx.Value 查找）。
func checkpoint(ctx context.Context) error {
	// ctx 取消优先：已取消就不必再看门闩。
	if err := ctx.Err(); err != nil {
		return err
	}
	g := gateFrom(ctx)
	if g == nil {
		return nil
	}
	return g.Wait(ctx)
}
