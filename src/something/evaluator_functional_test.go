// evaluator_functional_test.go contains functional tests for evaluator.go
// that exercise literal evaluation, interpolation, multiline strings,
// arrays, mappings, enums, structs, scopes, variable resolution, dot-path
// access, private variables, and error paths through the full pipeline.
//go:build functional

package something

import (
	"strings"
	"testing"
)

// Core literal tests

// TestEvalStringLiteral verifies eval string literal.
func TestEvalStringLiteral(t *testing.T) {
	r := evalText(t, `x := "hello";`)
	if r["x"] != "hello" {
		t.Errorf("expected 'hello', got %v", r["x"])
	}
}

// TestEvalIntegerLiteral verifies eval integer literal.
func TestEvalIntegerLiteral(t *testing.T) {
	r := evalText(t, "x := 42;")
	if r["x"] != 42 {
		t.Errorf("expected 42, got %v", r["x"])
	}
}

// TestEvalFloatLiteral verifies eval float literal.
func TestEvalFloatLiteral(t *testing.T) {
	r := evalText(t, "x := 3.14;")
	if r["x"] != 3.14 {
		t.Errorf("expected 3.14, got %v", r["x"])
	}
}

// TestEvalBooleanTrue verifies eval boolean true.
func TestEvalBooleanTrue(t *testing.T) {
	r := evalText(t, "x := true;")
	if r["x"] != true {
		t.Errorf("expected true, got %v", r["x"])
	}
}

// Interpolation tests

// TestEvalInterpolation verifies eval interpolation.
func TestEvalInterpolation(t *testing.T) {
	r := evalText(t, `a := "world"; b := "hello {a}";`)
	if r["b"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["b"])
	}
}

// TestEvalInterpolationInteger verifies eval interpolation integer.
func TestEvalInterpolationInteger(t *testing.T) {
	r := evalText(t, `a := 42; b := "the answer is {a}";`)
	if r["b"] != "the answer is 42" {
		t.Errorf("expected 'the answer is 42', got %q", r["b"])
	}
}

// TestEvalInterpolationFloat verifies eval interpolation float.
func TestEvalInterpolationFloat(t *testing.T) {
	r := evalText(t, `a := 3.14; b := "pi is {a}";`)
	if r["b"] != "pi is 3.14" {
		t.Errorf("expected 'pi is 3.14', got %q", r["b"])
	}
}

// TestEvalInterpolationBoolean verifies eval interpolation boolean.
func TestEvalInterpolationBoolean(t *testing.T) {
	r := evalText(t, `a := true; b := "flag is {a}";`)
	if r["b"] != "flag is true" {
		t.Errorf("expected 'flag is true', got %q", r["b"])
	}
}

// Multiline tests

// TestEvalMultiline verifies eval multiline.
func TestEvalMultiline(t *testing.T) {
	r := evalText(t, "x := #multiline EOF\nhello world\nEOF\n;")
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["x"])
	}
}

// TestEvalMultilineComment verifies multiline comments are removed from the value.
func TestEvalMultilineComment(t *testing.T) {
	r := evalText(t, "x := #multiline EOF\nhello // comment\nworld\nEOF\n;")
	if r["x"] != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", r["x"])
	}
}

// TestEvalMultilineEscapedSlash verifies \/\/ evaluates to a literal //.
func TestEvalMultilineEscapedSlash(t *testing.T) {
	r := evalText(t, "x := #multiline EOF\nhello \\/\\/ world\nEOF\n;")
	if r["x"] != "hello // world" {
		t.Errorf("expected 'hello // world', got %q", r["x"])
	}
}

// TestEvalMultilineCommentStripSpaces verifies comments are removed before strip_spaces.
func TestEvalMultilineCommentStripSpaces(t *testing.T) {
	r := evalText(t, "x := #multiline (strip_spaces) EOF\nhello // comment\nworld\nEOF\n;")
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["x"])
	}
}

// TestEvalMultilineNoNewline verifies eval multiline no newline.
func TestEvalMultilineNoNewline(t *testing.T) {
	r := evalText(t, "x := #multiline (no_newline) EOF\nhello\nworld\nEOF\n;")
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["x"])
	}
}

// TestEvalMultilineNoIndent verifies eval multiline no indent.
func TestEvalMultilineNoIndent(t *testing.T) {
	r := evalText(t, "x := #multiline (no_indent) EOF\n    hello\n    world\nEOF\n;")
	if r["x"] != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", r["x"])
	}
}

// TestEvalMultilineStripSpaces verifies eval multiline strip spaces.
func TestEvalMultilineStripSpaces(t *testing.T) {
	r := evalText(t, "x := #multiline (strip_spaces) EOF\nhello   world\nEOF\n;")
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["x"])
	}
}

// TestEvalMultilineCombined verifies eval multiline combined.
func TestEvalMultilineCombined(t *testing.T) {
	r := evalText(t, "x := #multiline (no_newline|strip_spaces) EOF\nhello   world\nfoo   bar\nEOF\n;")
	if r["x"] != "hello world foo bar" {
		t.Errorf("expected 'hello world foo bar', got %q", r["x"])
	}
}

// TestEvalMultilineInterpolation verifies eval multiline interpolation.
func TestEvalMultilineInterpolation(t *testing.T) {
	r := evalText(t, "name := \"world\"\n;x := #multiline EOF\nhello {name}\nEOF\n;")
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["x"])
	}
}

// TestEvalMultilineInterpolationDotPath verifies eval multiline interpolation dot path.
func TestEvalMultilineInterpolationDotPath(t *testing.T) {
	r := evalText(t, "s: scope = { n := 42; }\nx := #multiline EOF\nthe answer is {s.n}\nEOF\n;")
	if r["x"] != "the answer is 42" {
		t.Errorf("expected 'the answer is 42', got %q", r["x"])
	}
}

// TestEvalMultilineWithAllParams verifies eval multiline with all params.
func TestEvalMultilineWithAllParams(t *testing.T) {
	r := evalText(t, "x := #multiline (no_indent|no_newline|strip_spaces) EOF\n    hello\n    world\nEOF\n;")
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %q", r["x"])
	}
}

// Array tests

// TestEvalArray verifies eval array.
func TestEvalArray(t *testing.T) {
	r := evalText(t, `x := []string{"a", "b", "c"};`)
	arr, ok := r["x"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", r["x"])
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
		t.Errorf("unexpected array: %v", arr)
	}
}

// TestEvalArrayIndexAccess verifies eval array index access.
func TestEvalArrayIndexAccess(t *testing.T) {
	r := evalText(t, `x := []string{"a", "b", "c"}; y := x[0];`)
	if r["y"] != "a" {
		t.Errorf("expected 'a', got %v", r["y"])
	}
}

// TestEvalArrayOutOfBounds verifies eval array out of bounds.
func TestEvalArrayOutOfBounds(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := []string{"a"}; y := x[5];`)
	}, "Index 5 out of bounds")
}

// Mapping tests

// TestEvalMapping verifies eval mapping.
func TestEvalMapping(t *testing.T) {
	r := evalText(t, `x := mapping(string, string){["a"] => "x", ["b"] => "y"};`)
	m, ok := r["x"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", r["x"])
	}
	if m["a"] != "x" || m["b"] != "y" {
		t.Errorf("unexpected mapping: %v", m)
	}
}

// TestEvalMappingKeyAccess verifies eval mapping key access.
func TestEvalMappingKeyAccess(t *testing.T) {
	r := evalText(t, `x := mapping(string, string){["a"] => "x"}; y := x["a"];`)
	if r["y"] != "x" {
		t.Errorf("expected 'x', got %v", r["y"])
	}
}

// TestEvalCombinedFieldAndIndexAccess verifies eval combined field and index access.
func TestEvalCombinedFieldAndIndexAccess(t *testing.T) {
	r := evalText(t, `
Item: setup = { value: integer; }
items := [Item { value = 7 }];
matrix := [[1, 2], [3, 4]];
a := items[0].value;
b := matrix[1][0];
`)
	if r["a"] != 7 || r["b"] != 3 {
		t.Fatalf("unexpected combined access result: %v", r)
	}
}

// TestEvalQualifiedEnumMappingKey verifies eval qualified enum mapping key.
func TestEvalQualifiedEnumMappingKey(t *testing.T) {
	r := evalText(t, `Color: enum = { red; green; } m := mapping(Color, string){[Color.green] => "yes"}; x := m[Color.green];`)
	if r["x"] != "yes" {
		t.Fatalf("expected qualified enum mapping key to resolve, got %v", r)
	}
}

// TestEvalEmptyMappingUsesStringKeys verifies eval empty mapping uses string keys.
func TestEvalEmptyMappingUsesStringKeys(t *testing.T) {
	r := evalText(t, `m := mapping(string, string){};`)
	if _, ok := r["m"].(map[string]any); !ok {
		t.Fatalf("expected empty string-keyed mapping, got %T", r["m"])
	}
}

// TestEvalMappingEnumKeys verifies eval mapping enum keys.
func TestEvalMappingEnumKeys(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; blue; }\nx := mapping(Color, integer){[.red] => 1, [.green] => 2, [.blue] => 3};")
	m, ok := r["x"].(map[int]any)
	if !ok {
		t.Fatalf("expected map[int]any, got %T", r["x"])
	}
	if m[0] != 1 || m[1] != 2 || m[2] != 3 {
		t.Errorf("unexpected mapping: %v", m)
	}
}

// TestEvalMappingEnumKeyWithDotPrefix verifies eval mapping enum key with dot prefix.
func TestEvalMappingEnumKeyWithDotPrefix(t *testing.T) {
	// Test mapping with enum keys that have . prefix
	r := evalText(t, "Color: enum = { red; green; blue; }\nx := mapping(Color, string){[.red] => \"r\", [.green] => \"g\"};")
	m, ok := r["x"].(map[int]any)
	if !ok {
		t.Fatalf("expected map[int]any, got %T", r["x"])
	}
	if m[0] != "r" || m[1] != "g" {
		t.Errorf("unexpected mapping: %v", m)
	}
}

// TestEvalMappingKeyAccessEnumIndex verifies eval mapping key access enum index.
func TestEvalMappingKeyAccessEnumIndex(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; blue; }\nm := mapping(Color, string){[.red] => \"r\", [.green] => \"g\"}\n;x := m[.red];")
	if r["x"] != "r" {
		t.Errorf("expected 'r', got %v", r["x"])
	}
}

// TestEvalMappingCompositeKeys verifies eval mapping composite keys.
func TestEvalMappingCompositeKeys(t *testing.T) {
	r := evalText(t, `x := mapping(string, integer){[["red", "green"]] => 1};`)
	m := r["x"].(map[string]any)
	if m["red,green"] != 1 {
		t.Errorf("expected 1 at key 'red,green', got %v", m["red,green"])
	}
}

// TestEvalMappingCompositeStringKeys verifies eval mapping composite string keys.
func TestEvalMappingCompositeStringKeys(t *testing.T) {
	// Composite keys with string values (not enum)
	r := evalText(t, `x := mapping(string, string){[["a", "b"]] => "composite"};`)
	m := r["x"].(map[string]any)
	if m["a,b"] != "composite" {
		t.Errorf("expected 'composite', got %v", m["a,b"])
	}
}

// TestEvalMappingStringKeyWithDot verifies eval mapping string key with dot.
func TestEvalMappingStringKeyWithDot(t *testing.T) {
	// String key that looks like an enum member - should be treated as string
	r := evalText(t, `x := mapping(string, string){["red"] => "val"};`)
	m := r["x"].(map[string]any)
	if m["red"] != "val" {
		t.Errorf("expected 'val', got %v", m["red"])
	}
}

// Enum tests

// TestEvalEnumPlain verifies eval enum plain.
func TestEvalEnumPlain(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; blue; }\nx := Color.red;")
	if r["x"] != 0 {
		t.Errorf("expected 0 for .red, got %v", r["x"])
	}
}

// TestEvalEnumShorthand verifies eval enum shorthand.
func TestEvalEnumShorthand(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; blue; }\nx: Color = .green;")
	if r["x"] != 1 {
		t.Errorf("expected 1 for .green, got %v", r["x"])
	}
}

// TestEvalEnumTaggedValue verifies eval enum tagged value.
func TestEvalEnumTaggedValue(t *testing.T) {
	r := evalText(t, `Status: enum(string) = { ok = "good"; err = "bad"; } x := Status.ok.value;`)
	if r["x"] != "good" {
		t.Errorf("expected 'good', got %v", r["x"])
	}
}

// TestEvalEnumTaggedStructValue verifies eval enum tagged struct value.
func TestEvalEnumTaggedStructValue(t *testing.T) {
	r := evalText(t, "Point: setup = { x: integer; y: integer; }\nShape: enum(Point) = { circle = Point { x = 10, y = 20 }; square = Point { x = 5, y = 5 }; }\nv := Shape.circle.value;")
	v, ok := r["v"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", r["v"])
	}
	if v["x"] != 10 || v["y"] != 20 {
		t.Errorf("unexpected value: %v", v)
	}
}

// TestEvalEnumIndexedVariable verifies eval enum indexed variable.
func TestEvalEnumIndexedVariable(t *testing.T) {
	// This tests enum shorthand variable access via .member
	r := evalText(t, "Color: enum = { red; green; blue; }\nx: Color = .green;")
	if r["x"] != 1 {
		t.Errorf("expected 1, got %v", r["x"])
	}
}

// TestEvalEnumQualifiedAccess verifies eval enum qualified access.
func TestEvalEnumQualifiedAccess(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; blue; }\nx := Color.blue;")
	if r["x"] != 2 {
		t.Errorf("expected 2, got %v", r["x"])
	}
}

// TestEvalEnumValueViaStructField verifies eval enum value via struct field.
func TestEvalEnumValueViaStructField(t *testing.T) {
	// Enum value stored in a struct and accessed via .value
	r := evalText(t, `Status: enum(string) = { ok = "good"; err = "bad"; }
Container: setup = { status: Status; }
c := Container { status = .ok };
v := c.status.value;`)
	if r["v"] != "good" {
		t.Errorf("expected 'good', got %v", r["v"])
	}
}

// TestEvalEnumValueNoValueType verifies eval enum value no value type.
func TestEvalEnumValueNoValueType(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "Color: enum = { red; green; blue; }\nx := Color.red.value;")
	}, "has no tagged value")
}

// TestEvalEnumValueOrdinalInResult verifies eval enum value ordinal in result.
func TestEvalEnumValueOrdinalInResult(t *testing.T) {
	// EnumValue wrapped objects should be stripped to ordinals in the final result
	r := evalText(t, "Color: enum = { red; green; blue; }\ns: scope = { c: Color = .red; }\na := s.c;")
	if r["a"] != 0 {
		t.Errorf("expected 0, got %v", r["a"])
	}
}

// TestEvalOptionalEnumDefaultSupportsTaggedValueAccess verifies eval optional enum default supports tagged value access.
func TestEvalOptionalEnumDefaultSupportsTaggedValueAccess(t *testing.T) {
	r := evalText(t, `
Status: enum(string) = { ok = "good"; bad = "bad"; }
Cfg: setup = { status?: Status = .ok; }
c := Cfg {};
x := c.status.value;
`)
	if r["x"] != "good" {
		t.Fatalf("expected tagged enum default value, got %v", r)
	}
}

// Struct tests

// TestEvalStruct verifies eval struct.
func TestEvalStruct(t *testing.T) {
	r := evalText(t, "Point: setup = { x: integer; y: integer; }\np := Point { x = 10, y = 20 };")
	p, ok := r["p"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", r["p"])
	}
	if p["x"] != 10 || p["y"] != 20 {
		t.Errorf("unexpected struct: %v", p)
	}
}

// TestEvalNestedStruct verifies eval nested struct.
func TestEvalNestedStruct(t *testing.T) {
	r := evalText(t, "Inner: setup = { val: integer; }\nOuter: setup = { inner: Inner; }\no := Outer { inner = Inner { val = 99 } };")
	o := r["o"].(map[string]any)
	inner := o["inner"].(map[string]any)
	if inner["val"] != 99 {
		t.Errorf("expected 99, got %v", inner["val"])
	}
}

// Scope tests

// TestEvalScope verifies eval scope.
func TestEvalScope(t *testing.T) {
	r := evalText(t, "s: scope = { x := 1; y := 2; }\na := s.x;")
	if r["a"] != 1 {
		t.Errorf("expected 1, got %v", r["a"])
	}
}

// TestEvalScopePrivateVar verifies eval scope private var.
func TestEvalScopePrivateVar(t *testing.T) {
	r := evalText(t, "s: scope = { #priv x := 1; y := 2; }\na := s.y;")
	if r["a"] != 2 {
		t.Errorf("expected 2, got %v", r["a"])
	}
	if _, ok := r["x"]; ok {
		t.Error("private var 'x' should not be in result")
	}
}

// TestEvalPrivateVar verifies eval private var.
func TestEvalPrivateVar(t *testing.T) {
	r := evalText(t, "#priv x := \"hidden\"\n;y := \"visible\";")
	if _, ok := r["x"]; ok {
		t.Error("priv var 'x' should not be in result")
	}
	if r["y"] != "visible" {
		t.Errorf("expected 'visible', got %v", r["y"])
	}
}

// TestEvalScopeTwoPass verifies eval scope two pass.
func TestEvalScopeTwoPass(t *testing.T) {
	// Bare scopes use two-pass: first pass for includes, second pass for vars
	r := evalText(t, "s: scope = { x := 1; y := 2; }\na := s.x;")
	if r["a"] != 1 {
		t.Errorf("expected 1, got %v", r["a"])
	}
}

// TestEvalScopeVarDeclNamedType verifies eval scope var decl named type.
func TestEvalScopeVarDeclNamedType(t *testing.T) {
	// Typed variable inside a scope with a named (enum) type
	r := evalText(t, "Color: enum = { red; green; blue; }\ns: scope = { c: Color = .green; }\na := s.c;")
	if r["a"] != 1 {
		t.Errorf("expected 1, got %v", r["a"])
	}
}

// TestEvalScopeBodyNestedScope verifies eval scope body nested scope.
func TestEvalScopeBodyNestedScope(t *testing.T) {
	// A scope with a variable typed as a setup type, then accessed via dot path
	r := evalText(t, "Inner: setup = { a: integer; }\ns: scope = { inner: Inner = Inner { a = 1 }; }\na := s.inner.a;")
	if r["a"] != 1 {
		t.Errorf("expected 1, got %v", r["a"])
	}
}

// Dot path tests

// TestEvalDotPath verifies eval dot path.
func TestEvalDotPath(t *testing.T) {
	r := evalText(t, "s: scope = { x := 1; y := 2; }\na := s.x;")
	if r["a"] != 1 {
		t.Errorf("expected 1, got %v", r["a"])
	}
}

// TestEvalDotAccessEnumField verifies eval dot access enum field.
func TestEvalDotAccessEnumField(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; blue; }\ns: scope = { c: Color = .blue; }\na := s.c;")
	if r["a"] != 2 {
		t.Errorf("expected 2 for .blue, got %v", r["a"])
	}
}

// TestEvalResolveDotPathUndefinedField verifies eval resolve dot path undefined field.
func TestEvalResolveDotPathUndefinedField(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "s: scope = { x := 1; }\na := s.y;")
	}, "Undefined field 'y'")
}

// TestEvalResolveDotPathNonDict verifies eval resolve dot path non dict.
func TestEvalResolveDotPathNonDict(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, "x := 42\n;a := x.y;")
	}, "Cannot access field 'y' on integer")
}

// TestEvalResolveDotPathVariableKey verifies eval resolve dot path variable key.
func TestEvalResolveDotPathVariableKey(t *testing.T) {
	// Access map with variable key
	r := evalText(t, `m := mapping(string, string){["a"] => "val"}; k := "a"; x := m[k];`)
	if r["x"] != "val" {
		t.Errorf("expected 'val', got %v", r["x"])
	}
}

// TestEvalResolveDotPathVariableKeyUndefined verifies eval resolve dot path variable key undefined.
func TestEvalResolveDotPathVariableKeyUndefined(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `m := mapping(string, string){["a"] => "val"}; x := m[undefinedKey];`)
	}, "Undefined variable 'undefinedKey'")
}

// Variable resolution tests

// TestEvalUndefinedVar verifies eval undefined var.
func TestEvalUndefinedVar(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := undefinedVar;`)
	}, "Undefined variable")
}

// TestEvalVariableWithSemicolons verifies eval variable with semicolons.
func TestEvalVariableWithSemicolons(t *testing.T) {
	r := evalText(t, `x := 1; y := 2;`)
	if r["x"] != 1 || r["y"] != 2 {
		t.Errorf("expected x=1, y=2, got %v", r)
	}
}

// Number tests

// TestEvalNegativeInteger verifies eval negative integer.
func TestEvalNegativeInteger(t *testing.T) {
	r := evalText(t, "x := -42;")
	if r["x"] != -42 {
		t.Errorf("expected -42, got %v", r["x"])
	}
}

// TestEvalNegativeFloat verifies eval negative float.
func TestEvalNegativeFloat(t *testing.T) {
	r := evalText(t, "x := -3.14;")
	if r["x"] != -3.14 {
		t.Errorf("expected -3.14, got %v", r["x"])
	}
}

// Coverage gap tests - parser, evaluator, utility edge cases

// TestParseIntLiteral verifies parse int literal.
func TestParseIntLiteral(t *testing.T) {
	if v := ParseIntLiteral("42"); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
	if v := ParseIntLiteral("1_000"); v != 1000 {
		t.Errorf("expected 1000, got %d", v)
	}
}

// TestParseFloatLiteral verifies parse float literal.
func TestParseFloatLiteral(t *testing.T) {
	if v := ParseFloatLiteral("3.14"); v != 3.14 {
		t.Errorf("expected 3.14, got %f", v)
	}
	if v := ParseFloatLiteral("1_0.5"); v != 10.5 {
		t.Errorf("expected 10.5, got %f", v)
	}
}

// TestIsInt verifies is int.
func TestIsInt(t *testing.T) {
	if !IsInt("42") {
		t.Error("expected true for '42'")
	}
	if IsInt("3.14") {
		t.Error("expected false for '3.14'")
	}
	if !IsInt("1_000") {
		t.Error("expected true for '1_000'")
	}
}

// TestIsValidTimestampWithMicros verifies is valid timestamp with micros.
func TestIsValidTimestampWithMicros(t *testing.T) {
	r := evalText(t, `x: timestamp = "2026-01-01 22:10:01.002";`)
	if r["x"] != "2026-01-01 22:10:01.002" {
		t.Errorf("expected timestamp with micros, got %v", r["x"])
	}
}

// TestMapKeysErrorPath verifies map keys error path.
func TestMapKeysErrorPath(t *testing.T) {
	// Trigger mapKeys in resolveDotPath by accessing a missing key with suggestion
	assertPanic(t, func() {
		evalText(t, `m := mapping(string, string){["a"] => "1"}; x := m["b"];`)
	}, "Mapping key not found")
}

// typeNameOf coverage via evalText

// TestTypeNameOfAll verifies type name of all.
func TestTypeNameOfAll(t *testing.T) {
	// Trigger typeNameOf for various types via type check errors
	// bool
	assertPanic(t, func() {
		evalText(t, `x: string = true;`)
	}, "Type mismatch in assignment: expected string, got boolean")
	// []any (array)
	assertPanic(t, func() {
		evalText(t, `x: string = []string{"a"};`)
	}, "Type mismatch in assignment: expected string, got []string")
	// map[string]any
	assertPanic(t, func() {
		evalText(t, `x: string = mapping(string, string){["a"] => "b"};`)
	}, "Type mismatch in assignment: expected string, got mapping(string, string)")
}

// TestTypeNameOfArray verifies type name of array.
func TestTypeNameOfArray(t *testing.T) {
	// Trigger typeNameOf for array type
	assertPanic(t, func() {
		evalText(t, `x: string = []string{"a"};`)
	}, "Type mismatch in assignment: expected string, got []string")
}

// TestTypeNameOfMap verifies type name of map.
func TestTypeNameOfMap(t *testing.T) {
	// Trigger typeNameOf for map type
	assertPanic(t, func() {
		evalText(t, `x: string = mapping(string, string){["a"] => "b"};`)
	}, "Type mismatch in assignment: expected string, got mapping(string, string)")
}

// TestTypeNameOfFloat verifies type name of float.
func TestTypeNameOfFloat(t *testing.T) {
	// Trigger typeNameOf for float type
	assertPanic(t, func() {
		evalText(t, `x: string = 3.14;`)
	}, "Type mismatch in assignment: expected string, got float")
}

// TestEvalTypeNameOf verifies eval type name of.
func TestEvalTypeNameOf(t *testing.T) {
	// Trigger typeNameOf for various types through error messages
	assertPanic(t, func() {
		evalText(t, `x: string = []string{"a"};`)
	}, "Type mismatch in assignment: expected string, got []string")
}

// isValidTimestamp coverage

// TestIsValidTimestampAll verifies is valid timestamp all.
func TestIsValidTimestampAll(t *testing.T) {
	// Test valid timestamp with microseconds
	r := evalText(t, `x: timestamp = "2026-01-01 22:10:01.002";`)
	if r["x"] != "2026-01-01 22:10:01.002" {
		t.Errorf("expected timestamp with micros, got %v", r["x"])
	}
	// Test invalid timestamp format
	assertPanic(t, func() {
		evalText(t, `x: timestamp = "2026/01/01 22:10:01";`)
	}, "Invalid timestamp")
	// Test invalid timestamp (wrong separators)
	assertPanic(t, func() {
		evalText(t, `x: timestamp = "2026-01-01T22:10:01";`)
	}, "Invalid timestamp")
}

// getLocation coverage

// TestGetLocationIterationDecl verifies get location iteration decl.
func TestGetLocationIterationDecl(t *testing.T) {
	// Trigger getLocation for IterationDecl via typeCheckVarDecl
	assertPanic(t, func() {
		evalText(t, `#iteration: integer = "not-int";`)
	}, "Type mismatch in assignment: expected integer, got string")
}

// TestGetLocationAsLvalueDecl verifies get location as lvalue decl.
func TestGetLocationAsLvalueDecl(t *testing.T) {
	// Trigger getLocation for AsLvalueDecl via typeCheckVarDecl
	assertPanic(t, func() {
		evalText(t, `#as_lvalue("x"): integer = "not-int";`)
	}, "Type mismatch in assignment: expected integer, got string")
}

// validExprKindsForType direct tests

// TestValidExprKindsDefault verifies valid expr kinds default.
func TestValidExprKindsDefault(t *testing.T) {
	// Trigger the default case in validExprKindsForType (returns nil)
	// This is for PrimitiveKind values that don't match any known kind
	result := validExprKindsForType(PrimScope)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// TestValidExprKindsForTypeReturnNil verifies valid expr kinds for type return nil.
func TestValidExprKindsForTypeReturnNil(t *testing.T) {
	// Trigger the return nil at the end of validExprKindsForType
	// This is for types that don't match any case - test directly
	result := validExprKindsForType(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// Iteration peek in scope

// TestEvalIterationPeekInScope verifies eval iteration peek in scope.
func TestEvalIterationPeekInScope(t *testing.T) {
	// #iteration peek in a scope body - the peek value is inside the scope's pubs
	r := evalText(t, "s: scope = {\n  #iteration: string = \"val\"\n  ;x := #iteration\n;}")
	s, ok := r["s"].(map[string]any)
	if !ok {
		t.Fatalf("expected 's' to be a map, got %T", r["s"])
	}
	// x should get the next iteration key
	if !strings.HasPrefix(s["x"].(string), "iteration_") {
		t.Errorf("expected iteration key, got %v", s["x"])
	}
}
