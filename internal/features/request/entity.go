package request

import "time"

type Type string

const (
	ComplaintType    Type = "complaint"
	OfferType        Type = "offer"
	Print3DOfferType Type = "3dPrintOffer"
)

func (t Type) Valid() bool {
	switch t {
	case ComplaintType, OfferType, Print3DOfferType:
		return true
	default:
		return false
	}
}

type Status string

const (
	OpenStatus           Status = "open"
	InProgressStatus     Status = "in_progress"
	WaitingForUserStatus Status = "waiting_for_user"
	ClosedStatus         Status = "closed"
	RejectedStatus       Status = "rejected"
)

func (s Status) Valid() bool {
	switch s {
	case OpenStatus, InProgressStatus, WaitingForUserStatus, ClosedStatus, RejectedStatus:
		return true
	default:
		return false
	}
}

type Request struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Status      Status    `json:"status"`
	RequestType Type      `json:"request_type"`
	Theme       string    `json:"theme"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateInput struct {
	TelegramID     int64  `json:"telegram_id"`
	IdempotencyKey string `json:"idempotency_key"`
	RequestType    Type   `json:"request_type"`
	Theme          string `json:"theme"`
	Description    string `json:"description"`
}

type StatusUpdateInput struct {
	TelegramID int64  `json:"telegram_id"`
	Status     Status `json:"status"`
}
