// Package migrations embeds the ordered SQL migrations for PostgreSQL.
package migrations

import "embed"

// FS holds the migration files, applied in filename order.
//
//go:embed *.sql
var FS embed.FS
