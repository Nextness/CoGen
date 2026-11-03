// Command code-graph runs the optional code-graph command.
package main

import (
	"os"

	"analysis/tooling/agentgraph"
)

// main runs the command and preserves its exit status.
func main() { os.Exit(agentgraph.Run(os.Args[1:], os.Stdout, os.Stderr)) }
