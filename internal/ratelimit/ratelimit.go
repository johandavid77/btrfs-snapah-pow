package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	count    int
	lastSeen time.Time
}

type Limiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

func New(limit int, window time.Duration) *Limiter {
	l := &Limiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	// Limpiar visitantes viejos cada minuto
	go func() {
		for range time.Tick(time.Minute) {
			l.mu.Lock()
			for ip, v := range l.visitors {
				if time.Since(v.lastSeen) > l.window {
					delete(l.visitors, ip)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok || time.Since(v.lastSeen) > l.window {
		l.visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
		return true
	}
	v.lastSeen = time.Now()
	v.count++
	return v.count <= l.limit
}

func (l *Limiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// X-Forwarded-For si hay proxy
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = fwd
		}
		if !l.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit excedido, intenta en 60 segundos"}`))
			return
		}
		next(w, r)
	}
}
