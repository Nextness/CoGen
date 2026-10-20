// Command something-printer invokes the maintained something-printer implementation.
package main

import (
	somethingprinter "analysis/tooling/go/something-printer"
	"os"
)

// main runs the command and preserves its exit status.
func main() { os.Exit(somethingprinter.Run(os.Args[0], os.Args[1:], os.Stdout, os.Stderr)) }
