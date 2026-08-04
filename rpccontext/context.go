package rpccontext

import "time"

// RpcContext 贯穿一次 RPC 调用的上下文,承载路由信息、traceID、超时与元数据。
type RpcContext struct {
	Service  string
	Method   string
	TraceID  string
	Deadline time.Time // 零值表示无超时
	metadata map[string]string
}

func New(service, method string) *RpcContext {
	return &RpcContext{Service: service, Method: method, metadata: map[string]string{}}
}

func (c *RpcContext) SetMeta(k, v string) { c.metadata[k] = v }

func (c *RpcContext) Meta(k string) string { return c.metadata[k] }

func (c *RpcContext) Metadata() map[string]string { return c.metadata }
