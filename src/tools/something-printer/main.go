// Command something-printer reads a .something file, evaluates it, and prints
// the resulting public values in one of several text formats. The default and
// explicit --something format prints the evaluated result using SOMETHING
// setup syntax; --json and --yaml select the other supported serializations.
// Useful for inspecting config changes before running the full pipeline.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"analysis/something"
)

// printFormat identifies a serialization selected by a format flag.
type printFormat string

// The supported output formats.
const (
	formatJSON      printFormat = "json"
	formatYAML      printFormat = "yaml"
	formatSomething printFormat = "something"
)

// somethingIndent is the indentation unit used by the SOMETHING printer.
const somethingIndent = "    "

// main dispatches the tool over process arguments and exits on failure.
func main() {
	os.Exit(run(os.Args[0], os.Args[1:], os.Stdout, os.Stderr))
}

// run parses format flags, evaluates the configured .something file, writes
// the selected rendition to stdout, and returns a process exit code. It uses
// flag.ContinueOnError so tests can capture usage failures through stderr.
func run(prog string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "print the evaluated result as JSON")
	yamlFlag := fs.Bool("yaml", false, "print the evaluated result as YAML")
	somethingFlag := fs.Bool("something", false, "print the evaluated result in SOMETHING syntax (default)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: %s [--json|--yaml|--something] <file.something>\n", prog)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	selected := 0
	for _, set := range []bool{*jsonFlag, *yamlFlag, *somethingFlag} {
		if set {
			selected++
		}
	}
	if selected > 1 {
		fmt.Fprintln(stderr, "error: --json, --yaml, and --something are mutually exclusive")
		return 2
	}

	format := formatSomething
	switch {
	case *jsonFlag:
		format = formatJSON
	case *yamlFlag:
		format = formatYAML
	}

	result, err := something.LoadSomethingFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	out, err := render(result, format)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, out)
	return 0
}

// render serializes an evaluated config map in the requested format.
func render(result map[string]any, format printFormat) (string, error) {
	switch format {
	case formatJSON:
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshaling to JSON: %w", err)
		}
		return string(out), nil
	case formatYAML:
		out, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("marshaling to YAML: %w", err)
		}
		return string(out), nil
	default:
		return printSomething(result), nil
	}
}

// printSomething renders an evaluated config map as one public assignment per
// line in SOMETHING setup syntax, with braces for nested objects and brackets
// for lists, using the printer's indentation unit.
func printSomething(result map[string]any) string {
	var b strings.Builder
	for _, key := range sortedStringKeys(result) {
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(somethingValue(result[key], 0))
		b.WriteString(",\n")
	}
	return b.String()
}

// somethingValue renders one evaluated value in SOMETHING-like syntax at the
// given nesting depth. Strings are quoted, and braces, backslashes, newlines,
// tabs, and carriage returns are escaped only when present so the emitted
// literal re-lexes to the original value. Unknown Go types fall back to their
// plain string form.
func somethingValue(v any, depth int) string {
	pad := strings.Repeat(somethingIndent, depth)
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(val)
	case float64:
		return fmt.Sprintf("%v", val)
	case string:
		return quoteString(val)
	case map[string]any:
		return somethingObject(val, depth, pad)
	case map[int]any:
		return somethingIntegerObject(val, depth, pad)
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		var b strings.Builder
		b.WriteString("[\n")
		for _, elem := range val {
			b.WriteString(pad + somethingIndent)
			b.WriteString(somethingValue(elem, depth+1))
			b.WriteString(",\n")
		}
		b.WriteString(pad + "]")
		return b.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// somethingObject renders a string-keyed mapping as a setup-style literal.
func somethingObject(value map[string]any, depth int, pad string) string {
	if len(value) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{\n")
	for _, key := range sortedStringKeys(value) {
		b.WriteString(pad + somethingIndent + key)
		b.WriteString(" = ")
		b.WriteString(somethingValue(value[key], depth+1))
		b.WriteString(",\n")
	}
	b.WriteString(pad + "}")
	return b.String()
}

// somethingIntegerObject renders an integer-keyed mapping with integer entry
// keys in a setup-style block. Such mappings arise from integer- or
// enum-indexed mappings, whose original types are not preserved by evaluation.
func somethingIntegerObject(value map[int]any, depth int, pad string) string {
	if len(value) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{\n")
	for _, key := range sortedIntegerKeys(value) {
		b.WriteString(pad + somethingIndent + strconv.Itoa(key))
		b.WriteString(" = ")
		b.WriteString(somethingValue(value[key], depth+1))
		b.WriteString(",\n")
	}
	b.WriteString(pad + "}")
	return b.String()
}

// quoteString wraps a string in the quote character that avoids escaping, or
// escapes the quote when both quote kinds appear. Backslashes, braces,
// newlines, tabs, and carriage returns are escaped so the emitted literal
// re-lexes to the original value.
func quoteString(s string) string {
	quote := '"'
	if strings.Contains(s, `"`) && !strings.Contains(s, "'") {
		quote = '\''
	}
	var b strings.Builder
	b.WriteRune(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '{':
			b.WriteString(`{{`)
		case '}':
			b.WriteString(`}}`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '"', '\'':
			if r == quote {
				if quote == '"' {
					b.WriteString(`\"`)
				} else {
					b.WriteString(`\'`)
				}
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteRune(quote)
	return b.String()
}

// sortedStringKeys returns the keys of a string-keyed map in sorted order.
func sortedStringKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedIntegerKeys returns the keys of an integer-keyed map in sorted order.
func sortedIntegerKeys(value map[int]any) []int {
	keys := make([]int, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}
