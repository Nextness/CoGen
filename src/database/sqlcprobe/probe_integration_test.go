//go:build integration

package sqlcprobe_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
	"analysis/database/sqlcprobe"
)

// probeConfigPath points at the production database registry from the probe test directory.
var probeConfigPath = filepath.Join("..", "..", "..", "config", "database.something")

// openProbe returns a probe over a database created by the production migration runner.
func openProbe(t *testing.T) (*sqlcprobe.Probe, *database.Database) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "probe.db")
	db, err := database.Open(dbPath, probeConfigPath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return sqlcprobe.New(db.DB), db
}

// probeEntry returns one complete probe entry for a request fingerprint.
func probeEntry(fingerprint string) sqlcprobe.Entry {
	return sqlcprobe.Entry{
		Provider:           "crossref",
		Namespace:          "probe",
		RequestFingerprint: fingerprint,
		ResponseStatus:     200,
		FetchedAt:          "2026-01-01T00:00:00Z",
		ExtractorVersion:   "v1",
	}
}

// TestProbeInsertReportsResultAndMapsNullableOutput verifies named parameters, nullable input and output, and the inspected insert result.
func TestProbeInsertReportsResultAndMapsNullableOutput(t *testing.T) {
	probe, db := openProbe(t)
	ctx := context.Background()
	first, err := probe.InsertEntry(ctx, probeEntry("nullable-negative"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID <= 0 || first.RowsAffected != 1 {
		t.Fatalf("insert result = %+v, want positive id and one affected row", first)
	}
	record, err := probe.LatestEntry(ctx, "crossref", "probe")
	if err != nil || record == nil {
		t.Fatalf("latest entry = %+v err=%v", record, err)
	}
	if record.ID != first.ID || record.ResponseStatus != 200 || record.PayloadArtifactID != nil || record.ExpiresAt != "" {
		t.Fatalf("nullable negative record = %+v", record)
	}
	artifactID, err := db.Artifacts.Create("probe-payload", "application/json", 4)
	if err != nil {
		t.Fatal(err)
	}
	positive := probeEntry("nullable-positive")
	positive.PayloadArtifactID = &artifactID
	positive.ExpiresAt = "2026-02-01T00:00:00Z"
	second, err := probe.InsertEntry(ctx, positive)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := probe.LatestEntry(ctx, "crossref", "probe")
	if err != nil || latest == nil || latest.ID != second.ID {
		t.Fatalf("latest entry = %+v err=%v, want %d", latest, err, second.ID)
	}
	if latest.PayloadArtifactID == nil || *latest.PayloadArtifactID != artifactID || latest.ExpiresAt != "2026-02-01T00:00:00Z" {
		t.Fatalf("nullable positive record = %+v", latest)
	}
	missing, err := probe.LatestEntry(ctx, "openalex", "probe")
	if err != nil || missing != nil {
		t.Fatalf("missing record = %+v err=%v, want nil without error", missing, err)
	}
}

// TestProbeListEntriesReturnsDeterministicOrder verifies the generated many-row result follows explicit ordering and preserves an empty slice.
func TestProbeListEntriesReturnsDeterministicOrder(t *testing.T) {
	probe, _ := openProbe(t)
	ctx := context.Background()
	fingerprints := []string{"third", "first", "second"}
	for _, fingerprint := range fingerprints {
		if _, err := probe.InsertEntry(ctx, probeEntry(fingerprint)); err != nil {
			t.Fatal(err)
		}
	}
	records, err := probe.ListEntries(ctx, "crossref")
	if err != nil || len(records) != len(fingerprints) {
		t.Fatalf("records=%d err=%v, want %d", len(records), err, len(fingerprints))
	}
	for index, fingerprint := range fingerprints {
		if records[index].RequestFingerprint != fingerprint {
			t.Fatalf("record %d fingerprint = %q, want %q", index, records[index].RequestFingerprint, fingerprint)
		}
	}
	empty, err := probe.ListEntries(ctx, "absent")
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty records = %#v, want non-nil empty slice", empty)
	}
}

// TestProbeInsertPairCommitsAndRollsBack verifies Queries.WithTx executes on a transaction and rolls back a failed pair.
func TestProbeInsertPairCommitsAndRollsBack(t *testing.T) {
	probe, _ := openProbe(t)
	ctx := context.Background()
	if err := probe.InsertPair(ctx, probeEntry("tx-first"), probeEntry("tx-second")); err != nil {
		t.Fatalf("commit pair: %v", err)
	}
	committed, err := probe.ListEntries(ctx, "crossref")
	if err != nil || len(committed) != 2 {
		t.Fatalf("committed records=%d err=%v, want 2", len(committed), err)
	}
	rolledSecond := probeEntry("rollback-second")
	missingArtifact := int64(999999)
	rolledSecond.PayloadArtifactID = &missingArtifact
	err = probe.InsertPair(ctx, probeEntry("rollback-first"), rolledSecond)
	if err == nil || !strings.Contains(err.Error(), "insert second probe entry") || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("rollback pair error = %v, want foreign-key failure on the second insert", err)
	}
	after, err := probe.ListEntries(ctx, "crossref")
	if err != nil || len(after) != 2 {
		t.Fatalf("records after rollback=%d err=%v, want the two committed rows", len(after), err)
	}
	for _, record := range after {
		if record.RequestFingerprint == "rollback-first" || record.RequestFingerprint == "rollback-second" {
			t.Fatalf("rolled-back record persisted: %+v", record)
		}
	}
}
