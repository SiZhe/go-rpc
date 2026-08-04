package middlewares

import (
	"context"

	"go-rpc/breaker"
	"go-rpc/middleware"
	"google.golang.org/protobuf/proto"
)

// Breaker 熔断中间件:把一个"全局"熔断器挂到调用链上(对整个服务粒度熔断)。
//
// 【与 per-node 熔断的区别】
// 本中间件是"服务级"熔断:不区分是哪个节点失败,整体错误率超阈值就熔断。
// 若要"按节点"熔断并配合负载均衡摘除故障节点,见 nodebreaker.go(修复4)。
// 两者可按需选用;通常 per-node 更精细。
func Breaker(b *breaker.Breaker) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			if !b.Allow() {
				return nil, breaker.ErrOpen // 快速失败,不打下游
			}
			resp, err := next(ctx, req)
			b.Report(err == nil)
			return resp, err
		}
	}
}
