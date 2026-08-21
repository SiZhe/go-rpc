package middlewares

import (
	"context"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"go-rpc/trace"
	"google.golang.org/protobuf/proto"
)

// Trace 链路追踪中间件:确保每次调用都有 TraceID,并为本跳生成 SpanID,
// 写进 ctx 供下层透传。
//
// 【放最外层】生成的 TraceID/SpanID 能覆盖后续所有中间件(日志、指标)并透传给下游。
//
// 【TraceID:一整条链路不变】
//   - ctx 已有 TraceID(上游透传)→ 沿用,保证同一条链 ID 不变。
//   - 没有 → 生成新的(本服务是链路入口)。
//
// 【SpanID / ParentSpanID:每跳新生成,串出调用树】
// span 代表"一段操作"(如本服务调下游的这一跳)。每经过一次 Trace 中间件:
//   - 把 ctx 里"当前的 spanID"降级为本跳的 parentSpanID(它是上游那一跳的 span);
//   - 为本跳生成一个新的 spanID。
//
// 于是 A→B→C 链路上,每一跳都带 (traceID 不变, 新 spanID, parentSpanID=上游 spanID),
// 事后用 parent→span 的指向就能还原出调用树,而不只是把日志聚成一堆。
// 生成后 transport 会把 trace_id / span_id / parent_span_id 编码进 RpcHeader 发给下游。
func Trace() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			if rpccontext.TraceID(ctx) == "" {
				rpccontext.SetTraceID(ctx, trace.NewTraceID())
			}
			// 当前 ctx 里的 spanID(若有)是上游那一跳的 span,作为本跳的 parent。
			rpccontext.SetParentSpanID(ctx, rpccontext.SpanID(ctx))
			// 本跳生成新的 spanID。
			rpccontext.SetSpanID(ctx, trace.NewSpanID())
			return next(ctx, req)
		}
	}
}
