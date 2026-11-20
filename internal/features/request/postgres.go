package request

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type postgresRepository struct {
	db *sql.DB
}

func New(db *sql.DB) RequestRepository {
	return &postgresRepository{db: db}
}

const requestColumns = `id, user_id, request_type, theme, description, status, created_at, updated_at`

func (r *postgresRepository) Create(ctx context.Context, params CreateParams) (*Request, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create request transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `
		INSERT INTO requests (user_id, request_type, theme, description, status, source_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (source_key) DO NOTHING
		RETURNING ` + requestColumns

	created, err := scanRequest(tx.QueryRowContext(
		ctx,
		query,
		params.UserID,
		string(params.RequestType),
		params.Theme,
		params.Description,
		string(OpenStatus),
		params.IdempotencyKey,
	))
	if errors.Is(err, sql.ErrNoRows) {
		// Telegram may retry delivery after a timeout. Return the original row
		// for the same source key instead of creating a duplicate request.
		created, err = scanRequest(tx.QueryRowContext(
			ctx,
			`SELECT `+requestColumns+` FROM requests WHERE source_key = $1 AND deleted_at IS NULL`,
			params.IdempotencyKey,
		))
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConflict
		}
		if err != nil {
			return nil, fmt.Errorf("load idempotent request: %w", err)
		}
		if created.UserID != params.UserID || created.RequestType != params.RequestType ||
			created.Theme != params.Theme || created.Description != params.Description {
			return nil, ErrConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit idempotent request transaction: %w", err)
		}
		return created, nil
	}
	if err != nil {
		return nil, fmt.Errorf("insert request: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_events (request_id, actor_user_id, event_type, new_status)
		VALUES ($1, $2, 'created', $3)
	`, created.ID, params.UserID, string(OpenStatus)); err != nil {
		return nil, fmt.Errorf("write request creation event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create request transaction: %w", err)
	}
	return created, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id int64) (*Request, error) {
	request, err := scanRequest(r.db.QueryRowContext(
		ctx,
		`SELECT `+requestColumns+` FROM requests WHERE id = $1 AND deleted_at IS NULL`,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get request by ID: %w", err)
	}
	return request, nil
}

func (r *postgresRepository) ListByUserID(ctx context.Context, userID int64, limit int) ([]Request, error) {
	return r.list(ctx, `
		SELECT `+requestColumns+`
		FROM requests
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
}

func (r *postgresRepository) ListAll(ctx context.Context, limit int) ([]Request, error) {
	return r.list(ctx, `
		SELECT `+requestColumns+`
		FROM requests
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $1
	`, limit)
}

func (r *postgresRepository) list(ctx context.Context, query string, args ...any) ([]Request, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query requests: %w", err)
	}
	defer rows.Close()

	requests := make([]Request, 0)
	for rows.Next() {
		request, err := scanRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		requests = append(requests, *request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requests: %w", err)
	}
	return requests, nil
}

func (r *postgresRepository) Delete(ctx context.Context, id, actorID int64, onlyOpen bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete request transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status Status
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM requests
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock request for deletion: %w", err)
	}
	if onlyOpen && status != OpenStatus {
		return ErrConflict
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE requests
		SET deleted_at = NOW(), deleted_by_user_id = $1, updated_at = NOW()
		WHERE id = $2
	`, actorID, id); err != nil {
		return fmt.Errorf("soft-delete request: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_events (request_id, actor_user_id, event_type, old_status)
		VALUES ($1, $2, 'deleted', $3)
	`, id, actorID, string(status)); err != nil {
		return fmt.Errorf("write request deletion event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete request transaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id, actorID int64, status Status) (*Request, error) {
	return r.UpdateStatusWithDetails(ctx, id, actorID, status, "")
}

func (r *postgresRepository) UpdateStatusWithDetails(ctx context.Context, id, actorID int64, status Status, details string) (*Request, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin status update transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var oldStatus Status
	var theme string
	var telegramID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT r.status, r.theme, u.telegram_id
		FROM requests r
		JOIN users u ON u.id = r.user_id
		WHERE r.id = $1 AND r.deleted_at IS NULL
		FOR UPDATE OF r
	`, id).Scan(&oldStatus, &theme, &telegramID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock request for status update: %w", err)
	}

	if oldStatus != status {
		if _, err := tx.ExecContext(ctx, `
			UPDATE requests SET status = $1, updated_at = NOW()
			WHERE id = $2
		`, string(status), id); err != nil {
			return nil, fmt.Errorf("update request status: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO request_events (request_id, actor_user_id, event_type, old_status, new_status, details)
			VALUES ($1, $2, 'status_changed', $3, $4, $5)
		`, id, actorID, string(oldStatus), string(status), details); err != nil {
			return nil, fmt.Errorf("write status-change event: %w", err)
		}
		if telegramID.Valid {
			body := statusNotificationText(id, theme, status, details)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO telegram_notifications (telegram_id, request_id, body, status)
				VALUES ($1, $2, $3, $4)
			`, telegramID.Int64, id, body, string(status)); err != nil {
				return nil, fmt.Errorf("queue status notification: %w", err)
			}
		}
	}

	updated, err := scanRequest(tx.QueryRowContext(
		ctx,
		`SELECT `+requestColumns+` FROM requests WHERE id = $1`,
		id,
	))
	if err != nil {
		return nil, fmt.Errorf("read updated request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit status update transaction: %w", err)
	}
	return updated, nil
}

func statusNotificationText(requestID int64, theme string, status Status, reason string) string {
	var title, explanation string
	switch status {
	case OpenStatus:
		title, explanation = "Новая", "Обращение зарегистрировано и ожидает рассмотрения."
	case InProgressStatus:
		title, explanation = "В работе", "Команда школьного самоуправления занимается вашим обращением."
	case WaitingForUserStatus:
		title, explanation = "Ожидает вашего ответа", "По обращению нужна дополнительная информация. Нажмите кнопку ниже, чтобы написать команде."
	case ClosedStatus:
		title, explanation = "Завершена", "Работа над обращением завершена. Спасибо за обратную связь!"
	case RejectedStatus:
		title, explanation = "Отклонена", "К сожалению, обращение не может быть принято."
	default:
		title, explanation = string(status), "Статус обращения изменён."
	}
	message := fmt.Sprintf("Статус обращения №%d: %s\nТема: %s\n\n%s", requestID, title, theme, explanation)
	if status == RejectedStatus && strings.TrimSpace(reason) != "" {
		message += "\n\nПричина: " + strings.TrimSpace(reason)
	}
	if status == ClosedStatus && strings.TrimSpace(reason) != "" {
		message += "\n\nРезультат: " + strings.TrimSpace(reason)
	}

	return message
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (*Request, error) {
	var request Request
	err := row.Scan(
		&request.ID,
		&request.UserID,
		&request.RequestType,
		&request.Theme,
		&request.Description,
		&request.Status,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &request, nil
}
