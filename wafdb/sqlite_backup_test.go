package wafdb

import (
	"os"
	"path/filepath"
	"testing"

	sqlite "HomeWaf/internal/sqlitedialect"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSQLiteOnlineBackupIncludesWAL(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.db")
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(sourcePath, "journal_mode(WAL)", "wal_autocheckpoint(0)")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := db.DB()
	defer raw.Close()
	if err := db.Exec("CREATE TABLE backup_rows(id INTEGER PRIMARY KEY, value TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(dir, "backup #100%.db")
	for i := 1; i <= 2; i++ {
		if err := db.Exec("INSERT INTO backup_rows VALUES(?, ?)", i, "未 checkpoint 的记录").Error; err != nil {
			t.Fatal(err)
		}
		if fi, err := os.Stat(sourcePath + "-wal"); err != nil || fi.Size() == 0 {
			t.Fatalf("source must have an uncheckpointed WAL: %v", err)
		}
		// Reusing the destination also verifies the online backup API's overwrite
		// behaviour, which VACUUM INTO would not preserve.
		if err := BackupDatabase(db, backupPath); err != nil {
			t.Fatal(err)
		}
		backup, err := gorm.Open(sqlite.Open(sqlite.FileDSN(backupPath)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		backupRaw, _ := backup.DB()
		var count int
		queryErr := backup.Raw("SELECT count(*) FROM backup_rows").Scan(&count).Error
		var integrity string
		integrityErr := backup.Raw("PRAGMA integrity_check").Scan(&integrity).Error
		closeErr := backupRaw.Close()
		if queryErr != nil || count != i || integrityErr != nil || integrity != "ok" || closeErr != nil {
			t.Fatalf("backup rows=%d want=%d integrity=%q, errors: %v %v %v", count, i, integrity, queryErr, integrityErr, closeErr)
		}
	}
	if err := BackupDatabase(db, filepath.Join(dir, "missing", "backup.db")); err == nil {
		t.Fatal("backup to nonexistent directory succeeded")
	}
	if err := db.Exec("INSERT INTO backup_rows VALUES(3, 'source still usable')").Error; err != nil {
		t.Fatalf("backup released or damaged the source connection: %v", err)
	}
}
