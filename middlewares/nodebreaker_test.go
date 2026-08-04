package middlewares

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-rpc/breaker"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

func nbOpts() breaker.Options {
	return breaker.Options{
		WindowSize: time.Second, BucketCount: 10,
		MinRequests: 5, ErrorRate: 0.5, CoolDown: time.Second,
	}
}

// 某节点持续失败 → 该节点熔断 → Filter 把它从候选里摘除。
func TestNodeBreakerFiltersUnhealthy(t *testing.T) {
	nb := NewNodeBreaker(nbOpts())

	// 模拟节点 "bad:1" 连续失败:通过中间件按节点上报。
	failing := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		return nil, errors.New("down")
	}
	mw := nb.Middleware()(failing)

	for i := 0; i < 5; i++ {
		ctx := rpccontext.New(context.Background(), "S", "M")
		rpccontext.SetSelectedAddr(ctx, "bad:1") // 模拟 transport 选了 bad:1
		_, _ = mw(ctx, nil)
	}

	// 此时 bad:1 应已熔断,Filter 应摘除它,只留 good:1。
	healthy := nb.Filter([]string{"bad:1", "good:1"})
	if len(healthy) != 1 || healthy[0] != "good:1" {
		t.Fatalf("应只保留 good:1,实际 %v", healthy)
	}
}

// 健康节点不应被摘除。
func TestNodeBreakerKeepsHealthy(t *testing.T) {
	nb := NewNodeBreaker(nbOpts())
	healthy := nb.Filter([]string{"a:1", "b:1"})
	if len(healthy) != 2 {
		t.Fatalf("健康节点应全保留,实际 %v", healthy)
	}
}
