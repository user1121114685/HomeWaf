package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func openTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(Open(dsn), &gorm.Config{TranslateError: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return db
}

type driverRecord struct {
	ID        uint
	Name      string `gorm:"uniqueIndex"`
	CreatedAt time.Time
	Payload   []byte
}

func TestSQLiteCRUDAndTimeFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "数据库 #100% &.db")
	db := openTestDB(t, FileDSN(path))
	if err := db.AutoMigrate(&driverRecord{}); err != nil {
		t.Fatal(err)
	}
	want := driverRecord{Name: "original", CreatedAt: time.Date(2026, 11, 9, 20, 30, 45, 123456000, time.FixedZone("CST", 8*3600)), Payload: []byte{0, 1, 255}}
	if err := db.Create(&want).Error; err != nil || want.ID == 0 {
		t.Fatalf("insert returning: id=%d err=%v", want.ID, err)
	}
	var got driverRecord
	if err := db.First(&got, want.ID).Error; err != nil || !got.CreatedAt.Equal(want.CreatedAt) || !bytes.Equal(got.Payload, want.Payload) {
		t.Fatalf("round trip: %+v, %v", got, err)
	}
	var utc string
	if err := db.Raw("SELECT datetime(created_at) FROM driver_records WHERE id = ?", want.ID).Scan(&utc).Error; err != nil || utc != "2026-11-09 12:30:45" {
		t.Fatalf("SQL time expression: %q, %v", utc, err)
	}
	var count int64
	if err := db.Model(&driverRecord{}).Where("created_at >= ? AND created_at <= ?", want.CreatedAt, want.CreatedAt).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("time filter: %d, %v", count, err)
	}
	rollback := errors.New("rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&got).Update("name", "rolled-back").Error; err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if err := db.First(&got, want.ID).Error; err != nil || got.Name != "original" {
		t.Fatalf("transaction rollback: %+v, %v", got, err)
	}
	want.Payload = []byte("updated")
	if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoUpdates: clause.AssignmentColumns([]string{"payload"})}).Create(&want).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&got, want.ID).Error; err != nil || string(got.Payload) != "updated" {
		t.Fatalf("upsert: %+v, %v", got, err)
	}
	if err := db.Delete(&got).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&driverRecord{}, want.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, []byte("SQLite format 3\x00")) {
		t.Fatalf("standard file header: %v", err)
	}
}

func TestSQLiteConnectionPragmasAndReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	writer := openTestDB(t, FileDSN(path))
	if err := writer.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	for _, queryOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(queryOnly), func(t *testing.T) {
			dsn := FileDSN(path, "cache_size(-8192)", "temp_store(MEMORY)")
			if queryOnly {
				dsn += "&_query_only=1&mode=ro"
			}
			db := openTestDB(t, dsn)
			raw, _ := db.DB()
			raw.SetMaxOpenConns(3)
			// Holding all three connections forces the pool to open new ones.
			for i := 0; i < 3; i++ {
				conn, err := raw.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				for pragma, expected := range map[string]int{"busy_timeout": 5000, "cache_size": -8192, "temp_store": 2} {
					var got int
					if err := conn.QueryRowContext(context.Background(), "PRAGMA "+pragma).Scan(&got); err != nil || got != expected {
						t.Fatalf("connection %d %s=%d want %d, %v", i, pragma, got, expected, err)
					}
				}
				if queryOnly {
					if _, err := conn.ExecContext(context.Background(), "INSERT INTO items VALUES (?)", i); err == nil {
						t.Fatal("read-only connection accepted a write")
					}
				}
			}
		})
	}
}

func TestSQLiteConstraintTranslation(t *testing.T) {
	db := openTestDB(t, FileDSN(filepath.Join(t.TempDir(), "constraints.db"), "foreign_keys(ON)"))
	for _, stmt := range []string{
		"CREATE TABLE parents(id INTEGER PRIMARY KEY, name TEXT UNIQUE)",
		"CREATE TABLE children(parent_id INTEGER REFERENCES parents(id))",
		"INSERT INTO parents VALUES(1, 'one')",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{"INSERT INTO parents VALUES(1, 'two')", "INSERT INTO parents VALUES(2, 'one')"} {
		if err := db.Exec(stmt).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
			t.Fatalf("duplicate constraint: %v", err)
		}
	}
	if err := db.Exec("INSERT INTO children VALUES(99)").Error; !errors.Is(err, gorm.ErrForeignKeyViolated) {
		t.Fatalf("foreign key constraint: %v", err)
	}
}

func TestSQLiteRejectsEncryptionOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "must-not-be-created.db")
	if _, err := gorm.Open(Open(FileDSN(path)+"?_db_key=unused"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}); err == nil {
		t.Fatal("silently ignored encryption option")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("rejected DSN created a database: %v", err)
	}
}

func TestSQLiteRelativeFilePath(t *testing.T) {
	t.Chdir(t.TempDir())
	path := "./relative #100%.db"
	db := openTestDB(t, FileDSN(path))
	if err := db.Exec("CREATE TABLE relative_path_test(id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
