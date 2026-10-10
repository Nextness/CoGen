// types.go defines the search family application types. Generated sqlc row
// types stay inside the private internal/sql package and never cross this
// family boundary.
package search

// Search represents a single research question (stable identity).
type Search struct {
	ID        int64  `json:"id"`
	SearchID  string `json:"search_id"`
	CreatedAt string `json:"created_at"`
}

// Revision is a researcher-managed grouping for one version of a search's
// query intent. Its hashes describe the latest observed declaration;
// immutable historical configuration belongs to execution plans and attempts.
type Revision struct {
	ID                   int64  `json:"id"`
	SearchID             int64  `json:"search_id"`
	RevisionLabel        string `json:"revision_label"`
	ConfigArtifactHash   string `json:"config_artifact_hash"`
	ResolvedManifestHash string `json:"resolved_manifest_hash"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at,omitempty"`
}

// Plan represents a unique fingerprint per search revision and input policy.
type Plan struct {
	ID                   int64  `json:"id"`
	SearchRevisionID     int64  `json:"search_revision_id"`
	ExecutionFingerprint string `json:"execution_fingerprint"`
	ResolvedManifestHash string `json:"resolved_manifest_hash"`
	InputManifestHash    string `json:"input_manifest_hash"`
	EnrichmentEnabled    bool   `json:"enrichment_enabled"`
	CreatedAt            string `json:"created_at"`
}

// RevisionInput identifies one search revision declaration and the hashes of
// its latest observed configuration and resolved manifest.
type RevisionInput struct {
	SearchID             int64
	RevisionLabel        string
	ConfigArtifactHash   string
	ResolvedManifestHash string
}

// PlanInput identifies one execution plan fingerprint and its frozen input
// policy. An empty InputManifestHash selects the legacy policy that predates
// frozen input manifests.
type PlanInput struct {
	SearchRevisionID     int64
	ExecutionFingerprint string
	ResolvedManifestHash string
	InputManifestHash    string
	EnrichmentEnabled    bool
}
