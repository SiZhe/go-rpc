package middlewares

import (
	"context"
	"testing"

	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// 无 TraceID 时应生成一个,并在 next 里可见。
func TestTraceGeneratesID(t *testing.T) {
	var seen string
	h := Trace()(func(ctx context.Context, req proto.Message) (proto.Message, error) {
		seen = rpccontext.TraceID(ctx)
		return wrapperspb.String("ok"), nil
	})
	ctx := rpccontext.New(context.Background(), "S", "M")
	_, _ = h(ctx, nil)
	if seen == "" {
		t.Fatal("应生成 TraceID")
	}
}

// 已有 TraceID(上游透传)时应沿用,不覆盖。
func TestTracePreservesExistingID(t *testing.T) {
	ctx := rpccontext.New(context.Background(), "S", "M")
	rpccontext.SetTraceID(ctx, "upstream-trace")
	var seen string
	h := Trace()(func(ctx context.Context, req proto.Message) (proto.Message, error) {
		seen = rpccontext.TraceID(ctx)
		return wrapperspb.String("ok"), nil
	})
	_, _ = h(ctx, nil)
	if seen != "upstream-trace" {
		t.Fatalf("应沿用上游 TraceID,实际 %s", seen)
	}
}
