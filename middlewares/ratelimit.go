package middlewares

import (
	"context"
	"fmt"

	"go-rpc/middleware"
	"go-rpc/ratelimit"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// RateLimit 限流中间件:用令牌桶控制请求速率,拿不到令牌快速失败。
//
// 【放在链的哪一层】通常靠外(甚至链首),被限流的请求尽早拒,不浪费内层开销。
func RateLimit(tb *ratelimit.TokenBucket) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			if !tb.Allow() {
				return nil, fmt.Errorf("ratelimit: 请求被限流 %s.%s",
					rpccontext.Service(ctx), rpccontext.Method(ctx))
			}
			return next(ctx, req)
		}
	}
}
