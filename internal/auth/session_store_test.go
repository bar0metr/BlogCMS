package auth

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_cleanupExpired_RemovesExpiredSessions(t *testing.T) {
	s := NewMemoryStore()

	// Insert two sessions; one already expired.
	s.sessions["alive"] = Session{ID: "alive", UserID: 1, ExpiresAt: time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)}
	s.sessions["dead"] = Session{ID: "dead", UserID: 2, ExpiresAt: time.Date(2026, 1, 7, 9, 0, 0, 0, time.UTC)}

	removed := s.cleanupExpired(time.Date(2026, 1, 7, 9, 30, 0, 0, time.UTC))
	if removed != 1 {
		t.Fatalf("expected removed=1 got %d", removed)
	}
	if _, ok := s.sessions["dead"]; ok {
		t.Fatalf("expected expired session to be removed")
	}
	if _, ok := s.sessions["alive"]; !ok {
		t.Fatalf("expected non-expired session to remain")
	}

	// Ensure Get still respects expiration.
	_, ok := s.Get(context.Background(), "dead")
	if ok {
		t.Fatalf("expected dead session to not be retrievable")
	}
}
