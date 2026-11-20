package user

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("user not found")

type Role string

const (
	StudentRole   Role = "student"
	UserRole      Role = StudentRole // UserRole is retained as the student role name for existing callers.
	ModeratorRole Role = "moderator"
	AdminRole     Role = "admin"
)

type User struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	Role       Role      `json:"role"`
	Username   string    `json:"username"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// TelegramProfile contains only fields Telegram supplies with an update.
type TelegramProfile struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}
