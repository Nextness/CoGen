// Package manifest defines the canonical resolved-config and input manifests
// for a workspace pipeline run. It provides deterministic serialization and
// fingerprint computation for execution-plan identification and stage reuse.
//
// The resolved-config manifest is built from the SOMETHING evaluation result
// before any source file is read. The input manifest is created after hashing
// declared source files but before parsing. Together they form the execution
// fingerprint that uniquely identifies a plan.
package manifest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// ResolvedManifest is the canonical resolved configuration snapshot built from
// the SOMETHING evaluation result. It contains all interpolated values,
// expanded paths, resolved source settings, field lists, run policy, and
// pipeline-affecting configuration that determines the execution fingerprint.
// The pipeline builds this before any source file is read, persists it, and
// uses the in-memory copy for the rest of the attempt.
type ResolvedManifest struct {
	// FormatVersion is the workspace config format version (e.g. 2).
	FormatVersion int `json:"format_version"`

	// SearchID is the stable, human-chosen identifier for one research question.
	SearchID string `json:"search_id"`

	// SearchRevision is the intentional revision of query, filters, source
	// selection, or field policy.
	SearchRevision string `json:"search_revision"`

	// EnrichmentEnabled is required and determines whether this declared run
	// performs enrichment. It is part of the run manifest, not a CLI override.
	EnrichmentEnabled bool `json:"enrichment_enabled"`

	// ReusePolicy declares whether a matching completed plan may be reused.
	ReusePolicy string `json:"reuse_policy"`

	// CachePolicy declares the ordered cache read layers and explicit write
	// destinations.
	CachePolicy CachePolicy `json:"cache_policy"`

	// Sources is the ordered list of resolved source declarations.
	Sources []SourceManifest `json:"sources"`

	// EnrichmentProviders is the list of enrichment API configurations
	// including provider name, base URL, requested fields, and fill-missing
	// policy. Changes to provider settings alter the execution fingerprint.
	EnrichmentProviders []EnrichmentProvider `json:"enrichment_providers,omitempty"`

	// SchemaVersion is the database schema version (latest migration filename)
	// that this manifest was built against.
	SchemaVersion string `json:"schema_version"`
}

// CachePolicy declares cache read layers, write destinations, optional
// read_run_id for run-specific reads, and negative-entry TTL.
// Valid read layer names: "active_run", "global", "network", "run_specific",
// "run:N" (prior-run snapshot). Valid write layer names: "active_run", "global".
type CachePolicy struct {
	// Reads is the ordered list of cache layers to consult when reading.
	Reads []string `json:"reads"`

	// Writes is the list of cache layers to write to.
	Writes []string `json:"writes"`

	// ReadRunID specifies a specific run to read from when Reads contains
	// "run_specific". Ignored otherwise. Default -1; an error is raised when
	// "run_specific" is used without setting this to a valid run ID.
	ReadRunID int `json:"read_run_id,omitempty"`

	// NegativeTTLDays is the TTL in days for negative cache entries (404s, etc.).
	NegativeTTLDays int `json:"negative_ttl_days"`
}

// EnrichmentProvider describes one enrichment API configuration that affects
// the execution fingerprint. Each provider has a name, endpoint URL, requested
// fields, and whether it fills only missing values. The full set of configurable
// provider settings is captured so that any behavior-changing edit produces a
// distinct execution fingerprint.
type EnrichmentProvider struct {
	// Name is the provider key (e.g. "crossref", "openalex", "orcid").
	Name string `json:"name"`

	// BaseURL is the resolved API endpoint URL.
	BaseURL string `json:"base_url"`

	// ExtraURLs is a map of additional named URL endpoints for this provider
	// (e.g. ORCID's author search endpoint).
	ExtraURLs map[string]string `json:"extra_urls,omitempty"`

	// Fields is the list of enrichment fields requested from this provider.
	Fields []string `json:"fields"`

	// FillMissingOnly indicates whether this provider stores only fields
	// that are still missing after prior providers.
	FillMissingOnly bool `json:"fill_missing_only"`

	// RatePerSecond is the maximum number of requests per second.
	RatePerSecond int `json:"rate_per_second"`

	// Concurrency is the maximum number of concurrent requests.
	Concurrency int `json:"concurrency"`

	// TimeoutSeconds is the HTTP request timeout in seconds.
	TimeoutSeconds int `json:"timeout_seconds"`

	// MaxRetries is the number of retries after the initial failed request.
	MaxRetries int `json:"max_retries"`

	// BatchSize is the number of items per batch request (e.g. OpenAlex
	// reference resolution).
	BatchSize int `json:"batch_size"`
}

// RawDataFilter records one stage of source-level filtering with its
// cumulative article count. Filters are ordered from least to most
// restrictive within a source declaration.
type RawDataFilter struct {
	// Filters is the ordered list of filter names applied at this stage.
	Filters []string `json:"filters"`

	// Count is the number of articles that pass this filter stage.
	Count int `json:"count"`
}

// SourceManifest is a resolved source declaration with all interpolation
// expanded and paths resolved.
type SourceManifest struct {
	// Name is the source identifier (e.g. "scopus", "ieeexplore", "wos").
	Name string `json:"name"`

	// ExpectedFile is the resolved path to the raw source file.
	ExpectedFile string `json:"expected_file"`

	// FileType is the source file type (e.g. "csv", "bib"). Determines which
	// parser is used during ingestion.
	FileType string `json:"file_type"`

	// Query is the search query used to obtain this source.
	Query string `json:"query"`

	// Filters records the ordered source-level filter stages and their
	// cumulative article counts. The first entry (NO_FILTER) is the raw
	// total; subsequent entries represent progressive filter application.
	Filters []RawDataFilter `json:"filters,omitempty"`

	// ExpectedResultCount records the declared count for the selected filters.
	// It is an expectation, not a count measured while parsing the export.
	ExpectedResultCount int `json:"expected_result_count,omitempty"`

	// Date records when the source export was downloaded (provenance).
	Date string `json:"date,omitempty"`

	// RequestedFields is the list of enrichment fields requested for this source.
	RequestedFields []string `json:"requested_fields"`

	// PatchFields maps source-specific field names to canonical field names.
	// Changes to rename mappings alter the execution fingerprint.
	PatchFields map[string]string `json:"patch_fields,omitempty"`

	// KeepFields is the ordered whitelist of fields to retain after renaming.
	// Changes to the keep list alter the execution fingerprint.
	KeepFields []string `json:"keep_fields,omitempty"`
}

// InputManifest records the state of declared source files before parsing.
// It links to the resolved-config manifest via ResolvedManifestHash and
// supplies the source hashes for the execution-plan fingerprint.
type InputManifest struct {
	// ResolvedManifestHash is the SHA-256 hex digest of the canonical JSON
	// representation of the ResolvedManifest this input manifest belongs to.
	ResolvedManifestHash string `json:"resolved_manifest_hash"`

	// SourceFiles maps source name to its file information.
	SourceFiles map[string]SourceFileInfo `json:"source_files"`
}

// SourceFileInfo holds the identity and content hash of a single source file.
type SourceFileInfo struct {
	// Path is the resolved absolute or relative path to the source file.
	Path string `json:"path"`

	// SHA256 is the hex-encoded SHA-256 digest of the file content.
	SHA256 string `json:"sha256"`

	// Size is the file size in bytes.
	Size int64 `json:"size"`

	// ReadError records why the configured source could not be read during
	// preflight. It is present only on a failed input manifest; SHA256 and Size
	// are then unavailable and must not be interpreted as source content data.
	ReadError string `json:"read_error,omitempty"`
}

// ArtifactLayout defines where and how pipeline artifacts are stored.
type ArtifactLayout struct {
	// Root is the root directory for artifact storage.
	Root string `json:"root"`

	// ContentHashAlgo is the hash algorithm used for content-addressed naming
	// (e.g. "sha256").
	ContentHashAlgo string `json:"content_hash_algo"`

	// RetentionPolicy describes how artifacts are retained:
	// "per_run" — artifacts are scoped to and removed with the owning run.
	// "shared" — artifacts are content-addressed and retained while any run
	// references them.
	RetentionPolicy string `json:"retention_policy"`
}

// CacheStoreLayout defines the global cache store location.
type CacheStoreLayout struct {
	// Path is the filesystem path to the global cache store.
	Path string `json:"path"`
}

// ExecutionFingerprint is a SHA-256 hex digest that uniquely identifies an
// execution plan. It is deterministic: semantically equivalent resolved and
// input manifests produce the same fingerprint.
type ExecutionFingerprint string

// StageFingerprint identifies a single pipeline stage's input/output for
// reuse detection. A changed source-file hash, config, or upstream stage
// output produces a different stage fingerprint.
type StageFingerprint struct {
	// Stage is the stage name (e.g. "parse", "deduplicate", "enrich").
	Stage string `json:"stage"`

	// InputFingerprint is the execution fingerprint of the upstream input.
	InputFingerprint ExecutionFingerprint `json:"input_fingerprint"`

	// ConfigHash is the SHA-256 hex digest of the stage-specific configuration.
	ConfigHash string `json:"config_hash"`

	// OutputHash is the SHA-256 hex digest of the stage output, if available.
	OutputHash string `json:"output_hash,omitempty"`
}

// canonicalJSON returns the canonical JSON representation of v.
// The output is deterministic: struct fields are serialized in declaration
// order, map keys are sorted alphabetically, and no extra whitespace is added.
// Go's encoding/json performs default HTML escaping of <, >, &, which is
// deterministic and does not affect fingerprint stability.
func canonicalJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	return b, nil
}

// fingerprintInput combines the resolved and input manifests for fingerprint
// computation. The ResolvedManifestHash in InputManifest is excluded from the
// fingerprint to avoid circular dependency; the full ResolvedManifest is
// included directly.
type fingerprintInput struct {
	Resolved ResolvedManifest            `json:"resolved"`
	Input    inputManifestForFingerprint `json:"input"`
}

// inputManifestForFingerprint restricts input fingerprinting to authoritative source-file evidence.
type inputManifestForFingerprint struct {
	SourceFiles map[string]SourceFileInfo `json:"source_files"`
}

// ComputeFingerprint computes a deterministic execution fingerprint from the
// resolved manifest and input manifest pair. The fingerprint is the SHA-256
// hex digest of the canonical JSON representation of both manifests combined.
//
// The input manifest's ResolvedManifestHash is a persistence link, not a
// fingerprint input. It is excluded from the hash to avoid circular dependency.
// Callers should use NewInputManifest to create correctly linked pairs.
func ComputeFingerprint(rm *ResolvedManifest, im *InputManifest) (ExecutionFingerprint, error) {
	if rm == nil {
		return "", fmt.Errorf("manifest: resolved manifest is nil")
	}
	if im == nil {
		return "", fmt.Errorf("manifest: input manifest is nil")
	}

	input := fingerprintInput{
		Resolved: *rm,
		Input: inputManifestForFingerprint{
			SourceFiles: im.SourceFiles,
		},
	}

	jsonBytes, err := canonicalJSON(input)
	if err != nil {
		return "", fmt.Errorf("manifest: fingerprint serialization: %w", err)
	}

	digest := sha256.Sum256(jsonBytes)
	return ExecutionFingerprint(fmt.Sprintf("%x", digest)), nil
}

// ComputeStageFingerprint computes a deterministic fingerprint for a single
// pipeline stage from its identity, upstream input fingerprint,
// stage-specific configuration, and stage output hash. A non-empty outputHash
// is recorded for reuse detection; pass "" when the output is not yet computed.
func ComputeStageFingerprint(stage string, inputFP ExecutionFingerprint, stageConfig any, outputHash string) (StageFingerprint, error) {
	if stage == "" {
		return StageFingerprint{}, fmt.Errorf("manifest: stage name is empty")
	}

	configHash := ""
	if stageConfig != nil {
		b, err := canonicalJSON(stageConfig)
		if err != nil {
			return StageFingerprint{}, fmt.Errorf("manifest: stage config hash: %w", err)
		}
		digest := sha256.Sum256(b)
		configHash = fmt.Sprintf("%x", digest)
	}

	return StageFingerprint{
		Stage:            stage,
		InputFingerprint: inputFP,
		ConfigHash:       configHash,
		OutputHash:       outputHash,
	}, nil
}

// Hash computes the SHA-256 hex digest of the canonical JSON representation
// of the ResolvedManifest.
func (rm *ResolvedManifest) Hash() (string, error) {
	if rm == nil {
		return "", fmt.Errorf("manifest: resolved manifest is nil")
	}
	b, err := canonicalJSON(rm)
	if err != nil {
		return "", fmt.Errorf("manifest: resolved manifest hash: %w", err)
	}
	digest := sha256.Sum256(b)
	return fmt.Sprintf("%x", digest), nil
}

// NewInputManifest creates a new InputManifest linked to the given resolved
// manifest. It computes the ResolvedManifestHash automatically.
func NewInputManifest(rm *ResolvedManifest, sourceFiles map[string]SourceFileInfo) (*InputManifest, error) {
	if rm == nil {
		return nil, fmt.Errorf("manifest: resolved manifest is nil")
	}
	rmHash, err := rm.Hash()
	if err != nil {
		return nil, err
	}

	// Ensure deterministic key order for the map
	sf := make(map[string]SourceFileInfo, len(sourceFiles))
	for k, v := range sourceFiles {
		sf[k] = v
	}

	return &InputManifest{
		ResolvedManifestHash: rmHash,
		SourceFiles:          sf,
	}, nil
}

// SourceNames returns the source names in sorted order.
func (im *InputManifest) SourceNames() []string {
	names := make([]string, 0, len(im.SourceFiles))
	for name := range im.SourceFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
