// routes/routes.go
package routes

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin" //interface 是「規範一組方法」的清單。
	"github.com/google/uuid"

	"restapi/middlewares"
	"restapi/models"
	"restapi/utils"
)

// 依賴注入容器
type deps struct {
	users  models.UserRepository
	regs   models.RegistrationRepository
	events models.EventRepository
}

// 由 main 傳入各 Repository，避免在 routes 內部直接依賴特定 DB
func RegisterRoutes(server *gin.Engine, u models.UserRepository, r models.RegistrationRepository, e models.EventRepository) {
	d := &deps{users: u, regs: r, events: e}

	// ===== ① 全域 IP 限速（20 rps / 40 burst）=====  每 1 秒 20 次
	globalLimiter := middlewares.NewRateLimiter(middlewares.LimiterConfig{
		RPS:     20,
		Burst:   40,
		IdleTTL: 3 * time.Minute,
	})
	
	//把這個 middleware 掛在整個 server 上，所有請求進來都會先經過它。
	server.Use(globalLimiter.Middleware(func(c *gin.Context) string {
		return "ip:" + c.ClientIP()
	}))

	// ===== ② 敏感端點限速（更嚴）：/signup、/login 以 IP 做 0.5 rps =====
	authLimiter := middlewares.NewRateLimiter(middlewares.LimiterConfig{
		RPS:     0.5, // 每 2 秒 1 次
		Burst:   2,
		IdleTTL: 10 * time.Minute,
	})
	server.POST("/signup",
		authLimiter.Middleware(func(c *gin.Context) string { return "signup:" + c.ClientIP() }),
		d.signup,
	)
	server.POST("/login",
		authLimiter.Middleware(func(c *gin.Context) string { return "login:" + c.ClientIP() }),
		d.login,
	)

	// ===== ③ 受保護群組：先驗證，再以 userId 限速 =====
	auth := server.Group("/")
	auth.Use(middlewares.Authenticate) // 會把 userId 放入 context:contentReference[oaicite:2]{index=2}

	userLimiter := middlewares.NewRateLimiter(middlewares.LimiterConfig{
		RPS:     5, // 每 1 秒 5 次
		Burst:   10,
		IdleTTL: 10 * time.Minute,
	})
	auth.Use(userLimiter.Middleware(func(c *gin.Context) string {
		return "u:" + strconv.FormatInt(c.GetInt64("userId"), 10)
	}))

	// 公開 endpoints ///events (未登入) //只有全域 IP
	server.GET("/events", d.getEvents)
	server.GET("/events/:id", d.getEvent)

	// 需驗證的 endpoints（自帶 user 限速）///events (登入後) //全域 IP + 使用者限速
	auth.POST("/events", d.createEvent)
	auth.PUT("/events/:id", d.updateEvent)
	auth.DELETE("/events/:id", d.deleteEvent)
	auth.POST("/events/:id/register", d.registerForEvent)
	auth.DELETE("/events/:id/register", d.cancelRegistration)
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

	event.UserID = c.GetInt64("userId") // 由 middleware 注入:contentReference[oaicite:3]{index=3}
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
	eventId := c.Param("id")

	if _, err := d.events.GetByID(eventId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not fetch event."})
		return
	}

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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not parse request data."})
		return
	}

	u := models.User{Email: req.Email, Password: req.Password}
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

	user, err := d.users.ValidateCredentials(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Could not authenticate user1."})
		return
	}

	token, err := utils.GenerateToken(user.Email, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not authenticate user2."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful!", "token": token})
}
