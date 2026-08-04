// Unit tests for SQL helper functions.
//go:build unit

package database

import (
	"testing"
)

// TestNullStr verifies null str.
func TestNullStr(t *testing.T) {
	if v := nullStr(""); v != nil {
		t.Errorf("expected nil for empty string, got %v", *v)
	}
	if v := nullStr("hello"); v == nil || *v != "hello" {
		t.Errorf(`expected "hello", got %v`, v)
	}
}

// TestNullInt verifies null int.
func TestNullInt(t *testing.T) {
	if v := nullInt(0); v != nil {
		t.Errorf("expected nil for zero, got %v", v)
	}
	if v := nullInt(42); v == nil || v.(int64) != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}
