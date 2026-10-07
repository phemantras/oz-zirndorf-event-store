package v1

import (
	"fmt"
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
	// importSchemaPath serves the JSON Schema of the import file format v1
	// next to the spec, whose EventInput it includes by a relative $ref.
	importSchemaPath        = basePath + "/import-v1.schema.json"
	importSchemaContentType = "application/schema+json"
	// docsPath serves the spec as readable HTML documentation (NFR-3); the
	// page loads the Redoc script from docsScriptPath.
	docsPath       = basePath + "/docs"
	docsScriptPath = docsPath + "/redoc.standalone.js"
	// docsSlashPath matches only the docs path with a trailing slash, which
	// is redirected to docsPath, since the page loads its script and the
	// spec relative to docsPath.
	docsSlashPath = docsPath + "/{$}"
	// docsPageFile and docsScriptFile are the files behind the docs paths
	// in staticFiles.
	docsPageFile   = "static/docs.html"
	docsScriptFile = "static/redoc.standalone.js"
	// The content types are set explicitly, since the extension lookup
	// depends on the operating system.
	htmlContentType       = "text/html; charset=utf-8"
	javaScriptContentType = "text/javascript; charset=utf-8"
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
// api/v1/openapi.yaml, the spec itself at /v1/openapi.yaml and as readable
// documentation at /v1/docs, the import schema at
// /v1/import-v1.schema.json, open CORS on
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
	mux.HandleFunc(http.MethodGet+" "+specPath, respond.serveEmbedded(apispec.OpenAPISpec, specContentType))
	mux.HandleFunc(http.MethodGet+" "+importSchemaPath, respond.serveEmbedded(apispec.ImportSchemaV1, importSchemaContentType))
	mux.HandleFunc(http.MethodGet+" "+docsPath, respond.serveStaticFile(docsPageFile, htmlContentType))
	mux.HandleFunc(http.MethodGet+" "+docsSlashPath, redirectToDocs)
	mux.HandleFunc(http.MethodGet+" "+docsScriptPath, respond.serveStaticFile(docsScriptFile, javaScriptContentType))
	mux.HandleFunc(basePath+"/", respond.notFound)
	return readOnlyCORS(mux, respond)
}

// redirectToDocs sends the client permanently to the docs page.
func redirectToDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, docsPath, http.StatusMovedPermanently)
}

// serveEmbedded returns a handler that serves body with the given
// Content-Type.
func (rp responder) serveEmbedded(body []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(headerContentType, contentType)
		rp.writeBody(w, body)
	}
}

// serveStaticFile returns a handler that serves the embedded file name
// with the given Content-Type. Like the spec, it answers every GET with the
// whole file, so no range or precondition answer bypasses Problem Details.
func (rp responder) serveStaticFile(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := staticFiles.ReadFile(name)
		if err != nil {
			rp.internalServerError(w, r, fmt.Errorf("read embedded file %s: %w", name, err))
			return
		}
		w.Header().Set(headerContentType, contentType)
		rp.writeBody(w, body)
	}
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
