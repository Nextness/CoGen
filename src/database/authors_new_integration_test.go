// Integration tests for people, author occurrences, and authorship repositories.
//go:build integration

package database

import (
	"strings"
	"testing"
)

// TestPersonCreateByORCID verifies that a person can be created by ORCID and
// retrieved by ID and ORCID, and that duplicate ORCIDs return the same ID.
func TestPersonCreateByORCID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	pid, err := db.People.CreateByORCID("0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("CreateByORCID: %v", err)
	}
	if pid == 0 {
		t.Fatal("expected non-zero person ID")
	}

	// Retrieve by ID
	p, err := db.People.GetByID(pid)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if p == nil {
		t.Fatal("expected person to exist")
	}
	if p.ORCID != "0000-0001-2345-6789" {
		t.Fatalf("expected ORCID %q, got %q", "0000-0001-2345-6789", p.ORCID)
	}
	if p.CreatedAt == "" {
		t.Fatal("expected non-empty created_at")
	}

	// Retrieve by ORCID
	p2, err := db.People.GetByORCID("0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("GetByORCID: %v", err)
	}
	if p2 == nil || p2.ID != pid {
		t.Fatal("GetByORCID should return the same person")
	}

	// Duplicate ORCID returns same ID
	pid2, err := db.People.CreateByORCID("0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("CreateByORCID duplicate: %v", err)
	}
	if pid2 != pid {
		t.Fatalf("expected same person ID %d for duplicate ORCID, got %d", pid, pid2)
	}
}

// TestPersonByORCIDNormalizesInput verifies that ORCID normalization is
// consistent between CreateByORCID and GetByORCID.
func TestPersonByORCIDNormalizesInput(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	pid, err := db.People.CreateByORCID(" 0000-0001-2345-6789 ")
	if err != nil {
		t.Fatalf("CreateByORCID: %v", err)
	}

	// Lookup with different casing should find the same person
	p, err := db.People.GetByORCID("0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("GetByORCID: %v", err)
	}
	if p == nil || p.ID != pid {
		t.Fatal("normalized ORCID lookup should find the same person")
	}
}

// TestPersonEmptyORCID verifies that empty ORCID is rejected for person creation.
func TestPersonEmptyORCID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.People.CreateByORCID("")
	if err == nil {
		t.Fatal("expected error for empty ORCID")
	}
}

// TestPersonMalformedORCIDRejected verifies that a malformed ORCID is rejected.
func TestPersonMalformedORCIDRejected(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.People.CreateByORCID("not-an-orcid")
	if err == nil {
		t.Fatal("expected error for malformed ORCID")
	}
}

// TestPersonInvalidChecksumORCIDRejected verifies that a well-formed ORCID
// with a wrong checksum is rejected.
func TestPersonInvalidChecksumORCIDRejected(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.People.CreateByORCID("0000-0001-2345-6780")
	if err == nil {
		t.Fatal("expected error for ORCID with invalid checksum")
	}
}

// TestPeopleORCIDGuardTriggers verifies that direct SQL cannot insert or update
// a person to a null, empty, or whitespace-only ORCID.
func TestPeopleORCIDGuardTriggers(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	const validORCID = "0000-0002-1694-233X"
	res, err := db.DB.Exec("INSERT INTO people (orcid) VALUES (?)", validORCID)
	if err != nil {
		t.Fatalf("expected valid INSERT to succeed, got: %v", err)
	}
	personID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("valid person ID: %v", err)
	}

	invalidValues := []any{nil, "", "   "}
	for _, value := range invalidValues {
		_, err := db.DB.Exec("INSERT INTO people (orcid) VALUES (?)", value)
		if err == nil {
			t.Fatalf("expected INSERT guard error for %#v", value)
		}
		if !strings.Contains(err.Error(), "people.orcid must not be null, empty, or whitespace") {
			t.Fatalf("expected trigger-specific INSERT error, got: %v", err)
		}
	}

	for _, value := range invalidValues {
		_, err := db.DB.Exec("UPDATE people SET orcid = ? WHERE id = ?", value, personID)
		if err == nil {
			t.Fatalf("expected UPDATE guard error for %#v", value)
		}
		if !strings.Contains(err.Error(), "people.orcid must not be null, empty, or whitespace") {
			t.Fatalf("expected trigger-specific UPDATE error, got: %v", err)
		}

		var got string
		if err := db.DB.QueryRow("SELECT orcid FROM people WHERE id = ?", personID).Scan(&got); err != nil {
			t.Fatalf("read person after rejected UPDATE: %v", err)
		}
		if got != validORCID {
			t.Fatalf("expected ORCID %q after rejected UPDATE, got %q", validORCID, got)
		}
	}
}

// TestAuthorOccurrenceValidORCIDLinksToPerson verifies that an occurrence with
// a valid format-and-checksum ORCID creates or links to a Person record.
func TestAuthorOccurrenceValidORCIDLinksToPerson(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ao := &AuthorOccurrence{
		CitationName: "Smith, John",
		ORCID:        "0000-0002-1694-233X",
	}
	id, err := db.AuthorOccs.Create(ao)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := db.AuthorOccs.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected occurrence to exist")
	}
	if got.PersonID == 0 {
		t.Fatal("expected person_id to be set when ORCID is valid")
	}

	person, err := db.People.GetByID(got.PersonID)
	if err != nil {
		t.Fatalf("GetByID person: %v", err)
	}
	if person == nil || person.ORCID != "0000-0002-1694-233x" {
		t.Fatalf("expected person with normalized ORCID, got %q", person.ORCID)
	}
}

// TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson verifies that an
// occurrence with a malformed or checksum-invalid ORCID stores the raw value
// but does not create or link to a Person record.
func TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	tests := []struct {
		name  string
		orcid string
	}{
		{"malformed", "not-an-orcid"},
		{"bad_checksum", "0000-0001-2345-6780"},
		{"short", "0000-0001-2345-678"},
		{"extra_digit", "0000-0001-2345-67890"},
	}
	for _, tc := range tests {
		ao := &AuthorOccurrence{
			CitationName: "Test, " + tc.name,
			ORCID:        tc.orcid,
		}
		id, err := db.AuthorOccs.Create(ao)
		if err != nil {
			t.Fatalf("Create %s: %v", tc.name, err)
		}

		got, err := db.AuthorOccs.GetByID(id)
		if err != nil {
			t.Fatalf("GetByID %s: %v", tc.name, err)
		}
		if got.PersonID != 0 {
			t.Errorf("%s: expected person_id to be 0 for invalid ORCID, got %d", tc.name, got.PersonID)
		}
		if got.ORCID != tc.orcid {
			t.Errorf("%s: expected raw ORCID preserved, got %q", tc.name, got.ORCID)
		}
	}
}

// TestAuthorOccurrenceAppendOnlyTriggerUpdate verifies that direct UPDATE on
// author_occurrences is rejected by the database trigger.
func TestAuthorOccurrenceAppendOnlyTriggerUpdate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	aoID, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Immutable"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = db.DB.Exec("UPDATE author_occurrences SET citation_name = 'Changed' WHERE id = ?", aoID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on UPDATE")
	}
	if !strings.Contains(err.Error(), "author_occurrences is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}

	// Reread the row — it must be unchanged
	got, err := db.AuthorOccs.GetByID(aoID)
	if err != nil {
		t.Fatalf("GetByID after rejected UPDATE: %v", err)
	}
	if got == nil {
		t.Fatal("occurrence vanished after rejected UPDATE")
	}
	if got.CitationName != "Immutable" {
		t.Fatalf("expected citation_name %q after rejected UPDATE, got %q", "Immutable", got.CitationName)
	}
}

// TestAuthorOccurrenceAppendOnlyTriggerDelete verifies that direct DELETE on
// author_occurrences is rejected by the database trigger.
func TestAuthorOccurrenceAppendOnlyTriggerDelete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	aoID, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Undeletable"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = db.DB.Exec("DELETE FROM author_occurrences WHERE id = ?", aoID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on DELETE")
	}
	if !strings.Contains(err.Error(), "author_occurrences is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}

	// Reread the row — it must still exist
	got, err := db.AuthorOccs.GetByID(aoID)
	if err != nil {
		t.Fatalf("GetByID after rejected DELETE: %v", err)
	}
	if got == nil {
		t.Fatal("occurrence was deleted despite trigger rejection")
	}
}

// TestAuthorOccurrenceCreateAndRetrieve verifies basic creation and retrieval
// of an author occurrence without an ORCID.
func TestAuthorOccurrenceCreateAndRetrieve(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ao := &AuthorOccurrence{
		CitationName: "Smith, John",
		FirstName:    "John",
		LastName:     "Smith",
	}
	id, err := db.AuthorOccs.Create(ao)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero occurrence ID")
	}

	// Retrieve by ID
	got, err := db.AuthorOccs.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected occurrence to exist")
	}
	if got.CitationName != "Smith, John" {
		t.Fatalf("expected citation_name %q, got %q", "Smith, John", got.CitationName)
	}
	if got.FirstName != "John" {
		t.Fatalf("expected first_name %q, got %q", "John", got.FirstName)
	}
	if got.LastName != "Smith" {
		t.Fatalf("expected last_name %q, got %q", "Smith", got.LastName)
	}
	if got.ORCID != "" {
		t.Fatalf("expected empty ORCID, got %q", got.ORCID)
	}
	if got.PersonID != 0 {
		t.Fatalf("expected zero person_id, got %d", got.PersonID)
	}
}

// TestAuthorOccurrenceRejectsEmptyCitationName verifies that Create requires
// a non-empty citation_name.
func TestAuthorOccurrenceRejectsEmptyCitationName(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.AuthorOccs.Create(&AuthorOccurrence{})
	if err == nil {
		t.Fatal("expected error for empty citation_name")
	}
}

// TestAuthorOccurrenceWithORCIDLinksToPerson verifies that an occurrence with
// a non-empty ORCID automatically creates or links to a Person record.
func TestAuthorOccurrenceWithORCIDLinksToPerson(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ao := &AuthorOccurrence{
		CitationName: "Smith, John",
		ORCID:        "0000-0001-2345-6789",
	}
	id, err := db.AuthorOccs.Create(ao)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := db.AuthorOccs.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected occurrence to exist")
	}
	if got.PersonID == 0 {
		t.Fatal("expected person_id to be set when ORCID is provided")
	}
	if got.ORCID != "0000-0001-2345-6789" {
		t.Fatalf("expected ORCID %q, got %q", "0000-0001-2345-6789", got.ORCID)
	}

	// Person record should exist
	person, err := db.People.GetByID(got.PersonID)
	if err != nil {
		t.Fatalf("GetByID person: %v", err)
	}
	if person == nil {
		t.Fatal("expected person to exist")
	}
	if person.ORCID != "0000-0001-2345-6789" {
		t.Fatalf("expected person ORCID %q, got %q", "0000-0001-2345-6789", person.ORCID)
	}
}

// TestAuthorOccurrenceSameORCIDSharesPerson verifies that two occurrences
// with the same ORCID link to the same Person record.
func TestAuthorOccurrenceSameORCIDSharesPerson(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ao1 := &AuthorOccurrence{
		CitationName: "Smith, John",
		ORCID:        "0000-0001-2345-6789",
	}
	id1, err := db.AuthorOccs.Create(ao1)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	ao2 := &AuthorOccurrence{
		CitationName: "Smith, John",
		ORCID:        "0000-0001-2345-6789",
	}
	id2, err := db.AuthorOccs.Create(ao2)
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	// Both occurrences must be distinct rows
	if id1 == id2 {
		t.Fatal("distinct occurrences must have different IDs")
	}

	got1, _ := db.AuthorOccs.GetByID(id1)
	got2, _ := db.AuthorOccs.GetByID(id2)
	if got1.PersonID == 0 || got2.PersonID == 0 {
		t.Fatal("both occurrences must have a person_id")
	}
	if got1.PersonID != got2.PersonID {
		t.Fatal("same ORCID must link to the same person")
	}

	// Person should have two occurrences
	occs, err := db.AuthorOccs.GetByPersonID(got1.PersonID)
	if err != nil {
		t.Fatalf("GetByPersonID: %v", err)
	}
	if len(occs) != 2 {
		t.Fatalf("expected 2 occurrences for person, got %d", len(occs))
	}
}

// TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct verifies that two
// occurrences with the same citation name but no ORCID stay as separate
// rows with no person link, and cannot be merged by name alone.
func TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ao1 := &AuthorOccurrence{
		CitationName: "Smith, John",
	}
	id1, err := db.AuthorOccs.Create(ao1)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	ao2 := &AuthorOccurrence{
		CitationName: "Smith, John",
	}
	id2, err := db.AuthorOccs.Create(ao2)
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	if id1 == id2 {
		t.Fatal("same-name ORCID-less occurrences must have different IDs")
	}

	got1, _ := db.AuthorOccs.GetByID(id1)
	got2, _ := db.AuthorOccs.GetByID(id2)
	if got1.PersonID != 0 || got2.PersonID != 0 {
		t.Fatal("ORCID-less occurrences must not have a person_id")
	}
}

// TestAuthorshipAppendOnlyTriggerUpdate verifies that direct UPDATE on
// authorships is rejected by the database trigger.
func TestAuthorshipAppendOnlyTriggerUpdate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/authorship-trigger-update")
	runID, _ := db.PipelineRuns.StartRun("trigger test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "Trigger Update Test",
	})
	aoID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Locked Author"})
	shipID, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: aoID, AuthorOrder: 1,
	})
	if err != nil {
		t.Fatalf("Create authorship: %v", err)
	}

	_, err = db.DB.Exec("UPDATE authorships SET author_order = 99 WHERE id = ?", shipID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on UPDATE")
	}
	if !strings.Contains(err.Error(), "authorships is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}

	// Reread the row — it must be unchanged
	ship, err := db.Authorships.GetByRevisionID(revID)
	if err != nil {
		t.Fatalf("GetByRevisionID after rejected UPDATE: %v", err)
	}
	if len(ship) != 1 || ship[0].AuthorOrder != 1 {
		t.Fatalf("expected author_order=1 after rejected UPDATE, got %+v", ship)
	}
}

// TestAuthorshipAppendOnlyTriggerDelete verifies that direct DELETE on
// authorships is rejected by the database trigger.
func TestAuthorshipAppendOnlyTriggerDelete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/authorship-trigger-delete")
	runID, _ := db.PipelineRuns.StartRun("trigger test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "Trigger Delete Test",
	})
	aoID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Locked Author"})
	shipID, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: aoID, AuthorOrder: 1,
	})
	if err != nil {
		t.Fatalf("Create authorship: %v", err)
	}

	_, err = db.DB.Exec("DELETE FROM authorships WHERE id = ?", shipID)
	if err == nil {
		t.Fatal("expected error from append-only trigger on DELETE")
	}
	if !strings.Contains(err.Error(), "authorships is append-only") {
		t.Fatalf("expected trigger-specific error message, got: %v", err)
	}

	// Reread the row — it must still exist
	ship, err := db.Authorships.GetByRevisionID(revID)
	if err != nil {
		t.Fatalf("GetByRevisionID after rejected DELETE: %v", err)
	}
	if len(ship) != 1 {
		t.Fatal("authorship was deleted despite trigger rejection")
	}
}

// TestAuthorshipCreateAndRetrieveByRevision verifies that an authorship can
// be created linking a work revision and an author occurrence, and retrieved
// by revision ID in author order.
func TestAuthorshipCreateAndRetrieveByRevision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create a work, run, and revision
	workID, _ := db.Works.CreateByDOI("10.1000/authorship-test")
	runID, _ := db.PipelineRuns.StartRun("authorship test", "q")
	revID, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "Authorship Test",
	})
	if err != nil {
		t.Fatalf("Create revision: %v", err)
	}

	// Create two author occurrences
	ao1ID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Smith, John"})
	ao2ID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Doe, Jane"})

	// Create authorships
	a1, err := db.Authorships.Create(&Authorship{
		WorkRevisionID:     revID,
		AuthorOccurrenceID: ao1ID,
		AuthorOrder:        1,
		Affiliation:        "University A",
	})
	if err != nil {
		t.Fatalf("Create authorship 1: %v", err)
	}

	a2, err := db.Authorships.Create(&Authorship{
		WorkRevisionID:     revID,
		AuthorOccurrenceID: ao2ID,
		AuthorOrder:        2,
	})
	if err != nil {
		t.Fatalf("Create authorship 2: %v", err)
	}

	if a1 == a2 {
		t.Fatal("distinct authorships must have different IDs")
	}

	// Retrieve by revision ID — must be in author_order
	authorships, err := db.Authorships.GetByRevisionID(revID)
	if err != nil {
		t.Fatalf("GetByRevisionID: %v", err)
	}
	if len(authorships) != 2 {
		t.Fatalf("expected 2 authorships, got %d", len(authorships))
	}
	if authorships[0].AuthorOrder != 1 {
		t.Fatalf("expected first author order 1, got %d", authorships[0].AuthorOrder)
	}
	if authorships[0].AuthorOccurrenceID != ao1ID {
		t.Fatal("first author should be Smith, John")
	}
	if authorships[0].Affiliation != "University A" {
		t.Fatalf("expected affiliation %q, got %q", "University A", authorships[0].Affiliation)
	}
	if authorships[1].AuthorOrder != 2 {
		t.Fatalf("expected second author order 2, got %d", authorships[1].AuthorOrder)
	}
	if authorships[1].AuthorOccurrenceID != ao2ID {
		t.Fatal("second author should be Doe, Jane")
	}
	if authorships[1].Affiliation != "" {
		t.Fatalf("expected empty affiliation, got %q", authorships[1].Affiliation)
	}
}

// TestAuthorshipRejectsMissingWorkRevisionID verifies authorship rejects missing work revision id.
func TestAuthorshipRejectsMissingWorkRevisionID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.Authorships.Create(&Authorship{
		AuthorOccurrenceID: 1,
		AuthorOrder:        1,
	})
	if err == nil {
		t.Fatal("expected error for missing work_revision_id")
	}
}

// TestAuthorshipRejectsMissingOccurrenceID verifies authorship rejects missing occurrence id.
func TestAuthorshipRejectsMissingOccurrenceID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: 1,
		AuthorOrder:    1,
	})
	if err == nil {
		t.Fatal("expected error for missing author_occurrence_id")
	}
}

// TestAuthorshipRejectsInvalidOrder verifies authorship rejects invalid order.
func TestAuthorshipRejectsInvalidOrder(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.Authorships.Create(&Authorship{
		WorkRevisionID:     1,
		AuthorOccurrenceID: 1,
		AuthorOrder:        0,
	})
	if err == nil {
		t.Fatal("expected error for zero author_order")
	}
}

// TestAuthorshipUniqueOrderPerRevision verifies authorship unique order per revision.
func TestAuthorshipUniqueOrderPerRevision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/unique-order")
	runID, _ := db.PipelineRuns.StartRun("unique order test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "Unique Order",
	})

	ao1ID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "A"})
	ao2ID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "B"})

	_, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: ao1ID, AuthorOrder: 1,
	})
	if err != nil {
		t.Fatalf("first authorship: %v", err)
	}

	_, err = db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: ao2ID, AuthorOrder: 1,
	})
	if err == nil {
		t.Fatal("expected error for duplicate author_order on same revision")
	}
}

// TestAuthorshipUniqueOccurrencePerRevision verifies authorship unique occurrence per revision.
func TestAuthorshipUniqueOccurrencePerRevision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/unique-occurrence")
	runID, _ := db.PipelineRuns.StartRun("unique occurrence test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "Unique Occurrence",
	})

	aoID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "A"})

	_, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: aoID, AuthorOrder: 1,
	})
	if err != nil {
		t.Fatalf("first authorship: %v", err)
	}

	_, err = db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: aoID, AuthorOrder: 2,
	})
	if err == nil {
		t.Fatal("expected error for duplicate occurrence on same revision")
	}
}

// TestAuthorshipFkRejectsNonexistentRevision verifies authorship fk rejects nonexistent revision.
func TestAuthorshipFkRejectsNonexistentRevision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	aoID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "A"})

	_, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: 99999, AuthorOccurrenceID: aoID, AuthorOrder: 1,
	})
	if err == nil {
		t.Fatal("expected FK error for nonexistent work_revision")
	}
}

// TestAuthorshipFkRejectsNonexistentOccurrence verifies authorship fk rejects nonexistent occurrence.
func TestAuthorshipFkRejectsNonexistentOccurrence(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/fk-occurrence")
	runID, _ := db.PipelineRuns.StartRun("fk test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "FK Occurrence",
	})

	_, err := db.Authorships.Create(&Authorship{
		WorkRevisionID: revID, AuthorOccurrenceID: 99999, AuthorOrder: 1,
	})
	if err == nil {
		t.Fatal("expected FK error for nonexistent author_occurrence")
	}
}

// TestAuthorshipFkRejectsNonexistentPerson verifies authorship fk rejects nonexistent person.
func TestAuthorshipFkRejectsNonexistentPerson(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Direct INSERT to bypass the repository's auto-linking
	_, err := db.DB.Exec(`
		INSERT INTO author_occurrences (person_id, citation_name)
		VALUES (99999, 'Test')`)
	if err == nil {
		t.Fatal("expected FK error for nonexistent person_id")
	}
}

// TestTwoRevisionsDistinctAuthorshipSets verifies two revisions distinct authorship sets.
func TestTwoRevisionsDistinctAuthorshipSets(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/two-revisions")
	runID, _ := db.PipelineRuns.StartRun("two revisions test", "q")

	// First revision with author set A
	rev1ID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "First Revision",
	})
	ao1A, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Smith, John"})
	ao1B, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Doe, Jane"})
	db.Authorships.Create(&Authorship{
		WorkRevisionID: rev1ID, AuthorOccurrenceID: ao1A, AuthorOrder: 1,
		Affiliation: "Univ A",
	})
	db.Authorships.Create(&Authorship{
		WorkRevisionID: rev1ID, AuthorOccurrenceID: ao1B, AuthorOrder: 2,
	})

	// Second revision with different author set B
	rev2ID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageEnrich,
		Title: "Second Revision",
	})
	ao2A, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Brown, Bob"})
	ao2B, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Smith, John"})
	// Reversed order
	db.Authorships.Create(&Authorship{
		WorkRevisionID: rev2ID, AuthorOccurrenceID: ao2A, AuthorOrder: 1,
	})
	db.Authorships.Create(&Authorship{
		WorkRevisionID: rev2ID, AuthorOccurrenceID: ao2B, AuthorOrder: 2,
		Affiliation: "Univ B",
	})

	// Verify revision 1 still has the original set
	ship1, err := db.Authorships.GetByRevisionID(rev1ID)
	if err != nil {
		t.Fatalf("GetByRevisionID rev1: %v", err)
	}
	if len(ship1) != 2 {
		t.Fatalf("expected 2 authorships for rev1, got %d", len(ship1))
	}
	if ship1[0].AuthorOccurrenceID != ao1A {
		t.Fatal("rev1 first author changed")
	}
	if ship1[0].Affiliation != "Univ A" {
		t.Fatal("rev1 first author affiliation changed")
	}
	if ship1[1].AuthorOccurrenceID != ao1B {
		t.Fatal("rev1 second author changed")
	}

	// Verify revision 2 has its own set
	ship2, err := db.Authorships.GetByRevisionID(rev2ID)
	if err != nil {
		t.Fatalf("GetByRevisionID rev2: %v", err)
	}
	if len(ship2) != 2 {
		t.Fatalf("expected 2 authorships for rev2, got %d", len(ship2))
	}
	if ship2[0].AuthorOccurrenceID != ao2A {
		t.Fatal("rev2 first author should be Brown, Bob")
	}
	if ship2[1].AuthorOccurrenceID != ao2B {
		t.Fatal("rev2 second author should be Smith, John")
	}
	if ship2[1].Affiliation != "Univ B" {
		t.Fatal("rev2 second author affiliation should be Univ B")
	}
}

// TestAuthorshipGetByOccurrenceID verifies authorship get by occurrence id.
func TestAuthorshipGetByOccurrenceID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/by-occurrence")
	runID, _ := db.PipelineRuns.StartRun("by occurrence test", "q")

	aoID, _ := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Shared Author"})

	// Same author on two different revisions
	rev1, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		Title: "Rev 1",
	})
	rev2, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageEnrich,
		Title: "Rev 2",
	})

	db.Authorships.Create(&Authorship{
		WorkRevisionID: rev1, AuthorOccurrenceID: aoID, AuthorOrder: 1,
	})
	db.Authorships.Create(&Authorship{
		WorkRevisionID: rev2, AuthorOccurrenceID: aoID, AuthorOrder: 1,
	})

	ships, err := db.Authorships.GetByOccurrenceID(aoID)
	if err != nil {
		t.Fatalf("GetByOccurrenceID: %v", err)
	}
	if len(ships) != 2 {
		t.Fatalf("expected 2 authorships for occurrence, got %d", len(ships))
	}
}
