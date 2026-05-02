package web

import "embed"

//go:embed all:templates all:static all:locales
var EmbeddedFS embed.FS
