// mutations_integration_test.go verifies every review mutation runs its
// request validation, current-head verification, immutable version append,
// child rows, head compare-and-swap, and audit evidence in one transaction
// that commits atomically or rolls back completely.
//go:build integration

package review_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
	"analysis/database/review"
	"analysis/database/run"

	_ "modernc.org/sqlite"
)

// reviewMutationFixture is one migrated database with two completed runs and an
// initialized context for the first run.
type reviewMutationFixture struct {
	db         *database.Database
	run1, run2 int64
	revision1  int64
	revision2  int64
	context1   int64
}

// newReviewMutationFixture builds the two-run mutation fixture through production migrations.
func newReviewMutationFixture(t *testing.T) *reviewMutationFixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "mutations.db"), reviewFilterConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	exec := func(query string, args ...any) int64 {
		t.Helper()
		result, err := db.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture query %q: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	searchID := exec("INSERT INTO searches (search_id) VALUES ('mutation-search')")
	revisionID := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'config', 'manifest')", searchID)
	planID := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'fingerprint', 'manifest', 'input')", revisionID)
	run1 := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-01 00:00:00', '2026-01-01 00:01:00', 'completed', ?, 1)", planID)
	run2 := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('review', '2026-01-02 00:00:00', '2026-01-02 00:01:00', 'completed', ?, 2)", planID)
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: run1, Username: "Researcher", Email: "researcher@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.InsertReviewer(ctx, run.ReviewerInput{RunID: run2}); err != nil {
		t.Fatal(err)
	}
	workID := exec("INSERT INTO works (doi) VALUES ('10.1000/mutation')")
	revision1 := exec("INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, producer_stage) VALUES (?, ?, 'a1', 'A1 article', 'normalize')", workID, run1)
	revision2 := exec("INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, producer_stage) VALUES (?, ?, 'a2', 'A2 article', 'normalize')", workID, run2)
	for _, runID := range []int64{run1, run2} {
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (?, ?, 'validate', 'valid')", runID, workID)
	}
	context1, created, err := db.Review.CreateContext(ctx, run1, nil)
	if err != nil || !created {
		t.Fatalf("create fixture context: context=%+v created=%v err=%v", context1, created, err)
	}
	return &reviewMutationFixture{db: db, run1: run1, run2: run2, revision1: revision1, revision2: revision2, context1: context1.ID}
}

// reviewCount returns one COUNT(*) over the fixture database.
func reviewCount(t *testing.T, db *database.Database, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.DB.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	return count
}

// reviewAuditCount returns the number of audit events with one action.
func reviewAuditCount(t *testing.T, db *database.Database, action string) int {
	t.Helper()
	return reviewCount(t, db, "SELECT COUNT(*) FROM audit_events WHERE action=?", action)
}

// TestReviewDecisionMutationCommitsAtomically verifies the decision version,
// sub-statuses, head move, and redacted audit commit as one unit.
func TestReviewDecisionMutationCommitsAtomically(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	reason := "  bounded reason  "
	state, changed, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, nil, "not_approved", []string{"unrelated", "duplicate"}, &reason)
	if err != nil || !changed || state.Version == nil {
		t.Fatalf("append decision: state=%+v changed=%v err=%v", state, changed, err)
	}
	if state.Version.Reason == nil || *state.Version.Reason != "bounded reason" || len(state.Version.Substatuses) != 2 || state.Version.Substatuses[0] != "duplicate" {
		t.Fatalf("decision version=%+v", state.Version)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_work_heads WHERE review_context_id=? AND review_version_id=?", fixture.context1, state.Version.ID); head != 1 {
		t.Fatalf("decision head rows=%d, want 1", head)
	}
	if substatuses := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM work_review_version_substatuses WHERE review_version_id=?", state.Version.ID); substatuses != 2 {
		t.Fatalf("decision sub-status rows=%d, want 2", substatuses)
	}
	if audits := reviewAuditCount(t, fixture.db, "work_review_version_created"); audits != 1 {
		t.Fatalf("decision audit rows=%d, want 1", audits)
	}
	leaked := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM audit_events WHERE action='work_review_version_created' AND metadata_json LIKE '%bounded reason%'")
	if leaked != 0 {
		t.Fatal("decision audit metadata leaked the reason text")
	}
}

// TestReviewDecisionMutationRollsBackOnChildFailure verifies a failed
// sub-status insert removes the version, head move, and audit evidence.
func TestReviewDecisionMutationRollsBackOnChildFailure(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	if _, err := fixture.db.DB.Exec(`CREATE TRIGGER mutation_fail_substatus BEFORE INSERT ON work_review_version_substatuses
		BEGIN SELECT RAISE(ABORT, 'injected substatus failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, nil, "not_approved", []string{"duplicate"}, nil); err == nil || !strings.Contains(err.Error(), "injected substatus failure") {
		t.Fatalf("decision error=%v, want injected substatus failure", err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM work_review_versions"); versions != 0 {
		t.Fatalf("failed decision left %d versions", versions)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_work_heads WHERE review_version_id IS NOT NULL"); head != 0 {
		t.Fatalf("failed decision moved %d heads", head)
	}
	if audits := reviewAuditCount(t, fixture.db, "work_review_version_created"); audits != 0 {
		t.Fatalf("failed decision left %d audit rows", audits)
	}
}

// TestReviewNoteMutationCommitsAtomically verifies note creation, edits,
// tombstones, links, head moves, and audit commit as one unit.
func TestReviewNoteMutationCommitsAtomically(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Alpha [[ext:https://example.test|source]]")
	if err != nil {
		t.Fatal(err)
	}
	if note.Version.State != "active" || note.Version.Body == nil || *note.Version.Body != "Alpha [[ext:https://example.test|source]]" {
		t.Fatalf("created note=%+v", note)
	}
	if links := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_note_links WHERE note_version_id=?", note.Version.ID); links != 1 {
		t.Fatalf("created note links=%d, want 1", links)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_note_heads WHERE review_context_id=? AND note_version_id=?", fixture.context1, note.Version.ID); head != 1 {
		t.Fatalf("created note head rows=%d, want 1", head)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_note_created"); audits != 1 {
		t.Fatalf("created note audit rows=%d, want 1", audits)
	}

	edited, changed, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, note.Version.ID, "active", "Beta [[ext:https://example.test|source]]")
	if err != nil || !changed {
		t.Fatalf("edit note: note=%+v changed=%v err=%v", edited, changed, err)
	}
	if edited.Version.ParentVersionID == nil || *edited.Version.ParentVersionID != note.Version.ID {
		t.Fatalf("edited note parent=%v, want %d", edited.Version.ParentVersionID, note.Version.ID)
	}
	if links := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_note_links WHERE note_version_id=?", edited.Version.ID); links != 1 {
		t.Fatalf("edited note links=%d, want 1", links)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_note_version_created"); audits != 1 {
		t.Fatalf("edited note audit rows=%d, want 1", audits)
	}

	deleted, changed, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, edited.Version.ID, "deleted", "")
	if err != nil || !changed || deleted.Version.State != "deleted" || deleted.Version.Body != nil {
		t.Fatalf("delete note: note=%+v changed=%v err=%v", deleted, changed, err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_note_versions WHERE note_id=?", note.ID); versions != 3 {
		t.Fatalf("note versions=%d, want 3", versions)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_note_heads WHERE review_context_id=? AND note_version_id=?", fixture.context1, deleted.Version.ID); head != 1 {
		t.Fatalf("deleted note head rows=%d, want 1", head)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_note_tombstoned"); audits != 1 {
		t.Fatalf("deleted note audit rows=%d, want 1", audits)
	}
}

// TestReviewNoteMutationRollsBackOnLinkFailure verifies a failed link insert
// removes a new note or note version and leaves the head unchanged.
func TestReviewNoteMutationRollsBackOnLinkFailure(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Plain note")
	if err != nil {
		t.Fatal(err)
	}
	auditsBefore := reviewAuditCount(t, fixture.db, "review_note_created") + reviewAuditCount(t, fixture.db, "review_note_version_created")
	if _, err := fixture.db.DB.Exec(`CREATE TRIGGER mutation_fail_note_link BEFORE INSERT ON review_note_links
		BEGIN SELECT RAISE(ABORT, 'injected link failure'); END`); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "New [[ext:https://example.test]]"); err == nil || !strings.Contains(err.Error(), "injected link failure") {
		t.Fatalf("create note error=%v, want injected link failure", err)
	}
	if notes := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_notes"); notes != 1 {
		t.Fatalf("failed create left %d notes, want 1", notes)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_note_versions"); versions != 1 {
		t.Fatalf("failed create left %d versions, want 1", versions)
	}
	if heads := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_note_heads"); heads != 1 {
		t.Fatalf("failed create left %d heads, want 1", heads)
	}

	if _, _, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, note.Version.ID, "active", "Edited [[ext:https://example.test]]"); err == nil || !strings.Contains(err.Error(), "injected link failure") {
		t.Fatalf("edit note error=%v, want injected link failure", err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_note_versions WHERE note_id=?", note.ID); versions != 1 {
		t.Fatalf("failed edit left %d versions, want 1", versions)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_note_heads WHERE review_context_id=? AND note_version_id=?", fixture.context1, note.Version.ID); head != 1 {
		t.Fatalf("failed edit moved the head")
	}
	if audits := reviewAuditCount(t, fixture.db, "review_note_created") + reviewAuditCount(t, fixture.db, "review_note_version_created"); audits != auditsBefore {
		t.Fatalf("failed note mutations changed audit rows: before=%d after=%d", auditsBefore, audits)
	}
}

// TestReviewAnchorMutationCommitsAtomically verifies anchor creation, edits,
// identical saves, tombstones, head moves, and audit commit as one unit.
func TestReviewAnchorMutationCommitsAtomically(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	if anchor.Label != "methods-1" || anchor.Version.State != "active" || anchor.Version.Page == nil || *anchor.Version.Page != 1 || len(anchor.Version.Rectangles) != 1 {
		t.Fatalf("created anchor=%+v", anchor)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_version_id=?", fixture.context1, anchor.Version.ID); head != 1 {
		t.Fatalf("created anchor head rows=%d, want 1", head)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_anchor_created"); audits != 1 {
		t.Fatalf("created anchor audit rows=%d, want 1", audits)
	}

	moved, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "active", hash, 2, "Updated", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}})
	if err != nil || !changed || moved.Version.ParentVersionID == nil || *moved.Version.ParentVersionID != anchor.Version.ID {
		t.Fatalf("replace anchor: anchor=%+v changed=%v err=%v", moved, changed, err)
	}
	if _, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, moved.Version.ID, "active", hash, 2, "Updated", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}}); err != nil || changed {
		t.Fatalf("identical anchor save changed=%v err=%v", changed, err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchor_versions WHERE anchor_id=?", anchor.ID); versions != 2 {
		t.Fatalf("anchor versions=%d, want 2", versions)
	}

	deleted, changed, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, moved.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil || !changed || deleted.Version.State != "deleted" || deleted.Version.Page != nil || deleted.Version.SelectedText != nil || deleted.Version.Rectangles != nil {
		t.Fatalf("delete anchor: anchor=%+v changed=%v err=%v", deleted, changed, err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchor_versions WHERE anchor_id=?", anchor.ID); versions != 3 {
		t.Fatalf("anchor versions=%d, want 3", versions)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_version_id=?", fixture.context1, deleted.Version.ID); head != 1 {
		t.Fatalf("deleted anchor head rows=%d, want 1", head)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_anchor_tombstoned"); audits != 1 {
		t.Fatalf("deleted anchor audit rows=%d, want 1", audits)
	}
}

// TestReviewAnchorMutationRollsBackOnVersionFailure verifies a failed anchor
// version insert removes the logical anchor, head, and audit evidence.
func TestReviewAnchorMutationRollsBackOnVersionFailure(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	if _, err := fixture.db.DB.Exec(`CREATE TRIGGER mutation_fail_anchor_version BEFORE INSERT ON review_anchor_versions
		BEGIN SELECT RAISE(ABORT, 'injected anchor version failure'); END`); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	if _, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}}); err == nil || !strings.Contains(err.Error(), "injected anchor version failure") {
		t.Fatalf("create anchor error=%v, want injected anchor version failure", err)
	}
	if anchors := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchors"); anchors != 0 {
		t.Fatalf("failed create left %d anchors", anchors)
	}
	if heads := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_anchor_heads"); heads != 0 {
		t.Fatalf("failed create left %d anchor heads", heads)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_anchor_created"); audits != 0 {
		t.Fatalf("failed create left %d audit rows", audits)
	}
}

// TestReviewAnchorVersionRollsBackOnHeadFailure verifies a failed head
// compare-and-swap removes the appended version and leaves the head unchanged.
func TestReviewAnchorVersionRollsBackOnHeadFailure(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.DB.Exec(`CREATE TRIGGER mutation_fail_anchor_head BEFORE UPDATE ON review_context_anchor_heads
		BEGIN SELECT RAISE(ABORT, 'injected anchor head failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "active", hash, 2, "Updated", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}}); err == nil || !strings.Contains(err.Error(), "injected anchor head failure") {
		t.Fatalf("replace anchor error=%v, want injected anchor head failure", err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchor_versions WHERE anchor_id=?", anchor.ID); versions != 1 {
		t.Fatalf("failed replace left %d versions, want 1", versions)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_version_id=?", fixture.context1, anchor.Version.ID); head != 1 {
		t.Fatalf("failed replace moved the anchor head")
	}
}

// TestReviewContextMutationCommitsWithInheritedHeads verifies context
// initialization copies parent heads and records audit in one transaction.
func TestReviewContextMutationCommitsWithInheritedHeads(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Inherited note")
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	context2, created, err := fixture.db.Review.CreateContext(ctx, fixture.run2, &fixture.context1)
	if err != nil || !created || context2.ParentContextID == nil || *context2.ParentContextID != fixture.context1 {
		t.Fatalf("create child context: context=%+v created=%v err=%v", context2, created, err)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_work_heads WHERE review_context_id=? AND work_revision_id=?", context2.ID, fixture.revision2); head != 1 {
		t.Fatalf("child work head rows=%d, want 1", head)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_note_heads WHERE review_context_id=? AND note_id=? AND note_version_id=?", context2.ID, note.ID, note.Version.ID); head != 1 {
		t.Fatalf("child note head rows=%d, want 1", head)
	}
	if head := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_id=? AND anchor_version_id=?", context2.ID, anchor.ID, anchor.Version.ID); head != 1 {
		t.Fatalf("child anchor head rows=%d, want 1", head)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_context_created"); audits != 2 {
		t.Fatalf("context audit rows=%d, want 2", audits)
	}
}

// TestReviewContextMutationRollsBackOnWorkHeadFailure verifies a failed work
// head freeze removes the context row and audit evidence.
func TestReviewContextMutationRollsBackOnWorkHeadFailure(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	if _, err := fixture.db.DB.Exec(`CREATE TRIGGER mutation_fail_context_work_heads BEFORE INSERT ON review_context_work_heads
		BEGIN SELECT RAISE(ABORT, 'injected work head failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2, nil); err == nil || !strings.Contains(err.Error(), "injected work head failure") {
		t.Fatalf("create context error=%v, want injected work head failure", err)
	}
	if contexts := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_contexts WHERE pipeline_run_id=?", fixture.run2); contexts != 0 {
		t.Fatalf("failed context left %d rows", contexts)
	}
	if audits := reviewAuditCount(t, fixture.db, "review_context_created"); audits != 1 {
		t.Fatalf("failed context left %d audit rows, want only the fixture context", audits)
	}
}

// TestReviewMutationsRejectReadOnlyRun verifies trashed runs reject every
// mutation with a typed lifecycle error before writing any row.
func TestReviewMutationsRejectReadOnlyRun(t *testing.T) {
	fixture := newReviewMutationFixture(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	note, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Existing note")
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Run.Trash(ctx, fixture.run1, "mutation test"); err != nil {
		t.Fatal(err)
	}
	versionsBefore := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM work_review_versions")
	notesBefore := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_notes")
	anchorsBefore := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchors")

	assertLifecycle := func(name string, err error) {
		t.Helper()
		var repositoryError *review.Error
		if !errors.As(err, &repositoryError) || repositoryError.Kind != "lifecycle" {
			t.Fatalf("%s error=%v, want lifecycle error", name, err)
		}
	}
	if _, _, err := fixture.db.Review.AppendWorkReview(ctx, fixture.context1, fixture.revision1, nil, "approved", nil, nil); err == nil {
		t.Fatal("read-only decision unexpectedly succeeded")
	} else {
		assertLifecycle("decision", err)
	}
	if _, err := fixture.db.Review.CreateNote(ctx, fixture.context1, fixture.revision1, "Blocked note"); err == nil {
		t.Fatal("read-only note unexpectedly succeeded")
	} else {
		assertLifecycle("note", err)
	}
	if _, err := fixture.db.Review.CreateAnchor(ctx, fixture.context1, fixture.revision1, "blocked-anchor", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}}); err == nil {
		t.Fatal("read-only anchor unexpectedly succeeded")
	} else {
		assertLifecycle("anchor", err)
	}
	if _, _, err := fixture.db.Review.AppendNoteVersion(ctx, fixture.context1, note.ID, note.Version.ID, "active", "Blocked edit"); err == nil {
		t.Fatal("read-only note edit unexpectedly succeeded")
	} else {
		assertLifecycle("note version", err)
	}
	if _, _, err := fixture.db.Review.AppendAnchorVersion(ctx, fixture.context1, anchor.ID, anchor.Version.ID, "active", hash, 2, "Blocked", []review.AnchorRectangle{{X: .2, Y: .3, Width: .4, Height: .2}}); err == nil {
		t.Fatal("read-only anchor edit unexpectedly succeeded")
	} else {
		assertLifecycle("anchor version", err)
	}
	if versions := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM work_review_versions"); versions != versionsBefore {
		t.Fatalf("read-only mutations added versions: before=%d after=%d", versionsBefore, versions)
	}
	if notes := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_notes"); notes != notesBefore {
		t.Fatalf("read-only mutations added notes: before=%d after=%d", notesBefore, notes)
	}
	if anchors := reviewCount(t, fixture.db, "SELECT COUNT(*) FROM review_anchors"); anchors != anchorsBefore {
		t.Fatalf("read-only mutations added anchors: before=%d after=%d", anchorsBefore, anchors)
	}
	if _, err := fixture.db.Review.GetWorkReview(ctx, fixture.context1, fixture.revision1+999); err == nil {
		t.Fatal("missing work revision unexpectedly resolved")
	} else {
		var repositoryError *review.Error
		if !errors.As(err, &repositoryError) || repositoryError.Kind != "not_found" {
			t.Fatalf("missing work revision error=%v, want not_found", err)
		}
	}
	if _, _, err := fixture.db.Review.CreateContext(ctx, fixture.run2+999, nil); err == nil {
		t.Fatal("missing run unexpectedly initialized a context")
	} else {
		var repositoryError *review.Error
		if !errors.As(err, &repositoryError) || repositoryError.Kind != "not_found" {
			t.Fatalf("missing run error=%v, want not_found", err)
		}
	}
}
