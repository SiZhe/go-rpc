package middlewares

import (
	"testing"

	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// 无 TraceID 时应生成一个,并在 next 里可见。
func TestTraceGeneratesID(t *testing.T) {
	var seen string
	h := Trace()(func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		seen = c.TraceID
		return wrapperspb.String("ok"), nil
	})
	_, _ = h(rpccontext.New("S", "M"), nil)
	if seen == "" {
		t.Fatal("应生成 TraceID")
	}
}

// 已有 TraceID(上游透传)时应沿用,不覆盖。
func TestTracePreservesExistingID(t *testing.T) {
	c := rpccontext.New("S", "M")
	c.TraceID = "upstream-trace"
	var seen string
	h := Trace()(func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
		seen = c.TraceID
		return wrapperspb.String("ok"), nil
	})
	_, _ = h(c, nil)
	if seen != "upstream-trace" {
		t.Fatalf("应沿用上游 TraceID,实际 %s", seen)
	}
}
