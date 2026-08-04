package transport

import (
	"context"
	"net"
	"sync"
	"time"
)

// connPool 是一个"按地址分组的空闲连接池"。
//
// 【为什么需要连接池(修复短连接的性能问题)】
// 原来每次 RPC 都新建一条 TCP 连接、用完就关。建连有三次握手开销,高频调用下浪费严重。
// 连接池把用完的连接"还回来"复用,下次直接拿现成的,省去反复握手。
//
// 【本实现的策略(简单可靠版)】
//   - 每个地址一个空闲连接队列(带上限 maxIdle)。
//   - Get:优先从空闲队列拿;拿到的连接若已失效(对端关闭)则丢弃重拿;没有则新建。
//   - Put:调用成功后把连接放回队列;队列满了或连接坏了就直接关闭。
//
// 【与 MPRPC 短连接协议的矛盾怎么解决(重要)】
// MPRPC 服务端当前是"发完响应就关连接"的短连接。所以严格来说,和这个 C++ 服务端通信时
// 连接无法复用(服务端会主动关)。连接池在这里的价值有两点:
//   1. 面向未来:如果服务端改成长连接(加消息边界),客户端无需改动即可复用。
//   2. 教学价值:连接池是 RPC 框架标配,这里给出完整实现;isAlive 检测能识别被对端
//      关闭的连接并丢弃,保证即使对端是短连接也不会拿到坏连接。
type connPool struct {
	mu       sync.Mutex
	idle     map[string][]net.Conn // addr → 空闲连接列表
	maxIdle  int                   // 每个地址最多缓存多少空闲连接
	dialer   *net.Dialer
}

func newConnPool(maxIdle int, dialTimeout time.Duration) *connPool {
	return &connPool{
		idle:    make(map[string][]net.Conn),
		maxIdle: maxIdle,
		dialer:  &net.Dialer{Timeout: dialTimeout},
	}
}

// Get 取一个到 addr 的可用连接:先复用空闲的(跳过已失效),没有则新建。
// 用 DialContext 让建连也受 ctx 取消/超时控制。
func (p *connPool) Get(ctx context.Context, addr string) (net.Conn, error) {
	p.mu.Lock()
	conns := p.idle[addr]
	for len(conns) > 0 {
		// 从队尾取一个(栈式,热连接优先)
		c := conns[len(conns)-1]
		conns = conns[:len(conns)-1]
		if isAlive(c) {
			p.idle[addr] = conns
			p.mu.Unlock()
			return c, nil // 复用一个仍然存活的空闲连接
		}
		_ = c.Close() // 已失效,丢弃继续找
	}
	p.idle[addr] = conns
	p.mu.Unlock()

	// 没有可复用的,新建。DialContext:建连过程也响应 ctx 取消/超时。
	return p.dialer.DialContext(ctx, "tcp", addr)
}

// Put 把用完且健康的连接放回池;池满或连接坏则关闭。
func (p *connPool) Put(addr string, c net.Conn, healthy bool) {
	if !healthy {
		_ = c.Close()
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle[addr]) >= p.maxIdle {
		_ = c.Close() // 空闲已够多,不再缓存
		return
	}
	p.idle[addr] = append(p.idle[addr], c)
}

// isAlive 粗略探测连接是否还活着:设一个极短的读 deadline 试读 1 字节。
//   - 读到 io.EOF / 对端已关 → 认为已失效。
//   - 读超时(Timeout)→ 说明连接正常(只是当前没数据),存活。
// 探测后清掉读 deadline,不影响后续正常使用。
//
// 【为什么这样判断】TCP 连接被对端关闭后,本端读会立即返回 EOF;而健康的空闲连接此刻
// 没有数据可读,会因为我们设的短 deadline 而超时 —— 用"是否超时"区分"关闭"与"健康空闲"。
func isAlive(c net.Conn) bool {
	_ = c.SetReadDeadline(time.Now().Add(time.Millisecond))
	buf := make([]byte, 1)
	_, err := c.Read(buf)
	_ = c.SetReadDeadline(time.Time{}) // 清除 deadline

	if err == nil {
		// 居然读到了数据 —— 空闲连接不该有残留数据,视为异常连接,丢弃。
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true // 超时 = 健康的空闲连接
	}
	return false // 其它错误(含 EOF)= 连接已失效
}

// CloseAll 关闭池中所有空闲连接(优雅退出用)。
func (p *connPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for addr, conns := range p.idle {
		for _, c := range conns {
			_ = c.Close()
		}
		delete(p.idle, addr)
	}
}
