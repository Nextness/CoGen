// search_integration_test.go exercises the search family store against a
// database created by the production migration runner.
//go:build integration

package search_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"analysis/database"
	"analysis/database/search"

	_ "modernc.org/sqlite"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns a search store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*search.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "search.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return search.New(db.DB), db
}

// TestStoreFindOrCreateSearchIsIdempotent verifies repeated creation returns one stable ID and distinct identifiers stay independent.
func TestStoreFindOrCreateSearchIsIdempotent(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstID, err := store.FindOrCreateSearch(ctx, "test-search")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if firstID == 0 {
		t.Fatal("expected non-zero id")
	}
	secondID, err := store.FindOrCreateSearch(ctx, "test-search")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if secondID != firstID {
		t.Fatalf("duplicate create returned %d, want %d", secondID, firstID)
	}
	otherID, err := store.FindOrCreateSearch(ctx, "other-search")
	if err != nil {
		t.Fatalf("distinct create: %v", err)
	}
	if otherID == firstID {
		t.Fatal("distinct searches shared an id")
	}

	byID, err := store.GetSearchByID(ctx, firstID)
	if err != nil || byID == nil {
		t.Fatalf("get by id = %+v err=%v", byID, err)
	}
	if byID.SearchID != "test-search" || byID.CreatedAt == "" {
		t.Fatalf("search by id = %+v", byID)
	}
	byName, err := store.GetSearchBySearchID(ctx, "test-search")
	if err != nil || byName == nil || byName.ID != firstID {
		t.Fatalf("get by search_id = %+v err=%v", byName, err)
	}
}

// TestStoreSearchReadsPreserveNotFoundAndEmptyIdentity verifies optional reads return nil and empty lists stay nil.
func TestStoreSearchReadsPreserveNotFoundAndEmptyIdentity(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	missing, err := store.GetSearchByID(ctx, 999)
	if err != nil || missing != nil {
		t.Fatalf("missing search = %+v err=%v, want nil without error", missing, err)
	}
	missing, err = store.GetSearchBySearchID(ctx, "absent")
	if err != nil || missing != nil {
		t.Fatalf("missing search by name = %+v err=%v, want nil without error", missing, err)
	}
	searches, err := store.ListSearches(ctx)
	if err != nil || searches != nil {
		t.Fatalf("empty search list = %+v err=%v, want nil without error", searches, err)
	}
}

// TestStoreListSearchesOrdersByID verifies the declared ORDER BY rather than insertion or primary-key accident.
func TestStoreListSearchesOrdersByID(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	first, err := store.FindOrCreateSearch(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.FindOrCreateSearch(ctx, "beta")
	if err != nil {
		t.Fatal(err)
	}
	searches, err := store.ListSearches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(searches) != 2 || searches[0].ID != first || searches[1].ID != second {
		t.Fatalf("search list = %+v, want ids [%d %d]", searches, first, second)
	}
}

// TestStoreFindOrCreateSearchSerializesConcurrentInserts verifies concurrent writers converge on one row and one ID.
func TestStoreFindOrCreateSearchSerializesConcurrentInserts(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	const writers = 8
	ids := make([]int64, writers)
	errs := make([]error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ids[index], errs[index] = store.FindOrCreateSearch(ctx, "concurrent-search")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}
	for i := 1; i < writers; i++ {
		if ids[i] != ids[0] {
			t.Fatalf("writer %d returned id %d, want %d", i, ids[i], ids[0])
		}
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM searches WHERE search_id='concurrent-search'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent searches row count = %d, want 1", count)
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

	store := search.New(conn)
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
	if _, err := store.FindOrCreateSearch(context.Background(), "closed"); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
