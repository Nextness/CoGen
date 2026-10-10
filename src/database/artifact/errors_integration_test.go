// errors_integration_test.go verifies the artifact family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package artifact_test

import (
	"context"
	"testing"

	"analysis/database/artifact"
)

// TestStoreOperationsPropagateConnectionErrors verifies every public store
// operation reports a closed connection as an error rather than a value.
func TestStoreOperationsPropagateConnectionErrors(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"Create", func() error {
			_, err := store.Create(ctx, artifact.CreateInput{ContentHash: "closed", ContentType: "application/json", ByteSize: 1})
			return err
		}},
		{"CreateWithBlob", func() error {
			_, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{
				ContentHash: "closed", ContentType: "application/json", ByteSize: 1, PipelineRunID: 1, Data: []byte("x"),
			})
			return err
		}},
		{"GetByHash", func() error { _, err := store.GetByHash(ctx, "closed"); return err }},
		{"GetByID", func() error { _, err := store.GetByID(ctx, 1); return err }},
		{"CreateBlob", func() error {
			_, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: 1, PipelineRunID: 1, Data: []byte("x")})
			return err
		}},
		{"GetBlobByArtifactID", func() error { _, err := store.GetBlobByArtifactID(ctx, 1); return err }},
		{"GetContent", func() error { _, err := store.GetContent(ctx, 1); return err }},
		{"GetPreview", func() error { _, err := store.GetPreview(ctx, 1, 5); return err }},
		{"Link", func() error {
			return store.Link(ctx, artifact.LinkInput{PipelineRunID: 1, ArtifactID: 1, Role: artifact.RunArtifactWorkspaceConfig})
		}},
		{"ListRunArtifacts", func() error { _, err := store.ListRunArtifacts(ctx, 1); return err }},
		{"ListRunArtifactEvidence", func() error {
			_, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: 1, Order: "ASC", Limit: 10})
			return err
		}},
		{"ListRunArtifactEvidencePageMode", func() error {
			_, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: 1, Order: "ASC", Page: 1, PerPage: 10, PageMode: true, Limit: 10})
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed connection did not produce an error")
			}
		})
	}
}
