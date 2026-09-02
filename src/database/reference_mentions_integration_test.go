// Integration tests for reference mention operations and append-only enforcement.
//go:build integration

package database

import (
	"strings"
	"testing"
)

// TestReferenceMentionExternalReferencesRemainDistinct verifies reference mention external references remain distinct.
func TestReferenceMentionExternalReferencesRemainDistinct(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	revisionID := createReferenceMentionTestRevision(t, db, "10.1000/reference-source")
	firstID, err := db.ReferenceMentions.Create(&ReferenceMention{
		WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "Doe (2020)", DOI: "10.2000/external",
		Title: "External", Author: "Doe", Year: 2020, Source: "source-a",
	})
	if err != nil {
		t.Fatalf("create first mention: %v", err)
	}
	secondID, err := db.ReferenceMentions.Create(&ReferenceMention{
		WorkRevisionID: revisionID, MentionOrder: 2, RawReference: "Doe (2020)", DOI: "10.2000/external",
		Title: "External", Author: "Doe", Year: 2020, Source: "source-a",
	})
	if err != nil {
		t.Fatalf("create repeated external mention: %v", err)
	}
	if firstID == secondID {
		t.Fatal("identical external references must remain separate mentions")
	}

	mentions, err := db.ReferenceMentions.GetByRevisionID(revisionID)
	if err != nil {
		t.Fatalf("get mentions: %v", err)
	}
	if len(mentions) != 2 || mentions[0].MentionOrder != 1 || mentions[1].MentionOrder != 2 {
		t.Fatalf("expected two ordered mentions, got %+v", mentions)
	}
	if mentions[0].ResolvedWorkID != 0 || mentions[1].ResolvedWorkID != 0 {
		t.Fatal("external references must not resolve to a workspace work")
	}
}

// TestReferenceMentionResolvesKnownWork verifies reference mention resolves known work.
func TestReferenceMentionResolvesKnownWork(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	targetID, err := db.Works.CreateByDOI("10.1000/known-target")
	if err != nil {
		t.Fatalf("create target work: %v", err)
	}
	revisionID := createReferenceMentionTestRevision(t, db, "10.1000/known-source")
	mentionID, err := db.ReferenceMentions.Create(&ReferenceMention{
		WorkRevisionID: revisionID, MentionOrder: 1, DOI: "https://doi.org/10.1000/Known-Target",
	})
	if err != nil {
		t.Fatalf("create mention: %v", err)
	}
	mention, err := db.ReferenceMentions.GetByID(mentionID)
	if err != nil || mention == nil {
		t.Fatalf("get mention: %v, %#v", err, mention)
	}
	if mention.DOI != "10.1000/known-target" || mention.ResolvedWorkID != targetID {
		t.Fatalf("expected normalized DOI and resolved work %d, got %+v", targetID, mention)
	}
	resolved, err := db.ReferenceMentions.GetByResolvedWorkID(targetID)
	if err != nil || len(resolved) != 1 || resolved[0].ID != mentionID {
		t.Fatalf("get by resolved work: %v, %+v", err, resolved)
	}
}

// TestReferenceMentionValidationAndUniqueness verifies reference mention validation and uniqueness.
func TestReferenceMentionValidationAndUniqueness(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if _, err := db.ReferenceMentions.Create(&ReferenceMention{MentionOrder: 1}); err == nil {
		t.Fatal("expected missing work revision error")
	}
	revisionID := createReferenceMentionTestRevision(t, db, "10.1000/reference-validation")
	if _, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: revisionID}); err == nil {
		t.Fatal("expected non-positive mention order error")
	}
	if _, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: revisionID, MentionOrder: 1}); err != nil {
		t.Fatalf("create mention: %v", err)
	}
	if _, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: revisionID, MentionOrder: 1}); err == nil {
		t.Fatal("expected duplicate mention order error")
	}
}

// TestReferenceMentionSnapshotsAreAppendOnly verifies reference mention snapshots are append only.
func TestReferenceMentionSnapshotsAreAppendOnly(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	firstRevision := createReferenceMentionTestRevision(t, db, "10.1000/reference-snapshot")
	secondRevision := createReferenceMentionTestRevision(t, db, "10.1000/reference-snapshot")
	firstID, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: firstRevision, MentionOrder: 1, Title: "Old reference"})
	if err != nil {
		t.Fatalf("create first snapshot mention: %v", err)
	}
	if _, err := db.ReferenceMentions.Create(&ReferenceMention{WorkRevisionID: secondRevision, MentionOrder: 1, Title: "New reference"}); err != nil {
		t.Fatalf("create second snapshot mention: %v", err)
	}

	_, err = db.DB.Exec("UPDATE reference_mentions SET title = 'Changed' WHERE id = ?", firstID)
	if err == nil || !strings.Contains(err.Error(), "reference_mentions is append-only") {
		t.Fatalf("expected append-only update error, got %v", err)
	}
	_, err = db.DB.Exec("DELETE FROM reference_mentions WHERE id = ?", firstID)
	if err == nil || !strings.Contains(err.Error(), "reference_mentions is append-only") {
		t.Fatalf("expected append-only delete error, got %v", err)
	}
	first, err := db.ReferenceMentions.GetByRevisionID(firstRevision)
	if err != nil || len(first) != 1 || first[0].Title != "Old reference" {
		t.Fatalf("first snapshot changed: %v, %+v", err, first)
	}
	second, err := db.ReferenceMentions.GetByRevisionID(secondRevision)
	if err != nil || len(second) != 1 || second[0].Title != "New reference" {
		t.Fatalf("second snapshot changed: %v, %+v", err, second)
	}
}

// TestV00005ReferenceMentionsMigrationApplies verifies the reference-mentions migration remains applied in the current chain.
func TestV00005ReferenceMentionsMigrationApplies(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 27 {
		t.Fatalf("expected 27 applied migrations, got %d", count)
	}
	for _, name := range []string{"reference_mentions"} {
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count); err != nil || count != 1 {
			t.Fatalf("expected table %q: count=%d err=%v", name, count, err)
		}
	}
	for _, name := range []string{"idx_reference_mentions_revision", "idx_reference_mentions_resolved_work", "reference_mentions_abort_update", "reference_mentions_abort_delete"} {
		typeName := "index"
		if strings.Contains(name, "abort") {
			typeName = "trigger"
		}
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type=? AND name=?", typeName, name).Scan(&count); err != nil || count != 1 {
			t.Fatalf("expected %s %q: count=%d err=%v", typeName, name, count, err)
		}
	}
}
