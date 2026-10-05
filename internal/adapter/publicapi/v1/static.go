package v1

import "embed"

// staticFiles holds the readable API documentation: the docs page and the
// unchanged Redoc bundle it renders the spec with, served without a CDN.
//
//go:embed static
var staticFiles embed.FS
