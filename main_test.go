package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	// Inisialisasi database (sesuaikan koneksi dengan environment testing)
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname_test)
	var err error
	db, err = sql.Open("postgres", psqlconn)
	if err != nil {
		log.Fatalf("Error opening DB: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Error pinging DB: %v", err)
	}
	// Buat tabel jika belum ada (atau reset tabel untuk testing)
	createTableIfNotExists(db)

	// Inisialisasi Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// Panggil m.Run() untuk menjalankan semua test
	code := m.Run()
	// Opsional: bersihkan data atau tutup koneksi
	os.Exit(code)
}

// createTestApp menginisialisasi aplikasi Fiber dengan route yang akan di-test
func createTestApp() *fiber.App {
	app := fiber.New()
	app.Use(ErrorHandler())
	app.Post("/register", register)
	app.Post("/login", login)

	// Route user (untuk endpoint user)
	userRoutes := app.Group("/users", useToken)
	userRoutes.Get("/", getAllUsers)
	userRoutes.Get("/:id", getUser)
	userRoutes.Put("/:id", updateUser)
	userRoutes.Delete("/:id", deleteUser)

	// Route upload (jika diperlukan)
	uploadRoutes := app.Group("/upload", useToken)
	uploadRoutes.Post("/profile_picture", uploadProfilePicture)

	// Route task
	taskRoutes := app.Group("/tasks", useToken)
	taskRoutes.Post("/", createTask)
	taskRoutes.Get("/", listTasks)
	taskRoutes.Get("/:id", getTask)
	taskRoutes.Put("/:id", updateTask)
	taskRoutes.Delete("/:id", deleteTask)

	return app
}

func TestRegister(t *testing.T) {
	app := createTestApp()

	uniqueUsername := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	reqBody := map[string]string{
		"username": uniqueUsername,
		"email":    uniqueUsername + "@example.com",
		"password": "secret123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d or %d but got %d", http.StatusOK, http.StatusCreated, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding register response: %v", err)
	}

	if result["data"] == nil {
		t.Errorf("Expected data field in response")
	}
}

func TestLogin(t *testing.T) {
	app := createTestApp()

	// Pastikan user yang akan di-login sudah terdaftar.
	// Jika perlu, register terlebih dahulu
	{
		reqBody := map[string]string{
			"username": "testlogin",
			"email":    "testlogin@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		_, _ = app.Test(req)
		// Abaikan error karena user bisa saja sudah ada
		time.Sleep(100 * time.Millisecond) // sedikit delay untuk memastikan user tersimpan
	}

	// Siapkan data login
	loginBody := map[string]string{
		"username": "testlogin",
		"password": "password123",
	}
	body, _ := json.Marshal(loginBody)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	if data["token"] == nil {
		t.Errorf("Expected token in login response")
	}
}

func TestUploadProfilePicture(t *testing.T) {
	app := createTestApp()

	// Pertama, register dan login agar mendapatkan token
	{
		reqBody := map[string]string{
			"username": "uploaduser",
			"email":    "uploaduser@example.com",
			"password": "uploadpass",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		_, _ = app.Test(req)
		time.Sleep(100 * time.Millisecond)
	}

	// Lakukan login
	loginBody := map[string]string{
		"username": "uploaduser",
		"password": "uploadpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	data, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid token")
	}

	// Sekarang, buat request multipart untuk upload profile picture
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Buat header untuk file dengan Content-Type yang valid
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="profile_picture"; filename="testpic.png"`)
	h.Set("Content-Type", "image/png") // Pastikan header content-type diset
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("Error creating form part: %v", err)
	}
	// Tulis data dummy untuk file (contoh: PNG signature)
	dummyData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	_, err = part.Write(dummyData)
	if err != nil {
		t.Fatalf("Error writing dummy file data: %v", err)
	}

	writer.Close()

	uploadReq := httptest.NewRequest("POST", "/upload/profile_picture", &b)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+token)

	uploadResp, err := app.Test(uploadReq)
	if err != nil {
		t.Fatalf("Upload profile picture request failed: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, uploadResp.StatusCode)
	}

	uploadRespBody, err := io.ReadAll(uploadResp.Body)
	if err != nil {
		t.Fatalf("Error reading upload response: %v", err)
	}
	// Ubah response menjadi string untuk pengecekan sederhana
	respStr := string(uploadRespBody)
	if !strings.Contains(respStr, "Profile picture uploaded successfully") {
		t.Errorf("Unexpected upload response: %s", respStr)
	}
}

// createTestAdmin secara langsung menyisipkan user admin ke database dan login untuk mendapatkan token
func createTestAdmin(app *fiber.App, t *testing.T) (string, int, string) {
	uniqueAdmin := fmt.Sprintf("admin_%d", time.Now().UnixNano())
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("adminpass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Error hashing admin password: %v", err)
	}
	var adminID int
	// Masukkan admin ke database dengan role 'admin'
	err = db.QueryRow(
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

func TestGetAllUsers(t *testing.T) {
	app := createTestApp()

	// Buat admin user dan login untuk mendapatkan token
	adminToken, _, _ := createTestAdmin(app, t)

	// Lakukan request GET /users dengan header Authorization
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error in getAllUsers request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding getAllUsers response: %v", err)
	}
	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Errorf("Expected non-empty data field in response")
	}
}

func TestGetUser(t *testing.T) {
	app := createTestApp()

	// Buat admin user dan login
	adminToken, adminID, adminUsername := createTestAdmin(app, t)

	// GET user dengan admin token (admin dapat mengakses user lain)
	req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", adminID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Error in getUser request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Error decoding getUser response: %v", err)
	}
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in getUser response")
	}
	if data["username"] != adminUsername {
		t.Errorf("Expected username %s but got %v", adminUsername, data["username"])
	}
}

func TestUpdateUser(t *testing.T) {
	app := createTestApp()

	// Buat user reguler melalui register
	uniqueUser := fmt.Sprintf("user_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "userpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("Error in register request for updateUser test: %v", err)
	}
	defer regResp.Body.Close()

	var regResult map[string]interface{}
	if err := json.NewDecoder(regResp.Body).Decode(&regResult); err != nil {
		t.Fatalf("Error decoding register response: %v", err)
	}
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in register response")
	}
	userIDFloat, ok := data["id"].(float64)
	if !ok {
		t.Fatalf("Expected user ID in register response")
	}
	userID := int(userIDFloat)

	// Login user untuk mendapatkan token
	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "userpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Error in login request for updateUser test: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	loginData, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	token, ok := loginData["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid token for updateUser test")
	}

	// Lakukan update user, misalnya ubah username
	newUsername := uniqueUser + "_updated"
	updateBody := map[string]string{
		"username": newUsername,
		"password": "newpass123",
		// kita bisa update email atau password jika diinginkan
	}
	updateJSON, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/users/%d", userID), bytes.NewReader(updateJSON))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("Error in updateUser request: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, updateResp.StatusCode)
	}

	// Ambil user kembali untuk memastikan update
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", userID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("Error in getUser after update request: %v", err)
	}
	defer getResp.Body.Close()
	var getResult map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&getResult); err != nil {
		t.Fatalf("Error decoding getUser after update response: %v", err)
	}
	getData, ok := getResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in getUser after update response")
	}
	if getData["username"] != newUsername {
		t.Errorf("Expected updated username %s but got %v", newUsername, getData["username"])
	}
}

func TestDeleteUser(t *testing.T) {
	app := createTestApp()

	// Buat user reguler melalui register
	uniqueUser := fmt.Sprintf("user_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "deletepass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("Error in register request for deleteUser test: %v", err)
	}
	defer regResp.Body.Close()
	var regResult map[string]interface{}
	if err := json.NewDecoder(regResp.Body).Decode(&regResult); err != nil {
		t.Fatalf("Error decoding register response: %v", err)
	}
	data, ok := regResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in register response")
	}
	userIDFloat, ok := data["id"].(float64)
	if !ok {
		t.Fatalf("Expected user ID in register response")
	}
	userID := int(userIDFloat)

	// Login user untuk mendapatkan token
	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "deletepass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Error in login request for deleteUser test: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	loginData, ok := loginResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data field in login response")
	}
	token, ok := loginData["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Expected valid token for deleteUser test")
	}

	// Lakukan request DELETE /users/:id
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/users/%d", userID), nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	if err != nil {
		t.Fatalf("Error in deleteUser request: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, delResp.StatusCode)
	}

	// Coba GET user tersebut, harus menghasilkan error 404
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/users/%d", userID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("Error in getUser after delete request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d for deleted user but got %d", http.StatusNotFound, getResp.StatusCode)
	}
}

// TestCreateTask: Uji pembuatan task baru
func TestCreateTask(t *testing.T) {
	app := createTestApp()

	// Register dan login user untuk task
	uniqueUser := fmt.Sprintf("taskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "taskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	// Login
	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "taskpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("Error decoding login response: %v", err)
	}
	token := loginResult["data"].(map[string]interface{})["token"].(string)
	if token == "" {
		t.Fatalf("Expected valid token")
	}

	// Create Task
	taskBody := map[string]string{
		"title":         "Test Task",
		"description":   "Task description",
		"status":        "pending",
		"security_code": "12345",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()

	if taskResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", taskResp.StatusCode)
	}
	var taskResult map[string]interface{}
	if err := json.NewDecoder(taskResp.Body).Decode(&taskResult); err != nil {
		t.Fatalf("Error decoding createTask response: %v", err)
	}
	if taskResult["id"] == nil {
		t.Errorf("Expected task id in response")
	}
}

// TestListTasks: Uji endpoint list tasks
func TestListTasks(t *testing.T) {
	app := createTestApp()

	uniqueUser := fmt.Sprintf("listuser_%d", time.Now().UnixNano())
	// Register & login
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "listpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)
	time.Sleep(100 * time.Millisecond) // pastikan data tersimpan

	loginBody := map[string]string{
		"username": uniqueUser,
		"password": "listpass",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat satu task terlebih dahulu
	taskBody := map[string]string{
		"title":         "List Task",
		"description":   "List description",
		"status":        "pending",
		"security_code": "abcde",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	_, _ = app.Test(taskReq)

	// Panggil endpoint list tasks
	listReq := httptest.NewRequest("GET", "/tasks", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("ListTasks error: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for list tasks, got %d", listResp.StatusCode)
	}
	var listResult map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&listResult); err != nil {
		t.Fatalf("Error decoding listTasks response: %v", err)
	}
	tasksData, ok := listResult["data"].([]interface{})
	if !ok || len(tasksData) == 0 {
		t.Errorf("Expected non-empty tasks data")
	}
}

// TestGetTask: Uji endpoint ambil task berdasarkan ID
func TestGetTask(t *testing.T) {
	app := createTestApp()

	uniqueUser := fmt.Sprintf("gettaskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "gettaskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginBody := map[string]string{"username": uniqueUser, "password": "gettaskpass"}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat task
	taskBody := map[string]string{
		"title":         "Get Task",
		"description":   "Get Task description",
		"status":        "pending",
		"security_code": "67890",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()
	var taskResult map[string]interface{}
	json.NewDecoder(taskResp.Body).Decode(&taskResult)
	taskID := int(taskResult["id"].(float64))

	// Ambil task dengan GET /tasks/:id
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d", taskID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GetTask error: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for get task, got %d", getResp.StatusCode)
	}
	var getResult map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&getResult)
	getData, ok := getResult["data"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected data field in get task response")
	}
	if getData["title"] != "Get Task" {
		t.Errorf("Expected title 'Get Task' but got %v", getData["title"])
	}
}

// TestUpdateTask: Uji endpoint update task
func TestUpdateTask(t *testing.T) {
	app := createTestApp()

	uniqueUser := fmt.Sprintf("updatetaskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "updatetaskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginBody := map[string]string{"username": uniqueUser, "password": "updatetaskpass"}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat task
	taskBody := map[string]string{
		"title":         "Old Task Title",
		"description":   "Old description",
		"status":        "pending",
		"security_code": "oldcode",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()
	var taskResult map[string]interface{}
	json.NewDecoder(taskResp.Body).Decode(&taskResult)
	taskID := int(taskResult["id"].(float64))

	// Update task: ubah title dan security_code
	updateBody := map[string]string{
		"title":         "Updated Task Title",
		"security_code": "newsecret",
	}
	updateJSON, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/tasks/%d", taskID), bytes.NewReader(updateJSON))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("UpdateTask error: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for update task, got %d", updateResp.StatusCode)
	}

	// Ambil task kembali untuk verifikasi
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d", taskID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GetTask after update error: %v", err)
	}
	defer getResp.Body.Close()
	var getResult map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&getResult)
	getData, ok := getResult["data"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected data field in get task response")
	}
	if getData["title"] != "Updated Task Title" {
		t.Errorf("Expected updated title 'Updated Task Title' but got %v", getData["title"])
	}
	// Pastikan security_code sudah didekripsi dengan benar
	if getData["security_code"] != "newsecret" {
		t.Errorf("Expected decrypted security_code 'newsecret' but got %v", getData["security_code"])
	}
}

// TestDeleteTask: Uji endpoint hapus task
func TestDeleteTask(t *testing.T) {
	app := createTestApp()

	uniqueUser := fmt.Sprintf("deletetaskuser_%d", time.Now().UnixNano())
	regBody := map[string]string{
		"username": uniqueUser,
		"email":    uniqueUser + "@example.com",
		"password": "deletetaskpass",
	}
	regJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest("POST", "/register", bytes.NewReader(regJSON))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginBody := map[string]string{"username": uniqueUser, "password": "deletetaskpass"}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewReader(loginJSON))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	defer loginResp.Body.Close()
	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	token := loginResult["data"].(map[string]interface{})["token"].(string)

	// Buat task
	taskBody := map[string]string{
		"title":         "Task to Delete",
		"description":   "This task will be deleted",
		"status":        "pending",
		"security_code": "tobedeleted",
	}
	taskJSON, _ := json.Marshal(taskBody)
	taskReq := httptest.NewRequest("POST", "/tasks", bytes.NewReader(taskJSON))
	taskReq.Header.Set("Content-Type", "application/json")
	taskReq.Header.Set("Authorization", "Bearer "+token)
	taskResp, err := app.Test(taskReq)
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}
	defer taskResp.Body.Close()
	var taskResult map[string]interface{}
	json.NewDecoder(taskResp.Body).Decode(&taskResult)
	taskID := int(taskResult["id"].(float64))

	// Hapus task
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/tasks/%d", taskID), nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	if err != nil {
		t.Fatalf("DeleteTask error: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for delete task, got %d", delResp.StatusCode)
	}

	// Pastikan task sudah tidak ada (GET harus mengembalikan 404)
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d", taskID), nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GetTask after delete error: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for deleted task, got %d", getResp.StatusCode)
	}
}
