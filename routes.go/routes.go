// routes/routes.go
package routes

import (
	"net/http"

	"github.com/gin-gonic/gin" //interface 是「規範一組方法」的清單。
	// 任何 struct 只要實作了這些方法，就可以被視為這個 interface
	"github.com/google/uuid"

	"restapi/middlewares"
	"restapi/models"
	"restapi/utils"
)

// 依賴注入容器
type deps struct {
	users  models.UserRepository        //它們是「介面 (interface)」 interface要接收別人 **實體化的物件**就是func NewSQLUserRepository(db *sql.DB) UserRepository { return &sqlUserRepo{db} }
	regs   models.RegistrationRepository //管它內部怎樣幹的 介面的方法只要被實作就都可以用       
	events models.EventRepository			//但要把他藏起來麻所以看起來還是介面 裡面其實就是自己實體化的物件的東西
}

// 由 main 傳入各 Repository，避免在 routes 內部直接依賴特定 DB
func RegisterRoutes(server *gin.Engine, u models.UserRepository, r models.RegistrationRepository, e models.EventRepository) {
	d := &deps{users: u, regs: r, events: e}

	// 公開 endpoints
	server.GET("/events", d.getEvents)
	server.GET("/events/:id", d.getEvent)

	// 需驗證的 endpoints
	auth := server.Group("/")
	auth.Use(middlewares.Authenticate)

	auth.POST("/events", d.createEvent)
	auth.PUT("/events/:id", d.updateEvent)
	auth.DELETE("/events/:id", d.deleteEvent)

	auth.POST("/events/:id/register", d.registerForEvent)
	auth.DELETE("/events/:id/register", d.cancelRegistration)

	// Auth
	server.POST("/signup", d.signup)
	server.POST("/login", d.login)
}

/* -------------------- Events -------------------- */

// GET /events
func (d *deps) getEvents(c *gin.Context) {
	events, err := d.events.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not fetch events. Try again later."})
		return
	}
	c.JSON(http.StatusOK, events)
}

// GET /events/:id
func (d *deps) getEvent(c *gin.Context) {
	id := c.Param("id") // UUID 字串
	event, err := d.events.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not fetch event. Try again later."})
		return
	}
	c.JSON(http.StatusOK, event)
}

// POST /events
func (d *deps) createEvent(c *gin.Context) {
	var event models.Event
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not parse request data."})
		return
	}

	event.UserID = c.GetInt64("userId") // 由 middleware 注入
	if event.ID == "" {
		event.ID = uuid.NewString() // 與 SQL 的 registrations(event_id UUID) 對齊
	}

	if err := d.events.Create(&event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not create event. Try again later."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "event created!", "event": event})
}

// PUT /events/:id
func (d *deps) updateEvent(c *gin.Context) {
	id := c.Param("id")
	userId := c.GetInt64("userId")

	old, err := d.events.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not fetch the event. Try again later."})
		return
	}
	if old.UserID != userId {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authorized to update event."})
		return
	}

	var incoming models.Event
	if err := c.ShouldBindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not parse request data."})
		return
	}
	incoming.ID = id
	incoming.UserID = old.UserID

	if err := d.events.Update(&incoming); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not update event. Try again later."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Event updated successfully!"})
}

// DELETE /events/:id
func (d *deps) deleteEvent(c *gin.Context) {
	id := c.Param("id")
	userId := c.GetInt64("userId")

	ev, err := d.events.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not fetch the event. Try again later."})
		return
	}
	if ev.UserID != userId {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authorized to delete event."})
		return
	}

	// （可選）這裡可擴充：清空 registrations 中該 event 的所有報名
	// 你現在的 RegistrationRepository 只有 Register/Cancel，未提供 DeleteByEvent，
	// 如需一鍵清理可在 models 端加個方法。

	if err := d.events.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not delete the event."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Event deleted successfully!"})
}

/* --------------- Registrations ------------------ */

// POST /events/:id/register
func (d *deps) registerForEvent(c *gin.Context) {
	userId := c.GetInt64("userId")
	eventId := c.Param("id") // UUID 字串

	// 1) 先確認 event 存在（查 Mongo）
	if _, err := d.events.GetByID(eventId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not fetch event."})
		return
	}

	// 2) 寫 SQL registrations（靠 UNIQUE(user_id, event_id) 防重複）
	if err := d.regs.Register(userId, eventId); err != nil {
		c.JSON(http.StatusConflict, gin.H{"message": "Already registered or failed."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Registered!"})
}

// DELETE /events/:id/register
func (d *deps) cancelRegistration(c *gin.Context) {
	userId := c.GetInt64("userId")
	eventId := c.Param("id")

	if err := d.regs.Cancel(userId, eventId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not cancel registration."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cancelled!"})
}

/* --------------------- Auth --------------------- */

// POST /signup
func (d *deps) signup(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// 解析 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not parse request data."})
		return
	}

	// 🔹先加密密碼
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not hash password."})
		return
	}

	// 建立 User
	u := models.User{
		Email:    req.Email,
		Password: hashedPassword,
	}

	// 寫入資料庫
	if err := d.users.Create(&u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not save user."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "user created successfully"})
}

// POST /login
func (d *deps) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not parse request data."})
		return
	}

	// 建議在 UserRepository.ValidateCredentials 內部做 bcrypt 比對
	user, err := d.users.ValidateCredentials(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Could not authenticate user."})
		return
	}

	// 這裡沿用你現有的 JWT 產生邏輯
	token, err := utils.GenerateToken(user.Email, user.ID) 
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not authenticate user."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful!", "token": token})
}
