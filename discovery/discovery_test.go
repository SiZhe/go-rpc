package discovery

import "testing"

func TestStaticRegistryDiscover(t *testing.T) {
	r := NewStaticRegistry(map[string][]string{
		"UserServiceRpc/Login": {"127.0.0.1:8000", "127.0.0.1:8001"},
	})

	addrs, err := r.Discover("UserServiceRpc", "Login")
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 {
		t.Fatalf("addrs = %v, want 2 个", addrs)
	}
}

func TestStaticRegistryNotFound(t *testing.T) {
	r := NewStaticRegistry(map[string][]string{})
	if _, err := r.Discover("X", "Y"); err == nil {
		t.Fatal("找不到实例时应返回 error")
	}
}

// 验证返回的是副本,调用方改动不影响内部数据。
func TestStaticRegistryReturnsCopy(t *testing.T) {
	r := NewStaticRegistry(map[string][]string{"S/M": {"a", "b"}})
	addrs, _ := r.Discover("S", "M")
	addrs[0] = "MUTATED"
	again, _ := r.Discover("S", "M")
	if again[0] != "a" {
		t.Fatalf("内部数据被外部修改污染了: %v", again)
	}
}
