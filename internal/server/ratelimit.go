package server

import (
	"context"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
)

// 基础令牌桶
type tokenBucket struct {
	rate       float64 // tokens per second
	capacity   float64
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rps float64) *tokenBucket {
	if rps <= 0 {
		rps = 1
	}
	b := &tokenBucket{
		rate:       rps,
		capacity:   rps * 2,
		tokens:     rps * 2,
		lastRefill: time.Now(),
	}
	return b
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	delta := now.Sub(b.lastRefill).Seconds()
	b.tokens = minFloat(b.capacity, b.tokens+delta*b.rate)
	if b.tokens >= 1 {
		b.tokens -= 1
		b.lastRefill = now
		return true
	}
	b.lastRefill = now
	return false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// limiterMiddleware 将限流应用到 HTTP 请求
func limiterMiddleware(b *tokenBucket) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if !b.allow() {
				return nil, errors.New(429, "RATE_LIMIT", "rate limit exceeded")
			}
			return next(ctx, req)
		}
	}
}
