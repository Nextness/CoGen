// note_integration_test.go verifies immutable note lineage, tombstones,
// restoration history, and unsafe-link validation.
//go:build integration

package review_test

import (
	"context"
	"errors"
	"testing"

	"analysis/database/review"
)

// TestReviewNoteCopyOnWriteLineage verifies inherited notes, edits, tombstones, restoration, and stable parent heads.
func TestReviewNoteCopyOnWriteLineage(t *testing.T) {
	db, a1Run, a2Run, a1Revision, a2Revision := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	note, err := db.Review.CreateNote(ctx, a1.ID, a1Revision, "See [[article:10.1000/missing|future article]].")
	if err != nil {
		t.Fatal(err)
	}
	a2, _, err := db.Review.CreateContext(ctx, a2Run, &a1.ID)
	if err != nil {
		t.Fatal(err)
	}
	inheritedNotes, err := db.Review.ListNotes(ctx, a2.ID, a2Revision, 0, 20, false)
	if err != nil || len(inheritedNotes) != 1 || inheritedNotes[0].Version.ID != note.Version.ID {
		t.Fatalf("inherited notes = %+v err=%v", inheritedNotes, err)
	}
	edited, changed, err := db.Review.AppendNoteVersion(ctx, a2.ID, note.ID, note.Version.ID, "active", "Edited [[ext:https://example.test|source]].")
	if err != nil || !changed {
		t.Fatalf("edit inherited note: %+v changed=%v err=%v", edited, changed, err)
	}
	deleted, changed, err := db.Review.AppendNoteVersion(ctx, a2.ID, note.ID, edited.Version.ID, "deleted", "")
	if err != nil || !changed || deleted.Version.Body != nil {
		t.Fatalf("delete note: %+v changed=%v err=%v", deleted, changed, err)
	}
	restored, changed, err := db.Review.AppendNoteVersion(ctx, a2.ID, note.ID, deleted.Version.ID, "active", "Restored")
	if err != nil || !changed || restored.Version.ParentVersionID == nil || *restored.Version.ParentVersionID != deleted.Version.ID {
		t.Fatalf("restore note: %+v changed=%v err=%v", restored, changed, err)
	}
	versions, err := db.Review.ListNoteVersions(ctx, a2.ID, note.ID, 0, 20)
	if err != nil || len(versions) != 4 || versions[len(versions)-1].Body == nil || *versions[len(versions)-1].Body != *note.Version.Body {
		t.Fatalf("note history = %+v err=%v", versions, err)
	}
	a1Note, err := db.Review.GetNote(ctx, a1.ID, note.ID)
	if err != nil || a1Note.Version.ID != note.Version.ID {
		t.Fatalf("A1 note changed: %+v err=%v", a1Note, err)
	}
}

// TestReviewNoteRejectsUnsafeLink verifies save-blocking note-language validation.
func TestReviewNoteRejectsUnsafeLink(t *testing.T) {
	db, a1Run, _, a1Revision, _ := newReviewLineageFixture(t)
	ctx := context.Background()
	a1, _, err := db.Review.CreateContext(ctx, a1Run, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Review.CreateNote(ctx, a1.ID, a1Revision, "[[ext:javascript:alert(1)]]"); err == nil {
		t.Fatal("unsafe note link was accepted")
	} else {
		var syntax *review.NoteSyntaxError
		if !errors.As(err, &syntax) || len(syntax.Errors) == 0 {
			t.Fatalf("unsafe note link error=%v, want note syntax error", err)
		}
	}
}
