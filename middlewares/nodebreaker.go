package middlewares

import (
	"context"
	"sync"

	"go-rpc/breaker"
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"go-rpc/transport"
	"google.golang.org/protobuf/proto"
)

// NodeBreaker 按节点(per-address)维护独立熔断器,并与负载均衡联动:
// 把已熔断(Open)的节点从候选地址里摘除,让流量只走健康节点。
//
// 【为什么要 per-node,而不是服务级熔断(修复4)】
// 服务级熔断"一刀切":只要整体错误率高就全熔断,即使只有一个节点坏了也会误伤健康节点。
// per-node 熔断更精细:哪个节点坏了就摘哪个,其余健康节点继续服务,可用性更高。
// 这也是 kratos / 主流框架的做法(节点级熔断 + 负载均衡摘点)。
//
// 【工作流程】
//   1. 选址前:transport 调用 Filter(addrs),NodeBreaker 剔除所有 Open 状态的节点。
//   2. 调用后:Middleware 读出本次实际使用的节点(ctx.SelectedAddr),把成败上报给
//      该节点的熔断器,更新其状态。
// 于是"坏节点被摘除 → 无流量 → 冷却到期转 HalfOpen → 放一个探测 → 恢复则重新接入"闭环。
type NodeBreaker struct {
	opts breaker.Options

	mu       sync.Mutex
	breakers map[string]*breaker.Breaker // addr → 该节点的熔断器
}

// NewNodeBreaker 用一份熔断配置创建 per-node 熔断管理器。
func NewNodeBreaker(opts breaker.Options) *NodeBreaker {
	return &NodeBreaker{opts: opts, breakers: make(map[string]*breaker.Breaker)}
}

// get 取(或懒创建)某地址的熔断器。
func (nb *NodeBreaker) get(addr string) *breaker.Breaker {
	nb.mu.Lock()
	defer nb.mu.Unlock()
	b, ok := nb.breakers[addr]
	if !ok {
		b = breaker.New(nb.opts)
		nb.breakers[addr] = b
	}
	return b
}

// Filter 实现 transport.NodeFilter:剔除处于 Open(熔断中)的节点。
// 若过滤后全空(所有节点都熔断),返回空切片,transport 会据此报"无健康节点"。
func (nb *NodeBreaker) Filter(addrs []string) []string {
	healthy := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if nb.get(a).State() != breaker.StateOpen {
			healthy = append(healthy, a)
		}
	}
	return healthy
}

// Middleware 挂在链上:调用后按"实际使用的节点"上报成败给对应熔断器。
//
// 【放在链的哪一层】要放在 transport 已经选好节点之后能拿到 SelectedAddr 的位置 ——
// 也就是比较靠内(接近最内层 handler)。它不拦截请求(拦截由 Filter 在选址前完成),
// 只负责"事后上报"。
func (nb *NodeBreaker) Middleware() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			resp, err := next(ctx, req)
			// next 执行后,transport 已把选中的节点写进 ctx。
			if addr := rpccontext.SelectedAddr(ctx); addr != "" {
				nb.get(addr).Report(err == nil)
			}
			return resp, err
		}
	}
}

// 确保 transport.NodeFilter 类型匹配(编译期检查):Filter 方法可作为 NodeFilter 使用。
var _ transport.NodeFilter = (*NodeBreaker)(nil).Filter
