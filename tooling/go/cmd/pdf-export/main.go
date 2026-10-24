// Command pdf-export delegates to the maintained read-only PDF export implementation.
package main

import (
	"os"

	pdfexport "analysis/tooling/go/pdf-export"
)

// main preserves the command exit status.
func main() { os.Exit(pdfexport.Run(os.Args[1:], os.Stdout, os.Stderr)) }
