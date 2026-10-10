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

// TestStoreListRunArtifactEvidenceFiltersPagesAndFocuses verifies the viewer
// artifact page aggregates run roles, step links, and blob availability in
// cursor and page modes.
func TestStoreListRunArtifactEvidenceFiltersPagesAndFocuses(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-evidence")

	configID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "config-evidence", ContentType: "application/json", ByteSize: 12})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateBlob(ctx, artifact.CreateBlobInput{ArtifactID: configID, PipelineRunID: runID, Data: []byte(`{"ok":true}`)}); err != nil {
		t.Fatal(err)
	}
	unlinkedID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "unlinked-evidence", ContentType: "text/plain", ByteSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: configID, Role: artifact.RunArtifactWorkspaceConfig}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id) VALUES (?, 'preflight', 'completed', ?, ?)", runID, configID, unlinkedID); err != nil {
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
