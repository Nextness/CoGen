// occurrence_test.go exercises the observed-author store against a database
// created by the production migration runner.
//go:build integration

package author_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/author"
)

// TestAuthorOccurrenceValidORCIDLinksToPerson verifies that an occurrence with
// a valid format-and-checksum ORCID creates or links to a Person record.
func TestAuthorOccurrenceValidORCIDLinksToPerson(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	occurrence := &author.Occurrence{
		CitationName: "Smith, John",
		ORCID:        "0000-0002-1694-233X",
	}
	id, err := store.CreateOccurrence(ctx, occurrence)
	if err != nil {
		t.Fatalf("CreateOccurrence: %v", err)
	}

	got, err := store.GetOccurrenceByID(ctx, id)
	if err != nil {
		t.Fatalf("GetOccurrenceByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected occurrence to exist")
	}
	if got.PersonID == 0 {
		t.Fatal("expected person_id to be set when ORCID is valid")
	}

	person, err := store.GetPersonByID(ctx, got.PersonID)
	if err != nil {
		t.Fatalf("GetPersonByID: %v", err)
	}
	if person == nil || person.ORCID != "0000-0002-1694-233x" {
		t.Fatalf("expected person with normalized ORCID, got %+v", person)
	}
}

// TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson verifies that an
// occurrence with a malformed or checksum-invalid ORCID stores the raw value
// but does not create or link to a Person record.
func TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		orcid string
	}{
		{"malformed", "not-an-orcid"},
		{"bad_checksum", "0000-0001-2345-6780"},
		{"short", "0000-0001-2345-678"},
		{"extra_digit", "0000-0001-2345-67890"},
	}
	for _, test := range tests {
		occurrence := &author.Occurrence{
			CitationName: "Test, " + test.name,
			ORCID:        test.orcid,
		}
		id, err := store.CreateOccurrence(ctx, occurrence)
		if err != nil {
			t.Fatalf("CreateOccurrence %s: %v", test.name, err)
		}
		got, err := store.GetOccurrenceByID(ctx, id)
		if err != nil {
			t.Fatalf("GetOccurrenceByID %s: %v", test.name, err)
		}
		if got.PersonID != 0 {
			t.Errorf("%s: expected person_id to be 0 for invalid ORCID, got %d", test.name, got.PersonID)
		}
		if got.ORCID != test.orcid {
			t.Errorf("%s: expected raw ORCID preserved, got %q", test.name, got.ORCID)
		}
	}
}

// TestAuthorOccurrenceAppendOnlyTriggerUpdate verifies that direct UPDATE on
// author_occurrences is rejected by the database trigger.
func TestAuthorOccurrenceAppendOnlyTriggerUpdate(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Immutable"})
	if err != nil {
		t.Fatalf("CreateOccurrence: %v", err)
	}
	_, err = db.DB.Exec("UPDATE author_occurrences SET citation_name = 'Changed' WHERE id = ?", occurrenceID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on UPDATE")
	}
	if !strings.Contains(err.Error(), "author_occurrences is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}
	got, err := store.GetOccurrenceByID(ctx, occurrenceID)
	if err != nil {
		t.Fatalf("GetOccurrenceByID after rejected UPDATE: %v", err)
	}
	if got == nil || got.CitationName != "Immutable" {
		t.Fatalf("expected citation_name %q after rejected UPDATE, got %+v", "Immutable", got)
	}
}

// TestAuthorOccurrenceAppendOnlyTriggerDelete verifies that direct DELETE on
// author_occurrences is rejected by the database trigger.
func TestAuthorOccurrenceAppendOnlyTriggerDelete(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	occurrenceID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Undeletable"})
	if err != nil {
		t.Fatalf("CreateOccurrence: %v", err)
	}
	_, err = db.DB.Exec("DELETE FROM author_occurrences WHERE id = ?", occurrenceID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on DELETE")
	}
	if !strings.Contains(err.Error(), "author_occurrences is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}
	got, err := store.GetOccurrenceByID(ctx, occurrenceID)
	if err != nil {
		t.Fatalf("GetOccurrenceByID after rejected DELETE: %v", err)
	}
	if got == nil {
		t.Fatal("occurrence was deleted despite trigger rejection")
	}
}

// TestAuthorOccurrenceCreateAndRetrieve verifies basic creation and retrieval
// of an author occurrence without an ORCID.
func TestAuthorOccurrenceCreateAndRetrieve(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	occurrence := &author.Occurrence{
		CitationName: "Smith, John",
		FirstName:    "John",
		LastName:     "Smith",
	}
	id, err := store.CreateOccurrence(ctx, occurrence)
	if err != nil {
		t.Fatalf("CreateOccurrence: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero occurrence ID")
	}

	got, err := store.GetOccurrenceByID(ctx, id)
	if err != nil {
		t.Fatalf("GetOccurrenceByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected occurrence to exist")
	}
	if got.CitationName != "Smith, John" || got.FirstName != "John" || got.LastName != "Smith" {
		t.Fatalf("occurrence fields = %+v", got)
	}
	if got.ORCID != "" || got.PersonID != 0 {
		t.Fatalf("expected empty ORCID and zero person_id, got %+v", got)
	}
	if missing, err := store.GetOccurrenceByID(ctx, 99999); err != nil || missing != nil {
		t.Fatalf("missing occurrence = %+v err=%v, want nil without error", missing, err)
	}
}

// TestAuthorOccurrenceRejectsEmptyCitationName verifies that CreateOccurrence
// requires a non-empty citation_name.
func TestAuthorOccurrenceRejectsEmptyCitationName(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreateOccurrence(ctx, &author.Occurrence{}); err == nil {
		t.Fatal("expected error for empty citation_name")
	}
	if _, err := store.CreateOccurrence(ctx, nil); err == nil {
		t.Fatal("expected error for nil occurrence")
	}
}

// TestAuthorOccurrenceWithORCIDLinksToPerson verifies that an occurrence with
// a non-empty ORCID automatically creates or links to a Person record.
func TestAuthorOccurrenceWithORCIDLinksToPerson(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	occurrence := &author.Occurrence{
		CitationName: "Smith, John",
		ORCID:        "0000-0001-2345-6789",
	}
	id, err := store.CreateOccurrence(ctx, occurrence)
	if err != nil {
		t.Fatalf("CreateOccurrence: %v", err)
	}
	got, err := store.GetOccurrenceByID(ctx, id)
	if err != nil {
		t.Fatalf("GetOccurrenceByID: %v", err)
	}
	if got == nil || got.PersonID == 0 {
		t.Fatalf("expected person_id to be set when ORCID is provided, got %+v", got)
	}
	if got.ORCID != "0000-0001-2345-6789" {
		t.Fatalf("expected ORCID %q, got %q", "0000-0001-2345-6789", got.ORCID)
	}
	person, err := store.GetPersonByID(ctx, got.PersonID)
	if err != nil {
		t.Fatalf("GetPersonByID: %v", err)
	}
	if person == nil || person.ORCID != "0000-0001-2345-6789" {
		t.Fatalf("expected person ORCID %q, got %+v", "0000-0001-2345-6789", person)
	}
}

// TestAuthorOccurrenceSameORCIDSharesPerson verifies that two occurrences
// with the same ORCID link to the same Person record.
func TestAuthorOccurrenceSameORCIDSharesPerson(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John", ORCID: "0000-0001-2345-6789"})
	if err != nil {
		t.Fatalf("CreateOccurrence 1: %v", err)
	}
	secondID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John", ORCID: "0000-0001-2345-6789"})
	if err != nil {
		t.Fatalf("CreateOccurrence 2: %v", err)
	}
	if firstID == secondID {
		t.Fatal("distinct occurrences must have different IDs")
	}
	first, _ := store.GetOccurrenceByID(ctx, firstID)
	second, _ := store.GetOccurrenceByID(ctx, secondID)
	if first.PersonID == 0 || second.PersonID == 0 {
		t.Fatal("both occurrences must have a person_id")
	}
	if first.PersonID != second.PersonID {
		t.Fatal("same ORCID must link to the same person")
	}
	occurrences, err := store.ListOccurrencesByPersonID(ctx, first.PersonID)
	if err != nil {
		t.Fatalf("ListOccurrencesByPersonID: %v", err)
	}
	if len(occurrences) != 2 {
		t.Fatalf("expected 2 occurrences for person, got %d", len(occurrences))
	}
	if absent, err := store.ListOccurrencesByPersonID(ctx, 99999); err != nil || absent != nil {
		t.Fatalf("absent occurrence list = %+v err=%v, want nil without error", absent, err)
	}
}

// TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct verifies that two
// occurrences with the same citation name but no ORCID stay as separate
// rows with no person link, and cannot be merged by name alone.
func TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	firstID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John"})
	if err != nil {
		t.Fatalf("CreateOccurrence 1: %v", err)
	}
	secondID, err := store.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Smith, John"})
	if err != nil {
		t.Fatalf("CreateOccurrence 2: %v", err)
	}
	if firstID == secondID {
		t.Fatal("same-name ORCID-less occurrences must have different IDs")
	}
	first, _ := store.GetOccurrenceByID(ctx, firstID)
	second, _ := store.GetOccurrenceByID(ctx, secondID)
	if first.PersonID != 0 || second.PersonID != 0 {
		t.Fatal("ORCID-less occurrences must not have a person_id")
	}
}
