package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	limitWindow  = 15 * time.Minute
	limitPerUser = 5  // failures per IP and username
	limitPerIP   = 30 // failures per IP across all usernames
)

// Limiter throttles failed logins in memory. Failures are counted per IP and
// username rather than per username, so nobody can lock a player out on purpose.
type Limiter struct {
	mu       sync.Mutex
	failures map[string]*failure
	now      func() time.Time
}

type failure struct {
	count int
	reset time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{failures: map[string]*failure{}, now: time.Now}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Blocked reports whether this request has too many recent failures.
func (l *Limiter) Blocked(r *http.Request, username string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ip := clientIP(r)
	return l.over("ip|"+ip, limitPerIP) || l.over("user|"+ip+"|"+username, limitPerUser)
}

func (l *Limiter) over(key string, limit int) bool {
	f, ok := l.failures[key]
	return ok && l.now().Before(f.reset) && f.count >= limit
}

// Fail records a failed login.
func (l *Limiter) Fail(r *http.Request, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, f := range l.failures {
		if !now.Before(f.reset) {
			delete(l.failures, k)
		}
	}
	ip := clientIP(r)
	for _, key := range []string{"ip|" + ip, "user|" + ip + "|" + username} {
		f, ok := l.failures[key]
		if !ok {
			f = &failure{reset: now.Add(limitWindow)}
			l.failures[key] = f
		}
		f.count++
	}
}

// Succeed clears the username's failures after a good login.
func (l *Limiter) Succeed(r *http.Request, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, "user|"+clientIP(r)+"|"+username)
}
