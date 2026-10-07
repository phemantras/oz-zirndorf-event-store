package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"testing"
)

// railwayEnvironment is the value Railway sets in railwayEnvironmentVariable;
// any non-empty value marks a run on Railway.
const railwayEnvironment = "production"

// drainingLogLines decodes the JSON log lines checkDrainingTime wrote for
// the variables in env.
func drainingLogLines(t *testing.T, env map[string]string) []map[string]any {
	t.Helper()
	var logs bytes.Buffer
	checkDrainingTime(newLogger(&logs), func(name string) string { return env[name] })

	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("decode log line %q: %v", raw, err)
		}
		lines = append(lines, line)
	}
	return lines
}

func TestCheckDrainingTimeWarnsOnRailwayWhenTheShutdownCannotFinish(t *testing.T) {
	tests := map[string]string{
		"not set":        "",
		"not a number":   "dreißig",
		"railway 0":      "0",
		"equal shutdown": strconv.Itoa(int(shutdownTimeout.Seconds())),
	}
	for name, draining := range tests {
		t.Run(name, func(t *testing.T) {
			lines := drainingLogLines(t, map[string]string{
				railwayEnvironmentVariable: railwayEnvironment,
				drainingSecondsVariable:    draining,
			})

			if len(lines) != 1 {
				t.Fatalf("log lines = %v, want one warning", lines)
			}
			if lines[0]["level"] != slog.LevelWarn.String() || lines[0]["msg"] != logMsgDrainingTooShort {
				t.Errorf("log line = %v, want WARN %q", lines[0], logMsgDrainingTooShort)
			}
			if lines[0]["shutdown_timeout"] != shutdownTimeout.String() {
				t.Errorf("log line = %v, want shutdown_timeout %q", lines[0], shutdownTimeout)
			}
		})
	}
}

func TestCheckDrainingTimeIsQuietWhenTheShutdownCanFinish(t *testing.T) {
	lines := drainingLogLines(t, map[string]string{
		railwayEnvironmentVariable: railwayEnvironment,
		drainingSecondsVariable:    strconv.Itoa(int(shutdownTimeout.Seconds()) + 5),
	})

	if len(lines) != 0 {
		t.Errorf("log lines = %v, want none", lines)
	}
}

func TestCheckDrainingTimeIsQuietOutsideRailway(t *testing.T) {
	lines := drainingLogLines(t, map[string]string{})

	if len(lines) != 0 {
		t.Errorf("log lines = %v, want none outside Railway", lines)
	}
}
