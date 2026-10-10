// parity_integration_test.go verifies the legacy flat repositories and the
// author family store return and persist identical people, occurrence,
// authorship, resolution, and candidate evidence on separate copies of one
// migrated fixture.
//go:build integration

package author_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/database"
	"analysis/database/author"
	"analysis/database/work"
)

// TestAuthorFamilyParityWithLegacyRepositories verifies the adapters and
// family store agree on reads, writes, and errors for the author resources.
func TestAuthorFamilyParityWithLegacyRepositories(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := fixture.Run.StartRun(ctx, "author-parity", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := fixture.Work.CreateWorkByDOI(ctx, "10.1000/author-parity")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := fixture.Work.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageParse, Title: "parity",
	})
	if err != nil {
		t.Fatal(err)
	}
	personID, err := fixture.People.CreateByORCID("0000-0002-1694-233X")
	if err != nil {
		t.Fatal(err)
	}
	occurrenceID, err := fixture.AuthorOccs.Create(&database.AuthorOccurrence{
		CitationName: "Parity, Author", ORCID: "0000-0002-1694-233X",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Authorships.Create(&database.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: occurrenceID, AuthorOrder: 1, Affiliation: "Parity University",
	}); err != nil {
		t.Fatal(err)
	}
	resolutionID, err := fixture.IdentityResolutions.Create(&database.AuthorIdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: occurrenceID, Status: database.AuthorIdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Parity, Author", ResolvedAt: "2026-07-21T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.IdentityCandidates.Create(&database.AuthorIdentityCandidate{
		IdentityResolutionID: resolutionID, CandidateORCID: "0000-0001-5109-3700",
		QueryURL: "https://pub.orcid.org/v3.0/0000-0001-5109-3700", ProviderRank: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatal(err)
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	familyPath := filepath.Join(t.TempDir(), "family.db")
	copyDatabaseFile(t, fixturePath, legacyPath)
	copyDatabaseFile(t, fixturePath, familyPath)

	legacy, err := database.OpenExisting(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	family, err := database.OpenExisting(familyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer family.Close()
	store := author.New(family.DB)

	// Read parity for people, occurrences, and authorships.
	legacyPerson, err := legacy.People.GetByID(personID)
	if err != nil {
		t.Fatal(err)
	}
	familyPerson, err := store.GetPersonByID(ctx, personID)
	if err != nil {
		t.Fatal(err)
	}
	assertPersonParity(t, "GetPersonByID", legacyPerson, familyPerson)

	legacyOccurrence, err := legacy.AuthorOccs.GetByID(occurrenceID)
	if err != nil {
		t.Fatal(err)
	}
	familyOccurrence, err := store.GetOccurrenceByID(ctx, occurrenceID)
	if err != nil {
		t.Fatal(err)
	}
	assertOccurrenceParity(t, "GetOccurrenceByID", legacyOccurrence, familyOccurrence)

	legacyAuthorships, err := legacy.Authorships.GetByRevisionID(revisionID)
	if err != nil {
		t.Fatal(err)
	}
	familyAuthorships, err := store.ListAuthorshipsByRevisionID(ctx, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyAuthorships) != len(familyAuthorships) {
		t.Fatalf("authorship parity length: legacy=%d family=%d", len(legacyAuthorships), len(familyAuthorships))
	}
	for index := range legacyAuthorships {
		assertAuthorshipParity(t, "ListAuthorshipsByRevisionID", legacyAuthorships[index], familyAuthorships[index])
	}

	// Missing-read identity parity.
	legacyMissing, err := legacy.People.GetByID(99999)
	if err != nil || legacyMissing != nil {
		t.Fatalf("legacy missing person = %+v err=%v", legacyMissing, err)
	}
	familyMissing, err := store.GetPersonByID(ctx, 99999)
	if err != nil || familyMissing != nil {
		t.Fatalf("family missing person = %+v err=%v", familyMissing, err)
	}

	// Person write parity.
	legacyNewPerson, err := legacy.People.CreateByORCID("0000-0001-5109-3700")
	if err != nil {
		t.Fatal(err)
	}
	familyNewPerson, err := store.CreatePersonByORCID(ctx, "0000-0001-5109-3700")
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewPerson != familyNewPerson {
		t.Fatalf("CreatePersonByORCID id parity: legacy=%d family=%d", legacyNewPerson, familyNewPerson)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, orcid FROM people WHERE id=?", legacyNewPerson)

	// Occurrence write parity, including the person link copy-back.
	legacyNewOccurrence := &database.AuthorOccurrence{CitationName: "Parity, New", ORCID: "0000-0001-5109-3700"}
	familyNewOccurrence := &author.Occurrence{CitationName: "Parity, New", ORCID: "0000-0001-5109-3700"}
	legacyOccurrenceID, err := legacy.AuthorOccs.Create(legacyNewOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	familyOccurrenceID, err := store.CreateOccurrence(ctx, familyNewOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	if legacyOccurrenceID != familyOccurrenceID {
		t.Fatalf("CreateOccurrence id parity: legacy=%d family=%d", legacyOccurrenceID, familyOccurrenceID)
	}
	if legacyNewOccurrence.PersonID != familyNewOccurrence.PersonID {
		t.Fatalf("CreateOccurrence person parity: legacy=%d family=%d", legacyNewOccurrence.PersonID, familyNewOccurrence.PersonID)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, person_id, citation_name, orcid FROM author_occurrences WHERE id=?", legacyOccurrenceID)

	// Authorship write parity.
	legacyNewAuthorship := &database.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: legacyOccurrenceID, AuthorOrder: 2, Affiliation: "Parity Two",
	}
	familyNewAuthorship := &author.Authorship{
		WorkRevisionID: revisionID, AuthorOccurrenceID: familyOccurrenceID, AuthorOrder: 2, Affiliation: "Parity Two",
	}
	legacyAuthorshipID, err := legacy.Authorships.Create(legacyNewAuthorship)
	if err != nil {
		t.Fatal(err)
	}
	familyAuthorshipID, err := store.CreateAuthorship(ctx, familyNewAuthorship)
	if err != nil {
		t.Fatal(err)
	}
	if legacyAuthorshipID != familyAuthorshipID {
		t.Fatalf("CreateAuthorship id parity: legacy=%d family=%d", legacyAuthorshipID, familyAuthorshipID)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, work_revision_id, author_occurrence_id, author_order, affiliation FROM authorships WHERE id=?", legacyAuthorshipID)

	// Resolution and candidate write parity.
	legacyResolutionID, err := legacy.IdentityResolutions.Create(&database.AuthorIdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: legacyOccurrenceID, Status: database.AuthorIdentityStatusProviderFailed,
		Provider: "orcid", QueriedCitationName: "Parity, New", ErrorMessage: "provider failed", ResolvedAt: "2026-07-21T13:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	familyResolutionID, err := store.CreateIdentityResolution(ctx, &author.IdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: familyOccurrenceID, Status: author.IdentityStatusProviderFailed,
		Provider: "orcid", QueriedCitationName: "Parity, New", ErrorMessage: "provider failed", ResolvedAt: "2026-07-21T13:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyResolutionID != familyResolutionID {
		t.Fatalf("CreateIdentityResolution id parity: legacy=%d family=%d", legacyResolutionID, familyResolutionID)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, pipeline_run_id, author_occurrence_id, status, provider, queried_citation_name, error_message, resolved_at FROM author_identity_resolutions WHERE id=?", legacyResolutionID)

	legacyCandidateID, err := legacy.IdentityCandidates.Create(&database.AuthorIdentityCandidate{
		IdentityResolutionID: legacyResolutionID, CandidateORCID: "0000-0002-1825-0097",
		ProviderDisplayName: "ORCID", QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1825-0097", ProviderRank: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	familyCandidateID, err := store.CreateIdentityCandidate(ctx, &author.IdentityCandidate{
		IdentityResolutionID: familyResolutionID, CandidateORCID: "0000-0002-1825-0097",
		ProviderDisplayName: "ORCID", QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1825-0097", ProviderRank: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyCandidateID != familyCandidateID {
		t.Fatalf("CreateIdentityCandidate id parity: legacy=%d family=%d", legacyCandidateID, familyCandidateID)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, identity_resolution_id, candidate_orcid, provider_display_name, query_url, provider_rank FROM author_identity_candidates WHERE id=?", legacyCandidateID)

	// Error parity for invalid input.
	assertErrorParity(t, "empty ORCID",
		legacyPersonError(legacy, ""),
		familyPersonError(ctx, store, ""))
	assertErrorParity(t, "invalid ORCID",
		legacyPersonError(legacy, "not-an-orcid"),
		familyPersonError(ctx, store, "not-an-orcid"))
	assertErrorParity(t, "empty citation name",
		legacyOccurrenceError(legacy, &database.AuthorOccurrence{}),
		familyOccurrenceError(ctx, store, &author.Occurrence{}))
	assertErrorParity(t, "missing authorship revision",
		legacyAuthorshipError(legacy, &database.Authorship{AuthorOccurrenceID: 1, AuthorOrder: 1}),
		familyAuthorshipError(ctx, store, &author.Authorship{AuthorOccurrenceID: 1, AuthorOrder: 1}))
	assertErrorParity(t, "invalid authorship order",
		legacyAuthorshipError(legacy, &database.Authorship{WorkRevisionID: 1, AuthorOccurrenceID: 1, AuthorOrder: 0}),
		familyAuthorshipError(ctx, store, &author.Authorship{WorkRevisionID: 1, AuthorOccurrenceID: 1, AuthorOrder: 0}))
	assertErrorParity(t, "invalid identity status",
		legacyResolutionError(legacy, &database.AuthorIdentityResolution{
			PipelineRunID: 1, AuthorOccurrenceID: 1, Status: "bogus", Provider: "orcid",
			QueriedCitationName: "x", ResolvedAt: "2026-07-21T12:00:00Z",
		}),
		familyResolutionError(ctx, store, &author.IdentityResolution{
			PipelineRunID: 1, AuthorOccurrenceID: 1, Status: "bogus", Provider: "orcid",
			QueriedCitationName: "x", ResolvedAt: "2026-07-21T12:00:00Z",
		}))
	assertErrorParity(t, "invalid candidate rank",
		legacyCandidateError(legacy, &database.AuthorIdentityCandidate{
			IdentityResolutionID: 1, CandidateORCID: "0000-0002-1694-233X",
			QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1694-233X", ProviderRank: 0,
		}),
		familyCandidateError(ctx, store, &author.IdentityCandidate{
			IdentityResolutionID: 1, CandidateORCID: "0000-0002-1694-233X",
			QueryURL: "https://pub.orcid.org/v3.0/0000-0002-1694-233X", ProviderRank: 0,
		}))
}

// legacyPersonError attempts one invalid person create and returns only its error.
func legacyPersonError(db *database.Database, orcid string) error {
	_, err := db.People.CreateByORCID(orcid)
	return err
}

// familyPersonError attempts one invalid person create and returns only its error.
func familyPersonError(ctx context.Context, store *author.Store, orcid string) error {
	_, err := store.CreatePersonByORCID(ctx, orcid)
	return err
}

// legacyOccurrenceError attempts one invalid occurrence create and returns only its error.
func legacyOccurrenceError(db *database.Database, occurrence *database.AuthorOccurrence) error {
	_, err := db.AuthorOccs.Create(occurrence)
	return err
}

// familyOccurrenceError attempts one invalid occurrence create and returns only its error.
func familyOccurrenceError(ctx context.Context, store *author.Store, occurrence *author.Occurrence) error {
	_, err := store.CreateOccurrence(ctx, occurrence)
	return err
}

// legacyAuthorshipError attempts one invalid authorship create and returns only its error.
func legacyAuthorshipError(db *database.Database, authorship *database.Authorship) error {
	_, err := db.Authorships.Create(authorship)
	return err
}

// familyAuthorshipError attempts one invalid authorship create and returns only its error.
func familyAuthorshipError(ctx context.Context, store *author.Store, authorship *author.Authorship) error {
	_, err := store.CreateAuthorship(ctx, authorship)
	return err
}

// legacyResolutionError attempts one invalid resolution create and returns only its error.
func legacyResolutionError(db *database.Database, resolution *database.AuthorIdentityResolution) error {
	_, err := db.IdentityResolutions.Create(resolution)
	return err
}

// familyResolutionError attempts one invalid resolution create and returns only its error.
func familyResolutionError(ctx context.Context, store *author.Store, resolution *author.IdentityResolution) error {
	_, err := store.CreateIdentityResolution(ctx, resolution)
	return err
}

// legacyCandidateError attempts one invalid candidate create and returns only its error.
func legacyCandidateError(db *database.Database, candidate *database.AuthorIdentityCandidate) error {
	_, err := db.IdentityCandidates.Create(candidate)
	return err
}

// familyCandidateError attempts one invalid candidate create and returns only its error.
func familyCandidateError(ctx context.Context, store *author.Store, candidate *author.IdentityCandidate) error {
	_, err := store.CreateIdentityCandidate(ctx, candidate)
	return err
}

// assertPersonParity compares legacy and family person projections.
func assertPersonParity(t *testing.T, operation string, legacy *database.Person, family *author.Person) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.ORCID != family.ORCID || legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertOccurrenceParity compares legacy and family occurrence projections.
func assertOccurrenceParity(t *testing.T, operation string, legacy *database.AuthorOccurrence, family *author.Occurrence) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.PersonID != family.PersonID || legacy.CitationName != family.CitationName ||
		legacy.FirstName != family.FirstName || legacy.LastName != family.LastName || legacy.ORCID != family.ORCID ||
		legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertAuthorshipParity compares legacy and family authorship projections.
func assertAuthorshipParity(t *testing.T, operation string, legacy *database.Authorship, family *author.Authorship) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.WorkRevisionID != family.WorkRevisionID ||
		legacy.AuthorOccurrenceID != family.AuthorOccurrenceID || legacy.AuthorOrder != family.AuthorOrder ||
		legacy.Affiliation != family.Affiliation || legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertQueryParity runs the same read on both databases and compares every returned row.
func assertQueryParity(t *testing.T, legacy, family *sql.DB, query string, args ...any) {
	t.Helper()
	legacyRows := queryRowValues(t, legacy, query, args...)
	familyRows := queryRowValues(t, family, query, args...)
	if !reflect.DeepEqual(legacyRows, familyRows) {
		t.Fatalf("query parity mismatch:\nlegacy=%v\nfamily=%v", legacyRows, familyRows)
	}
}

// queryRowValues renders every row of one query as ordered comparable text.
func queryRowValues(t *testing.T, db *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// copyDatabaseFile copies one checkpointed fixture into an independent working copy.
func copyDatabaseFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// assertErrorParity verifies legacy and family operations reject input with the same error text.
func assertErrorParity(t *testing.T, operation string, legacyErr, familyErr error) {
	t.Helper()
	if (legacyErr == nil) != (familyErr == nil) {
		t.Fatalf("%s error mismatch: legacy=%v family=%v", operation, legacyErr, familyErr)
	}
	if legacyErr != nil && legacyErr.Error() != familyErr.Error() {
		t.Fatalf("%s error text mismatch: legacy=%q family=%q", operation, legacyErr, familyErr)
	}
}
