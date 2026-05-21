package server

import "embed"

// webFS holds the embedded frontend asset tree. Real assets land in
// later viewer phases; v1 ships placeholder files so the embed
// directive resolves and the static-asset route has something to
// serve.
//
//go:embed web
var webFS embed.FS
