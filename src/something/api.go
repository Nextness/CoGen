// api.go provides the public entry points for loading, parsing, and
// evaluating .something configuration files. It exposes LoadSomethingFile
// and LoadSomethingBytes as the primary API, and Pprint for formatting
// resolved config values.
package something

import (
	"fmt"
	"os"
	"strings"
)

// LoadSomethingFile loads, parses, and evaluates a .something file.
// Returns the evaluated config map. Only variables not marked with `#priv`
// are included in the result.
func LoadSomethingFile(filepath string) (result map[string]any, err error) {
	data, readErr := os.ReadFile(filepath)
	if readErr != nil {
		return nil, &SomethingError{Message: "Could not read file: " + filepath, Filepath: filepath}
	}
	return LoadSomethingBytes(data, filepath)
}

// LoadSomethingBytes compiles and evaluates one already-read SOMETHING file.
// It allows callers that must retain an immutable source snapshot to evaluate
// exactly those bytes rather than reading a mutable path a second time.
func LoadSomethingBytes(data []byte, filepath string) (result map[string]any, err error) {
	text := string(data)

	// Use defer/recover to catch SomethingError panics from the pipeline
	defer func() {
		if r := recover(); r != nil {
			if se, ok := r.(*SomethingError); ok {
				err = se
			} else {
				panic(r)
			}
		}
	}()

	tokens := NewLexer(text, filepath).Tokenize()
	syntax := NewParser(tokens, filepath).ParseProgram()
	expanded := NewDirectiveGenerator(filepath).Expand(syntax)
	checked := NewTypeChecker(expanded, filepath).Check()
	ev := NewEvaluator(checked, filepath)
	result = ev.evaluate()
	return result, nil
}

// Pprint pretty-prints a resolved SOMETHING value.
func Pprint(v any, indent int) string {
	sp := "  "
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case string:
		return `"` + val + `"`
	case map[string]any:
		if len(val) == 0 {
			return "{}"
		}
		items := []string{}
		keys := sortedMapKeys(val)
		for _, k := range keys {
			items = append(items, strings.Repeat(sp, indent+1)+`"`+k+`": `+Pprint(val[k], indent+1))
		}
		return "{\n" + strings.Join(items, ",\n") + "\n" + strings.Repeat(sp, indent) + "}"
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		items := make([]string, len(val))
		for i, elem := range val {
			items[i] = strings.Repeat(sp, indent+1) + Pprint(elem, indent+1)
		}
		return "[\n" + strings.Join(items, ",\n") + "\n" + strings.Repeat(sp, indent) + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}
