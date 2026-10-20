// Integration tests for reference mention migration state.
//go:build integration

package database

import (
	"strings"
	"testing"
)

// TestV00005ReferenceMentionsMigrationApplies verifies the reference-mentions migration remains applied in the current chain.
func TestV00005ReferenceMentionsMigrationApplies(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	var count int
	if err := db.RawDBForTest().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 28 {
		t.Fatalf("expected 28 applied migrations, got %d", count)
	}
	for _, name := range []string{"reference_mentions"} {
		if err := db.RawDBForTest().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count); err != nil || count != 1 {
			t.Fatalf("expected table %q: count=%d err=%v", name, count, err)
		}
	}
	for _, name := range []string{"idx_reference_mentions_revision", "idx_reference_mentions_resolved_work", "reference_mentions_abort_update", "reference_mentions_abort_delete"} {
		typeName := "index"
		if strings.Contains(name, "abort") {
			typeName = "trigger"
		}
		if err := db.RawDBForTest().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type=? AND name=?", typeName, name).Scan(&count); err != nil || count != 1 {
			t.Fatalf("expected %s %q: count=%d err=%v", typeName, name, count, err)
		}
	}
}
