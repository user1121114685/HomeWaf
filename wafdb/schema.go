// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type schemaIndex struct {
	table, name, ddl string
}

// initializeSchema creates missing objects from the current model. It does not
// replay upstream migrations, convert legacy records, or delete user data.
func initializeSchema(db *gorm.DB, models []any, indexes []schemaIndex) error {
	for _, model := range models {
		if !db.Migrator().HasTable(model) {
			if err := db.Migrator().CreateTable(model); err != nil {
				return fmt.Errorf("initialize table %T: %w", model, err)
			}
		}
	}
	for _, index := range indexes {
		ddl := index.ddl
		if db.Dialector.Name() == "mysql" {
			ddl = strings.Replace(ddl, "IF NOT EXISTS ", "", 1)
			// MySQL requires a prefix length for the TEXT rule column.
			ddl = strings.Replace(ddl, ", rule,", ", rule(191),", 1)
		}
		if !db.Migrator().HasIndex(index.table, index.name) {
			if err := db.Exec(ddl).Error; err != nil {
				return fmt.Errorf("initialize index %s: %w", index.name, err)
			}
		}
	}
	return nil
}
