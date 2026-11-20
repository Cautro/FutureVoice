package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBackendClientCreateRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/internal/telegram/requests" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Bot-Secret"); got != "test-secret" {
			t.Errorf("X-Bot-Secret = %q", got)
		}

		var input CreateRequestInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if input.TelegramID != 123 || input.IdempotencyKey != "tg:123:456" {
			t.Errorf("unexpected create input: %+v", input)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Request{ID: 77, Status: RequestOpen, Theme: "Library"})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-secret")
	created, err := client.CreateRequest(context.Background(), CreateRequestInput{
		TelegramID:     123,
		IdempotencyKey: "tg:123:456",
		RequestType:    ComplaintType,
		Theme:          "Library",
		Description:    "The library closes too early.",
	})
	if err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}
	if created.ID != 77 || created.Status != RequestOpen {
		t.Fatalf("unexpected response: %+v", created)
	}
}

func TestBackendClientReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-secret")
	_, err := client.ListRequests(context.Background(), 123, true, 20)
	var backendErr *BackendHTTPError
	if !errors.As(err, &backendErr) {
		t.Fatalf("expected BackendHTTPError, got %v", err)
	}
	if backendErr.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", backendErr.StatusCode, http.StatusForbidden)
	}
}
