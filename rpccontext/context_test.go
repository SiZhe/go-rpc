package rpccontext

import "testing"

func TestNewCarriesServiceMethod(t *testing.T) {
	c := New("UserServiceRpc", "Login")
	if c.Service != "UserServiceRpc" || c.Method != "Login" {
		t.Fatalf("got %s/%s", c.Service, c.Method)
	}
}

func TestMetadataSetGet(t *testing.T) {
	c := New("S", "M")
	c.SetMeta("k", "v")
	if got := c.Meta("k"); got != "v" {
		t.Fatalf("meta = %q, want v", got)
	}
	if got := c.Meta("missing"); got != "" {
		t.Fatalf("missing meta = %q, want empty", got)
	}
}
