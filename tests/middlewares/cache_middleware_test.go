// 測試目的：ResponseCache 中介層（MISS → HIT）
// 1) 第一次 GET /events：X-Cache=MISS
// 2) 第二次 GET /events：X-Cache=HIT
package tests

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"restapi/middlewares"
)

//1st GET /events → X-Cache=MISS；
//2nd GET /events → X-Cache=HIT（命中回應快取）。
func TestResponseCache_MissThenHit(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// in-memory Redis
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// 只掛 ResponseCache，避免干擾
	s := gin.New()
	s.Use(middlewares.ResponseCache(rdb, 30*time.Second))
	s.GET("/events", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": 1})
	})

	// 第一次：MISS
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/events", nil)
	s.ServeHTTP(w1, req1)
	if w1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("want MISS, got %q", w1.Header().Get("X-Cache"))
	}

	// 第二次：HIT
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/events", nil)
	s.ServeHTTP(w2, req2)
	if w2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("want HIT, got %q", w2.Header().Get("X-Cache"))
	}
}
