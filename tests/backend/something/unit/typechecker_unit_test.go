// typechecker_unit_test.go exercises semantic checking through its public phase boundary.
package something_test

import (
	"fmt"
	"strings"
	"testing"

	subject "analysis/backend/something"
)

// checkSource invokes the actual checker over a parsed program and captures only language diagnostics.
func checkSource(source string) (checked *subject.CheckedProgram, problem *subject.SomethingError) {
	defer func() {
		if value := recover(); value != nil {
			if err, ok := value.(*subject.SomethingError); ok {
				problem = err
			} else {
				panic(value)
			}
		}
	}()
	program := subject.NewParser(subject.NewLexer(source, "fixture.something").Tokenize(), "fixture.something").ParseProgram()
	return subject.NewTypeChecker(program, "fixture.something").Check(), nil
}

// TestCheckedAssignmentsPreserveTypeCompatibility covers primitive, nominal, collection, and structural assignment rules.
func TestCheckedAssignmentsPreserveTypeCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, source string
		valid        bool
	}{
		{"string", `value:string="text";`, true},
		{"integer", `value:integer=1;`, true},
		{"boolean", `value:boolean=true;`, true},
		{"float", `value:float=1.5;`, true},
		{"timestamp", `value:timestamp="2025-01-15 10:30:00";`, true},
		{"scope", `value:scope={x:=1;}`, true},
		{"string integer", `value:string=1;`, false},
		{"string boolean", `value:string=true;`, false},
		{"integer string", `value:integer="text";`, false},
		{"integer boolean", `value:integer=true;`, false},
		{"boolean integer", `value:boolean=1;`, false},
		{"boolean string", `value:boolean="text";`, false},
		{"float boolean", `value:float=true;`, false},
		{"string float", `value:string=1.5;`, false},
		{"integer float", `value:integer=1.5;`, false},
		{"boolean float", `value:boolean=1.5;`, false},
		{"timestamp integer", `value:timestamp=1;`, false},
		{"timestamp boolean", `value:timestamp=true;`, false},
		{"float promotion", `value:float=1;`, true},
		{"timestamp from string", `text:string="2025-01-15 10:30:00";value:timestamp=text;`, true},
		{"string from timestamp", `stamp:timestamp="2025-01-15 10:30:00";value:string=stamp;`, false},
		{"scope from string", `value:scope="text";`, false},
		{"scope from integer", `value:scope=1;`, false},
		{"namespace from string", `value:namespace="text";`, false},
		{"same enum", `Color:enum={red;}a:Color=.red;b:Color=a;`, true},
		{"different enum", `Color:enum={red;}Status:enum={ok;}a:Status=.ok;b:Color=a;`, false},
		{"enum scalar", `Color:enum={red;}b:Color=1;`, false},
		{"same setup", `Point:setup={x:integer;}a:Point={x=1};b:Point=a;`, true},
		{"different setup", `Point:setup={x:integer;}Rect:setup={x:integer;}a:Point={x=1};b:Rect=a;`, false},
		{"setup scalar", `Point:setup={x:integer;}a:Point=1;`, false},
		{"same array", `a:[]string=[]string{"a"};b:[]string=a;`, true},
		{"different array", `a:[]integer=[]integer{1};b:[]string=a;`, false},
		{"array scalar", `a:[]string="a";`, false},
		{"same mapping", `a:mapping(string,integer)=mapping(string,integer){["x"]=>1};b:mapping(string,integer)=a;`, true},
		{"different mapping values", `a:mapping(string,string)=mapping(string,string){["x"]=>"a"};b:mapping(string,integer)=a;`, false},
		{"different mapping keys", `a:mapping(integer,integer)=mapping(integer,integer){[1]=>1};b:mapping(string,integer)=a;`, false},
		{"mapping scalar", `a:mapping(string,integer)=1;`, false},
		{"same enum array", `Color:enum={red;}a:[Color]integer=[]integer{1};b:[Color]integer=a;`, true},
		{"different enum array values", `Color:enum={red;}a:[Color]string=[]string{"a"};b:[Color]integer=a;`, false},
		{"different enum array keys", `Color:enum={red;}Status:enum={ok;}a:[Color]integer=[]integer{1};b:[Status]integer=a;`, false},
		{"same scope structure", `a:scope={x: string="a";}b:scope={x:string="b";}a=b;`, true},
		{"different scope field", `a:scope={x:string="a";}b:scope={x:integer=1;}a=b;`, false},
		{"missing scope field", `a:scope={x:string="a";}b:scope={}a=b;`, false},
		{"additional scope field", `a:scope={x:string="a";}b:scope={x:string="b";y:integer=1;}a=b;`, false},
		{"empty scope", `a:scope={}b:scope={}a=b;`, true},
		{"enum red", `Color:enum={red;green;blue;}value:Color=.red;`, true},
		{"enum green", `Color:enum={red;green;blue;}value:Color=.green;`, true},
		{"enum blue", `Color:enum={red;green;blue;}value:Color=.blue;`, true},
		{"enum absent", `Color:enum={red;green;blue;}value:Color=.yellow;`, false},
		{"empty enum", `Empty:enum={}value:Empty=.anything;`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			checked, problem := checkSource(test.source)
			if !test.valid && problem != nil && strings.HasPrefix(problem.Message, "Expected") {
				t.Fatalf("invalid test syntax: %v", problem)
			}
			if (problem == nil) != test.valid {
				t.Fatalf("valid=%v, diagnostic=%v", test.valid, problem)
			}
			if test.valid && (checked == nil || checked.Program == nil) {
				t.Fatal("missing checked program")
			}
		})
	}
}

// TestDependencyCyclesExposeEveryReferenceKind verifies dependency discovery yields useful cycle diagnostics rather than forward-reference errors.
func TestDependencyCyclesExposeEveryReferenceKind(t *testing.T) {
	for _, test := range []struct {
		name, source string
		names        []string
	}{
		{"identifier", `a:=b;b:=a;`, []string{"a", "b"}},
		{"self", `value:=value;`, []string{"value"}},
		{"array", `a:[]integer=[]integer{b};b:integer=a[0];`, []string{"a", "b"}},
		{"setup field", `Point:setup={x:integer;}a:=Point{x=b};b:=a;`, []string{"a", "b"}},
		{"mapping key", `a:mapping(integer,integer)=mapping(integer,integer){[b]=>1};b:integer=a[0];`, []string{"a", "b"}},
		{"mapping value", `a:mapping(integer,integer)=mapping(integer,integer){[0]=>b};b:integer=a[0];`, []string{"a", "b"}},
		{"type", `A:setup={value:B;}B:setup={value:A;}`, []string{"A", "B"}},
		{"array type", `A:setup={value:[]B;}B:setup={value:A;}`, []string{"A", "B"}},
		{"mapping type", `A:setup={value:mapping(string,B);}B:setup={value:A;}`, []string{"A", "B"}},
		{"enum array type", `Keys:enum={key;}A:setup={value:[Keys]B;}B:setup={value:A;}`, []string{"A", "B"}},
		{"scope member", `s:scope={a:=s.b;b:=s.a;}`, []string{"s.a", "s.b"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, problem := checkSource(test.source)
			if problem == nil || !strings.Contains(strings.ToLower(problem.Message), "circular") {
				t.Fatalf("expected dependency cycle, got %v", problem)
			}
			for _, name := range test.names {
				if !strings.Contains(problem.Message, name) {
					t.Fatalf("cycle omitted %q: %v", name, problem)
				}
			}
		})
	}
	if _, problem := checkSource(`value:=42;`); problem != nil {
		t.Fatalf("literal without dependencies: %v", problem)
	}
}

// TestCheckerScopeAndConstantStrings verifies lexical lookup, independent checker state, and static timestamp validation.
func TestCheckerScopeAndConstantStrings(t *testing.T) {
	for _, test := range []struct {
		source string
		valid  bool
	}{
		{`outer:=1;s:scope={inner:=outer;}`, true},
		{`outer:=1;s:scope={inner:="child";}after:=outer;`, true},
		{`s:scope={hidden:=1;}value:=hidden;`, false},
		{`value:timestamp="2025-01-15 10:30:00";`, true},
		{`key:="2025-01-15 10:30:00";value:timestamp="{key}";`, true},
		{`value:timestamp="";`, false},
	} {
		if _, problem := checkSource(test.source); (problem == nil) != test.valid {
			t.Errorf("source=%s, diagnostic=%v", test.source, problem)
		}
	}
	for range 2 {
		if checked, problem := checkSource(`same:=1;`); problem != nil || len(checked.AssignmentTypes) != 1 {
			t.Fatalf("checker state leaked: %v", problem)
		}
	}
}

// TestTypedExpressionDependencies verifies expanded typed results still contribute reference edges to cycle detection.
func TestTypedExpressionDependencies(t *testing.T) {
	loc := &subject.SourceLocation{Line: 1, Col: 1}
	program := &subject.Program{Statements: []subject.Statement{
		&subject.Assignment{StatementBase: subject.StatementBase{Location: loc}, Target: &subject.IdentifierLValue{Name: "inner"}, Mode: subject.AssignInferred, Value: &subject.TypedExpression{Type: subject.PrimInteger, Value: &subject.ReferenceExpression{Root: "inner", Location: loc}, Location: loc}},
	}}
	defer func() {
		value := recover()
		err, ok := value.(*subject.SomethingError)
		if !ok || !strings.Contains(err.Message, "inner") {
			t.Fatalf("typed cycle=%v", value)
		}
	}()
	subject.NewTypeChecker(program, "fixture.something").Check()
	t.Fatal("typed cycle accepted")
}

// TestCheckedAnnotationRetainsDeclaredType verifies semantic annotations preserve named and primitive identities.
func TestCheckedAnnotationRetainsDeclaredType(t *testing.T) {
	checked, problem := checkSource(`value:timestamp="2025-01-15 10:30:00";`)
	if problem != nil {
		t.Fatal(problem)
	}
	assignment := checked.Program.Statements[0].(*subject.Assignment)
	if got := fmt.Sprint(checked.AssignmentTypes[assignment]); got != "timestamp" {
		t.Fatalf("annotation=%q", got)
	}
}
