package client

import (
	"testing"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestClientCallInvokesMiddlewareAndRoundTrip(t *testing.T) {
	var mwHit bool
	mw := func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			mwHit = true
			return next(c, req)
		}
	}
	// 假 transport:回显一个 StringValue 响应
	rt := func(c *rpccontext.RpcContext, frame []byte) ([]byte, error) {
		resp := wrapperspb.String("ok:" + c.Method)
		return proto.Marshal(resp)
	}
	cli := New(rt, mw)

	req := wrapperspb.String("ping")
	var resp wrapperspb.StringValue
	err := cli.Call(rpccontext.New("UserServiceRpc", "Login"), req, &resp)
	if err != nil {
		t.Fatal(err)
	}
	if !mwHit {
		t.Fatal("中间件未被执行")
	}
	if resp.Value != "ok:Login" {
		t.Fatalf("resp = %q, want ok:Login", resp.Value)
	}
}
