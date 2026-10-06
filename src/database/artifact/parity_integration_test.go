// parity_integration_test.go verifies the legacy flat repositories and the
// artifact family store return and persist identical artifact, blob, and
// run-artifact evidence on separate copies of one migrated fixture.
//go:build integration

package artifact_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/database"
	"analysis/database/artifact"
)

// TestArtifactFamilyParityWithLegacyRepositories verifies the adapters and family store agree on reads, writes, and errors.
func TestArtifactFamilyParityWithLegacyRepositories(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := fixture.Run.StartRun(ctx, "artifact-parity", "")
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := fixture.Artifacts.Create("parity-hash", "application/json", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.ArtifactBlobs.Create(artifactID, runID, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := fixture.RunArtifacts.Link(runID, artifactID, database.RunArtifactWorkspaceConfig); err != nil {
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
	store := artifact.New(family.DB)

	// Read parity for artifact identity and blob bytes.
	legacyArtifact, err := legacy.Artifacts.GetByHash("parity-hash")
	if err != nil {
		t.Fatal(err)
	}
	familyArtifact, err := store.GetByHash(ctx, "parity-hash")
	if err != nil {
		t.Fatal(err)
	}
	assertArtifactsEqual(t, "GetByHash", legacyArtifact, familyArtifact)

	legacyByID, err := legacy.Artifacts.GetByID(artifactID)
	if err != nil {
		t.Fatal(err)
	}
	familyByID, err := store.GetByID(ctx, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	assertArtifactsEqual(t, "GetByID", legacyByID, familyByID)

	legacyBlob, err := legacy.ArtifactBlobs.GetByArtifactID(artifactID)
	if err != nil {
		t.Fatal(err)
	}
	familyBlob, err := store.GetBlobByArtifactID(ctx, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	assertBlobsEqual(t, "GetByArtifactID", legacyBlob, familyBlob)

	// Missing-read identity parity.
	legacyMissing, err := legacy.Artifacts.GetByHash("absent")
	if err != nil || legacyMissing != nil {
		t.Fatalf("legacy missing artifact = %+v err=%v", legacyMissing, err)
	}
	familyMissing, err := store.GetByHash(ctx, "absent")
	if err != nil || familyMissing != nil {
		t.Fatalf("family missing artifact = %+v err=%v", familyMissing, err)
	}

	// Artifact write parity.
	legacyNewArtifact, err := legacy.Artifacts.Create("parity-new", "text/plain", 7)
	if err != nil {
		t.Fatal(err)
	}
	familyNewArtifact, err := store.Create(ctx, artifact.CreateInput{ContentHash: "parity-new", ContentType: "text/plain", ByteSize: 7})
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewArtifact != familyNewArtifact {
		t.Fatalf("Create ID parity: legacy=%d family=%d", legacyNewArtifact, familyNewArtifact)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, content_hash, byte_size, content_type FROM artifacts WHERE id=?", legacyNewArtifact)

	// Atomic artifact-plus-blob write parity.
	legacyAtomic, err := legacy.Artifacts.CreateWithBlob("parity-atomic", "application/json", 3, runID, []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	familyAtomic, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "parity-atomic", ContentType: "application/json", ByteSize: 3, PipelineRunID: runID, Data: []byte("two"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyAtomic != familyAtomic {
		t.Fatalf("CreateWithBlob ID parity: legacy=%d family=%d", legacyAtomic, familyAtomic)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, content_hash, byte_size, content_type FROM artifacts WHERE id=?", legacyAtomic)
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT artifact_id, pipeline_run_id, data FROM artifact_blobs WHERE artifact_id=?", legacyAtomic)

	// Blob write parity.
	legacyNewBlob, err := legacy.ArtifactBlobs.Create(legacyNewArtifact, runID, []byte("seven!!"))
	if err != nil {
		t.Fatal(err)
	}
	familyNewBlob, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: familyNewArtifact, PipelineRunID: runID, Data: []byte("seven!!")})
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewBlob != familyNewBlob {
		t.Fatalf("CreateBlob ID parity: legacy=%d family=%d", legacyNewBlob, familyNewBlob)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, artifact_id, pipeline_run_id, data FROM artifact_blobs WHERE id=?", legacyNewBlob)

	// Run-artifact link parity.
	if err := legacy.RunArtifacts.Link(runID, legacyNewArtifact, database.RunArtifactResolvedManifest); err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: familyNewArtifact, Role: artifact.RunArtifactResolvedManifest}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, artifact_id, artifact_role FROM run_artifacts WHERE pipeline_run_id=? ORDER BY artifact_role", runID)

	// Error parity for conflicting metadata, bytes, and role assignments.
	assertErrorParity(t, "conflicting metadata",
		legacyCreateError(legacy, "parity-hash", "text/plain", 3),
		familyCreateError(ctx, store, "parity-hash", "text/plain", 3))
	assertErrorParity(t, "conflicting blob bytes",
		legacyBlobError(legacy, artifactID, runID, []byte("two")),
		familyBlobError(ctx, store, artifactID, runID, []byte("two")))
	assertErrorParity(t, "conflicting role link",
		legacy.RunArtifacts.Link(runID, legacyNewArtifact, database.RunArtifactWorkspaceConfig),
		store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: familyNewArtifact, Role: artifact.RunArtifactWorkspaceConfig}))
}

// legacyCreateError attempts one conflicting artifact create and returns only its error.
func legacyCreateError(db *database.Database, hash, contentType string, byteSize int64) error {
	_, err := db.Artifacts.Create(hash, contentType, byteSize)
	return err
}

// familyCreateError attempts one conflicting artifact create and returns only its error.
func familyCreateError(ctx context.Context, store *artifact.Store, hash, contentType string, byteSize int64) error {
	_, err := store.Create(ctx, artifact.CreateInput{ContentHash: hash, ContentType: contentType, ByteSize: byteSize})
	return err
}

// legacyBlobError attempts one conflicting blob create and returns only its error.
func legacyBlobError(db *database.Database, artifactID, runID int64, data []byte) error {
	_, err := db.ArtifactBlobs.Create(artifactID, runID, data)
	return err
}

// familyBlobError attempts one conflicting blob create and returns only its error.
func familyBlobError(ctx context.Context, store *artifact.Store, artifactID, runID int64, data []byte) error {
	_, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: data})
	return err
}

// assertArtifactsEqual compares legacy and family artifact projections.
func assertArtifactsEqual(t *testing.T, operation string, legacy *database.Artifact, family *artifact.Artifact) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.ContentHash != family.ContentHash ||
		legacy.ByteSize != family.ByteSize || legacy.ContentType != family.ContentType ||
		legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertBlobsEqual compares legacy and family blob projections.
func assertBlobsEqual(t *testing.T, operation string, legacy *database.ArtifactBlob, family *artifact.Blob) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.ArtifactID != family.ArtifactID ||
		legacy.PipelineRunID != family.PipelineRunID || !reflect.DeepEqual(legacy.Data, family.Data) ||
		legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertQueryParity runs the same read on both databases and compares every returned row.
func assertQueryParity(t *testing.T, legacy, family *sql.DB, query string, args ...any) {
	t.Helper()
	legacyRows := queryRowValues(t, legacy, query, args...)
	familyRows := queryRowValues(t, family, query, args...)
	if !reflect.DeepEqual(legacyRows, familyRows) {
		t.Fatalf("query parity mismatch:\nlegacy=%v\nfamily=%v", legacyRows, familyRows)
	}
}

// queryRowValues renders every row of one query as ordered comparable text.
func queryRowValues(t *testing.T, db *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
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

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
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
