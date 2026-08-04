// Integration tests for execution plans, run sources, source records, and run steps.
//go:build integration

package database

import "testing"

// TestExecutionPlanCreateAndGetByFingerprint verifies execution plan create and get by fingerprint.
func TestExecutionPlanCreateAndGetByFingerprint(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("plan-test")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg-hash", "res-hash")

	planID, err := db.Plans.Create(revID, "fp-abc123", "manifest-xyz")
	if err != nil {
		t.Fatalf("Create plan: %v", err)
	}
	if planID == 0 {
		t.Fatal("expected non-zero plan id")
	}

	got, err := db.Plans.GetByID(planID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("plan not found by id")
	}
	if got.SearchRevisionID != revID {
		t.Errorf("expected search_revision_id %d, got %d", revID, got.SearchRevisionID)
	}
	if got.ExecutionFingerprint != "fp-abc123" {
		t.Errorf("expected fingerprint 'fp-abc123', got %q", got.ExecutionFingerprint)
	}
	if got.ResolvedManifestHash != "manifest-xyz" {
		t.Errorf("expected manifest hash 'manifest-xyz', got %q", got.ResolvedManifestHash)
	}

	got2, err := db.Plans.GetByFingerprint(revID, "fp-abc123")
	if err != nil {
		t.Fatalf("GetByFingerprint: %v", err)
	}
	if got2 == nil {
		t.Fatal("plan not found by fingerprint")
	}
	if got2.ID != planID {
		t.Errorf("expected id %d, got %d", planID, got2.ID)
	}
}

// TestExecutionPlanWithInputManifestAndEnrichmentPolicy verifies execution plan with input manifest and enrichment policy.
func TestExecutionPlanWithInputManifestAndEnrichmentPolicy(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("plan-policy-test")
	revisionID, _, _ := db.Revisions.Create(searchID, "v1", "config", "manifest")
	planID, err := db.Plans.CreateWithInputManifest(revisionID, "fp-policy", "manifest", "input-manifest-hash", true)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := db.Plans.GetByID(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.InputManifestHash != "input-manifest-hash" || !plan.EnrichmentEnabled {
		t.Fatalf("plan provenance = %+v, want input hash and enabled policy", plan)
	}
	if _, err := db.Plans.CreateWithInputManifest(revisionID, "fp-policy", "manifest", "input-manifest-hash", false); err == nil {
		t.Fatal("expected a conflicting enrichment policy to be rejected")
	}
}

// TestExecutionPlanDuplicateSameHashReusesID: same fingerprint + same manifest hash reuses the existing plan.
func TestExecutionPlanDuplicateSameHashReusesID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("dup-plan-test")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")

	planID1, err := db.Plans.Create(revID, "same-fingerprint", "same-manifest")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	planID2, err := db.Plans.Create(revID, "same-fingerprint", "same-manifest")
	if err != nil {
		t.Fatalf("second Create with same manifest hash: %v", err)
	}

	if planID2 != planID1 {
		t.Errorf("duplicate plan should return existing id %d, got %d", planID1, planID2)
	}
}

// TestExecutionPlanDuplicateDifferentHashRejected: same fingerprint but different manifest hash returns an error.
func TestExecutionPlanDuplicateDifferentHashRejected(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("dup-plan-hash-test")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")

	_, err := db.Plans.Create(revID, "same-fingerprint", "manifest-a")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = db.Plans.Create(revID, "same-fingerprint", "manifest-b")
	if err == nil {
		t.Fatal("expected error for duplicate fingerprint with different manifest hash")
	}
}

// TestExecutionPlanDistinctRevisions: distinct search revisions may have distinct
// plans for the same source files (same fingerprint, different revision).
func TestExecutionPlanDistinctRevisions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("distinct-rev-plan")

	revID1, _, _ := db.Revisions.Create(searchID, "v1", "cfg-a", "res-a")
	revID2, _, _ := db.Revisions.Create(searchID, "v2", "cfg-b", "res-b")

	// Same fingerprint for both revisions
	planID1, err := db.Plans.Create(revID1, "same-fingerprint", "manifest-a")
	if err != nil {
		t.Fatalf("Create plan for revision v1: %v", err)
	}
	planID2, err := db.Plans.Create(revID2, "same-fingerprint", "manifest-b")
	if err != nil {
		t.Fatalf("Create plan for revision v2: %v", err)
	}

	if planID1 == planID2 {
		t.Error("distinct revisions should produce distinct plans even with the same fingerprint")
	}

	// Verify each plan is scoped to its revision
	got1, _ := db.Plans.GetByID(planID1)
	got2, _ := db.Plans.GetByID(planID2)
	if got1.SearchRevisionID != revID1 {
		t.Errorf("plan 1 should belong to revision %d, got %d", revID1, got1.SearchRevisionID)
	}
	if got2.SearchRevisionID != revID2 {
		t.Errorf("plan 2 should belong to revision %d, got %d", revID2, got2.SearchRevisionID)
	}
}

// TestExecutionPlanListBySearchRevision verifies execution plan list by search revision.
func TestExecutionPlanListBySearchRevision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	searchID, _ := db.Searches.Create("list-plans")
	revID, _, _ := db.Revisions.Create(searchID, "v1", "cfg", "res")

	db.Plans.Create(revID, "fp-a", "mh-a")
	db.Plans.Create(revID, "fp-b", "mh-b")

	plans, err := db.Plans.ListBySearchRevision(revID)
	if err != nil {
		t.Fatalf("ListBySearchRevision: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
}

// TestRunSourceCreateAndList verifies creating run sources and listing them.
func TestRunSourceCreateAndList(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("source_test", "search query")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	srcID, err := db.RunSources.Create(runID, "scopus", "csv", "corpus/scopus.csv", "TITLE-ABS-KEY(test)", "doi,title,authors", 4, "")
	if err != nil {
		t.Fatalf("Create run source: %v", err)
	}
	if srcID == 0 {
		t.Fatal("expected non-zero source id")
	}

	srcID2, err := db.RunSources.Create(runID, "ieee", "csv", "corpus/ieee.csv", "", "", 2, "")
	if err != nil {
		t.Fatalf("Create run source 2: %v", err)
	}
	if srcID2 == srcID {
		t.Error("expected different source IDs for distinct sources")
	}
	if err := db.RunSources.SetObservedResultCount(srcID, 3, "below"); err != nil {
		t.Fatalf("SetObservedResultCount: %v", err)
	}

	sources, err := db.RunSources.ListByRun(runID)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].SourceName != "scopus" {
		t.Errorf("expected source 'scopus', got %q", sources[0].SourceName)
	}
	if sources[0].ExpectedResultCount == nil || *sources[0].ExpectedResultCount != 4 {
		t.Errorf("expected source result count = %v, want 4", sources[0].ExpectedResultCount)
	}
	if sources[0].ObservedResultCount == nil || *sources[0].ObservedResultCount != 3 || sources[0].ResultCountComparison != "below" {
		t.Errorf("observed source result count = %+v, want 3/below", sources[0])
	}
	if sources[1].SourceName != "ieee" {
		t.Errorf("expected source 'ieee', got %q", sources[1].SourceName)
	}
}

// TestSourceRecordParseAndReject verifies source record creation and parse status updates.
func TestSourceRecordParseAndReject(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("record_test", "")
	srcID, _ := db.RunSources.Create(runID, "scopus", "csv", "corpus/scopus.csv", "", "", 0, "")

	// Create a record that will be accepted
	recID1, err := db.SourceRecords.Create(srcID, 0, `{"title":"Good Article"}`, "hash-abc")
	if err != nil {
		t.Fatalf("Create record 1: %v", err)
	}
	if recID1 == 0 {
		t.Fatal("expected non-zero record id")
	}

	// Create a record that will be rejected
	recID2, err := db.SourceRecords.Create(srcID, 1, `{"title":""}`, "hash-def")
	if err != nil {
		t.Fatalf("Create record 2: %v", err)
	}

	// Update parse status for both
	if err := db.SourceRecords.UpdateParseStatus(recID1, "parsed", ""); err != nil {
		t.Fatalf("UpdateParseStatus accepted: %v", err)
	}
	if err := db.SourceRecords.UpdateParseStatus(recID2, "rejected", "missing required fields"); err != nil {
		t.Fatalf("UpdateParseStatus rejected: %v", err)
	}

	// Verify
	records, err := db.SourceRecords.ListBySource(srcID)
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Check accepted record
	if records[0].ParseStatus != "parsed" {
		t.Errorf("expected record 0 status 'parsed', got %q", records[0].ParseStatus)
	}
	if records[0].RejectReason != "" {
		t.Errorf("expected record 0 empty reject_reason, got %q", records[0].RejectReason)
	}

	// Check rejected record
	if records[1].ParseStatus != "rejected" {
		t.Errorf("expected record 1 status 'rejected', got %q", records[1].ParseStatus)
	}
	if records[1].RejectReason != "missing required fields" {
		t.Errorf("expected reject_reason 'missing required fields', got %q", records[1].RejectReason)
	}
}

// TestSourceRecordCount verifies the CountBySource method.
func TestSourceRecordCount(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("count_test", "")
	srcID, _ := db.RunSources.Create(runID, "scopus", "csv", "corpus/scopus.csv", "", "", 0, "")

	db.SourceRecords.Create(srcID, 0, "record-0", "hash-0")
	db.SourceRecords.Create(srcID, 1, "record-1", "hash-1")
	db.SourceRecords.Create(srcID, 2, "record-2", "hash-2")

	count, err := db.SourceRecords.CountBySource(srcID)
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 records, got %d", count)
	}
}

// TestRunStepCreateAndUpdate verifies run step lifecycle.
func TestRunStepCreateAndUpdate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("step_test", "")

	stepID, err := db.RunSteps.Create(runID, "parse")
	if err != nil {
		t.Fatalf("Create step: %v", err)
	}
	if stepID == 0 {
		t.Fatal("expected non-zero step id")
	}

	// Update status
	if err := db.RunSteps.UpdateStatus(stepID, "completed"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// Create another step
	_, _ = db.RunSteps.Create(runID, "enrich")

	steps, err := db.RunSteps.ListByRun(runID)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	if steps[0].StepName != "parse" {
		t.Errorf("expected step 0 name 'parse', got %q", steps[0].StepName)
	}
	if steps[0].StepStatus != "completed" {
		t.Errorf("expected step 0 status 'completed', got %q", steps[0].StepStatus)
	}
	if steps[0].StartedAt == "" {
		t.Error("expected non-empty started_at")
	}
	if steps[0].FinishedAt == "" {
		t.Error("expected non-empty finished_at after update")
	}

	if steps[1].StepName != "enrich" {
		t.Errorf("expected step 1 name 'enrich', got %q", steps[1].StepName)
	}
}

// TestRunStepReuseLink verifies the reuse linking mechanism.
func TestRunStepReuseLink(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create two runs
	runID1, _ := db.PipelineRuns.StartRun("first_run", "")
	runID2, _ := db.PipelineRuns.StartRun("second_run", "")

	// Create a step in the second run that reuses from the first run
	stepID, _ := db.RunSteps.Create(runID2, "parse")
	if err := db.RunSteps.LinkReuse(stepID, runID1); err != nil {
		t.Fatalf("LinkReuse: %v", err)
	}

	steps, err := db.RunSteps.ListByRun(runID2)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].StepStatus != "reused" {
		t.Errorf("expected step status 'reused', got %q", steps[0].StepStatus)
	}
	if steps[0].ReusedFromRunID == nil || *steps[0].ReusedFromRunID != runID1 {
		t.Errorf("expected reused_from_run_id %d, got %v", runID1, steps[0].ReusedFromRunID)
	}
}

// TestRunStepArtifactLinks verifies artifact linking to steps.
func TestRunStepArtifactLinks(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("artifact_links", "")
	stepID, _ := db.RunSteps.Create(runID, "parse")

	// Create artifacts
	artID1, _ := db.Artifacts.Create("hash-input", "text/csv", 500)
	artID2, _ := db.Artifacts.Create("hash-output", "application/json", 2000)

	// Link artifacts to step
	if err := db.RunSteps.LinkInputArtifact(stepID, artID1); err != nil {
		t.Fatalf("LinkInputArtifact: %v", err)
	}
	if err := db.RunSteps.LinkOutputArtifact(stepID, artID2); err != nil {
		t.Fatalf("LinkOutputArtifact: %v", err)
	}

	steps, err := db.RunSteps.ListByRun(runID)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].InputArtifactID == nil || *steps[0].InputArtifactID != artID1 {
		t.Errorf("expected input_artifact_id %d, got %v", artID1, steps[0].InputArtifactID)
	}
	if steps[0].OutputArtifactID == nil || *steps[0].OutputArtifactID != artID2 {
		t.Errorf("expected output_artifact_id %d, got %v", artID2, steps[0].OutputArtifactID)
	}
}

// TestRunStepFingerprints verifies run step fingerprints.
func TestRunStepFingerprints(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("fingerprint_test", "")
	if err != nil {
		t.Fatal(err)
	}
	stepID, err := db.RunSteps.Create(runID, "preflight")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunSteps.SetFingerprints(stepID, "input-sha256", "output-sha256"); err != nil {
		t.Fatalf("SetFingerprints: %v", err)
	}

	steps, err := db.RunSteps.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(steps))
	}
	if steps[0].InputFingerprint != "input-sha256" || steps[0].OutputFingerprint != "output-sha256" {
		t.Fatalf("fingerprints = (%q, %q), want persisted values", steps[0].InputFingerprint, steps[0].OutputFingerprint)
	}
}

// TestRunStepUpdateStatusInvalidStatus verifies that UpdateStatus rejects invalid
// stage outcomes.
func TestRunStepUpdateStatusInvalidStatus(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("step_invalid_status", "")
	stepID, _ := db.RunSteps.Create(runID, "parse")

	// Invalid status should be rejected
	err := db.RunSteps.UpdateStatus(stepID, "invalid_status")
	if err == nil {
		t.Fatal("expected error for invalid stage outcome")
	}

	// Valid terminal status should succeed and set finished_at
	err = db.RunSteps.UpdateStatus(stepID, "completed")
	if err != nil {
		t.Fatalf("UpdateStatus with valid status: %v", err)
	}

	steps, _ := db.RunSteps.ListByRun(runID)
	if len(steps) == 0 {
		t.Fatal("expected steps")
	}
	if steps[0].FinishedAt == "" {
		t.Error("expected non-empty finished_at for terminal status")
	}
}

// TestRunStepUpdateStatusNonTerminalDoesNotSetFinishedAt verifies that
// setting a non-terminal status (pending, running) does not set finished_at.
func TestRunStepUpdateStatusNonTerminalDoesNotSetFinishedAt(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, _ := db.PipelineRuns.StartRun("step_non_terminal", "")
	stepID, _ := db.RunSteps.Create(runID, "parse")

	// Update to "running" — should NOT set finished_at
	err := db.RunSteps.UpdateStatus(stepID, "running")
	if err != nil {
		t.Fatalf("UpdateStatus to running: %v", err)
	}

	steps, _ := db.RunSteps.ListByRun(runID)
	if len(steps) == 0 {
		t.Fatal("expected steps")
	}
	if steps[0].FinishedAt != "" {
		t.Error("expected empty finished_at for non-terminal status 'running'")
	}
}

// TestSourceFilterCountSetAndGet verifies SetFilterData then GetByRunAndSource returns matching data.
func TestSourceFilterCountSetAndGet(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("sfc_set_get", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	filterData := `[{"filters":["open access"],"count":42}]`
	if err := db.SourceFilterCounts.SetFilterData(runID, "crossref", filterData); err != nil {
		t.Fatalf("SetFilterData: %v", err)
	}

	sfc, err := db.SourceFilterCounts.GetByRunAndSource(runID, "crossref")
	if err != nil {
		t.Fatalf("GetByRunAndSource: %v", err)
	}
	if sfc == nil {
		t.Fatal("expected non-nil SourceFilterCount")
	}
	if sfc.PipelineRunID != runID {
		t.Errorf("expected PipelineRunID %d, got %d", runID, sfc.PipelineRunID)
	}
	if sfc.SourceName != "crossref" {
		t.Errorf("expected SourceName 'crossref', got %q", sfc.SourceName)
	}
	if sfc.FilterData != filterData {
		t.Errorf("expected FilterData %q, got %q", filterData, sfc.FilterData)
	}
}

// TestSourceFilterCountSetAndList verifies SetFilterData for two sources then
// ListByRun returns both ordered by source name.
func TestSourceFilterCountSetAndList(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("sfc_set_list", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	if err := db.SourceFilterCounts.SetFilterData(runID, "openalex", `[{"filters":[],"count":10}]`); err != nil {
		t.Fatalf("SetFilterData openalex: %v", err)
	}
	if err := db.SourceFilterCounts.SetFilterData(runID, "crossref", `[{"filters":[],"count":5}]`); err != nil {
		t.Fatalf("SetFilterData crossref: %v", err)
	}

	results, err := db.SourceFilterCounts.ListByRun(runID)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Ordered by source_name: crossref < openalex
	if results[0].SourceName != "crossref" {
		t.Errorf("expected result[0] SourceName 'crossref', got %q", results[0].SourceName)
	}
	if results[1].SourceName != "openalex" {
		t.Errorf("expected result[1] SourceName 'openalex', got %q", results[1].SourceName)
	}
}

// TestSourceFilterCountUpsert verifies that calling SetFilterData twice for the same
// run+source replaces the previous filter data.
func TestSourceFilterCountUpsert(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("sfc_upsert", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	if err := db.SourceFilterCounts.SetFilterData(runID, "scopus", `[{"filters":["original"],"count":1}]`); err != nil {
		t.Fatalf("first SetFilterData: %v", err)
	}

	if err := db.SourceFilterCounts.SetFilterData(runID, "scopus", `[{"filters":["replaced"],"count":99}]`); err != nil {
		t.Fatalf("second SetFilterData: %v", err)
	}

	sfc, err := db.SourceFilterCounts.GetByRunAndSource(runID, "scopus")
	if err != nil {
		t.Fatalf("GetByRunAndSource: %v", err)
	}
	if sfc == nil {
		t.Fatal("expected non-nil SourceFilterCount")
	}
	if sfc.FilterData != `[{"filters":["replaced"],"count":99}]` {
		t.Errorf("expected updated filter data, got %q", sfc.FilterData)
	}
}

// TestSourceFilterCountGetNotFound verifies GetByRunAndSource returns nil, nil
// for a non-existent run.
func TestSourceFilterCountGetNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	sfc, err := db.SourceFilterCounts.GetByRunAndSource(999, "nonexistent")
	if err != nil {
		t.Fatalf("GetByRunAndSource: %v", err)
	}
	if sfc != nil {
		t.Fatal("expected nil SourceFilterCount for non-existent run")
	}
}

// TestSourceFilterCountListEmpty verifies ListByRun returns an empty slice for
// a run with no filter counts.
func TestSourceFilterCountListEmpty(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	runID, err := db.PipelineRuns.StartRun("sfc_list_empty", "")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	results, err := db.SourceFilterCounts.ListByRun(runID)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
