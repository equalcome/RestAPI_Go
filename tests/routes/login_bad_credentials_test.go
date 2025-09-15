// 測試目的：/login 錯誤密碼 → 401（覆蓋 routes.login 的錯誤分支）【】
package tests

import (
	"net/http"
	"testing"
)

//POST /login｜帳密錯誤 → 401（先 POST /signup 建帳，再用錯密碼登入）。
func TestLogin_BadPassword_401(t *testing.T) {
	deps := setupServerWithDeps(t)

	// 先註冊
	_ = doReq(deps.s, http.MethodPost, "/signup", `{"email":"a@b.com","password":"p"}`, "")

	// 再用錯密碼登入 → 401
	w := doReq(deps.s, http.MethodPost, "/login", `{"email":"a@b.com","password":"wrong"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d body=%s", w.Code, w.Body.String())
	}
}
