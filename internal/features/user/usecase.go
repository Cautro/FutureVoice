package user

import (
	"context"
	"errors"
	"strings"
)

type UserUseCase struct {
	repo UserRepository
}

func NewUseCase(repo UserRepository) *UserUseCase {
	return &UserUseCase{repo: repo}
}

func (uc *UserUseCase) GetByID(ctx context.Context, id int64) (*User, error) {
	if id <= 0 {
		return nil, errors.New("user ID must be positive")
	}
	return uc.repo.GetByID(ctx, id)
}

func (uc *UserUseCase) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	if telegramID <= 0 {
		return nil, errors.New("Telegram ID must be positive")
	}
	return uc.repo.GetByTelegramID(ctx, telegramID)
}

func (uc *UserUseCase) GetOrCreateTelegramUser(ctx context.Context, profile TelegramProfile) (*User, error) {
	if profile.TelegramID <= 0 {
		return nil, errors.New("Telegram ID must be positive")
	}
	profile.Username = strings.TrimSpace(profile.Username)
	profile.FirstName = strings.TrimSpace(profile.FirstName)
	profile.LastName = strings.TrimSpace(profile.LastName)
	return uc.repo.GetOrCreateTelegramUser(ctx, profile)
}

func (uc *UserUseCase) ListByRole(ctx context.Context, role Role) ([]User, error) {
	if role != StudentRole && role != ModeratorRole && role != AdminRole {
		return nil, errors.New("unknown user role")
	}
	return uc.repo.ListByRole(ctx, role)
}
