// macro_functional_test.go contains macro-specific functional tests that
// exercise macro declaration, call, and type-checking through the full
// SOMETHING pipeline.
//go:build functional

package something

import (
	"testing"
)

// TestEvalMacroNoParams verifies eval macro no params.
func TestEvalMacroNoParams(t *testing.T) {
	// Evaluate a simple macro with no parameters
	r := evalText(t, `#macro greet := () -> string {
    #set "hello";
}
x := greet!();
`)
	if r["x"] != "hello" {
		t.Errorf("expected 'hello', got %v", r["x"])
	}
}

// TestEvalMacroWithParams verifies eval macro with params.
func TestEvalMacroWithParams(t *testing.T) {
	// Evaluate a macro with parameters
	r := evalText(t, `#macro concat := (a: string, b: string) -> string {
    #set "{a}{b}";
}
x := concat!("hello", "world");
`)
	if r["x"] != "helloworld" {
		t.Errorf("expected 'helloworld', got %v", r["x"])
	}
}

// TestEvalMacroWithBodyVars verifies eval macro with body vars.
func TestEvalMacroWithBodyVars(t *testing.T) {
	// Evaluate a macro with local variables in the body
	r := evalText(t, `#macro greeting := () -> string {
    #priv base := "Hello";
    #priv name := "World";
    #set "{base}, {name}!";
}
x := greeting!();
`)
	if r["x"] != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", r["x"])
	}
}

// TestEvalMacroReturningArray verifies eval macro returning array.
func TestEvalMacroReturningArray(t *testing.T) {
	// Evaluate a macro that returns an array
	r := evalText(t, `enrichment_provider_config: setup = {
    name: string;
    base_url: string;
    fields: []string;
}
#macro make_providers := () -> []enrichment_provider_config {
    #set [
        enrichment_provider_config {
            name = "crossref",
            base_url = "https://api.crossref.org/works/",
            fields = []string { "title", "authors" },
        },
        enrichment_provider_config {
            name = "openalex",
            base_url = "https://api.openalex.org/works/",
            fields = []string { "title", "abstract" },
        },
    ];
}
providers := make_providers!();
`)
	providers, ok := r["providers"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", r["providers"])
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	p0, ok := providers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", providers[0])
	}
	if p0["name"] != "crossref" {
		t.Errorf("expected 'crossref', got %v", p0["name"])
	}
	p1, ok := providers[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", providers[1])
	}
	if p1["name"] != "openalex" {
		t.Errorf("expected 'openalex', got %v", p1["name"])
	}
}

// TestEvalMacroReturningInteger verifies eval macro returning integer.
func TestEvalMacroReturningInteger(t *testing.T) {
	// Evaluate a macro returning an integer
	r := evalText(t, `#macro answer := () -> integer {
    #set 42;
}
x := answer!();
`)
	if r["x"] != 42 {
		t.Errorf("expected 42, got %v", r["x"])
	}
}

// TestEvalMacroReturningStruct verifies eval macro returning struct.
func TestEvalMacroReturningStruct(t *testing.T) {
	// Evaluate a macro returning a struct
	r := evalText(t, `Point: setup = { x: integer; y: integer; }
#macro origin := () -> Point {
    #set Point { x = 0, y = 0 };
}
o := origin!();
`)
	o, ok := r["o"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", r["o"])
	}
	if o["x"] != 0 || o["y"] != 0 {
		t.Errorf("expected {x:0, y:0}, got %v", o)
	}
}

// TestEvalMacroTypeMismatch verifies eval macro type mismatch.
func TestEvalMacroTypeMismatch(t *testing.T) {
	// Macro return type mismatch should fail
	assertPanic(t, func() {
		evalText(t, `#macro bad := () -> integer {
    #set "hello";
}
x := bad!();
`)
	}, "expected integer")
}

// TestEvalMacroUndefined verifies eval macro undefined.
func TestEvalMacroUndefined(t *testing.T) {
	// Calling an undefined macro should fail
	assertPanic(t, func() {
		evalText(t, `x := undefined!();
`)
	}, "Undefined macro")
}

// TestEvalMacroArgCountMismatch verifies eval macro arg count mismatch.
func TestEvalMacroArgCountMismatch(t *testing.T) {
	// Wrong number of arguments should fail
	assertPanic(t, func() {
		evalText(t, `#macro greet := (a: string) -> string {
    #set a;
}
x := greet!();
`)
	}, "expects 1 arguments, got 0")
}

// TestEvalMacroParamOrder verifies eval macro param order.
func TestEvalMacroParamOrder(t *testing.T) {
	// Parameters should be bound in order
	r := evalText(t, `#macro pair := (a: string, b: string) -> string {
    #set "{a}{b}";
}
x := pair!("1", "2");
y := pair!("2", "1");
`)
	if r["x"] != "12" {
		t.Errorf("expected '12', got %v", r["x"])
	}
	if r["y"] != "21" {
		t.Errorf("expected '21', got %v", r["y"])
	}
}

// TestEvalMacroBodyVarsNotExposed verifies eval macro body vars not exposed.
func TestEvalMacroBodyVarsNotExposed(t *testing.T) {
	// Variables inside the macro body should not be visible outside
	r := evalText(t, `#macro test := () -> string {
    #priv internal := "secret";
    #set "ok";
}
x := test!();
`)
	if _, ok := r["internal"]; ok {
		t.Error("macro body variable 'internal' should not be in result")
	}
	if r["x"] != "ok" {
		t.Errorf("expected 'ok', got %v", r["x"])
	}
}

// TestEvalMacroMultipleCalls verifies eval macro multiple calls.
func TestEvalMacroMultipleCalls(t *testing.T) {
	// Calling the same macro multiple times should work
	r := evalText(t, `#macro get_val := () -> string {
    #set "value";
}
x := get_val!();
y := get_val!();
`)
	if r["x"] != "value" || r["y"] != "value" {
		t.Errorf("expected both 'value', got x=%v, y=%v", r["x"], r["y"])
	}
}

// TestEvalMacroInStructField verifies eval macro in struct field.
func TestEvalMacroInStructField(t *testing.T) {
	// Macro call as a struct field value
	r := evalText(t, `Cfg: setup = { name: string; }
#macro default_name := () -> string {
    #set "default";
}
c := Cfg { name = default_name!() };
`)
	c, ok := r["c"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", r["c"])
	}
	if c["name"] != "default" {
		t.Errorf("expected 'default', got %v", c["name"])
	}
}

// TestEvalMacroWithForDirective verifies eval macro with for directive.
func TestEvalMacroWithForDirective(t *testing.T) {
	// Macro body with #for directive
	r := evalText(t, `#macro gen := () -> []string {
    #priv items := []string{"a", "b"};
	#priv result := "";
    #for elem: items {
		result = elem;
    }
	result = "c";
    #set []string { "x", "y", "z" };
}
out := gen!();
`)
	out, ok := r["out"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", r["out"])
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(out))
	}
	if out[0] != "x" || out[1] != "y" || out[2] != "z" {
		t.Errorf("expected [x, y, z], got %v", out)
	}
}

// TestEvalMacroParamTypeString verifies eval macro param type string.
func TestEvalMacroParamTypeString(t *testing.T) {
	// Correct string argument should pass
	r := evalText(t, `#macro greet := (name: string) -> string {
    #set "hello {name}";
}
x := greet!("world");
`)
	if r["x"] != "hello world" {
		t.Errorf("expected 'hello world', got %v", r["x"])
	}
}

// TestEvalMacroParamTypeStringWrong verifies eval macro param type string wrong.
func TestEvalMacroParamTypeStringWrong(t *testing.T) {
	// Integer argument where string expected should fail
	assertPanic(t, func() {
		evalText(t, `#macro greet := (name: string) -> string {
    #set "hello";
}
x := greet!(42);
`)
	}, "expected string")
}

// TestEvalMacroParamTypeInteger verifies eval macro param type integer.
func TestEvalMacroParamTypeInteger(t *testing.T) {
	// Correct integer argument should pass
	r := evalText(t, `#macro double := (n: integer) -> integer {
    #set n;
}
x := double!(21);
`)
	if r["x"] != 21 {
		t.Errorf("expected 21, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeIntegerWrong verifies eval macro param type integer wrong.
func TestEvalMacroParamTypeIntegerWrong(t *testing.T) {
	// String argument where integer expected should fail
	assertPanic(t, func() {
		evalText(t, `#macro double := (n: integer) -> integer {
    #set n;
}
x := double!("not-a-number");
`)
	}, "expected integer")
}

// TestEvalMacroParamTypeFloat verifies eval macro param type float.
func TestEvalMacroParamTypeFloat(t *testing.T) {
	// Correct float argument should pass
	r := evalText(t, `#macro half := (n: float) -> float {
    #set n;
}
x := half!(10.0);
`)
	if r["x"] != 10.0 {
		t.Errorf("expected 10.0, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeBoolean verifies eval macro param type boolean.
func TestEvalMacroParamTypeBoolean(t *testing.T) {
	// Correct boolean argument should pass
	r := evalText(t, `#macro negate := (b: boolean) -> boolean {
    #set false;
}
x := negate!(true);
`)
	if r["x"] != false {
		t.Errorf("expected false, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeEnum verifies eval macro param type enum.
func TestEvalMacroParamTypeEnum(t *testing.T) {
	// Correct enum argument should pass
	r := evalText(t, `Color: enum = { red; green; blue; }
#macro color_name := (c: Color) -> string {
    #set "color";
}
x := color_name!(.red);
`)
	if r["x"] != "color" {
		t.Errorf("expected 'color', got %v", r["x"])
	}
}

// TestEvalMacroParamTypeEnumWrong verifies eval macro param type enum wrong.
func TestEvalMacroParamTypeEnumWrong(t *testing.T) {
	// Integer where enum expected should fail
	assertPanic(t, func() {
		evalText(t, `Color: enum = { red; green; blue; }
#macro color_name := (c: Color) -> string {
    #set "color";
}
x := color_name!(42);
`)
	}, "argument 0 (c) expected Color, got integer")
}

// TestEvalMacroParamTypeSetup verifies eval macro param type setup.
func TestEvalMacroParamTypeSetup(t *testing.T) {
	// Correct setup argument should pass
	r := evalText(t, `Point: setup = { x: integer; y: integer; }
#macro get_x := (p: Point) -> integer {
    #set p.x;
}
x := get_x!(Point { x = 10, y = 20 });
`)
	if r["x"] != 10 {
		t.Errorf("expected 10, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeSetupWrong verifies eval macro param type setup wrong.
func TestEvalMacroParamTypeSetupWrong(t *testing.T) {
	// String where setup expected should fail
	assertPanic(t, func() {
		evalText(t, `Point: setup = { x: integer; y: integer; }
#macro get_x := (p: Point) -> integer {
    #set 0;
}
x := get_x!("not-a-point");
`)
	}, "argument 0 (p) expected Point, got string")
}

// TestEvalMacroParamTypeSetupAnonymous verifies eval macro param type setup anonymous.
func TestEvalMacroParamTypeSetupAnonymous(t *testing.T) {
	// Anonymous struct literal as setup argument should work (type inferred)
	r := evalText(t, `Point: setup = { x: integer; y: integer; }
#macro get_x := (p: Point) -> integer {
    #set p.x;
}
x := get_x!({ x = 10, y = 20 });
`)
	if r["x"] != 10 {
		t.Errorf("expected 10, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeArray verifies eval macro param type array.
func TestEvalMacroParamTypeArray(t *testing.T) {
	// Correct array argument should pass
	r := evalText(t, `#macro first := (items: []string) -> string {
    #set items[0];
}
x := first!([]string{"a", "b", "c"});
`)
	if r["x"] != "a" {
		t.Errorf("expected 'a', got %v", r["x"])
	}
}

// TestEvalMacroParamTypeArrayWrong verifies eval macro param type array wrong.
func TestEvalMacroParamTypeArrayWrong(t *testing.T) {
	// String where array expected should fail
	assertPanic(t, func() {
		evalText(t, `#macro first := (items: []string) -> string {
    #set items[0];
}
x := first!("not-an-array");
`)
	}, "argument 0 (items) expected []string, got string")
}

// TestEvalMacroParamTypeMapping verifies eval macro param type mapping.
func TestEvalMacroParamTypeMapping(t *testing.T) {
	// Correct mapping argument should pass
	r := evalText(t, `#macro get_val := (m: mapping(string, string)) -> string {
    #set m["key"];
}
x := get_val!(mapping(string, string){["key"] => "value"});
`)
	if r["x"] != "value" {
		t.Errorf("expected 'value', got %v", r["x"])
	}
}

// TestEvalMacroParamTypeMappingWrong verifies eval macro param type mapping wrong.
func TestEvalMacroParamTypeMappingWrong(t *testing.T) {
	// String where mapping expected should fail
	assertPanic(t, func() {
		evalText(t, `#macro get_val := (m: mapping(string, string)) -> string {
    #set "value";
}
x := get_val!("not-a-mapping");
`)
	}, "expected mapping")
}

// TestEvalMacroParamTypeTimestamp verifies eval macro param type timestamp.
func TestEvalMacroParamTypeTimestamp(t *testing.T) {
	// Correct timestamp argument should pass
	r := evalText(t, `#macro get_year := (ts: timestamp) -> timestamp {
    #set ts;
}
x := get_year!("2026-01-01 22:10:01");
`)
	if r["x"] != "2026-01-01 22:10:01" {
		t.Errorf("expected timestamp, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeTimestampWrong verifies eval macro param type timestamp wrong.
func TestEvalMacroParamTypeTimestampWrong(t *testing.T) {
	// Invalid timestamp should fail
	assertPanic(t, func() {
		evalText(t, `#macro get_year := (ts: timestamp) -> string {
    #set ts;
}
x := get_year!("not-a-timestamp");
`)
	}, "argument 0 (ts) expected timestamp, got string")
}

// TestEvalMacroParamTypeMultipleCorrect verifies eval macro param type multiple correct.
func TestEvalMacroParamTypeMultipleCorrect(t *testing.T) {
	// Multiple typed parameters all correct
	r := evalText(t, `#macro make_point := (x: integer, y: integer, label: string) -> string {
    #set "{label}({x}, {y})";
}
result := make_point!(3, 4, "origin");
`)
	if r["result"] != "origin(3, 4)" {
		t.Errorf("expected 'origin(3, 4)', got %v", r["result"])
	}
}

// TestEvalMacroParamTypeMultipleWrong verifies eval macro param type multiple wrong.
func TestEvalMacroParamTypeMultipleWrong(t *testing.T) {
	// Multiple typed parameters, one wrong
	assertPanic(t, func() {
		evalText(t, `#macro make_point := (x: integer, y: integer, label: string) -> string {
    #set "point";
}
result := make_point!(3, "not-int", "label");
`)
	}, "expected integer")
}

// TestEvalMacroReturnTypeString verifies eval macro return type string.
func TestEvalMacroReturnTypeString(t *testing.T) {
	// Set expression with wrong return type (integer instead of string)
	assertPanic(t, func() {
		evalText(t, `#macro bad := () -> string {
    #set 42;
}
x := bad!();
`)
	}, "expected string")
}

// TestEvalMacroReturnTypeArray verifies eval macro return type array.
func TestEvalMacroReturnTypeArray(t *testing.T) {
	// Set expression with wrong return type (string instead of array)
	assertPanic(t, func() {
		evalText(t, `#macro bad := () -> []string {
    #set "not-an-array";
}
x := bad!();
`)
	}, "Type mismatch in macro 'bad' return: expected []string, got string")
}

// TestEvalMacroReturnTypeSetup verifies eval macro return type setup.
func TestEvalMacroReturnTypeSetup(t *testing.T) {
	// Set expression with wrong return type (string instead of setup)
	assertPanic(t, func() {
		evalText(t, `Point: setup = { x: integer; y: integer; }
#macro bad := () -> Point {
    #set "not-a-point";
}
x := bad!();
`)
	}, "Type mismatch in macro 'bad' return: expected Point, got string")
}

// TestEvalMacroReturnTypeEnum verifies eval macro return type enum.
func TestEvalMacroReturnTypeEnum(t *testing.T) {
	// Set expression with wrong return type (integer out of range)
	assertPanic(t, func() {
		evalText(t, `Color: enum = { red; green; blue; }
#macro bad := () -> Color {
    #set 99;
}
x := bad!();
`)
	}, "Type mismatch in macro 'bad' return: expected Color, got integer")
}

// TestEvalMacroReturnTypeMapping verifies eval macro return type mapping.
func TestEvalMacroReturnTypeMapping(t *testing.T) {
	// Set expression with wrong return type (string instead of mapping)
	assertPanic(t, func() {
		evalText(t, `#macro bad := () -> mapping(string, string) {
    #set "not-a-mapping";
}
x := bad!();
`)
	}, "expected mapping")
}

// TestEvalMacroReturnTypeBoolean verifies eval macro return type boolean.
func TestEvalMacroReturnTypeBoolean(t *testing.T) {
	// Set expression with wrong return type (string instead of boolean)
	assertPanic(t, func() {
		evalText(t, `#macro bad := () -> boolean {
    #set "not-a-boolean";
}
x := bad!();
`)
	}, "expected boolean")
}

// TestEvalMacroReturnTypeFloat verifies eval macro return type float.
func TestEvalMacroReturnTypeFloat(t *testing.T) {
	// Set expression with wrong return type (string instead of float)
	assertPanic(t, func() {
		evalText(t, `#macro bad := () -> float {
    #set "not-a-float";
}
x := bad!();
`)
	}, "expected float")
}

// TestEvalMacroParamTypeScope verifies eval macro param type scope.
func TestEvalMacroParamTypeScope(t *testing.T) {
	// Scope parameter should work
	r := evalText(t, `#macro get_val := (s: scope) -> integer {
    #set s.x;
}
s: scope = { x := 42; }
x := get_val!(s);
`)
	if r["x"] != 42 {
		t.Errorf("expected 42, got %v", r["x"])
	}
}

// TestEvalMacroParamTypeScopeWrong verifies eval macro param type scope wrong.
func TestEvalMacroParamTypeScopeWrong(t *testing.T) {
	// String where scope expected should fail
	assertPanic(t, func() {
		evalText(t, `#macro get_val := (s: scope) -> integer {
    #set 0;
}
x := get_val!("not-a-scope");
`)
	}, "expected scope")
}
