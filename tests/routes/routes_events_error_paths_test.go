// 測試目的：路由錯誤分支（getEvents 內部錯誤、registerForEvent 找不到事件）
// 直接用臨時 stub 實作 interface 來回傳錯誤【路由錯誤分支：】
package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"restapi/models"
	"restapi/routes"
	"restapi/tests/mocks"
	"restapi/utils"
)

// 讓 GetAll() 回錯
type failingEventRepo struct{ models.EventRepository }
func (f failingEventRepo) GetAll() ([]models.Event, error) { return nil, errors.New("boom") }

// 讓 GetByID() 回錯
type nfEventRepo struct{ models.EventRepository }
func (nf nfEventRepo) GetByID(id string) (models.Event, error) { return models.Event{}, errors.New("nf") }

func setupWithRepos(t *testing.T, er models.EventRepository, ur models.UserRepository, rr models.RegistrationRepository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t); t.Cleanup(func(){ mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	inv := utils.NewCacheInvalidator(rdb)
	if ur == nil { ur = &mocks.MockUserRepo{Users: map[string]models.User{}} }
	if rr == nil { rr = &mocks.MockRegRepo{Pairs: map[string]bool{}} }
	s := gin.New()
	routes.RegisterRoutes(s, ur, rr, er, rdb, inv)
	return s
}

//GET /events｜GetAll() 故意回錯 → 500。
func TestGetEvents_InternalError_500(t *testing.T) {
	s := setupWithRepos(t, failingEventRepo{}, &mocks.MockUserRepo{Users: map[string]models.User{}}, &mocks.MockRegRepo{Pairs: map[string]bool{}})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	s.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d body=%s", w.Code, w.Body.String())
	}
}

//POST /events/:id/register｜GetByID() 失敗 → 500（帶有效 JWT）
func TestRegister_NotFoundEvent_500(t *testing.T) {
	// 準備一個有效的 JWT（uid=1）
	token, _ := utils.GenerateToken("x@x.com", 1)

	s := setupWithRepos(t, nfEventRepo{}, &mocks.MockUserRepo{Users: map[string]models.User{}}, &mocks.MockRegRepo{Pairs: map[string]bool{}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/does-not-exist/register", strings.NewReader(""))
	req.Header.Set("Authorization", token)
	s.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d body=%s", w.Code, w.Body.String())
	}
}
