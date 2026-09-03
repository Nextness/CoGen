// artifact_integrity_integration_test.go verifies content-addressed artifact conflicts fail closed.
//go:build integration

package database

import "testing"

// TestArtifactRepositoriesRejectConflictingContent verifies duplicate identities cannot conceal different metadata or bytes.
func TestArtifactRepositoriesRejectConflictingContent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("artifact integrity", "")
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := db.Artifacts.Create("content-hash", "application/json", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Artifacts.Create("content-hash", "text/plain", 3); err == nil {
		t.Fatal("expected conflicting artifact content type to fail")
	}
	if _, err := db.Artifacts.Create("content-hash", "application/json", 4); err == nil {
		t.Fatal("expected conflicting artifact byte size to fail")
	}
	if _, err := db.ArtifactBlobs.Create(artifactID, runID, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ArtifactBlobs.Create(artifactID, runID, []byte("two")); err == nil {
		t.Fatal("expected conflicting artifact bytes to fail")
	}
	if _, err := db.Artifacts.CreateWithBlob("atomic-hash", "application/json", 3, runID, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Artifacts.CreateWithBlob("atomic-hash", "application/json", 3, runID, []byte("two")); err == nil {
		t.Fatal("expected atomic conflicting artifact bytes to fail")
	}
}
