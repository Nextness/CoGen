// evaluator.go evaluates an already expanded and checked, source-ordered AST.
// The same runtime machinery supplies the temporary values needed by directive
// expansion, but the public pipeline evaluates the program only after checking.
package something

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// EnumValue preserves enum identity internally while expressions are evaluated.
type EnumValue struct {
	Ordinal  int
	EnumName string
}

// runtimeBinding stores one evaluated value with its type and visibility.
type runtimeBinding struct {
	value   any
	typeRef TypeRef
	private bool
}

// runtimeEnvironment stores lexically nested runtime bindings and named types.
type runtimeEnvironment struct {
	parent   *runtimeEnvironment
	bindings map[string]*runtimeBinding
	types    map[string]TypeRef
}

// newRuntimeEnvironment creates an empty runtime scope linked to its parent.
func newRuntimeEnvironment(parent *runtimeEnvironment) *runtimeEnvironment {
	return &runtimeEnvironment{
		parent:   parent,
		bindings: make(map[string]*runtimeBinding),
		types:    make(map[string]TypeRef),
	}
}

// lookupBinding searches the current and enclosing scopes for a value binding.
func (environment *runtimeEnvironment) lookupBinding(name string) (*runtimeBinding, bool) {
	for current := environment; current != nil; current = current.parent {
		if binding, ok := current.bindings[name]; ok {
			return binding, true
		}
	}
	return nil, false
}

// lookupType searches the current and enclosing scopes for a named type.
func (environment *runtimeEnvironment) lookupType(name string) (TypeRef, bool) {
	for current := environment; current != nil; current = current.parent {
		if typeRef, ok := current.types[name]; ok {
			return typeRef, true
		}
	}
	return nil, false
}

// runtimeObject carries scope, namespace, or setup fields in a nested runtime environment.
type runtimeObject struct {
	environment *runtimeEnvironment
	kind        PrimitiveKind
	typeName    string
}

// runtimeMapping preserves ordered key-value entries and their resolved types.
type runtimeMapping struct {
	entries   []runtimeMappingEntry
	keyType   TypeRef
	valueType TypeRef
}

// runtimeMappingEntry stores one evaluated mapping key and value.
type runtimeMappingEntry struct {
	key   any
	value any
}

// lookup returns the value associated with a runtime-equivalent mapping key.
func (mapping *runtimeMapping) lookup(key any) (any, bool) {
	for _, entry := range mapping.entries {
		if runtimeKeysEqual(entry.key, key) {
			return entry.value, true
		}
	}
	return nil, false
}

// update replaces the value for a runtime-equivalent mapping key and reports whether it existed.
func (mapping *runtimeMapping) update(key, value any) bool {
	for index := range mapping.entries {
		if runtimeKeysEqual(mapping.entries[index].key, key) {
			mapping.entries[index].value = value
			return true
		}
	}
	return false
}

// runtimeKeysEqual compares evaluated mapping keys while preserving enum ordinal compatibility.
func runtimeKeysEqual(left, right any) bool {
	switch leftValue := left.(type) {
	case *EnumValue:
		switch rightValue := right.(type) {
		case *EnumValue:
			return leftValue.EnumName == rightValue.EnumName && leftValue.Ordinal == rightValue.Ordinal
		case int:
			return leftValue.Ordinal == rightValue
		}
	case int:
		if rightValue, ok := right.(*EnumValue); ok {
			return leftValue == rightValue.Ordinal
		}
	}
	return fmt.Sprintf("%T:%v", left, left) == fmt.Sprintf("%T:%v", right, right)
}

// Evaluator evaluates a checked program into its public configuration map.
type Evaluator struct {
	checked  *CheckedProgram
	filepath string
	state    *runtimeState
}

// runtimeState owns evaluator scopes, checked assignment types, assertions, and source context.
type runtimeState struct {
	filepath         string
	root             *runtimeEnvironment
	current          *runtimeEnvironment
	types            map[*Assignment]TypeRef
	assertions       map[string][]Statement // type name -> assertion bodies
	instanceLocation *SourceLocation        // set when evaluating assertions for a specific instance
}

// newRuntimeState creates evaluator state with an empty root environment.
func newRuntimeState(filepath string, types map[*Assignment]TypeRef) *runtimeState {
	root := newRuntimeEnvironment(nil)
	return &runtimeState{filepath: filepath, root: root, current: root, types: types, assertions: make(map[string][]Statement)}
}

// NewEvaluator constructs evaluator.
func NewEvaluator(checked *CheckedProgram, filepath string) *Evaluator {
	if filepath == "" && checked != nil && checked.Program != nil {
		filepath = checked.Program.Filepath
	}
	return &Evaluator{checked: checked, filepath: filepath, state: newRuntimeState(filepath, checked.AssignmentTypes)}
}

// evaluate executes a checked program and returns its public configuration map.
func (evaluator *Evaluator) evaluate() map[string]any {
	if evaluator.checked == nil || evaluator.checked.Program == nil {
		return map[string]any{}
	}
	evaluator.state.evaluateStatements(evaluator.checked.Program.Statements)
	return evaluator.state.publicMap(evaluator.state.root)
}

// err panics with a source-located SomethingError for a runtime semantic failure.
func (state *runtimeState) err(message string, location *SourceLocation, suggestion string) {
	panic(errLoc(message, location, state.filepath, suggestion))
}

// evaluateStatements evaluates statements against the current evaluator state.
func (state *runtimeState) evaluateStatements(statements []Statement) {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *Assignment:
			state.evaluateAssignment(node)
		case *AssertDirective:
			state.evaluateAssertDirective(node)
		case *IfDirective:
			state.evaluateIfDirective(node)
		case *ErrorDirective:
			state.evaluateErrorDirective(node)
		default:
			state.err("Unexpanded directive reached evaluation", statement.statementBase().Location, "Run directive generation before evaluation")
		}
	}
}

// evaluateAssertDirective evaluates assert directive against the current evaluator state.
func (state *runtimeState) evaluateAssertDirective(assertion *AssertDirective) {
	// Validate the type exists and is a setup type
	typeRef, ok := state.current.lookupType(assertion.TypeName)
	if !ok {
		state.err("Undefined assertion target type '"+assertion.TypeName+"'", assertion.Location, "Assertions must reference an existing setup type name")
	}
	_, ok = typeRef.(*SetupType)
	if !ok {
		state.err("Assertion target '"+assertion.TypeName+"' is not a setup type", assertion.Location, "Only setup types can be asserted")
	}
	// Store the assertion body to be evaluated when an instance of this type is created
	state.assertions[assertion.TypeName] = append(state.assertions[assertion.TypeName], assertion.Body...)
}

// evaluateIfDirective evaluates if directive against the current evaluator state.
func (state *runtimeState) evaluateIfDirective(ifDir *IfDirective) {
	condValue := state.evaluateExpression(ifDir.Condition, PrimBoolean)
	cond, ok := condValue.(bool)
	if !ok {
		state.err("#if condition must evaluate to a boolean", ifDir.Condition.expressionLocation(), "Use a boolean expression")
	}
	if cond {
		state.evaluateStatements(ifDir.Body)
	}
}

// evaluateErrorDirective evaluates error directive against the current evaluator state.
func (state *runtimeState) evaluateErrorDirective(errDir *ErrorDirective) {
	msg := state.evaluateExpression(errDir.Message, PrimString)
	text, ok := msg.(string)
	if !ok {
		state.err("#error message must evaluate to a string", errDir.Message.expressionLocation(), "Use a string expression")
	}
	suggestion := "Assertion failed"
	if state.instanceLocation != nil {
		loc := state.instanceLocation
		suggestion = fmt.Sprintf("Assertion failed for instance at %s:%d:%d", loc.Filepath, loc.Line, loc.Col)
	}
	panic(errLoc(text, errDir.Location, state.filepath, suggestion))
}

// evaluateAssignment evaluates assignment against the current evaluator state.
func (state *runtimeState) evaluateAssignment(assignment *Assignment) {
	switch value := assignment.Value.(type) {
	case *EnumDefinition:
		state.evaluateEnumDefinition(assignment, value)
		return
	case *SetupDefinition:
		state.evaluateSetupDefinition(assignment, value)
		return
	case *ScopeExpression:
		state.evaluateObjectAssignment(assignment, value.Statements, PrimScope)
		return
	}

	expectedType := assignment.DeclaredType
	if resolved, ok := state.types[assignment]; ok {
		expectedType = resolved
	} else if expectedType != nil {
		expectedType = state.resolveType(expectedType, assignment.Location)
	}
	value := state.evaluateExpression(assignment.Value.(Expression), expectedType)
	state.assignValue(assignment, value, expectedType)
}

// evaluateEnumDefinition evaluates enum definition against the current evaluator state.
func (state *runtimeState) evaluateEnumDefinition(assignment *Assignment, definition *EnumDefinition) {
	name := requireIdentifierTarget(state, assignment.Target, assignment.Location)
	if _, exists := state.current.types[name]; exists {
		state.err("Type '"+name+"' is already declared in this scope", assignment.Location, "Use a unique type name")
	}
	valueType := definition.ValueType
	enumType := &EnumType{Name: name, ValueType: valueType, Members: make(map[string]Expression)}
	for _, member := range definition.Members {
		if _, exists := enumType.Members[member.Name]; exists {
			state.err("Duplicate enum member '"+member.Name+"'", member.Location, "Enum member names must be unique")
		}
		enumType.MemberList = append(enumType.MemberList, member.Name)
		enumType.Members[member.Name] = member.Value
	}
	state.current.types[name] = enumType
}

// evaluateSetupDefinition evaluates setup definition against the current evaluator state.
func (state *runtimeState) evaluateSetupDefinition(assignment *Assignment, definition *SetupDefinition) {
	name := requireIdentifierTarget(state, assignment.Target, assignment.Location)
	if _, exists := state.current.types[name]; exists {
		state.err("Type '"+name+"' is already declared in this scope", assignment.Location, "Use a unique type name")
	}
	setup := &SetupType{Name: name, Fields: make(map[string]*FieldDefinition)}
	for _, field := range definition.Fields {
		copy := *field
		setup.Fields[copy.Name] = &copy
	}
	state.current.types[name] = setup
}

// requireIdentifierTarget requires a valid identifier target value.
func requireIdentifierTarget(state *runtimeState, target LValue, location *SourceLocation) string {
	identifier, ok := target.(*IdentifierLValue)
	if !ok {
		state.err("Type definitions require an identifier target", location, "Assign enum and setup definitions to a simple name")
	}
	return identifier.Name
}

// evaluateObjectAssignment evaluates object assignment against the current evaluator state.
func (state *runtimeState) evaluateObjectAssignment(assignment *Assignment, statements []Statement, kind PrimitiveKind) {
	objectEnvironment := newRuntimeEnvironment(state.current)
	object := &runtimeObject{environment: objectEnvironment, kind: kind}
	typeRef := TypeRef(&ScopeType{Fields: make(map[string]*BindingType)})
	if kind == PrimNamespace {
		typeRef = &NamespaceType{Fields: make(map[string]*BindingType), Types: make(map[string]TypeRef)}
	}
	state.assignValue(assignment, object, typeRef)
	previous := state.current
	state.current = objectEnvironment
	state.evaluateStatements(statements)
	state.current = previous
}

// assignValue declares or reassigns an evaluated assignment target.
func (state *runtimeState) assignValue(assignment *Assignment, value any, typeRef TypeRef) {
	if assignment.Mode == AssignExisting {
		state.reassign(assignment.Target, value, assignment.Location)
		return
	}
	if typeRef == nil {
		typeRef = state.runtimeType(value)
	}
	switch target := assignment.Target.(type) {
	case *IdentifierLValue:
		if _, exists := state.current.bindings[target.Name]; exists {
			state.err("Variable '"+target.Name+"' is already declared in this scope", assignment.Location, "Use '=' to reassign an existing variable")
		}
		state.current.bindings[target.Name] = &runtimeBinding{value: value, typeRef: typeRef, private: assignment.Private}
	case *MemberLValue:
		state.declareMember(target, value, typeRef, assignment.Private, assignment.Location)
	default:
		state.err("Unexpanded directive lvalue reached evaluation", assignment.Location, "Run directive generation before evaluation")
	}
}

// declareMember adds a previously undeclared field to a scope or namespace value.
func (state *runtimeState) declareMember(target *MemberLValue, value any, typeRef TypeRef, private bool, location *SourceLocation) {
	container, _, final := state.resolveLValueContainer(target.Root, target.Accesses, location)
	switch access := final.(type) {
	case *FieldAccess:
		object, ok := container.(*runtimeObject)
		if !ok {
			state.err("New members can only be declared in a scope or namespace", access.Location, "Use '=' to reassign an existing setup, array, or mapping member")
		}
		if _, exists := object.environment.bindings[access.Name]; exists {
			state.err("Member '"+access.Name+"' is already declared", access.Location, "Use '=' to reassign the member")
		}
		object.environment.bindings[access.Name] = &runtimeBinding{value: value, typeRef: typeRef, private: private}
	default:
		state.err("Indexed members cannot be declared", location, "Declare the collection value first, then reassign an existing index")
	}
}

// reassign replaces an existing variable, field, array element, or mapping entry.
func (state *runtimeState) reassign(target LValue, value any, location *SourceLocation) {
	switch target := target.(type) {
	case *IdentifierLValue:
		binding, ok := state.current.lookupBinding(target.Name)
		if !ok {
			state.err("Cannot reassign undeclared variable '"+target.Name+"'", location, "Declare it with ':=' or ': type =' before reassigning it")
		}
		binding.value = value
	case *MemberLValue:
		container, containerType, final := state.resolveLValueContainer(target.Root, target.Accesses, location)
		switch access := final.(type) {
		case *FieldAccess:
			switch object := container.(type) {
			case *runtimeObject:
				binding, exists := object.environment.bindings[access.Name]
				if !exists {
					state.err("Cannot reassign undeclared member '"+access.Name+"'", access.Location, "Declare the member before reassigning it")
				}
				binding.value = value
			default:
				state.err("Field reassignment requires a scope, namespace, or setup value", access.Location, "Check the assignment target")
			}
		case *IndexAccess:
			switch collection := container.(type) {
			case []any:
				index := state.evaluateExpression(access.Index, state.collectionIndexType(containerType, access.Location))
				position := state.integerIndex(index, access.Location)
				if position < 0 || position >= len(collection) {
					state.err(fmt.Sprintf("Index %d out of bounds for array of length %d", position, len(collection)), access.Location, "Use an existing array index")
				}
				collection[position] = value
			case *runtimeMapping:
				index := state.evaluateExpression(access.Index, collection.keyType)
				if !collection.update(index, value) {
					state.err("Cannot reassign a missing mapping key", access.Location, "Mapping reassignment only replaces an existing key")
				}
			default:
				state.err("Indexed reassignment requires an array or mapping", access.Location, "Check the assignment target")
			}
		}
	default:
		state.err("Unexpanded directive lvalue reached reassignment", location, "Run directive generation before evaluation")
	}
}

// resolveLValueContainer resolves l value container from the supplied context.
func (state *runtimeState) resolveLValueContainer(root string, accesses []Access, location *SourceLocation) (any, TypeRef, Access) {
	if len(accesses) == 0 {
		state.err("Member assignment requires at least one access", location, "Use a simple identifier for a variable assignment")
	}
	binding, ok := state.current.lookupBinding(root)
	if !ok {
		state.err("Undefined assignment root '"+root+"'", location, "Declare the root value before assigning one of its members")
	}
	container := binding.value
	containerType := binding.typeRef
	for _, access := range accesses[:len(accesses)-1] {
		container, containerType = state.resolveTypedAccess(container, containerType, access, location)
	}
	return container, containerType, accesses[len(accesses)-1]
}

// evaluateExpression evaluates expression against the current evaluator state.
func (state *runtimeState) evaluateExpression(expression Expression, expected TypeRef) any {
	if expression == nil {
		return nil
	}
	switch value := expression.(type) {
	case *StringExpression:
		return state.evaluateString(value)
	case *IntegerExpression:
		return value.Value
	case *FloatExpression:
		return value.Value
	case *BooleanExpression:
		return value.Value
	case *ReferenceExpression:
		return state.evaluateReference(value, expected)
	case *ArrayExpression:
		return state.evaluateArray(value, expected)
	case *MappingExpression:
		return state.evaluateMapping(value, expected)
	case *StructExpression:
		return state.evaluateStruct(value, expected)
	case *NamespaceExpression:
		environment := newRuntimeEnvironment(state.current)
		object := &runtimeObject{environment: environment, kind: PrimNamespace}
		previous := state.current
		state.current = environment
		state.evaluateStatements(value.Statements)
		state.current = previous
		return object
	case *TypedExpression:
		return state.evaluateExpression(value.Value, value.Type)
	case *IncludeExpression, *IterationExpression, *MacroCallExpression:
		state.err("Unexpanded directive expression reached evaluation", expression.expressionLocation(), "Run directive generation before evaluation")
	case *BinaryOpExpression:
		return state.evaluateBinaryOp(value)
	case *UnaryOpExpression:
		return state.evaluateUnaryOp(value)
	case *MatchExpression:
		return state.evaluateMatch(value)
	case *LenExpression:
		return state.evaluateLen(value)
	default:
		state.err(fmt.Sprintf("Unknown expression node %T", expression), expression.expressionLocation(), "Report this as an implementation defect")
	}
	return nil
}

// evaluateString evaluates string against the current evaluator state.
func (state *runtimeState) evaluateString(expression *StringExpression) string {
	if expression.Literal != nil {
		var result strings.Builder
		for _, part := range expression.Literal.Parts {
			switch value := part.(type) {
			case StringText:
				result.WriteString(string(value))
			case *InterpolationRef:
				resolved := state.evaluateInterpolationReference(value.Name, expression.Location)
				result.WriteString(fmt.Sprintf("%v", materializeValue(resolved, true)))
			}
		}
		return result.String()
	}

	content := expression.Multiline
	var result strings.Builder
	for index := 0; index < len(content); {
		if content[index] == '}' && index+1 < len(content) && content[index+1] == '}' {
			result.WriteByte('}')
			index += 2
			continue
		}
		if content[index] != '{' {
			result.WriteByte(content[index])
			index++
			continue
		}
		if index+1 < len(content) && content[index+1] == '{' {
			result.WriteByte('{')
			index += 2
			continue
		}
		end := index + 1
		for end < len(content) && (isAlphaNum(content[end]) || content[end] == '_' || content[end] == '.' || content[end] == '#') {
			end++
		}
		if end == index+1 || end >= len(content) || content[end] != '}' {
			result.WriteByte(content[index])
			index++
			continue
		}
		name := content[index+1 : end]
		resolved := state.evaluateInterpolationReference(name, expression.Location)
		result.WriteString(fmt.Sprintf("%v", materializeValue(resolved, true)))
		index = end + 1
	}
	return result.String()
}

// evaluateInterpolationReference evaluates interpolation reference against the current evaluator state.
func (state *runtimeState) evaluateInterpolationReference(name string, location *SourceLocation) any {
	root, accesses := dottedReference(name, location)
	return state.evaluateReference(&ReferenceExpression{Root: root, Accesses: accesses, Location: location}, nil)
}

// evaluateBinaryOp evaluates binary op against the current evaluator state.
func (state *runtimeState) evaluateBinaryOp(expression *BinaryOpExpression) any {
	left := state.evaluateExpression(expression.Left, nil)

	switch expression.Op {
	case OpAnd:
		leftBool, ok := left.(bool)
		if !ok {
			state.err("#and requires boolean operands", expression.Left.expressionLocation(), "Use boolean expressions")
		}
		if !leftBool {
			return false // short-circuit: don't evaluate right
		}
		right := state.evaluateExpression(expression.Right, nil)
		rightBool, ok := right.(bool)
		if !ok {
			state.err("#and requires boolean operands", expression.Right.expressionLocation(), "Use boolean expressions")
		}
		return rightBool
	case OpOr:
		leftBool, ok := left.(bool)
		if !ok {
			state.err("#or requires boolean operands", expression.Left.expressionLocation(), "Use boolean expressions")
		}
		if leftBool {
			return true // short-circuit: don't evaluate right
		}
		right := state.evaluateExpression(expression.Right, nil)
		rightBool, ok := right.(bool)
		if !ok {
			state.err("#or requires boolean operands", expression.Right.expressionLocation(), "Use boolean expressions")
		}
		return rightBool
	}

	// For comparison operators, both sides must be evaluated
	right := state.evaluateExpression(expression.Right, nil)

	switch expression.Op {
	case OpEQ:
		return runtimeValuesEqual(left, right)
	case OpNEQ:
		return !runtimeValuesEqual(left, right)
	case OpLT:
		return compareValues(left, right, expression.Location, state) < 0
	case OpLE:
		return compareValues(left, right, expression.Location, state) <= 0
	case OpGT:
		return compareValues(left, right, expression.Location, state) > 0
	case OpGE:
		return compareValues(left, right, expression.Location, state) >= 0
	}
	return false
}

// evaluateUnaryOp evaluates unary op against the current evaluator state.
func (state *runtimeState) evaluateUnaryOp(expression *UnaryOpExpression) any {
	operand := state.evaluateExpression(expression.Operand, PrimBoolean)
	operandBool, ok := operand.(bool)
	if !ok {
		state.err("#not requires a boolean operand", expression.Operand.expressionLocation(), "Use a boolean expression")
	}
	return !operandBool
}

// evaluateMatch evaluates match against the current evaluator state.
func (state *runtimeState) evaluateMatch(expression *MatchExpression) any {
	value := state.evaluateExpression(expression.Value, PrimString)
	pattern := state.evaluateExpression(expression.Pattern, PrimString)
	valueStr, ok := value.(string)
	if !ok {
		state.err("#match value must be a string", expression.Value.expressionLocation(), "Use a string expression")
	}
	patternStr, ok := pattern.(string)
	if !ok {
		state.err("#match pattern must be a string", expression.Pattern.expressionLocation(), "Use a string expression containing a valid regex")
	}
	matched, err := regexp.MatchString(patternStr, valueStr)
	if err != nil {
		state.err("#match invalid regex pattern '"+patternStr+"': "+err.Error(), expression.Pattern.expressionLocation(), "Use a valid Go RE2 regex pattern")
	}
	return matched
}

// evaluateLen evaluates len against the current evaluator state.
func (state *runtimeState) evaluateLen(expression *LenExpression) any {
	value := state.evaluateExpression(expression.Value, nil)
	switch v := value.(type) {
	case []any:
		return len(v)
	case *runtimeMapping:
		return len(v.entries)
	default:
		state.err("#len requires an array or mapping, got "+runtimeTypeName(value), expression.Value.expressionLocation(), "Use #len on an array or mapping value")
	}
	return 0
}

// runtimeValuesEqual compares two runtime values for equality.
func runtimeValuesEqual(left, right any) bool {
	switch leftValue := left.(type) {
	case bool:
		rightValue, ok := right.(bool)
		return ok && leftValue == rightValue
	case int:
		switch rightValue := right.(type) {
		case int:
			return leftValue == rightValue
		case float64:
			return float64(leftValue) == rightValue
		}
	case float64:
		switch rightValue := right.(type) {
		case float64:
			return leftValue == rightValue
		case int:
			return leftValue == float64(rightValue)
		}
	case string:
		rightValue, ok := right.(string)
		return ok && leftValue == rightValue
	case *EnumValue:
		rightValue, ok := right.(*EnumValue)
		return ok && leftValue.EnumName == rightValue.EnumName && leftValue.Ordinal == rightValue.Ordinal
	}
	return fmt.Sprintf("%T:%v", left, left) == fmt.Sprintf("%T:%v", right, right)
}

// compareValues performs ordered comparison returning -1/0/1.
func compareValues(left, right any, location *SourceLocation, state *runtimeState) int {
	if state != nil {
		// Type mismatch safety net (should be caught by type checker)
		if fmt.Sprintf("%T", left) != fmt.Sprintf("%T", right) {
			// Allow int/float comparison
			if _, leftIsInt := left.(int); leftIsInt {
				if _, rightIsFloat := right.(float64); rightIsFloat {
					left = float64(left.(int))
				}
			} else if _, leftIsFloat := left.(float64); leftIsFloat {
				if _, rightIsInt := right.(int); rightIsInt {
					right = float64(right.(int))
				}
			}
		}
	}
	switch leftValue := left.(type) {
	case int:
		rightValue, ok := right.(int)
		if !ok {
			return 0
		}
		if leftValue < rightValue {
			return -1
		} else if leftValue > rightValue {
			return 1
		}
		return 0
	case float64:
		rightValue, ok := right.(float64)
		if !ok {
			return 0
		}
		if leftValue < rightValue {
			return -1
		} else if leftValue > rightValue {
			return 1
		}
		return 0
	case string:
		rightValue, ok := right.(string)
		if !ok {
			return 0
		}
		if leftValue < rightValue {
			return -1
		} else if leftValue > rightValue {
			return 1
		}
		return 0
	}
	return 0
}

// dottedReference converts a dotted path into a root name and field-access sequence.
func dottedReference(path string, location *SourceLocation) (string, []Access) {
	parts := strings.Split(path, ".")
	accesses := make([]Access, 0, len(parts)-1)
	for _, part := range parts[1:] {
		accesses = append(accesses, &FieldAccess{Name: part, Location: location})
	}
	return parts[0], accesses
}

// evaluateReference evaluates reference against the current evaluator state.
func (state *runtimeState) evaluateReference(reference *ReferenceExpression, expected TypeRef) any {
	if reference.Root == "" {
		enumType := state.expectedEnum(expected)
		if enumType == nil {
			state.err("Enum shorthand requires a known enum type", reference.Location, "Use the qualified form 'enum_name.member'")
		}
		first, ok := reference.Accesses[0].(*FieldAccess)
		if !ok {
			state.err("Enum shorthand requires a member name", reference.Location, "Use '.member'")
		}
		var value any = state.enumMember(enumType, first.Name, first.Location)
		var valueType TypeRef = enumType
		for _, access := range reference.Accesses[1:] {
			value, valueType = state.resolveTypedAccess(value, valueType, access, reference.Location)
		}
		return value
	}

	var current any
	var currentType TypeRef
	if binding, ok := state.current.lookupBinding(reference.Root); ok {
		current = binding.value
		currentType = binding.typeRef
	} else if typeRef, ok := state.current.lookupType(reference.Root); ok {
		enumType, enumOK := typeRef.(*EnumType)
		if !enumOK || len(reference.Accesses) == 0 {
			state.err("Type '"+reference.Root+"' cannot be used as a value", reference.Location, "Reference an enum member or declare a value")
		}
		first, fieldOK := reference.Accesses[0].(*FieldAccess)
		if !fieldOK {
			state.err("Enum values use member access", reference.Location, "Use 'enum_name.member'")
		}
		current = state.enumMember(enumType, first.Name, first.Location)
		currentType = enumType
		reference = &ReferenceExpression{Root: reference.Root, Accesses: reference.Accesses[1:], Location: reference.Location}
	} else {
		state.err("Undefined variable '"+reference.Root+"'", reference.Location, "Declare it before this reference")
	}

	for _, access := range reference.Accesses {
		current, currentType = state.resolveTypedAccess(current, currentType, access, reference.Location)
	}
	return current
}

// enumMember returns the typed ordinal for a named enum member or raises a diagnostic.
func (state *runtimeState) enumMember(enumType *EnumType, member string, location *SourceLocation) *EnumValue {
	for ordinal, name := range enumType.MemberList {
		if name == member {
			return &EnumValue{Ordinal: ordinal, EnumName: enumType.Name}
		}
	}
	state.err("Undefined enum member '"+member+"' in '"+enumType.Name+"'", location, "Known members: "+strings.Join(enumType.MemberList, ", "))
	return nil
}

// expectedEnum resolves an expected type and returns it only when it is an enum.
func (state *runtimeState) expectedEnum(expected TypeRef) *EnumType {
	resolved := state.resolveType(expected, nil)
	enumType, _ := resolved.(*EnumType)
	return enumType
}

// resolveAccess resolves access from the supplied context.
func (state *runtimeState) resolveAccess(current any, access Access, location *SourceLocation) any {
	value, _ := state.resolveTypedAccess(current, nil, access, location)
	return value
}

// resolveTypedAccess resolves typed access from the supplied context.
func (state *runtimeState) resolveTypedAccess(current any, currentType TypeRef, access Access, location *SourceLocation) (any, TypeRef) {
	switch access := access.(type) {
	case *FieldAccess:
		switch value := current.(type) {
		case *runtimeObject:
			binding, ok := value.environment.bindings[access.Name]
			if !ok {
				state.err("Undefined field '"+access.Name+"'", access.Location, "Known fields: "+strings.Join(sortedRuntimeBindingKeys(value.environment.bindings), ", "))
			}
			return binding.value, binding.typeRef
		case *EnumValue:
			if access.Name != "value" {
				state.err("Enum values only expose '.value'", access.Location, "Use '.value' for a tagged enum")
			}
			typeRef, ok := state.current.lookupType(value.EnumName)
			if !ok {
				state.err("Unknown enum type '"+value.EnumName+"'", access.Location, "Declare the enum before use")
			}
			enumType := typeRef.(*EnumType)
			if enumType.ValueType == nil {
				state.err("Enum '"+enumType.Name+"' has no tagged value", access.Location, "Remove '.value' or use a tagged enum")
			}
			member := enumType.MemberList[value.Ordinal]
			return state.evaluateExpression(enumType.Members[member], enumType.ValueType), enumType.ValueType
		default:
			state.err("Cannot access field '"+access.Name+"' on "+runtimeTypeName(current), access.Location, "Use field access on a scope, namespace, setup, or tagged enum")
		}
	case *IndexAccess:
		switch value := current.(type) {
		case []any:
			index := state.evaluateExpression(access.Index, state.collectionIndexType(currentType, access.Location))
			position := state.integerIndex(index, access.Location)
			if position < 0 || position >= len(value) {
				state.err(fmt.Sprintf("Index %d out of bounds for array of length %d", position, len(value)), access.Location, "Use an index within the array bounds")
			}
			return value[position], state.collectionElementType(currentType, access.Location)
		case *runtimeMapping:
			keyType := value.keyType
			valueType := value.valueType
			if mappingType, ok := state.resolveType(currentType, access.Location).(*MappingType); ok {
				keyType = mappingType.KeyType
				valueType = mappingType.ValueType
			}
			index := state.evaluateExpression(access.Index, keyType)
			result, ok := value.lookup(index)
			if !ok {
				state.err("Mapping key not found", access.Location, "Use a key declared in the mapping literal")
			}
			return result, valueType
		default:
			state.err("Cannot index "+runtimeTypeName(current), access.Location, "Use indexing on an array or mapping")
		}
	}
	state.err("Unknown access node", location, "Report this as an implementation defect")
	return nil, nil
}

// collectionIndexType returns the accepted index or key type of a collection.
func (state *runtimeState) collectionIndexType(typeRef TypeRef, location *SourceLocation) TypeRef {
	switch collectionType := state.resolveType(typeRef, location).(type) {
	case *ArrayType:
		return PrimInteger
	case *EnumKeyType:
		indexType, ok := state.current.lookupType(collectionType.EnumName)
		if !ok {
			state.err("Unknown enum index type '"+collectionType.EnumName+"'", location, "Declare the enum before the array")
		}
		return indexType
	case *MappingType:
		return collectionType.KeyType
	}
	return nil
}

// collectionElementType returns the element or value type of an array, enum-keyed array, or mapping.
func (state *runtimeState) collectionElementType(typeRef TypeRef, location *SourceLocation) TypeRef {
	switch collectionType := state.resolveType(typeRef, location).(type) {
	case *ArrayType:
		return collectionType.ElementType
	case *EnumKeyType:
		return collectionType.ElementType
	case *MappingType:
		return collectionType.ValueType
	}
	return nil
}

// integerIndex converts an integer or enum value to an array index.
func (state *runtimeState) integerIndex(index any, location *SourceLocation) int {
	switch value := index.(type) {
	case int:
		return value
	case *EnumValue:
		return value.Ordinal
	default:
		state.err("Array index must be an integer or enum member", location, "Use an integer or the configured enum index type")
	}
	return 0
}

// evaluateArray evaluates array against the current evaluator state.
func (state *runtimeState) evaluateArray(expression *ArrayExpression, expected TypeRef) any {
	typeRef := expression.DeclaredType
	if typeRef == nil {
		typeRef = expected
	}
	typeRef = state.resolveType(typeRef, expression.Location)
	var elementType TypeRef
	switch arrayType := typeRef.(type) {
	case *ArrayType:
		elementType = arrayType.ElementType
	case *EnumKeyType:
		elementType = arrayType.ElementType
	}
	result := make([]any, len(expression.Elements))
	for index, element := range expression.Elements {
		result[index] = state.evaluateExpression(element, elementType)
	}
	return result
}

// evaluateMapping evaluates mapping against the current evaluator state.
func (state *runtimeState) evaluateMapping(expression *MappingExpression, expected TypeRef) any {
	var typeRef TypeRef
	if expression.DeclaredType != nil {
		typeRef = expression.DeclaredType
	} else {
		typeRef = expected
	}
	typeRef = state.resolveType(typeRef, expression.Location)
	var keyType, valueType TypeRef
	if mappingType, ok := typeRef.(*MappingType); ok {
		keyType = mappingType.KeyType
		valueType = mappingType.ValueType
	}
	mapping := &runtimeMapping{keyType: keyType, valueType: valueType}
	for _, entry := range expression.Entries {
		keys := make([]any, len(entry.Keys))
		for index, key := range entry.Keys {
			keys[index] = state.evaluateExpression(key, keyType)
		}
		var key any
		if entry.IsComposite {
			parts := make([]string, len(keys))
			for index, component := range keys {
				parts[index] = fmt.Sprintf("%v", materializeValue(component, true))
			}
			key = strings.Join(parts, ",")
		} else {
			key = keys[0]
		}
		mapping.entries = append(mapping.entries, runtimeMappingEntry{key: key, value: state.evaluateExpression(entry.Value, valueType)})
	}
	return mapping
}

// evaluateStruct evaluates struct against the current evaluator state.
func (state *runtimeState) evaluateStruct(expression *StructExpression, expected TypeRef) any {
	var setup *SetupType
	if expression.TypeName != "" {
		typeRef, ok := state.current.lookupType(expression.TypeName)
		if !ok {
			state.err("Unknown setup type '"+expression.TypeName+"'", expression.Location, "Declare the setup before using it")
		}
		setup, ok = typeRef.(*SetupType)
		if !ok {
			state.err("Type '"+expression.TypeName+"' is not a setup", expression.Location, "Use a setup type for a struct literal")
		}
	} else {
		resolved := state.resolveType(expected, expression.Location)
		setup, _ = resolved.(*SetupType)
	}

	environment := newRuntimeEnvironment(state.current)
	object := &runtimeObject{environment: environment}
	if setup == nil {
		for _, field := range expression.Fields {
			environment.bindings[field.Name] = &runtimeBinding{value: state.evaluateExpression(field.Value, nil)}
		}
		return object
	}
	object.typeName = setup.Name
	provided := make(map[string]bool)
	for _, field := range expression.Fields {
		definition, ok := setup.Fields[field.Name]
		if !ok {
			state.err("Unknown field '"+field.Name+"' in setup '"+setup.Name+"'", field.Location, "Known fields: "+strings.Join(sortedFieldDefinitionKeys(setup.Fields), ", "))
		}
		provided[field.Name] = true
		fieldType := state.resolveType(definition.DeclaredType, definition.Location)
		environment.bindings[field.Name] = &runtimeBinding{value: state.evaluateExpression(field.Value, fieldType), typeRef: fieldType}
	}
	for _, name := range sortedFieldDefinitionKeys(setup.Fields) {
		if provided[name] {
			continue
		}
		definition := setup.Fields[name]
		if !definition.Optional {
			state.err("Struct literal missing required field '"+name+"' in '"+setup.Name+"'", expression.Location, "Add '"+name+" = <value>'")
		}
		fieldType := state.resolveType(definition.DeclaredType, definition.Location)
		defaultValue := state.evaluateExpression(definition.DefaultValue, fieldType)
		environment.bindings[name] = &runtimeBinding{value: defaultValue, typeRef: fieldType}
	}

	// Evaluate any stored assertions for this setup type in the object's environment
	if setup != nil {
		if body, hasAssertions := state.assertions[setup.Name]; hasAssertions {
			previousLoc := state.instanceLocation
			state.instanceLocation = expression.Location
			previous := state.current
			state.current = environment
			state.evaluateStatements(body)
			state.current = previous
			state.instanceLocation = previousLoc
		}
	}

	return object
}

// resolveType resolves type from the supplied context.
func (state *runtimeState) resolveType(typeRef TypeRef, location *SourceLocation) TypeRef {
	if typeRef == nil {
		return nil
	}
	switch value := typeRef.(type) {
	case TypeName:
		resolved, ok := state.current.lookupType(string(value))
		if !ok {
			state.err("Unknown type '"+string(value)+"'", location, "Declare the type before this use")
		}
		return resolved
	case *ArrayType:
		return &ArrayType{ElementType: state.resolveType(value.ElementType, location)}
	case *EnumKeyType:
		return &EnumKeyType{EnumName: value.EnumName, ElementType: state.resolveType(value.ElementType, location)}
	case *MappingType:
		return &MappingType{KeyType: state.resolveType(value.KeyType, location), ValueType: state.resolveType(value.ValueType, location)}
	default:
		return typeRef
	}
}

// runtimeType derives a SOMETHING type reference from an evaluated value.
func (state *runtimeState) runtimeType(value any) TypeRef {
	switch value := value.(type) {
	case string:
		return PrimString
	case int:
		return PrimInteger
	case float64:
		return PrimFloat
	case bool:
		return PrimBoolean
	case []any:
		if len(value) == 0 {
			return &ArrayType{}
		}
		return &ArrayType{ElementType: state.runtimeType(value[0])}
	case *runtimeMapping:
		if value.keyType != nil || value.valueType != nil {
			return &MappingType{KeyType: value.keyType, ValueType: value.valueType}
		}
		if len(value.entries) == 0 {
			return &MappingType{}
		}
		return &MappingType{KeyType: state.runtimeType(value.entries[0].key), ValueType: state.runtimeType(value.entries[0].value)}
	case *runtimeObject:
		if value.typeName != "" {
			if typeRef, ok := state.current.lookupType(value.typeName); ok {
				return typeRef
			}
		}
		fields := make(map[string]*BindingType, len(value.environment.bindings))
		for name, binding := range value.environment.bindings {
			fields[name] = &BindingType{Type: binding.typeRef, Private: binding.private}
		}
		if value.kind == PrimNamespace {
			types := make(map[string]TypeRef, len(value.environment.types))
			for name, typeRef := range value.environment.types {
				types[name] = typeRef
			}
			return &NamespaceType{Fields: fields, Types: types}
		}
		return &ScopeType{Fields: fields}
	case *EnumValue:
		if typeRef, ok := state.current.lookupType(value.EnumName); ok {
			return typeRef
		}
	}
	return nil
}

// publicMap materializes the non-private bindings in a runtime environment.
func (state *runtimeState) publicMap(environment *runtimeEnvironment) map[string]any {
	result := make(map[string]any)
	for name, binding := range environment.bindings {
		if binding.private {
			continue
		}
		result[name] = materializeValue(binding.value, false)
	}
	return result
}

// materializeValue recursively converts internal runtime values to public Go values.
func materializeValue(value any, includePrivate bool) any {
	switch value := value.(type) {
	case *runtimeObject:
		result := make(map[string]any)
		for name, binding := range value.environment.bindings {
			if binding.private && !includePrivate {
				continue
			}
			result[name] = materializeValue(binding.value, includePrivate)
		}
		return result
	case *runtimeMapping:
		allInteger := value.keyType == PrimInteger
		if _, ok := value.keyType.(*EnumType); ok {
			allInteger = true
		}
		if value.keyType == nil && len(value.entries) > 0 {
			allInteger = true
		}
		for _, entry := range value.entries {
			switch entry.key.(type) {
			case int, *EnumValue:
			default:
				allInteger = false
			}
		}
		if allInteger {
			result := make(map[int]any)
			for _, entry := range value.entries {
				key := entry.key
				if enum, ok := key.(*EnumValue); ok {
					key = enum.Ordinal
				}
				result[key.(int)] = materializeValue(entry.value, includePrivate)
			}
			return result
		}
		result := make(map[string]any)
		for _, entry := range value.entries {
			result[fmt.Sprintf("%v", materializeValue(entry.key, true))] = materializeValue(entry.value, includePrivate)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for index, element := range value {
			result[index] = materializeValue(element, includePrivate)
		}
		return result
	case *EnumValue:
		return value.Ordinal
	default:
		return value
	}
}

// runtimeTypeName returns the user-facing type name of an evaluated value.
func runtimeTypeName(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case int:
		return "integer"
	case float64:
		return "float"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case *runtimeMapping:
		return "mapping"
	case *runtimeObject:
		return "object"
	case *EnumValue:
		return "enum"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// sortedRuntimeBindingKeys returns runtime binding keys in deterministic order.
func sortedRuntimeBindingKeys(values map[string]*runtimeBinding) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedMapKeys returns map keys in deterministic order.
func sortedMapKeys(values map[string]any) []string {
	keys := mapKeys(values)
	sort.Strings(keys)
	return keys
}

// mapKeys returns all keys from a string-keyed runtime map.
func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

// sortedFieldDefinitionKeys returns field definition keys in deterministic order.
func sortedFieldDefinitionKeys(values map[string]*FieldDefinition) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// isValidTimestamp reports whether a string matches the supported timestamp shape.
func isValidTimestamp(value string) bool {
	if len(value) != 19 && len(value) != 23 {
		return false
	}
	if value[4] != '-' || value[7] != '-' || value[10] != ' ' || value[13] != ':' || value[16] != ':' {
		return false
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17, 18} {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	if len(value) == 23 {
		if value[19] != '.' {
			return false
		}
		for _, index := range []int{20, 21, 22} {
			if value[index] < '0' || value[index] > '9' {
				return false
			}
		}
	}
	return true
}

// typeNameOf returns the SOMETHING runtime type name for a value.
func typeNameOf(value any) string {
	return runtimeTypeName(value)
}

// typeRefDisplayName returns a diagnostic name for a type reference.
func typeRefDisplayName(typeRef TypeRef) string {
	if typeRef == nil {
		return "?"
	}
	return typeRefString(typeRef)
}

// isAlphaNum reports whether a byte is an ASCII letter or digit.
func isAlphaNum(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

// indexOf returns the first matching string index, or minus one when absent.
func indexOf(values []string, expected string) int {
	for index, value := range values {
		if value == expected {
			return index
		}
	}
	return -1
}
