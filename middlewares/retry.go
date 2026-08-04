package middlewares

import (
	"time"

	"go-rpc/middleware"
	"go-rpc/rpccontext"
	"google.golang.org/protobuf/proto"
)

// Retry 重试中间件:调用失败时,按指数退避重试若干次。
//
// 【为什么需要重试】
// 网络抖动、对端瞬时过载导致的失败往往是"暂时的",过一小会儿再试大概率能成功。
// 重试能显著提升最终成功率,是提升可用性的常用手段。
//
// 【为什么用"指数退避"而不是立刻重试】
// 如果失败后立刻猛重试,而对端正因为过载才失败,你的重试反而是火上浇油(重试风暴)。
// 指数退避让每次重试的等待时间翻倍(如 100ms → 200ms → 400ms),给对端喘息恢复的时间。
//
// 【重要前提:幂等性(面试必问)】
// 重试只适合"幂等"操作 —— 即执行多次和执行一次效果相同(如查询)。
// 对"扣款""下单"这类非幂等操作,盲目重试可能导致重复扣款/重复下单。
// 生产框架会区分方法是否幂等;本实现为教学默认对所有失败重试,使用时需自行确保幂等。
//
// 参数:
//   - maxAttempts:总尝试次数(含首次)。如 3 表示"首次 + 最多 2 次重试"。
//   - baseDelay:  首次重试前的等待,之后每次翻倍。
func Retry(maxAttempts int, baseDelay time.Duration) middleware.Middleware {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(c *rpccontext.RpcContext, req proto.Message) (proto.Message, error) {
			var lastErr error
			delay := baseDelay
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				resp, err := next(c, req)
				if err == nil {
					return resp, nil // 成功,立即返回
				}
				lastErr = err
				// 最后一次失败就不再等待,直接跳出返回错误。
				if attempt < maxAttempts {
					time.Sleep(delay)
					delay *= 2 // 指数退避:等待时间翻倍
				}
			}
			return nil, lastErr // 用尽次数,返回最后一次的错误
		}
	}
}
