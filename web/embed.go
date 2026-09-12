// Package web embeds the production build of the React frontend.
package web

import "embed"

// Dist holds the Vite build output. It is populated by "npm run build" in
// this directory; before the first build only a placeholder is present and
// the server shows a "frontend is not built" page.
//
//go:embed all:dist
var Dist embed.FS
