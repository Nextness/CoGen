// run_adapter.go provides temporary forwarding adapters that keep the legacy
// flat pipeline-run, run-step, metric, and reviewer repository API working over
// the run family store. The adapters contain no SQL or second implementation;
// remove them after every caller migrates to Database.Run.
package database

import (
	"context"

	"analysis/database/run"
)

// PipelineRun represents a row in the pipeline_runs table.
type PipelineRun struct {
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

// AttemptAlreadyRunningError is the run family's active-attempt conflict.
type AttemptAlreadyRunningError = run.AttemptAlreadyRunningError

// RunStep records a stage's execution within a pipeline run, including its
// input/output artifacts and optional reuse from a prior run.
type RunStep struct {
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

// PipelineRunMetric is a single counter snapshot for a pipeline run.
type PipelineRunMetric struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	Metric        string `json:"metric"`
	Source        string `json:"source"`
	Value         int    `json:"value"`
}

// PipelineRunReviewer is the immutable reviewer identity captured for one run.
type PipelineRunReviewer struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	CreatedAt     string `json:"created_at"`
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

// PipelineRunRepository forwards the legacy pipeline-run API to the run family store.
type PipelineRunRepository struct{ db *Database }

// RunStepRepository forwards the legacy run-step API to the run family store.
type RunStepRepository struct{ db *Database }

// MetricsRepository forwards the legacy run-metric API to the run family store.
type MetricsRepository struct{ db *Database }

// PipelineRunReviewerRepository forwards the legacy reviewer API to the run family store.
type PipelineRunReviewerRepository struct{ db *Database }

// StartRun records the start of a pipeline step. Returns the run ID.
// This is the legacy entry point; new code should use StartAttempt.
func (r *PipelineRunRepository) StartRun(step, searchQuery string) (int64, error) {
	return r.db.Run.StartRun(context.Background(), step, searchQuery)
}

// StartAttempt records the start of a pipeline run attempt linked to an execution plan.
// Returns the run ID and attempt number.
func (r *PipelineRunRepository) StartAttempt(executionPlanID int64, step, searchQuery string) (int64, int, error) {
	return r.db.Run.StartAttempt(context.Background(), run.StartAttemptInput{
		ExecutionPlanID: executionPlanID,
		Step:            step,
		SearchQuery:     searchQuery,
	})
}

// StartAttemptIfIdle starts a new attempt only when this execution plan has no
// running attempt.
func (r *PipelineRunRepository) StartAttemptIfIdle(executionPlanID int64, step, searchQuery string) (int64, int, error) {
	return r.db.Run.StartAttemptIfIdle(context.Background(), run.StartAttemptInput{
		ExecutionPlanID: executionPlanID,
		Step:            step,
		SearchQuery:     searchQuery,
	})
}

// FinishRun marks a pipeline run as completed (or failed).
// Supports both legacy runs and new attempt-based runs.
func (r *PipelineRunRepository) FinishRun(runID int64, status string, summary string) error {
	return r.db.Run.FinishRun(context.Background(), runID, status, summary)
}

// Trash marks a pipeline run as trashed (soft-deleted).
func (r *PipelineRunRepository) Trash(runID int64, reason string) error {
	return r.db.Run.Trash(context.Background(), runID, reason)
}

// Restore sets a trashed pipeline run back to active visibility.
func (r *PipelineRunRepository) Restore(runID int64) error {
	return r.db.Run.Restore(context.Background(), runID)
}

// GetByID returns a pipeline run by its primary key, or nil if not found.
func (r *PipelineRunRepository) GetByID(runID int64) (*PipelineRun, error) {
	found, err := r.db.Run.GetByID(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	return pipelineRunFromFamily(found), nil
}

// ListByPlan returns all runs for a given execution plan, ordered by attempt number.
func (r *PipelineRunRepository) ListByPlan(executionPlanID int64) ([]*PipelineRun, error) {
	found, err := r.db.Run.ListByPlan(context.Background(), executionPlanID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*PipelineRun, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, pipelineRunFromFamily(item))
	}
	return legacy, nil
}

// ListByVisibility returns all runs with a given visibility state, ordered by ID.
func (r *PipelineRunRepository) ListByVisibility(visibilityState string) ([]*PipelineRun, error) {
	found, err := r.db.Run.ListByVisibility(context.Background(), visibilityState)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*PipelineRun, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, pipelineRunFromFamily(item))
	}
	return legacy, nil
}

// CheckPurgeEligibility verifies that no other run shares artifacts or reusable
// stage outputs from the given run. It is the safety check before purge.
// Returns an error if no pipeline run with the given ID exists.
func (r *PipelineRunRepository) CheckPurgeEligibility(runID int64) (*PurgeEligibility, error) {
	found, err := r.db.Run.CheckPurgeEligibility(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	return &PurgeEligibility{
		Eligible:                    found.Eligible,
		SharedArtifactCount:         found.SharedArtifactCount,
		ReusedByCount:               found.ReusedByCount,
		OwnedReviewContextCount:     found.OwnedReviewContextCount,
		DependentReviewContextCount: found.DependentReviewContextCount,
	}, nil
}

// RecoverAbandoned atomically fails a running attempt and records recovery audit.
func (r *PipelineRunRepository) RecoverAbandoned(ctx context.Context, runID int64) error {
	return r.db.Run.RecoverAbandoned(ctx, runID)
}

// Create inserts a new run step record. Returns the step ID.
func (r *RunStepRepository) Create(pipelineRunID int64, stepName string) (int64, error) {
	return r.db.Run.CreateStep(context.Background(), pipelineRunID, stepName)
}

// UpdateStatus updates the status and optional finish time of a run step.
func (r *RunStepRepository) UpdateStatus(stepID int64, status string) error {
	return r.db.Run.UpdateStepStatus(context.Background(), stepID, status)
}

// LinkReuse records that a step reused output from a prior run.
func (r *RunStepRepository) LinkReuse(stepID int64, reusedFromRunID int64) error {
	return r.db.Run.LinkStepReuse(context.Background(), run.StepReuseInput{
		StepID:          stepID,
		ReusedFromRunID: reusedFromRunID,
	})
}

// LinkInputArtifact records that a step consumed a specific artifact as input.
func (r *RunStepRepository) LinkInputArtifact(stepID, artifactID int64) error {
	return r.db.Run.LinkStepInputArtifact(context.Background(), run.StepArtifactInput{
		StepID:     stepID,
		ArtifactID: artifactID,
	})
}

// LinkOutputArtifact records that a step produced a specific artifact as output.
func (r *RunStepRepository) LinkOutputArtifact(stepID, artifactID int64) error {
	return r.db.Run.LinkStepOutputArtifact(context.Background(), run.StepArtifactInput{
		StepID:     stepID,
		ArtifactID: artifactID,
	})
}

// SetFingerprints records the immutable stage input and output fingerprints.
func (r *RunStepRepository) SetFingerprints(stepID int64, inputFingerprint, outputFingerprint string) error {
	return r.db.Run.SetStepFingerprints(context.Background(), run.StepFingerprintInput{
		StepID:            stepID,
		InputFingerprint:  inputFingerprint,
		OutputFingerprint: outputFingerprint,
	})
}

// ListByRun returns all steps for a given pipeline run, ordered by ID.
func (r *RunStepRepository) ListByRun(pipelineRunID int64) ([]*RunStep, error) {
	found, err := r.db.Run.ListStepsByRun(context.Background(), pipelineRunID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*RunStep, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, runStepFromFamily(item))
	}
	return legacy, nil
}

// Set inserts or replaces a metric value for a given run, metric name, and source.
func (r *MetricsRepository) Set(runID int64, metric, source string, value int) error {
	return r.db.Run.SetMetric(context.Background(), run.MetricInput{
		RunID:  runID,
		Metric: metric,
		Source: source,
		Value:  value,
	})
}

// Get returns a single metric for a given run, metric name, and source.
// Returns nil, nil if the metric is not recorded for this run.
func (r *MetricsRepository) Get(runID int64, metric, source string) (*PipelineRunMetric, error) {
	found, err := r.db.Run.GetMetric(context.Background(), runID, metric, source)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	return &PipelineRunMetric{
		PipelineRunID: found.PipelineRunID,
		Metric:        found.Metric,
		Source:        found.Source,
		Value:         found.Value,
	}, nil
}

// ListByRun returns all metrics for a given pipeline run, ordered by metric name then source.
func (r *MetricsRepository) ListByRun(runID int64) ([]*PipelineRunMetric, error) {
	found, err := r.db.Run.ListMetricsByRun(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	legacy := make([]*PipelineRunMetric, 0, len(found))
	for _, item := range found {
		legacy = append(legacy, &PipelineRunMetric{
			PipelineRunID: item.PipelineRunID,
			Metric:        item.Metric,
			Source:        item.Source,
			Value:         item.Value,
		})
	}
	return legacy, nil
}

// Insert records one immutable reviewer identity for a newly created run.
func (r *PipelineRunReviewerRepository) Insert(runID int64, username, email string) error {
	return r.db.Run.InsertReviewer(context.Background(), run.ReviewerInput{
		RunID:    runID,
		Username: username,
		Email:    email,
	})
}

// Get returns the reviewer captured for a run, or nil when a legacy writer omitted it.
func (r *PipelineRunReviewerRepository) Get(runID int64) (*PipelineRunReviewer, error) {
	found, err := r.db.Run.GetReviewer(context.Background(), runID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	return &PipelineRunReviewer{
		PipelineRunID: found.PipelineRunID,
		Username:      found.Username,
		Email:         found.Email,
		CreatedAt:     found.CreatedAt,
	}, nil
}

// pipelineRunFromFamily maps a run family run into the legacy application type.
func pipelineRunFromFamily(found *run.Run) *PipelineRun {
	if found == nil {
		return nil
	}
	return &PipelineRun{
		ID:              found.ID,
		Step:            found.Step,
		StartedAt:       found.StartedAt,
		FinishedAt:      found.FinishedAt,
		Status:          found.Status,
		Summary:         found.Summary,
		SearchQuery:     found.SearchQuery,
		ExecutionPlanID: found.ExecutionPlanID,
		AttemptNumber:   found.AttemptNumber,
		VisibilityState: found.VisibilityState,
		TrashedAt:       found.TrashedAt,
		TrashReason:     found.TrashReason,
	}
}

// runStepFromFamily maps a run family step into the legacy application type.
func runStepFromFamily(found *run.Step) *RunStep {
	if found == nil {
		return nil
	}
	return &RunStep{
		ID:                found.ID,
		PipelineRunID:     found.PipelineRunID,
		StepName:          found.StepName,
		StepStatus:        found.StepStatus,
		InputArtifactID:   found.InputArtifactID,
		OutputArtifactID:  found.OutputArtifactID,
		ReusedFromRunID:   found.ReusedFromRunID,
		InputFingerprint:  found.InputFingerprint,
		OutputFingerprint: found.OutputFingerprint,
		StartedAt:         found.StartedAt,
		FinishedAt:        found.FinishedAt,
	}
}
