// authorship_test.go exercises the ordered-authorship store against a database
// created by the production migration runner.
//go:build integration

package author_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/author"
	"analysis/database/work"
)

// TestAuthorshipAppendOnlyTriggerUpdate verifies that direct UPDATE on
// authorships is rejected by the database trigger.
func TestAuthorshipAppendOnlyTriggerUpdate(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	revisionID := createTestRevision(t, db, "10.1000/authorship-trigger-update", work.ProducerStageParse)
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Locked Author"})
	if err != nil {
		t.Fatal(err)
	}
	authorshipID, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1,
	})
	if err != nil {
		t.Fatalf("CreateAuthorship: %v", err)
	}
	_, err = db.DB.Exec("UPDATE authorships SET author_order = 99 WHERE id = ?", authorshipID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on UPDATE")
	}
	if !strings.Contains(err.Error(), "authorships is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}
	authorships, err := store.ListAuthorshipsByRevisionID(ctx, revisionID)
	if err != nil {
		t.Fatalf("ListAuthorshipsByRevisionID after rejected UPDATE: %v", err)
	}
	if len(authorships) != 1 || authorships[0].AuthorOrder != 1 {
		t.Fatalf("expected author_order=1 after rejected UPDATE, got %+v", authorships)
	}
}

// TestAuthorshipAppendOnlyTriggerDelete verifies that direct DELETE on
// authorships is rejected by the database trigger.
func TestAuthorshipAppendOnlyTriggerDelete(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	revisionID := createTestRevision(t, db, "10.1000/authorship-trigger-delete", work.ProducerStageParse)
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Locked Author"})
	if err != nil {
		t.Fatal(err)
	}
	authorshipID, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1,
	})
	if err != nil {
		t.Fatalf("CreateAuthorship: %v", err)
	}
	_, err = db.DB.Exec("DELETE FROM authorships WHERE id = ?", authorshipID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on DELETE")
	}
	if !strings.Contains(err.Error(), "authorships is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}
	authorships, err := store.ListAuthorshipsByRevisionID(ctx, revisionID)
	if err != nil {
		t.Fatalf("ListAuthorshipsByRevisionID after rejected DELETE: %v", err)
	}
	if len(authorships) != 1 {
		t.Fatal("authorship was deleted despite trigger rejection")
	}
}

// TestAuthorshipCreateAndRetrieveByRevision verifies that an authorship can
// be created linking a work revision and an author occurrence, and retrieved
// by revision ID in author order.
func TestAuthorshipCreateAndRetrieveByRevision(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	revisionID := createTestRevision(t, db, "10.1000/authorship-test", work.ProducerStageParse)
	firstOccurrence, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John"})
	if err != nil {
		t.Fatal(err)
	}
	secondOccurrence, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Doe, Jane"})
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: firstOccurrence, AuthorOrder: 1, Affiliation: "University A",
	})
	if err != nil {
		t.Fatalf("CreateAuthorship 1: %v", err)
	}
	second, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: secondOccurrence, AuthorOrder: 2,
	})
	if err != nil {
		t.Fatalf("CreateAuthorship 2: %v", err)
	}
	if first == second {
		t.Fatal("distinct authorships must have different IDs")
	}

	authorships, err := store.ListAuthorshipsByRevisionID(ctx, revisionID)
	if err != nil {
		t.Fatalf("ListAuthorshipsByRevisionID: %v", err)
	}
	if len(authorships) != 2 {
		t.Fatalf("expected 2 authorships, got %d", len(authorships))
	}
	if authorships[0].AuthorOrder != 1 || authorships[0].AuthorOccurrenceID != firstOccurrence || authorships[0].Affiliation != "University A" {
		t.Fatalf("first authorship = %+v", authorships[0])
	}
	if authorships[1].AuthorOrder != 2 || authorships[1].AuthorOccurrenceID != secondOccurrence || authorships[1].Affiliation != "" {
		t.Fatalf("second authorship = %+v", authorships[1])
	}
	if absent, err := store.ListAuthorshipsByRevisionID(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent authorship list = %+v err=%v, want nil without error", absent, err)
	}
}

// TestAuthorshipRejectsMissingWorkRevisionID verifies authorship rejects missing work revision id.
func TestAuthorshipRejectsMissingWorkRevisionID(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateAuthorship(ctx, &author.Authorship{AuthorOccurrenceID: 1, AuthorOrder: 1}); err == nil {
		t.Fatal("expected error for missing work_revision_id")
	}
}

// TestAuthorshipRejectsMissingOccurrenceID verifies authorship rejects missing occurrence id.
func TestAuthorshipRejectsMissingOccurrenceID(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: 1, AuthorOrder: 1}); err == nil {
		t.Fatal("expected error for missing author_occurrence_id")
	}
}

// TestAuthorshipRejectsInvalidOrder verifies authorship rejects invalid order.
func TestAuthorshipRejectsInvalidOrder(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateAuthorship(ctx, &author.Authorship{WorkRevisionID: 1, AuthorOccurrenceID: 1, AuthorOrder: 0}); err == nil {
		t.Fatal("expected error for zero author_order")
	}
	if _, err := store.CreateAuthorship(ctx, nil); err == nil {
		t.Fatal("expected error for nil authorship")
	}
}

// TestAuthorshipUniqueOrderPerRevision verifies authorship unique order per revision.
func TestAuthorshipUniqueOrderPerRevision(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	revisionID := createTestRevision(t, db, "10.1000/unique-order", work.ProducerStageParse)
	firstOccurrence, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	secondOccurrence, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: firstOccurrence, AuthorOrder: 1,
	}); err != nil {
		t.Fatalf("first authorship: %v", err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: secondOccurrence, AuthorOrder: 1,
	}); err == nil {
		t.Fatal("expected error for duplicate author_order on same revision")
	}
}

// TestAuthorshipUniqueOccurrencePerRevision verifies authorship unique occurrence per revision.
func TestAuthorshipUniqueOccurrencePerRevision(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	revisionID := createTestRevision(t, db, "10.1000/unique-occurrence", work.ProducerStageParse)
	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1,
	}); err != nil {
		t.Fatalf("first authorship: %v", err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: 2,
	}); err == nil {
		t.Fatal("expected error for duplicate occurrence on same revision")
	}
}

// TestAuthorshipFkRejectsNonexistentRevision verifies authorship fk rejects nonexistent revision.
func TestAuthorshipFkRejectsNonexistentRevision(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: 99999, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1,
	}); err == nil {
		t.Fatal("expected FK error for nonexistent work_revision")
	}
}

// TestAuthorshipFkRejectsNonexistentOccurrence verifies authorship fk rejects nonexistent occurrence.
func TestAuthorshipFkRejectsNonexistentOccurrence(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	revisionID := createTestRevision(t, db, "10.1000/fk-occurrence", work.ProducerStageParse)
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: 99999, AuthorOrder: 1,
	}); err == nil {
		t.Fatal("expected FK error for nonexistent author_occurrence")
	}
}

// TestAuthorshipFkRejectsNonexistentPerson verifies authorship fk rejects nonexistent person.
func TestAuthorshipFkRejectsNonexistentPerson(t *testing.T) {
	_, db := openFamilyStore(t)

	// Direct INSERT to bypass the repository's auto-linking.
	if _, err := db.DB.Exec(`INSERT INTO author_occurrences (person_id, citation_name) VALUES (99999, 'Test')`); err == nil {
		t.Fatal("expected FK error for nonexistent person_id")
	}
}

// TestTwoRevisionsDistinctAuthorshipSets verifies two revisions distinct authorship sets.
func TestTwoRevisionsDistinctAuthorshipSets(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	firstRevision := createTestRevision(t, db, "10.1000/two-revisions", work.ProducerStageParse)
	secondRevision := createTestRevision(t, db, "10.1000/two-revisions", work.ProducerStageEnrich)

	firstA, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John"})
	if err != nil {
		t.Fatal(err)
	}
	firstB, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Doe, Jane"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: firstRevision, AuthorOccurrenceID: firstA, AuthorOrder: 1, Affiliation: "Univ A",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: firstRevision, AuthorOccurrenceID: firstB, AuthorOrder: 2,
	}); err != nil {
		t.Fatal(err)
	}

	secondA, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Brown, Bob"})
	if err != nil {
		t.Fatal(err)
	}
	secondB, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: secondRevision, AuthorOccurrenceID: secondA, AuthorOrder: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: secondRevision, AuthorOccurrenceID: secondB, AuthorOrder: 2, Affiliation: "Univ B",
	}); err != nil {
		t.Fatal(err)
	}

	firstSet, err := store.ListAuthorshipsByRevisionID(ctx, firstRevision)
	if err != nil {
		t.Fatalf("ListAuthorshipsByRevisionID first: %v", err)
	}
	if len(firstSet) != 2 || firstSet[0].AuthorOccurrenceID != firstA || firstSet[0].Affiliation != "Univ A" || firstSet[1].AuthorOccurrenceID != firstB {
		t.Fatalf("first revision authorship set = %+v", firstSet)
	}
	secondSet, err := store.ListAuthorshipsByRevisionID(ctx, secondRevision)
	if err != nil {
		t.Fatalf("ListAuthorshipsByRevisionID second: %v", err)
	}
	if len(secondSet) != 2 || secondSet[0].AuthorOccurrenceID != secondA || secondSet[1].AuthorOccurrenceID != secondB || secondSet[1].Affiliation != "Univ B" {
		t.Fatalf("second revision authorship set = %+v", secondSet)
	}
}

// TestAuthorshipGetByOccurrenceID verifies authorship get by occurrence id.
func TestAuthorshipGetByOccurrenceID(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Shared Author"})
	if err != nil {
		t.Fatal(err)
	}
	firstRevision := createTestRevision(t, db, "10.1000/by-occurrence", work.ProducerStageParse)
	secondRevision := createTestRevision(t, db, "10.1000/by-occurrence", work.ProducerStageEnrich)
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: firstRevision, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAuthorship(ctx, &author.Authorship{
		WorkRevisionID: secondRevision, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1,
	}); err != nil {
		t.Fatal(err)
	}
	authorships, err := store.ListAuthorshipsByOccurrenceID(ctx, occurrenceID)
	if err != nil {
		t.Fatalf("ListAuthorshipsByOccurrenceID: %v", err)
	}
	if len(authorships) != 2 {
		t.Fatalf("expected 2 authorships for occurrence, got %d", len(authorships))
	}
	if absent, err := store.ListAuthorshipsByOccurrenceID(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent authorship list = %+v err=%v, want nil without error", absent, err)
	}
}
