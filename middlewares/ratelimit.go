package middlewares

import (
	"fmt"

	"go-rpc/middleware"
	"go-rpc/ratelimit"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// RateLimit 限流中间件:用令牌桶控制请求速率。
//
// 【放在链的哪一层】通常放在很靠外的位置(甚至链首),这样被限流的请求能尽早被拒,
// 不浪费后面中间件和网络的开销。拒绝时快速返回错误(fail-fast)。
//
// 【客户端限流 vs 服务端限流】
//   - 客户端限流(本实现):调用方自我约束,避免自己把下游打爆。
//   - 服务端限流:服务端保护自己不被任何调用方打爆。
// 两者互补;这里演示客户端侧。
func RateLimit(tb *ratelimit.TokenBucket) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			if !tb.Allow() {
				return nil, fmt.Errorf("ratelimit: 请求被限流 %s.%s", c.Service, c.Method)
			}
			return next(c, req)
		}
	}
}
