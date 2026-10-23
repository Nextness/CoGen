// Command prepare-osf invokes the maintained prepare-osf implementation.
package main

import (
	prepareosf "analysis/tooling/go/prepare-osf"
	"os"
)

// main preserves the command exit status.
func main() { os.Exit(prepareosf.Run(os.Args[1:], os.Stdout, os.Stderr)) }
