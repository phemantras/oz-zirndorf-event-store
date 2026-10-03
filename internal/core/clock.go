package core

import "time"

// Clock is the port that tells the core the current time. The core never
// calls time.Now itself, so tests can fix the time (AD-2).
type Clock interface {
	Now() time.Time
}

// Period is the half-open effective period [Start, End) of an event.
type Period struct {
	Start time.Time
	End   time.Time
}

// IsOver reports whether the period has ended, that is End <= now. This is
// the only rule for "past" and "archived" (AD-16).
func (p Period) IsOver(clock Clock) bool {
	return !clock.Now().Before(p.End)
}
