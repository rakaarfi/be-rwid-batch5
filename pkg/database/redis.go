package database

import (
	"belajar-go/configs"
	"belajar-go/internal/config"
	"belajar-go/pkg/logger"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func ConnectRedis(cfg configs.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: "",
		DB:       0,
	})
	if err := client.Ping(config.Ctx).Err(); err != nil {
		logger.ErrorLogger.Error("Redis connection error", zap.Error(err))
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	return client
}
