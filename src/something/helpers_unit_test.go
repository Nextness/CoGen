// helpers_unit_test.go contains unit tests for helper functions (getLocation,
// kindName, interface markers, IncludeDecl.Priv) tested directly without
// going through the SOMETHING pipeline.
//go:build unit

package something

import "testing"

// TestIncludeDeclPrivStub verifies include decl priv stub.
func TestIncludeDeclPrivStub(t *testing.T) {
	inc := &IncludeDecl{}
	if inc.Priv() {
		t.Error("IncludeDecl.Priv() should return false")
	}
}

// TestScopeBodyItemInterfaceMarkers verifies scope body item interface markers.
func TestScopeBodyItemInterfaceMarkers(t *testing.T) {
	(&VarDecl{}).scopeBodyItemMarker()
	(&ForDecl{}).scopeBodyItemMarker()
	(&InsertDecl{}).scopeBodyItemMarker()
	(&IncludeDecl{}).scopeBodyItemMarker()
	(&IterationDecl{}).scopeBodyItemMarker()
	(&AsLvalueDecl{}).scopeBodyItemMarker()
	(&ScopeDecl{}).scopeBodyItemMarker()
}

// TestGetLocationDefault verifies get location default.
func TestGetLocationDefault(t *testing.T) {
	loc := getLocation("not-a-decl")
	if loc != nil {
		t.Error("expected nil for non-decl type")
	}
}

// TestKindNameInclude verifies kind name include.
func TestKindNameInclude(t *testing.T) {
	result := kindName(KindInclude)
	if result != "include" {
		t.Errorf("expected 'include', got %q", result)
	}
}

// TestKindNameUnknown verifies kind name unknown.
func TestKindNameUnknown(t *testing.T) {
	result := kindName(NodeKind(999))
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

// TestKindNameReference verifies kind name reference.
func TestKindNameReference(t *testing.T) {
	result := kindName(KindReference)
	if result != "reference" {
		t.Errorf("expected 'reference', got %q", result)
	}
}
