// Package migrations embeds the SQL migration files so the server can apply
// them on startup without shipping the .sql files alongside the binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
