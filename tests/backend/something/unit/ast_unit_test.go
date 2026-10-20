// ast_unit_test.go contains unit tests for ast.go: TypeRef, Statement,
// Expression, LValue, and Access type definitions. Tests exercise
// interface marker methods directly without going through the pipeline.

package something_test

import subject "analysis/backend/something"

import (
	"testing"
)

// TestTypeRefInterfaceMarkers verifies type ref interface markers.
func TestTypeRefInterfaceMarkers(t *testing.T) {
	// Direct calls to exercise the typeRefMarker interface compliance methods
	subject.PrimString.typeRefMarker()
	subject.TypeName("test").typeRefMarker()
	(&subject.EnumType{}).typeRefMarker()
	(&subject.SetupType{}).typeRefMarker()
	(&subject.MappingType{}).typeRefMarker()
	(&subject.ArrayType{}).typeRefMarker()
	(&subject.EnumKeyType{}).typeRefMarker()
}
