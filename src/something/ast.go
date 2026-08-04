// ast.go defines the ordered syntax tree produced by parsing and transformed
// by directive generation before semantic checking.
package something

// PrimitiveKind represents a built-in value type.
type PrimitiveKind int

const (
	PrimString PrimitiveKind = iota
	PrimInteger
	PrimBoolean
	PrimFloat
	PrimTimestamp
	PrimScope
	PrimNamespace
)

// String returns the receiver's textual representation.
func (k PrimitiveKind) String() string {
	switch k {
	case PrimString:
		return "string"
	case PrimInteger:
		return "integer"
	case PrimBoolean:
		return "boolean"
	case PrimFloat:
		return "float"
	case PrimTimestamp:
		return "timestamp"
	case PrimScope:
		return "scope"
	case PrimNamespace:
		return "namespace"
	default:
		return "?"
	}
}

// TypeRef is a syntactic or resolved SOMETHING type.
type TypeRef interface {
	typeRefMarker()
}

// typeRefMarker marks PrimitiveKind as a TypeRef implementation.
func (PrimitiveKind) typeRefMarker() {}

// typeRefMarker marks TypeName as a TypeRef implementation.
func (TypeName) typeRefMarker() {}

// typeRefMarker marks MappingType as a TypeRef implementation.
func (*MappingType) typeRefMarker() {}

// typeRefMarker marks ArrayType as a TypeRef implementation.
func (*ArrayType) typeRefMarker() {}

// typeRefMarker marks EnumKeyType as a TypeRef implementation.
func (*EnumKeyType) typeRefMarker() {}

// TypeName is an unresolved named type reference.
type TypeName string

// MappingType defines mapping key and value types.
type MappingType struct {
	KeyType   TypeRef
	ValueType TypeRef
}

// ArrayType defines an integer-indexed array.
type ArrayType struct {
	ElementType TypeRef
}

// EnumKeyType defines an array indexed by members of a named enum.
type EnumKeyType struct {
	EnumName    string
	ElementType TypeRef
}

// Program is an ordered syntax or expanded AST. Statement order is semantic.
type Program struct {
	Statements []Statement
	Filepath   string
}

// Statement is a source-ordered AST statement.
type Statement interface {
	statementMarker()
	statementBase() *StatementBase
}

// StatementBase contains metadata shared by every statement.
type StatementBase struct {
	Private  bool
	Location *SourceLocation
}

// AssignmentMode distinguishes declaration, inference, and reassignment.
type AssignmentMode int

const (
	AssignExplicit AssignmentMode = iota
	AssignInferred
	AssignExisting
)

// Assignment is the common representation for every value or type binding.
// Directives may appear in its target or value before directive expansion.
type Assignment struct {
	StatementBase
	Target       LValue
	Mode         AssignmentMode
	DeclaredType TypeRef
	Value        AssignmentValue
}

// statementMarker marks Assignment as a Statement implementation.
func (*Assignment) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *Assignment) statementBase() *StatementBase { return &n.StatementBase }

// LValue is a syntactic assignment destination.
type LValue interface {
	lvalueMarker()
}

// IdentifierLValue names a binding in the current scope.
type IdentifierLValue struct {
	Name string
}

// MemberLValue identifies a field or indexed member below a root binding.
type MemberLValue struct {
	Root     string
	Accesses []Access
}

// IterationLValue is replaced with an IdentifierLValue during expansion.
type IterationLValue struct {
	Label Expression
}

// AsLValue is replaced with an identifier or member target during expansion.
type AsLValue struct {
	Name Expression
}

// lvalueMarker marks IdentifierLValue as an LValue implementation.
func (*IdentifierLValue) lvalueMarker() {}

// lvalueMarker marks MemberLValue as an LValue implementation.
func (*MemberLValue) lvalueMarker() {}

// lvalueMarker marks IterationLValue as an LValue implementation.
func (*IterationLValue) lvalueMarker() {}

// lvalueMarker marks AsLValue as an LValue implementation.
func (*AsLValue) lvalueMarker() {}

// Access is one field or index operation in a reference or lvalue.
type Access interface {
	accessMarker()
}

// FieldAccess represents a field access AST node.
type FieldAccess struct {
	Name     string
	Location *SourceLocation
}

// IndexAccess represents an index access AST node.
type IndexAccess struct {
	Index    Expression
	Location *SourceLocation
}

// accessMarker marks FieldAccess as an Access implementation.
func (*FieldAccess) accessMarker() {}

// accessMarker marks IndexAccess as an Access implementation.
func (*IndexAccess) accessMarker() {}

// AssignmentValue is an expression, scope, or type definition on an assignment.
type AssignmentValue interface {
	assignmentValueMarker()
}

// Expression is an unevaluated value expression.
type Expression interface {
	AssignmentValue
	expressionMarker()
	expressionLocation() *SourceLocation
}

// StringExpression represents a string expression AST node.
type StringExpression struct {
	Literal   *StringLiteral
	Multiline string
	Location  *SourceLocation
}

// IntegerExpression represents an integer expression AST node.
type IntegerExpression struct {
	Value    int
	Location *SourceLocation
}

// FloatExpression represents a float expression AST node.
type FloatExpression struct {
	Value    float64
	Location *SourceLocation
}

// BooleanExpression represents a boolean expression AST node.
type BooleanExpression struct {
	Value    bool
	Location *SourceLocation
}

// ReferenceExpression represents a reference expression AST node.
type ReferenceExpression struct {
	Root     string
	Accesses []Access
	Location *SourceLocation
}

// ArrayExpression represents an array expression AST node.
type ArrayExpression struct {
	DeclaredType TypeRef
	Elements     []Expression
	Location     *SourceLocation
}

// MappingExpression represents a mapping expression AST node.
type MappingExpression struct {
	DeclaredType *MappingType
	Entries      []*MappingEntry
	Location     *SourceLocation
}

// StructExpression represents a struct expression AST node.
type StructExpression struct {
	TypeName string
	Fields   []*FieldAssignment
	Location *SourceLocation
}

// IncludeExpression represents an include expression AST node.
type IncludeExpression struct {
	Filepath string
	Location *SourceLocation
}

// IterationExpression represents an iteration expression AST node.
type IterationExpression struct {
	Label    Expression
	Location *SourceLocation
}

// MacroCallExpression represents a macro call expression AST node.
type MacroCallExpression struct {
	Name      string
	Arguments []Expression
	Location  *SourceLocation
}

// NamespaceExpression represents a namespace expression AST node.
type NamespaceExpression struct {
	Statements []Statement
	Location   *SourceLocation
}

// TypedExpression preserves a directive's checked result type after its
// compile-time value has been converted back into an AST expression.
type TypedExpression struct {
	Value    Expression
	Type     TypeRef
	Location *SourceLocation
}

// assignmentValueMarker marks StringExpression as an AssignmentValue implementation.
func (*StringExpression) assignmentValueMarker() {}

// assignmentValueMarker marks IntegerExpression as an AssignmentValue implementation.
func (*IntegerExpression) assignmentValueMarker() {}

// assignmentValueMarker marks FloatExpression as an AssignmentValue implementation.
func (*FloatExpression) assignmentValueMarker() {}

// assignmentValueMarker marks BooleanExpression as an AssignmentValue implementation.
func (*BooleanExpression) assignmentValueMarker() {}

// assignmentValueMarker marks ReferenceExpression as an AssignmentValue implementation.
func (*ReferenceExpression) assignmentValueMarker() {}

// assignmentValueMarker marks ArrayExpression as an AssignmentValue implementation.
func (*ArrayExpression) assignmentValueMarker() {}

// assignmentValueMarker marks MappingExpression as an AssignmentValue implementation.
func (*MappingExpression) assignmentValueMarker() {}

// assignmentValueMarker marks StructExpression as an AssignmentValue implementation.
func (*StructExpression) assignmentValueMarker() {}

// assignmentValueMarker marks IncludeExpression as an AssignmentValue implementation.
func (*IncludeExpression) assignmentValueMarker() {}

// assignmentValueMarker marks IterationExpression as an AssignmentValue implementation.
func (*IterationExpression) assignmentValueMarker() {}

// assignmentValueMarker marks MacroCallExpression as an AssignmentValue implementation.
func (*MacroCallExpression) assignmentValueMarker() {}

// assignmentValueMarker marks NamespaceExpression as an AssignmentValue implementation.
func (*NamespaceExpression) assignmentValueMarker() {}

// assignmentValueMarker marks TypedExpression as an AssignmentValue implementation.
func (*TypedExpression) assignmentValueMarker() {}

// expressionMarker marks StringExpression as an Expression implementation.
func (*StringExpression) expressionMarker() {}

// expressionMarker marks IntegerExpression as an Expression implementation.
func (*IntegerExpression) expressionMarker() {}

// expressionMarker marks FloatExpression as an Expression implementation.
func (*FloatExpression) expressionMarker() {}

// expressionMarker marks BooleanExpression as an Expression implementation.
func (*BooleanExpression) expressionMarker() {}

// expressionMarker marks ReferenceExpression as an Expression implementation.
func (*ReferenceExpression) expressionMarker() {}

// expressionMarker marks ArrayExpression as an Expression implementation.
func (*ArrayExpression) expressionMarker() {}

// expressionMarker marks MappingExpression as an Expression implementation.
func (*MappingExpression) expressionMarker() {}

// expressionMarker marks StructExpression as an Expression implementation.
func (*StructExpression) expressionMarker() {}

// expressionMarker marks IncludeExpression as an Expression implementation.
func (*IncludeExpression) expressionMarker() {}

// expressionMarker marks IterationExpression as an Expression implementation.
func (*IterationExpression) expressionMarker() {}

// expressionMarker marks MacroCallExpression as an Expression implementation.
func (*MacroCallExpression) expressionMarker() {}

// expressionMarker marks NamespaceExpression as an Expression implementation.
func (*NamespaceExpression) expressionMarker() {}

// expressionMarker marks TypedExpression as an Expression implementation.
func (*TypedExpression) expressionMarker() {}

// expressionLocation returns the receiver's source location.
func (n *StringExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *IntegerExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *FloatExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *BooleanExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *ReferenceExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *ArrayExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *MappingExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *StructExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *IncludeExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *IterationExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *MacroCallExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *NamespaceExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *TypedExpression) expressionLocation() *SourceLocation { return n.Location }

// MappingEntry is one mapping literal entry. Composite keys have multiple keys.
type MappingEntry struct {
	Keys        []Expression
	Value       Expression
	IsComposite bool
	Location    *SourceLocation
}

// FieldAssignment is one field initializer in a struct literal.
type FieldAssignment struct {
	Name     string
	Value    Expression
	Location *SourceLocation
}

// ScopeExpression is a block whose declarations form a scope value.
type ScopeExpression struct {
	Statements []Statement
	Location   *SourceLocation
}

// assignmentValueMarker marks ScopeExpression as an AssignmentValue implementation.
func (*ScopeExpression) assignmentValueMarker() {}

// FieldDefinition is one field in a setup definition.
type FieldDefinition struct {
	Name         string
	DeclaredType TypeRef
	InferType    bool
	Optional     bool
	DefaultValue Expression
	Location     *SourceLocation
}

// SetupDefinition is the right-hand side of a `name: setup = { ... }` assignment.
type SetupDefinition struct {
	Fields   []*FieldDefinition
	Location *SourceLocation
}

// assignmentValueMarker marks SetupDefinition as an AssignmentValue implementation.
func (*SetupDefinition) assignmentValueMarker() {}

// EnumMember is one ordered member of an enum definition.
type EnumMember struct {
	Name     string
	Value    Expression
	Location *SourceLocation
}

// EnumDefinition is the right-hand side of a `name: enum = { ... }` assignment.
type EnumDefinition struct {
	ValueType TypeRef
	Members   []*EnumMember
	Location  *SourceLocation
}

// assignmentValueMarker marks EnumDefinition as an AssignmentValue implementation.
func (*EnumDefinition) assignmentValueMarker() {}

// IncludeDirective inserts another file's statements at its source position.
type IncludeDirective struct {
	StatementBase
	Filepath string
}

// statementMarker marks IncludeDirective as a Statement implementation.
func (*IncludeDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *IncludeDirective) statementBase() *StatementBase { return &n.StatementBase }

// ForDirective expands its body once for each source element.
type ForDirective struct {
	StatementBase
	ElementName string
	KeyName     string
	Source      Expression
	Body        []Statement
}

// statementMarker marks ForDirective as a Statement implementation.
func (*ForDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *ForDirective) statementBase() *StatementBase { return &n.StatementBase }

// InsertDirective parses evaluated strings as statements at its source position.
type InsertDirective struct {
	StatementBase
	Contents []Expression
}

// statementMarker marks InsertDirective as a Statement implementation.
func (*InsertDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *InsertDirective) statementBase() *StatementBase { return &n.StatementBase }

// MacroParam is one typed macro input.
type MacroParam struct {
	Name     string
	Type     TypeRef
	Location *SourceLocation
}

// MacroDirective is a compile-time expression template. It is removed by expansion.
type MacroDirective struct {
	StatementBase
	Name       string
	Params     []MacroParam
	ReturnType TypeRef
	Body       []Statement
	Return     Expression
}

// statementMarker marks MacroDirective as a Statement implementation.
func (*MacroDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *MacroDirective) statementBase() *StatementBase { return &n.StatementBase }

// AssertDirective validates a setup type definition with a body of
// assertion statements evaluated in a scope that inherits the type's fields.
type AssertDirective struct {
	StatementBase
	TypeName string
	Body     []Statement
}

// statementMarker marks AssertDirective as a Statement implementation.
func (*AssertDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *AssertDirective) statementBase() *StatementBase { return &n.StatementBase }

// IfDirective conditionally evaluates its body when the condition is true.
// Body is always non-nil, either from the block form (#if cond { ... })
// or the single-statement form (#if cond stmt;).
type IfDirective struct {
	StatementBase
	Condition Expression
	Body      []Statement
}

// statementMarker marks IfDirective as a Statement implementation.
func (*IfDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *IfDirective) statementBase() *StatementBase { return &n.StatementBase }

// ErrorDirective terminates evaluation with a user-defined error message.
type ErrorDirective struct {
	StatementBase
	Message Expression // string expression, may use interpolation
}

// statementMarker marks ErrorDirective as a Statement implementation.
func (*ErrorDirective) statementMarker() {}

// statementBase returns the receiver's shared statement metadata.
func (n *ErrorDirective) statementBase() *StatementBase { return &n.StatementBase }

// BinaryOpKind identifies a binary comparison or logical operator.
type BinaryOpKind int

const (
	OpEQ  BinaryOpKind = iota // ==
	OpNEQ                     // !=
	OpLT                      // <
	OpLE                      // <=
	OpGT                      // >
	OpGE                      // >=
	OpAnd                     // #and
	OpOr                      // #or
)

// BinaryOpExpression is a binary comparison or logical expression.
type BinaryOpExpression struct {
	Left     Expression
	Op       BinaryOpKind
	Right    Expression
	Location *SourceLocation
}

// UnaryOpKind identifies a unary operator.
type UnaryOpKind int

const (
	OpNot UnaryOpKind = iota // #not
)

// UnaryOpExpression is a unary expression (e.g., #not).
type UnaryOpExpression struct {
	Op       UnaryOpKind
	Operand  Expression
	Location *SourceLocation
}

// MatchExpression evaluates a regex match.
type MatchExpression struct {
	Value    Expression
	Pattern  Expression
	Location *SourceLocation
}

// LenExpression evaluates the length of an array or mapping.
type LenExpression struct {
	Value    Expression
	Location *SourceLocation
}

// assignmentValueMarker marks BinaryOpExpression as an AssignmentValue implementation.
func (*BinaryOpExpression) assignmentValueMarker() {}

// assignmentValueMarker marks UnaryOpExpression as an AssignmentValue implementation.
func (*UnaryOpExpression) assignmentValueMarker() {}

// assignmentValueMarker marks MatchExpression as an AssignmentValue implementation.
func (*MatchExpression) assignmentValueMarker() {}

// assignmentValueMarker marks LenExpression as an AssignmentValue implementation.
func (*LenExpression) assignmentValueMarker() {}

// expressionMarker marks BinaryOpExpression as an Expression implementation.
func (*BinaryOpExpression) expressionMarker() {}

// expressionMarker marks UnaryOpExpression as an Expression implementation.
func (*UnaryOpExpression) expressionMarker() {}

// expressionMarker marks MatchExpression as an Expression implementation.
func (*MatchExpression) expressionMarker() {}

// expressionMarker marks LenExpression as an Expression implementation.
func (*LenExpression) expressionMarker() {}

// expressionLocation returns the receiver's source location.
func (n *BinaryOpExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *UnaryOpExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *MatchExpression) expressionLocation() *SourceLocation { return n.Location }

// expressionLocation returns the receiver's source location.
func (n *LenExpression) expressionLocation() *SourceLocation { return n.Location }
