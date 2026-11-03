package unit

import (
	"bytes"
	"strings"
	"testing"

	"analysis/tooling/agentgraph"
)

// TestRunPrintsVersion verifies the thin command reports the optional module version.
func TestRunPrintsVersion(t *testing.T) {
	for _, args := range [][]string{nil, {"version"}} {
		var stdout, stderr bytes.Buffer
		code := agentgraph.Run(args, &stdout, &stderr)
		if code != 0 || strings.TrimSpace(stdout.String()) != "dev" || stderr.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

// TestRunRejectsUnknownCommands verifies usage mistakes cannot report success.
func TestRunRejectsUnknownCommands(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"version", "extra"}} {
		var stdout, stderr bytes.Buffer
		code := agentgraph.Run(args, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "usage: code-graph [version]") || stdout.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}
