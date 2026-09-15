// Integration tests for cache entry and run cache use repositories.
//go:build integration

package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"analysis/manifest"
)

// TestCacheEntryUpsertAndKeySeparation verifies immutable response history
// and key separation across provider/request/version.
func TestCacheEntryUpsertAndKeySeparation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	artifactID, err := db.Artifacts.Create("cache-payload-a", "application/json", 12)
	if err != nil {
		t.Fatal(err)
	}
	entry := &CacheEntry{Provider: "crossref", Namespace: "works", RequestFingerprint: "request-a", ResponseStatus: 200, PayloadArtifactID: &artifactID, FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: "1"}
	id, err := db.CacheEntries.Upsert(entry)
	if err != nil {
		t.Fatal(err)
	}
	entry.ResponseStatus = 404
	entry.PayloadArtifactID = nil
	entry.ExpiresAt = "2026-07-23T00:00:00Z"
	updatedID, err := db.CacheEntries.Upsert(entry)
	if err != nil {
		t.Fatal(err)
	}
	if updatedID == id {
		t.Fatal("refresh reused an immutable response ID")
	}
	var oldStatus int
	var oldPayload int64
	if err := db.DB.QueryRow("SELECT response_status, payload_artifact_id FROM cache_entries WHERE id=?", id).Scan(&oldStatus, &oldPayload); err != nil || oldStatus != 200 || oldPayload != artifactID {
		t.Fatalf("original response changed: %d %d %v", oldStatus, oldPayload, err)
	}
	if _, err := db.DB.Exec("UPDATE cache_entries SET response_status=404 WHERE id=?", id); err == nil {
		t.Fatal("immutable response update was accepted")
	}
	got, err := db.CacheEntries.Get("crossref", "works", "request-a", "1")
	if err != nil || got == nil {
		t.Fatalf("get cache entry: %v, %+v", err, got)
	}
	if got.ResponseStatus != 404 || got.PayloadArtifactID != nil || got.ExpiresAt == "" {
		t.Fatalf("upserted entry = %+v", got)
	}
	for _, key := range []struct{ provider, request, version string }{
		{"openalex", "request-a", "1"}, {"crossref", "request-b", "1"}, {"crossref", "request-a", "2"},
	} {
		if _, err := db.CacheEntries.Upsert(&CacheEntry{Provider: key.provider, Namespace: "works", RequestFingerprint: key.request, ResponseStatus: 200, FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: key.version}); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []struct{ provider, request, version string }{
		{"openalex", "request-a", "1"}, {"crossref", "request-b", "1"}, {"crossref", "request-a", "2"},
	} {
		if got, err := db.CacheEntries.Get(key.provider, "works", key.request, key.version); err != nil || got == nil {
			t.Fatalf("separate cache key missing: %v, %+v", err, got)
		}
	}
}

// TestCacheEntryConcurrentUpsertAndRunUse verifies cache entry concurrent upsert and run use.
func TestCacheEntryConcurrentUpsertAndRunUse(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	const writers = 12
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := db.CacheEntries.Upsert(&CacheEntry{Provider: "crossref", Namespace: "works", RequestFingerprint: "same-request", ResponseStatus: 200, FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: "1"})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM cache_entries WHERE provider='crossref' AND namespace='works' AND request_fingerprint='same-request' AND extractor_version='1'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != writers {
		t.Fatalf("concurrent refresh retained %d responses, want %d", count, writers)
	}
	entry, err := db.CacheEntries.Get("crossref", "works", "same-request", "1")
	if err != nil || entry == nil {
		t.Fatalf("get concurrent entry: %v, %+v", err, entry)
	}
	runID, err := db.PipelineRuns.StartRun("cache-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunCacheUses.Create(&RunCacheUse{PipelineRunID: runID, CacheEntryID: entry.ID, CacheLayer: "global", Outcome: string(manifest.CacheHit)}); err != nil {
		t.Fatal(err)
	}
	uses, err := db.RunCacheUses.ListByRun(runID)
	if err != nil || len(uses) != 1 || uses[0].CacheEntryID != entry.ID {
		t.Fatalf("run cache uses: %v, %+v", err, uses)
	}
	fromRun, err := db.RunCacheUses.FindEntry(runID, "global", "crossref", "works", "same-request", "1")
	if err != nil || fromRun == nil || fromRun.ID != entry.ID {
		t.Fatalf("find run cache entry: %v, %+v", err, fromRun)
	}
	if _, err := db.RunCacheUses.Create(&RunCacheUse{PipelineRunID: runID, CacheEntryID: entry.ID, CacheLayer: "global", Outcome: "invalid"}); err == nil {
		t.Fatal("invalid cache outcome was accepted")
	}
}

// TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity verifies concurrent database instances preserve cache and attempt integrity.
func TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "workspace.db")
	first, err := Open(dbPath, testConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	searchID, err := first.Searches.Create("concurrent-cache")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := first.Revisions.Create(searchID, "r1", "config", "manifest")
	if err != nil {
		t.Fatal(err)
	}
	planID, err := first.Plans.Create(revisionID, "concurrent-cache-plan", "manifest")
	if err != nil {
		t.Fatal(err)
	}

	const workers = 6
	databases := make([]*Database, 0, workers)
	for range workers {
		db, err := Open(dbPath, testConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		databases = append(databases, db)
	}
	defer func() {
		for _, db := range databases {
			_ = db.Close()
		}
	}()

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for _, db := range databases {
		wg.Add(1)
		go func(db *Database) {
			defer wg.Done()
			runID, _, err := db.PipelineRuns.StartAttempt(planID, "concurrent-cache", "")
			if err != nil {
				errs <- err
				return
			}
			entryID, err := db.CacheEntries.Upsert(&CacheEntry{
				Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "same-request",
				ResponseStatus: 200, FetchedAt: "2026-07-27T00:00:00Z", ExtractorVersion: "concurrency-test",
			})
			if err != nil {
				errs <- err
				return
			}
			_, err = db.RunCacheUses.Create(&RunCacheUse{PipelineRunID: runID, CacheEntryID: entryID, CacheLayer: "global", Outcome: string(manifest.CacheHit)})
			errs <- err
		}(db)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var entries, uses, foreignKeyProblems int
	if err := first.DB.QueryRow(`SELECT COUNT(*) FROM cache_entries
        WHERE provider='crossref' AND namespace='work_by_doi'
          AND request_fingerprint='same-request' AND extractor_version='concurrency-test'`).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := first.DB.QueryRow("SELECT COUNT(*) FROM run_cache_uses").Scan(&uses); err != nil {
		t.Fatal(err)
	}
	if err := first.DB.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyProblems); err != nil {
		t.Fatal(err)
	}
	if entries != workers || uses != workers || foreignKeyProblems != 0 {
		t.Fatalf("concurrent integrity entries=%d uses=%d foreign_key_problems=%d", entries, uses, foreignKeyProblems)
	}
}

// TestImmutableCacheMigrationPreservesHistory verifies V00028 preserves legacy IDs, payloads, and foreign keys during a real upgrade.
func TestImmutableCacheMigrationPreservesHistory(t *testing.T) {
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	entries, err := loadMigrationChain(filepath.Join("..", "..", "config", "database.corpus.metadata.something"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries[:len(entries)-1] {
		up, err := extractUpSQL(filepath.Join("..", "..", "migrations", "corpus.metadata", entry.filename))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(up); err != nil {
			t.Fatalf("%s: %v", entry.filename, err)
		}
	}
	db := &Database{DB: conn}
	db.initRepositories()
	runID, err := db.PipelineRuns.StartRun("legacy", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO cache_entries (id,provider,namespace,request_fingerprint,response_status,fetched_at,extractor_version) VALUES (42,'crossref','work_by_doi','key',404,'2026-01-01','v1')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunCacheUses.Create(&RunCacheUse{PipelineRunID: runID, CacheEntryID: 42, CacheLayer: "global", Outcome: "negative"}); err != nil {
		t.Fatal(err)
	}
	up, err := extractUpSQL(filepath.Join("..", "..", "migrations", "corpus.metadata", "V00028_immutable_cache_responses.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.withTx(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec(up); return err }); err != nil {
		t.Fatal(err)
	}
	id, err := db.CacheEntries.Upsert(&CacheEntry{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "key", ResponseStatus: 200, FetchedAt: "2026-01-02", ExtractorVersion: "v1"})
	if err != nil || id <= 42 {
		t.Fatalf("new response ID=%d err=%v", id, err)
	}
	old, err := db.RunCacheUses.FindAnyEntry(runID, "crossref", "work_by_doi", "key", "v1")
	if err != nil || old == nil || old.ID != 42 || old.ResponseStatus != 404 {
		t.Fatalf("legacy use changed: %+v %v", old, err)
	}
	var failures int
	if err := conn.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&failures); err != nil || failures != 0 {
		t.Fatalf("foreign key failures=%d err=%v", failures, err)
	}
}
