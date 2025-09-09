package db

import (
	"database/sql"
	"log"

	// 1) 改用 Postgres driver
	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	var err error
	// 2) 換成 Postgres 連線字串（依你的環境調整）
	dsn := "postgres://user:pass@localhost:5432/app?sslmode=disable"
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}

	if err := DB.Ping(); err != nil {
		log.Fatal("Could not ping database:", err)
	}

	// Postgres 通常不需要像 SQLite 設 MaxOpen/Idle 這麼小，但留著也可
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)

	createTables()
}

func createTables() {
	// 3) 建 users（維持你原本 UNIQUE email 的需求）
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL
	);`
	if _, err := DB.Exec(createUsersTable); err != nil {
		log.Fatal("Could not create users table:", err)
	}

	// 4) 刪掉原本的 createEventsTable（因為 events 會改由 Mongo 管）

	// 5) 建 registrations，event_id 用 UUID，並加入複合唯一鍵避免重複報名
	createRegistrationsTable := `
	CREATE TABLE IF NOT EXISTS registrations (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id),
		event_id UUID NOT NULL,
		UNIQUE (user_id, event_id)
	);`
	if _, err := DB.Exec(createRegistrationsTable); err != nil {
		log.Fatal("Could not create registrations table:", err)
	}
}
