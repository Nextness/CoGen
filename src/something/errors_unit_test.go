// errors_unit_test.go contains unit tests for errors.go: SomethingError,
// errAt, and errLoc. Tests construct error values directly without
// going through the SOMETHING pipeline.
//go:build unit

package something

import (
	"strings"
	"testing"
)

// TestEvalErrorLocation verifies eval error location.
func TestEvalErrorLocation(t *testing.T) {
	// Test that errors include location info
	err := &SomethingError{Message: "test", Line: 5, Col: 10, Filepath: "test.something"}
	msg := err.Error()
	if !strings.Contains(msg, "test.something:5:10") {
		t.Errorf("expected location in error, got %q", msg)
	}

	err2 := &SomethingError{Message: "test", Suggestion: "try this"}
	msg2 := err2.Error()
	if !strings.Contains(msg2, "suggestion: try this") {
		t.Errorf("expected suggestion, got %q", msg2)
	}

	err3 := &SomethingError{Message: "test", Line: 3, Col: 7}
	msg3 := err3.Error()
	if !strings.Contains(msg3, "line 3, col 7") {
		t.Errorf("expected line/col, got %q", msg3)
	}
}

// TestErrAtHelper verifies err at helper.
func TestErrAtHelper(t *testing.T) {
	// Direct test of the errAt helper
	tok := Token{Kind: TkIDENTIFIER, Value: "x", Line: 3, Col: 7}
	err := errAt("test error", tok, "file.something", "try this")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Message != "test error" {
		t.Errorf("expected 'test error', got %q", err.Message)
	}
	if err.Line != 3 || err.Col != 7 {
		t.Errorf("expected line=3, col=7, got line=%d, col=%d", err.Line, err.Col)
	}
	if err.Filepath != "file.something" {
		t.Errorf("expected 'file.something', got %q", err.Filepath)
	}
}
