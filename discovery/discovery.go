// Package discovery 服务发现。
//
// 【这个包解决什么问题】
// 客户端要调用 "UserServiceRpc.Login",但它不知道这个服务部署在哪台机器、哪个端口。
// 服务发现负责把 "服务名/方法名" 翻译成一组可用的 "ip:port" 地址。
//
// 【为什么要抽象成接口】
// 真实环境用 ZooKeeper(或 etcd/consul/nacos)做服务注册中心;但单元测试时不可能起一个
// zk 集群。所以我们把"如何发现地址"抽象成 Registry 接口,提供两个实现:
//   - StaticRegistry:地址写死在内存里,给测试和本地 demo 用,零依赖。
//   - ZkRegistry:  连真正的 ZooKeeper,读 C++ MPRPC 服务端注册的节点。
// 上层(负载均衡、transport)只依赖 Registry 接口,不关心底层是 zk 还是内存 —— 这就是
// 依赖倒置(面试常问的"面向接口编程")。
package discovery

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
)

// Registry 服务注册中心的抽象:给定服务名+方法名,返回一组可用地址("ip:port")。
//
// 【为什么 key 是 service/method 两级】
// 因为 C++ MPRPC 的 zk 路径就是 "/service_name/method_name" → "ip:port"(见 MprpcProvider),
// 我们必须对齐它的注册结构,才能发现到 C++ 服务端注册的节点。
type Registry interface {
	// Discover 返回 service.method 对应的所有实例地址。找不到返回空切片 + error。
	Discover(service, method string) ([]string, error)
}

// ─────────────────────────────────────────────────────────────
// StaticRegistry:内存静态实现(测试 / 本地 demo 用)
// ─────────────────────────────────────────────────────────────

// StaticRegistry 把地址写死在内存里。不依赖任何外部组件,方便测试。
type StaticRegistry struct {
	// key 形如 "UserServiceRpc/Login",value 是该方法的实例地址列表。
	table map[string][]string
}

// NewStaticRegistry 用一张 "service/method" → 地址列表 的表构造静态注册中心。
func NewStaticRegistry(table map[string][]string) *StaticRegistry {
	return &StaticRegistry{table: table}
}

func (r *StaticRegistry) Discover(service, method string) ([]string, error) {
	key := service + "/" + method
	addrs := r.table[key]
	if len(addrs) == 0 {
		return nil, fmt.Errorf("discovery: 未找到服务实例 %q", key)
	}
	// 返回副本,避免调用方修改内部数据(防御性拷贝)。
	out := make([]string, len(addrs))
	copy(out, addrs)
	return out, nil
}

// ─────────────────────────────────────────────────────────────
// ZkRegistry:ZooKeeper 实现(真实环境,对接 C++ MPRPC)
// ─────────────────────────────────────────────────────────────

// ZkRegistry 从 ZooKeeper 读取服务地址。
//
// 【C++ MPRPC 的注册结构】
// 服务端启动时在 zk 上创建:
//   /UserServiceRpc                 (持久节点)
//   /UserServiceRpc/Login  → "127.0.0.1:8000"   (临时节点,存 ip:port)
// 所以我们读 znode "/service/method" 的 data,就是实例地址。
//
// 【注意】当前 C++ 实现每个方法只注册一个地址(单实例)。我们的 Discover 仍返回切片,
// 是为了兼容未来多实例(以及配合负载均衡)。
type ZkRegistry struct {
	conn *zk.Conn

	// 简单的本地缓存:避免每次调用都打 zk。生产级会用 watch 监听变更来刷新,
	// 这里为教学简化成"带 TTL 的缓存"。
	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	addrs    []string
	cachedAt time.Time
}

// NewZkRegistry 连接 zk 集群(servers 如 []string{"127.0.0.1:2181"})。
func NewZkRegistry(servers []string, sessionTimeout time.Duration) (*ZkRegistry, error) {
	conn, _, err := zk.Connect(servers, sessionTimeout)
	if err != nil {
		return nil, fmt.Errorf("discovery: 连接 zookeeper 失败: %w", err)
	}
	return &ZkRegistry{
		conn:  conn,
		cache: make(map[string]cacheEntry),
		ttl:   10 * time.Second,
	}, nil
}

func (r *ZkRegistry) Discover(service, method string) ([]string, error) {
	key := service + "/" + method

	// 先查缓存(读锁)。
	r.mu.RLock()
	if e, ok := r.cache[key]; ok && time.Since(e.cachedAt) < r.ttl {
		r.mu.RUnlock()
		return e.addrs, nil
	}
	r.mu.RUnlock()

	// 缓存未命中,读 zk。路径与 C++ MPRPC 对齐:"/service/method"。
	path := "/" + service + "/" + method
	data, _, err := r.conn.Get(path)
	if err != nil {
		return nil, fmt.Errorf("discovery: 读取 zk 节点 %q 失败: %w", path, err)
	}
	addrs := []string{string(data)} // C++ 侧一个节点存一个 "ip:port"

	// 写回缓存(写锁)。
	r.mu.Lock()
	r.cache[key] = cacheEntry{addrs: addrs, cachedAt: time.Now()}
	r.mu.Unlock()

	return addrs, nil
}

// Close 关闭 zk 连接。
func (r *ZkRegistry) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
}
