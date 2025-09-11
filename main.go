package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restapi/models"
	"restapi/routes" // ✅ 用資料夾名，不是 routes.go
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

    // 3) repositories（假設這三個建構子都存在）
    userRepo  := models.NewSQLUserRepository(sqldb)
    regRepo   := models.NewSQLRegistrationRepository(sqldb)
    eventRepo := models.NewMongoEventRepository(eventsCol)

    // 4) routes
    server := gin.Default()
    routes.RegisterRoutes(server, userRepo, regRepo, eventRepo)
    if err := server.Run(":8080"); err != nil {
        log.Fatal("gin.Run error:", err)
    }
}
