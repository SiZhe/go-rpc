package breaker

import (
	"testing"
	"time"
)

func testOpts() Options {
	return Options{
		WindowSize:  1 * time.Second,
		BucketCount: 10,
		MinRequests: 10,
		ErrorRate:   0.5,
		CoolDown:    100 * time.Millisecond,
	}
}

// 初始 Closed,放行。
func TestInitiallyClosed(t *testing.T) {
	b := New(testOpts())
	if b.State() != StateClosed {
		t.Fatalf("初始状态应为 Closed,实际 %s", b.State())
	}
	if !b.Allow() {
		t.Fatal("Closed 状态应放行")
	}
}

// 错误率超阈值 → 跳闸到 Open,之后 Allow 拒绝。
func TestTripsToOpen(t *testing.T) {
	b := New(testOpts())
	// 上报 10 次全失败(>= MinRequests 且错误率 100% > 50%)。
	for i := 0; i < 10; i++ {
		b.Report(false)
	}
	if b.State() != StateOpen {
		t.Fatalf("应跳闸为 Open,实际 %s", b.State())
	}
	if b.Allow() {
		t.Fatal("Open 状态应拒绝请求")
	}
}

// 样本不足(< MinRequests)时不熔断,避免误判。
func TestNoTripBelowMinRequests(t *testing.T) {
	b := New(testOpts())
	for i := 0; i < 5; i++ { // 只有 5 次,少于 MinRequests=10
		b.Report(false)
	}
	if b.State() != StateClosed {
		t.Fatalf("样本不足不应熔断,实际 %s", b.State())
	}
}

// Open 冷却到期 → HalfOpen → 探测成功 → 回 Closed。
func TestHalfOpenRecovers(t *testing.T) {
	b := New(testOpts())
	for i := 0; i < 10; i++ {
		b.Report(false)
	}
	if b.State() != StateOpen {
		t.Fatalf("先应为 Open")
	}
	time.Sleep(120 * time.Millisecond) // 等过冷却期(100ms)
	if b.State() != StateHalfOpen {
		t.Fatalf("冷却到期应转 HalfOpen,实际 %s", b.State())
	}
	// State() 已把状态推进到 HalfOpen;此时探测成功应回 Closed。
	b.Report(true)
	if b.State() != StateClosed {
		t.Fatalf("探测成功应回 Closed,实际 %s", b.State())
	}
}

// HalfOpen 探测失败 → 回 Open。
func TestHalfOpenFailsBackToOpen(t *testing.T) {
	b := New(testOpts())
	for i := 0; i < 10; i++ {
		b.Report(false)
	}
	time.Sleep(120 * time.Millisecond)
	_ = b.State() // 推进到 HalfOpen
	b.Report(false)
	if b.State() != StateOpen {
		t.Fatalf("探测失败应回 Open,实际 %s", b.State())
	}
}
