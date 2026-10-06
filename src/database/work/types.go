// types.go defines the work family application types and vocabulary. Generated
// sqlc row types stay inside the private internal/sql package and never cross
// this family boundary.
package work

// Work is a stable corpus work identity. A work with a globally unique DOI is
// shared across searches; title-only records each get their own row because
// title identity is uncertain.
type Work struct {
	ID        int64  `json:"id"`
	DOI       string `json:"doi"`
	CreatedAt string `json:"created_at"`
}

// Identifier is an alternative identifier for a work, scoped by namespace.
// Examples: ("scopus", "2-s2.0-84912345678"), ("openalex", "W1234567890").
type Identifier struct {
	ID         int64  `json:"id"`
	WorkID     int64  `json:"work_id"`
	Namespace  string `json:"namespace"`
	Identifier string `json:"identifier"`
	CreatedAt  string `json:"created_at"`
}

// Revision is an immutable snapshot of a work's typed core metadata and
// extension data at the point it was produced by a pipeline run stage.
// ProducerStage records which pipeline stage created the revision.
type Revision struct {
	ID                 int64  `json:"id"`
	WorkID             int64  `json:"work_id"`
	PipelineRunID      int64  `json:"pipeline_run_id"`
	ProducerStage      string `json:"producer_stage"`
	FieldSchemaVersion string `json:"field_schema_version"`
	PayloadHash        string `json:"payload_hash"`
	Title              string `json:"title"`
	Abstract           string `json:"abstract"`
	Year               int    `json:"year"`
	Journal            string `json:"journal"`
	Publisher          string `json:"publisher"`
	Source             string `json:"source"`
	Keywords           string `json:"keywords"`      // JSON array
	KeywordsPlus       string `json:"keywords_plus"` // JSON array
	CitationCount      int    `json:"citation_count"`
	ReferenceCount     int    `json:"reference_count"`
	ExtensionData      string `json:"extension_data"` // JSON object
	CreatedAt          string `json:"created_at"`
}

// StageOutcome records what happened to one work at one pipeline stage within
// a single pipeline run. The (pipeline_run_id, work_id, stage_name) triplet is
// unique so the same stage cannot report two different outcomes for the same
// work in the same run. CreatedAt is the first time the outcome was set;
// UpdatedAt is the most recent time it changed.
type StageOutcome struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	WorkID        int64  `json:"work_id"`
	StageName     string `json:"stage_name"`
	Outcome       string `json:"outcome"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Reference is one cited reference observed on an immutable work revision.
// ResolvedWorkID is optional because most citations are external to the
// current workspace.
type Reference struct {
	ID             int64  `json:"id"`
	WorkRevisionID int64  `json:"work_revision_id"`
	ResolvedWorkID int64  `json:"resolved_work_id"`
	MentionOrder   int    `json:"mention_order"`
	RawReference   string `json:"raw_reference"`
	DOI            string `json:"doi"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	Year           int    `json:"year"`
	Source         string `json:"source"`
	CreatedAt      string `json:"created_at"`
}

// RunTerm is one stored search term and the source that declared it.
type RunTerm struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	Term          string `json:"term"`
	CreatedAt     string `json:"created_at"`
}

// Valid producer stages (which pipeline stage created a work revision).
// New revisions must use one of these; LegacyUnknown is a reserved sentinel
// that is rejected by CreateRevision (no longer produced by any pipeline
// stage).
const (
	ProducerStageParse          = "parse"
	ProducerStageDeduplicate    = "deduplicate"
	ProducerStageValidate       = "validate"
	ProducerStageEnrich         = "enrich"
	ProducerStageEnrichMetadata = "enrich_metadata"
	ProducerStageEnrichIdentity = "enrich_identity"
	ProducerStageNormalize      = "normalize"
	// ProducerStageLegacyUnknown is a reserved sentinel value. It is not
	// produced by any current pipeline stage and is rejected by CreateRevision.
	ProducerStageLegacyUnknown = "legacy_unknown"
)

// Valid pipeline stage names (used as run_work_stages.stage_name).
const (
	StageNameParse          = "parse"
	StageNameDeduplicate    = "deduplicate"
	StageNameValidate       = "validate"
	StageNameEnrich         = "enrich"
	StageNameEnrichMetadata = "enrich_metadata"
	StageNameEnrichIdentity = "enrich_identity"
	StageNameNormalize      = "normalize"
)

// Valid stage outcomes (used as run_work_stages.outcome).
const (
	OutcomeParsed       = "parsed"
	OutcomeDuplicate    = "duplicate"
	OutcomeDeduplicated = "deduplicated"
	OutcomeValid        = "valid"
	OutcomeDiscarded    = "discarded"
	OutcomeEnriched     = "enriched"
	OutcomeFailed       = "failed"
	OutcomeNormalized   = "normalized"
	OutcomeSkipped      = "skipped"
	OutcomePending      = "pending"
)

// InsertIdentifierInput identifies one alternate identifier to attach to a work.
type InsertIdentifierInput struct {
	WorkID     int64
	Namespace  string
	Identifier string
}

// StageOutcomeInput identifies one stage outcome to insert or update.
type StageOutcomeInput struct {
	RunID     int64
	WorkID    int64
	StageName string
	Outcome   string
	Reason    string
}

// ReplaceTermDataInput replaces one run's term inventory and revision matches.
type ReplaceTermDataInput struct {
	RunID         int64
	TermsBySource map[string][]string
	Matches       map[int64]map[string][]string
}
