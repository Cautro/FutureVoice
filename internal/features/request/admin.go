package request

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"future-voice/internal/features/user"
)

type AdminRequest struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"-"`
	AuthorUsername    string    `json:"author_username"`
	AuthorFirstName   string    `json:"author_first_name"`
	AuthorLastName    string    `json:"author_last_name"`
	AssignedTo        *int64    `json:"assigned_to"`
	AssigneeUsername  string    `json:"assignee_username"`
	AssigneeFirstName string    `json:"assignee_first_name"`
	RequestType       Type      `json:"request_type"`
	Theme             string    `json:"theme"`
	Description       string    `json:"description"`
	Status            Status    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AdminFilter struct {
	Status Status
	Type   Type
	Query  string
	Page   int
	Limit  int
}

type AdminList struct {
	Items []AdminRequest `json:"items"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}

type Comment struct {
	ID         int64     `json:"id"`
	RequestID  int64     `json:"request_id"`
	AuthorID   int64     `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	Visibility string    `json:"visibility"`
	AuthorRole string    `json:"author_role"`
	CreatedAt  time.Time `json:"created_at"`
}

type Event struct {
	ID            int64     `json:"id"`
	RequestID     int64     `json:"request_id"`
	ActorID       int64     `json:"actor_id"`
	ActorName     string    `json:"actor_name"`
	EventType     string    `json:"event_type"`
	OldStatus     *Status   `json:"old_status,omitempty"`
	NewStatus     *Status   `json:"new_status,omitempty"`
	OldAssigneeID *int64    `json:"old_assignee_id,omitempty"`
	NewAssigneeID *int64    `json:"new_assignee_id,omitempty"`
	Details       string    `json:"details"`
	CreatedAt     time.Time `json:"created_at"`
}

type AdminDetails struct {
	Request  AdminRequest `json:"request"`
	Comments []Comment    `json:"comments"`
	Events   []Event      `json:"events"`
}

type AdminRequestRepository interface {
	ListAdmin(ctx context.Context, filter AdminFilter) (AdminList, error)
	GetAdminByID(ctx context.Context, id int64) (*AdminRequest, error)
	ListComments(ctx context.Context, requestID int64) ([]Comment, error)
	ListEvents(ctx context.Context, requestID int64) ([]Event, error)
	Assign(ctx context.Context, requestID, actorID int64, assigneeID *int64) error
	AddComment(ctx context.Context, requestID, authorID int64, body string) error
}

type ConversationRepository interface {
	AddStaffMessage(ctx context.Context, requestID, authorID int64, body, visibility string) error
	AddStudentMessage(ctx context.Context, requestID, authorID int64, body string) error
	ListStudentMessages(ctx context.Context, requestID int64) ([]Comment, error)
}

type statusDetailsRepository interface {
	UpdateStatusWithDetails(ctx context.Context, requestID, actorID int64, status Status, details string) (*Request, error)
}

type userByIDReader interface {
	GetByID(ctx context.Context, id int64) (*user.User, error)
}

func (uc *RequestUseCase) ListForPanel(ctx context.Context, actorID int64, filter AdminFilter) (AdminList, error) {
	if _, err := uc.staffUserByID(ctx, actorID); err != nil {
		return AdminList{}, err
	}
	if !filter.Status.Valid() && filter.Status != "" {
		return AdminList{}, ErrInvalidStatus
	}
	if !filter.Type.Valid() && filter.Type != "" {
		return AdminList{}, ErrInvalidType
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if utf8.RuneCountInString(filter.Query) > 100 {
		return AdminList{}, ErrInvalidInput
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	repo, err := uc.adminRepository()
	if err != nil {
		return AdminList{}, err
	}
	return repo.ListAdmin(ctx, filter)
}

func (uc *RequestUseCase) GetForPanel(ctx context.Context, actorID, requestID int64) (*AdminDetails, error) {
	if requestID <= 0 {
		return nil, ErrInvalidInput
	}
	if _, err := uc.staffUserByID(ctx, actorID); err != nil {
		return nil, err
	}
	repo, err := uc.adminRepository()
	if err != nil {
		return nil, err
	}
	item, err := repo.GetAdminByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	comments, err := repo.ListComments(ctx, requestID)
	if err != nil {
		return nil, err
	}
	events, err := repo.ListEvents(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return &AdminDetails{
		Request:  *item,
		Comments: comments,
		Events:   events,
	}, nil
}

func (uc *RequestUseCase) Assign(ctx context.Context, actorID, requestID int64, assigneeID *int64) error {
	actor, err := uc.staffUserByID(ctx, actorID)
	if err != nil {
		return err
	}
	if requestID <= 0 {
		return ErrInvalidInput
	}
	if assigneeID != nil {
		reader, ok := uc.users.(userByIDReader)
		if !ok {
			return errors.New("user lookup by ID is not configured")
		}
		assignee, err := reader.GetByID(ctx, *assigneeID)
		if err != nil {
			return err
		}
		if !assignee.IsActive || (assignee.Role != user.AdminRole && assignee.Role != user.ModeratorRole) {
			return ErrInvalidAssignee
		}
	}
	repo, err := uc.adminRepository()
	if err != nil {
		return err
	}
	existing, err := repo.GetAdminByID(ctx, requestID)
	if err != nil {
		return err
	}
	if actor.Role == user.ModeratorRole {
		if existing.AssignedTo != nil && *existing.AssignedTo != actor.ID {
			// Moderators may not take over or unassign another moderator's work.
			return ErrForbidden
		}
		if assigneeID != nil && *assigneeID != actor.ID {
			return ErrForbidden
		}
	}
	return repo.Assign(ctx, requestID, actor.ID, assigneeID)
}

func (uc *RequestUseCase) AddInternalComment(ctx context.Context, actorID, requestID int64, body string) error {
	actor, err := uc.staffUserByID(ctx, actorID)
	if err != nil {
		return err
	}
	if requestID <= 0 {
		return ErrInvalidInput
	}
	body = strings.TrimSpace(body)
	length := utf8.RuneCountInString(body)
	if length < 1 || length > 4000 {
		return ErrInvalidInput
	}
	repo, err := uc.adminRepository()
	if err != nil {
		return err
	}
	return repo.AddComment(ctx, requestID, actor.ID, body)
}

// UpdateStatusFromPanel performs a status change for a signed-in staff member.
// A rejection must explain the reason so the student receives useful feedback.
func (uc *RequestUseCase) UpdateStatusFromPanel(ctx context.Context, actorID, requestID int64, status Status, reason string) (*Request, error) {
	actor, err := uc.staffUserByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if requestID <= 0 {
		return nil, ErrInvalidInput
	}
	if !status.Valid() {
		return nil, ErrInvalidStatus
	}
	reason = strings.TrimSpace(reason)
	if status == RejectedStatus && utf8.RuneCountInString(reason) < 5 {
		return nil, fmt.Errorf("%w: для отклонения укажи причину (не менее 5 символов)", ErrInvalidInput)
	}
	if utf8.RuneCountInString(reason) > 1000 {
		return nil, ErrInvalidInput
	}
	repo, ok := uc.repo.(statusDetailsRepository)
	if !ok {
		return nil, errors.New("request repository does not support status details")
	}
	updated, err := repo.UpdateStatusWithDetails(ctx, requestID, actor.ID, status, reason)
	if err != nil {
		return nil, fmt.Errorf("update request status: %w", err)
	}
	return updated, nil
}

// AddStaffMessage sends a public reply to the student or stores an internal note.
func (uc *RequestUseCase) AddStaffMessage(ctx context.Context, actorID, requestID int64, body, visibility string) error {
	actor, err := uc.staffUserByID(ctx, actorID)
	if err != nil {
		return err
	}
	if requestID <= 0 {
		return ErrInvalidInput
	}
	body = strings.TrimSpace(body)
	if visibility != "student" && visibility != "internal" {
		return ErrInvalidInput
	}
	maxLength := 4000
	if visibility == "student" {
		// Leave room for the request title and notification prefix within
		// Telegram's message-size limit.
		maxLength = 3700
	}
	length := utf8.RuneCountInString(body)
	if length < 1 || length > maxLength {
		return ErrInvalidInput
	}
	repo, ok := uc.repo.(ConversationRepository)
	if !ok {
		return errors.New("request repository does not support messages")
	}
	return repo.AddStaffMessage(ctx, requestID, actor.ID, body, visibility)
}

// ListMessagesForStudent returns only public messages for a request owned by the student.
func (uc *RequestUseCase) ListMessagesForStudent(ctx context.Context, telegramID, requestID int64) ([]Comment, error) {
	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}
	if requestID <= 0 {
		return nil, ErrInvalidInput
	}
	item, err := uc.repo.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if item.UserID != actor.ID {
		return nil, ErrNotFound
	}
	repo, ok := uc.repo.(ConversationRepository)
	if !ok {
		return nil, errors.New("request repository does not support messages")
	}
	return repo.ListStudentMessages(ctx, requestID)
}

// AddMessageFromStudent stores a public message only when the request belongs to the sender.
func (uc *RequestUseCase) AddMessageFromStudent(ctx context.Context, telegramID, requestID int64, body string) error {
	actor, err := uc.activeUser(ctx, telegramID)
	if err != nil {
		return err
	}
	if requestID <= 0 {
		return ErrInvalidInput
	}
	body = strings.TrimSpace(body)
	length := utf8.RuneCountInString(body)
	if length < 1 || length > 4000 {
		return ErrInvalidInput
	}
	item, err := uc.repo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if item.UserID != actor.ID {
		return ErrNotFound
	}
	repo, ok := uc.repo.(ConversationRepository)
	if !ok {
		return errors.New("request repository does not support messages")
	}
	return repo.AddStudentMessage(ctx, requestID, actor.ID, body)
}

func (uc *RequestUseCase) DeleteFromPanel(ctx context.Context, actorID, requestID int64) error {
	actor, err := uc.staffUserByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor.Role != user.AdminRole {
		return ErrForbidden
	}
	if requestID <= 0 {
		return ErrInvalidInput
	}
	return uc.repo.Delete(ctx, requestID, actor.ID, false)
}

func (uc *RequestUseCase) staffUserByID(ctx context.Context, actorID int64) (*user.User, error) {
	if actorID <= 0 {
		return nil, ErrInvalidInput
	}
	reader, ok := uc.users.(userByIDReader)
	if !ok {
		return nil, errors.New("user lookup by ID is not configured")
	}
	actor, err := reader.GetByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive || (actor.Role != user.AdminRole && actor.Role != user.ModeratorRole) {
		return nil, ErrForbidden
	}
	return actor, nil
}

func (uc *RequestUseCase) adminRepository() (AdminRequestRepository, error) {
	repo, ok := uc.repo.(AdminRequestRepository)
	if !ok {
		return nil, fmt.Errorf("request repository does not support admin operations")
	}
	return repo, nil
}

type TelegramNotification struct {
	ID         int64  `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	RequestID  int64  `json:"request_id"`
	Body       string `json:"body"`
	Status     Status `json:"status"`
}

type TelegramNotificationRepository interface {
	ListPendingTelegramNotifications(ctx context.Context, limit int) ([]TelegramNotification, error)
	MarkTelegramNotificationDelivered(ctx context.Context, id int64) error
	FailTelegramNotification(ctx context.Context, id int64, message string) error
}

func (uc *RequestUseCase) PendingTelegramNotifications(ctx context.Context, limit int) ([]TelegramNotification, error) {
	repo, ok := uc.repo.(TelegramNotificationRepository)
	if !ok {
		return nil, errors.New("request repository does not support Telegram notifications")
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return repo.ListPendingTelegramNotifications(ctx, limit)
}

func (uc *RequestUseCase) MarkTelegramNotificationDelivered(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}
	repo, ok := uc.repo.(TelegramNotificationRepository)
	if !ok {
		return errors.New("request repository does not support Telegram notifications")
	}
	return repo.MarkTelegramNotificationDelivered(ctx, id)
}

func (uc *RequestUseCase) FailTelegramNotification(ctx context.Context, id int64, message string) error {
	if id <= 0 {
		return ErrInvalidInput
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > 500 {
		message = string([]rune(message)[:500])
	}
	repo, ok := uc.repo.(TelegramNotificationRepository)
	if !ok {
		return errors.New("request repository does not support Telegram notifications")
	}
	return repo.FailTelegramNotification(ctx, id, message)
}
