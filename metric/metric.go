// Package metric 指标统计:量化服务的运行状况(QPS、耗时、错误率)。
//
// 【这个包解决什么问题】
// 光有日志还不够 —— 日志是"一条条离散事件",而指标是"聚合后的数字趋势"。
// 运维要回答"现在 QPS 多少?P99 耗时多高?错误率涨了吗?",靠的是指标。
// 生产环境这些指标会被 Prometheus 抓走、在 Grafana 画成曲线、超阈值报警。
//
// 【本实现的定位】
// 自实现一个"内存指标收集器",按 服务.方法 维度累计:总请求数、错误数、总耗时。
// 由此可算出错误率、平均耗时。不引入 Prometheus 依赖,聚焦讲清"指标是怎么聚合的"。
// (生产会用 Histogram 桶来算 P99 分位;这里用平均耗时,原理相通、实现更简单。)
//
// 【为什么必须并发安全】
// 指标会被大量并发请求同时更新,所有累加操作必须加锁(或用 atomic),否则计数会丢。
package metric

import (
	"sort"
	"sync"
	"time"
)

// Stat 单个 服务.方法 的聚合指标。
type Stat struct {
	Total     int64         // 总请求数
	Errors    int64         // 失败请求数
	TotalCost time.Duration // 总耗时(用于算平均)
}

// ErrorRate 错误率 = 错误数 / 总数。无请求时返回 0。
func (s Stat) ErrorRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Errors) / float64(s.Total)
}

// AvgCost 平均耗时。无请求时返回 0。
func (s Stat) AvgCost() time.Duration {
	if s.Total == 0 {
		return 0
	}
	return s.TotalCost / time.Duration(s.Total)
}

// Collector 指标收集器,按 key(如 "UserServiceRpc.Login")聚合。并发安全。
type Collector struct {
	mu    sync.Mutex
	stats map[string]*Stat
}

func NewCollector() *Collector {
	return &Collector{stats: make(map[string]*Stat)}
}

// Observe 记录一次调用:key 是 服务.方法,cost 是耗时,isErr 表示是否失败。
func (c *Collector) Observe(key string, cost time.Duration, isErr bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.stats[key]
	if !ok {
		s = &Stat{}
		c.stats[key] = s
	}
	s.Total++
	s.TotalCost += cost
	if isErr {
		s.Errors++
	}
}

// Snapshot 返回当前所有指标的只读快照(拷贝,避免外部读时内部还在改)。
func (c *Collector) Snapshot() map[string]Stat {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]Stat, len(c.stats))
	for k, v := range c.stats {
		out[k] = *v // 值拷贝
	}
	return out
}

// Keys 返回所有已统计的 key(排序,便于稳定打印)。
func (c *Collector) Keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]string, 0, len(c.stats))
	for k := range c.stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
