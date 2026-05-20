// Package skill exposes the canonical ezida-kanban SKILL.md as
// embedded bytes. The file at internal/skill/SKILL.md is the single
// source of truth for the embedded skill; refs/SKILL.md is the
// historical human reference and is NOT propagated automatically (see
// the HTML comment at the top of SKILL.md).
package skill

import _ "embed"

// Bytes is the verbatim content of internal/skill/SKILL.md, embedded
// at build time via //go:embed. The build fails if SKILL.md is
// missing.
//
//go:embed SKILL.md
var Bytes []byte
