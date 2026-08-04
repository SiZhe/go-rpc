package middlewares

import (
	"errors"
	"testing"
	"time"

	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// 前 2 次失败、第 3 次成功:总尝试 3 次应最终成功。
func TestRetryEventuallySucceeds(t *testing.T) {
	calls := 0
	flaky := func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("temporary failure")
		}
		return wrapperspb.String("ok"), nil
	}
	h := Retry(3, time.Millisecond)(flaky)
	resp, err := h(rpccontext.New("S", "M"), nil)
	if err != nil {
		t.Fatalf("应最终成功: %v", err)
	}
	if resp.(*wrapperspb.StringValue).Value != "ok" || calls != 3 {
		t.Fatalf("calls = %d, 期望 3", calls)
	}
}

// 一直失败:用尽 maxAttempts 次后返回错误,且尝试次数正确。
func TestRetryExhausted(t *testing.T) {
	calls := 0
	always := func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		calls++
		return nil, errors.New("down")
	}
	h := Retry(3, time.Millisecond)(always)
	_, err := h(rpccontext.New("S", "M"), nil)
	if err == nil {
		t.Fatal("应返回错误")
	}
	if calls != 3 {
		t.Fatalf("calls = %d, 期望尝试 3 次", calls)
	}
}
