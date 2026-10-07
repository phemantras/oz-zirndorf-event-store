package v1

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	// gzipLevel compresses the static files once when the handler is
	// built, so the slowest, smallest level costs nothing per request.
	gzipLevel    = gzip.BestCompression
	encodingGzip = "gzip"
	// The separators and the weight parameter of Accept-Encoding
	// (RFC 9110, section 12.5.3).
	codingSeparator    = ","
	parameterSeparator = ";"
	weightPrefix       = "q="
)

// gzipOf returns body compressed with gzip at level.
func gzipOf(body []byte, level int) ([]byte, error) {
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, level)
	if err != nil {
		return nil, fmt.Errorf("create gzip writer: %w", err)
	}
	// Writing to and closing a writer on a bytes.Buffer cannot fail.
	_, _ = writer.Write(body)
	_ = writer.Close()
	return compressed.Bytes(), nil
}

// acceptsGzip reports whether the request's Accept-Encoding lists gzip
// without a weight of zero. Every header value is read, the coding name
// ignores case, and an unreadable weight counts as a refusal.
func acceptsGzip(r *http.Request) bool {
	for _, value := range r.Header.Values(headerAcceptEncoding) {
		for coding := range strings.SplitSeq(value, codingSeparator) {
			name, parameters, _ := strings.Cut(coding, parameterSeparator)
			if strings.EqualFold(strings.TrimSpace(name), encodingGzip) {
				return hasPositiveWeight(parameters)
			}
		}
	}
	return false
}

// hasPositiveWeight reports whether the parameters of a coding leave its
// weight above zero; without a weight it is 1.
func hasPositiveWeight(parameters string) bool {
	for parameter := range strings.SplitSeq(parameters, parameterSeparator) {
		parameter = strings.TrimSpace(parameter)
		if len(parameter) < len(weightPrefix) || !strings.EqualFold(parameter[:len(weightPrefix)], weightPrefix) {
			continue
		}
		weight, err := strconv.ParseFloat(parameter[len(weightPrefix):], 64)
		return err == nil && weight > 0
	}
	return true
}
