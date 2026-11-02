// sync_integration_test.go verifies bundle inventory reconciliation registers
// normalized works and flushes audit without creating a research attempt.

package bundle_test

import (
	"analysis/backend/bundle"
	"analysis/backend/database"
	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/work"
	"context"
	"path/filepath"
	"testing"
)

// TestSyncInventoryRegistersNormalizedWorksWithoutResearchAttempt verifies inventory synchronization stays idempotent.
func TestSyncInventoryRegistersNormalizedWorksWithoutResearchAttempt(t *testing.T) {
	ctx := context.Background()
	root, registry := setupBundle(t)
	metadataPath := filepath.Join(root, "corpus.metadata.db")
	report, err := bundle.PrepareResolved(ctx, registry, metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.OpenExisting(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	searchID, err := db.Search.FindOrCreateSearch(ctx, "sync-search")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "sync-fingerprint",
		ResolvedManifestHash: "manifest", InputManifestHash: "input",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: planID, Step: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, runID, "completed", "fixture complete"); err != nil {
		t.Fatal(err)
	}
	for _, doi := range []string{"10.1000/sync-inventory", "10.1000/sync-inventory-two"} {
		workID, err := db.Work.CreateWorkByDOI(ctx, doi)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Work.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize, Title: "Sync inventory article",
		}); err != nil {
			t.Fatal(err)
		}
	}

	registered, flushed, err := bundle.SyncInventory(ctx, db, metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if registered != 2 || flushed != 2 {
		t.Fatalf("first sync registered=%d flushed=%d, want 2 and 2", registered, flushed)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM pipeline_runs"); got != 1 {
		t.Fatalf("inventory sync created research attempts: %d", got)
	}
	if got := queryOne[int](t, report.Resolved.PDF, "SELECT COUNT(*) FROM pdf_documents WHERE status = 'not_available'"); got != 2 {
		t.Fatalf("normalized inventory rows = %d, want 2", got)
	}
	registered, flushed, err = bundle.SyncInventory(ctx, db, metadataPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if registered != 0 || flushed != 0 {
		t.Fatalf("second sync registered=%d flushed=%d, want 0 and 0", registered, flushed)
	}
	if got := queryOne[int](t, metadataPath, "SELECT COUNT(*) FROM audit_events WHERE action = 'pdf_inventory_registered' AND pipeline_run_id = ?", runID); got != 2 {
		t.Fatalf("inventory registration audit events = %d, want 2", got)
	}
}
