package v1

import (
	"belajar-go/internal/api/v1/handlers"
	"belajar-go/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// Auth
	api.Post("/login", handlers.Login)
	api.Post("/register", handlers.Register)

	// User
	userRoutes := api.Group("/users", middleware.UseToken)
	userRoutes.Get("/", handlers.GetAllUsers)
	userRoutes.Get("/:id", handlers.GetUser)
	userRoutes.Put("/:id", handlers.UpdateUser)
	userRoutes.Delete("/:id", handlers.DeleteUser)

	// Task
	taskRoutes := api.Group("/tasks", middleware.UseToken)
	taskRoutes.Post("/", handlers.CreateTask)
	taskRoutes.Get("/", handlers.ListTasks)
	taskRoutes.Get("/:id", handlers.GetTask)
	taskRoutes.Put("/:id", handlers.UpdateTask)
	taskRoutes.Delete("/:id", handlers.DeleteTask)

	// File Upload
	uploadRoutes := api.Group("/upload", middleware.UseToken)
	uploadRoutes.Post("/", handlers.UploadFile)
	uploadRoutes.Get("/:filename", handlers.GetFile)
	uploadRoutes.Post("/profile_picture", handlers.UploadProfilePicture)
}
