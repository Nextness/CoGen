// helpers_test.go contains shared test helpers, test-only types, and
// interface marker methods used by all SOMETHING test files. It is the
// common infrastructure for lexer, parser, evaluator, macro, and
// workspace-config tests.
package something

import (
	"fmt"
	"strings"
	"testing"
)

// testData supports the package test suite's test data setup or assertions.
func testData() map[string]any {
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
func evalText(t *testing.T, text string) map[string]any {
	t.Helper()
	tokens := NewLexer(text, "").Tokenize()
	syntax := NewParser(tokens, "").ParseProgram()
	expanded := NewDirectiveGenerator("").Expand(syntax)
	checked := NewTypeChecker(expanded, "").Check()
	ev := NewEvaluator(checked, "")
	return ev.evaluate()
}

// evalTextErr supports the package test suite's eval text err setup or assertions.
func evalTextErr(t *testing.T, text string) string {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if se, ok := r.(*SomethingError); ok {
				t.Logf("expected error: %s", se.Message)
			}
		}
	}()
	tokens := NewLexer(text, "").Tokenize()
	syntax := NewParser(tokens, "").ParseProgram()
	expanded := NewDirectiveGenerator("").Expand(syntax)
	checked := NewTypeChecker(expanded, "").Check()
	ev := NewEvaluator(checked, "")
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
	Location *SourceLocation
}

// VarDecl is a fixture type used by the package test suite.
type VarDecl struct {
	Name         string
	InferType    bool
	ExplicitType string
	DeclaredType TypeRef
	Value        *ValueNode
	Priv         bool
	Location     *SourceLocation
}

// IterationDecl is a fixture type used by the package test suite.
type IterationDecl struct {
	IterationLabel Expression
	Value          *ValueNode
	InferType      bool
	ExplicitType   string
	DeclaredType   TypeRef
	Priv           bool
	Location       *SourceLocation
}

// AsLvalueDecl is a fixture type used by the package test suite.
type AsLvalueDecl struct {
	NameExpr     *ValueNode
	Value        *ValueNode
	InferType    bool
	ExplicitType string
	DeclaredType TypeRef
	Priv         bool
	Location     *SourceLocation
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
type parsedAssignmentView struct {
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
	ValueType TypeRef
	Members   []MemberPair
}

// SetupDecl is a fixture type used by the package test suite.
type SetupDecl struct {
	Name   string
	Fields []*FieldDefinition
}

// MacroDecl is a fixture type used by the package test suite.
type MacroDecl struct {
	Name    string
	Params  []MacroParam
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
func getLocation(value any) *SourceLocation {
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
func validExprKindsForType(TypeRef) []NodeKind { return nil }

// kindName supports the package test suite's kind name setup or assertions.
func kindName(kind NodeKind) string {
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
type parsedProgramView struct {
	Enums              []*EnumDecl
	Setups             []*SetupDecl
	TopLevelVars       []*VarDecl
	TopLevelFors       []*ForDecl
	TopLevelInserts    []*InsertDecl
	TopLevelIncludes   []*IncludeDecl
	TopLevelIterations []*IterationDecl
	TopLevelAsLvalues  []*AsLvalueDecl
	TopLevelBareScopes []*ScopeDecl
	Scopes             []*parsedAssignmentView
	Macros             []*MacroDecl
}

// parsedValue supports the package test suite's parsed value setup or assertions.
func parsedValue(expression Expression) *ValueNode {
	if expression == nil {
		return nil
	}
	value := &ValueNode{Location: expression.expressionLocation()}
	switch expression := expression.(type) {
	case *StringExpression:
		value.Kind = KindString
		value.Raw = expression.Literal
		if expression.Multiline != "" {
			value.Kind = KindMultiline
			value.Resolved = expression.Multiline
		}
	case *IntegerExpression:
		value.Kind = KindInteger
		value.Resolved = expression.Value
	case *FloatExpression:
		value.Kind = KindFloat
		value.Resolved = expression.Value
	case *BooleanExpression:
		value.Kind = KindBoolean
		value.Resolved = expression.Value
	case *ReferenceExpression:
		value.Kind = KindReference
		value.Raw = parsedReferencePath(expression)
	case *ArrayExpression:
		value.Kind = KindArray
		elements := make([]*ValueNode, len(expression.Elements))
		for index, element := range expression.Elements {
			elements[index] = parsedValue(element)
		}
		value.Resolved = elements
	case *MappingExpression:
		value.Kind = KindMapping
		value.Resolved = expression
	case *StructExpression:
		value.Kind = KindStruct
		if expression.TypeName != "" {
			value.Raw = expression.TypeName
		}
		value.Resolved = expression.Fields
	case *IncludeExpression:
		value.Kind = KindInclude
		value.Raw = expression.Filepath
	case *MacroCallExpression:
		value.Kind = KindMacroCall
		value.Raw = expression.Name
		arguments := make([]*ValueNode, len(expression.Arguments))
		for index, argument := range expression.Arguments {
			arguments[index] = parsedValue(argument)
		}
		value.Resolved = arguments
	}
	return value
}

// parsedReferencePath supports the package test suite's parsed reference path setup or assertions.
func parsedReferencePath(reference *ReferenceExpression) string {
	var result strings.Builder
	result.WriteString(reference.Root)
	for _, access := range reference.Accesses {
		switch access := access.(type) {
		case *FieldAccess:
			result.WriteString(".")
			result.WriteString(access.Name)
		case *IndexAccess:
			result.WriteString("[")
			switch index := access.Index.(type) {
			case *IntegerExpression:
				result.WriteString(fmt.Sprint(index.Value))
			case *StringExpression:
				result.WriteString(fmt.Sprintf("%q", stringLiteralToStringValue(index)))
			case *ReferenceExpression:
				result.WriteString(parsedReferencePath(index))
			}
			result.WriteString("]")
		}
	}
	return result.String()
}

// stringLiteralToStringValue supports the package test suite's string literal to string value setup or assertions.
func stringLiteralToStringValue(expression *StringExpression) string {
	if expression.Multiline != "" {
		return expression.Multiline
	}
	var result strings.Builder
	for _, part := range expression.Literal.Parts {
		if text, ok := part.(StringText); ok {
			result.WriteString(string(text))
		}
	}
	return result.String()
}

// assignmentName supports the package test suite's assignment name setup or assertions.
func assignmentName(target LValue) string {
	if identifier, ok := target.(*IdentifierLValue); ok {
		return identifier.Name
	}
	return ""
}

// explicitType supports the package test suite's explicit type setup or assertions.
func explicitType(assignment *Assignment) string {
	if assignment.DeclaredType == nil {
		if _, ok := assignment.Value.(*IncludeExpression); ok {
			return "namespace"
		}
		return ""
	}
	switch assignment.DeclaredType.(type) {
	case *ArrayType, *EnumKeyType:
		return "array"
	case *MappingType:
		return "mapping"
	}
	return typeRefString(assignment.DeclaredType)
}

// parsedBody supports the package test suite's parsed body setup or assertions.
func parsedBody(statements []Statement) []any {
	items := make([]any, 0, len(statements))
	for _, statement := range statements {
		switch statement := statement.(type) {
		case *Assignment:
			switch target := statement.Target.(type) {
			case *IdentifierLValue:
				items = append(items, &VarDecl{Name: target.Name, InferType: statement.Mode == AssignInferred, ExplicitType: explicitType(statement), DeclaredType: statement.DeclaredType, Value: parsedValue(statement.Value.(Expression)), Priv: statement.Private, Location: statement.Location})
			case *IterationLValue:
				if scope, ok := statement.Value.(*ScopeExpression); ok {
					items = append(items, &parsedAssignmentView{LValue: target, RValue: &ScopeDecl{Body: parsedBody(scope.Statements)}})
				} else {
					items = append(items, &IterationDecl{IterationLabel: target.Label, Value: parsedValue(statement.Value.(Expression)), InferType: statement.Mode == AssignInferred, ExplicitType: explicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
				}
			case *AsLValue:
				items = append(items, &AsLvalueDecl{NameExpr: parsedValue(target.Name), Value: parsedValue(statement.Value.(Expression)), InferType: statement.Mode == AssignInferred, ExplicitType: explicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
			}
		case *ForDirective:
			items = append(items, &ForDecl{ElementName: statement.ElementName, KeyName: statement.KeyName, Source: parsedValue(statement.Source), Body: parsedBody(statement.Body)})
		case *InsertDirective:
			values := make([]*ValueNode, len(statement.Contents))
			for index, content := range statement.Contents {
				values[index] = parsedValue(content)
			}
			items = append(items, &InsertDecl{Contents: values})
		case *IncludeDirective:
			items = append(items, &IncludeDecl{Filepath: statement.Filepath})
		}
	}
	return items
}

// parsedView supports the package test suite's parsed view setup or assertions.
func parsedView(program *Program) *parsedProgramView {
	view := &parsedProgramView{}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *Assignment:
			name := assignmentName(statement.Target)
			switch value := statement.Value.(type) {
			case *EnumDefinition:
				members := make([]MemberPair, len(value.Members))
				for index, member := range value.Members {
					members[index] = MemberPair{Name: member.Name, Value: parsedValue(member.Value)}
				}
				view.Enums = append(view.Enums, &EnumDecl{Name: name, ValueType: value.ValueType, Members: members})
			case *SetupDefinition:
				view.Setups = append(view.Setups, &SetupDecl{Name: name, Fields: value.Fields})
			case *ScopeExpression:
				scope := &ScopeDecl{Body: parsedBody(value.Statements)}
				if iteration, ok := statement.Target.(*IterationLValue); ok {
					view.Scopes = append(view.Scopes, &parsedAssignmentView{LValue: iteration, RValue: scope})
				} else {
					view.TopLevelBareScopes = append(view.TopLevelBareScopes, scope)
				}
			default:
				switch target := statement.Target.(type) {
				case *IdentifierLValue:
					view.TopLevelVars = append(view.TopLevelVars, &VarDecl{Name: target.Name, InferType: statement.Mode == AssignInferred, ExplicitType: explicitType(statement), DeclaredType: statement.DeclaredType, Value: parsedValue(statement.Value.(Expression)), Priv: statement.Private, Location: statement.Location})
				case *IterationLValue:
					view.TopLevelIterations = append(view.TopLevelIterations, &IterationDecl{IterationLabel: target.Label, Value: parsedValue(statement.Value.(Expression)), InferType: statement.Mode == AssignInferred, ExplicitType: explicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
				case *AsLValue:
					view.TopLevelAsLvalues = append(view.TopLevelAsLvalues, &AsLvalueDecl{NameExpr: parsedValue(target.Name), Value: parsedValue(statement.Value.(Expression)), InferType: statement.Mode == AssignInferred, ExplicitType: explicitType(statement), DeclaredType: statement.DeclaredType, Priv: statement.Private, Location: statement.Location})
				}
			}
		case *ForDirective:
			view.TopLevelFors = append(view.TopLevelFors, &ForDecl{ElementName: statement.ElementName, KeyName: statement.KeyName, Source: parsedValue(statement.Source), Body: parsedBody(statement.Body)})
		case *InsertDirective:
			values := make([]*ValueNode, len(statement.Contents))
			for index, content := range statement.Contents {
				values[index] = parsedValue(content)
			}
			view.TopLevelInserts = append(view.TopLevelInserts, &InsertDecl{Contents: values})
		case *IncludeDirective:
			view.TopLevelIncludes = append(view.TopLevelIncludes, &IncludeDecl{Filepath: statement.Filepath})
		case *MacroDirective:
			view.Macros = append(view.Macros, &MacroDecl{Name: statement.Name, Params: statement.Params, SetExpr: parsedValue(statement.Return)})
		}
	}
	return view
}

// tokenize supports the package test suite's tokenize setup or assertions.
func tokenize(t *testing.T, text string) []Token {
	t.Helper()
	return NewLexer(text, "").Tokenize()
}

// tokenKinds supports the package test suite's token kinds setup or assertions.
func tokenKinds(t *testing.T, text string) []TokenKind {
	t.Helper()
	ts := tokenize(t, text)
	kinds := make([]TokenKind, len(ts))
	for i, tok := range ts {
		kinds[i] = tok.Kind
	}
	return kinds
}

// assertKind supports the package test suite's assert kind setup or assertions.
func assertKind(t *testing.T, tok Token, expected TokenKind) {
	t.Helper()
	if tok.Kind != expected {
		t.Errorf("expected token kind %v, got %v (value=%v)", expected, tok.Kind, tok.Value)
	}
}

// assertPanic supports the package test suite's assert panic setup or assertions.
func assertPanic(t *testing.T, fn func(), msgContains string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic with message containing %q, but none occurred", msgContains)
			return
		}
		se, ok := r.(*SomethingError)
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
func parseText(t *testing.T, text string) *parsedProgramView {
	t.Helper()
	tokens := NewLexer(text, "").Tokenize()
	program := NewParser(tokens, "").ParseProgram()
	return parsedView(program)
}
