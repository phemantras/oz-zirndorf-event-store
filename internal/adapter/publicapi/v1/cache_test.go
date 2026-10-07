package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListsMayBeCachedForAMinute(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{eventsPath, archivePath, eventTypesPath} {
			var logs bytes.Buffer
			rec := serve(newEventsHandler(&recordingLister{}, &logs), method, path)

			if rec.Code != http.StatusOK {
				t.Errorf("%s %s status = %d, want %d", method, path, rec.Code, http.StatusOK)
			}
			assertAllowsAnyOrigin(t, rec.Header())
			assertCacheControl(t, rec.Header(), "public, max-age=60")
		}
	}
}

func TestRejectedListFiltersAreNotStored(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{eventsPath + "?to=x", archivePath + "?to=x"} {
			var logs bytes.Buffer
			events, _ := coreEvents(t)
			rec := serve(newEventsHandler(events, &logs), method, path)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s status = %d, want %d", method, path, rec.Code, http.StatusBadRequest)
			}
			assertCacheControl(t, rec.Header(), "no-store")
		}
	}
}

// answerWith returns a handler that answers with status, or writes a body
// without WriteHeader when status is 0.
func answerWith(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte("body"))
	})
}

func TestCacheForSetsItsValueOnSuccessAndNoStoreOtherwise(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{"implicit 200", 0, "public, max-age=60"},
		{"200", http.StatusOK, "public, max-age=60"},
		{"204", http.StatusNoContent, "public, max-age=60"},
		{"299", 299, "public, max-age=60"},
		{"300", http.StatusMultipleChoices, "no-store"},
		{"400", http.StatusBadRequest, "no-store"},
		{"500", http.StatusInternalServerError, "no-store"},
		{"503", http.StatusServiceUnavailable, "no-store"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			cacheFor(cacheControlLists)(answerWith(test.status)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, eventsPath, nil))

			assertCacheControl(t, rec.Header(), test.want)
			if rec.Body.String() != "body" {
				t.Errorf("body = %q, want body", rec.Body.String())
			}
		})
	}
}
