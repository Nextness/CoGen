// revisions_integration_test.go exercises the search revision store against a
// database created by the production migration runner.
//go:build integration

package search_test

import (
	"context"
	"testing"

	"analysis/database/search"
)

// createTestSearch creates one search row and returns its ID.
func createTestSearch(t *testing.T, store *search.Store, searchID string) int64 {
	t.Helper()
	id, err := store.FindOrCreateSearch(context.Background(), searchID)
	if err != nil {
		t.Fatalf("create search: %v", err)
	}
	return id
}

// TestStoreUpsertRevisionPreservesHashesAndIdentity verifies the revision ID is stable while differing hashes refresh the row and updated_at.
func TestStoreUpsertRevisionPreservesHashesAndIdentity(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	searchID := createTestSearch(t, store, "revision-test")

	revisionID, updated, err := store.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "v1",
		ConfigArtifactHash: "config-hash-abc", ResolvedManifestHash: "manifest-hash-xyz",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if revisionID == 0 || updated {
		t.Fatalf("first upsert returned id=%d updated=%v", revisionID, updated)
	}

	reusedID, updated, err := store.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "v1",
		ConfigArtifactHash: "config-hash-abc", ResolvedManifestHash: "manifest-hash-xyz",
	})
	if err != nil {
		t.Fatalf("identical upsert: %v", err)
	}
	if reusedID != revisionID || updated {
		t.Fatalf("identical upsert returned id=%d updated=%v, want %d false", reusedID, updated, revisionID)
	}

	refreshedID, updated, err := store.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "v1",
		ConfigArtifactHash: "config-hash-new", ResolvedManifestHash: "manifest-hash-new",
	})
	if err != nil {
		t.Fatalf("changed upsert: %v", err)
	}
	if refreshedID != revisionID || !updated {
		t.Fatalf("changed upsert returned id=%d updated=%v, want %d true", refreshedID, updated, revisionID)
	}

	got, err := store.GetRevisionByID(ctx, revisionID)
	if err != nil || got == nil {
		t.Fatalf("get by id = %+v err=%v", got, err)
	}
	if got.ConfigArtifactHash != "config-hash-new" || got.ResolvedManifestHash != "manifest-hash-new" {
		t.Fatalf("refreshed revision = %+v", got)
	}
	if got.UpdatedAt == "" {
		t.Fatal("expected updated_at to be set after a hash refresh")
	}

	byLabel, err := store.GetRevisionBySearchAndLabel(ctx, searchID, "v1")
	if err != nil || byLabel == nil || byLabel.ID != revisionID {
		t.Fatalf("get by search and label = %+v err=%v", byLabel, err)
	}
}

// TestStoreRevisionsOrderAndEmptyIdentity verifies distinct labels keep distinct IDs, lists follow ORDER BY id, and absent reads stay nil.
func TestStoreRevisionsOrderAndEmptyIdentity(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	searchID := createTestSearch(t, store, "multi-revision")

	missing, err := store.GetRevisionByID(ctx, 999)
	if err != nil || missing != nil {
		t.Fatalf("missing revision = %+v err=%v, want nil without error", missing, err)
	}
	missing, err = store.GetRevisionBySearchAndLabel(ctx, 999, "absent")
	if err != nil || missing != nil {
		t.Fatalf("missing revision by label = %+v err=%v, want nil without error", missing, err)
	}
	empty, err := store.ListRevisionsBySearch(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty revision list = %+v err=%v, want nil without error", empty, err)
	}

	first, _, err := store.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "v1", ConfigArtifactHash: "hash-a", ResolvedManifestHash: "hash-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := store.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "v2", ConfigArtifactHash: "hash-b", ResolvedManifestHash: "hash-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("distinct labels shared a revision id")
	}
	revisions, err := store.ListRevisionsBySearch(ctx, searchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0].ID != first || revisions[1].ID != second {
		t.Fatalf("revision list = %+v, want ids [%d %d]", revisions, first, second)
	}
}
