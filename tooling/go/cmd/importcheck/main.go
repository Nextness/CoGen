// Command importcheck invokes the maintained importcheck implementation.
package main

import (
	"analysis/tooling/go/importcheck"
	"os"
)

// main runs the command and preserves its exit status.
func main() { os.Exit(importcheck.Run(os.Args[1:], os.Stdout, os.Stderr)) }
