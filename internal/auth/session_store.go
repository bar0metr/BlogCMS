package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	ID        string
	UserID    int64
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Store interface {
	Create(ctx context.Context, userID int64, ttl time.Duration) (Session, error)
	Get(ctx context.Context, id string) (Session, bool)
	Delete(ctx context.Context, id string)
}

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]Session)}
}

func (s *MemoryStore) Create(_ context.Context, userID int64, ttl time.Duration) (Session, error) {
	now := time.Now()
	id, err := randomHex(32)
	if err != nil {
		return Session{}, err
	}
	ss := Session{
		ID:        id,
		UserID:    userID,
		CSRFToken: "",
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	s.mu.Lock()
	s.sessions[id] = ss
	s.mu.Unlock()
	return ss, nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (Session, bool) {
	s.mu.RLock()
	ss, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().After(ss.ExpiresAt) {
		s.Delete(context.Background(), id)
		return Session{}, false
	}
	return ss, true
}

func (s *MemoryStore) Delete(_ context.Context, id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// StartJanitor starts a background goroutine that periodically removes expired sessions.
// It exits when ctx is done.
func (s *MemoryStore) StartJanitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	t := time.NewTicker(interval)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				s.cleanupExpired(now)
			}
		}
	}()
}

func (s *MemoryStore) cleanupExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, ss := range s.sessions {
		if now.After(ss.ExpiresAt) {
			delete(s.sessions, id)
			removed++
		}
	}
	return removed
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
