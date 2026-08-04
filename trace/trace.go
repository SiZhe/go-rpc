// Package trace 分布式链路追踪的最小实现。
//
// 【这个包解决什么问题(面试高频)】
// 一个请求在微服务里往往要穿过好几个服务(A→B→C)。线上出问题时,如何把"同一个请求"
// 在各个服务产生的日志/耗时串成一条完整链路?答案是给每个请求分配一个全局唯一的
// TraceID,并在服务间调用时把它透传下去。所有服务打日志都带上这个 TraceID,
// 事后用它一搜,整条链路就串起来了。
//
// 【TraceID / SpanID 的区别】
//   - TraceID:一整条链路的唯一 ID,从入口到最深的下游全程不变。
//   - SpanID: 链路里"一段"操作的 ID(如"A 调 B"是一个 span)。每一跳生成新的 SpanID,
//             但 TraceID 保持不变。通过 parentSpanID 能还原出调用树。
// 本实现聚焦 TraceID 的生成与跨进程透传(SpanID 简化处理),够讲清核心思想。
//
// 【怎么跨进程透传】
// 在本项目里,TraceID 被写进 RpcContext.TraceID → 由 transport 编码进 RpcHeader.trace_id
// 字段 → 跟着 TCP 包发给 C++ 服务端 → C++ 侧解出 trace_id 打进它的日志。这样 Go 客户端
// 和 C++ 服务端的日志就能用同一个 TraceID 关联起来 —— 这正是我们给 RpcHeader 加
// trace_id 字段的原因。
package trace

import (
	"crypto/rand"
	"encoding/hex"
)

// NewTraceID 生成一个全局唯一的 TraceID(16 字节随机数的 hex,32 个十六进制字符)。
//
// 【为什么用随机数而不是自增 ID】
// 分布式环境下没有全局自增器(或代价很高)。128 位随机数碰撞概率极低,各服务本地即可
// 生成、无需协调,这也是 OpenTelemetry TraceID 的做法(128-bit)。
func NewTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand 极少出错;即便出错得到全 0,也不影响功能正确性
	return hex.EncodeToString(b)
}

// NewSpanID 生成一个 SpanID(8 字节随机数的 hex,16 个十六进制字符)。
func NewSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
