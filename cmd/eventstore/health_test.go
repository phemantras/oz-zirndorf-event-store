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
	router := newRouter(&fakePinger{}, slog.New(slog.DiscardHandler), routeHandlers{admin: http.NotFoundHandler(), public: http.NotFoundHandler()})

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

func TestRouterMountsAdminBelowAdminPath(t *testing.T) {
	const adminMarker = "admin-stub"
	admin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(adminMarker))
	})
	router := newRouter(&fakePinger{}, slog.New(slog.DiscardHandler), routeHandlers{admin: admin, public: http.NotFoundHandler()})

	for _, path := range []string{"/admin/", "/admin/login", "/admin/static/htmx.min.js"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Body.String() != adminMarker {
			t.Errorf("GET %s did not reach the admin handler", path)
		}
	}

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, healthPath, nil))
	if health.Code != http.StatusOK || health.Body.String() == adminMarker {
		t.Errorf("health status = %d, want %d from the health handler", health.Code, http.StatusOK)
	}
}

func TestRouterMountsPublicAPIBelowV1(t *testing.T) {
	const publicMarker = "public-stub"
	public := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(publicMarker))
	})
	router := newRouter(&fakePinger{}, slog.New(slog.DiscardHandler), routeHandlers{admin: http.NotFoundHandler(), public: public})

	for _, path := range []string{"/v1/events", "/v1/archive/events", "/v1/event-types", "/v1/openapi.yaml", "/v1/nope"} {
		for _, method := range []string{http.MethodGet, http.MethodOptions, http.MethodPost} {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
			if rec.Body.String() != publicMarker {
				t.Errorf("%s %s did not reach the public API handler", method, path)
			}
		}
	}
	for _, path := range []string{"/admin/login", healthPath} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Body.String() == publicMarker {
			t.Errorf("GET %s reached the public API handler", path)
		}
	}
}

// Client addresses are personal data, so a successful health check must not
// log anything about the request (ENT-21 measurement is finished).
func TestHealthLogsNothingWhenDatabaseAnswers(t *testing.T) {
	var logs bytes.Buffer
	handler := newHealthHandler(&fakePinger{}, newLogger(&logs))

	req := httptest.NewRequest(http.MethodGet, healthPath, nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.4")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if logs.Len() != 0 {
		t.Errorf("log = %q, want no output", logs.String())
	}
}
