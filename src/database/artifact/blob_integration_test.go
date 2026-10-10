// blob_integration_test.go exercises artifact-blob deduplication and byte
// integrity against a database created by the production migration runner.
//go:build integration

package artifact_test

import (
	"context"
	"errors"
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

// TestStoreGetContentReturnsBytesRoleAndMediaType verifies the download read
// preserves stored metadata, the first run role, and the missing-blob contract.
func TestStoreGetContentReturnsBytesRoleAndMediaType(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-content")
	artifactID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "content-hash", ContentType: "application/x-something-config", ByteSize: 14})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: []byte("workspace = {}")}); err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: artifactID, Role: artifact.RunArtifactWorkspaceConfig}); err != nil {
		t.Fatal(err)
	}

	content, err := store.GetContent(ctx, artifactID)
	if err != nil {
		t.Fatal(err)
	}
	if content == nil || content.ContentType != "application/x-something-config" ||
		content.Role != artifact.RunArtifactWorkspaceConfig || string(content.Data) != "workspace = {}" {
		t.Fatalf("artifact content = %+v", content)
	}

	noBlobID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "content-no-blob", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	missingBlob, err := store.GetContent(ctx, noBlobID)
	if !errors.Is(err, artifact.ErrNoBlob) || missingBlob != nil {
		t.Fatalf("content without blob = %+v err=%v, want ErrNoBlob", missingBlob, err)
	}
	missing, err := store.GetContent(ctx, 99999)
	if err != nil || missing != nil {
		t.Fatalf("missing content = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreGetPreviewReturnsBoundedPrefixAndMetadata verifies the inspection
// read preserves stored sizes, bounded prefixes, and the missing-blob contract.
func TestStoreGetPreviewReturnsBoundedPrefixAndMetadata(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-preview")
	artifactID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "preview-hash", ContentType: "application/json", ByteSize: 14})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: artifactID, PipelineRunID: runID, Data: []byte("workspace = {}")}); err != nil {
		t.Fatal(err)
	}

	bounded, err := store.GetPreview(ctx, artifactID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if bounded == nil || bounded.ContentType != "application/json" || bounded.ByteSize != 14 ||
		bounded.BlobSize != 14 || string(bounded.Data) != "works" {
		t.Fatalf("bounded preview = %+v", bounded)
	}
	full, err := store.GetPreview(ctx, artifactID, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if full == nil || string(full.Data) != "workspace = {}" {
		t.Fatalf("full preview = %+v", full)
	}

	noBlobID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "preview-no-blob", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	missingBlob, err := store.GetPreview(ctx, noBlobID, 5)
	if !errors.Is(err, artifact.ErrNoBlob) || missingBlob != nil {
		t.Fatalf("preview without blob = %+v err=%v, want ErrNoBlob", missingBlob, err)
	}
	missing, err := store.GetPreview(ctx, 99999, 5)
	if err != nil || missing != nil {
		t.Fatalf("missing preview = %+v err=%v, want nil without error", missing, err)
	}
}
