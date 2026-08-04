// Package workspace loads the declared workspace iterations used by pipeline
// execution. It owns the boundary between SOMETHING values and the typed,
// immutable manifest consumed by the rest of the program.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"analysis/enrich"
	"analysis/manifest"
	"analysis/something"
)

const SupportedFormatVersion = 2

// Config is one immutable evaluation of a workspace configuration file.
// OriginalBytes are retained so a run can persist exactly what was evaluated.
type Config struct {
	OriginalBytes []byte
	Runs          []*Run
}

// Run is one declared workspace iteration.
type Run struct {
	Manifest   *manifest.ResolvedManifest
	Enrichment *enrich.Config
}

// Selector identifies one workspace iteration by its stable search ID and
// revision label. Its CLI form is search_id@search_revision.
func Selector(searchID, revision string) string {
	return searchID + "@" + revision
}

// Load reads a workspace configuration exactly once, evaluates those bytes,
// and converts every workspace iteration to typed run configuration.
func Load(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace config path: %w", err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read workspace config: %w", err)
	}
	values, err := something.LoadSomethingBytes(data, absPath)
	if err != nil {
		return nil, fmt.Errorf("evaluate workspace config: %w", err)
	}

	entries, err := something.GetStructAll(values, "[iteration]_workspace")
	if err != nil {
		return nil, fmt.Errorf("get workspace iterations: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("workspace config has no workspace iterations")
	}

	configDir := filepath.Dir(absPath)
	result := &Config{OriginalBytes: data, Runs: make([]*Run, 0, len(entries))}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, declared := entry["workspace"]; !declared {
			continue
		}
		run, err := parseRun(entry, configDir)
		if err != nil {
			return nil, err
		}
		selector := Selector(run.Manifest.SearchID, run.Manifest.SearchRevision)
		if _, exists := seen[selector]; exists {
			return nil, fmt.Errorf("duplicate workspace iteration %q", selector)
		}
		seen[selector] = struct{}{}
		result.Runs = append(result.Runs, run)
	}
	if len(result.Runs) == 0 {
		return nil, fmt.Errorf("workspace config has no declared workspace values")
	}
	return result, nil
}

// Select returns all runs when selectors is empty. Otherwise it returns the
// requested iterations in declaration order and rejects unknown selectors.
func (c *Config) Select(selectors []string) ([]*Run, error) {
	if c == nil {
		return nil, fmt.Errorf("workspace config is nil")
	}
	if len(selectors) == 0 {
		return append([]*Run(nil), c.Runs...), nil
	}
	wanted := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		if selector == "" || !strings.Contains(selector, "@") {
			return nil, fmt.Errorf("invalid workspace selector %q; use search_id@search_revision", selector)
		}
		wanted[selector] = struct{}{}
	}
	selected := make([]*Run, 0, len(selectors))
	for _, run := range c.Runs {
		selector := Selector(run.Manifest.SearchID, run.Manifest.SearchRevision)
		if _, ok := wanted[selector]; ok {
			selected = append(selected, run)
			delete(wanted, selector)
		}
	}
	if len(wanted) != 0 {
		unknown := make([]string, 0, len(wanted))
		for selector := range wanted {
			unknown = append(unknown, selector)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("workspace selector not found: %s", strings.Join(unknown, ", "))
	}
	return selected, nil
}

// parseRun parses run from the supplied input.
func parseRun(entry map[string]any, configDir string) (*Run, error) {
	workspace, ok := entry["workspace"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("workspace scope is required in iteration")
	}
	formatVersion, err := requiredInt(workspace, "format_version")
	if err != nil {
		return nil, err
	}
	if formatVersion != SupportedFormatVersion {
		return nil, fmt.Errorf("unsupported workspace format_version %d; supported version is %d", formatVersion, SupportedFormatVersion)
	}
	searchID, err := requiredString(workspace, "search_id")
	if err != nil {
		return nil, err
	}
	revision, err := requiredString(workspace, "search_revision")
	if err != nil {
		return nil, err
	}
	enrichmentEnabled, err := requiredBool(workspace, "enrichment_enabled")
	if err != nil {
		return nil, err
	}
	reusePolicy, err := nestedString(workspace, "reuse_policy", "policy")
	if err != nil {
		return nil, err
	}
	if reusePolicy != "reuse_completed" && reusePolicy != "fresh" && reusePolicy != "retry" {
		return nil, fmt.Errorf("invalid reuse policy %q", reusePolicy)
	}
	cachePolicy, err := parseCachePolicy(workspace, reusePolicy)
	if err != nil {
		return nil, err
	}
	sources, err := parseSources(workspace, configDir)
	if err != nil {
		return nil, err
	}
	providers, enrichConfig, err := parseProviders(workspace)
	if err != nil {
		return nil, err
	}
	if enrichmentEnabled && len(providers) == 0 {
		return nil, fmt.Errorf("workspace %q enables enrichment but declares no providers", Selector(searchID, revision))
	}
	return &Run{
		Manifest: &manifest.ResolvedManifest{
			FormatVersion:       formatVersion,
			SearchID:            searchID,
			SearchRevision:      revision,
			EnrichmentEnabled:   enrichmentEnabled,
			ReusePolicy:         reusePolicy,
			CachePolicy:         cachePolicy,
			Sources:             sources,
			EnrichmentProviders: providers,
		},
		Enrichment: enrichConfig,
	}, nil
}

var cacheReadLayerNames = []string{"active_run", "global", "network", "run_specific"}
var cacheWriteLayerNames = []string{"active_run", "global"}
var availableFilterNames = []string{"NO_FILTER", "RANGE_10_YEARS", "ARTICLE_ONLY", "ENGLISH_ONLY"}

// parseCachePolicy parses cache policy from the supplied input.
func parseCachePolicy(entry map[string]any, reusePolicy string) (manifest.CachePolicy, error) {
	policy, ok := entry["cache_policy"].(map[string]any)
	if !ok {
		return manifest.CachePolicy{}, fmt.Errorf("cache_policy is required")
	}
	readOrdinals, err := enumIntList(policy, "reads", cacheReadLayerNames)
	if err != nil {
		return manifest.CachePolicy{}, err
	}
	writeOrdinals, err := enumIntList(policy, "writes", cacheWriteLayerNames)
	if err != nil {
		return manifest.CachePolicy{}, err
	}
	reads := make([]string, len(readOrdinals))
	for i, ord := range readOrdinals {
		reads[i] = cacheReadLayerNames[ord]
	}
	writes := make([]string, len(writeOrdinals))
	for i, ord := range writeOrdinals {
		writes[i] = cacheWriteLayerNames[ord]
	}
	ttl, err := requiredInt(policy, "negative_ttl_days")
	if err != nil {
		return manifest.CachePolicy{}, err
	}
	if ttl < 0 {
		return manifest.CachePolicy{}, fmt.Errorf("cache_policy.negative_ttl_days must not be negative")
	}
	if len(reads) == 0 || len(writes) == 0 {
		return manifest.CachePolicy{}, fmt.Errorf("cache_policy requires at least one read and one write layer")
	}
	readRunID, err := optionalInt(policy, "read_run_id", -1)
	if err != nil {
		return manifest.CachePolicy{}, err
	}

	// Validate read_run_id: required when run_specific is used, must be > 0
	hasRunSpecific := false
	hasRunN := false
	for _, r := range reads {
		if r == "run_specific" {
			hasRunSpecific = true
		}
		if strings.HasPrefix(r, "run:") {
			hasRunN = true
		}
	}
	if hasRunSpecific {
		if readRunID <= 0 {
			return manifest.CachePolicy{}, fmt.Errorf("cache_policy.read_run_id must be set to a valid run ID when reads contains run_specific")
		}
		// Replace run_specific with run:N
		for i, r := range reads {
			if r == "run_specific" {
				reads[i] = "run:" + strconv.FormatInt(int64(readRunID), 10)
			}
		}
	}
	if hasRunN {
		if err := validateRunLayer(reads); err != nil {
			return manifest.CachePolicy{}, err
		}
	}
	if err := validateCacheLayers(reads, false); err != nil {
		return manifest.CachePolicy{}, err
	}
	if err := validateCacheLayers(writes, true); err != nil {
		return manifest.CachePolicy{}, err
	}
	if reusePolicy == "fresh" && contains(writes, "global") {
		return manifest.CachePolicy{}, fmt.Errorf("fresh reuse policy must not write to the global cache")
	}
	return manifest.CachePolicy{Reads: reads, Writes: writes, ReadRunID: readRunID, NegativeTTLDays: ttl}, nil
}

// parseSources parses sources from the supplied input.
func parseSources(entry map[string]any, configDir string) ([]manifest.SourceManifest, error) {
	rawSources, ok := entry["sources"].([]any)
	if !ok || len(rawSources) == 0 {
		return nil, fmt.Errorf("sources must contain at least one declaration")
	}
	sources := make([]manifest.SourceManifest, 0, len(rawSources))
	seen := make(map[string]struct{}, len(rawSources))
	for _, raw := range rawSources {
		source, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("source declaration is not an object")
		}
		name, err := requiredString(source, "name")
		if err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate source name %q", name)
		}
		seen[name] = struct{}{}
		file, err := requiredPath(source, "expected_file", configDir)
		if err != nil {
			return nil, err
		}
		fileType, err := requiredString(source, "file_type")
		if err != nil {
			return nil, err
		}
		if fileType != "csv" && fileType != "bib" {
			return nil, fmt.Errorf("source %q has unsupported file_type %q", name, fileType)
		}
		if ext := strings.TrimPrefix(filepath.Ext(file), "."); ext != fileType {
			return nil, fmt.Errorf("source %q file type %q does not match path extension %q", name, fileType, ext)
		}
		query, err := requiredString(source, "query")
		if err != nil {
			return nil, err
		}
		requestedFields, err := stringList(source, "requested_fields")
		if err != nil {
			return nil, err
		}
		filters, err := parseRawDataFilters(source, "filters")
		if err != nil {
			return nil, err
		}
		count, err := requiredInt(source, "expected_result_count")
		if err != nil {
			return nil, err
		}
		if count < 0 {
			return nil, fmt.Errorf("source %q expected_result_count must not be negative", name)
		}
		patchFields, err := stringMap(source, "patch_fields")
		if err != nil {
			return nil, err
		}
		keepFields, err := stringList(source, "keep_fields")
		if err != nil {
			return nil, err
		}
		sources = append(sources, manifest.SourceManifest{
			Name:                name,
			ExpectedFile:        file,
			FileType:            fileType,
			Query:               query,
			Filters:             filters,
			ExpectedResultCount: count,
			Date:                optionalString(source, "date"),
			RequestedFields:     requestedFields,
			PatchFields:         patchFields,
			KeepFields:          keepFields,
		})
	}
	return sources, nil
}

// parseProviders parses providers from the supplied input.
func parseProviders(entry map[string]any) ([]manifest.EnrichmentProvider, *enrich.Config, error) {
	rawProviders, ok := entry["enrichment_providers"].([]any)
	if !ok {
		return nil, nil, fmt.Errorf("enrichment_providers is required")
	}
	providers := make([]manifest.EnrichmentProvider, 0, len(rawProviders))
	config := &enrich.Config{Sources: make(map[string]enrich.SourceConfig, len(rawProviders))}
	for _, raw := range rawProviders {
		provider, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("enrichment provider is not an object")
		}
		name, err := requiredString(provider, "name")
		if err != nil {
			return nil, nil, err
		}
		if _, exists := config.Sources[name]; exists {
			return nil, nil, fmt.Errorf("duplicate enrichment provider %q", name)
		}
		baseURL, err := requiredString(provider, "base_url")
		if err != nil {
			return nil, nil, err
		}
		fields, err := stringList(provider, "fields")
		if err != nil {
			return nil, nil, err
		}
		extraURLs, err := optionalStringMap(provider, "extra_urls")
		if err != nil {
			return nil, nil, err
		}
		rate, err := optionalInt(provider, "rate_per_second", 10)
		if err != nil {
			return nil, nil, err
		}
		concurrency, err := optionalInt(provider, "concurrency", 10)
		if err != nil {
			return nil, nil, err
		}
		timeout, err := optionalInt(provider, "timeout_seconds", 30)
		if err != nil {
			return nil, nil, err
		}
		retries, err := optionalInt(provider, "max_retries", 5)
		if err != nil {
			return nil, nil, err
		}
		batchSize, err := optionalInt(provider, "batch_size", 50)
		if err != nil {
			return nil, nil, err
		}
		fillMissing, err := optionalBool(provider, "fill_missing_only", false)
		if err != nil {
			return nil, nil, err
		}
		userAgent, _ := provider["user_agent"].(string)
		contactEmail, _ := provider["contact_email"].(string)
		providers = append(providers, manifest.EnrichmentProvider{
			Name: name, BaseURL: baseURL, ExtraURLs: extraURLs, Fields: fields,
			FillMissingOnly: fillMissing, RatePerSecond: rate, Concurrency: concurrency,
			TimeoutSeconds: timeout, MaxRetries: retries, BatchSize: batchSize,
		})
		config.Sources[name] = enrich.SourceConfig{
			Name: name, BaseURL: baseURL, UserAgent: userAgent, ContactEmail: contactEmail,
			RatePerSecond: rate, Concurrency: concurrency, TimeoutSecs: timeout,
			MaxRetries: retries, Fields: fields,
			ExtraURLs: extraURLs, BatchSize: batchSize, FillMissingOnly: fillMissing,
		}
	}
	return providers, config, nil
}

// requiredString returns a required non-empty string from evaluated configuration.
func requiredString(values map[string]any, name string) (string, error) {
	value, ok := values[name].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

// optionalString reads the optional string value.
func optionalString(values map[string]any, name string) string {
	value, ok := values[name].(string)
	if !ok {
		return ""
	}
	return value
}

// nestedString reads a required string from a required nested mapping.
func nestedString(values map[string]any, parent, name string) (string, error) {
	nested, ok := values[parent].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s is required", parent)
	}
	return requiredString(nested, name)
}

// requiredBool returns a required Boolean from evaluated configuration.
func requiredBool(values map[string]any, name string) (bool, error) {
	value, ok := values[name].(bool)
	if !ok {
		return false, fmt.Errorf("%s is required", name)
	}
	return value, nil
}

// optionalBool reads the optional bool value.
func optionalBool(values map[string]any, name string, fallback bool) (bool, error) {
	value, exists := values[name]
	if !exists {
		return fallback, nil
	}
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return b, nil
}

// requiredInt returns a required integer from evaluated configuration.
func requiredInt(values map[string]any, name string) (int, error) {
	value, ok := values[name].(int)
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	return value, nil
}

// optionalInt reads the optional int value.
func optionalInt(values map[string]any, name string, fallback int) (int, error) {
	value, exists := values[name]
	if !exists {
		return fallback, nil
	}
	i, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return i, nil
}

// requiredPath returns a required path resolved relative to the configuration directory.
func requiredPath(values map[string]any, name, configDir string) (string, error) {
	path, err := requiredString(values, name)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}
	return filepath.Clean(path), nil
}

// stringList reads a required list of non-empty strings.
func stringList(values map[string]any, name string) ([]string, error) {
	raw, ok := values[name].([]any)
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	result := make([]string, len(raw))
	for i, value := range raw {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string", name, i)
		}
		result[i] = text
	}
	return result, nil
}

// stringMap reads a required mapping whose keys and values are strings.
func stringMap(values map[string]any, name string) (map[string]string, error) {
	return requiredStringMap(values, name, true)
}

// optionalStringMap reads the optional string map value.
func optionalStringMap(values map[string]any, name string) (map[string]string, error) {
	return requiredStringMap(values, name, false)
}

// requiredStringMap validates a required or optional mapping of non-empty strings.
func requiredStringMap(values map[string]any, name string, required bool) (map[string]string, error) {
	raw, exists := values[name]
	if !exists && !required {
		return map[string]string{}, nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	result := make(map[string]string, len(mapping))
	for key, value := range mapping {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%q] must be a string", name, key)
		}
		result[key] = text
	}
	return result, nil
}

// validateCacheLayers validates cache-layer names, uniqueness, and read or write eligibility.
func validateCacheLayers(layers []string, write bool) error {
	seen := make(map[string]struct{}, len(layers))
	for _, layer := range layers {
		if _, exists := seen[layer]; exists {
			return fmt.Errorf("cache policy repeats layer %q", layer)
		}
		seen[layer] = struct{}{}
		if write {
			if layer != "active_run" && layer != "global" {
				return fmt.Errorf("cache write layer %q is invalid", layer)
			}
			continue
		}
		if layer == "active_run" || layer == "global" || layer == "network" {
			continue
		}
		if strings.HasPrefix(layer, "run:") {
			if _, err := strconv.ParseInt(strings.TrimPrefix(layer, "run:"), 10, 64); err == nil {
				continue
			}
		}
		return fmt.Errorf("cache read layer %q is invalid", layer)
	}
	return nil
}

// validateRunLayer checks that a resolved run:N layer has a valid run ID.
func validateRunLayer(reads []string) error {
	for _, r := range reads {
		if strings.HasPrefix(r, "run:") {
			idStr := strings.TrimPrefix(r, "run:")
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid run layer %q: run ID must be a positive integer", r)
			}
		}
	}
	return nil
}

// enumIntList reads a list of int enum ordinals from a map and validates
// they are within range of the provided member names list.
func enumIntList(values map[string]any, name string, memberNames []string) ([]int, error) {
	raw, ok := values[name].([]any)
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	result := make([]int, len(raw))
	for i, value := range raw {
		ord, ok := value.(int)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an enum value (integer)", name, i)
		}
		if ord < 0 || ord >= len(memberNames) {
			return nil, fmt.Errorf("%s[%d] enum ordinal %d out of range (0-%d)", name, i, ord, len(memberNames)-1)
		}
		result[i] = ord
	}
	return result, nil
}

// parseRawDataFilters reads the "filters" field as a list of raw_data_filters
// structs, each containing a "filters" array of available_filters enum ordinals
// and a "count" integer.
func parseRawDataFilters(source map[string]any, name string) ([]manifest.RawDataFilter, error) {
	raw, ok := source[name].([]any)
	if !ok {
		// filters is optional; return empty slice if absent
		return nil, nil
	}
	result := make([]manifest.RawDataFilter, len(raw))
	for i, item := range raw {
		filterDef, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", name, i)
		}
		filterOrdinals, err := enumIntList(filterDef, "filters", availableFilterNames)
		if err != nil {
			return nil, fmt.Errorf("%s[%d].filters: %w", name, i, err)
		}
		filterNames := make([]string, len(filterOrdinals))
		for j, ord := range filterOrdinals {
			filterNames[j] = availableFilterNames[ord]
		}
		count, err := requiredInt(filterDef, "count")
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		if count < 0 {
			return nil, fmt.Errorf("%s[%d] count must not be negative", name, i)
		}
		result[i] = manifest.RawDataFilter{Filters: filterNames, Count: count}
	}
	return result, nil
}

// contains reports whether a string slice contains an exact target.
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
