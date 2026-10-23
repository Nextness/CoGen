// directive_generator_unit_test.go verifies expansion outputs, source ownership, and lexical state through Expand.
package something_test

import (
	"reflect"
	"testing"

	subject "analysis/backend/something"
	testhelpers "analysis/tests/support/go/backend/something"
)

// expandSource expands one in-memory source through the public directive phase.
func expandSource(source string) *subject.Program {
	program := subject.NewParser(subject.NewLexer(source, "expansion.something").Tokenize(), "expansion.something").ParseProgram()
	return subject.NewDirectiveGenerator("expansion.something").Expand(program)
}

// TestIterationExpansionPreservesIndependentCountersAndPeeks verifies concrete lvalues and non-advancing rvalues for default and labeled iterations.
func TestIterationExpansionPreservesIndependentCountersAndPeeks(t *testing.T) {
	expanded := expandSource(`before:=#iteration;#iteration: string="first";
 before_label:=#iteration("_a");#iteration("_a"):=1;
 after:=#iteration;again:=#iteration;#iteration:=2;
 after_label:=#iteration("_a");#iteration("_a"):=3;#iteration("tag"):=4;`)
	checked := subject.NewTypeChecker(expanded, "").Check()
	values := subject.NewEvaluator(checked, "").Evaluate()
	for name, want := range map[string]any{
		"before": "iteration_0000000000", "iteration_0000000000": "first",
		"before_label": "iteration_0000000000_a", "iteration_0000000000_a": 1,
		"after": "iteration_0000000001", "again": "iteration_0000000001", "iteration_0000000001": 2,
		"after_label": "iteration_0000000001_a", "iteration_0000000001_a": 3, "iteration_0000000000tag": 4,
	} {
		if values[name] != want {
			t.Errorf("%s=%v, want %v", name, values[name], want)
		}
	}
}

// TestExpansionCopiesDefinitionsAndLocations verifies public expansion copies nested setup and enum declarations without mutating syntax.
func TestExpansionCopiesDefinitionsAndLocations(t *testing.T) {
	loc := &subject.SourceLocation{Filepath: "definition.something", Line: 3, Col: 7}
	setup := &subject.SetupDefinition{Fields: []*subject.FieldDefinition{
		{Name: "x", DeclaredType: subject.PrimString},
		{Name: "y", DeclaredType: subject.PrimInteger, DefaultValue: &subject.IntegerExpression{Value: 42, Location: loc}},
	}}
	enum := &subject.EnumDefinition{ValueType: subject.PrimString, Members: []*subject.EnumMember{{Name: "ok", Value: &subject.StringExpression{Multiline: "good", Location: loc}}}}
	plain := &subject.EnumDefinition{Members: []*subject.EnumMember{{Name: "red"}, {Name: "green"}}}
	program := &subject.Program{Statements: []subject.Statement{
		&subject.Assignment{StatementBase: subject.StatementBase{Location: loc}, Target: &subject.IdentifierLValue{Name: "Config"}, Value: setup},
		&subject.Assignment{StatementBase: subject.StatementBase{Location: loc}, Target: &subject.IdentifierLValue{Name: "Status"}, Value: enum},
		&subject.Assignment{StatementBase: subject.StatementBase{Location: loc}, Target: &subject.IdentifierLValue{Name: "Color"}, Value: plain},
	}}
	expanded := subject.NewDirectiveGenerator("").Expand(program)
	copiedSetup := expanded.Statements[0].(*subject.Assignment).Value.(*subject.SetupDefinition)
	copiedEnum := expanded.Statements[1].(*subject.Assignment).Value.(*subject.EnumDefinition)
	copiedPlain := expanded.Statements[2].(*subject.Assignment).Value.(*subject.EnumDefinition)
	if copiedSetup == setup || copiedSetup.Fields[0] == setup.Fields[0] || copiedEnum == enum || copiedEnum.Members[0] == enum.Members[0] {
		t.Fatal("expanded declarations alias syntax")
	}
	if len(copiedSetup.Fields) != 2 || copiedSetup.Fields[0].DefaultValue != nil {
		t.Fatalf("setup fields=%v", copiedSetup.Fields)
	}
	integer := copiedSetup.Fields[1].DefaultValue.(*subject.IntegerExpression)
	if integer.Value != 42 || integer.Location != loc || copiedEnum.ValueType != subject.PrimString || copiedEnum.Members[0].Name != "ok" {
		t.Fatal("expanded values or source location changed")
	}
	if len(copiedPlain.Members) != 2 || copiedPlain.Members[0].Name != "red" || copiedPlain.Members[1].Name != "green" || copiedPlain.Members[1].Value != nil {
		t.Fatalf("plain enum=%v", copiedPlain)
	}
	setup.Fields[0].Name = "changed"
	enum.Members[0].Name = "changed"
	if copiedSetup.Fields[0].Name != "x" || copiedEnum.Members[0].Name != "ok" {
		t.Fatal("syntax mutation changed expansion")
	}
}

// TestExpansionPreservesReferenceAccesses verifies field and index access nodes and locations survive the public phase.
func TestExpansionPreservesReferenceAccesses(t *testing.T) {
	expanded := expandSource(`s:scope={items:=[]string{"a","b"};}value:=s.items[0];`)
	assignment := expanded.Statements[1].(*subject.Assignment)
	ref := assignment.Value.(*subject.ReferenceExpression)
	if ref.Root != "s" || len(ref.Accesses) != 2 {
		t.Fatalf("reference=%+v", ref)
	}
	field := ref.Accesses[0].(*subject.FieldAccess)
	index := ref.Accesses[1].(*subject.IndexAccess)
	if field.Name != "items" || index.Index.(*subject.IntegerExpression).Value != 0 || field.Location == nil || index.Location == nil {
		t.Fatalf("accesses=%+v", ref.Accesses)
	}
}

// TestLoopConstantsAndMacroEnvironmentsRespectLexicalScope verifies parent lookup, shadowing, stack restoration, and materialized values.
func TestLoopConstantsAndMacroEnvironmentsRespectLexicalScope(t *testing.T) {
	values := testhelpers.EvalText(t, `
#macro value := () -> string { #set "root"; }
outside:=value!();
child:scope={
 #macro value := () -> string { #set "child"; }
 inner:=value!();
}
after:=value!();
#for item: []integer{1,2} {
 #for nested: []string{"two"} { #iteration("_inner") := nested; }
 #iteration("_outer") := item;
}
#for value: []float{3.14} { #iteration("_float") := value; }
#for value: []boolean{true} { #iteration("_bool") := value; }
`)
	if values["outside"] != "root" || values["after"] != "root" || values["child"].(map[string]any)["inner"] != "child" {
		t.Fatalf("macro scope=%v", values)
	}
	for name, want := range map[string]any{"iteration_0000000000_outer": 1, "iteration_0000000001_outer": 2, "iteration_0000000000_inner": "two", "iteration_0000000001_inner": "two", "iteration_0000000000_float": 3.14, "iteration_0000000000_bool": true} {
		if !reflect.DeepEqual(values[name], want) {
			t.Errorf("%s=%v, want %v", name, values[name], want)
		}
	}
	if _, exists := values["item"]; exists {
		t.Fatal("loop constant escaped")
	}
	testhelpers.AssertPanic(t, func() { expandSource(`value:=nonexistent!();`) }, "Undefined macro")
}

// TestExpansionReportsLocatedErrors verifies diagnostics retain the generator's source path and position.
func TestExpansionReportsLocatedErrors(t *testing.T) {
	defer func() {
		value := recover()
		problem, ok := value.(*subject.SomethingError)
		if !ok || problem.Filepath != "expansion.something" || problem.Line == 0 || problem.Col == 0 || problem.Suggestion == "" {
			t.Fatalf("diagnostic=%v", value)
		}
	}()
	expandSource(`value:=nonexistent!();`)
	t.Fatal("undefined macro accepted")
}
