package test

import (
	"belajar-go/configs"
	"belajar-go/internal/api/v1/handlers"
	"belajar-go/internal/config"
	"belajar-go/internal/middleware"
	"belajar-go/internal/repository"
	"belajar-go/pkg/database"
	"belajar-go/pkg/logger"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func connectDBTest(cfg configs.Config) *sql.DB {
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBNameTest)
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	return db
}

func TestMain(m *testing.M) {
	// Initialize logger for testing
	logger.InitLoggers()
	// Ensure all loggers are synced at the end
	defer func() {
		_ = logger.ErrorLogger.Sync()
		_ = logger.AuditLogger.Sync()
		_ = logger.RequestLogger.Sync()
		_ = logger.SecurityLogger.Sync()
		_ = logger.SystemLogger.Sync()
		_ = logger.ContextLogger.Sync()
	}()

	// Set GO_ENV to "test" so LoadConfig does not print .env logs
	os.Setenv("GO_ENV", "test")

	// Try to load .env (if exists)
	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			logger.SystemLogger.Info("No .env file found, using default values")
		} else {
			logger.SystemLogger.Info(".env file loaded from parent directory")
		}
	} else {
		logger.SystemLogger.Info(".env file loaded successfully")
	}

	// Initialize database for testing
	cfg := configs.LoadConfig()
	config.DB = connectDBTest(cfg)
	// Make sure to defer DB close
	defer config.DB.Close()

	logger.SystemLogger.Info("Database Connected")

	// Create tables if they don't exist (or reset tables for testing)
	repository.CreateTableIfNotExists(config.DB)

	// Initialize Redis client
	config.RedisClient = database.ConnectRedis(cfg)
	defer config.RedisClient.Close()

	// Run all tests
	code := m.Run()

	// Clean up: delete all tables so the database is empty after tests
	repository.DeleteAllTable(config.DB)

	// Exit with the test code
	os.Exit(code)
}

// createTestApp menginisialisasi aplikasi Fiber dengan route yang akan di-test
func CreateTestApp() *fiber.App {
	app := fiber.New()
	app.Use(middleware.ErrorHandler())
	app.Post("/register", handlers.Register)
	app.Post("/login", handlers.Login)

	// Route user (untuk endpoint user)
	userRoutes := app.Group("/users", middleware.UseToken)
	userRoutes.Get("/", handlers.GetAllUsers)
	userRoutes.Get("/:id", handlers.GetUser)
	userRoutes.Put("/:id", handlers.UpdateUser)
	userRoutes.Delete("/:id", handlers.DeleteUser)

	// Route upload (jika diperlukan)
	uploadRoutes := app.Group("/upload", middleware.UseToken)
	uploadRoutes.Post("/profile_picture", handlers.UploadProfilePicture)

	// Route task
	taskRoutes := app.Group("/tasks", middleware.UseToken)
	taskRoutes.Post("/", handlers.CreateTask)
	taskRoutes.Get("/", handlers.ListTasks)
	taskRoutes.Get("/:id", handlers.GetTask)
	taskRoutes.Put("/:id", handlers.UpdateTask)
	taskRoutes.Delete("/:id", handlers.DeleteTask)

	return app
}

// createTestAdmin secara langsung menyisipkan user admin ke database dan login untuk mendapatkan token
func CreateTestAdmin(app *fiber.App, t *testing.T) (string, int, string) {
	uniqueAdmin := fmt.Sprintf("admin_%d", time.Now().UnixNano())
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("adminpass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Error hashing admin password: %v", err)
	}
	var adminID int
	// Masukkan admin ke database dengan role 'admin'
	err = config.DB.QueryRow(
		"INSERT INTO users (username, email, password, role) VALUES ($1, $2, $3, 'admin') RETURNING id",
		uniqueAdmin, uniqueAdmin+"@example.com", string(hashedPassword),
	).Scan(&adminID)
	if err != nil {
		t.Fatalf("Error inserting admin user: %v", err)
	}

	// Login admin
	loginBody := map[string]string{
		"username": uniqueAdmin,
		"password": "adminpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error logging in admin: %v", err)
	}
	defer resp.Body.Close()

	var loginResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding admin login: %v", err)
	}
	data, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in admin login response")
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid admin token")
	}

	// Kembalikan token, adminID, dan username
	return token, adminID, uniqueAdmin
}
