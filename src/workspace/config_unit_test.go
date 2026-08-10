// config_unit_test.go tests the workspace configuration loader, verifying
// that workspace iterations are correctly resolved from SOMETHING
// configuration files into typed ResolvedManifest values.
//go:build unit

package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadBuildsResolvedWorkspaceRuns verifies load builds resolved workspace runs.
func TestLoadBuildsResolvedWorkspaceRuns(t *testing.T) {
	configPath := writeConfig(t, testConfig())
	config, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(config.Runs))
	}
	first := config.Runs[0]
	if first.Manifest.SearchID != "search-one" || first.Manifest.SearchRevision != "r1" {
		t.Fatalf("unexpected first selector %q", Selector(first.Manifest.SearchID, first.Manifest.SearchRevision))
	}
	if !filepath.IsAbs(first.Manifest.Sources[0].ExpectedFile) {
		t.Fatalf("source path was not resolved: %q", first.Manifest.Sources[0].ExpectedFile)
	}
	if first.Manifest.Sources[0].ExpectedResultCount != 4 || len(first.Manifest.Sources[0].Filters) != 2 {
		t.Fatalf("source result provenance missing: %+v", first.Manifest.Sources[0])
	}
	if first.Manifest.Sources[0].Filters[0].Count != 10 || first.Manifest.Sources[0].Filters[1].Count != 4 {
		t.Fatalf("source filter counts wrong: %+v", first.Manifest.Sources[0].Filters)
	}
	if first.Reviewer != (Reviewer{}) {
		t.Fatalf("default reviewer = %+v, want empty", first.Reviewer)
	}
	if !first.Enrichment.Sources["crossref"].FillMissingOnly {
		t.Fatal("fill_missing_only was not retained in the runtime source configuration")
	}
}

// TestLoadNormalizesReviewerWithoutChangingManifestIdentity verifies optional attribution is trimmed and excluded from plan fingerprints.
func TestLoadNormalizesReviewerWithoutChangingManifestIdentity(t *testing.T) {
	path := writeConfig(t, testConfig())
	withoutReviewer, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	withReviewerText := strings.Replace(testConfig(), "enrichment_providers = [enrichment_provider_config {", `reviewer = reviewer_config { username = "  Alice  ", email = "  alice@example.test  ", },
        enrichment_providers = [enrichment_provider_config {`, 1)
	if err := os.WriteFile(path, []byte(withReviewerText), 0644); err != nil {
		t.Fatal(err)
	}
	withReviewer, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := withReviewer.Runs[0].Reviewer; got.Username != "Alice" || got.Email != "alice@example.test" {
		t.Fatalf("reviewer = %+v", got)
	}
	baseHash, err := withoutReviewer.Runs[0].Manifest.Hash()
	if err != nil {
		t.Fatal(err)
	}
	reviewerHash, err := withReviewer.Runs[0].Manifest.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if baseHash != reviewerHash {
		t.Fatalf("reviewer changed resolved manifest hash: %s != %s", baseHash, reviewerHash)
	}
}

// TestProductionWorkspaceConfigLoads verifies production workspace config loads.
func TestProductionWorkspaceConfigLoads(t *testing.T) {
	config, err := Load(filepath.Join("..", "..", "config", "workspace.something"))
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Runs) == 0 {
		t.Fatal("production workspace config has no runs")
	}
	run := config.Runs[0]
	if !run.Manifest.EnrichmentEnabled {
		t.Fatal("production workspace enrichment is disabled")
	}
	for _, provider := range []string{"crossref", "openalex", "orcid"} {
		if _, ok := run.Enrichment.Sources[provider]; !ok {
			t.Fatalf("production workspace is missing %s enrichment", provider)
		}
	}
	if run.Enrichment.Sources["openalex"].FillMissingOnly {
		t.Fatal("production workspace must allow OpenAlex to refresh configured metadata")
	}
	sources := make(map[string]string, len(run.Manifest.Sources))
	for _, source := range run.Manifest.Sources {
		sources[source.Name] = source.Query
	}
	for _, tc := range []struct {
		name      string
		fragments []string
	}{
		{"scopus", []string{"TITLE-ABS-KEY((", "\"BPMN 2.0\"", "\"workflow scheduling\"", "\"genetic algorithm\"", "\"business process optimization\"", "\"formal semantics\""}},
		{"wos", []string{"TS=(\"BPMN\"", " AND TS=(", "\"BPMN 2.0\"", "\"workflow scheduling\"", "\"genetic algorithm\"", "\"business process optimization\"", "\"formal semantics\""}},
		{"ieeexplore", []string{"(\"Document Title\": (", ") OR (\"documentAbstract\": (", ") OR (\"authorTerms\": (", "\"BPMN 2.0\"", "\"workflow scheduling\"", "\"genetic algorithm\"", "\"business process optimization\"", "\"formal semantics\""}},
	} {
		query, ok := sources[tc.name]
		if !ok {
			t.Fatalf("production workspace is missing %s source", tc.name)
		}
		for _, fragment := range tc.fragments {
			if !strings.Contains(query, fragment) {
				t.Fatalf("%s query is missing %q: %s", tc.name, fragment, query)
			}
		}
	}
}

// TestSelectUsesDeclaredOrderAndRejectsUnknown verifies select uses declared order and rejects unknown.
func TestSelectUsesDeclaredOrderAndRejectsUnknown(t *testing.T) {
	config, err := Load(writeConfig(t, testConfig()))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := config.Select([]string{"search-two@r2", "search-one@r1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].Manifest.SearchID != "search-one" || selected[1].Manifest.SearchID != "search-two" {
		t.Fatalf("selection did not preserve declaration order: %+v", selected)
	}
	if _, err := config.Select([]string{"missing@r1"}); err == nil {
		t.Fatal("expected unknown selector error")
	}
}

// TestLoadRetainsEvaluatedBytesAfterConfigChanges verifies load retains evaluated bytes after config changes.
func TestLoadRetainsEvaluatedBytesAfterConfigChanges(t *testing.T) {
	path := writeConfig(t, testConfig())
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(testConfig(), "TITLE(test)", "TITLE(changed)", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if config.Runs[0].Manifest.Sources[0].Query != "TITLE(test)" {
		t.Fatalf("in-memory workspace changed after file edit: %q", config.Runs[0].Manifest.Sources[0].Query)
	}
	if !strings.Contains(string(config.OriginalBytes), "TITLE(test)") {
		t.Fatal("original config bytes were not retained")
	}
}

// TestLoadRejectsUnsupportedWorkspacePolicies verifies load rejects unsupported workspace policies.
func TestLoadRejectsUnsupportedWorkspacePolicies(t *testing.T) {
	cases := []struct {
		name        string
		replace     string
		replacement string
		want        string
	}{
		{"format version", "format_version = 2", "format_version = 3", "unsupported workspace format_version"},
		{"duplicate cache read layer", `.NETWORK`, `.GLOBAL`, "repeats layer"},
		{"fresh global cache write", `policy = "reuse_completed"`, `policy = "fresh"`, "fresh reuse policy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, strings.Replace(testConfig(), tc.replace, tc.replacement, 1)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %v, want %q", err, tc.want)
			}
		})
	}
}

// writeConfig supports the package test suite's write config setup or assertions.
func writeConfig(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace.something")
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// testConfig supports the package test suite's test config setup or assertions.
func testConfig() string {
	return `
available_filters: enum(string) = {
    NO_FILTER = "No filter";
    RANGE_10_YEARS = "Range 10 years";
    ARTICLE_ONLY = "Article only";
    ENGLISH_ONLY = "English only";
}
cache_policy_read_options: enum = {
    ACTIVE_RUN;
    GLOBAL;
    NETWORK;
    RUN_SPECIFIC;
}
cache_policy_write_options: enum = {
    ACTIVE_RUN;
    GLOBAL;
}
cache_policy_config: setup = {
    reads: []cache_policy_read_options;
    read_run_id?: integer = -1;
    writes: []cache_policy_write_options;
    negative_ttl_days: integer;
}
reuse_policy_config: setup = { policy: string; }
raw_data_filters: setup = {
    filters: []available_filters;
    count: integer;
}
source_declaration: setup = {
    name: string;
    date: timestamp;
    expected_file: string;
    file_type: string;
    query: string;
    filters: []raw_data_filters;
    expected_result_count: integer;
    requested_fields: []string;
    patch_fields: mapping(string, string);
    keep_fields: []string;
}
enrichment_provider_config: setup = {
    name: string;
    base_url: string;
    fields: []string;
    rate_per_second?: integer = 10;
    concurrency?: integer = 10;
    timeout_seconds?: integer = 30;
    max_retries?: integer = 5;
    batch_size?: integer = 50;
    extra_urls?: mapping(string, string) = mapping(string, string){};
    fill_missing_only?: boolean = false;
}
reviewer_config: setup = {
    username?: string = "";
    email?: string = "";
}
workspace_config: setup = {
    format_version: integer;
    search_id: string;
    search_revision: string;
    enrichment_enabled: boolean;
    reuse_policy: reuse_policy_config;
    cache_policy: cache_policy_config;
    sources: []source_declaration;
    enrichment_providers: []enrichment_provider_config;
    reviewer?: reviewer_config = reviewer_config {
        username = "",
        email = "",
    };
}
#iteration("_workspace"): scope = {
    workspace: workspace_config = {
        format_version = 2,
        search_id = "search-one",
        search_revision = "r1",
        enrichment_enabled = true,
        reuse_policy = reuse_policy_config { policy = "reuse_completed", },
        cache_policy = cache_policy_config {
            reads = []cache_policy_read_options { .GLOBAL, .NETWORK },
            writes = []cache_policy_write_options { .ACTIVE_RUN, .GLOBAL },
            negative_ttl_days = 7,
        },
        sources = [{
            name = "fixture",
            date = "2026-01-01 00:00:00",
            expected_file = "input.csv",
            file_type = "csv",
            query = "TITLE(test)",
            filters = []raw_data_filters{
                { filters = [.NO_FILTER], count = 10 },
                { filters = [.NO_FILTER, .ARTICLE_ONLY], count = 4 },
            },
            expected_result_count = 4,
            requested_fields = []string { "doi", "title" },
            patch_fields = mapping(string, string) { ["title"] => "title" },
            keep_fields = []string { "doi", "title" },
        }],
        enrichment_providers = [enrichment_provider_config {
            name = "crossref",
            base_url = "https://example.test/works/",
            fields = []string { "title" },
            fill_missing_only = true,
        }],
    };
}
#iteration("_workspace"): scope = {
    workspace: workspace_config = {
        format_version = 2,
        search_id = "search-two",
        search_revision = "r2",
        enrichment_enabled = false,
        reuse_policy = reuse_policy_config { policy = "reuse_completed", },
        cache_policy = cache_policy_config {
            reads = []cache_policy_read_options { .GLOBAL, .NETWORK },
            writes = []cache_policy_write_options { .ACTIVE_RUN, .GLOBAL },
            negative_ttl_days = 7,
        },
        sources = [{
            name = "fixture-two",
            date = "2026-01-01 00:00:00",
            expected_file = "input.csv",
            file_type = "csv",
            query = "TITLE(two)",
            filters = []raw_data_filters{
                { filters = [.NO_FILTER], count = 5 },
                { filters = [.NO_FILTER, .ARTICLE_ONLY], count = 2 },
            },
            expected_result_count = 2,
            requested_fields = []string { "doi", "title" },
            patch_fields = mapping(string, string) { ["title"] => "title" },
            keep_fields = []string { "doi", "title" },
        }],
        enrichment_providers = [],
    };
}
`
}

// TestConfigHelpers verifies config helpers.
func TestConfigHelpers(t *testing.T) {
	t.Run("requiredString", func(t *testing.T) {
		val, err := requiredString(map[string]any{"key": "hello"}, "key")
		if val != "hello" || err != nil {
			t.Fatalf(`got (%q, %v), want ("hello", nil)`, val, err)
		}

		_, err = requiredString(map[string]any{}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for missing key, got %v", err)
		}

		val, err = requiredString(map[string]any{"key": ""}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for empty key, got %v", err)
		}

		_, err = requiredString(map[string]any{"key": 42}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}
	})

	t.Run("optionalString", func(t *testing.T) {
		if val := optionalString(map[string]any{"key": "hello"}, "key"); val != "hello" {
			t.Fatalf(`got %q, want "hello"`, val)
		}

		if val := optionalString(map[string]any{}, "key"); val != "" {
			t.Fatalf(`got %q, want ""`, val)
		}

		if val := optionalString(map[string]any{"key": 42}, "key"); val != "" {
			t.Fatalf(`got %q, want ""`, val)
		}
	})

	t.Run("requiredBool", func(t *testing.T) {
		val, err := requiredBool(map[string]any{"key": true}, "key")
		if val != true || err != nil {
			t.Fatalf(`got (%v, %v), want (true, nil)`, val, err)
		}

		val, err = requiredBool(map[string]any{"key": false}, "key")
		if val != false || err != nil {
			t.Fatalf(`got (%v, %v), want (false, nil)`, val, err)
		}

		_, err = requiredBool(map[string]any{}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for missing key, got %v", err)
		}

		_, err = requiredBool(map[string]any{"key": "true"}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}
	})

	t.Run("optionalBool", func(t *testing.T) {
		val, err := optionalBool(map[string]any{"key": true}, "key", false)
		if val != true || err != nil {
			t.Fatalf(`got (%v, %v), want (true, nil)`, val, err)
		}

		val, err = optionalBool(map[string]any{"key": false}, "key", true)
		if val != false || err != nil {
			t.Fatalf(`got (%v, %v), want (false, nil)`, val, err)
		}

		val, err = optionalBool(map[string]any{}, "key", true)
		if val != true || err != nil {
			t.Fatalf(`got (%v, %v), want (true, nil)`, val, err)
		}

		_, err = optionalBool(map[string]any{"key": 42}, "key", false)
		if err == nil || !strings.Contains(err.Error(), "key must be a boolean") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}
	})

	t.Run("requiredInt", func(t *testing.T) {
		val, err := requiredInt(map[string]any{"key": 7}, "key")
		if val != 7 || err != nil {
			t.Fatalf(`got (%d, %v), want (7, nil)`, val, err)
		}

		_, err = requiredInt(map[string]any{}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for missing key, got %v", err)
		}

		_, err = requiredInt(map[string]any{"key": "7"}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}

		_, err = requiredInt(map[string]any{"key": 7.5}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for non-integer float, got %v", err)
		}
	})

	t.Run("optionalInt", func(t *testing.T) {
		val, err := optionalInt(map[string]any{"key": 3}, "key", 99)
		if val != 3 || err != nil {
			t.Fatalf(`got (%d, %v), want (3, nil)`, val, err)
		}

		val, err = optionalInt(map[string]any{}, "key", 99)
		if val != 99 || err != nil {
			t.Fatalf(`got (%d, %v), want (99, nil)`, val, err)
		}

		_, err = optionalInt(map[string]any{"key": "hello"}, "key", 99)
		if err == nil || !strings.Contains(err.Error(), "key must be an integer") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}
	})

	t.Run("optionalStringMap", func(t *testing.T) {
		val, err := optionalStringMap(map[string]any{"key": map[string]any{"a": "1", "b": "2"}}, "key")
		if err != nil {
			t.Fatal(err)
		}
		if len(val) != 2 || val["a"] != "1" || val["b"] != "2" {
			t.Fatalf("got %v, want map[a:1 b:2]", val)
		}

		val, err = optionalStringMap(map[string]any{}, "key")
		if err != nil {
			t.Fatal(err)
		}
		if len(val) != 0 {
			t.Fatalf("got %v, want empty map", val)
		}

		_, err = optionalStringMap(map[string]any{"key": "not-a-map"}, "key")
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}

		_, err = optionalStringMap(map[string]any{"key": map[string]any{"a": 1}}, "key")
		if err == nil || !strings.Contains(err.Error(), `key["a"] must be a string`) {
			t.Fatalf("expected error for non-string value, got %v", err)
		}
	})

	t.Run("enumIntList", func(t *testing.T) {
		names := []string{"A", "B", "C"}
		val, err := enumIntList(map[string]any{"key": []any{0, 1, 2}}, "key", names)
		if err != nil {
			t.Fatal(err)
		}
		if len(val) != 3 || val[0] != 0 || val[1] != 1 || val[2] != 2 {
			t.Fatalf("got %v, want [0 1 2]", val)
		}

		_, err = enumIntList(map[string]any{}, "key", names)
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for missing key, got %v", err)
		}

		_, err = enumIntList(map[string]any{"key": "not-a-list"}, "key", names)
		if err == nil || !strings.Contains(err.Error(), "key is required") {
			t.Fatalf("expected error for wrong type, got %v", err)
		}

		_, err = enumIntList(map[string]any{"key": []any{"x"}}, "key", names)
		if err == nil || !strings.Contains(err.Error(), "key[0] must be an enum value") {
			t.Fatalf("expected error for non-int element, got %v", err)
		}

		_, err = enumIntList(map[string]any{"key": []any{5}}, "key", names)
		if err == nil || !strings.Contains(err.Error(), "key[0] enum ordinal 5 out of range") {
			t.Fatalf("expected error for out-of-range ordinal, got %v", err)
		}

		_, err = enumIntList(map[string]any{"key": []any{-1}}, "key", names)
		if err == nil || !strings.Contains(err.Error(), "key[0] enum ordinal -1 out of range") {
			t.Fatalf("expected error for negative ordinal, got %v", err)
		}
	})
}
