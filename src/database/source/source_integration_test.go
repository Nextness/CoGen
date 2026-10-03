// source_integration_test.go exercises the run-source store against a database
// created by the production migration runner.
//go:build integration

package source_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/database/source"

	_ "modernc.org/sqlite"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns a source store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*source.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "source.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return source.New(db.DB), db
}

// createTestRun creates one pipeline run for source linkage and returns its ID.
func createTestRun(t *testing.T, db *database.Database, step string) int64 {
	t.Helper()
	runID, err := db.Run.StartRun(context.Background(), step, "")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	return runID
}

// TestStoreCreateSourceAndListPreservesNullableFieldsAndOrdering verifies declared export metadata, optional text identity, and declared ordering.
func TestStoreCreateSourceAndListPreservesNullableFieldsAndOrdering(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "source-create")

	firstID, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID:           runID,
		Name:            "scopus",
		Type:            "csv",
		ExpectedFile:    "corpus/scopus.csv",
		Query:           "TITLE-ABS-KEY(test)",
		RequestedFields: "doi,title,authors",
		ExpectedCount:   4,
		ExportDate:      "2026-01-02",
	})
	if err != nil {
		t.Fatalf("create full source: %v", err)
	}
	if firstID == 0 {
		t.Fatal("expected non-zero source id")
	}
	secondID, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID:        runID,
		Name:         "ieee",
		Type:         "csv",
		ExpectedFile: "corpus/ieee.csv",
	})
	if err != nil {
		t.Fatalf("create minimal source: %v", err)
	}
	if secondID == firstID {
		t.Fatal("distinct sources shared an id")
	}

	sources, err := store.ListSourcesByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].ID != firstID || sources[1].ID != secondID {
		t.Fatalf("source list = %+v, want ids [%d %d]", sources, firstID, secondID)
	}
	full := sources[0]
	if full.SourceName != "scopus" || full.SourceType != "csv" || full.ExpectedFile != "corpus/scopus.csv" ||
		full.Query != "TITLE-ABS-KEY(test)" || full.RequestedFields != "doi,title,authors" || full.ExportDate != "2026-01-02" {
		t.Fatalf("full source = %+v", full)
	}
	if full.ExpectedResultCount == nil || *full.ExpectedResultCount != 4 || full.ObservedResultCount != nil {
		t.Fatalf("full source counts = %+v", full)
	}
	minimal := sources[1]
	if minimal.Query != "" || minimal.RequestedFields != "" || minimal.ExportDate != "" {
		t.Fatalf("minimal optional text = %+v", minimal)
	}
	if minimal.ExpectedResultCount == nil || *minimal.ExpectedResultCount != 0 {
		t.Fatalf("minimal expected count = %v, want stored zero", minimal.ExpectedResultCount)
	}

	empty, err := store.ListSourcesByRun(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty source list = %+v err=%v, want nil without error", empty, err)
	}
}

// TestStoreSetObservedResultCountPreservesComparison verifies the observed count and comparison update.
func TestStoreSetObservedResultCountPreservesComparison(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "source-observed")
	sourceID, err := store.CreateSource(ctx, source.CreateSourceInput{
		RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv", ExpectedCount: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetObservedResultCount(ctx, source.ObservedResultCountInput{
		RunSourceID: sourceID, ObservedResultCount: 3, Comparison: "below",
	}); err != nil {
		t.Fatal(err)
	}

	sources, err := store.ListSourcesByRun(ctx, runID)
	if err != nil || len(sources) != 1 {
		t.Fatalf("source list = %+v err=%v", sources, err)
	}
	if sources[0].ObservedResultCount == nil || *sources[0].ObservedResultCount != 3 || sources[0].ResultCountComparison != "below" {
		t.Fatalf("observed count = %+v, want 3/below", sources[0])
	}
}

// TestStoreCreateSourceRejectsDuplicateRunSourceName verifies the run-and-name uniqueness contract.
func TestStoreCreateSourceRejectsDuplicateRunSourceName(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "source-duplicate")
	input := source.CreateSourceInput{RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv"}
	if _, err := store.CreateSource(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSource(ctx, input); err == nil {
		t.Fatal("duplicate run source was accepted")
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

	store := source.New(conn)
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
	if _, err := store.CreateSource(context.Background(), source.CreateSourceInput{RunID: 1, Name: "closed"}); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
