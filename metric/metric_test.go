package metric

import (
	"sync"
	"testing"
	"time"
)

func TestCollectorAggregates(t *testing.T) {
	c := NewCollector()
	c.Observe("S.M", 10*time.Millisecond, false)
	c.Observe("S.M", 20*time.Millisecond, false)
	c.Observe("S.M", 30*time.Millisecond, true) // 1 次失败

	s := c.Snapshot()["S.M"]
	if s.Total != 3 {
		t.Fatalf("Total = %d, 期望 3", s.Total)
	}
	if s.Errors != 1 {
		t.Fatalf("Errors = %d, 期望 1", s.Errors)
	}
	// 错误率 1/3
	if r := s.ErrorRate(); r < 0.33 || r > 0.34 {
		t.Fatalf("ErrorRate = %f, 期望 ≈0.333", r)
	}
	// 平均耗时 (10+20+30)/3 = 20ms
	if s.AvgCost() != 20*time.Millisecond {
		t.Fatalf("AvgCost = %v, 期望 20ms", s.AvgCost())
	}
}

// 并发安全:100 个 goroutine 各记 100 次,总数必须精确 = 10000(无锁会丢计数)。
func TestCollectorConcurrent(t *testing.T) {
	c := NewCollector()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Observe("S.M", time.Millisecond, false)
			}
		}()
	}
	wg.Wait()
	if got := c.Snapshot()["S.M"].Total; got != 10000 {
		t.Fatalf("并发累计 = %d, 期望 10000(说明有数据竞争丢计数)", got)
	}
}
