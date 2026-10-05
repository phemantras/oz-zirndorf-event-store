package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	apispec "github.com/phemantras/oz-zirndorf-event-store/api/v1"
)

const (
	eventTypesPath = "/v1/event-types"
	unknownPath    = "/v1/nope"
)

var writeMethods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

func newTestHandler() http.Handler {
	return NewHandler(Config{Logger: slog.New(slog.DiscardHandler)})
}

func serve(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func assertAllowsAnyOrigin(t *testing.T, header http.Header) {
	t.Helper()
	if got := header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
}

// assertProblem checks an RFC 9457 response with the given status and
// returns its detail.
func assertProblem(t *testing.T, rec *httptest.ResponseRecorder, status int) string {
	t.Helper()
	if rec.Code != status {
		t.Errorf("status = %d, want %d", rec.Code, status)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem %q: %v", rec.Body.String(), err)
	}
	want := Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: problem.Detail}
	if problem != want {
		t.Errorf("problem = %+v, want %+v", problem, want)
	}
	if problem.Detail == "" {
		t.Error("problem has no detail")
	}
	return problem.Detail
}

func TestListEventTypesReturnsAllSevenInCoreOrder(t *testing.T) {
	rec := serve(newTestHandler(), http.MethodGet, eventTypesPath)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
	want := `{"data":[` +
		`{"code":"festival","label":"Fest/Kirchweih"},` +
		`{"code":"market","label":"Markt"},` +
		`{"code":"culture","label":"Kultur/Bühne"},` +
		`{"code":"politics","label":"Politik/Sitzung"},` +
		`{"code":"club","label":"Verein/Treff"},` +
		`{"code":"sports","label":"Sport"},` +
		`{"code":"other","label":"Sonstiges"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestHeadAnswersWithoutBody(t *testing.T) {
	server := httptest.NewServer(newTestHandler())
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, server.URL+eventTypesPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want none", body)
	}
	assertAllowsAnyOrigin(t, resp.Header)
}

func TestServesTheEmbeddedSpec(t *testing.T) {
	rec := serve(newTestHandler(), http.MethodGet, specPath)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/yaml" {
		t.Errorf("Content-Type = %q, want application/yaml", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
	if len(apispec.OpenAPISpec) == 0 || !bytes.Equal(rec.Body.Bytes(), apispec.OpenAPISpec) {
		t.Error("body is not the embedded spec")
	}
}

func TestServesTheDocsPage(t *testing.T) {
	rec := serve(newTestHandler(), http.MethodGet, docsPath)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
	page := rec.Body.String()
	for _, want := range []string{
		`<script src="docs/redoc.standalone.js">`,
		`<redoc spec-url="openapi.yaml"`,
		`<a href="openapi.yaml">`,
		// The policy keeps Redoc's logo from cdn.redoc.ly off the page.
		`<meta http-equiv="Content-Security-Policy" content="img-src 'self' data:; font-src 'self' data:; connect-src 'self'">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s:\n%s", want, page)
		}
	}
	// The page must not load anything from another host (ENT-14).
	for _, external := range []string{"http://", "https://", `src="//`, `href="//`} {
		if strings.Contains(page, external) {
			t.Errorf("page refers to another host with %q", external)
		}
	}
}

func TestDocsPageIgnoresRangeRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, docsPath, nil)
	req.Header.Set("Range", "bytes=99999999-")
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	page, err := fs.ReadFile(staticFiles, docsPageFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rec.Body.Bytes(), page) {
		t.Error("body is not the full docs page")
	}
}

func TestMissingStaticFileIsAnInternalServerErrorAndLogged(t *testing.T) {
	var logs bytes.Buffer
	respond := responder{logger: slog.New(slog.NewJSONHandler(&logs, nil))}

	rec := httptest.NewRecorder()
	respond.serveStaticFile("static/missing.html", htmlContentType)(rec, httptest.NewRequest(http.MethodGet, docsPath, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	if !strings.Contains(logs.String(), "missing.html") {
		t.Errorf("log %q does not name the missing file", logs.String())
	}
}

func TestServesTheEmbeddedRedocScript(t *testing.T) {
	rec := serve(newTestHandler(), http.MethodGet, docsScriptPath)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/javascript; charset=utf-8", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
	script, err := fs.ReadFile(staticFiles, docsScriptFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(script) == 0 || !bytes.Equal(rec.Body.Bytes(), script) {
		t.Error("body is not the embedded Redoc script")
	}
}

func TestHeadOnTheDocsPageAnswersWithoutBody(t *testing.T) {
	server := httptest.NewServer(newTestHandler())
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, server.URL+docsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want none", body)
	}
	assertAllowsAnyOrigin(t, resp.Header)
}

func TestPreflightIsAnsweredOnEveryPath(t *testing.T) {
	for _, path := range []string{eventTypesPath, specPath, docsPath, docsScriptPath, unknownPath, "/v1/"} {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		req.Header.Set("Origin", "https://karte.example")
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		req.Header.Set("Access-Control-Request-Headers", "X-Requested-With, Accept")
		rec := httptest.NewRecorder()
		newTestHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("OPTIONS %s status = %d, want %d", path, rec.Code, http.StatusNoContent)
		}
		assertAllowsAnyOrigin(t, rec.Header())
		wantHeaders := map[string]string{
			"Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
			"Access-Control-Allow-Headers": "X-Requested-With, Accept",
			"Access-Control-Max-Age":       "86400",
			"Allow":                        "GET, HEAD, OPTIONS",
		}
		for name, want := range wantHeaders {
			if got := rec.Header().Get(name); got != want {
				t.Errorf("OPTIONS %s %s = %q, want %q", path, name, got, want)
			}
		}
		if !slices.Contains(rec.Header().Values("Vary"), "Access-Control-Request-Headers") {
			t.Errorf("OPTIONS %s Vary = %q, want it to contain Access-Control-Request-Headers", path, rec.Header().Values("Vary"))
		}
		if rec.Body.Len() != 0 {
			t.Errorf("OPTIONS %s body = %q, want none", path, rec.Body.String())
		}
	}
}

func TestPreflightWithoutRequestHeadersAllowsNoHeaders(t *testing.T) {
	rec := serve(newTestHandler(), http.MethodOptions, eventTypesPath)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got, present := rec.Header()["Access-Control-Allow-Headers"]; present {
		t.Errorf("Access-Control-Allow-Headers = %q, want absent", got)
	}
}

func TestWriteMethodsAreRejectedOnEveryPath(t *testing.T) {
	for _, method := range writeMethods {
		for _, path := range []string{eventTypesPath, specPath, docsPath, docsScriptPath, unknownPath} {
			rec := serve(newTestHandler(), method, path)

			detail := assertProblem(t, rec, http.StatusMethodNotAllowed)
			if got := rec.Header().Get("Allow"); got != "GET, HEAD, OPTIONS" {
				t.Errorf("%s %s Allow = %q, want %q", method, path, got, "GET, HEAD, OPTIONS")
			}
			if want := "Method " + method + " is not allowed; the API is read-only and allows GET, HEAD and OPTIONS."; detail != want {
				t.Errorf("%s %s detail = %q, want %q as the KON-3 example says", method, path, detail, want)
			}
		}
	}
}

func TestUnknownPathsAreNotFound(t *testing.T) {
	for _, path := range []string{unknownPath, "/v1/", "/v1/event-types/festival", "/v1/docs/nope", "/v1/docs/"} {
		rec := serve(newTestHandler(), http.MethodGet, path)

		detail := assertProblem(t, rec, http.StatusNotFound)
		if !strings.Contains(detail, path) {
			t.Errorf("GET %s detail %q does not name the path", path, detail)
		}
	}
}

// failingServer stands in for a server whose answer fails, to reach the
// error handling of the generated strict server.
type failingServer struct{}

var errAnswerFailed = errors.New("answer failed")

func (failingServer) ListEventTypes(context.Context, ListEventTypesRequestObject) (ListEventTypesResponseObject, error) {
	return nil, errAnswerFailed
}

func (failingServer) ListEvents(context.Context, ListEventsRequestObject) (ListEventsResponseObject, error) {
	return nil, errAnswerFailed
}

func (failingServer) ListArchivedEvents(context.Context, ListArchivedEventsRequestObject) (ListArchivedEventsResponseObject, error) {
	return nil, errAnswerFailed
}

func TestFailingAnswerIsAnInternalServerErrorAndLogged(t *testing.T) {
	var logs bytes.Buffer
	handler := newHandler(Config{Logger: slog.New(slog.NewJSONHandler(&logs, nil))}, failingServer{})

	rec := serve(handler, http.MethodGet, eventTypesPath)

	detail := assertProblem(t, rec, http.StatusInternalServerError)
	if strings.Contains(detail, errAnswerFailed.Error()) {
		t.Errorf("detail %q leaks the internal error", detail)
	}
	if !strings.Contains(logs.String(), errAnswerFailed.Error()) {
		t.Errorf("log %q does not contain the error", logs.String())
	}
}

// failingWriter is a response writer whose client has gone away.
type failingWriter struct {
	header http.Header
}

var errClientGone = errors.New("client gone")

func (w *failingWriter) Header() http.Header       { return w.header }
func (w *failingWriter) WriteHeader(int)           {}
func (w *failingWriter) Write([]byte) (int, error) { return 0, errClientGone }

func TestFailedWritesAreLogged(t *testing.T) {
	for _, path := range []string{specPath, unknownPath} {
		var logs bytes.Buffer
		handler := NewHandler(Config{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})

		handler.ServeHTTP(&failingWriter{header: http.Header{}}, httptest.NewRequest(http.MethodGet, path, nil))

		if !strings.Contains(logs.String(), errClientGone.Error()) {
			t.Errorf("GET %s log %q does not contain the write error", path, logs.String())
		}
	}
}

func TestEveryAnswerAllowsAnyOrigin(t *testing.T) {
	for _, method := range slices.Concat([]string{http.MethodGet, http.MethodHead, http.MethodOptions}, writeMethods) {
		for _, path := range []string{eventTypesPath, specPath, docsPath, docsScriptPath, unknownPath} {
			rec := serve(newTestHandler(), method, path)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Errorf("%s %s Access-Control-Allow-Origin = %q, want *", method, path, got)
			}
		}
	}
}
