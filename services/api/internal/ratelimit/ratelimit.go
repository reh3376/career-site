// Package ratelimit is an in-process, per-key token-bucket limiter
// used by high-value endpoints (login is the current caller) to slow
// down credential-stuffing without needing an external cache. Buckets
// are keyed by a string the caller composes (e.g. "login:ip=A|email=B")
// so a caller can pick whatever granularity fits.
//
// Every request consumes one token. If the bucket is empty the limiter
// returns the duration the caller must wait before another attempt
// would succeed, so the HTTP handler can render a Retry-After.
//
// Buckets are lazily created and GC'd on the periodic sweep. All state
// is in-process — a horizontally scaled deployment would need Redis or
// a similar shared store; see project_admin_console_backlog.md for
// that future work.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens     float64
	updatedAt  time.Time
	lastAccess time.Time
}

// Limiter is a per-key token-bucket limiter. Zero-value not usable;
// construct with New.
type Limiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	capacity   float64
	refillRate float64 // tokens per second
	sweepEvery time.Duration
	lastSweep  time.Time
}

// New builds a Limiter with the given bucket capacity and refill
// rate. Example: `New(5, 5.0/900)` = five attempts allowed
// immediately, refilled at 1 token per 180 seconds, so ~5 attempts
// per 15 minutes with no burst allowance beyond the initial 5.
func New(capacity float64, refillPerSecond float64) *Limiter {
	return &Limiter{
		buckets:    map[string]*bucket{},
		capacity:   capacity,
		refillRate: refillPerSecond,
		sweepEvery: 10 * time.Minute,
		lastSweep:  time.Now(),
	}
}

// Allow decrements the caller's bucket by one token. Returns
// `ok=true` on success (request allowed) or `ok=false, retryAfter>0`
// when the bucket is empty. Safe for concurrent use.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.maybeSweep(now)

	b, exists := l.buckets[key]
	if !exists {
		b = &bucket{tokens: l.capacity, updatedAt: now}
		l.buckets[key] = b
	} else {
		// Refill for the elapsed time since last update.
		elapsed := now.Sub(b.updatedAt).Seconds()
		b.tokens += elapsed * l.refillRate
		if b.tokens > l.capacity {
			b.tokens = l.capacity
		}
		b.updatedAt = now
	}
	b.lastAccess = now

	if b.tokens < 1 {
		// Time until we accumulate 1 full token.
		need := 1 - b.tokens
		wait := time.Duration(need/l.refillRate*float64(time.Second)) + 100*time.Millisecond
		return false, wait
	}
	b.tokens--
	return true, 0
}

// maybeSweep evicts buckets that haven't been touched in a while so
// the map doesn't grow unbounded from unique keys.
func (l *Limiter) maybeSweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.sweepEvery {
		return
	}
	cutoff := now.Add(-l.sweepEvery)
	for k, b := range l.buckets {
		if b.lastAccess.Before(cutoff) && b.tokens >= l.capacity {
			delete(l.buckets, k)
		}
	}
	l.lastSweep = now
}
