// audit_integration_test.go verifies review audit evidence records identifiers
// and bounded decision state without leaking note bodies, reasons, or selected
// text into audit metadata.
//go:build integration

package review_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/review"
)

// TestReviewAuditRedaction verifies every review mutation appends audit
// evidence and none of the bounded research content reaches audit metadata.
func TestReviewAuditRedaction(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	reason := "Strong match"
	if _, _, err := db.Review.AppendWorkReview(ctx, a1.ID, a1Revision, nil, "approved", nil, &reason); err != nil {
		t.Fatal(err)
	}
	note, err := db.Review.CreateNote(ctx, a1.ID, a1Revision, "Secret note body")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Review.AppendNoteVersion(ctx, a1.ID, note.ID, note.Version.ID, "active", "Secret edited body"); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	if _, err := db.Review.CreateAnchor(ctx, a1.ID, a1Revision, "methods-1", hash, 1, "Secret selected text", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}}); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"review_context_created", "work_review_version_created", "review_note_created", "review_note_version_created", "review_anchor_created"} {
		if count := reviewAuditCount(t, db, action); count != 1 {
			t.Fatalf("audit action %q rows=%d, want 1", action, count)
		}
	}
	leaked := reviewCount(t, db, `SELECT COUNT(*) FROM audit_events
		WHERE metadata_json LIKE '%Strong match%'
		   OR metadata_json LIKE '%Secret note body%'
		   OR metadata_json LIKE '%Secret edited body%'
		   OR metadata_json LIKE '%Secret selected text%'`)
	if leaked != 0 {
		t.Fatalf("review audit metadata leaked note, reason, or selected text in %d rows", leaked)
	}
}
