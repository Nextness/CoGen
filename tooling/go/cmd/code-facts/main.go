// Command code-facts exports deterministic portable repository facts as JSON.
package main

import (
	"os"

	"analysis/tooling/go/codefacts"
)

// main runs the command and preserves its exit status.
func main() { os.Exit(codefacts.Run(os.Args[1:], os.Stdout, os.Stderr)) }
