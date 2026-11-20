package request

import "context"

type CreateParams struct {
	UserID         int64
	IdempotencyKey string
	RequestType    Type
	Theme          string
	Description    string
}

type RequestRepository interface {
	Create(ctx context.Context, params CreateParams) (*Request, error)
	GetByID(ctx context.Context, id int64) (*Request, error)
	ListByUserID(ctx context.Context, userID int64, limit int) ([]Request, error)
	ListAll(ctx context.Context, limit int) ([]Request, error)
	Delete(ctx context.Context, id, actorID int64, onlyOpen bool) error
	UpdateStatus(ctx context.Context, id, actorID int64, status Status) (*Request, error)
}
