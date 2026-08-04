package middlewares

import (
	"time"

	"go-rpc/metric"
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// Metric 指标中间件:统计每次调用的耗时与成败,写入 Collector。
//
// 【逻辑】记录开始时间 → 执行 next → 计算耗时 → 按 服务.方法 维度 Observe。
// err != nil 记为一次错误。这样 Collector 里就能算出每个方法的 QPS/错误率/平均耗时。
func Metric(c *metric.Collector) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			c.Observe(ctx.Service+"."+ctx.Method, time.Since(start), err != nil)
			return resp, err
		}
	}
}
