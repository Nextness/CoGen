// api_unit_test.go contains unit tests for api.go: Pprint, LoadSomethingFile,
// and LoadSomethingBytes. Unit tests call Pprint directly; functional tests
// that use LoadSomethingFile with temp files live in api_functional_test.go.
//go:build unit

package something

import (
	"strings"
	"testing"
)

// TestEvalPprint verifies eval pprint.
func TestEvalPprint(t *testing.T) {
	tests := []struct {
		input    any
		indent   int
		expected string
	}{
		{nil, 0, "null"},
		{true, 0, "true"},
		{false, 0, "false"},
		{42, 0, "42"},
		{3.14, 0, "3.14"},
		{"hello", 0, `"hello"`},
		{map[string]any{}, 0, "{}"},
		{[]any{}, 0, "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := Pprint(tt.input, tt.indent)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestEvalPprintMapWithItems verifies eval pprint map with items.
func TestEvalPprintMapWithItems(t *testing.T) {
	v := Pprint(map[string]any{"a": 1, "b": 2}, 0)
	if !strings.Contains(v, `"a": 1`) || !strings.Contains(v, `"b": 2`) {
		t.Errorf("unexpected Pprint: %q", v)
	}
}

// TestEvalPprintArrayWithItems verifies eval pprint array with items.
func TestEvalPprintArrayWithItems(t *testing.T) {
	v := Pprint([]any{"x", "y"}, 0)
	if !strings.Contains(v, `"x"`) || !strings.Contains(v, `"y"`) {
		t.Errorf("unexpected Pprint: %q", v)
	}
}

// TestEvalPprintDefault verifies eval pprint default.
func TestEvalPprintDefault(t *testing.T) {
	type custom struct{ v int }
	v := Pprint(custom{v: 5}, 0)
	if v != "{5}" {
		t.Errorf("expected '{5}', got %q", v)
	}
}
