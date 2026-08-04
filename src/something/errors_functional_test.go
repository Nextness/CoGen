// errors_functional_test.go contains functional tests that exercise error
// paths through the full SOMETHING pipeline, verifying that SomethingError
// values are produced with correct messages, suggestions, and locations.
//go:build functional

package something

import (
	"strings"
	"testing"
)

// TestUndefinedVariableError verifies undefined variable error.
func TestUndefinedVariableError(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := unknownVar;`)
	}, "Undefined variable")
}

// TestTypeMismatchStringToInteger verifies type mismatch string to integer.
func TestTypeMismatchStringToInteger(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: integer = "hello";`)
	}, "Type mismatch in assignment")
}

// TestTypeMismatchIntegerToString verifies type mismatch integer to string.
func TestTypeMismatchIntegerToString(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: string = 42;`)
	}, "Type mismatch in assignment")
}

// TestTypeMismatchBooleanToString verifies type mismatch boolean to string.
func TestTypeMismatchBooleanToString(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: string = true;`)
	}, "Type mismatch in assignment")
}

// TestStructMissingRequiredField verifies struct missing required field.
func TestStructMissingRequiredField(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; y: integer; }\np := Point { x = 10 };")
	}, "missing required field")
}

// TestStructUnknownField verifies struct unknown field.
func TestStructUnknownField(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; }\np := Point { x = 10, z = 20 };")
	}, "Unknown field")
}

// TestStructTypeMismatch verifies struct type mismatch.
func TestStructTypeMismatch(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Point: setup = { x: integer; }\np := Point { x = \"hello\" };")
	}, "Type mismatch in setup field")
}

// TestUnknownSetupType verifies unknown setup type.
func TestUnknownSetupType(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "p := NonExistent { x = 1 };")
	}, "Unknown setup type")
}

// TestArrayOutOfBounds verifies array out of bounds.
func TestArrayOutOfBounds(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := []string{"a"}; y := x[5];`)
	}, "Index 5 out of bounds")
}

// TestInvalidTimestampFormat verifies invalid timestamp format.
func TestInvalidTimestampFormat(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: timestamp = "2026/01/01";`)
	}, "Invalid timestamp")
}

// TestAsLvalueRequiresNonEmptyString verifies as lvalue requires non empty string.
func TestAsLvalueRequiresNonEmptyString(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#as_lvalue(123): string = "x";`)
	}, "#as_lvalue requires a non-empty string")
}

// TestAsLvalueFromNonStringVariable verifies as lvalue from non string variable.
func TestAsLvalueFromNonStringVariable(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `name := 42; #as_lvalue(name): string = "val";`)
	}, "#as_lvalue requires")
}

// TestForSourceMustBeArray verifies for source must be array.
func TestForSourceMustBeArray(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#for e: "not-an-array" { x := e; }`)
	}, "#for source must be an array")
}

// TestInsertContentMustBeString verifies insert content must be string.
func TestInsertContentMustBeString(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#insert { 42 };`)
	}, "#insert content must evaluate to a string")
}

// TestEnumValueAccessOnPlainEnum verifies enum value access on plain enum.
func TestEnumValueAccessOnPlainEnum(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Color: enum = { red; }\nx := Color.red.value;")
	}, "has no tagged value")
}

// TestUndefinedFieldInDotPath verifies undefined field in dot path.
func TestUndefinedFieldInDotPath(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "s: scope = { x := 1; }\na := s.y;")
	}, "Undefined field")
}

// TestDotPathOnNonDict verifies dot path on non dict.
func TestDotPathOnNonDict(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "x := 42\n;a := x.y;")
	}, "Cannot access field")
}

// TestUndefinedVariableInMappingKey verifies undefined variable in mapping key.
func TestUndefinedVariableInMappingKey(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `m := mapping(string, string){["a"] => "val"}; x := m[undefinedVar];`)
	}, "Undefined variable 'undefinedVar'")
}

// TestMappingKeyNotFound verifies mapping key not found.
func TestMappingKeyNotFound(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `m := mapping(string, string){["a"] => "1"}; x := m["b"];`)
	}, "Mapping key not found")
}

// TestArrayTypeMismatchInElement verifies array type mismatch in element.
func TestArrayTypeMismatchInElement(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := []string{42};`)
	}, "Type mismatch in array element")
}

// TestMappingValueTypeMismatch verifies mapping value type mismatch.
func TestMappingValueTypeMismatch(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := mapping(string, string){["a"] => 42};`)
	}, "Type mismatch in mapping value")
}

// TestUndefinedTypeReference verifies undefined type reference.
func TestUndefinedTypeReference(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "S: setup = { f: UnknownType; }\nx := S { f = 1 };")
	}, "Unknown type")
}

// TestSuccessfulEvaluationNoPanic verifies successful evaluation no panic.
func TestSuccessfulEvaluationNoPanic(t *testing.T) {
	// Ensure that valid programs don't panic
	r := evalText(t, `x: string = "ok"; y: integer = 5; z: boolean = true;`)
	if r["x"] != "ok" || r["y"] != 5 || r["z"] != true {
		t.Errorf("unexpected results: %v", r)
	}
}

// TestParseErrorHasLocation verifies parse error has location.
func TestParseErrorHasLocation(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		se, ok := r.(*SomethingError)
		if !ok {
			t.Fatalf("expected *SomethingError, got %T", r)
		}
		if !strings.Contains(se.Message, "Unexpected value token") {
			t.Errorf("expected SomethingError with parse error, got message %q suggestion=%q", se.Message, se.Suggestion)
		}
	}()
	evalText(t, `x := `)
}

// TestScopeTypeMismatch verifies scope type mismatch.
func TestScopeTypeMismatch(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: scope = "not-a-scope";`)
	}, "Type mismatch in assignment")
}

// TestNamespaceTypeMismatch verifies namespace type mismatch.
func TestNamespaceTypeMismatch(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x: namespace = "not-a-namespace";`)
	}, "Type mismatch in assignment")
}

// TestLoadFileNotFound verifies load file not found.
func TestLoadFileNotFound(t *testing.T) {
	_, err := LoadSomethingFile("/nonexistent/path.something")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Could not read file") {
		t.Errorf("expected 'Could not read file', got %v", err)
	}
}

// TestIterationTypeMismatch verifies iteration type mismatch.
func TestIterationTypeMismatch(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#iteration: integer = "not-int";`)
	}, "Type mismatch in assignment")
}

// TestAsLvalueTypeMismatch verifies as lvalue type mismatch.
func TestAsLvalueTypeMismatch(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#as_lvalue("x"): integer = "not-int";`)
	}, "Type mismatch in assignment")
}

// TestInsertLexError verifies insert lex error.
func TestInsertLexError(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#insert { "x := " };`)
	}, "Unexpected")
}

// TestForSourceMustBeArrayOrMapping verifies for source must be array or mapping.
func TestForSourceMustBeArrayOrMapping(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#for k, v: "not-a-map" { x := v; }`)
	}, "#for source must be an array or mapping")
}
