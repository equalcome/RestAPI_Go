// middlewares/rate_limiter.go
package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// 限速器設定
type LimiterConfig struct {
	RPS     float64       // 每秒補充多少令牌（穩態速率）
	Burst   int           // 桶子容量（允許的突發）
	IdleTTL time.Duration // key 閒置多久就自動清除
}

// 每個 key 的 limiter 與最近使用時間 //token bucket (keyLimiter)
type keyLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter 管理所有 key 的 limiter（in-memory）
type RateLimiter struct {
	conf    LimiterConfig
	mu      sync.Mutex
	buckets map[string]*keyLimiter
}

// 一個user一個key 一個key一個桶

// 建立全域 RateLimiter，並啟動背景清理
func NewRateLimiter(conf LimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		conf:    conf,
		buckets: make(map[string]*keyLimiter),
	}

	// 週期性清理閒置 key，避免記憶體累積
	go func() {
		interval := conf.IdleTTL / 2
		if interval <= 0 {
			interval = time.Minute
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			rl.mu.Lock()
			for k, v := range rl.buckets {
				if now.Sub(v.lastSeen) > rl.conf.IdleTTL {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if b, ok := rl.buckets[key]; ok {
		b.lastSeen = now
		return b.limiter    //*rate.Limiter真桶子  //Limiterr就是桶子
	}

	lim := rate.NewLimiter(rate.Limit(rl.conf.RPS), rl.conf.Burst)  //key 建一個自己的 limiter(桶子)
	rl.buckets[key] = &keyLimiter{limiter: lim, lastSeen: now}
	return lim
}

// KeySelector 讓你決定「以什麼 key 限速」（例如 IP、userId、或 userId+路徑）
type KeySelector func(c *gin.Context) string

// Middleware 回傳可掛在 Gin 的中介層
func (rl *RateLimiter) Middleware(selectKey KeySelector) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := selectKey(c) // key名字
		lim := rl.getLimiter(key) //拿到該 key 的桶子 rate.Limiter

		// 若沒有令牌，回 429
		if !lim.Allow() {
			// 附上 Retry-After，這裡簡單回 1 秒
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"message": "Too many requests. Please try again later.",
			})
			return
		}
		c.Next()
	}
}
// 算出 key。

// 拿到該 key 的桶（getLimiter）。

// lim.Allow() 拿令牌：成功 → c.Next()；失敗 → 回 429 + Retry-After。
