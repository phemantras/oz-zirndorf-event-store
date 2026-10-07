package v1

import (
	"bytes"
	"compress/gzip"
	"context"
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

// invalidGzipLevel is no level compress/gzip accepts.
const invalidGzipLevel = 42

func gunzip(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	return plain
}

func TestGzipOfCompressesLosslessly(t *testing.T) {
	body := []byte(strings.Repeat("Zirndorf ", 1000))

	compressed, err := gzipOf(body, gzipLevel)

	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) >= len(body) {
		t.Errorf("compressed %d bytes to %d", len(body), len(compressed))
	}
	if !bytes.Equal(gunzip(t, compressed), body) {
		t.Error("decompressed body differs")
	}
}

func TestGzipOfRejectsAnInvalidLevel(t *testing.T) {
	if _, err := gzipOf([]byte("body"), invalidGzipLevel); err == nil {
		t.Error("want an error for an invalid level")
	}
}

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{"no header", nil, false},
		{"empty", []string{""}, false},
		{"gzip", []string{"gzip"}, true},
		{"upper case", []string{"GZIP"}, true},
		{"in a list", []string{"gzip, deflate"}, true},
		{"later in a list with weight", []string{"br, gzip;q=0.5"}, true},
		{"spaces around weight", []string{"gzip ; q=1"}, true},
		{"other parameter", []string{"gzip;level=1"}, true},
		{"second header value", []string{"br", "gzip"}, true},
		{"identity", []string{"identity"}, false},
		{"br", []string{"br"}, false},
		{"x-gzip", []string{"x-gzip"}, false},
		{"wildcard", []string{"*"}, false},
		{"q=0", []string{"gzip;q=0"}, false},
		{"q=0.0", []string{"gzip; q=0.0"}, false},
		{"Q=0.000", []string{"gzip;Q=0.000"}, false},
		{"invalid weight", []string{"gzip;q=x"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, specPath, nil)
			for _, value := range test.values {
				req.Header.Add("Accept-Encoding", value)
			}

			if got := acceptsGzip(req); got != test.want {
				t.Errorf("acceptsGzip(%q) = %v, want %v", test.values, got, test.want)
			}
		})
	}
}

// compressedFiles are the paths served gzip-compressed with the file
// behind each.
func compressedFiles(t *testing.T) map[string][]byte {
	t.Helper()
	script, err := fs.ReadFile(staticFiles, docsScriptFile)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		specPath:         apispec.OpenAPISpec,
		importSchemaPath: apispec.ImportSchemaV1,
		docsScriptPath:   script,
	}
}

func serveWithAcceptEncoding(method, path, acceptEncoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	return rec
}

func assertVariesByEncoding(t *testing.T, header http.Header) {
	t.Helper()
	if !slices.Contains(header.Values("Vary"), "Accept-Encoding") {
		t.Errorf("Vary = %q, want it to contain Accept-Encoding", header.Values("Vary"))
	}
}

func TestStaticFilesAreServedCompressedWhenTheClientAcceptsGzip(t *testing.T) {
	for path, file := range compressedFiles(t) {
		rec := serveWithAcceptEncoding(http.MethodGet, path, "gzip, deflate")

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Errorf("%s Content-Encoding = %q, want gzip", path, got)
		}
		assertVariesByEncoding(t, rec.Header())
		assertAllowsAnyOrigin(t, rec.Header())
		if !bytes.Equal(gunzip(t, rec.Body.Bytes()), file) {
			t.Errorf("%s decompressed body is not the embedded file", path)
		}
	}
}

func TestRedocScriptShrinksToLessThanAThird(t *testing.T) {
	script := compressedFiles(t)[docsScriptPath]

	rec := serveWithAcceptEncoding(http.MethodGet, docsScriptPath, "gzip")

	if got, limit := rec.Body.Len(), len(script)/3; got >= limit {
		t.Errorf("compressed script has %d bytes, want less than %d", got, limit)
	}
}

func TestStaticFilesAreServedUncompressedWhenTheClientRefusesGzip(t *testing.T) {
	for path, file := range compressedFiles(t) {
		for _, acceptEncoding := range []string{"", "identity", "br", "gzip;q=0"} {
			rec := serveWithAcceptEncoding(http.MethodGet, path, acceptEncoding)

			if got, present := rec.Header()["Content-Encoding"]; present {
				t.Errorf("%s with %q Content-Encoding = %q, want none", path, acceptEncoding, got)
			}
			assertVariesByEncoding(t, rec.Header())
			if !bytes.Equal(rec.Body.Bytes(), file) {
				t.Errorf("%s with %q body is not the embedded file", path, acceptEncoding)
			}
		}
	}
}

func TestHeadOnACompressedFileCarriesTheHeadersOfGet(t *testing.T) {
	for path := range compressedFiles(t) {
		get := serveWithAcceptEncoding(http.MethodGet, path, "gzip")
		head := serveWithAcceptEncoding(http.MethodHead, path, "gzip")

		for _, name := range []string{"Content-Type", "Content-Encoding", "Cache-Control", "Vary", "Access-Control-Allow-Origin"} {
			if got, want := head.Header().Values(name), get.Header().Values(name); !slices.Equal(got, want) {
				t.Errorf("HEAD %s %s = %q, want %q as on GET", path, name, got, want)
			}
		}
	}
}

func TestDocsPageAndListsAreNeverCompressed(t *testing.T) {
	for _, path := range []string{docsPath, eventTypesPath} {
		rec := serveWithAcceptEncoding(http.MethodGet, path, "gzip")

		if got, present := rec.Header()["Content-Encoding"]; present {
			t.Errorf("%s Content-Encoding = %q, want none", path, got)
		}
	}
}

func TestFileThatFailsToCompressIsServedUncompressedAndWarned(t *testing.T) {
	var logs bytes.Buffer
	respond := responder{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	answer := staticAnswer{contentType: specContentType, cacheControl: cacheControlDocuments, compress: true}

	serveSpec := respond.serveBodyCompressedAt(apispec.OpenAPISpec, answer, invalidGzipLevel)
	req := httptest.NewRequest(http.MethodGet, specPath, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	serveSpec(rec, req)

	if got, present := rec.Header()["Content-Encoding"]; present {
		t.Errorf("Content-Encoding = %q, want none", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), apispec.OpenAPISpec) {
		t.Error("body is not the embedded spec")
	}
	assertLogEntry(t, &logs, logEntry{Level: "WARN", Msg: logMsgCompressionFailed})
}

func TestHeadOnACompressedFileAnswersWithoutBody(t *testing.T) {
	server := httptest.NewServer(newTestHandler())
	defer server.Close()

	for path := range compressedFiles(t) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", "gzip")
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("HEAD %s status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
		if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
			t.Errorf("HEAD %s Content-Encoding = %q, want gzip", path, got)
		}
		if len(body) != 0 {
			t.Errorf("HEAD %s body has %d bytes, want none", path, len(body))
		}
	}
}

func TestCompressedFilesIgnoreRangeRequests(t *testing.T) {
	for path, file := range compressedFiles(t) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Range", "bytes=0-9")
		rec := httptest.NewRecorder()
		newTestHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Errorf("%s Content-Encoding = %q, want gzip", path, got)
		}
		if !bytes.Equal(gunzip(t, rec.Body.Bytes()), file) {
			t.Errorf("%s body is not the whole compressed file", path)
		}
	}
}
