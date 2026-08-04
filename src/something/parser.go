// parser.go builds a source-ordered syntax AST without evaluating directives,
// resolving names, or checking types.
package something

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser tracks a token stream and file identity while building an ordered syntax tree.
type Parser struct {
	tokens   []Token
	pos      int
	filepath string
}

// NewParser constructs parser.
func NewParser(tokens []Token, filepath string) *Parser {
	return &Parser{tokens: tokens, filepath: filepath}
}

// Precedence constants for Pratt-style expression parsing.
const (
	precLowest  = 0
	precOr      = 1
	precAnd     = 2
	precEquals  = 3
	precRel     = 4
	precPrefix  = 5
	precPrimary = 6
)

// peekPrecedence returns the precedence of the next infix operator, or precLowest.
func (p *Parser) peekPrecedence() int {
	// Two-token infix operators: #and, #or
	if p.peek(0).Kind == TkHASH {
		switch p.peek(1).Kind {
		case TkAND:
			return precAnd
		case TkOR:
			return precOr
		}
		return precLowest
	}
	// Single-token infix operators
	switch p.peek(0).Kind {
	case TkEQ, TkNEQ:
		return precEquals
	case TkLT, TkLE, TkGT, TkGE:
		return precRel
	default:
		return precLowest
	}
}

// infixBinding returns the BinaryOpKind for the next infix operator, if any.
func (p *Parser) infixBinding() (BinaryOpKind, bool) {
	if p.peek(0).Kind == TkHASH {
		switch p.peek(1).Kind {
		case TkAND:
			return OpAnd, true
		case TkOR:
			return OpOr, true
		}
		return 0, false
	}
	switch p.peek(0).Kind {
	case TkEQ:
		return OpEQ, true
	case TkNEQ:
		return OpNEQ, true
	case TkLT:
		return OpLT, true
	case TkLE:
		return OpLE, true
	case TkGT:
		return OpGT, true
	case TkGE:
		return OpGE, true
	default:
		return 0, false
	}
}

// peek returns a token relative to the current parser position, using EOF outside the stream.
func (p *Parser) peek(offset int) Token {
	index := p.pos + offset
	if index >= 0 && index < len(p.tokens) {
		return p.tokens[index]
	}
	return p.tokens[len(p.tokens)-1]
}

// advance consumes and returns the current token.
func (p *Parser) advance() Token {
	token := p.peek(0)
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return token
}

// location converts token coordinates to a file-aware source location.
func (p *Parser) location(token Token) *SourceLocation {
	return &SourceLocation{Line: token.Line, Col: token.Col, Filepath: p.filepath}
}

// err panics with a source-located SomethingError at the supplied or current location.
func (p *Parser) err(message, suggestion string, location *SourceLocation) {
	if location == nil {
		location = p.location(p.peek(0))
	}
	panic(errLoc(message, location, p.filepath, suggestion))
}

// expect consumes the required token kind or raises the requested syntax error.
func (p *Parser) expect(kind TokenKind, message string) Token {
	token := p.peek(0)
	if token.Kind != kind {
		if message == "" {
			message = "Expected " + kind.String() + ", got " + token.Kind.String()
		}
		p.err(message, "", p.location(token))
	}
	return p.advance()
}

// expectStatementTerminator consumes the semicolon required after a statement context.
func (p *Parser) expectStatementTerminator(context string) {
	p.expect(TkSEMICOLON, "Expected ';' after "+context)
}

// ParseProgram parses one complete file. Directives remain in the syntax AST.
func (p *Parser) ParseProgram() *Program {
	return &Program{Statements: p.parseStatements(TkEOF), Filepath: p.filepath}
}

// parseStatements parses statements from the supplied input.
func (p *Parser) parseStatements(end TokenKind) []Statement {
	statements := []Statement{}
	for p.peek(0).Kind != end {
		if p.peek(0).Kind == TkEOF {
			p.err("Unexpected end of file", "Close the current block with '}'", nil)
		}
		statements = append(statements, p.parseStatement())
	}
	return statements
}

// parseStatement parses statement from the supplied input.
func (p *Parser) parseStatement() Statement {
	if p.peek(0).Kind == TkHASH {
		hash := p.advance()
		directive := p.peek(0)
		if directive.Kind == TkPRIV {
			p.advance()
			statement := p.parseStatement()
			statement.statementBase().Private = true
			if statement.statementBase().Location == nil {
				statement.statementBase().Location = p.location(hash)
			}
			return statement
		}
		return p.parseDirectiveStatement(hash)
	}

	if p.peek(0).Kind != TkIDENTIFIER {
		p.err("Expected an assignment or directive, got "+p.peek(0).Kind.String(), "Statements start with an identifier or '#'", nil)
	}
	start := p.peek(0)
	target := p.parseNamedLValue()
	return p.parseAssignment(target, p.location(start))
}

// parseDirectiveStatement parses directive statement from the supplied input.
func (p *Parser) parseDirectiveStatement(hash Token) Statement {
	location := p.location(hash)
	directive := p.peek(0)
	switch directive.Kind {
	case TkINCLUDE:
		p.advance()
		path := p.parseIncludePath()
		p.expectStatementTerminator("#include directive")
		return &IncludeDirective{StatementBase: StatementBase{Location: location}, Filepath: path}
	case TkFOR:
		p.advance()
		return p.parseForDirective(location)
	case TkINSERT:
		p.advance()
		insert := p.parseInsertDirective(location)
		p.expectStatementTerminator("#insert directive")
		return insert
	case TkITERATION:
		p.advance()
		target := &IterationLValue{Label: p.parseOptionalDirectiveArgument("#iteration")}
		return p.parseAssignment(target, location)
	case TkASLVALUE:
		p.advance()
		p.expect(TkLPAREN, "Expected '(' after #as_lvalue")
		name := p.parseExpression()
		p.expect(TkRPAREN, "Expected ')' after #as_lvalue argument")
		return p.parseAssignment(&AsLValue{Name: name}, location)
	case TkMACRO:
		p.advance()
		return p.parseMacroDirective(location)
	case TkASSERT:
		p.advance()
		return p.parseAssertDirective(location)
	case TkIF:
		p.advance()
		return p.parseIfDirective(location)
	case TkERROR:
		p.advance()
		return p.parseErrorDirective(location)
	default:
		name := directive.StrValue()
		if name == "" {
			name = directive.Kind.String()
		}
		p.err("Unknown directive '#"+name+"'", "Valid directives: #include, #for, #insert, #iteration, #as_lvalue, #macro, #priv, #assert, #if, #error", location)
	}
	return nil
}

// parseAssertDirective parses assert directive from the supplied input.
func (p *Parser) parseAssertDirective(location *SourceLocation) *AssertDirective {
	nameToken := p.expect(TkIDENTIFIER, "Expected setup type name after #assert")
	p.expect(TkLBRACE, "Expected '{' after #assert type name")
	body := p.parseStatements(TkRBRACE)
	p.expect(TkRBRACE, "Expected '}' after #assert body")
	return &AssertDirective{
		StatementBase: StatementBase{Location: location},
		TypeName:      nameToken.StrValue(),
		Body:          body,
	}
}

// parseIfDirective parses if directive from the supplied input.
func (p *Parser) parseIfDirective(location *SourceLocation) *IfDirective {
	condition := p.parseExpression()
	if p.peek(0).Kind == TkLBRACE {
		p.advance()
		body := p.parseStatements(TkRBRACE)
		p.expect(TkRBRACE, "Expected '}' after #if body")
		return &IfDirective{
			StatementBase: StatementBase{Location: location},
			Condition:     condition,
			Body:          body,
		}
	}
	// Single-statement form: #if cond stmt;
	stmt := p.parseStatement()
	return &IfDirective{
		StatementBase: StatementBase{Location: location},
		Condition:     condition,
		Body:          []Statement{stmt},
	}
}

// parseErrorDirective parses error directive from the supplied input.
func (p *Parser) parseErrorDirective(location *SourceLocation) *ErrorDirective {
	p.expect(TkLPAREN, "Expected '(' after #error")
	message := p.parseExpression()
	p.expect(TkRPAREN, "Expected ')' after #error message")
	p.expectStatementTerminator("#error directive")
	return &ErrorDirective{
		StatementBase: StatementBase{Location: location},
		Message:       message,
	}
}

// parseOptionalDirectiveArgument parses optional directive argument from the supplied input.
func (p *Parser) parseOptionalDirectiveArgument(name string) Expression {
	if p.peek(0).Kind != TkLPAREN {
		return nil
	}
	p.advance()
	if p.peek(0).Kind == TkRPAREN {
		p.err(name+" expects a string argument when parentheses are present", "Remove the parentheses or provide a string label", nil)
	}
	argument := p.parseExpression()
	p.expect(TkRPAREN, "Expected ')' after "+name+" argument")
	return argument
}

// parseAssignment parses assignment from the supplied input.
func (p *Parser) parseAssignment(target LValue, location *SourceLocation) *Assignment {
	assignment := &Assignment{StatementBase: StatementBase{Location: location}, Target: target}
	requiresTerminator := true
	switch p.peek(0).Kind {
	case TkEQUALS:
		p.advance()
		assignment.Mode = AssignExisting
		assignment.Value = p.parseExpression()
	case TkCOLON:
		p.advance()
		if p.peek(0).Kind == TkEQUALS {
			p.advance()
			assignment.Mode = AssignInferred
			assignment.Value = p.parseExpression()
			break
		}
		assignment.Mode = AssignExplicit
		switch p.peek(0).Kind {
		case TkENUM:
			p.advance()
			assignment.Value = p.parseEnumDefinition(location)
			requiresTerminator = false
		case TkSETUP:
			p.advance()
			assignment.Value = p.parseSetupDefinition(location)
			requiresTerminator = false
		default:
			assignment.DeclaredType = p.parseTypeRef()
			p.expect(TkEQUALS, "Expected '=' after assignment type")
			if assignment.DeclaredType == PrimScope && p.peek(0).Kind == TkLBRACE {
				assignment.Value = p.parseScopeExpression()
				requiresTerminator = false
			} else {
				assignment.Value = p.parseExpression()
			}
		}
	default:
		p.err("Expected ':', ':=', or '=' after assignment target", "Use 'name: type = value', 'name := value', or 'name = value'", nil)
	}
	if requiresTerminator {
		p.expectStatementTerminator("assignment")
	}
	return assignment
}

// parseNamedLValue parses named l value from the supplied input.
func (p *Parser) parseNamedLValue() LValue {
	root := p.expect(TkIDENTIFIER, "Expected assignment target").StrValue()
	accesses := p.parseAccesses()
	if len(accesses) == 0 {
		return &IdentifierLValue{Name: root}
	}
	return &MemberLValue{Root: root, Accesses: accesses}
}

// parseAccesses parses accesses from the supplied input.
func (p *Parser) parseAccesses() []Access {
	accesses := []Access{}
	for {
		switch p.peek(0).Kind {
		case TkDOT:
			dot := p.advance()
			name := p.expect(TkIDENTIFIER, "Expected member name after '.'").StrValue()
			accesses = append(accesses, &FieldAccess{Name: name, Location: p.location(dot)})
		case TkLBRACKET:
			open := p.advance()
			index := p.parseExpression()
			p.expect(TkRBRACKET, "Expected ']' after index expression")
			accesses = append(accesses, &IndexAccess{Index: index, Location: p.location(open)})
		default:
			return accesses
		}
	}
}

// parseIncludePath parses include path from the supplied input.
func (p *Parser) parseIncludePath() string {
	p.expect(TkLPAREN, "Expected '(' after #include")
	token := p.expect(TkSTRING_LITERAL, "Expected a string filepath in #include")
	p.expect(TkRPAREN, "Expected ')' after #include filepath")
	return stringLiteralToString(token)
}

// parseForDirective parses for directive from the supplied input.
func (p *Parser) parseForDirective(location *SourceLocation) *ForDirective {
	first := p.expect(TkIDENTIFIER, "Expected loop variable after #for").StrValue()
	keyName := ""
	elementName := first
	if p.peek(0).Kind == TkCOMMA {
		p.advance()
		keyName = first
		elementName = p.expect(TkIDENTIFIER, "Expected element variable after ','").StrValue()
	}
	p.expect(TkCOLON, "Expected ':' after #for variables")
	source := p.parseForSource()
	p.expect(TkLBRACE, "Expected '{' for #for body")
	body := p.parseStatements(TkRBRACE)
	p.expect(TkRBRACE, "Expected '}' after #for body")
	return &ForDirective{
		StatementBase: StatementBase{Location: location},
		ElementName:   elementName,
		KeyName:       keyName,
		Source:        source,
		Body:          body,
	}
}

// parseForSource keeps the loop body delimiter from being interpreted as the
// opening brace of a typed struct literal. Macro calls still use the general
// expression parser because they may produce an iterable value.
func (p *Parser) parseForSource() Expression {
	if p.peek(0).Kind == TkIDENTIFIER && p.peek(1).Kind != TkBANG {
		token := p.advance()
		return &ReferenceExpression{
			Root:     token.StrValue(),
			Accesses: p.parseAccesses(),
			Location: p.location(token),
		}
	}
	return p.parseExpression()
}

// parseInsertDirective parses insert directive from the supplied input.
func (p *Parser) parseInsertDirective(location *SourceLocation) *InsertDirective {
	p.expect(TkLBRACE, "Expected '{' after #insert")
	contents := []Expression{}
	for p.peek(0).Kind != TkRBRACE {
		contents = append(contents, p.parseExpression())
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRBRACE {
			p.err("Expected ',' between #insert values", "Separate generated source strings with commas", nil)
		}
	}
	p.expect(TkRBRACE, "Expected '}' after #insert values")
	return &InsertDirective{StatementBase: StatementBase{Location: location}, Contents: contents}
}

// parseMacroDirective parses macro directive from the supplied input.
func (p *Parser) parseMacroDirective(location *SourceLocation) *MacroDirective {
	name := p.expect(TkIDENTIFIER, "Expected macro name").StrValue()
	p.expect(TkCOLON, "Expected ':=' after macro name")
	p.expect(TkEQUALS, "Expected ':=' after macro name")
	p.expect(TkLPAREN, "Expected '(' before macro parameters")
	params := []MacroParam{}
	for p.peek(0).Kind != TkRPAREN {
		paramToken := p.expect(TkIDENTIFIER, "Expected macro parameter name")
		p.expect(TkCOLON, "Expected ':' after macro parameter name")
		params = append(params, MacroParam{Name: paramToken.StrValue(), Type: p.parseTypeRef(), Location: p.location(paramToken)})
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRPAREN {
			p.err("Expected ',' between macro parameters", "Separate macro parameters with commas", nil)
		}
	}
	p.expect(TkRPAREN, "Expected ')' after macro parameters")
	p.expect(TkRARROW, "Expected '->' after macro parameters")
	returnType := p.parseTypeRef()
	p.expect(TkLBRACE, "Expected '{' for macro body")
	body := []Statement{}
	var returnExpression Expression
	for p.peek(0).Kind != TkRBRACE {
		if p.peek(0).Kind == TkHASH && p.peek(1).Kind == TkSET {
			p.advance()
			p.advance()
			returnExpression = p.parseExpression()
			p.expectStatementTerminator("#set expression")
			if p.peek(0).Kind != TkRBRACE {
				p.err("#set must be the final statement in a macro", "Move statements before #set", nil)
			}
			break
		}
		body = append(body, p.parseStatement())
	}
	p.expect(TkRBRACE, "Expected '}' after macro body")
	if returnExpression == nil {
		p.err("Macro body must end with #set", "Add '#set <expression>' as the final macro statement", location)
	}
	return &MacroDirective{
		StatementBase: StatementBase{Location: location},
		Name:          name,
		Params:        params,
		ReturnType:    returnType,
		Body:          body,
		Return:        returnExpression,
	}
}

// parseEnumDefinition parses enum definition from the supplied input.
func (p *Parser) parseEnumDefinition(location *SourceLocation) *EnumDefinition {
	var valueType TypeRef
	if p.peek(0).Kind == TkLPAREN {
		p.advance()
		valueType = p.parseTypeRef()
		p.expect(TkRPAREN, "Expected ')' after enum value type")
	}
	p.expect(TkEQUALS, "Expected '=' after enum declaration")
	p.expect(TkLBRACE, "Expected '{' for enum members")
	members := []*EnumMember{}
	for p.peek(0).Kind != TkRBRACE {
		memberToken := p.expect(TkIDENTIFIER, "Expected enum member name")
		member := &EnumMember{Name: memberToken.StrValue(), Location: p.location(memberToken)}
		if valueType != nil {
			p.expect(TkEQUALS, "Expected '=' after tagged enum member")
			member.Value = p.parseExpression()
		}
		members = append(members, member)
		p.expectStatementTerminator("enum member")
	}
	p.expect(TkRBRACE, "Expected '}' after enum members")
	return &EnumDefinition{ValueType: valueType, Members: members, Location: location}
}

// parseSetupDefinition parses setup definition from the supplied input.
func (p *Parser) parseSetupDefinition(location *SourceLocation) *SetupDefinition {
	p.expect(TkEQUALS, "Expected '=' after setup declaration")
	p.expect(TkLBRACE, "Expected '{' for setup fields")
	fields := []*FieldDefinition{}
	for p.peek(0).Kind != TkRBRACE {
		fields = append(fields, p.parseFieldDefinition())
		p.expectStatementTerminator("setup field")
	}
	p.expect(TkRBRACE, "Expected '}' after setup fields")
	return &SetupDefinition{Fields: fields, Location: location}
}

// parseFieldDefinition parses field definition from the supplied input.
func (p *Parser) parseFieldDefinition() *FieldDefinition {
	nameToken := p.expect(TkIDENTIFIER, "Expected setup field name")
	field := &FieldDefinition{Name: nameToken.StrValue(), Location: p.location(nameToken)}
	if p.peek(0).Kind == TkOPTIONAL {
		field.Optional = true
		p.advance()
	}
	p.expect(TkCOLON, "Expected ':' after setup field name")
	if p.peek(0).Kind == TkEQUALS {
		if !field.Optional {
			p.err("Inferred setup fields must be optional", "Use 'field?:= value' or declare an explicit field type", field.Location)
		}
		p.advance()
		field.InferType = true
		field.DefaultValue = p.parseExpression()
		return field
	}
	field.DeclaredType = p.parseTypeRef()
	if p.peek(0).Kind == TkEQUALS {
		p.advance()
		field.DefaultValue = p.parseExpression()
	}
	if field.Optional && field.DefaultValue == nil {
		p.err("Optional field '"+field.Name+"' must have a default value", "Add '= <value>' after the field type", field.Location)
	}
	return field
}

// parseScopeExpression parses scope expression from the supplied input.
func (p *Parser) parseScopeExpression() *ScopeExpression {
	open := p.expect(TkLBRACE, "Expected '{' for scope value")
	statements := p.parseStatements(TkRBRACE)
	p.expect(TkRBRACE, "Expected '}' after scope value")
	return &ScopeExpression{Statements: statements, Location: p.location(open)}
}

// parseTypeRef parses type ref from the supplied input.
func (p *Parser) parseTypeRef() TypeRef {
	token := p.peek(0)
	switch token.Kind {
	case TkSTRING:
		p.advance()
		return PrimString
	case TkINTEGER:
		p.advance()
		return PrimInteger
	case TkBOOLEAN:
		p.advance()
		return PrimBoolean
	case TkFLOAT:
		p.advance()
		return PrimFloat
	case TkTIMESTAMP:
		p.advance()
		return PrimTimestamp
	case TkSCOPE:
		p.advance()
		return PrimScope
	case TkNAMESPACE:
		p.advance()
		return PrimNamespace
	case TkMAPPING:
		p.advance()
		p.expect(TkLPAREN, "Expected '(' after mapping type")
		keyType := p.parseTypeRef()
		p.expect(TkCOMMA, "Expected ',' between mapping type parameters")
		valueType := p.parseTypeRef()
		p.expect(TkRPAREN, "Expected ')' after mapping type parameters")
		return &MappingType{KeyType: keyType, ValueType: valueType}
	case TkLBRACKET:
		p.advance()
		if p.peek(0).Kind == TkRBRACKET {
			p.advance()
			return &ArrayType{ElementType: p.parseTypeRef()}
		}
		index := p.peek(0)
		p.advance()
		p.expect(TkRBRACKET, "Expected ']' after array index type")
		elementType := p.parseTypeRef()
		if index.Kind == TkINTEGER {
			return &ArrayType{ElementType: elementType}
		}
		if index.Kind != TkIDENTIFIER {
			p.err("Array index type must be integer or an enum", "Use '[]type', '[integer]type', or '[enum_name]type'", p.location(index))
		}
		return &EnumKeyType{EnumName: index.StrValue(), ElementType: elementType}
	case TkIDENTIFIER:
		p.advance()
		return TypeName(token.StrValue())
	default:
		p.err("Unexpected token in type: "+token.Kind.String(), "Expected a built-in or previously declared type", p.location(token))
	}
	return nil
}

// parseExpression parses expression from the supplied input.
func (p *Parser) parseExpression() Expression {
	return p.parseExpressionPrecedence(precLowest)
}

// parseExpressionPrecedence parses expression precedence from the supplied input.
func (p *Parser) parseExpressionPrecedence(minPrec int) Expression {
	left := p.parsePrefix()
	if left == nil {
		return nil
	}

	for p.peekPrecedence() > minPrec {
		op, ok := p.infixBinding()
		if !ok {
			break
		}
		left = p.parseInfix(left, op)
	}

	return left
}

// parsePrefix parses prefix from the supplied input.
func (p *Parser) parsePrefix() Expression {
	token := p.peek(0)
	location := p.location(token)

	// Handle #not (prefix unary)
	if token.Kind == TkHASH && p.peek(1).Kind == TkNOT {
		p.advance() // consume #
		p.advance() // consume not
		operand := p.parseExpressionPrecedence(precPrefix)
		return &UnaryOpExpression{Op: OpNot, Operand: operand, Location: location}
	}

	// Handle #match (call-like)
	if token.Kind == TkHASH && p.peek(1).Kind == TkMATCH {
		p.advance() // consume #
		p.advance() // consume match
		p.expect(TkLPAREN, "Expected '(' after #match")
		value := p.parseExpression()
		p.expect(TkCOMMA, "Expected ',' between #match arguments")
		pattern := p.parseExpression()
		p.expect(TkRPAREN, "Expected ')' after #match arguments")
		return &MatchExpression{Value: value, Pattern: pattern, Location: location}
	}

	// Handle #len (call-like)
	if token.Kind == TkHASH && p.peek(1).Kind == TkLEN {
		p.advance() // consume #
		p.advance() // consume len
		p.expect(TkLPAREN, "Expected '(' after #len")
		value := p.parseExpression()
		p.expect(TkRPAREN, "Expected ')' after #len arguments")
		return &LenExpression{Value: value, Location: location}
	}

	// Handle parenthesized grouping
	if token.Kind == TkLPAREN {
		p.advance()
		expr := p.parseExpression()
		p.expect(TkRPAREN, "Expected ')' after grouped expression")
		return expr
	}

	switch token.Kind {
	case TkINTEGER_LITERAL:
		p.advance()
		value, err := strconv.Atoi(strings.ReplaceAll(token.StrValue(), "_", ""))
		if err != nil {
			p.err("Invalid integer literal '"+token.StrValue()+"'", "Use an integer representable by this platform", location)
		}
		return &IntegerExpression{Value: value, Location: location}
	case TkFLOAT_LITERAL:
		p.advance()
		value, err := strconv.ParseFloat(strings.ReplaceAll(token.StrValue(), "_", ""), 64)
		if err != nil {
			p.err("Invalid float literal '"+token.StrValue()+"'", "Use a valid 64-bit float", location)
		}
		return &FloatExpression{Value: value, Location: location}
	case TkTRUE, TkFALSE:
		p.advance()
		return &BooleanExpression{Value: token.Kind == TkTRUE, Location: location}
	case TkSTRING_LITERAL:
		p.advance()
		return &StringExpression{Literal: token.Value.(*StringLiteral), Location: location}
	case TkMULTILINE_STRING:
		p.advance()
		parts := token.Value.([2]string)
		return &StringExpression{Multiline: processMultilineContent(parts[0], parts[1]), Location: location}
	case TkMAPPING:
		return p.parseMappingExpression()
	case TkHASH:
		p.advance()
		switch p.peek(0).Kind {
		case TkINCLUDE:
			p.advance()
			return &IncludeExpression{Filepath: p.parseIncludePath(), Location: location}
		case TkITERATION:
			p.advance()
			return &IterationExpression{Label: p.parseOptionalDirectiveArgument("#iteration"), Location: location}
		default:
			p.err("Directive is not valid as an expression", "Only #include, #iteration, #not, #match, and #len are expression directives", location)
		}
	case TkDOT:
		p.advance()
		member := p.expect(TkIDENTIFIER, "Expected enum member after '.'")
		accesses := []Access{&FieldAccess{Name: member.StrValue(), Location: p.location(member)}}
		accesses = append(accesses, p.parseAccesses()...)
		return &ReferenceExpression{Accesses: accesses, Location: location}
	case TkIDENTIFIER:
		p.advance()
		name := token.StrValue()
		if p.peek(0).Kind == TkBANG {
			p.advance()
			return p.parseMacroCall(name, location)
		}
		if p.peek(0).Kind == TkLBRACE {
			return p.parseStructExpression(name, location)
		}
		return &ReferenceExpression{Root: name, Accesses: p.parseAccesses(), Location: location}
	case TkLBRACE:
		if p.peek(1).Kind == TkLBRACKET {
			return p.parseMappingBody(nil)
		}
		return p.parseStructExpression("", location)
	case TkLBRACKET:
		if p.peek(1).Kind == TkRBRACKET && tokenStartsType(p.peek(2).Kind) {
			return p.parseTypedArrayExpression()
		}
		return p.parseArrayExpression()
	default:
		p.err("Unexpected value token: "+token.Kind.String(), "Expected a literal, reference, struct, array, mapping, or expression directive", location)
	}
	return nil
}

// parseInfix parses infix from the supplied input.
func (p *Parser) parseInfix(left Expression, op BinaryOpKind) Expression {
	location := p.location(p.peek(0))
	prec := p.peekPrecedence()

	// Consume the operator tokens
	if op == OpAnd || op == OpOr {
		p.advance() // consume #
		p.advance() // consume and/or
	} else {
		p.advance() // consume the operator token
	}

	right := p.parseExpressionPrecedence(prec)
	return &BinaryOpExpression{Left: left, Op: op, Right: right, Location: location}
}

// tokenStartsType reports whether a token can begin a SOMETHING type reference.
func tokenStartsType(kind TokenKind) bool {
	return isTypeToken(kind) || kind == TkIDENTIFIER || kind == TkMAPPING || kind == TkLBRACKET
}

// isTypeToken reports whether a token is a primitive type keyword.
func isTypeToken(kind TokenKind) bool {
	return typeTokens[kind]
}

// parseArrayExpression parses array expression from the supplied input.
func (p *Parser) parseArrayExpression() Expression {
	open := p.expect(TkLBRACKET, "Expected '['")
	elements := []Expression{}
	for p.peek(0).Kind != TkRBRACKET {
		elements = append(elements, p.parseExpression())
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRBRACKET {
			p.err("Expected ',' between array elements", "Separate array values with commas", nil)
		}
	}
	p.expect(TkRBRACKET, "Expected ']' after array elements")
	return &ArrayExpression{Elements: elements, Location: p.location(open)}
}

// parseTypedArrayExpression parses typed array expression from the supplied input.
func (p *Parser) parseTypedArrayExpression() Expression {
	open := p.expect(TkLBRACKET, "Expected '['")
	p.expect(TkRBRACKET, "Expected ']' in typed array literal")
	elementType := p.parseTypeRef()
	p.expect(TkLBRACE, "Expected '{' after typed array element type")
	elements := []Expression{}
	for p.peek(0).Kind != TkRBRACE {
		elements = append(elements, p.parseExpression())
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRBRACE {
			p.err("Expected ',' between typed array elements", "Separate array values with commas", nil)
		}
	}
	p.expect(TkRBRACE, "Expected '}' after typed array elements")
	return &ArrayExpression{DeclaredType: &ArrayType{ElementType: elementType}, Elements: elements, Location: p.location(open)}
}

// parseMappingExpression parses mapping expression from the supplied input.
func (p *Parser) parseMappingExpression() Expression {
	start := p.expect(TkMAPPING, "Expected mapping")
	var declaredType *MappingType
	if p.peek(0).Kind == TkLPAREN {
		p.advance()
		keyType := p.parseTypeRef()
		p.expect(TkCOMMA, "Expected ',' between mapping literal types")
		valueType := p.parseTypeRef()
		p.expect(TkRPAREN, "Expected ')' after mapping literal types")
		declaredType = &MappingType{KeyType: keyType, ValueType: valueType}
	}
	return p.parseMappingBodyAt(declaredType, p.location(start))
}

// parseMappingBody parses mapping body from the supplied input.
func (p *Parser) parseMappingBody(declaredType *MappingType) Expression {
	return p.parseMappingBodyAt(declaredType, p.location(p.peek(0)))
}

// parseMappingBodyAt parses mapping body at from the supplied input.
func (p *Parser) parseMappingBodyAt(declaredType *MappingType, location *SourceLocation) Expression {
	p.expect(TkLBRACE, "Expected '{' for mapping literal")
	entries := []*MappingEntry{}
	for p.peek(0).Kind != TkRBRACE {
		entryLocation := p.location(p.peek(0))
		p.expect(TkLBRACKET, "Expected '[' before mapping key")
		composite := p.peek(0).Kind == TkLBRACKET
		if composite {
			p.advance()
		}
		keys := []Expression{}
		for p.peek(0).Kind != TkRBRACKET {
			keys = append(keys, p.parseExpression())
			if p.peek(0).Kind == TkCOMMA {
				p.advance()
			} else if p.peek(0).Kind != TkRBRACKET {
				p.err("Expected ',' between mapping keys", "Separate composite keys with commas", nil)
			}
		}
		p.expect(TkRBRACKET, "Expected ']' after mapping key")
		if composite {
			p.expect(TkRBRACKET, "Expected second ']' after composite mapping key")
		}
		if len(keys) == 0 {
			p.err("Mapping keys cannot be empty", "Provide at least one key", entryLocation)
		}
		p.expect(TkARROW, "Expected '=>' after mapping key")
		entries = append(entries, &MappingEntry{Keys: keys, Value: p.parseExpression(), IsComposite: composite, Location: entryLocation})
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRBRACE {
			p.err("Expected ',' between mapping entries", "Separate mapping entries with commas", nil)
		}
	}
	p.expect(TkRBRACE, "Expected '}' after mapping entries")
	return &MappingExpression{DeclaredType: declaredType, Entries: entries, Location: location}
}

// parseStructExpression parses struct expression from the supplied input.
func (p *Parser) parseStructExpression(typeName string, location *SourceLocation) Expression {
	p.expect(TkLBRACE, "Expected '{' for struct literal")
	fields := []*FieldAssignment{}
	for p.peek(0).Kind != TkRBRACE {
		nameToken := p.expect(TkIDENTIFIER, "Expected struct field name")
		p.expect(TkEQUALS, "Expected '=' after struct field name")
		fields = append(fields, &FieldAssignment{Name: nameToken.StrValue(), Value: p.parseExpression(), Location: p.location(nameToken)})
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRBRACE {
			p.err("Expected ',' between struct fields", "Separate struct fields with commas", nil)
		}
	}
	p.expect(TkRBRACE, "Expected '}' after struct fields")
	return &StructExpression{TypeName: typeName, Fields: fields, Location: location}
}

// parseMacroCall parses macro call from the supplied input.
func (p *Parser) parseMacroCall(name string, location *SourceLocation) Expression {
	p.expect(TkLPAREN, "Expected '(' after macro name + '!'")
	arguments := []Expression{}
	for p.peek(0).Kind != TkRPAREN {
		arguments = append(arguments, p.parseExpression())
		if p.peek(0).Kind == TkCOMMA {
			p.advance()
		} else if p.peek(0).Kind != TkRPAREN {
			p.err("Expected ',' between macro arguments", "Separate macro arguments with commas", nil)
		}
	}
	p.expect(TkRPAREN, "Expected ')' after macro arguments")
	return &MacroCallExpression{Name: name, Arguments: arguments, Location: location}
}

// processMultilineContent applies declared indentation, newline, and whitespace transformations to multiline text.
func processMultilineContent(raw, params string) string {
	lines := strings.Split(raw, "\n")
	if strings.Contains(params, "no_indent") {
		minimum := -1
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if minimum == -1 || indent < minimum {
				minimum = indent
			}
		}
		if minimum > 0 {
			for index, line := range lines {
				if len(line) >= minimum {
					lines[index] = line[minimum:]
				}
			}
		}
		raw = strings.Join(lines, "\n")
	}
	if strings.Contains(params, "no_newline") {
		raw = strings.ReplaceAll(raw, "\n", " ")
	}
	if strings.Contains(params, "strip_spaces") {
		raw = strings.Join(strings.Fields(raw), " ")
	}
	return strings.TrimSpace(raw)
}

// stringLiteralToString reconstructs a string token while preserving interpolation placeholders.
func stringLiteralToString(token Token) string {
	literal, ok := token.Value.(*StringLiteral)
	if !ok {
		return token.StrValue()
	}
	var result strings.Builder
	for _, part := range literal.Parts {
		switch value := part.(type) {
		case StringText:
			result.WriteString(string(value))
		case *InterpolationRef:
			result.WriteString("{")
			result.WriteString(value.Name)
			result.WriteString("}")
		}
	}
	return result.String()
}

// typeRefString returns the canonical diagnostic representation of a type reference.
func typeRefString(ref TypeRef) string {
	switch value := ref.(type) {
	case PrimitiveKind:
		return value.String()
	case TypeName:
		return string(value)
	case *ArrayType:
		return "[]" + typeRefString(value.ElementType)
	case *EnumKeyType:
		return "[" + value.EnumName + "]" + typeRefString(value.ElementType)
	case *MappingType:
		return "mapping(" + typeRefString(value.KeyType) + ", " + typeRefString(value.ValueType) + ")"
	case *EnumType:
		return value.Name
	case *SetupType:
		return value.Name
	case *ScopeType:
		return "scope"
	case *NamespaceType:
		return "namespace"
	default:
		return fmt.Sprintf("%T", ref)
	}
}
