// Package web embeds the built Vue app so the API binary can serve it.
// `npm run build` at the repo root copies apps/app/dist here before compiling.
package web

import "embed"

// Dist holds the built web app under "dist/". It contains only .gitkeep until a build runs.
//
//go:embed all:dist
var Dist embed.FS
