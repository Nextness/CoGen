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
)

// TestReviewParentSelection verifies same-plan preference, same-search fallback, explicit cross-search parents, and later-parent rejection.
func TestReviewParentSelection(t *testing.T) {
	db, a1Run, a2Run, _, _ := newReviewLineageFixture(t)
	ctx := context.Background()
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
	exec := func(query string, args ...any) int64 {
		result, err := db.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture query %q: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	var stableSearchID int64
	if err := db.DB.QueryRow(`SELECT sr.search_id FROM pipeline_runs run JOIN execution_plans plan ON plan.id=run.execution_plan_id
		JOIN search_revisions sr ON sr.id=plan.search_revision_id WHERE run.id=?`, a1Run).Scan(&stableSearchID); err != nil {
		t.Fatal(err)
	}
	secondRevision := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r2', 'config-r2', 'manifest-r2')", stableSearchID)
	secondPlan := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'fingerprint-r2', 'manifest-r2', 'input-r2')", secondRevision)
	sameSearchRun := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-03 00:00:00', '2026-01-03 00:01:00', 'completed', ?, 1)", secondPlan)
	proposed, err = db.Review.ProposeParent(ctx, sameSearchRun)
	if err != nil || proposed == nil || proposed.ContextID != a2.ID {
		t.Fatalf("same-search proposed parent=%+v err=%v", proposed, err)
	}
	sameSearch, _, err := db.Review.CreateContext(ctx, sameSearchRun, &a2.ID)
	if err != nil {
		t.Fatal(err)
	}
	crossSearch := exec("INSERT INTO searches (search_id) VALUES ('cross-search')")
	crossRevision := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'cross-config', 'cross-manifest')", crossSearch)
	crossPlan := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'cross-fingerprint', 'cross-manifest', 'cross-input')", crossRevision)
	earlierRun := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-01 12:00:00', '2026-01-01 12:01:00', 'completed', ?, 1)", crossPlan)
	laterRun := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-04 00:00:00', '2026-01-04 00:01:00', 'completed', ?, 2)", crossPlan)
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
// heads, and review-context purge protection.
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
	eligibility, err := db.Run.CheckPurgeEligibility(ctx, a1Run)
	if err != nil || eligibility.Eligible || eligibility.OwnedReviewContextCount != 1 || eligibility.DependentReviewContextCount != 1 {
		t.Fatalf("purge eligibility = %+v err=%v", eligibility, err)
	}
}
