// 測試目的：Authenticate 中介層（缺少/無效 Token → 401）
// 覆蓋 middlewares/auth.go 與 utils.VerifyToken 的錯誤支線【routes 用到 Authenticate：】【JWT 驗證：】
package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"restapi/middlewares"

	"github.com/gin-gonic/gin"
)

//沒帶 Authorization → 應回 401
func TestAuthMiddleware_MissingToken_401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.Authenticate)
	r.GET("/p", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil) // 沒帶 Authorization
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

//無效字串作為 token → 應回 401。涵蓋 utils.VerifyToken 的錯誤分支
func TestAuthMiddleware_InvalidToken_401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.Authenticate)
	r.GET("/p", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.Header.Set("Authorization", "this-is-not-a-jwt") // 無效字串
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
