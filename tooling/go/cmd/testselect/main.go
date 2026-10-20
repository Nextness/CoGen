// Command testselect runs domain-owned suites through the standard test runners.
package main

import (
	"analysis/tooling/go/testselect"
	"os"
)

// main preserves the selector's exit status.
func main() { os.Exit(testselect.Run(os.Args[1:], os.Stdout, os.Stderr)) }
