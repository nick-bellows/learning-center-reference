// Package migrations embeds the SQL migration files into the binary so the
// server can apply them itself at startup (MIGRATE_ON_START=1). Embedding
// keeps the distroless runtime image workable: no shell, no separate
// migration container, one source of truth for schema.
package migrations

import "embed"

// Files holds every up-migration, ordered by their NNNN_ filename prefix.
//
//go:embed *.up.sql
var Files embed.FS
