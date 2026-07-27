package migrations

import "embed"

// FS embeds the Goose SQL migrations so the integration test harness can reset
// and re-apply the schema in-process (setup_api_test.go), with no dependency on
// the working directory or an external goose binary.
//
// The Goose CLI used by the dev scripts and CI still reads the .sql files
// directly from this directory; it ignores this .go file (its name has no
// migration version prefix, so it is never treated as a Go migration).
//
//go:embed *.sql
var FS embed.FS
