// Command rawsqlcheck invokes the maintained rawsqlcheck implementation.
package main

import (
	"analysis/tooling/go/rawsqlcheck"
	"os"
)

// main runs the command and preserves its exit status.
func main() { os.Exit(rawsqlcheck.Run(os.Args[1:], os.Stdout, os.Stderr)) }
