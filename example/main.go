// Command example 演示 go-rpc 的完整调用链:把所有中间件组装起来,发起真实 TCP 调用。
//
// 运行:cd go-rpc && go run ./example
//
// 【这个 demo 展示什么】
//  1. 内嵌一个"假 MPRPC 服务端"(模拟 C++ 那边),前几次故意返回错误,用来触发重试/熔断。
//  2. 客户端组装完整中间件洋葱链:Trace → AccessLog → Metric → RateLimit → Breaker → Retry → Timeout。
//  3. 发起多次调用,观察:trace 日志(带同一 TraceID)、指标汇总、熔断/重试行为。
//
// 【中间件顺序为什么这么排(重要)】
// 洋葱从外到内的顺序决定"谁先执行":
//   Trace(最外,先生成 TraceID,后面都能用)
//    └ AccessLog(记录整个内层的总耗时和结果)
//       └ Metric(统计耗时/错误率)
//          └ RateLimit(尽早挡掉超额请求)
//             └ Breaker(下游挂了就快速失败)
//                └ Retry(在熔断放行后,对瞬时失败做重试)
//                   └ Timeout(最内,给"真正的网络调用"加超时)
//                      └ 真正的 RPC(transport 发 TCP)
// 一个原则:治理类(限流/熔断)靠外,尽早拦截;重试要在熔断内侧(重试产生的失败也计入熔断);
// 超时最贴近真实调用。
package main

import (
	"fmt"
	"net"
	"time"

	"go-rpc/balancer"
	"go-rpc/breaker"
	"go-rpc/client"
	"go-rpc/discovery"
	"go-rpc/log"
	"go-rpc/metric"
	"go-rpc/middleware"
	"go-rpc/middlewares"
	"go-rpc/ratelimit"
	"go-rpc/rpccontext"
	"go-rpc/transport"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	// ── 1. 启动一个假 MPRPC 服务端(模拟 C++ 那边)─────────────
	// 前 2 个请求返回错误(触发重试),之后正常返回 "pong"。
	addr := startFakeServer()
	time.Sleep(100 * time.Millisecond) // 等服务端就绪

	// ── 2. 准备基础设施 ───────────────────────────
	reg := discovery.NewStaticRegistry(map[string][]string{
		"UserServiceRpc/Login": {addr},
	})
	tr := transport.NewTCPTransport(reg, balancer.NewRoundRobin())

	logger := log.New(log.INFO)
	defer logger.Close()
	metrics := metric.NewCollector()
	brk := breaker.New(breaker.DefaultOptions())
	limiter := ratelimit.NewTokenBucket(100, 10) // 100 QPS,允许突发 10

	// ── 3. 组装完整中间件洋葱链 ──────────────────────
	cli := client.New(
		tr.Send, // 最内层:真实 TCP 传输(阶段一的可注入 RoundTrip,这里换成真的)
		middlewares.Trace(),
		middlewares.AccessLog(logger),
		middlewares.Metric(metrics),
		middlewares.RateLimit(limiter),
		middlewares.Breaker(brk),
		middlewares.Retry(3, 50*time.Millisecond), // 最多 3 次,退避 50ms 起
		middlewares.Timeout(2*time.Second),
	)

	// ── 4. 发起若干次调用 ────────────────────────
	fmt.Println("=== 发起 5 次调用 ===")
	for i := 1; i <= 5; i++ {
		req := wrapperspb.String(fmt.Sprintf("user-%d", i))
		var resp wrapperspb.StringValue
		err := cli.Call(rpccontext.New("UserServiceRpc", "Login"), req, &resp)
		if err != nil {
			fmt.Printf("第 %d 次调用失败: %v\n", i, err)
		} else {
			fmt.Printf("第 %d 次调用成功: %s\n", i, resp.Value)
		}
	}

	// ── 5. 打印指标汇总 ─────────────────────────
	time.Sleep(200 * time.Millisecond) // 等异步日志刷完
	fmt.Println("\n=== 指标汇总 ===")
	for _, key := range metrics.Keys() {
		s := metrics.Snapshot()[key]
		fmt.Printf("%s: 总数=%d 错误=%d 错误率=%.0f%% 平均耗时=%v\n",
			key, s.Total, s.Errors, s.ErrorRate()*100, s.AvgCost())
	}

	// 用一个 no-op 变量避免 middleware 包被判定为未使用(演示中直接用了 middlewares 包)。
	_ = middleware.Chain
}

// startFakeServer 起一个模拟 C++ MPRPC 的 TCP 服务端。前 2 次连接返回空(制造失败),
// 之后返回 StringValue("pong") 并短连接关闭。
func startFakeServer() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go func() {
		count := 0
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			count++
			buf := make([]byte, 4096)
			_, _ = conn.Read(buf)
			if count <= 2 {
				// 前两次直接关连接(客户端读到空响应 → 失败 → 触发重试)
				_ = conn.Close()
				continue
			}
			resp := wrapperspb.String("pong")
			b, _ := proto.Marshal(resp)
			_, _ = conn.Write(b)
			_ = conn.Close()
		}
	}()
	return ln.Addr().String()
}
