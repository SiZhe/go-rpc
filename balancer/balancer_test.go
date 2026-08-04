package balancer

import "testing"

func TestRoundRobinCycles(t *testing.T) {
	b := NewRoundRobin()
	addrs := []string{"a", "b", "c"}
	// 连续选 6 次,应是 a b c a b c(严格轮流)。
	want := []string{"a", "b", "c", "a", "b", "c"}
	for i, w := range want {
		got, err := b.Pick(addrs)
		if err != nil {
			t.Fatal(err)
		}
		if got != w {
			t.Fatalf("第 %d 次选择 = %s, 期望 %s", i, got, w)
		}
	}
}

func TestRoundRobinEmpty(t *testing.T) {
	if _, err := NewRoundRobin().Pick(nil); err == nil {
		t.Fatal("空地址应报错")
	}
}

func TestRandomAlwaysInSet(t *testing.T) {
	b := NewRandom()
	addrs := []string{"a", "b", "c"}
	set := map[string]bool{"a": true, "b": true, "c": true}
	for i := 0; i < 100; i++ {
		got, err := b.Pick(addrs)
		if err != nil {
			t.Fatal(err)
		}
		if !set[got] {
			t.Fatalf("选出了不在集合里的地址: %s", got)
		}
	}
}

// 验证平滑加权轮询:权重 a:3 b:1,4 次一轮里 a 出现 3 次、b 出现 1 次。
func TestWeightedDistribution(t *testing.T) {
	b := NewWeighted(map[string]int{"a": 3, "b": 1})
	count := map[string]int{}
	for i := 0; i < 4; i++ {
		got, _ := b.Pick(nil)
		count[got]++
	}
	if count["a"] != 3 || count["b"] != 1 {
		t.Fatalf("加权分布 = %v, 期望 a:3 b:1", count)
	}
}
