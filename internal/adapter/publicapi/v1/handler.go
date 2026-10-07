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
	headerCacheControl     = "Cache-Control"
	headerContentEncoding  = "Content-Encoding"
	headerAcceptEncoding   = "Accept-Encoding"
)

// Cache-Control values of AD-18. Lists change with time, so they are kept
// shortest; the spec, the import schema and the docs page change only with
// a deployment; the Redoc script only with a Redoc update. No cache may
// store an error.
const (
	cacheControlLists      = "public, max-age=60"
	cacheControlDocuments  = "public, max-age=300"
	cacheControlDocsScript = "public, max-age=86400"
	cacheControlNoStore    = "no-store"
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
		RequestErrorHandlerFunc:  respond.answerFailedRequest,
		ResponseErrorHandlerFunc: respond.answerFailedRequest,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL:          basePath,
		BaseRouter:       mux,
		Middlewares:      []MiddlewareFunc{cacheFor(cacheControlLists)},
		ErrorHandlerFunc: respond.invalidParameter,
	})
	mux.HandleFunc(http.MethodGet+" "+specPath, respond.serveEmbedded(apispec.OpenAPISpec,
		staticAnswer{contentType: specContentType, cacheControl: cacheControlDocuments, compress: true}))
	mux.HandleFunc(http.MethodGet+" "+importSchemaPath, respond.serveEmbedded(apispec.ImportSchemaV1,
		staticAnswer{contentType: importSchemaContentType, cacheControl: cacheControlDocuments, compress: true}))
	mux.HandleFunc(http.MethodGet+" "+docsPath, respond.serveStaticFile(docsPageFile,
		staticAnswer{contentType: htmlContentType, cacheControl: cacheControlDocuments}))
	mux.HandleFunc(http.MethodGet+" "+docsSlashPath, redirectToDocs)
	mux.HandleFunc(http.MethodGet+" "+docsScriptPath, respond.serveStaticFile(docsScriptFile,
		staticAnswer{contentType: javaScriptContentType, cacheControl: cacheControlDocsScript, compress: true}))
	mux.HandleFunc(basePath+"/", respond.notFound)
	return readOnlyCORS(mux, respond)
}

// redirectToDocs sends the client permanently to the docs page.
func redirectToDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, docsPath, http.StatusMovedPermanently)
}

// staticAnswer says how a static file is served.
type staticAnswer struct {
	contentType  string
	cacheControl string
	// compress serves the file gzip-compressed to clients that accept it.
	compress bool
}

// serveEmbedded returns a handler that serves body as answer says. Like
// every static file, it answers every GET with the whole body, so no range
// or precondition answer bypasses Problem Details.
func (rp responder) serveEmbedded(body []byte, answer staticAnswer) http.HandlerFunc {
	return rp.serveBodyCompressedAt(body, answer, gzipLevel)
}

// serveStaticFile returns a handler that serves the embedded file name as
// answer says. The file is read once here; if it is missing, the handler
// answers every request with 500 and logs the file.
func (rp responder) serveStaticFile(name string, answer staticAnswer) http.HandlerFunc {
	body, err := staticFiles.ReadFile(name)
	if err != nil {
		readErr := fmt.Errorf("read embedded file %s: %w", name, err)
		return func(w http.ResponseWriter, r *http.Request) {
			rp.answerFailedRequest(w, r, readErr)
		}
	}
	return rp.serveEmbedded(body, answer)
}

// serveBodyCompressedAt is serveEmbedded with the gzip level replaceable
// for tests. It compresses body once, not per request; if that fails, it
// warns and serves body uncompressed only.
func (rp responder) serveBodyCompressedAt(body []byte, answer staticAnswer, level int) http.HandlerFunc {
	var compressed []byte
	if answer.compress {
		compressed = rp.gzipOrWarn(body, level)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set(headerContentType, answer.contentType)
		header.Set(headerCacheControl, answer.cacheControl)
		if answer.compress {
			// Caches must keep the compressed and the plain answer apart.
			header.Add(headerVary, headerAcceptEncoding)
		}
		if compressed != nil && acceptsGzip(r) {
			header.Set(headerContentEncoding, encodingGzip)
			rp.writeBody(w, compressed)
			return
		}
		rp.writeBody(w, body)
	}
}

// gzipOrWarn returns body compressed at level, or nil after a warning if
// that fails.
func (rp responder) gzipOrWarn(body []byte, level int) []byte {
	compressed, err := gzipOf(body, level)
	if err != nil {
		rp.logger.Warn(logMsgCompressionFailed, "error", err)
		return nil
	}
	return compressed
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
