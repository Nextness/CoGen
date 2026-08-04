// Integration tests for works, work revisions, and run work stages.
//go:build integration

package database

import (
	"testing"
	"time"
)

// TestWorkCreateByDOI verifies that inserting by DOI creates a work row
// and that the same normalized DOI returns the existing ID.
func TestWorkCreateByDOI(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Plain DOI
	doi := "10.1000/xyz123"
	id1, err := db.Works.CreateByDOI(doi)
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero work ID")
	}

	// Same plain DOI must return the same ID
	id2, err := db.Works.CreateByDOI(doi)
	if err != nil {
		t.Fatalf("CreateByDOI (duplicate): %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected duplicate work ID %d, got %d", id1, id2)
	}

	// Uppercase DOI must normalize to the same work
	id3, err := db.Works.CreateByDOI("10.1000/XYZ123")
	if err != nil {
		t.Fatalf("CreateByDOI (uppercase): %v", err)
	}
	if id3 != id1 {
		t.Fatalf("uppercase DOI must resolve to same work ID %d, got %d", id1, id3)
	}

	// URL-prefixed DOI must normalize to the same work
	id4, err := db.Works.CreateByDOI("https://doi.org/10.1000/xyz123")
	if err != nil {
		t.Fatalf("CreateByDOI (URL): %v", err)
	}
	if id4 != id1 {
		t.Fatalf("URL-prefixed DOI must resolve to same work ID %d, got %d", id1, id4)
	}

	// Verify stored DOI is normalized
	w, err := db.Works.GetByID(id1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if w.DOI != "10.1000/xyz123" {
		t.Fatalf("expected normalized DOI %q, got %q", "10.1000/xyz123", w.DOI)
	}
}

// TestWorkEmptyDOIReturnsError verifies that CreateByDOI with an empty string fails.
func TestWorkEmptyDOIReturnsError(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.Works.CreateByDOI("")
	if err == nil {
		t.Fatal("expected error for empty DOI")
	}
}

// TestWorkCreateWithoutDOI verifies that title-only records get distinct work rows.
func TestWorkCreateWithoutDOI(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	id1, err := db.Works.CreateWithoutDOI()
	if err != nil {
		t.Fatalf("CreateWithoutDOI: %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero work ID")
	}

	// Second call creates a separate row (no global merge for uncertain records)
	id2, err := db.Works.CreateWithoutDOI()
	if err != nil {
		t.Fatalf("CreateWithoutDOI (second): %v", err)
	}
	if id2 == id1 {
		t.Fatal("title-only records must not be globally merged; expected distinct IDs")
	}

	// Verify both exist
	w1, err := db.Works.GetByID(id1)
	if err != nil {
		t.Fatalf("GetByID(%d): %v", id1, err)
	}
	if w1 == nil {
		t.Fatalf("work %d not found", id1)
	}
	if w1.DOI != "" {
		t.Fatalf("expected empty DOI for title-only work, got %q", w1.DOI)
	}

	w2, err := db.Works.GetByID(id2)
	if err != nil {
		t.Fatalf("GetByID(%d): %v", id2, err)
	}
	if w2 == nil {
		t.Fatalf("work %d not found", id2)
	}
}

// TestWorkGetByIDAndDOI verifies GetByID and GetByDOI lookups.
func TestWorkGetByIDAndDOI(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	doi := "10.1000/test-work-lookup"
	id, err := db.Works.CreateByDOI(doi)
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	// GetByID
	w, err := db.Works.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if w == nil {
		t.Fatal("work not found by ID")
	}
	if w.DOI != doi {
		t.Fatalf("expected DOI %q, got %q", doi, w.DOI)
	}
	if w.ID != id {
		t.Fatalf("expected ID %d, got %d", id, w.ID)
	}

	// GetByDOI
	w2, err := db.Works.GetByDOI(doi)
	if err != nil {
		t.Fatalf("GetByDOI: %v", err)
	}
	if w2 == nil {
		t.Fatal("work not found by DOI")
	}
	if w2.ID != id {
		t.Fatalf("expected ID %d, got %d", id, w2.ID)
	}

	// GetByDOI for non-existent DOI
	w3, err := db.Works.GetByDOI("10.9999/nonexistent")
	if err != nil {
		t.Fatalf("GetByDOI (missing): %v", err)
	}
	if w3 != nil {
		t.Fatal("expected nil for non-existent DOI")
	}

	// GetByDOI with URL-prefixed DOI must resolve to the same work
	w4, err := db.Works.GetByDOI("https://doi.org/10.1000/test-work-lookup")
	if err != nil {
		t.Fatalf("GetByDOI (URL): %v", err)
	}
	if w4 == nil {
		t.Fatal("work not found by URL-prefixed DOI")
	}
	if w4.ID != id {
		t.Fatalf("URL-prefixed DOI must resolve to work ID %d, got %d", id, w4.ID)
	}

	// GetByDOI with uppercase DOI must resolve to the same work
	w5, err := db.Works.GetByDOI("10.1000/TEST-WORK-LOOKUP")
	if err != nil {
		t.Fatalf("GetByDOI (uppercase): %v", err)
	}
	if w5 == nil {
		t.Fatal("work not found by uppercase DOI")
	}
	if w5.ID != id {
		t.Fatalf("uppercase DOI must resolve to work ID %d, got %d", id, w5.ID)
	}
}

// TestWorkListByIDs verifies batch lookup of works.
func TestWorkListByIDs(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	id1, _ := db.Works.CreateByDOI("10.1000/list-test-1")
	id2, _ := db.Works.CreateByDOI("10.1000/list-test-2")
	db.Works.CreateWithoutDOI()

	works, err := db.Works.ListByIDs([]int64{id1, id2})
	if err != nil {
		t.Fatalf("ListByIDs: %v", err)
	}
	if len(works) != 2 {
		t.Fatalf("expected 2 works, got %d", len(works))
	}
	if works[0].ID != id1 || works[1].ID != id2 {
		t.Fatalf("expected IDs [%d, %d], got [%d, %d]", id1, id2, works[0].ID, works[1].ID)
	}

	// Empty list
	empty, err := db.Works.ListByIDs(nil)
	if err != nil {
		t.Fatalf("ListByIDs (nil): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 works for nil list, got %d", len(empty))
	}
}

// TestWorkCount verifies the count of works.
func TestWorkCount(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	n, err := db.Works.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 works initially, got %d", n)
	}

	_, _ = db.Works.CreateByDOI("10.1000/count-1")
	_, _ = db.Works.CreateByDOI("10.1000/count-2")
	_, _ = db.Works.CreateWithoutDOI()

	n, err = db.Works.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 works, got %d", n)
	}
}

// TestWorkIdentifierInsert verifies adding identifiers to a work.
func TestWorkIdentifierInsert(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/identifiers-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	// Insert identifier
	wiID, err := db.WorkIdentifiers.Insert(workID, "scopus", "2-s2.0-84912345678")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if wiID == 0 {
		t.Fatal("expected non-zero identifier ID")
	}

	// Duplicate (namespace, identifier) returns existing ID
	wiID2, err := db.WorkIdentifiers.Insert(workID, "scopus", "2-s2.0-84912345678")
	if err != nil {
		t.Fatalf("Insert (duplicate): %v", err)
	}
	if wiID2 != wiID {
		t.Fatalf("expected duplicate identifier ID %d, got %d", wiID, wiID2)
	}

	// Same namespace, different identifier is allowed
	wiID3, err := db.WorkIdentifiers.Insert(workID, "scopus", "2-s2.0-84987654321")
	if err != nil {
		t.Fatalf("Insert (different): %v", err)
	}
	if wiID3 == 0 || wiID3 == wiID {
		t.Fatal("expected different identifier ID for a different identifier")
	}
}

// TestWorkIdentifierInsertOwnershipConflict verifies that inserting a
// (namespace, identifier) pair that already belongs to a different work
// returns an error.
func TestWorkIdentifierInsertOwnershipConflict(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	work1ID, err := db.Works.CreateByDOI("10.1000/ownership-work-1")
	if err != nil {
		t.Fatalf("CreateByDOI (work1): %v", err)
	}
	work2ID, err := db.Works.CreateByDOI("10.1000/ownership-work-2")
	if err != nil {
		t.Fatalf("CreateByDOI (work2): %v", err)
	}

	// Assign identifier to work1
	id1, err := db.WorkIdentifiers.Insert(work1ID, "scopus", "SCO-OWNERSHIP")
	if err != nil {
		t.Fatalf("Insert (work1): %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero identifier ID")
	}

	// Same (namespace, identifier) for work2 must fail
	_, err = db.WorkIdentifiers.Insert(work2ID, "scopus", "SCO-OWNERSHIP")
	if err == nil {
		t.Fatal("expected ownership conflict error")
	}
}

// TestWorkIdentifierEmptyArgs verifies that empty namespace or identifier fails.
func TestWorkIdentifierEmptyArgs(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/empty-id-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	_, err = db.WorkIdentifiers.Insert(workID, "", "val")
	if err == nil {
		t.Fatal("expected error for empty namespace")
	}

	_, err = db.WorkIdentifiers.Insert(workID, "ns", "")
	if err == nil {
		t.Fatal("expected error for empty identifier")
	}
}

// TestWorkIdentifierGetByWorkID verifies listing identifiers for a work.
func TestWorkIdentifierGetByWorkID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/list-ids-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	// Insert two identifiers
	_, _ = db.WorkIdentifiers.Insert(workID, "scopus", "SCO-123")
	_, _ = db.WorkIdentifiers.Insert(workID, "openalex", "W987654321")

	ids, err := db.WorkIdentifiers.GetByWorkID(workID)
	if err != nil {
		t.Fatalf("GetByWorkID: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 identifiers, got %d", len(ids))
	}
	if ids[0].Namespace != "scopus" || ids[1].Namespace != "openalex" {
		t.Fatalf("unexpected identifier order: scopus=%q, openalex=%q", ids[0].Namespace, ids[1].Namespace)
	}
	if ids[0].WorkID != workID || ids[1].WorkID != workID {
		t.Fatal("identifiers must reference the correct work")
	}

	// Non-existent work
	empty, err := db.WorkIdentifiers.GetByWorkID(99999)
	if err != nil {
		t.Fatalf("GetByWorkID (non-existent): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 identifiers for non-existent work, got %d", len(empty))
	}
}

// TestWorkIdentifierGetByNamespaceAndIdentifier verifies lookup by namespace+identifier.
func TestWorkIdentifierGetByNamespaceAndIdentifier(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/ns-id-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	_, err = db.WorkIdentifiers.Insert(workID, "wos", "WOS:0001234567")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	wi, err := db.WorkIdentifiers.GetByNamespaceAndIdentifier("wos", "WOS:0001234567")
	if err != nil {
		t.Fatalf("GetByNamespaceAndIdentifier: %v", err)
	}
	if wi == nil {
		t.Fatal("expected identifier to be found")
	}
	if wi.WorkID != workID {
		t.Fatalf("expected work_id %d, got %d", workID, wi.WorkID)
	}
	if wi.Namespace != "wos" {
		t.Fatalf("expected namespace 'wos', got %q", wi.Namespace)
	}

	// Non-existent pair
	missing, err := db.WorkIdentifiers.GetByNamespaceAndIdentifier("wos", "NONEXISTENT")
	if err != nil {
		t.Fatalf("GetByNamespaceAndIdentifier (missing): %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for non-existent identifier")
	}
}

// TestWorkIdentifierCountByWorkID verifies the identifier count for a work.
func TestWorkIdentifierCountByWorkID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/count-ids-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	n, err := db.WorkIdentifiers.CountByWorkID(workID)
	if err != nil {
		t.Fatalf("CountByWorkID: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 identifiers initially, got %d", n)
	}

	_, _ = db.WorkIdentifiers.Insert(workID, "scopus", "SCO-1")
	_, _ = db.WorkIdentifiers.Insert(workID, "openalex", "W1")

	n, err = db.WorkIdentifiers.CountByWorkID(workID)
	if err != nil {
		t.Fatalf("CountByWorkID: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 identifiers, got %d", n)
	}
}

// TestWorkIdentifierGetByID verifies lookup by primary key.
func TestWorkIdentifierGetByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/get-by-id-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	wiID, err := db.WorkIdentifiers.Insert(workID, "doi", "10.1000/alt")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	wi, err := db.WorkIdentifiers.GetByID(wiID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if wi == nil {
		t.Fatal("expected identifier to be found")
	}
	if wi.ID != wiID {
		t.Fatalf("expected ID %d, got %d", wiID, wi.ID)
	}
	if wi.WorkID != workID {
		t.Fatalf("expected work_id %d, got %d", workID, wi.WorkID)
	}

	// Non-existent
	missing, err := db.WorkIdentifiers.GetByID(99999)
	if err != nil {
		t.Fatalf("GetByID (non-existent): %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for non-existent identifier")
	}
}

// TestWorkSameDOIMultipleSearches verifies that the same DOI used across
// different searches always resolves to the same global work identity.
// This is the foundation for per-search membership in Phase 2.2.
func TestWorkSameDOIMultipleSearches(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	doi := "10.1000/shared-doi"

	// Simulate first search creating the work
	id1, err := db.Works.CreateByDOI(doi)
	if err != nil {
		t.Fatalf("CreateByDOI (first): %v", err)
	}

	// Simulate second search creating the same work
	id2, err := db.Works.CreateByDOI(doi)
	if err != nil {
		t.Fatalf("CreateByDOI (second): %v", err)
	}

	if id1 != id2 {
		t.Fatalf("same DOI must resolve to the same work ID: %d vs %d", id1, id2)
	}

	// Count should be 1, not 2
	n, err := db.Works.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 work for the same DOI, got %d", n)
	}
}

// TestWorkRevisionCreate verifies basic creation and retrieval of a work revision
// with producer_stage, field_schema_version defaulting, and payload hash computation.
func TestWorkRevisionCreate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/rev-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	runID, err := db.PipelineRuns.StartRun("test_step", "test query")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	rev := &WorkRevision{
		WorkID:        workID,
		PipelineRunID: runID,
		ProducerStage: "parse",
		Title:         "Test Revision",
		Year:          2023,
		Journal:       "Test Journal",
	}

	id, err := db.WorkRevisions.Create(rev)
	if err != nil {
		t.Fatalf("Create work revision: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero revision ID")
	}
	if rev.PayloadHash == "" {
		t.Fatal("expected PayloadHash to be set")
	}

	// Get by ID
	got, err := db.WorkRevisions.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("revision not found by ID")
	}
	if got.WorkID != workID {
		t.Fatalf("expected WorkID %d, got %d", workID, got.WorkID)
	}
	if got.PipelineRunID != runID {
		t.Fatalf("expected PipelineRunID %d, got %d", runID, got.PipelineRunID)
	}
	if got.ProducerStage != "parse" {
		t.Fatalf("expected ProducerStage %q, got %q", "parse", got.ProducerStage)
	}
	if got.Title != "Test Revision" {
		t.Fatalf("expected Title %q, got %q", "Test Revision", got.Title)
	}
	if got.Year != 2023 {
		t.Fatalf("expected Year 2023, got %d", got.Year)
	}
	if got.PayloadHash == "" {
		t.Fatal("expected PayloadHash to be populated")
	}
	if got.FieldSchemaVersion != "1" {
		t.Fatalf("expected default FieldSchemaVersion %q, got %q", "1", got.FieldSchemaVersion)
	}

	// Non-existent revision
	missing, err := db.WorkRevisions.GetByID(99999)
	if err != nil {
		t.Fatalf("GetByID (missing): %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for non-existent revision")
	}
}

// TestWorkRevisionRejectsEmptyProducerStage verifies that Create rejects
// a revision without a producer_stage.
func TestWorkRevisionRejectsEmptyProducerStage(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/no-stage")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, err = db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID,
		Title: "no stage",
	})
	if err == nil {
		t.Fatal("expected error for empty producer_stage")
	}
}

// TestWorkRevisionImmutability verifies that two revisions for the same work
// coexist without overwriting each other.
func TestWorkRevisionImmutability(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/immutable-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	runID1, err := db.PipelineRuns.StartRun("first_run", "q1")
	if err != nil {
		t.Fatalf("StartRun 1: %v", err)
	}
	runID2, err := db.PipelineRuns.StartRun("second_run", "q2")
	if err != nil {
		t.Fatalf("StartRun 2: %v", err)
	}

	rev1 := &WorkRevision{
		WorkID:        workID,
		PipelineRunID: runID1,
		ProducerStage: "parse",
		Title:         "First Revision",
		Year:          2020,
	}
	rev2 := &WorkRevision{
		WorkID:        workID,
		PipelineRunID: runID2,
		ProducerStage: "enrich",
		Title:         "Second Revision",
		Year:          2023,
	}

	id1, err := db.WorkRevisions.Create(rev1)
	if err != nil {
		t.Fatalf("Create revision 1: %v", err)
	}
	id2, err := db.WorkRevisions.Create(rev2)
	if err != nil {
		t.Fatalf("Create revision 2: %v", err)
	}

	// Both must exist with distinct IDs
	if id1 == id2 {
		t.Fatal("two revisions must have distinct IDs")
	}

	got1, err := db.WorkRevisions.GetByID(id1)
	if err != nil {
		t.Fatalf("GetByID(1): %v", err)
	}
	if got1 == nil || got1.Title != "First Revision" || got1.Year != 2020 || got1.ProducerStage != "parse" {
		t.Fatal("first revision was overwritten or not found")
	}

	got2, err := db.WorkRevisions.GetByID(id2)
	if err != nil {
		t.Fatalf("GetByID(2): %v", err)
	}
	if got2 == nil || got2.Title != "Second Revision" || got2.Year != 2023 || got2.ProducerStage != "enrich" {
		t.Fatal("second revision was not stored correctly")
	}

	// List by work ID
	revisions, err := db.WorkRevisions.GetByWorkID(workID)
	if err != nil {
		t.Fatalf("GetByWorkID: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revisions))
	}

	// Count
	n, err := db.WorkRevisions.CountByWorkID(workID)
	if err != nil {
		t.Fatalf("CountByWorkID: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected count 2, got %d", n)
	}
}

// TestWorkRevisionGetByRunID verifies listing revisions by pipeline run.
func TestWorkRevisionGetByRunID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID1, err := db.Works.CreateByDOI("10.1000/run1-a")
	if err != nil {
		t.Fatalf("CreateByDOI 1: %v", err)
	}
	workID2, err := db.Works.CreateByDOI("10.1000/run1-b")
	if err != nil {
		t.Fatalf("CreateByDOI 2: %v", err)
	}

	runID, err := db.PipelineRuns.StartRun("shared_run", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, err = db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID1, PipelineRunID: runID, ProducerStage: "parse", Title: "Work A",
	})
	if err != nil {
		t.Fatalf("Create revision A: %v", err)
	}
	_, err = db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID2, PipelineRunID: runID, ProducerStage: "parse", Title: "Work B",
	})
	if err != nil {
		t.Fatalf("Create revision B: %v", err)
	}

	revisions, err := db.WorkRevisions.GetByRunID(runID)
	if err != nil {
		t.Fatalf("GetByRunID: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions for run, got %d", len(revisions))
	}
}

// TestWorkRevisionAbortUpdate verifies that the append-only trigger on
// work_revisions rejects UPDATE statements.
func TestWorkRevisionAbortUpdate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/no-update")
	runID, _ := db.PipelineRuns.StartRun("trigger_test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: "parse",
		Title: "original",
	})

	_, err := db.DB.Exec("UPDATE work_revisions SET title = 'hacked' WHERE id = ?", revID)
	if err == nil {
		t.Fatal("expected error when updating an immutable work_revision")
	}
	// Verify the original is unchanged
	rev, _ := db.WorkRevisions.GetByID(revID)
	if rev.Title != "original" {
		t.Fatalf("expected title %q, got %q", "original", rev.Title)
	}
}

// TestWorkRevisionAbortDelete verifies that the append-only trigger on
// work_revisions rejects DELETE statements.
func TestWorkRevisionAbortDelete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/no-delete")
	runID, _ := db.PipelineRuns.StartRun("trigger_test", "q")
	revID, _ := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: "parse",
		Title: "permanent",
	})

	_, err := db.DB.Exec("DELETE FROM work_revisions WHERE id = ?", revID)
	if err == nil {
		t.Fatal("expected error when deleting an immutable work_revision")
	}
	// Verify the row still exists
	rev, _ := db.WorkRevisions.GetByID(revID)
	if rev == nil {
		t.Fatal("work_revision was deleted despite the trigger")
	}
}

// TestWorkRevisionRejectsUnknownProducerStage verifies work revision rejects unknown producer stage.
func TestWorkRevisionRejectsUnknownProducerStage(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/unknown-stage")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, err = db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: "bogus_stage",
		Title: "unknown stage",
	})
	if err == nil {
		t.Fatal("expected error for unknown producer_stage")
	}
}

// TestWorkRevisionRejectsLegacyUnknown verifies that legacy_unknown is rejected
// for new revisions (it is only for pre-existing migrated rows).
func TestWorkRevisionRejectsLegacyUnknown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/legacy-unknown-reject")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, err = db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageLegacyUnknown,
		Title: "legacy unknown",
	})
	if err == nil {
		t.Fatal("expected error for legacy_unknown producer_stage on new revision")
	}
}

// TestWorkRevisionAcceptsValidProducerStages verifies that all known pipeline
// stages are accepted as producer_stage.
func TestWorkRevisionAcceptsValidProducerStages(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/valid-stages")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	for _, stage := range []string{
		ProducerStageParse,
		ProducerStageDeduplicate,
		ProducerStageValidate,
		ProducerStageEnrich,
		ProducerStageEnrichMetadata,
		ProducerStageEnrichIdentity,
		ProducerStageNormalize,
	} {
		runID, err := db.PipelineRuns.StartRun("test", "q")
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		_, err = db.WorkRevisions.Create(&WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: stage,
			Title: "stage " + stage,
		})
		if err != nil {
			t.Fatalf("unexpected error for producer_stage %q: %v", stage, err)
		}
	}
}

// TestRunWorkStageSetOutcome verifies setting and getting stage outcomes.
func TestRunWorkStageSetOutcome(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/stage-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	runID, err := db.PipelineRuns.StartRun("stage_test_run", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Set parse outcome
	err = db.RunWorkStages.SetOutcome(runID, workID, "parse", "parsed", "")
	if err != nil {
		t.Fatalf("SetOutcome (parse): %v", err)
	}

	// Set validate outcome with reason
	err = db.RunWorkStages.SetOutcome(runID, workID, "validate", "valid", "")
	if err != nil {
		t.Fatalf("SetOutcome (validate): %v", err)
	}

	// Get parse outcome
	stage, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "parse")
	if err != nil {
		t.Fatalf("GetByRunAndWork (parse): %v", err)
	}
	if stage == nil {
		t.Fatal("expected parse stage to exist")
	}
	if stage.Outcome != "parsed" {
		t.Fatalf("expected outcome %q, got %q", "parsed", stage.Outcome)
	}
	if stage.StageName != "parse" {
		t.Fatalf("expected stage_name %q, got %q", "parse", stage.StageName)
	}

	// Get validate outcome
	stage2, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork (validate): %v", err)
	}
	if stage2 == nil {
		t.Fatal("expected validate stage to exist")
	}
	if stage2.Outcome != "valid" {
		t.Fatalf("expected outcome %q, got %q", "valid", stage2.Outcome)
	}

	// Non-existent stage
	missing, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "nonexistent")
	if err != nil {
		t.Fatalf("GetByRunAndWork (missing): %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for non-existent stage")
	}
}

// TestRunWorkStageReplaceOutcome verifies that INSERT OR REPLACE updates
// an existing stage outcome (e.g., moving from "pending" to "parsed").
func TestRunWorkStageReplaceOutcome(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/replace-test")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	runID, err := db.PipelineRuns.StartRun("replace_test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Set initial
	err = db.RunWorkStages.SetOutcome(runID, workID, "deduplicate", "pending", "")
	if err != nil {
		t.Fatalf("SetOutcome (pending): %v", err)
	}

	// Replace with new outcome
	err = db.RunWorkStages.SetOutcome(runID, workID, "deduplicate", "deduplicated", "")
	if err != nil {
		t.Fatalf("SetOutcome (deduplicated): %v", err)
	}

	stage, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "deduplicate")
	if err != nil {
		t.Fatalf("GetByRunAndWork: %v", err)
	}
	if stage == nil {
		t.Fatal("expected stage to exist")
	}
	if stage.Outcome != "deduplicated" {
		t.Fatalf("expected outcome %q, got %q", "deduplicated", stage.Outcome)
	}

	// Verify only one row exists
	stages, err := db.RunWorkStages.GetByRunID(runID)
	if err != nil {
		t.Fatalf("GetByRunID: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage row after replace, got %d", len(stages))
	}
}

// TestRunWorkStageCountByStageAndOutcome verifies funnel counting.
func TestRunWorkStageCountByStageAndOutcome(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("count_test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Create two works with different outcomes
	w1, _ := db.Works.CreateByDOI("10.1000/count-a")
	w2, _ := db.Works.CreateByDOI("10.1000/count-b")
	w3, _ := db.Works.CreateByDOI("10.1000/count-c")

	_ = db.RunWorkStages.SetOutcome(runID, w1, "validate", "valid", "")
	_ = db.RunWorkStages.SetOutcome(runID, w2, "validate", "valid", "")
	_ = db.RunWorkStages.SetOutcome(runID, w3, "validate", "discarded", "missing title")

	validCount, err := db.RunWorkStages.CountByStageAndOutcome(runID, "validate", "valid")
	if err != nil {
		t.Fatalf("CountByStageAndOutcome (valid): %v", err)
	}
	if validCount != 2 {
		t.Fatalf("expected 2 valid works, got %d", validCount)
	}

	discardedCount, err := db.RunWorkStages.CountByStageAndOutcome(runID, "validate", "discarded")
	if err != nil {
		t.Fatalf("CountByStageAndOutcome (discarded): %v", err)
	}
	if discardedCount != 1 {
		t.Fatalf("expected 1 discarded work, got %d", discardedCount)
	}
}

// TestRunWorkStageCrossRunScoping verifies that two runs can record different
// stage outcomes for the same work without interfering with each other, and
// that revisions from different runs are independently stored.
func TestRunWorkStageCrossRunScoping(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/two-runs")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}

	// Run 1 — records a "valid" validation outcome for the work
	runID1, err := db.PipelineRuns.StartRun("first_run", "q1")
	if err != nil {
		t.Fatalf("StartRun 1: %v", err)
	}
	revID1, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID1, ProducerStage: "parse",
		Title: "Run 1 Title", Year: 2020,
	})
	if err != nil {
		t.Fatalf("Create revision 1: %v", err)
	}
	err = db.RunWorkStages.SetOutcome(runID1, workID, "validate", "valid", "")
	if err != nil {
		t.Fatalf("SetOutcome 1: %v", err)
	}

	// Run 2 — same work, different revision, different validation outcome
	runID2, err := db.PipelineRuns.StartRun("second_run", "q2")
	if err != nil {
		t.Fatalf("StartRun 2: %v", err)
	}
	revID2, err := db.WorkRevisions.Create(&WorkRevision{
		WorkID: workID, PipelineRunID: runID2, ProducerStage: "parse",
		Title: "Run 2 Title", Year: 2023,
	})
	if err != nil {
		t.Fatalf("Create revision 2: %v", err)
	}
	err = db.RunWorkStages.SetOutcome(runID2, workID, "validate", "discarded", "different reason")
	if err != nil {
		t.Fatalf("SetOutcome 2: %v", err)
	}

	// Verify both revisions exist and are unchanged
	rev1, err := db.WorkRevisions.GetByID(revID1)
	if err != nil {
		t.Fatalf("GetByID 1: %v", err)
	}
	if rev1 == nil || rev1.Title != "Run 1 Title" || rev1.Year != 2020 || rev1.PipelineRunID != runID1 {
		t.Fatal("first revision was modified or is missing")
	}

	rev2, err := db.WorkRevisions.GetByID(revID2)
	if err != nil {
		t.Fatalf("GetByID 2: %v", err)
	}
	if rev2 == nil || rev2.Title != "Run 2 Title" || rev2.Year != 2023 || rev2.PipelineRunID != runID2 {
		t.Fatal("second revision is incorrect")
	}

	// Verify run 1 stage outcomes are not affected by run 2
	stage1, err := db.RunWorkStages.GetByRunAndWork(runID1, workID, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork 1: %v", err)
	}
	if stage1 == nil || stage1.Outcome != "valid" {
		t.Fatal("run 1 validate outcome was modified by run 2")
	}

	stage2, err := db.RunWorkStages.GetByRunAndWork(runID2, workID, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork 2: %v", err)
	}
	if stage2 == nil || stage2.Outcome != "discarded" {
		t.Fatal("run 2 validate outcome is incorrect")
	}

	// run 1 must have exactly 1 stage row (validate), not 2
	run1Stages, err := db.RunWorkStages.GetByRunID(runID1)
	if err != nil {
		t.Fatalf("GetByRunID 1: %v", err)
	}
	if len(run1Stages) != 1 {
		t.Fatalf("run 1 should have 1 stage, got %d", len(run1Stages))
	}
	if run1Stages[0].Outcome != "valid" {
		t.Fatal("run 1 stage outcome was affected by run 2")
	}
}

// TestRunWorkStageCrossWorkScoping verifies that two different works in the
// same run can record different validation reasons without cross-contamination.
func TestRunWorkStageCrossWorkScoping(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workA, err := db.Works.CreateByDOI("10.1000/work-a")
	if err != nil {
		t.Fatalf("CreateByDOI A: %v", err)
	}
	workB, err := db.Works.CreateByDOI("10.1000/work-b")
	if err != nil {
		t.Fatalf("CreateByDOI B: %v", err)
	}

	runID, err := db.PipelineRuns.StartRun("cross_work_test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Same run, two works, different validation reasons
	err = db.RunWorkStages.SetOutcome(runID, workA, "validate", "discarded", "missing title")
	if err != nil {
		t.Fatalf("SetOutcome A: %v", err)
	}
	err = db.RunWorkStages.SetOutcome(runID, workB, "validate", "discarded", "no authors")
	if err != nil {
		t.Fatalf("SetOutcome B: %v", err)
	}

	stageA, err := db.RunWorkStages.GetByRunAndWork(runID, workA, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork A: %v", err)
	}
	if stageA == nil || stageA.Reason != "missing title" {
		t.Fatalf("work A expected reason %q, got %q", "missing title", stageA.Reason)
	}

	stageB, err := db.RunWorkStages.GetByRunAndWork(runID, workB, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork B: %v", err)
	}
	if stageB == nil || stageB.Reason != "no authors" {
		t.Fatalf("work B expected reason %q, got %q", "no authors", stageB.Reason)
	}

	// Verify counts
	total, err := db.RunWorkStages.CountByStageAndOutcome(runID, "validate", "discarded")
	if err != nil {
		t.Fatalf("CountByStageAndOutcome: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 discarded works, got %d", total)
	}
}

// TestRunWorkStageInvalidStageName verifies SetOutcome rejects unknown stages.
func TestRunWorkStageInvalidStageName(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/bad-stage")
	runID, _ := db.PipelineRuns.StartRun("validation test", "q")

	err := db.RunWorkStages.SetOutcome(runID, workID, "nonexistent_stage", "parsed", "")
	if err == nil {
		t.Fatal("expected error for invalid stage name")
	}
}

// TestRunWorkStageInvalidOutcome verifies SetOutcome rejects unknown outcomes.
func TestRunWorkStageInvalidOutcome(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/bad-outcome")
	runID, _ := db.PipelineRuns.StartRun("validation test", "q")

	err := db.RunWorkStages.SetOutcome(runID, workID, "parse", "doesnotexist", "")
	if err == nil {
		t.Fatal("expected error for invalid outcome")
	}
}

// TestRunWorkStageEmptyStageName verifies SetOutcome rejects empty stage name.
func TestRunWorkStageEmptyStageName(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/empty-stage")
	runID, _ := db.PipelineRuns.StartRun("validation test", "q")

	err := db.RunWorkStages.SetOutcome(runID, workID, "", "parsed", "")
	if err == nil {
		t.Fatal("expected error for empty stage name")
	}
}

// TestRunWorkStageEmptyOutcome verifies SetOutcome rejects empty outcome.
func TestRunWorkStageEmptyOutcome(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/empty-outcome")
	runID, _ := db.PipelineRuns.StartRun("validation test", "q")

	err := db.RunWorkStages.SetOutcome(runID, workID, "parse", "", "")
	if err == nil {
		t.Fatal("expected error for empty outcome")
	}
}

// TestRunWorkStageReplacePreservesIdentity verifies that ON CONFLICT DO UPDATE
// preserves the original created_at and row ID across progressive updates,
// and that updated_at is set to a later timestamp.
func TestRunWorkStageReplacePreservesIdentity(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, _ := db.Works.CreateByDOI("10.1000/identity-test")
	runID, _ := db.PipelineRuns.StartRun("identity test", "q")

	// Set initial
	err := db.RunWorkStages.SetOutcome(runID, workID, "deduplicate", OutcomePending, "")
	if err != nil {
		t.Fatalf("SetOutcome (pending): %v", err)
	}

	first, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "deduplicate")
	if err != nil {
		t.Fatalf("GetByRunAndWork: %v", err)
	}
	if first == nil {
		t.Fatal("expected stage to exist")
	}

	firstID := first.ID
	firstCreatedAt := first.CreatedAt
	if first.UpdatedAt == "" {
		t.Fatal("updated_at must be populated on initial insert")
	}

	// Replace with new outcome
	err = db.RunWorkStages.SetOutcome(runID, workID, "deduplicate", OutcomeDeduplicated, "")
	if err != nil {
		t.Fatalf("SetOutcome (deduplicated): %v", err)
	}

	second, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "deduplicate")
	if err != nil {
		t.Fatalf("GetByRunAndWork 2: %v", err)
	}
	if second == nil {
		t.Fatal("expected stage to still exist")
	}

	// Row identity must be preserved
	if second.ID != firstID {
		t.Fatalf("row ID changed: was %d, now %d", firstID, second.ID)
	}
	// created_at must be preserved (it is the first time the stage was set)
	if second.CreatedAt != firstCreatedAt {
		t.Fatalf("created_at changed: was %q, now %q", firstCreatedAt, second.CreatedAt)
	}
	// updated_at must be set (it is the last time the outcome changed)
	if second.UpdatedAt == "" {
		t.Fatal("updated_at must be populated after update")
	}
	if second.UpdatedAt < second.CreatedAt {
		t.Fatal("updated_at must not be before created_at")
	}

	// Outcome must be the new value
	if second.Outcome != OutcomeDeduplicated {
		t.Fatalf("expected outcome %q, got %q", OutcomeDeduplicated, second.Outcome)
	}

	// Verify only one row exists
	stages, err := db.RunWorkStages.GetByRunID(runID)
	if err != nil {
		t.Fatalf("GetByRunID: %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage row after replacement, got %d", len(stages))
	}
}

// TestRunWorkStageInvalidCombination verifies that impossible stage/outcome
// pairs are rejected (e.g. parse/valid, validate/enriched).
func TestRunWorkStageInvalidCombination(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/bad-combo")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	tests := []struct {
		stage   string
		outcome string
	}{
		{StageNameParse, OutcomeValid},
		{StageNameValidate, OutcomeEnriched},
		{StageNameNormalize, OutcomeDeduplicated},
		{StageNameEnrich, OutcomeParsed},
		{StageNameDeduplicate, OutcomeNormalized},
	}
	for _, tc := range tests {
		err := db.RunWorkStages.SetOutcome(runID, workID, tc.stage, tc.outcome, "")
		if err == nil {
			t.Fatalf("expected error for stage %q / outcome %q", tc.stage, tc.outcome)
		}
	}
}

// TestRunWorkStageValidCombinations verifies that all valid stage/outcome
// pairs are accepted.  There are 17 pairs: 3 + 4 + 4 + 3 + 3.
func TestRunWorkStageValidCombinations(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	valid := map[string][]string{
		StageNameParse:          {OutcomeParsed, OutcomeSkipped, OutcomePending},
		StageNameDeduplicate:    {OutcomeDuplicate, OutcomeDeduplicated, OutcomeSkipped, OutcomePending},
		StageNameValidate:       {OutcomeValid, OutcomeDiscarded, OutcomeSkipped, OutcomePending},
		StageNameEnrich:         {OutcomeEnriched, OutcomeSkipped, OutcomePending},
		StageNameEnrichMetadata: {OutcomeEnriched, OutcomeSkipped, OutcomePending},
		StageNameEnrichIdentity: {OutcomeEnriched, OutcomeFailed, OutcomeSkipped, OutcomePending},
		StageNameNormalize:      {OutcomeNormalized, OutcomeSkipped, OutcomePending},
	}
	for stage, outcomes := range valid {
		for _, outcome := range outcomes {
			// Use a unique work per pair to avoid UNIQUE constraint conflicts
			wID, err := db.Works.CreateByDOI("10.1000/good-combo-" + stage + "-" + outcome)
			if err != nil {
				t.Fatalf("CreateByDOI: %v", err)
			}
			err = db.RunWorkStages.SetOutcome(runID, wID, stage, outcome, "")
			if err != nil {
				t.Fatalf("unexpected error for stage %q / outcome %q: %v", stage, outcome, err)
			}
		}
	}
}

// TestRunWorkStageUpdatedAtProgression verifies that updated_at advances when
// SetOutcome progressively replaces an outcome.
func TestRunWorkStageUpdatedAtProgression(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	workID, err := db.Works.CreateByDOI("10.1000/updated-at-progress")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("updated_at test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Set initial outcome and capture the timestamp
	err = db.RunWorkStages.SetOutcome(runID, workID, "validate", OutcomePending, "")
	if err != nil {
		t.Fatalf("SetOutcome (pending): %v", err)
	}
	first, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork: %v", err)
	}
	if first == nil {
		t.Fatal("expected stage to exist")
	}
	firstUpdatedAt := first.UpdatedAt
	if firstUpdatedAt == "" {
		t.Fatal("updated_at must be populated on initial insert")
	}

	// Wait at least one second so the timestamp has a chance to differ
	time.Sleep(1 * time.Second)

	// Replace outcome
	err = db.RunWorkStages.SetOutcome(runID, workID, "validate", OutcomeValid, "progressive")
	if err != nil {
		t.Fatalf("SetOutcome (valid): %v", err)
	}
	second, err := db.RunWorkStages.GetByRunAndWork(runID, workID, "validate")
	if err != nil {
		t.Fatalf("GetByRunAndWork: %v", err)
	}
	if second == nil {
		t.Fatal("expected stage to exist after update")
	}

	// Row identity must be preserved
	if second.ID != first.ID {
		t.Fatalf("row ID changed: was %d, now %d", first.ID, second.ID)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatal("created_at must not change across SetOutcome")
	}
	// updated_at must advance
	if second.UpdatedAt <= firstUpdatedAt {
		t.Fatal("updated_at must advance across progressive SetOutcome")
	}
}
