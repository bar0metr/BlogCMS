package static

import "embed"

// FS contains embedded static web assets (CSS, etc.).
//
//go:embed *.css
var FS embed.FS
