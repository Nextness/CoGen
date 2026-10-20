// evaluator_unit_test.go contains unit tests for evaluator.go functions
// (runtimeTypeName, indexOf, typeRefDisplayName) called directly without going
// through the SOMETHING pipeline.

package something_test

import subject "analysis/backend/something"

import (
	"strings"
	"testing"
)

// TestTypeRefDisplayNameDefault verifies type ref display name default.
func TestTypeRefDisplayNameDefault(t *testing.T) {
	// Trigger the default case in typeRefDisplayName (returns "?")
	// This is hard to reach naturally, test directly
	result := subject.typeRefDisplayName(nil)
	if result != "?" {
		t.Errorf("expected '?', got %q", result)
	}
}

// TestTypeRefDisplayNameTypeName verifies type ref display name type name.
func TestTypeRefDisplayNameTypeName(t *testing.T) {
	// TypeName case in typeRefDisplayName - test directly since resolved types
	// are never TypeName at the point typeRefDisplayName is called
	result := subject.typeRefDisplayName(subject.TypeName("SomeName"))
	if result != "SomeName" {
		t.Errorf("expected 'SomeName', got %q", result)
	}
}

// TestRuntimeTypeNameDefault verifies the runtime type name default.
func TestRuntimeTypeNameDefault(t *testing.T) {
	result := subject.runtimeTypeName(struct{}{})
	if !strings.Contains(result, "struct") {
		t.Errorf("expected struct type name, got %q", result)
	}
}

// TestIndexOfNotFound verifies index of not found.
func TestIndexOfNotFound(t *testing.T) {
	// Test indexOf returning -1
	list := []string{"a", "b", "c"}
	if idx := subject.indexOf(list, "d"); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

// TestNewEvaluatorNilCheckedProgram verifies a nil checked program evaluates
// consistently with the evaluator's existing empty-result behavior.
func TestNewEvaluatorNilCheckedProgram(t *testing.T) {
	result := subject.NewEvaluator(nil, "").evaluate()
	if len(result) != 0 {
		t.Fatalf("expected an empty result, got %v", result)
	}
}
