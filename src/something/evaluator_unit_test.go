// evaluator_unit_test.go contains unit tests for evaluator.go functions
// (typeNameOf, indexOf, typeRefDisplayName) called directly without going
// through the SOMETHING pipeline.
//go:build unit

package something

import (
	"strings"
	"testing"
)

// TestTypeRefDisplayNameDefault verifies type ref display name default.
func TestTypeRefDisplayNameDefault(t *testing.T) {
	// Trigger the default case in typeRefDisplayName (returns "?")
	// This is hard to reach naturally, test directly
	result := typeRefDisplayName(nil)
	if result != "?" {
		t.Errorf("expected '?', got %q", result)
	}
}

// TestTypeRefDisplayNameTypeName verifies type ref display name type name.
func TestTypeRefDisplayNameTypeName(t *testing.T) {
	// TypeName case in typeRefDisplayName - test directly since resolved types
	// are never TypeName at the point typeRefDisplayName is called
	result := typeRefDisplayName(TypeName("SomeName"))
	if result != "SomeName" {
		t.Errorf("expected 'SomeName', got %q", result)
	}
}

// TestTypeNameOfDefault verifies type name of default.
func TestTypeNameOfDefault(t *testing.T) {
	// Trigger the default case in typeNameOf (unexpected Go type)
	result := typeNameOf(struct{}{})
	if !strings.Contains(result, "struct") {
		t.Errorf("expected struct type name, got %q", result)
	}
}

// TestIndexOfNotFound verifies index of not found.
func TestIndexOfNotFound(t *testing.T) {
	// Test indexOf returning -1
	list := []string{"a", "b", "c"}
	if idx := indexOf(list, "d"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

// TestNewEvaluatorNilCheckedProgram verifies a nil checked program evaluates
// consistently with the evaluator's existing empty-result behavior.
func TestNewEvaluatorNilCheckedProgram(t *testing.T) {
	result := NewEvaluator(nil, "").evaluate()
	if len(result) != 0 {
		t.Fatalf("expected an empty result, got %v", result)
	}
}
