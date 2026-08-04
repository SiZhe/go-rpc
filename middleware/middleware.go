package middleware

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// Handler 处理一次 RPC 调用。第一个参数是标准 context.Context,承载取消/超时信号
// 和 RPC 元信息(通过 rpccontext 包读写)。最内层 handler 负责真正发起网络请求。
//
// 【为什么第一个参数是 context.Context(Go 惯例)】
// Go 社区约定:凡是可能阻塞、可能需要取消/超时的函数,第一个参数都传 context.Context。
// 这样超时/取消信号能沿调用链一路透传到最底层的网络 IO。
type Handler func(ctx context.Context, req proto.Message) (proto.Message, error)

// Middleware 接收下一个 Handler,返回包装后的 Handler(高阶函数)。
type Middleware func(Handler) Handler

// Chain 把多个中间件按声明顺序组装成洋葱:
// Chain(m1, m2)(h) == m1(m2(h)),执行时 m1 最先进入、最后退出。
func Chain(ms ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(ms) - 1; i >= 0; i-- {
			next = ms[i](next)
		}
		return next
	}
}
