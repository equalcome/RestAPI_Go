// 測試目的：ResponseCache 與 CacheInvalidator 串接的完整流程
// 流程：GET /events (MISS) → GET /events (HIT) → POST /events 建立事件（觸發清單快取失效） → 再 GET /events (MISS)
package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"restapi/middlewares"
	"restapi/models"
	"restapi/routes"
	"restapi/tests/mocks"
	"restapi/utils"
)

// GET /events → MISS；
// 再 GET → HIT；
// POST /events（帶 JWT）→ 觸發路由內清單快取失效；
// 再 GET → 回到 MISS（驗證「寫入後清單失效」的整體閉環）。
func TestCache_MissHitThenInvalidatedByCreate(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// 起 in-memory Redis
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()}) // ✅ 修正這行：括號結尾

	// 失效器
	inv := utils.NewCacheInvalidator(rdb)

	// 用 mock repos（你已在 tests/mocks.go 寫好）:contentReference[oaicite:0]{index=0}
	ur := &mocks.MockUserRepo{Users: map[string]models.User{}}
	rr := &mocks.MockRegRepo{Pairs: map[string]bool{}}
	er := &mocks.MockEventRepo{Items: map[string]models.Event{}}

	// 先掛 ResponseCache（模擬 main.go 的全域快取），再註冊路由
	s := gin.New()
	s.Use(middlewares.ResponseCache(rdb, 30*time.Second))
	routes.RegisterRoutes(s, ur, rr, er, rdb, inv)

	// 1) 第一次 GET：MISS
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	s.ServeHTTP(w, req)
	if w.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("want MISS, got %q", w.Header().Get("X-Cache"))
	}

	// 2) 第二次 GET：HIT（命中回應快取）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/events", nil)
	s.ServeHTTP(w, req)
	if w.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("want HIT, got %q", w.Header().Get("X-Cache"))
	}

	// 3) 建立事件 → 路由內會呼叫 CacheInvalidator 清掉列表快取
	token, err := utils.GenerateToken("u@x.com", 1)
	if err != nil {
		t.Fatalf("gen token: %v", err)
	}
	body := `{"name":"N","description":"D","location":"L","dateTime":"2025-01-01T00:00:00Z"}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token) // Authenticate 會從這裡驗證
	s.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create code=%d body=%s", w.Code, w.Body.String())
	}

	// 4) 再 GET：由於剛剛清單快取被清 → 應該回 MISS
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/events", nil)
	s.ServeHTTP(w, req)
	if w.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("want MISS after invalidation, got %q", w.Header().Get("X-Cache"))
	}
}
