package middlewares

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-rpc/breaker"
	"google.golang.org/protobuf/proto"
)

// 下游持续失败 → 熔断打开 → 后续请求被快速拒绝(ErrOpen),不再调用 next。
func TestBreakerMiddlewareFastFails(t *testing.T) {
	b := breaker.New(breaker.Options{
		WindowSize: time.Second, BucketCount: 10,
		MinRequests: 5, ErrorRate: 0.5, CoolDown: time.Second,
	})
	nextCalls := 0
	failing := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		nextCalls++
		return nil, errors.New("down")
	}
	h := Breaker(b)(failing)

	for i := 0; i < 5; i++ {
		_, _ = h(context.Background(), nil)
	}
	callsBefore := nextCalls

	_, err := h(context.Background(), nil)
	if !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("熔断后应返回 ErrOpen,实际 %v", err)
	}
	if nextCalls != callsBefore {
		t.Fatal("熔断后不应再调用 next")
	}
}
