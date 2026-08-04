package rpccontext

import (
	"context"
	"testing"
)

func TestNewCarriesServiceMethod(t *testing.T) {
	ctx := New(context.Background(), "UserServiceRpc", "Login")
	if Service(ctx) != "UserServiceRpc" || Method(ctx) != "Login" {
		t.Fatalf("got %s/%s", Service(ctx), Method(ctx))
	}
}

func TestTraceIDSetGet(t *testing.T) {
	ctx := New(context.Background(), "S", "M")
	SetTraceID(ctx, "trace-xyz")
	if TraceID(ctx) != "trace-xyz" {
		t.Fatalf("traceID = %q", TraceID(ctx))
	}
}

func TestMetadataSetGet(t *testing.T) {
	ctx := New(context.Background(), "S", "M")
	SetMeta(ctx, "k", "v")
	if Meta(ctx, "k") != "v" {
		t.Fatalf("meta k = %q", Meta(ctx, "k"))
	}
	if Meta(ctx, "missing") != "" {
		t.Fatalf("missing meta 应为空")
	}
}

// 未挂 meta 的裸 ctx 读取应安全返回零值,不 panic。
func TestBareContextSafe(t *testing.T) {
	if Service(context.Background()) != "" {
		t.Fatal("裸 ctx 的 Service 应为空")
	}
}

// 派生 ctx 应保留父 ctx 的取消能力(context 传播的关键)。
func TestNewPreservesCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	ctx := New(parent, "S", "M")
	cancel()
	select {
	case <-ctx.Done():
		// 正确:父取消后派生 ctx 也被取消
	default:
		t.Fatal("派生 ctx 应继承父 ctx 的取消信号")
	}
}
