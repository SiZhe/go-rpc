package trace

import "testing"

func TestNewTraceIDFormat(t *testing.T) {
	id := NewTraceID()
	if len(id) != 32 { // 16 字节 → 32 个 hex 字符
		t.Fatalf("TraceID 长度 = %d, 期望 32", len(id))
	}
}

func TestNewTraceIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewTraceID()
		if seen[id] {
			t.Fatal("TraceID 出现重复")
		}
		seen[id] = true
	}
}

func TestNewSpanIDFormat(t *testing.T) {
	if len(NewSpanID()) != 16 { // 8 字节 → 16 hex
		t.Fatal("SpanID 长度错误")
	}
}
