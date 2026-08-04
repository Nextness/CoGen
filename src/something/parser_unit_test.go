// parser_unit_test.go contains parser tests for the SOMETHING config language.
//go:build unit

package something

import (
	"testing"
)

// TestParseEmpty verifies parse empty.
func TestParseEmpty(t *testing.T) {
	prog := parseText(t, "")
	if len(prog.TopLevelVars) != 0 {
		t.Errorf("expected 0 vars, got %d", len(prog.TopLevelVars))
	}
}

// TestParseVarInfer verifies parse var infer.
func TestParseVarInfer(t *testing.T) {
	prog := parseText(t, `x := "hello";`)
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	v := prog.TopLevelVars[0]
	if v.Name != "x" {
		t.Errorf("expected name 'x', got %q", v.Name)
	}
	if !v.InferType {
		t.Error("expected InferType = true")
	}
}

// TestParseVarTyped verifies parse var typed.
func TestParseVarTyped(t *testing.T) {
	prog := parseText(t, `x: string = "hello";`)
	v := prog.TopLevelVars[0]
	if v.Name != "x" {
		t.Errorf("expected name 'x', got %q", v.Name)
	}
	if v.InferType {
		t.Error("expected InferType = false")
	}
	if v.ExplicitType != "string" {
		t.Errorf("expected type 'string', got %q", v.ExplicitType)
	}
}

// TestParseVarInteger verifies parse var integer.
func TestParseVarInteger(t *testing.T) {
	prog := parseText(t, "x: integer = 42;")
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "integer" {
		t.Errorf("expected type 'integer', got %q", v.ExplicitType)
	}
}

// TestParseVarBoolean verifies parse var boolean.
func TestParseVarBoolean(t *testing.T) {
	prog := parseText(t, "x: boolean = true;")
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "boolean" {
		t.Errorf("expected type 'boolean', got %q", v.ExplicitType)
	}
}

// TestParseVarFloat verifies parse var float.
func TestParseVarFloat(t *testing.T) {
	prog := parseText(t, "x: float = 3.14;")
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "float" {
		t.Errorf("expected type 'float', got %q", v.ExplicitType)
	}
}

// TestParseVarTimestamp verifies parse var timestamp.
func TestParseVarTimestamp(t *testing.T) {
	prog := parseText(t, `x: timestamp = "2026-01-01 22:10:01";`)
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "timestamp" {
		t.Errorf("expected type 'timestamp', got %q", v.ExplicitType)
	}
}

// TestParseEnumPlain verifies parse enum plain.
func TestParseEnumPlain(t *testing.T) {
	prog := parseText(t, "Color: enum = { red; green; blue; }")
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}
	ed := prog.Enums[0]
	if ed.Name != "Color" {
		t.Errorf("expected 'Color', got %q", ed.Name)
	}
	if ed.ValueType != nil {
		t.Error("expected nil ValueType")
	}
	if len(ed.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(ed.Members))
	}
	if ed.Members[0].Name != "red" || ed.Members[2].Name != "blue" {
		t.Errorf("members: %v", ed.Members)
	}
}

// TestParseEnumTagged verifies parse enum tagged.
func TestParseEnumTagged(t *testing.T) {
	prog := parseText(t, `Status: enum(string) = { ok = "good"; err = "bad"; }`)
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}
	ed := prog.Enums[0]
	if ed.ValueType == nil {
		t.Fatal("expected non-nil ValueType")
	}
	if len(ed.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(ed.Members))
	}
}

// TestParseSetup verifies parse setup.
func TestParseSetup(t *testing.T) {
	prog := parseText(t, "Point: setup = { x: integer; y: integer; }")
	if len(prog.Setups) != 1 {
		t.Fatalf("expected 1 setup, got %d", len(prog.Setups))
	}
	sd := prog.Setups[0]
	if sd.Name != "Point" {
		t.Errorf("expected 'Point', got %q", sd.Name)
	}
	if len(sd.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(sd.Fields))
	}
	if sd.Fields[0].Name != "x" || sd.Fields[1].Name != "y" {
		t.Errorf("fields: %v", sd.Fields)
	}
}

// TestParseSetupOptional verifies parse setup optional.
func TestParseSetupOptional(t *testing.T) {
	prog := parseText(t, `Cfg: setup = { name: string; label?: string = "default"; }`)
	sd := prog.Setups[0]
	if len(sd.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(sd.Fields))
	}
	if !sd.Fields[1].Optional {
		t.Error("expected field 1 to be optional")
	}
	if sd.Fields[1].DefaultValue == nil {
		t.Error("expected default value")
	}
}

// TestParseBareScope verifies parse bare scope.
func TestParseBareScope(t *testing.T) {
	prog := parseText(t, "myscope: scope = { x := 1; }")
	if len(prog.TopLevelBareScopes) != 1 {
		t.Fatalf("expected 1 bare scope, got %d", len(prog.TopLevelBareScopes))
	}
	scope := prog.TopLevelBareScopes[0]
	if len(scope.Body) != 1 {
		t.Fatalf("expected 1 body item, got %d", len(scope.Body))
	}
}

// TestParseArrayType verifies parse array type.
func TestParseArrayType(t *testing.T) {
	prog := parseText(t, `x: []string = []string{"a", "b"};`)
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "array" {
		t.Errorf("expected type 'array', got %q", v.ExplicitType)
	}
}

// TestParseMappingType verifies parse mapping type.
func TestParseMappingType(t *testing.T) {
	prog := parseText(t, `x: mapping(string, integer) = mapping(string, integer){["a"] => 1};`)
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "mapping" {
		t.Errorf("expected type 'mapping', got %q", v.ExplicitType)
	}
}

// TestParseEnumKeyType verifies parse enum key type.
func TestParseEnumKeyType(t *testing.T) {
	prog := parseText(t, "Color: enum = { red; green; blue; }\nx: [Color]string = []string{\"a\", \"b\", \"c\"};")
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	v := prog.TopLevelVars[0]
	// EnumKey types print as "[Color]" via typeRefToString
	if v.ExplicitType == "" {
		t.Error("expected non-empty ExplicitType")
	}
}

// TestParseIterationDirective verifies parse iteration directive.
func TestParseIterationDirective(t *testing.T) {
	prog := parseText(t, `#iteration: string = "a";`)
	if len(prog.TopLevelIterations) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(prog.TopLevelIterations))
	}
	it := prog.TopLevelIterations[0]
	if it.InferType {
		t.Error("iteration with explicit type 'string' should have InferType=false")
	}
	if it.ExplicitType != "string" {
		t.Errorf("expected ExplicitType 'string', got %q", it.ExplicitType)
	}
}

// TestParseIterationInfer verifies parse iteration infer.
func TestParseIterationInfer(t *testing.T) {
	prog := parseText(t, `#iteration := "a";`)
	if len(prog.TopLevelIterations) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(prog.TopLevelIterations))
	}
	it := prog.TopLevelIterations[0]
	if !it.InferType {
		t.Error("expected InferType = true")
	}
}

// TestParseIterationWithLabel verifies parse iteration with label.
func TestParseIterationWithLabel(t *testing.T) {
	prog := parseText(t, `#iteration("_label"): string = "a";`)
	if len(prog.TopLevelIterations) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(prog.TopLevelIterations))
	}
	it := prog.TopLevelIterations[0]
	if _, ok := it.IterationLabel.(*StringExpression); !ok {
		t.Errorf("expected string iteration label, got %T", it.IterationLabel)
	}
}

// TestParseIterationScope verifies parse iteration scope.
func TestParseIterationScope(t *testing.T) {
	prog := parseText(t, "#iteration: scope = {\n  x := 1\n;}")
	if len(prog.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(prog.Scopes))
	}
	asgn := prog.Scopes[0]
	iv, ok := asgn.LValue.(*IterationLValue)
	if !ok {
		t.Fatalf("expected *IterationLValue lvalue, got %T", asgn.LValue)
	}
	if iv.Label != nil {
		t.Errorf("expected no label, got %T", iv.Label)
	}
	if _, ok := asgn.RValue.(*ScopeDecl); !ok {
		t.Fatalf("expected *ScopeDecl rvalue, got %T", asgn.RValue)
	}
}

// TestParseForDirective verifies parse for directive.
func TestParseForDirective(t *testing.T) {
	prog := parseText(t, `#for elem: ["a","b"] { x := elem; }`)
	if len(prog.TopLevelFors) != 1 {
		t.Fatalf("expected 1 for, got %d", len(prog.TopLevelFors))
	}
	fd := prog.TopLevelFors[0]
	if fd.ElementName != "elem" {
		t.Errorf("expected 'elem', got %q", fd.ElementName)
	}
	if fd.KeyName != "" {
		t.Errorf("expected empty key, got %q", fd.KeyName)
	}
}

// TestParseForMapping verifies parse for mapping.
func TestParseForMapping(t *testing.T) {
	prog := parseText(t, `m := mapping(string, string){["a"]=>"b"};
#for key, val: m { x := val; }`)
	if len(prog.TopLevelFors) != 1 {
		t.Fatalf("expected 1 for, got %d", len(prog.TopLevelFors))
	}
	fd := prog.TopLevelFors[0]
	if fd.KeyName != "key" {
		t.Errorf("expected key 'key', got %q", fd.KeyName)
	}
	if fd.ElementName != "val" {
		t.Errorf("expected val 'val', got %q", fd.ElementName)
	}
}

// TestParseInsertDirective verifies parse insert directive.
func TestParseInsertDirective(t *testing.T) {
	prog := parseText(t, `#insert { "a := 1;" };`)
	if len(prog.TopLevelInserts) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(prog.TopLevelInserts))
	}
}

// TestParseInsertDirectiveMultipleValues verifies parse insert directive multiple values.
func TestParseInsertDirectiveMultipleValues(t *testing.T) {
	prog := parseText(t, `#insert { "a := 1;", "b := 2;", };`)
	if got := len(prog.TopLevelInserts[0].Contents); got != 2 {
		t.Fatalf("expected 2 insert values, got %d", got)
	}
}

// TestParseIncludeDirective verifies parse include directive.
func TestParseIncludeDirective(t *testing.T) {
	prog := parseText(t, `#include("somefile.something");`)
	if len(prog.TopLevelIncludes) != 1 {
		t.Fatalf("expected 1 include, got %d", len(prog.TopLevelIncludes))
	}
	inc := prog.TopLevelIncludes[0]
	if inc.Filepath != "somefile.something" {
		t.Errorf("expected 'somefile.something', got %q", inc.Filepath)
	}
}

// TestParseAsLvalueDirective verifies parse as lvalue directive.
func TestParseAsLvalueDirective(t *testing.T) {
	prog := parseText(t, `#as_lvalue("target"): string = "value";`)
	if len(prog.TopLevelAsLvalues) != 1 {
		t.Fatalf("expected 1 as_lvalue, got %d", len(prog.TopLevelAsLvalues))
	}
}

// TestParsePrivModifier verifies parse priv modifier.
func TestParsePrivModifier(t *testing.T) {
	prog := parseText(t, `#priv x := "hidden";`)
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	if !prog.TopLevelVars[0].Priv {
		t.Error("expected Priv = true")
	}
}

// TestParseStructValue verifies parse struct value.
func TestParseStructValue(t *testing.T) {
	prog := parseText(t, "Point: setup = { x: integer; y: integer; }\np := Point { x = 10, y = 20 };")
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindStruct {
		t.Errorf("expected KindStruct, got %v", vn.Kind)
	}
	if vn.Raw.(string) != "Point" {
		t.Errorf("expected 'Point', got %q", vn.Raw.(string))
	}
}

// TestParseStructAnonymous verifies parse struct anonymous.
func TestParseStructAnonymous(t *testing.T) {
	prog := parseText(t, `x := { a = 1, b = 2 };`)
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindStruct {
		t.Errorf("expected KindStruct, got %v", vn.Kind)
	}
	// Anonymous struct - Raw is nil
	if vn.Raw != nil {
		t.Errorf("expected nil Raw for anonymous struct, got %v", vn.Raw)
	}
}

// TestParseMappingLiteral verifies parse mapping literal.
func TestParseMappingLiteral(t *testing.T) {
	prog := parseText(t, `x := mapping(string, integer){["a"] => 1, ["b"] => 2};`)
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindMapping {
		t.Errorf("expected KindMapping, got %v", vn.Kind)
	}
}

// TestParseArrayLiteral verifies parse array literal.
func TestParseArrayLiteral(t *testing.T) {
	prog := parseText(t, `x := []string{"a", "b"};`)
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindArray {
		t.Errorf("expected KindArray, got %v", vn.Kind)
	}
}

// TestParseEnumMemberShorthand verifies parse enum member shorthand.
func TestParseEnumMemberShorthand(t *testing.T) {
	prog := parseText(t, `x := .red;`)
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindReference {
		t.Errorf("expected KindReference, got %v", vn.Kind)
	}
	if vn.Raw.(string) != ".red" {
		t.Errorf("expected '.red', got %q", vn.Raw.(string))
	}
}

// TestParseReferenceWithDots verifies parse reference with dots.
func TestParseReferenceWithDots(t *testing.T) {
	prog := parseText(t, `x := a.b.c;`)
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindReference {
		t.Errorf("expected KindReference, got %v", vn.Kind)
	}
	if vn.Raw.(string) != "a.b.c" {
		t.Errorf("expected 'a.b.c', got %q", vn.Raw.(string))
	}
}

// TestParseReferenceWithIndex verifies parse reference with index.
func TestParseReferenceWithIndex(t *testing.T) {
	prog := parseText(t, `x := arr[0];`)
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindReference {
		t.Errorf("expected KindReference, got %v", vn.Kind)
	}
	if vn.Raw.(string) != "arr[0]" {
		t.Errorf("expected 'arr[0]', got %q", vn.Raw.(string))
	}
}

// TestParseReferenceWithCombinedAccess verifies parse reference with combined access.
func TestParseReferenceWithCombinedAccess(t *testing.T) {
	prog := parseText(t, `x := matrix[0][1].value["key"];`)
	if got := prog.TopLevelVars[0].Value.Raw.(string); got != `matrix[0][1].value["key"]` {
		t.Fatalf("unexpected reference path: %q", got)
	}
}

// TestParseMultilineValue verifies parse multiline value.
func TestParseMultilineValue(t *testing.T) {
	prog := parseText(t, "x := #multiline EOF\nhello world\nEOF\n;")
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != KindMultiline {
		t.Errorf("expected KindMultiline, got %v", vn.Kind)
	}
}

// TestParseErrorMissingBrace verifies parse error missing brace.
func TestParseErrorMissingBrace(t *testing.T) {
	assertPanic(t, func() {
		parseText(t, "x: string = \"hello\"\n;{")
	}, "Expected an assignment or directive")
}

// TestParseErrorUnknownDirective verifies parse error unknown directive.
func TestParseErrorUnknownDirective(t *testing.T) {
	assertPanic(t, func() {
		parseText(t, "#unknown_directive")
	}, "Unknown directive")
}

// TestParseArrayTypeWithExplicitIndex verifies parse array type with explicit index.
func TestParseArrayTypeWithExplicitIndex(t *testing.T) {
	prog := parseText(t, `x: [integer]string = []string{"a"};`)
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	v := prog.TopLevelVars[0]
	if v.ExplicitType != "array" {
		t.Errorf("expected type 'array', got %q", v.ExplicitType)
	}
}

// TestParseScopeBodyItemPrivIteration verifies parse scope body item priv iteration.
func TestParseScopeBodyItemPrivIteration(t *testing.T) {
	// #priv #iteration: string = "a" inside a scope
	prog := parseText(t, "s: scope = {\n  #priv #iteration: string = \"a\"\n;}")
	if len(prog.TopLevelBareScopes) != 1 {
		t.Fatalf("expected 1 bare scope, got %d", len(prog.TopLevelBareScopes))
	}
	scope := prog.TopLevelBareScopes[0]
	if len(scope.Body) != 1 {
		t.Fatalf("expected 1 body item, got %d", len(scope.Body))
	}
	it, ok := scope.Body[0].(*IterationDecl)
	if !ok {
		t.Fatalf("expected *IterationDecl, got %T", scope.Body[0])
	}
	if !it.Priv {
		t.Error("expected Priv=true")
	}
}

// TestParseScopeBodyUnknownDirective verifies parse scope body unknown directive.
func TestParseScopeBodyUnknownDirective(t *testing.T) {
	assertPanic(t, func() {
		parseText(t, "s: scope = { #unknown_directive }")
	}, "Unknown directive")
}

// TestParseScopeBodyNonIdentifier verifies parse scope body non identifier.
func TestParseScopeBodyNonIdentifier(t *testing.T) {
	assertPanic(t, func() {
		parseText(t, "s: scope = { 123 }")
	}, "Expected an assignment or directive")
}

// TestParseForSourceDottedPath verifies parse for source dotted path.
func TestParseForSourceDottedPath(t *testing.T) {
	prog := parseText(t, "s: scope = { x := 1; }\n#for elem: s.x { y := elem; }")
	if len(prog.TopLevelFors) != 1 {
		t.Fatalf("expected 1 for, got %d", len(prog.TopLevelFors))
	}
}

// TestParseIndexAccessAllBranches verifies parse index access all branches.
func TestParseIndexAccessAllBranches(t *testing.T) {
	// String literal index: arr["key"]
	prog := parseText(t, `x := arr["key"];`)
	vn := prog.TopLevelVars[0].Value
	if vn.Raw.(string) != `arr["key"]` {
		t.Errorf("expected 'arr[\"key\"]', got %q", vn.Raw.(string))
	}

	// Identifier index: arr[idx]
	prog2 := parseText(t, `x := arr[idx];`)
	if prog2.TopLevelVars[0].Value.Raw.(string) != "arr[idx]" {
		t.Errorf("expected 'arr[idx]', got %q", prog2.TopLevelVars[0].Value.Raw.(string))
	}
}

// TestParseIndexAccessError verifies parse index access error.
func TestParseIndexAccessError(t *testing.T) {
	assertPanic(t, func() {
		parseText(t, `x := arr[?]`)
	}, "Unexpected value token: OPTIONAL")
}

// TestParseScopeBodyItemTypedVar verifies parse scope body item typed var.
func TestParseScopeBodyItemTypedVar(t *testing.T) {
	prog := parseText(t, "s: scope = { x: string = \"hello\"; }")
	scope := prog.TopLevelBareScopes[0]
	if len(scope.Body) != 1 {
		t.Fatalf("expected 1 body item, got %d", len(scope.Body))
	}
	vd, ok := scope.Body[0].(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", scope.Body[0])
	}
	if vd.ExplicitType != "string" {
		t.Errorf("expected 'string', got %q", vd.ExplicitType)
	}
}

// TestParseScopeBodyItemIncludeVar verifies parse scope body item include var.
func TestParseScopeBodyItemIncludeVar(t *testing.T) {
	// When a scope body var uses #include as value, it should be namespace typed
	// This is a parse-only test since include needs file I/O
	prog := parseText(t, `s: scope = { x := #include("test.something"); }`)
	scope := prog.TopLevelBareScopes[0]
	if len(scope.Body) != 1 {
		t.Fatalf("expected 1 body item, got %d", len(scope.Body))
	}
	vd, ok := scope.Body[0].(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", scope.Body[0])
	}
	if vd.ExplicitType != "namespace" {
		t.Errorf("expected 'namespace', got %q", vd.ExplicitType)
	}
}

// TestParserExpectError verifies parser expect error.
func TestParserExpectError(t *testing.T) {
	// Trigger unexpected token at top level
	assertPanic(t, func() {
		parseText(t, `x: string = "hello"; }`)
	}, "Expected an assignment or directive")
}

// TestParserExpectEmptyMsg verifies parser expect empty msg.
func TestParserExpectEmptyMsg(t *testing.T) {
	// #insert values require commas between them.
	assertPanic(t, func() {
		parseText(t, `#insert { "hello" "world" };`)
	}, "Expected ',' between #insert values")
}

// TestExpectErrorPath verifies expect error path.
func TestExpectErrorPath(t *testing.T) {
	// Trigger error in parseValue with unexpected EOF
	assertPanic(t, func() {
		parseText(t, `x: string = `)
	}, "Unexpected value token")
}
