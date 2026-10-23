// Command pdf-store invokes the maintained pdf-store implementation.
package main

import (
	pdfstorecmd "analysis/tooling/go/pdf-store"
	"os"
)

// main preserves the command exit status.
func main() { os.Exit(pdfstorecmd.Run(os.Args[1:], os.Stdout, os.Stderr)) }
