// helpers_test.go contains shared test helpers, test-only types, and
// interface marker methods used by all SOMETHING test files. It is the
// common infrastructure for lexer, parser, evaluator, macro, and
// workspace-config tests.
package testhelpers

import subject "analysis/backend/something"

import (
	"fmt"
	"strings"
	"testing"
)

// testData supports the package test suite's test data setup or assertions.
func TestData() map[string]any {
	return map[string]any{
		"name":     "test",
		"count":    42,
		"ratio":    3.14,
		"active":   true,
		"tags":     []any{"a", "b", "c"},
		"metadata": map[string]any{"version": 1, "author": "me"},
		"iteration_0000000000": map[string]any{
			"title": "first",
			"value": 100,
		},
		"iteration_0000000001": map[string]any{
			"title": "second",
			"value": 200,
		},
		"iteration_0000000002": map[string]any{
			"title": "third",
			"value": 300,
		},
		"iteration_0000000000_label": map[string]any{
			"x": "labeled",
		},
		"iteration_0000000001_label": map[string]any{
			"x": "labeled2",
		},
	}
}

// evalText supports the package test suite's eval text setup or assertions.
func EvalText(t *testing.T, text string) map[string]any {
	t.Helper()
	tokens := subject.NewLexer(text, "").Tokenize()
	syntax := subject.NewParser(tokens, "").ParseProgram()
	expanded := subject.NewDirectiveGenerator("").Expand(syntax)
	checked := subject.NewTypeChecker(expanded, "").Check()
	ev := subject.NewEvaluator(checked, "")
	return ev.evaluate()
}

// evalTextErr supports the package test suite's eval text err setup or assertions.
func EvalTextErr(t *testing.T, text string) string {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if se, ok := r.(*subject.SomethingError); ok {
				t.Logf("expected error: %s", se.Message)
			}
		}
	}()
	tokens := subject.NewLexer(text, "").Tokenize()
	syntax := subject.NewParser(tokens, "").ParseProgram()
	expanded := subject.NewDirectiveGenerator("").Expand(syntax)
	checked := subject.NewTypeChecker(expanded, "").Check()
	ev := subject.NewEvaluator(checked, "")
	ev.evaluate()
	return ""
}

// NodeKind identifies the compact test-only AST view used by parser assertions while production retains one source-ordered representation.
type NodeKind int

const (
	KindString NodeKind = iota
	KindInteger
	KindFloat
	KindBoolean
	KindMapping
	KindMultiline
	KindReference
	KindStruct
	KindArray
	KindInclude
	KindMacroCall
)

// ValueNode is a fixture type used by the package test suite.
type ValueNode struct {
	Kind     NodeKind
	Raw      any
	Resolved any
	Location *subject.SourceLocation
}

// VarDecl is a fixture type used by the package test suite.
type VarDecl struct {
	Name         string
	InferType    bool
	ExplicitType string
	DeclaredType subject.TypeRef
	Value        *ValueNode
	Priv         bool
	Location     *subject.SourceLocation
}

// IterationDecl is a fixture type used by the package test suite.
type IterationDecl struct {
	IterationLabel subject.Expression
	Value          *ValueNode
	InferType      bool
	ExplicitType   string
	DeclaredType   subject.TypeRef
	Priv           bool
	Location       *subject.SourceLocation
}

// AsLvalueDecl is a fixture type used by the package test suite.
type AsLvalueDecl struct {
	NameExpr     *ValueNode
	Value        *ValueNode
	InferType    bool
	ExplicitType string
	DeclaredType subject.TypeRef
	Priv         bool
	Location     *subject.SourceLocation
}

// ForDecl is a fixture type used by the package test suite.
type ForDecl struct {
	ElementName string
	KeyName     string
	Source      *ValueNode
	Body        []any
}

// InsertDecl is a fixture type used by the package test suite.
type InsertDecl struct {
	Contents []*ValueNode
}

// IncludeDecl is a fixture type used by the package test suite.
type IncludeDecl struct {
	Filepath string
}

// ScopeDecl is a fixture type used by the package test suite.
type ScopeDecl struct {
	Body []any
}

// parsedAssignmentView is a fixture type used by the package test suite.
type ParsedAssignmentView struct {
	LValue any
	RValue any
}

// MemberPair is a fixture type used by the package test suite.
type MemberPair struct {
	Name  string
	Value any
}

// EnumDecl is a fixture type used by the package test suite.
type EnumDecl struct {
	Name      string
	ValueType subject.TypeRef
	Members   []MemberPair
}

// SetupDecl is a fixture type used by the package test suite.
type SetupDecl struct {
	Name   string
	Fields []*subject.FieldDefinition
}

// MacroDecl is a fixture type used by the package test suite.
type MacroDecl struct {
	Name    string
	Params  []subject.MacroParam
	SetExpr *ValueNode
}

// Priv supplies IncludeDecl test-fixture behavior.
func (*IncludeDecl) Priv() bool { return false }

// scopeBodyItemMarker supplies VarDecl test-fixture behavior.
func (*VarDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies ForDecl test-fixture behavior.
func (*ForDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies InsertDecl test-fixture behavior.
func (*InsertDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies IncludeDecl test-fixture behavior.
func (*IncludeDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies IterationDecl test-fixture behavior.
func (*IterationDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies AsLvalueDecl test-fixture behavior.
func (*AsLvalueDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies ScopeDecl test-fixture behavior.
func (*ScopeDecl) scopeBodyItemMarker() {}

// scopeBodyItemMarker supplies MacroDecl test-fixture behavior.
func (*MacroDecl) scopeBodyItemMarker() {}

// getLocation supports the package test suite's get location setup or assertions.
func GetLocation(value any) *subject.SourceLocation {
	switch value := value.(type) {
	case *VarDecl:
		return value.Location
	case *IterationDecl:
		return value.Location
	case *AsLvalueDecl:
		return value.Location
	}
	return nil
}

// validExprKindsForType supports the package test suite's valid expr kinds for type setup or assertions.
func ValidExprKindsForType(subject.TypeRef) []NodeKind { return nil }

// kindName supports the package test suite's kind name setup or assertions.
func KindName(kind NodeKind) string {
	switch kind {
	case KindString:
		return "string"
	case KindInteger:
		return "integer"
	case KindFloat:
		return "float"
	case KindBoolean:
		return "boolean"
	case KindMapping:
		return "mapping"
	case KindMultiline:
		return "multiline"
	case KindReference:
		return "reference"
	case KindStruct:
		return "struct"
	case KindArray:
		return "array"
	case KindInclude:
		return "include"
	case KindMacroCall:
		return "macro call"
	default:
		return "unknown"
	}
}

// parsedProgramView is a fixture type used by the package test suite.
type ParsedProgramView struct {
	Enums              []*EnumDecl
	Setups             []*SetupDecl
	TopLevelVars       []*VarDecl
	TopLevelFors       []*ForDecl
	TopLevelInserts    []*InsertDecl
	TopLevelIncludes   []*IncludeDecl
	TopLevelIterations []*IterationDecl
	TopLevelAsLvalues  []*AsLvalueDecl
	TopLevelBareScopes []*ScopeDecl
	Scopes             []*ParsedAssignmentView
	Macros             []*MacroDecl
}

// parsedValue supports the package test suite's parsed value setup or assertions.
func ParsedValue(expression subject.Expression) *ValueNode {
	if expression == nil {
		return nil
	}
	value := &ValueNode{Location: expression.expressionLocation()}
	switch expression := expression.(type) {
	case *subject.StringExpression:
		value.Kind = KindString
		value.Raw = expression.Literal
		if expression.Multiline != "" {
			value.Kind = KindMultiline
			value.Resolved = expression.Multiline
		}
	case *subject.IntegerExpression:
		value.Kind = KindInteger
		value.Resolved = expression.Value
	case *subject.FloatExpression:
		value.Kind = KindFloat
		value.Resolved = expression.Value
	case *subject.BooleanExpression:
		value.Kind = KindBoolean
		value.Resolved = expression.Value
	case *subject.ReferenceExpression:
		value.Kind = KindReference
		value.Raw = ParsedReferencePath(expression)
	case *subject.ArrayExpression:
		value.Kind = KindArray
		elements := make([]*ValueNode, len(expression.Elements))
		for index, element := range expression.Elements {
			elements[index] = ParsedValue(element)
		}
		value.Resolved = elements
	case *subject.MappingExpression:
		value.Kind = KindMapping
		value.Resolved = expression
	case *subject.StructExpression:
		value.Kind = KindStruct
		if expression.TypeName != "" {
			value.Raw = expression.TypeName
		}
		value.Resolved = expression.Fields
	case *subject.IncludeExpression:
		value.Kind = KindInclude
		value.Raw = expression.Filepath
	case *subject.MacroCallExpression:
		value.Kind = KindMacroCall
		value.Raw = expression.Name
		arguments := make([]*ValueNode, len(expression.Arguments))
		for index, argument := range expression.Arguments {
			arguments[index] = ParsedValue(argument)
		}
		value.Resolved = arguments
	}
	return value
}

// parsedReferencePath supports the package test suite's parsed reference path setup or assertions.
func ParsedReferencePath(reference *subject.ReferenceExpression) string {
	var result strings.Builder
	result.WriteString(reference.Root)
	for _, access := range reference.Accesses {
		switch access := access.(type) {
		case *subject.FieldAccess:
			result.WriteString(".")
			result.WriteString(access.Name)
		case *subject.IndexAccess:
			result.WriteString("[")
			switch index := access.Index.(type) {
			case *subject.IntegerExpression:
				result.WriteString(fmt.Sprint(index.Value))
			case *subject.StringExpression:
				result.WriteString(fmt.Sprintf("%q", StringLiteralToStringValue(index)))
			case *subject.ReferenceExpression:
				result.WriteString(ParsedReferencePath(index))
			}
			result.WriteString("]")
		}
	}
	return result.String()
}

// stringLiteralToStringValue supports the package test suite's string literal to string value setup or assertions.
func StringLiteralToStringValue(expression *subject.StringExpression) string {
	if expression.Multiline != "" {
		return expression.Multiline
	}
	var result strings.Builder
	for _, part := range expression.Literal.Parts {
		if text, ok := part.(subject.StringText); ok {
			result.WriteString(string(text))
		}
	}
	return result.String()
}

// assignmentName supports the package test suite's assignment name setup or assertions.
func AssignmentName(target subject.LValue) string {
	if identifier, ok := target.(*subject.IdentifierLValue); ok {
		return identifier.Name
	}
	return ""
}

// explicitType supports the package test suite's explicit type setup or assertions.
func ExplicitType(assignment *subject.Assignment) string {
	if assignment.DeclaredType == nil {
		if _, ok := assignment.Value.(*subject.IncludeExpression); ok {
			return "namespace"
		}
		return ""
	}
	switch assignment.DeclaredType.(type) {
	case *subject.ArrayType, *subject.EnumKeyType:
		return "array"
	case *subject.MappingType:
		return "mapping"
	}
	return subject.typeRefString(assignment.DeclaredType)
}

// parsedBody supports the package test suite's parsed body setup or assertions.
func ParsedBody(statements []subject.Statement) []any {
	items := make([]any, 0, len(statements))
	for _, statement := range statements {
		switch statement := statement.(type) {
		case *subject.Assignment:
			switch target := statement.Target.(type) {
			case *subject.IdentifierLValue:
				items = append(items, &VarDecl{Name: target.Name, InferType: statement.Mode == subject.AssignInferred, ExplicitType: ExplicitType(statement), DeclaredType: statement.DeclaredType, Value: ParsedValue(statement.Value.(subject.Expression)), Priv: statement.Private, Location: statement.Location})
			case *subject.IterationLValue:
				if scope, ok := statement.Value.(*subject.ScopeExpression); ok {
					items = append(items, &ParsedAssignmentView{LValue: target, RValue: &ScopeDecl{Body: ParsedBody(scope.Statements)}})
				} else {
					items = append(items, &IterationDecl{IterationLabel: target.Label, Value: ParsedValue(statement.Value.(subject.Expression)), InferType: statement.Mode == subject.AssignInferred, ExplicitType: ExplicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
				}
			case *subject.AsLValue:
				items = append(items, &AsLvalueDecl{NameExpr: ParsedValue(target.Name), Value: ParsedValue(statement.Value.(subject.Expression)), InferType: statement.Mode == subject.AssignInferred, ExplicitType: ExplicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
			}
		case *subject.ForDirective:
			items = append(items, &ForDecl{ElementName: statement.ElementName, KeyName: statement.KeyName, Source: ParsedValue(statement.Source), Body: ParsedBody(statement.Body)})
		case *subject.InsertDirective:
			values := make([]*ValueNode, len(statement.Contents))
			for index, content := range statement.Contents {
				values[index] = ParsedValue(content)
			}
			items = append(items, &InsertDecl{Contents: values})
		case *subject.IncludeDirective:
			items = append(items, &IncludeDecl{Filepath: statement.Filepath})
		}
	}
	return items
}

// parsedView supports the package test suite's parsed view setup or assertions.
func ParsedView(program *subject.Program) *ParsedProgramView {
	view := &ParsedProgramView{}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *subject.Assignment:
			name := AssignmentName(statement.Target)
			switch value := statement.Value.(type) {
			case *subject.EnumDefinition:
				members := make([]MemberPair, len(value.Members))
				for index, member := range value.Members {
					members[index] = MemberPair{Name: member.Name, Value: ParsedValue(member.Value)}
				}
				view.Enums = append(view.Enums, &EnumDecl{Name: name, ValueType: value.ValueType, Members: members})
			case *subject.SetupDefinition:
				view.Setups = append(view.Setups, &SetupDecl{Name: name, Fields: value.Fields})
			case *subject.ScopeExpression:
				scope := &ScopeDecl{Body: ParsedBody(value.Statements)}
				if iteration, ok := statement.Target.(*subject.IterationLValue); ok {
					view.Scopes = append(view.Scopes, &ParsedAssignmentView{LValue: iteration, RValue: scope})
				} else {
					view.TopLevelBareScopes = append(view.TopLevelBareScopes, scope)
				}
			default:
				switch target := statement.Target.(type) {
				case *subject.IdentifierLValue:
					view.TopLevelVars = append(view.TopLevelVars, &VarDecl{Name: target.Name, InferType: statement.Mode == subject.AssignInferred, ExplicitType: ExplicitType(statement), DeclaredType: statement.DeclaredType, Value: ParsedValue(statement.Value.(subject.Expression)), Priv: statement.Private, Location: statement.Location})
				case *subject.IterationLValue:
					view.TopLevelIterations = append(view.TopLevelIterations, &IterationDecl{IterationLabel: target.Label, Value: ParsedValue(statement.Value.(subject.Expression)), InferType: statement.Mode == subject.AssignInferred, ExplicitType: ExplicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
				case *subject.AsLValue:
					view.TopLevelAsLvalues = append(view.TopLevelAsLvalues, &AsLvalueDecl{NameExpr: ParsedValue(target.Name), Value: ParsedValue(statement.Value.(subject.Expression)), InferType: statement.Mode == subject.AssignInferred, ExplicitType: ExplicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
				}
			}
		case *subject.ForDirective:
			view.TopLevelFors = append(view.TopLevelFors, &ForDecl{ElementName: statement.ElementName, KeyName: statement.KeyName, Source: ParsedValue(statement.Source), Body: ParsedBody(statement.Body)})
		case *subject.InsertDirective:
			values := make([]*ValueNode, len(statement.Contents))
			for index, content := range statement.Contents {
				values[index] = ParsedValue(content)
			}
			view.TopLevelInserts = append(view.TopLevelInserts, &InsertDecl{Contents: values})
		case *subject.IncludeDirective:
			view.TopLevelIncludes = append(view.TopLevelIncludes, &IncludeDecl{Filepath: statement.Filepath})
		case *subject.MacroDirective:
			view.Macros = append(view.Macros, &MacroDecl{Name: statement.Name, Params: statement.Params, SetExpr: ParsedValue(statement.Return)})
		}
	}
	return view
}

// tokenize supports the package test suite's tokenize setup or assertions.
func Tokenize(t *testing.T, text string) []subject.Token {
	t.Helper()
	return subject.NewLexer(text, "").Tokenize()
}

// tokenKinds supports the package test suite's token kinds setup or assertions.
func TokenKinds(t *testing.T, text string) []subject.TokenKind {
	t.Helper()
	ts := Tokenize(t, text)
	kinds := make([]subject.TokenKind, len(ts))
	for i, tok := range ts {
		kinds[i] = tok.Kind
	}
	return kinds
}

// assertKind supports the package test suite's assert kind setup or assertions.
func AssertKind(t *testing.T, tok subject.Token, expected subject.TokenKind) {
	t.Helper()
	if tok.Kind != expected {
		t.Errorf("expected token kind %v, got %v (value=%v)", expected, tok.Kind, tok.Value)
	}
}

// assertPanic supports the package test suite's assert panic setup or assertions.
func AssertPanic(t *testing.T, fn func(), msgContains string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic with message containing %q, but none occurred", msgContains)
			return
		}
		se, ok := r.(*subject.SomethingError)
		if !ok {
			t.Errorf("expected *SomethingError panic, got %T: %v", r, r)
			return
		}
		if !strings.Contains(se.Message, msgContains) {
			t.Errorf("expected panic message containing %q, got %q", msgContains, se.Message)
		}
	}()
	fn()
}

// parseText builds a parsed program view from source text.
func ParseText(t *testing.T, text string) *ParsedProgramView {
	t.Helper()
	tokens := subject.NewLexer(text, "").Tokenize()
	program := subject.NewParser(tokens, "").ParseProgram()
	return ParsedView(program)
}
