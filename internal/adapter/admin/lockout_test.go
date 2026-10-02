package admin

import (
	"testing"
	"time"
)

const (
	testIP      = "203.0.113.7"
	otherTestIP = "198.51.100.4"
)

// fakeClock is a settable time source for tests.
type fakeClock struct{ current time.Time }

func (c *fakeClock) now() time.Time           { return c.current }
func (c *fakeClock) advance(by time.Duration) { c.current = c.current.Add(by) }

func failRepeatedly(t *testing.T, l *loginLockout, ip string, times int) {
	t.Helper()
	for i := range times {
		if !l.beginAttempt(ip) {
			t.Fatalf("attempt %d rejected, want allowed", i+1)
		}
	}
}

func TestLockoutLocksAfterMaximumFailures(t *testing.T) {
	lockout := newLoginLockout((&fakeClock{current: testLoginTime}).now)

	failRepeatedly(t, lockout, testIP, maxFailedLogins)

	if lockout.beginAttempt(testIP) {
		t.Fatalf("attempt %d allowed, want locked", maxFailedLogins+1)
	}
	if !lockout.beginAttempt(otherTestIP) {
		t.Error("lockout leaked to another IP")
	}
}

func TestLockoutEndsAfterDurationSinceLastFailure(t *testing.T) {
	clock := &fakeClock{current: testLoginTime}
	lockout := newLoginLockout(clock.now)
	failRepeatedly(t, lockout, testIP, maxFailedLogins)

	clock.advance(lockoutDuration - time.Second)
	if lockout.beginAttempt(testIP) {
		t.Fatal("unlocked before the lockout duration elapsed")
	}
	clock.advance(time.Second)
	failRepeatedly(t, lockout, testIP, maxFailedLogins)
}

func TestLockoutFailuresExpireOnlyAfterDurationSinceLastFailure(t *testing.T) {
	clock := &fakeClock{current: testLoginTime}
	lockout := newLoginLockout(clock.now)

	failRepeatedly(t, lockout, testIP, maxFailedLogins-1)
	clock.advance(lockoutDuration - time.Second)
	failRepeatedly(t, lockout, testIP, 1)
	if lockout.beginAttempt(testIP) {
		t.Fatal("failures expired although the last one was recent")
	}
}

func TestLockoutResetClearsFailures(t *testing.T) {
	lockout := newLoginLockout((&fakeClock{current: testLoginTime}).now)
	failRepeatedly(t, lockout, testIP, maxFailedLogins-1)

	lockout.reset(testIP)
	failRepeatedly(t, lockout, testIP, maxFailedLogins)
}

func TestLockoutPrunesExpiredEntriesOnAccess(t *testing.T) {
	clock := &fakeClock{current: testLoginTime}
	lockout := newLoginLockout(clock.now)
	failRepeatedly(t, lockout, testIP, 1)
	failRepeatedly(t, lockout, otherTestIP, 1)

	clock.advance(lockoutDuration)
	lockout.reset("192.0.2.1")
	if n := len(lockout.failures); n != 0 {
		t.Errorf("%d entries left after expiry, want 0", n)
	}
}
