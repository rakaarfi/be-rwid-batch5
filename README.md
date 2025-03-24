# BE-RWID Batch 5

This project is a backend application built with Fiber and PostgreSQL, featuring Redis caching, and structured logging with zap. The project follows a modular architecture with separate layers for API, repository, service, and middleware. Configuration is managed through environment variables using a `.env` file.

## Project Structure

```
my_fiber_project/
├── cmd/
│   └── api/
│       └── main.go            # Application entry point
├── configs/
│   └── config.go              # Configuration settings (using .env)
├── go.mod
├── go.sum
├── internal/
│   ├── api/
│   │   ├── v1/
│   │   │   ├── handlers/      # API v1 handlers
│   │   │   │   ├── auth.go
│   │   │   │   ├── file.go
│   │   │   │   ├── task.go
│   │   │   │   └── user.go
│   │   │   └── routes.go      # API v1 routes registration
│   │   └── v2/               # (Optional: Additional API version)
│   ├── config/
│   │   └── dependencies.go    # Global dependencies (DB, Redis, Validator, etc.)
│   ├── middleware/           # Custom middleware
│   │   ├── auth.go           # Authentication middleware
│   │   └── logger.go         # Logging middleware (error handler, request logger)
│   ├── models/               # Database models
│   │   └── models.go
│   ├── repository/           # Database operations (setup tables, CRUD, etc.)
│   │   └── db_setup.go       # Functions: CreateTableIfNotExists, CreateAdminUser, DeleteAllTable
│   ├── service/              # Business logic (optional)
│   └── websocket/            # WebSocket implementation (if needed)
│       └── hub.go            # Definitions for Hub and Client
├── logs/                     # Log files output
│   ├── audit.log
│   ├── context.log
│   ├── errors.log
│   ├── request.log
│   ├── security.log
│   └── system.log
├── pkg/
│   ├── crypto/               # Encryption/Decryption functions
│   │   └── crypto.go
│   ├── database/             # Database and Redis connection helpers
│   │   ├── database.go
│   │   └── redis.go
│   └── logger/               # Logger initialization using zap
│       └── logger.go
├── test/                     # Test files
│   ├── auth_test.go
│   ├── file_test.go
│   ├── main_test.go
│   ├── task_test.go
│   └── user_test.go
├── tests/                    # Additional tests (e.g., mathutils)
│   ├── mathutils.go
│   └── mathutils_test.go
├── uploads/                  # Folder for uploaded files (images, PDFs, etc.)
├── .env                      # Environment variables file
├── .gitignore
└── README.md
```

## Configuration

The application configuration is loaded from the `.env` file located in the root of the project. For example:

```
DB_HOST=000.000.000.000
DB_PORT=00000
DB_USER=postgres
DB_PASSWORD=00000000
DB_NAME=postgres
DB_NAME_TEST=postgres_test
REDIS_HOST=000.000.000.000
REDIS_PORT=0000
```

The function `configs.LoadConfig()` reads these environment variables. In a testing environment, you can override these values or set a `GO_ENV` variable to `"test"`.

## Running the Application

From the project root (where `go.mod` is located), run:

```bash
go run cmd/api/main.go
```

This will start the Fiber application on port 3004.

## Running Tests

To run all tests located in the `test` directory, use:

```bash
go test -v ./test/...
```

TestMain in your tests will load the environment variables and initialize the necessary dependencies.

## Features

- **User Authentication:**  
  Endpoints for user registration and login using JWT.  
  - `/api/v1/register`
  - `/api/v1/login`

- **User CRUD:**  
  Endpoints to manage user data (accessible by admin or the user themselves).  
  - `/api/v1/users`

- **Task Management:**  
  Endpoints to create, list, update, retrieve, and delete tasks.  
  - `/api/v1/tasks`

- **File Upload:**  
  Endpoints to upload files and profile pictures.  
  - `/api/v1/upload`

- **Structured Logging:**  
  Uses zap to log different types of events to separate files:
  - **errors.log:** Errors and panics
  - **audit.log:** Important events (registration, login, CRUD operations)
  - **request.log:** Incoming HTTP requests
  - **security.log:** Security-related warnings (unauthorized access, duplicate entries)
  - **system.log:** System information (startup, connections, etc.)
  - **context.log:** Additional context data

- **Environment-based Configuration:**  
  All settings are loaded from environment variables (via .env) for flexibility and easier configuration management.

## Logging and Environment

The project uses [godotenv](https://github.com/joho/godotenv) to load environment variables. In production or local development, the `.env` file is read automatically. In tests, you can set `GO_ENV` to `"test"` to avoid duplicate logging of environment load events.

## License

MIT License

Copyright (c) 2025 Raka Arfi

Permission is hereby granted, free of charge, to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of this software, provided the above copyright notice and this permission notice are included in all copies.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.

