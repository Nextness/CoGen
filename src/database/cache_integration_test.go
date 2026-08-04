// Integration tests for cache entry and run cache use repositories.
//go:build integration

package database

import (
	"path/filepath"
	"sync"
	"testing"

	"analysis/manifest"
)

// TestCacheEntryUpsertAndKeySeparation verifies upsert identity preservation
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
	if updatedID != id {
		t.Fatalf("upsert changed row identity: got %d, want %d", updatedID, id)
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
	if count != 1 {
		t.Fatalf("concurrent upsert created %d rows, want 1", count)
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
	if entries != 1 || uses != workers || foreignKeyProblems != 0 {
		t.Fatalf("concurrent integrity entries=%d uses=%d foreign_key_problems=%d", entries, uses, foreignKeyProblems)
	}
}
