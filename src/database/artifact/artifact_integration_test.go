// artifact_integration_test.go exercises the artifact store against a database
// created by the production migration runner.
//go:build integration

package artifact_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/database/artifact"

	_ "modernc.org/sqlite"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns an artifact store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*artifact.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "artifact.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return artifact.New(db.DB), db
}

// createTestRun creates one pipeline run for artifact linkage and returns its ID.
func createTestRun(t *testing.T, db *database.Database, step string) int64 {
	t.Helper()
	runID, err := db.Run.StartRun(context.Background(), step, "")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	return runID
}

// TestStoreCreateAndLookupPreservesIdentityAndDeduplicates verifies content-hash reuse and identity lookup.
func TestStoreCreateAndLookupPreservesIdentityAndDeduplicates(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "sha256-abc", ContentType: "application/json", ByteSize: 1024})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if firstID == 0 {
		t.Fatal("expected non-zero artifact id")
	}
	secondID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "sha256-abc", ContentType: "application/json", ByteSize: 1024})
	if err != nil {
		t.Fatalf("create duplicate artifact: %v", err)
	}
	if secondID != firstID {
		t.Fatalf("duplicate artifact id = %d, want %d", secondID, firstID)
	}
	thirdID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "sha256-def", ContentType: "text/plain", ByteSize: 512})
	if err != nil {
		t.Fatalf("create distinct artifact: %v", err)
	}
	if thirdID == firstID {
		t.Fatal("distinct content hash shared an artifact id")
	}

	byHash, err := store.GetByHash(ctx, "sha256-abc")
	if err != nil {
		t.Fatal(err)
	}
	if byHash == nil || byHash.ID != firstID || byHash.ByteSize != 1024 || byHash.ContentType != "application/json" {
		t.Fatalf("artifact by hash = %+v", byHash)
	}
	byID, err := store.GetByID(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if byID == nil || byID.ContentHash != "sha256-abc" {
		t.Fatalf("artifact by id = %+v", byID)
	}

	missingHash, err := store.GetByHash(ctx, "sha256-absent")
	if err != nil || missingHash != nil {
		t.Fatalf("missing artifact by hash = %+v err=%v, want nil without error", missingHash, err)
	}
	missingID, err := store.GetByID(ctx, 99999)
	if err != nil || missingID != nil {
		t.Fatalf("missing artifact by id = %+v err=%v, want nil without error", missingID, err)
	}
}

// TestStoreCreateRejectsConflictingMetadata verifies a repeated identity cannot conceal different metadata.
func TestStoreCreateRejectsConflictingMetadata(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.Create(ctx, artifact.CreateInput{ContentHash: "content-hash", ContentType: "application/json", ByteSize: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, artifact.CreateInput{ContentHash: "content-hash", ContentType: "text/plain", ByteSize: 3}); err == nil {
		t.Fatal("expected conflicting content type to fail")
	}
	if _, err := store.Create(ctx, artifact.CreateInput{ContentHash: "content-hash", ContentType: "application/json", ByteSize: 4}); err == nil {
		t.Fatal("expected conflicting byte size to fail")
	}
}

// TestStoreCreateWithBlobIsAtomicAndIdempotent verifies the artifact and bytes commit together and replay safely.
func TestStoreCreateWithBlobIsAtomicAndIdempotent(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-atomic")

	artifactID, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "atomic-hash", ContentType: "application/json", ByteSize: 3, PipelineRunID: runID, Data: []byte("one"),
	})
	if err != nil {
		t.Fatalf("create artifact with blob: %v", err)
	}
	replayedID, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "atomic-hash", ContentType: "application/json", ByteSize: 3, PipelineRunID: runID, Data: []byte("one"),
	})
	if err != nil {
		t.Fatalf("replay artifact with blob: %v", err)
	}
	if replayedID != artifactID {
		t.Fatalf("replayed artifact id = %d, want %d", replayedID, artifactID)
	}
	blob, err := store.GetBlobByArtifactID(ctx, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	if blob == nil || string(blob.Data) != "one" || blob.PipelineRunID != runID {
		t.Fatalf("stored blob = %+v", blob)
	}

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "atomic-hash", ContentType: "application/json", ByteSize: 3, PipelineRunID: runID, Data: []byte("two"),
	}); err == nil {
		t.Fatal("expected conflicting atomic bytes to fail")
	}
	unchanged, err := store.GetBlobByArtifactID(ctx, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged == nil || string(unchanged.Data) != "one" {
		t.Fatalf("conflicting write changed stored bytes: %+v", unchanged)
	}

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "atomic-size-mismatch", ContentType: "application/json", ByteSize: 4, PipelineRunID: runID, Data: []byte("one"),
	}); err == nil {
		t.Fatal("expected declared byte size mismatch to fail")
	}
	missing, err := store.GetByHash(ctx, "atomic-size-mismatch")
	if err != nil || missing != nil {
		t.Fatalf("failed atomic write left an artifact row: %+v err=%v", missing, err)
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

	store := artifact.New(conn)
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
	if _, err := store.Create(context.Background(), artifact.CreateInput{ContentHash: "closed"}); err == nil {
		t.Fatal("store used a connection the constructor opened instead of the supplied one")
	}
}
