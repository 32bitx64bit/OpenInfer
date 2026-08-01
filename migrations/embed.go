// Package migrations exposes the SQL schema migrations as an embedded
// filesystem so the backend binary is self-contained.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
