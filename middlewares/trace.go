package middlewares

import (
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"go-rpc/trace"
	"google.golang.org/protobuf/proto"
)

// Trace 链路追踪中间件:确保每次调用都有 TraceID,并放进 ctx 供下层透传。
//
// 【放在链的最外层】这样它生成的 TraceID 能覆盖后续所有中间件(日志、metrics 都能拿到),
// 也能透传给下游。
//
// 【逻辑】
//   - ctx 里已有 TraceID(说明是上游透传下来的)→ 沿用,保证同一条链 TraceID 不变。
//   - 没有 → 生成一个新的(说明本服务是链路入口)。
// 生成后写回 ctx.TraceID,transport 会把它编码进 RpcHeader.trace_id 发给 C++ 服务端。
func Trace() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			if c.TraceID == "" {
				c.TraceID = trace.NewTraceID()
			}
			return next(c, req)
		}
	}
}
