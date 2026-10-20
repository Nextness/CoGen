// constructor_integration_test.go verifies the family constructor only binds
// the caller's already configured connection.
//go:build integration

package review_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"analysis/database/review"

	_ "modernc.org/sqlite"
)

// TestNewBindsConfiguredConnectionWithoutSideEffects verifies the constructor neither opens, closes, reconfigures, nor migrates the supplied connection.
func TestNewBindsConfiguredConnectionWithoutSideEffects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unmigrated.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	defer conn.Close()

	readPragmas := func() (string, int, int) {
		t.Helper()
		var journalMode string
		if err := conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatal(err)
		}
		var foreignKeys, busyTimeout int
		if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatal(err)
		}
		return journalMode, foreignKeys, busyTimeout
	}
	modeBefore, foreignKeysBefore, busyTimeoutBefore := readPragmas()

	store := review.New(conn)
	if store == nil {
		t.Fatal("New returned nil")
	}

	modeAfter, foreignKeysAfter, busyTimeoutAfter := readPragmas()
	if modeBefore != modeAfter || foreignKeysBefore != foreignKeysAfter || busyTimeoutBefore != busyTimeoutAfter {
		t.Fatalf("constructor altered pragmas: before=(%s,%d,%d) after=(%s,%d,%d)",
			modeBefore, foreignKeysBefore, busyTimeoutBefore, modeAfter, foreignKeysAfter, busyTimeoutAfter)
	}

	var tables int
	if err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("constructor created schema: tables=%d", tables)
	}

	if _, err := conn.Exec("CREATE TABLE constructor_probe (id INTEGER)"); err != nil {
		t.Fatalf("caller connection unusable after New: %v", err)
	}
	if _, err := conn.Exec("DROP TABLE constructor_probe"); err != nil {
		t.Fatal(err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetContextByRun(context.Background(), 1); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
