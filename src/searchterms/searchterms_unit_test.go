// searchterms_unit_test.go tests term extraction and matching rules against
// the query shapes used in config/workspace.something.
//go:build unit

package searchterms

import (
	"reflect"
	"testing"
)

// TestParseScopusQuery verifies the Scopus TITLE-ABS-KEY query shape.
func TestParseScopusQuery(t *testing.T) {
	query := `TITLE-ABS-KEY(("BPMN" OR "BPMN 2.0" OR "Business Process Model and Notation") AND ("scheduling" OR "resource allocation" OR "resource assignment" OR "makespan" OR "workflow scheduling" OR "task scheduling" OR "genetic algorithm" OR "particle swarm" OR "ant colony" OR "simulated annealing" OR "MILP" OR "mixed integer" OR "constraint programming" OR "metaheuristic" OR "optimization" OR "BPO" OR "business process optimization" OR "multi-objective" OR "QoS" OR "quality of service" OR "cost optimization" OR "throughput" OR "cycle time" OR "formal semantics" OR "formal model" OR "formalization" OR "Petri net" OR "workflow net" OR "soundness" OR "process algebra" OR "verification" OR "model checking" OR "state space" OR "mathematical model" OR "MILP" OR "constraint programming"))`
	terms := Parse(query)
	if len(terms) != 37 {
		t.Fatalf("Parse(scopus) = %d terms, want 37: %v", len(terms), terms)
	}
	if terms[0] != "BPMN" {
		t.Fatalf("first term = %q, want BPMN", terms[0])
	}
	for _, want := range []string{"BPMN 2.0", "Business Process Model and Notation", "scheduling", "genetic algorithm", "MILP", "constraint programming", "multi-objective", "QoS", "Petri net", "state space"} {
		if !contains(terms, want) {
			t.Errorf("missing term %q in %v", want, terms)
		}
	}
	for _, unwanted := range []string{"TITLE-ABS-KEY", "AND", "OR"} {
		if contains(terms, unwanted) {
			t.Errorf("unexpected term %q in %v", unwanted, terms)
		}
	}
}

// TestParseWOSQuery verifies the WOS TS= query shape.
func TestParseWOSQuery(t *testing.T) {
	query := `TS=("BPMN" OR "BPMN 2.0" OR "Business Process Model and Notation") AND TS=("scheduling" OR "resource allocation" OR "makespan" OR "genetic algorithm" OR "MILP")`
	terms := Parse(query)
	if len(terms) != 8 {
		t.Fatalf("Parse(wos) = %d terms, want 8: %v", len(terms), terms)
	}
	if contains(terms, "TS") {
		t.Fatalf("field prefix TS kept: %v", terms)
	}
}

// TestParseIEEEXploreQuery verifies the IEEE Xplore quoted field-label shape.
func TestParseIEEEXploreQuery(t *testing.T) {
	query := `("Document Title": ("BPMN" OR "BPMN 2.0") AND ("scheduling" OR "resource allocation")) OR ("documentAbstract": ("BPMN" OR "scheduling")) OR ("authorTerms": ("BPMN" OR "genetic algorithm"))`
	terms := Parse(query)
	for _, unwanted := range []string{"Document Title", "documentAbstract", "authorTerms"} {
		if contains(terms, unwanted) {
			t.Errorf("quoted field label %q kept: %v", unwanted, terms)
		}
	}
	if len(terms) != 5 {
		t.Fatalf("Parse(ieee) = %d terms, want 5: %v", len(terms), terms)
	}
}

// TestParseWildcardQuery verifies workspace 2's wildcard-heavy WOS query shape.
func TestParseWildcardQuery(t *testing.T) {
	query := `TS=(("BPMN" OR "BPMN 2.0" OR "Business Process Model and Notation" OR "Business Process Modeling Notation" OR "Business Process Modelling Notation") AND ("mathematical model*" OR "mathematical formulation*" OR "optimization model*" OR "formal model*" OR "state space" OR "state-space model*" OR "Petri net*" OR "queueing model*" OR "Markov chain*") OR (optimi* OR "mathematical optimization" OR ILP OR MILP OR MINLP OR "constraint programming" OR "Pareto optim*") OR (heuristic* OR metaheuristic* OR matheuristic* OR "local search" OR "genetic algorithm*" OR "particle swarm" OR "ant colony" OR "simulated annealing" OR "tabu search"))`
	terms := Parse(query)
	for _, want := range []string{"mathematical model*", "state-space model*", "Petri net*", "optimi*", "ILP", "MILP", "MINLP", "Pareto optim*", "heuristic*", "metaheuristic*", "genetic algorithm*"} {
		if !contains(terms, want) {
			t.Errorf("missing wildcard term %q in %v", want, terms)
		}
	}
}

// TestParseSkipsOperators verifies operators are skipped case-insensitively.
func TestParseSkipsOperators(t *testing.T) {
	query := `"BPMN" AND "scheduling" OR NOT "x" NEAR/3 "y" W/2 "z" and "a" or "b"`
	terms := Parse(query)
	want := []string{"BPMN", "scheduling", "x", "y", "z", "a", "b"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("Parse(operators) = %v, want %v", terms, want)
	}
}

// TestParseSkipsFieldPrefixes verifies bare field prefixes are skipped.
func TestParseSkipsFieldPrefixes(t *testing.T) {
	query := `TS=("a") AND TI=("b") AND AB=("c") AND AK=("d")`
	terms := Parse(query)
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("Parse(field prefixes) = %v, want %v", terms, want)
	}
}

// TestParseKeepsBareTokens verifies bare digit-only and single-letter tokens.
func TestParseKeepsBareTokens(t *testing.T) {
	query := `"a" 2 W "b" 3.0`
	terms := Parse(query)
	want := []string{"a", "2", "b", "3.0"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("Parse(bare tokens) = %v, want %v", terms, want)
	}
}

// TestParseDeduplicatesCaseInsensitively verifies deduplication keeps the first spelling.
func TestParseDeduplicatesCaseInsensitively(t *testing.T) {
	terms := Parse(`"BPMN" OR "bpmn" OR "Bpmn"`)
	want := []string{"BPMN"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("Parse(dedupe) = %v, want %v", terms, want)
	}
}

// TestParseSkipsPunctuationOnly verifies punctuation-only tokens are skipped.
func TestParseSkipsPunctuationOnly(t *testing.T) {
	terms := Parse(`"a" , : / - ( ) "b"`)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("Parse(punctuation) = %v, want %v", terms, want)
	}
}

// TestParseEmptyInput verifies empty and punctuation-only queries yield no terms.
func TestParseEmptyInput(t *testing.T) {
	if terms := Parse(""); len(terms) != 0 {
		t.Fatalf("Parse(empty) = %v, want none", terms)
	}
	if terms := Parse("AND OR NOT , :"); len(terms) != 0 {
		t.Fatalf("Parse(operators only) = %v, want none", terms)
	}
}

// TestParseSourcesAttribution verifies deterministic source attribution and cross-source deduplication.
func TestParseSourcesAttribution(t *testing.T) {
	terms := ParseSources(map[string]string{
		"wos":    `TS=("BPMN" OR "scheduling")`,
		"scopus": `TITLE-ABS-KEY(("bpmn" OR "genetic algorithm"))`,
	})
	want := []Term{
		{Text: "bpmn", Sources: []string{"scopus", "wos"}},
		{Text: "genetic algorithm", Sources: []string{"scopus"}},
		{Text: "scheduling", Sources: []string{"wos"}},
	}
	if !reflect.DeepEqual(terms, want) {
		t.Fatalf("ParseSources = %+v, want %+v", terms, want)
	}
}

// TestParseSourcesEmptyQueries verifies empty queries contribute no terms.
func TestParseSourcesEmptyQueries(t *testing.T) {
	terms := ParseSources(map[string]string{"scopus": "", "wos": `TS=("a")`})
	if len(terms) != 1 || terms[0].Text != "a" || !reflect.DeepEqual(terms[0].Sources, []string{"wos"}) {
		t.Fatalf("ParseSources(empty) = %+v", terms)
	}
}

// TestMatchWholeWord verifies case-insensitive whole-word matching.
func TestMatchWholeWord(t *testing.T) {
	for _, tc := range []struct {
		text, term string
		want       bool
	}{
		{"BPMN 2.0 processes", "BPMN", true},
		{"using BPMN", "BPMN", true},
		{"BPMN2", "BPMN", false},
		{"SBPMN", "BPMN", false},
		{"bpmn", "BPMN", true},
		{"Business Process Model and Notation", "Business Process Model and Notation", true},
		{"business process model and notation", "Business Process Model and Notation", true},
		{"", "BPMN", false},
	} {
		if got := Match(tc.text, tc.term); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
		}
	}
}

// TestMatchPrefixWildcard verifies trailing-star prefix matching.
func TestMatchPrefixWildcard(t *testing.T) {
	for _, tc := range []struct {
		text, term string
		want       bool
	}{
		{"optimization", "optimi*", true},
		{"optimized", "optimi*", true},
		{"optimi", "optimi*", true},
		{"pessimistic", "optimi*", false},
		{"Petri nets", "Petri net*", true},
		{"Petri net", "Petri net*", true},
		{"Petri network", "Petri net*", true},
		{"Petri", "Petri net*", false},
	} {
		if got := Match(tc.text, tc.term); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
		}
	}
}

// TestMatchLeadingAndBothWildcards verifies suffix and substring wildcards.
func TestMatchLeadingAndBothWildcards(t *testing.T) {
	for _, tc := range []struct {
		text, term string
		want       bool
	}{
		{"internet", "*net", true},
		{"net", "*net", true},
		{"netting", "*net", false},
		{"internet", "*net*", true},
		{"net", "*net*", true},
		{"network", "*net*", true},
		{"netting", "*net*", true},
		{"nettles", "*net*", true},
	} {
		if got := Match(tc.text, tc.term); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
		}
	}
}

// TestMatchRegexMetacharacters verifies terms with regex metacharacters are escaped.
func TestMatchRegexMetacharacters(t *testing.T) {
	for _, tc := range []struct {
		text, term string
		want       bool
	}{
		{"C++ programs", "C++", true},
		{"version 2.0 released", "2.0", true},
		{"a+b equals c", "a+b", true},
		{"aXb equals c", "a+b", false},
	} {
		if got := Match(tc.text, tc.term); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
		}
	}
}

// TestMatchFields verifies per-field matching with per-element keyword semantics.
func TestMatchFields(t *testing.T) {
	terms := []Term{
		{Text: "BPMN"},
		{Text: "scheduling"},
		{Text: "genetic algorithm"},
		{Text: "algorithm BPMN"},
		{Text: "BPMN 2.0"},
	}
	got := MatchFields(
		"BPMN 2.0 for workflow scheduling",
		"we propose a genetic algorithm for scheduling",
		[]string{"genetic algorithm", "BPMN"},
		[]string{"BPMN", "2.0"},
		terms,
	)
	want := map[string][]string{
		"title":         {"BPMN", "scheduling", "BPMN 2.0"},
		"abstract":      {"scheduling", "genetic algorithm"},
		"keywords":      {"BPMN", "genetic algorithm"},
		"keywords_plus": {"BPMN"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchFields = %v, want %v", got, want)
	}
}

// TestMatchFieldsEmpty verifies empty fields produce empty lists.
func TestMatchFieldsEmpty(t *testing.T) {
	got := MatchFields("", "", nil, nil, []Term{{Text: "BPMN"}})
	want := map[string][]string{"title": {}, "abstract": {}, "keywords": {}, "keywords_plus": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchFields(empty) = %v, want %v", got, want)
	}
}

// contains reports whether a string slice contains an exact target.
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
