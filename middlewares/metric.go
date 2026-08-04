package middlewares

import (
	"context"
	"time"

	"go-rpc/metric"
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// Metric 指标中间件:统计每次调用的耗时与成败,按 服务.方法 维度写入 Collector。
func Metric(c *metric.Collector) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			key := rpccontext.Service(ctx) + "." + rpccontext.Method(ctx)
			c.Observe(key, time.Since(start), err != nil)
			return resp, err
		}
	}
}
