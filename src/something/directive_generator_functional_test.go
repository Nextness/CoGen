// directive_generator_functional_test.go contains functional tests for
// directive_generator.go that exercise #for, #insert, #iteration,
// #as_lvalue, and #priv directives through the full SOMETHING pipeline.
//go:build functional

package something

import (
	"strings"
	"testing"
)

// TestEvalForArray_Functional verifies eval for array functional.
func TestEvalForArray_Functional(t *testing.T) {
	r := evalText(t, `x := ""; #for elem: ["a","b"] { x = elem; }`)
	// Each expansion explicitly reassigns the existing declaration.
	if r["x"] != "b" {
		t.Errorf("expected 'b', got %v", r["x"])
	}
}

// TestEvalInsert_Functional verifies eval insert functional.
func TestEvalInsert_Functional(t *testing.T) {
	r := evalText(t, `#insert { "x := 42;" };`)
	if r["x"] != 42 {
		t.Errorf("expected 42, got %v", r["x"])
	}
}

// TestEvalInsertInterpolation_Functional verifies eval insert interpolation functional.
func TestEvalInsertInterpolation_Functional(t *testing.T) {
	r := evalText(t, `a := "hello"; #insert { "b := \"{a}_world\";" };`)
	if r["b"] != "hello_world" {
		t.Errorf("expected 'hello_world', got %q", r["b"])
	}
}

// TestEvalInsertMultipleValues_Functional verifies eval insert multiple values functional.
func TestEvalInsertMultipleValues_Functional(t *testing.T) {
	r := evalText(t, `#insert { "a := 1;", "b := 2;" };`)
	if r["a"] != 1 || r["b"] != 2 {
		t.Fatalf("expected both inserted declarations, got %v", r)
	}
}

// TestEvalIteration_Functional verifies eval iteration functional.
func TestEvalIteration_Functional(t *testing.T) {
	r := evalText(t, `#iteration: string = "a"; #iteration: string = "b";`)
	// iteration keys are auto-generated
	found := false
	for k, v := range r {
		if strings.HasPrefix(k, "iteration_") && v == "a" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected iteration_* key with value 'a', got %v", r)
	}
}

// TestEvalIterationPeek_Functional verifies eval iteration peek functional.
func TestEvalIterationPeek_Functional(t *testing.T) {
	r := evalText(t, `#iteration: string = "a"; x := #iteration;`)
	// x should get the NEXT iteration key (iteration_0000000001)
	if !strings.HasPrefix(r["x"].(string), "iteration_") {
		t.Errorf("expected iteration key, got %v", r["x"])
	}
}

// TestEvalAsLvalue_Functional verifies eval as lvalue functional.
func TestEvalAsLvalue_Functional(t *testing.T) {
	r := evalText(t, `#as_lvalue("target"): string = "value";`)
	if r["target"] != "value" {
		t.Errorf("expected 'value', got %v", r["target"])
	}
}

// TestEvalAsLvalueFromVar_Functional verifies eval as lvalue from var functional.
func TestEvalAsLvalueFromVar_Functional(t *testing.T) {
	r := evalText(t, `name := "dyn"; #as_lvalue(name): string = "resolved";`)
	if r["dyn"] != "resolved" {
		t.Errorf("expected 'resolved', got %v", r["dyn"])
	}
}

// TestEvalScopeFromIteration_Functional verifies eval scope from iteration functional.
func TestEvalScopeFromIteration_Functional(t *testing.T) {
	r := evalText(t, "#iteration: scope = {\n  x := 1\n;}")
	// The scope should be in the result as an auto-generated key
	found := false
	for k, v := range r {
		if strings.HasPrefix(k, "iteration_") {
			if scope, ok := v.(map[string]any); ok && scope["x"] == 1 {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected iteration_* scope with x=1, got %v", r)
	}
}

// TestEvalNestedIterationScope_Functional verifies eval nested iteration scope functional.
func TestEvalNestedIterationScope_Functional(t *testing.T) {
	r := evalText(t, "#iteration: scope = {\n  #iteration: scope = {\n    a := \"deep\"\n; }\n}")
	// Should have iteration_0000000000 with iteration_0000000001 inside
	found := false
	for k, v := range r {
		if strings.HasPrefix(k, "iteration_") {
			if outer, ok := v.(map[string]any); ok {
				for k2, v2 := range outer {
					if strings.HasPrefix(k2, "iteration_") {
						if inner, ok := v2.(map[string]any); ok && inner["a"] == "deep" {
							found = true
						}
					}
				}
			}
		}
	}
	if !found {
		t.Errorf("expected nested iteration scopes with a='deep', got %v", r)
	}
}

// TestEvalIterationWithLabel_Functional verifies eval iteration with label functional.
func TestEvalIterationWithLabel_Functional(t *testing.T) {
	r := evalText(t, `#iteration("_my_label"): string = "val";`)
	found := false
	for k, v := range r {
		if strings.HasPrefix(k, "iteration_") && strings.Contains(k, "_my_label") && v == "val" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected iteration key with label '_my_label', got %v", r)
	}
}

// TestEvalIterationLabelsHaveIndependentCounters_Functional verifies eval iteration labels have independent counters functional.
func TestEvalIterationLabelsHaveIndependentCounters_Functional(t *testing.T) {
	r := evalText(t, `
#iteration: string = "plain-0";
#iteration("_a"): string = "a-0";
#iteration("_b"): string = "b-0";
#iteration("_a"): string = "a-1";
#iteration: string = "plain-1";
`)
	want := map[string]any{
		"iteration_0000000000":   "plain-0",
		"iteration_0000000001":   "plain-1",
		"iteration_0000000000_a": "a-0",
		"iteration_0000000001_a": "a-1",
		"iteration_0000000000_b": "b-0",
	}
	for key, value := range want {
		if r[key] != value {
			t.Errorf("%s: expected %q, got %v (all: %v)", key, value, r[key], r)
		}
	}
}

// TestEvalBareScopeDoesNotDoubleConsumeIterationCounters_Functional verifies eval bare scope does not double consume iteration counters functional.
func TestEvalBareScopeDoesNotDoubleConsumeIterationCounters_Functional(t *testing.T) {
	r := evalText(t, `s: scope = { #iteration("_inside"): string = "value"; }`)
	s := r["s"].(map[string]any)
	if s["iteration_0000000000_inside"] != "value" {
		t.Fatalf("expected first labeled counter in bare scope, got %v", s)
	}
}

// TestEvalForMappingIteration_Functional verifies eval for mapping iteration functional.
func TestEvalForMappingIteration_Functional(t *testing.T) {
	r := evalText(t, `m := mapping(string, string){["a"] => "1", ["b"] => "2"};
result := "";
#for k, v: m { result = "{k}={v}"; }`)
	if r["result"] != "b=2" {
		t.Errorf("expected deterministic final value 'b=2', got %v", r["result"])
	}
}

// TestEvalForMapIntKey_Functional verifies eval for map int key functional.
func TestEvalForMapIntKey_Functional(t *testing.T) {
	// #for with a map[int]any source (enum-indexed mapping)
	r := evalText(t, "Color: enum = { red; green; blue; }\nm := mapping(Color, string){[.red] => \"a\", [.green] => \"b\"}\n;result := \"\"\n;#for k, v: m { result = \"{k}={v}\"; }")
	if r["result"] != "1=b" {
		t.Errorf("expected deterministic final value '1=b', got %v", r["result"])
	}
}

// TestEvalForSourceError_Functional verifies eval for source error functional.
func TestEvalForSourceError_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#for elem: "not-an-array" { x := elem; }`)
	}, "#for source must be an array")
}

// TestEvalForMappingError_Functional verifies eval for mapping error functional.
func TestEvalForMappingError_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#for k, v: "not-a-map" { x := v; }`)
	}, "#for source must be an array or mapping")
}

// TestEvalForSourceArray_Functional verifies eval for source array functional.
func TestEvalForSourceArray_Functional(t *testing.T) {
	// #for with an array literal as source
	r := evalText(t, `x := ""; #for elem: ["a", "b"] { x = elem; }`)
	if r["x"] != "b" {
		t.Errorf("expected 'b', got %v", r["x"])
	}
}

// TestEvalScopeBodyForDecl_Functional verifies eval scope body for decl functional.
func TestEvalScopeBodyForDecl_Functional(t *testing.T) {
	// #for inside a scope body
	r := evalText(t, "s: scope = {\n  z := \"\"\n  ;#for elem: [\"x\", \"y\"] { z = elem; }\n}")
	s := r["s"].(map[string]any)
	if s["z"] != "y" {
		t.Errorf("expected 'y', got %v", s["z"])
	}
}

// TestEvalScopeBodyInsertDecl_Functional verifies eval scope body insert decl functional.
func TestEvalScopeBodyInsertDecl_Functional(t *testing.T) {
	// #insert inside a scope body
	r := evalText(t, "s: scope = {\n  #insert { \"z := 42;\" }\n;}")
	s := r["s"].(map[string]any)
	if s["z"] != 42 {
		t.Errorf("expected 42, got %v", s["z"])
	}
}

// TestEvalInsertErrorNotString_Functional verifies eval insert error not string functional.
func TestEvalInsertErrorNotString_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#insert { 42 };`)
	}, "#insert content must evaluate to a string")
}

// TestEvalScopeBodyIterationTyped_Functional verifies eval scope body iteration typed functional.
func TestEvalScopeBodyIterationTyped_Functional(t *testing.T) {
	r := evalText(t, "s: scope = {\n  #iteration: string = \"hello\"\n;}")
	s, ok := r["s"].(map[string]any)
	if !ok {
		t.Fatalf("expected 's' to be a map, got %T", r["s"])
	}
	found := false
	for k, v := range s {
		if strings.HasPrefix(k, "iteration_") && v == "hello" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected iteration_* key with 'hello' in scope, got %v", s)
	}
}

// TestEvalScopeBodyAsLvalueTyped_Functional verifies eval scope body as lvalue typed functional.
func TestEvalScopeBodyAsLvalueTyped_Functional(t *testing.T) {
	r := evalText(t, "s: scope = {\n  #as_lvalue(\"dyn\"): string = \"val\"\n;}")
	s, ok := r["s"].(map[string]any)
	if !ok {
		t.Fatalf("expected 's' to be a map, got %T", r["s"])
	}
	if s["dyn"] != "val" {
		t.Errorf("expected 'val', got %v", s["dyn"])
	}
}

// TestParseAsLvalueError_Functional verifies parse as lvalue error functional.
func TestParseAsLvalueError_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#as_lvalue(123): string = "x";`)
	}, "#as_lvalue requires a non-empty string")
}

// TestEvalResolveAsLvalueFromString_Functional verifies eval resolve as lvalue from string functional.
func TestEvalResolveAsLvalueFromString_Functional(t *testing.T) {
	// as_lvalue using a string literal name
	r := evalText(t, `#as_lvalue("target"): string = "worked";`)
	if r["target"] != "worked" {
		t.Errorf("expected 'worked', got %v", r["target"])
	}
}

// TestEvalPrivIteration_Functional verifies eval priv iteration functional.
func TestEvalPrivIteration_Functional(t *testing.T) {
	// #priv #iteration should not appear in the result
	r := evalText(t, `#priv #iteration: string = "hidden";`)
	for k := range r {
		if strings.HasPrefix(k, "iteration_") {
			t.Errorf("priv iteration should not be in result, found %s", k)
		}
	}
}

// TestEvalPrivAsLvalue_Functional verifies eval priv as lvalue functional.
func TestEvalPrivAsLvalue_Functional(t *testing.T) {
	// #priv #as_lvalue should not appear in the result
	r := evalText(t, `#priv #as_lvalue("x"): string = "hidden";`)
	if _, ok := r["x"]; ok {
		t.Error("priv as_lvalue should not be in result")
	}
}

// TestEvalPrivIterationInScope_Functional verifies eval priv iteration in scope functional.
func TestEvalPrivIterationInScope_Functional(t *testing.T) {
	// #priv #iteration in a scope should not appear in scope's pubs
	r := evalText(t, "s: scope = {\n  #priv #iteration: string = \"hidden\"\n;}")
	s := r["s"].(map[string]any)
	for k := range s {
		if strings.HasPrefix(k, "iteration_") {
			t.Errorf("priv iteration in scope should not be in result, found %s", k)
		}
	}
}

// TestEvalPrivAsLvalueInScope_Functional verifies eval priv as lvalue in scope functional.
func TestEvalPrivAsLvalueInScope_Functional(t *testing.T) {
	// #priv #as_lvalue in a scope should not appear in scope's pubs
	r := evalText(t, "s: scope = {\n  #priv #as_lvalue(\"x\"): string = \"hidden\"\n;}")
	s := r["s"].(map[string]any)
	if _, ok := s["x"]; ok {
		t.Error("priv as_lvalue in scope should not be in result")
	}
}

// TestResolveAsLvalueNameAll_Functional verifies resolve as lvalue name all functional.
func TestResolveAsLvalueNameAll_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `name := 42; #as_lvalue(name): string = "val";`)
	}, "#as_lvalue requires a non-empty string")
	assertPanic(t, func() {
		evalText(t, `#as_lvalue(undefinedVar): string = "val";`)
	}, "Undefined variable 'undefinedVar'")
}

// TestResolveAsLvalueNameError_Functional verifies resolve as lvalue name error functional.
func TestResolveAsLvalueNameError_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#as_lvalue(42) := "value";`)
	}, "#as_lvalue requires")
}
