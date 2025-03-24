package database

import (
	"belajar-go/configs"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func ConnectDB(cfg configs.Config) *sql.DB {
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	return db
}
