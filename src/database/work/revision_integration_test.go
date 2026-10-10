// revision_integration_test.go exercises the immutable work-revision store
// against a database created by the production migration runner.
//go:build integration

package work_test

import (
	"context"
	"testing"

	"analysis/database/work"
)

// TestStoreCreateRevisionPreservesHashDefaultsAndNullableFields verifies the
// payload hash, schema default, nullable round trip, and identity reads.
func TestStoreCreateRevisionPreservesHashDefaultsAndNullableFields(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/revision-fields")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "revision-fields")

	revision := &work.Revision{
		WorkID:        workID,
		PipelineRunID: runID,
		ProducerStage: work.ProducerStageParse,
		Title:         "Test Revision",
		Year:          2023,
		Journal:       "Test Journal",
	}
	id, err := store.CreateRevision(ctx, revision)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero revision id")
	}
	if revision.PayloadHash == "" {
		t.Fatal("expected the payload hash to be set on the created revision")
	}
	if revision.FieldSchemaVersion != "1" {
		t.Fatalf("default field schema version = %q, want %q", revision.FieldSchemaVersion, "1")
	}

	got, err := store.GetRevisionByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("revision not found by id")
	}
	if got.WorkID != workID || got.PipelineRunID != runID || got.ProducerStage != work.ProducerStageParse {
		t.Fatalf("revision identity = %+v", got)
	}
	if got.Title != "Test Revision" || got.Year != 2023 || got.Journal != "Test Journal" {
		t.Fatalf("revision core fields = %+v", got)
	}
	if got.PayloadHash != revision.PayloadHash || got.FieldSchemaVersion != "1" {
		t.Fatalf("revision hash/schema = %+v", got)
	}
	if got.Abstract != "" || got.Publisher != "" || got.Source != "" || got.Keywords != "" ||
		got.KeywordsPlus != "" || got.ExtensionData != "" || got.CitationCount != 0 || got.ReferenceCount != 0 {
		t.Fatalf("nullable fields = %+v, want zero values", got)
	}
	if missing, err := store.GetRevisionByID(ctx, 99999); err != nil || missing != nil {
		t.Fatalf("missing revision = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreRevisionsAreImmutableAndOrdered verifies revisions accumulate in
// chronological order for a work and a run.
func TestStoreRevisionsAreImmutableAndOrdered(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/revision-order")
	if err != nil {
		t.Fatal(err)
	}
	firstRun := createTestRun(t, db, "revision-order-first")
	secondRun := createTestRun(t, db, "revision-order-second")

	firstID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: firstRun, ProducerStage: work.ProducerStageParse, Title: "first",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: secondRun, ProducerStage: work.ProducerStageEnrich, Title: "second",
	})
	if err != nil {
		t.Fatal(err)
	}

	byWork, err := store.ListRevisionsByWorkID(ctx, workID)
	if err != nil {
		t.Fatal(err)
	}
	if len(byWork) != 2 || byWork[0].ID != firstID || byWork[1].ID != secondID {
		t.Fatalf("revisions by work = %+v, want chronological order", byWork)
	}
	byRun, err := store.ListRevisionsByRunID(ctx, firstRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(byRun) != 1 || byRun[0].ID != firstID {
		t.Fatalf("revisions by run = %+v", byRun)
	}
	if absent, err := store.ListRevisionsByWorkID(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent revision list = %+v err=%v, want nil without error", absent, err)
	}
	count, err := store.CountRevisionsByWorkID(ctx, workID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("revision count = %d, want 2", count)
	}
}

// TestStoreRevisionsAreAppendOnly verifies the database trigger rejects
// mutation of an immutable revision snapshot.
func TestStoreRevisionsAreAppendOnly(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/revision-append-only")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: createTestRun(t, db, "revision-append-only"),
		ProducerStage: work.ProducerStageParse, Title: "immutable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE work_revisions SET title='changed' WHERE id=?", revisionID); err == nil {
		t.Fatal("expected an update to be rejected by the append-only trigger")
	}
	if _, err := db.DB.Exec("DELETE FROM work_revisions WHERE id=?", revisionID); err == nil {
		t.Fatal("expected a delete to be rejected by the append-only trigger")
	}
	stored, err := store.GetRevisionByID(ctx, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Title != "immutable" {
		t.Fatalf("revision changed after rejected mutation: %+v", stored)
	}
}

// TestStoreCreateRevisionValidatesProducerStage verifies the producer-stage
// vocabulary gates new immutable revisions.
func TestStoreCreateRevisionValidatesProducerStage(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/revision-stage")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "revision-stage")

	if _, err := store.CreateRevision(ctx, nil); err == nil {
		t.Fatal("expected a nil revision to fail")
	}
	if _, err := store.CreateRevision(ctx, &work.Revision{WorkID: workID, PipelineRunID: runID}); err == nil {
		t.Fatal("expected an empty producer stage to fail")
	}
	if _, err := store.CreateRevision(ctx, &work.Revision{WorkID: workID, PipelineRunID: runID, ProducerStage: "bogus"}); err == nil {
		t.Fatal("expected an unknown producer stage to fail")
	}
	if _, err := store.CreateRevision(ctx, &work.Revision{WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageLegacyUnknown}); err == nil {
		t.Fatal("expected the legacy_unknown sentinel to fail")
	}
	for _, stage := range []string{
		work.ProducerStageParse,
		work.ProducerStageDeduplicate,
		work.ProducerStageValidate,
		work.ProducerStageEnrich,
		work.ProducerStageEnrichMetadata,
		work.ProducerStageEnrichIdentity,
		work.ProducerStageNormalize,
	} {
		stageRun := createTestRun(t, db, "revision-stage-"+stage)
		if _, err := store.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: stageRun, ProducerStage: stage, Title: stage,
		}); err != nil {
			t.Fatalf("unexpected error for producer stage %q: %v", stage, err)
		}
	}
}

// TestStoreCreateRevisionHashIgnoresProvenance verifies the persisted payload
// hash covers content only, not producer stage.
func TestStoreCreateRevisionHashIgnoresProvenance(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/revision-hash")
	if err != nil {
		t.Fatal(err)
	}

	first := &work.Revision{WorkID: workID, PipelineRunID: createTestRun(t, db, "hash-first"), ProducerStage: work.ProducerStageParse, Title: "same"}
	second := &work.Revision{WorkID: workID, PipelineRunID: createTestRun(t, db, "hash-second"), ProducerStage: work.ProducerStageEnrich, Title: "same"}
	if _, err := store.CreateRevision(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRevision(ctx, second); err != nil {
		t.Fatal(err)
	}
	if first.PayloadHash != second.PayloadHash {
		t.Fatalf("payload hashes differ across provenance: %q vs %q", first.PayloadHash, second.PayloadHash)
	}
}
