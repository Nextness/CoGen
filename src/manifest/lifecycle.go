// Package manifest lifecycle types define the statuses, actions, and policies
// governing pipeline attempts, stages, cache operations, run visibility, and
// audit events. These are the vocabulary of the workspace provenance model.
//
// Execution plans are immutable reusable definitions; they do not carry a
// lifecycle status. The "no matching plan" case is a lookup/creation-code
// path, not a persisted status. Attempt statuses (running, completed, failed)
// describe the latest attempt's state for a given plan.
package manifest

import "fmt"

// AttemptStatus represents the lifecycle state of a pipeline run attempt.
type AttemptStatus string

const (
	// AttemptRunning means the attempt is currently executing.
	AttemptRunning AttemptStatus = "running"
	// AttemptCompleted means the attempt finished successfully.
	AttemptCompleted AttemptStatus = "completed"
	// AttemptFailed means the attempt finished with an error.
	AttemptFailed AttemptStatus = "failed"
)

// ValidAttemptStatuses returns all valid attempt status values.
func ValidAttemptStatuses() []AttemptStatus {
	return []AttemptStatus{AttemptRunning, AttemptCompleted, AttemptFailed}
}

// ValidateAttemptStatus returns an error if s is not a valid attempt status.
func ValidateAttemptStatus(s string) error {
	switch AttemptStatus(s) {
	case AttemptRunning, AttemptCompleted, AttemptFailed:
		return nil
	default:
		return fmt.Errorf("manifest: invalid attempt status %q", s)
	}
}

// StageOutcome represents the result of a single pipeline stage.
type StageOutcome string

const (
	// StagePending means the stage has not started yet.
	StagePending StageOutcome = "pending"
	// StageRunning means the stage is currently executing.
	StageRunning StageOutcome = "running"
	// StageCompleted means the stage finished successfully.
	StageCompleted StageOutcome = "completed"
	// StageSkipped means the stage was intentionally skipped (e.g. enrichment
	// disabled by config).
	StageSkipped StageOutcome = "skipped"
	// StageReused means the stage output was reused from a prior run.
	StageReused StageOutcome = "reused"
	// StageFailed means the stage failed.
	StageFailed StageOutcome = "failed"
)

// ValidStageOutcomes returns all valid stage outcome values.
func ValidStageOutcomes() []StageOutcome {
	return []StageOutcome{StagePending, StageRunning, StageCompleted, StageSkipped, StageReused, StageFailed}
}

// ValidateStageOutcome returns an error if s is not a valid stage outcome.
func ValidateStageOutcome(s string) error {
	switch StageOutcome(s) {
	case StagePending, StageRunning, StageCompleted, StageSkipped, StageReused, StageFailed:
		return nil
	default:
		return fmt.Errorf("manifest: invalid stage outcome %q", s)
	}
}

// CacheOutcome represents the result of a cache lookup operation.
// It describes what the cache layer returned, not where the data was
// ultimately resolved. The resolution source (network, prior-run snapshot,
// etc.) is tracked separately as a fetch/resolution event.
type CacheOutcome string

const (
	// CacheHit means the cache entry was found and is valid.
	CacheHit CacheOutcome = "hit"
	// CacheMiss means the cache entry was not found.
	CacheMiss CacheOutcome = "miss"
	// CacheNegative means a negative (known-missing) entry was found.
	CacheNegative CacheOutcome = "negative"
	// CacheStale means the cache entry exists but has expired.
	CacheStale CacheOutcome = "stale"
)

// ValidCacheOutcomes returns all valid cache outcome values.
func ValidCacheOutcomes() []CacheOutcome {
	return []CacheOutcome{CacheHit, CacheMiss, CacheNegative, CacheStale}
}

// ValidateCacheOutcome returns an error if s is not a valid cache outcome.
func ValidateCacheOutcome(s string) error {
	switch CacheOutcome(s) {
	case CacheHit, CacheMiss, CacheNegative, CacheStale:
		return nil
	default:
		return fmt.Errorf("manifest: invalid cache outcome %q", s)
	}
}

// RunVisibility represents the visibility state of a pipeline run.
type RunVisibility string

const (
	// RunVisible means the run is visible to normal queries.
	RunVisible RunVisibility = "active"
	// RunArchived means the run is hidden from normal queries but still available
	// when explicitly requested.
	RunArchived RunVisibility = "archived"
	// RunTrashed means the run is marked for deletion and hidden from normal
	// queries. Trashed runs can be restored.
	RunTrashed RunVisibility = "trashed"
)

// ValidRunVisibilities returns all valid run visibility values.
func ValidRunVisibilities() []RunVisibility {
	return []RunVisibility{RunVisible, RunArchived, RunTrashed}
}

// ValidateRunVisibility returns an error if s is not a valid run visibility.
func ValidateRunVisibility(s string) error {
	switch RunVisibility(s) {
	case RunVisible, RunArchived, RunTrashed:
		return nil
	default:
		return fmt.Errorf("manifest: invalid run visibility %q", s)
	}
}

// AuditAction identifies one kind of append-only audit event.
type AuditAction string

const (
	// AuditPlanCreated records the creation of a new execution plan.
	AuditPlanCreated AuditAction = "plan_created"
	// AuditDuplicatePlanSkipped records that a duplicate plan was not re-created.
	AuditDuplicatePlanSkipped AuditAction = "duplicate_plan_skipped"
	// AuditRunStarted records the start of a pipeline run attempt.
	AuditRunStarted AuditAction = "run_started"
	// AuditStepReused records that a stage output was reused from a prior run.
	AuditStepReused AuditAction = "step_reused"
	// AuditCacheHit records a cache hit.
	AuditCacheHit AuditAction = "cache_hit"
	// AuditNetworkFetch records a live API fetch.
	AuditNetworkFetch AuditAction = "network_fetch"
	// AuditFieldEnriched records that a field was enriched by a provider.
	AuditFieldEnriched AuditAction = "field_enriched"
	// AuditValidationChanged records that a validation status changed.
	AuditValidationChanged AuditAction = "validation_changed"
	// AuditRunCompleted records successful completion of a run attempt.
	AuditRunCompleted AuditAction = "run_completed"
	// AuditRunFailed records that a run attempt failed.
	AuditRunFailed AuditAction = "run_failed"
	// AuditRunTrashed records that a run was trashed (soft-deleted).
	AuditRunTrashed AuditAction = "run_trashed"
	// AuditRunRestored records that a trashed run was restored.
	AuditRunRestored AuditAction = "run_restored"
	// AuditRunPurged records that a run was permanently purged.
	AuditRunPurged AuditAction = "run_purged"
	// AuditRevisionConfigChanged records that a search revision's config
	// or manifest hashes were updated because the workspace config changed.
	AuditRevisionConfigChanged AuditAction = "revision_config_changed"
	// AuditPDFDocumentAdded remains valid for audit events emitted by the
	// original manual PDF store before inventory registration was introduced.
	AuditPDFDocumentAdded AuditAction = "pdf_document_added"
	// AuditPDFInventoryRegistered records creation of a normalized DOI's
	// not-available inventory row.
	AuditPDFInventoryRegistered AuditAction = "pdf_inventory_registered"
	// AuditPDFDocumentInventoried records a validated manual PDF insertion.
	AuditPDFDocumentInventoried AuditAction = "pdf_document_inventoried"
	// AuditReviewContextCreated records explicit initialization of a run review context.
	AuditReviewContextCreated AuditAction = "review_context_created"
	// AuditWorkReviewVersionCreated records a new immutable article-review state.
	AuditWorkReviewVersionCreated AuditAction = "work_review_version_created"
	// AuditReviewNoteCreated records a new logical review note and its first version.
	AuditReviewNoteCreated AuditAction = "review_note_created"
	// AuditReviewNoteVersionCreated records an immutable active note edit.
	AuditReviewNoteVersionCreated AuditAction = "review_note_version_created"
	// AuditReviewNoteTombstoned records an immutable note deletion marker.
	AuditReviewNoteTombstoned AuditAction = "review_note_tombstoned"
	// AuditReviewAnchorCreated records a new logical PDF anchor and its first version.
	AuditReviewAnchorCreated AuditAction = "review_anchor_created"
	// AuditReviewAnchorVersionCreated records an immutable replacement PDF anchor.
	AuditReviewAnchorVersionCreated AuditAction = "review_anchor_version_created"
	// AuditReviewAnchorTombstoned records an immutable PDF-anchor deletion marker.
	AuditReviewAnchorTombstoned AuditAction = "review_anchor_tombstoned"
)

// ValidAuditActions returns all valid audit action values.
func ValidAuditActions() []AuditAction {
	return []AuditAction{
		AuditPlanCreated, AuditDuplicatePlanSkipped, AuditRunStarted,
		AuditStepReused, AuditCacheHit, AuditNetworkFetch, AuditFieldEnriched,
		AuditValidationChanged, AuditRunCompleted, AuditRunFailed, AuditRunTrashed,
		AuditRunRestored, AuditRunPurged, AuditRevisionConfigChanged,
		AuditPDFDocumentAdded, AuditPDFInventoryRegistered,
		AuditPDFDocumentInventoried, AuditReviewContextCreated,
		AuditWorkReviewVersionCreated, AuditReviewNoteCreated,
		AuditReviewNoteVersionCreated, AuditReviewNoteTombstoned,
		AuditReviewAnchorCreated, AuditReviewAnchorVersionCreated,
		AuditReviewAnchorTombstoned,
	}
}

// ValidateAuditAction returns an error if s is not a valid audit action.
func ValidateAuditAction(s string) error {
	switch AuditAction(s) {
	case AuditPlanCreated, AuditDuplicatePlanSkipped, AuditRunStarted,
		AuditStepReused, AuditCacheHit, AuditNetworkFetch, AuditFieldEnriched,
		AuditValidationChanged, AuditRunCompleted, AuditRunFailed, AuditRunTrashed,
		AuditRunRestored, AuditRunPurged, AuditRevisionConfigChanged:
		return nil
	case AuditPDFDocumentAdded, AuditPDFInventoryRegistered, AuditPDFDocumentInventoried:
		return nil
	case AuditReviewContextCreated, AuditWorkReviewVersionCreated,
		AuditReviewNoteCreated, AuditReviewNoteVersionCreated, AuditReviewNoteTombstoned,
		AuditReviewAnchorCreated, AuditReviewAnchorVersionCreated, AuditReviewAnchorTombstoned:
		return nil
	default:
		return fmt.Errorf("manifest: invalid audit action %q", s)
	}
}

// AuditEvent is a single append-only audit record. Every event records when it
// occurred, which actor produced it, which pipeline run it belongs to, the
// affected entity, the action performed, optional before/after state, and a
// correlation id for deduplication.
type AuditEvent struct {
	// OccurredAt is the ISO 8601 UTC timestamp of the event.
	OccurredAt string `json:"occurred_at"`

	// Actor identifies the system component or user that produced the event
	// (e.g. "pipeline", "viewer", "admin").
	Actor string `json:"actor"`

	// PipelineRunID links this event to a specific pipeline run attempt.
	// Zero means the event is not associated with a specific run.
	PipelineRunID int64 `json:"pipeline_run_id,omitempty"`

	// EntityType identifies the kind of entity affected (e.g. "article",
	// "author", "plan", "run", "source").
	EntityType string `json:"entity_type"`

	// EntityID is the identifier of the affected entity.
	EntityID string `json:"entity_id"`

	// Action is the audit action name.
	Action AuditAction `json:"action"`

	// BeforeJSON is the JSON representation of the entity state before the
	// action, if applicable.
	BeforeJSON string `json:"before_json,omitempty"`

	// AfterJSON is the JSON representation of the entity state after the
	// action, if applicable.
	AfterJSON string `json:"after_json,omitempty"`

	// MetadataJSON is additional event-specific context as JSON.
	MetadataJSON string `json:"metadata_json,omitempty"`

	// CorrelationID is an optional idempotency key for deduplication.
	CorrelationID string `json:"correlation_id,omitempty"`
}

// RetentionPolicy describes how long trashed run data is retained before it
// becomes eligible for purge. Audit events are never deleted; RetentionPolicy
// does not govern audit records.
type RetentionPolicy struct {
	// TrashRetentionDays is the minimum number of days a trashed run is kept
	// before it becomes eligible for purge. Zero means indefinite retention.
	TrashRetentionDays int `json:"trash_retention_days"`
}

// PurgePolicy describes the authorization and safety checks required before
// data is permanently removed.
type PurgePolicy struct {
	// RequireVerification, when true, requires that purging a run first
	// verifies no shared artifacts or cache entries are referenced by other
	// runs.
	RequireVerification bool `json:"require_verification"`

	// KeepTombstone, when true, retains a lightweight purge event or tombstone
	// record instead of removing all evidence of the purged run.
	KeepTombstone bool `json:"keep_tombstone"`
}

// DefaultRetentionPolicy returns the default retention policy.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		TrashRetentionDays: 30, // 30 days before purge eligible
	}
}

// DefaultPurgePolicy returns the default purge policy.
func DefaultPurgePolicy() PurgePolicy {
	return PurgePolicy{
		RequireVerification: true,
		KeepTombstone:       true,
	}
}
