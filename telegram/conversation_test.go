package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmissionRetryKeepsIdempotencyKey(t *testing.T) {
	store := newConversationStore()
	store.Set(123, requestDraft{
		Stage:       stageEnterDescription,
		RequestType: ComplaintType,
		Theme:       "Library",
	})

	first, ok := store.BeginSubmission(123, 456)
	if !ok {
		t.Fatal("BeginSubmission() failed")
	}
	store.RetrySubmission(123, first)

	second, ok := store.BeginSubmission(123, 789)
	if !ok {
		t.Fatal("retry BeginSubmission() failed")
	}
	if first.IdempotencyKey == "" || second.IdempotencyKey != first.IdempotencyKey {
		t.Fatalf("idempotency key changed between attempts: %q -> %q", first.IdempotencyKey, second.IdempotencyKey)
	}
}

func TestDeleteExpiredRemovesStaleDrafts(t *testing.T) {
	store := newConversationStore()
	store.Set(123, requestDraft{Stage: stageChooseType})
	store.deleteExpired(time.Now().Add(31 * time.Minute))

	if _, ok := store.Get(123); ok {
		t.Fatal("expired draft was not removed")
	}
}

func TestLockUserSerializesUpdatesForSameUser(t *testing.T) {
	store := newConversationStore()
	var current int32
	var maximum int32
	done := make(chan struct{}, 2)

	for range 2 {
		go func() {
			unlock := store.LockUser(123)
			active := atomic.AddInt32(&current, 1)
			for {
				observed := atomic.LoadInt32(&maximum)
				if active <= observed || atomic.CompareAndSwapInt32(&maximum, observed, active) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&current, -1)
			unlock()
			done <- struct{}{}
		}()
	}

	<-done
	<-done
	if got := atomic.LoadInt32(&maximum); got != 1 {
		t.Fatalf("maximum concurrent updates = %d, want 1", got)
	}
}
