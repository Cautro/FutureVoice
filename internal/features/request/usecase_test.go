package request

import (
	"context"
	"errors"
	"testing"

	"future-voice/internal/features/user"
)

type fakeUsers struct {
	users map[int64]*user.User
}

func (f fakeUsers) GetByTelegramID(_ context.Context, telegramID int64) (*user.User, error) {
	found, ok := f.users[telegramID]
	if !ok {
		return nil, user.ErrNotFound
	}
	copy := *found
	return &copy, nil
}

type fakeRequests struct {
	created        *Request
	requestStatus  Status
	deleted        bool
	deleteOnlyOpen bool
	updatedStatus  Status
	listAllCalled  bool
}

func (f *fakeRequests) Create(_ context.Context, params CreateParams) (*Request, error) {
	f.created = &Request{ID: 42, UserID: params.UserID, Status: OpenStatus, RequestType: params.RequestType, Theme: params.Theme, Description: params.Description}
	return f.created, nil
}
func (f *fakeRequests) GetByID(_ context.Context, id int64) (*Request, error) {
	if id != 42 {
		return nil, ErrNotFound
	}
	status := f.requestStatus
	if status == "" {
		status = OpenStatus
	}
	return &Request{ID: 42, UserID: 10, Status: status}, nil
}
func (f *fakeRequests) ListByUserID(context.Context, int64, int) ([]Request, error) {
	return []Request{}, nil
}
func (f *fakeRequests) ListAll(context.Context, int) ([]Request, error) {
	f.listAllCalled = true
	return []Request{}, nil
}
func (f *fakeRequests) Delete(_ context.Context, _ int64, _ int64, onlyOpen bool) error {
	f.deleted = true
	f.deleteOnlyOpen = onlyOpen
	return nil
}
func (f *fakeRequests) UpdateStatus(_ context.Context, id, _ int64, status Status) (*Request, error) {
	f.updatedStatus = status
	return &Request{ID: id, Status: status}, nil
}

func TestCreateTrimsAndPersistsAsAuthenticatedUser(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	created, err := uc.Create(context.Background(), CreateInput{
		TelegramID:     777,
		IdempotencyKey: "telegram:777:15",
		RequestType:    ComplaintType,
		Theme:          "  Столовая  ",
		Description:    "  Нужно исправить ситуацию в столовой.  ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.UserID != 10 {
		t.Fatalf("created user ID = %d, want 10", created.UserID)
	}
	if created.Theme != "Столовая" {
		t.Fatalf("theme = %q, want trimmed value", created.Theme)
	}
	if repo.created.Description != "Нужно исправить ситуацию в столовой." {
		t.Fatalf("description was not trimmed: %q", repo.created.Description)
	}
}

func TestDeleteAllowsOwnerOnlyWhileOpen(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	if err := uc.Delete(context.Background(), 777, 42); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !repo.deleted || !repo.deleteOnlyOpen {
		t.Fatalf("Delete() did not request an open-only soft delete")
	}
}

func TestDeleteDoesNotDiscloseAnotherUsersRequest(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 99, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	err := uc.Delete(context.Background(), 777, 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
	if repo.deleted {
		t.Fatal("repository must not be called for a forbidden deletion")
	}
}

func TestAdminCanUpdateStatus(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 1, TelegramID: 777, Role: user.AdminRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	updated, err := uc.UpdateStatus(context.Background(), 777, 42, InProgressStatus)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Status != InProgressStatus || repo.updatedStatus != InProgressStatus {
		t.Fatalf("status was not updated")
	}
}

func TestStudentCannotUpdateStatus(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	_, err := uc.UpdateStatus(context.Background(), 777, 42, ClosedStatus)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("UpdateStatus() error = %v, want ErrForbidden", err)
	}
}

func TestListAllRequiresAdministrator(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	_, err := uc.ListAll(context.Background(), 777, 20)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListAll() error = %v, want ErrForbidden", err)
	}
	if repo.listAllCalled {
		t.Fatal("repository must not be called for a non-admin")
	}
}

func TestInactiveUserCannotCreateRequest(t *testing.T) {
	repo := &fakeRequests{}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: false},
	}}
	uc := NewUseCase(repo, users)

	_, err := uc.Create(context.Background(), CreateInput{
		TelegramID:     777,
		IdempotencyKey: "telegram:777:15",
		RequestType:    ComplaintType,
		Theme:          "Столовая",
		Description:    "Нужно исправить ситуацию в столовой.",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create() error = %v, want ErrForbidden", err)
	}
	if repo.created != nil {
		t.Fatal("repository must not be called for an inactive user")
	}
}

func TestCreateRejectsUnknownRequestType(t *testing.T) {
	input := CreateInput{
		TelegramID:     777,
		IdempotencyKey: "telegram:777:15",
		RequestType:    Type("unknown"),
		Theme:          "Столовая",
		Description:    "Нужно исправить ситуацию в столовой.",
	}
	if err := validateCreateInput(&input); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("validateCreateInput() error = %v, want ErrInvalidType", err)
	}
}

func TestStudentCannotDeleteClosedRequest(t *testing.T) {
	repo := &fakeRequests{requestStatus: ClosedStatus}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	err := uc.Delete(context.Background(), 777, 42)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Delete() error = %v, want ErrConflict", err)
	}
	if repo.deleted {
		t.Fatal("repository must not delete a closed request for a student")
	}
}

func TestAdminCanDeleteClosedRequest(t *testing.T) {
	repo := &fakeRequests{requestStatus: ClosedStatus}
	users := fakeUsers{users: map[int64]*user.User{
		777: {ID: 1, TelegramID: 777, Role: user.AdminRole, IsActive: true},
	}}
	uc := NewUseCase(repo, users)

	if err := uc.Delete(context.Background(), 777, 42); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !repo.deleted || repo.deleteOnlyOpen {
		t.Fatal("admin deletion should allow any request status")
	}
}

type contextAwareUsers struct {
	fakeUsers
	seenContext context.Context
}

func (f *contextAwareUsers) GetByTelegramID(ctx context.Context, telegramID int64) (*user.User, error) {
	f.seenContext = ctx
	return f.fakeUsers.GetByTelegramID(ctx, telegramID)
}

type contextAwareRequests struct {
	*fakeRequests
	seenContext context.Context
}

func (f *contextAwareRequests) Create(ctx context.Context, params CreateParams) (*Request, error) {
	f.seenContext = ctx
	return f.fakeRequests.Create(ctx, params)
}

func TestCreatePropagatesCallerContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "trace-value")

	users := &contextAwareUsers{fakeUsers: fakeUsers{users: map[int64]*user.User{
		777: {ID: 10, TelegramID: 777, Role: user.StudentRole, IsActive: true},
	}}}
	repo := &contextAwareRequests{fakeRequests: &fakeRequests{}}
	uc := NewUseCase(repo, users)

	_, err := uc.Create(ctx, CreateInput{
		TelegramID:     777,
		IdempotencyKey: "telegram:777:15",
		RequestType:    ComplaintType,
		Theme:          "Столовая",
		Description:    "Нужно исправить ситуацию в столовой.",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if users.seenContext != ctx || repo.seenContext != ctx {
		t.Fatal("Create() did not pass the caller context to dependencies")
	}
	if users.seenContext.Value(contextKey{}) != "trace-value" || repo.seenContext.Value(contextKey{}) != "trace-value" {
		t.Fatal("caller context values were not propagated")
	}
}
