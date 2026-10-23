// run_artifact_integration_test.go exercises run-artifact role links against a
// database created by the production migration runner.

package artifact_test

import (
	"context"
	"testing"

	"analysis/backend/database/artifact"
	"analysis/backend/database/run"
	"analysis/tests/support/go/testfixture"
)

// TestStoreLinkIsRoleScopedAndImmutable verifies idempotent replay and role conflict rejection.
func TestStoreLinkIsRoleScopedAndImmutable(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-link")
	configID, err := testfixture.Artifact(ctx, db.Conn, "config-snapshot", "application/x-something-config", 10)
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := testfixture.Artifact(ctx, db.Conn, "other-snapshot", "application/json", 10)
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

	related, err := store.ListRunArtifacts(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	var linkedID int64
	for _, item := range related {
		if item.Role == "run_role" && item.Detail == artifact.RunArtifactWorkspaceConfig {
			linkedID = item.ID
		}
	}
	if linkedID != configID {
		t.Fatalf("stored link artifact = %d, want %d", linkedID, configID)
	}
}

// TestStoreListRunArtifactEvidenceFiltersPagesAndFocuses verifies the viewer
// artifact page aggregates run roles, step links, and blob availability in
// cursor and page modes.
func TestStoreListRunArtifactEvidenceFiltersPagesAndFocuses(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-evidence")

	configID, err := store.CreateWithBlob(ctx, artifact.CreateWithBlobInput{ContentHash: "config-evidence", ContentType: "application/json", ByteSize: int64(len([]byte(`{"ok":true}`))), PipelineRunID: runID, Data: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	unlinkedID, err := testfixture.Artifact(ctx, db.Conn, "unlinked-evidence", "text/plain", 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: configID, Role: artifact.RunArtifactWorkspaceConfig}); err != nil {
		t.Fatal(err)
	}
	stepID, err := db.Run.CreateStep(ctx, runID, "preflight")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: configID}); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.LinkStepOutputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: unlinkedID}); err != nil {
		t.Fatal(err)
	}

	page, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Order: "ASC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.HasMore {
		t.Fatalf("cursor artifact page = %d items hasMore=%v, want 2", len(page.Items), page.HasMore)
	}
	if page.Items[0].ID != configID || !page.Items[0].HasBlob {
		t.Fatalf("config artifact = %+v, want a stored blob", page.Items[0])
	}
	if page.Items[0].ArtifactRoles != "workspace_config" || page.Items[0].ConsumedBySteps != "preflight" {
		t.Fatalf("config relationships = %+v", page.Items[0])
	}
	if page.Items[1].ProducedBySteps != "preflight" {
		t.Fatalf("unlinked relationships = %+v", page.Items[1])
	}

	rolePage, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Role: "run_role", Order: "ASC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(rolePage.Items) != 1 || rolePage.Items[0].ID != configID {
		t.Fatalf("role-filtered artifact page = %+v", rolePage.Items)
	}

	focusPage, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, CursorID: 0, FocusID: configID, Order: "ASC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(focusPage.Items) != 2 || focusPage.Items[0].ID != configID {
		t.Fatalf("focused artifact page = %+v", focusPage.Items)
	}

	counted, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Order: "ASC", Page: 1, PerPage: 1, PageMode: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if counted.Total != 2 || len(counted.Items) != 1 || !counted.HasMore {
		t.Fatalf("page-mode artifact page = %d items total=%d hasMore=%v", len(counted.Items), counted.Total, counted.HasMore)
	}
	searched, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Query: "workspace_config", Order: "ASC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(searched.Items) != 1 || searched.Items[0].ID != configID {
		t.Fatalf("searched artifact page = %+v", searched.Items)
	}
	empty, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID + 1, Order: "ASC", Page: 5, PerPage: 5, PageMode: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 || len(empty.Items) != 0 || empty.Page != 1 || empty.HasMore {
		t.Fatalf("empty artifact page = %+v", empty)
	}
	if _, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Order: "SIDEWAYS", Limit: 25}); err == nil {
		t.Fatal("expected an invalid order error")
	}
}
