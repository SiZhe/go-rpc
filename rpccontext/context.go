// Package rpccontext 在标准 context.Context 之上,携带一次 RPC 调用的元信息。
//
// 【为什么用标准 context.Context(而不是自造结构体)】
// Go 的 context.Context 是整个生态处理"取消 / 超时 / 请求级数据传递"的标准方式。
// 用它有两个关键好处:
//   1. 取消传播:超时中间件用 context.WithTimeout 派生带截止时间的 ctx,一旦超时,
//      ctx.Done() 会被关闭。下层(net 包的 DialContext / Read)只要监听这个信号,
//      就能"立刻停下正在进行的网络操作",而不是傻等 —— 这才是真正的超时,不会泄漏
//      goroutine。这是自造 RpcContext 做不到的(之前版本的缺陷)。
//   2. 生态兼容:database/sql、net、grpc 等所有库都接受 context.Context,天然打通。
//
// 【元信息怎么存】
// RPC 的路由信息(service/method)、TraceID、metadata 不适合塞进函数参数一路传递,
// 而是用 context.WithValue 挂在 ctx 上,需要时用本包的 getter 取出。这是 context 的
// 标准用法:携带"请求作用域"的数据。
package rpccontext

import "context"

// meta 是挂在 context 上的 RPC 元信息。用私有类型 + 私有 key,避免与其它包的
// context value 冲突(context 官方推荐做法:key 用自定义私有类型)。
type meta struct {
	service  string
	method   string
	traceID  string
	metadata map[string]string
	// selectedAddr 记录本次调用负载均衡选中的节点地址。由 transport 写入,
	// per-node 熔断中间件读取它来按节点上报成败。见 middlewares/nodebreaker.go。
	selectedAddr string
}

// ctxKey 私有类型作为 context value 的 key,防止跨包碰撞。
type ctxKey struct{}

// New 基于父 ctx 派生一个携带 service/method 元信息的新 ctx。
// 若父 ctx 未提供则用 context.Background()。
func New(parent context.Context, service, method string) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	m := &meta{service: service, method: method, metadata: map[string]string{}}
	return context.WithValue(parent, ctxKey{}, m)
}

// fromCtx 取出挂在 ctx 上的 meta;没有则返回一个空 meta(避免调用方判空)。
func fromCtx(ctx context.Context) *meta {
	if m, ok := ctx.Value(ctxKey{}).(*meta); ok {
		return m
	}
	return &meta{metadata: map[string]string{}}
}

// Service / Method 读取路由信息。
func Service(ctx context.Context) string { return fromCtx(ctx).service }
func Method(ctx context.Context) string  { return fromCtx(ctx).method }

// TraceID 读取链路追踪 ID。
func TraceID(ctx context.Context) string { return fromCtx(ctx).traceID }

// SetTraceID 设置 TraceID(在同一个 meta 指针上原地修改;因为 meta 是指针,
// 同一条 ctx 链上的读取都能看到)。由 trace 中间件调用。
func SetTraceID(ctx context.Context, id string) { fromCtx(ctx).traceID = id }

// SetMeta / Meta 读写透传元数据。
func SetMeta(ctx context.Context, k, v string) { fromCtx(ctx).metadata[k] = v }
func Meta(ctx context.Context, k string) string { return fromCtx(ctx).metadata[k] }
func Metadata(ctx context.Context) map[string]string { return fromCtx(ctx).metadata }

// SelectedAddr / SetSelectedAddr 读写本次调用选中的节点地址(transport 写、熔断中间件读)。
func SelectedAddr(ctx context.Context) string       { return fromCtx(ctx).selectedAddr }
func SetSelectedAddr(ctx context.Context, addr string) { fromCtx(ctx).selectedAddr = addr }
