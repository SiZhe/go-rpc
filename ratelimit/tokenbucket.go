// Package ratelimit 限流:控制单位时间内允许通过的请求数,保护系统不被突发流量打垮。
//
// 【令牌桶算法(Token Bucket,面试高频)】
// 想象一个桶,容量固定(capacity)。系统以恒定速率(rate 个/秒)往桶里放令牌,
// 桶满则溢出丢弃。每个请求来了要先从桶里拿一个令牌:拿到→放行;桶空→限流拒绝。
//
// 【令牌桶 vs 漏桶(经典对比题)】
//   - 漏桶:请求以恒定速率流出,强行平滑,不允许突发。
//   - 令牌桶:平时攒令牌,遇到突发流量时可以一次性消耗攒下的令牌(允许一定突发),
//             更贴合真实业务(短时抖动可容忍,长期速率受限)。所以令牌桶更常用。
//
// 【实现技巧:惰性计算,不用后台定时器】
// 不需要真起一个 goroutine 定时加令牌。只需记住"上次加令牌的时间",每次取令牌时,
// 根据"距离上次过了多久 × 速率"算出这期间该补多少令牌,一次性补上(不超过容量)。
// 这样零后台开销,且并发下只需一把锁。
package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket 令牌桶限流器。并发安全。
type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64   // 桶容量(最多攒多少令牌 → 决定能容忍多大突发)
	tokens     float64   // 当前令牌数(用 float64 便于按比例累加)
	rate       float64   // 每秒补充的令牌数(即长期平均限流速率)
	lastRefill time.Time // 上次补充令牌的时刻
}

// NewTokenBucket 创建令牌桶。
//   - rate:每秒允许的请求数(令牌补充速率)。
//   - capacity:桶容量(允许的突发量)。初始装满,允许启动即突发。
func NewTokenBucket(rate float64, capacity float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // 初始装满
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Allow 尝试取一个令牌:成功(放行)返回 true,桶空(限流)返回 false。
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill() // 先按流逝时间补令牌

	if tb.tokens >= 1 {
		tb.tokens-- // 取走一个
		return true
	}
	return false
}

// refill 惰性补充令牌:根据距离上次的时间差补令牌,上限为 capacity。必须在持锁下调用。
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate // 这段时间应补的令牌 = 时长 × 速率
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity // 桶满溢出
	}
	tb.lastRefill = now
}
