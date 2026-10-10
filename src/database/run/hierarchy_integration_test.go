// hierarchy_integration_test.go exercises the run family's viewer hierarchy
// reads against a database created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"testing"

	"analysis/database/run"
)

// TestStoreHierarchyAttemptsFilterOrderAndLookahead verifies the attempt page
// excludes trashed attempts, follows descending IDs, honors the searchable
// predicate, and reports one-row lookahead.
func TestStoreHierarchyAttemptsFilterOrderAndLookahead(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "hierarchy-attempts")

	firstRun, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query")
	if err != nil {
		t.Fatal(err)
	}
	secondRun, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query")
	if err != nil {
		t.Fatal(err)
	}
	thirdRun, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(secondRun, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE pipeline_runs SET visibility_state='trashed', trashed_at=datetime('now'), trash_reason='test' WHERE id=?", firstRun); err != nil {
		t.Fatal(err)
	}

	items, hasMore, err := store.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{ExecutionPlanID: planID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(items) != 1 || items[0].ID != thirdRun {
		t.Fatalf("attempt page = %+v hasMore=%v, want only newest attempt %d with lookahead", items, hasMore, thirdRun)
	}
	full, hasMore, err := store.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{ExecutionPlanID: planID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if hasMore || len(full) != 2 {
		t.Fatalf("full attempt page = %d items hasMore=%v, want 2 items", len(full), hasMore)
	}
	if full[0].ID != thirdRun || full[1].ID != secondRun {
		t.Fatalf("attempt order = [%d %d], want [%d %d]", full[0].ID, full[1].ID, thirdRun, secondRun)
	}
	if full[1].AttemptNumber == nil || *full[1].AttemptNumber != 2 {
		t.Fatalf("second attempt number = %v, want 2", full[1].AttemptNumber)
	}
	if full[0].FinishedAt != nil || full[1].FinishedAt == nil {
		t.Fatalf("attempt finished_at = %v/%v, want nil then set", full[0].FinishedAt, full[1].FinishedAt)
	}

	filtered, _, err := store.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{ExecutionPlanID: planID, Query: "completed", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ID != secondRun {
		t.Fatalf("completed filter = %+v, want attempt %d", filtered, secondRun)
	}

	afterCursor, _, err := store.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{ExecutionPlanID: planID, CursorID: thirdRun, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(afterCursor) != 1 || afterCursor[0].ID != secondRun {
		t.Fatalf("cursor page = %+v, want only attempt %d", afterCursor, secondRun)
	}
	if _, _, err := store.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{ExecutionPlanID: planID, Limit: 0}); err == nil {
		t.Fatal("expected a limit validation error")
	}
}

// TestStoreHierarchyRunsVisibilityStatusAndAncestry verifies the run page
// filters visibility, status, and start boundaries while preserving complete
// ancestry and nullable lineage for unplanned runs.
func TestStoreHierarchyRunsVisibilityStatusAndAncestry(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "hierarchy-runs")

	activeRun, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(activeRun, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	trashedRun, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(trashedRun, "failed", "failed"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE pipeline_runs SET visibility_state='trashed', trashed_at=datetime('now') WHERE id=?", trashedRun); err != nil {
		t.Fatal(err)
	}
	unplannedRun, err := db.PipelineRuns.StartRun("legacy", "query")
	if err != nil {
		t.Fatal(err)
	}

	active, hasMore, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "active", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if hasMore || len(active) != 2 {
		t.Fatalf("active runs = %d hasMore=%v, want 2", len(active), hasMore)
	}
	planned := active[1]
	if planned.ID != activeRun || planned.SearchID == nil || planned.SearchRevisionID == nil || planned.ExecutionPlanID == nil {
		t.Fatalf("planned ancestry = %+v, want complete lineage for run %d", planned, activeRun)
	}
	if planned.SearchName != "run-family-hierarchy-runs" || planned.RevisionLabel != "v1" {
		t.Fatalf("planned names = %q/%q", planned.SearchName, planned.RevisionLabel)
	}
	unplanned := active[0]
	if unplanned.ID != unplannedRun || unplanned.SearchID != nil || unplanned.ExecutionPlanID != nil || unplanned.SearchName != "" {
		t.Fatalf("unplanned ancestry = %+v, want empty lineage for run %d", unplanned, unplannedRun)
	}

	trashed, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "trashed", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(trashed) != 1 || trashed[0].ID != trashedRun {
		t.Fatalf("trashed runs = %+v, want only run %d", trashed, trashedRun)
	}

	completed, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "all", Status: "completed", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 1 || completed[0].ID != activeRun {
		t.Fatalf("completed runs = %+v, want only run %d", completed, activeRun)
	}

	future, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "all", StartedAfter: "2999-01-01 00:00:00", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(future) != 0 {
		t.Fatalf("future runs = %+v, want none", future)
	}
	named, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "all", Query: "run-family-hierarchy-runs", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(named) != 2 {
		t.Fatalf("named runs = %+v, want the two planned attempts", named)
	}
	bounded, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "all", StartedBefore: "2999-01-01 00:00:00", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded) != 3 {
		t.Fatalf("bounded runs = %+v, want all three attempts", bounded)
	}
	if _, _, err := store.ListHierarchyRuns(ctx, run.HierarchyRunFilter{Visibility: "invalid", Limit: 20}); err == nil {
		t.Fatal("expected a visibility validation error")
	}
}

// TestStoreListLegacyRunsFiltersAndIncludesTrashed verifies the deprecated run
// discovery projection honors revision, plan, visibility, and limit bounds.
func TestStoreListLegacyRunsFiltersAndIncludesTrashed(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	planID := createTestPlan(t, db, "legacy-runs")

	firstRun, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.PipelineRuns.StartAttempt(planID, "workspace", "query"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE pipeline_runs SET visibility_state='trashed', trashed_at=datetime('now'), trash_reason='legacy' WHERE id=?", firstRun); err != nil {
		t.Fatal(err)
	}

	active, err := store.ListLegacyRuns(ctx, run.LegacyRunFilter{PlanID: planID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ExecutionPlanID == nil || *active[0].ExecutionPlanID != planID || active[0].SearchRevisionID == nil {
		t.Fatalf("legacy active runs = %+v, want one planned non-trashed row", active)
	}
	if active[0].TrashReason != nil {
		t.Fatalf("legacy active trash reason = %v, want nil", active[0].TrashReason)
	}
	byRevision, err := store.ListLegacyRuns(ctx, run.LegacyRunFilter{SearchRevisionID: *active[0].SearchRevisionID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(byRevision) != 1 || byRevision[0].ID != active[0].ID {
		t.Fatalf("legacy revision runs = %+v, want the active planned attempt", byRevision)
	}

	all, err := store.ListLegacyRuns(ctx, run.LegacyRunFilter{PlanID: planID, IncludeTrashed: true, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID == firstRun || all[1].ID != firstRun {
		t.Fatalf("legacy trashed page = %+v, want newest first and trashed run %d last", all, firstRun)
	}
	trashed := all[1]
	if trashed.TrashReason == nil || *trashed.TrashReason != "legacy" || trashed.TrashedAt == nil {
		t.Fatalf("legacy trashed evidence = %+v", trashed)
	}
	bounded, err := store.ListLegacyRuns(ctx, run.LegacyRunFilter{PlanID: planID, IncludeTrashed: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded) != 1 || bounded[0].ID != all[0].ID {
		t.Fatalf("legacy bounded page = %+v, want newest run %d", bounded, all[0].ID)
	}
	if _, err := store.ListLegacyRuns(ctx, run.LegacyRunFilter{PlanID: planID, Limit: 0}); err == nil {
		t.Fatal("expected a limit validation error")
	}
}
