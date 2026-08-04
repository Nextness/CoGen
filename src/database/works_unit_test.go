// Unit tests for work revision payload hash computation.
//go:build unit

package database

import (
	"testing"
)

// TestRevisionPayloadHashDeterminism verifies revision payload hash determinism.
func TestRevisionPayloadHashDeterminism(t *testing.T) {
	base := &WorkRevision{
		Title:    "Same Title",
		Year:     2023,
		Journal:  "Same Journal",
		Keywords: `["kw1"]`,
	}

	// Same inputs → same hash
	h1 := computeRevisionPayloadHash(base)
	h2 := computeRevisionPayloadHash(base)
	if h1 != h2 {
		t.Fatal("identical revisions must produce the same hash")
	}

	// Changed metadata → different hash
	diff := &WorkRevision{
		Title:    "Same Title",
		Year:     2024, // different
		Journal:  "Same Journal",
		Keywords: `["kw1"]`,
	}
	h3 := computeRevisionPayloadHash(diff)
	if h1 == h3 {
		t.Fatal("different year must produce a different hash")
	}

	// producer_stage is provenance → must NOT affect hash
	stageA := &WorkRevision{
		Title:         "Same Title",
		Year:          2023,
		Journal:       "Same Journal",
		ProducerStage: "parse",
	}
	stageB := &WorkRevision{
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
	verA := &WorkRevision{
		Title:              "Version test",
		FieldSchemaVersion: "1",
	}
	verB := &WorkRevision{
		Title:              "Version test",
		FieldSchemaVersion: "2",
	}
	h6 := computeRevisionPayloadHash(verA)
	h7 := computeRevisionPayloadHash(verB)
	if h6 == h7 {
		t.Fatal("field_schema_version must affect the payload hash")
	}
}
