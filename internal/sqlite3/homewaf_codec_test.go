// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package sqlite3

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestEncryptedLegacyDatabaseCompatibility(t *testing.T) {
	legacy, err := sql.Open("sqlite3", "file:testdata/legacy-aes128.db?mode=ro&_db_key=homewaf-test-fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	var value string
	if err = legacy.QueryRow("SELECT value FROM compatibility").Scan(&value); err != nil || value != "legacy-encrypted-row" {
		t.Fatalf("legacy encrypted database: value=%q err=%v", value, err)
	}
	wrong, err := sql.Open("sqlite3", "file:testdata/legacy-aes128.db?mode=ro&_db_key=wrong-key")
	if err != nil {
		t.Fatal(err)
	}
	defer wrong.Close()
	if err = wrong.QueryRow("SELECT value FROM compatibility").Scan(&value); err == nil {
		t.Fatal("incorrect database key accepted")
	}
	path := filepath.Join(t.TempDir(), "encrypted.db")
	db, err := sql.Open("sqlite3", path+"?_db_key=new-fixture-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE fixture(value TEXT); INSERT INTO fixture VALUES ('new-encrypted-row')"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	db, err = sql.Open("sqlite3", path+"?_db_key=new-fixture-key")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.QueryRow("SELECT value FROM fixture").Scan(&value); err != nil || value != "new-encrypted-row" {
		t.Fatalf("encrypted reopen: %q %v", value, err)
	}
}
