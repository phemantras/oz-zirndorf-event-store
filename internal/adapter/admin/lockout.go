package admin

import (
	"sync"
	"time"
)

const (
	// maxFailedLogins failures from one client IP lock it out.
	maxFailedLogins = 5
	// lockoutDuration is how long failures of an IP count after its most
	// recent failure, and therefore how long a lockout lasts.
	lockoutDuration = 15 * time.Minute
)

// failureRecord counts the failed logins of one client IP.
type failureRecord struct {
	count       int
	lastFailure time.Time
}

// loginLockout tracks failed logins per client IP in memory. This relies on
// running exactly one replica (railway.json).
type loginLockout struct {
	mu       sync.Mutex
	now      func() time.Time
	failures map[string]failureRecord
}

func newLoginLockout(now func() time.Time) *loginLockout {
	return &loginLockout{now: now, failures: make(map[string]failureRecord)}
}

// beginAttempt reports whether ip may try to log in. An allowed attempt is
// counted as a failure right away, in the same critical section as the
// check, so parallel attempts cannot all slip past the limit while bcrypt
// runs; a successful login then calls reset.
func (l *loginLockout) beginAttempt(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneExpired()
	record := l.failures[ip]
	if record.count >= maxFailedLogins {
		return false
	}
	record.count++
	record.lastFailure = l.now()
	l.failures[ip] = record
	return true
}

// reset forgets the failures of ip after a successful login.
func (l *loginLockout) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, ip)
	l.pruneExpired()
}

// pruneExpired drops every record whose last failure is lockoutDuration or
// longer ago. Callers hold l.mu.
func (l *loginLockout) pruneExpired() {
	now := l.now()
	for ip, record := range l.failures {
		if !now.Before(record.lastFailure.Add(lockoutDuration)) {
			delete(l.failures, ip)
		}
	}
}
