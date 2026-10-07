// author_test.go exercises the people store against a database created by the
// production migration runner.
//go:build integration

package author_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
	"analysis/database/author"
	"analysis/database/work"
)

// familyConfigPath points at the production database registry from the family test directory.
var familyConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openFamilyStore returns an author store over a database created by the production migration runner.
func openFamilyStore(t *testing.T) (*author.Store, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "author.db")
	db, err := database.Open(dbPath, familyConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return author.New(db.DB), db
}

// createTestRevision creates one work revision for authorship linkage and returns its ID.
func createTestRevision(t *testing.T, db *database.Database, doi, stage string) int64 {
	t.Helper()
	ctx := context.Background()
	workID, err := db.Work.CreateWorkByDOI(ctx, doi)
	if err != nil {
		t.Fatalf("create work: %v", err)
	}
	runID, err := db.Run.StartRun(ctx, "author-test", "")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: stage, Title: doi,
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	return revisionID
}

// TestPersonCreateByORCID verifies that a person can be created by ORCID and
// retrieved by ID and ORCID, and that duplicate ORCIDs return the same ID.
func TestPersonCreateByORCID(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	pid, err := store.CreatePersonByORCID(ctx, "0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("CreatePersonByORCID: %v", err)
	}
	if pid == 0 {
		t.Fatal("expected non-zero person ID")
	}

	person, err := store.GetPersonByID(ctx, pid)
	if err != nil {
		t.Fatalf("GetPersonByID: %v", err)
	}
	if person == nil {
		t.Fatal("expected person to exist")
	}
	if person.ORCID != "0000-0001-2345-6789" {
		t.Fatalf("expected ORCID %q, got %q", "0000-0001-2345-6789", person.ORCID)
	}
	if person.CreatedAt == "" {
		t.Fatal("expected non-empty created_at")
	}

	byORCID, err := store.GetPersonByORCID(ctx, "0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("GetPersonByORCID: %v", err)
	}
	if byORCID == nil || byORCID.ID != pid {
		t.Fatal("GetPersonByORCID should return the same person")
	}

	duplicate, err := store.CreatePersonByORCID(ctx, "0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("CreatePersonByORCID duplicate: %v", err)
	}
	if duplicate != pid {
		t.Fatalf("expected same person ID %d for duplicate ORCID, got %d", pid, duplicate)
	}
	if missing, err := store.GetPersonByID(ctx, 99999); err != nil || missing != nil {
		t.Fatalf("missing person = %+v err=%v, want nil without error", missing, err)
	}
	if missing, err := store.GetPersonByORCID(ctx, ""); err != nil || missing != nil {
		t.Fatalf("empty ORCID lookup = %+v err=%v, want nil without error", missing, err)
	}
}

// TestPersonByORCIDNormalizesInput verifies that ORCID normalization is
// consistent between CreatePersonByORCID and GetPersonByORCID.
func TestPersonByORCIDNormalizesInput(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	pid, err := store.CreatePersonByORCID(ctx, " 0000-0001-2345-6789 ")
	if err != nil {
		t.Fatalf("CreatePersonByORCID: %v", err)
	}
	person, err := store.GetPersonByORCID(ctx, "0000-0001-2345-6789")
	if err != nil {
		t.Fatalf("GetPersonByORCID: %v", err)
	}
	if person == nil || person.ID != pid {
		t.Fatal("normalized ORCID lookup should find the same person")
	}
}

// TestPersonEmptyORCID verifies that empty ORCID is rejected for person creation.
func TestPersonEmptyORCID(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreatePersonByORCID(ctx, ""); err == nil {
		t.Fatal("expected error for empty ORCID")
	}
}

// TestPersonMalformedORCIDRejected verifies that a malformed ORCID is rejected.
func TestPersonMalformedORCIDRejected(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreatePersonByORCID(ctx, "not-an-orcid"); err == nil {
		t.Fatal("expected error for malformed ORCID")
	}
}

// TestPersonInvalidChecksumORCIDRejected verifies that a well-formed ORCID
// with a wrong checksum is rejected.
func TestPersonInvalidChecksumORCIDRejected(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()

	if _, err := store.CreatePersonByORCID(ctx, "0000-0001-2345-6780"); err == nil {
		t.Fatal("expected error for ORCID with invalid checksum")
	}
}

// TestPeopleORCIDGuardTriggers verifies that direct SQL cannot insert or update
// a person to a null, empty, or whitespace-only ORCID.
func TestPeopleORCIDGuardTriggers(t *testing.T) {
	_, db := openFamilyStore(t)

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
