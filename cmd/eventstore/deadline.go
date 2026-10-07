package main

import (
	"context"
	"net/http"
	"time"
)

// withRequestDeadline gives every request a context that ends after
// timeout, so a slow database or an exhausted pool ends the queries and the
// wait for a connection instead of letting requests pile up (AD-18).
func withRequestDeadline(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
