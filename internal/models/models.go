package models

import (
	"database/sql"
	"time"
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

type Task struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	SecurityCode string    `json:"security_code,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
