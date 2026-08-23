// pipeline_functional_test.go tests the SOMETHING three-phase pipeline (parse,
//go:build functional

package something

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compileText supports the package test suite's compile text setup or assertions.
func compileText(t *testing.T, text string) (*Program, *Program, *CheckedProgram) {
	t.Helper()
	syntax := NewParser(NewLexer(text, "").Tokenize(), "").ParseProgram()
	expanded := NewDirectiveGenerator("").Expand(syntax)
	checked := NewTypeChecker(expanded, "").Check()
	return syntax, expanded, checked
}

// assertExpandedAssignments supports the package test suite's assert expanded assignments setup or assertions.
func assertExpandedAssignments(t *testing.T, statements []Statement) {
	t.Helper()
	for _, statement := range statements {
		assignment, ok := statement.(*Assignment)
		if !ok {
			t.Fatalf("expanded AST contains directive %T", statement)
		}
		switch value := assignment.Value.(type) {
		case *ScopeExpression:
			assertExpandedAssignments(t, value.Statements)
		case *NamespaceExpression:
			assertExpandedAssignments(t, value.Statements)
		}
	}
}

// TestOrderedASTInterfaceMarkers verifies ordered ast interface markers.
func TestOrderedASTInterfaceMarkers(t *testing.T) {
	PrimScope.typeRefMarker()
	PrimNamespace.typeRefMarker()
	(&ScopeType{}).typeRefMarker()
	(&NamespaceType{}).typeRefMarker()
	StringText("").stringPartMarker()
	(&InterpolationRef{}).stringPartMarker()

	(&Assignment{}).statementMarker()
	(&IncludeDirective{}).statementMarker()
	(&ForDirective{}).statementMarker()
	(&InsertDirective{}).statementMarker()
	(&MacroDirective{}).statementMarker()
	(&AssertDirective{}).statementMarker()
	(&IfDirective{}).statementMarker()
	(&ErrorDirective{}).statementMarker()
	(&IdentifierLValue{}).lvalueMarker()
	(&MemberLValue{}).lvalueMarker()
	(&IterationLValue{}).lvalueMarker()
	(&AsLValue{}).lvalueMarker()
	(&FieldAccess{}).accessMarker()
	(&IndexAccess{}).accessMarker()

	expressions := []Expression{
		&StringExpression{},
		&IntegerExpression{},
		&FloatExpression{},
		&BooleanExpression{},
		&ReferenceExpression{},
		&ArrayExpression{},
		&MappingExpression{},
		&StructExpression{},
		&IncludeExpression{},
		&IterationExpression{},
		&MacroCallExpression{},
		&NamespaceExpression{},
		&TypedExpression{},
		&BinaryOpExpression{},
		&UnaryOpExpression{},
		&MatchExpression{},
		&LenExpression{},
		&IntrinsicExpression{},
	}
	for _, expression := range expressions {
		expression.assignmentValueMarker()
		expression.expressionMarker()
		expression.expressionLocation()
	}
	(&ScopeExpression{}).assignmentValueMarker()
	(&SetupDefinition{}).assignmentValueMarker()
	(&EnumDefinition{}).assignmentValueMarker()
}

// TestPipelinePreservesSyntaxOrderThenRemovesDirectives verifies pipeline preserves syntax order then removes directives.
func TestPipelinePreservesSyntaxOrderThenRemovesDirectives(t *testing.T) {
	syntax, expanded, _ := compileText(t, `
value := 1;
#for item: [2, 3] {
    #iteration("_generated") := item;
}
value = 4;
`)
	if len(syntax.Statements) != 3 {
		t.Fatalf("expected three ordered syntax statements, got %d", len(syntax.Statements))
	}
	if _, ok := syntax.Statements[0].(*Assignment); !ok {
		t.Fatalf("expected first syntax statement to be an assignment, got %T", syntax.Statements[0])
	}
	if _, ok := syntax.Statements[1].(*ForDirective); !ok {
		t.Fatalf("expected second syntax statement to remain a #for directive, got %T", syntax.Statements[1])
	}
	last, ok := syntax.Statements[2].(*Assignment)
	if !ok || last.Mode != AssignExisting {
		t.Fatalf("expected final syntax statement to be a reassignment, got %T", syntax.Statements[2])
	}

	assertExpandedAssignments(t, expanded.Statements)
	if len(expanded.Statements) != 4 {
		t.Fatalf("expected declaration, two generated assignments, and reassignment, got %d statements", len(expanded.Statements))
	}
	for index, expected := range []string{"value", "iteration_0000000000_generated", "iteration_0000000001_generated", "value"} {
		assignment := expanded.Statements[index].(*Assignment)
		target := assignment.Target.(*IdentifierLValue)
		if target.Name != expected {
			t.Fatalf("expanded statement %d has target %q, expected %q", index, target.Name, expected)
		}
	}
}

// TestDirectiveExpansionResolvesLoopElementMembers verifies directive expansion resolves loop element members.
func TestDirectiveExpansionResolvesLoopElementMembers(t *testing.T) {
	result := evalText(t, `
Item: setup = { value: string; }
items := []Item{Item { value = "first" }, Item { value = "second" }};
#for item: items {
    #iteration("_value") := item.value;
    #iteration("_multiline") := #multiline (no_newline) END
    {item.value}
    END;
}
`)
	if result["iteration_0000000000_value"] != "first" || result["iteration_0000000001_value"] != "second" {
		t.Fatalf("loop element members were not replaced during expansion: %v", result)
	}
	if result["iteration_0000000000_multiline"] != "first" || result["iteration_0000000001_multiline"] != "second" {
		t.Fatalf("multiline loop interpolation was not replaced during expansion: %v", result)
	}
}

// TestSourceOrderRejectsReferencesBeforeDeclaration verifies source order rejects references before declaration.
func TestSourceOrderRejectsReferencesBeforeDeclaration(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `copy := later; later := "declared";`)
	}, "Undefined variable 'later'")
	assertPanic(t, func() {
		evalText(t, `result := later!(); #macro later := () -> string { #set "declared"; }`)
	}, "Undefined macro 'later'")
}

// TestScopesAllowIndependentNamesAndQualifiedOuterAccess verifies scopes allow independent names and qualified outer access.
func TestScopesAllowIndependentNamesAndQualifiedOuterAccess(t *testing.T) {
	result := evalText(t, `
a: scope = {
    b := "something";
    c: scope = {
        b := "another";
        outer_value := a.b;
    }
}
d := a.b;
e := a.c.b;
f := a.c.outer_value;
`)
	if result["d"] != "something" || result["e"] != "another" || result["f"] != "something" {
		t.Fatalf("unexpected qualified scope values: %v", result)
	}
}

// TestPrivateSourcesRemainAccessibleAndDestinationControlsPublication verifies private sources remain accessible and destination controls publication.
func TestPrivateSourcesRemainAccessibleAndDestinationControlsPublication(t *testing.T) {
	result := evalText(t, `
a: scope = {
    #priv hidden := "secret";
    public := hidden;
}
published := a.hidden;
#priv retained_private := "initial";
retained_private = a.hidden;
`)
	a := result["a"].(map[string]any)
	if _, exists := a["hidden"]; exists {
		t.Fatalf("private scope field was published: %v", a)
	}
	if a["public"] != "secret" || result["published"] != "secret" {
		t.Fatalf("private source did not flow into public destinations: %v", result)
	}
	if _, exists := result["retained_private"]; exists {
		t.Fatalf("reassignment changed the destination's private visibility: %v", result)
	}
}

// TestReassignmentSupportsBindingsAndMembers verifies reassignment supports bindings and members.
func TestReassignmentSupportsBindingsAndMembers(t *testing.T) {
	result := evalText(t, `
value := 1;
value = 2;
holder: scope = { field := 1; }
holder.field = 3;
Point: setup = { x: integer; }
point := Point { x = 1 };
point.x = 4;
items := []integer{1, 2};
items[0] = 5;
values := mapping(string, integer){["a"] => 1};
values["a"] = 6;
`)
	if result["value"] != 2 {
		t.Fatalf("identifier reassignment failed: %v", result)
	}
	if result["holder"].(map[string]any)["field"] != 3 || result["point"].(map[string]any)["x"] != 4 {
		t.Fatalf("field reassignment failed: %v", result)
	}
	if result["items"].([]any)[0] != 5 || result["values"].(map[string]any)["a"] != 6 {
		t.Fatalf("indexed reassignment failed: %v", result)
	}
}

// TestReassignmentEnforcesExistingDestinationAndType verifies reassignment enforces existing destination and type.
func TestReassignmentEnforcesExistingDestinationAndType(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `missing = 1;`)
	}, "Cannot reassign undeclared variable 'missing'")
	assertPanic(t, func() {
		evalText(t, `value := 1; value = "wrong";`)
	}, "Type mismatch in reassignment: expected integer, got string")
	assertPanic(t, func() {
		evalText(t, `values := mapping(string, integer){["a"] => 1}; values["missing"] = 2;`)
	}, "Cannot reassign a missing mapping key")
}

// TestDeclarationsRejectNameAndDestinationConflicts verifies declarations reject name and destination conflicts.
func TestDeclarationsRejectNameAndDestinationConflicts(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `Thing: setup = { value: integer; } Thing := 1;`)
	}, "Name 'Thing' is already used by a type")
	assertPanic(t, func() {
		evalText(t, `value := 1; value: setup = { field: integer; }`)
	}, "Name 'value' is already used by a value")
	assertPanic(t, func() {
		evalText(t, `Point: setup = { x: integer; } point := Point { x = 1 }; point.y := 2;`)
	}, "New members can only be declared in a scope or namespace")
	assertPanic(t, func() {
		evalText(t, `items := []integer{1}; items[0] := 2;`)
	}, "Indexed members cannot be declared")
	assertPanic(t, func() {
		evalText(t, `key: scope = { value := 1; } values := mapping(scope, string){[key] => "invalid"};`)
	}, "Mapping key type must be primitive or enum")
}

// TestEnumIndexedCollectionsRetainIndexTypesDuringEvaluation verifies enum indexed collections retain index types during evaluation.
func TestEnumIndexedCollectionsRetainIndexTypesDuringEvaluation(t *testing.T) {
	result := evalText(t, `
Color: enum = { red; green; }
items: [Color]string = []string{"red", "green"};
selected := items[.green];
items[.red] = "RED";
values := mapping(Color, string){[.red] => "r", [.green] => "g"};
values[.green] = "G";
mapped := values[.green];
`)
	if result["selected"] != "green" || result["items"].([]any)[0] != "RED" || result["mapped"] != "G" {
		t.Fatalf("enum-indexed access or reassignment failed: %v", result)
	}
}

// TestIterationAndAsLvalueExpandToConcreteDestinations verifies iteration and as lvalue expand to concrete destinations.
func TestIterationAndAsLvalueExpandToConcreteDestinations(t *testing.T) {
	result := evalText(t, `
holder: scope = { value := 1; }
member_name := "holder.value";
#as_lvalue(member_name) = 2;
#as_lvalue("holder.created") := "member";
new_name := "created";
#as_lvalue(new_name) := "value";
#iteration("_entry") := "first";
next_name := #iteration("_entry");
`)
	holder := result["holder"].(map[string]any)
	if holder["value"] != 2 || holder["created"] != "member" || result["created"] != "value" {
		t.Fatalf("#as_lvalue did not produce declaration and reassignment targets: %v", result)
	}
	if result["iteration_0000000000_entry"] != "first" || result["next_name"] != "iteration_0000000001_entry" {
		t.Fatalf("#iteration did not produce the expected lvalue and rvalue: %v", result)
	}
}

// TestIncludeExpansionUsesSourcePosition verifies include expansion uses source position.
func TestIncludeExpansionUsesSourcePosition(t *testing.T) {
	directory := t.TempDir()
	includedPath := filepath.Join(directory, "included.something")
	if err := os.WriteFile(includedPath, []byte(`copy := before;`), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(directory, "main.something")
	mainSource := `before := "available"; #include("included.something"); after := copy;`
	if err := os.WriteFile(mainPath, []byte(mainSource), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := LoadSomethingFile(mainPath)
	if err != nil {
		t.Fatalf("include expansion failed: %v", err)
	}
	if result["after"] != "available" {
		t.Fatalf("included statements were not expanded at their source position: %v", result)
	}
}

// TestIncludeCyclesReportTheDependencyChain verifies include cycles report the dependency chain.
func TestIncludeCyclesReportTheDependencyChain(t *testing.T) {
	directory := t.TempDir()
	aPath := filepath.Join(directory, "a.something")
	bPath := filepath.Join(directory, "b.something")
	if err := os.WriteFile(aPath, []byte(`#include("b.something");`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte(`#include("a.something");`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSomethingFile(aPath)
	if err == nil {
		t.Fatal("expected an include cycle error")
	}
	if !strings.Contains(err.Error(), "Circular include dependency: a.something -> b.something -> a.something") {
		t.Fatalf("include cycle did not report its chain: %v", err)
	}
}

// TestMacroExpansionChecksBodyInputsAndOutput verifies macro expansion checks body inputs and output.
func TestMacroExpansionChecksBodyInputsAndOutput(t *testing.T) {
	result := evalText(t, `
#macro identity := (input: string) -> string {
    local := input;
    #set local;
}
output := identity!("value");
`)
	if result["output"] != "value" {
		t.Fatalf("macro expansion returned the wrong value: %v", result)
	}

	assertPanic(t, func() {
		evalText(t, `
#macro invalid_body := (input: string) -> string {
    local: integer = input;
    #set "unused";
}
output := invalid_body!("value");
`)
	}, "Type mismatch in assignment: expected integer, got string")
	assertPanic(t, func() {
		evalText(t, `
#macro invalid_output := () -> integer { #set "wrong"; }
output := invalid_output!();
`)
	}, "Type mismatch in macro 'invalid_output' return: expected integer, got string")
	assertPanic(t, func() {
		evalText(t, `#macro duplicate := (value: string, value: string) -> string { #set value; }`)
	}, "declares parameter 'value' more than once")
}

// TestMacroExpansionPreservesDeclaredResultTypes verifies macro expansion preserves declared result types.
func TestMacroExpansionPreservesDeclaredResultTypes(t *testing.T) {
	_, expanded, checked := compileText(t, `
#macro timestamp_value := () -> timestamp { #set "2026-01-01 00:00:00"; }
value := timestamp_value!();
`)
	valueAssignment := expanded.Statements[0].(*Assignment)
	if typeRefString(checked.AssignmentTypes[valueAssignment]) != "timestamp" {
		t.Fatalf("inferred macro result type was not preserved: %s", typeRefString(checked.AssignmentTypes[valueAssignment]))
	}

	result := evalText(t, `
Color: enum = { red; green; }
#macro empty_list := () -> []string { #set []string{}; }
#macro empty_mapping := () -> mapping(string, integer) { #set mapping(string, integer){}; }
#macro indexed := () -> [Color]string { #set []string{"red", "green"}; }
list := empty_list!();
mapping_value := empty_mapping!();
items := indexed!();
selected := items[.green];
`)
	if len(result["list"].([]any)) != 0 || len(result["mapping_value"].(map[string]any)) != 0 {
		t.Fatalf("empty typed macro results were not preserved: %v", result)
	}
	if result["selected"] != "green" {
		t.Fatalf("enum-indexed macro result lost its index type: %v", result)
	}
}

// TestRuntimeDiagnosticTypeNamesAndKeyEquality verifies runtime diagnostic type names and key equality.
func TestRuntimeDiagnosticTypeNamesAndKeyEquality(t *testing.T) {
	values := []struct {
		value any
		name  string
	}{
		{"value", "string"},
		{1, "integer"},
		{1.5, "float"},
		{true, "boolean"},
		{[]any{}, "array"},
		{&runtimeMapping{}, "mapping"},
		{&runtimeObject{}, "object"},
		{&EnumValue{}, "enum"},
		{struct{}{}, "struct {}"},
	}
	for _, test := range values {
		if actual := runtimeTypeName(test.value); actual != test.name {
			t.Fatalf("runtimeTypeName(%T) = %q, expected %q", test.value, actual, test.name)
		}
	}

	red := &EnumValue{EnumName: "Color", Ordinal: 0}
	otherRed := &EnumValue{EnumName: "Color", Ordinal: 0}
	blue := &EnumValue{EnumName: "Color", Ordinal: 1}
	if !runtimeKeysEqual(red, otherRed) || !runtimeKeysEqual(red, 0) || !runtimeKeysEqual(0, red) {
		t.Fatal("equivalent enum runtime keys did not compare equal")
	}
	if runtimeKeysEqual(red, blue) || runtimeKeysEqual("0", 0) {
		t.Fatal("different runtime keys compared equal")
	}

	keys := sortedBindingTypeKeys(map[string]*BindingType{"z": {}, "a": {}})
	if strings.Join(keys, ",") != "a,z" {
		t.Fatalf("binding type keys are not sorted: %v", keys)
	}
}

// TestMacroRecursionReportsDirectAndIndirectChains verifies macro recursion reports direct and indirect chains.
func TestMacroRecursionReportsDirectAndIndirectChains(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `
#macro repeat := () -> string { #set repeat!(); }
output := repeat!();
`)
	}, "Recursive macro expansion: repeat -> repeat")
	assertPanic(t, func() {
		evalText(t, `
#macro first := () -> string { #set second!(); }
#macro second := () -> string { #set first!(); }
output := first!();
`)
	}, "Recursive macro expansion: first -> second -> first")
}

// TestValueAndTypeCyclesReportDependencyChains verifies value and type cycles report dependency chains.
func TestValueAndTypeCyclesReportDependencyChains(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `value := value;`)
	}, "Circular dependency: value -> value")
	assertPanic(t, func() {
		evalText(t, `first := second; second := first;`)
	}, "Circular dependency: first -> second -> first")
	assertPanic(t, func() {
		evalText(t, `Node: setup = { next: Node; }`)
	}, "Circular dependency: type:Node -> type:Node")
	assertPanic(t, func() {
		evalText(t, `First: setup = { next: Second; } Second: setup = { next: First; }`)
	}, "Circular dependency: type:First -> type:Second -> type:First")
	assertPanic(t, func() {
		evalText(t, `container: scope = { first := second.value; second: scope = { value := first; } }`)
	}, "Circular dependency: container.first -> container.second.value -> container.first")
	assertPanic(t, func() {
		evalText(t, `container: scope = { First: setup = { next: Second; } Second: setup = { next: First; } }`)
	}, "Circular dependency: type:container.First -> type:container.Second -> type:container.First")
	assertPanic(t, func() {
		evalText(t, `#for item: [1] { first := second; second := first; }`)
	}, "Circular dependency: first -> second -> first")
	assertPanic(t, func() {
		evalText(t, `#insert { "first := second; second := first;" };`)
	}, "Circular dependency: first -> second -> first")
}

// TestRequiredStatementTerminators verifies required statement terminators.
func TestRequiredStatementTerminators(t *testing.T) {
	valid := []string{
		`value: string = "declared";`,
		`value := "declared";`,
		`value := "declared"; value = "reassigned";`,
		`#priv value := "private";`,
		`#iteration: string = "value";`,
		`#iteration("_label") := 10;`,
		`#as_lvalue("value") := "dynamic";`,
		`#include("file.something");`,
		`included := #include("file.something");`,
		`#insert {};`,
		"value: string = #multiline END\ntext\nEND;",
		`#macro value := () -> string { #set "result"; } result := value!();`,
	}
	for _, source := range valid {
		NewParser(NewLexer(source, "").Tokenize(), "").ParseProgram()
	}

	invalid := []string{
		`value: string = "declared"`,
		`value := "declared"`,
		`value := "declared"; value = "reassigned"`,
		`#priv value := "private"`,
		`#iteration: string = "value"`,
		`#iteration("_label") := 10`,
		`#as_lvalue("value") := "dynamic"`,
		`#include("file.something")`,
		`included := #include("file.something")`,
		`#insert {}`,
		"value: string = #multiline END\ntext\nEND",
		`#macro value := () -> string { #set "result" }`,
		`#macro value := () -> string { #set "result"; } result := value!()`,
	}
	for _, source := range invalid {
		assertPanic(t, func() {
			NewParser(NewLexer(source, "").Tokenize(), "").ParseProgram()
		}, "Expected ';'")
	}
}

// TestCompoundDeclarationsRejectTrailingSemicolons verifies compound declarations reject trailing semicolons.
func TestCompoundDeclarationsRejectTrailingSemicolons(t *testing.T) {
	valid := []string{
		`value: scope = { nested := 1; }`,
		`Value: setup = { nested: integer; }`,
		`Value: enum = { A; }`,
		`#for item: []integer{1} { value := item; }`,
		`#macro value := () -> string { #set "result"; }`,
	}
	for _, source := range valid {
		NewParser(NewLexer(source, "").Tokenize(), "").ParseProgram()
	}

	for _, source := range valid {
		assertPanic(t, func() {
			NewParser(NewLexer(source+";", "").Tokenize(), "").ParseProgram()
		}, "Expected an assignment or directive, got SEMICOLON")
	}
}

// TestDefinitionMembersAndLiteralElementsRequireTheirSeparators verifies definition members and literal elements require their separators.
func TestDefinitionMembersAndLiteralElementsRequireTheirSeparators(t *testing.T) {
	valid := []string{
		`Value: setup = { first: string; second?: float = 0.1; }`,
		`Value: enum = { A; B; C; }`,
		`Value: enum(string) = { A = "A"; B = "B"; C = "C"; }`,
		`Value: setup = { first: string; second?: float = 0.1; } value: Value = { first = "", second = 0.1 };`,
		`values := ["a", "b", "c",];`,
		`values := []string{"a", "b", "c"};`,
		`values := []string{"a", "b", "c",};`,
		`values := mapping(string, string){["a"] => "a", ["b"] => "b"};`,
		`values := mapping(string, string){["a"] => "a", ["b"] => "b",};`,
		`#insert { "a := 10;", "b := a;", };`,
	}
	for _, source := range valid {
		NewParser(NewLexer(source, "").Tokenize(), "").ParseProgram()
	}

	invalid := []struct {
		source  string
		message string
	}{
		{`Value: setup = { first: string; second: integer }`, "Expected ';' after setup field"},
		{`Value: enum = { A; B; C }`, "Expected ';' after enum member"},
		{`Value: enum(string) = { A = "A"; B = "B"; C = "C" }`, "Expected ';' after enum member"},
		{`Value: setup = { first: string; second: integer; } value: Value = { first = "" second = 1 };`, "Expected ',' between struct fields"},
		{`values := ["a" "b"];`, "Expected ',' between array elements"},
		{`values := ["a"; "b"];`, "Expected ',' between array elements"},
		{`values := []string{"a" "b"};`, "Expected ',' between typed array elements"},
		{`values := []string{"a"; "b"};`, "Expected ',' between typed array elements"},
		{`values := mapping(string, string){["a"] => "a" ["b"] => "b"};`, "Expected ',' between mapping entries"},
		{`values := mapping(string, string){["a"] => "a"; ["b"] => "b"};`, "Expected ',' between mapping entries"},
		{`#insert { "a := 10;" "b := a;" };`, "Expected ',' between #insert values"},
	}
	for _, test := range invalid {
		assertPanic(t, func() {
			NewParser(NewLexer(test.source, "").Tokenize(), "").ParseProgram()
		}, test.message)
	}
}

// TestRequiredColons verifies required colons.
func TestRequiredColons(t *testing.T) {
	invalid := []struct {
		source  string
		message string
	}{
		{`value string = "declared";`, "Expected ':', ':=', or '=' after assignment target"},
		{`value scope = {}`, "Expected ':', ':=', or '=' after assignment target"},
		{`Value setup = { field: string; }`, "Expected ':', ':=', or '=' after assignment target"},
		{`Value: setup = { field string; }`, "Expected ':' after setup field name"},
		{`#for item []integer{1} { value := item; }`, "Expected ':' after #for variables"},
		{`#macro value = () -> string { #set "result"; }`, "Expected ':=' after macro name"},
	}
	for _, test := range invalid {
		assertPanic(t, func() {
			NewParser(NewLexer(test.source, "").Tokenize(), "").ParseProgram()
		}, test.message)
	}
}

// TestEmptyInsertIsANoOp verifies empty insert is a no op.
func TestEmptyInsertIsANoOp(t *testing.T) {
	result := evalText(t, `#insert {}; value := 10;`)
	if len(result) != 1 || result["value"] != 10 {
		t.Fatalf("empty #insert changed evaluation: %v", result)
	}
}

// TestAssertValidatesSetupInstance verifies assert validates setup instance.
func TestAssertValidatesSetupInstance(t *testing.T) {
	// Assertions run when a struct instance of the asserted type is created.
	// The assertion body runs in the instance's scope, so variables declared
	// inside become fields on the instance object.
	result := evalText(t, `
Point: setup = { x: integer; y: integer; }
#assert Point {
    valid := false;
    #if x > 5 {
        valid = true;
    }
}
p := Point { x = 10, y = 20 };
`)
	p := result["p"].(map[string]any)
	if p["valid"] != true {
		t.Fatalf("#assert body did not run on struct instantiation: %v", p)
	}
}

// TestAssertPanicsOnNonSetup verifies assert panics on non setup.
func TestAssertPanicsOnNonSetup(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `
value: string = "hello";
#assert value {}
`)
	}, "Undefined assertion target type")
}

// TestAssertPanicsOnUndefinedTarget verifies assert panics on undefined target.
func TestAssertPanicsOnUndefinedTarget(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#assert Missing {}`)
	}, "Undefined assertion target type")
}

// TestAssertTriggersErrorOnInvalidInstance verifies assert triggers error on invalid instance.
func TestAssertTriggersErrorOnInvalidInstance(t *testing.T) {
	// Assertion with #error should fire when an instance has invalid values
	assertPanic(t, func() {
		evalText(t, `
Cfg: setup = { name: string; version: integer; }
#assert Cfg {
    #if #not #match(name, "^[a-z]+$") {
        #error("Invalid name '{name}'");
    }
}
cfg := Cfg { name = "INVALID!", version = 1 };
`)
	}, "Invalid name 'INVALID!'")
}

// TestAssertErrorIncludesInstanceLocation verifies assert error includes instance location.
func TestAssertErrorIncludesInstanceLocation(t *testing.T) {
	// The error from an assertion should include the instance location in the suggestion
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		se, ok := r.(*SomethingError)
		if !ok {
			t.Fatalf("expected *SomethingError, got %T", r)
		}
		if !strings.Contains(se.Suggestion, "Assertion failed for instance") {
			t.Fatalf("suggestion should mention instance location, got: %s", se.Suggestion)
		}
	}()
	evalText(t, `
Cfg: setup = { name: string; version: integer; }
#assert Cfg {
    #if #not #match(name, "^[a-z]+$") {
        #error("Invalid name '{name}'");
    }
}
cfg := Cfg { name = "INVALID!", version = 1 };
`)
}

// TestAssertPassesOnValidInstance verifies assert passes on valid instance.
func TestAssertPassesOnValidInstance(t *testing.T) {
	// Assertion with #error should NOT fire when an instance has valid values
	result := evalText(t, `
Cfg: setup = { name: string; version: integer; }
#assert Cfg {
    #if #not #match(name, "^[a-z]+$") {
        #error("Invalid name '{name}'");
    }
}
cfg := Cfg { name = "valid", version = 1 };
`)
	if _, ok := result["cfg"]; !ok {
		t.Fatalf("valid instance should not trigger #error: %v", result)
	}
}

// TestIfTrueExecutesBody verifies if true executes body.
func TestIfTrueExecutesBody(t *testing.T) {
	result := evalText(t, `
result := "no";
#if true {
    result = "yes";
}
`)
	if result["result"] != "yes" {
		t.Fatalf("#if true did not execute body: %v", result)
	}
}

// TestIfFalseSkipsBody verifies if false skips body.
func TestIfFalseSkipsBody(t *testing.T) {
	result := evalText(t, `
result := "no";
#if false {
    result = "yes";
}
`)
	if result["result"] != "no" {
		t.Fatalf("#if false executed body when it should not: %v", result)
	}
}

// TestIfWithSingleStatement verifies if with single statement.
func TestIfWithSingleStatement(t *testing.T) {
	result := evalText(t, `
result := "no";
#if true result = "yes";
`)
	if result["result"] != "yes" {
		t.Fatalf("#if with single statement form did not execute: %v", result)
	}
}

// TestIfWithFalseSingleStatement verifies if with false single statement.
func TestIfWithFalseSingleStatement(t *testing.T) {
	result := evalText(t, `
result := "no";
#if false result = "yes";
`)
	if result["result"] != "no" {
		t.Fatalf("#if false executed single statement when it should not: %v", result)
	}
}

// TestErrorDirectivePanics verifies error directive panics.
func TestErrorDirectivePanics(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `#error("custom error message");`)
	}, "custom error message")
}

// TestErrorDirectiveUsesInterpolation verifies error directive uses interpolation.
func TestErrorDirectiveUsesInterpolation(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `name := "world"; #error("Hello {name}");`)
	}, "Hello world")
}

// TestComparisonOperators verifies comparison operators.
func TestComparisonOperators(t *testing.T) {
	result := evalText(t, `
eq := 5 == 5;
neq := 5 != 3;
lt := 3 < 5;
le1 := 3 <= 5;
le2 := 5 <= 5;
gt := 5 > 3;
ge1 := 5 >= 3;
ge2 := 5 >= 5;
`)
	if result["eq"] != true || result["neq"] != true {
		t.Fatalf("equality operators failed: %v", result)
	}
	if result["lt"] != true || result["le1"] != true || result["le2"] != true {
		t.Fatalf("less-than operators failed: %v", result)
	}
	if result["gt"] != true || result["ge1"] != true || result["ge2"] != true {
		t.Fatalf("greater-than operators failed: %v", result)
	}
}

// TestBooleanOperators verifies boolean operators.
func TestBooleanOperators(t *testing.T) {
	result := evalText(t, `
and_true := true #and true;
and_false := true #and false;
or_true := false #or true;
or_false := false #or false;
not_true := #not false;
not_false := #not true;
`)
	if result["and_true"] != true || result["and_false"] != false {
		t.Fatalf("#and operator failed: %v", result)
	}
	if result["or_true"] != true || result["or_false"] != false {
		t.Fatalf("#or operator failed: %v", result)
	}
	if result["not_true"] != true || result["not_false"] != false {
		t.Fatalf("#not operator failed: %v", result)
	}
}

// TestShortCircuitAnd verifies short circuit and.
func TestShortCircuitAnd(t *testing.T) {
	// false #and side_effect should not evaluate RHS
	result := evalText(t, `
items := []boolean{true, false};
result := false #and items[5];
`)
	if result["result"] != false {
		t.Fatalf("short-circuit #and failed: got %v, expected false", result["result"])
	}

	// true #and side_effect MUST evaluate RHS (should panic on out-of-bounds)
	assertPanic(t, func() {
		evalText(t, `
items := []boolean{true, false};
result := true #and items[5];
`)
	}, "out of bounds")
}

// TestShortCircuitOr verifies short circuit or.
func TestShortCircuitOr(t *testing.T) {
	// true #or side_effect should not evaluate RHS
	result := evalText(t, `
items := []boolean{true, false};
result := true #or items[5];
`)
	if result["result"] != true {
		t.Fatalf("short-circuit #or failed: got %v, expected true", result["result"])
	}

	// false #or side_effect MUST evaluate RHS (should panic on out-of-bounds)
	assertPanic(t, func() {
		evalText(t, `
items := []boolean{true, false};
result := false #or items[5];
`)
	}, "out of bounds")
}

// TestParenthesizedGrouping verifies parenthesized grouping.
func TestParenthesizedGrouping(t *testing.T) {
	result := evalText(t, `
result := (3 < 5) #and (5 > 3);
`)
	if result["result"] != true {
		t.Fatalf("parenthesized grouping failed: %v", result)
	}
}

// TestMatchExpression verifies match expression.
func TestMatchExpression(t *testing.T) {
	result := evalText(t, `
match_true := #match("hello world", "hello");
match_false := #match("hello world", "goodbye");
`)
	if result["match_true"] != true || result["match_false"] != false {
		t.Fatalf("#match operator failed: %v", result)
	}
}

// TestMatchInvalidRegex verifies match invalid regex.
func TestMatchInvalidRegex(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `result := #match("hello", "[invalid");`)
	}, "invalid regex pattern")
}

// TestLenOnArray verifies len on array.
func TestLenOnArray(t *testing.T) {
	result := evalText(t, `
items := []string{"a", "b", "c"};
count := #len(items);
`)
	if result["count"] != 3 {
		t.Fatalf("#len on array failed: got %v, expected 3", result["count"])
	}
}

// TestLenOnEmptyArray verifies len on empty array.
func TestLenOnEmptyArray(t *testing.T) {
	result := evalText(t, `
items := []string{};
count := #len(items);
`)
	if result["count"] != 0 {
		t.Fatalf("#len on empty array failed: got %v, expected 0", result["count"])
	}
}

// TestLenOnMapping verifies len on mapping.
func TestLenOnMapping(t *testing.T) {
	result := evalText(t, `
m := mapping(string, integer){["a"] => 1, ["b"] => 2};
count := #len(m);
`)
	if result["count"] != 2 {
		t.Fatalf("#len on mapping failed: got %v, expected 2", result["count"])
	}
}

// TestOperatorPrecedence verifies operator precedence.
func TestOperatorPrecedence(t *testing.T) {
	result := evalText(t, `
// and binds tighter than or
result := false #and false #or true;
`)
	if result["result"] != true {
		t.Fatalf("precedence (#and > #or) failed: got %v, expected true", result["result"])
	}
}

// TestStringEscapingDoubleBrace verifies string escaping double brace.
func TestStringEscapingDoubleBrace(t *testing.T) {
	result := evalText(t, `
literal := "Hello {{name}}";
`)
	expected := "Hello {name}"
	if result["literal"] != expected {
		t.Fatalf("string escaping failed: got %q, expected %q", result["literal"], expected)
	}
}

// TestNestedIfInAssert verifies nested if in assert.
func TestNestedIfInAssert(t *testing.T) {
	result := evalText(t, `
Point: setup = { x: integer; y: integer; }
#assert Point {
    is_big := false;
    is_twenty := false;
    #if x > 5 {
        is_big = true;
    }
    #if y == 20 {
        is_twenty = true;
    }
}
p := Point { x = 10, y = 20 };
`)
	p := result["p"].(map[string]any)
	if p["is_big"] != true || p["is_twenty"] != true {
		t.Fatalf("nested #if inside #assert failed: %v", p)
	}
}

// TestUnknownDirectiveErrorMessage verifies unknown directive error message.
func TestUnknownDirectiveErrorMessage(t *testing.T) {
	assertPanic(t, func() {
		parseText(t, "#unknown_directive")
	}, "Unknown directive")
}
