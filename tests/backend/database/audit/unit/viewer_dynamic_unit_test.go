// viewer_dynamic_unit_test.go verifies bounded audit SQL and bindings through public evidence reads.
package audit_test

import (
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"analysis/backend/database/audit"
	"analysis/tests/support/go/sqlprobe"
	"analysis/tests/support/go/testfixture"
)

// auditQuery records the exact query and bindings produced by the public List operation.
func auditQuery(t *testing.T, filter audit.Filter) sqlprobe.Call {
	t.Helper()
	var call sqlprobe.Call
	d := &sqlprobe.Driver{Query: func(c sqlprobe.Call) (driver.Rows, error) {
		call = c
		return &sqlprobe.Rows{Names: []string{"id"}}, nil
	}}
	db := d.OpenDB()
	defer db.Close()
	if _, err := audit.New(db).List(t.Context(), filter); err != nil {
		t.Fatal(err)
	}
	return call
}

// TestAuditListBindsFacetValues verifies single, multiple, empty, and hostile values are never interpolated.
func TestAuditListBindsFacetValues(t *testing.T) {
	for _, values := range [][]string{{"val1"}, {"a", "b", "c"}, {}, {"x') OR 1=1 --"}} {
		call := auditQuery(t, audit.Filter{Actors: values})
		if len(call.Args) != len(values)+1 {
			t.Fatalf("bindings=%v, values=%v", call.Args, values)
		}
		for i, value := range values {
			if call.Args[i].Value != value {
				t.Fatalf("binding %d=%v, want %q", i, call.Args[i].Value, value)
			}
			if strings.Contains(call.SQL, "'"+value+"'") {
				t.Fatalf("unbound actor in %q", call.SQL)
			}
		}
		if len(values) == 0 {
			if strings.Contains(call.SQL, " WHERE ") {
				t.Fatalf("unexpected WHERE: %q", call.SQL)
			}
		} else if !strings.Contains(call.SQL, "actor IN ("+strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")+")") {
			t.Fatalf("unexpected markers: %q", call.SQL)
		}
	}
}

// TestAuditListCombinesFilters verifies all predicates and values survive the public list boundary.
func TestAuditListCombinesFilters(t *testing.T) {
	call := auditQuery(t, audit.Filter{EntityID: "42", EntityTypes: []string{"work", "work_revision"}, Actions: []string{"field_enriched"}, Actors: []string{"crossref"}, Categories: []string{"enrichment", "unknown"}, Stage: "enrich", Outcome: "completed", ReviewStatus: "accepted", ReviewReason: "looks good", ReviewSubstatus: "methodology", Query: "Title", RunID: 7})
	var got []any
	for _, arg := range call.Args {
		got = append(got, arg.Value)
	}
	want := []any{"42", "work", "work_revision", "field_enriched", "crossref", "enrich", "completed", "accepted", "looks good", "methodology", "%title%", "%title%", "%title%", "%title%", int64(7), int64(1)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bindings=%v, want %v", got, want)
	}
	for _, clause := range []string{" WHERE entity_id=? AND ", "(action IN ('field_enriched','cache_hit','network_fetch'))", "pipeline_run_id=?", "json_each(after_json, '$.sub_statuses')"} {
		if !strings.Contains(call.SQL, clause) {
			t.Fatalf("missing %q in %q", clause, call.SQL)
		}
	}
}

// TestAuditReadsValidateBeforeStorage verifies each public read rejects oversized filters before touching a closed connection.
func TestAuditReadsValidateBeforeStorage(t *testing.T) {
	values := make([]string, 101)
	for i := range values {
		values[i] = "value"
	}
	for _, filter := range []audit.Filter{{Actors: values}, {Actors: []string{strings.Repeat("x", 201)}}, {EntityID: strings.Repeat("x", 201)}, {Stage: strings.Repeat("x", 201)}, {Outcome: strings.Repeat("x", 201)}, {ReviewStatus: strings.Repeat("x", 201)}, {ReviewReason: strings.Repeat("x", 1001)}, {ReviewSubstatus: strings.Repeat("x", 201)}, {Query: strings.Repeat("x", 201)}} {
		store := audit.New(testfixture.ClosedDB())
		_, a := store.List(t.Context(), filter)
		_, b := store.Summary(t.Context(), filter)
		_, c := store.Facets(t.Context(), filter)
		for _, err := range []error{a, b, c} {
			if err == nil || !strings.HasPrefix(err.Error(), "audit filter:") {
				t.Fatalf("validation=%v", err)
			}
		}
	}
	auditQuery(t, audit.Filter{Actors: []string{"crossref"}, Query: "title"})
}

// TestAuditFacetsUseOnlySupportedColumns verifies the public facet operation queries exactly its closed column set.
func TestAuditFacetsUseOnlySupportedColumns(t *testing.T) {
	var queries []string
	d := &sqlprobe.Driver{Query: func(call sqlprobe.Call) (driver.Rows, error) {
		queries = append(queries, call.SQL)
		return &sqlprobe.Rows{Names: []string{"value"}}, nil
	}}
	db := d.OpenDB()
	defer db.Close()
	if _, err := audit.New(db).Facets(t.Context(), audit.Filter{}); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 3 {
		t.Fatalf("facet queries=%v", queries)
	}
	for i, column := range []string{"actor", "action", "entity_type"} {
		want := "SELECT DISTINCT COALESCE(" + column + ", '') FROM audit_events ORDER BY " + column + " LIMIT 101"
		if queries[i] != want {
			t.Fatalf("facet query=%q, want %q", queries[i], want)
		}
	}
}
