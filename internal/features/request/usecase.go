package request

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"future-voice/internal/features/user"
)

const (
	minThemeLength       = 3
	maxThemeLength       = 120
	minDescriptionLength = 10
	maxDescriptionLength = 5000
	maxListLimit         = 100
	defaultListLimit     = 20
)

type UserReader interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*user.User, error)
}

type RequestUseCase struct {
	repo  RequestRepository
	users UserReader
}

func NewUseCase(repo RequestRepository, users UserReader) *RequestUseCase {
	return &RequestUseCase{repo: repo, users: users}
}

func (uc *RequestUseCase) Create(ctx context.Context, input CreateInput) (*Request, error) {
	if err := validateCreateInput(&input); err != nil {
		return nil, err
	}

	actor, err := uc.activeUser(ctx, input.TelegramID)
	if err != nil {
		return nil, err
	}

	created, err := uc.repo.Create(ctx, CreateParams{
		UserID:         actor.ID,
		IdempotencyKey: input.IdempotencyKey,
		RequestType:    input.RequestType,
		Theme:          input.Theme,
		Description:    input.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return created, nil
}

func (uc *RequestUseCase) ListMine(ctx context.Context, telegramID int64, limit int) ([]Request, error) {
	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	return uc.repo.ListByUserID(ctx, actor.ID, normalizeLimit(limit))
}

// GetMine returns one request only when it belongs to the active Telegram user.
func (uc *RequestUseCase) GetMine(ctx context.Context, telegramID, requestID int64) (*Request, error) {
	if requestID <= 0 {
		return nil, ErrInvalidInput
	}
	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	item, err := uc.repo.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if item.UserID != actor.ID {
		return nil, ErrNotFound
	}
	return item, nil
}

func (uc *RequestUseCase) ListAll(ctx context.Context, telegramID int64, limit int) ([]Request, error) {
	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	if actor.Role != user.AdminRole && actor.Role != user.ModeratorRole {
		return nil, ErrForbidden
	}
	return uc.repo.ListAll(ctx, normalizeLimit(limit))
}

func (uc *RequestUseCase) Delete(ctx context.Context, telegramID, requestID int64) error {
	if requestID <= 0 {
		return ErrInvalidInput
	}
	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return err
	}

	existing, err := uc.repo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	isAdmin := actor.Role == user.AdminRole
	if !isAdmin && existing.UserID != actor.ID {
		// Avoid disclosing whether another student's request exists.
		return ErrNotFound
	}
	if !isAdmin && existing.Status != OpenStatus {
		return ErrConflict
	}

	if err := uc.repo.Delete(ctx, requestID, actor.ID, !isAdmin); err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	return nil
}

func (uc *RequestUseCase) UpdateStatus(
	ctx context.Context,
	telegramID, requestID int64,
	status Status,
) (*Request, error) {
	if requestID <= 0 {
		return nil, ErrInvalidInput
	}
	if !status.Valid() {
		return nil, ErrInvalidStatus
	}

	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	if actor.Role != user.AdminRole && actor.Role != user.ModeratorRole {
		return nil, ErrForbidden
	}

	updated, err := uc.repo.UpdateStatus(ctx, requestID, actor.ID, status)
	if err != nil {
		return nil, fmt.Errorf("update request status: %w", err)
	}
	return updated, nil
}

func (uc *RequestUseCase) activeUser(ctx context.Context, telegramID int64) (*user.User, error) {
	if telegramID <= 0 {
		return nil, ErrInvalidInput
	}
	actor, err := uc.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, fmt.Errorf("get Telegram user: %w", err)
	}
	if !actor.IsActive {
		return nil, ErrForbidden
	}
	return actor, nil
}

func validateCreateInput(input *CreateInput) error {
	if input.TelegramID <= 0 {
		return ErrInvalidInput
	}
	if !input.RequestType.Valid() {
		return ErrInvalidType
	}
	input.Theme = strings.TrimSpace(input.Theme)
	input.Description = strings.TrimSpace(input.Description)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)

	themeLength := utf8.RuneCountInString(input.Theme)
	descriptionLength := utf8.RuneCountInString(input.Description)
	if themeLength < minThemeLength || themeLength > maxThemeLength {
		return fmt.Errorf("%w: theme must contain %d..%d characters", ErrInvalidInput, minThemeLength, maxThemeLength)
	}
	if descriptionLength < minDescriptionLength || descriptionLength > maxDescriptionLength {
		return fmt.Errorf("%w: description must contain %d..%d characters", ErrInvalidInput, minDescriptionLength, maxDescriptionLength)
	}
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 128 {
		return fmt.Errorf("%w: idempotency_key must contain 8..128 bytes", ErrInvalidInput)
	}
	return nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}
