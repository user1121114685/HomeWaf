// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package sqlite

import (
	"errors"

	"gorm.io/gorm"
	"modernc.org/sqlite"
)

// The error codes to map sqlite errors to gorm errors, here is a reference about error codes for sqlite https://www.sqlite.org/rescode.html.
var errCodes = map[int]error{
	1555: gorm.ErrDuplicatedKey,
	2067: gorm.ErrDuplicatedKey,
	787:  gorm.ErrForeignKeyViolated,
}

// Translate it will translate the error to native gorm errors.
func (dialector Dialector) Translate(err error) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		if translatedErr, found := errCodes[sqliteErr.Code()]; found {
			return translatedErr
		}
	}
	return err
}
