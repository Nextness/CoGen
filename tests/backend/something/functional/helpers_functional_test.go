// helpers_functional_test.go contains functional tests for helper functions
// that exercise kindName through the full SOMETHING pipeline.

package something_test

import testhelpers "analysis/tests/support/go/backend/something"

import (
	"testing"
)

// TestKindNameAll verifies kind name all.
func TestKindNameAll(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.EvalText(t, "S: setup = { f: integer; }\nx := S { f = #multiline EOF\nhello\nEOF\n };")
	}, "Type mismatch in setup field: expected integer, got string")
	testhelpers.AssertPanic(t, func() {
		testhelpers.EvalText(t, "S: setup = { f: integer; }\nx := S { f = mapping(string, string){[\"a\"] => \"b\"} };")
	}, "Type mismatch in setup field: expected integer, got mapping(string, string)")
	testhelpers.AssertPanic(t, func() {
		testhelpers.EvalText(t, "S: setup = { f: integer; }\nx := S { f = []string{\"a\"} };")
	}, "Type mismatch in setup field: expected integer, got []string")
	testhelpers.AssertPanic(t, func() {
		testhelpers.EvalText(t, "S: setup = { f: integer; }\nx := S { f = { a = 1 } };")
	}, "Type mismatch in setup field: expected integer, got scope")
}

// TestKindNameFloat verifies kind name float.
func TestKindNameFloat(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.EvalText(t, "S: setup = { f: string; }\nx := S { f = 3.14 };")
	}, "Type mismatch in setup field: expected string, got float")
}
