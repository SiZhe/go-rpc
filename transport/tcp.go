package transport

import (
	"fmt"
	"io"
	"net"
	"time"

	"go-rpc/balancer"
	"go-rpc/discovery"
	"go-rpc/rpccontext"
)

// TCPTransport 真实的网络传输层:把"一次请求"完整地送到 C++ MPRPC 服务端并收回响应。
//
// 【它做的事(一次调用的完整网络流程)】
//  1. 服务发现:问 Registry 要 "service/method" 的实例地址列表。
//  2. 负载均衡:用 Balancer 从地址列表里挑一台。
//  3. 建立 TCP 连接(MPRPC 是短连接:一次调用一条连接,用完即关)。
//  4. 把已编码的 wire 帧发出去。
//  5. 读响应:MPRPC 服务端发完 response 就关闭连接,所以我们一直读到 EOF,
//     读到的全部字节就是 response 的 protobuf。
//
// 【为什么它是 client.RoundTrip 的真实现】
// 阶段一 client 里 RoundTrip 是可注入的假函数(内存回显)。这里 Send 方法的签名
// 完全匹配 client.RoundTrip:func(ctx, frame) ([]byte, error),所以可以直接注入进 Client,
// 把假的换成真的 —— 这就是阶段一"可注入 transport"设计埋下的接口。
type TCPTransport struct {
	registry discovery.Registry
	balancer balancer.Balancer
	// dialTimeout 建连超时。真正的调用超时由 timeout 中间件在更上层控制;
	// 这里只兜底"连不上对端"的情况,避免永久阻塞。
	dialTimeout time.Duration
}

// NewTCPTransport 组装服务发现 + 负载均衡。
func NewTCPTransport(reg discovery.Registry, lb balancer.Balancer) *TCPTransport {
	return &TCPTransport{
		registry:    reg,
		balancer:    lb,
		dialTimeout: 3 * time.Second,
	}
}

// Send 匹配 client.RoundTrip 签名:输入已编码的 wire 帧,返回响应字节。
func (t *TCPTransport) Send(c *rpccontext.RpcContext, frame []byte) ([]byte, error) {
	// 1. 服务发现
	addrs, err := t.registry.Discover(c.Service, c.Method)
	if err != nil {
		return nil, fmt.Errorf("transport: 服务发现失败: %w", err)
	}

	// 2. 负载均衡选址
	addr, err := t.balancer.Pick(addrs)
	if err != nil {
		return nil, fmt.Errorf("transport: 选址失败: %w", err)
	}

	// 3. 建立 TCP 连接(带建连超时)
	conn, err := net.DialTimeout("tcp", addr, t.dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("transport: 连接 %s 失败: %w", addr, err)
	}
	defer conn.Close() // 短连接:函数返回时关闭

	// 【超时透传】如果上层在 ctx 里设了 deadline,把它同步到 socket 读写 deadline,
	// 这样 timeout 中间件的截止时间能真正作用到网络 IO 上(否则读会一直阻塞)。
	if !c.Deadline.IsZero() {
		_ = conn.SetDeadline(c.Deadline)
	}

	// 4. 发送请求帧
	if _, err := conn.Write(frame); err != nil {
		return nil, fmt.Errorf("transport: 发送失败: %w", err)
	}

	// 5. 读响应直到 EOF。
	// 【为什么用 ReadAll】MPRPC 服务端 sendRpcResponse 发完就 conn.shutdown(),
	// 客户端这一侧会读到 EOF。此前读到的全部字节 = 完整的 response protobuf。
	// (这也是"短连接"协议的天然分帧方式:连接关闭即一条消息结束。)
	respBytes, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("transport: 读取响应失败: %w", err)
	}
	if len(respBytes) == 0 {
		return nil, fmt.Errorf("transport: 服务端返回空响应(可能服务忙或方法不存在)")
	}
	return respBytes, nil
}
