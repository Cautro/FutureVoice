package request

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (r *postgresRepository) ListAdmin(ctx context.Context, filter AdminFilter) (AdminList, error) {
	offset := (filter.Page - 1) * filter.Limit
	search := strings.TrimSpace(filter.Query)

	var total int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM requests r
		JOIN users author ON author.id = r.user_id
		WHERE r.deleted_at IS NULL
		  AND ($1 = '' OR r.status = $1)
		  AND ($2 = '' OR r.request_type = $2)
		  AND ($3 = '' OR
		       r.theme ILIKE '%' || $3 || '%' OR
		       r.description ILIKE '%' || $3 || '%' OR
		       author.username ILIKE '%' || $3 || '%' OR
		       author.first_name ILIKE '%' || $3 || '%' OR
		       author.last_name ILIKE '%' || $3 || '%' OR
		       CAST(r.id AS TEXT) = $3)
	`, string(filter.Status), string(filter.Type), search).Scan(&total)
	if err != nil {
		return AdminList{}, fmt.Errorf("count admin requests: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			r.id, r.user_id, author.username,
			author.first_name, author.last_name,
			r.assigned_to, COALESCE(assignee.username, ''),
			COALESCE(assignee.first_name, ''),
			r.request_type, r.theme, r.description, r.status,
			r.created_at, r.updated_at
		FROM requests r
		JOIN users author ON author.id = r.user_id
		LEFT JOIN users assignee ON assignee.id = r.assigned_to
		WHERE r.deleted_at IS NULL
		  AND ($1 = '' OR r.status = $1)
		  AND ($2 = '' OR r.request_type = $2)
		  AND ($3 = '' OR
		       r.theme ILIKE '%' || $3 || '%' OR
		       r.description ILIKE '%' || $3 || '%' OR
		       author.username ILIKE '%' || $3 || '%' OR
		       author.first_name ILIKE '%' || $3 || '%' OR
		       author.last_name ILIKE '%' || $3 || '%' OR
		       CAST(r.id AS TEXT) = $3)
		ORDER BY
			CASE WHEN r.status = 'open' THEN 0
			     WHEN r.status = 'in_progress' THEN 1
			     ELSE 2 END,
			r.created_at DESC, r.id DESC
		LIMIT $4 OFFSET $5
	`, string(filter.Status), string(filter.Type), search, filter.Limit, offset)
	if err != nil {
		return AdminList{}, fmt.Errorf("query admin requests: %w", err)
	}
	defer rows.Close()

	items := make([]AdminRequest, 0)
	for rows.Next() {
		item, err := scanAdminRequest(rows)
		if err != nil {
			return AdminList{}, fmt.Errorf("scan admin request: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AdminList{}, fmt.Errorf("iterate admin requests: %w", err)
	}

	return AdminList{Items: items, Total: total, Page: filter.Page, Limit: filter.Limit}, nil
}

func (r *postgresRepository) GetAdminByID(ctx context.Context, id int64) (*AdminRequest, error) {
	item, err := scanAdminRequest(r.db.QueryRowContext(ctx, `
		SELECT
			r.id, r.user_id, author.username,
			author.first_name, author.last_name,
			r.assigned_to, COALESCE(assignee.username, ''),
			COALESCE(assignee.first_name, ''),
			r.request_type, r.theme, r.description, r.status,
			r.created_at, r.updated_at
		FROM requests r
		JOIN users author ON author.id = r.user_id
		LEFT JOIN users assignee ON assignee.id = r.assigned_to
		WHERE r.id = $1 AND r.deleted_at IS NULL
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get admin request: %w", err)
	}
	return &item, nil
}

func (r *postgresRepository) ListComments(ctx context.Context, requestID int64) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.id, c.request_id, c.author_user_id,
			COALESCE(NULLIF(users.first_name, ''), users.username, 'Пользователь'),
			c.body, c.visibility, users.role, c.created_at
		FROM request_comments c
		JOIN users ON users.id = c.author_user_id
		WHERE c.request_id = $1
		ORDER BY c.created_at, c.id
	`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list request comments: %w", err)
	}
	defer rows.Close()

	comments := make([]Comment, 0)
	for rows.Next() {
		var item Comment
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&item.AuthorID,
			&item.AuthorName,
			&item.Body,
			&item.Visibility,
			&item.AuthorRole,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request comment: %w", err)
		}
		comments = append(comments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request comments: %w", err)
	}
	return comments, nil
}

func (r *postgresRepository) ListEvents(ctx context.Context, requestID int64) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			e.id, e.request_id, e.actor_user_id,
			COALESCE(NULLIF(users.first_name, ''), users.username, 'Пользователь'),
			e.event_type, e.old_status, e.new_status,
			e.old_assignee_id, e.new_assignee_id,
			e.details, e.created_at
		FROM request_events e
		JOIN users ON users.id = e.actor_user_id
		WHERE e.request_id = $1
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT 100
	`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list request events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var item Event
		var oldStatus, newStatus sql.NullString
		var oldAssignee, newAssignee sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&item.ActorID,
			&item.ActorName,
			&item.EventType,
			&oldStatus,
			&newStatus,
			&oldAssignee,
			&newAssignee,
			&item.Details,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request event: %w", err)
		}
		if oldStatus.Valid {
			status := Status(oldStatus.String)
			item.OldStatus = &status
		}
		if newStatus.Valid {
			status := Status(newStatus.String)
			item.NewStatus = &status
		}
		if oldAssignee.Valid {
			value := oldAssignee.Int64
			item.OldAssigneeID = &value
		}
		if newAssignee.Valid {
			value := newAssignee.Int64
			item.NewAssigneeID = &value
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request events: %w", err)
	}
	return events, nil
}

func (r *postgresRepository) Assign(ctx context.Context, requestID, actorID int64, assigneeID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin request assignment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var oldAssignee sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT assigned_to FROM requests
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, requestID).Scan(&oldAssignee)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock request for assignment: %w", err)
	}

	var newValue any
	if assigneeID != nil {
		var active bool
		var role string
		err := tx.QueryRowContext(ctx, `
			SELECT is_active, role FROM users WHERE id = $1 FOR UPDATE
		`, *assigneeID).Scan(&active, &role)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidAssignee
		}
		if err != nil {
			return fmt.Errorf("validate request assignee: %w", err)
		}
		if !active || (role != "admin" && role != "moderator") {
			return ErrInvalidAssignee
		}
		newValue = *assigneeID
	}
	if (oldAssignee.Valid && assigneeID != nil && oldAssignee.Int64 == *assigneeID) ||
		(!oldAssignee.Valid && assigneeID == nil) {
		return tx.Commit()
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE requests SET assigned_to = $1, updated_at = NOW()
		WHERE id = $2
	`, newValue, requestID); err != nil {
		return fmt.Errorf("update request assignee: %w", err)
	}

	var oldValue any
	if oldAssignee.Valid {
		oldValue = oldAssignee.Int64
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_events (
			request_id, actor_user_id, event_type,
			old_assignee_id, new_assignee_id
		) VALUES ($1, $2, 'assignee_changed', $3, $4)
	`, requestID, actorID, oldValue, newValue); err != nil {
		return fmt.Errorf("write assignment event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit request assignment: %w", err)
	}
	return nil
}

func (r *postgresRepository) AddComment(ctx context.Context, requestID, authorID int64, body string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin comment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT TRUE FROM requests
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, requestID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check request for comment: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_comments (request_id, author_user_id, body, visibility)
		VALUES ($1, $2, $3, 'internal')
	`, requestID, authorID, body); err != nil {
		return fmt.Errorf("insert request comment: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_events (request_id, actor_user_id, event_type, details)
		VALUES ($1, $2, 'internal_comment_added', 'Добавлен внутренний комментарий')
	`, requestID, authorID); err != nil {
		return fmt.Errorf("write comment event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit comment transaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) AddStaffMessage(ctx context.Context, requestID, authorID int64, body, visibility string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staff message transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var theme string
	var currentStatus Status
	var telegramID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT r.theme, r.status, u.telegram_id
		FROM requests r
		JOIN users u ON u.id = r.user_id
		WHERE r.id = $1 AND r.deleted_at IS NULL
		FOR UPDATE OF r
	`, requestID).Scan(&theme, &currentStatus, &telegramID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock request for staff message: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_comments (request_id, author_user_id, body, visibility)
		VALUES ($1, $2, $3, $4)
	`, requestID, authorID, body, visibility); err != nil {
		return fmt.Errorf("insert staff message: %w", err)
	}

	eventType, eventDetails := "internal_comment_added", "Добавлена внутренняя заметка"
	if visibility == "student" {
		eventType, eventDetails = "staff_message_added", "Отправлен ответ пользователю"
		if telegramID.Valid {
			notification := fmt.Sprintf("Ответ по обращению №%d «%s» от команды FutureVoice:\n\n%s", requestID, theme, body)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO telegram_notifications (telegram_id, request_id, body, status)
				VALUES ($1, $2, $3, $4)
			`, telegramID.Int64, requestID, notification, string(currentStatus)); err != nil {
				return fmt.Errorf("queue staff message notification: %w", err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_events (request_id, actor_user_id, event_type, details)
		VALUES ($1, $2, $3, $4)
	`, requestID, authorID, eventType, eventDetails); err != nil {
		return fmt.Errorf("write staff message event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff message transaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) AddStudentMessage(ctx context.Context, requestID, authorID int64, body string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin student message transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	err = tx.QueryRowContext(ctx, `
		SELECT TRUE FROM requests
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`, requestID, authorID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check request ownership for student message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_comments (request_id, author_user_id, body, visibility)
		VALUES ($1, $2, $3, 'student')
	`, requestID, authorID, body); err != nil {
		return fmt.Errorf("insert student message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO request_events (request_id, actor_user_id, event_type, details)
		VALUES ($1, $2, 'student_message_added', 'Получено сообщение от пользователя')
	`, requestID, authorID); err != nil {
		return fmt.Errorf("write student message event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit student message transaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListStudentMessages(ctx context.Context, requestID int64) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.request_id, c.author_user_id,
		       COALESCE(NULLIF(u.first_name, ''), u.username, 'Пользователь'),
		       c.body, c.visibility, u.role, c.created_at
		FROM request_comments c
		JOIN users u ON u.id = c.author_user_id
		WHERE c.request_id = $1 AND c.visibility = 'student'
		ORDER BY c.created_at, c.id
	`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list student-visible request messages: %w", err)
	}
	defer rows.Close()

	items := make([]Comment, 0)
	for rows.Next() {
		var item Comment
		if err := rows.Scan(&item.ID, &item.RequestID, &item.AuthorID, &item.AuthorName, &item.Body, &item.Visibility, &item.AuthorRole, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan student-visible message: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student-visible messages: %w", err)
	}
	return items, nil
}

type adminRequestScanner interface {
	Scan(dest ...any) error
}

func scanAdminRequest(row adminRequestScanner) (AdminRequest, error) {
	var item AdminRequest
	var assignedTo sql.NullInt64
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.AuthorUsername,
		&item.AuthorFirstName,
		&item.AuthorLastName,
		&assignedTo,
		&item.AssigneeUsername,
		&item.AssigneeFirstName,
		&item.RequestType,
		&item.Theme,
		&item.Description,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return AdminRequest{}, err
	}
	if assignedTo.Valid {
		value := assignedTo.Int64
		item.AssignedTo = &value
	}
	return item, nil
}
