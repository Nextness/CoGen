// Command viewer-fixture builds the disposable metadata/PDF pair used by viewer development and browser tests.
package main

import (
	"flag"
	"fmt"
	"os"

	"analysis/tests/support/go/viewerfixture"
)

// main parses explicit fixture inputs and reports generation failure to the caller.
func main() {
	output := flag.String("out", "build/fixtures/viewer/workspace.fixture.db", "generated viewer metadata path")
	registry := flag.String("registry", "config/database.something", "production database registry")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "viewer-fixture accepts only --out and --registry")
		os.Exit(2)
	}
	if err := viewerfixture.Generate(*output, *registry); err != nil {
		fmt.Fprintln(os.Stderr, "viewer fixture:", err)
		os.Exit(1)
	}
	fmt.Println("Generated viewer fixture:", *output)
}
