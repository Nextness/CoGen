// ast_functional_test.go contains functional tests for ast.go that exercise
// AST type construction and behavior through the full SOMETHING pipeline
// (parsing, evaluation, and value inspection).
//go:build functional

package something

import (
	"strings"
	"testing"
)

// Basic literal parsing through the pipeline

// TestStringLiteralViaPipeline verifies string literal via pipeline.
func TestStringLiteralViaPipeline(t *testing.T) {
	r := evalText(t, `x := "hello world";`)
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %v", r["x"])
	}
}

// TestIntegerLiteralViaPipeline verifies integer literal via pipeline.
func TestIntegerLiteralViaPipeline(t *testing.T) {
	r := evalText(t, `x := 100;`)
	if r["x"] != 100 {
		t.Errorf("expected 100, got %v", r["x"])
	}
}

// TestFloatLiteralViaPipeline verifies float literal via pipeline.
func TestFloatLiteralViaPipeline(t *testing.T) {
	r := evalText(t, `x := 2.5;`)
	if r["x"] != 2.5 {
		t.Errorf("expected 2.5, got %v", r["x"])
	}
}

// TestBooleanLiteralViaPipeline verifies boolean literal via pipeline.
func TestBooleanLiteralViaPipeline(t *testing.T) {
	r := evalText(t, `x := false;`)
	if r["x"] != false {
		t.Errorf("expected false, got %v", r["x"])
	}
}

// Variable declaration with explicit type (uses DeclaredType)

// TestTypedStringDeclaration verifies typed string declaration.
func TestTypedStringDeclaration(t *testing.T) {
	r := evalText(t, `x: string = "typed";`)
	if r["x"] != "typed" {
		t.Errorf("expected 'typed', got %v", r["x"])
	}
}

// TestTypedIntegerDeclaration verifies typed integer declaration.
func TestTypedIntegerDeclaration(t *testing.T) {
	r := evalText(t, `x: integer = 99;`)
	if r["x"] != 99 {
		t.Errorf("expected 99, got %v", r["x"])
	}
}

// String interpolation (exercises StringExpression -> StringLiteral with InterpolationRef parts)

// TestInterpolationBasic verifies interpolation basic.
func TestInterpolationBasic(t *testing.T) {
	r := evalText(t, `name := "Alice"; greeting := "Hello, {name}!";`)
	if r["greeting"] != "Hello, Alice!" {
		t.Errorf("expected 'Hello, Alice!', got %q", r["greeting"])
	}
}

// TestInterpolationMultiple verifies interpolation multiple.
func TestInterpolationMultiple(t *testing.T) {
	r := evalText(t, `a := "x"; b := "y"; r := "{a} and {b}";`)
	if r["r"] != "x and y" {
		t.Errorf("expected 'x and y', got %q", r["r"])
	}
}

// Array type (exercises ArrayType and ArrayExpression)

// TestArrayTypeParsing verifies array type parsing.
func TestArrayTypeParsing(t *testing.T) {
	r := evalText(t, `x := []string{"a", "b"};`)
	arr, ok := r["x"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", r["x"])
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[0] != "a" {
		t.Errorf("expected 'a', got %v", arr[0])
	}
}

// Mapping type (exercises MappingType and MappingExpression)

// TestMappingTypeParsing verifies mapping type parsing.
func TestMappingTypeParsing(t *testing.T) {
	r := evalText(t, `x := mapping(string, string){["k"] => "v"};`)
	m, ok := r["x"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", r["x"])
	}
	if m["k"] != "v" {
		t.Errorf("expected 'v', got %v", m["k"])
	}
}

// Enum type (exercises EnumDefinition)

// TestEnumParsing verifies enum parsing.
func TestEnumParsing(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; }\nx := Color.green;")
	if r["x"] != 1 {
		t.Errorf("expected 1, got %v", r["x"])
	}
}

// Setup type (exercises SetupDefinition)

// TestSetupParsing verifies setup parsing.
func TestSetupParsing(t *testing.T) {
	r := evalText(t, "Point: setup = { x: integer; y: integer; }\np := Point { x = 1, y = 2 };")
	p, ok := r["p"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", r["p"])
	}
	if p["x"] != 1 || p["y"] != 2 {
		t.Errorf("unexpected struct: %v", p)
	}
}

// Scope expression

// TestScopeParsing verifies scope parsing.
func TestScopeParsing(t *testing.T) {
	r := evalText(t, "s: scope = { x := 1; }\nv := s.x;")
	if r["v"] != 1 {
		t.Errorf("expected 1, got %v", r["v"])
	}
}

// Include expression (tests IncludeExpression)

// TestIncludeExpressionParsing verifies include expression parsing.
func TestIncludeExpressionParsing(t *testing.T) {
	// #include as part of an expression value is stored as a VarDecl with KindInclude
	view := parseText(t, `ns := #include("other.something");`)
	if len(view.TopLevelVars) != 1 {
		t.Fatalf("expected 1 top-level var, got %d", len(view.TopLevelVars))
	}
	if view.TopLevelVars[0].Value == nil || view.TopLevelVars[0].Value.Kind != KindInclude {
		t.Errorf("expected KindInclude in var value, got Kind=%v", view.TopLevelVars[0].Value.Kind)
	}
}

// Multiline string (exercises StringExpression.Multiline)

// TestMultilineParsing verifies multiline parsing.
func TestMultilineParsing(t *testing.T) {
	r := evalText(t, "x := #multiline END\ncontent\nEND\n;")
	if r["x"] != "content" {
		t.Errorf("expected 'content', got %q", r["x"])
	}
}

// Empty result (no declarations)

// TestEmptyProgram verifies empty program.
func TestEmptyProgram(t *testing.T) {
	r := evalText(t, "")
	if len(r) != 0 {
		t.Errorf("expected empty result, got %v", r)
	}
}

// Multiple declarations in one program

// TestMultipleDeclarations verifies multiple declarations.
func TestMultipleDeclarations(t *testing.T) {
	r := evalText(t, `a := 1; b := "two"; c := true;`)
	if r["a"] != 1 {
		t.Errorf("expected a=1, got %v", r["a"])
	}
	if r["b"] != "two" {
		t.Errorf("expected b='two', got %v", r["b"])
	}
	if r["c"] != true {
		t.Errorf("expected c=true, got %v", r["c"])
	}
}

// TypeName with dot access on a declaration reference

// TestReferenceRootIdentity verifies reference root identity.
func TestReferenceRootIdentity(t *testing.T) {
	r := evalText(t, "s: scope = { v := 1; }\nx := s.v;")
	if r["x"] != 1 {
		t.Errorf("expected 1, got %v", r["x"])
	}
}

// For directive (exercises ForDirective)

// TestForDirectiveParsing verifies for directive parsing.
func TestForDirectiveParsing(t *testing.T) {
	view := parseText(t, `x := ""; #for e: ["a"] { x = e; }`)
	if len(view.TopLevelFors) != 1 {
		t.Errorf("expected 1 for, got %d", len(view.TopLevelFors))
	}
}

// Insert directive (exercises InsertDirective)

// TestInsertDirectiveParsing verifies insert directive parsing.
func TestInsertDirectiveParsing(t *testing.T) {
	view := parseText(t, `#insert { "x := 1;" };`)
	if len(view.TopLevelInserts) != 1 {
		t.Errorf("expected 1 insert, got %d", len(view.TopLevelInserts))
	}
}

// Iteration directive (exercises IterationLValue)

// TestIterationDirectiveParsing verifies iteration directive parsing.
func TestIterationDirectiveParsing(t *testing.T) {
	r := evalText(t, `#iteration: string = "val";`)
	found := false
	for k, v := range r {
		if strings.HasPrefix(k, "iteration_") && v == "val" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected iteration_* key with 'val', got %v", r)
	}
}

// AsLvalue directive

// TestAsLvalueDirectiveParsing verifies as lvalue directive parsing.
func TestAsLvalueDirectiveParsing(t *testing.T) {
	r := evalText(t, `#as_lvalue("dyn"): string = "resolved";`)
	if r["dyn"] != "resolved" {
		t.Errorf("expected 'resolved', got %v", r["dyn"])
	}
}

// Struct with optional field (exercises FieldDefinition Optional)

// TestOptionalFieldParsing verifies optional field parsing.
func TestOptionalFieldParsing(t *testing.T) {
	r := evalText(t, "Cfg: setup = { name: string; label?: string = \"default\"; }\nc := Cfg { name = \"test\" };")
	c := r["c"].(map[string]any)
	if c["label"] != "default" {
		t.Errorf("expected 'default', got %v", c["label"])
	}
}

// Enum with tagged values (exercises EnumMember.Value)

// TestEnumTaggedValueParsing verifies enum tagged value parsing.
func TestEnumTaggedValueParsing(t *testing.T) {
	r := evalText(t, `Status: enum(string) = { ok = "good"; } x := Status.ok.value;`)
	if r["x"] != "good" {
		t.Errorf("expected 'good', got %v", r["x"])
	}
}

// Negative numbers (exercises unary negation parsing)

// TestNegativeNumberParsing verifies negative number parsing.
func TestNegativeNumberParsing(t *testing.T) {
	r := evalText(t, "x := -5;")
	if r["x"] != -5 {
		t.Errorf("expected -5, got %v", r["x"])
	}
}

// Mapping composite keys

// TestCompositeKeyParsing verifies composite key parsing.
func TestCompositeKeyParsing(t *testing.T) {
	r := evalText(t, `x := mapping(string, integer){[["a", "b"]] => 1};`)
	m := r["x"].(map[string]any)
	if m["a,b"] != 1 {
		t.Errorf("expected 1 at key 'a,b', got %v", m["a,b"])
	}
}

// Array index access (exercises IndexAccess)

// TestIndexAccessParsing verifies index access parsing.
func TestIndexAccessParsing(t *testing.T) {
	r := evalText(t, `x := []string{"a", "b", "c"}; y := x[1];`)
	if r["y"] != "b" {
		t.Errorf("expected 'b', got %v", r["y"])
	}
}

// Field access (exercises FieldAccess)

// TestFieldAccessParsing verifies field access parsing.
func TestFieldAccessParsing(t *testing.T) {
	r := evalText(t, "s: scope = { x := 42; }\nv := s.x;")
	if r["v"] != 42 {
		t.Errorf("expected 42, got %v", r["v"])
	}
}

// Typed expression (exercises TypedExpression)

// TestTypedExpressionViaStruct verifies typed expression via struct.
func TestTypedExpressionViaStruct(t *testing.T) {
	r := evalText(t, "S: setup = { x: integer; }\no := S { x = 7 };")
	o := r["o"].(map[string]any)
	if o["x"] != 7 {
		t.Errorf("expected 7, got %v", o["x"])
	}
}

// Namespace expression (exercises NamespaceExpression via #include)

// TestNamespaceExpressionParsing verifies namespace expression parsing.
func TestNamespaceExpressionParsing(t *testing.T) {
	view := parseText(t, `ns := #include("other.something");`)
	if view.TopLevelVars[0].ExplicitType != "namespace" {
		t.Errorf("expected type 'namespace', got %q", view.TopLevelVars[0].ExplicitType)
	}
}

// Enum key type mapping access

// TestEnumKeyTypeParsing verifies enum key type parsing.
func TestEnumKeyTypeParsing(t *testing.T) {
	r := evalText(t, "Color: enum = { red; green; }\nm := mapping(Color, string){[.red] => \"r\"}\n;x := m[.red];")
	if r["x"] != "r" {
		t.Errorf("expected 'r', got %v", r["x"])
	}
}
