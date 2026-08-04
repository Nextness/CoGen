// Integration tests for artifact and run-artifact link repositories.
//go:build integration

package database

import (
	"testing"
)

// TestArtifactCreateAndLookup verifies artifact creation and content-hash dedup.
func TestArtifactCreateAndLookup(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create first artifact
	artID1, err := db.Artifacts.Create("sha256-abc", "application/json", 1024)
	if err != nil {
		t.Fatalf("Create artifact 1: %v", err)
	}
	if artID1 == 0 {
		t.Fatal("expected non-zero artifact id")
	}

	// Same hash should return the same ID (immutable reuse)
	artID2, err := db.Artifacts.Create("sha256-abc", "application/json", 1024)
	if err != nil {
		t.Fatalf("Create artifact duplicate: %v", err)
	}
	if artID2 != artID1 {
		t.Errorf("duplicate artifact should return existing id %d, got %d", artID1, artID2)
	}

	// Different hash should create a new artifact
	artID3, err := db.Artifacts.Create("sha256-def", "text/plain", 512)
	if err != nil {
		t.Fatalf("Create artifact 3: %v", err)
	}
	if artID3 == artID1 {
		t.Error("different hash should produce different artifact ID")
	}

	// Lookup by hash
	got, err := db.Artifacts.GetByHash("sha256-abc")
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got == nil {
		t.Fatal("artifact not found by hash")
	}
	if got.ByteSize != 1024 {
		t.Errorf("expected byte_size 1024, got %d", got.ByteSize)
	}
	if got.ContentType != "application/json" {
		t.Errorf("expected content_type 'application/json', got %q", got.ContentType)
	}

	// Non-existent hash
	missing, err := db.Artifacts.GetByHash("sha256-nonexistent")
	if err != nil {
		t.Fatalf("GetByHash missing: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for non-existent hash")
	}

	// Lookup by ID
	gotByID, err := db.Artifacts.GetByID(artID1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if gotByID == nil {
		t.Fatal("artifact not found by id")
	}
	if gotByID.ContentHash != "sha256-abc" {
		t.Errorf("expected hash 'sha256-abc', got %q", gotByID.ContentHash)
	}

	// Non-existent ID
	missingID, err := db.Artifacts.GetByID(99999)
	if err != nil {
		t.Fatalf("GetByID missing: %v", err)
	}
	if missingID != nil {
		t.Fatal("expected nil for non-existent id")
	}
}

// TestRunArtifactLinksAreRoleScopedAndImmutable verifies run artifact links are role scoped and immutable.
func TestRunArtifactLinksAreRoleScopedAndImmutable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("snapshot-links", "")
	if err != nil {
		t.Fatal(err)
	}
	configID, err := db.Artifacts.Create("config-snapshot", "application/x-something-config", 10)
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := db.Artifacts.Create("other-snapshot", "application/json", 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunArtifacts.Link(runID, configID, RunArtifactWorkspaceConfig); err != nil {
		t.Fatal(err)
	}
	if err := db.RunArtifacts.Link(runID, configID, RunArtifactWorkspaceConfig); err != nil {
		t.Fatalf("repeat link should be idempotent: %v", err)
	}
	if err := db.RunArtifacts.Link(runID, otherID, RunArtifactWorkspaceConfig); err == nil {
		t.Fatal("expected a conflicting role assignment to fail")
	}
}
