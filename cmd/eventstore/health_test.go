package main

import (
	"bytes"
	"context"
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
