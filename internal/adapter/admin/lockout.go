package admin

import (
	"sync"
	"time"
)

const (
	// maxFailedLogins failures under one lockout key lock it out.
	maxFailedLogins = 5
	// lockoutDuration is how long failures of a lockout key count after its
	// most recent failure, and therefore how long a lockout lasts.
	lockoutDuration = 15 * time.Minute
)

// failureRecord counts the failed logins under one lockout key.
type failureRecord struct {
	count       int
	lastFailure time.Time
}

// loginLockout tracks failed logins per lockout key (see lockoutKey) in
// memory. This relies on running exactly one replica (Railway service
// settings, see README).
type loginLockout struct {
	mu       sync.Mutex
	now      func() time.Time
	failures map[string]failureRecord
}

func newLoginLockout(now func() time.Time) *loginLockout {
	return &loginLockout{now: now, failures: make(map[string]failureRecord)}
}

// beginAttempt reports whether key may try to log in. An allowed attempt is
// counted as a failure right away, in the same critical section as the
// check, so parallel attempts cannot all slip past the limit while bcrypt
// runs; a successful login then calls reset.
func (l *loginLockout) beginAttempt(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneExpired()
	record := l.failures[key]
	if record.count >= maxFailedLogins {
		return false
	}
	record.count++
	record.lastFailure = l.now()
	l.failures[key] = record
	return true
}

// reset forgets the failures of key after a successful login.
func (l *loginLockout) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
	l.pruneExpired()
}

// pruneExpired drops every record whose last failure is lockoutDuration or
// longer ago. Callers hold l.mu.
func (l *loginLockout) pruneExpired() {
	now := l.now()
	for key, record := range l.failures {
		if !now.Before(record.lastFailure.Add(lockoutDuration)) {
			delete(l.failures, key)
		}
	}
}
