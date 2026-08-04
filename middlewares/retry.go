package middlewares

import (
	"context"
	"math/rand"
	"time"

	"go-rpc/middleware"
	"google.golang.org/protobuf/proto"
)

// IdempotentFunc 判定某次调用是否幂等(可安全重试)。返回 false 则不重试。
type IdempotentFunc func(ctx context.Context) bool

// RetryOptions 重试配置。
type RetryOptions struct {
	MaxAttempts int           // 总尝试次数(含首次)
	BaseDelay   time.Duration // 首次重试前的基础等待,之后指数增长
	MaxDelay    time.Duration // 退避上限,避免等待过久
	// Idempotent 判定是否可重试。为 nil 时默认"全部可重试"(教学默认;生产应显式指定)。
	Idempotent IdempotentFunc
}

// Retry 重试中间件:失败时按"指数退避 + 随机抖动"重试;仅对幂等调用重试。
//
// 【修复点 1:jitter 随机抖动(避免重试风暴/惊群)】
// 纯指数退避下,大量客户端可能在同一时刻一起失败、又在同一时刻一起重试,形成周期性
// 流量尖峰(惊群)。给每次退避加一个随机抖动,把重试时间打散,削平尖峰。
// 本实现用 "全抖动(full jitter)":实际等待 = random(0, 指数退避值),AWS 推荐做法之一。
//
// 【修复点 2:幂等控制(避免重复副作用)】
// 只有幂等操作(多次执行==一次执行,如查询)才能安全重试。对"扣款/下单"等非幂等操作
// 盲目重试会导致重复扣款。通过 Idempotent 判定函数,让调用方声明哪些方法可重试。
//
// 【修复点 3:尊重 context 取消】
// 每次重试等待期间监听 ctx.Done():若整体已超时/被取消,立即停止重试并返回,不做无谓等待。
func Retry(opts RetryOptions) middleware.Middleware {
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 10 * time.Second
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req proto.Message) (proto.Message, error) {
			// 非幂等则不重试,只调一次。
			if opts.Idempotent != nil && !opts.Idempotent(ctx) {
				return next(ctx, req)
			}

			var lastErr error
			backoff := opts.BaseDelay
			for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
				resp, err := next(ctx, req)
				if err == nil {
					return resp, nil
				}
				lastErr = err
				if attempt == opts.MaxAttempts {
					break // 最后一次,不再等待
				}

				// 计算本次退避(带上限),再取全抖动:random(0, backoff)。
				if backoff > opts.MaxDelay {
					backoff = opts.MaxDelay
				}
				sleep := time.Duration(rand.Int63n(int64(backoff) + 1))

				// 等待期间尊重 ctx 取消/超时。
				select {
				case <-time.After(sleep):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				backoff *= 2 // 指数增长
			}
			return nil, lastErr
		}
	}
}

// 说明:如需按方法名判定幂等,可这样写 Idempotent:
//   func(ctx context.Context) bool {
//       switch rpccontext.Method(ctx) {
//       case "Get", "Query", "List": return true
//       default: return false
//       }
//   }
