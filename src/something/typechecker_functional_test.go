// typechecker_functional_test.go contains functional tests for typechecker.go
// that exercise type resolution, assignability, struct validation, and
// expression kind checking through the full SOMETHING pipeline.
//go:build functional

package something

import (
	"testing"
)

// TestEvalTypedVar_Functional verifies eval typed var functional.
func TestEvalTypedVar_Functional(t *testing.T) {
	r := evalText(t, `x: string = "hello";`)
	if r["x"] != "hello" {
		t.Errorf("expected 'hello', got %v", r["x"])
	}
}

// TestEvalTypedVarTypeMismatch_Functional verifies eval typed var type mismatch functional.
func TestEvalTypedVarTypeMismatch_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: integer = "hello";`)
	}, "Type mismatch in assignment: expected integer, got string")
}

// TestEvalTypeMismatchFloat_Functional verifies eval type mismatch float functional.
func TestEvalTypeMismatchFloat_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: float = "hello";`)
	}, "Type mismatch in assignment: expected float, got string")
}

// TestEvalTypeMismatchBoolean_Functional verifies eval type mismatch boolean functional.
func TestEvalTypeMismatchBoolean_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: boolean = 42;`)
	}, "Type mismatch in assignment: expected boolean, got integer")
}

// TestEvalTypeMismatchTimestamp_Functional verifies eval type mismatch timestamp functional.
func TestEvalTypeMismatchTimestamp_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: timestamp = "not-a-timestamp";`)
	}, "Invalid timestamp")
}

// TestEvalValidTimestamp_Functional verifies eval valid timestamp functional.
func TestEvalValidTimestamp_Functional(t *testing.T) {
	r := evalText(t, `x: timestamp = "2026-01-01 22:10:01";`)
	if r["x"] != "2026-01-01 22:10:01" {
		t.Errorf("expected timestamp, got %v", r["x"])
	}
}

// TestEvalTypeRefResolution_Functional verifies eval type ref resolution functional.
func TestEvalTypeRefResolution_Functional(t *testing.T) {
	// Test that setups with nested types resolve correctly
	r := evalText(t, "Inner: setup = { x: integer; }\nOuter: setup = { inner: Inner; }\no := Outer { inner = Inner { x = 5 } };")
	o := r["o"].(map[string]any)
	inner := o["inner"].(map[string]any)
	if inner["x"] != 5 {
		t.Errorf("expected 5, got %v", inner["x"])
	}
}

// TestEvalTypeRefUnknown_Functional verifies eval type ref unknown functional.
func TestEvalTypeRefUnknown_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Outer: setup = { inner: UnknownType; }\no := Outer { inner = 1 };")
	}, "Unknown type 'UnknownType'")
}

// TestEvalFormatUnknownTypeSuggestion_Functional verifies eval format unknown type suggestion functional.
func TestEvalFormatUnknownTypeSuggestion_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; }\np := NonExistent { x = 1 };")
	}, "Unknown setup type 'NonExistent'")
}

// TestEvalResolveTypeRefArray_Functional verifies eval resolve type ref array functional.
func TestEvalResolveTypeRefArray_Functional(t *testing.T) {
	r := evalText(t, `x: []string = []string{"a", "b"};`)
	arr := r["x"].([]any)
	if len(arr) != 2 {
		t.Errorf("expected 2 elements, got %d", len(arr))
	}
}

// TestEvalTypedArrayRejectsWrongElementType_Functional verifies eval typed array rejects wrong element type functional.
func TestEvalTypedArrayRejectsWrongElementType_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := []string{"ok", 42};`)
	}, "Type mismatch in array element: expected string, got integer")
	assertPanic(t, func() {
		evalText(t, `x: []string = ["ok", 42];`)
	}, "Type mismatch in array element: expected string, got integer")
}

// TestEvalResolveTypeRefMapping_Functional verifies eval resolve type ref mapping functional.
func TestEvalResolveTypeRefMapping_Functional(t *testing.T) {
	r := evalText(t, `x: mapping(string, integer) = mapping(string, integer){["a"] => 1};`)
	m := r["x"].(map[string]any)
	if m["a"] != 1 {
		t.Errorf("expected 1, got %v", m["a"])
	}
}

// TestEvalTypedMappingRejectsWrongValueType_Functional verifies eval typed mapping rejects wrong value type functional.
func TestEvalTypedMappingRejectsWrongValueType_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := mapping(string, integer){["a"] => "wrong"};`)
	}, "Type mismatch in mapping value: expected integer, got string")
	assertPanic(t, func() {
		evalText(t, `x: mapping(string, integer) = { ["a"] => "wrong" };`)
	}, "Type mismatch in mapping value: expected integer, got string")
}

// TestEvalTypeCheckScope_Functional verifies eval type check scope functional.
func TestEvalTypeCheckScope_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: scope = "not-a-scope";`)
	}, "Type mismatch in assignment: expected scope, got string")
}

// TestEvalTypeCheckNamespace_Functional verifies eval type check namespace functional.
func TestEvalTypeCheckNamespace_Functional(t *testing.T) {
	// namespace type check via explicit type declaration
	assertPanic(t, func() {
		evalText(t, `x: namespace = "not-a-namespace";`)
	}, "Type mismatch in assignment: expected namespace, got string")
}

// TestEvalParseExplicitTypeScope_Functional verifies eval parse explicit type scope functional.
func TestEvalParseExplicitTypeScope_Functional(t *testing.T) {
	// scope type via explicit type
	r := evalText(t, "s: scope = { x := 1; }")
	if _, ok := r["s"]; !ok {
		t.Error("expected scope 's' in result")
	}
}

// TestEvalStructTypeCheckNested_Functional verifies eval struct type check nested functional.
func TestEvalStructTypeCheckNested_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Inner: setup = { x: integer; }\nOuter: setup = { inner: Inner; }\no := Outer { inner = Inner { x = \"not-int\" } };")
	}, "Type mismatch in setup field: expected integer, got string")
}

// TestEvalStructMissingRequired_Functional verifies eval struct missing required functional.
func TestEvalStructMissingRequired_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; y: integer; }\np := Point { x = 10 };")
	}, "missing required field")
}

// TestEvalStructUnknownField_Functional verifies eval struct unknown field functional.
func TestEvalStructUnknownField_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; }\np := Point { x = 10, z = 20 };")
	}, "Unknown field")
}

// TestEvalStructTypeMismatch_Functional verifies eval struct type mismatch functional.
func TestEvalStructTypeMismatch_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; }\np := Point { x = \"hello\" };")
	}, "Type mismatch in setup field: expected integer, got string")
}

// TestEvalStructDefault_Functional verifies eval struct default functional.
func TestEvalStructDefault_Functional(t *testing.T) {
	r := evalText(t, `Cfg: setup = { name: string; label?: string = "default"; } c := Cfg { name = "test" };`)
	c := r["c"].(map[string]any)
	if c["label"] != "default" {
		t.Errorf("expected 'default', got %v", c["label"])
	}
}

// TestResolveTypeRefArray_Functional verifies resolve type ref array functional.
func TestResolveTypeRefArray_Functional(t *testing.T) {
	// resolveTypeRef with ArrayType containing a named type
	r := evalText(t, "Inner: setup = { x: integer; }\nOuter: setup = { items: []Inner; }\no := Outer { items = []Inner{ Inner { x = 1 }, Inner { x = 2 } } };")
	o := r["o"].(map[string]any)
	items := o["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

// TestResolveTypeRefMapping_Functional verifies resolve type ref mapping functional.
func TestResolveTypeRefMapping_Functional(t *testing.T) {
	// resolveTypeRef with MappingType containing named types
	r := evalText(t, "Inner: setup = { x: integer; }\nOuter: setup = { m: mapping(string, Inner); }\no := Outer { m = mapping(string, Inner){[\"a\"] => Inner { x = 5 } } };")
	o := r["o"].(map[string]any)
	m := o["m"].(map[string]any)
	inner := m["a"].(map[string]any)
	if inner["x"] != 5 {
		t.Errorf("expected 5, got %v", inner["x"])
	}
}

// TestResolveTypeRefEnumKey_Functional verifies resolve type ref enum key functional.
func TestResolveTypeRefEnumKey_Functional(t *testing.T) {
	// resolveTypeRef with EnumKeyType - just verify the setup parses and resolves
	prog := parseText(t, "Color: enum = { red; green; blue; }\nInner: setup = { x: integer; }\nOuter: setup = { items: [Color]Inner; }")
	if len(prog.Setups) != 2 {
		t.Fatalf("expected 2 setups, got %d", len(prog.Setups))
	}
	// Verify it evaluates without error
	r := evalText(t, "Color: enum = { red; green; blue; }\nInner: setup = { x: integer; }\nOuter: setup = { items: [Color]Inner; }")
	if _, ok := r["Outer"]; ok {
		t.Error("setup declarations should not appear in result")
	}
}

// TestTypeRefDisplayNameAll_Functional verifies type ref display name all functional.
func TestTypeRefDisplayNameAll_Functional(t *testing.T) {
	// Trigger typeRefDisplayName for various types via struct field type mismatch
	// MappingType
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: mapping(string, string); }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected mapping(string, string), got integer")
	// ArrayType
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: []string; }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected []string, got integer")
}

// TestTypeRefDisplayNameMapping_Functional verifies type ref display name mapping functional.
func TestTypeRefDisplayNameMapping_Functional(t *testing.T) {
	// Trigger typeRefDisplayName for MappingType via struct field type mismatch
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: mapping(string, string); }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected mapping(string, string), got integer")
}

// TestTypeRefDisplayNameArray_Functional verifies type ref display name array functional.
func TestTypeRefDisplayNameArray_Functional(t *testing.T) {
	// Trigger typeRefDisplayName for ArrayType
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: []string; }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected []string, got integer")
}

// TestTypeRefDisplayNameEnumKey_Functional verifies type ref display name enum key functional.
func TestTypeRefDisplayNameEnumKey_Functional(t *testing.T) {
	// Trigger typeRefDisplayName for EnumKeyType
	assertPanic(t, func() {
		evalText(t, "Color: enum = { red; green; blue; }\nS: setup = { f: [Color]string; }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected [Color]string, got integer")
}

// TestValidExprKindsForTypeAll_Functional verifies valid expr kinds for type all functional.
func TestValidExprKindsForTypeAll_Functional(t *testing.T) {
	// Trigger validExprKindsForType for various types via struct field type mismatch
	// EnumType - expects KindReference
	assertPanic(t, func() {
		evalText(t, "Color: enum = { red; green; blue; }\nS: setup = { c: Color; }\nx := S { c = 42 };")
	}, "Type mismatch in setup field: expected Color, got integer")
	// SetupType - expects KindStruct or KindReference
	assertPanic(t, func() {
		evalText(t, "Inner: setup = { x: integer; }\nS: setup = { inner: Inner; }\nx := S { inner = 42 };")
	}, "Type mismatch in setup field: expected Inner, got integer")
}

// TestValidExprKindsPrimString_Functional verifies valid expr kinds prim string functional.
func TestValidExprKindsPrimString_Functional(t *testing.T) {
	// PrimString expects KindString or KindMultiline
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: string; }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected string, got integer")
}

// TestValidExprKindsPrimFloat_Functional verifies valid expr kinds prim float functional.
func TestValidExprKindsPrimFloat_Functional(t *testing.T) {
	// PrimFloat expects KindInteger or KindFloat
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: float; }\nx := S { f = true };")
	}, "Type mismatch in setup field: expected float, got boolean")
}

// TestValidExprKindsPrimTimestamp_Functional verifies valid expr kinds prim timestamp functional.
func TestValidExprKindsPrimTimestamp_Functional(t *testing.T) {
	// PrimTimestamp expects KindString
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: timestamp; }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected timestamp, got integer")
}

// TestValidExprKindsPrimBoolean_Functional verifies valid expr kinds prim boolean functional.
func TestValidExprKindsPrimBoolean_Functional(t *testing.T) {
	// PrimBoolean expects KindBoolean
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: boolean; }\nx := S { f = 42 };")
	}, "Type mismatch in setup field: expected boolean, got integer")
}

// TestFormatUnknownTypeSuggestionNoSetups_Functional verifies format unknown type suggestion no setups functional.
func TestFormatUnknownTypeSuggestionNoSetups_Functional(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: Unknown = 1;`)
	}, "Unknown type")
}

// TestResolveTypeRefUnknown_Functional verifies resolve type ref unknown functional.
func TestResolveTypeRefUnknown_Functional(t *testing.T) {
	// Trigger resolveTypeRef with an unknown type name
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: UnknownType; }\nx := S { f = 1 };")
	}, "Unknown type 'UnknownType'")
}
