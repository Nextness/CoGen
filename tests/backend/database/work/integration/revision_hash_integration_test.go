// Unit tests for work revision payload hash computation.

package work_test

import subject "analysis/backend/database/work"

import (
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"path/filepath"
	"testing"
)

// TestRevisionPayloadHashDeterminism verifies revision payload hash determinism.
func TestRevisionPayloadHashDeterminism(t *testing.T) {
	base := &subject.Revision{
		Title:    "Same Title",
		Year:     2023,
		Journal:  "Same Journal",
		Keywords: `["kw1"]`,
	}

	// Same inputs -> same hash
	h1 := persistedRevisionHash(t, base)
	h2 := persistedRevisionHash(t, base)
	if h1 != h2 {
		t.Fatal("identical revisions must produce the same hash")
	}

	// Changed metadata -> different hash
	diff := &subject.Revision{
		Title:    "Same Title",
		Year:     2024, // different
		Journal:  "Same Journal",
		Keywords: `["kw1"]`,
	}
	h3 := persistedRevisionHash(t, diff)
	if h1 == h3 {
		t.Fatal("different year must produce a different hash")
	}

	// producer_stage is provenance -> must NOT affect hash
	stageA := &subject.Revision{
		Title:         "Same Title",
		Year:          2023,
		Journal:       "Same Journal",
		ProducerStage: "parse",
	}
	stageB := &subject.Revision{
		Title:         "Same Title",
		Year:          2023,
		Journal:       "Same Journal",
		ProducerStage: subject.ProducerStageEnrichMetadata,
	}
	h4 := persistedRevisionHash(t, stageA)
	h5 := persistedRevisionHash(t, stageB)
	if h4 != h5 {
		t.Fatal("producer_stage must not affect the payload hash")
	}

	// field_schema_version IS part of the payload interpretation
	verA := &subject.Revision{
		Title:              "Version test",
		FieldSchemaVersion: "1",
	}
	verB := &subject.Revision{
		Title:              "Version test",
		FieldSchemaVersion: "2",
	}
	h6 := persistedRevisionHash(t, verA)
	h7 := persistedRevisionHash(t, verB)
	if h6 == h7 {
		t.Fatal("field_schema_version must affect the payload hash")
	}
}

// TestRevisionPayloadHashGoldenValue pins the exact field set covered by the
// payload hash so adding, removing, or renaming a covered field is caught.
func TestRevisionPayloadHashGoldenValue(t *testing.T) {
	revision := &subject.Revision{
		Title:              "Title",
		Abstract:           "Abstract",
		Year:               2024,
		Journal:            "Journal",
		Publisher:          "Publisher",
		Source:             "Source",
		Keywords:           `["kw"]`,
		KeywordsPlus:       `["kwp"]`,
		CitationCount:      7,
		ReferenceCount:     3,
		ExtensionData:      `{"k":"v"}`,
		FieldSchemaVersion: "1",
	}
	const want = "279a1d13aee6ad29d86730265d6533f6178b121488a2a18b853864c59081c945"
	if got := persistedRevisionHash(t, revision); got != want {
		t.Fatalf("payload hash = %q, want %q", got, want)
	}
}

// TestRevisionPayloadHashCoversEveryContentField verifies every content field
// changes the hash while producer stage remains excluded.
func TestRevisionPayloadHashCoversEveryContentField(t *testing.T) {
	base := &subject.Revision{
		Title:              "Title",
		Abstract:           "Abstract",
		Year:               2024,
		Journal:            "Journal",
		Publisher:          "Publisher",
		Source:             "Source",
		Keywords:           `["kw"]`,
		KeywordsPlus:       `["kwp"]`,
		CitationCount:      7,
		ReferenceCount:     3,
		ExtensionData:      `{"k":"v"}`,
		FieldSchemaVersion: "1",
	}
	baseHash := persistedRevisionHash(t, base)
	mutations := []struct {
		name   string
		mutate func(*subject.Revision)
	}{
		{"title", func(revision *subject.Revision) { revision.Title = "Other" }},
		{"abstract", func(revision *subject.Revision) { revision.Abstract = "Other" }},
		{"year", func(revision *subject.Revision) { revision.Year = 2025 }},
		{"journal", func(revision *subject.Revision) { revision.Journal = "Other" }},
		{"publisher", func(revision *subject.Revision) { revision.Publisher = "Other" }},
		{"source", func(revision *subject.Revision) { revision.Source = "Other" }},
		{"keywords", func(revision *subject.Revision) { revision.Keywords = `["other"]` }},
		{"keywords_plus", func(revision *subject.Revision) { revision.KeywordsPlus = `["other"]` }},
		{"citation_count", func(revision *subject.Revision) { revision.CitationCount = 8 }},
		{"reference_count", func(revision *subject.Revision) { revision.ReferenceCount = 4 }},
		{"extension_data", func(revision *subject.Revision) { revision.ExtensionData = `{"k":"other"}` }},
		{"field_schema_version", func(revision *subject.Revision) { revision.FieldSchemaVersion = "2" }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			mutated := *base
			test.mutate(&mutated)
			if got := persistedRevisionHash(t, &mutated); got == baseHash {
				t.Fatalf("hash did not change when %s changed", test.name)
			}
		})
	}
	provenance := *base
	provenance.ProducerStage = subject.ProducerStageNormalize
	if got := persistedRevisionHash(t, &provenance); got != baseHash {
		t.Fatal("producer stage must not affect the payload hash")
	}
}

// persistedRevisionHash observes content hashing through the supported immutable revision API.
func persistedRevisionHash(t *testing.T, input *subject.Revision) string {
	t.Helper()
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "hash.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	revision := *input
	revision.WorkID, err = db.Work.CreateWorkByDOI(context.Background(), "10.1000/hash")
	if err != nil {
		t.Fatal(err)
	}
	revision.PipelineRunID, err = testfixture.Run(context.Background(), db.Conn, "hash", "")
	if err != nil {
		t.Fatal(err)
	}
	if revision.ProducerStage == "" {
		revision.ProducerStage = subject.ProducerStageParse
	}
	id, err := db.Work.CreateRevision(context.Background(), &revision)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := db.Work.GetRevisionByID(context.Background(), id)
	if err != nil || stored == nil {
		t.Fatalf("revision=%+v err=%v", stored, err)
	}
	if stored.PayloadHash != revision.PayloadHash {
		t.Fatal("returned and persisted payload hashes differ")
	}
	return stored.PayloadHash
}
