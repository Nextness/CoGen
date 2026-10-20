// viewer_dynamic_unit_test.go verifies the retained dynamic audit filter
// builder without a database.
//go:build unit

package audit

import (
	"strings"
	"testing"
)

// TestAuditInClauseBuildsParameterizedMarkers verifies every value is bound and
// no caller value is interpolated into SQL text.
func TestAuditInClauseBuildsParameterizedMarkers(t *testing.T) {
	t.Run("single value", func(t *testing.T) {
		clause, args := auditInClause("col", []string{"val1"})
		if clause != "col IN (?)" {
			t.Errorf("clause = %q, want %q", clause, "col IN (?)")
		}
		if len(args) != 1 || args[0] != "val1" {
			t.Errorf("args = %v, want [val1]", args)
		}
	})
	t.Run("multiple values", func(t *testing.T) {
		clause, args := auditInClause("actor", []string{"a", "b", "c"})
		if clause != "actor IN (?,?,?)" {
			t.Errorf("clause = %q, want %q", clause, "actor IN (?,?,?)")
		}
		if len(args) != 3 {
			t.Fatalf("len(args) = %d, want 3", len(args))
		}
		for i, want := range []string{"a", "b", "c"} {
			if args[i] != want {
				t.Errorf("args[%d] = %v, want %v", i, args[i], want)
			}
		}
	})
	t.Run("empty", func(t *testing.T) {
		clause, args := auditInClause("col", []string{})
		if clause != "col IN ()" {
			t.Errorf("clause = %q, want %q", clause, "col IN ()")
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})
}

// TestAuditWhereJoinsPredicates verifies the optional WHERE clause assembly.
func TestAuditWhereJoinsPredicates(t *testing.T) {
	tests := []struct {
		clauses []string
		want    string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"a = 1"}, " WHERE a = 1"},
		{[]string{"a = 1", "b = 2"}, " WHERE a = 1 AND b = 2"},
		{[]string{"x IN (?,?,?)", "y IS NULL"}, " WHERE x IN (?,?,?) AND y IS NULL"},
	}
	for _, tc := range tests {
		got := auditWhere(tc.clauses)
		if got != tc.want {
			t.Errorf("auditWhere(%v) = %q, want %q", tc.clauses, got, tc.want)
		}
	}
}

// TestAuditFilterClausesBindEveryValue verifies the dynamic builder maps closed
// categories and binds every user value.
func TestAuditFilterClausesBindEveryValue(t *testing.T) {
	clauses, args := auditFilterClauses(Filter{
		EntityID:        "42",
		EntityTypes:     []string{"work", "work_revision"},
		Actions:         []string{"field_enriched"},
		Actors:          []string{"crossref"},
		Categories:      []string{"enrichment", "unknown"},
		Stage:           "enrich",
		Outcome:         "completed",
		ReviewStatus:    "accepted",
		ReviewReason:    "looks good",
		ReviewSubstatus: "methodology",
		Query:           "Title",
		RunID:           7,
	})
	if len(clauses) != 12 {
		t.Fatalf("clause count = %d, want 12: %v", len(clauses), clauses)
	}
	if len(args) != 15 {
		t.Fatalf("arg count = %d, want 15: %v", len(args), args)
	}
	for _, clause := range clauses {
		if clause == "" {
			t.Fatalf("empty clause in %v", clauses)
		}
	}
	if clauses[4] != "(action IN ('field_enriched','cache_hit','network_fetch'))" {
		t.Fatalf("category clause = %q", clauses[4])
	}
	if args[0] != "42" || args[1] != "work" || args[2] != "work_revision" {
		t.Fatalf("leading args = %v", args[:3])
	}
	if args[len(args)-1] != int64(7) {
		t.Fatalf("run scope arg = %v, want 7", args[len(args)-1])
	}
}

// TestValidateAuditFilterCapsInputs verifies the retained builder rejects
// oversized value lists and filter strings.
func TestValidateAuditFilterCapsInputs(t *testing.T) {
	oversizedValues := make([]string, 101)
	for index := range oversizedValues {
		oversizedValues[index] = "value"
	}
	for _, test := range []struct {
		name   string
		filter Filter
	}{
		{"value list", Filter{Actors: oversizedValues}},
		{"value length", Filter{Actors: []string{strings.Repeat("x", 201)}}},
		{"entity id", Filter{EntityID: strings.Repeat("x", 201)}},
		{"stage", Filter{Stage: strings.Repeat("x", 201)}},
		{"outcome", Filter{Outcome: strings.Repeat("x", 201)}},
		{"review status", Filter{ReviewStatus: strings.Repeat("x", 201)}},
		{"review reason", Filter{ReviewReason: strings.Repeat("x", 1001)}},
		{"review substatus", Filter{ReviewSubstatus: strings.Repeat("x", 201)}},
		{"query", Filter{Query: strings.Repeat("x", 201)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateAuditFilter(test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
	if err := validateAuditFilter(Filter{Actors: []string{"crossref"}, Query: "title"}); err != nil {
		t.Fatalf("valid filter rejected: %v", err)
	}
}

// TestAuditFacetColumnsAllowlist verifies only the three supported facet
// columns are available to the dynamic facet builder.
func TestAuditFacetColumnsAllowlist(t *testing.T) {
	for _, facet := range []string{"actor", "action", "entity_type"} {
		if _, ok := auditFacetColumns[facet]; !ok {
			t.Fatalf("facet %q is missing from the allowlist", facet)
		}
	}
	if len(auditFacetColumns) != 3 {
		t.Fatalf("facet allowlist = %v, want exactly three entries", auditFacetColumns)
	}
}
