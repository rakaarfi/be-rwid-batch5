package handlers

import (
	"belajar-go/internal/config"
	"belajar-go/internal/models"
	"belajar-go/pkg/logger"
	"belajar-go/pkg/crypto"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

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
func CreateTask(c *fiber.Ctx) error {
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
		logger.ErrorLogger.Error("Bad request in create task", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"success": false,
			"status":  400,
		})
	}

	// Enkripsi Security Code
	encryptedCode, err := crypto.Encrypt(req.SecurityCode, "MySecretEncryptionKey!")
	if err != nil {
		logger.ErrorLogger.Error("Error encrypting security code", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error encrypting security code",
			"success": false,
			"status":  500,
		})
	}

	if err := config.Validate.Struct(req); err != nil {
		logger.ErrorLogger.Error("Validation error in create task", zap.Error(err))
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
		logger.ErrorLogger.Error("Invalid status in create task")
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid status",
			"success": false,
			"status":  400,
		})
	}

	// lakukan eksekusi query untuk membuat task baru di database
	// jika gagal, maka kembalikan error 500
	var taskID int
	err = config.DB.QueryRow(
		"INSERT INTO tasks (user_id, title, description, status, security_code) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		userID, req.Title, req.Description, req.Status, encryptedCode,
	).Scan(&taskID)
	if err != nil {
		log.Printf("Error creating task: %v", err)
		logger.ErrorLogger.Error("Error creating task", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error creating task",
			"success": false,
			"status":  500,
		})
	}

	// kembalikan respons sukses jika task berhasil dibuat
	logger.AuditLogger.Info("Task created successfully", zap.Int("task_id", taskID))
	return c.Status(201).JSON(fiber.Map{
		"message": "Task created successfully",
		"success": true,
		"status":  201,
		"id":      taskID,
	})
}

// listTasks adalah fungsi untuk mengambil semua task
func ListTasks(c *fiber.Ctx) error {
	// ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// variabel rows digunakan untuk menyimpan hasil query
	// variabel err digunakan untuk menyimpan error jika ada
	var rows *sql.Rows
	var err error

	// query untuk mengambil semua task
	if role == "admin" {
		rows, err = config.DB.Query("SELECT * FROM tasks")
	} else {
		// query untuk mengambil task berdasarkan user_id
		rows, err = config.DB.Query("SELECT * FROM tasks WHERE user_id = $1", userID)
	}

	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengambil data dari database
		logger.ErrorLogger.Error("Error fetching tasks", zap.Error(err))
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

	tasks := []models.Task{}
	for rows.Next() {
		var task models.Task
		// Scan semua kolom yang diambil
		err := rows.Scan(&task.ID, &task.UserID, &task.Title, &task.Description, &task.Status, &task.SecurityCode, &task.CreatedAt, &task.UpdatedAt)
		if err != nil {
			// kembalikan error 500 jika terjadi kesalahan saat mengambil data dari database
			logger.ErrorLogger.Error("Error scanning tasks", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{
				"message": "Error scanning tasks",
				"success": false,
				"status":  500,
			})
		}

		// Dekripsi security_code jika tidak kosong
		if task.SecurityCode != "" {
			decrypted, err := crypto.Decrypt(task.SecurityCode, "MySecretEncryptionKey!")
			if err != nil {
				// kembalikan error 500 jika terjadi kesalahan saat mengambil data dari database
				logger.ErrorLogger.Error("Error decrypting security code", zap.Error(err))
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
		logger.ErrorLogger.Error("Error iterating over tasks", zap.Error(err))
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
			log.Println("Error encoding task to JSON:", err)
			logger.ErrorLogger.Error("Error encoding task to JSON", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{
				"message": "Error encoding task to JSON",
				"success": false,
				"status":  500,
			})
		}

		// Set ke Redis dengan waktu kadaluarsa 1 jam
		err = config.RedisClient.Set(config.Ctx, cacheKey, jsonData, time.Hour).Err()
		if err != nil {
			log.Println("Redis error:", err) // Debugging log
			logger.ErrorLogger.Error("Error caching task", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{
				"message": "Error caching task",
				"success": false,
				"status":  500,
			})
		}
	}

	// kembalikan respons sukses jika task berhasil diambil
	logger.AuditLogger.Info("Tasks fetched successfully")
	return c.JSON(fiber.Map{
		"message": "Tasks fetched successfully",
		"success": true,
		"status":  200,
		"data":    tasks,
	})
}

// getTask
func GetTask(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// Dapatkan task ID dari parameter URL
	taskID, err := c.ParamsInt("id")
	if err != nil {
		// Kembalikan error jika ID task tidak valid
		logger.ErrorLogger.Error("Invalid task ID", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid task ID",
			"success": false,
			"status":  400,
		})
	}

	// Coba ambil data task dari cache Redis
	cacheKey := fmt.Sprintf("task:%d", taskID)
	if cached, err := config.RedisClient.Get(config.Ctx, cacheKey).Result(); err == nil {
		var task models.Task
		if err = json.Unmarshal([]byte(cached), &task); err == nil {
			// Validasi hak akses: admin bisa akses semua, user hanya jika task miliknya
			if role != "admin" && task.UserID != userID {
				// Kembalikan error jika hak akses tidak sesuai
				logger.ErrorLogger.Error("Forbidden", zap.Error(err))
				return c.Status(403).JSON(fiber.Map{
					"message": "Forbidden",
					"success": false,
					"status":  403,
				})
			}

			// Kembalikan data task
			logger.AuditLogger.Info("Task found (from cache)")
			return c.JSON(fiber.Map{
				"message": "Task found (from cache)",
				"success": true,
				"status":  200,
				"data":    task,
			})
		}
	}

	// Ambil data task dari database
	var task models.Task
	err = config.DB.QueryRow(
		"SELECT id, user_id, title, description, status, security_code FROM tasks WHERE id = $1",
		taskID).Scan(&task.ID, &task.UserID, &task.Title, &task.Description, &task.Status, &task.SecurityCode)
	if err != nil {
		// Kembalikan error jika task tidak ditemukan
		logger.ErrorLogger.Error("Task not found", zap.Error(err))
		return c.Status(404).JSON(fiber.Map{
			"message": "Task not found",
			"success": false,
			"status":  404,
		})
	}
	// Periksa apakah user memiliki izin untuk melihat task ini
	if role != "admin" && task.UserID != userID {
		// Kembalikan status 403 jika user tidak memiliki izin
		logger.ErrorLogger.Error("Forbidden", zap.Error(err))
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// Dekripsi Security Code
	task.SecurityCode, err = crypto.Decrypt(task.SecurityCode, "MySecretEncryptionKey!")
	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengambil data dari database
		logger.ErrorLogger.Error("Error decrypting security code", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error decrypting security code",
			"success": false,
			"status":  500,
		})
	}

	// Simpan data task ke cache selama 1 jam
	taskJSON, err := json.Marshal(task)
	if err == nil {
		config.RedisClient.SetEX(config.Ctx, cacheKey, taskJSON, time.Hour)
	}

	// Kembalikan respons sukses jika task ditemukan
	logger.AuditLogger.Info("Task found")
	return c.JSON(fiber.Map{
		"message": "Task found",
		"success": true,
		"status":  200,
		"data":    task,
	})
}

// updateTask
func UpdateTask(c *fiber.Ctx) error {
	// ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// dapatkan target ID dari parameter URL
	taskID, err := c.ParamsInt("id")
	if err != nil {
		// kembalikan error 400 jika ID tidak valid
		logger.ErrorLogger.Error("Invalid task ID", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid task ID",
			"success": false,
			"status":  400,
		})
	}

	var task models.Task
	err = config.DB.QueryRow("SELECT user_id FROM tasks WHERE id = $1", taskID).Scan(&task.UserID)
	if err != nil {
		// kembalikan error 404 jika task tidak ditemukan
		logger.ErrorLogger.Error("Task not found", zap.Error(err))
		return c.Status(404).JSON(fiber.Map{
			"message": "Task not found",
			"success": false,
			"status":  404,
		})
	}

	// periksa apakah user memiliki izin untuk mengupdate task ini
	if role != "admin" && task.UserID != userID {
		// kembalikan error 403 jika user tidak memiliki izin
		logger.ErrorLogger.Error("You don't have permission to update this task", zap.Error(err))
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
		logger.ErrorLogger.Error("Bad request in update task", zap.Error(err))
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
			logger.ErrorLogger.Error("Invalid status", zap.Error(err))
			return c.Status(400).JSON(fiber.Map{
				"message": "Invalid status",
				"success": false,
				"status":  400,
			})
		}
	}

	var encryptedCode string
	if req.SecurityCode != nil {
		encryptedCode, err = crypto.Encrypt(*req.SecurityCode, "MySecretEncryptionKey!")
		if err != nil {
			logger.ErrorLogger.Error("Error encrypting security code", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{
				"message": "Error encrypting security code",
				"success": false,
				"status":  500,
			})
		}
	}

	// lakukan eksekusi query untuk mengupdate task di database
	_, err = config.DB.Exec(`
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
		logger.ErrorLogger.Error("Error updating task", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error updating task",
			"success": false,
			"status":  500,
		})
	}

	// Ambil data task terbaru dari database
	var updatedTask models.Task
	err = config.DB.QueryRow(
		"SELECT id, user_id, title, description, status, security_code FROM tasks WHERE id = $1",
		taskID,
	).Scan(&updatedTask.ID, &updatedTask.UserID, &updatedTask.Title, &updatedTask.Description, &updatedTask.Status, &updatedTask.SecurityCode)
	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengambil data task
		logger.ErrorLogger.Error("Error fetching updated task", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching updated task",
			"success": false,
			"status":  500,
		})
	}

	// Dekripsi security code
	updatedTask.SecurityCode, err = crypto.Decrypt(updatedTask.SecurityCode, "MySecretEncryptionKey!")
	if err != nil {
		logger.ErrorLogger.Error("Error decrypting security code", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error decrypting security code",
			"success": false,
			"status":  500,
		})
	}

	// Perbarui cache Redis untuk task ini
	cacheKey := fmt.Sprintf("task:%d", taskID)
	config.RedisClient.Del(config.Ctx, cacheKey)
	taskJSON, err := json.Marshal(updatedTask)
	if err == nil {
		config.RedisClient.SetEX(config.Ctx, cacheKey, taskJSON, time.Hour)
	}

	// kembalikan respons sukses jika task berhasil diupdate
	logger.AuditLogger.Info("Task updated", zap.Int("taskID", taskID))
	return c.Status(200).JSON(fiber.Map{
		"message": "Task updated successfully",
		"success": true,
		"status":  200,
	})
}

// deleteTask
func DeleteTask(c *fiber.Ctx) error {
	// ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// dapatkan task ID dari parameter URL
	taskID, err := c.ParamsInt("id")
	if err != nil {
		// kembalikan error 400 jika ID tidak valid
		logger.ErrorLogger.Error("Invalid task ID", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid task ID",
			"success": false,
			"status":  400,
		})
	}

	var task models.Task
	err = config.DB.QueryRow("SELECT user_id FROM tasks WHERE id = $1", taskID).Scan(&task.UserID)
	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat mengambil data task
		logger.ErrorLogger.Error("Error fetching task", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching task",
			"success": false,
			"status":  500,
		})
	}

	// Periksa apakah task ditemukan
	if task.UserID == 0 {
		// kembalikan status 404 jika task tidak ditemukan
		if err == sql.ErrNoRows {
			logger.ErrorLogger.Error("Task not found", zap.Error(err))
			return c.Status(404).JSON(fiber.Map{
				"message": "Task not found",
				"success": false,
				"status":  404,
			})
		}
	}

	// periksa apakah user memiliki izin untuk menghapus task ini
	if role != "admin" && userID != task.UserID {
		// kembalikan status 403 jika user tidak memiliki izin
		logger.SecurityLogger.Warn("You don't have permission to delete this task", zap.String("role", role), zap.Int("user_id", userID), zap.Int("task_id", taskID))
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// lakukan eksekusi query untuk menghapus task di database
	_, err = config.DB.Exec("DELETE FROM tasks WHERE id = $1", taskID)
	if err != nil {
		// kembalikan error 500 jika terjadi kesalahan saat menghapus dari database
		logger.ErrorLogger.Error("Error deleting task", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error deleting task",
			"success": false,
			"status":  500,
		})
	}

	// Hapus cache Redis untuk task ini
	cacheKey := fmt.Sprintf("task:%d", taskID)
	config.RedisClient.Del(config.Ctx, cacheKey)

	// kembalikan respons sukses jika task berhasil dihapus
	logger.AuditLogger.Info("Task deleted", zap.Int("taskID", taskID))
	return c.Status(200).JSON(fiber.Map{
		"message": "Task deleted successfully",
		"success": true,
		"status":  200,
	})
}
