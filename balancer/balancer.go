// Package balancer 负载均衡:从一组候选地址里挑一个来发起请求。
//
// 【这个包解决什么问题】
// 服务发现返回的是"一组"实例地址(如 3 台机器)。到底把请求发给哪一台?这就是负载均衡
// 要决定的。目标是让流量尽量均匀,避免某台机器被打爆、其它机器闲着。
//
// 【三种经典策略】
//   - 轮询 RoundRobin:1→2→3→1→2→3… 依次轮流,绝对均匀,最常用。
//   - 随机 Random:    每次随机挑一个,实现最简单,大量请求下也近似均匀。
//   - 加权轮询 Weighted:机器配置不同时按权重分配(强的机器多分)。这里实现的是
//                       "平滑加权轮询"(Nginx 同款算法),避免同一台被连续集中打。
//
// 【为什么抽象成接口】
// 和 discovery 一样,面向接口:上层只调用 Balancer.Pick(addrs),不关心内部是轮询还是随机,
// 想换策略只需换一个实现。
package balancer

import (
	"fmt"
	"math/rand"
	"sync"
)

// Balancer 负载均衡器:从候选地址里挑一个。
type Balancer interface {
	// Pick 从 addrs 中选一个地址返回。addrs 为空时返回 error。
	Pick(addrs []string) (string, error)
}

// ─────────────────────────────────────────────────────────────
// RoundRobin:轮询
// ─────────────────────────────────────────────────────────────

// RoundRobin 轮询:用一个自增计数器对地址数取模,依次轮流。
type RoundRobin struct {
	mu   sync.Mutex
	next int // 下一个要选的下标
}

func NewRoundRobin() *RoundRobin { return &RoundRobin{} }

func (b *RoundRobin) Pick(addrs []string) (string, error) {
	if len(addrs) == 0 {
		return "", fmt.Errorf("balancer: 无可用地址")
	}
	// 【为什么要加锁】next 是共享状态,多个 goroutine 并发调用 Pick 会产生数据竞争。
	// 用 Mutex 保护"读-改-写"这一组操作的原子性。
	b.mu.Lock()
	defer b.mu.Unlock()
	addr := addrs[b.next%len(addrs)]
	b.next++
	return addr, nil
}

// ─────────────────────────────────────────────────────────────
// Random:随机
// ─────────────────────────────────────────────────────────────

// Random 随机选择。无状态,天然并发安全(rand 全局函数内部有锁)。
type Random struct{}

func NewRandom() *Random { return &Random{} }

func (b *Random) Pick(addrs []string) (string, error) {
	if len(addrs) == 0 {
		return "", fmt.Errorf("balancer: 无可用地址")
	}
	return addrs[rand.Intn(len(addrs))], nil
}

// ─────────────────────────────────────────────────────────────
// Weighted:平滑加权轮询(Smooth Weighted Round-Robin,Nginx 算法)
// ─────────────────────────────────────────────────────────────

// weightedNode 一个带权重的节点。
type weightedNode struct {
	addr          string
	weight        int // 配置权重(固定不变):机器越强权重越大
	currentWeight int // 当前权重(动态变化):算法的核心状态
}

// Weighted 平滑加权轮询。
//
// 【为什么不用"简单加权"】
// 简单加权(如权重 5:1 就连发 5 次 A 再发 1 次 B)会导致流量突刺 —— A 被连续集中打 5 下。
// 平滑加权让选择结果尽量分散:权重 5:1 时序列是 A A B A A A(而不是 A A A A A B),更平滑。
//
// 【算法(每次 Pick)】
//  1. 每个节点 currentWeight += weight
//  2. 选出 currentWeight 最大的节点
//  3. 被选中节点 currentWeight -= 所有节点 weight 之和
// 这样高权重节点被选中后会被"扣分",暂时让位给别人,实现平滑。
type Weighted struct {
	mu    sync.Mutex
	nodes []*weightedNode
}

// NewWeighted 用 "地址→权重" 构造。weights 如 {"a:1":5, "b:1":1}。
func NewWeighted(weights map[string]int) *Weighted {
	nodes := make([]*weightedNode, 0, len(weights))
	for addr, w := range weights {
		nodes = append(nodes, &weightedNode{addr: addr, weight: w})
	}
	return &Weighted{nodes: nodes}
}

// Pick 忽略传入的 addrs,直接在构造时配置的加权节点里选(因为权重是预先配好的)。
func (b *Weighted) Pick(_ []string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.nodes) == 0 {
		return "", fmt.Errorf("balancer: 无可用加权节点")
	}

	total := 0
	var best *weightedNode
	for _, n := range b.nodes {
		n.currentWeight += n.weight // 1. 累加配置权重
		total += n.weight
		if best == nil || n.currentWeight > best.currentWeight { // 2. 找当前权重最大者
			best = n
		}
	}
	best.currentWeight -= total // 3. 中选者扣掉总权重
	return best.addr, nil
}
