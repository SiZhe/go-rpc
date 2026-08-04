package middlewares

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func fastRetryOpts(maxAttempts int) RetryOptions {
	return RetryOptions{MaxAttempts: maxAttempts, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
}

// 前 2 次失败、第 3 次成功:应最终成功。
func TestRetryEventuallySucceeds(t *testing.T) {
	calls := 0
	flaky := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("temporary failure")
		}
		return wrapperspb.String("ok"), nil
	}
	h := Retry(fastRetryOpts(3))(flaky)
	resp, err := h(context.Background(), nil)
	if err != nil {
		t.Fatalf("应最终成功: %v", err)
	}
	if resp.(*wrapperspb.StringValue).Value != "ok" || calls != 3 {
		t.Fatalf("calls = %d, 期望 3", calls)
	}
}

// 一直失败:用尽次数后返回错误。
func TestRetryExhausted(t *testing.T) {
	calls := 0
	always := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		calls++
		return nil, errors.New("down")
	}
	h := Retry(fastRetryOpts(3))(always)
	if _, err := h(context.Background(), nil); err == nil {
		t.Fatal("应返回错误")
	}
	if calls != 3 {
		t.Fatalf("calls = %d, 期望 3", calls)
	}
}

// 非幂等:Idempotent 返回 false,只调一次,不重试。
func TestRetrySkipsNonIdempotent(t *testing.T) {
	calls := 0
	always := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		calls++
		return nil, errors.New("down")
	}
	opts := fastRetryOpts(3)
	opts.Idempotent = func(ctx context.Context) bool { return false }
	h := Retry(opts)(always)
	_, _ = h(context.Background(), nil)
	if calls != 1 {
		t.Fatalf("非幂等应只调 1 次,实际 %d", calls)
	}
}

// ctx 取消:重试等待期间被取消,应尽快返回 ctx 错误。
func TestRetryRespectsContextCancel(t *testing.T) {
	always := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		return nil, errors.New("down")
	}
	opts := RetryOptions{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	h := Retry(opts)(always)
	_, _ = h(ctx, nil)
	if time.Since(start) > 300*time.Millisecond {
		t.Fatalf("应因 ctx 取消尽快返回,实际耗时 %v", time.Since(start))
	}
}
