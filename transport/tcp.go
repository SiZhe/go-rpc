package transport

import (
	"context"
	"fmt"
	"io"
	"time"

	"go-rpc/balancer"
	"go-rpc/discovery"
	"go-rpc/rpccontext"
)

// NodeFilter 在选址前过滤候选地址(如剔除已熔断的节点)。返回过滤后的地址列表。
// 为 nil 表示不过滤。这是"熔断与负载均衡联动"的接入点(见修复4)。
type NodeFilter func(addrs []string) []string

// TCPTransport 真实网络传输层:服务发现 → 过滤故障节点 → 负载均衡 → 连接池取连接
// → 发请求 → 收响应。全程受 context 的取消/超时控制。
type TCPTransport struct {
	registry discovery.Registry
	balancer balancer.Balancer
	pool     *connPool
	filter   NodeFilter // 可选:选址前剔除故障节点
}

// Option 用于可选配置(如注入节点过滤器)。
type Option func(*TCPTransport)

// WithNodeFilter 注入故障节点过滤器(熔断联动用)。
func WithNodeFilter(f NodeFilter) Option {
	return func(t *TCPTransport) { t.filter = f }
}

// NewTCPTransport 组装服务发现 + 负载均衡 + 连接池。
func NewTCPTransport(reg discovery.Registry, lb balancer.Balancer, opts ...Option) *TCPTransport {
	t := &TCPTransport{
		registry: reg,
		balancer: lb,
		pool:     newConnPool(8, 3*time.Second), // 每地址最多缓存 8 条空闲连接
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Send 匹配 client.RoundTrip 签名:输入 ctx 与已编码的 wire 帧,返回响应字节。
// 【ctx 的作用】建连(DialContext)与读写 deadline 都绑定 ctx,一旦上层超时/取消,
// 正在进行的网络操作会立即中止,不会泄漏。
func (t *TCPTransport) Send(ctx context.Context, frame []byte) ([]byte, error) {
	service, method := rpccontext.Service(ctx), rpccontext.Method(ctx)

	// 1. 服务发现
	addrs, err := t.registry.Discover(service, method)
	if err != nil {
		return nil, fmt.Errorf("transport: 服务发现失败: %w", err)
	}

	// 2. 剔除故障节点(熔断联动)。全被剔除则说明无健康节点。
	if t.filter != nil {
		addrs = t.filter(addrs)
		if len(addrs) == 0 {
			return nil, fmt.Errorf("transport: 无健康节点可用(全部熔断)")
		}
	}

	// 3. 负载均衡选址
	addr, err := t.balancer.Pick(addrs)
	if err != nil {
		return nil, fmt.Errorf("transport: 选址失败: %w", err)
	}
	// 记录选中的节点,供 per-node 熔断中间件按节点上报成败。
	rpccontext.SetSelectedAddr(ctx, addr)

	// 4. 从连接池取连接(建连受 ctx 控制)
	conn, err := t.pool.Get(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("transport: 连接 %s 失败: %w", addr, err)
	}

	// 把 ctx 的 deadline 同步到 socket 读写 deadline,让超时真正作用到网络 IO。
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	// 【监听 ctx 取消】起一个 watcher:ctx 被取消时立即关连接,打断阻塞的读写。
	// 用 done channel 在函数返回时停掉 watcher,避免泄漏。
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetDeadline(time.Now()) // 立即让读写超时返回
		case <-done:
		}
	}()

	// 5. 发送请求帧
	if _, err := conn.Write(frame); err != nil {
		t.pool.Put(addr, conn, false) // 连接可能已坏,不复用
		return nil, fmt.Errorf("transport: 发送失败: %w", err)
	}

	// 6. 读响应直到 EOF(MPRPC 短连接:服务端发完即关,读到 EOF 即完整响应)。
	respBytes, err := io.ReadAll(conn)
	if err != nil {
		t.pool.Put(addr, conn, false)
		// ctx 取消导致的读失败,归一化成 ctx.Err() 便于上层识别超时/取消。
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("transport: 读取响应失败: %w", err)
	}
	// MPRPC 短连接:服务端已关连接,这条连接不再健康,不放回池。
	t.pool.Put(addr, conn, false)

	if len(respBytes) == 0 {
		return nil, fmt.Errorf("transport: 服务端返回空响应(可能服务忙或方法不存在)")
	}
	return respBytes, nil
}

// Close 关闭连接池。
func (t *TCPTransport) Close() { t.pool.CloseAll() }
