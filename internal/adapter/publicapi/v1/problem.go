package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const (
	problemContentType = "application/problem+json"
	// problemTypeBlank says that the HTTP status alone names the problem
	// (RFC 9457, section 4.2.1).
	problemTypeBlank = "about:blank"
)

// English details of the problems the API reports.
const (
	detailNotFoundFormat         = "No resource exists at path %s."
	detailMethodNotAllowedFormat = "Method %s is not allowed; the API is read-only and allows GET, HEAD and OPTIONS."
	detailInternalServerError    = "The server failed to answer the request."
	detailInvalidParameterFormat = "Parameter %s is invalid; each parameter but type may be given only once."
	detailInvalidParameters      = "The query parameters are invalid."
)

// Log messages of the public API.
const (
	logMsgRequestFailed    = "public api request failed"
	logMsgRequestCancelled = "public api request cancelled by client"
	logMsgWriteFailed      = "public api response could not be written"
)

// responder writes the bodies of the public API, the spec and RFC 9457
// problem responses, and logs what fails.
type responder struct {
	logger *slog.Logger
}

// writeProblem answers with status and an English detail as
// application/problem+json.
func (rp responder) writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set(headerContentType, problemContentType)
	w.WriteHeader(status)
	// Encoding a Problem cannot fail, so an error is a failed write.
	err := json.NewEncoder(w).Encode(problemOf(status, detail))
	rp.logFailedWrite(err)
}

// writeBody writes body and logs a failed write.
func (rp responder) writeBody(w http.ResponseWriter, body []byte) {
	_, err := w.Write(body)
	rp.logFailedWrite(err)
}

// logFailedWrite logs a write error, since the client can no longer be
// told.
func (rp responder) logFailedWrite(err error) {
	if err != nil {
		rp.logger.Warn(logMsgWriteFailed, "error", err)
	}
}

// notFound answers a path the API does not have.
func (rp responder) notFound(w http.ResponseWriter, r *http.Request) {
	rp.writeProblem(w, http.StatusNotFound, fmt.Sprintf(detailNotFoundFormat, r.URL.Path))
}

// methodNotAllowed answers a method other than GET, HEAD and OPTIONS.
func (rp responder) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerAllow, allowedMethods)
	rp.writeProblem(w, http.StatusMethodNotAllowed, fmt.Sprintf(detailMethodNotAllowedFormat, r.Method))
}

// invalidParameter answers a query parameter the generated server could
// not read, naming the parameter. The generated server calls it.
func (rp responder) invalidParameter(w http.ResponseWriter, _ *http.Request, err error) {
	detail := detailInvalidParameters
	var invalid *InvalidParamFormatError
	if errors.As(err, &invalid) {
		detail = fmt.Sprintf(detailInvalidParameterFormat, invalid.ParamName)
	}
	rp.writeProblem(w, http.StatusBadRequest, detail)
}

// problemOf returns the RFC 9457 body for status and an English detail.
func problemOf(status int, detail string) Problem {
	return Problem{Type: problemTypeBlank, Title: http.StatusText(status), Status: status, Detail: detail}
}

// internalServerError logs err and answers without revealing it. The
// generated strict server calls it when a request or response fails. A
// request the client cancelled is no fault of the server, so it is logged
// as Info, not as Error.
func (rp responder) internalServerError(w http.ResponseWriter, _ *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		rp.logger.Info(logMsgRequestCancelled, "error", err)
	} else {
		rp.logger.Error(logMsgRequestFailed, "error", err)
	}
	rp.writeProblem(w, http.StatusInternalServerError, detailInternalServerError)
}
