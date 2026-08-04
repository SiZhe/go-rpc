package middlewares

import (
	"go-rpc/breaker"
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// Breaker 熔断中间件:把 breaker.Breaker 挂到调用链上。
//
// 【一次调用的流程】
//  1. 先问熔断器 Allow():不放行(熔断中)→ 直接返回 ErrOpen,快速失败,不打下游。
//  2. 放行 → 执行 next(真正调用)。
//  3. 把结果(成功/失败)Report 给熔断器,用于更新滑动窗口和状态。
//
// 【设计要点】熔断器实例由外部传入,因此可以做到"每个服务/方法一个熔断器"
// (不同下游的健康度互相独立),这也是生产做法。
func Breaker(b *breaker.Breaker) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			if !b.Allow() {
				return nil, breaker.ErrOpen // 快速失败
			}
			resp, err := next(c, req)
			b.Report(err == nil) // 上报结果
			return resp, err
		}
	}
}
