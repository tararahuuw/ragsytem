// Package ratelimit is an in-memory, per-key token-bucket rate limiter with
// per-category limits — a Go adaptation of elArch's Bucket4j approach (minus
// Redis; single-instance for now, swap to a distributed store for multi-instance).
package ratelimit

import (
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Config holds per-category limits (requests per minute). A category with limit
// <= 0 (and no default) is treated as unlimited.
type Config struct {
	Enabled          bool
	PerMinute        map[string]int // category -> limit/min
	DefaultPerMinute int
}

type entry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// Limiter tracks one token bucket per (category, key). Buckets are evicted when
// idle to bound memory.
type Limiter struct {
	cfg     Config
	mu      sync.Mutex
	buckets map[string]*entry
}

// New builds a Limiter and starts its idle-bucket janitor.
func New(cfg Config) *Limiter {
	l := &Limiter{cfg: cfg, buckets: make(map[string]*entry)}
	go l.janitor()
	return l
}

// limitFor returns the per-minute limit for a category (0 = unlimited).
func (l *Limiter) limitFor(category string) int {
	if v, ok := l.cfg.PerMinute[category]; ok && v > 0 {
		return v
	}
	return l.cfg.DefaultPerMinute
}

// Allow reports whether a request for (category, key) is permitted now.
func (l *Limiter) Allow(category, key string) bool {
	if !l.cfg.Enabled {
		return true
	}
	perMin := l.limitFor(category)
	if perMin <= 0 {
		return true // unlimited
	}

	bucketKey := category + "|" + key
	l.mu.Lock()
	e, ok := l.buckets[bucketKey]
	if !ok {
		// perMin requests/minute: refill perMin/60 tokens per second, burst = perMin.
		e = &entry{lim: rate.NewLimiter(rate.Limit(float64(perMin)/60.0), perMin)}
		l.buckets[bucketKey] = e
	}
	e.lastSeen = time.Now()
	l.mu.Unlock()

	return e.lim.Allow()
}

func (l *Limiter) janitor() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("ratelimit janitor panic recovered", "panic", r)
		}
	}()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-30 * time.Minute)
		l.mu.Lock()
		for k, e := range l.buckets {
			if e.lastSeen.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}
