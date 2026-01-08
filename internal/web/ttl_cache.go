package web

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// ttlCache is a small in-memory cache with TTL and singleflight loading.
// It is used to reduce repeated database reads on hot pages.
type ttlCache[T any] struct {
	mu  sync.RWMutex
	val T
	exp time.Time
	sf  singleflight.Group
}

func (c *ttlCache[T]) GetOrLoad(ctx context.Context, ttl time.Duration, loader func(context.Context) (T, error)) (T, error) {
	// Fast path.
	c.mu.RLock()
	if !c.exp.IsZero() && time.Now().Before(c.exp) {
		v := c.val
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	// Singleflight load.
	v, err, _ := c.sf.Do("k", func() (any, error) {
		c.mu.RLock()
		if !c.exp.IsZero() && time.Now().Before(c.exp) {
			vv := c.val
			c.mu.RUnlock()
			return vv, nil
		}
		c.mu.RUnlock()

		vv, e := loader(ctx)
		if e != nil {
			var zero T
			return zero, e
		}
		c.mu.Lock()
		c.val = vv
		c.exp = time.Now().Add(ttl)
		c.mu.Unlock()
		return vv, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}

func (c *ttlCache[T]) Invalidate() {
	c.mu.Lock()
	var zero T
	c.val = zero
	c.exp = time.Time{}
	c.mu.Unlock()
}
