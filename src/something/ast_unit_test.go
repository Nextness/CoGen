// ast_unit_test.go contains unit tests for ast.go: TypeRef, Statement,
// Expression, LValue, and Access type definitions. Tests exercise
// interface marker methods directly without going through the pipeline.
//go:build unit

package something

import (
	"testing"
)

// TestTypeRefInterfaceMarkers verifies type ref interface markers.
func TestTypeRefInterfaceMarkers(t *testing.T) {
	// Direct calls to exercise the typeRefMarker interface compliance methods
	PrimString.typeRefMarker()
	TypeName("test").typeRefMarker()
	(&EnumType{}).typeRefMarker()
	(&SetupType{}).typeRefMarker()
	(&MappingType{}).typeRefMarker()
	(&ArrayType{}).typeRefMarker()
	(&EnumKeyType{}).typeRefMarker()
}
