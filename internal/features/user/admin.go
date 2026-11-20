package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type AdminUserRepository interface {
	ListAdminUsers(ctx context.Context, role Role, query string, page, limit int) ([]User, int64, error)
	UpdateRole(ctx context.Context, actorID, targetID int64, role Role) error
	UpdateActive(ctx context.Context, actorID, targetID int64, active bool) error
}

type AdminUserList struct {
	Items []User `json:"items"`
	Total int64  `json:"total"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
}

var (
	ErrForbidden    = errors.New("operation is not permitted")
	ErrLastAdmin    = errors.New("cannot remove or deactivate the last active administrator")
	ErrInvalidRole  = errors.New("invalid user role")
	ErrInvalidQuery = errors.New("user search query is too long")
	ErrSelfChange   = errors.New("you cannot change your own role or deactivate your own account")
)

func (uc *UserUseCase) ListForAdmin(ctx context.Context, actorID int64, role Role, query string, page, limit int) (AdminUserList, error) {
	actor, err := uc.repo.GetByID(ctx, actorID)
	if err != nil {
		return AdminUserList{}, err
	}
	if !actor.IsActive || actor.Role != AdminRole {
		return AdminUserList{}, ErrForbidden
	}
	if role != "" && role != StudentRole && role != ModeratorRole && role != AdminRole {
		return AdminUserList{}, ErrInvalidRole
	}
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) > 100 {
		return AdminUserList{}, ErrInvalidQuery
	}
	if page < 1 {
		page = 1
	}
	if page > 1_000_000 {
		return AdminUserList{}, errors.New("page is too large")
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	repo, ok := uc.repo.(AdminUserRepository)
	if !ok {
		return AdminUserList{}, errors.New("user repository does not support admin operations")
	}
	items, total, err := repo.ListAdminUsers(ctx, role, query, page, limit)
	if err != nil {
		return AdminUserList{}, fmt.Errorf("list users for admin: %w", err)
	}
	return AdminUserList{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (uc *UserUseCase) ChangeRole(ctx context.Context, actorID, targetID int64, role Role) error {
	actor, err := uc.repo.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if !actor.IsActive || actor.Role != AdminRole {
		return ErrForbidden
	}
	if targetID <= 0 || (role != StudentRole && role != ModeratorRole && role != AdminRole) {
		return ErrInvalidRole
	}
	if actorID == targetID {
		return ErrSelfChange
	}
	repo, ok := uc.repo.(AdminUserRepository)
	if !ok {
		return errors.New("user repository does not support admin operations")
	}
	return repo.UpdateRole(ctx, actorID, targetID, role)
}

func (uc *UserUseCase) ChangeActive(ctx context.Context, actorID, targetID int64, active bool) error {
	actor, err := uc.repo.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if !actor.IsActive || actor.Role != AdminRole {
		return ErrForbidden
	}
	if targetID <= 0 {
		return ErrInvalidRole
	}
	if actorID == targetID {
		return ErrSelfChange
	}
	repo, ok := uc.repo.(AdminUserRepository)
	if !ok {
		return errors.New("user repository does not support admin operations")
	}
	return repo.UpdateActive(ctx, actorID, targetID, active)
}
