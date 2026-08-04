// Package log 结构化异步日志。
//
// 【对标 C++ MPRPC 的 Logger】
// C++ 侧的 Logger 是"单例 + LockQueue 异步队列":业务线程只把日志字符串塞进队列就返回,
// 由一个后台线程慢慢落盘。本包用 Go 复刻同样的思想,并升级两点:
//   1. 结构化:每条日志带 时间/级别/TraceID/字段,而非一句纯文本 —— 方便机器检索。
//   2. TraceID 关联:日志里带上 TraceID,就能和 trace、metrics 用同一个 ID 串起来。
//
// 【为什么日志要异步(面试点)】
// 写磁盘是慢 IO。如果业务线程同步写日志,每条请求都被磁盘 IO 拖慢。异步日志让业务线程
// 只做一次"塞 channel"(极快)就返回,真正的写盘交给后台 goroutine。channel 天然起到
// "缓冲 + 串行化"的作用,多个并发请求的日志排队写,不会互相错乱。
//
// 【本实现】用一个带缓冲的 channel 当队列,一个后台 goroutine 消费并输出。
// 为教学简化,输出到 stdout;换成文件只需改 sink。
package log

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "?"
	}
}

// Entry 一条结构化日志。
type Entry struct {
	Time    time.Time
	Level   Level
	TraceID string            // 关联链路的 TraceID(可空)
	Msg     string            // 主消息
	Fields  map[string]string // 结构化附加字段(如 service/method/cost)
}

// format 把一条日志格式化成一行文本(key=value 风格,便于阅读也便于日志系统解析)。
func (e Entry) format() string {
	s := fmt.Sprintf("%s [%s]", e.Time.Format("2006-01-02 15:04:05.000"), e.Level)
	if e.TraceID != "" {
		s += " trace=" + e.TraceID
	}
	s += " " + e.Msg
	for k, v := range e.Fields {
		s += fmt.Sprintf(" %s=%s", k, v)
	}
	return s
}

// Logger 异步结构化日志器。
type Logger struct {
	minLevel Level
	queue    chan Entry     // 异步队列(带缓冲)
	wg       sync.WaitGroup // 等后台 goroutine 退出,保证 Close 时日志不丢
	sink     func(string)   // 输出目的地(默认 stdout);可替换为写文件
}

// New 创建异步日志器,minLevel 以下的日志被丢弃。启动后台消费 goroutine。
func New(minLevel Level) *Logger {
	l := &Logger{
		minLevel: minLevel,
		queue:    make(chan Entry, 1024), // 缓冲 1024:突发日志不阻塞业务
		sink:     func(s string) { fmt.Fprintln(os.Stdout, s) },
	}
	l.wg.Add(1)
	go l.consume()
	return l
}

// consume 后台 goroutine:不断从队列取日志、格式化、输出。队列关闭时退出。
func (l *Logger) consume() {
	defer l.wg.Done()
	for e := range l.queue {
		l.sink(e.format())
	}
}

// log 组装并异步投递一条日志。级别不够直接丢弃。
func (l *Logger) log(level Level, traceID, msg string, fields map[string]string) {
	if level < l.minLevel {
		return
	}
	entry := Entry{Time: time.Now(), Level: level, TraceID: traceID, Msg: msg, Fields: fields}
	// 【非阻塞投递】队列满时直接丢弃这条日志,绝不阻塞业务线程 —— 日志不能拖垮主流程。
	select {
	case l.queue <- entry:
	default:
	}
}

// Infoc / Warnc / Errorc:带 TraceID 的日志方法(c = with context/trace)。
func (l *Logger) Infoc(traceID, msg string, fields map[string]string)  { l.log(INFO, traceID, msg, fields) }
func (l *Logger) Warnc(traceID, msg string, fields map[string]string)  { l.log(WARN, traceID, msg, fields) }
func (l *Logger) Errorc(traceID, msg string, fields map[string]string) { l.log(ERROR, traceID, msg, fields) }

// Close 关闭日志器:关闭队列并等后台把剩余日志写完(优雅退出,不丢日志)。
func (l *Logger) Close() {
	close(l.queue)
	l.wg.Wait()
}

// SetSink 替换输出目的地(测试用:可捕获输出做断言;生产可换成写文件)。
func (l *Logger) SetSink(fn func(string)) { l.sink = fn }
