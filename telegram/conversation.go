package main

import (
	"context"
	"sync"
	"time"
)

type conversationStage uint8

const (
	stageChooseType conversationStage = iota + 1
	stageEnterTheme
	stageEnterDescription
	stageSubmitting
	stageReplyMessage
)

type requestDraft struct {
	Stage          conversationStage
	RequestType    RequestType
	Theme          string
	RequestID      int64
	Status         RequestStatus
	IsAppeal       bool
	IdempotencyKey string
	ExpiresAt      time.Time
}

type conversationLock struct {
	mu   sync.Mutex
	refs int
}

// conversationStore keeps short-lived Telegram form state in memory.
// Persisted requests are stored by the backend; only unfinished drafts live here.
type conversationStore struct {
	mu     sync.Mutex
	drafts map[int64]requestDraft
	locks  map[int64]*conversationLock
}

func newConversationStore() *conversationStore {
	return &conversationStore{
		drafts: make(map[int64]requestDraft),
		locks:  make(map[int64]*conversationLock),
	}
}

// LockUser prevents two updates from mutating the same user's draft concurrently.
// Telego processes updates concurrently, so draft state must be protected per user.
func (s *conversationStore) LockUser(telegramID int64) func() {
	s.mu.Lock()
	lock := s.locks[telegramID]
	if lock == nil {
		lock = &conversationLock{}
		s.locks[telegramID] = lock
	}
	lock.refs++
	s.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()

		s.mu.Lock()
		defer s.mu.Unlock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.locks, telegramID)
		}
	}
}

func (s *conversationStore) Set(telegramID int64, draft requestDraft) {
	draft.ExpiresAt = time.Now().Add(30 * time.Minute)

	s.mu.Lock()
	s.drafts[telegramID] = draft
	s.mu.Unlock()
}

func (s *conversationStore) Get(telegramID int64) (requestDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	draft, ok := s.drafts[telegramID]
	if !ok {
		return requestDraft{}, false
	}
	if time.Now().After(draft.ExpiresAt) {
		delete(s.drafts, telegramID)
		return requestDraft{}, false
	}
	return draft, true
}

func (s *conversationStore) BeginSubmission(telegramID int64, messageID int) (requestDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	draft, ok := s.drafts[telegramID]
	if !ok || time.Now().After(draft.ExpiresAt) || draft.Stage != stageEnterDescription {
		return requestDraft{}, false
	}
	if draft.IdempotencyKey == "" {
		draft.IdempotencyKey = idempotencyKey(telegramID, messageID)
	}
	draft.Stage = stageSubmitting
	draft.ExpiresAt = time.Now().Add(30 * time.Minute)
	s.drafts[telegramID] = draft
	return draft, true
}

func (s *conversationStore) RetrySubmission(telegramID int64, draft requestDraft) {
	draft.Stage = stageEnterDescription
	s.Set(telegramID, draft)
}

func (s *conversationStore) Clear(telegramID int64) {
	s.mu.Lock()
	delete(s.drafts, telegramID)
	s.mu.Unlock()
}

func (s *conversationStore) runCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.deleteExpired(now)
		}
	}
}

func (s *conversationStore) deleteExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for telegramID, draft := range s.drafts {
		if !now.Before(draft.ExpiresAt) {
			delete(s.drafts, telegramID)
		}
	}
}
