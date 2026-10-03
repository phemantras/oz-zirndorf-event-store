package main

import "time"

// systemClock is the core.Clock of the running service: the wall clock.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
