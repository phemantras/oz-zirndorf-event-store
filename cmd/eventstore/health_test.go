package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePinger struct {
	err         error
	hadDeadline bool
}

func (f *fakePinger) Ping(ctx context.Context) error {
	_, f.hadDeadline = ctx.Deadline()
	return f.err
}

func TestHealthReturnsOKWhenDatabaseAnswers(t *testing.T) {
	pinger := &fakePinger{}
	handler := newHealthHandler(pinger, slog.New(slog.DiscardHandler))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, healthPath, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !pinger.hadDeadline {
		t.Error("ping context had no deadline")
	}
}

func TestHealthReturnsUnavailableAndLogsWhenPingFails(t *testing.T) {
	var logs bytes.Buffer
	handler := newHealthHandler(&fakePinger{err: errors.New("connection refused")}, newLogger(&logs))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, healthPath, nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(logs.String(), "connection refused") {
		t.Errorf("log %q does not contain the ping error", logs.String())
	}
}

func TestRouterServesHealthOnlyForGet(t *testing.T) {
	router := newRouter(&fakePinger{}, slog.New(slog.DiscardHandler))

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, healthPath, nil))
	if get.Code != http.StatusOK {
		t.Errorf("GET status = %d, want %d", get.Code, http.StatusOK)
	}

	post := httptest.NewRecorder()
	router.ServeHTTP(post, httptest.NewRequest(http.MethodPost, healthPath, nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want %d", post.Code, http.StatusMethodNotAllowed)
	}
}

func TestHealthLogsForwardedForAndRemoteAddress(t *testing.T) {
	var logs bytes.Buffer
	handler := newHealthHandler(&fakePinger{}, newLogger(&logs))

	req := httptest.NewRequest(http.MethodGet, healthPath, nil)
	req.Header.Set(forwardedForHeader, "203.0.113.7, 10.0.0.1")
	req.RemoteAddr = "10.0.0.2:41234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	entry := decodeSingleLogEntry(t, &logs)
	if entry["msg"] != healthRequestLogMessage {
		t.Errorf("msg = %v, want %q", entry["msg"], healthRequestLogMessage)
	}
	if entry[forwardedForLogKey] != "203.0.113.7, 10.0.0.1" {
		t.Errorf("%s = %v, want the raw header value", forwardedForLogKey, entry[forwardedForLogKey])
	}
	if entry[remoteAddrLogKey] != "10.0.0.2:41234" {
		t.Errorf("%s = %v, want the request remote address", remoteAddrLogKey, entry[remoteAddrLogKey])
	}
}

func TestHealthLogsAllForwardedForLinesJoined(t *testing.T) {
	var logs bytes.Buffer
	handler := newHealthHandler(&fakePinger{}, newLogger(&logs))

	req := httptest.NewRequest(http.MethodGet, healthPath, nil)
	req.Header.Add(forwardedForHeader, "203.0.113.7")
	req.Header.Add(forwardedForHeader, "198.51.100.4")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	entry := decodeSingleLogEntry(t, &logs)
	if entry[forwardedForLogKey] != "203.0.113.7, 198.51.100.4" {
		t.Errorf("%s = %v, want both header lines joined in order", forwardedForLogKey, entry[forwardedForLogKey])
	}
}

func TestHealthLogsEmptyForwardedForWhenHeaderIsMissing(t *testing.T) {
	var logs bytes.Buffer
	handler := newHealthHandler(&fakePinger{}, newLogger(&logs))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, healthPath, nil))

	entry := decodeSingleLogEntry(t, &logs)
	value, ok := entry[forwardedForLogKey]
	if !ok || value != "" {
		t.Errorf("%s = %v (present: %t), want an empty string", forwardedForLogKey, value, ok)
	}
}

// decodeSingleLogEntry parses logs as exactly one JSON log line.
func decodeSingleLogEntry(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), logs.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("decode log line %q: %v", lines[0], err)
	}
	return entry
}
