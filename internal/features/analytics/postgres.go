package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Overview struct {
	From                   string       `json:"from"`
	To                     string       `json:"to"`
	TotalRequests          int64        `json:"total_requests"`
	OpenRequests           int64        `json:"open_requests"`
	InProgressRequests     int64        `json:"in_progress_requests"`
	WaitingForUserRequests int64        `json:"waiting_for_user_requests"`
	ClosedRequests         int64        `json:"closed_requests"`
	RejectedRequests       int64        `json:"rejected_requests"`
	ResolutionRate         float64      `json:"resolution_rate"`
	AverageResolutionH     *float64     `json:"average_resolution_hours"`
	Daily                  []DailyPoint `json:"daily"`
	ByType                 []CountPoint `json:"by_type"`
	ByStatus               []CountPoint `json:"by_status"`
}

type DailyPoint struct {
	Date     string `json:"date"`
	Created  int64  `json:"created"`
	Resolved int64  `json:"resolved"`
}

type CountPoint struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Overview(ctx context.Context, from, to time.Time) (Overview, error) {
	start := from.UTC()
	endExclusive := to.UTC().AddDate(0, 0, 1)

	result := Overview{
		From:     start.Format("2006-01-02"),
		To:       to.UTC().Format("2006-01-02"),
		Daily:    make([]DailyPoint, 0),
		ByType:   make([]CountPoint, 0),
		ByStatus: make([]CountPoint, 0),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'open'),
			COUNT(*) FILTER (WHERE status = 'in_progress'),
			COUNT(*) FILTER (WHERE status = 'waiting_for_user'),
			COUNT(*) FILTER (WHERE status = 'closed'),
			COUNT(*) FILTER (WHERE status = 'rejected')
		FROM requests
		WHERE deleted_at IS NULL
		  AND created_at >= $1
		  AND created_at < $2
	`, start, endExclusive).Scan(
		&result.TotalRequests,
		&result.OpenRequests,
		&result.InProgressRequests,
		&result.WaitingForUserRequests,
		&result.ClosedRequests,
		&result.RejectedRequests,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("query analytics totals: %w", err)
	}
	if result.TotalRequests > 0 {
		result.ResolutionRate = float64(result.ClosedRequests+result.RejectedRequests) / float64(result.TotalRequests) * 100
	}

	var average sql.NullFloat64
	err = r.db.QueryRowContext(ctx, `
		SELECT AVG(EXTRACT(EPOCH FROM (resolved.first_decision_at - r.created_at)) / 3600.0)
		FROM requests r
		JOIN LATERAL (
			SELECT MIN(e.created_at) AS first_decision_at
			FROM request_events e
			WHERE e.request_id = r.id
			  AND e.event_type = 'status_changed'
			  AND e.new_status IN ('closed', 'rejected')
		) resolved ON resolved.first_decision_at IS NOT NULL
		WHERE r.deleted_at IS NULL
		  AND r.created_at >= $1
		  AND r.created_at < $2
	`, start, endExclusive).Scan(&average)
	if err != nil {
		return Overview{}, fmt.Errorf("query average resolution time: %w", err)
	}
	if average.Valid {
		value := average.Float64
		result.AverageResolutionH = &value
	}

	dailyRows, err := r.db.QueryContext(ctx, `
		WITH days AS (
			SELECT generate_series($1::date, ($2::date - INTERVAL '1 day')::date, INTERVAL '1 day')::date AS day
		),
		created AS (
			SELECT (created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS amount
			FROM requests
			WHERE deleted_at IS NULL
			  AND created_at >= $1 AND created_at < $2
			GROUP BY (created_at AT TIME ZONE 'UTC')::date
		),
		resolved AS (
			SELECT (e.created_at AT TIME ZONE 'UTC')::date AS day, COUNT(DISTINCT e.request_id) AS amount
			FROM request_events e
			JOIN requests r ON r.id = e.request_id
			WHERE r.deleted_at IS NULL
			  AND e.event_type = 'status_changed'
			  AND e.new_status IN ('closed', 'rejected')
			  AND e.created_at >= $1 AND e.created_at < $2
			GROUP BY (e.created_at AT TIME ZONE 'UTC')::date
		)
		SELECT days.day::text, COALESCE(created.amount, 0), COALESCE(resolved.amount, 0)
		FROM days
		LEFT JOIN created ON created.day = days.day
		LEFT JOIN resolved ON resolved.day = days.day
		ORDER BY days.day
	`, start, endExclusive)
	if err != nil {
		return Overview{}, fmt.Errorf("query daily analytics: %w", err)
	}
	for dailyRows.Next() {
		var point DailyPoint
		if err := dailyRows.Scan(&point.Date, &point.Created, &point.Resolved); err != nil {
			dailyRows.Close()
			return Overview{}, fmt.Errorf("scan daily analytics: %w", err)
		}
		result.Daily = append(result.Daily, point)
	}
	if err := dailyRows.Err(); err != nil {
		dailyRows.Close()
		return Overview{}, fmt.Errorf("iterate daily analytics: %w", err)
	}
	dailyRows.Close()

	typeRows, err := r.db.QueryContext(ctx, `
		SELECT request_type, COUNT(*)
		FROM requests
		WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2
		GROUP BY request_type
		ORDER BY COUNT(*) DESC
	`, start, endExclusive)
	if err != nil {
		return Overview{}, fmt.Errorf("query request types: %w", err)
	}
	for typeRows.Next() {
		var point CountPoint
		if err := typeRows.Scan(&point.Key, &point.Count); err != nil {
			typeRows.Close()
			return Overview{}, fmt.Errorf("scan request type: %w", err)
		}
		result.ByType = append(result.ByType, point)
	}
	if err := typeRows.Err(); err != nil {
		typeRows.Close()
		return Overview{}, fmt.Errorf("iterate request types: %w", err)
	}
	typeRows.Close()

	statusRows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM requests
		WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2
		GROUP BY status
		ORDER BY CASE status WHEN 'open' THEN 1 WHEN 'in_progress' THEN 2 WHEN 'waiting_for_user' THEN 3 WHEN 'closed' THEN 4 WHEN 'rejected' THEN 5 ELSE 6 END
	`, start, endExclusive)
	if err != nil {
		return Overview{}, fmt.Errorf("query request statuses: %w", err)
	}
	for statusRows.Next() {
		var point CountPoint
		if err := statusRows.Scan(&point.Key, &point.Count); err != nil {
			statusRows.Close()
			return Overview{}, fmt.Errorf("scan request status: %w", err)
		}
		result.ByStatus = append(result.ByStatus, point)
	}
	if err := statusRows.Err(); err != nil {
		statusRows.Close()
		return Overview{}, fmt.Errorf("iterate request statuses: %w", err)
	}
	statusRows.Close()

	return result, nil
}

func ParseRange(fromRaw, toRaw string, now time.Time) (time.Time, time.Time, error) {
	now = now.UTC()
	defaultTo := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	defaultFrom := defaultTo.AddDate(0, 0, -29)

	from := defaultFrom
	to := defaultTo
	if fromRaw != "" {
		parsed, err := time.Parse("2006-01-02", fromRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("from must use YYYY-MM-DD")
		}
		from = parsed.UTC()
	}
	if toRaw != "" {
		parsed, err := time.Parse("2006-01-02", toRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("to must use YYYY-MM-DD")
		}
		to = parsed.UTC()
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to must not be earlier than from")
	}
	if to.Sub(from) > 365*24*time.Hour {
		return time.Time{}, time.Time{}, errors.New("date range cannot exceed 366 days")
	}
	return from, to, nil
}
