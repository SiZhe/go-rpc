// Package middlewares 汇集所有"挂在中间件链上"的具体治理与可观测能力。
//
// 【和 middleware 包的区别】
//   - middleware 包:只有链引擎(Handler/Middleware/Chain 三个类型),是"骨架"。
//   - middlewares 包(本包):一个个具体中间件(超时、重试、熔断、限流、trace、metric、log),
//     是"挂件"。每个都是 middleware.Middleware 类型,可以任意组合挂到链上。
//
// 这种"引擎 + 挂件"的分离,正是 kratos / go-common 的做法:框架核心稳定不变,
// 能力靠中间件横向扩展。
package middlewares

import (
	"fmt"
	"time"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// Timeout 超时中间件:给一次调用设置最长耗时,超时则放弃并返回错误。
//
// 【为什么需要超时】
// 下游服务可能因为故障、网络问题迟迟不返回。如果客户端无限等待,请求会越积越多,
// 最终拖垮自己(连接、goroutine、内存耗尽)—— 这是雪崩的常见起点。超时是"及时止损"。
//
// 【实现原理:goroutine + select】
// Go 里给一个"同步阻塞的函数调用"加超时的经典模式:
//  1. 把 next(真正的调用)丢进一个 goroutine 执行,结果通过 channel 送回。
//  2. 主流程 select 同时等两件事:channel 有结果 / 定时器到点。谁先到用谁。
//
// 【一个诚实的说明(面试点)】
// 超时后我们"返回"了,但那个跑 next 的 goroutine 其实还在后台跑(没法强杀 goroutine)。
// 真正让它尽快结束的,是我们把 Deadline 写进了 ctx,transport 层据此设置了 socket 读写
// deadline(见 tcp.go),网络 IO 会到点报错退出。所以"超时"要真正生效,需要底层配合 ctx,
// 而不只是上层 select —— 这也是 Go context 超时传播的核心思想。
func Timeout(d time.Duration) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			// 把截止时间写进 ctx,供下层(transport)设置 socket deadline。
			c.Deadline = time.Now().Add(d)

			type result struct {
				resp proto.Message
				err  error
			}
			done := make(chan result, 1) // 缓冲 1:即使超时后 goroutine 才写,也不会泄漏阻塞

			go func() {
				resp, err := next(c, req)
				done <- result{resp, err}
			}()

			select {
			case r := <-done:
				return r.resp, r.err
			case <-time.After(d):
				return nil, fmt.Errorf("timeout: 调用 %s.%s 超过 %v", c.Service, c.Method, d)
			}
		}
	}
}
