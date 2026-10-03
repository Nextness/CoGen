//go:build integration

package cache_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"analysis/database"
	"analysis/database/cache"
	"analysis/manifest"
)

// TestCacheFamilyParityWithLegacyRepositories verifies the family store and the legacy repositories return and persist identical cache evidence on separate copies of one migrated fixture.
func TestCacheFamilyParityWithLegacyRepositories(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := fixture.Artifacts.Create("parity-cache-payload", "application/json", 8)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := fixture.PipelineRuns.StartRun("parity-cache", "")
	if err != nil {
		t.Fatal(err)
	}
	recordedID, err := fixture.CacheEntries.Upsert(&database.CacheEntry{
		Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "parity-key",
		ResponseStatus: 200, PayloadArtifactID: &artifactID,
		FetchedAt: "2026-01-01T00:00:00Z", ExpiresAt: "2026-02-01T00:00:00Z", ExtractorVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	newerID, err := fixture.CacheEntries.Upsert(&database.CacheEntry{
		Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "parity-key",
		ResponseStatus: 404, FetchedAt: "2026-01-02T00:00:00Z", ExtractorVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if newerID == recordedID {
		t.Fatal("legacy fixture reused an immutable response ID")
	}
	if _, err := fixture.RunCacheUses.Create(&database.RunCacheUse{
		PipelineRunID: runID, CacheEntryID: recordedID, CacheLayer: "global", Outcome: string(manifest.CacheHit),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatal(err)
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	familyPath := filepath.Join(t.TempDir(), "family.db")
	copyDatabaseFile(t, fixturePath, legacyPath)
	copyDatabaseFile(t, fixturePath, familyPath)

	legacy, err := database.OpenExisting(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	family, err := database.OpenExisting(familyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer family.Close()
	store := cache.New(family.DB)

	key := cache.Key{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "parity-key", ExtractorVersion: "v1"}

	legacyLatest, err := legacy.CacheEntries.Get(key.Provider, key.Namespace, key.RequestFingerprint, key.ExtractorVersion)
	if err != nil {
		t.Fatal(err)
	}
	familyLatest, err := store.LatestEntry(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	assertEntriesEqual(t, "LatestEntry", legacyLatest, familyLatest)

	legacyGlobal, err := legacy.CacheEntries.GetGlobal(key.Provider, key.Namespace, key.RequestFingerprint, key.ExtractorVersion)
	if err != nil {
		t.Fatal(err)
	}
	familyGlobal, err := store.GlobalEntry(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	assertEntriesEqual(t, "GlobalEntry", legacyGlobal, familyGlobal)

	legacyFind, err := legacy.RunCacheUses.FindEntry(runID, "global", key.Provider, key.Namespace, key.RequestFingerprint, key.ExtractorVersion)
	if err != nil {
		t.Fatal(err)
	}
	familyFind, err := store.FindEntry(ctx, runID, "global", key)
	if err != nil {
		t.Fatal(err)
	}
	assertEntriesEqual(t, "FindEntry", legacyFind, familyFind)

	legacyAny, err := legacy.RunCacheUses.FindAnyEntry(runID, key.Provider, key.Namespace, key.RequestFingerprint, key.ExtractorVersion)
	if err != nil {
		t.Fatal(err)
	}
	familyAny, err := store.FindAnyEntry(ctx, runID, key)
	if err != nil {
		t.Fatal(err)
	}
	assertEntriesEqual(t, "FindAnyEntry", legacyAny, familyAny)

	legacyUses, err := legacy.RunCacheUses.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	familyUses, err := store.ListUsesByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	assertUsesEqual(t, "ListUsesByRun", legacyUses, familyUses)

	absentKey := cache.Key{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "absent", ExtractorVersion: "v1"}
	legacyMissing, err := legacy.CacheEntries.Get(absentKey.Provider, absentKey.Namespace, absentKey.RequestFingerprint, absentKey.ExtractorVersion)
	if err != nil {
		t.Fatal(err)
	}
	familyMissing, err := store.LatestEntry(ctx, absentKey)
	if err != nil {
		t.Fatal(err)
	}
	assertEntriesEqual(t, "LatestEntry absent", legacyMissing, familyMissing)

	legacyEmpty, err := legacy.RunCacheUses.ListByRun(runID + 999)
	if err != nil {
		t.Fatal(err)
	}
	familyEmpty, err := store.ListUsesByRun(ctx, runID+999)
	if err != nil {
		t.Fatal(err)
	}
	if legacyEmpty == nil || familyEmpty == nil || len(legacyEmpty) != 0 || len(familyEmpty) != 0 {
		t.Fatalf("empty list parity: legacy=%#v family=%#v", legacyEmpty, familyEmpty)
	}

	legacyWriteID, err := legacy.CacheEntries.Upsert(&database.CacheEntry{
		Provider: "openalex", Namespace: "works", RequestFingerprint: "parity-write",
		ResponseStatus: 201, FetchedAt: "2026-03-01T00:00:00Z", ExtractorVersion: "v2",
	})
	if err != nil {
		t.Fatal(err)
	}
	familyWriteID, err := store.AppendEntry(ctx, &cache.Entry{
		Provider: "openalex", Namespace: "works", RequestFingerprint: "parity-write",
		ResponseStatus: 201, FetchedAt: "2026-03-01T00:00:00Z", ExtractorVersion: "v2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyWriteID != familyWriteID {
		t.Fatalf("appended entry ID mismatch: legacy=%d family=%d", legacyWriteID, familyWriteID)
	}
	assertRowEqual(t, legacy.DB, family.DB, "cache_entries",
		"id, provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, expires_at, extractor_version", legacyWriteID)

	legacyUseID, err := legacy.RunCacheUses.Create(&database.RunCacheUse{
		PipelineRunID: runID, CacheEntryID: legacyWriteID, CacheLayer: "global", Outcome: string(manifest.CacheMiss),
	})
	if err != nil {
		t.Fatal(err)
	}
	familyUseID, err := store.AppendUse(ctx, &cache.Use{
		PipelineRunID: runID, CacheEntryID: familyWriteID, CacheLayer: "global", Outcome: string(manifest.CacheMiss),
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyUseID != familyUseID {
		t.Fatalf("appended use ID mismatch: legacy=%d family=%d", legacyUseID, familyUseID)
	}
	assertRowEqual(t, legacy.DB, family.DB, "run_cache_uses",
		"id, pipeline_run_id, cache_entry_id, cache_layer, outcome", legacyUseID)

	assertErrorParity(t, "nil entry",
		legacyEntryError(legacy.CacheEntries, nil),
		familyEntryError(ctx, store, nil))
	assertErrorParity(t, "missing entry namespace",
		legacyEntryError(legacy.CacheEntries, &database.CacheEntry{Provider: "crossref", RequestFingerprint: "key", ExtractorVersion: "v1", FetchedAt: "2026-01-01T00:00:00Z", ResponseStatus: 200}),
		familyEntryError(ctx, store, &cache.Entry{Provider: "crossref", RequestFingerprint: "key", ExtractorVersion: "v1", FetchedAt: "2026-01-01T00:00:00Z", ResponseStatus: 200}))
	assertErrorParity(t, "invalid entry status",
		legacyEntryError(legacy.CacheEntries, &database.CacheEntry{Provider: "crossref", Namespace: "works", RequestFingerprint: "key", ExtractorVersion: "v1", FetchedAt: "2026-01-01T00:00:00Z", ResponseStatus: 42}),
		familyEntryError(ctx, store, &cache.Entry{Provider: "crossref", Namespace: "works", RequestFingerprint: "key", ExtractorVersion: "v1", FetchedAt: "2026-01-01T00:00:00Z", ResponseStatus: 42}))
	assertErrorParity(t, "nil use",
		legacyUseError(legacy.RunCacheUses, nil),
		familyUseError(ctx, store, nil))
	assertErrorParity(t, "invalid use outcome",
		legacyUseError(legacy.RunCacheUses, &database.RunCacheUse{PipelineRunID: runID, CacheEntryID: legacyWriteID, CacheLayer: "global", Outcome: "invalid"}),
		familyUseError(ctx, store, &cache.Use{PipelineRunID: runID, CacheEntryID: familyWriteID, CacheLayer: "global", Outcome: "invalid"}))
}

// legacyEntryError runs one legacy entry write and returns only its error.
func legacyEntryError(repository *database.CacheEntryRepository, entry *database.CacheEntry) error {
	_, err := repository.Upsert(entry)
	return err
}

// familyEntryError runs one family entry write and returns only its error.
func familyEntryError(ctx context.Context, store *cache.Store, entry *cache.Entry) error {
	_, err := store.AppendEntry(ctx, entry)
	return err
}

// legacyUseError runs one legacy run-cache-use write and returns only its error.
func legacyUseError(repository *database.RunCacheUseRepository, use *database.RunCacheUse) error {
	_, err := repository.Create(use)
	return err
}

// familyUseError runs one family run-cache-use write and returns only its error.
func familyUseError(ctx context.Context, store *cache.Store, use *cache.Use) error {
	_, err := store.AppendUse(ctx, use)
	return err
}

// copyDatabaseFile copies one checkpointed fixture into an independent working copy.
func copyDatabaseFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertEntriesEqual compares one legacy and one family entry projection field by field.
func assertEntriesEqual(t *testing.T, operation string, legacy *database.CacheEntry, family *cache.Entry) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	legacyPayload := int64(0)
	if legacy.PayloadArtifactID != nil {
		legacyPayload = *legacy.PayloadArtifactID
	}
	familyPayload := int64(0)
	if family.PayloadArtifactID != nil {
		familyPayload = *family.PayloadArtifactID
	}
	if legacy.ID != family.ID ||
		legacy.Provider != family.Provider ||
		legacy.Namespace != family.Namespace ||
		legacy.RequestFingerprint != family.RequestFingerprint ||
		legacy.ResponseStatus != family.ResponseStatus ||
		legacyPayload != familyPayload ||
		(legacy.PayloadArtifactID == nil) != (family.PayloadArtifactID == nil) ||
		legacy.FetchedAt != family.FetchedAt ||
		legacy.ExpiresAt != family.ExpiresAt ||
		legacy.ExtractorVersion != family.ExtractorVersion ||
		legacy.CreatedAt != family.CreatedAt ||
		legacy.UpdatedAt != family.UpdatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertUsesEqual compares legacy and family run-cache-use projections in order.
func assertUsesEqual(t *testing.T, operation string, legacy []*database.RunCacheUse, family []*cache.Use) {
	t.Helper()
	if len(legacy) != len(family) {
		t.Fatalf("%s length mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	for index := range legacy {
		if legacy[index].ID != family[index].ID ||
			legacy[index].PipelineRunID != family[index].PipelineRunID ||
			legacy[index].CacheEntryID != family[index].CacheEntryID ||
			legacy[index].CacheLayer != family[index].CacheLayer ||
			legacy[index].Outcome != family[index].Outcome ||
			legacy[index].UsedAt != family[index].UsedAt {
			t.Fatalf("%s item %d mismatch: legacy=%+v family=%+v", operation, index, legacy[index], family[index])
		}
	}
}

// assertRowEqual verifies two databases hold identical values for one identified row.
func assertRowEqual(t *testing.T, legacy, family *sql.DB, table, columns string, id int64) {
	t.Helper()
	legacyRow := databaseRow(t, legacy, table, columns, id)
	familyRow := databaseRow(t, family, table, columns, id)
	if !reflect.DeepEqual(legacyRow, familyRow) {
		t.Fatalf("%s row %d mismatch: legacy=%v family=%v", table, id, legacyRow, familyRow)
	}
}

// databaseRow reads one row into a column-to-text map for cross-database comparison.
func databaseRow(t *testing.T, db *sql.DB, table, columns string, id int64) map[string]string {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("SELECT %s FROM %s WHERE id=?", columns, table), id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatalf("row %d missing from %s", id, table)
	}
	values := make([]any, len(names))
	pointers := make([]any, len(names))
	for index := range values {
		pointers[index] = &values[index]
	}
	if err := rows.Scan(pointers...); err != nil {
		t.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	row := make(map[string]string, len(names))
	for index, name := range names {
		row[name] = formatDatabaseValue(values[index])
	}
	return row
}

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprint(typed)
	}
}

// assertErrorParity verifies legacy and family operations reject input with the same error text.
func assertErrorParity(t *testing.T, operation string, legacyErr, familyErr error) {
	t.Helper()
	if (legacyErr == nil) != (familyErr == nil) {
		t.Fatalf("%s error mismatch: legacy=%v family=%v", operation, legacyErr, familyErr)
	}
	if legacyErr != nil && legacyErr.Error() != familyErr.Error() {
		t.Fatalf("%s error text mismatch: legacy=%q family=%q", operation, legacyErr, familyErr)
	}
}
