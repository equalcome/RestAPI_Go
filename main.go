package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"restapi/middlewares"
	"restapi/models"
	"restapi/routes"
	"restapi/utils"

	"github.com/redis/go-redis/v9"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Postgres
	pgDSN := os.Getenv("PG_DSN")
	if pgDSN == "" {
		pgDSN = "postgres://appuser:apppass@127.0.0.1:5432/app?sslmode=disable"
	}
	sqldb, err := sql.Open("postgres", pgDSN)
	if err != nil { log.Fatal("sql.Open error:", err) }
	if err := sqldb.Ping(); err != nil { log.Fatal("Postgres ping error:", err) }
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(10)

	// Mongo
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://127.0.0.1:27018"
	}
	mg, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI)) // ← 用變數，不要加引號
	if err != nil { log.Fatal("mongo.Connect error:", err) }
	if err := mg.Ping(ctx, nil); err != nil { log.Fatal("Mongo ping error:", err) }
	defer func() { _ = mg.Disconnect(context.Background()) }()

	eventsCol := mg.Database("app").Collection("events")

	// Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr, // ← 用變數，不要加引號
	})

	// Cache invalidator
	inv := utils.NewCacheInvalidator(rdb)

	// Gin + middlewares
	server := gin.Default()
	server.Use(middlewares.ResponseCache(rdb, 30*time.Second))

	// Routes
	routes.RegisterRoutes(server, 
		models.NewSQLUserRepository(sqldb), 
		models.NewSQLRegistrationRepository(sqldb), 
		models.NewMongoEventRepository(eventsCol), 
		rdb, inv)

	if err := server.Run(":8080"); err != nil {
		log.Fatal("gin.Run error:", err)
	}
}
