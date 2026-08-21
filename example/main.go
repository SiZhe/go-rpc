// Command example 演示 go-rpc 完整调用链(含 context 超时、连接池、per-node 熔断、
// retry jitter/幂等、trace/metric/log 联动)。
//
// 运行:cd go-rpc && go run ./example
//
// 【中间件顺序(洋葱从外到内)】
//
//	Trace → AccessLog → Metric → RateLimit → Retry → NodeBreaker.Middleware → Timeout → 真正调用
//	- Trace 最外:先生成 TraceID,后续都能用。
//	- RateLimit / 熔断靠外:尽早拦截。
//	- Retry 在熔断内侧:重试的失败也计入熔断。
//	- NodeBreaker.Middleware 需在 transport 选好节点后上报,故靠内。
//	- Timeout 最内:用 context.WithTimeout 派生 ctx,取消信号透传到网络层。
//
// 节点摘除由 NodeBreaker.Filter 在 transport 选址前完成(不在洋葱链上)。
package main

import (
	"context"
	"fmt"
	"time"

	"go-rpc/balancer"
	"go-rpc/breaker"
	"go-rpc/client"
	"go-rpc/discovery"
	"go-rpc/log"
	"go-rpc/metric"
	"go-rpc/middlewares"
	"go-rpc/ratelimit"
	"go-rpc/rpccontext"
	"go-rpc/transport"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	// ── 1. 两个假服务端:goodAddr 正常,badAddr 永远失败(演示 per-node 熔断摘除)──
	goodAddr := startServer(false)
	badAddr := startServer(true)
	time.Sleep(100 * time.Millisecond)

	// ── 2. 基础设施 ────────────────────────────
	reg := discovery.NewStaticRegistry(map[string][]string{
		// 两个节点都注册:一个好一个坏,看熔断能否把坏的摘掉。
		"UserServiceRpc/Login": {goodAddr, badAddr},
	})

	logger := log.New(log.INFO)
	defer logger.Close()
	metrics := metric.NewCollector()

	// per-node 熔断器:坏节点连续失败后会被摘除。
	nodeBreaker := middlewares.NewNodeBreaker(breaker.Options{
		WindowSize: time.Second, BucketCount: 10,
		MinRequests: 3, ErrorRate: 0.5, CoolDown: 5 * time.Second,
	})

	// transport 注入 NodeFilter:选址前剔除熔断节点。
	tr := transport.NewTCPTransport(reg, balancer.NewRoundRobin(),
		transport.WithNodeFilter(nodeBreaker.Filter))
	defer tr.Close()

	limiter := ratelimit.NewTokenBucket(1000, 50)

	// 幂等判定:只有 Get/Query/List 才重试(演示;Login 这里当作可重试)。
	idempotent := func(ctx context.Context) bool {
		switch rpccontext.Method(ctx) {
		case "Login", "Get", "Query", "List":
			return true
		default:
			return false
		}
	}

	// ── 3. 组装完整中间件洋葱链 ──────────────────
	cli := client.New(
		tr.Send,
		middlewares.Trace(),
		middlewares.AccessLog(logger),
		middlewares.Metric(metrics),
		middlewares.RateLimit(limiter),
		middlewares.Retry(middlewares.RetryOptions{
			MaxAttempts: 3, BaseDelay: 20 * time.Millisecond,
			MaxDelay: 200 * time.Millisecond, Idempotent: idempotent,
		}),
		nodeBreaker.Middleware(),
		middlewares.Timeout(500*time.Millisecond),
	)

	// ── 4. 发起多次调用:轮询会交替打到 good/bad,bad 连续失败后被摘除 ──
	fmt.Println("=== 发起 10 次调用(good/bad 两节点轮询,bad 会被熔断摘除)===")
	for i := 1; i <= 10; i++ {
		req := wrapperspb.String(fmt.Sprintf("user-%d", i))
		var resp wrapperspb.StringValue
		// 每次调用用 context.WithTimeout 派生:演示超时/取消传播。
		ctx, cancel := context.WithTimeout(
			rpccontext.New(context.Background(), "UserServiceRpc", "Login"),
			2*time.Second)
		err := cli.Call(ctx, req, &resp)
		cancel()
		if err != nil {
			fmt.Printf("第 %2d 次: 失败 %v\n", i, err)
		} else {
			fmt.Printf("第 %2d 次: 成功 %s\n", i, resp.Value)
		}
		time.Sleep(30 * time.Millisecond)
	}

	// ── 5. 指标汇总 ──────────────────────────
	time.Sleep(200 * time.Millisecond)
	fmt.Println("\n=== 指标汇总 ===")
	for _, key := range metrics.Keys() {
		s := metrics.Snapshot()[key]
		fmt.Printf("%s: 总数=%d 错误=%d 错误率=%.0f%% 平均耗时=%v\n",
			key, s.Total, s.Errors, s.ErrorRate()*100, s.AvgCost())
	}
	fmt.Printf("\n(good=%s bad=%s;后半程 bad 被熔断摘除,请求应集中打到 good)\n", goodAddr, badAddr)
}
