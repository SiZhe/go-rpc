package middleware

import (
	"testing"

	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// 记录执行顺序,验证洋葱:mw1 前 → mw2 前 → handler → mw2 后 → mw1 后
func TestChainOnionOrder(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
				order = append(order, name+"-before")
				resp, err := next(c, req)
				order = append(order, name+"-after")
				return resp, err
			}
		}
	}
	final := func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		order = append(order, "handler")
		return nil, nil
	}
	h := Chain(mk("mw1"), mk("mw2"))(final)
	_, _ = h(rpccontext.New("S", "M"), nil)

	want := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %s, want %s", i, order[i], want[i])
		}
	}
}
