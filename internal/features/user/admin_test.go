package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type adminTestRepo struct {
	users map[int64]*User
}

func (r *adminTestRepo) GetByID(_ context.Context, id int64) (*User, error) {
	item, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *item
	return &copy, nil
}
func (r *adminTestRepo) GetByTelegramID(_ context.Context, id int64) (*User, error) {
	for _, item := range r.users {
		if item.TelegramID == id {
			copy := *item
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}
func (r *adminTestRepo) GetOrCreateTelegramUser(context.Context, TelegramProfile) (*User, error) {
	return nil, errors.New("not implemented")
}
func (r *adminTestRepo) ListByRole(context.Context, Role) ([]User, error) { return nil, nil }

func TestChangeRoleRejectsSelfChange(t *testing.T) {
	uc := NewUseCase(&adminTestRepo{users: map[int64]*User{
		1: {ID: 1, Role: AdminRole, IsActive: true},
	}})
	err := uc.ChangeRole(context.Background(), 1, 1, ModeratorRole)
	if !errors.Is(err, ErrSelfChange) {
		t.Fatalf("ChangeRole error = %v, want ErrSelfChange", err)
	}
}

func TestChangeActiveRejectsSelfChange(t *testing.T) {
	uc := NewUseCase(&adminTestRepo{users: map[int64]*User{
		1: {ID: 1, Role: AdminRole, IsActive: true},
	}})
	err := uc.ChangeActive(context.Background(), 1, 1, false)
	if !errors.Is(err, ErrSelfChange) {
		t.Fatalf("ChangeActive error = %v, want ErrSelfChange", err)
	}
}

func TestListForAdminRejectsLongQuery(t *testing.T) {
	uc := NewUseCase(&adminTestRepo{users: map[int64]*User{
		1: {ID: 1, Role: AdminRole, IsActive: true, CreatedAt: time.Now()},
	}})
	_, err := uc.ListForAdmin(context.Background(), 1, "", strings.Repeat("x", 101), 1, 20)
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("ListForAdmin error = %v, want ErrInvalidQuery", err)
	}
}
