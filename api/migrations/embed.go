// Package migrations embarque les fichiers SQL dans le binaire.
//
// Le binaire porte donc son propre schema : Coolify n'a ni entrypoint shell ni
// outil migrate a installer, et un rollback d'image ramene mecaniquement le
// schema attendu par cette version du code.
package migrations

import "embed"

// FS expose les migrations au runner golang-migrate.
//
//go:embed *.sql
var FS embed.FS
