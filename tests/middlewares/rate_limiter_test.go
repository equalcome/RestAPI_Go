// 測試目的：RateLimiter（瞬時限速）
// 設 RPS=1, Burst=1；連打兩次：第 2 次 429，且帶有 Retry-After header
package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"restapi/middlewares"
)

// 設定 RPS=1, Burst=1，連打兩次同一路徑：第 1 次 200、第 2 次 429，
// 而且必須帶 Retry-After Header（瞬時尖峰限流）
func TestRateLimiter_429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := middlewares.NewRateLimiter(middlewares.LimiterConfig{
		RPS: 1, Burst: 1, IdleTTL: time.Minute,
	})

	s := gin.New()
	s.Use(rl.Middleware(func(c *gin.Context) string { return "k" })) // 固定 key
	s.GET("/x", func(c *gin.Context) { c.String(200, "ok") })

	// 第一次：200
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/x", nil)
	s.ServeHTTP(w1, req1)

	// 立刻第二次：429
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	s.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w2.Code)
	}
	if w2.Header().Get("Retry-After") == "" {
		t.Fatalf("missing Retry-After header")
	}
}
