// errors_integration_test.go verifies the audit family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package audit_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"analysis/database/audit"
	"analysis/manifest"

	_ "modernc.org/sqlite"
)

// TestStoreOperationsPropagateConnectionErrors verifies every public store
// operation reports a closed connection as an error rather than a value.
func TestStoreOperationsPropagateConnectionErrors(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"Insert", func() error {
			_, err := store.Insert(ctx, &manifest.AuditEvent{OccurredAt: "2026-01-01", Actor: "pipeline", EntityType: "work", EntityID: "1", Action: manifest.AuditValidationChanged})
			return err
		}},
		{"ListByRun", func() error { _, err := store.ListByRun(ctx, 1); return err }},
		{"ListByEntity", func() error { _, err := store.ListByEntity(ctx, "work", "1"); return err }},
		{"List", func() error { _, err := store.List(ctx, audit.Filter{Limit: 10}); return err }},
		{"Summary", func() error { _, err := store.Summary(ctx, audit.Filter{}); return err }},
		{"Facets", func() error { _, err := store.Facets(ctx, audit.Filter{}); return err }},
		{"RecordedData", func() error { _, err := store.RecordedData(ctx, 1, 1); return err }},
		{"EnrichmentSummary", func() error { _, err := store.EnrichmentSummary(ctx, 1, 1); return err }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed connection did not produce an error")
			}
		})
	}
}

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

	store := audit.New(conn)
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

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Insert(context.Background(), &manifest.AuditEvent{OccurredAt: "2026-01-01", Actor: "pipeline", EntityType: "work", EntityID: "1", Action: manifest.AuditValidationChanged}); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
