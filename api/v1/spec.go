// Package v1 embeds the OpenAPI contract of the public API v1, so the
// public API can serve it at /v1/openapi.yaml.
package v1

import _ "embed"

// OpenAPISpec is the content of openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
