// Package breaker 熔断器(Circuit Breaker)。
//
// 【这个包解决什么问题(面试高频)】
// 当下游服务持续故障时,继续把请求发过去只会:①白白等待超时,浪费资源;②给已经
// 挂掉的下游雪上加霜。熔断器像家里的"保险丝":发现下游错误率过高时,直接"跳闸",
// 后续请求快速失败(fail-fast),不再打到下游 —— 既保护自己,也给下游恢复的机会。
//
// 【三态机(核心模型)】
//
//	          错误率超阈值
//	 Closed ───────────────▶ Open
//	 (关闭,正常放行)          (打开,直接快速失败)
//	   ▲                        │ 冷却时间到
//	   │ 探测成功                │
//	   │                        ▼
//	   └──────────────── HalfOpen
//	      探测失败,回 Open   (半开,放一个探测请求试水)
//
//   - Closed(闭合):正常状态,请求正常放行,同时用滑动窗口统计成功/失败。
//   - Open(打开):  熔断中,所有请求直接快速失败,不打下游。持续一段"冷却时间"。
//   - HalfOpen(半开):冷却结束后进入,放"一个"探测请求过去:成功→认为下游恢复了→回 Closed;
//                      失败→下游还没好→回 Open 再冷却。
//
// 【为什么用"滑动窗口"统计错误率(对齐 sentinel/kratos)】
// 简单的"累计计数"没有时间概念:服务昨天的失败不该影响今天的判断。滑动窗口只统计
// "最近 N 秒"的请求,老数据自动过期,反映的是"当下"的健康度。本实现用按秒分桶的
// 滑动窗口:把时间切成若干个 1 秒的桶,统计落在窗口内所有桶的成功/失败总数。
package breaker

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen 熔断打开时,请求被快速拒绝返回的错误。
var ErrOpen = errors.New("breaker: 熔断已打开,请求被快速拒绝")

// State 熔断器状态。
type State int

const (
	StateClosed   State = iota // 闭合(正常)
	StateOpen                  // 打开(熔断中)
	StateHalfOpen              // 半开(探测中)
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateOpen:
		return "Open"
	case StateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// Options 熔断器配置。
type Options struct {
	WindowSize   time.Duration // 滑动窗口总时长(如 10s):只统计最近这段时间
	BucketCount  int           // 窗口切成多少个桶(如 10 个,即每桶 1s)
	MinRequests  int64         // 窗口内请求数低于此值不触发熔断(样本太少不下结论,避免误判)
	ErrorRate    float64       // 错误率阈值(如 0.5 = 50%):超过则熔断
	CoolDown     time.Duration // Open 状态的冷却时长,到点后转 HalfOpen 探测
}

// DefaultOptions 一组合理的默认值。
func DefaultOptions() Options {
	return Options{
		WindowSize:  10 * time.Second,
		BucketCount: 10,
		MinRequests: 20,
		ErrorRate:   0.5,
		CoolDown:    5 * time.Second,
	}
}

// bucket 滑动窗口里的一个时间桶,记录该秒内的成功/失败数。
type bucket struct {
	start   time.Time // 这个桶代表的时间起点(用于判断是否过期)
	success int64
	failure int64
}

// Breaker 熔断器。并发安全。
type Breaker struct {
	opts Options

	mu         sync.Mutex
	state      State
	buckets    []*bucket // 环形使用的时间桶
	openedAt   time.Time // 进入 Open 的时刻,用于计算冷却是否结束
	bucketDur  time.Duration
}

// New 创建熔断器,初始为 Closed。
func New(opts Options) *Breaker {
	if opts.BucketCount <= 0 {
		opts.BucketCount = 10
	}
	return &Breaker{
		opts:      opts,
		state:     StateClosed,
		buckets:   make([]*bucket, opts.BucketCount),
		bucketDur: opts.WindowSize / time.Duration(opts.BucketCount),
	}
}

// State 返回当前状态(会先根据时间推进状态,如 Open 冷却到期自动转 HalfOpen)。
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.advance()
	return b.state
}

// Allow 判断当前是否放行请求。
//   - Closed:放行。
//   - Open:  冷却未到 → 拒绝(false);冷却到了 → 转 HalfOpen 放这一个探测请求。
//   - HalfOpen:已在探测中,拒绝其它并发请求(只允许一个探测)。
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.advance()

	switch b.state {
	case StateOpen:
		return false // 冷却期内,拒绝
	case StateHalfOpen:
		return false // 探测请求已由 advance() 触发放行,这里拒绝后续并发
	default: // Closed
		return true
	}
}

// advance 根据时间推进状态:Open 冷却到期 → HalfOpen。必须在持锁下调用。
func (b *Breaker) advance() {
	if b.state == StateOpen && time.Since(b.openedAt) >= b.opts.CoolDown {
		b.state = StateHalfOpen
	}
}

// Report 上报一次调用结果(success=true 表示成功)。熔断器据此更新统计与状态。
func (b *Breaker) Report(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateHalfOpen:
		// 半开状态下的探测结果直接决定去向。
		if success {
			b.reset() // 探测成功:下游恢复,回 Closed 并清空窗口
		} else {
			b.trip() // 探测失败:回 Open 再冷却
		}
		return
	case StateOpen:
		// Open 期间正常不会有请求进来(Allow 已拒绝),忽略。
		return
	}

	// Closed 状态:记入滑动窗口,再判断是否要熔断。
	b.currentBucket(success)
	succ, fail := b.counts()
	total := succ + fail
	if total >= b.opts.MinRequests {
		rate := float64(fail) / float64(total)
		if rate >= b.opts.ErrorRate {
			b.trip() // 错误率超阈值:跳闸熔断
		}
	}
}

// trip 转入 Open。
func (b *Breaker) trip() {
	b.state = StateOpen
	b.openedAt = time.Now()
}

// reset 转回 Closed 并清空窗口。
func (b *Breaker) reset() {
	b.state = StateClosed
	for i := range b.buckets {
		b.buckets[i] = nil
	}
}

// currentBucket 把本次结果记入"当前时间"对应的桶。若该桶已过期(属于上一轮),重置它。
func (b *Breaker) currentBucket(success bool) {
	now := time.Now()
	idx := int(now.UnixNano()/int64(b.bucketDur)) % b.opts.BucketCount
	bk := b.buckets[idx]
	// 桶为空,或桶的时间已经不在当前时间片 → 新建/重置这个桶。
	if bk == nil || now.Sub(bk.start) >= b.bucketDur {
		bk = &bucket{start: now}
		b.buckets[idx] = bk
	}
	if success {
		bk.success++
	} else {
		bk.failure++
	}
}

// counts 汇总"仍在窗口内"的所有桶的成功/失败数。过期桶不计入。
func (b *Breaker) counts() (success, failure int64) {
	now := time.Now()
	for _, bk := range b.buckets {
		if bk == nil {
			continue
		}
		if now.Sub(bk.start) < b.opts.WindowSize {
			success += bk.success
			failure += bk.failure
		}
	}
	return
}
