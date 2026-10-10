// characterization_integration_test.go pins the review compare-and-swap,
// context-cancellation, and transaction-rollback contracts that the legacy
// root characterization suite covered before the family relocation.
//go:build integration

package review_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"analysis/database/review"
)

// TestCharacterizationReviewCompareAndSwapAndReplay verifies stale expected
// versions conflict, identical writes report no change, and heads move only
// through the affected-row compare-and-swap.
func TestCharacterizationReviewCompareAndSwapAndReplay(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	contextRecord, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}

	state, changed, err := db.Review.AppendWorkReview(ctx, contextRecord.ID, a1Revision, nil, "approved", nil, nil)
	if err != nil || !changed || state.Version == nil {
		t.Fatalf("first decision: state=%+v changed=%v err=%v", state, changed, err)
	}
	versionID := state.Version.ID
	replayed, changed, err := db.Review.AppendWorkReview(ctx, contextRecord.ID, a1Revision, &versionID, "approved", nil, nil)
	if err != nil || changed || replayed.Version == nil || replayed.Version.ID != versionID {
		t.Fatalf("replayed decision: state=%+v changed=%v err=%v", replayed, changed, err)
	}
	stale := int64(999999)
	if _, _, err := db.Review.AppendWorkReview(ctx, contextRecord.ID, a1Revision, &stale, "removed", []string{"duplicate"}, nil); !review.IsConflict(err) {
		t.Fatalf("stale decision error = %v, want review conflict", err)
	}
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM work_review_versions WHERE work_revision_id=?", a1Revision); versions != 1 {
		t.Fatalf("work review versions = %d, want 1 after replay and conflict", versions)
	}

	note, err := db.Review.CreateNote(ctx, contextRecord.ID, a1Revision, "Characterization note body")
	if err != nil {
		t.Fatal(err)
	}
	replayedNote, changed, err := db.Review.AppendNoteVersion(ctx, contextRecord.ID, note.ID, note.Version.ID, "active", "Characterization note body")
	if err != nil || changed || replayedNote.Version.ID != note.Version.ID {
		t.Fatalf("replayed note: note=%+v changed=%v err=%v", replayedNote, changed, err)
	}
	staleNoteVersion := note.Version.ID + 999
	if _, _, err := db.Review.AppendNoteVersion(ctx, contextRecord.ID, note.ID, staleNoteVersion, "active", "stale body"); !review.IsConflict(err) {
		t.Fatalf("stale note error = %v, want review conflict", err)
	}

	hash := strings.Repeat("b", 64)
	anchor, err := db.Review.CreateAnchor(ctx, contextRecord.ID, a1Revision, "characterization-anchor", hash, 1, "selected text",
		[]review.AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	replayedAnchor, changed, err := db.Review.AppendAnchorVersion(ctx, contextRecord.ID, anchor.ID, anchor.Version.ID,
		"active", hash, 1, "selected text", []review.AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil || changed || replayedAnchor.Version.ID != anchor.Version.ID {
		t.Fatalf("replayed anchor: anchor=%+v changed=%v err=%v", replayedAnchor, changed, err)
	}
	if _, _, err := db.Review.AppendAnchorVersion(ctx, contextRecord.ID, anchor.ID, anchor.Version.ID+999,
		"deleted", hash, 0, "", nil); !review.IsConflict(err) {
		t.Fatalf("stale anchor error = %v, want review conflict", err)
	}
	moved, changed, err := db.Review.AppendAnchorVersion(ctx, contextRecord.ID, anchor.ID, anchor.Version.ID,
		"active", hash, 2, "moved text", []review.AnchorRectangle{{X: 0.4, Y: 0.4, Width: 0.2, Height: 0.2}})
	if err != nil || !changed || moved.Version.ID == anchor.Version.ID {
		t.Fatalf("moved anchor: anchor=%+v changed=%v err=%v", moved, changed, err)
	}
	versions, err := db.Review.ListAnchorVersions(ctx, contextRecord.ID, anchor.ID, 0, 10)
	if err != nil || len(versions) != 2 || versions[0].ID != moved.Version.ID {
		t.Fatalf("anchor versions=%d err=%v", len(versions), err)
	}
}

// TestCharacterizationContextCancellation verifies repository reads and
// transaction-owned writes propagate a canceled context and leave no partial rows.
func TestCharacterizationContextCancellation(t *testing.T) {
	db, a1Run, a2Run, a1Revision, _ := newReviewLineageFixture(t)
	contextRecord, _, err := db.Review.CreateContext(context.Background(), a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := db.Review.CreateContext(canceled, a2Run, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled CreateContext error = %v, want context.Canceled", err)
	}
	if contexts := reviewCount(t, db, "SELECT COUNT(*) FROM review_contexts WHERE pipeline_run_id=?", a2Run); contexts != 0 {
		t.Fatalf("canceled CreateContext wrote %d contexts", contexts)
	}
	if _, _, err := db.Review.AppendWorkReview(canceled, contextRecord.ID, a1Revision, nil, "approved", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled AppendWorkReview error = %v, want context.Canceled", err)
	}
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM work_review_versions"); versions != 0 {
		t.Fatalf("canceled AppendWorkReview wrote %d versions", versions)
	}
	if _, err := db.Review.ListNotes(canceled, contextRecord.ID, a1Revision, 0, 10, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ListNotes error = %v, want context.Canceled", err)
	}
}

// TestCharacterizationTransactionRollback verifies a mid-transaction failure
// rolls back every earlier statement in the same review mutation.
func TestCharacterizationTransactionRollback(t *testing.T) {
	db, a1Run, a2Run, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	contextRecord, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	auditsBefore := reviewCount(t, db, "SELECT COUNT(*) FROM audit_events")
	if _, err := db.DB.Exec(`CREATE TRIGGER characterization_abort_audit
		BEFORE INSERT ON audit_events
		BEGIN
			SELECT RAISE(ABORT, 'characterization injected audit failure');
		END`); err != nil {
		t.Fatal(err)
	}
	defer db.DB.Exec("DROP TRIGGER IF EXISTS characterization_abort_audit")

	if _, _, err := db.Review.AppendWorkReview(ctx, contextRecord.ID, a1Revision, nil, "approved", nil, nil); err == nil || !strings.Contains(err.Error(), "characterization injected audit failure") {
		t.Fatalf("decision error = %v, want injected audit failure", err)
	}
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM work_review_versions"); versions != 0 {
		t.Fatalf("failed decision left %d versions", versions)
	}
	if audits := reviewCount(t, db, "SELECT COUNT(*) FROM audit_events"); audits != auditsBefore {
		t.Fatalf("failed decision left %d audits, want %d", audits, auditsBefore)
	}
	if _, err := db.Review.CreateNote(ctx, contextRecord.ID, a1Revision, "Rollback note body"); err == nil || !strings.Contains(err.Error(), "characterization injected audit failure") {
		t.Fatalf("note error = %v, want injected audit failure", err)
	}
	if notes := reviewCount(t, db, "SELECT COUNT(*) FROM review_notes"); notes != 0 {
		t.Fatalf("failed note left %d notes", notes)
	}
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM review_note_versions"); versions != 0 {
		t.Fatalf("failed note left %d versions", versions)
	}
	if heads := reviewCount(t, db, "SELECT COUNT(*) FROM review_context_note_heads"); heads != 0 {
		t.Fatalf("failed note left %d heads", heads)
	}
	if _, _, err := db.Review.CreateContext(ctx, a2Run, nil); err == nil || !strings.Contains(err.Error(), "characterization injected audit failure") {
		t.Fatalf("context error = %v, want injected audit failure", err)
	}
	if contexts := reviewCount(t, db, "SELECT COUNT(*) FROM review_contexts WHERE pipeline_run_id=?", a2Run); contexts != 0 {
		t.Fatalf("failed context left %d rows, want rollback", contexts)
	}
}
