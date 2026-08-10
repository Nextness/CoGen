//go:build integration

package database

import (
	"context"
	"strings"
	"testing"
)

// TestReviewCopyOnWriteLineage verifies context inheritance, immutable heads, note history, anchors, audit, and purge protection.
func TestReviewCopyOnWriteLineage(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	a1Run, a2Run, _, a1Revision, a2Revision := createReviewLineageFixture(t, db)
	a1, _, err := db.Reviews.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	reason := "Strong match"
	a1State, changed, err := db.Reviews.AppendWorkReview(ctx, a1.ID, a1Revision, nil, "approved", nil, &reason)
	if err != nil || !changed || a1State.Version == nil {
		t.Fatalf("append A1 review: state=%+v changed=%v err=%v", a1State, changed, err)
	}
	note, err := db.Reviews.CreateNote(ctx, a1.ID, a1Revision, "See [[article:10.1000/missing|future article]].")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	anchor, err := db.Reviews.CreateAnchor(ctx, a1.ID, a1Revision, "methods-1", hash, 1, "Methods", []AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}

	a2, _, err := db.Reviews.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	inherited, err := db.Reviews.GetWorkReview(ctx, a2.ID, a2Revision)
	if err != nil || inherited.Version == nil || inherited.Version.ID != a1State.Version.ID || inherited.InheritedFromContextID == nil {
		t.Fatalf("inherited review = %+v, err=%v", inherited, err)
	}
	a2State, changed, err := db.Reviews.AppendWorkReview(ctx, a2.ID, a2Revision, &inherited.Version.ID, "not_approved", []string{"unrelated"}, nil)
	if err != nil || !changed || a2State.Version.ParentVersionID == nil || *a2State.Version.ParentVersionID != a1State.Version.ID {
		t.Fatalf("append A2 review: %+v changed=%v err=%v", a2State, changed, err)
	}
	_, _, err = db.Reviews.AppendWorkReview(ctx, a2.ID, a2Revision, &inherited.Version.ID, "removed", []string{"duplicate"}, nil)
	if !IsReviewConflict(err) {
		t.Fatalf("stale save error = %v", err)
	}
	_, _, err = db.Reviews.AppendWorkReview(ctx, a1.ID, a1Revision, &a1State.Version.ID, "removed", []string{"duplicate"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a2AfterParentEdit, err := db.Reviews.GetWorkReview(ctx, a2.ID, a2Revision)
	if err != nil || a2AfterParentEdit.Version.ID != a2State.Version.ID {
		t.Fatalf("A2 moved after parent edit: %+v err=%v", a2AfterParentEdit, err)
	}

	inheritedNotes, err := db.Reviews.ListNotes(ctx, a2.ID, a2Revision, 0, 20, false)
	if err != nil || len(inheritedNotes) != 1 || inheritedNotes[0].Version.ID != note.Version.ID {
		t.Fatalf("inherited notes = %+v err=%v", inheritedNotes, err)
	}
	edited, changed, err := db.Reviews.AppendNoteVersion(ctx, a2.ID, note.ID, note.Version.ID, "active", "Edited [[anchor:methods-1]].")
	if err != nil || !changed {
		t.Fatalf("edit inherited note: %+v changed=%v err=%v", edited, changed, err)
	}
	deleted, changed, err := db.Reviews.AppendNoteVersion(ctx, a2.ID, note.ID, edited.Version.ID, "deleted", "")
	if err != nil || !changed || deleted.Version.Body != nil {
		t.Fatalf("delete note: %+v changed=%v err=%v", deleted, changed, err)
	}
	restored, changed, err := db.Reviews.AppendNoteVersion(ctx, a2.ID, note.ID, deleted.Version.ID, "active", "Restored")
	if err != nil || !changed || restored.Version.ParentVersionID == nil || *restored.Version.ParentVersionID != deleted.Version.ID {
		t.Fatalf("restore note: %+v changed=%v err=%v", restored, changed, err)
	}
	versions, err := db.Reviews.ListNoteVersions(ctx, a2.ID, note.ID, 0, 20)
	if err != nil || len(versions) != 4 || versions[len(versions)-1].Body == nil || *versions[len(versions)-1].Body != *note.Version.Body {
		t.Fatalf("note history = %+v err=%v", versions, err)
	}
	a1Note, err := db.Reviews.GetNote(ctx, a1.ID, note.ID)
	if err != nil || a1Note.Version.ID != note.Version.ID {
		t.Fatalf("A1 note changed: %+v err=%v", a1Note, err)
	}

	anchors, err := db.Reviews.ListAnchors(ctx, a2.ID, a2Revision, "", 20)
	if err != nil || len(anchors) != 1 || anchors[0].Version.ID != anchor.Version.ID {
		t.Fatalf("inherited anchors = %+v err=%v", anchors, err)
	}
	deletedAnchor, changed, err := db.Reviews.AppendAnchorVersion(ctx, a2.ID, anchor.ID, anchor.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil || !changed || deletedAnchor.Version.State != "deleted" {
		t.Fatalf("delete anchor: %+v changed=%v err=%v", deletedAnchor, changed, err)
	}

	eligibility, err := db.PipelineRuns.CheckPurgeEligibility(a1Run)
	if err != nil || eligibility.Eligible || eligibility.OwnedReviewContextCount != 1 || eligibility.DependentReviewContextCount != 1 {
		t.Fatalf("purge eligibility = %+v err=%v", eligibility, err)
	}
	var leaked int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM audit_events
		WHERE action LIKE 'review_%' AND (metadata_json LIKE '%Strong match%' OR metadata_json LIKE '%Methods%' OR metadata_json LIKE '%Edited%')`).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatal("review audit metadata leaked note, reason, or selected text")
	}
}

// TestReviewValidationAndNoOp verifies invalid vocabulary, geometry, syntax, and identical saves.
func TestReviewValidationAndNoOp(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	runID, _, _, revisionID, _ := createReviewLineageFixture(t, db)
	contextRecord, _, err := db.Reviews.CreateContext(ctx, runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, changed, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, revisionID, nil, "not_evaluated", nil, nil)
	if err != nil || changed || state.Version != nil {
		t.Fatalf("default no-op = %+v changed=%v err=%v", state, changed, err)
	}
	if _, _, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, revisionID, nil, "approved", []string{"duplicate"}, nil); err == nil {
		t.Fatal("approved review accepted sub-status")
	}
	if _, err := db.Reviews.CreateNote(ctx, contextRecord.ID, revisionID, "[[ext:javascript:alert(1)]]"); err == nil {
		t.Fatal("unsafe note link was accepted")
	}
	if _, err := db.Reviews.CreateAnchor(ctx, contextRecord.ID, revisionID, "bad id", strings.Repeat("a", 64), 1, "text", []AnchorRectangle{{Width: 1, Height: 1}}); err == nil {
		t.Fatal("invalid anchor ID was accepted")
	}
}

// TestReviewParentSelection verifies same-plan preference, same-search fallback, explicit cross-search parents, and later-parent rejection.
func TestReviewParentSelection(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	a1Run, a2Run, _, _, _ := createReviewLineageFixture(t, db)
	if proposed, err := db.Reviews.ProposeParent(ctx, a1Run); err != nil || proposed != nil {
		t.Fatalf("first run proposed parent=%+v err=%v", proposed, err)
	}
	a1, _, err := db.Reviews.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	proposed, err := db.Reviews.ProposeParent(ctx, a2Run)
	if err != nil || proposed == nil || proposed.ContextID != a1.ID {
		t.Fatalf("same-plan proposed parent=%+v err=%v", proposed, err)
	}
	a2, _, err := db.Reviews.CreateContext(ctx, a2Run, &a1.ID)
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
	proposed, err = db.Reviews.ProposeParent(ctx, sameSearchRun)
	if err != nil || proposed == nil || proposed.ContextID != a2.ID {
		t.Fatalf("same-search proposed parent=%+v err=%v", proposed, err)
	}
	sameSearch, _, err := db.Reviews.CreateContext(ctx, sameSearchRun, &a2.ID)
	if err != nil {
		t.Fatal(err)
	}
	crossSearch := exec("INSERT INTO searches (search_id) VALUES ('cross-search')")
	crossRevision := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'cross-config', 'cross-manifest')", crossSearch)
	crossPlan := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'cross-fingerprint', 'cross-manifest', 'cross-input')", crossRevision)
	earlierRun := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-01 12:00:00', '2026-01-01 12:01:00', 'completed', ?, 1)", crossPlan)
	laterRun := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-04 00:00:00', '2026-01-04 00:01:00', 'completed', ?, 2)", crossPlan)
	if proposed, err := db.Reviews.ProposeParent(ctx, laterRun); err != nil || proposed != nil {
		t.Fatalf("cross-search implicit parent=%+v err=%v", proposed, err)
	}
	later, _, err := db.Reviews.CreateContext(ctx, laterRun, &sameSearch.ID)
	if err != nil || later.ParentContextID == nil || *later.ParentContextID != sameSearch.ID {
		t.Fatalf("explicit cross-search parent=%+v err=%v", later, err)
	}
	if _, _, err := db.Reviews.CreateContext(ctx, earlierRun, &later.ID); err == nil || !strings.Contains(err.Error(), "earlier run") {
		t.Fatalf("later parent error=%v", err)
	}
}

// createReviewLineageFixture creates completed A1 and A2 runs with one overlapping stable work.
func createReviewLineageFixture(t *testing.T, db *Database) (int64, int64, int64, int64, int64) {
	t.Helper()
	exec := func(query string, args ...any) int64 {
		result, err := db.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture query %q: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	searchID := exec("INSERT INTO searches (search_id) VALUES ('review-search')")
	revisionID := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'config', 'manifest')", searchID)
	planID := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'fingerprint', 'manifest', 'input')", revisionID)
	a1Run := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-01 00:00:00', '2026-01-01 00:01:00', 'completed', ?, 1)", planID)
	a2Run := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-02 00:00:00', '2026-01-02 00:01:00', 'completed', ?, 2)", planID)
	if err := db.PipelineRunReviewers.Insert(a1Run, "Researcher", "researcher@example.test"); err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRunReviewers.Insert(a2Run, "", ""); err != nil {
		t.Fatal(err)
	}
	workID := exec("INSERT INTO works (doi) VALUES ('10.1000/review')")
	makeRevision := func(runID int64, hash, title string) int64 {
		return exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, producer_stage)
			VALUES (?, ?, ?, ?, 'normalize')`, workID, runID, hash, title)
	}
	return a1Run, a2Run, workID, makeRevision(a1Run, "a1", "A1 article"), makeRevision(a2Run, "a2", "A2 article")
}
