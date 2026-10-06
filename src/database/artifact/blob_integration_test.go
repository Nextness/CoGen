// blob_integration_test.go exercises artifact-blob deduplication and byte
// integrity against a database created by the production migration runner.
//go:build integration

package artifact_test

import (
	"context"
	"testing"

	"analysis/database/artifact"
)

// TestStoreCreateBlobDeduplicatesAndRejectsConflictingBytes verifies one artifact keeps exactly one byte payload.
func TestStoreCreateBlobDeduplicatesAndRejectsConflictingBytes(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-blob")
	artifactID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "blob-hash", ContentType: "application/json", ByteSize: 3})
	if err != nil {
		t.Fatal(err)
	}

	blobID, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: []byte("one")})
	if err != nil {
		t.Fatalf("create blob: %v", err)
	}
	replayedID, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: []byte("one")})
	if err != nil {
		t.Fatalf("replay blob: %v", err)
	}
	if replayedID != blobID {
		t.Fatalf("replayed blob id = %d, want %d", replayedID, blobID)
	}
	if _, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: []byte("two")}); err == nil {
		t.Fatal("expected conflicting blob bytes to fail")
	}

	blob, err := store.GetBlobByArtifactID(ctx, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	if blob == nil || blob.ID != blobID || string(blob.Data) != "one" || blob.PipelineRunID != runID {
		t.Fatalf("stored blob = %+v", blob)
	}
	missing, err := store.GetBlobByArtifactID(ctx, 99999)
	if err != nil || missing != nil {
		t.Fatalf("missing blob = %+v err=%v, want nil without error", missing, err)
	}
}
