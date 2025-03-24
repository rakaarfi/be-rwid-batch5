package handlers

import (
	"belajar-go/internal/config"
	"belajar-go/internal/models"
	"belajar-go/pkg/logger"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// User handlers
// getAllUsers is a function to get all users, accessible only by admin
func GetAllUsers(c *fiber.Ctx) error {
	// Ambil role dari locals
	role := c.Locals("role").(string)

	// Jika role bukan admin, kembalikan status 403 Forbidden
	if role != "admin" {
		logger.SecurityLogger.Warn("Forbidden", zap.String("role", role))
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// Ambil semua data user dari database
	rows, err := config.DB.Query("SELECT id, username, email, role, profile_picture, created_at, updated_at FROM users")
	if err != nil {
		logger.ErrorLogger.Error("Error fetching users", zap.Error(err))
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

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.ProfilePicture, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			logger.ErrorLogger.Error("Error scanning users", zap.Error(err))
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
		logger.ErrorLogger.Error("Error iterating over users", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error iterating over users",
			"success": false,
			"status":  500,
		})
	}

	// kembalikan response success
	logger.AuditLogger.Info("Users fetched successfully")
	return c.JSON(fiber.Map{
		"message": "Users fetched successfully",
		"success": true,
		"status":  200,
		"data":    users,
	})
}

// getUser is a function to get a single user by ID
// accessible by admin and the user itself
func GetUser(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)
	targetID, err := c.ParamsInt("id")
	if err != nil {
		logger.ErrorLogger.Error("Invalid user ID", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID",
			"success": false,
			"status":  400,
		})
	}

	// Jika role bukan admin dan user ID tidak sama dengan target ID
	if role != "admin" && userID != targetID {
		logger.SecurityLogger.Warn("Forbidden", zap.String("role", role), zap.Int("user_id", userID), zap.Int("target_id", targetID))
		return c.Status(403).JSON(fiber.Map{
			"message": "Forbidden",
			"success": false,
			"status":  403,
		})
	}

	// Coba ambil data dari cache Redis
	cacheKey := fmt.Sprintf("user:%d", targetID)
	if cached, err := config.RedisClient.Get(config.Ctx, cacheKey).Result(); err == nil {
		var user models.User
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
	var user models.User
	err = config.DB.QueryRow(
		"SELECT id, username, email, role, created_at, updated_at FROM users WHERE id = $1",
		targetID).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		logger.SecurityLogger.Warn("User not found", zap.Error(err))
		return c.Status(404).JSON(fiber.Map{
			"message": "User not found",
			"success": false,
			"status":  404,
		})
	}

	// Simpan data user ke cache Redis selama 1 jam
	userJSON, err := json.Marshal(user)
	if err == nil {
		config.RedisClient.SetEX(config.Ctx, cacheKey, userJSON, time.Hour)
	}

	// Kembalikan response
	logger.AuditLogger.Info("User found")
	return c.JSON(fiber.Map{
		"message": "User found",
		"success": true,
		"status":  200,
		"data":    user,
	})
}

// updateUser
func UpdateUser(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// Dapatkan target ID dari parameter URL
	targetID, err := c.ParamsInt("id")
	if err != nil {
		// Kembalikan error jika ID tidak valid
		logger.ErrorLogger.Error("Invalid user ID", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID",
			"success": false,
			"status":  400,
		})
	}

	// Periksa apakah user memiliki izin untuk memperbarui user ini
	if role != "admin" && userID != targetID {
		logger.SecurityLogger.Warn("You don't have permission to update this user", zap.String("role", role), zap.Int("user_id", userID), zap.Int("target_id", targetID))
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
		logger.ErrorLogger.Error("Bad request", zap.Error(err))
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
		logger.ErrorLogger.Error("Error hashing password", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error hashing password",
			"success": false,
			"status":  500,
		})
	}

	// Update hanya field yang dikirim (gunakan COALESCE di SQL)
	_, err = config.DB.Exec(`
        UPDATE users 
        SET username = COALESCE(NULLIF($1, ''), username), 
			email = COALESCE(NULLIF($2, ''), email),
			password = COALESCE(NULLIF($3, ''), password)
        WHERE id = $4`,
		req.Username, req.Email, string(hashedPassword), targetID,
	)
	if err != nil {
		// Kembalikan error jika terjadi kesalahan saat memperbarui database
		logger.ErrorLogger.Error("Error updating user", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error updating user",
			"success": false,
			"status":  500,
		})
	}

	// Ambil data user terbaru dari database
	var updatedUser models.User
	err = config.DB.QueryRow(
		"SELECT id, username, email, role, created_at, updated_at FROM users WHERE id = $1",
		targetID,
	).Scan(&updatedUser.ID, &updatedUser.Username, &updatedUser.Email, &updatedUser.Role, &updatedUser.CreatedAt, &updatedUser.UpdatedAt)
	if err != nil {
		// Kembalikan error jika terjadi kesalahan saat mengambil data user
		logger.ErrorLogger.Error("Error fetching updated user", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error fetching updated user",
			"success": false,
			"status":  500,
		})
	}

	// Perbarui cache Redis
	cacheKey := fmt.Sprintf("user:%d", targetID)
	config.RedisClient.Del(config.Ctx, cacheKey)
	userJSON, err := json.Marshal(updatedUser)
	if err == nil {
		config.RedisClient.SetEX(config.Ctx, cacheKey, userJSON, time.Hour)
	}

	// Kembalikan respons sukses jika user berhasil diperbarui
	logger.AuditLogger.Info("User updated successfully", zap.Int("user_id", targetID))
	return c.JSON(fiber.Map{
		"message": "User updated successfully",
		"success": true,
		"status":  200,
	})
}

// deleteUser
func DeleteUser(c *fiber.Ctx) error {
	// Ambil user ID dan role dari locals
	userID := c.Locals("userID").(int)
	role := c.Locals("role").(string)

	// Dapatkan target ID dari parameter URL
	targetID, err := c.ParamsInt("id")
	if err != nil {
		// Kembalikan error jika ID tidak valid
		logger.ErrorLogger.Error("Invalid user ID", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID",
			"success": false,
			"status":  400,
		})
	}

	// Periksa apakah user memiliki izin untuk menghapus user ini
	if role != "admin" && userID != targetID {
		logger.SecurityLogger.Warn("You don't have permission to delete this user", zap.String("role", role), zap.Int("user_id", userID), zap.Int("target_id", targetID))
		return c.Status(403).JSON(fiber.Map{
			"message": "You don't have permission to delete this user",
			"success": false,
			"status":  403,
		})
	}

	// Lakukan eksekusi query untuk menghapus user dari database
	_, err = config.DB.Exec(
		"DELETE FROM users WHERE id = $1",
		targetID)
	if err != nil {
		// Kembalikan error jika terjadi kesalahan saat menghapus dari database
		logger.ErrorLogger.Error("Error deleting user", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error deleting user",
			"success": false,
			"status":  500,
		})
	}

	// Hapus cache Redis untuk user ini
	cacheKey := fmt.Sprintf("user:%d", targetID)
	config.RedisClient.Del(config.Ctx, cacheKey)

	// Kembalikan respons sukses jika user berhasil dihapus
	logger.AuditLogger.Info("User deleted successfully", zap.Int("user_id", targetID))
	return c.Status(200).JSON(fiber.Map{
		"message": "User deleted successfully",
		"success": true,
		"status":  200,
	})
}
