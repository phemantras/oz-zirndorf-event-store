// Package v1 embeds the contracts of the public API v1, so the public API
// can serve them: the OpenAPI spec at /v1/openapi.yaml and the import
// schema at /v1/import-v1.schema.json.
package v1

import _ "embed"

// OpenAPISpec is the content of openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPISpec []byte

// ImportSchemaV1 is the JSON Schema of the import file format v1. It
// includes EventInput from openapi.yaml by $ref.
//
//go:embed import-v1.schema.json
var ImportSchemaV1 []byte
