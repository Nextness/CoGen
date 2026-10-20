// types.go defines the run family application types. Generated sqlc row types
// stay inside the private internal/sql package and never cross this family
// boundary.
package run

import "fmt"

// Visibility state values for the reversible run lifecycle.
const (
	VisibilityActive  = "active"
	VisibilityTrashed = "trashed"
)

// Run represents one pipeline_runs row: the lifecycle, plan linkage, and
// visibility state of a single pipeline attempt.
type Run struct {
	ID              int64   `json:"id"`
	Step            string  `json:"step"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      *string `json:"finished_at,omitempty"`
	Status          string  `json:"status"`
	Summary         *string `json:"summary,omitempty"`
	SearchQuery     *string `json:"search_query,omitempty"`
	ExecutionPlanID *int64  `json:"execution_plan_id,omitempty"`
	AttemptNumber   *int    `json:"attempt_number,omitempty"`
	VisibilityState string  `json:"visibility_state"`
	TrashedAt       *string `json:"trashed_at,omitempty"`
	TrashReason     *string `json:"trash_reason,omitempty"`
}

// AttemptAlreadyRunningError reports the active attempt that prevents another
// attempt for the same execution plan from starting.
type AttemptAlreadyRunningError struct {
	ExecutionPlanID int64
	PipelineRunID   int64
}

// Error returns the receiver's diagnostic message.
func (e *AttemptAlreadyRunningError) Error() string {
	return fmt.Sprintf("execution plan %d already has running attempt %d", e.ExecutionPlanID, e.PipelineRunID)
}

// Reviewer is the immutable reviewer identity captured for one run.
type Reviewer struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	CreatedAt     string `json:"created_at"`
}

// Step records a stage's execution within a pipeline run, including its
// input/output artifacts and optional reuse from a prior run.
type Step struct {
	ID                int64  `json:"id"`
	PipelineRunID     int64  `json:"pipeline_run_id"`
	StepName          string `json:"step_name"`
	StepStatus        string `json:"step_status"`
	InputArtifactID   *int64 `json:"input_artifact_id,omitempty"`
	OutputArtifactID  *int64 `json:"output_artifact_id,omitempty"`
	ReusedFromRunID   *int64 `json:"reused_from_run_id,omitempty"`
	InputFingerprint  string `json:"input_fingerprint,omitempty"`
	OutputFingerprint string `json:"output_fingerprint,omitempty"`
	StartedAt         string `json:"started_at,omitempty"`
	FinishedAt        string `json:"finished_at,omitempty"`
}

// Metric is a single counter snapshot for a pipeline run.
type Metric struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	Metric        string `json:"metric"`
	Source        string `json:"source"`
	Value         int    `json:"value"`
}

// PurgeEligibility describes whether a pipeline run can be safely purged.
type PurgeEligibility struct {
	// Eligible is true when no other run references this run's artifacts or
	// reusable outputs.
	Eligible bool `json:"eligible"`
	// SharedArtifactCount is the number of artifacts from this run that are
	// referenced by other runs.
	SharedArtifactCount int `json:"shared_artifact_count"`
	// ReusedByCount is the number of other runs that reuse one or more stages
	// from this run.
	ReusedByCount int `json:"reused_by_count"`
	// OwnedReviewContextCount is the number of review contexts initialized for this run.
	OwnedReviewContextCount int `json:"owned_review_context_count"`
	// DependentReviewContextCount is the number of child contexts inheriting directly or indirectly from this run's context.
	DependentReviewContextCount int `json:"dependent_review_context_count"`
}

// StartAttemptInput identifies one planned attempt to start.
type StartAttemptInput struct {
	ExecutionPlanID int64
	Step            string
	SearchQuery     string
}

// ReviewerInput identifies one immutable reviewer identity to record.
type ReviewerInput struct {
	RunID    int64
	Username string
	Email    string
}

// MetricInput identifies one run metric value to upsert.
type MetricInput struct {
	RunID  int64
	Metric string
	Source string
	Value  int
}

// StepFingerprintInput identifies one run step's immutable reuse fingerprints.
type StepFingerprintInput struct {
	StepID            int64
	InputFingerprint  string
	OutputFingerprint string
}

// StepArtifactInput links one artifact to one run step in a declared role.
type StepArtifactInput struct {
	StepID     int64
	ArtifactID int64
}

// StepReuseInput records that one run step reused output from a prior run.
type StepReuseInput struct {
	StepID          int64
	ReusedFromRunID int64
}

// VisibilityInput requests one reversible visibility transition for a run.
type VisibilityInput struct {
	RunID           int64
	VisibilityState string
	Reason          string
}

// VisibilityResult reports the committed visibility state and whether it changed.
type VisibilityResult struct {
	VisibilityState string
	Changed         bool
}

// Visibility is the lifecycle state read for one run.
type Visibility struct {
	Status          string
	VisibilityState string
}

// RunAncestry is one run with its complete search, revision, and plan lineage
// for the viewer Home and hierarchy sections. Lineage fields stay nil when the
// run has no execution plan.
type RunAncestry struct {
	ID               int64
	AttemptNumber    *int
	StartedAt        string
	FinishedAt       *string
	Status           string
	VisibilityState  string
	SearchID         *int64
	SearchName       string
	SearchRevisionID *int64
	RevisionLabel    string
	ExecutionPlanID  *int64
}

// AttemptSummary is the selected non-trashed attempt of one execution plan.
type AttemptSummary struct {
	ID              int64
	AttemptNumber   *int
	StartedAt       string
	Status          string
	VisibilityState string
}

// RunArtifactContext is the canonical ancestry shown above the run artifact
// browser.
type RunArtifactContext struct {
	SearchID             string
	SearchRevisionID     int64
	SearchRevisionLabel  string
	ExecutionPlanID      int64
	ExecutionFingerprint string
	RunID                int64
	AttemptNumber        *int
}

// RunContextSearch is the search identity in one canonical run context.
type RunContextSearch struct {
	ID        int64
	SearchID  string
	CreatedAt string
}

// RunContextRevision is the selected search revision in one canonical run context.
type RunContextRevision struct {
	ID                   int64
	SearchID             int64
	Label                string
	ConfigArtifactHash   string
	ResolvedManifestHash string
	CreatedAt            string
}

// RunContextPlan is the frozen execution plan in one canonical run context.
type RunContextPlan struct {
	ID                   int64
	SearchRevisionID     int64
	ExecutionFingerprint string
	ResolvedManifestHash string
	InputManifestHash    string
	EnrichmentEnabled    bool
	CreatedAt            string
}

// RunContextRun is the lifecycle projection of the selected run.
type RunContextRun struct {
	ID              int64
	ExecutionPlanID int64
	Step            string
	StartedAt       string
	FinishedAt      *string
	Status          string
	Summary         *string
	AttemptNumber   int64
	VisibilityState string
	TrashedAt       *string
	TrashReason     *string
}

// RunContext is the complete ancestry and lifecycle projection for one run.
type RunContext struct {
	Search          RunContextSearch
	Revision        RunContextRevision
	Plan            RunContextPlan
	Run             RunContextRun
	ReviewContextID *int64
}

// ViewerStep is one run step with its derived duration for the stage timeline.
type ViewerStep struct {
	StepName          string
	StepStatus        string
	InputArtifactID   *int64
	OutputArtifactID  *int64
	StartedAt         *string
	FinishedAt        *string
	InputFingerprint  string
	OutputFingerprint string
	DurationSeconds   *float64
}
