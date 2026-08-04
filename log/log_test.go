package log

import (
	"strings"
	"sync"
	"testing"
)

// 验证:日志被异步写出,且包含 TraceID 和字段。
func TestLoggerWritesWithTrace(t *testing.T) {
	l := New(INFO)

	var mu sync.Mutex
	var lines []string
	l.SetSink(func(s string) {
		mu.Lock()
		lines = append(lines, s)
		mu.Unlock()
	})

	l.Infoc("trace-abc", "call done", map[string]string{"method": "Login"})
	l.Close() // 等后台写完

	if len(lines) != 1 {
		t.Fatalf("应输出 1 条日志,实际 %d", len(lines))
	}
	line := lines[0]
	if !strings.Contains(line, "trace-abc") {
		t.Errorf("日志应含 TraceID: %s", line)
	}
	if !strings.Contains(line, "method=Login") {
		t.Errorf("日志应含字段 method=Login: %s", line)
	}
	if !strings.Contains(line, "INFO") {
		t.Errorf("日志应含级别 INFO: %s", line)
	}
}

// 低于最小级别的日志被丢弃。
func TestLoggerLevelFilter(t *testing.T) {
	l := New(WARN) // 只记 WARN 及以上

	var mu sync.Mutex
	var count int
	l.SetSink(func(s string) { mu.Lock(); count++; mu.Unlock() })

	l.Infoc("t", "should be dropped", nil) // INFO < WARN,丢弃
	l.Errorc("t", "should be kept", nil)   // ERROR >= WARN,保留
	l.Close()

	if count != 1 {
		t.Fatalf("应只输出 1 条(ERROR),实际 %d", count)
	}
}
