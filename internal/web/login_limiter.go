package web

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

type loginLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	attempts map[string][]time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		max:      max,
		window:   window,
		attempts: make(map[string][]time.Time),
	}
}

func (l *loginLimiter) Allow(r *http.Request) (bool, time.Duration) {
	ip := clientIP(r)

	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	list := l.attempts[ip]
	// drop old
	n := 0
	for _, t := range list {
		if t.After(cutoff) {
			list[n] = t
			n++
		}
	}
	list = list[:n]

	if len(list) >= l.max {
		retryAfter := l.window - now.Sub(list[0])
		if retryAfter < 0 {
			retryAfter = 0
		}
		l.attempts[ip] = list
		return false, retryAfter
	}

	list = append(list, now)
	l.attempts[ip] = list
	return true, 0
}

func (l *loginLimiter) StartJanitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	t := time.NewTicker(interval)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				l.cleanup(now)
			}
		}
	}()
}

func (l *loginLimiter) cleanup(now time.Time) {
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	for ip, list := range l.attempts {
		n := 0
		for _, t := range list {
			if t.After(cutoff) {
				list[n] = t
				n++
			}
		}
		if n == 0 {
			delete(l.attempts, ip)
			continue
		}
		l.attempts[ip] = list[:n]
	}
}

func clientIP(r *http.Request) string {
	// If you run behind a reverse proxy, terminate and set X-Forwarded-For there,
	// then trust it explicitly. This project defaults to RemoteAddr for safety.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
