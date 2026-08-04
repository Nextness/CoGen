// directive_generator.go executes compile-time directives and produces an AST
// containing only ordered assignments. It uses temporary runtime state for
// directive inputs, then discards that state before full checking/evaluation.
package something

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// macroEnvironment stores lexically nested macro definitions for directive expansion.
type macroEnvironment struct {
	parent      *macroEnvironment
	definitions map[string]*MacroDirective
}

// newMacroEnvironment creates an empty macro scope linked to its parent.
func newMacroEnvironment(parent *macroEnvironment) *macroEnvironment {
	return &macroEnvironment{parent: parent, definitions: make(map[string]*MacroDirective)}
}

// lookup searches the current and enclosing macro scopes for a definition.
func (environment *macroEnvironment) lookup(name string) (*MacroDirective, bool) {
	for current := environment; current != nil; current = current.parent {
		if macro, ok := current.definitions[name]; ok {
			return macro, true
		}
	}
	return nil, false
}

// DirectiveGenerator expands directives from one parsed syntax tree.
type DirectiveGenerator struct {
	filepath          string
	runtime           *runtimeState
	macros            *macroEnvironment
	includeStack      []string
	parsedIncludes    map[string]*Program
	bareIncludes      map[string]bool
	macroStack        []string
	iterationCount    int
	iterationCounters map[string]int
	constantValues    []map[string]any
}

// NewDirectiveGenerator constructs directive generator.
func NewDirectiveGenerator(filepath string) *DirectiveGenerator {
	return &DirectiveGenerator{
		filepath:          filepath,
		runtime:           newRuntimeState(filepath, nil),
		macros:            newMacroEnvironment(nil),
		parsedIncludes:    make(map[string]*Program),
		bareIncludes:      make(map[string]bool),
		iterationCounters: make(map[string]int),
	}
}

// err panics with a source-located SomethingError for a directive failure.
func (generator *DirectiveGenerator) err(message string, location *SourceLocation, suggestion string) {
	panic(errLoc(message, location, generator.filepath, suggestion))
}

// Expand removes every directive and preserves the generated assignment order.
func (generator *DirectiveGenerator) Expand(program *Program) *Program {
	if program == nil {
		return &Program{Filepath: generator.filepath}
	}
	if generator.filepath == "" {
		generator.filepath = program.Filepath
		generator.runtime.filepath = generator.filepath
	}
	if program.Filepath != "" {
		generator.includeStack = append(generator.includeStack, cleanKnownPath(program.Filepath))
	}
	NewTypeChecker(program, program.Filepath).detectDependencyCycles()
	statements := generator.expandStatements(program.Statements, false)
	return &Program{Statements: statements, Filepath: program.Filepath}
}

// expandStatements expands statements into its structural AST form.
func (generator *DirectiveGenerator) expandStatements(statements []Statement, inheritedPrivate bool) []Statement {
	result := []Statement{}
	for _, statement := range statements {
		private := inheritedPrivate || statement.statementBase().Private
		switch node := statement.(type) {
		case *Assignment:
			result = append(result, generator.expandAssignment(node, private))
		case *IncludeDirective:
			result = append(result, generator.expandBareInclude(node, private)...)
		case *ForDirective:
			result = append(result, generator.expandFor(node, private)...)
		case *InsertDirective:
			result = append(result, generator.expandInsert(node, private)...)
		case *MacroDirective:
			generator.registerMacro(node)
		case *AssertDirective:
			result = append(result, node)
		case *IfDirective:
			result = append(result, node)
		case *ErrorDirective:
			result = append(result, node)
		default:
			generator.err(fmt.Sprintf("Unknown directive AST node %T", statement), statement.statementBase().Location, "Report this as an implementation defect")
		}
	}
	return result
}

// expandAssignment expands assignment into its structural AST form.
func (generator *DirectiveGenerator) expandAssignment(source *Assignment, private bool) *Assignment {
	assignment := &Assignment{
		StatementBase: StatementBase{Private: private, Location: source.Location},
		Mode:          source.Mode,
		DeclaredType:  source.DeclaredType,
	}
	assignment.Target = generator.expandLValue(source.Target, source.Location)

	switch value := source.Value.(type) {
	case *ScopeExpression:
		assignment.Value = generator.expandScopeValue(assignment, value, PrimScope)
		return assignment
	case *SetupDefinition:
		assignment.Value = generator.expandSetupDefinition(value)
	case *EnumDefinition:
		assignment.Value = generator.expandEnumDefinition(value)
	case Expression:
		assignment.Value = generator.expandExpression(value)
	default:
		generator.err(fmt.Sprintf("Unknown assignment value %T", source.Value), source.Location, "Report this as an implementation defect")
	}
	generator.validateAndApply(assignment)
	return assignment
}

// expandScopeValue expands scope value into its structural AST form.
func (generator *DirectiveGenerator) expandScopeValue(assignment *Assignment, source *ScopeExpression, kind PrimitiveKind) *ScopeExpression {
	objectEnvironment := newRuntimeEnvironment(generator.runtime.current)
	object := &runtimeObject{environment: objectEnvironment, kind: kind}
	typeRef := TypeRef(&ScopeType{Fields: make(map[string]*BindingType)})
	if kind == PrimNamespace {
		typeRef = &NamespaceType{Fields: make(map[string]*BindingType), Types: make(map[string]TypeRef)}
	}
	if assignment.Mode == AssignExisting {
		generator.runtime.reassign(assignment.Target, object, assignment.Location)
	} else {
		generator.runtime.assignValue(assignment, object, typeRef)
	}
	previousRuntime := generator.runtime.current
	previousMacros := generator.macros
	generator.runtime.current = objectEnvironment
	generator.macros = newMacroEnvironment(previousMacros)
	statements := generator.expandStatements(source.Statements, false)
	generator.macros = previousMacros
	generator.runtime.current = previousRuntime
	return &ScopeExpression{Statements: statements, Location: source.Location}
}

// expandSetupDefinition expands setup definition into its structural AST form.
func (generator *DirectiveGenerator) expandSetupDefinition(source *SetupDefinition) *SetupDefinition {
	fields := make([]*FieldDefinition, len(source.Fields))
	for index, field := range source.Fields {
		cpy := *field
		if cpy.DefaultValue != nil {
			cpy.DefaultValue = generator.expandExpression(cpy.DefaultValue)
		}
		fields[index] = &cpy
	}
	return &SetupDefinition{Fields: fields, Location: source.Location}
}

// expandEnumDefinition expands enum definition into its structural AST form.
func (generator *DirectiveGenerator) expandEnumDefinition(source *EnumDefinition) *EnumDefinition {
	members := make([]*EnumMember, len(source.Members))
	for index, member := range source.Members {
		cpy := *member
		if cpy.Value != nil {
			cpy.Value = generator.expandExpression(cpy.Value)
		}
		members[index] = &cpy
	}
	return &EnumDefinition{ValueType: source.ValueType, Members: members, Location: source.Location}
}

// expandLValue expands l value into its structural AST form.
func (generator *DirectiveGenerator) expandLValue(target LValue, location *SourceLocation) LValue {
	switch target := target.(type) {
	case *IdentifierLValue:
		return &IdentifierLValue{Name: target.Name}
	case *MemberLValue:
		if _, temporary := generator.lookupConstant(target.Root); temporary {
			generator.err("A #for variable cannot be used as an assignment destination", location, "Generate a stable destination with #iteration or #as_lvalue")
		}
		return &MemberLValue{Root: target.Root, Accesses: generator.expandAccesses(target.Accesses)}
	case *IterationLValue:
		label := generator.directiveLabel(target.Label, "#iteration", location)
		return &IdentifierLValue{Name: generator.nextIterationKey(label)}
	case *AsLValue:
		nameExpression := generator.expandExpression(target.Name)
		nameValue := generator.runtime.evaluateExpression(nameExpression, PrimString)
		name, ok := nameValue.(string)
		if !ok || name == "" {
			generator.err("#as_lvalue requires a non-empty string", target.Name.expressionLocation(), "Pass a string or a variable containing a string")
		}
		return generator.parseGeneratedLValue(name, location)
	default:
		generator.err(fmt.Sprintf("Unknown lvalue node %T", target), location, "Report this as an implementation defect")
	}
	return nil
}

// expandAccesses expands accesses into its structural AST form.
func (generator *DirectiveGenerator) expandAccesses(accesses []Access) []Access {
	result := make([]Access, len(accesses))
	for index, access := range accesses {
		switch access := access.(type) {
		case *FieldAccess:
			result[index] = &FieldAccess{Name: access.Name, Location: access.Location}
		case *IndexAccess:
			result[index] = &IndexAccess{Index: generator.expandExpression(access.Index), Location: access.Location}
		}
	}
	return result
}

// parseGeneratedLValue parses generated l value from the supplied input.
func (generator *DirectiveGenerator) parseGeneratedLValue(name string, location *SourceLocation) LValue {
	tokens := NewLexer(name, location.Filepath).Tokenize()
	parser := NewParser(tokens, location.Filepath)
	if parser.peek(0).Kind != TkIDENTIFIER {
		generator.err("#as_lvalue produced an invalid assignment target '"+name+"'", location, "Generate an identifier or member path")
	}
	target := parser.parseNamedLValue()
	if parser.peek(0).Kind != TkEOF {
		generator.err("#as_lvalue produced trailing syntax in '"+name+"'", location, "Generate only one identifier or member path")
	}
	return target
}

// expandExpression expands expression into its structural AST form.
func (generator *DirectiveGenerator) expandExpression(expression Expression) Expression {
	if expression == nil {
		return nil
	}
	switch value := expression.(type) {
	case *StringExpression:
		return generator.expandString(value)
	case *IntegerExpression:
		return &IntegerExpression{Value: value.Value, Location: value.Location}
	case *FloatExpression:
		return &FloatExpression{Value: value.Value, Location: value.Location}
	case *BooleanExpression:
		return &BooleanExpression{Value: value.Value, Location: value.Location}
	case *ReferenceExpression:
		if constant, ok := generator.lookupConstant(value.Root); ok {
			resolved := constant
			for _, access := range value.Accesses {
				resolved = generator.runtime.resolveAccess(resolved, access, value.Location)
			}
			return generator.expressionFromRuntime(resolved, value.Location)
		}
		return &ReferenceExpression{Root: value.Root, Accesses: generator.expandAccesses(value.Accesses), Location: value.Location}
	case *ArrayExpression:
		elements := make([]Expression, len(value.Elements))
		for index, element := range value.Elements {
			elements[index] = generator.expandExpression(element)
		}
		return &ArrayExpression{DeclaredType: value.DeclaredType, Elements: elements, Location: value.Location}
	case *MappingExpression:
		entries := make([]*MappingEntry, len(value.Entries))
		for index, entry := range value.Entries {
			keys := make([]Expression, len(entry.Keys))
			for keyIndex, key := range entry.Keys {
				keys[keyIndex] = generator.expandExpression(key)
			}
			entries[index] = &MappingEntry{Keys: keys, Value: generator.expandExpression(entry.Value), IsComposite: entry.IsComposite, Location: entry.Location}
		}
		return &MappingExpression{DeclaredType: value.DeclaredType, Entries: entries, Location: value.Location}
	case *StructExpression:
		fields := make([]*FieldAssignment, len(value.Fields))
		for index, field := range value.Fields {
			fields[index] = &FieldAssignment{Name: field.Name, Value: generator.expandExpression(field.Value), Location: field.Location}
		}
		return &StructExpression{TypeName: value.TypeName, Fields: fields, Location: value.Location}
	case *IncludeExpression:
		return generator.expandNamespaceInclude(value)
	case *IterationExpression:
		label := generator.directiveLabel(value.Label, "#iteration", value.Location)
		return stringExpression(generator.peekIterationKey(label), value.Location)
	case *MacroCallExpression:
		return generator.expandMacroCall(value)
	case *NamespaceExpression:
		return value
	case *TypedExpression:
		return &TypedExpression{Value: generator.expandExpression(value.Value), Type: value.Type, Location: value.Location}
	case *BinaryOpExpression:
		value.Left = generator.expandExpression(value.Left)
		value.Right = generator.expandExpression(value.Right)
		return value
	case *UnaryOpExpression:
		value.Operand = generator.expandExpression(value.Operand)
		return value
	case *MatchExpression:
		value.Value = generator.expandExpression(value.Value)
		value.Pattern = generator.expandExpression(value.Pattern)
		return value
	case *LenExpression:
		value.Value = generator.expandExpression(value.Value)
		return value
	default:
		generator.err(fmt.Sprintf("Unknown expression node %T", expression), expression.expressionLocation(), "Report this as an implementation defect")
	}
	return nil
}

// expandString expands string into its structural AST form.
func (generator *DirectiveGenerator) expandString(source *StringExpression) Expression {
	if source.Literal != nil {
		parts := []StringPart{}
		for _, part := range source.Literal.Parts {
			reference, ok := part.(*InterpolationRef)
			if !ok {
				parts = append(parts, part)
				continue
			}
			root := strings.SplitN(reference.Name, ".", 2)[0]
			if _, exists := generator.lookupConstant(root); !exists {
				parts = append(parts, &InterpolationRef{Name: reference.Name})
				continue
			}
			resolved := generator.evaluateConstantReference(reference.Name, source.Location)
			parts = append(parts, StringText(fmt.Sprintf("%v", materializeValue(resolved, true))))
		}
		return &StringExpression{Literal: &StringLiteral{Parts: parts}, Location: source.Location}
	}

	content := source.Multiline
	var result strings.Builder
	for index := 0; index < len(content); {
		if content[index] != '{' {
			result.WriteByte(content[index])
			index++
			continue
		}
		end := index + 1
		for end < len(content) && (isAlphaNum(content[end]) || content[end] == '_' || content[end] == '.') {
			end++
		}
		if end == index+1 || end >= len(content) || content[end] != '}' {
			result.WriteByte(content[index])
			index++
			continue
		}
		path := content[index+1 : end]
		root := strings.SplitN(path, ".", 2)[0]
		if _, exists := generator.lookupConstant(root); !exists {
			result.WriteString(content[index : end+1])
			index = end + 1
			continue
		}
		resolved := generator.evaluateConstantReference(path, source.Location)
		result.WriteString(fmt.Sprintf("%v", materializeValue(resolved, true)))
		index = end + 1
	}
	return &StringExpression{Multiline: result.String(), Location: source.Location}
}

// directiveLabel expands and evaluates an optional directive label as a string.
func (generator *DirectiveGenerator) directiveLabel(expression Expression, directive string, location *SourceLocation) string {
	if expression == nil {
		return ""
	}
	expanded := generator.expandExpression(expression)
	value := generator.runtime.evaluateExpression(expanded, PrimString)
	label, ok := value.(string)
	if !ok {
		generator.err(directive+" label must be a string", expression.expressionLocation(), "Use a string literal or string variable")
	}
	return label
}

// evaluateConstantReference evaluates constant reference against the current evaluator state.
func (generator *DirectiveGenerator) evaluateConstantReference(path string, location *SourceLocation) any {
	root, accesses := dottedReference(path, location)
	value, ok := generator.lookupConstant(root)
	if !ok {
		generator.err("Unknown generated constant '"+root+"'", location, "Declare the #for variable before use")
	}
	for _, access := range accesses {
		value = generator.runtime.resolveAccess(value, access, location)
	}
	return value
}

// nextIterationKey returns and advances the deterministic counter for an iteration label.
func (generator *DirectiveGenerator) nextIterationKey(label string) string {
	counter := generator.iterationCount
	if label == "" {
		generator.iterationCount++
	} else {
		counter = generator.iterationCounters[label]
		generator.iterationCounters[label] = counter + 1
	}
	return fmt.Sprintf("iteration_%010d%s", counter, label)
}

// peekIterationKey returns the next deterministic key for an iteration label without advancing it.
func (generator *DirectiveGenerator) peekIterationKey(label string) string {
	counter := generator.iterationCount
	if label != "" {
		counter = generator.iterationCounters[label]
	}
	return fmt.Sprintf("iteration_%010d%s", counter, label)
}

// validateAndApply type-checks an expanded assignment against the temporary runtime and applies it.
func (generator *DirectiveGenerator) validateAndApply(assignment *Assignment) {
	switch assignment.Value.(type) {
	case *EnumDefinition, *SetupDefinition:
		generator.runtime.evaluateAssignment(assignment)
		return
	}
	expression, ok := assignment.Value.(Expression)
	if !ok {
		generator.runtime.evaluateAssignment(assignment)
		return
	}

	var expected TypeRef
	switch assignment.Mode {
	case AssignExplicit:
		expected = generator.runtime.resolveType(assignment.DeclaredType, assignment.Location)
	case AssignExisting:
		expected = generator.runtimeTargetType(assignment.Target, assignment.Location)
	}
	if typed, ok := expression.(*TypedExpression); ok && expected == nil {
		expected = generator.runtime.resolveType(typed.Type, typed.Location)
	}
	value := generator.runtime.evaluateExpression(expression, expected)
	generator.runtime.assignValue(assignment, value, expected)
}

// runtimeTargetType resolves the current type of an assignment target during expansion.
func (generator *DirectiveGenerator) runtimeTargetType(target LValue, location *SourceLocation) TypeRef {
	switch target := target.(type) {
	case *IdentifierLValue:
		binding, ok := generator.runtime.current.lookupBinding(target.Name)
		if !ok {
			generator.err("Cannot reassign undeclared variable '"+target.Name+"'", location, "Declare it before using '='")
		}
		return binding.typeRef
	case *MemberLValue:
		container, containerType, final := generator.runtime.resolveLValueContainer(target.Root, target.Accesses, location)
		switch access := final.(type) {
		case *FieldAccess:
			object, ok := container.(*runtimeObject)
			if !ok {
				generator.err("Field reassignment requires an object", access.Location, "Check the target path")
			}
			binding, ok := object.environment.bindings[access.Name]
			if !ok {
				generator.err("Cannot reassign undeclared member '"+access.Name+"'", access.Location, "Declare it before using '='")
			}
			return binding.typeRef
		case *IndexAccess:
			return generator.runtime.collectionElementType(containerType, access.Location)
		}
	}
	return nil
}

// runtimeValueAssignable reports whether an evaluated directive value conforms to an expected type.
func (generator *DirectiveGenerator) runtimeValueAssignable(expected TypeRef, value any) bool {
	expected = generator.runtime.resolveType(expected, nil)
	switch typeRef := expected.(type) {
	case PrimitiveKind:
		switch typeRef {
		case PrimString:
			_, ok := value.(string)
			return ok
		case PrimInteger:
			_, ok := value.(int)
			return ok
		case PrimBoolean:
			_, ok := value.(bool)
			return ok
		case PrimFloat:
			switch value.(type) {
			case int, float64:
				return true
			}
		case PrimTimestamp:
			text, ok := value.(string)
			return ok && isValidTimestamp(text)
		case PrimScope:
			object, ok := value.(*runtimeObject)
			return ok && object.kind != PrimNamespace
		case PrimNamespace:
			object, ok := value.(*runtimeObject)
			return ok && object.kind == PrimNamespace
		}
	case *EnumType:
		enum, ok := value.(*EnumValue)
		return ok && enum.EnumName == typeRef.Name
	case *SetupType:
		object, ok := value.(*runtimeObject)
		return ok && object.typeName == typeRef.Name
	case *ArrayType:
		values, ok := value.([]any)
		if !ok {
			return false
		}
		for _, element := range values {
			if !generator.runtimeValueAssignable(typeRef.ElementType, element) {
				return false
			}
		}
		return true
	case *EnumKeyType:
		values, ok := value.([]any)
		if !ok {
			return false
		}
		for _, element := range values {
			if !generator.runtimeValueAssignable(typeRef.ElementType, element) {
				return false
			}
		}
		return true
	case *MappingType:
		mapping, ok := value.(*runtimeMapping)
		if !ok {
			return false
		}
		for _, entry := range mapping.entries {
			if !generator.runtimeValueAssignable(typeRef.KeyType, entry.key) || !generator.runtimeValueAssignable(typeRef.ValueType, entry.value) {
				return false
			}
		}
		return true
	case *ScopeType, *NamespaceType:
		_, ok := value.(*runtimeObject)
		return ok
	}
	return false
}

// expandBareInclude expands bare include into its structural AST form.
func (generator *DirectiveGenerator) expandBareInclude(include *IncludeDirective, private bool) []Statement {
	resolved := generator.resolveIncludePath(include.Filepath, include.Location)
	if generator.pathInIncludeStack(resolved) {
		generator.includeCycleError(resolved, include.Location)
	}
	if generator.bareIncludes[resolved] {
		return nil
	}
	generator.bareIncludes[resolved] = true
	program := generator.loadInclude(resolved, include.Location)
	generator.includeStack = append(generator.includeStack, resolved)
	statements := generator.expandStatements(program.Statements, private)
	generator.includeStack = generator.includeStack[:len(generator.includeStack)-1]
	return statements
}

// expandNamespaceInclude expands namespace include into its structural AST form.
func (generator *DirectiveGenerator) expandNamespaceInclude(include *IncludeExpression) Expression {
	resolved := generator.resolveIncludePath(include.Filepath, include.Location)
	if generator.pathInIncludeStack(resolved) {
		generator.includeCycleError(resolved, include.Location)
	}
	program := generator.loadInclude(resolved, include.Location)
	previousRuntime := generator.runtime.current
	previousMacros := generator.macros
	child := newRuntimeEnvironment(previousRuntime)
	generator.runtime.current = child
	generator.macros = newMacroEnvironment(previousMacros)
	generator.includeStack = append(generator.includeStack, resolved)
	statements := generator.expandStatements(program.Statements, false)
	generator.includeStack = generator.includeStack[:len(generator.includeStack)-1]
	generator.macros = previousMacros
	generator.runtime.current = previousRuntime
	return &NamespaceExpression{Statements: statements, Location: include.Location}
}

// resolveIncludePath resolves include path from the supplied context.
func (generator *DirectiveGenerator) resolveIncludePath(path string, location *SourceLocation) string {
	baseFile := generator.filepath
	if location != nil && location.Filepath != "" {
		baseFile = location.Filepath
	}
	if baseFile != "" {
		return cleanKnownPath(filepath.Join(filepath.Dir(baseFile), path))
	}
	return cleanKnownPath(path)
}

// cleanKnownPath cleans a path and makes it absolute when the runtime can resolve it.
func cleanKnownPath(path string) string {
	cleaned := filepath.Clean(path)
	if absolute, err := filepath.Abs(cleaned); err == nil {
		return absolute
	}
	return cleaned
}

// loadInclude loads include from the supplied source.
func (generator *DirectiveGenerator) loadInclude(path string, location *SourceLocation) *Program {
	if program, ok := generator.parsedIncludes[path]; ok {
		return program
	}
	data, err := os.ReadFile(path)
	if err != nil {
		generator.err("Included file not found: '"+path+"'", location, "Check the #include path")
	}
	program := NewParser(NewLexer(string(data), path).Tokenize(), path).ParseProgram()
	NewTypeChecker(program, path).detectDependencyCycles()
	generator.parsedIncludes[path] = program
	return program
}

// pathInIncludeStack reports whether an include path is already active.
func (generator *DirectiveGenerator) pathInIncludeStack(path string) bool {
	for _, active := range generator.includeStack {
		if active == path {
			return true
		}
	}
	return false
}

// includeCycleError raises a diagnostic containing the active recursive include chain.
func (generator *DirectiveGenerator) includeCycleError(path string, location *SourceLocation) {
	start := indexOf(generator.includeStack, path)
	cycle := append(append([]string{}, generator.includeStack[start:]...), path)
	for index := range cycle {
		cycle[index] = filepath.Base(cycle[index])
	}
	generator.err("Circular include dependency: "+strings.Join(cycle, " -> "), location, "Remove the recursive #include; SOMETHING does not permit recursion")
}

// expandFor expands for into its structural AST form.
func (generator *DirectiveGenerator) expandFor(loop *ForDirective, private bool) []Statement {
	source := generator.expandExpression(loop.Source)
	value := generator.runtime.evaluateExpression(source, nil)
	result := []Statement{}
	switch collection := value.(type) {
	case []any:
		if loop.KeyName != "" {
			generator.err("#for over an array accepts only an element variable", loop.Location, "Use '#for element: array { ... }'")
		}
		for _, element := range collection {
			generator.pushConstants(map[string]any{loop.ElementName: element})
			result = append(result, generator.expandStatements(loop.Body, private)...)
			generator.popConstants()
		}
	case *runtimeMapping:
		if loop.KeyName == "" {
			generator.err("#for over a mapping requires key and value variables", loop.Location, "Use '#for key, value: mapping { ... }'")
		}
		entries := append([]runtimeMappingEntry{}, collection.entries...)
		sort.SliceStable(entries, func(left, right int) bool {
			return fmt.Sprintf("%v", materializeValue(entries[left].key, true)) < fmt.Sprintf("%v", materializeValue(entries[right].key, true))
		})
		for _, entry := range entries {
			generator.pushConstants(map[string]any{loop.KeyName: entry.key, loop.ElementName: entry.value})
			result = append(result, generator.expandStatements(loop.Body, private)...)
			generator.popConstants()
		}
	default:
		generator.err("#for source must be an array or mapping, got "+runtimeTypeName(value), loop.Source.expressionLocation(), "Use an array with one loop variable or a mapping with key and value variables")
	}
	return result
}

// expandInsert expands insert into its structural AST form.
func (generator *DirectiveGenerator) expandInsert(insert *InsertDirective, private bool) []Statement {
	result := []Statement{}
	for _, content := range insert.Contents {
		expanded := generator.expandExpression(content)
		value := generator.runtime.evaluateExpression(expanded, PrimString)
		text, ok := value.(string)
		if !ok {
			generator.err("#insert content must evaluate to a string", content.expressionLocation(), "Use string or #multiline values")
		}
		filepath := generator.filepath
		if insert.Location != nil && insert.Location.Filepath != "" {
			filepath = insert.Location.Filepath
		}
		program := NewParser(NewLexer(text, filepath).Tokenize(), filepath).ParseProgram()
		NewTypeChecker(program, filepath).detectDependencyCycles()
		result = append(result, generator.expandStatements(program.Statements, private)...)
	}
	return result
}

// registerMacro registers macro.
func (generator *DirectiveGenerator) registerMacro(macro *MacroDirective) {
	if _, exists := generator.macros.definitions[macro.Name]; exists {
		generator.err("Macro '"+macro.Name+"' is already declared in this scope", macro.Location, "Use a unique macro name")
	}
	parameters := make(map[string]bool)
	for _, parameter := range macro.Params {
		if parameters[parameter.Name] {
			generator.err("Macro '"+macro.Name+"' declares parameter '"+parameter.Name+"' more than once", parameter.Location, "Use a unique name for each macro parameter")
		}
		parameters[parameter.Name] = true
	}
	NewTypeChecker(&Program{Statements: macro.Body, Filepath: macro.Location.Filepath}, macro.Location.Filepath).detectDependencyCycles()
	generator.macros.definitions[macro.Name] = macro
}

// expandMacroCall expands macro call into its structural AST form.
func (generator *DirectiveGenerator) expandMacroCall(call *MacroCallExpression) Expression {
	macro, ok := generator.macros.lookup(call.Name)
	if !ok {
		generator.err("Undefined macro '"+call.Name+"'", call.Location, "Macros must be declared before their call site")
	}
	if start := indexOf(generator.macroStack, call.Name); start >= 0 {
		cycle := append(append([]string{}, generator.macroStack[start:]...), call.Name)
		generator.err("Recursive macro expansion: "+strings.Join(cycle, " -> "), call.Location, "Remove the recursive macro call; SOMETHING does not permit recursion")
	}
	if len(call.Arguments) != len(macro.Params) {
		generator.err(fmt.Sprintf("Macro '%s' expects %d arguments, got %d", call.Name, len(macro.Params), len(call.Arguments)), call.Location, "Pass one argument for each declared parameter")
	}

	arguments := make([]any, len(call.Arguments))
	for index, argument := range call.Arguments {
		expanded := generator.expandExpression(argument)
		paramType := generator.runtime.resolveType(macro.Params[index].Type, macro.Params[index].Location)
		arguments[index] = generator.runtime.evaluateExpression(expanded, paramType)
		if !generator.runtimeValueAssignable(paramType, arguments[index]) {
			generator.err(fmt.Sprintf("Macro '%s' argument %d (%s) expected %s, got %s", call.Name, index, macro.Params[index].Name, typeRefString(paramType), runtimeTypeName(arguments[index])), argument.expressionLocation(), "Pass a value matching the macro parameter type")
		}
	}

	previousRuntime := generator.runtime.current
	previousMacros := generator.macros
	localRuntime := newRuntimeEnvironment(previousRuntime)
	localMacros := newMacroEnvironment(previousMacros)
	for index, parameter := range macro.Params {
		localRuntime.bindings[parameter.Name] = &runtimeBinding{
			value:   arguments[index],
			typeRef: generator.runtime.resolveType(parameter.Type, parameter.Location),
			private: true,
		}
	}
	generator.runtime.current = localRuntime
	generator.macros = localMacros
	generator.macroStack = append(generator.macroStack, call.Name)
	body := generator.expandStatements(macro.Body, true)
	returnExpression := generator.expandExpression(macro.Return)
	returnType := generator.runtime.resolveType(macro.ReturnType, macro.Location)
	generator.checkExpandedMacro(macro, arguments, body, returnExpression, returnType, previousRuntime)
	result := generator.runtime.evaluateExpression(returnExpression, returnType)
	if !generator.runtimeValueAssignable(returnType, result) {
		generator.err("Macro '"+call.Name+"' return expected "+typeRefString(returnType)+", got "+runtimeTypeName(result), macro.Return.expressionLocation(), "Make #set return the declared macro type")
	}
	generator.macroStack = generator.macroStack[:len(generator.macroStack)-1]
	generator.macros = previousMacros
	generator.runtime.current = previousRuntime
	return &TypedExpression{Value: generator.expressionFromRuntime(result, call.Location), Type: returnType, Location: call.Location}
}

// checkExpandedMacro checks expanded macro against the current invariants.
func (generator *DirectiveGenerator) checkExpandedMacro(macro *MacroDirective, arguments []any, body []Statement, returnExpression Expression, returnType TypeRef, outerRuntime *runtimeEnvironment) {
	statements := make([]Statement, 0, len(macro.Params)+len(body))
	for index, parameter := range macro.Params {
		parameterType := generator.runtime.resolveType(parameter.Type, parameter.Location)
		if parameterType == PrimScope || parameterType == PrimNamespace {
			parameterType = generator.runtime.runtimeType(arguments[index])
		}
		statements = append(statements, &Assignment{
			StatementBase: StatementBase{Private: true, Location: parameter.Location},
			Target:        &IdentifierLValue{Name: parameter.Name},
			Mode:          AssignExplicit,
			DeclaredType:  parameterType,
			Value:         generator.expressionFromRuntime(arguments[index], parameter.Location),
		})
	}
	statements = append(statements, body...)

	checker := NewTypeChecker(&Program{Statements: statements, Filepath: macro.Location.Filepath}, macro.Location.Filepath)
	outer := newStaticEnvironment(nil)
	for environment := outerRuntime; environment != nil; environment = environment.parent {
		for name, binding := range environment.bindings {
			if _, exists := outer.bindings[name]; !exists {
				outer.bindings[name] = &staticBinding{typeRef: binding.typeRef, private: binding.private}
			}
		}
		for name, typeRef := range environment.types {
			if _, exists := outer.types[name]; !exists {
				outer.types[name] = typeRef
			}
		}
	}
	checker.current = newStaticEnvironment(outer)
	checker.detectDependencyCycles()
	checker.checkStatements(statements)
	actualReturnType := checker.expressionType(returnExpression, returnType)
	checker.requireAssignable(returnType, actualReturnType, returnExpression.expressionLocation(), "macro '"+macro.Name+"' return")
}

// expressionFromRuntime converts a compile-time runtime value back into a syntax expression.
func (generator *DirectiveGenerator) expressionFromRuntime(value any, location *SourceLocation) Expression {
	switch value := value.(type) {
	case string:
		return stringExpression(value, location)
	case int:
		return &IntegerExpression{Value: value, Location: location}
	case float64:
		return &FloatExpression{Value: value, Location: location}
	case bool:
		return &BooleanExpression{Value: value, Location: location}
	case *EnumValue:
		typeRef, ok := generator.runtime.current.lookupType(value.EnumName)
		if !ok {
			generator.err("Macro produced an unknown enum type '"+value.EnumName+"'", location, "Declare the enum before the macro call")
		}
		enumType := typeRef.(*EnumType)
		return &ReferenceExpression{Root: value.EnumName, Accesses: []Access{&FieldAccess{Name: enumType.MemberList[value.Ordinal], Location: location}}, Location: location}
	case []any:
		elements := make([]Expression, len(value))
		for index, element := range value {
			elements[index] = generator.expressionFromRuntime(element, location)
		}
		return &ArrayExpression{Elements: elements, Location: location}
	case *runtimeMapping:
		entries := make([]*MappingEntry, len(value.entries))
		for index, entry := range value.entries {
			entries[index] = &MappingEntry{
				Keys:     []Expression{generator.expressionFromRuntime(entry.key, location)},
				Value:    generator.expressionFromRuntime(entry.value, location),
				Location: location,
			}
		}
		return &MappingExpression{Entries: entries, Location: location}
	case *runtimeObject:
		fields := []*FieldAssignment{}
		for _, name := range sortedRuntimeBindingKeys(value.environment.bindings) {
			binding := value.environment.bindings[name]
			fields = append(fields, &FieldAssignment{Name: name, Value: generator.expressionFromRuntime(binding.value, location), Location: location})
		}
		return &StructExpression{TypeName: value.typeName, Fields: fields, Location: location}
	default:
		generator.err(fmt.Sprintf("Cannot convert compile-time value %T into an expression", value), location, "Report this as an implementation defect")
	}
	return nil
}

// stringExpression wraps plain text in a string-literal expression at a source location.
func stringExpression(value string, location *SourceLocation) *StringExpression {
	parts := []StringPart{}
	if value != "" {
		parts = append(parts, StringText(value))
	}
	return &StringExpression{Literal: &StringLiteral{Parts: parts}, Location: location}
}

// pushConstants adds one lexical constant scope for directive expansion.
func (generator *DirectiveGenerator) pushConstants(values map[string]any) {
	generator.constantValues = append(generator.constantValues, values)
}

// popConstants removes the innermost directive constant scope.
func (generator *DirectiveGenerator) popConstants() {
	generator.constantValues = generator.constantValues[:len(generator.constantValues)-1]
}

// lookupConstant searches the innermost-to-outermost directive constant scopes.
func (generator *DirectiveGenerator) lookupConstant(name string) (any, bool) {
	for index := len(generator.constantValues) - 1; index >= 0; index-- {
		if value, ok := generator.constantValues[index][name]; ok {
			return value, true
		}
	}
	return nil, false
}
