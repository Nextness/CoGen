// validation_unit_test.go tests the shared validation rules for DOIs,
// file paths, and other workspace input constraints.
//go:build unit

package validation

import (
	"reflect"
	"testing"
)

// TestIsRealDOI verifies is real doi.
func TestIsRealDOI(t *testing.T) {
	for _, test := range []struct {
		input string
		want  bool
	}{
		{"10.1000/test", true},
		{"10.1016/j.is.2021.101765", true},
		{"  10.1000/test  ", true},
		{"", false},
		{"not-a-doi", false},
		{"https://doi.org/10.1000/test", false},
		{"10.1000/has space", false},
	} {
		if got := IsRealDOI(test.input); got != test.want {
			t.Errorf("IsRealDOI(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}

// TestValidateFields verifies validate fields.
func TestValidateFields(t *testing.T) {
	valid := Fields{DOI: "10.1000/test", Title: "Title", Year: 2024, Publisher: "Publisher", ReferenceCount: 1}
	if reasons := ValidateFields(valid, 1); len(reasons) != 0 {
		t.Fatalf("valid fields returned reasons: %v", reasons)
	}
	invalid := Fields{DOI: "invalid", Year: 0}
	want := []string{"missing title", "missing authors", "invalid year (0)", "invalid DOI (invalid)", "missing publisher", "missing references"}
	if got := ValidateFields(invalid, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("ValidateFields invalid = %v, want %v", got, want)
	}
}

// TestSortedReasons verifies sorted reasons.
func TestSortedReasons(t *testing.T) {
	got := sortedReasons(map[string]int{"b": 1, "a": 1, "c": 2})
	want := []string{"c", "a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedReasons = %v, want %v", got, want)
	}
}
