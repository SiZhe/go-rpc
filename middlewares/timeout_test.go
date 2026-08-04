package middlewares

import (
	"testing"
	"time"

	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestTimeoutFastCallPasses(t *testing.T) {
	// next 立刻返回,不该超时。
	fast := func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		return wrapperspb.String("ok"), nil
	}
	h := Timeout(100 * time.Millisecond)(fast)
	resp, err := h(rpccontext.New("S", "M"), nil)
	if err != nil {
		t.Fatalf("快速调用不应超时: %v", err)
	}
	if resp.(*wrapperspb.StringValue).Value != "ok" {
		t.Fatal("响应错误")
	}
}

func TestTimeoutSlowCallFails(t *testing.T) {
	// next 睡 200ms,超时阈值 50ms,应超时。
	slow := func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		time.Sleep(200 * time.Millisecond)
		return wrapperspb.String("late"), nil
	}
	h := Timeout(50 * time.Millisecond)(slow)
	_, err := h(rpccontext.New("S", "M"), nil)
	if err == nil {
		t.Fatal("慢调用应返回超时错误")
	}
}
