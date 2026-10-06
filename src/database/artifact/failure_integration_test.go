// failure_integration_test.go verifies the artifact family fails closed on
// invalid run linkage, canceled contexts, and ignored inserts without leaving
// partial rows.
//go:build integration

package artifact_test

import (
	"context"
	"testing"

	"analysis/database/artifact"
)

// TestStoreCreateWithBlobRejectsMissingRun verifies a foreign-key violation
// rolls back the artifact identity inserted in the same transaction.
func TestStoreCreateWithBlobRejectsMissingRun(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "missing-run-hash", ContentType: "application/json", ByteSize: 1, PipelineRunID: 99999, Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected a missing pipeline run to fail the blob insert")
	}
	missing, err := store.GetByHash(ctx, "missing-run-hash")
	if err != nil || missing != nil {
		t.Fatalf("failed atomic write left an artifact row: %+v err=%v", missing, err)
	}
}

// TestStoreCreateWithBlobRejectsExistingMetadataConflict verifies an existing
// content identity with different metadata is rejected before any blob write.
func TestStoreCreateWithBlobRejectsExistingMetadataConflict(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-metadata-conflict")
	if _, err := store.Create(ctx, artifact.CreateInput{ContentHash: "metadata-conflict", ContentType: "application/json", ByteSize: 1}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "metadata-conflict", ContentType: "text/plain", ByteSize: 1, PipelineRunID: runID, Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected conflicting stored metadata to fail")
	}
	blob, err := store.GetBlobByArtifactID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if blob != nil {
		t.Fatalf("conflicting metadata write stored a blob: %+v", blob)
	}
}

// TestStoreCreateReportsIgnoredInsertWithoutExistingRow verifies an ignored
// insert that leaves no row is reported instead of returning a phantom ID.
func TestStoreCreateReportsIgnoredInsertWithoutExistingRow(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if _, err := db.DB.Exec("CREATE TRIGGER ignore_artifact_insert BEFORE INSERT ON artifacts BEGIN SELECT RAISE(IGNORE); END"); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Create(ctx, artifact.CreateInput{ContentHash: "ignored", ContentType: "application/json", ByteSize: 1}); err == nil {
		t.Fatal("expected an ignored insert without an existing row to fail")
	}
}

// TestStoreCreateWithBlobReportsIgnoredIdentityInsert verifies an ignored
// identity insert fails closed before any blob write.
func TestStoreCreateWithBlobReportsIgnoredIdentityInsert(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-ignored-identity")
	if _, err := db.DB.Exec("CREATE TRIGGER ignore_artifact_insert BEFORE INSERT ON artifacts BEGIN SELECT RAISE(IGNORE); END"); err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "ignored-atomic", ContentType: "application/json", ByteSize: 1, PipelineRunID: runID, Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected an ignored identity insert to fail")
	}
}

// TestStoreCreateWithBlobReportsMissingStoredMetadata verifies a vanished
// identity row fails closed after a successful insert.
func TestStoreCreateWithBlobReportsMissingStoredMetadata(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-vanished-identity")
	if _, err := db.DB.Exec("CREATE TRIGGER delete_artifact_after_insert AFTER INSERT ON artifacts BEGIN DELETE FROM artifacts WHERE id = NEW.id; END"); err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "vanished", ContentType: "application/json", ByteSize: 1, PipelineRunID: runID, Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected a vanished identity row to fail")
	}
}

// TestStoreCreateWithBlobReportsIgnoredBlobInsert verifies an ignored blob
// insert without an existing row fails closed.
func TestStoreCreateWithBlobReportsIgnoredBlobInsert(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-ignored-blob")
	if _, err := db.DB.Exec("CREATE TRIGGER ignore_blob_insert BEFORE INSERT ON artifact_blobs BEGIN SELECT RAISE(IGNORE); END"); err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
		ContentHash: "ignored-blob", ContentType: "application/json", ByteSize: 1, PipelineRunID: runID, Data: []byte("x"),
	}); err == nil {
		t.Fatal("expected an ignored blob insert to fail")
	}
}

// TestStoreCreateBlobReportsIgnoredInsertWithoutExistingRow verifies an ignored
// blob insert that leaves no row is reported instead of returning a phantom ID.
func TestStoreCreateBlobReportsIgnoredInsertWithoutExistingRow(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-ignored-blob-create")
	artifactID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "ignored-blob-create", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("CREATE TRIGGER ignore_blob_insert BEFORE INSERT ON artifact_blobs BEGIN SELECT RAISE(IGNORE); END"); err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: []byte("x")}); err == nil {
		t.Fatal("expected an ignored blob insert without an existing row to fail")
	}
}

// TestStoreOperationsHonorCanceledContext verifies canceled contexts propagate
// through reads and transaction-owned writes.
func TestStoreOperationsHonorCanceledContext(t *testing.T) {
	store, db := openFamilyStore(t)
	runID := createTestRun(t, db, "artifact-canceled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name string
		call func() error
	}{
		{"Create", func() error {
			_, err := store.Create(ctx, artifact.CreateInput{ContentHash: "canceled", ContentType: "application/json", ByteSize: 1})
			return err
		}},
		{"CreateWithBlob", func() error {
			_, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
				ContentHash: "canceled-atomic", ContentType: "application/json", ByteSize: 1, PipelineRunID: runID, Data: []byte("x"),
			})
			return err
		}},
		{"GetByHash", func() error { _, err := store.GetByHash(ctx, "canceled"); return err }},
		{"GetByID", func() error { _, err := store.GetByID(ctx, 1); return err }},
		{"CreateBlob", func() error {
			_, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: 1, PipelineRunID: runID, Data: []byte("x")})
			return err
		}},
		{"GetBlobByArtifactID", func() error { _, err := store.GetBlobByArtifactID(ctx, 1); return err }},
		{"Link", func() error {
			return store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: 1, Role: artifact.RunArtifactWorkspaceConfig})
		}},
		{"ListRunArtifacts", func() error { _, err := store.ListRunArtifacts(ctx, runID); return err }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("canceled context did not produce an error")
			}
		})
	}
}
