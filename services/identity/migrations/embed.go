// Package migrations embeds this service's *.sql migration files into the
// compiled binary so no external file path needs to exist at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
