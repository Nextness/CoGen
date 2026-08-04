// directive_generator_unit_test.go contains unit tests for directive_generator.go
// that exercise pure functions and methods on DirectiveGenerator directly,
// without going through the full SOMETHING pipeline.
//go:build unit

package something

import "testing"

// TestNewDirectiveGenerator verifies new directive generator.
func TestNewDirectiveGenerator(t *testing.T) {
	gen := NewDirectiveGenerator("test.something")
	if gen == nil {
		t.Fatal("expected non-nil DirectiveGenerator")
	}
	if gen.filepath != "test.something" {
		t.Errorf("expected filepath 'test.something', got %q", gen.filepath)
	}
	if gen.runtime == nil {
		t.Error("expected non-nil runtime")
	}
	if gen.macros == nil {
		t.Error("expected non-nil macro environment")
	}
	if gen.iterationCount != 0 {
		t.Errorf("expected iterationCount 0, got %d", gen.iterationCount)
	}
	if len(gen.iterationCounters) != 0 {
		t.Errorf("expected empty iterationCounters, got %d", len(gen.iterationCounters))
	}
}

// TestNextIterationKeyDefault verifies next iteration key default.
func TestNextIterationKeyDefault(t *testing.T) {
	gen := NewDirectiveGenerator("")
	key := gen.nextIterationKey("")
	if key != "iteration_0000000000" {
		t.Errorf("expected 'iteration_0000000000', got %q", key)
	}
	key2 := gen.nextIterationKey("")
	if key2 != "iteration_0000000001" {
		t.Errorf("expected 'iteration_0000000001', got %q", key2)
	}
}

// TestNextIterationKeyLabeled verifies next iteration key labeled.
func TestNextIterationKeyLabeled(t *testing.T) {
	gen := NewDirectiveGenerator("")
	key := gen.nextIterationKey("_my_label")
	if key != "iteration_0000000000_my_label" {
		t.Errorf("expected 'iteration_0000000000_my_label', got %q", key)
	}
	key2 := gen.nextIterationKey("_my_label")
	if key2 != "iteration_0000000001_my_label" {
		t.Errorf("expected 'iteration_0000000001_my_label', got %q", key2)
	}
}

// TestNextIterationKeyIndependentCounters verifies next iteration key independent counters.
func TestNextIterationKeyIndependentCounters(t *testing.T) {
	gen := NewDirectiveGenerator("")
	// Default counter advances independently from labeled counters
	k1 := gen.nextIterationKey("")
	kA1 := gen.nextIterationKey("_a")
	k2 := gen.nextIterationKey("")
	if k1 != "iteration_0000000000" {
		t.Errorf("expected 'iteration_0000000000', got %q", k1)
	}
	if kA1 != "iteration_0000000000_a" {
		t.Errorf("expected 'iteration_0000000000_a', got %q", kA1)
	}
	if k2 != "iteration_0000000001" {
		t.Errorf("expected 'iteration_0000000001', got %q", k2)
	}
}

// TestPeekIterationKey verifies peek iteration key.
func TestPeekIterationKey(t *testing.T) {
	gen := NewDirectiveGenerator("")
	// Peek before any iteration returns 0
	key := gen.peekIterationKey("")
	if key != "iteration_0000000000" {
		t.Errorf("expected 'iteration_0000000000' before any iteration, got %q", key)
	}
	// After one default iteration, peek should return 1 (nextIterationKey advances then peek reads current)
	gen.nextIterationKey("")
	key2 := gen.peekIterationKey("")
	if key2 != "iteration_0000000001" {
		t.Errorf("expected 'iteration_0000000001' after one nextIterationKey, got %q", key2)
	}
	// Advance again to 2, peek should read 2
	gen.nextIterationKey("")
	key3 := gen.peekIterationKey("")
	if key3 != "iteration_0000000002" {
		t.Errorf("expected 'iteration_0000000002', got %q", key3)
	}
}

// TestPeekIterationKeyLabeled verifies peek iteration key labeled.
func TestPeekIterationKeyLabeled(t *testing.T) {
	gen := NewDirectiveGenerator("")
	gen.nextIterationKey("_x")
	key := gen.peekIterationKey("_x")
	if key != "iteration_0000000001_x" {
		t.Errorf("expected 'iteration_0000000001_x', got %q", key)
	}
}

// TestPathInIncludeStackEmpty verifies path in include stack empty.
func TestPathInIncludeStackEmpty(t *testing.T) {
	gen := NewDirectiveGenerator("")
	if gen.pathInIncludeStack("/any/path") {
		t.Error("expected false for empty include stack")
	}
}

// TestPathInIncludeStackFound verifies path in include stack found.
func TestPathInIncludeStackFound(t *testing.T) {
	gen := NewDirectiveGenerator("")
	gen.includeStack = []string{"/first", "/second"}
	if !gen.pathInIncludeStack("/first") {
		t.Error("expected true for '/first' in stack")
	}
	if !gen.pathInIncludeStack("/second") {
		t.Error("expected true for '/second' in stack")
	}
}

// TestPathInIncludeStackNotFound verifies path in include stack not found.
func TestPathInIncludeStackNotFound(t *testing.T) {
	gen := NewDirectiveGenerator("")
	gen.includeStack = []string{"/first", "/second"}
	if gen.pathInIncludeStack("/third") {
		t.Error("expected false for '/third' not in stack")
	}
}

// TestCleanKnownPathAlreadyClean verifies clean known path already clean.
func TestCleanKnownPathAlreadyClean(t *testing.T) {
	result := cleanKnownPath("/a/b/c")
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// TestCleanKnownPathWithRelative verifies clean known path with relative.
func TestCleanKnownPathWithRelative(t *testing.T) {
	result := cleanKnownPath("/a/b/../c")
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// TestNewMacroEnvironmentNoParent verifies new macro environment no parent.
func TestNewMacroEnvironmentNoParent(t *testing.T) {
	env := newMacroEnvironment(nil)
	if env == nil {
		t.Fatal("expected non-nil environment")
	}
	if env.parent != nil {
		t.Error("expected nil parent")
	}
	if env.definitions == nil {
		t.Error("expected non-nil definitions map")
	}
	if _, ok := env.lookup("nonexistent"); ok {
		t.Error("expected false for nonexistent macro")
	}
}

// TestNewMacroEnvironmentWithParent verifies new macro environment with parent.
func TestNewMacroEnvironmentWithParent(t *testing.T) {
	parent := newMacroEnvironment(nil)
	child := newMacroEnvironment(parent)
	if child.parent != parent {
		t.Error("expected parent to be set")
	}
}

// TestStringExpressionBasic verifies string expression basic.
func TestStringExpressionBasic(t *testing.T) {
	loc := &SourceLocation{Line: 1, Col: 5}
	expr := stringExpression("hello", loc)
	if expr == nil {
		t.Fatal("expected non-nil expression")
	}
	if expr.Literal != nil {
		combined := ""
		for _, part := range expr.Literal.Parts {
			if text, ok := part.(StringText); ok {
				combined += string(text)
			}
		}
		if combined != "hello" {
			t.Errorf("expected 'hello', got %q", combined)
		}
	}
}

// TestExpressionFromRuntimeString verifies expression from runtime string.
func TestExpressionFromRuntimeString(t *testing.T) {
	gen := NewDirectiveGenerator("")
	expr := gen.expressionFromRuntime("hello", nil)
	se, ok := expr.(*StringExpression)
	if !ok {
		t.Fatalf("expected *StringExpression, got %T", expr)
	}
	if se.Location != nil {
		t.Error("expected nil location")
	}
}

// TestExpressionFromRuntimeInteger verifies expression from runtime integer.
func TestExpressionFromRuntimeInteger(t *testing.T) {
	gen := NewDirectiveGenerator("")
	expr := gen.expressionFromRuntime(42, nil)
	ie, ok := expr.(*IntegerExpression)
	if !ok {
		t.Fatalf("expected *IntegerExpression, got %T", expr)
	}
	if ie.Value != 42 {
		t.Errorf("expected 42, got %d", ie.Value)
	}
}

// TestExpressionFromRuntimeFloat verifies expression from runtime float.
func TestExpressionFromRuntimeFloat(t *testing.T) {
	gen := NewDirectiveGenerator("")
	expr := gen.expressionFromRuntime(3.14, nil)
	fe, ok := expr.(*FloatExpression)
	if !ok {
		t.Fatalf("expected *FloatExpression, got %T", expr)
	}
	if fe.Value != 3.14 {
		t.Errorf("expected 3.14, got %f", fe.Value)
	}
}

// TestExpressionFromRuntimeBool verifies expression from runtime bool.
func TestExpressionFromRuntimeBool(t *testing.T) {
	gen := NewDirectiveGenerator("")
	expr := gen.expressionFromRuntime(true, nil)
	be, ok := expr.(*BooleanExpression)
	if !ok {
		t.Fatalf("expected *BooleanExpression, got %T", expr)
	}
	if be.Value != true {
		t.Errorf("expected true, got %v", be.Value)
	}
}

// TestExpressionFromRuntimeArray verifies expression from runtime array.
func TestExpressionFromRuntimeArray(t *testing.T) {
	gen := NewDirectiveGenerator("")
	expr := gen.expressionFromRuntime([]any{"a", 1}, nil)
	ae, ok := expr.(*ArrayExpression)
	if !ok {
		t.Fatalf("expected *ArrayExpression, got %T", expr)
	}
	if len(ae.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(ae.Elements))
	}
}

// TestConstantStackPushPopLookup verifies constant stack push pop lookup.
func TestConstantStackPushPopLookup(t *testing.T) {
	gen := NewDirectiveGenerator("")
	// Empty stack lookup
	if _, ok := gen.lookupConstant("x"); ok {
		t.Error("expected false for empty stack")
	}
	// Push and lookup
	gen.pushConstants(map[string]any{"x": 1, "y": "two"})
	val, ok := gen.lookupConstant("x")
	if !ok {
		t.Fatal("expected true for 'x'")
	}
	if val != 1 {
		t.Errorf("expected 1, got %v", val)
	}
	val2, ok := gen.lookupConstant("y")
	if !ok {
		t.Fatal("expected true for 'y'")
	}
	if val2 != "two" {
		t.Errorf("expected 'two', got %v", val2)
	}
	// Push another layer, shadows
	gen.pushConstants(map[string]any{"x": 999})
	val3, _ := gen.lookupConstant("x")
	if val3 != 999 {
		t.Errorf("expected 999 (shadowed), got %v", val3)
	}
	// Pop, original restored
	gen.popConstants()
	val4, _ := gen.lookupConstant("x")
	if val4 != 1 {
		t.Errorf("expected 1 (restored), got %v", val4)
	}
	// Pop again, stack empty
	gen.popConstants()
	if _, ok := gen.lookupConstant("x"); ok {
		t.Error("expected false after final pop")
	}
}

// TestExpandSetupDefinitionWithoutDefaults verifies expand setup definition without defaults.
func TestExpandSetupDefinitionWithoutDefaults(t *testing.T) {
	gen := NewDirectiveGenerator("")
	source := &SetupDefinition{
		Fields: []*FieldDefinition{
			{Name: "x", DeclaredType: PrimString},
		},
	}
	result := gen.expandSetupDefinition(source)
	if len(result.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(result.Fields))
	}
	if result.Fields[0].Name != "x" {
		t.Errorf("expected 'x', got %q", result.Fields[0].Name)
	}
	if result.Fields[0].DefaultValue != nil {
		t.Error("expected nil DefaultValue")
	}
	// Original should be unmodified (copy semantics)
	if source.Fields[0].DefaultValue != nil {
		t.Error("original should remain unmodified")
	}
}

// TestExpandSetupDefinitionWithDefaults verifies expand setup definition with defaults.
func TestExpandSetupDefinitionWithDefaults(t *testing.T) {
	gen := NewDirectiveGenerator("")
	source := &SetupDefinition{
		Fields: []*FieldDefinition{
			{
				Name:         "y",
				DeclaredType: PrimInteger,
				DefaultValue: &IntegerExpression{Value: 42},
			},
		},
	}
	result := gen.expandSetupDefinition(source)
	if result.Fields[0].DefaultValue == nil {
		t.Fatal("expected non-nil DefaultValue")
	}
	ie, ok := result.Fields[0].DefaultValue.(*IntegerExpression)
	if !ok {
		t.Fatalf("expected *IntegerExpression, got %T", result.Fields[0].DefaultValue)
	}
	if ie.Value != 42 {
		t.Errorf("expected 42, got %d", ie.Value)
	}
}

// TestExpandEnumDefinitionWithoutTaggedValues verifies expand enum definition without tagged values.
func TestExpandEnumDefinitionWithoutTaggedValues(t *testing.T) {
	gen := NewDirectiveGenerator("")
	source := &EnumDefinition{
		Members: []*EnumMember{
			{Name: "red"},
			{Name: "green"},
		},
	}
	result := gen.expandEnumDefinition(source)
	if len(result.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(result.Members))
	}
	if result.Members[0].Name != "red" {
		t.Errorf("expected 'red', got %q", result.Members[0].Name)
	}
	if result.Members[1].Value != nil {
		t.Error("expected nil Value for second member")
	}
}

// TestExpandEnumDefinitionWithTaggedValues verifies expand enum definition with tagged values.
func TestExpandEnumDefinitionWithTaggedValues(t *testing.T) {
	gen := NewDirectiveGenerator("")
	source := &EnumDefinition{
		ValueType: PrimString,
		Members: []*EnumMember{
			{Name: "ok", Value: &StringExpression{Multiline: "good"}},
		},
	}
	result := gen.expandEnumDefinition(source)
	if result.ValueType != PrimString {
		t.Errorf("expected PrimString value type, got %T", result.ValueType)
	}
	if result.Members[0].Value == nil {
		t.Fatal("expected non-nil Value for tagged member")
	}
}

// TestExpandAccessesFieldAccess verifies expand accesses field access.
func TestExpandAccessesFieldAccess(t *testing.T) {
	gen := NewDirectiveGenerator("")
	accesses := []Access{&FieldAccess{Name: "x"}}
	result := gen.expandAccesses(accesses)
	if len(result) != 1 {
		t.Fatalf("expected 1 access, got %d", len(result))
	}
	fa, ok := result[0].(*FieldAccess)
	if !ok {
		t.Fatalf("expected *FieldAccess, got %T", result[0])
	}
	if fa.Name != "x" {
		t.Errorf("expected 'x', got %q", fa.Name)
	}
}

// TestExpandAccessesIndexAccess verifies expand accesses index access.
func TestExpandAccessesIndexAccess(t *testing.T) {
	gen := NewDirectiveGenerator("")
	accesses := []Access{&IndexAccess{Index: &IntegerExpression{Value: 0}}}
	result := gen.expandAccesses(accesses)
	if len(result) != 1 {
		t.Fatalf("expected 1 access, got %d", len(result))
	}
	ia, ok := result[0].(*IndexAccess)
	if !ok {
		t.Fatalf("expected *IndexAccess, got %T", result[0])
	}
	ie, ok := ia.Index.(*IntegerExpression)
	if !ok {
		t.Fatalf("expected *IntegerExpression index, got %T", ia.Index)
	}
	if ie.Value != 0 {
		t.Errorf("expected 0, got %d", ie.Value)
	}
}

// TestRuntimeValueAssignableString verifies runtime value assignable string.
func TestRuntimeValueAssignableString(t *testing.T) {
	gen := NewDirectiveGenerator("")
	if !gen.runtimeValueAssignable(PrimString, "hello") {
		t.Error("expected assignable: string -> PrimString")
	}
	if gen.runtimeValueAssignable(PrimString, 42) {
		t.Error("expected not assignable: int -> PrimString")
	}
	if gen.runtimeValueAssignable(PrimString, true) {
		t.Error("expected not assignable: bool -> PrimString")
	}
}

// TestRuntimeValueAssignableInteger verifies runtime value assignable integer.
func TestRuntimeValueAssignableInteger(t *testing.T) {
	gen := NewDirectiveGenerator("")
	if !gen.runtimeValueAssignable(PrimInteger, 42) {
		t.Error("expected assignable: int -> PrimInteger")
	}
	if gen.runtimeValueAssignable(PrimInteger, "hello") {
		t.Error("expected not assignable: string -> PrimInteger")
	}
}

// TestRuntimeValueAssignableBoolean verifies runtime value assignable boolean.
func TestRuntimeValueAssignableBoolean(t *testing.T) {
	gen := NewDirectiveGenerator("")
	if !gen.runtimeValueAssignable(PrimBoolean, true) {
		t.Error("expected assignable: bool -> PrimBoolean")
	}
	if gen.runtimeValueAssignable(PrimBoolean, 42) {
		t.Error("expected not assignable: int -> PrimBoolean")
	}
}

// TestRuntimeValueAssignableFloat verifies runtime value assignable float.
func TestRuntimeValueAssignableFloat(t *testing.T) {
	gen := NewDirectiveGenerator("")
	if !gen.runtimeValueAssignable(PrimFloat, 3.14) {
		t.Error("expected assignable: float64 -> PrimFloat")
	}
	if !gen.runtimeValueAssignable(PrimFloat, 42) {
		t.Error("expected assignable: int -> PrimFloat (int promotes to float)")
	}
	if gen.runtimeValueAssignable(PrimFloat, "hello") {
		t.Error("expected not assignable: string -> PrimFloat")
	}
}

// TestExpandLValueIdentifier verifies expand l value identifier.
func TestExpandLValueIdentifier(t *testing.T) {
	gen := NewDirectiveGenerator("")
	loc := &SourceLocation{Line: 1, Col: 1}
	result := gen.expandLValue(&IdentifierLValue{Name: "x"}, loc)
	ilv, ok := result.(*IdentifierLValue)
	if !ok {
		t.Fatalf("expected *IdentifierLValue, got %T", result)
	}
	if ilv.Name != "x" {
		t.Errorf("expected 'x', got %q", ilv.Name)
	}
}

// TestExpandLValueIteration verifies expand l value iteration.
func TestExpandLValueIteration(t *testing.T) {
	gen := NewDirectiveGenerator("")
	loc := &SourceLocation{Line: 1, Col: 1}
	result := gen.expandLValue(&IterationLValue{}, loc)
	ilv, ok := result.(*IdentifierLValue)
	if !ok {
		t.Fatalf("expected *IdentifierLValue, got %T", result)
	}
	if ilv.Name != "iteration_0000000000" {
		t.Errorf("expected 'iteration_0000000000', got %q", ilv.Name)
	}
}

// TestExpandLValueIterationWithLabel verifies expand l value iteration with label.
func TestExpandLValueIterationWithLabel(t *testing.T) {
	gen := NewDirectiveGenerator("")
	loc := &SourceLocation{Line: 1, Col: 1}
	result := gen.expandLValue(&IterationLValue{Label: &StringExpression{Multiline: "tag"}}, loc)
	ilv, ok := result.(*IdentifierLValue)
	if !ok {
		t.Fatalf("expected *IdentifierLValue, got %T", result)
	}
	expected := "iteration_0000000000tag"
	if ilv.Name != expected {
		t.Errorf("expected %q, got %q", expected, ilv.Name)
	}
}

// TestDirectiveGeneratorErrPanic verifies directive generator err panic.
func TestDirectiveGeneratorErrPanic(t *testing.T) {
	gen := NewDirectiveGenerator("test.something")
	assertPanic(t, func() {
		gen.err("boom", nil, "try harder")
	}, "boom")
}

// TestMacroEnvironmentLookupWithParent verifies macro environment lookup with parent.
func TestMacroEnvironmentLookupWithParent(t *testing.T) {
	root := newMacroEnvironment(nil)
	macro := &MacroDirective{Name: "my_macro"}
	root.definitions["my_macro"] = macro
	child := newMacroEnvironment(root)
	// Lookup from child should find parent's macro
	found, ok := child.lookup("my_macro")
	if !ok {
		t.Fatal("expected to find macro in parent")
	}
	if found != macro {
		t.Error("expected the same macro pointer")
	}
	// Child can shadow
	shadow := &MacroDirective{Name: "my_macro"}
	child.definitions["my_macro"] = shadow
	found2, _ := child.lookup("my_macro")
	if found2 != shadow {
		t.Error("expected shadowed macro from child")
	}
}

// TestIncludeCycleError verifies include cycle error.
func TestIncludeCycleError(t *testing.T) {
	gen := NewDirectiveGenerator("")
	// includeStack must contain the path for includeCycleError to work
	gen.includeStack = []string{"/cycle/path"}
	assertPanic(t, func() {
		gen.includeCycleError("/cycle/path", nil)
	}, "Circular include dependency")
}

// TestExpandAccessesEmpty verifies expand accesses empty.
func TestExpandAccessesEmpty(t *testing.T) {
	gen := NewDirectiveGenerator("")
	result := gen.expandAccesses(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// TestRuntimeValueAssignableNilType verifies runtime value assignable nil type.
func TestRuntimeValueAssignableNilType(t *testing.T) {
	gen := NewDirectiveGenerator("")
	// Nil expected type returns false
	if gen.runtimeValueAssignable(nil, "hello") {
		t.Error("expected false for nil expected type")
	}
}

// TestExpandAccessesPreservesLocation verifies expand accesses preserves location.
func TestExpandAccessesPreservesLocation(t *testing.T) {
	gen := NewDirectiveGenerator("")
	loc := &SourceLocation{Line: 5, Col: 10}
	accesses := []Access{&FieldAccess{Name: "z", Location: loc}}
	result := gen.expandAccesses(accesses)
	fa, ok := result[0].(*FieldAccess)
	if !ok {
		t.Fatalf("expected *FieldAccess, got %T", result[0])
	}
	if fa.Location != loc {
		t.Error("expected location to be preserved")
	}
}

// TestExpandEnumDefinitionCopySemantics verifies enum expansion does not share member pointers with its input.
func TestExpandEnumDefinitionCopySemantics(t *testing.T) {
	gen := NewDirectiveGenerator("")
	origMember := &EnumMember{Name: "original"}
	source := &EnumDefinition{
		Members: []*EnumMember{origMember},
	}
	result := gen.expandEnumDefinition(source)
	// Modify original slice element pointer — should not affect result
	source.Members[0].Name = "modified"
	if result.Members[0].Name == "modified" {
		t.Error("expected deep copy, but modification to source affected result")
	}
	// Verify member is a different pointer
	if result.Members[0] == origMember {
		t.Error("expected different pointer for copied member")
	}
}

// TestExpandSetupDefinitionCopySemantics verifies setup expansion copies field definitions.
func TestExpandSetupDefinitionCopySemantics(t *testing.T) {
	gen := NewDirectiveGenerator("")
	origField := &FieldDefinition{Name: "original"}
	source := &SetupDefinition{
		Fields: []*FieldDefinition{origField},
	}
	result := gen.expandSetupDefinition(source)
	source.Fields[0].Name = "modified"
	if result.Fields[0].Name == "modified" {
		t.Error("expected deep copy, but modification to source affected result")
	}
}

// TestPrimitiveKindIdentity verifies primitive kinds compare by value.
func TestPrimitiveKindIdentity(t *testing.T) {
	if PrimString != PrimString {
		t.Error("PrimString should equal itself")
	}
	if PrimString == PrimInteger {
		t.Error("PrimString should not equal PrimInteger")
	}
}

// TestPrimitiveKindTypeAssertion verifies primitive kinds retain their concrete TypeRef identity.
func TestPrimitiveKindTypeAssertion(t *testing.T) {
	var kind TypeRef = PrimString
	_, ok := kind.(PrimitiveKind)
	if !ok {
		t.Error("expected PrimString to be a PrimitiveKind")
	}
	_, ok = kind.(*EnumType)
	if ok {
		t.Error("expected PrimString NOT to be *EnumType")
	}
}

// TestLookupConstantReturnsFalseOnEmptyStack verifies lookup constant returns false on empty stack.
func TestLookupConstantReturnsFalseOnEmptyStack(t *testing.T) {
	gen := NewDirectiveGenerator("")
	constantValues := gen.constantValues
	if constantValues == nil {
		// initial nil is fine; lookup should handle it
	}
	_, ok := gen.lookupConstant("anything")
	if ok {
		t.Error("expected false when constantValues is nil/empty")
	}
}

// TestLookupConstantScansTopToBottom verifies lookup constant scans top to bottom.
func TestLookupConstantScansTopToBottom(t *testing.T) {
	gen := NewDirectiveGenerator("")
	gen.pushConstants(map[string]any{"a": 1})
	gen.pushConstants(map[string]any{"a": 2, "b": 3})
	v, ok := gen.lookupConstant("a")
	if !ok || v != 2 {
		t.Errorf("expected top-most 'a' = 2, got %v", v)
	}
	v, ok = gen.lookupConstant("b")
	if !ok || v != 3 {
		t.Errorf("expected 'b' = 3, got %v", v)
	}
}
