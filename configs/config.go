package configs

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBNameTest string
	RedisHost  string
	RedisPort  int
}

func LoadConfig() Config {
	// Muat file .env
	if err := godotenv.Load(); err != nil {
		// Hanya log jika tidak dalam mode test
		if os.Getenv("GO_ENV") != "test" {
			log.Println("No .env file found, using default values")
		}
	}

	dbPort, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		dbPort = 10501
	}

	redisPort, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		redisPort = 6379
	}

	return Config{
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     dbPort,
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBNameTest: os.Getenv("DB_NAME_TEST"),
		RedisHost:  os.Getenv("REDIS_HOST"),
		RedisPort:  redisPort,
	}
}
