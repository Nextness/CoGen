// run_artifact_integration_test.go exercises run-artifact role links against a
// database created by the production migration runner.
//go:build integration

package artifact_test

import (
	"context"
	"testing"

	"analysis/database/artifact"
)

// TestStoreLinkIsRoleScopedAndImmutable verifies idempotent replay and role conflict rejection.
func TestStoreLinkIsRoleScopedAndImmutable(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-link")
	configID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "config-snapshot", ContentType: "application/x-something-config", ByteSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "other-snapshot", ContentType: "application/json", ByteSize: 10})
	if err != nil {
		t.Fatal(err)
	}

	link := artifact.LinkInput{PipelineRunID: runID, ArtifactID: configID, Role: artifact.RunArtifactWorkspaceConfig}
	if err := store.Link(ctx, link); err != nil {
		t.Fatalf("link run artifact: %v", err)
	}
	if err := store.Link(ctx, link); err != nil {
		t.Fatalf("repeat link should be idempotent: %v", err)
	}
	conflict := artifact.LinkInput{PipelineRunID: runID, ArtifactID: otherID, Role: artifact.RunArtifactWorkspaceConfig}
	if err := store.Link(ctx, conflict); err == nil {
		t.Fatal("expected a conflicting role assignment to fail")
	}

	var linkedID int64
	if err := db.DB.QueryRowContext(ctx,
		"SELECT artifact_id FROM run_artifacts WHERE pipeline_run_id=? AND artifact_role=?",
		runID, artifact.RunArtifactWorkspaceConfig,
	).Scan(&linkedID); err != nil {
		t.Fatal(err)
	}
	if linkedID != configID {
		t.Fatalf("stored link artifact = %d, want %d", linkedID, configID)
	}
}
