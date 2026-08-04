// Command something-json reads a .something file, parses and evaluates it,
// and prints the result as indented JSON. Useful for inspecting config
// changes before running the full pipeline.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"analysis/something"
)

// main dispatches the analysis command selected by process arguments and exits on command failure.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <file.something>\n", os.Args[0])
		os.Exit(1)
	}

	path := os.Args[1]
	result, err := something.LoadSomethingFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling to JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}
