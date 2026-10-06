// work_integration_test.go exercises the work store against a database created
// by the production migration runner.
//go:build integration

package work_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/database/work"

	_ "modernc.org/sqlite"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns a work store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*work.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "work.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return work.New(db.DB), db
}

// createTestRun creates one pipeline run for work-family linkage and returns its ID.
func createTestRun(t *testing.T, db *database.Database, step string) int64 {
	t.Helper()
	runID, err := db.Run.StartRun(context.Background(), step, "")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	return runID
}

// TestStoreCreateWorkByDOIAndLookupPreservesIdentity verifies DOI
// normalization, idempotent reuse, and ordered reads.
func TestStoreCreateWorkByDOIAndLookupPreservesIdentity(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstID, err := store.CreateWorkByDOI(ctx, "https://doi.org/10.1000/Example")
	if err != nil {
		t.Fatalf("create work by DOI: %v", err)
	}
	if firstID == 0 {
		t.Fatal("expected non-zero work id")
	}
	secondID, err := store.CreateWorkByDOI(ctx, "10.1000/example")
	if err != nil {
		t.Fatalf("create duplicate work by DOI: %v", err)
	}
	if secondID != firstID {
		t.Fatalf("duplicate DOI id = %d, want %d", secondID, firstID)
	}
	thirdID, err := store.CreateWorkByDOI(ctx, "10.1000/other")
	if err != nil {
		t.Fatal(err)
	}
	if thirdID == firstID {
		t.Fatal("distinct DOIs shared a work id")
	}
	if _, err := store.CreateWorkByDOI(ctx, "   "); err == nil {
		t.Fatal("expected an empty DOI to fail")
	}

	byDOI, err := store.GetWorkByDOI(ctx, "https://doi.org/10.1000/EXAMPLE")
	if err != nil {
		t.Fatal(err)
	}
	if byDOI == nil || byDOI.ID != firstID || byDOI.DOI != "10.1000/example" {
		t.Fatalf("work by DOI = %+v", byDOI)
	}
	byID, err := store.GetWorkByID(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if byID == nil || byID.DOI != "10.1000/example" {
		t.Fatalf("work by ID = %+v", byID)
	}
	if missing, err := store.GetWorkByID(ctx, 99999); err != nil || missing != nil {
		t.Fatalf("missing work by ID = %+v err=%v, want nil without error", missing, err)
	}
	if missing, err := store.GetWorkByDOI(ctx, "10.1000/absent"); err != nil || missing != nil {
		t.Fatalf("missing work by DOI = %+v err=%v, want nil without error", missing, err)
	}
	if missing, err := store.GetWorkByDOI(ctx, ""); err != nil || missing != nil {
		t.Fatalf("empty DOI lookup = %+v err=%v, want nil without error", missing, err)
	}

	works, err := store.ListWorksByIDs(ctx, []int64{thirdID, firstID})
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 2 || works[0].ID != firstID || works[1].ID != thirdID {
		t.Fatalf("works by IDs = %+v, want ID order", works)
	}
	if empty, err := store.ListWorksByIDs(ctx, nil); err != nil || empty != nil {
		t.Fatalf("empty ID list = %+v err=%v, want nil without error", empty, err)
	}
	if absent, err := store.ListWorksByIDs(ctx, []int64{99999}); err != nil || absent != nil {
		t.Fatalf("absent ID list = %+v err=%v, want nil without error", absent, err)
	}

	count, err := store.CountWorks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("work count = %d, want 2", count)
	}
}

// TestStoreCreateWorkWithoutDOIAlwaysCreatesDistinctRows verifies uncertain
// title-only records are never globally merged.
func TestStoreCreateWorkWithoutDOIAlwaysCreatesDistinctRows(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstID, err := store.CreateWorkWithoutDOI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := store.CreateWorkWithoutDOI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatal("title-only works must not be merged")
	}
	work, err := store.GetWorkByID(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if work == nil || work.DOI != "" {
		t.Fatalf("title-only work = %+v, want empty DOI", work)
	}
}

// TestStoreInsertIdentifierPreservesOwnershipAndReuse verifies alternate
// identifiers are idempotent per work and cannot be reassigned silently.
func TestStoreInsertIdentifierPreservesOwnershipAndReuse(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstWork, err := store.CreateWorkByDOI(ctx, "10.1000/identifier-first")
	if err != nil {
		t.Fatal(err)
	}
	secondWork, err := store.CreateWorkByDOI(ctx, "10.1000/identifier-second")
	if err != nil {
		t.Fatal(err)
	}

	id, err := store.InsertIdentifier(ctx, work.InsertIdentifierInput{WorkID: firstWork, Namespace: "openalex", Identifier: "W123"})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero identifier id")
	}
	reused, err := store.InsertIdentifier(ctx, work.InsertIdentifierInput{WorkID: firstWork, Namespace: "openalex", Identifier: "W123"})
	if err != nil {
		t.Fatal(err)
	}
	if reused != id {
		t.Fatalf("repeated identifier id = %d, want %d", reused, id)
	}
	if _, err := store.InsertIdentifier(ctx, work.InsertIdentifierInput{WorkID: secondWork, Namespace: "openalex", Identifier: "W123"}); err == nil {
		t.Fatal("expected ownership conflict for a different work")
	}
	if _, err := store.InsertIdentifier(ctx, work.InsertIdentifierInput{WorkID: firstWork, Namespace: "", Identifier: "W123"}); err == nil {
		t.Fatal("expected empty namespace to fail")
	}

	byID, err := store.GetIdentifierByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if byID == nil || byID.WorkID != firstWork || byID.Identifier != "W123" {
		t.Fatalf("identifier by ID = %+v", byID)
	}
	byPair, err := store.GetIdentifierByNamespaceAndIdentifier(ctx, "openalex", "W123")
	if err != nil {
		t.Fatal(err)
	}
	if byPair == nil || byPair.ID != id {
		t.Fatalf("identifier by pair = %+v", byPair)
	}
	if missing, err := store.GetIdentifierByID(ctx, 99999); err != nil || missing != nil {
		t.Fatalf("missing identifier = %+v err=%v, want nil without error", missing, err)
	}

	identifiers, err := store.ListIdentifiersByWorkID(ctx, firstWork)
	if err != nil {
		t.Fatal(err)
	}
	if len(identifiers) != 1 || identifiers[0].ID != id {
		t.Fatalf("identifiers by work = %+v", identifiers)
	}
	if absent, err := store.ListIdentifiersByWorkID(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent identifier list = %+v err=%v, want nil without error", absent, err)
	}
	count, err := store.CountIdentifiersByWorkID(ctx, firstWork)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("identifier count = %d, want 1", count)
	}
}

// TestNewBindsConfiguredConnectionWithoutSideEffects verifies the constructor
// neither opens, closes, reconfigures, nor migrates the supplied connection.
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

	store := work.New(conn)
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
		t.Fatal(err)
	}
	if _, err := conn.Exec("DROP TABLE constructor_probe"); err != nil {
		t.Fatal(err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetWorkByID(context.Background(), 1); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
