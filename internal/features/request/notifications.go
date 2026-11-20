package request

import (
	"context"
	"database/sql"
	"fmt"
)

func (r *postgresRepository) ListPendingTelegramNotifications(ctx context.Context, limit int) ([]TelegramNotification, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin Telegram notification claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT n.id, n.telegram_id, n.request_id, n.body, COALESCE(NULLIF(n.status, ''), r.status, '')
		FROM telegram_notifications n
		LEFT JOIN requests r ON r.id = n.request_id
		WHERE n.delivered_at IS NULL
		  AND n.available_at <= NOW()
		  AND (n.locked_until IS NULL OR n.locked_until < NOW())
		ORDER BY n.id
		LIMIT $1
		FOR UPDATE OF n SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("select pending Telegram notifications: %w", err)
	}
	items := make([]TelegramNotification, 0)
	for rows.Next() {
		var item TelegramNotification
		if err := rows.Scan(&item.ID, &item.TelegramID, &item.RequestID, &item.Body, &item.Status); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan pending Telegram notification: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate pending Telegram notifications: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close pending Telegram notifications: %w", err)
	}

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			UPDATE telegram_notifications
			SET locked_until = NOW() + INTERVAL '1 minute'
			WHERE id = $1 AND delivered_at IS NULL
		`, item.ID); err != nil {
			return nil, fmt.Errorf("lease Telegram notification %d: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit Telegram notification claim: %w", err)
	}
	return items, nil
}

func (r *postgresRepository) MarkTelegramNotificationDelivered(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET delivered_at = NOW(), locked_until = NULL, last_error = ''
		WHERE id = $1 AND delivered_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("mark Telegram notification delivered: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		var exists bool
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telegram_notifications WHERE id = $1)`, id).Scan(&exists); err != nil {
			return fmt.Errorf("check Telegram notification: %w", err)
		}
		if !exists {
			return sql.ErrNoRows
		}
	}
	return nil
}

func (r *postgresRepository) FailTelegramNotification(ctx context.Context, id int64, message string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE telegram_notifications
		SET attempts = attempts + 1,
		    last_error = $2,
		    available_at = NOW() + (LEAST(900, 5 * POWER(2, LEAST(attempts, 8))) * INTERVAL '1 second'),
		    locked_until = NULL
		WHERE id = $1 AND delivered_at IS NULL
	`, id, message)
	if err != nil {
		return fmt.Errorf("reschedule Telegram notification: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
