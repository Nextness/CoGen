// Integration tests for search and search revision repositories.
//go:build integration

package database

import (
	"testing"
)

// TestSearchCreateAndGet verifies search create and get.
func TestSearchCreateAndGet(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	id, err := db.Searches.Create("test-search")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	got, err := db.Searches.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("search not found by id")
	}
	if got.SearchID != "test-search" {
		t.Errorf("expected search_id 'test-search', got %q", got.SearchID)
	}

	got2, err := db.Searches.GetBySearchID("test-search")
	if err != nil {
		t.Fatalf("GetBySearchID: %v", err)
	}
	if got2 == nil {
		t.Fatal("search not found by search_id")
	}
	if got2.ID != id {
		t.Errorf("expected id %d, got %d", id, got2.ID)
	}
}

// TestSearchCreateDuplicateReturnsSameID verifies search create duplicate returns same id.
func TestSearchCreateDuplicateReturnsSameID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	id1, err := db.Searches.Create("dup-search")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	id2, err := db.Searches.Create("dup-search")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	if id2 != id1 {
		t.Errorf("expected same id %d, got %d", id1, id2)
	}
}

// TestSearchCreateDistinctIDs verifies search creation returns distinct IDs.
func TestSearchCreateDistinctIDs(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	id1, _ := db.Searches.Create("search-a")
	id2, _ := db.Searches.Create("search-b")

	if id1 == id2 {
		t.Error("expected different ids for distinct searches")
	}
}

// TestSearchList verifies search list.
func TestSearchList(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	db.Searches.Create("search-a")
	db.Searches.Create("search-b")
	db.Searches.Create("search-c")

	all, err := db.Searches.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 searches, got %d", len(all))
	}
}

// TestSearchRevisionCreateAndLookup verifies search revision create and lookup.
func TestSearchRevisionCreateAndLookup(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("revision-test")

	revID, _, err := db.Revisions.Create(searchID, "v1", "config-hash-abc", "manifest-hash-xyz")
	if err != nil {
		t.Fatalf("Create revision: %v", err)
	}
	if revID == 0 {
		t.Fatal("expected non-zero revision id")
	}

	got, err := db.Revisions.GetByID(revID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("revision not found by id")
	}
	if got.RevisionLabel != "v1" {
		t.Errorf("expected revision_label 'v1', got %q", got.RevisionLabel)
	}
	if got.ConfigArtifactHash != "config-hash-abc" {
		t.Errorf("expected config_artifact_hash 'config-hash-abc', got %q", got.ConfigArtifactHash)
	}
	if got.ResolvedManifestHash != "manifest-hash-xyz" {
		t.Errorf("expected resolved_manifest_hash 'manifest-hash-xyz', got %q", got.ResolvedManifestHash)
	}

	got2, err := db.Revisions.GetBySearchAndRevision(searchID, "v1")
	if err != nil {
		t.Fatalf("GetBySearchAndRevision: %v", err)
	}
	if got2 == nil {
		t.Fatal("revision not found by search+revision")
	}
	if got2.ID != revID {
		t.Errorf("expected id %d, got %d", revID, got2.ID)
	}
}

// TestSearchRevisionDuplicateSameHashReturnsSameID verifies search revision duplicate same hash returns same id.
func TestSearchRevisionDuplicateSameHashReturnsSameID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("dup-revision-test")

	revID1, _, err := db.Revisions.Create(searchID, "v1", "hash-a", "hash-b")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	revID2, _, err := db.Revisions.Create(searchID, "v1", "hash-a", "hash-b")
	if err != nil {
		t.Fatalf("second Create with same hashes: %v", err)
	}

	if revID2 != revID1 {
		t.Errorf("expected same id %d, got %d", revID1, revID2)
	}
}

// TestSearchRevisionDuplicateDifferentHashUpdated verifies search revision duplicate different hash updated.
func TestSearchRevisionDuplicateDifferentHashUpdated(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("dup-revision-hash-test")

	id1, _, err := db.Revisions.Create(searchID, "v1", "hash-a", "hash-b")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	id2, updated, err := db.Revisions.Create(searchID, "v1", "hash-c", "hash-d")
	if err != nil {
		t.Fatalf("second Create with different hashes should succeed, got: %v", err)
	}
	if id2 != id1 {
		t.Errorf("expected same id %d, got %d", id1, id2)
	}
	if !updated {
		t.Fatal("expected updated=true for different hashes")
	}

	// Verify the hashes were actually updated in the DB
	rev, err := db.Revisions.GetByID(id1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if rev.ConfigArtifactHash != "hash-c" {
		t.Errorf("expected config hash %q, got %q", "hash-c", rev.ConfigArtifactHash)
	}
	if rev.ResolvedManifestHash != "hash-d" {
		t.Errorf("expected manifest hash %q, got %q", "hash-d", rev.ResolvedManifestHash)
	}
	if rev.UpdatedAt == "" {
		t.Error("expected updated_at to be set")
	}
}

// TestSearchRevisionDistinctLabels verifies search revision distinct labels.
func TestSearchRevisionDistinctLabels(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("multi-revision")

	rev1, _, _ := db.Revisions.Create(searchID, "v1", "hash-a", "hash-a")
	rev2, _, _ := db.Revisions.Create(searchID, "v2", "hash-b", "hash-b")

	if rev1 == rev2 {
		t.Error("expected different revision ids for distinct labels")
	}

	revisions, err := db.Revisions.ListBySearch(searchID)
	if err != nil {
		t.Fatalf("ListBySearch: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revisions))
	}
}
