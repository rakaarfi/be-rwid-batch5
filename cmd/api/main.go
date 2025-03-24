package main

import (
	"belajar-go/configs"
	v1 "belajar-go/internal/api/v1"
	"belajar-go/internal/middleware"
	"belajar-go/internal/repository"
	// myws "belajar-go/internal/websocket"
	"belajar-go/internal/config"
	"belajar-go/pkg/database"
	"belajar-go/pkg/logger"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	// "github.com/gofiber/websocket/v2"
	"go.uber.org/zap"
)

func main() {
	// Inisialisasi logger
	logger.InitLoggers()
	defer logger.SyncLoggers()
	logger.SystemLogger.Info("Starting application", zap.String("time", time.Now().Format(time.RFC3339)))

	// Load config
	cfg := configs.LoadConfig()
	// Inisialisasi database
	config.DB = database.ConnectDB(cfg)
	defer config.DB.Close()

	logger.SystemLogger.Info("Database Connected")

	// ----- Inisialisasi repository ----- //
	// Buat tabel jika belum ada:
	repository.CreateTableIfNotExists(config.DB)
	// Jika ingin membuat admin user:
	// repository.CreateAdminUser(config.DB)
	// Jika ingin menghapus tabel:
	// repository.DeleteAllTable(config.DB)

	// Inisialisasi Redis (Anda bisa pindahkan ke pkg/database juga jika diperlukan)
	config.RedisClient = database.ConnectRedis(cfg)
	defer config.RedisClient.Close()

	app := fiber.New()

	// Middleware
	app.Use(middleware.ErrorHandler())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
	}))

	// Daftarkan route API v1
	v1.RegisterRoutes(app)

	// // WebSocket Routes (sesuai kebutuhan)
	// hub := myws.NewHub()
	// go hub.Run()
	// app.Use("/ws", func(c *fiber.Ctx) error {
	// 	if websocket.IsWebSocketUpgrade(c) {
	// 		c.Locals("allowed", true)
	// 		return c.Next()
	// 	}
	// 	return fiber.ErrUpgradeRequired
	// })
	// app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
	// 	client := &myws.Client{Conn: c}
	// 	hub.Register <- client
	// 	defer func() {
	// 		hub.Unregister <- client
	// 	}()
	// 	for {
	// 		messageType, message, err := c.ReadMessage()
	// 		if err != nil {
	// 			break
	// 		}
	// 		if messageType == websocket.TextMessage {
	// 			hub.Broadcast <- message
	// 		}
	// 	}
	// }))

	logger.SystemLogger.Info("Application ready, listening on port 3004")
	if err := app.Listen(":3004"); err != nil {
		logger.ErrorLogger.Error("Application failed to start", zap.Error(err))
	}
}
