package main

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
)

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

type rateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*tokenBucket
	ratePerSec  float64
	burst       float64
	ttl         time.Duration
	lastCleanup time.Time
}

func newRateLimiter(ratePerSec float64, burst int) *rateLimiter {
	return &rateLimiter{
		buckets:     make(map[string]*tokenBucket),
		ratePerSec:  ratePerSec,
		burst:       float64(burst),
		ttl:         10 * time.Minute,
		lastCleanup: time.Now(),
	}
}

func (rl *rateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if now.Sub(rl.lastCleanup) > rl.ttl {
		for k, b := range rl.buckets {
			if now.Sub(b.lastRefill) > rl.ttl {
				delete(rl.buckets, k)
			}
		}
		rl.lastCleanup = now
	}

	b, exists := rl.buckets[key]
	if !exists {
		rl.buckets[key] = &tokenBucket{
			tokens:     rl.burst - 1,
			lastRefill: now,
		}
		return true, 0
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * rl.ratePerSec
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastRefill = now
	}

	if b.tokens >= 1 {
		b.tokens -= 1
		return true, 0
	}

	needed := (1 - b.tokens) / rl.ratePerSec
	retryAfter := time.Duration(needed*float64(time.Second)) + time.Second
	return false, retryAfter
}

func clientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			if trimmed := strings.TrimSpace(first); trimmed != "" {
				return trimmed
			}
		} else if trimmed := strings.TrimSpace(xff); trimmed != "" {
			return trimmed
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func isAuthEndpoint(path string) bool {
	return path == "/login" ||
		path == "/login/verify" ||
		path == "/register" ||
		path == "/confirm" ||
		path == "/password/forgot" ||
		path == "/password/reset" ||
		path == "/magic/request" ||
		path == "/magic/login"
}

func enforceRateLimit(
	w http.ResponseWriter,
	r *http.Request,
	limiter *rateLimiter,
	bucketPrefix string,
	logger *slog.Logger,
) bool {
	if limiter == nil {
		return true
	}
	key := bucketPrefix + ":" + clientKey(r)
	allowed, retryAfter := limiter.allow(key, time.Now())
	if !allowed {
		secs := int(retryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		logger.Warn("rate limit exceeded", "bucket", bucketPrefix, "path", r.URL.Path, "client", clientKey(r))
		platform.WriteError(w, http.StatusTooManyRequests, "too many requests, please try again later")
		return false
	}
	return true
}
