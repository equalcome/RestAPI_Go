// main.go
package main

import (
	"context"
	"log"
	"time"

	"database/sql"

	_ "github.com/lib/pq" // Postgres driver

	"github.com/gin-gonic/gin"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restapi/models"
	"restapi/routes.go"
)

func main() {
	// ---------- 1) SQL 連線（PostgreSQL） ----------
	pgDSN := "postgres://appuser:apppass@127.0.0.1:5432/app?sslmode=disable" // TODO: 替換你的帳密/DB
	sqldb, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatal("sql.Open error:", err)
	}
	if err := sqldb.Ping(); err != nil {
		log.Fatal("Postgres ping error:", err)
	}
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(10)

	// ---------- 2) Mongo 連線 ----------
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mg, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		log.Fatal("mongo.Connect error:", err)
	}
	if err := mg.Ping(ctx, nil); err != nil {
		log.Fatal("Mongo ping error:", err)
	}
	defer func() { _ = mg.Disconnect(context.Background()) }()

	eventsCol := mg.Database("app").Collection("events")

	// ---------- 3) 建立 repositories ----------
	userRepo := models.NewSQLUserRepository(sqldb)         // users -> SQL（你上傳的實作） :contentReference[oaicite:4]{index=4}
	regRepo := models.NewSQLRegistrationRepository(sqldb)  // registrations -> SQL（你上傳的實作） :contentReference[oaicite:5]{index=5}
	eventRepo := models.NewMongoEventRepository(eventsCol) // events -> Mongo（你上傳的實作） :contentReference[oaicite:6]{index=6}

	// ---------- 4) 啟動 HTTP server 並注入 ----------
	server := gin.Default()
	routes.RegisterRoutes(server, userRepo, regRepo, eventRepo)
	if err := server.Run(":8080"); err != nil {
		log.Fatal("gin.Run error:", err)
	}
}
