package unit

import (
	"testing"

	"analysis/tooling/agentgraph"
)

// TestVersion verifies the optional module resolves through its local replacement.
func TestVersion(t *testing.T) {
	if agentgraph.Version() != "dev" {
		t.Fatalf("unexpected version %q", agentgraph.Version())
	}
}
