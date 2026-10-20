// context_integration_test.go verifies review context initialization, parent
// selection, lineage validation, and inherited work-head freezing.
//go:build integration

package review_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"analysis/database/review"
	"analysis/database/run"
	"analysis/database/search"
)

// TestReviewParentSelection verifies same-plan preference, same-search fallback, explicit cross-search parents, and later-parent rejection.
func TestReviewParentSelection(t *testing.T) {
	db, a1Run, a2Run, _, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	if missing, err := db.Review.GetContextByRun(ctx, 999); err != nil || missing != nil {
		t.Fatalf("missing context = %+v err=%v, want nil without error", missing, err)
	}
	if proposed, err := db.Review.ProposeParent(ctx, a1Run); err != nil || proposed != nil {
		t.Fatalf("first run proposed parent=%+v err=%v", proposed, err)
	}
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	proposed, err := db.Review.ProposeParent(ctx, a2Run)
	if err != nil || proposed == nil || proposed.ContextID != a1.ID {
		t.Fatalf("same-plan proposed parent=%+v err=%v", proposed, err)
	}
	a2, _, err := db.Review.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	stableSearchID, err := db.Search.FindOrCreateSearch(ctx, "review-search")
	if err != nil {
		t.Fatal(err)
	}
	secondRevision, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: stableSearchID, RevisionLabel: "r2", ConfigArtifactHash: "config-r2", ResolvedManifestHash: "manifest-r2",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: secondRevision, ExecutionFingerprint: "fingerprint-r2", ResolvedManifestHash: "manifest-r2", InputManifestHash: "input-r2",
	})
	if err != nil {
		t.Fatal(err)
	}
	sameSearchRun, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: secondPlan, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, sameSearchRun, "completed", ""); err != nil {
		t.Fatal(err)
	}
	proposed, err = db.Review.ProposeParent(ctx, sameSearchRun)
	if err != nil || proposed == nil || proposed.ContextID != a2.ID {
		t.Fatalf("same-search proposed parent=%+v err=%v", proposed, err)
	}
	sameSearch, _, err := db.Review.CreateContext(ctx, sameSearchRun, &a2.ID)
	if err != nil {
		t.Fatal(err)
	}
	crossSearch, err := db.Search.FindOrCreateSearch(ctx, "cross-search")
	if err != nil {
		t.Fatal(err)
	}
	crossRevision, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: crossSearch, RevisionLabel: "r1", ConfigArtifactHash: "cross-config", ResolvedManifestHash: "cross-manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	crossPlan, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: crossRevision, ExecutionFingerprint: "cross-fingerprint", ResolvedManifestHash: "cross-manifest", InputManifestHash: "cross-input",
	})
	if err != nil {
		t.Fatal(err)
	}
	earlierRun, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: crossPlan, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, earlierRun, "completed", ""); err != nil {
		t.Fatal(err)
	}
	laterRun, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: crossPlan, Step: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, laterRun, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if proposed, err := db.Review.ProposeParent(ctx, laterRun); err != nil || proposed != nil {
		t.Fatalf("cross-search implicit parent=%+v err=%v", proposed, err)
	}
	later, _, err := db.Review.CreateContext(ctx, laterRun, &sameSearch.ID)
	if err != nil || later.ParentContextID == nil || *later.ParentContextID != sameSearch.ID {
		t.Fatalf("explicit cross-search parent=%+v err=%v", later, err)
	}
	if _, _, err := db.Review.CreateContext(ctx, earlierRun, &later.ID); err == nil || !strings.Contains(err.Error(), "earlier run") {
		t.Fatalf("later parent error=%v", err)
	}
}

// TestReviewContextCopyOnWriteLineage verifies parent conflicts, frozen work
// heads, and immutable parent selection.
func TestReviewContextCopyOnWriteLineage(t *testing.T) {
	db, a1Run, a2Run, a1Revision, a2Revision := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, created, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil || !created {
		t.Fatalf("create A1 context: context=%+v created=%v err=%v", a1, created, err)
	}
	a2, created, err := db.Review.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil || !created || a2.ParentContextID == nil || *a2.ParentContextID != a1.ID {
		t.Fatalf("create A2 context: context=%+v created=%v err=%v", a2, created, err)
	}
	if _, _, err := db.Review.CreateContext(ctx, a2Run, nil); err == nil {
		t.Fatal("context initialization accepted a different parent")
	} else {
		var conflict *review.ContextParentConflictError
		if !errors.As(err, &conflict) || conflict.Existing == nil || *conflict.Existing != a1.ID || conflict.Requested != nil {
			t.Fatalf("parent conflict=%+v err=%v", conflict, err)
		}
	}
	if head := reviewCount(t, db, "SELECT COUNT(*) FROM review_context_work_heads WHERE review_context_id=? AND work_revision_id=?", a2.ID, a2Revision); head != 1 {
		t.Fatalf("A2 work head rows=%d, want revision %d", head, a2Revision)
	}
	if head := reviewCount(t, db, "SELECT COUNT(*) FROM review_context_work_heads WHERE review_context_id=? AND work_revision_id=?", a1.ID, a1Revision); head != 1 {
		t.Fatalf("A1 work head rows=%d, want revision %d", head, a1Revision)
	}
}
