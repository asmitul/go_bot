package forward

import (
	"context"
	"time"
)

// RateLimiter Token Bucket 速率限制器
// 用于控制消息发送频率，避免触发 Telegram API 限制
type RateLimiter struct {
	tokens   chan struct{} // 令牌桶
	stopCh   chan struct{} // 停止信号
	interval time.Duration // 令牌补充间隔
}

// NewRateLimiter 创建速率限制器
// ratePerSecond: 每秒允许的请求数（例如 30 表示每秒 30 个请求）
func NewRateLimiter(ratePerSecond int) *RateLimiter {
	limiter := &RateLimiter{
		tokens:   make(chan struct{}, ratePerSecond),
		stopCh:   make(chan struct{}),
		interval: time.Second / time.Duration(ratePerSecond),
	}

	// 初始填充令牌桶
	for i := 0; i < ratePerSecond; i++ {
		limiter.tokens <- struct{}{}
	}

	// 启动令牌补充 goroutine
	go limiter.refill()

	return limiter
}

// Wait 等待获取令牌（阻塞直到有可用令牌或上下文取消）
func (r *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.tokens:
		return nil
	}
}

// refill 定时补充令牌
func (r *RateLimiter) refill() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			select {
			case r.tokens <- struct{}{}:
				// 成功添加令牌
			default:
				// 令牌桶已满，跳过
			}
		}
	}
}

// Close 关闭速率限制器
func (r *RateLimiter) Close() {
	close(r.stopCh)
}
