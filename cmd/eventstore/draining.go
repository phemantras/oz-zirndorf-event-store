package main

import (
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	// railwayEnvironmentVariable is set by Railway in every deployment; it
	// tells a run on Railway from a local one.
	railwayEnvironmentVariable = "RAILWAY_ENVIRONMENT_NAME"
	// drainingSecondsVariable is the Railway service variable for the time
	// between SIGTERM and SIGKILL of the previous deployment. Railway's
	// default is 0.
	drainingSecondsVariable = "RAILWAY_DEPLOYMENT_DRAINING_SECONDS"
	logMsgDrainingTooShort  = "railway draining time does not cover the shutdown timeout"
)

// checkDrainingTime warns when the service runs on Railway and Railway would
// kill it before shutdownTimeout has run out, so a request waiting until its
// deadline would lose its answer on the next deploy. It only logs: the
// service runs either way.
func checkDrainingTime(logger *slog.Logger, getenv func(string) string) {
	if getenv(railwayEnvironmentVariable) == "" {
		return
	}
	raw := strings.TrimSpace(getenv(drainingSecondsVariable))
	seconds, err := strconv.ParseFloat(raw, 64)
	if err == nil && time.Duration(seconds*float64(time.Second)) > shutdownTimeout {
		return
	}
	logger.Warn(logMsgDrainingTooShort,
		"variable", drainingSecondsVariable, "value", raw, "shutdown_timeout", shutdownTimeout.String())
}
