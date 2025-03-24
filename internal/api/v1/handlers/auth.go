package handlers

import (
	"belajar-go/internal/config"
	"belajar-go/pkg/logger"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Auth handlers
func Register(c *fiber.Ctx) error {
	// struct RegisterRequest menerima inputan dari user
	type RegisterRequest struct {
		Username string `json:"username" validate:"required,excludesall=@?"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=6"`
	}

	// variabel req digunakan untuk menerima inputan dari user
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		logger.ErrorLogger.Error("Bad request in register", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	// Validasi dengan validator
	if err := config.Validate.Struct(req); err != nil {
		logger.AuditLogger.Warn("Validation error during register", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Validation error",
			"errors":  err.Error(),
			"success": false,
			"status":  400,
		})
	}

	// validasi email harus ada @ dan .
	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid email format",
			"success": false,
			"status":  400,
		})
	}

	// Hash the password using bcrypt with default cost
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		// Return error response if password hashing fails
		logger.ErrorLogger.Error("Error hashing password", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error hashing password",
			"success": false,
			"status":  500,
		})
	}

	// Insert data user ke dalam database
	// Jika gagal, maka akan dikembalikan response error 500
	// Jika username sudah ada, maka akan dikembalikan response error 409
	var userID int
	err = config.DB.QueryRow(
		"INSERT INTO users (username, email, password, role) VALUES ($1, $2, $3, 'member') RETURNING id",
		req.Username, req.Email, string(hashedPassword)).Scan(&userID) // Scan the generated ID into the userID variable
	if err != nil {
		// Jika error adalah unique violation error,
		// maka kita ingin mengembalikan status code 409 dengan message
		// yang mengindikasikan bahwa username sudah ada
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" {
				logger.SecurityLogger.Warn("Duplicate username", zap.String("username", req.Username))
				return c.Status(409).JSON(fiber.Map{
					"message": "Username already exists",
					"success": false,
					"status":  409,
				})
			}
		}
		logger.ErrorLogger.Error("Error creating user", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error creating user",
			"success": false,
			"status":  500,
		})
	}

	logger.AuditLogger.Info("User registered successfully", zap.Int("userID", userID))
	return c.JSON(fiber.Map{
		"message": "User created successfully",
		"status":  201,
		"data": fiber.Map{
			"id": userID,
		},
	})
}

// fungsi login dengan menggunakan JSON Web Token (JWT)
func Login(c *fiber.Ctx) error {
	// struct LoginRequest menerima inputan dari user
	type LoginRequest struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required"`
	}

	// variabel req digunakan untuk menerima inputan dari user
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		// jika inputan tidak valid, maka akan dikembalikan response error 400
		logger.ErrorLogger.Error("Bad request in login", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	if err := config.Validate.Struct(req); err != nil {
		// jika inputan tidak valid, maka akan dikembalikan response error 400
		logger.AuditLogger.Warn("Validation error during login", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Validation error",
			"errors":  err.Error(),
			"success": false,
			"status":  400,
		})
	}

	// variabel user digunakan untuk menerima data user dari database
	var user struct {
		ID       int
		Username string
		Email    string
		Password string
		Role     string
	}

	// query select digunakan untuk mengambil data user dari database
	// berdasarkan username yang dikirimkan oleh user
	err := config.DB.QueryRow(
		"SELECT id, username, email, password, role FROM users WHERE username = $1",
		req.Username).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.Role)
	if err != nil {
		// error 401, jika data user tidak ditemukan
		logger.SecurityLogger.Warn("User not found", zap.String("username", req.Username))
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid credentials",
			"success": false,
			"status":  401,
		})
	}

	// invalid password
	// user.Password -> password yang ada di database
	// req.Password -> password yang dikirimkan oleh user
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logger.SecurityLogger.Warn("Invalid password", zap.String("password", req.Password))
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid credentials",
			"success": false,
			"status":  401,
		})
	}

	// membuat token JWT dengan menggunakan secret key
	// token JWT ini akan berisi user_id, role, dan exp (expired time)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
	})

	// token JWT di encode menjadi string
	tokenString, err := token.SignedString(config.SecretKey)
	if err != nil {
		// error 500, jika terjadi error saat mengencode token
		logger.ErrorLogger.Error("Error generating token", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error generating token",
			"success": false,
			"status":  500,
		})
	}

	// kembalikan response success
	logger.AuditLogger.Info("Login success", zap.Int("user_id", user.ID), zap.String("role", user.Role))
	return c.JSON(fiber.Map{
		"message": "Login success",
		"success": true,
		"status":  200,
		"data": fiber.Map{
			"user_id": user.ID,
			"role":    user.Role,
			"token":   tokenString,
		},
	})
}
