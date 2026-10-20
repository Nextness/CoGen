// Unit tests for work revision payload hash computation.
//go:build unit

package work

import (
	"testing"
)

// TestRevisionPayloadHashDeterminism verifies revision payload hash determinism.
func TestRevisionPayloadHashDeterminism(t *testing.T) {
	base := &Revision{
		Title:    "Same Title",
		Year:     2023,
		Journal:  "Same Journal",
		Keywords: `["kw1"]`,
	}

	// Same inputs -> same hash
	h1 := computeRevisionPayloadHash(base)
	h2 := computeRevisionPayloadHash(base)
	if h1 != h2 {
		t.Fatal("identical revisions must produce the same hash")
	}

	// Changed metadata -> different hash
	diff := &Revision{
		Title:    "Same Title",
		Year:     2024, // different
		Journal:  "Same Journal",
		Keywords: `["kw1"]`,
	}
	h3 := computeRevisionPayloadHash(diff)
	if h1 == h3 {
		t.Fatal("different year must produce a different hash")
	}

	// producer_stage is provenance -> must NOT affect hash
	stageA := &Revision{
		Title:         "Same Title",
		Year:          2023,
		Journal:       "Same Journal",
		ProducerStage: "parse",
	}
	stageB := &Revision{
		Title:         "Same Title",
		Year:          2023,
		Journal:       "Same Journal",
		ProducerStage: "enrich",
	}
	h4 := computeRevisionPayloadHash(stageA)
	h5 := computeRevisionPayloadHash(stageB)
	if h4 != h5 {
		t.Fatal("producer_stage must not affect the payload hash")
	}

	// field_schema_version IS part of the payload interpretation
	verA := &Revision{
		Title:              "Version test",
		FieldSchemaVersion: "1",
	}
	verB := &Revision{
		Title:              "Version test",
		FieldSchemaVersion: "2",
	}
	h6 := computeRevisionPayloadHash(verA)
	h7 := computeRevisionPayloadHash(verB)
	if h6 == h7 {
		t.Fatal("field_schema_version must affect the payload hash")
	}
}

// TestRevisionPayloadHashGoldenValue pins the exact field set covered by the
// payload hash so adding, removing, or renaming a covered field is caught.
func TestRevisionPayloadHashGoldenValue(t *testing.T) {
	revision := &Revision{
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
	if got := computeRevisionPayloadHash(revision); got != want {
		t.Fatalf("payload hash = %q, want %q", got, want)
	}
}

// TestRevisionPayloadHashCoversEveryContentField verifies every content field
// changes the hash while producer stage remains excluded.
func TestRevisionPayloadHashCoversEveryContentField(t *testing.T) {
	base := &Revision{
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
	baseHash := computeRevisionPayloadHash(base)
	mutations := []struct {
		name   string
		mutate func(*Revision)
	}{
		{"title", func(revision *Revision) { revision.Title = "Other" }},
		{"abstract", func(revision *Revision) { revision.Abstract = "Other" }},
		{"year", func(revision *Revision) { revision.Year = 2025 }},
		{"journal", func(revision *Revision) { revision.Journal = "Other" }},
		{"publisher", func(revision *Revision) { revision.Publisher = "Other" }},
		{"source", func(revision *Revision) { revision.Source = "Other" }},
		{"keywords", func(revision *Revision) { revision.Keywords = `["other"]` }},
		{"keywords_plus", func(revision *Revision) { revision.KeywordsPlus = `["other"]` }},
		{"citation_count", func(revision *Revision) { revision.CitationCount = 8 }},
		{"reference_count", func(revision *Revision) { revision.ReferenceCount = 4 }},
		{"extension_data", func(revision *Revision) { revision.ExtensionData = `{"k":"other"}` }},
		{"field_schema_version", func(revision *Revision) { revision.FieldSchemaVersion = "2" }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			mutated := *base
			test.mutate(&mutated)
			if got := computeRevisionPayloadHash(&mutated); got == baseHash {
				t.Fatalf("hash did not change when %s changed", test.name)
			}
		})
	}
	provenance := *base
	provenance.ProducerStage = ProducerStageNormalize
	if got := computeRevisionPayloadHash(&provenance); got != baseHash {
		t.Fatal("producer stage must not affect the payload hash")
	}
}
