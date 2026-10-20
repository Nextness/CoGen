// utils_functional_test.go contains functional tests for utils.go that
// exercise path-walking accessors through a real evaluated config.

package something_test

import subject "analysis/backend/something"

import (
	"testing"
)

// TestUtilsWithEvaluatedConfig verifies utils with evaluated config.
func TestUtilsWithEvaluatedConfig(t *testing.T) {
	input := `Point: setup = { x: integer; y: integer; }
#iteration("_db"): string = "first";
#iteration("_db"): string = "second";
s: scope = { items := []string{"a", "b", "c"}; }
x := Point { x = 10, y = 20 };
`
	tokens := subject.NewLexer(input, "").Tokenize()
	syntax := subject.NewParser(tokens, "").ParseProgram()
	expanded := subject.NewDirectiveGenerator("").Expand(syntax)
	checked := subject.NewTypeChecker(expanded, "").Check()
	ev := subject.NewEvaluator(checked, "")
	data := ev.evaluate()

	// Test _index with iteration wildcard
	v2, err := subject.GetStringIndex(data, 0, "[iteration]_db")
	if err != nil {
		t.Fatalf("GetStringIndex failed: %v", err)
	}
	if v2 != "first" {
		t.Errorf("expected 'first', got %q", v2)
	}

	v2b, err := subject.GetStringIndex(data, 1, "[iteration]_db")
	if err != nil {
		t.Fatalf("GetStringIndex failed: %v", err)
	}
	if v2b != "second" {
		t.Errorf("expected 'second', got %q", v2b)
	}

	// Test _all with iteration wildcard
	vals, err := subject.GetStringAll(data, "[iteration]_db")
	if err != nil {
		t.Fatalf("GetStringAll failed: %v", err)
	}
	if len(vals) != 2 || vals[0] != "first" || vals[1] != "second" {
		t.Errorf("expected ['first', 'second'], got %v", vals)
	}

	// Test scope access
	scope, err := subject.GetScopeOnce(data, "s")
	if err != nil {
		t.Fatalf("GetScopeOnce failed: %v", err)
	}
	if _, ok := scope["items"]; !ok {
		t.Error("expected 'items' in scope")
	}

	// Test struct value
	xVal, err := subject.GetStructOnce(data, "x")
	if err != nil {
		t.Fatalf("GetStructOnce failed: %v", err)
	}
	if xVal["x"] != 10 || xVal["y"] != 20 {
		t.Errorf("expected {x:10, y:20}, got %v", xVal)
	}

	// Test integer inside struct
	xInt, err := subject.GetIntegerOnce(data, "x", "x")
	if err != nil {
		t.Fatalf("GetIntegerOnce failed: %v", err)
	}
	if xInt != 10 {
		t.Errorf("expected 10, got %d", xInt)
	}
}
