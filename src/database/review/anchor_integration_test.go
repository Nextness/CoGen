// anchor_integration_test.go verifies immutable anchor lineage, inherited
// heads, tombstones, label identity, and label validation.
//go:build integration

package review_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/review"
	"analysis/notes"
)

// TestReviewAnchorCopyOnWriteLineage verifies inherited anchors, stable parent heads, and tombstones.
func TestReviewAnchorCopyOnWriteLineage(t *testing.T) {
	db, a1Run, a2Run, a1Revision, a2Revision := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	anchor, err := db.Review.CreateAnchor(ctx, a1.ID, a1Revision, "methods-1", hash, 1, "Methods", []review.AnchorRectangle{{X: .1, Y: .2, Width: .3, Height: .1}})
	if err != nil {
		t.Fatal(err)
	}
	a2, _, err := db.Review.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	anchors, err := db.Review.ListAnchors(ctx, a2.ID, a2Revision, "", 20)
	if err != nil || len(anchors) != 1 || anchors[0].Version.ID != anchor.Version.ID {
		t.Fatalf("inherited anchors = %+v err=%v", anchors, err)
	}
	if anchor.ID == anchor.Label || anchor.Label != "methods-1" || !notes.ValidAnchorID(anchor.ID) {
		t.Fatalf("generated anchor identity=%q label=%q", anchor.ID, anchor.Label)
	}
	deletedAnchor, changed, err := db.Review.AppendAnchorVersion(ctx, a2.ID, anchor.ID, anchor.Version.ID, "deleted", hash, 0, "", nil)
	if err != nil || !changed || deletedAnchor.Version.State != "deleted" {
		t.Fatalf("delete anchor: %+v changed=%v err=%v", deletedAnchor, changed, err)
	}
	a1Anchor, err := db.Review.GetAnchor(ctx, a1.ID, anchor.ID)
	if err != nil || a1Anchor.Version.ID != anchor.Version.ID {
		t.Fatalf("A1 anchor changed: %+v err=%v", a1Anchor, err)
	}
}

// TestReviewAnchorRejectsInvalidLabel verifies anchor labels must satisfy the note-language grammar.
func TestReviewAnchorRejectsInvalidLabel(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	if _, err := db.Review.CreateAnchor(ctx, a1.ID, a1Revision, "bad id", hash, 1, "text", []review.AnchorRectangle{{Width: 1, Height: 1}}); err == nil {
		t.Fatal("invalid anchor ID was accepted")
	}
}
