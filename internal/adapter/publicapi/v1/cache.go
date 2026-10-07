package v1

import "net/http"

// cacheFor returns a middleware that marks a successful answer as
// cacheable with cacheControl and every other answer as no-store (AD-18).
// It decides at WriteHeader, since the generated strict server writes its
// own 400 without writeProblem.
func cacheFor(cacheControl string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&cacheControlWriter{ResponseWriter: w, onSuccess: cacheControl}, r)
		})
	}
}

// cacheControlWriter sets Cache-Control from the status before the header
// is written.
type cacheControlWriter struct {
	http.ResponseWriter
	onSuccess   string
	wroteHeader bool
}

// WriteHeader sets onSuccess for a 2xx status and no-store for any other,
// then writes the header.
func (w *cacheControlWriter) WriteHeader(status int) {
	w.wroteHeader = true
	cacheControl := cacheControlNoStore
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		cacheControl = w.onSuccess
	}
	w.Header().Set(headerCacheControl, cacheControl)
	w.ResponseWriter.WriteHeader(status)
}

// Write writes body; without a prior WriteHeader it answers 200, as
// net/http does.
func (w *cacheControlWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
