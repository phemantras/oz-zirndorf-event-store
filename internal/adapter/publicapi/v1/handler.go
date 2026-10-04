package v1

import (
	"log/slog"
	"net/http"

	apispec "github.com/phemantras/oz-zirndorf-event-store/api/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

const (
	// basePath is the subtree the public API v1 owns (KON-1); the paths of
	// the spec are relative to it.
	basePath = "/v1"
	// specPath serves the contract itself, outside the spec's paths.
	specPath        = basePath + "/openapi.yaml"
	specContentType = "application/yaml"
)

// HTTP header names and values of the public API.
const (
	headerContentType      = "Content-Type"
	headerAllow            = "Allow"
	headerVary             = "Vary"
	headerAllowOrigin      = "Access-Control-Allow-Origin"
	headerAllowMethods     = "Access-Control-Allow-Methods"
	headerAllowHeaders     = "Access-Control-Allow-Headers"
	headerMaxAge           = "Access-Control-Max-Age"
	headerRequestHeaders   = "Access-Control-Request-Headers"
	anyOrigin              = "*"
	allowedMethods         = "GET, HEAD, OPTIONS"
	preflightMaxAgeSeconds = "86400"
)

// Config holds what the public API needs from its surroundings.
type Config struct {
	Logger *slog.Logger
	// Events answers the event lists from the core.
	Events EventLister
	// Clock is the time source of every query; the core reads it once per
	// query (AD-2).
	Clock core.Clock
}

// NewHandler returns the public API v1 below /v1/: the operations of
// api/v1/openapi.yaml, the spec itself at /v1/openapi.yaml, open CORS on
// every answer, 405 for write methods and 404 for unknown paths.
func NewHandler(cfg Config) http.Handler {
	return newHandler(cfg, server{events: cfg.Events, clock: cfg.Clock})
}

// newHandler is NewHandler with the strict server replaceable for tests.
func newHandler(cfg Config, strictServer StrictServerInterface) http.Handler {
	respond := responder{logger: cfg.Logger}
	mux := http.NewServeMux()
	strict := NewStrictHandlerWithOptions(strictServer, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  respond.internalServerError,
		ResponseErrorHandlerFunc: respond.internalServerError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL:          basePath,
		BaseRouter:       mux,
		ErrorHandlerFunc: respond.invalidParameter,
	})
	mux.HandleFunc(http.MethodGet+" "+specPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(headerContentType, specContentType)
		respond.writeBody(w, apispec.OpenAPISpec)
	})
	mux.HandleFunc(basePath+"/", respond.notFound)
	return readOnlyCORS(mux, respond)
}

// readOnlyCORS allows every origin on every answer, answers preflight
// requests itself and rejects every method but GET, HEAD and OPTIONS
// before routing (NFR-1).
func readOnlyCORS(next http.Handler, respond responder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerAllowOrigin, anyOrigin)
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			next.ServeHTTP(w, r)
		case http.MethodOptions:
			answerPreflight(w, r)
		default:
			respond.methodNotAllowed(w, r)
		}
	})
}

// answerPreflight allows the read methods and whatever headers the browser
// asks for, since the API has no credentials to protect.
func answerPreflight(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set(headerAllow, allowedMethods)
	header.Set(headerAllowMethods, allowedMethods)
	header.Set(headerMaxAge, preflightMaxAgeSeconds)
	header.Add(headerVary, headerRequestHeaders)
	if requested := r.Header.Get(headerRequestHeaders); requested != "" {
		header.Set(headerAllowHeaders, requested)
	}
	w.WriteHeader(http.StatusNoContent)
}
