// Integration tests for cache entry and run cache use repositories.
//go:build integration

package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"analysis/database/cache"
	"analysis/database/run"
	"analysis/database/search"
	"analysis/manifest"
)

// TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity verifies concurrent database instances preserve cache and attempt integrity.
func TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "workspace.db")
	first, err := Open(dbPath, testConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	searchID, err := first.Search.FindOrCreateSearch(ctx, "concurrent-cache")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := first.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	planID, err := first.Search.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "concurrent-cache-plan", ResolvedManifestHash: "manifest",
	})
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
			runID, _, err := db.Run.StartAttempt(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "concurrent-cache"})
			if err != nil {
				errs <- err
				return
			}
			entryID, err := db.Cache.AppendEntry(ctx, &cache.Entry{
				Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "same-request",
				ResponseStatus: 200, FetchedAt: "2026-07-27T00:00:00Z", ExtractorVersion: "concurrency-test",
			})
			if err != nil {
				errs <- err
				return
			}
			_, err = db.Cache.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: entryID, CacheLayer: "global", Outcome: string(manifest.CacheHit)})
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
	if err := first.RawDBForTest().QueryRow(`SELECT COUNT(*) FROM cache_entries
        WHERE provider='crossref' AND namespace='work_by_doi'
          AND request_fingerprint='same-request' AND extractor_version='concurrency-test'`).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := first.RawDBForTest().QueryRow("SELECT COUNT(*) FROM run_cache_uses").Scan(&uses); err != nil {
		t.Fatal(err)
	}
	if err := first.RawDBForTest().QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyProblems); err != nil {
		t.Fatal(err)
	}
	if entries != workers || uses != workers || foreignKeyProblems != 0 {
		t.Fatalf("concurrent integrity entries=%d uses=%d foreign_key_problems=%d", entries, uses, foreignKeyProblems)
	}
}

// TestImmutableCacheMigrationPreservesHistory verifies V00028 preserves legacy IDs, payloads, and foreign keys during a real upgrade.
func TestImmutableCacheMigrationPreservesHistory(t *testing.T) {
	ctx := context.Background()
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
	db := &Database{db: conn}
	db.initRepositories()
	runID, err := db.Run.StartRun(ctx, "legacy", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO cache_entries (id,provider,namespace,request_fingerprint,response_status,fetched_at,extractor_version) VALUES (42,'crossref','work_by_doi','key',404,'2026-01-01','v1')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Cache.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: 42, CacheLayer: "global", Outcome: "negative"}); err != nil {
		t.Fatal(err)
	}
	up, err := extractUpSQL(filepath.Join("..", "..", "migrations", "corpus.metadata", "V00028_immutable_cache_responses.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(up); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	id, err := db.Cache.AppendEntry(ctx, &cache.Entry{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "key", ResponseStatus: 200, FetchedAt: "2026-01-02", ExtractorVersion: "v1"})
	if err != nil || id <= 42 {
		t.Fatalf("new response ID=%d err=%v", id, err)
	}
	old, err := db.Cache.FindAnyEntry(ctx, runID, cache.Key{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "key", ExtractorVersion: "v1"})
	if err != nil || old == nil || old.ID != 42 || old.ResponseStatus != 404 {
		t.Fatalf("legacy use changed: %+v %v", old, err)
	}
	var failures int
	if err := conn.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&failures); err != nil || failures != 0 {
		t.Fatalf("foreign key failures=%d err=%v", failures, err)
	}
}
