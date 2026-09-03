// typechecker.go performs the standalone semantic pass over an expanded AST.
// It resolves types and names in source order and does not evaluate the program.
package something

import (
	"fmt"
	"sort"
	"strings"
)

// BindingType is the checked type and visibility of a scope member.
type BindingType struct {
	Type    TypeRef
	Private bool
}

// ScopeType is the checked structural type of a scope value.
type ScopeType struct {
	Fields map[string]*BindingType
}

// NamespaceType is the checked structural type of an included namespace.
type NamespaceType struct {
	Fields map[string]*BindingType
	Types  map[string]TypeRef
}

// EnumType is a resolved enum definition.
type EnumType struct {
	Name       string
	ValueType  TypeRef
	Members    map[string]Expression
	MemberList []string
}

// SetupType is a resolved setup definition.
type SetupType struct {
	Name   string
	Fields map[string]*FieldDefinition
}

// typeRefMarker marks EnumType as a TypeRef implementation.
func (*EnumType) typeRefMarker() {}

// typeRefMarker marks SetupType as a TypeRef implementation.
func (*SetupType) typeRefMarker() {}

// typeRefMarker marks ScopeType as a TypeRef implementation.
func (*ScopeType) typeRefMarker() {}

// typeRefMarker marks NamespaceType as a TypeRef implementation.
func (*NamespaceType) typeRefMarker() {}

// CheckedProgram is the result of the standalone type-checking phase.
type CheckedProgram struct {
	Program         *Program
	AssignmentTypes map[*Assignment]TypeRef
}

// staticBinding records the inferred or declared type and assignment state of one name.
type staticBinding struct {
	typeRef TypeRef
	private bool
}

// staticEnvironment stores lexical type bindings and named type definitions.
type staticEnvironment struct {
	parent   *staticEnvironment
	bindings map[string]*staticBinding
	types    map[string]TypeRef
}

// newStaticEnvironment creates an empty lexical type-checking scope linked to its parent.
func newStaticEnvironment(parent *staticEnvironment) *staticEnvironment {
	return &staticEnvironment{
		parent:   parent,
		bindings: make(map[string]*staticBinding),
		types:    make(map[string]TypeRef),
	}
}

// lookupBinding searches the current and enclosing static scopes for a binding.
func (environment *staticEnvironment) lookupBinding(name string) (*staticBinding, bool) {
	for current := environment; current != nil; current = current.parent {
		if binding, ok := current.bindings[name]; ok {
			return binding, true
		}
	}
	return nil, false
}

// lookupType searches the current and enclosing static scopes for a named type.
func (environment *staticEnvironment) lookupType(name string) (TypeRef, bool) {
	for current := environment; current != nil; current = current.parent {
		if typeRef, ok := current.types[name]; ok {
			return typeRef, true
		}
	}
	return nil, false
}

// TypeChecker resolves and validates one fully expanded program.
type TypeChecker struct {
	program         *Program
	filepath        string
	current         *staticEnvironment
	assignmentTypes map[*Assignment]TypeRef
}

// NewTypeChecker constructs type checker.
func NewTypeChecker(program *Program, filepath string) *TypeChecker {
	if filepath == "" && program != nil {
		filepath = program.Filepath
	}
	return &TypeChecker{program: program, filepath: filepath, current: newStaticEnvironment(nil), assignmentTypes: make(map[*Assignment]TypeRef)}
}

// err panics with a source-located SomethingError for a semantic failure.
func (checker *TypeChecker) err(message string, location *SourceLocation, suggestion string) {
	panic(errLoc(message, location, checker.filepath, suggestion))
}

// Check returns semantic annotations used by evaluation.
func (checker *TypeChecker) Check() *CheckedProgram {
	checker.detectDependencyCycles()
	checker.checkStatements(checker.program.Statements)
	return &CheckedProgram{Program: checker.program, AssignmentTypes: checker.assignmentTypes}
}

// checkStatements checks statements against the current invariants.
func (checker *TypeChecker) checkStatements(statements []Statement) {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *Assignment:
			checker.checkAssignment(node)
		case *AssertDirective:
			checker.checkAssertDirective(node)
		case *IfDirective:
			checker.checkIfDirective(node)
		case *ErrorDirective:
			checker.checkErrorDirective(node)
		default:
			checker.err("Directive generation did not remove "+fmt.Sprintf("%T", statement), statement.statementBase().Location, "Expand every directive before type checking")
		}
	}
}

// checkAssignment checks assignment against the current invariants.
func (checker *TypeChecker) checkAssignment(assignment *Assignment) {
	switch value := assignment.Value.(type) {
	case *EnumDefinition:
		checker.checkEnumDefinition(assignment, value)
		return
	case *SetupDefinition:
		checker.checkSetupDefinition(assignment, value)
		return
	case *ScopeExpression:
		checker.checkScopeAssignment(assignment, value, false)
		return
	}

	expression := assignment.Value.(Expression)
	if namespace, ok := expression.(*NamespaceExpression); ok {
		checker.checkNamespaceAssignment(assignment, namespace)
		return
	}

	switch assignment.Mode {
	case AssignExisting:
		targetType := checker.resolveExistingTarget(assignment.Target, assignment.Location)
		actualType := checker.expressionType(expression, targetType)
		checker.requireAssignable(targetType, actualType, expression.expressionLocation(), "reassignment")
		checker.assignmentTypes[assignment] = targetType
	case AssignExplicit:
		expectedType := checker.resolveType(assignment.DeclaredType, assignment.Location)
		actualType := checker.expressionType(expression, expectedType)
		checker.requireAssignable(expectedType, actualType, expression.expressionLocation(), "assignment")
		checker.declareTarget(assignment, expectedType)
		checker.assignmentTypes[assignment] = expectedType
	case AssignInferred:
		actualType := checker.expressionType(expression, nil)
		if actualType == nil {
			checker.err("Could not infer assignment type", expression.expressionLocation(), "Add an explicit type annotation")
		}
		checker.declareTarget(assignment, actualType)
		checker.assignmentTypes[assignment] = actualType
	}
}

// checkEnumDefinition checks enum definition against the current invariants.
func (checker *TypeChecker) checkEnumDefinition(assignment *Assignment, definition *EnumDefinition) {
	name := checker.requireTypeTarget(assignment)
	checker.ensureTypeNameAvailable(name, assignment.Location)
	valueType := checker.resolveType(definition.ValueType, definition.Location)
	enumType := &EnumType{Name: name, ValueType: valueType, Members: make(map[string]Expression)}
	for _, member := range definition.Members {
		if _, exists := enumType.Members[member.Name]; exists {
			checker.err("Duplicate enum member '"+member.Name+"'", member.Location, "Enum member names must be unique")
		}
		if valueType != nil {
			actual := checker.expressionType(member.Value, valueType)
			checker.requireAssignable(valueType, actual, member.Location, "tagged enum member")
		}
		enumType.Members[member.Name] = member.Value
		enumType.MemberList = append(enumType.MemberList, member.Name)
	}
	checker.current.types[name] = enumType
}

// checkAssertDirective checks assert directive against the current invariants.
func (checker *TypeChecker) checkAssertDirective(assertion *AssertDirective) {
	typeRef, ok := checker.current.lookupType(assertion.TypeName)
	if !ok {
		checker.err("Undefined assertion target type '"+assertion.TypeName+"'",
			assertion.Location, "Assertions must reference an existing setup type name")
	}
	setupType, ok := typeRef.(*SetupType)
	if !ok {
		checker.err("Assertion target '"+assertion.TypeName+"' is not a setup type",
			assertion.Location, "Only setup types can be asserted")
	}
	child := newStaticEnvironment(checker.current)
	for _, field := range setupType.Fields {
		child.bindings[field.Name] = &staticBinding{typeRef: field.DeclaredType, private: false}
	}
	previous := checker.current
	checker.current = child
	checker.checkStatements(assertion.Body)
	checker.current = previous
}

// checkIfDirective checks if directive against the current invariants.
func (checker *TypeChecker) checkIfDirective(ifDir *IfDirective) {
	condType := checker.expressionType(ifDir.Condition, PrimBoolean)
	checker.requireAssignable(PrimBoolean, condType, ifDir.Condition.expressionLocation(), "#if condition")
	child := newStaticEnvironment(checker.current)
	previous := checker.current
	checker.current = child
	checker.checkStatements(ifDir.Body)
	checker.current = previous
}

// checkErrorDirective checks error directive against the current invariants.
func (checker *TypeChecker) checkErrorDirective(errDir *ErrorDirective) {
	msgType := checker.expressionType(errDir.Message, PrimString)
	checker.requireAssignable(PrimString, msgType, errDir.Message.expressionLocation(), "#error message")
}

// checkSetupDefinition checks setup definition against the current invariants.
func (checker *TypeChecker) checkSetupDefinition(assignment *Assignment, definition *SetupDefinition) {
	name := checker.requireTypeTarget(assignment)
	checker.ensureTypeNameAvailable(name, assignment.Location)
	setup := &SetupType{Name: name, Fields: make(map[string]*FieldDefinition)}
	for _, field := range definition.Fields {
		if _, exists := setup.Fields[field.Name]; exists {
			checker.err("Duplicate setup field '"+field.Name+"'", field.Location, "Setup field names must be unique")
		}
		copy := *field
		if copy.InferType {
			copy.DeclaredType = checker.expressionType(copy.DefaultValue, nil)
			if copy.DeclaredType == nil {
				checker.err("Could not infer type of setup field '"+copy.Name+"'", copy.Location, "Declare the field type explicitly")
			}
		} else {
			copy.DeclaredType = checker.resolveType(copy.DeclaredType, copy.Location)
		}
		if copy.DefaultValue != nil {
			actual := checker.expressionType(copy.DefaultValue, copy.DeclaredType)
			checker.requireAssignable(copy.DeclaredType, actual, copy.Location, "setup field default")
		}
		setup.Fields[copy.Name] = &copy
	}
	checker.current.types[name] = setup
}

// checkScopeAssignment checks scope assignment against the current invariants.
func (checker *TypeChecker) checkScopeAssignment(assignment *Assignment, scope *ScopeExpression, namespace bool) {
	var objectType TypeRef
	var existingType TypeRef
	fields := make(map[string]*BindingType)
	if namespace {
		objectType = &NamespaceType{Fields: fields, Types: make(map[string]TypeRef)}
	} else {
		objectType = &ScopeType{Fields: fields}
	}
	if assignment.Mode == AssignExisting {
		existingType = checker.resolveExistingTarget(assignment.Target, assignment.Location)
		if namespace {
			if _, ok := existingType.(*NamespaceType); !ok && existingType != PrimNamespace {
				checker.err("Namespace reassignment requires a namespace target", assignment.Location, "Keep the destination's original type")
			}
		} else if _, ok := existingType.(*ScopeType); !ok && existingType != PrimScope {
			checker.err("Scope reassignment requires a scope target", assignment.Location, "Keep the destination's original type")
		}
	} else {
		checker.declareTarget(assignment, objectType)
	}

	child := newStaticEnvironment(checker.current)
	previous := checker.current
	checker.current = child
	for _, statement := range scope.Statements {
		assignment, ok := statement.(*Assignment)
		if !ok {
			checker.err("Directive generation did not remove "+fmt.Sprintf("%T", statement), statement.statementBase().Location, "Expand every directive before type checking")
		}
		checker.checkAssignment(assignment)
		for name, binding := range child.bindings {
			fields[name] = &BindingType{Type: binding.typeRef, Private: binding.private}
		}
	}
	checker.current = previous
	if namespaceType, ok := objectType.(*NamespaceType); ok {
		for name, typeRef := range child.types {
			namespaceType.Types[name] = typeRef
		}
	}
	if existingType != nil {
		checker.requireAssignable(existingType, objectType, assignment.Location, "reassignment")
		checker.assignmentTypes[assignment] = existingType
	} else {
		checker.assignmentTypes[assignment] = objectType
	}
}

// checkNamespaceAssignment checks namespace assignment against the current invariants.
func (checker *TypeChecker) checkNamespaceAssignment(assignment *Assignment, namespace *NamespaceExpression) {
	scope := &ScopeExpression{Statements: namespace.Statements, Location: namespace.Location}
	checker.checkScopeAssignment(assignment, scope, true)
}

// requireTypeTarget requires a valid type target value.
func (checker *TypeChecker) requireTypeTarget(assignment *Assignment) string {
	identifier, ok := assignment.Target.(*IdentifierLValue)
	if !ok || assignment.Mode != AssignExplicit {
		checker.err("Type definitions require a declared identifier", assignment.Location, "Use 'name: setup = ...' or 'name: enum = ...'")
	}
	return identifier.Name
}

// ensureTypeNameAvailable rejects duplicate or shadowed named type declarations.
func (checker *TypeChecker) ensureTypeNameAvailable(name string, location *SourceLocation) {
	if _, exists := checker.current.types[name]; exists {
		checker.err("Type '"+name+"' is already declared in this scope", location, "Use a unique type name")
	}
	if _, exists := checker.current.bindings[name]; exists {
		checker.err("Name '"+name+"' is already used by a value in this scope", location, "Type and value declarations cannot share a name")
	}
}

// declareTarget records a new assignment target and enforces declaration rules.
func (checker *TypeChecker) declareTarget(assignment *Assignment, typeRef TypeRef) {
	switch target := assignment.Target.(type) {
	case *IdentifierLValue:
		if _, exists := checker.current.bindings[target.Name]; exists {
			checker.err("Variable '"+target.Name+"' is already declared in this scope", assignment.Location, "Use '=' to reassign it")
		}
		if _, exists := checker.current.types[target.Name]; exists {
			checker.err("Name '"+target.Name+"' is already used by a type", assignment.Location, "Use a unique value name")
		}
		checker.current.bindings[target.Name] = &staticBinding{typeRef: typeRef, private: assignment.Private}
	case *MemberLValue:
		containerType, final := checker.resolveStaticContainer(target.Root, target.Accesses, assignment.Location)
		field, ok := final.(*FieldAccess)
		if !ok {
			checker.err("Indexed members cannot be declared", assignment.Location, "Declare the collection, then use '=' to reassign an existing index")
		}
		switch object := containerType.(type) {
		case *ScopeType:
			if _, exists := object.Fields[field.Name]; exists {
				checker.err("Member '"+field.Name+"' is already declared", field.Location, "Use '=' to reassign it")
			}
			object.Fields[field.Name] = &BindingType{Type: typeRef, Private: assignment.Private}
		case *NamespaceType:
			if _, exists := object.Fields[field.Name]; exists {
				checker.err("Member '"+field.Name+"' is already declared", field.Location, "Use '=' to reassign it")
			}
			object.Fields[field.Name] = &BindingType{Type: typeRef, Private: assignment.Private}
		default:
			checker.err("New members can only be declared in a scope or namespace", field.Location, "Setup, array, and mapping members have fixed declarations")
		}
	default:
		checker.err("Directive lvalue remained after expansion", assignment.Location, "Expand #iteration and #as_lvalue before type checking")
	}
}

// resolveExistingTarget resolves existing target from the supplied context.
func (checker *TypeChecker) resolveExistingTarget(target LValue, location *SourceLocation) TypeRef {
	switch target := target.(type) {
	case *IdentifierLValue:
		binding, ok := checker.current.lookupBinding(target.Name)
		if !ok {
			checker.err("Cannot reassign undeclared variable '"+target.Name+"'", location, "Declare it before using '='")
		}
		return binding.typeRef
	case *MemberLValue:
		containerType, final := checker.resolveStaticContainer(target.Root, target.Accesses, location)
		return checker.accessType(containerType, final, location, true)
	default:
		checker.err("Directive lvalue remained after expansion", location, "Expand #iteration and #as_lvalue before type checking")
	}
	return nil
}

// resolveStaticContainer resolves static container from the supplied context.
func (checker *TypeChecker) resolveStaticContainer(root string, accesses []Access, location *SourceLocation) (TypeRef, Access) {
	if len(accesses) == 0 {
		checker.err("Member target requires at least one access", location, "Use an identifier for a simple variable")
	}
	binding, ok := checker.current.lookupBinding(root)
	if !ok {
		checker.err("Undefined assignment root '"+root+"'", location, "Declare the root before assigning a member")
	}
	typeRef := binding.typeRef
	for _, access := range accesses[:len(accesses)-1] {
		typeRef = checker.accessType(typeRef, access, location, false)
	}
	return typeRef, accesses[len(accesses)-1]
}

// expressionType resolves and validates the static type of an expression.
func (checker *TypeChecker) expressionType(expression Expression, expected TypeRef) TypeRef {
	if expression == nil {
		return nil
	}
	switch value := expression.(type) {
	case *StringExpression:
		checker.checkStringReferences(value)
		if expected == PrimTimestamp {
			if literal, ok := checker.constantString(value); ok && !isValidTimestamp(literal) {
				checker.err("Invalid timestamp '"+literal+"'", value.Location, "Use YYYY-MM-DD HH:MM:SS or YYYY-MM-DD HH:MM:SS.fff")
			}
			return PrimTimestamp
		}
		return PrimString
	case *IntegerExpression:
		return PrimInteger
	case *FloatExpression:
		return PrimFloat
	case *BooleanExpression:
		return PrimBoolean
	case *ReferenceExpression:
		return checker.referenceType(value, expected)
	case *ArrayExpression:
		return checker.arrayExpressionType(value, expected)
	case *MappingExpression:
		return checker.mappingExpressionType(value, expected)
	case *StructExpression:
		return checker.structExpressionType(value, expected)
	case *NamespaceExpression:
		checker.err("Namespace expressions must be the direct value of an assignment", value.Location, "Assign #include(...) directly to a namespace variable")
	case *TypedExpression:
		typeRef := checker.resolveType(value.Type, value.Location)
		actual := checker.expressionType(value.Value, typeRef)
		checker.requireAssignable(typeRef, actual, value.Location, "expanded directive result")
		return typeRef
	case *IncludeExpression, *IterationExpression, *MacroCallExpression:
		checker.err("Directive expression remained after expansion", expression.expressionLocation(), "Expand directives before type checking")
	case *BinaryOpExpression:
		return checker.checkBinaryOpType(value)
	case *UnaryOpExpression:
		operandType := checker.expressionType(value.Operand, PrimBoolean)
		checker.requireAssignable(PrimBoolean, operandType, value.Operand.expressionLocation(), "#not operand")
		return PrimBoolean
	case *MatchExpression:
		valueType := checker.expressionType(value.Value, PrimString)
		checker.requireAssignable(PrimString, valueType, value.Value.expressionLocation(), "#match value")
		patternType := checker.expressionType(value.Pattern, PrimString)
		checker.requireAssignable(PrimString, patternType, value.Pattern.expressionLocation(), "#match pattern")
		return checker.applyAccessesType(PrimBoolean, value.Accesses, value.Location)
	case *LenExpression:
		operandType := checker.resolveType(checker.expressionType(value.Value, nil), value.Value.expressionLocation())
		switch operandType.(type) {
		case *ArrayType, *EnumKeyType, *MappingType:
			// valid
		default:
			checker.err("#len requires an array or mapping, got "+typeRefString(operandType), value.Value.expressionLocation(), "Use #len on an array or mapping value")
		}
		return checker.applyAccessesType(PrimInteger, value.Accesses, value.Location)
	case *IntrinsicExpression:
		return checker.applyAccessesType(checker.intrinsicExpressionType(value), value.Accesses, value.Location)
	}
	return nil
}

// applyAccessesType resolves the type reached by member accesses on a base type.
func (checker *TypeChecker) applyAccessesType(base TypeRef, accesses []Access, location *SourceLocation) TypeRef {
	result := base
	for _, access := range accesses {
		result = checker.accessType(result, access, location, false)
	}
	return result
}

// intrinsicExpressionType checks an intrinsic call's arguments and returns its
// declared return type.
func (checker *TypeChecker) intrinsicExpressionType(expression *IntrinsicExpression) TypeRef {
	def, ok := lookupIntrinsic(expression.Name)
	if !ok {
		checker.err(unknownIntrinsicMessage(expression.Name), expression.Location, "Known intrinsics: "+strings.Join(sortedIntrinsicNames(), ", "))
	}
	if len(expression.Arguments) != len(def.params) {
		checker.err(fmt.Sprintf("Intrinsic '@%s' expects %d arguments, got %d", expression.Name, len(def.params), len(expression.Arguments)), expression.Location, "Pass one argument for each declared parameter")
	}
	for index, argument := range expression.Arguments {
		actual := checker.expressionType(argument, def.params[index].typeRef)
		checker.requireAssignable(def.params[index].typeRef, actual, argument.expressionLocation(), "intrinsic '@"+expression.Name+"' argument '"+def.params[index].name+"'")
	}
	return def.returnType
}

// checkBinaryOpType checks binary op type against the current invariants.
func (checker *TypeChecker) checkBinaryOpType(expression *BinaryOpExpression) TypeRef {
	leftType := checker.resolveType(checker.expressionType(expression.Left, nil), expression.Left.expressionLocation())
	rightType := checker.resolveType(checker.expressionType(expression.Right, nil), expression.Right.expressionLocation())

	switch expression.Op {
	case OpAnd, OpOr:
		checker.requireAssignable(PrimBoolean, leftType, expression.Left.expressionLocation(), "#and/#or left operand")
		checker.requireAssignable(PrimBoolean, rightType, expression.Right.expressionLocation(), "#and/#or right operand")
		return PrimBoolean
	case OpEQ, OpNEQ:
		if !checker.assignable(leftType, rightType) && !checker.assignable(rightType, leftType) {
			checker.err("Type mismatch in comparison: cannot compare "+typeRefString(leftType)+" and "+typeRefString(rightType),
				expression.Location, "Both sides of ==/!= must have the same type")
		}
		return PrimBoolean
	case OpLT, OpLE, OpGT, OpGE:
		if !checker.assignable(leftType, rightType) && !checker.assignable(rightType, leftType) {
			checker.err("Type mismatch in comparison: cannot compare "+typeRefString(leftType)+" and "+typeRefString(rightType),
				expression.Location, "Both sides of </<=/>/>= must have the same comparable type")
		}
		if !checker.isComparableType(leftType) || !checker.isComparableType(rightType) {
			checker.err("Relational comparison requires comparable types (integer, float, string, timestamp)",
				expression.Location, "Use ==/!= for non-ordered types")
		}
		return PrimBoolean
	}
	return PrimBoolean
}

// isComparableType reports whether a resolved type supports ordered comparison.
func (checker *TypeChecker) isComparableType(typeRef TypeRef) bool {
	switch typeRef {
	case PrimInteger, PrimFloat, PrimString, PrimTimestamp:
		return true
	}
	return false
}

// checkStringReferences checks string references against the current invariants.
func (checker *TypeChecker) checkStringReferences(expression *StringExpression) {
	if expression.Literal != nil {
		for _, part := range expression.Literal.Parts {
			if reference, ok := part.(*InterpolationRef); ok {
				root, accesses := dottedReference(reference.Name, expression.Location)
				checker.referenceType(&ReferenceExpression{Root: root, Accesses: accesses, Location: expression.Location}, nil)
			}
		}
		return
	}
	content := expression.Multiline
	for index := 0; index < len(content); index++ {
		if content[index] != '{' {
			continue
		}
		end := index + 1
		for end < len(content) && (isAlphaNum(content[end]) || content[end] == '_' || content[end] == '.') {
			end++
		}
		if end > index+1 && end < len(content) && content[end] == '}' {
			root, accesses := dottedReference(content[index+1:end], expression.Location)
			checker.referenceType(&ReferenceExpression{Root: root, Accesses: accesses, Location: expression.Location}, nil)
			index = end
		}
	}
}

// constantString returns a statically known string expression when one is available.
func (checker *TypeChecker) constantString(expression *StringExpression) (string, bool) {
	if expression.Literal == nil {
		if strings.Contains(expression.Multiline, "{") {
			return "", false
		}
		return expression.Multiline, true
	}
	var result strings.Builder
	for _, part := range expression.Literal.Parts {
		text, ok := part.(StringText)
		if !ok {
			return "", false
		}
		result.WriteString(string(text))
	}
	return result.String(), true
}

// referenceType resolves the type reached by a root binding and its member accesses.
func (checker *TypeChecker) referenceType(reference *ReferenceExpression, expected TypeRef) TypeRef {
	if reference.Root == "" {
		resolved := checker.resolveType(expected, reference.Location)
		enumType, ok := resolved.(*EnumType)
		if !ok {
			checker.err("Enum shorthand requires an expected enum type", reference.Location, "Use the qualified form 'enum_name.member'")
		}
		first, ok := reference.Accesses[0].(*FieldAccess)
		if !ok || !enumHasMember(enumType, first.Name) {
			checker.err("Unknown enum member in shorthand reference", reference.Location, "Known members: "+strings.Join(enumType.MemberList, ", "))
		}
		result := TypeRef(enumType)
		for _, access := range reference.Accesses[1:] {
			result = checker.accessType(result, access, reference.Location, false)
		}
		return result
	}

	var result TypeRef
	if binding, ok := checker.current.lookupBinding(reference.Root); ok {
		result = binding.typeRef
	} else if typeRef, ok := checker.current.lookupType(reference.Root); ok {
		enumType, enumOK := typeRef.(*EnumType)
		if !enumOK || len(reference.Accesses) == 0 {
			checker.err("Type '"+reference.Root+"' cannot be used as a value", reference.Location, "Reference a declared value")
		}
		first, fieldOK := reference.Accesses[0].(*FieldAccess)
		if !fieldOK || !enumHasMember(enumType, first.Name) {
			checker.err("Unknown enum member on '"+reference.Root+"'", reference.Location, "Known members: "+strings.Join(enumType.MemberList, ", "))
		}
		result = enumType
		reference = &ReferenceExpression{Root: reference.Root, Accesses: reference.Accesses[1:], Location: reference.Location}
	} else {
		checker.err("Undefined variable or type '"+reference.Root+"'", reference.Location, "Declarations are visible only after their source position")
	}
	for _, access := range reference.Accesses {
		result = checker.accessType(result, access, reference.Location, false)
	}
	return result
}

// accessType validates one field or index access and returns the resulting type.
func (checker *TypeChecker) accessType(base TypeRef, access Access, location *SourceLocation, assignment bool) TypeRef {
	base = checker.resolveType(base, location)
	switch access := access.(type) {
	case *FieldAccess:
		switch typeRef := base.(type) {
		case *ScopeType:
			field, ok := typeRef.Fields[access.Name]
			if !ok {
				checker.err("Undefined scope member '"+access.Name+"'", access.Location, "Known members: "+strings.Join(sortedBindingTypeKeys(typeRef.Fields), ", "))
			}
			return field.Type
		case *NamespaceType:
			field, ok := typeRef.Fields[access.Name]
			if !ok {
				checker.err("Undefined namespace member '"+access.Name+"'", access.Location, "Known members: "+strings.Join(sortedBindingTypeKeys(typeRef.Fields), ", "))
			}
			return field.Type
		case *SetupType:
			field, ok := typeRef.Fields[access.Name]
			if !ok {
				checker.err("Undefined setup field '"+access.Name+"'", access.Location, "Known fields: "+strings.Join(sortedFieldDefinitionKeys(typeRef.Fields), ", "))
			}
			return field.DeclaredType
		case *EnumType:
			if access.Name != "value" {
				checker.err("Enum values only expose '.value'", access.Location, "Use '.value' on a tagged enum")
			}
			if assignment {
				checker.err("Enum tagged values cannot be reassigned", access.Location, "Assign a different enum member to the destination")
			}
			if typeRef.ValueType == nil {
				checker.err("Enum '"+typeRef.Name+"' has no tagged value", access.Location, "Remove '.value'")
			}
			return typeRef.ValueType
		default:
			checker.err("Cannot access field '"+access.Name+"' on type "+typeRefString(base), access.Location, "Use field access on a scope, namespace, setup, or tagged enum")
		}
	case *IndexAccess:
		switch typeRef := base.(type) {
		case *ArrayType:
			indexType := checker.expressionType(access.Index, PrimInteger)
			checker.requireAssignable(PrimInteger, indexType, access.Location, "array index")
			return typeRef.ElementType
		case *EnumKeyType:
			enumRef, ok := checker.current.lookupType(typeRef.EnumName)
			if !ok {
				checker.err("Unknown enum index type '"+typeRef.EnumName+"'", access.Location, "Declare the enum before the array type")
			}
			indexType := checker.expressionType(access.Index, enumRef)
			checker.requireAssignable(enumRef, indexType, access.Location, "enum array index")
			return typeRef.ElementType
		case *MappingType:
			indexType := checker.expressionType(access.Index, typeRef.KeyType)
			checker.requireAssignable(typeRef.KeyType, indexType, access.Location, "mapping key")
			return typeRef.ValueType
		default:
			checker.err("Cannot index type "+typeRefString(base), access.Location, "Use indexing on an array or mapping")
		}
	}
	return nil
}

// arrayExpressionType checks array elements and returns the resolved array type.
func (checker *TypeChecker) arrayExpressionType(expression *ArrayExpression, expected TypeRef) TypeRef {
	declared := checker.resolveType(expression.DeclaredType, expression.Location)
	expected = checker.resolveType(expected, expression.Location)
	if enumIndexed, ok := expected.(*EnumKeyType); ok {
		if literalArray, literalOK := declared.(*ArrayType); literalOK {
			checker.requireAssignable(enumIndexed.ElementType, literalArray.ElementType, expression.Location, "array literal element type")
			declared = enumIndexed
		}
	}
	if declared == nil {
		declared = expected
	}
	var elementType TypeRef
	switch typeRef := declared.(type) {
	case *ArrayType:
		elementType = typeRef.ElementType
	case *EnumKeyType:
		elementType = typeRef.ElementType
		indexType, ok := checker.current.lookupType(typeRef.EnumName)
		if !ok {
			checker.err("Unknown enum index type '"+typeRef.EnumName+"'", expression.Location, "Declare the enum before the array")
		}
		enumType := indexType.(*EnumType)
		if len(expression.Elements) != len(enumType.MemberList) {
			checker.err(fmt.Sprintf("Enum-indexed array requires %d elements, got %d", len(enumType.MemberList), len(expression.Elements)), expression.Location, "Provide one element for each enum member")
		}
	}
	if elementType == nil {
		if len(expression.Elements) == 0 {
			checker.err("Cannot infer the element type of an empty array", expression.Location, "Use a typed literal such as '[]string{}'")
		}
		elementType = checker.expressionType(expression.Elements[0], nil)
		declared = &ArrayType{ElementType: elementType}
	}
	for _, element := range expression.Elements {
		actual := checker.expressionType(element, elementType)
		checker.requireAssignable(elementType, actual, element.expressionLocation(), "array element")
	}
	return declared
}

// mappingExpressionType checks mapping keys and values and returns the resolved mapping type.
func (checker *TypeChecker) mappingExpressionType(expression *MappingExpression, expected TypeRef) TypeRef {
	var declared TypeRef
	if expression.DeclaredType != nil {
		declared = expression.DeclaredType
	} else {
		declared = expected
	}
	declared = checker.resolveType(declared, expression.Location)
	mappingType, _ := declared.(*MappingType)
	if mappingType == nil {
		if len(expression.Entries) == 0 {
			checker.err("Cannot infer the type of an empty mapping", expression.Location, "Use mapping(key_type, value_type){}")
		}
		entry := expression.Entries[0]
		mappingType = &MappingType{KeyType: checker.expressionType(entry.Keys[0], nil), ValueType: checker.expressionType(entry.Value, nil)}
	}
	checker.requireMappingKeyType(mappingType.KeyType, expression.Location)
	for _, entry := range expression.Entries {
		for _, key := range entry.Keys {
			actual := checker.expressionType(key, mappingType.KeyType)
			checker.requireAssignable(mappingType.KeyType, actual, key.expressionLocation(), "mapping key")
		}
		actual := checker.expressionType(entry.Value, mappingType.ValueType)
		checker.requireAssignable(mappingType.ValueType, actual, entry.Value.expressionLocation(), "mapping value")
	}
	return mappingType
}

// requireMappingKeyType requires a valid mapping key type value.
func (checker *TypeChecker) requireMappingKeyType(typeRef TypeRef, location *SourceLocation) {
	resolved := checker.resolveType(typeRef, location)
	switch resolved {
	case PrimString, PrimInteger, PrimBoolean, PrimFloat, PrimTimestamp:
		return
	}
	if _, ok := resolved.(*EnumType); ok {
		return
	}
	checker.err("Mapping key type must be primitive or enum, got "+typeRefString(resolved), location, "Use string, integer, float, boolean, timestamp, or an enum")
}

// structExpressionType checks named struct fields and returns the resolved setup type.
func (checker *TypeChecker) structExpressionType(expression *StructExpression, expected TypeRef) TypeRef {
	var setup *SetupType
	if expression.TypeName != "" {
		resolved, ok := checker.current.lookupType(expression.TypeName)
		if !ok {
			checker.err("Unknown setup type '"+expression.TypeName+"'", expression.Location, "Declare the setup before use")
		}
		setup, ok = resolved.(*SetupType)
		if !ok {
			checker.err("Type '"+expression.TypeName+"' is not a setup", expression.Location, "Use a setup type for a struct literal")
		}
	} else if resolved := checker.resolveType(expected, expression.Location); resolved != nil {
		setup, _ = resolved.(*SetupType)
	}

	if setup == nil {
		fields := make(map[string]*BindingType)
		for _, field := range expression.Fields {
			if _, exists := fields[field.Name]; exists {
				checker.err("Duplicate struct field '"+field.Name+"'", field.Location, "Field initializers must be unique")
			}
			fields[field.Name] = &BindingType{Type: checker.expressionType(field.Value, nil)}
		}
		return &ScopeType{Fields: fields}
	}

	provided := make(map[string]bool)
	for _, field := range expression.Fields {
		if provided[field.Name] {
			checker.err("Duplicate struct field '"+field.Name+"'", field.Location, "Field initializers must be unique")
		}
		definition, ok := setup.Fields[field.Name]
		if !ok {
			checker.err("Unknown field '"+field.Name+"' in setup '"+setup.Name+"'", field.Location, "Known fields: "+strings.Join(sortedFieldDefinitionKeys(setup.Fields), ", "))
		}
		provided[field.Name] = true
		actual := checker.expressionType(field.Value, definition.DeclaredType)
		checker.requireAssignable(definition.DeclaredType, actual, field.Location, "setup field")
	}
	for name, field := range setup.Fields {
		if !field.Optional && !provided[name] {
			checker.err("Struct literal missing required field '"+name+"' in '"+setup.Name+"'", expression.Location, "Add '"+name+" = <value>'")
		}
	}
	return setup
}

// resolveType resolves type from the supplied context.
func (checker *TypeChecker) resolveType(typeRef TypeRef, location *SourceLocation) TypeRef {
	if typeRef == nil {
		return nil
	}
	switch value := typeRef.(type) {
	case TypeName:
		resolved, ok := checker.current.lookupType(string(value))
		if !ok {
			checker.err("Unknown type '"+string(value)+"'", location, "Types must be declared before use")
		}
		return resolved
	case *ArrayType:
		return &ArrayType{ElementType: checker.resolveType(value.ElementType, location)}
	case *EnumKeyType:
		resolved, ok := checker.current.lookupType(value.EnumName)
		if !ok {
			checker.err("Unknown enum index type '"+value.EnumName+"'", location, "Declare the enum before this array type")
		}
		if _, ok := resolved.(*EnumType); !ok {
			checker.err("Array index type '"+value.EnumName+"' is not an enum", location, "Use an enum name inside '[...]'")
		}
		return &EnumKeyType{EnumName: value.EnumName, ElementType: checker.resolveType(value.ElementType, location)}
	case *MappingType:
		return &MappingType{KeyType: checker.resolveType(value.KeyType, location), ValueType: checker.resolveType(value.ValueType, location)}
	default:
		return typeRef
	}
}

// requireAssignable requires a valid assignable value.
func (checker *TypeChecker) requireAssignable(expected, actual TypeRef, location *SourceLocation, context string) {
	if checker.assignable(expected, actual) {
		return
	}
	checker.err("Type mismatch in "+context+": expected "+typeRefString(expected)+", got "+typeRefString(actual), location, "Use a value matching the destination type")
}

// assignable reports whether an actual type may be assigned to an expected type.
func (checker *TypeChecker) assignable(expected, actual TypeRef) bool {
	expected = checker.resolveType(expected, nil)
	actual = checker.resolveType(actual, nil)
	if expected == nil || actual == nil {
		return false
	}
	if expected == PrimFloat && actual == PrimInteger {
		return true
	}
	if expected == PrimTimestamp && actual == PrimString {
		return true
	}
	if expected == PrimScope {
		_, ok := actual.(*ScopeType)
		return ok || actual == PrimScope
	}
	if expected == PrimNamespace {
		_, ok := actual.(*NamespaceType)
		return ok || actual == PrimNamespace
	}
	switch expectedType := expected.(type) {
	case PrimitiveKind:
		actualType, ok := actual.(PrimitiveKind)
		return ok && expectedType == actualType
	case *EnumType:
		actualType, ok := actual.(*EnumType)
		return ok && expectedType.Name == actualType.Name
	case *SetupType:
		actualType, ok := actual.(*SetupType)
		return ok && expectedType.Name == actualType.Name
	case *ArrayType:
		actualType, ok := actual.(*ArrayType)
		return ok && checker.assignable(expectedType.ElementType, actualType.ElementType)
	case *EnumKeyType:
		actualType, ok := actual.(*EnumKeyType)
		return ok && expectedType.EnumName == actualType.EnumName && checker.assignable(expectedType.ElementType, actualType.ElementType)
	case *MappingType:
		actualType, ok := actual.(*MappingType)
		return ok && checker.assignable(expectedType.KeyType, actualType.KeyType) && checker.assignable(expectedType.ValueType, actualType.ValueType)
	case *ScopeType:
		actualType, ok := actual.(*ScopeType)
		return ok && checker.structurallyAssignable(expectedType.Fields, actualType.Fields)
	case *NamespaceType:
		actualType, ok := actual.(*NamespaceType)
		return ok && checker.structurallyAssignable(expectedType.Fields, actualType.Fields)
	}
	return false
}

// structurallyAssignable reports whether two compound types have compatible structure.
func (checker *TypeChecker) structurallyAssignable(expected, actual map[string]*BindingType) bool {
	if len(expected) != len(actual) {
		return false
	}
	for name, expectedField := range expected {
		actualField, ok := actual[name]
		if !ok || !checker.assignable(expectedField.Type, actualField.Type) {
			return false
		}
	}
	return true
}

// enumHasMember reports whether a resolved enum contains a named member.
func enumHasMember(enumType *EnumType, name string) bool {
	for _, member := range enumType.MemberList {
		if member == name {
			return true
		}
	}
	return false
}

// sortedBindingTypeKeys returns binding type keys in deterministic order.
func sortedBindingTypeKeys(values map[string]*BindingType) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// detectDependencyCycles reports direct and indirect value or type cycles before
// source-order checking reports a less useful forward-reference error.
func (checker *TypeChecker) detectDependencyCycles() {
	graph := make(map[string][]string)
	locations := make(map[string]*SourceLocation)
	checker.collectCycleNodes(checker.program.Statements, "", graph, locations)
	checker.collectCycleEdges(checker.program.Statements, "", graph, nil, nil)
	state := make(map[string]int)
	stack := []string{}
	var visit func(string)
	visit = func(node string) {
		switch state[node] {
		case 1:
			start := indexOf(stack, node)
			cycle := append(append([]string{}, stack[start:]...), node)
			checker.err("Circular dependency: "+strings.Join(cycle, " -> "), locations[node], "Remove the recursive reference; SOMETHING does not permit recursion")
		case 2:
			return
		}
		state[node] = 1
		stack = append(stack, node)
		for _, dependency := range graph[node] {
			if _, exists := graph[dependency]; exists {
				visit(dependency)
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = 2
	}
	keys := make([]string, 0, len(graph))
	for key := range graph {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		visit(key)
	}
}

// collectCycleNodes collects cycle nodes from the supplied inputs.
func (checker *TypeChecker) collectCycleNodes(statements []Statement, prefix string, graph map[string][]string, locations map[string]*SourceLocation) {
	for _, statement := range statements {
		if loop, ok := statement.(*ForDirective); ok {
			checker.collectCycleNodes(loop.Body, prefix, graph, locations)
			continue
		}
		if assertDir, ok := statement.(*AssertDirective); ok {
			checker.collectCycleNodes(assertDir.Body, prefix, graph, locations)
			continue
		}
		if ifDir, ok := statement.(*IfDirective); ok {
			checker.collectCycleNodes(ifDir.Body, prefix, graph, locations)
			continue
		}
		assignment, ok := statement.(*Assignment)
		if !ok || assignment.Mode == AssignExisting {
			continue
		}
		name := cycleTargetName(assignment.Target, prefix)
		if name == "" {
			continue
		}
		graph[name] = graph[name]
		locations[name] = assignment.Location
		switch value := assignment.Value.(type) {
		case *SetupDefinition, *EnumDefinition:
			typeNode := "type:" + name
			graph[typeNode] = graph[typeNode]
			locations[typeNode] = assignment.Location
		case *ScopeExpression:
			checker.collectCycleNodes(value.Statements, name+".", graph, locations)
		case *NamespaceExpression:
			checker.collectCycleNodes(value.Statements, name+".", graph, locations)
		}
	}
}

// collectCycleEdges collects cycle edges from the supplied inputs.
func (checker *TypeChecker) collectCycleEdges(statements []Statement, prefix string, graph map[string][]string, visibleValues, visibleTypes map[string]string) {
	locals := copyCycleNames(visibleValues)
	types := copyCycleNames(visibleTypes)
	for _, statement := range statements {
		assignment, ok := statement.(*Assignment)
		if !ok || assignment.Mode == AssignExisting {
			continue
		}
		if identifier, ok := assignment.Target.(*IdentifierLValue); ok {
			switch assignment.Value.(type) {
			case *SetupDefinition, *EnumDefinition:
				types[identifier.Name] = prefix + identifier.Name
			default:
				locals[identifier.Name] = prefix + identifier.Name
			}
		}
	}
	for _, statement := range statements {
		if loop, ok := statement.(*ForDirective); ok {
			checker.collectCycleEdges(loop.Body, prefix, graph, locals, types)
			continue
		}
		if assertDir, ok := statement.(*AssertDirective); ok {
			checker.collectCycleEdges(assertDir.Body, prefix, graph, locals, types)
			continue
		}
		if ifDir, ok := statement.(*IfDirective); ok {
			checker.collectCycleEdges(ifDir.Body, prefix, graph, locals, types)
			continue
		}
		assignment, ok := statement.(*Assignment)
		if !ok || assignment.Mode == AssignExisting {
			continue
		}
		name := cycleTargetName(assignment.Target, prefix)
		if name == "" {
			continue
		}
		switch value := assignment.Value.(type) {
		case *SetupDefinition:
			node := "type:" + name
			for _, field := range value.Fields {
				for _, dependency := range typeDependencies(field.DeclaredType) {
					if resolved, ok := types[dependency]; ok {
						dependency = resolved
					}
					graph[node] = append(graph[node], "type:"+dependency)
				}
			}
		case *EnumDefinition:
			node := "type:" + name
			for _, dependency := range typeDependencies(value.ValueType) {
				if resolved, ok := types[dependency]; ok {
					dependency = resolved
				}
				graph[node] = append(graph[node], "type:"+dependency)
			}
		case *ScopeExpression:
			checker.collectCycleEdges(value.Statements, name+".", graph, locals, types)
		case *NamespaceExpression:
			checker.collectCycleEdges(value.Statements, name+".", graph, locals, types)
		case Expression:
			for _, dependency := range expressionDependencies(value) {
				root := strings.SplitN(dependency, ".", 2)[0]
				if local, ok := locals[root]; ok {
					if dot := strings.Index(dependency, "."); dot >= 0 {
						dependency = local + dependency[dot:]
					} else {
						dependency = local
					}
				}
				graph[name] = append(graph[name], dependency)
			}
		}
	}
}

// copyCycleNames copies cycle names into an independent value.
func copyCycleNames(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for name, target := range source {
		result[name] = target
	}
	return result
}

// cycleTargetName returns the declared root name participating in dependency analysis.
func cycleTargetName(target LValue, prefix string) string {
	switch target := target.(type) {
	case *IdentifierLValue:
		return prefix + target.Name
	case *MemberLValue:
		var result strings.Builder
		result.WriteString(target.Root)
		for _, access := range target.Accesses {
			field, ok := access.(*FieldAccess)
			if !ok {
				return ""
			}
			result.WriteString(".")
			result.WriteString(field.Name)
		}
		return result.String()
	}
	return ""
}

// typeDependencies collects named types referenced by a type expression.
func typeDependencies(typeRef TypeRef) []string {
	switch value := typeRef.(type) {
	case TypeName:
		return []string{string(value)}
	case *ArrayType:
		return typeDependencies(value.ElementType)
	case *EnumKeyType:
		return append([]string{value.EnumName}, typeDependencies(value.ElementType)...)
	case *MappingType:
		return append(typeDependencies(value.KeyType), typeDependencies(value.ValueType)...)
	default:
		return nil
	}
}

// expressionDependencies collects root bindings referenced by an expression.
func expressionDependencies(expression Expression) []string {
	dependencies := []string{}
	var walk func(Expression)
	walk = func(expression Expression) {
		switch value := expression.(type) {
		case *StringExpression:
			if value.Literal != nil {
				for _, part := range value.Literal.Parts {
					if reference, ok := part.(*InterpolationRef); ok {
						dependencies = append(dependencies, reference.Name)
					}
				}
			} else {
				content := value.Multiline
				for index := 0; index < len(content); index++ {
					if content[index] != '{' {
						continue
					}
					end := index + 1
					for end < len(content) && (isAlphaNum(content[end]) || content[end] == '_' || content[end] == '.') {
						end++
					}
					if end > index+1 && end < len(content) && content[end] == '}' {
						dependencies = append(dependencies, content[index+1:end])
						index = end
					}
				}
			}
		case *ReferenceExpression:
			if value.Root != "" {
				dependencies = append(dependencies, referencePath(value))
			}
			for _, access := range value.Accesses {
				if index, ok := access.(*IndexAccess); ok {
					walk(index.Index)
				}
			}
		case *ArrayExpression:
			for _, element := range value.Elements {
				walk(element)
			}
		case *MappingExpression:
			for _, entry := range value.Entries {
				for _, key := range entry.Keys {
					walk(key)
				}
				walk(entry.Value)
			}
		case *StructExpression:
			for _, field := range value.Fields {
				walk(field.Value)
			}
		case *TypedExpression:
			walk(value.Value)
		case *BinaryOpExpression:
			walk(value.Left)
			walk(value.Right)
		case *UnaryOpExpression:
			walk(value.Operand)
		case *MatchExpression:
			walk(value.Value)
			walk(value.Pattern)
			for _, access := range value.Accesses {
				if index, ok := access.(*IndexAccess); ok {
					walk(index.Index)
				}
			}
		case *LenExpression:
			walk(value.Value)
			for _, access := range value.Accesses {
				if index, ok := access.(*IndexAccess); ok {
					walk(index.Index)
				}
			}
		case *IntrinsicExpression:
			for _, argument := range value.Arguments {
				walk(argument)
			}
			for _, access := range value.Accesses {
				if index, ok := access.(*IndexAccess); ok {
					walk(index.Index)
				}
			}
		}
	}
	walk(expression)
	return dependencies
}

// referencePath renders a reference root and accesses for dependency diagnostics.
func referencePath(reference *ReferenceExpression) string {
	var result strings.Builder
	result.WriteString(reference.Root)
	for _, access := range reference.Accesses {
		field, ok := access.(*FieldAccess)
		if !ok {
			break
		}
		result.WriteString(".")
		result.WriteString(field.Name)
	}
	return result.String()
}
