package middlewares

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestTimeoutFastCallPasses(t *testing.T) {
	fast := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		return wrapperspb.String("ok"), nil
	}
	h := Timeout(100 * time.Millisecond)(fast)
	resp, err := h(context.Background(), nil)
	if err != nil {
		t.Fatalf("快速调用不应超时: %v", err)
	}
	if resp.(*wrapperspb.StringValue).Value != "ok" {
		t.Fatal("响应错误")
	}
}

// 慢调用:next 监听 ctx.Done(),超时后应尽快返回 ctx 的错误(验证 context 取消传播)。
func TestTimeoutSlowCallCanceled(t *testing.T) {
	slow := func(ctx context.Context, req proto.Message) (proto.Message, error) {
		select {
		case <-time.After(500 * time.Millisecond):
			return wrapperspb.String("late"), nil
		case <-ctx.Done(): // 超时信号到达,立即返回
			return nil, ctx.Err()
		}
	}
	start := time.Now()
	h := Timeout(50 * time.Millisecond)(slow)
	_, err := h(context.Background(), nil)
	if err == nil {
		t.Fatal("慢调用应返回超时错误")
	}
	// 应在 ~50ms 附近返回,而不是等满 500ms —— 证明 ctx 取消真正打断了调用。
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("超时未及时打断,耗时 %v", time.Since(start))
	}
}
