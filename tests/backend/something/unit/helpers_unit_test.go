// helpers_unit_test.go contains unit tests for helper functions (getLocation,
// kindName, interface markers, IncludeDecl.Priv) tested directly without
// going through the SOMETHING pipeline.

package something_test

import testhelpers "analysis/tests/support/go/backend/something"

import "testing"

// TestIncludeDeclPrivStub verifies include decl priv stub.
func TestIncludeDeclPrivStub(t *testing.T) {
	inc := &testhelpers.IncludeDecl{}
	if inc.Priv() {
		t.Error("IncludeDecl.Priv() should return false")
	}
}

// TestGetLocationDefault verifies get location default.
func TestGetLocationDefault(t *testing.T) {
	loc := testhelpers.GetLocation("not-a-decl")
	if loc != nil {
		t.Error("expected nil for non-decl type")
	}
}

// TestKindNameInclude verifies kind name include.
func TestKindNameInclude(t *testing.T) {
	result := testhelpers.KindName(testhelpers.KindInclude)
	if result != "include" {
		t.Errorf("expected 'include', got %q", result)
	}
}

// TestKindNameUnknown verifies kind name unknown.
func TestKindNameUnknown(t *testing.T) {
	result := testhelpers.KindName(testhelpers.NodeKind(999))
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

// TestKindNameReference verifies kind name reference.
func TestKindNameReference(t *testing.T) {
	result := testhelpers.KindName(testhelpers.KindReference)
	if result != "reference" {
		t.Errorf("expected 'reference', got %q", result)
	}
}
