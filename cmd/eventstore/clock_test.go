package main

import (
	"testing"
	"time"
)

func TestSystemClockShowsTheWallClock(t *testing.T) {
	before := time.Now()
	got := systemClock{}.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, want between %v and %v", got, before, after)
	}
}
