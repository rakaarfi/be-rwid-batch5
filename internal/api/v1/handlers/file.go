package handlers

import (
	"belajar-go/internal/config"
	"belajar-go/pkg/logger"
	"fmt"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

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
func GetFile(c *fiber.Ctx) error {
	filename := c.Params("filename")
	filePath := path.Join("uploads", filename)
	return c.SendFile(filePath)
}

// Fungsi untuk mengunggah file
func UploadFile(c *fiber.Ctx) error {
	// Pastikan folder uploads sudah ada
	uploadDir := "uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		// Buat folder jika belum ada
		if err := os.Mkdir(uploadDir, os.ModePerm); err != nil {
			// kembalikan error 500 jika terjadi kesalahan saat membuat folder
			logger.ErrorLogger.Error("Error creating upload directory", zap.Error(err))
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
		// kembalikan error 400 jika terjadi kesalahan saat mengunggah file
		logger.ErrorLogger.Error("Error uploading file", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Error uploading file",
			"success": false,
			"status":  400,
		})
	}

	// Validasi file
	if err := validateFile(file); err != nil {
		// kembalikan error 400 jika terjadi kesalahan saat validasi file
		logger.ErrorLogger.Error("Error validating file", zap.Error(err))
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
		// kembalikan error 500 jika terjadi kesalahan saat menyimpan file
		logger.ErrorLogger.Error("Error saving file", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error saving file",
			"success": false,
			"status":  500,
		})
	}

	// kembalikan respons sukses
	logger.AuditLogger.Info("File uploaded", zap.String("filename", newFilename))
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
func UploadProfilePicture(c *fiber.Ctx) error {
	userID := c.Locals("userID").(int)

	uploadDir := "uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.Mkdir(uploadDir, os.ModePerm); err != nil {
			logger.ErrorLogger.Error("Error creating upload directory", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{
				"message": "Error creating upload directory",
				"success": false,
				"status":  500,
			})
		}
	}

	file, err := c.FormFile("profile_picture")
	if err != nil {
		logger.ErrorLogger.Error("Error uploading file", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": "Error uploading file",
			"success": false,
			"status":  400,
		})
	}

	if err := validateFile(file); err != nil {
		logger.ErrorLogger.Error("Error validating file", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"message": err.Error(),
			"success": false,
			"status":  400,
		})
	}

	newFilename := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(file.Filename))
	filePath := path.Join(uploadDir, newFilename)

	if err := c.SaveFile(file, filePath); err != nil {
		logger.ErrorLogger.Error("Error saving file", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error saving file",
			"success": false,
			"status":  500,
		})
	}

	fileURL := fmt.Sprintf("/uploads/%s", newFilename)

	_, err = config.DB.Exec("UPDATE users SET profile_picture = $1 WHERE id = $2", fileURL, userID)
	if err != nil {
		logger.ErrorLogger.Error("Error updating profile picture", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Error updating profile picture",
			"success": false,
			"status":  500,
		})
	}

	logger.AuditLogger.Info("Profile picture uploaded", zap.String("filename", newFilename))
	return c.JSON(fiber.Map{
		"message": "Profile picture uploaded successfully",
		"success": true,
		"status":  200,
		"data": fiber.Map{
			"profile_picture": fileURL,
		},
	})
}
