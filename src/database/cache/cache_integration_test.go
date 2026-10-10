//go:build integration

package cache_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"analysis/database"
	"analysis/database/cache"
	"analysis/manifest"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns a cache store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*cache.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cache.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return cache.New(db.DB), db
}

// appendTestEntry appends one entry for a key with the supplied status and returns its ID.
func appendTestEntry(t *testing.T, store *cache.Store, key cache.Key, status int) int64 {
	t.Helper()
	id, err := store.AppendEntry(context.Background(), &cache.Entry{
		Provider:           key.Provider,
		Namespace:          key.Namespace,
		RequestFingerprint: key.RequestFingerprint,
		ExtractorVersion:   key.ExtractorVersion,
		ResponseStatus:     status,
		FetchedAt:          "2026-07-22T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("append entry: %v", err)
	}
	return id
}

// TestStoreAppendEntryPreservesImmutableHistory verifies appends create new immutable versions and latest reads map nullable fields.
func TestStoreAppendEntryPreservesImmutableHistory(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	artifactID, err := db.Artifacts.Create("cache-payload-a", "application/json", 12)
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := store.AppendEntry(ctx, &cache.Entry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "request-a",
		ResponseStatus: 200, PayloadArtifactID: &artifactID,
		FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := store.AppendEntry(ctx, &cache.Entry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "request-a",
		ResponseStatus: 404, ExpiresAt: "2026-07-23T00:00:00Z",
		FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondID == firstID {
		t.Fatal("refresh reused an immutable response ID")
	}
	var oldStatus int
	var oldPayload int64
	if err := db.DB.QueryRow("SELECT response_status, payload_artifact_id FROM cache_entries WHERE id=?", firstID).Scan(&oldStatus, &oldPayload); err != nil || oldStatus != 200 || oldPayload != artifactID {
		t.Fatalf("original response changed: %d %d %v", oldStatus, oldPayload, err)
	}
	if _, err := db.DB.Exec("UPDATE cache_entries SET response_status=404 WHERE id=?", firstID); err == nil {
		t.Fatal("immutable response update was accepted")
	}
	latest, err := store.LatestEntry(ctx, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "request-a", ExtractorVersion: "1"})
	if err != nil || latest == nil {
		t.Fatalf("latest entry = %+v err=%v", latest, err)
	}
	if latest.ID != secondID || latest.ResponseStatus != 404 || latest.PayloadArtifactID != nil || latest.ExpiresAt != "2026-07-23T00:00:00Z" {
		t.Fatalf("latest entry = %+v", latest)
	}
	missing, err := store.LatestEntry(ctx, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "absent", ExtractorVersion: "1"})
	if err != nil || missing != nil {
		t.Fatalf("missing entry = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreAppendEntrySeparatesKeysAndNullableFields verifies provider, request, and extractor keys stay independent with nullable round trips.
func TestStoreAppendEntrySeparatesKeysAndNullableFields(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	keys := []cache.Key{
		{Provider: "openalex", Namespace: "works", RequestFingerprint: "request-a", ExtractorVersion: "1"},
		{Provider: "crossref", Namespace: "works", RequestFingerprint: "request-b", ExtractorVersion: "1"},
		{Provider: "crossref", Namespace: "works", RequestFingerprint: "request-a", ExtractorVersion: "2"},
	}
	for index, key := range keys {
		if _, err := store.AppendEntry(ctx, &cache.Entry{
			Provider: key.Provider, Namespace: key.Namespace,
			RequestFingerprint: key.RequestFingerprint, ExtractorVersion: key.ExtractorVersion,
			ResponseStatus: 200 + index, FetchedAt: "2026-07-22T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
	}
	for index, key := range keys {
		entry, err := store.LatestEntry(ctx, key)
		if err != nil || entry == nil || entry.ResponseStatus != 200+index {
			t.Fatalf("key %+v = %+v err=%v", key, entry, err)
		}
		if entry.PayloadArtifactID != nil || entry.ExpiresAt != "" {
			t.Fatalf("nullable fields = %+v, want nil payload and empty expiry", entry)
		}
	}
}

// TestStoreGlobalEntryRequiresPublishedUse verifies global reads only see entries a run published to the global layer.
func TestStoreGlobalEntryRequiresPublishedUse(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.PipelineRuns.StartRun("cache-family-global", "")
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "global-key", ExtractorVersion: "1"}
	entryID := appendTestEntry(t, store, key, 200)
	if entry, err := store.GlobalEntry(ctx, key); err != nil || entry != nil {
		t.Fatalf("unpublished global entry = %+v err=%v, want nil", entry, err)
	}
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: entryID, CacheLayer: "active_run", Outcome: string(manifest.CacheHit)}); err != nil {
		t.Fatal(err)
	}
	if entry, err := store.GlobalEntry(ctx, key); err != nil || entry != nil {
		t.Fatalf("active-run entry leaked into global reads: %+v err=%v", entry, err)
	}
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: entryID, CacheLayer: "global", Outcome: string(manifest.CacheHit)}); err != nil {
		t.Fatal(err)
	}
	published, err := store.GlobalEntry(ctx, key)
	if err != nil || published == nil || published.ID != entryID {
		t.Fatalf("published global entry = %+v err=%v, want %d", published, err, entryID)
	}
	newerID := appendTestEntry(t, store, key, 404)
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: newerID, CacheLayer: "global", Outcome: string(manifest.CacheNegative)}); err != nil {
		t.Fatal(err)
	}
	newest, err := store.GlobalEntry(ctx, key)
	if err != nil || newest == nil || newest.ID != newerID {
		t.Fatalf("newest global entry = %+v err=%v, want %d", newest, err, newerID)
	}
}

// TestStoreFindEntryReplaysRecordedResponse verifies layer-scoped and any-layer replay return the exact recorded entry after a newer version exists.
func TestStoreFindEntryReplaysRecordedResponse(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.PipelineRuns.StartRun("cache-family-replay", "")
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key", ExtractorVersion: "1"}
	recordedID := appendTestEntry(t, store, key, 200)
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: recordedID, CacheLayer: "global", Outcome: string(manifest.CacheHit)}); err != nil {
		t.Fatal(err)
	}
	newerID := appendTestEntry(t, store, key, 404)
	if newerID == recordedID {
		t.Fatal("newer version reused the recorded response ID")
	}
	for attempt := 0; attempt < 2; attempt++ {
		recorded, err := store.FindEntry(ctx, runID, "global", key)
		if err != nil || recorded == nil || recorded.ID != recordedID || recorded.ResponseStatus != 200 {
			t.Fatalf("FindEntry = %+v err=%v, want recorded %d", recorded, err, recordedID)
		}
		anyEntry, err := store.FindAnyEntry(ctx, runID, key)
		if err != nil || anyEntry == nil || anyEntry.ID != recordedID || anyEntry.ResponseStatus != 200 {
			t.Fatalf("FindAnyEntry = %+v err=%v, want recorded %d", anyEntry, err, recordedID)
		}
	}
	if entry, err := store.FindEntry(ctx, runID, "active_run", key); err != nil || entry != nil {
		t.Fatalf("layer mismatch = %+v err=%v, want nil without error", entry, err)
	}
	if entry, err := store.FindAnyEntry(ctx, runID+999, key); err != nil || entry != nil {
		t.Fatalf("run mismatch = %+v err=%v, want nil without error", entry, err)
	}
}

// TestStoreFindAnyEntryMatchesRunScopedExtractorVersions verifies the prior-run read accepts run-scoped extractor version suffixes.
func TestStoreFindAnyEntryMatchesRunScopedExtractorVersions(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.PipelineRuns.StartRun("cache-family-scoped", "")
	if err != nil {
		t.Fatal(err)
	}
	baseKey := cache.Key{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "prior-run", ExtractorVersion: "v1"}
	baseID := appendTestEntry(t, store, baseKey, 200)
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: baseID, CacheLayer: "global", Outcome: string(manifest.CacheHit)}); err != nil {
		t.Fatal(err)
	}
	scopedKey := baseKey
	scopedKey.ExtractorVersion = "v1:run:abc"
	scopedID, err := store.AppendEntry(ctx, &cache.Entry{
		Provider: scopedKey.Provider, Namespace: scopedKey.Namespace,
		RequestFingerprint: scopedKey.RequestFingerprint, ExtractorVersion: scopedKey.ExtractorVersion,
		ResponseStatus: 404, FetchedAt: "2026-07-22T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: scopedID, CacheLayer: "global", Outcome: string(manifest.CacheNegative)}); err != nil {
		t.Fatal(err)
	}
	anyEntry, err := store.FindAnyEntry(ctx, runID, baseKey)
	if err != nil || anyEntry == nil || anyEntry.ID != scopedID || anyEntry.ExtractorVersion != "v1:run:abc" {
		t.Fatalf("FindAnyEntry = %+v err=%v, want scoped %d", anyEntry, err, scopedID)
	}
	exact, err := store.FindEntry(ctx, runID, "global", baseKey)
	if err != nil || exact == nil || exact.ID != baseID {
		t.Fatalf("FindEntry = %+v err=%v, want exact %d", exact, err, baseID)
	}
}

// TestStoreListUsesByRunPreservesOrderAndEmptyIdentity verifies insertion ordering and the non-nil empty slice contract.
func TestStoreListUsesByRunPreservesOrderAndEmptyIdentity(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.PipelineRuns.StartRun("cache-family-uses", "")
	if err != nil {
		t.Fatal(err)
	}
	entryID := appendTestEntry(t, store, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "uses-key", ExtractorVersion: "1"}, 200)
	layers := []string{"global", "active_run", "global"}
	for _, layer := range layers {
		if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: entryID, CacheLayer: layer, Outcome: string(manifest.CacheHit)}); err != nil {
			t.Fatal(err)
		}
	}
	uses, err := store.ListUsesByRun(ctx, runID)
	if err != nil || len(uses) != len(layers) {
		t.Fatalf("uses = %+v err=%v, want %d", uses, err, len(layers))
	}
	for index, layer := range layers {
		if uses[index].CacheLayer != layer || uses[index].CacheEntryID != entryID {
			t.Fatalf("use %d = %+v, want layer %q entry %d", index, uses[index], layer, entryID)
		}
	}
	empty, err := store.ListUsesByRun(ctx, runID+999)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty uses = %#v, want non-nil empty slice", empty)
	}
}

// TestStoreRejectsInvalidInputs verifies family validation runs before any row is written.
func TestStoreRejectsInvalidInputs(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if _, err := store.AppendEntry(ctx, nil); err == nil || err.Error() != "cache entry is required" {
		t.Fatalf("nil entry error = %v", err)
	}
	if _, err := store.AppendEntry(ctx, &cache.Entry{Provider: "crossref"}); err == nil || !strings.Contains(err.Error(), "namespace is required") {
		t.Fatalf("incomplete entry error = %v", err)
	}
	if _, err := store.AppendUse(ctx, nil); err == nil || err.Error() != "pipeline run, cache entry, and cache layer are required" {
		t.Fatalf("nil use error = %v", err)
	}
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: 1, CacheEntryID: 1, CacheLayer: "global", Outcome: "invalid"}); err == nil || !strings.Contains(err.Error(), "invalid cache outcome") {
		t.Fatalf("invalid outcome error = %v", err)
	}
	var entries, uses int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM cache_entries").Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM run_cache_uses").Scan(&uses); err != nil {
		t.Fatal(err)
	}
	if entries != 0 || uses != 0 {
		t.Fatalf("rejected inputs wrote entries=%d uses=%d", entries, uses)
	}
}

// TestStoreConcurrentAppendsRetainEveryVersion verifies concurrent appends keep every immutable version and valid foreign keys.
func TestStoreConcurrentAppendsRetainEveryVersion(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	const writers = 12
	var waitGroup sync.WaitGroup
	errs := make(chan error, writers)
	for index := 0; index < writers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := store.AppendEntry(ctx, &cache.Entry{
				Provider: "crossref", Namespace: "works", RequestFingerprint: "same-request",
				ResponseStatus: 200, FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: "1",
			})
			errs <- err
		}()
	}
	waitGroup.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var entries, foreignKeyProblems int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM cache_entries WHERE request_fingerprint='same-request'").Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyProblems); err != nil {
		t.Fatal(err)
	}
	if entries != writers || foreignKeyProblems != 0 {
		t.Fatalf("concurrent entries=%d foreign_key_problems=%d, want %d and 0", entries, foreignKeyProblems, writers)
	}
}

// TestStoreContextCancellationLeavesNoRows verifies canceled contexts fail reads and writes without partial rows.
func TestStoreContextCancellationLeavesNoRows(t *testing.T) {
	store, db := openFamilyStore(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.AppendEntry(canceled, &cache.Entry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "canceled",
		ResponseStatus: 200, FetchedAt: "2026-07-22T00:00:00Z", ExtractorVersion: "1",
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled append error = %v, want context.Canceled", err)
	}
	if _, err := store.LatestEntry(canceled, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "canceled", ExtractorVersion: "1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read error = %v, want context.Canceled", err)
	}
	var entries int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM cache_entries").Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if entries != 0 {
		t.Fatalf("canceled append wrote %d entries", entries)
	}
}

// TestStoreListCacheUsesForRunOrdersSearchesAndPaginates verifies the joined
// cache-use page preserves ordering, nullable entry fields, search binding, and
// exact totals.
func TestStoreListCacheUsesForRunOrdersSearchesAndPaginates(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "cache-viewer", "")
	if err != nil {
		t.Fatal(err)
	}
	alphaEntry := appendTestEntry(t, store, cache.Key{Provider: "crossref", Namespace: "doi", RequestFingerprint: "alpha", ExtractorVersion: "v1"}, 200)
	betaEntry := appendTestEntry(t, store, cache.Key{Provider: "openalex", Namespace: "doi", RequestFingerprint: "beta", ExtractorVersion: "v1"}, 404)
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: alphaEntry, CacheLayer: "global", Outcome: "hit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: betaEntry, CacheLayer: "global", Outcome: "negative"}); err != nil {
		t.Fatal(err)
	}

	page, err := store.ListCacheUsesForRun(ctx, cache.CacheUseFilter{RunID: runID, Sort: "id", Order: "DESC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("cache use page = %d items total=%d, want 2", len(page.Items), page.Total)
	}
	if page.Items[0].Provider != "openalex" || page.Items[1].Provider != "crossref" {
		t.Fatalf("cache use order = %+v", page.Items)
	}
	if page.Items[0].ExpiresAt != nil || page.Items[0].PayloadArtifactID != nil || page.Items[0].ResponseStatus != 404 {
		t.Fatalf("cache use nullable entry = %+v", page.Items[0])
	}

	filtered, err := store.ListCacheUsesForRun(ctx, cache.CacheUseFilter{RunID: runID, Query: "crossref", Sort: "provider", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].Outcome != "hit" {
		t.Fatalf("filtered cache use page = %+v", filtered.Items)
	}
	if _, err := store.ListCacheUsesForRun(ctx, cache.CacheUseFilter{RunID: runID, Sort: "unknown", Order: "ASC", Page: 1, PerPage: 20}); err == nil {
		t.Fatal("expected an unsupported sort error")
	}
}
