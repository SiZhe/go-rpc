package client

import (
	"context"
	"testing"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestClientCallInvokesMiddlewareAndRoundTrip(t *testing.T) {
	var mwHit bool
	mw := func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			mwHit = true
			return next(ctx, req)
		}
	}
	// 假 transport:回显一个 StringValue 响应,内容带上方法名验证 ctx 透传。
	rt := func(ctx context.Context, frame []byte) ([]byte, error) {
		resp := wrapperspb.String("ok:" + rpccontext.Method(ctx))
		return proto.Marshal(resp)
	}
	cli := New(rt, mw)

	req := wrapperspb.String("ping")
	var resp wrapperspb.StringValue
	ctx := rpccontext.New(context.Background(), "UserServiceRpc", "Login")
	if err := cli.Call(ctx, req, &resp); err != nil {
		t.Fatal(err)
	}
	if !mwHit {
		t.Fatal("中间件未被执行")
	}
	if resp.Value != "ok:Login" {
		t.Fatalf("resp = %q, want ok:Login", resp.Value)
	}
}
