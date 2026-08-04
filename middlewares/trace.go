package middlewares

import (
	"context"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"go-rpc/trace"
	"google.golang.org/protobuf/proto"
)

// Trace 链路追踪中间件:确保每次调用都有 TraceID,写进 ctx 供下层透传。
//
// 【放最外层】生成的 TraceID 能覆盖后续所有中间件(日志、指标)并透传给下游。
//   - ctx 已有 TraceID(上游透传)→ 沿用,保证同一条链 ID 不变。
//   - 没有 → 生成新的(本服务是链路入口)。
// 生成后 transport 会把它编码进 RpcHeader.trace_id 发给 C++ 服务端。
func Trace() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			if rpccontext.TraceID(ctx) == "" {
				rpccontext.SetTraceID(ctx, trace.NewTraceID())
			}
			return next(ctx, req)
		}
	}
}
