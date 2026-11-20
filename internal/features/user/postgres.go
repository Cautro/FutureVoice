package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type postgresRepository struct {
	db *sql.DB
}

func New(db *sql.DB) UserRepository {
	return &postgresRepository{db: db}
}

const userColumns = `id, telegram_id, role, username, first_name, last_name, is_active, created_at`

func (r *postgresRepository) GetOrCreateTelegramUser(ctx context.Context, profile TelegramProfile) (*User, error) {
	if profile.TelegramID <= 0 {
		return nil, errors.New("Telegram ID must be positive")
	}

	query := `
		INSERT INTO users (telegram_id, role, username, first_name, last_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			updated_at = NOW()
		RETURNING ` + userColumns

	user, err := scanUser(r.db.QueryRowContext(
		ctx,
		query,
		profile.TelegramID,
		string(StudentRole),
		profile.Username,
		profile.FirstName,
		profile.LastName,
	))
	if err != nil {
		return nil, fmt.Errorf("get or create Telegram user: %w", err)
	}
	return user, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by ID: %w", err)
	}
	return user, nil
}

func (r *postgresRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE telegram_id = $1`, telegramID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by Telegram ID: %w", err)
	}
	return user, nil
}

func (r *postgresRepository) ListByRole(ctx context.Context, role Role) ([]User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE role = $1 ORDER BY id`,
		string(role),
	)
	if err != nil {
		return nil, fmt.Errorf("list users by role: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var user User
	var telegramID sql.NullInt64
	if err := row.Scan(
		&user.ID,
		&telegramID,
		&user.Role,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.IsActive,
		&user.CreatedAt,
	); err != nil {
		return nil, err
	}
	if telegramID.Valid {
		user.TelegramID = telegramID.Int64
	}
	return &user, nil
}
