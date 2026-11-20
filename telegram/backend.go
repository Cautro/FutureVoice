package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RequestType string

const (
	ComplaintType    RequestType = "complaint"
	OfferType        RequestType = "offer"
	Print3DOfferType RequestType = "3dPrintOffer"
)

type RequestStatus string

const (
	RequestOpen           RequestStatus = "open"
	RequestInProgress     RequestStatus = "in_progress"
	RequestWaitingForUser RequestStatus = "waiting_for_user"
	RequestClosed         RequestStatus = "closed"
	RequestRejected       RequestStatus = "rejected"
)

type TelegramUserRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}

type TelegramUserResponse struct {
	ID         int64  `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	Role       string `json:"role"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	IsActive   bool   `json:"is_active"`
}

type Request struct {
	ID          int64         `json:"id"`
	UserID      int64         `json:"user_id"`
	Status      RequestStatus `json:"status"`
	RequestType RequestType   `json:"request_type"`
	Theme       string        `json:"theme"`
	Description string        `json:"description"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type RequestMessage struct {
	ID         int64     `json:"id"`
	RequestID  int64     `json:"request_id"`
	AuthorName string    `json:"author_name"`
	AuthorRole string    `json:"author_role"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type TelegramNotification struct {
	ID         int64         `json:"id"`
	TelegramID int64         `json:"telegram_id"`
	RequestID  int64         `json:"request_id"`
	Body       string        `json:"body"`
	Status     RequestStatus `json:"status"`
}

type CreateRequestInput struct {
	TelegramID     int64       `json:"telegram_id"`
	IdempotencyKey string      `json:"idempotency_key"`
	RequestType    RequestType `json:"request_type"`
	Theme          string      `json:"theme"`
	Description    string      `json:"description"`
}

type updateRequestStatusInput struct {
	TelegramID int64         `json:"telegram_id"`
	Status     RequestStatus `json:"status"`
}

type BackendHTTPError struct {
	StatusCode int
	Message    string
}

func (e *BackendHTTPError) Error() string {
	return fmt.Sprintf("backend returned HTTP %d: %s", e.StatusCode, e.Message)
}

type BackendClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func NewBackendClient(baseURL, secret string) *BackendClient {
	return &BackendClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		secret:  strings.TrimSpace(secret),
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (c *BackendClient) RegisterTelegramUser(ctx context.Context, profile TelegramUserRequest) (*TelegramUserResponse, error) {
	if profile.TelegramID <= 0 {
		return nil, errors.New("Telegram user ID must be positive")
	}

	var result TelegramUserResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/internal/telegram/users", nil, profile, &result); err != nil {
		return nil, fmt.Errorf("register Telegram user: %w", err)
	}
	return &result, nil
}

func (c *BackendClient) CreateRequest(ctx context.Context, input CreateRequestInput) (*Request, error) {
	var result Request
	if err := c.do(ctx, http.MethodPost, "/api/v1/internal/telegram/requests", nil, input, &result); err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return &result, nil
}

func (c *BackendClient) ListRequests(ctx context.Context, telegramID int64, all bool, limit int) ([]Request, error) {
	query := url.Values{}
	query.Set("telegram_id", strconv.FormatInt(telegramID, 10))
	query.Set("limit", strconv.Itoa(limit))
	if all {
		query.Set("scope", "all")
	}

	var result []Request
	if err := c.do(ctx, http.MethodGet, "/api/v1/internal/telegram/requests", query, nil, &result); err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	return result, nil
}

func (c *BackendClient) GetRequest(ctx context.Context, telegramID, requestID int64) (*Request, error) {
	query := url.Values{}
	query.Set("telegram_id", strconv.FormatInt(telegramID, 10))
	path := "/api/v1/internal/telegram/requests/" + strconv.FormatInt(requestID, 10)
	var result Request
	if err := c.do(ctx, http.MethodGet, path, query, nil, &result); err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return &result, nil
}

func (c *BackendClient) DeleteRequest(ctx context.Context, telegramID, requestID int64) error {
	query := url.Values{}
	query.Set("telegram_id", strconv.FormatInt(telegramID, 10))
	path := "/api/v1/internal/telegram/requests/" + strconv.FormatInt(requestID, 10)
	if err := c.do(ctx, http.MethodDelete, path, query, nil, nil); err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	return nil
}

func (c *BackendClient) ListRequestMessages(ctx context.Context, telegramID, requestID int64) ([]RequestMessage, error) {
	query := url.Values{}
	query.Set("telegram_id", strconv.FormatInt(telegramID, 10))
	path := "/api/v1/internal/telegram/requests/" + strconv.FormatInt(requestID, 10) + "/messages"
	var result []RequestMessage
	if err := c.do(ctx, http.MethodGet, path, query, nil, &result); err != nil {
		return nil, fmt.Errorf("list request messages: %w", err)
	}
	return result, nil
}

func (c *BackendClient) AddRequestMessage(ctx context.Context, telegramID, requestID int64, body string) error {
	input := struct {
		TelegramID int64  `json:"telegram_id"`
		Body       string `json:"body"`
	}{TelegramID: telegramID, Body: body}
	path := "/api/v1/internal/telegram/requests/" + strconv.FormatInt(requestID, 10) + "/messages"
	if err := c.do(ctx, http.MethodPost, path, nil, input, nil); err != nil {
		return fmt.Errorf("send request message: %w", err)
	}
	return nil
}

func (c *BackendClient) ListTelegramNotifications(ctx context.Context, limit int) ([]TelegramNotification, error) {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	var result []TelegramNotification
	if err := c.do(ctx, http.MethodGet, "/api/v1/internal/telegram/notifications", query, nil, &result); err != nil {
		return nil, fmt.Errorf("list Telegram notifications: %w", err)
	}
	return result, nil
}

func (c *BackendClient) MarkTelegramNotificationDelivered(ctx context.Context, id int64) error {
	path := "/api/v1/internal/telegram/notifications/" + strconv.FormatInt(id, 10) + "/delivered"
	if err := c.do(ctx, http.MethodPost, path, nil, nil, nil); err != nil {
		return fmt.Errorf("acknowledge Telegram notification: %w", err)
	}
	return nil
}

func (c *BackendClient) FailTelegramNotification(ctx context.Context, id int64, cause error) error {
	message := "delivery failed"
	if cause != nil {
		message = cause.Error()
	}
	path := "/api/v1/internal/telegram/notifications/" + strconv.FormatInt(id, 10) + "/failed"
	if err := c.do(ctx, http.MethodPost, path, nil, map[string]string{"error": message}, nil); err != nil {
		return fmt.Errorf("reschedule Telegram notification: %w", err)
	}
	return nil
}

func (c *BackendClient) UpdateRequestStatus(ctx context.Context, telegramID, requestID int64, status RequestStatus) (*Request, error) {
	input := updateRequestStatusInput{TelegramID: telegramID, Status: status}
	path := "/api/v1/internal/telegram/requests/" + strconv.FormatInt(requestID, 10) + "/status"
	var result Request
	if err := c.do(ctx, http.MethodPatch, path, nil, input, &result); err != nil {
		return nil, fmt.Errorf("update request status: %w", err)
	}
	return &result, nil
}

func (c *BackendClient) do(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body any,
	result any,
) error {
	if c == nil || c.baseURL == "" {
		return errors.New("backend URL is not configured")
	}
	if c.secret == "" {
		return errors.New("bot API secret is not configured")
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return fmt.Errorf("build backend request: %w", err)
	}
	req.Header.Set("X-Bot-Secret", c.secret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send backend request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return &BackendHTTPError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(message)),
		}
	}

	if result == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result); err != nil {
		return fmt.Errorf("decode backend response: %w", err)
	}
	return nil
}
