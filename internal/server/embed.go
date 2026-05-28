package server

import (
	"embed"
	"mime"
)

// webFS holds the embedded frontend asset tree. Real assets land in
// later viewer phases; v1 ships placeholder files so the embed
// directive resolves and the static-asset route has something to
// serve.
//
//go:embed web
var webFS embed.FS

// init registers MIME types not built into the standard library so
// http.FileServerFS serves them with a sensible Content-Type. .jsx is
// fetched by Babel-standalone via <script type="text/babel" src=…>;
// browsers tolerate any text MIME there but reporting it as JS keeps
// the dev console quiet.
func init() {
	_ = mime.AddExtensionType(".jsx", "application/javascript; charset=utf-8")
}
