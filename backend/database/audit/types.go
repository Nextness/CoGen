// types.go defines the audit family application types. Generated sqlc row
// types stay inside the private internal/sql package and never cross this
// family boundary.
package audit

// Event is one persisted audit event.
type Event struct {
	ID            int64  `json:"id"`
	OccurredAt    string `json:"occurred_at"`
	Actor         string `json:"actor"`
	PipelineRunID *int64 `json:"pipeline_run_id,omitempty"`
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	Action        string `json:"action"`
	BeforeJSON    string `json:"before_json,omitempty"`
	AfterJSON     string `json:"after_json,omitempty"`
	MetadataJSON  string `json:"metadata_json,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// RecordedData is one event's recorded JSON payloads.
type RecordedData struct {
	ID           int64
	BeforeJSON   string
	AfterJSON    string
	MetadataJSON string
}

// EnrichmentSummary is the bounded provider and field label set for one work revision.
type EnrichmentSummary struct {
	Providers []string
	Fields    []string
	Truncated bool
}

// Summary is the filtered audit event count and per-action counts.
type Summary struct {
	TotalEvents int64
	Actions     []ActionCount
}

// ActionCount is one action's filtered event count.
type ActionCount struct {
	Action string
	Count  int64
}

// Facets are the distinct actor, action, and entity type values in a run scope.
type Facets struct {
	Actors      []string
	Actions     []string
	EntityTypes []string
}

// Filter selects audit events for the viewer timeline. Empty fields are
// ignored; list fields and filter strings are capped by the family builder.
type Filter struct {
	EntityID        string
	EntityTypes     []string
	Actions         []string
	Actors          []string
	Categories      []string
	Stage           string
	Outcome         string
	ReviewStatus    string
	ReviewReason    string
	ReviewSubstatus string
	Query           string
	RunID           int64
	PDFScope        string
	PDFSelected     bool
	Cursor          int64
	Limit           int
}
