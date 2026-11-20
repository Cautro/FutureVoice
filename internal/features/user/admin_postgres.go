package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (r *postgresRepository) ListAdminUsers(ctx context.Context, role Role, query string, page, limit int) ([]User, int64, error) {
	query = strings.TrimSpace(query)
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE ($1 = '' OR role = $1)
		  AND ($2 = '' OR username ILIKE '%' || $2 || '%'
		       OR first_name ILIKE '%' || $2 || '%'
		       OR last_name ILIKE '%' || $2 || '%'
		       OR CAST(telegram_id AS TEXT) = $2)
	`, string(role), query).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count admin users: %w", err)
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE ($1 = '' OR role = $1)
		  AND ($2 = '' OR username ILIKE '%' || $2 || '%'
		       OR first_name ILIKE '%' || $2 || '%'
		       OR last_name ILIKE '%' || $2 || '%'
		       OR CAST(telegram_id AS TEXT) = $2)
		ORDER BY
			CASE role WHEN 'admin' THEN 0 WHEN 'moderator' THEN 1 ELSE 2 END,
			id DESC
		LIMIT $3 OFFSET $4
	`, string(role), query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan admin user: %w", err)
		}
		users = append(users, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate admin users: %w", err)
	}
	return users, total, nil
}

func (r *postgresRepository) UpdateRole(ctx context.Context, actorID, targetID int64, role Role) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin role update transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockActiveAdministrators(ctx, tx); err != nil {
		return err
	}

	var oldRole Role
	var targetActive bool
	err = tx.QueryRowContext(ctx, `
		SELECT role, is_active FROM users WHERE id = $1 FOR UPDATE
	`, targetID).Scan(&oldRole, &targetActive)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user for role update: %w", err)
	}
	if oldRole == role {
		return tx.Commit()
	}
	if oldRole == AdminRole && role != AdminRole && targetActive {
		var activeAdmins int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = TRUE
		`).Scan(&activeAdmins); err != nil {
			return fmt.Errorf("count active administrators: %w", err)
		}
		if activeAdmins <= 1 {
			return ErrLastAdmin
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2
	`, string(role), targetID); err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_admin_events (actor_user_id, target_user_id, event_type, old_value, new_value)
		VALUES ($1, $2, 'role_changed', $3, $4)
	`, actorID, targetID, string(oldRole), string(role)); err != nil {
		return fmt.Errorf("write user role event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit role update: %w", err)
	}
	return nil
}

func (r *postgresRepository) UpdateActive(ctx context.Context, actorID, targetID int64, active bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user activity transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockActiveAdministrators(ctx, tx); err != nil {
		return err
	}

	var role Role
	var oldActive bool
	err = tx.QueryRowContext(ctx, `
		SELECT role, is_active FROM users WHERE id = $1 FOR UPDATE
	`, targetID).Scan(&role, &oldActive)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user for activity update: %w", err)
	}
	if oldActive == active {
		return tx.Commit()
	}
	if role == AdminRole && oldActive && !active {
		var activeAdmins int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = TRUE
		`).Scan(&activeAdmins); err != nil {
			return fmt.Errorf("count active administrators: %w", err)
		}
		if activeAdmins <= 1 {
			return ErrLastAdmin
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2
	`, active, targetID); err != nil {
		return fmt.Errorf("update user activity: %w", err)
	}
	eventType := "user_deactivated"
	if active {
		eventType = "user_activated"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_admin_events (actor_user_id, target_user_id, event_type, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5)
	`, actorID, targetID, eventType, fmt.Sprint(oldActive), fmt.Sprint(active)); err != nil {
		return fmt.Errorf("write user activity event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE admin_sessions SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL AND $2 = FALSE
	`, targetID, active); err != nil {
		return fmt.Errorf("revoke sessions for deactivated user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user activity update: %w", err)
	}
	return nil
}

// lockActiveAdministrators serializes changes to the administrator roster. Without
// this lock, two concurrent transactions could each observe two admins and
// demote/deactivate a different account, leaving the system without an admin.
func lockActiveAdministrators(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM users
		WHERE role = 'admin' AND is_active = TRUE
		ORDER BY id
		FOR UPDATE
	`)
	if err != nil {
		return fmt.Errorf("lock active administrators: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan active administrator: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate active administrators: %w", err)
	}
	return nil
}
