// Command analysis runs the corpus pipeline and local viewer.
package main

import (
	"analysis/backend/cli"
	"os"
)

// main preserves command dispatch exit status.
func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }
