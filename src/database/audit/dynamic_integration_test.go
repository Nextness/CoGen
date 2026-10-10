// dynamic_integration_test.go verifies the retained audit filter builder
// rejects injection attempts and caps its list and filter inputs.
//go:build integration

package audit_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/audit"
)

// TestStoreAuditFilterConstrainsDynamicInputs verifies the retained audit
// filter builder binds search values, caps value lists and filter strings, and
// resolves only allowlisted facet columns.
func TestStoreAuditFilterConstrainsDynamicInputs(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "audit-filter-dynamic")
	insert := func(actor, entityType, entityID, action string) {
		t.Helper()
		if _, err := db.DB.Exec(`INSERT INTO audit_events
			(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action)
			VALUES (datetime('now'), ?, ?, ?, ?, ?)`, actor, runID, entityType, entityID, action); err != nil {
			t.Fatal(err)
		}
	}
	insert("pipeline", "work", "1", "validation_changed")
	insert(`x' OR '1'='1`, "work", "2", "validation_changed")

	// A malicious search value is bound as a literal LIKE pattern and matches
	// only the event that stores the same text.
	literal, err := store.List(ctx, audit.Filter{RunID: runID, Query: `' OR '1'='1`, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(literal) != 1 || literal[0].Actor != `x' OR '1'='1` {
		t.Fatalf("literal search = %+v, want only the stored malicious text", literal)
	}

	facets, err := store.Facets(ctx, audit.Filter{RunID: runID})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets.Actors) != 2 || len(facets.Actions) != 1 || len(facets.EntityTypes) != 1 {
		t.Fatalf("facets = %+v", facets)
	}

	oversizedValues := make([]string, 101)
	for index := range oversizedValues {
		oversizedValues[index] = "value"
	}
	for _, test := range []struct {
		name   string
		filter audit.Filter
	}{
		{"oversized value list", audit.Filter{RunID: runID, Actors: oversizedValues, Limit: 10}},
		{"oversized value", audit.Filter{RunID: runID, Actors: []string{strings.Repeat("x", 201)}, Limit: 10}},
		{"oversized query", audit.Filter{RunID: runID, Query: strings.Repeat("x", 201), Limit: 10}},
		{"oversized review reason", audit.Filter{RunID: runID, ReviewReason: strings.Repeat("x", 1001), Limit: 10}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.List(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
			if _, err := store.Summary(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
			if _, err := store.Facets(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}
