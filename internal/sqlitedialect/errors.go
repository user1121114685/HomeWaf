// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package sqlite

import "errors"

var (
	ErrConstraintsNotImplemented = errors.New("constraints not implemented on sqlite, consider using DisableForeignKeyConstraintWhenMigrating, more details https://github.com/go-gorm/gorm/wiki/GORM-V2-Release-Note-Draft#all-new-migrator")
)
