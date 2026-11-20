package services

import (
	"context"

	"future-voice/internal/features/user"
)

type Service struct {
	userUseCase user.UserUseCase
}

func NewService(userUseCase user.UserUseCase) *Service {
	return &Service{userUseCase: userUseCase}
}

func (s *Service) CheckRole(ctx context.Context, userID int64, requiredRole user.Role) (bool, error) {
	account, err := s.userUseCase.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return account.Role == requiredRole && account.IsActive, nil
}
