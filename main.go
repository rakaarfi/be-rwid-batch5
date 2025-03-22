package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/websocket/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lib/pq"

	// _ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	db          *sql.DB
	secretKey   = []byte("secret")
	validate    = validator.New()
	ctx         = context.Background()
	redisClient *redis.Client
)

type User struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	Email          string         `json:"email"`
	Role           string         `json:"role"`
	ProfilePicture sql.NullString `json:"profile_picture"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// users variable for storing user data
// var users = []User{}

type Task struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	SecurityCode string    `json:"security_code,omitempty"` // omitempty adalah tag untuk menghilangkan field jika nil
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// tasks variable for storing task data
// var tasks = []Task{}

const (
	host        = "103.172.204.237"
	port        = 10501
	user        = "postgres"
	password    = "qr6AByxRbcSTLnKhLDpMlmNBu65x8ujlfiLoLZQrTD6xKjugEly4BHcV95JhTuL3"
	dbname      = "postgres"
	dbname_test = "postgres_test"
)

// ======= WebSocket Integration =======

// Client represents a WebSocket client
type Client struct {
	Conn *websocket.Conn
	Mu   sync.Mutex
}

// Hub manages WebSocket connections
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Conn.Close()
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				client.Mu.Lock()
				err := client.Conn.WriteMessage(websocket.TextMessage, message)
				client.Mu.Unlock()
				if err != nil {
					h.unregister <- client
				}
			}
		}
	}
}

// ======= End WebSocket Integration =======

func main() {
	var err error
	// ------------------------------- //
	// Connect to database //
	// ------------------------------- //
	// connection string
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	// open database
	db, err = sql.Open("postgres", psqlconn)
	CheckError(err)

	// Set database connection pool size
	db.SetMaxOpenConns(25) // Maximum number of open connections
	db.SetMaxIdleConns(5)  // Maximum number of idle connections
	db.SetConnMaxLifetime(time.Hour) // Maximum lifetime of a connection

	// close database
	// .Close() digunakan untuk menutup koneksi setelah selesai digunakan
	// hal ini perlu dilakukan agar koneksi tidak terbuka terus menerus
	// dan agar database tidak mengalami overload
	defer db.Close()

	// check db
	err = db.Ping()
	CheckError(err)

	fmt.Println("Database Connected!")

	// create table if not exists and create admin user
	createTableIfNotExists(db)
	// createAdminUser(db)

	// delete table if needed
	// deteleAllTable(db)

	// ------------------------------- //
	// Inisialisasi Redis client //
	// ------------------------------- //
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // pastikan alamat ini sesuai dengan konfigurasi Redis Anda
		Password: "",               // kosong jika tidak ada password
		DB:       0,                // gunakan DB default
	})

	// Cek koneksi Redis
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	fmt.Println("Redis Connected!")

	app := fiber.New()

	// Activate error handler
	app.Use(ErrorHandler())

	// 🔹 Middleware CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Bisa diatur dengan domain tertentu misalnya: "http://localhost:3000"
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// 🔹 Middleware Rate Limiter
	app.Use(limiter.New(limiter.Config{
		Max:        100,             // Maksimal 100 request per menit
		Expiration: 1 * time.Minute, // Reset kuota tiap 1 menit
	}))

	// Auth routes
	app.Post("/login", login)
	app.Post("/register", register)

	// User routes
	userRoutes := app.Group("/users", useToken)
	userRoutes.Get("/", getAllUsers)
	userRoutes.Get("/:id", getUser)
	userRoutes.Put("/:id", updateUser)
	userRoutes.Delete("/:id", deleteUser)

	// Task routes
	taskRoutes := app.Group("/tasks", useToken)
	taskRoutes.Post("/", createTask)
	taskRoutes.Get("/", listTasks)
	taskRoutes.Get("/:id", getTask)
	taskRoutes.Put("/:id", updateTask)
	taskRoutes.Delete("/:id", deleteTask)

	// Upload routes
	uploadRoutes := app.Group("/upload", useToken)
	uploadRoutes.Post("/", uploadFile)
	uploadRoutes.Get("/:filename", getFile)
	uploadRoutes.Post("/profile_picture", uploadProfilePicture)

	// ------------------------------- //
	// WebSocket Routes              //
	// ------------------------------- //
	hub := newHub()
	go hub.run()

	// Middleware untuk WebSocket upgrade
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Endpoint WebSocket
	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		client := &Client{Conn: c}
		hub.register <- client

		defer func() {
			hub.unregister <- client
		}()

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			if messageType == websocket.TextMessage {
				hub.broadcast <- message
			}
		}
	}))

	// Test panic error
	app.Get("/test-panic", func(c *fiber.Ctx) error {
		panic("Simulasi panic error untuk uji ErrorHandler")
	})

	app.Listen(":3004")
}

func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

func createTableIfNotExists(db *sql.DB) {
	query := `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(255) NOT NULL,
    profile_picture VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
        id SERIAL PRIMARY KEY,
        user_id INT NOT NULL REFERENCES users (id),
        title VARCHAR(255) NOT NULL,
        description TEXT,
        status VARCHAR(255) NOT NULL,
        security_code TEXT,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    `

	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Error creating table: %v", err)
	} else {
		fmt.Println("Table 'data', 'tasks', 'users' are ready.")
	}
}

func createAdminUser(db *sql.DB) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}

	// Insert admin user
	query := "INSERT INTO users (username, email, password, role) VALUES ($1, $2, $3, $4)"
	_, err = db.Exec(query, "admin", "admin@mail.com", string(hashedPassword), "admin")
	if err != nil {
		log.Fatalf("Error inserting admin user: %v", err)
	} else {
		fmt.Println("Admin user 'admin' is created.")
	}
}

func deteleAllTable(db *sql.DB) {
	query := `
    DROP TABLE IF EXISTS tasks;
    DROP TABLE IF EXISTS users;
    `

	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Error deleting table: %v", err)
	} else {
		fmt.Println("Table 'data', 'tasks', 'users' are deleted.")
	}
}

func encrypt(data string, key string) (string, error) {
	key = fixEncryptionKey(key)

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	plaintext := []byte(data)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(data string, key string) (string, error) {
	key = fixEncryptionKey(key)

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	ciphertext, _ := base64.StdEncoding.DecodeString(data)
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

func fixEncryptionKey(key string) string {
	if len(key) < 32 {
		return key + strings.Repeat("0", 32-len(key))
	}
	return key[:32]
}

func useToken(c *fiber.Ctx) error {
	// Ambil token dari header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{
			"message": "No token provided",
			"success": false,
			"status":  401,
		})
	}

	// Split token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid token format",
			"success": false,
			"status":  401,
		})
	}

	// Parse token
	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil || !token.Valid {
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid token",
			"success": false,
			"status":  401,
		})
	}

	// Ambil claims dari token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid token claims",
			"success": false,
			"status":  401,
		})
	}

	// Cek apakah token sudah expired
	if exp, ok := claims["exp"].(float64); !ok || int64(exp) < time.Now().Unix() {
		return c.Status(401).JSON(fiber.Map{
			"message": "Token expired",
			"success": false,
			"status":  401,
		})
	}

	// Ambil user_id dan role dari claims
	userID, ok := claims["user_id"].(float64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid user ID in token",
			"success": false,
			"status":  401,
		})
	}

	// Ambil role dari claims
	role, ok := claims["role"].(string)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"message": "Invalid role in token",
			"success": false,
			"status":  401,
		})
	}

	c.Locals("userID", int(userID)) // Menyimpan userID ke context
	c.Locals("role", role)          // Menyimpan role ke context
	return c.Next()                 // Melanjutkan ke handler berikutnya
}

func ErrorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				error_message := fmt.Sprintf("Recovered from panic: %v\nStack Trace: %s", r, debug.Stack())
				log.Println(error_message)
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": error_message,
				})
				return
			}
		}()
		return c.Next()
	}
}

// Auth handlers
func register(c *fiber.Ctx) error {
	// struct RegisterRequest menerima inputan dari user
	type RegisterRequest struct {
		Username string `json:"username" validate:"required,excludesall=@?"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=6"`
	}

	// variabel req digunakan untuk menerima inputan dari user
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	// Validasi dengan validator
	if err := validate.Struct(req); err != nil {
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
	err = db.QueryRow(
		"INSERT INTO users (username, email, password, role) VALUES ($1, $2, $3, 'member') RETURNING id",
		req.Username, req.Email, string(hashedPassword)).Scan(&userID) // Scan the generated ID into the userID variable
	if err != nil {
		// Jika error adalah unique violation error,
		// maka kita ingin mengembalikan status code 409 dengan message
		// yang mengindikasikan bahwa username sudah ada
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" {
				return c.Status(409).JSON(fiber.Map{
					"message": "Username already exists",
					"success": false,
					"status":  409,
				})
			}
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Error creating user",
			"success": false,
			"status":  500,
		})
	}

	return c.JSON(fiber.Map{
		"message": "User created successfully",
		"status":  201,
		"data": fiber.Map{
			"id": userID,
		},
	})
}

// fungsi login dengan menggunakan JSON Web Token (JWT)
func login(c *fiber.Ctx) error {
	// struct LoginRequest menerima inputan dari user
	type LoginRequest struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required"`
	}

	// variabel req digunakan untuk menerima inputan dari user
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		// jika inputan tidak valid, maka akan dikembalikan response error 400
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	if err := validate.Struct(req); err != nil {
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
	err := db.QueryRow(
		"SELECT id, username, email, password, role FROM users WHERE username = $1",
		req.Username).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.Role)
	if err != nil {
		// error 401, jika data user tidak ditemukan
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
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		// error 500, jika terjadi error saat mengencode token
		return c.Status(500).JSON(fiber.Map{
			"message": "Error generating token",
			"success": false,
			"status":  500,
		})
	}

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

// User handlers
// getAllUsers is a function to get all users, accessible only by admin
func getAllUsers(c *fiber.Ctx) error {
	// Ambil role dari locals
	role := c.Locals("role").(string)

	// Jika role bukan admin, kembalikan status 403 Forbidden
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// Ambil semua data user dari database
	rows, err := db.Query("SELECT id, username, email, role, profile_picture, created_at, updated_at FROM users")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching users",
			"success": false,
			"status":  500,
		})
	}
	// .Close() digunakan untuk menutup koneksi setelah selesai digunakan
	// hal ini perlu dilakukan agar koneksi tidak terbuka terus menerus
	// dan agar database tidak mengalami overload
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.ProfilePicture, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error scanning users",
				"success": false,
				"status":  500,
			})
		}

		// Jika ProfilePicture NULL, set jadi string kosong
		if !user.ProfilePicture.Valid {
			user.ProfilePicture.String = ""
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error iterating over users",
			"success": false,
			"status":  500,
		})
	}

	return c.JSON(fiber.Map{
		"message": "Users fetched successfully",
		"success": true,
		"status":  200,
		"data":    users,
	})
}

// getUser is a function to get a single user by ID
// accessible by admin and the user itself
func getUser(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)
	targetID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID",
			"success": false,
			"status":  400,
		})
	}

	// Jika role bukan admin dan user ID tidak sama dengan target ID
	if role != "admin" && userID != targetID {
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// Coba ambil data dari cache Redis
	cacheKey := fmt.Sprintf("user:%d", targetID)
	if cached, err := redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var user User
		if err = json.Unmarshal([]byte(cached), &user); err == nil {
			return c.JSON(fiber.Map{
				"message": "User found (from cache)",
				"success": true,
				"status":  200,
				"data":    user,
			})
		}
	}

	// Jika tidak ada di cache, ambil data dari databas
	var user User
	err = db.QueryRow(
		"SELECT id, username, email, role, created_at, updated_at FROM users WHERE id = $1",
		targetID).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"message": "User not found",
			"success": false,
			"status":  404,
		})
	}

	// Simpan data user ke cache Redis selama 1 jam
	userJSON, err := json.Marshal(user)
	if err == nil {
		redisClient.SetEX(ctx, cacheKey, userJSON, time.Hour)
	}

	return c.JSON(fiber.Map{
		"message": "User found",
		"success": true,
		"status":  200,
		"data":    user,
	})
}

// updateUser
func updateUser(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// Dapatkan target ID dari parameter URL
	targetID, err := c.ParamsInt("id")
	if err != nil {
		// Kembalikan error jika ID tidak valid
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID",
			"success": false,
			"status":  400,
		})
	}

	// Periksa apakah user memiliki izin untuk memperbarui user ini
	if role != "admin" && userID != targetID {
		return c.Status(403).JSON(fiber.Map{
			"message": "You don't have permission to update this user",
			"success": false,
			"status":  403,
		})
	}

	// Definisikan struktur untuk request update user
	// pointer (*) untuk menandakan bahwa field bisa kosong
	type UpdateUserRequest struct {
		Username *string `json:"username"`
		Email    *string `json:"email"`
		Password *string `json:"password"`
	}

	// Parsing body request ke dalam struct
	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		// Kembalikan error jika body request tidak dapat diparsing
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	// Hash the password using bcrypt with default cost
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
	if err != nil {
		// Return error response if password hashing fails
		return c.Status(500).JSON(fiber.Map{
			"message": "Error hashing password",
			"success": false,
			"status":  500,
		})
	}

	// Update hanya field yang dikirim (gunakan COALESCE di SQL)
	_, err = db.Exec(`
        UPDATE users 
        SET username = COALESCE(NULLIF($1, ''), username), 
			email = COALESCE(NULLIF($2, ''), email),
			password = COALESCE(NULLIF($3, ''), password)
        WHERE id = $4`,
		req.Username, req.Email, string(hashedPassword), targetID,
	)
	if err != nil {
		// Kembalikan error jika terjadi kesalahan saat memperbarui database
		return c.Status(500).JSON(fiber.Map{
			"message": "Error updating user",
			"success": false,
			"status":  500,
		})
	}

	// Ambil data user terbaru dari database
	var updatedUser User
	err = db.QueryRow(
		"SELECT id, username, email, role, created_at, updated_at FROM users WHERE id = $1",
		targetID,
	).Scan(&updatedUser.ID, &updatedUser.Username, &updatedUser.Email, &updatedUser.Role, &updatedUser.CreatedAt, &updatedUser.UpdatedAt)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching updated user",
			"success": false,
			"status":  500,
		})
	}

	// Perbarui cache Redis
	cacheKey := fmt.Sprintf("user:%d", targetID)
	redisClient.Del(ctx, cacheKey)
	userJSON, err := json.Marshal(updatedUser)
	if err == nil {
		redisClient.SetEX(ctx, cacheKey, userJSON, time.Hour)
	}

	// Kembalikan respons sukses jika user berhasil diperbarui
	return c.JSON(fiber.Map{
		"message": "User updated successfully",
		"success": true,
		"status":  200,
	})
}

// deleteUser
func deleteUser(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// Dapatkan target ID dari parameter URL
	targetID, err := c.ParamsInt("id")
	if err != nil {
		// Kembalikan error jika ID tidak valid
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID",
			"success": false,
			"status":  400,
		})
	}

	// Periksa apakah user memiliki izin untuk menghapus user ini
	if role != "admin" && userID != targetID {
		return c.Status(403).JSON(fiber.Map{
			"message": "You don't have permission to delete this user",
			"success": false,
			"status":  403,
		})
	}

	// Lakukan eksekusi query untuk menghapus user dari database
	_, err = db.Exec(
		"DELETE FROM users WHERE id = $1",
		targetID)
	if err != nil {
		// Kembalikan error jika terjadi kesalahan saat menghapus dari database
		return c.Status(500).JSON(fiber.Map{
			"message": "Error deleting user",
			"success": false,
			"status":  500,
		})
	}

	// Hapus cache Redis untuk user ini
	cacheKey := fmt.Sprintf("user:%d", targetID)
	redisClient.Del(ctx, cacheKey)

	// Kembalikan respons sukses jika user berhasil dihapus
	return c.Status(200).JSON(fiber.Map{
		"message": "User deleted successfully",
		"success": true,
		"status":  200,
	})
}

// Task handlers

// validStatus is a function to validate the status of a task
// it will return true if the status is one of the following:
// - pending
// - in_progress
// - completed
// and false otherwise
func validStatus(status string) bool {
	switch status {
	case "pending", "in_progress", "completed":
		return true
	default:
		return false
	}
}

// createTask adalah fungsi untuk membuat task baru
func createTask(c *fiber.Ctx) error {
	// ambil user ID dari locals
	userID := c.Locals("userID").(int)

	// struct TaskRequest menerima inputan dari user
	type TaskRequest struct {
		Title        string `json:"title" validate:"required"`
		Description  string `json:"description" validate:"required"`
		Status       string `json:"status" validate:"required,oneof=pending in_progress completed"`
		SecurityCode string `json:"security_code"`
	}

	// variabel req digunakan untuk menerima inputan dari user
	var req TaskRequest
	if err := c.BodyParser(&req); err != nil {
		// kembalikan error 400 jika inputan tidak valid
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	// Enkripsi Security Code
	encryptedCode, err := encrypt(req.SecurityCode, "MySecretEncryptionKey!")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error encrypting security code",
			"success": false,
			"status":  500,
		})
	}

	if err := validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Validation error",
			"errors":  err.Error(),
			"success": false,
			"status":  400,
		})
	}

	// validasi status, jika status tidak valid maka kembalikan error 400
	// status hanya boleh berisi: pending, in_progress, completed
	if !validStatus(req.Status) {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid status",
			"success": false,
			"status":  400,
		})
	}

	// lakukan eksekusi query untuk membuat task baru di database
	// jika gagal, maka kembalikan error 500
	var taskID int
	err = db.QueryRow(
		"INSERT INTO tasks (user_id, title, description, status, security_code) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		userID, req.Title, req.Description, req.Status, encryptedCode,
	).Scan(&taskID)
	if err != nil {
		log.Printf("Error creating task: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"message": "Error creating task",
			"success": false,
			"status":  500,
		})
	}

	// kembalikan respons sukses jika task berhasil dibuat
	return c.Status(201).JSON(fiber.Map{
		"message": "Task created successfully",
		"success": true,
		"status":  201,
		"id":      taskID,
	})
}

// listTasks adalah fungsi untuk mengambil semua task
func listTasks(c *fiber.Ctx) error {
	// ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// variabel rows digunakan untuk menyimpan hasil query
	// variabel err digunakan untuk menyimpan error jika ada
	var rows *sql.Rows
	var err error

	// query untuk mengambil semua task
	if role == "admin" {
		rows, err = db.Query("SELECT * FROM tasks")
	} else {
		// query untuk mengambil task berdasarkan user_id
		rows, err = db.Query("SELECT * FROM tasks WHERE user_id = $1", userID)
	}

	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengambil data dari database
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching tasks",
			"success": false,
			"status":  500,
		})
	}
	// .Close() digunakan untuk menutup koneksi setelah selesai digunakan
	// hal ini perlu dilakukan agar koneksi tidak terbuka terus menerus
	// dan agar database tidak mengalami overload
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		// Scan semua kolom yang diambil
		err := rows.Scan(&task.ID, &task.UserID, &task.Title, &task.Description, &task.Status, &task.SecurityCode, &task.CreatedAt, &task.UpdatedAt)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error scanning tasks",
				"success": false,
				"status":  500,
			})
		}

		// Dekripsi security_code jika tidak kosong
		if task.SecurityCode != "" {
			decrypted, err := decrypt(task.SecurityCode, "MySecretEncryptionKey!")
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"message": "Error decrypting security code",
					"success": false,
					"status":  500,
				})
			}
			task.SecurityCode = decrypted
		}

		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengulang data dari database
		return c.Status(500).JSON(fiber.Map{
			"message": "Error iterating over tasks",
			"success": false,
			"status":  500,
		})
	}

	// Simpan ke Redis
	for _, task := range tasks {
		cacheKey := fmt.Sprintf("task:%d", task.ID)

		// Convert task struct ke JSON sebelum disimpan di Redis
		jsonData, err := json.Marshal(task)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error encoding task to JSON",
				"success": false,
				"status":  500,
			})
		}

		// Set ke Redis dengan waktu kadaluarsa 1 jam
		err = redisClient.Set(ctx, cacheKey, jsonData, time.Hour).Err()
		if err != nil {
			log.Println("Redis error:", err) // Debugging log
			return c.Status(500).JSON(fiber.Map{
				"message": "Error caching task",
				"success": false,
				"status":  500,
			})
		}
	}

	// kembalikan respons sukses jika task berhasil diambil
	return c.JSON(fiber.Map{
		"message": "Tasks fetched successfully",
		"success": true,
		"status":  200,
		"data":    tasks,
	})
}

// getTask
func getTask(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// Dapatkan task ID dari parameter URL
	taskID, err := c.ParamsInt("id")
	if err != nil {
		// Kembalikan error jika ID task tidak valid
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid task ID",
			"success": false,
			"status":  400,
		})
	}

	// Coba ambil data task dari cache Redis
	cacheKey := fmt.Sprintf("task:%d", taskID)
	if cached, err := redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var task Task
		if err = json.Unmarshal([]byte(cached), &task); err == nil {
			// Validasi hak akses: admin bisa akses semua, user hanya jika task miliknya
			if role != "admin" && task.UserID != userID {
				return c.Status(403).JSON(fiber.Map{
					"message": "Forbidden",
					"success": false,
					"status":  403,
				})
			}
			return c.JSON(fiber.Map{
				"message": "Task found (from cache)",
				"success": true,
				"status":  200,
				"data":    task,
			})
		}
	}

	// Ambil data task dari database
	var task Task
	err = db.QueryRow(
		"SELECT id, user_id, title, description, status, security_code FROM tasks WHERE id = $1",
		taskID).Scan(&task.ID, &task.UserID, &task.Title, &task.Description, &task.Status, &task.SecurityCode)
	if err != nil {
		// Kembalikan error jika task tidak ditemukan
		return c.Status(404).JSON(fiber.Map{
			"message": "Task not found",
			"success": false,
			"status":  404,
		})
	}
	// Periksa apakah user memiliki izin untuk melihat task ini
	if role != "admin" && task.UserID != userID {
		// Kembalikan status 403 jika user tidak memiliki izin
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// Dekripsi Security Code
	task.SecurityCode, err = decrypt(task.SecurityCode, "MySecretEncryptionKey!")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error decrypting security code",
			"success": false,
			"status":  500,
		})
	}

	// Simpan data task ke cache selama 1 jam
	taskJSON, err := json.Marshal(task)
	if err == nil {
		redisClient.SetEX(ctx, cacheKey, taskJSON, time.Hour)
	}

	// Kembalikan respons sukses jika task ditemukan
	return c.JSON(fiber.Map{
		"message": "Task found",
		"success": true,
		"status":  200,
		"data":    task,
	})
}

// updateTask
func updateTask(c *fiber.Ctx) error {
	// ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// dapatkan target ID dari parameter URL
	taskID, err := c.ParamsInt("id")
	if err != nil {
		// kembalikan error 400 jika ID tidak valid
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid task ID",
			"success": false,
			"status":  400,
		})
	}

	var task Task
	err = db.QueryRow("SELECT user_id FROM tasks WHERE id = $1", taskID).Scan(&task.UserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"message": "Task not found",
			"success": false,
			"status":  404,
		})
	}

	// periksa apakah user memiliki izin untuk mengupdate task ini
	if role != "admin" && task.UserID != userID {
		return c.Status(403).JSON(fiber.Map{
			"message": "You don't have permission to update this task",
			"success": false,
			"status":  403,
		})
	}

	// struktur request untuk mengupdate task
	// pointer (*) untuk menandakan bahwa field bisa kosong
	type UpdateTaskRequest struct {
		Title        *string `json:"title"`
		Description  *string `json:"description"`
		Status       *string `json:"status"`
		SecurityCode *string `json:"security_code"`
	}

	// parsing body request ke dalam struct
	var req UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		// kembalikan error 400 jika body request tidak dapat diparsing
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	// periksa apakah status yang diinputkan valid
	// status hanya boleh berisi: pending, in_progress, completed
	if req.Status != nil {
		if !validStatus(*req.Status) {
			// kembalikan error 400 jika status tidak valid
			return c.Status(400).JSON(fiber.Map{
				"message": "Invalid status",
				"success": false,
				"status":  400,
			})
		}
	}

	var encryptedCode string
	if req.SecurityCode != nil {
		encryptedCode, err = encrypt(*req.SecurityCode, "MySecretEncryptionKey!")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error encrypting security code",
				"success": false,
				"status":  500,
			})
		}
	}

	// lakukan eksekusi query untuk mengupdate task di database
	_, err = db.Exec(`
		UPDATE tasks 
		SET title = COALESCE(NULLIF($1, ''), title), 
			description = COALESCE(NULLIF($2, ''), description), 
			status = COALESCE(NULLIF($3, ''), status),
			security_code = COALESCE(NULLIF($4, ''), security_code)
		WHERE id = $5`,
		req.Title, req.Description, req.Status, encryptedCode, taskID,
	)
	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengupdate database
		return c.Status(500).JSON(fiber.Map{
			"message": "Error updating task",
			"success": false,
			"status":  500,
		})
	}

	// Ambil data task terbaru dari database
	var updatedTask Task
	err = db.QueryRow(
		"SELECT id, user_id, title, description, status, security_code FROM tasks WHERE id = $1",
		taskID,
	).Scan(&updatedTask.ID, &updatedTask.UserID, &updatedTask.Title, &updatedTask.Description, &updatedTask.Status, &updatedTask.SecurityCode)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching updated task",
			"success": false,
			"status":  500,
		})
	}

	// Dekripsi security code
	updatedTask.SecurityCode, err = decrypt(updatedTask.SecurityCode, "MySecretEncryptionKey!")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error decrypting security code",
			"success": false,
			"status":  500,
		})
	}

	// Perbarui cache Redis untuk task ini
	cacheKey := fmt.Sprintf("task:%d", taskID)
	redisClient.Del(ctx, cacheKey)
	taskJSON, err := json.Marshal(updatedTask)
	if err == nil {
		redisClient.SetEX(ctx, cacheKey, taskJSON, time.Hour)
	}

	// kembalikan respons sukses jika task berhasil diupdate
	return c.Status(200).JSON(fiber.Map{
		"message": "Task updated successfully",
		"success": true,
		"status":  200,
	})
}

// deleteTask
func deleteTask(c *fiber.Ctx) error {
	// ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// dapatkan task ID dari parameter URL
	taskID, err := c.ParamsInt("id")
	if err != nil {
		// kembalikan error 400 jika ID tidak valid
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid task ID",
			"success": false,
			"status":  400,
		})
	}

	var task Task
	err = db.QueryRow("SELECT user_id FROM tasks WHERE id = $1", taskID).Scan(&task.UserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"message": "Task not found",
			"success": false,
			"status":  404,
		})
	}

	// periksa apakah user memiliki izin untuk menghapus task ini
	if role != "admin" && userID != task.UserID {
		// kembalikan status 403 jika user tidak memiliki izin
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// lakukan eksekusi query untuk menghapus task di database
	_, err = db.Exec("DELETE FROM tasks WHERE id = $1", taskID)
	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat menghapus dari database
		return c.Status(500).JSON(fiber.Map{
			"message": "Error deleting task",
			"success": false,
			"status":  500,
		})
	}

	// Hapus cache Redis untuk task ini
	cacheKey := fmt.Sprintf("task:%d", taskID)
	redisClient.Del(ctx, cacheKey)

	// kembalikan respons sukses jika task berhasil dihapus
	return c.Status(200).JSON(fiber.Map{
		"message": "Task deleted successfully",
		"success": true,
		"status":  200,
	})
}

// File Handling
// Fungsi untuk validasi file
func validateFile(file *multipart.FileHeader) error {
	// Validasi ukuran file maksimal 5MB
	if file.Size > 5<<20 {
		return fiber.NewError(fiber.StatusBadRequest, "File size exceeds the limit of 5MB")
	}

	// Validasi ekstensi file
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".pdf": true}
	if !allowedExts[ext] {
		return fiber.NewError(fiber.StatusBadRequest, "File type not allowed")
	}

	// Validasi tipe konten
	contentType := file.Header.Get("Content-Type")
	if !strings.Contains(contentType, "image") && !strings.Contains(contentType, "pdf") {
		return fiber.NewError(fiber.StatusBadRequest, "File must be an image or PDF")
	}

	return nil
}

// Fungsi untuk mendapatkan file
func getFile(c *fiber.Ctx) error {
	filename := c.Params("filename")
	filePath := path.Join("uploads", filename)
	return c.SendFile(filePath)
}

// Fungsi untuk mengunggah file
func uploadFile(c *fiber.Ctx) error {
	// Pastikan folder uploads sudah ada
	uploadDir := "uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		// Buat folder jika belum ada
		if err := os.Mkdir(uploadDir, os.ModePerm); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error creating upload directory",
				"success": false,
				"status":  500,
			})
		}
	}

	// Ambil file dari form-data
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Error uploading file",
			"success": false,
			"status":  400,
		})
	}

	// Validasi file
	if err := validateFile(file); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": err.Error(),
			"success": false,
			"status":  400,
		})
	}

	// Ubah nama file menjadi unik (berdasarkan timestamp)
	newFilename := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(file.Filename))

	// Simpan file ke dalam folder uploads
	filePath := path.Join(uploadDir, newFilename)
	if err := c.SaveFile(file, filePath); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error saving file",
			"success": false,
			"status":  500,
		})
	}

	return c.JSON(fiber.Map{
		"message": "File uploaded successfully",
		"success": true,
		"status":  200,
		"data": fiber.Map{
			"filename": newFilename,
			"size":     file.Size,
		},
	})
}

// Profile Picture Handling
func uploadProfilePicture(c *fiber.Ctx) error {
	userID := c.Locals("userID").(int)

	uploadDir := "uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.Mkdir(uploadDir, os.ModePerm); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error creating upload directory",
				"success": false,
				"status":  500,
			})
		}
	}

	file, err := c.FormFile("profile_picture")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Error uploading file",
			"success": false,
			"status":  400,
		})
	}

	if err := validateFile(file); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": err.Error(),
			"success": false,
			"status":  400,
		})
	}

	newFilename := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(file.Filename))
	filePath := path.Join(uploadDir, newFilename)

	if err := c.SaveFile(file, filePath); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error saving file",
			"success": false,
			"status":  500,
		})
	}

	fileURL := fmt.Sprintf("/uploads/%s", newFilename)

	_, err = db.Exec("UPDATE users SET profile_picture = $1 WHERE id = $2", fileURL, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error updating profile picture",
			"success": false,
			"status":  500,
		})
	}

	return c.JSON(fiber.Map{
		"message": "Profile picture uploaded successfully",
		"success": true,
		"status":  200,
		"data": fiber.Map{
			"profile_picture": fileURL,
		},
	})
}
