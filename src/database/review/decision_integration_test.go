// decision_integration_test.go verifies immutable work-review decision
// lineage, inherited head stability, optimistic concurrency, and validation.
//go:build integration

package review_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/review"
)

// TestReviewDecisionCopyOnWriteLineage verifies inherited decisions, stale conflicts, and stable parent heads.
func TestReviewDecisionCopyOnWriteLineage(t *testing.T) {
	db, a1Run, a2Run, a1Revision, a2Revision := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	reason := "Strong match"
	a1State, changed, err := db.Review.AppendWorkReview(ctx, a1.ID, a1Revision, nil, "approved", nil, &reason)
	if err != nil || !changed || a1State.Version == nil {
		t.Fatalf("append A1 review: state=%+v changed=%v err=%v", a1State, changed, err)
	}
	if a1State.Version.ReviewerDisplay != "Researcher" || strings.Contains(a1State.Version.ReviewerDisplay, "@") {
		t.Fatalf("reviewer display=%q, want username without email", a1State.Version.ReviewerDisplay)
	}
	a2, _, err := db.Review.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	inherited, err := db.Review.GetWorkReview(ctx, a2.ID, a2Revision)
	if err != nil || inherited.Version == nil || inherited.Version.ID != a1State.Version.ID || inherited.InheritedFromContextID == nil {
		t.Fatalf("inherited review = %+v, err=%v", inherited, err)
	}
	a2State, changed, err := db.Review.AppendWorkReview(ctx, a2.ID, a2Revision, &inherited.Version.ID, "not_approved", []string{"unrelated"}, nil)
	if err != nil || !changed || a2State.Version.ParentVersionID == nil || *a2State.Version.ParentVersionID != a1State.Version.ID {
		t.Fatalf("append A2 review: %+v changed=%v err=%v", a2State, changed, err)
	}
	if _, _, err := db.Review.AppendWorkReview(ctx, a2.ID, a2Revision, &inherited.Version.ID, "removed", []string{"duplicate"}, nil); !review.IsConflict(err) {
		t.Fatalf("stale save error = %v", err)
	}
	if _, _, err := db.Review.AppendWorkReview(ctx, a1.ID, a1Revision, &a1State.Version.ID, "removed", []string{"duplicate"}, nil); err != nil {
		t.Fatal(err)
	}
	a2AfterParentEdit, err := db.Review.GetWorkReview(ctx, a2.ID, a2Revision)
	if err != nil || a2AfterParentEdit.Version.ID != a2State.Version.ID {
		t.Fatalf("A2 moved after parent edit: %+v err=%v", a2AfterParentEdit, err)
	}
}

// TestReviewDecisionValidationAndNoOp verifies default no-op saves and vocabulary compatibility.
func TestReviewDecisionValidationAndNoOp(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, changed, err := db.Review.AppendWorkReview(ctx, a1.ID, a1Revision, nil, "not_evaluated", nil, nil)
	if err != nil || changed || state.Version != nil {
		t.Fatalf("default no-op = %+v changed=%v err=%v", state, changed, err)
	}
	if _, _, err := db.Review.AppendWorkReview(ctx, a1.ID, a1Revision, nil, "approved", []string{"duplicate"}, nil); err == nil {
		t.Fatal("approved review accepted sub-status")
	}
}
