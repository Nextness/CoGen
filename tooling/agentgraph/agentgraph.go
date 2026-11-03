// Package agentgraph exposes optional code-graph helpers.
package agentgraph

import (
	"fmt"
	"io"
)

// Version identifies the maintained optional module.
func Version() string { return "dev" }

// Run dispatches the thin optional code-graph command without native setup.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (len(args) == 1 && args[0] == "version") {
		fmt.Fprintln(stdout, Version())
		return 0
	}
	fmt.Fprintln(stderr, "usage: code-graph [version]")
	return 2
}
