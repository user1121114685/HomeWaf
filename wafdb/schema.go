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
	if err := ensureStorageFormat(db); err != nil {
		return err
	}
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

const storageFormatTable = "homewaf_storage_format_v3"

// Remote databases need the same fresh-install boundary as local data directories.
// The version is in the table name so the entire marker is created by one DDL statement.
func ensureStorageFormat(db *gorm.DB) error {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return err
	}
	marked, populated := false, false
	for _, name := range tables {
		switch {
		case name == storageFormatTable:
			marked = true
		case strings.HasPrefix(name, "homewaf_storage_format"):
			return fmt.Errorf("unsupported HomeWaf database format %q", name)
		case !strings.HasPrefix(name, "sqlite_"):
			populated = true
		}
	}
	if marked {
		return validateStorageFormat(db)
	}
	if populated {
		return fmt.Errorf("HomeWaf v3 requires a new database; existing unversioned tables were found")
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS " + storageFormatTable + " (version INTEGER PRIMARY KEY CHECK (version = 3))").Error; err != nil && !db.Migrator().HasTable(storageFormatTable) {
		return err
	}
	insert := "INSERT INTO " + storageFormatTable + " (version) VALUES (3) ON CONFLICT DO NOTHING"
	if db.Dialector.Name() == "mysql" {
		insert = "INSERT INTO " + storageFormatTable + " (version) VALUES (3) ON DUPLICATE KEY UPDATE version=version"
	}
	if err := db.Exec(insert).Error; err != nil {
		return err
	}
	return validateStorageFormat(db)
}

func validateStorageFormat(db *gorm.DB) error {
	if !db.Migrator().HasTable(storageFormatTable) {
		return fmt.Errorf("source requires HomeWaf v3 storage format")
	}
	if !db.Migrator().HasColumn(storageFormatTable, "version") {
		return fmt.Errorf("invalid HomeWaf database format marker")
	}
	var versions []int
	if err := db.Raw("SELECT version FROM " + storageFormatTable).Scan(&versions).Error; err != nil {
		return err
	}
	if len(versions) != 1 || versions[0] != 3 {
		return fmt.Errorf("unsupported HomeWaf database format %v", versions)
	}
	return nil
}
