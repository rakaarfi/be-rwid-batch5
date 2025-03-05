package main

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// User represent user data
type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UserBody represent body request for user
type UserBody struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// users variable for storing user data
var users = []User{}

// Task represent task data
type Task struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskBody represent body request for task
type TaskBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

// tasks variable for storing task data
var tasks = []Task{}

func main() {
	app := fiber.New(fiber.Config{
		Prefork: true,
	})

	// --------------------------------------------- //
	// --------------- Global Middleware ----------- //
	// --------------------------------------------- //
	app.Use(func(c *fiber.Ctx) error {
		log.Println("Global middleware")
		return c.Next()
	})

	// --------------------------------------------- //
	// --------------- Local Middleware ----------- //
	// --------------------------------------------- //
	localMiddleware := func(c *fiber.Ctx) error {
		log.Println("Local middleware for this route")
		UserID := c.Get("UserID") // get userID from header → return as string
		if UserID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusUnauthorized,
				"message": "Unauthorized",
			})
		}

		// userID validation
		userID, err := strconv.ParseInt(UserID, 10, 64) // convert string to int
		/*
        10 → Basis bilangan (desimal = 10).
        64 → Konversi ke tipe int64 (bilangan bulat 64-bit).
		*/
		if err != nil {
			/*
            if conversion fails.
                example:
                UserID = "abc123"
                strconv.ParseInt(UserID, 10, 64)
                error: strconv.ParseInt: parsing "abc123": invalid syntax
			*/
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		for _, user := range users {
			if user.ID == userID {
				c.Locals("userID", userID)
                /*
                set userID ke locals
                store temporary data in locals
                */
				break
			} else {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"success": false,
					"status":  fiber.StatusUnauthorized,
					"message": "User not found",
				})
			}
		}

		return c.Next()
	}

	// first endpoint
	/*
		    • func(c *fiber.Ctx) → handler function
			• fiber.Ctx → represents the context for the current request
			• c → the variable used to access the context
	*/
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	// second endpoint
	app.Get("/api-medium-rss", func(c *fiber.Ctx) error {
		resp, err := http.Get("https://api.rss2json.com/v1/api.json?rss_url=https://medium.com/feed/@rakaarfi")
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		return c.SendString(string(body))
	})

	// third endpoint
	app.Get("/book", func(c *fiber.Ctx) error {
		book := map[string]any{
			"title":    "The Go Programming Language",
			"author":   "Robert Griesemer",
			"year":     "2009",
			"is_ready": true,
		}
		return c.JSON(book)
	})

	// fourth endpoint
	app.Get("/book/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.SendString("Book ID: " + id)
	})

	// ---------------------------------------//
	// --------------- CRUD USERS ----------- //
	// ---------------------------------------//
	// Get all users
	app.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			// DTO: Data Transfer Object (data contract kesepakatan antara fe dan be)
			"success": true,
			"status":  http.StatusOK,
			"data":    users,
		})
	})

	// Create user
	app.Post("/users", func(c *fiber.Ctx) error {
		var body UserBody
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		// validasi body
		if body.Name == "" || body.Email == "" || body.Password == "" || body.Role == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": "Name, Email, Password, and Role are required",
			})
		}

		// validasi role harus "admin" atau "member"
		if body.Role != "admin" && body.Role != "member" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": "Role must be either 'admin' or 'member'",
			})
		}

		// validasi email harus ada @
		if !strings.Contains(body.Email, "@") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": "Email is invalid",
			})
		}

		// validasi email harus unique
		for _, user := range users {
			if user.Email == body.Email {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"status":  fiber.StatusBadRequest,
					"message": "Email already exists",
				})
			}
		}

		// insert ke users
		newUser := User{
			ID:       time.Now().Unix(),
			Name:     body.Name,
			Email:    body.Email,
			Password: body.Password,
			Role:     body.Role,
		}

		users = append(users, newUser)

		return c.JSON(fiber.Map{
			// DTO: Data Trasfer Object (data contract kesepakatan antara frontend dan backend)
			"success": true,
			"status":  200,
			"data":    newUser,
		})
	})

	// Update user (PUT - mengganti seluruh data)
	app.Put("/users/:id", func(c *fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		var body UserBody
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		for i, user := range users {
			if user.ID == id {
				// Update only the sent fields
				if body.Name != "" {
					users[i].Name = body.Name
				}

				if body.Email != "" {
					users[i].Email = body.Email
				}

				if body.Password != "" {
					users[i].Password = body.Password
				}

				if body.Role != "" {
					// Validasi: Role hanya boleh "admin" atau "member"
					if body.Role != "admin" && body.Role != "member" {
						return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
							"success": false,
							"status":  fiber.StatusBadRequest,
							"message": "Role must be either 'admin' or 'member'",
						})
					}
					users[i].Role = body.Role
				}

				return c.JSON(fiber.Map{
					"success": true,
					"status":  200,
					"data":    users[i],
				})
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"status":  fiber.StatusNotFound,
			"message": "User not found",
		})
	})

	// PATCH hanya untuk mengupdate role (harus "admin" atau "member")
	app.Patch("/users/:id", func(c *fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		// Ambil body request
		var body struct {
			Role string `json:"role"`
		}

		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		// Validasi: Role hanya boleh "admin" atau "member"
		if body.Role != "admin" && body.Role != "member" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Role must be either 'admin' or 'member'"})
		}

		// Update role user jika ID ditemukan
		for i, user := range users {
			if user.ID == id {
				users[i].Role = body.Role
				return c.JSON(fiber.Map{
					"success": true,
					"status":  200,
					"data":    users[i],
				})
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"status":  fiber.StatusNotFound,
			"message": "User not found",
		})
	})

	// Delete user
	app.Delete("/users/:id", func(c *fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		for i, user := range users {
			if user.ID == id {
				users = append(users[:i], users[i+1:]...) // remove user
				// users[:i] → mengambil semua elemen sebelum elemen yang dihapus.
				// users[i+1:] → mengambil semua elemen setelah elemen yang dihapus.
				// append(users[:i], users[i+1:]...) → menggabungkan keduanya tanpa elemen yang dihapus.
				return c.JSON(fiber.Map{
					"success": true,
					"status":  200,
					"message": "User deleted",
				})
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"status":  fiber.StatusNotFound,
			"message": "User not found",
		})
	})

	// ------------------------------------------- //
	// ------------- CRUD Task ------------------- //
	// ------------------------------------------- //
	// Get all tasks
	app.Get("/tasks", localMiddleware, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"status":  200,
			"data":    tasks,
		})
	})

	// Create task
	app.Post("/tasks", localMiddleware, func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(int64)
        /*
        Ambil `userID` yang sudah disimpan di middleware
        .(int64) → konversi dari interface{} ke int64
        */
        if !ok {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "success": false,
                "status":  fiber.StatusInternalServerError,
                "message": "Failed to get userID from middleware",
            })
        }
        
		var body TaskBody
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		// validasi body
		if body.Title == "" || body.Description == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": "Title and Description are required",
			})
		}

		task := Task{
			ID:          time.Now().Unix(),
			UserID:      userID,
			Title:       body.Title,
			Description: body.Description,
			Completed:   body.Completed,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		tasks = append(tasks, task)

		return c.JSON(fiber.Map{
			"success": true,
			"status":  200,
			"data":    task,
		})
	})

	// Update task
	app.Put("/tasks/:id", localMiddleware, func(c *fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		var body TaskBody
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		for i, task := range tasks {
			if task.ID == id {
				// Update only the sent fields
				if body.Title != "" {
					tasks[i].Title = body.Title
				}

				if body.Description != "" {
					tasks[i].Description = body.Description
				}

				if body.Completed != task.Completed {
					tasks[i].Completed = body.Completed
				}

				tasks[i].UpdatedAt = time.Now()

				return c.JSON(fiber.Map{
					"success": true,
					"status":  200,
					"data":    tasks[i],
				})
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"status":  fiber.StatusNotFound,
			"message": "Task not found",
		})
	})

	// Patch task
	app.Patch("/tasks/:id", localMiddleware, func(c *fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		// Ambil body
		var body struct {
			Completed bool `json:"completed"`
		}

		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		for i, task := range tasks {
			if task.ID == id {
				tasks[i].Completed = body.Completed
				return c.JSON(fiber.Map{
					"success": true,
					"status":  200,
					"data":    tasks[i],
				})
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"status":  fiber.StatusNotFound,
			"message": "Task not found",
		})
	})

	// Delete task
	app.Delete("/tasks/:id", localMiddleware, func(c *fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"status":  fiber.StatusBadRequest,
				"message": err.Error(),
			})
		}

		for i, task := range tasks {
			if task.ID == id {
				tasks = append(tasks[:i], tasks[i+1:]...) // remove task
				// users[:i] → mengambil semua elemen sebelum elemen yang dihapus.
				// users[i+1:] → mengambil semua elemen setelah elemen yang dihapus.
				// append(users[:i], users[i+1:]...) → menggabungkan keduanya tanpa elemen yang dihapus.
				return c.JSON(fiber.Map{
					"success": true,
					"status":  200,
					"message": "Task deleted",
				})
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"status":  fiber.StatusNotFound,
			"message": "Task not found",
		})
	})

	log.Fatal(app.Listen(":3003"))
}
