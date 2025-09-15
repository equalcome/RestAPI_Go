// 測試目的：路由錯誤分支（容易漏測、但覆蓋率高）
// 1) POST /events：壞 JSON → 400
// 2) PUT /events/:id：找不到事件（GetByID 失敗）→ 500
// 3) GET /events/:id：不存在 → 500
package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"restapi/models"
)

//POST /events｜壞 JSON → 400
func TestCreateEvent_BadJSON_400(t *testing.T) {
	deps := setupServerWithDeps(t)
	token := authToken(t, 1)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events",
		strings.NewReader(`{ bad json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	deps.s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

//PUT /events/:id｜GetByID 失敗（不存在）→ 500
func TestUpdateEvent_NotFound_500(t *testing.T) {
	deps := setupServerWithDeps(t)
	token := authToken(t, 1)

	// 沒有預先放任何事件 → GetByID 會失敗（mock 回 nf）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/events/does-not-exist",
		strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	deps.s.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d; body=%s", w.Code, w.Body.String())
	}
}

//GET /events/:id｜不存在 → 500；對照測：放入一筆後再查 → 200
func TestGetEvent_NotFound_500(t *testing.T) {
	deps := setupServerWithDeps(t)

	// 也未放該 id → 500
	w := doReq(deps.s, http.MethodGet, "/events/nope", "", "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d; body=%s", w.Code, w.Body.String())
	}

	// 對照：放入一筆就會 200
	ev := models.Event{ID: "ok"}
	deps.er.Items["ok"] = ev
	w = doReq(deps.s, http.MethodGet, "/events/ok", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d; body=%s", w.Code, w.Body.String())
	}
}
