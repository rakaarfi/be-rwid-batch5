package config


import (
	"context"
	"database/sql"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
)

var (
	// Global dependency yang akan digunakan di seluruh aplikasi
	DB          *sql.DB
	SecretKey   = []byte("secret")
	Validate    = validator.New()
	Ctx         = context.Background()
	RedisClient *redis.Client
)
