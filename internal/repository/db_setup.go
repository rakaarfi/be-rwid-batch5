package repository

import (
	"database/sql"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func CreateTableIfNotExists(db *sql.DB) {
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

func CreateAdminUser(db *sql.DB) {
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

func DeleteAllTable(db *sql.DB) {
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
