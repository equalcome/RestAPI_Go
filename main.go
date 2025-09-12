package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"restapi/middlewares" // 🔥 用於 ResponseCache
	"restapi/models"
	"restapi/routes"
	"restapi/utils" // 🔥 用於 CacheInvalidator

	"github.com/redis/go-redis/v9" // 🔥 新增

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// 1) Postgres（和 docker-compose 對上）
	pgDSN := "postgres://appuser:apppass@127.0.0.1:5432/app?sslmode=disable"
	sqldb, err := sql.Open("postgres", pgDSN)
	if err != nil { log.Fatal("sql.Open error:", err) }
	if err := sqldb.Ping(); err != nil { log.Fatal("Postgres ping error:", err) }
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(10)

	// 2) Mongo（本機跑 Go，用 127.0.0.1；若改成容器內跑，改成 mongodb://mongo:27017）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 本機跑 go，用宿主機 127.0.0.1:27018 連容器
	mg, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://127.0.0.1:27018"))
	if err != nil { log.Fatal("mongo.Connect error:", err) }
	if err := mg.Ping(ctx, nil); err != nil { log.Fatal("Mongo ping error:", err) }
	defer func() { _ = mg.Disconnect(context.Background()) }()

	eventsCol := mg.Database("app").Collection("events")

	// 3) repositories
	userRepo  := models.NewSQLUserRepository(sqldb)
	regRepo   := models.NewSQLRegistrationRepository(sqldb)
	eventRepo := models.NewMongoEventRepository(eventsCol)

	// 4) Redis（快取 + 配額）
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379", // 若 Go 與 Redis 同在 docker network，改 "redis:6379"
	})

	// Cache invalidator（事件寫入後清快取用）
	inv := utils.NewCacheInvalidator(rdb)

	// 5) Gin + 中介層
	server := gin.Default()

	// 全域回應快取（僅作用於 GET；TTL 30 秒，可自行調整）
	server.Use(middlewares.ResponseCache(rdb, 30*time.Second))

	// 6) 註冊路由（把 rdb 與 inv 傳入）
	routes.RegisterRoutes(server, userRepo, regRepo, eventRepo, rdb, inv)

	if err := server.Run(":8080"); err != nil {
		log.Fatal("gin.Run error:", err)
	}
}
