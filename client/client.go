package client

import (
	"context"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"go-rpc/transport"
	"google.golang.org/protobuf/proto"
)

// RoundTrip 执行一次真正的请求发送:输入 ctx 与已编码的 wire 帧,返回响应字节。
// 生产用 transport.TCPTransport.Send;测试可注入内存假实现。
type RoundTrip func(ctx context.Context, frame []byte) ([]byte, error)

type Client struct {
	rt    RoundTrip
	chain middleware.Middleware
}

// New 构造 Client,mws 按声明顺序组成洋葱链。
func New(rt RoundTrip, mws ...middleware.Middleware) *Client {
	return &Client{rt: rt, chain: middleware.Chain(mws...)}
}

// Call 发起一次 RPC。
// ctx 应由 rpccontext.New(parent, service, method) 构造,携带路由信息;
// 也可在外层用 context.WithTimeout/WithCancel 派生,取消/超时会一路传到网络层。
func (cli *Client) Call(ctx context.Context, req, resp proto.Message) error {
	final := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		args, err := proto.Marshal(req)
		if err != nil {
			return nil, err
		}
		// 从 ctx 取出元信息编码进 wire 帧(traceID/deadline/metadata 跨进程透传)。
		var deadlineMs int64
		if dl, ok := ctx.Deadline(); ok {
			deadlineMs = dl.UnixMilli()
		}
		frame, err := transport.EncodeRequest(
			rpccontext.Service(ctx), rpccontext.Method(ctx), args,
			rpccontext.TraceID(ctx), deadlineMs, rpccontext.Metadata(ctx),
		)
		if err != nil {
			return nil, err
		}
		respBytes, err := cli.rt(ctx, frame)
		if err != nil {
			return nil, err
		}
		if err := proto.Unmarshal(transport.DecodeResponse(respBytes), resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	_, err := cli.chain(final)(ctx, req)
	return err
}
