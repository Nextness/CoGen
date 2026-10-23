// Command coveragecheck invokes the maintained coveragecheck implementation.
package main

import (
	"analysis/tooling/go/coveragecheck"
	"os"
)

// main preserves the command exit status.
func main() { os.Exit(coveragecheck.Run(os.Args[1:], os.Stdout, os.Stderr)) }
