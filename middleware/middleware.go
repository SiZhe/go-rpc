package middleware

import (
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// Handler 处理一次 RPC 调用。最内层 handler 负责真正发起网络请求。
type Handler func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error)

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
