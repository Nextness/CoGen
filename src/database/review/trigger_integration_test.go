// trigger_integration_test.go verifies direct append-only trigger protection
// for every immutable review table.
//go:build integration

package review_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/review"
)

// TestReviewAppendOnlyTablesRejectUpdatesAndDeletes verifies database triggers
// reject direct mutation of every immutable review table.
func TestReviewAppendOnlyTablesRejectUpdatesAndDeletes(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err := db.Review.AppendWorkReview(ctx, a1.ID, a1Revision, nil, "not_approved", []string{"duplicate"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	note, err := db.Review.CreateNote(ctx, a1.ID, a1Revision, "Trigger note [[ext:https://example.test|source]]")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	anchor, err := db.Review.CreateAnchor(ctx, a1.ID, a1Revision, "trigger-anchor", hash, 1, "Trigger text", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		update string
		delete string
		args   []any
	}{
		{"review_contexts", "UPDATE review_contexts SET created_at=created_at WHERE id=?", "DELETE FROM review_contexts WHERE id=?", []any{a1.ID}},
		{"work_review_versions", "UPDATE work_review_versions SET status=status WHERE id=?", "DELETE FROM work_review_versions WHERE id=?", []any{state.Version.ID}},
		{"work_review_version_substatuses", "UPDATE work_review_version_substatuses SET sub_status=sub_status WHERE review_version_id=?", "DELETE FROM work_review_version_substatuses WHERE review_version_id=?", []any{state.Version.ID}},
		{"review_notes", "UPDATE review_notes SET work_id=work_id WHERE id=?", "DELETE FROM review_notes WHERE id=?", []any{note.ID}},
		{"review_note_versions", "UPDATE review_note_versions SET state=state WHERE id=?", "DELETE FROM review_note_versions WHERE id=?", []any{note.Version.ID}},
		{"review_note_links", "UPDATE review_note_links SET ordinal=ordinal WHERE note_version_id=?", "DELETE FROM review_note_links WHERE note_version_id=?", []any{note.Version.ID}},
		{"review_anchors", "UPDATE review_anchors SET work_id=work_id WHERE id=?", "DELETE FROM review_anchors WHERE id=?", []any{anchor.ID}},
		{"review_anchor_versions", "UPDATE review_anchor_versions SET state=state WHERE id=?", "DELETE FROM review_anchor_versions WHERE id=?", []any{anchor.Version.ID}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.DB.Exec(test.update, test.args...); err == nil || !strings.Contains(err.Error(), "append-only") {
				t.Fatalf("update error=%v, want append-only rejection", err)
			}
			if _, err := db.DB.Exec(test.delete, test.args...); err == nil || !strings.Contains(err.Error(), "append-only") {
				t.Fatalf("delete error=%v, want append-only rejection", err)
			}
		})
	}
	if contexts := reviewCount(t, db, "SELECT COUNT(*) FROM review_contexts WHERE id=?", a1.ID); contexts != 1 {
		t.Fatalf("context rows=%d after rejected mutations, want 1", contexts)
	}
	if versions := reviewCount(t, db, "SELECT COUNT(*) FROM work_review_versions WHERE id=?", state.Version.ID); versions != 1 {
		t.Fatalf("decision rows=%d after rejected mutations, want 1", versions)
	}
}
