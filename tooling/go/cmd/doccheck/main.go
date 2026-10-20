// Command doccheck invokes the maintained doccheck implementation.
package main

import (
	"analysis/tooling/go/doccheck"
	"os"
)

// main runs the command and preserves its exit status.
func main() { os.Exit(doccheck.Run(os.Args[1:], os.Stdout, os.Stderr)) }
