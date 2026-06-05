// Package migrations embeds the goose SQL migration files so the migrate
// command can run them without depending on files being present on disk at
// runtime (e.g. inside a distroless container).
package migrations

import "embed"

// FS holds every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
