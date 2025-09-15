package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"restapi/models"
	"restapi/routes"
	"restapi/tests/mocks"
	"restapi/utils"
)

/* ---------- helpers ---------- */

type serverDeps struct {
	s  *gin.Engine
	ur *mocks.MockUserRepo
	rr *mocks.MockRegRepo
	er *mocks.MockEventRepo
}

func setupServerWithDeps(t *testing.T) serverDeps {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	inv := utils.NewCacheInvalidator(rdb)

	ur := &mocks.MockUserRepo{Users: map[string]models.User{}}   //介面 物件有實作丟進去
	rr := &mocks.MockRegRepo{Pairs: map[string]bool{}}           //介面 物件有實作丟進去
	er := &mocks.MockEventRepo{Items: map[string]models.Event{}} //介面 物件有實作丟進去

	s := gin.New()
	routes.RegisterRoutes(s, ur, rr, er, rdb, inv) // 會掛上 Authenticate / RateLimiter / Quota 等
	return serverDeps{s: s, ur: ur, rr: rr, er: er}
}

func authToken(t *testing.T, uid int64) string {
	t.Helper()
	// email 用不到驗證流程，只要 payload 有 userId 即可通過 middleware
	token, err := utils.GenerateToken("tester@example.com", uid)
	if err != nil {
		t.Fatalf("gen token: %v", err)
	}
	return token
}

func doReq(s *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", token) // middleware 直接讀原字串作 VerifyToken
	}
	s.ServeHTTP(w, req)
	return w
}

func TestSignupAndLogin(t *testing.T) {
	deps := setupServerWithDeps(t)
	s := deps.s

	// 測試 路由 /signup 這個Handler  // db fake //請求fake  //Handler real！！
	// signup
	// POST /signup
	w := httptest.NewRecorder() //模擬 HTTP Response  //使用者發送註冊請求
	req := httptest.NewRequest(http.MethodPost, "/signup",
		strings.NewReader(`{"email":"a@b.com","password":"p"}`)) //模擬 HTTP Request
	req.Header.Set("Content-Type", "application/json")          //模擬傳 JSON 的請求頭

	s.ServeHTTP(w, req) // 呼叫你的路由 & handler  //把這個假請求 (req) 丟進整個伺服器 (s) 處理，結果寫到假回應 (w) 裡
	if w.Code != 201 && w.Code != 200 { //回傳的 w 內容也會是 201 和 JSON (跟真實一樣 但這裡只要測試而已 所以不用json)
		t.Fatalf("signup got %d", w.Code)
	}

	// login
	// POST /login
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"a@b.com","password":"p"}`))
	req.Header.Set("Content-Type", "application/json")
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login got %d", w.Code)
	}
}

/* ---------- tests for /events ---------- */

//GET /events｜初始為空 → 200（body 為空陣列）
func TestEvents_ListEmpty(t *testing.T) {
	deps := setupServerWithDeps(t)

	// GET /events
	w := doReq(deps.s, http.MethodGet, "/events", "", "")
	if w.Code != 200 {
		t.Fatalf("GET /events code=%d body=%s", w.Code, w.Body.String())
	}
	var got []models.Event
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty list, got %d", len(got))
	}
}

//GET /events/:id｜先放一筆到 mock，再查 → 200
func TestEvents_GetByID_OK(t *testing.T) {
	deps := setupServerWithDeps(t)
	ev := models.Event{
		ID:          "e-1",
		Name:        "n1",
		Description: "d1",
		Location:    "loc",
		DateTime:    time.Now().UTC(),
		UserID:      42,
	}
	deps.er.Items[ev.ID] = ev

	// GET /events/:id
	w := doReq(deps.s, http.MethodGet, "/events/"+ev.ID, "", "")
	if w.Code != 200 {
		t.Fatalf("GET /events/:id code=%d body=%s", w.Code, w.Body.String())
	}
	var got models.Event
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != ev.ID || got.Name != ev.Name {
		t.Fatalf("mismatch: %+v", got)
	}
}

//POST /events｜帶 JWT 建立 → 201；回傳 JSON 需含 event.id（UUID）且 event.userId 應等於發 token 的 uid；並且事件已寫進 mock repo
func TestEvents_Create_OK(t *testing.T) {
	deps := setupServerWithDeps(t)
	token := authToken(t, 1001)

	// POST /events
	body := `{"name":"GoConf","description":"fun","location":"TW","dateTime":"2025-01-01T00:00:00Z"}`
	w := doReq(deps.s, http.MethodPost, "/events", body, token)
	if w.Code != 201 {
		t.Fatalf("POST /events code=%d body=%s", w.Code, w.Body.String())
	}

	// 回傳格式：{"message":"event created!","event":{...}}
	var resp struct {
		Event models.Event `json:"event"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Event.ID == "" {
		t.Fatalf("expect server to assign UUID id")
	}
	if resp.Event.UserID != 1001 {
		t.Fatalf("expect userId=1001 got %d", resp.Event.UserID)
	}
	// 確認 mock repo 也被寫入
	if _, ok := deps.er.Items[resp.Event.ID]; !ok {
		t.Fatalf("event not persisted into mock repo")
	}
}

//成功更新同一使用者的事件 → 200；
//其他使用者嘗試更新 → 401（未授權）。＊此段在檔內續篇，語意如上（對應路由授權檢查）。
func TestEvents_Update_OK_and_Unauthorized(t *testing.T) {
	deps := setupServerWithDeps(t)

	// 先準備一筆屬於 uid=7 的事件
	ownerID := int64(7)
	ev := models.Event{
		ID:          "e-7",
		Name:        "Old",
		Description: "old",
		Location:    "A",
		DateTime:    time.Now().UTC(),
		UserID:      ownerID,
	}
	deps.er.Items[ev.ID] = ev

	// 成功更新（同一 userId）
	// PUT /events/:id
	tokenOwner := authToken(t, ownerID)
	updateBody := `{"name":"NewName","description":"new","location":"B","dateTime":"2026-01-01T00:00:00Z"}`
	w := doReq(deps.s, http.MethodPut, "/events/"+ev.ID, updateBody, tokenOwner)
	if w.Code != 200 {
		t.Fatalf("PUT /events/:id code=%d body=%s", w.Code, w.Body.String())
	}
	// 檢查 mock 是否被更新
	if deps.er.Items[ev.ID].Name != "NewName" {
		t.Fatalf("expect name updated, got %+v", deps.er.Items[ev.ID])
	}

	// 不同 userId → 應 401（Not authorized to update event.）
	// PUT /events/:id (unauthorized)
	tokenOther := authToken(t, 99)
	w = doReq(deps.s, http.MethodPut, "/events/"+ev.ID, updateBody, tokenOther)
	if w.Code != 401 {
		t.Fatalf("PUT /events/:id unauthorized code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestEvents_Delete_OK_and_Unauthorized(t *testing.T) {
	deps := setupServerWithDeps(t)

	// 事件屬於 uid=11
	ownerID := int64(11)
	ev := models.Event{
		ID:          "e-del",
		Name:        "tbd",
		Description: "tbd",
		Location:    "X",
		DateTime:    time.Now().UTC(),
		UserID:      ownerID,
	}
	deps.er.Items[ev.ID] = ev

	// 擁有者刪除成功
	// DELETE /events/:id
	tokenOwner := authToken(t, ownerID)
	w := doReq(deps.s, http.MethodDelete, "/events/"+ev.ID, "", tokenOwner)
	if w.Code != 200 {
		t.Fatalf("DELETE /events/:id code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := deps.er.Items[ev.ID]; ok {
		t.Fatalf("expect event deleted from repo")
	}

	// 重新放一筆，讓非擁有者嘗試刪除 → 401
	// DELETE /events/:id (unauthorized)
	deps.er.Items[ev.ID] = ev
	tokenOther := authToken(t, 404)
	w = doReq(deps.s, http.MethodDelete, "/events/"+ev.ID, "", tokenOther)
	if w.Code != 401 {
		t.Fatalf("DELETE /events/:id unauthorized code=%d body=%s", w.Code, w.Body.String())
	}
}

// POST /events/:id/register 第一次 → 201；
// POST /events/:id/register 重複 → 409；
// DELETE /events/:id/register → 200。
func TestEvents_Register_Cancel_And_Conflict(t *testing.T) {
	deps := setupServerWithDeps(t)
	uid := int64(777)

	// 先要有事件，因為 handler 會先 GetByID 檢查事件存在
	ev := models.Event{
		ID:          "e-reg",
		Name:        "reg",
		Description: "d",
		Location:    "L",
		DateTime:    time.Now().UTC(),
		UserID:      1,
	}
	deps.er.Items[ev.ID] = ev

	token := authToken(t, uid)

	// 第一次報名成功 → 201
	// POST /events/:id/register
	w := doReq(deps.s, http.MethodPost, "/events/"+ev.ID+"/register", "", token)
	if w.Code != 201 {
		t.Fatalf("POST /events/:id/register code=%d body=%s", w.Code, w.Body.String())
	}

	// 重複報名 → 409（mock 會回 dup，handler 統一回 Conflict）
	// POST /events/:id/register (duplicate)
	w = doReq(deps.s, http.MethodPost, "/events/"+ev.ID+"/register", "", token)
	if w.Code != 409 {
		t.Fatalf("POST /events/:id/register dup code=%d body=%s", w.Code, w.Body.String())
	}

	// 取消報名 → 200
	// DELETE /events/:id/register
	w = doReq(deps.s, http.MethodDelete, "/events/"+ev.ID+"/register", "", token)
	if w.Code != 200 {
		t.Fatalf("DELETE /events/:id/register code=%d body=%s", w.Code, w.Body.String())
	}
}

/* 可選：驗證 create 後回傳的 event.ID 真的寫進 repo（不靠外部變數） */
func TestEvents_Create_WritesIntoRepo(t *testing.T) {
	deps := setupServerWithDeps(t)
	token := authToken(t, 555)

	// POST /events
	body := `{"name":"X","description":"Y","location":"Z","dateTime":"2025-01-02T03:04:05Z"}`
	w := doReq(deps.s, http.MethodPost, "/events", body, token)
	if w.Code != 201 {
		t.Fatalf("POST /events code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct{ Event models.Event `json:"event"` }
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := deps.er.Items[resp.Event.ID]; !ok {
		t.Fatalf("repo not updated with new event")
	}
}
