package user

import "context"

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	GetOrCreateTelegramUser(ctx context.Context, profile TelegramProfile) (*User, error)
	ListByRole(ctx context.Context, role Role) ([]User, error)
}
