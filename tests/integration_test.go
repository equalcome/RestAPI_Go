//go:build integration

// 測試目的：真正連 Postgres + Mongo + Redis 的端到端整合測試
// 流程：/signup → /login 拿 JWT → POST /events → GET /events (MISS→HIT) → GET /events/:id
//
//	→ PUT /events/:id → POST/DELETE /events/:id/register → DELETE /events/:id
package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restapi/middlewares"
	"restapi/models"
	"restapi/routes"
	"restapi/utils"
)

/* ---------- env & dsn ---------- */

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

type itDeps struct {
	s      *gin.Engine
	sqlDB  *sql.DB
	mgoCli *mongo.Client
	rdb    *redis.Client
}

/* ---------- boot helpers ---------- */

func waitUntil(t *testing.T, name string, f func() error, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	var last error
	for time.Now().Before(deadline) {
		if err := f(); err == nil {
			return
		} else {
			last = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s not ready: %v", name, last)
}

func applyInitSQLIfExists(t *testing.T, sqldb *sql.DB) {
	t.Helper()
	b, err := os.ReadFile("init.sql")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Log("init.sql not found; skip schema bootstrap (assume already applied)")
			return
		}
		t.Fatalf("read init.sql: %v", err)
	}
	if _, err := sqldb.Exec(string(b)); err != nil {
		t.Fatalf("apply init.sql: %v", err)
	}
}

/* ---------- server with real repos ---------- */

func newIntegrationServer(t *testing.T) itDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pg := getenv("PG_DSN", "postgres://appuser:apppass@127.0.0.1:5432/app?sslmode=disable")
	mongoURI := getenv("MONGO_URI", "mongodb://127.0.0.1:27018")
	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:6379")

	// Postgres
	sqldb, err := sql.Open("postgres", pg)
	if err != nil { t.Fatalf("sql.Open: %v", err) }
	waitUntil(t, "postgres", func() error { return sqldb.Ping() }, 30*time.Second)
	applyInitSQLIfExists(t, sqldb)

	// Mongo
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mgoCli, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil { t.Fatalf("mongo.Connect: %v", err) }
	waitUntil(t, "mongo", func() error { return mgoCli.Ping(ctx, nil) }, 30*time.Second)
	eventsCol := mgoCli.Database("app").Collection("events")

	// Redis
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	waitUntil(t, "redis", func() error {
		_, err := rdb.Ping(context.Background()).Result()
		return err
	}, 30*time.Second)

	// 實際 repos
	ur := models.NewSQLUserRepository(sqldb)
	rr := models.NewSQLRegistrationRepository(sqldb)
	er := models.NewMongoEventRepository(eventsCol)

	// 失效器 + 回應快取（跟 main 類似）
	inv := utils.NewCacheInvalidator(rdb)
	s := gin.New()
	s.Use(middlewares.ResponseCache(rdb, 30*time.Second))
	routes.RegisterRoutes(s, ur, rr, er, rdb, inv)

	return itDeps{s: s, sqlDB: sqldb, mgoCli: mgoCli, rdb: rdb}
}

/* ---------- tiny http helpers ---------- */

func req(s *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var r *strings.Reader
	if body == "" { r = strings.NewReader("") } else { r = strings.NewReader(body) }
	req := httptest.NewRequest(method, path, r)
	if body != "" { req.Header.Set("Content-Type", "application/json") }
	if token != "" { req.Header.Set("Authorization", token) }
	s.ServeHTTP(w, req)
	return w
}

/* ---------- the test ---------- */

func TestIntegration_FullFlow(t *testing.T) {
	deps := newIntegrationServer(t)
	defer func() {
		_ = deps.sqlDB.Close()
		_ = deps.mgoCli.Disconnect(context.Background())
		_ = deps.rdb.Close()
	}()

	// 1) signup
	email := "it_user_" + time.Now().Format("150405") + "@ex.com"
	w := req(deps.s, http.MethodPost, "/signup",
		`{"email":"`+email+`","password":"p"}`, "")
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("signup code=%d body=%s", w.Code, w.Body.String())
	}

	// 2) login -> token
	w = req(deps.s, http.MethodPost, "/login",
		`{"email":"`+email+`","password":"p"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", w.Code, w.Body.String())
	}
	var loginResp struct{ Token string `json:"token"` }
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	if loginResp.Token == "" { t.Fatalf("empty token") }

	// 3) 第一次 GET /events：MISS
	w = req(deps.s, http.MethodGet, "/events", "", "")
	if miss := w.Header().Get("X-Cache"); miss != "MISS" {
		t.Fatalf("expect MISS, got %q", miss)
	}

	// 4) 第二次 GET /events：HIT
	w = req(deps.s, http.MethodGet, "/events", "", "")
	if hit := w.Header().Get("X-Cache"); hit != "HIT" {
		t.Fatalf("expect HIT, got %q", hit)
	}

	// 5) 建立事件（Mongo 寫入 + 清單快取失效）
	body := `{"name":"IT Demo","description":"d","location":"L","dateTime":"2025-01-01T00:00:00Z"}`
	w = req(deps.s, http.MethodPost, "/events", body, loginResp.Token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create event code=%d body=%s", w.Code, w.Body.String())
	}
	var created struct{ Event models.Event `json:"event"` }
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Event.ID == "" { t.Fatalf("empty event id") }

	// 6) 重新 GET /events：因為剛剛清單快取被清 → 重新 MISS
	w = req(deps.s, http.MethodGet, "/events", "", "")
	if miss := w.Header().Get("X-Cache"); miss != "MISS" {
		t.Fatalf("expect MISS after create, got %q", miss)
	}

	// 7) GET /events/:id（讀單筆）
	w = req(deps.s, http.MethodGet, "/events/"+created.Event.ID, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get event by id code=%d body=%s", w.Code, w.Body.String())
	}

	// 8) 更新事件（必須為擁有者）
	upd := `{"name":"IT Demo v2","description":"changed","location":"Room 2","dateTime":"2025-01-02T03:04:05Z"}`
	w = req(deps.s, http.MethodPut, "/events/"+created.Event.ID, upd, loginResp.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("update code=%d body=%s", w.Code, w.Body.String())
	}

	// 9) 報名（寫 Postgres registrations）、重複報名衝突、取消報名
	w = req(deps.s, http.MethodPost, "/events/"+created.Event.ID+"/register", "", loginResp.Token)
	if w.Code != http.StatusCreated {
		t.Fatalf("register code=%d body=%s", w.Code, w.Body.String())
	}
	w = req(deps.s, http.MethodPost, "/events/"+created.Event.ID+"/register", "", loginResp.Token)
	if w.Code != http.StatusConflict {
		t.Fatalf("dup register want 409 got %d body=%s", w.Code, w.Body.String())
	}
	w = req(deps.s, http.MethodDelete, "/events/"+created.Event.ID+"/register", "", loginResp.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel register code=%d body=%s", w.Code, w.Body.String())
	}

	// 10) 刪除事件
	w = req(deps.s, http.MethodDelete, "/events/"+created.Event.ID, "", loginResp.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("delete event code=%d body=%s", w.Code, w.Body.String())
	}
}
