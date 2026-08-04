// Package middlewares 汇集所有"挂在中间件链上"的具体治理与可观测能力。
//
// 【和 middleware 包的区别】
//   - middleware 包:只有链引擎(Handler/Middleware/Chain),是"骨架"。
//   - middlewares 包(本包):一个个具体中间件,是"挂件"。每个都是 middleware.Middleware,
//     可任意组合挂到链上。这正是 kratos / go-common 的"引擎 + 挂件"分离。
package middlewares

import (
	"context"
	"time"

	"go-rpc/middleware"
	"google.golang.org/protobuf/proto"
)

// Timeout 超时中间件:用标准 context.WithTimeout 给调用设置截止时间。
//
// 【为什么这次用 context.WithTimeout(修复了旧版缺陷)】
// 旧版用 "goroutine + select + time.After",超时后那个跑 next 的 goroutine 仍在后台
// 运行,直到它自己结束 —— 若 next 卡在网络读上,goroutine 会一直泄漏。
//
// 新版用 context.WithTimeout 派生一个带截止时间的 ctx 传给 next:
//   - 到点后 ctx 内部的定时器会 close 掉 ctx.Done()。
//   - transport 层监听 ctx(SetDeadline / watcher),会立即中止正在进行的网络 IO 并返回。
// 于是超时能真正"打断"底层操作,不泄漏 goroutine。这就是 context 取消传播的价值。
//
// 【cancel 必须调用】WithTimeout 返回的 cancel 用 defer 调用,释放定时器资源,
// 否则即使提前返回也会泄漏,直到超时才回收。
func Timeout(d time.Duration) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			return next(ctx, req)
		}
	}
}
