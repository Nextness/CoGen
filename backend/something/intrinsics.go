// intrinsics.go defines the built-in intrinsic functions callable with the
// '@' prefix, such as @split_by. Intrinsics are pure, compile-time-evaluated
// functions with a fixed parameter list and return type. They are usable in
// any expression position and are checked and evaluated like the call-like
// directives #match and #len.
package something

import (
	"fmt"
	"sort"
	"strings"
)

// intrinsicParam is one declared parameter of an intrinsic function.
type intrinsicParam struct {
	name    string
	typeRef TypeRef
}

// intrinsicDef describes one intrinsic function: its parameter types, return
// type, and runtime evaluation behavior.
type intrinsicDef struct {
	name       string
	params     []intrinsicParam
	returnType TypeRef
	evaluate   func(state *runtimeState, arguments []any, location *SourceLocation) any
}

// intrinsics is the registry of supported intrinsic functions keyed by name.
var intrinsics = map[string]*intrinsicDef{
	"split_by": {
		name: "split_by",
		params: []intrinsicParam{
			{name: "input", typeRef: PrimString},
			{name: "delimiter", typeRef: PrimString},
		},
		returnType: &ArrayType{ElementType: PrimString},
		evaluate: func(state *runtimeState, arguments []any, location *SourceLocation) any {
			input, _ := arguments[0].(string)
			delimiter, _ := arguments[1].(string)
			parts := strings.Split(input, delimiter)
			result := make([]any, len(parts))
			for index, part := range parts {
				result[index] = part
			}
			return result
		},
	},
	"concat": {
		name: "concat",
		params: []intrinsicParam{
			{name: "list", typeRef: &ArrayType{ElementType: PrimString}},
			{name: "delimiter", typeRef: PrimString},
		},
		returnType: PrimString,
		evaluate: func(state *runtimeState, arguments []any, location *SourceLocation) any {
			list, _ := arguments[0].([]any)
			delimiter, _ := arguments[1].(string)
			parts := make([]string, len(list))
			for index, element := range list {
				parts[index], _ = element.(string)
			}
			return strings.Join(parts, delimiter)
		},
	},
	"append_prefix_for_each": {
		name: "append_prefix_for_each",
		params: []intrinsicParam{
			{name: "list", typeRef: &ArrayType{ElementType: PrimString}},
			{name: "prefix", typeRef: PrimString},
		},
		returnType: &ArrayType{ElementType: PrimString},
		evaluate: func(state *runtimeState, arguments []any, location *SourceLocation) any {
			list, _ := arguments[0].([]any)
			prefix, _ := arguments[1].(string)
			result := make([]any, len(list))
			for index, element := range list {
				text, _ := element.(string)
				result[index] = prefix + text
			}
			return result
		},
	},
}

// lookupIntrinsic returns the definition for a named intrinsic, if any.
func lookupIntrinsic(name string) (*intrinsicDef, bool) {
	def, ok := intrinsics[name]
	return def, ok
}

// sortedIntrinsicNames returns the supported intrinsic names in deterministic order.
func sortedIntrinsicNames() []string {
	names := make([]string, 0, len(intrinsics))
	for name := range intrinsics {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// unknownIntrinsicMessage builds the diagnostic for an unrecognized intrinsic name.
func unknownIntrinsicMessage(name string) string {
	return fmt.Sprintf("Unknown intrinsic '@%s'", name)
}
