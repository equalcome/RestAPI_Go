// 測試目的：Quota（每日配額）中介層
// 限制 Limit=2，連打 3 次：前兩次 200，第三次 429
package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"restapi/middlewares"
)

//設定 Limit=2、Window=1h；同一使用者連打 3 次：前兩次 200，第 3 次 429（長期用量控管）。
func TestQuota_Exceed429(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	s := gin.New()
	// 模擬已驗證：塞 userId 進 context
	s.Use(func(c *gin.Context) { c.Set("userId", int64(7)); c.Next() })
	s.Use(middlewares.Quota(rdb, middlewares.QuotaRule{
		Limit:  2,
		Window: time.Hour,
		KeyFn: func(c *gin.Context) string {
			return fmt.Sprintf("quota:user:%d:day", c.GetInt64("userId"))
		},
	}))
	s.GET("/x", func(c *gin.Context) { c.String(200, "ok") })

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		s.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("unexpected %d", w.Code)
		}
	}

	// 第 3 次超限 → 429
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	s.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Fatalf("want 429, got %d; body=%s", w.Code, w.Body.String())
	}
}
