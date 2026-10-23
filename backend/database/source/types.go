// types.go defines the source family application types. Generated sqlc row
// types stay inside the private internal/sql package and never cross this
// family boundary.
package source

// Source links a pipeline run to one of its declared sources.
type Source struct {
	ID                    int64  `json:"id"`
	PipelineRunID         int64  `json:"pipeline_run_id"`
	SourceName            string `json:"source_name"`
	SourceType            string `json:"source_type"`
	ExpectedFile          string `json:"expected_file"`
	Query                 string `json:"query,omitempty"`
	RequestedFields       string `json:"requested_fields,omitempty"`
	ExpectedResultCount   *int   `json:"expected_result_count,omitempty"`
	ObservedResultCount   *int   `json:"observed_result_count,omitempty"`
	ResultCountComparison string `json:"result_count_comparison,omitempty"`
	ExportDate            string `json:"export_date,omitempty"`
	CreatedAt             string `json:"created_at"`
}

// Record represents a single raw record loaded from a source.
type Record struct {
	ID           int64  `json:"id"`
	RunSourceID  int64  `json:"run_source_id"`
	RecordIndex  int    `json:"record_index"`
	RawPayload   string `json:"raw_payload"`
	ContentHash  string `json:"content_hash"`
	ParseStatus  string `json:"parse_status"`
	RejectReason string `json:"reject_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// FilterCount holds per-source filter stage article counts for a run.
type FilterCount struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	FilterData    string `json:"filter_data"` // JSON array of {filters: string[], count: int}
}

// CreateSourceInput identifies one declared run source and its expected export metadata.
type CreateSourceInput struct {
	RunID           int64
	Name            string
	Type            string
	ExpectedFile    string
	Query           string
	RequestedFields string
	ExpectedCount   int
	ExportDate      string
}

// ObservedResultCountInput records the observed export count and its informational comparison.
type ObservedResultCountInput struct {
	RunSourceID         int64
	ObservedResultCount int
	Comparison          string
}

// CreateRecordInput identifies one raw source record and its content identity.
type CreateRecordInput struct {
	RunSourceID int64
	RecordIndex int
	RawPayload  string
	ContentHash string
}

// RecordStatusInput identifies one source record's parse outcome.
type RecordStatusInput struct {
	RecordID     int64
	Status       string
	RejectReason string
}

// FilterCountInput identifies one run source filter count snapshot.
type FilterCountInput struct {
	RunID      int64
	SourceName string
	FilterData string
}
