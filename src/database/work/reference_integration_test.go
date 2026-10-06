// reference_integration_test.go exercises the cited-reference store against a
// database created by the production migration runner.
//go:build integration

package work_test

import (
	"context"
	"testing"

	"analysis/database/work"
)

// TestStoreCreateReferenceNormalizesAndResolvesDOI verifies DOI normalization,
// best-effort resolution to a known work, and source ordering.
func TestStoreCreateReferenceNormalizesAndResolvesDOI(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/reference-target")
	if err != nil {
		t.Fatal(err)
	}
	runID := createTestRun(t, db, "reference-target")
	revisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageParse, Title: "target",
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := store.CreateReference(ctx, &work.Reference{
		WorkRevisionID: revisionID, MentionOrder: 2, RawReference: "second", DOI: "https://doi.org/10.1000/reference-target",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreateReference(ctx, &work.Reference{
		WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "first",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first == 0 || second == 0 || first == second {
		t.Fatalf("reference ids = %d, %d", first, second)
	}

	resolved, err := store.GetReferenceByID(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ResolvedWorkID != workID || resolved.DOI != "10.1000/reference-target" {
		t.Fatalf("resolved reference = %+v", resolved)
	}
	external, err := store.GetReferenceByID(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if external == nil || external.ResolvedWorkID != 0 || external.DOI != "" {
		t.Fatalf("external reference = %+v, want unresolved empty DOI", external)
	}

	ordered, err := store.ListReferencesByRevisionID(ctx, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 2 || ordered[0].ID != first || ordered[1].ID != second {
		t.Fatalf("references by revision = %+v, want mention order", ordered)
	}
	if absent, err := store.ListReferencesByRevisionID(ctx, 99999); err != nil || absent == nil || len(absent) != 0 {
		t.Fatalf("absent reference list = %+v err=%v, want empty non-nil without error", absent, err)
	}
	citing, err := store.ListReferencesByResolvedWorkID(ctx, workID)
	if err != nil {
		t.Fatal(err)
	}
	if len(citing) != 1 || citing[0].ID != second {
		t.Fatalf("references by resolved work = %+v", citing)
	}
	if missing, err := store.GetReferenceByID(ctx, 99999); err != nil || missing != nil {
		t.Fatalf("missing reference = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreCreateReferenceValidatesInput verifies required identity and order
// are enforced before any write.
func TestStoreCreateReferenceValidatesInput(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/reference-validation")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: createTestRun(t, db, "reference-validation"), ProducerStage: work.ProducerStageParse, Title: "validation",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.CreateReference(ctx, nil); err == nil {
		t.Fatal("expected a nil reference to fail")
	}
	if _, err := store.CreateReference(ctx, &work.Reference{MentionOrder: 1}); err == nil {
		t.Fatal("expected a missing work revision id to fail")
	}
	if _, err := store.CreateReference(ctx, &work.Reference{WorkRevisionID: revisionID, MentionOrder: 0}); err == nil {
		t.Fatal("expected a non-positive mention order to fail")
	}
}

// TestStoreReferencesAreAppendOnly verifies the database trigger rejects
// mutation of an immutable reference snapshot.
func TestStoreReferencesAreAppendOnly(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	workID, err := store.CreateWorkByDOI(ctx, "10.1000/reference-append-only")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: createTestRun(t, db, "reference-append-only"), ProducerStage: work.ProducerStageParse, Title: "append-only",
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateReference(ctx, &work.Reference{WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "immutable"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE reference_mentions SET raw_reference='changed' WHERE id=?", id); err == nil {
		t.Fatal("expected an update to be rejected by the append-only trigger")
	}
	if _, err := db.DB.Exec("DELETE FROM reference_mentions WHERE id=?", id); err == nil {
		t.Fatal("expected a delete to be rejected by the append-only trigger")
	}
}
