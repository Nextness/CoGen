// lifecycle_unit_test.go tests the manifest lifecycle types — attempt status
// validation, stage outcomes, cache outcomes, run visibility, and
// audit action constants.
//go:build unit

package manifest

import "testing"

// TestValidAttemptStatuses verifies valid attempt statuses.
func TestValidAttemptStatuses(t *testing.T) {
	statuses := ValidAttemptStatuses()
	expected := []AttemptStatus{AttemptRunning, AttemptCompleted, AttemptFailed}
	if len(statuses) != len(expected) {
		t.Fatalf("ValidAttemptStatuses length = %d, want %d", len(statuses), len(expected))
	}
	for i, s := range statuses {
		if s != expected[i] {
			t.Errorf("ValidAttemptStatuses[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

// TestValidateAttemptStatusValid verifies validate attempt status valid.
func TestValidateAttemptStatusValid(t *testing.T) {
	for _, s := range ValidAttemptStatuses() {
		if err := ValidateAttemptStatus(string(s)); err != nil {
			t.Errorf("ValidateAttemptStatus(%q) returned error: %v", s, err)
		}
	}
}

// TestValidateAttemptStatusInvalid verifies validate attempt status invalid.
func TestValidateAttemptStatusInvalid(t *testing.T) {
	if err := ValidateAttemptStatus("unknown"); err == nil {
		t.Error("expected error for invalid attempt status")
	}
}

// TestValidateAttemptStatusEmpty verifies validate attempt status empty.
func TestValidateAttemptStatusEmpty(t *testing.T) {
	if err := ValidateAttemptStatus(""); err == nil {
		t.Error("expected error for empty attempt status")
	}
}

// TestValidStageOutcomes verifies valid stage outcomes.
func TestValidStageOutcomes(t *testing.T) {
	statuses := ValidStageOutcomes()
	expected := []StageOutcome{StagePending, StageRunning, StageCompleted, StageSkipped, StageReused, StageFailed}
	if len(statuses) != len(expected) {
		t.Fatalf("ValidStageOutcomes length = %d, want %d", len(statuses), len(expected))
	}
	for i, s := range statuses {
		if s != expected[i] {
			t.Errorf("ValidStageOutcomes[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

// TestValidateStageOutcomeValid verifies validate stage outcome valid.
func TestValidateStageOutcomeValid(t *testing.T) {
	for _, s := range ValidStageOutcomes() {
		if err := ValidateStageOutcome(string(s)); err != nil {
			t.Errorf("ValidateStageOutcome(%q) returned error: %v", s, err)
		}
	}
}

// TestValidateStageOutcomeInvalid verifies validate stage outcome invalid.
func TestValidateStageOutcomeInvalid(t *testing.T) {
	if err := ValidateStageOutcome("bogus"); err == nil {
		t.Error("expected error for invalid stage outcome")
	}
}

// TestValidateStageOutcomeEmpty verifies validate stage outcome empty.
func TestValidateStageOutcomeEmpty(t *testing.T) {
	if err := ValidateStageOutcome(""); err == nil {
		t.Error("expected error for empty stage outcome")
	}
}

// TestValidCacheOutcomes verifies valid cache outcomes.
func TestValidCacheOutcomes(t *testing.T) {
	statuses := ValidCacheOutcomes()
	expected := []CacheOutcome{CacheHit, CacheMiss, CacheNegative, CacheStale}
	if len(statuses) != len(expected) {
		t.Fatalf("ValidCacheOutcomes length = %d, want %d", len(statuses), len(expected))
	}
	for i, s := range statuses {
		if s != expected[i] {
			t.Errorf("ValidCacheOutcomes[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

// TestValidateCacheOutcomeValid verifies validate cache outcome valid.
func TestValidateCacheOutcomeValid(t *testing.T) {
	for _, s := range ValidCacheOutcomes() {
		if err := ValidateCacheOutcome(string(s)); err != nil {
			t.Errorf("ValidateCacheOutcome(%q) returned error: %v", s, err)
		}
	}
}

// TestValidateCacheOutcomeInvalid verifies validate cache outcome invalid.
func TestValidateCacheOutcomeInvalid(t *testing.T) {
	if err := ValidateCacheOutcome("invalid"); err == nil {
		t.Error("expected error for invalid cache outcome")
	}
}

// TestValidateCacheOutcomeEmpty verifies validate cache outcome empty.
func TestValidateCacheOutcomeEmpty(t *testing.T) {
	if err := ValidateCacheOutcome(""); err == nil {
		t.Error("expected error for empty cache outcome")
	}
}

// TestCacheOutcomeConstants verifies cache outcome constants.
func TestCacheOutcomeConstants(t *testing.T) {
	if CacheHit != "hit" {
		t.Errorf("CacheHit = %q, want %q", CacheHit, "hit")
	}
	if CacheMiss != "miss" {
		t.Errorf("CacheMiss = %q, want %q", CacheMiss, "miss")
	}
	if CacheNegative != "negative" {
		t.Errorf("CacheNegative = %q, want %q", CacheNegative, "negative")
	}
	if CacheStale != "stale" {
		t.Errorf("CacheStale = %q, want %q", CacheStale, "stale")
	}
}

// TestValidRunVisibilities verifies valid run visibilities.
func TestValidRunVisibilities(t *testing.T) {
	visibilities := ValidRunVisibilities()
	expected := []RunVisibility{RunVisible, RunArchived, RunTrashed}
	if len(visibilities) != len(expected) {
		t.Fatalf("ValidRunVisibilities length = %d, want %d", len(visibilities), len(expected))
	}
	for i, v := range visibilities {
		if v != expected[i] {
			t.Errorf("ValidRunVisibilities[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

// TestValidateRunVisibilityValid verifies validate run visibility valid.
func TestValidateRunVisibilityValid(t *testing.T) {
	for _, v := range ValidRunVisibilities() {
		if err := ValidateRunVisibility(string(v)); err != nil {
			t.Errorf("ValidateRunVisibility(%q) returned error: %v", v, err)
		}
	}
}

// TestValidateRunVisibilityInvalid verifies validate run visibility invalid.
func TestValidateRunVisibilityInvalid(t *testing.T) {
	if err := ValidateRunVisibility("invisible"); err == nil {
		t.Error("expected error for invalid run visibility")
	}
}

// TestValidateRunVisibilityEmpty verifies validate run visibility empty.
func TestValidateRunVisibilityEmpty(t *testing.T) {
	if err := ValidateRunVisibility(""); err == nil {
		t.Error("expected error for empty run visibility")
	}
}

// TestRunVisibilityConstants verifies run visibility constants.
func TestRunVisibilityConstants(t *testing.T) {
	if RunVisible != "active" {
		t.Errorf("RunVisible = %q, want %q", RunVisible, "active")
	}
	if RunArchived != "archived" {
		t.Errorf("RunArchived = %q, want %q", RunArchived, "archived")
	}
	if RunTrashed != "trashed" {
		t.Errorf("RunTrashed = %q, want %q", RunTrashed, "trashed")
	}
}

// TestValidAuditActions verifies valid audit actions.
func TestValidAuditActions(t *testing.T) {
	actions := ValidAuditActions()
	expected := []AuditAction{
		AuditPlanCreated, AuditDuplicatePlanSkipped, AuditRunStarted,
		AuditStepReused, AuditCacheHit, AuditNetworkFetch, AuditFieldEnriched,
		AuditValidationChanged, AuditRunCompleted, AuditRunFailed, AuditRunTrashed,
		AuditRunRestored, AuditRunPurged, AuditRevisionConfigChanged,
		AuditPDFDocumentAdded, AuditPDFInventoryRegistered,
		AuditPDFDocumentInventoried,
	}
	if len(actions) != len(expected) {
		t.Fatalf("ValidAuditActions length = %d, want %d", len(actions), len(expected))
	}
	for i, a := range actions {
		if a != expected[i] {
			t.Errorf("ValidAuditActions[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

// TestValidateAuditActionValid verifies validate audit action valid.
func TestValidateAuditActionValid(t *testing.T) {
	for _, a := range ValidAuditActions() {
		if err := ValidateAuditAction(string(a)); err != nil {
			t.Errorf("ValidateAuditAction(%q) returned error: %v", a, err)
		}
	}
}

// TestValidateAuditActionInvalid verifies validate audit action invalid.
func TestValidateAuditActionInvalid(t *testing.T) {
	if err := ValidateAuditAction("unknown_action"); err == nil {
		t.Error("expected error for invalid audit action")
	}
}

// TestValidateAuditActionEmpty verifies validate audit action empty.
func TestValidateAuditActionEmpty(t *testing.T) {
	if err := ValidateAuditAction(""); err == nil {
		t.Error("expected error for empty audit action")
	}
}

// TestAuditActionConstants verifies audit action constants.
func TestAuditActionConstants(t *testing.T) {
	tests := []struct {
		got  AuditAction
		want string
	}{
		{AuditPlanCreated, "plan_created"},
		{AuditDuplicatePlanSkipped, "duplicate_plan_skipped"},
		{AuditRunStarted, "run_started"},
		{AuditStepReused, "step_reused"},
		{AuditCacheHit, "cache_hit"},
		{AuditNetworkFetch, "network_fetch"},
		{AuditFieldEnriched, "field_enriched"},
		{AuditValidationChanged, "validation_changed"},
		{AuditRunCompleted, "run_completed"},
		{AuditRunFailed, "run_failed"},
		{AuditRunTrashed, "run_trashed"},
		{AuditRunRestored, "run_restored"},
		{AuditRunPurged, "run_purged"},
		{AuditPDFDocumentAdded, "pdf_document_added"},
		{AuditPDFInventoryRegistered, "pdf_inventory_registered"},
		{AuditPDFDocumentInventoried, "pdf_document_inventoried"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("audit action constant = %q, want %q", tt.got, tt.want)
		}
	}
}

// TestAuditEventFields verifies audit event fields.
func TestAuditEventFields(t *testing.T) {
	e := AuditEvent{
		OccurredAt:    "2026-07-21T12:00:00Z",
		Actor:         "pipeline",
		PipelineRunID: 1,
		EntityType:    "article",
		EntityID:      "10.1234/example",
		Action:        AuditFieldEnriched,
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
	if e.Action != AuditFieldEnriched {
		t.Errorf("Action = %q, want %q", e.Action, AuditFieldEnriched)
	}
	if e.CorrelationID != "uuid-abc-123" {
		t.Errorf("CorrelationID = %q, want %q", e.CorrelationID, "uuid-abc-123")
	}
}

// TestDefaultRetentionPolicy verifies default retention policy.
func TestDefaultRetentionPolicy(t *testing.T) {
	p := DefaultRetentionPolicy()
	if p.TrashRetentionDays != 30 {
		t.Errorf("TrashRetentionDays = %d, want 30", p.TrashRetentionDays)
	}
}

// TestDefaultPurgePolicy verifies default purge policy.
func TestDefaultPurgePolicy(t *testing.T) {
	p := DefaultPurgePolicy()
	if !p.RequireVerification {
		t.Error("RequireVerification should be true by default")
	}
	if !p.KeepTombstone {
		t.Error("KeepTombstone should be true by default")
	}
}

// TestRetentionPolicyCustom verifies retention policy custom.
func TestRetentionPolicyCustom(t *testing.T) {
	p := RetentionPolicy{
		TrashRetentionDays: 60,
	}
	if p.TrashRetentionDays != 60 {
		t.Errorf("TrashRetentionDays = %d, want 60", p.TrashRetentionDays)
	}
}

// TestPurgePolicyCustom verifies purge policy custom.
func TestPurgePolicyCustom(t *testing.T) {
	p := PurgePolicy{
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
