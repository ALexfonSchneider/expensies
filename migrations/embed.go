// Package migrations embeds the SQL schema so the binary can apply it on
// startup regardless of the working directory. The same files are also
// usable with "goplatform migrate up --dir migrations".
package migrations

import "embed"

// FS contains the golang-migrate style *.up.sql / *.down.sql files.
//
//go:embed *.sql
var FS embed.FS
