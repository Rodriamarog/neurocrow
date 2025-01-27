package handlers

import (
    "net/http"
    "sync"
    "time"
)

type RateLimiter struct {
    MessageLimit *IPRateLimiter  // Capitalized for export
    ViewLimit    *IPRateLimiter  // Capitalized for export
}

func NewRateLimiter() *RateLimiter {
    return &RateLimiter{
        MessageLimit: NewIPRateLimiter(300, time.Minute),  // 300 per minute for testing
        ViewLimit:    NewIPRateLimiter(1200, time.Minute), // 1200 per minute for testing
    }
}

type IPRateLimiter struct {
    ips    map[string][]time.Time
    mu     sync.RWMutex
    limit  int
    window time.Duration
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
    return &IPRateLimiter{
        ips:    make(map[string][]time.Time),
        limit:  limit,
        window: window,
    }
}

func (l *IPRateLimiter) RateLimit(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ip := r.RemoteAddr

        l.mu.Lock()
        defer l.mu.Unlock()

        now := time.Now()
        windowStart := now.Add(-l.window)
        
        requests := l.ips[ip]
        valid := make([]time.Time, 0)
        
        for _, req := range requests {
            if req.After(windowStart) {
                valid = append(valid, req)
            }
        }
        
        l.ips[ip] = valid

        if len(valid) >= l.limit {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        l.ips[ip] = append(l.ips[ip], now)

        next(w, r)
    }
}