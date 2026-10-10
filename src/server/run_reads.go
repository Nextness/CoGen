// run_reads.go converts run family application types into the viewer's stable
// JSON row shapes. Nullable fields are made explicit here so family types never
// leak into responses.
package server

import (
	"analysis/database/run"
)

// optionalInt renders an optional integer as the invariant JSON null-or-number shape.
func optionalInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

// optionalInt64 renders an optional identifier as the invariant JSON null-or-number shape.
func optionalInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// optionalText renders an optional string as the invariant JSON null-or-string shape.
func optionalText(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// runAncestryRow maps one run family ancestry projection into a viewer row.
func runAncestryRow(ancestry *run.RunAncestry) map[string]any {
	return map[string]any{
		"id":                 ancestry.ID,
		"attempt_number":     optionalInt(ancestry.AttemptNumber),
		"started_at":         ancestry.StartedAt,
		"finished_at":        optionalText(ancestry.FinishedAt),
		"status":             ancestry.Status,
		"visibility_state":   ancestry.VisibilityState,
		"search_id":          optionalInt64(ancestry.SearchID),
		"search_name":        ancestry.SearchName,
		"search_revision_id": optionalInt64(ancestry.SearchRevisionID),
		"revision_label":     ancestry.RevisionLabel,
		"execution_plan_id":  optionalInt64(ancestry.ExecutionPlanID),
	}
}

// selectedAttemptRow maps one run family attempt summary into a viewer row.
func selectedAttemptRow(attempt *run.AttemptSummary) map[string]any {
	return map[string]any{
		"id":               attempt.ID,
		"attempt_number":   optionalInt(attempt.AttemptNumber),
		"started_at":       attempt.StartedAt,
		"status":           attempt.Status,
		"visibility_state": attempt.VisibilityState,
	}
}

// runArtifactContextRow maps one run family artifact context into a viewer row.
func runArtifactContextRow(context *run.RunArtifactContext) map[string]any {
	return map[string]any{
		"search_id":             context.SearchID,
		"search_revision_id":    context.SearchRevisionID,
		"search_revision_label": context.SearchRevisionLabel,
		"execution_plan_id":     context.ExecutionPlanID,
		"execution_fingerprint": context.ExecutionFingerprint,
		"run_id":                context.RunID,
		"attempt_number":        optionalInt(context.AttemptNumber),
	}
}

// runContextPayload maps one run family context into the canonical viewer payload.
func runContextPayload(context *run.RunContext, runWritable bool) map[string]any {
	var contextID any
	if context.ReviewContextID != nil {
		contextID = *context.ReviewContextID
	}
	return map[string]any{
		"search": map[string]any{
			"id":         context.Search.ID,
			"search_id":  context.Search.SearchID,
			"created_at": context.Search.CreatedAt,
		},
		"revision": map[string]any{
			"id":                     context.Revision.ID,
			"search_id":              context.Revision.SearchID,
			"label":                  context.Revision.Label,
			"config_artifact_hash":   context.Revision.ConfigArtifactHash,
			"resolved_manifest_hash": context.Revision.ResolvedManifestHash,
			"created_at":             context.Revision.CreatedAt,
		},
		"plan": map[string]any{
			"id":                     context.Plan.ID,
			"search_revision_id":     context.Plan.SearchRevisionID,
			"execution_fingerprint":  context.Plan.ExecutionFingerprint,
			"resolved_manifest_hash": context.Plan.ResolvedManifestHash,
			"input_manifest_hash":    context.Plan.InputManifestHash,
			"enrichment_enabled":     context.Plan.EnrichmentEnabled,
			"created_at":             context.Plan.CreatedAt,
		},
		"run": map[string]any{
			"id":                context.Run.ID,
			"execution_plan_id": context.Run.ExecutionPlanID,
			"step":              context.Run.Step,
			"started_at":        context.Run.StartedAt,
			"finished_at":       optionalText(context.Run.FinishedAt),
			"status":            context.Run.Status,
			"summary":           optionalText(context.Run.Summary),
			"attempt_number":    context.Run.AttemptNumber,
			"visibility_state":  context.Run.VisibilityState,
			"trashed_at":        optionalText(context.Run.TrashedAt),
			"trash_reason":      optionalText(context.Run.TrashReason),
		},
		"lifecycle": map[string]any{
			"status":           context.Run.Status,
			"visibility_state": context.Run.VisibilityState,
			"review_writable":  runWritable,
		},
		"review": map[string]any{
			"initialized":  context.ReviewContextID != nil,
			"context_id":   contextID,
			"run_writable": runWritable,
		},
	}
}

// viewerStepRow maps one run family step into a viewer row.
func viewerStepRow(step *run.ViewerStep) map[string]any {
	return map[string]any{
		"step_name":          step.StepName,
		"step_status":        step.StepStatus,
		"input_artifact_id":  optionalInt64(step.InputArtifactID),
		"output_artifact_id": optionalInt64(step.OutputArtifactID),
		"started_at":         optionalText(step.StartedAt),
		"finished_at":        optionalText(step.FinishedAt),
		"input_fingerprint":  step.InputFingerprint,
		"output_fingerprint": step.OutputFingerprint,
		"duration_seconds":   step.DurationSeconds,
	}
}
