package ratelimit

import (
	"testing"
	"time"
)

// 桶容量 3、速率极低:前 3 个请求靠初始令牌放行,第 4 个应被限流。
func TestTokenBucketBurstThenLimit(t *testing.T) {
	tb := NewTokenBucket(0.0001, 3) // 速率几乎为 0,基本只有初始的 3 个令牌
	for i := 0; i < 3; i++ {
		if !tb.Allow() {
			t.Fatalf("前 3 个请求(初始令牌)应放行,第 %d 个被拒", i+1)
		}
	}
	if tb.Allow() {
		t.Fatal("第 4 个请求应被限流")
	}
}

// 令牌用尽后,等待一段时间,应按速率补充出新令牌。
func TestTokenBucketRefill(t *testing.T) {
	tb := NewTokenBucket(100, 1) // 每秒补 100 个,容量 1
	if !tb.Allow() {
		t.Fatal("初始令牌应放行")
	}
	if tb.Allow() {
		t.Fatal("紧接着第二个应被限流(容量只有 1)")
	}
	time.Sleep(50 * time.Millisecond) // 50ms 按 100/s 速率应补出约 5 个令牌
	if !tb.Allow() {
		t.Fatal("等待补充后应放行")
	}
}
