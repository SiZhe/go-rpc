package client

import (
	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"go-rpc/transport"
	"google.golang.org/protobuf/proto"
)

// RoundTrip 执行一次真正的请求发送:输入已编码的 wire 帧,返回响应字节。
// 阶段一注入假实现;阶段二替换为 zk 发现 + TCP。
type RoundTrip func(c *rpccontext.RpcContext, frame []byte) ([]byte, error)

type Client struct {
	rt    RoundTrip
	chain middleware.Middleware
}

// New 构造 Client,mws 按声明顺序组成洋葱链。
func New(rt RoundTrip, mws ...middleware.Middleware) *Client {
	return &Client{rt: rt, chain: middleware.Chain(mws...)}
}

// Call 发起一次 RPC:req 序列化 → 编码 wire 帧 → 经中间件链 → RoundTrip → 反序列化到 resp。
func (cli *Client) Call(c *rpccontext.RpcContext, req, resp proto.Message) error {
	final := func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		args, err := proto.Marshal(req)
		if err != nil {
			return nil, err
		}
		frame, err := transport.EncodeRequest(c.Service, c.Method, args, c.TraceID)
		if err != nil {
			return nil, err
		}
		respBytes, err := cli.rt(c, frame)
		if err != nil {
			return nil, err
		}
		if err := proto.Unmarshal(transport.DecodeResponse(respBytes), resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	_, err := cli.chain(final)(c, req)
	return err
}
