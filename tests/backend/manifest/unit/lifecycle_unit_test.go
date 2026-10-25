// lifecycle_unit_test.go tests the manifest lifecycle types — attempt status
// validation, stage outcomes, cache outcomes, run visibility, and
// audit action constants.

package manifest_test

import subject "analysis/backend/manifest"

import "testing"

// TestValidateAttemptStatusInvalid verifies validate attempt status invalid.
func TestValidateAttemptStatusInvalid(t *testing.T) {
	if err := subject.ValidateAttemptStatus("unknown"); err == nil {
		t.Error("expected error for invalid attempt status")
	}
}

// TestValidateStageOutcomeInvalid verifies validate stage outcome invalid.
func TestValidateStageOutcomeInvalid(t *testing.T) {
	if err := subject.ValidateStageOutcome("bogus"); err == nil {
		t.Error("expected error for invalid stage outcome")
	}
}

// TestValidateStageOutcomeEmpty verifies validate stage outcome empty.
func TestValidateStageOutcomeEmpty(t *testing.T) {
	if err := subject.ValidateStageOutcome(""); err == nil {
		t.Error("expected error for empty stage outcome")
	}
}

// TestValidateCacheOutcomeInvalid verifies validate cache outcome invalid.
func TestValidateCacheOutcomeInvalid(t *testing.T) {
	if err := subject.ValidateCacheOutcome("invalid"); err == nil {
		t.Error("expected error for invalid cache outcome")
	}
}

// TestValidateCacheOutcomeEmpty verifies validate cache outcome empty.
func TestValidateCacheOutcomeEmpty(t *testing.T) {
	if err := subject.ValidateCacheOutcome(""); err == nil {
		t.Error("expected error for empty cache outcome")
	}
}

// TestCacheOutcomeConstants verifies cache outcome constants.
func TestCacheOutcomeConstants(t *testing.T) {
	if subject.CacheHit != "hit" {
		t.Errorf("CacheHit = %q, want %q", subject.CacheHit, "hit")
	}
	if subject.CacheMiss != "miss" {
		t.Errorf("CacheMiss = %q, want %q", subject.CacheMiss, "miss")
	}
	if subject.CacheNegative != "negative" {
		t.Errorf("CacheNegative = %q, want %q", subject.CacheNegative, "negative")
	}
	if subject.CacheStale != "stale" {
		t.Errorf("CacheStale = %q, want %q", subject.CacheStale, "stale")
	}
}

// TestValidateRunVisibilityInvalid verifies validate run visibility invalid.
func TestValidateRunVisibilityInvalid(t *testing.T) {
	if err := subject.ValidateRunVisibility("invisible"); err == nil {
		t.Error("expected error for invalid run visibility")
	}
}

// TestValidateRunVisibilityEmpty verifies validate run visibility empty.
func TestValidateRunVisibilityEmpty(t *testing.T) {
	if err := subject.ValidateRunVisibility(""); err == nil {
		t.Error("expected error for empty run visibility")
	}
}

// TestRunVisibilityConstants verifies run visibility constants.
func TestRunVisibilityConstants(t *testing.T) {
	if subject.RunVisible != "active" {
		t.Errorf("RunVisible = %q, want %q", subject.RunVisible, "active")
	}
	if subject.RunArchived != "archived" {
		t.Errorf("RunArchived = %q, want %q", subject.RunArchived, "archived")
	}
	if subject.RunTrashed != "trashed" {
		t.Errorf("RunTrashed = %q, want %q", subject.RunTrashed, "trashed")
	}
}

// TestValidateAuditActionInvalid verifies validate audit action invalid.
func TestValidateAuditActionInvalid(t *testing.T) {
	if err := subject.ValidateAuditAction("unknown_action"); err == nil {
		t.Error("expected error for invalid audit action")
	}
}

// TestValidateAuditActionEmpty verifies validate audit action empty.
func TestValidateAuditActionEmpty(t *testing.T) {
	if err := subject.ValidateAuditAction(""); err == nil {
		t.Error("expected error for empty audit action")
	}
}

// TestAuditActionConstants verifies audit action constants.
func TestAuditActionConstants(t *testing.T) {
	tests := []struct {
		got  subject.AuditAction
		want string
	}{
		{subject.AuditPlanCreated, "plan_created"},
		{subject.AuditDuplicatePlanSkipped, "duplicate_plan_skipped"},
		{subject.AuditRunStarted, "run_started"},
		{subject.AuditStepReused, "step_reused"},
		{subject.AuditCacheHit, "cache_hit"},
		{subject.AuditNetworkFetch, "network_fetch"},
		{subject.AuditFieldEnriched, "field_enriched"},
		{subject.AuditValidationChanged, "validation_changed"},
		{subject.AuditRunCompleted, "run_completed"},
		{subject.AuditRunFailed, "run_failed"},
		{subject.AuditRunTrashed, "run_trashed"},
		{subject.AuditRunRestored, "run_restored"},
		{subject.AuditRunPurged, "run_purged"},
		{subject.AuditPDFDocumentAdded, "pdf_document_added"},
		{subject.AuditPDFInventoryRegistered, "pdf_inventory_registered"},
		{subject.AuditPDFDocumentInventoried, "pdf_document_inventoried"},
		{subject.AuditPDFDocumentReplaced, "pdf_document_replaced"},
		{subject.AuditPDFDocumentRemoved, "pdf_document_removed"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("audit action constant = %q, want %q", tt.got, tt.want)
		}
	}
}

// TestAuditEventFields verifies audit event fields.
func TestAuditEventFields(t *testing.T) {
	e := subject.AuditEvent{
		OccurredAt:    "2026-07-21T12:00:00Z",
		Actor:         "pipeline",
		PipelineRunID: 1,
		EntityType:    "article",
		EntityID:      "10.1234/example",
		Action:        subject.AuditFieldEnriched,
		BeforeJSON:    `{"title": "old"}`,
		AfterJSON:     `{"title": "new"}`,
		MetadataJSON:  `{"provider": "crossref"}`,
		CorrelationID: "uuid-abc-123",
	}

	if e.OccurredAt != "2026-07-21T12:00:00Z" {
		t.Errorf("OccurredAt = %q, want %q", e.OccurredAt, "2026-07-21T12:00:00Z")
	}
	if e.Actor != "pipeline" {
		t.Errorf("Actor = %q, want %q", e.Actor, "pipeline")
	}
	if e.PipelineRunID != 1 {
		t.Errorf("PipelineRunID = %d, want 1", e.PipelineRunID)
	}
	if e.EntityType != "article" {
		t.Errorf("EntityType = %q, want %q", e.EntityType, "article")
	}
	if e.Action != subject.AuditFieldEnriched {
		t.Errorf("Action = %q, want %q", e.Action, subject.AuditFieldEnriched)
	}
	if e.CorrelationID != "uuid-abc-123" {
		t.Errorf("CorrelationID = %q, want %q", e.CorrelationID, "uuid-abc-123")
	}
}

// TestDefaultRetentionPolicy verifies default retention policy.
func TestDefaultRetentionPolicy(t *testing.T) {
	p := subject.DefaultRetentionPolicy()
	if p.TrashRetentionDays != 30 {
		t.Errorf("TrashRetentionDays = %d, want 30", p.TrashRetentionDays)
	}
}

// TestDefaultPurgePolicy verifies default purge policy.
func TestDefaultPurgePolicy(t *testing.T) {
	p := subject.DefaultPurgePolicy()
	if !p.RequireVerification {
		t.Error("RequireVerification should be true by default")
	}
	if !p.KeepTombstone {
		t.Error("KeepTombstone should be true by default")
	}
}

// TestRetentionPolicyCustom verifies retention policy custom.
func TestRetentionPolicyCustom(t *testing.T) {
	p := subject.RetentionPolicy{
		TrashRetentionDays: 60,
	}
	if p.TrashRetentionDays != 60 {
		t.Errorf("TrashRetentionDays = %d, want 60", p.TrashRetentionDays)
	}
}

// TestPurgePolicyCustom verifies purge policy custom.
func TestPurgePolicyCustom(t *testing.T) {
	p := subject.PurgePolicy{
		RequireVerification: false,
		KeepTombstone:       false,
	}
	if p.RequireVerification {
		t.Error("RequireVerification should be false")
	}
	if p.KeepTombstone {
		t.Error("KeepTombstone should be false")
	}
}
