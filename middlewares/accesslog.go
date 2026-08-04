package middlewares

import (
	"time"

	"go-rpc/log"
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// AccessLog 访问日志中间件:每次调用进出各打一条日志,带 TraceID、方法、耗时、结果。
//
// 【为什么带 TraceID(全局联动的关键)】
// 日志里的 TraceID 和 trace 中间件生成的是同一个,也和 metrics、下游 C++ 服务端用的
// 是同一个。于是一次请求出问题时,用这个 TraceID 就能把 "客户端日志 + 链路 + 指标 +
// 服务端日志" 全部串起来定位 —— 这就是可观测性三大支柱(Log/Trace/Metric)的联动。
//
// 【放在链的位置】通常放在较外层(trace 之后),这样能记录到整个内层链路的总耗时,
// 且此时 TraceID 已由 trace 中间件准备好。
func AccessLog(l *log.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			start := time.Now()
			resp, err := next(c, req)
			cost := time.Since(start)

			fields := map[string]string{
				"service": c.Service,
				"method":  c.Method,
				"cost":    cost.String(),
			}
			if err != nil {
				fields["error"] = err.Error()
				l.Errorc(c.TraceID, "rpc call failed", fields)
			} else {
				l.Infoc(c.TraceID, "rpc call ok", fields)
			}
			return resp, err
		}
	}
}
