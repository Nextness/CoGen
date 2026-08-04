// workspace_config_integration_test.go contains integration tests for parsing and
// evaluating workspace configuration declarations in the SOMETHING
// config language.
//go:build integration

package something

import (
	"fmt"
	"testing"
)

// TestWorkspaceConfigValidDeclarations verifies workspace config valid declarations.
func TestWorkspaceConfigValidDeclarations(t *testing.T) {
	// A complete workspace config with all declared types should parse and
	// evaluate to the expected values.
	text := `
cache_policy_config: setup = {
    reads: []string;
    writes: []string;
    negative_ttl_days: integer;
}
source_declaration: setup = {
    name: string;
    expected_file: string;
    query: string;
    requested_fields: []string;
}
reuse_policy_config: setup = {
    policy: string;
}
workspace_config: setup = {
    format_version: integer;
    search_id: string;
    search_revision: string;
    enrichment_enabled: boolean;
    reuse_policy: reuse_policy_config;
    cache_policy: cache_policy_config;
    sources: []source_declaration;
}
workspace: workspace_config = {
    format_version = 2,
    search_id = "bpmn-optimisation",
    search_revision = "2026-07-query-expansion",
    enrichment_enabled = true,
    reuse_policy = reuse_policy_config { policy = "reuse_completed" },
    cache_policy = cache_policy_config {
        reads = []string { "global", "network" },
        writes = []string { "active_run", "global" },
        negative_ttl_days = 14,
    },
    sources = [
        {
            name = "scopus",
            expected_file = "corpus/scopus.csv",
            query = "TITLE-ABS-KEY(BPMN)",
            requested_fields = []string { "title", "doi" },
        },
    ],
};
`
	result := evalText(t, text)
	ws, ok := result["workspace"]
	if !ok {
		t.Fatal("workspace key missing from result")
	}
	m, ok := ws.(map[string]any)
	if !ok {
		t.Fatalf("workspace is %T, want map[string]any", ws)
	}
	tests := []struct {
		key      string
		wantType string
		wantVal  any
	}{
		{"format_version", "int", 2},
		{"search_id", "string", "bpmn-optimisation"},
		{"search_revision", "string", "2026-07-query-expansion"},
		{"enrichment_enabled", "bool", true},
	}
	for _, tt := range tests {
		v, ok := m[tt.key]
		if !ok {
			t.Errorf("workspace.%s missing", tt.key)
			continue
		}
		if fmt.Sprintf("%T", v) != tt.wantType {
			t.Errorf("workspace.%s type = %T, want %s", tt.key, v, tt.wantType)
		}
		if v != tt.wantVal {
			t.Errorf("workspace.%s = %v, want %v", tt.key, v, tt.wantVal)
		}
	}
	// Verify nested structs
	rp, ok := m["reuse_policy"].(map[string]any)
	if !ok {
		t.Fatal("reuse_policy is not a map")
	}
	if rp["policy"] != "reuse_completed" {
		t.Errorf("reuse_policy.policy = %v, want reuse_completed", rp["policy"])
	}
	cp, ok := m["cache_policy"].(map[string]any)
	if !ok {
		t.Fatal("cache_policy is not a map")
	}
	if cp["negative_ttl_days"] != 14 {
		t.Errorf("cache_policy.negative_ttl_days = %v, want 14", cp["negative_ttl_days"])
	}
	// Verify source array
	ss, ok := m["sources"].([]any)
	if !ok {
		t.Fatal("sources is not an array")
	}
	if len(ss) != 1 {
		t.Fatalf("sources length = %d, want 1", len(ss))
	}
	s0, ok := ss[0].(map[string]any)
	if !ok {
		t.Fatal("sources[0] is not a map")
	}
	if s0["name"] != "scopus" {
		t.Errorf("sources[0].name = %v, want scopus", s0["name"])
	}
}

// TestWorkspaceConfigMissingFields verifies workspace config missing fields.
func TestWorkspaceConfigMissingFields(t *testing.T) {
	// A workspace config missing required fields should be rejected by the
	// evaluator. enrichment_enabled is required (no default).
	text := `
workspace_config: setup = {
    format_version: integer;
    search_id: string;
    enrichment_enabled: boolean;
}
workspace: workspace_config = {
    format_version = 2,
    search_id = "test",
};
`
	assertPanic(t, func() { evalText(t, text) }, "missing required field 'enrichment_enabled'")
}

// TestWorkspaceConfigFormatVersionOne verifies workspace config format version one.
func TestWorkspaceConfigFormatVersionOne(t *testing.T) {
	// format_version = 1 is valid SOMETHING syntax; Go-level validation
	// would reject it. The parser/evaluator must accept it.
	text := `
workspace_config: setup = {
    format_version: integer;
}
workspace: workspace_config = {
    format_version = 1,
};
`
	result := evalText(t, text)
	ws, ok := result["workspace"].(map[string]any)
	if !ok {
		t.Fatal("workspace key missing")
	}
	fv, ok := ws["format_version"]
	if !ok {
		t.Fatal("format_version missing")
	}
	if fv != 1 {
		t.Errorf("format_version = %v, want 1", fv)
	}
}

// TestWorkspaceConfigEmptyCachePolicy verifies workspace config empty cache policy.
func TestWorkspaceConfigEmptyCachePolicy(t *testing.T) {
	// Empty cache policy reads/writes are valid SOMETHING; Go-level
	// validation would reject invalid combinations.
	text := `
cache_policy_config: setup = {
    reads: []string;
    writes: []string;
    negative_ttl_days: integer;
}
workspace_config: setup = {
    cache_policy: cache_policy_config;
}
workspace: workspace_config = {
    cache_policy = cache_policy_config {
        reads = []string {},
        writes = []string {},
        negative_ttl_days = 0,
    },
};
`
	result := evalText(t, text)
	ws, ok := result["workspace"].(map[string]any)
	if !ok {
		t.Fatal("workspace key missing")
	}
	cp, ok := ws["cache_policy"].(map[string]any)
	if !ok {
		t.Fatal("cache_policy missing")
	}
	reads, ok := cp["reads"].([]any)
	if !ok {
		t.Fatal("cache_policy.reads missing or not an array")
	}
	if len(reads) != 0 {
		t.Errorf("cache_policy.reads = %v, want empty", reads)
	}
}
