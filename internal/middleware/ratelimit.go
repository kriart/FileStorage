package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]rateBucket
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]rateBucket),
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), time.Now()) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) > 2048 {
		for bucketKey, bucket := range l.buckets {
			if now.Sub(bucket.lastSeen) > 2*l.window {
				delete(l.buckets, bucketKey)
			}
		}
	}

	bucket := l.buckets[key]
	if bucket.windowStart.IsZero() || now.Sub(bucket.windowStart) >= l.window {
		bucket = rateBucket{windowStart: now, count: 0}
	}
	bucket.count++
	bucket.lastSeen = now
	l.buckets[key] = bucket
	return bucket.count <= l.limit
}

func clientIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		ip, _, _ := strings.Cut(value, ",")
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
