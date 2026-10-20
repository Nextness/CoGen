// typechecker_unit_test.go contains unit tests for typechecker.go that
// exercise pure type-checking functions (assignable, enumHasMember,
// typeDependencies, expressionDependencies, referencePath, etc.) directly
// without going through the full SOMETHING pipeline.

package something_test

import subject "analysis/backend/something"
import testhelpers "analysis/tests/support/go/backend/something"

import (
	"reflect"
	"sort"
	"testing"
)

// newTestTypeChecker supports the package test suite's new test type checker setup or assertions.
func newTestTypeChecker() *subject.TypeChecker {
	return subject.NewTypeChecker(&subject.Program{}, "")
}

// TestAssignablePrimitiveMatching verifies assignable primitive matching.
func TestAssignablePrimitiveMatching(t *testing.T) {
	tc := newTestTypeChecker()
	cases := []struct {
		expected, actual subject.TypeRef
	}{
		{subject.PrimString, subject.PrimString},
		{subject.PrimInteger, subject.PrimInteger},
		{subject.PrimBoolean, subject.PrimBoolean},
		{subject.PrimFloat, subject.PrimFloat},
		{subject.PrimTimestamp, subject.PrimTimestamp},
		{subject.PrimScope, subject.PrimScope},
		{subject.PrimNamespace, subject.PrimNamespace},
	}
	for _, c := range cases {
		if !tc.assignable(c.expected, c.actual) {
			t.Errorf("expected assignable(%v, %v) = true", c.expected, c.actual)
		}
	}
}

// TestAssignablePrimitiveMismatched verifies assignable primitive mismatched.
func TestAssignablePrimitiveMismatched(t *testing.T) {
	tc := newTestTypeChecker()
	cases := []struct {
		expected, actual subject.TypeRef
	}{
		{subject.PrimString, subject.PrimInteger},
		{subject.PrimString, subject.PrimBoolean},
		{subject.PrimInteger, subject.PrimString},
		{subject.PrimInteger, subject.PrimBoolean},
		{subject.PrimBoolean, subject.PrimInteger},
		{subject.PrimBoolean, subject.PrimString},
		{subject.PrimFloat, subject.PrimBoolean},
		{subject.PrimString, subject.PrimFloat},
	}
	for _, c := range cases {
		if tc.assignable(c.expected, c.actual) {
			t.Errorf("expected assignable(%v, %v) = false", c.expected, c.actual)
		}
	}
}

// TestAssignableFloatFromInt verifies assignable float from int.
func TestAssignableFloatFromInt(t *testing.T) {
	tc := newTestTypeChecker()
	// Green path: float can receive int
	if !tc.assignable(subject.PrimFloat, subject.PrimInteger) {
		t.Error("expected assignable(PrimFloat, PrimInteger) = true")
	}
	// Red path: int cannot receive float
	if tc.assignable(subject.PrimInteger, subject.PrimFloat) {
		t.Error("expected assignable(PrimInteger, PrimFloat) = false")
	}
}

// TestAssignableTimestampFromString verifies assignable timestamp from string.
func TestAssignableTimestampFromString(t *testing.T) {
	tc := newTestTypeChecker()
	// Green path: timestamp can receive string
	if !tc.assignable(subject.PrimTimestamp, subject.PrimString) {
		t.Error("expected assignable(PrimTimestamp, PrimString) = true")
	}
	// Red path: string cannot receive timestamp
	if tc.assignable(subject.PrimString, subject.PrimTimestamp) {
		t.Error("expected assignable(PrimString, PrimTimestamp) = false")
	}
}

// TestAssignableScopeTypes verifies assignable scope types.
func TestAssignableScopeTypes(t *testing.T) {
	tc := newTestTypeChecker()
	// Green path: scope can receive ScopeType
	scopeType := &subject.ScopeType{Fields: map[string]*subject.BindingType{}}
	if !tc.assignable(subject.PrimScope, scopeType) {
		t.Error("expected assignable(PrimScope, *ScopeType) = true")
	}
	// Red path: scope cannot receive other types
	if tc.assignable(subject.PrimScope, subject.PrimString) {
		t.Error("expected assignable(PrimScope, PrimString) = false")
	}
	if tc.assignable(subject.PrimScope, subject.PrimInteger) {
		t.Error("expected assignable(PrimScope, PrimInteger) = false")
	}
}

// TestAssignableNamespaceTypes verifies assignable namespace types.
func TestAssignableNamespaceTypes(t *testing.T) {
	tc := newTestTypeChecker()
	nsType := &subject.NamespaceType{Fields: map[string]*subject.BindingType{}}
	if !tc.assignable(subject.PrimNamespace, nsType) {
		t.Error("expected assignable(PrimNamespace, *NamespaceType) = true")
	}
	if tc.assignable(subject.PrimNamespace, subject.PrimString) {
		t.Error("expected assignable(PrimNamespace, PrimString) = false")
	}
}

// TestAssignableEnumTypes verifies assignable enum types.
func TestAssignableEnumTypes(t *testing.T) {
	tc := newTestTypeChecker()
	enumA := &subject.EnumType{Name: "Color", Members: map[string]subject.Expression{"red": nil}, MemberList: []string{"red"}}
	enumB := &subject.EnumType{Name: "Status", Members: map[string]subject.Expression{"ok": nil}, MemberList: []string{"ok"}}
	// Same name
	if !tc.assignable(enumA, enumA) {
		t.Error("expected assignable(same enum) = true")
	}
	if !tc.assignable(enumA, &subject.EnumType{Name: "Color"}) {
		t.Error("expected assignable(enum with same name) = true")
	}
	// Different name
	if tc.assignable(enumA, enumB) {
		t.Error("expected assignable(different enum) = false")
	}
}

// TestAssignableSetupTypes verifies assignable setup types.
func TestAssignableSetupTypes(t *testing.T) {
	tc := newTestTypeChecker()
	setupA := &subject.SetupType{Name: "Point"}
	setupB := &subject.SetupType{Name: "Rect"}
	if !tc.assignable(setupA, setupA) {
		t.Error("expected assignable(same setup) = true")
	}
	if !tc.assignable(setupA, &subject.SetupType{Name: "Point"}) {
		t.Error("expected assignable(setup with same name) = true")
	}
	if tc.assignable(setupA, setupB) {
		t.Error("expected assignable(different setup) = false")
	}
}

// TestAssignableNilTypes verifies assignable nil types.
func TestAssignableNilTypes(t *testing.T) {
	tc := newTestTypeChecker()
	// Nil expected
	if tc.assignable(nil, subject.PrimString) {
		t.Error("expected assignable(nil, string) = false")
	}
}

// TestAssignableArrayTypes verifies assignable array types.
func TestAssignableArrayTypes(t *testing.T) {
	tc := newTestTypeChecker()
	strArr := &subject.ArrayType{ElementType: subject.PrimString}
	intArr := &subject.ArrayType{ElementType: subject.PrimInteger}
	// Same element type
	if !tc.assignable(strArr, &subject.ArrayType{ElementType: subject.PrimString}) {
		t.Error("expected assignable([]string, []string) = true")
	}
	// Different element type
	if tc.assignable(strArr, intArr) {
		t.Error("expected assignable([]string, []integer) = false")
	}
	// Non-array
	if tc.assignable(strArr, subject.PrimString) {
		t.Error("expected assignable([]string, string) = false")
	}
}

// TestAssignableMappingTypes verifies assignable mapping types.
func TestAssignableMappingTypes(t *testing.T) {
	tc := newTestTypeChecker()
	m1 := &subject.MappingType{KeyType: subject.PrimString, ValueType: subject.PrimInteger}
	m2 := &subject.MappingType{KeyType: subject.PrimString, ValueType: subject.PrimString}
	m3 := &subject.MappingType{KeyType: subject.PrimInteger, ValueType: subject.PrimInteger}
	// Same types
	if !tc.assignable(m1, &subject.MappingType{KeyType: subject.PrimString, ValueType: subject.PrimInteger}) {
		t.Error("expected assignable(mapping(string,int), mapping(string,int)) = true")
	}
	// Different value type
	if tc.assignable(m1, m2) {
		t.Error("expected assignable(mapping(string,int), mapping(string,string)) = false")
	}
	// Different key type
	if tc.assignable(m1, m3) {
		t.Error("expected assignable(mapping(string,int), mapping(int,int)) = false")
	}
}

// TestAssignableEnumKeyTypes verifies assignable enum key types.
func TestAssignableEnumKeyTypes(t *testing.T) {
	tc := newTestTypeChecker()
	// Register the Color enum so resolveType can find it
	tc.current.types["Color"] = &subject.EnumType{
		Name:       "Color",
		MemberList: []string{"red", "green", "blue"},
	}
	ek1 := &subject.EnumKeyType{EnumName: "Color", ElementType: subject.PrimInteger}
	ek2 := &subject.EnumKeyType{EnumName: "Color", ElementType: subject.PrimString}
	ek3 := &subject.EnumKeyType{EnumName: "Status", ElementType: subject.PrimInteger}
	// Same
	if !tc.assignable(ek1, &subject.EnumKeyType{EnumName: "Color", ElementType: subject.PrimInteger}) {
		t.Error("expected assignable(same enumkey) = true")
	}
	// Different element type — ElementType is resolved recursively so same-kind
	// comparison applies after resolution; PrimInteger != PrimString
	if tc.assignable(ek1, ek2) {
		t.Error("expected assignable(different element) = false")
	}
	// Different enum name — Status not registered, resolveType panics
	testhelpers.AssertPanic(t, func() {
		tc.assignable(ek1, ek3)
	}, "Unknown enum index type")
}

// TestAssignableScopeStructurally verifies assignable scope structurally.
func TestAssignableScopeStructurally(t *testing.T) {
	tc := newTestTypeChecker()
	fields := map[string]*subject.BindingType{
		"a": {Type: subject.PrimString},
	}
	scopeA := &subject.ScopeType{Fields: fields}
	scopeB := &subject.ScopeType{Fields: map[string]*subject.BindingType{"a": {Type: subject.PrimString}}}
	scopeC := &subject.ScopeType{Fields: map[string]*subject.BindingType{"a": {Type: subject.PrimInteger}}}
	scopeD := &subject.ScopeType{Fields: map[string]*subject.BindingType{}}
	// Structurally equal
	if !tc.assignable(scopeA, scopeB) {
		t.Error("expected assignable(structurally equal scopes) = true")
	}
	// Different field type
	if tc.assignable(scopeA, scopeC) {
		t.Error("expected assignable(different field type) = false")
	}
	// Missing field
	if tc.assignable(scopeA, scopeD) {
		t.Error("expected assignable(missing field) = false")
	}
}

// TestStructurallyAssignable verifies structurally assignable.
func TestStructurallyAssignable(t *testing.T) {
	tc := newTestTypeChecker()
	fieldsA := map[string]*subject.BindingType{
		"x": {Type: subject.PrimString},
		"y": {Type: subject.PrimInteger},
	}
	fieldsB := map[string]*subject.BindingType{
		"x": {Type: subject.PrimString},
		"y": {Type: subject.PrimInteger},
	}
	fieldsC := map[string]*subject.BindingType{
		"x": {Type: subject.PrimString},
	}
	if !tc.structurallyAssignable(fieldsA, fieldsB) {
		t.Error("expected structurallyAssignable(matching) = true")
	}
	if tc.structurallyAssignable(fieldsA, fieldsC) {
		t.Error("expected structurallyAssignable(missing field) = false")
	}
	// nil/nil: len(nil)==0 for both, so the function proceeds past the
	// len check and the range loops are no-ops, returning true.
	if !tc.structurallyAssignable(nil, nil) {
		t.Error("expected structurallyAssignable(nil, nil) = true (both empty)")
	}
}

// TestEnumHasMemberFound verifies enum has member found.
func TestEnumHasMemberFound(t *testing.T) {
	enum := &subject.EnumType{
		Name:       "Color",
		MemberList: []string{"red", "green", "blue"},
	}
	if !subject.enumHasMember(enum, "red") {
		t.Error("expected true for 'red'")
	}
	if !subject.enumHasMember(enum, "green") {
		t.Error("expected true for 'green'")
	}
	if !subject.enumHasMember(enum, "blue") {
		t.Error("expected true for 'blue'")
	}
}

// TestEnumHasMemberNotFound verifies enum has member not found.
func TestEnumHasMemberNotFound(t *testing.T) {
	enum := &subject.EnumType{
		Name:       "Color",
		MemberList: []string{"red", "green"},
	}
	if subject.enumHasMember(enum, "yellow") {
		t.Error("expected false for 'yellow'")
	}
	if subject.enumHasMember(enum, "") {
		t.Error("expected false for empty string")
	}
}

// TestEnumHasMemberEmptyEnum verifies enum has member empty enum.
func TestEnumHasMemberEmptyEnum(t *testing.T) {
	enum := &subject.EnumType{Name: "Empty", MemberList: []string{}}
	if subject.enumHasMember(enum, "anything") {
		t.Error("expected false for empty enum")
	}
}

// TestSortedKeys verifies deterministic string-keyed map ordering.
func TestSortedKeys(t *testing.T) {
	fields := map[string]*subject.BindingType{
		"z": {Type: subject.PrimString},
		"a": {Type: subject.PrimInteger},
		"m": {Type: subject.PrimFloat},
	}
	result := subject.sortedKeys(fields)
	expected := []string{"a", "m", "z"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
	if !sort.StringsAreSorted(result) {
		t.Error("expected sorted result")
	}
}

// TestSortedKeysEmpty verifies sorting an empty map.
func TestSortedKeysEmpty(t *testing.T) {
	result := subject.sortedKeys(map[string]*subject.BindingType{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

// TestSortedKeysNil verifies sorting a nil map.
func TestSortedKeysNil(t *testing.T) {
	result := subject.sortedKeys(map[string]*subject.BindingType(nil))
	if len(result) != 0 {
		t.Errorf("expected empty slice for nil map, got %v", result)
	}
}

// TestTypeDependenciesTypeName verifies type dependencies type name.
func TestTypeDependenciesTypeName(t *testing.T) {
	deps := subject.typeDependencies(subject.TypeName("MySetup"))
	expected := []string{"MySetup"}
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("expected %v, got %v", expected, deps)
	}
}

// TestTypeDependenciesArray verifies type dependencies array.
func TestTypeDependenciesArray(t *testing.T) {
	deps := subject.typeDependencies(&subject.ArrayType{ElementType: subject.TypeName("Inner")})
	expected := []string{"Inner"}
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("expected %v, got %v", expected, deps)
	}
}

// TestTypeDependenciesMapping verifies type dependencies mapping.
func TestTypeDependenciesMapping(t *testing.T) {
	deps := subject.typeDependencies(&subject.MappingType{
		KeyType:   subject.TypeName("KeyType"),
		ValueType: subject.TypeName("ValueType"),
	})
	expected := []string{"KeyType", "ValueType"}
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("expected %v, got %v", expected, deps)
	}
}

// TestTypeDependenciesEnumKey verifies type dependencies enum key.
func TestTypeDependenciesEnumKey(t *testing.T) {
	deps := subject.typeDependencies(&subject.EnumKeyType{
		EnumName:    "Color",
		ElementType: subject.TypeName("Inner"),
	})
	if len(deps) < 2 {
		t.Fatalf("expected at least 2 dependencies, got %v", deps)
	}
	if deps[0] != "Color" {
		t.Errorf("expected first dep 'Color', got %q", deps[0])
	}
}

// TestTypeDependenciesPrimitive verifies type dependencies primitive.
func TestTypeDependenciesPrimitive(t *testing.T) {
	// Primitives have no type dependencies
	deps := subject.typeDependencies(subject.PrimString)
	if deps != nil {
		t.Errorf("expected nil for primitive, got %v", deps)
	}
	deps = subject.typeDependencies(subject.PrimInteger)
	if deps != nil {
		t.Errorf("expected nil for primitive, got %v", deps)
	}
}

// TestExpressionDependenciesReference verifies expression dependencies reference.
func TestExpressionDependenciesReference(t *testing.T) {
	expr := &subject.ReferenceExpression{Root: "x", Accesses: nil}
	deps := subject.expressionDependencies(expr)
	expected := []string{"x"}
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("expected %v, got %v", expected, deps)
	}
}

// TestExpressionDependenciesArray verifies expression dependencies array.
func TestExpressionDependenciesArray(t *testing.T) {
	expr := &subject.ArrayExpression{
		Elements: []subject.Expression{
			&subject.ReferenceExpression{Root: "a"},
			&subject.ReferenceExpression{Root: "b"},
		},
	}
	deps := subject.expressionDependencies(expr)
	expected := []string{"a", "b"}
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("expected %v, got %v", expected, deps)
	}
}

// TestExpressionDependenciesNoDependencies verifies expression dependencies no dependencies.
func TestExpressionDependenciesNoDependencies(t *testing.T) {
	expr := &subject.IntegerExpression{Value: 42}
	deps := subject.expressionDependencies(expr)
	if len(deps) != 0 {
		t.Errorf("expected no dependencies for literal, got %v", deps)
	}
}

// TestReferencePathSimple verifies reference path simple.
func TestReferencePathSimple(t *testing.T) {
	ref := &subject.ReferenceExpression{Root: "x"}
	path := subject.referencePath(ref)
	if path != "x" {
		t.Errorf("expected 'x', got %q", path)
	}
}

// TestReferencePathWithAccesses verifies reference path with accesses.
func TestReferencePathWithAccesses(t *testing.T) {
	ref := &subject.ReferenceExpression{
		Root: "a",
		Accesses: []subject.Access{
			&subject.FieldAccess{Name: "b"},
			&subject.FieldAccess{Name: "c"},
		},
	}
	path := subject.referencePath(ref)
	if path != "a.b.c" {
		t.Errorf("expected 'a.b.c', got %q", path)
	}
}

// TestReferencePathStopsAtIndex verifies reference path stops at index.
func TestReferencePathStopsAtIndex(t *testing.T) {
	ref := &subject.ReferenceExpression{
		Root: "a",
		Accesses: []subject.Access{
			&subject.FieldAccess{Name: "b"},
			&subject.IndexAccess{},
			&subject.FieldAccess{Name: "d"},
		},
	}
	// Stops at the IndexAccess because referencePath only handles FieldAccess
	path := subject.referencePath(ref)
	if path != "a.b" {
		t.Errorf("expected 'a.b', got %q", path)
	}
}

// TestReferencePathEmptyRoot verifies reference path empty root.
func TestReferencePathEmptyRoot(t *testing.T) {
	ref := &subject.ReferenceExpression{}
	path := subject.referencePath(ref)
	if path != "" {
		t.Errorf("expected empty string, got %q", path)
	}
}

// TestCycleTargetNameIdentifier verifies cycle target name identifier.
func TestCycleTargetNameIdentifier(t *testing.T) {
	target := &subject.IdentifierLValue{Name: "myVar"}
	name := subject.cycleTargetName(target, "cycle_")
	if name != "cycle_myVar" {
		t.Errorf("expected 'cycle_myVar', got %q", name)
	}
}

// TestCycleTargetNameMember verifies cycle target name member.
func TestCycleTargetNameMember(t *testing.T) {
	target := &subject.MemberLValue{
		Root: "s",
		Accesses: []subject.Access{
			&subject.FieldAccess{Name: "x"},
			&subject.FieldAccess{Name: "y"},
		},
	}
	name := subject.cycleTargetName(target, "")
	if name != "s.x.y" {
		t.Errorf("expected 's.x.y', got %q", name)
	}
}

// TestCycleTargetNameMemberWithIndexAccess verifies cycle target name member with index access.
func TestCycleTargetNameMemberWithIndexAccess(t *testing.T) {
	// Index access makes it return ""
	target := &subject.MemberLValue{
		Root: "a",
		Accesses: []subject.Access{
			&subject.FieldAccess{Name: "b"},
			&subject.IndexAccess{},
		},
	}
	name := subject.cycleTargetName(target, "")
	if name != "" {
		t.Errorf("expected empty string for member with index access, got %q", name)
	}
}

// TestCycleTargetNameDefault verifies cycle target name default.
func TestCycleTargetNameDefault(t *testing.T) {
	// An unhandled LValue type returns ""
	name := subject.cycleTargetName(&subject.IterationLValue{}, "")
	if name != "" {
		t.Errorf("expected empty string for IterationLValue, got %q", name)
	}
}

// TestCopyCycleNames verifies copy cycle names.
func TestCopyCycleNames(t *testing.T) {
	source := map[string]string{
		"a": "b",
		"c": "d",
	}
	result := subject.copyCycleNames(source)
	if !reflect.DeepEqual(result, source) {
		t.Errorf("expected %v, got %v", source, result)
	}
	// Verify it's a deep copy
	source["a"] = "modified"
	if result["a"] != "b" {
		t.Error("expected result to be a copy, not a reference")
	}
}

// TestCopyCycleNamesEmpty verifies copy cycle names empty.
func TestCopyCycleNamesEmpty(t *testing.T) {
	result := subject.copyCycleNames(map[string]string{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

// TestCopyCycleNamesNil verifies copy cycle names nil.
func TestCopyCycleNamesNil(t *testing.T) {
	result := subject.copyCycleNames(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil input, got %v", result)
	}
}

// TestConstantStringNoInterpolation verifies constant string no interpolation.
func TestConstantStringNoInterpolation(t *testing.T) {
	tc := newTestTypeChecker()
	expr := &subject.StringExpression{Multiline: "hello"}
	s, ok := tc.constantString(expr)
	if !ok {
		t.Fatal("expected ok = true")
	}
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}

// TestConstantStringLiteralNoInterpolation verifies constant string literal no interpolation.
func TestConstantStringLiteralNoInterpolation(t *testing.T) {
	tc := newTestTypeChecker()
	expr := &subject.StringExpression{
		Literal: &subject.StringLiteral{
			Parts: []subject.StringPart{subject.StringText("hello")},
		},
	}
	s, ok := tc.constantString(expr)
	if !ok {
		t.Fatal("expected ok = true")
	}
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}

// TestConstantStringWithInterpolation verifies constant string with interpolation.
func TestConstantStringWithInterpolation(t *testing.T) {
	tc := newTestTypeChecker()
	expr := &subject.StringExpression{
		Literal: &subject.StringLiteral{
			Parts: []subject.StringPart{
				subject.StringText("hello "),
				&subject.InterpolationRef{Name: "x"},
			},
		},
	}
	_, ok := tc.constantString(expr)
	if ok {
		t.Error("expected ok = false for interpolation reference")
	}
}

// TestConstantStringMultilineWithInterpolation verifies constant string multiline with interpolation.
func TestConstantStringMultilineWithInterpolation(t *testing.T) {
	tc := newTestTypeChecker()
	expr := &subject.StringExpression{Multiline: "hello {x}"}
	_, ok := tc.constantString(expr)
	if ok {
		t.Error("expected ok = false for multiline with interpolation")
	}
}

// TestConstantStringNilLiteralEmptyMultiline verifies constant string nil literal empty multiline.
func TestConstantStringNilLiteralEmptyMultiline(t *testing.T) {
	tc := newTestTypeChecker()
	expr := &subject.StringExpression{}
	s, ok := tc.constantString(expr)
	if !ok {
		t.Fatal("expected ok = true")
	}
	if s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

// TestNewStaticEnvironmentNoParent verifies new static environment no parent.
func TestNewStaticEnvironmentNoParent(t *testing.T) {
	env := subject.newStaticEnvironment(nil)
	if env.parent != nil {
		t.Error("expected nil parent")
	}
	if env.bindings == nil {
		t.Error("expected non-nil bindings")
	}
	if env.types == nil {
		t.Error("expected non-nil types")
	}
}

// TestNewStaticEnvironmentWithParent verifies new static environment with parent.
func TestNewStaticEnvironmentWithParent(t *testing.T) {
	parent := subject.newStaticEnvironment(nil)
	child := subject.newStaticEnvironment(parent)
	if child.parent != parent {
		t.Error("expected parent to be set")
	}
}

// TestAssignableExpectedIsPrimitiveKindSwitch verifies assignable expected is primitive kind switch.
func TestAssignableExpectedIsPrimitiveKindSwitch(t *testing.T) {
	tc := newTestTypeChecker()
	// PrimitiveKind switch: same kind = true
	if !tc.assignable(subject.PrimString, subject.PrimString) {
		t.Error("expected assignable(string, string) = true")
	}
	// Float -> Int is handled separately before the switch
	if tc.assignable(subject.PrimInteger, subject.PrimFloat) {
		t.Error("expected assignable(int, float) = false")
	}
}

// TestAssignableNotAssignable verifies assignable not assignable.
func TestAssignableNotAssignable(t *testing.T) {
	tc := newTestTypeChecker()
	// Different primitives that aren't special-cased
	if tc.assignable(subject.PrimBoolean, subject.PrimInteger) {
		t.Error("expected assignable(bool, int) = false")
	}
}

// TestAssignableBothPrimitiveDifferent verifies assignable both primitive different.
func TestAssignableBothPrimitiveDifferent(t *testing.T) {
	tc := newTestTypeChecker()
	// All different primitive pairs (except the special-cased ones)
	pairs := [][2]subject.TypeRef{
		{subject.PrimString, subject.PrimBoolean},
		{subject.PrimString, subject.PrimFloat},
		{subject.PrimInteger, subject.PrimFloat},
		{subject.PrimInteger, subject.PrimBoolean},
		{subject.PrimBoolean, subject.PrimFloat},
		{subject.PrimTimestamp, subject.PrimInteger},
		{subject.PrimTimestamp, subject.PrimBoolean},
	}
	for _, p := range pairs {
		if tc.assignable(p[0], p[1]) {
			t.Errorf("expected assignable(%v, %v) = false", p[0], p[1])
		}
	}
}

// TestExpressionDependenciesStructExpression verifies expression dependencies struct expression.
func TestExpressionDependenciesStructExpression(t *testing.T) {
	expr := &subject.StructExpression{
		Fields: []*subject.FieldAssignment{
			{Value: &subject.ReferenceExpression{Root: "dep1"}},
			{Value: &subject.ReferenceExpression{Root: "dep2"}},
		},
	}
	deps := subject.expressionDependencies(expr)
	if len(deps) != 2 {
		t.Errorf("expected 2 dependencies, got %v", deps)
	}
}

// TestExpressionDependenciesMappingExpression verifies expression dependencies mapping expression.
func TestExpressionDependenciesMappingExpression(t *testing.T) {
	expr := &subject.MappingExpression{
		Entries: []*subject.MappingEntry{
			{
				Keys:  []subject.Expression{&subject.ReferenceExpression{Root: "k"}},
				Value: &subject.ReferenceExpression{Root: "v"},
			},
		},
	}
	deps := subject.expressionDependencies(expr)
	if len(deps) != 2 {
		t.Errorf("expected 2 dependencies (key + value), got %v", deps)
	}
}

// TestExpressionDependenciesTypedExpression verifies expression dependencies typed expression.
func TestExpressionDependenciesTypedExpression(t *testing.T) {
	expr := &subject.TypedExpression{
		Value: &subject.ReferenceExpression{Root: "inner"},
	}
	deps := subject.expressionDependencies(expr)
	if len(deps) != 1 || deps[0] != "inner" {
		t.Errorf("expected ['inner'], got %v", deps)
	}
}
