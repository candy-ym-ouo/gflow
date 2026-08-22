package web

import "embed"

// FS contains the static UI so the server binary is self-contained.
//
//go:embed index.html app.js style.css
var FS embed.FS
