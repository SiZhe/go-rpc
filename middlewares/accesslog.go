package middlewares

import (
	"context"
	"time"

	"go-rpc/log"
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// AccessLog 访问日志中间件:每次调用打一条日志,带 TraceID、方法、耗时、结果。
//
// 【全局联动的关键】日志里的 TraceID 与 trace 中间件、metrics、下游 C++ 服务端用的是
// 同一个。出问题时用它把"客户端日志 + 链路 + 指标 + 服务端日志"全部串起来定位。
func AccessLog(l *log.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			cost := time.Since(start)

			fields := map[string]string{
				"service": rpccontext.Service(ctx),
				"method":  rpccontext.Method(ctx),
				"cost":    cost.String(),
				"span":    rpccontext.SpanID(ctx),
			}
			if p := rpccontext.ParentSpanID(ctx); p != "" {
				fields["parent"] = p
			}
			traceID := rpccontext.TraceID(ctx)
			if err != nil {
				fields["error"] = err.Error()
				l.Errorc(traceID, "rpc call failed", fields)
			} else {
				l.Infoc(traceID, "rpc call ok", fields)
			}
			return resp, err
		}
	}
}
