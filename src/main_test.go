// main_test.go provides end-to-end pipeline tests that build a
// temporary binary, run the workspace pipeline against a fixture,
// and verify idempotent database rows and configuration artifacts.
package main

import (
	"analysis/database"
	"analysis/manifest"
	"analysis/pdfstore"
	"analysis/workspace"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPipelineEndToEndWorkspace verifies pipeline end to end workspace.
func TestPipelineEndToEndWorkspace(t *testing.T) {
	srcDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rootDir := filepath.Dir(srcDir)
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "fixture.raw.csv")
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	configPath := filepath.Join(tempDir, "workspace.something")
	binaryPath := filepath.Join(tempDir, "analysis")

	csv := "doi,title,year,authors,affiliations,publisher,cited_references\n" +
		"10.1234/example,Test Article,2024,Alice Smith,Example University,Example Publisher,Reference DOI 10.2345/ref\n"
	if err := os.WriteFile(rawPath, []byte(csv), 0644); err != nil {
		t.Fatal(err)
	}

	config := fmt.Sprintf(`
	available_filters: enum(string) = {
	    NO_FILTER = "No filter";
	    RANGE_10_YEARS = "Range 10 years";
	    ARTICLE_ONLY = "Article only";
	    ENGLISH_ONLY = "English only";
	}
	cache_policy_read_options: enum = {
	    ACTIVE_RUN;
	    GLOBAL;
	    NETWORK;
	    RUN_SPECIFIC;
	}
	cache_policy_write_options: enum = {
	    ACTIVE_RUN;
	    GLOBAL;
	}
	cache_policy_config: setup = {
	    reads: []cache_policy_read_options;
	    read_run_id?: integer = -1;
	    writes: []cache_policy_write_options;
	    negative_ttl_days: integer;
	}
	reuse_policy_config: setup = { policy: string; }
	raw_data_filters: setup = {
	    filters: []available_filters;
	    count: integer;
	}
	source_declaration: setup = {
	    name: string;
	    date: timestamp;
	    expected_file: string;
	    file_type: string;
	    query: string;
	    filters: []raw_data_filters;
	    expected_result_count: integer;
	    requested_fields: []string;
	    patch_fields: mapping(string, string);
	    keep_fields: []string;
	}
	enrichment_provider_config: setup = {
	    name: string;
	    base_url: string;
	    fields: []string;
	}
	workspace_config: setup = {
	    format_version: integer;
	    search_id: string;
	    search_revision: string;
	    enrichment_enabled: boolean;
	    reuse_policy: reuse_policy_config;
	    cache_policy: cache_policy_config;
	    sources: []source_declaration;
	    enrichment_providers: []enrichment_provider_config;
	}
	#iteration("_workspace"): scope = {
	    workspace: workspace_config = {
	        format_version = 2,
	        search_id = "fixture-search",
	        search_revision = "fixture-revision",
	        enrichment_enabled = false,
	        reuse_policy = reuse_policy_config { policy = "reuse_completed", },
	        cache_policy = cache_policy_config {
	            reads = []cache_policy_read_options { .GLOBAL },
	            writes = []cache_policy_write_options { .ACTIVE_RUN },
	            negative_ttl_days = 7,
	        },
	        sources = [{
	            name = "fixture",
	            date = "2026-01-01 00:00:00",
	            expected_file = %q,
	            file_type = "csv",
	            query = "TITLE(fixture)",
	            filters = []raw_data_filters{
	                { filters = [.NO_FILTER], count = 1 },
	                { filters = [.NO_FILTER, .ARTICLE_ONLY], count = 1 },
	            },
	            expected_result_count = 1,
	            requested_fields = []string { "doi", "title", "authors", "references" },
	            patch_fields = mapping(string, string) { ["title"] => "title" },
	            keep_fields = []string {
	                "doi", "title", "year", "authors", "affiliations",
	                "publisher", "cited_references",
	            },
	        }],
	        enrichment_providers = [],
	    };
	}
`, rawPath)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Dir = srcDir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build pipeline: %v\n%s", err, output)
	}

	for _, args := range [][]string{
		nil,
		{"unknown"},
		{"run", "--config", configPath},
		{"run", "--unexpected"},
		{"run", "--skip-enrichment"},
	} {
		cmd := exec.Command(binaryPath, args...)
		cmd.Dir = rootDir
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("expected command %q to fail, output: %s", args, output)
		}
	}

	for run := 1; run <= 3; run++ {
		args := []string{"run", "--config", configPath, "--db", dbPath}
		if run == 3 {
			args = append(args, "--fresh")
		}
		cmd := exec.Command(binaryPath, args...)
		cmd.Dir = rootDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("pipeline run %d: %v\n%s", run, err, output)
		}
	}

	db, err := database.Open(dbPath, filepath.Join(rootDir, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table := range []string{"articles", "authors", "article_authors", "article_references", "enrichment_log"} {
		var got int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != 0 {
			t.Errorf("deprecated table %s still exists", table)
		}
	}
	for table, want := range map[string]int{
		"works":              1,
		"work_revisions":     8,
		"author_occurrences": 8,
		"authorships":        8,
		"reference_mentions": 8,
		"run_sources":        2,
		"source_records":     2,
	} {
		var got int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s count after rerun: got %d, want %d", table, got, want)
		}
	}

	var completedRuns int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM pipeline_runs WHERE status = 'completed'").Scan(&completedRuns); err != nil {
		t.Fatal(err)
	}
	if completedRuns != 2 {
		t.Errorf("completed pipeline runs: got %d, want 2", completedRuns)
	}

	var artifactCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM artifacts").Scan(&artifactCount); err != nil {
		t.Fatal(err)
	}
	if artifactCount < 6 {
		t.Errorf("immutable stage artifact count: got %d, want at least 6", artifactCount)
	}
	var inputManifestHash string
	if err := db.DB.QueryRow("SELECT input_manifest_hash FROM execution_plans").Scan(&inputManifestHash); err != nil {
		t.Fatal(err)
	}
	var linkedArtifactCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM artifacts WHERE content_hash = ?", inputManifestHash).Scan(&linkedArtifactCount); err != nil {
		t.Fatal(err)
	}
	if linkedArtifactCount != 1 {
		t.Errorf("input manifest artifact link count: got %d, want 1", linkedArtifactCount)
	}
	var configurationSnapshotCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM run_artifacts WHERE pipeline_run_id=1").Scan(&configurationSnapshotCount); err != nil {
		t.Fatal(err)
	}
	if configurationSnapshotCount != 3 {
		t.Errorf("run configuration snapshots: got %d, want 3", configurationSnapshotCount)
	}
	var resolvedManifestArtifactCount int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM run_artifacts ra
		JOIN artifacts a ON a.id=ra.artifact_id
		JOIN execution_plans ep ON ep.id=(SELECT execution_plan_id FROM pipeline_runs WHERE id=ra.pipeline_run_id)
		WHERE ra.pipeline_run_id=1 AND ra.artifact_role='resolved_manifest' AND a.content_hash=ep.resolved_manifest_hash`).Scan(&resolvedManifestArtifactCount); err != nil {
		t.Fatal(err)
	}
	if resolvedManifestArtifactCount != 1 {
		t.Errorf("resolved manifest snapshot link count: got %d, want 1", resolvedManifestArtifactCount)
	}
	var enrichmentEnabled bool
	if err := db.DB.QueryRow("SELECT enrichment_enabled FROM execution_plans").Scan(&enrichmentEnabled); err != nil {
		t.Fatal(err)
	}
	if enrichmentEnabled {
		t.Fatal("execution plan enrichment policy = true, want false")
	}
	metric, err := db.Metrics.Get(1, "enrichment_enabled", "")
	if err != nil {
		t.Fatal(err)
	}
	if metric == nil || metric.Value != 0 {
		t.Fatalf("enrichment metric = %+v, want 0", metric)
	}
	for _, expected := range []struct {
		name   string
		source string
		value  int
	}{
		{"input_records", "", 1}, {"parsed_articles", "", 1},
		{"deduplicated_articles", "", 1}, {"duplicate_articles", "", 0},
		{"enrichment_skipped", "", 1}, {"enrichment_candidates", "", 1},
		{"enriched_article_updates", "", 0}, {"valid_articles", "", 1},
		{"discarded_articles", "", 0}, {"normalized_articles_processed", "", 1},
		{"input_records", "fixture", 1}, {"parsed_articles", "fixture", 1},
	} {
		metric, err := db.Metrics.Get(1, expected.name, expected.source)
		if err != nil || metric == nil || metric.Value != expected.value {
			t.Fatalf("metric %s/%s = %+v, %v; want %d", expected.name, expected.source, metric, err, expected.value)
		}
	}
	auditEvents, err := db.AuditEvents.ListByRun(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(auditEvents) == 0 || !strings.Contains(auditEvents[0].MetadataJSON, `"enrichment_enabled":false`) {
		t.Fatalf("run audit does not record enrichment policy: %+v", auditEvents)
	}
	var planMetadata string
	if err := db.DB.QueryRow("SELECT metadata_json FROM audit_events WHERE action=?", manifest.AuditDuplicatePlanSkipped).Scan(&planMetadata); err != nil || !strings.Contains(planMetadata, "matching_completed_plan") {
		t.Fatalf("completed-plan reuse audit = %q, %v", planMetadata, err)
	}
	steps, err := db.RunSteps.ListByRun(2)
	if err != nil {
		t.Fatal(err)
	}
	var preflight *database.RunStep
	for _, step := range steps {
		if step.StepName == "preflight" {
			preflight = step
		}
	}
	if len(steps) != 5 || preflight == nil || preflight.StepStatus != "reused" || preflight.ReusedFromRunID == nil || *preflight.ReusedFromRunID != 1 {
		t.Fatalf("fresh-run workspace steps = %+v", steps)
	}
	if preflight.InputFingerprint == "" || preflight.OutputFingerprint == "" || preflight.InputArtifactID == nil || preflight.OutputArtifactID == nil {
		t.Fatalf("preflight provenance is incomplete: %+v", preflight)
	}
	var stepRunID int64
	if err := db.DB.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action=?", manifest.AuditStepReused).Scan(&stepRunID); err != nil || stepRunID != 2 {
		t.Fatalf("preflight reuse audit run ID = %d, %v", stepRunID, err)
	}
	pdfInventory, err := pdfstore.Open(filepath.Join(tempDir, pdfstore.DefaultStoreFilename), filepath.Join(rootDir, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer pdfInventory.Close()
	var inventoryRows int
	if err := pdfInventory.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents WHERE status='not_available'").Scan(&inventoryRows); err != nil {
		t.Fatal(err)
	}
	if inventoryRows != 1 {
		t.Fatalf("normalized PDF inventory rows = %d, want 1", inventoryRows)
	}

}

// TestWorkspaceAttemptRetryAndSourceHash verifies workspace attempt retry and source hash.
func TestWorkspaceAttemptRetryAndSourceHash(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "workspace.db")
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("first source version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(dbPath, filepath.Join("..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	run := testWorkspaceRun(sourcePath)
	firstRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), run, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(firstRunID, "failed", "fixture failure"); err != nil {
		t.Fatal(err)
	}

	retryRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), run, false)
	if err != nil {
		t.Fatalf("retry failed plan: %v", err)
	}
	if retryRunID == firstRunID {
		t.Fatal("retry did not create a new attempt")
	}
	if err := db.PipelineRuns.FinishRun(retryRunID, "completed", ""); err != nil {
		t.Fatal(err)
	}

	noOpRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), run, false)
	if err != nil {
		t.Fatal(err)
	}
	if noOpRunID != 0 {
		t.Fatalf("completed matching plan started run %d, want no-op", noOpRunID)
	}

	if err := os.WriteFile(sourcePath, []byte("changed source version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	changedRun := testWorkspaceRun(sourcePath)
	changedRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), changedRun, false)
	if err != nil {
		t.Fatal(err)
	}
	if changedRunID == 0 {
		t.Fatal("changed source hash incorrectly reused a completed plan")
	}

	var planCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM execution_plans").Scan(&planCount); err != nil {
		t.Fatal(err)
	}
	if planCount != 2 {
		t.Fatalf("execution plans after source change = %d, want 2", planCount)
	}
}

// TestWorkspaceRevisionPlanAndAttemptGrouping verifies workspace revision plan and attempt grouping.
func TestWorkspaceRevisionPlanAndAttemptGrouping(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "workspace.db")
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("source version one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(dbPath, filepath.Join("..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	base := testWorkspaceRun(sourcePath)
	firstRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config-v1"), base, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(firstRunID, "completed", ""); err != nil {
		t.Fatal(err)
	}

	freshRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config-v1"), testWorkspaceRun(sourcePath), true)
	if err != nil {
		t.Fatal(err)
	}
	if freshRunID == firstRunID {
		t.Fatal("--fresh did not create a distinct run attempt")
	}
	var firstPlanID, freshPlanID int64
	if err := db.DB.QueryRow("SELECT execution_plan_id FROM pipeline_runs WHERE id=?", firstRunID).Scan(&firstPlanID); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT execution_plan_id FROM pipeline_runs WHERE id=?", freshRunID).Scan(&freshPlanID); err != nil {
		t.Fatal(err)
	}
	if firstPlanID != freshPlanID {
		t.Fatalf("--fresh plan = %d, want matching plan %d", freshPlanID, firstPlanID)
	}
	if err := db.PipelineRuns.FinishRun(freshRunID, "completed", ""); err != nil {
		t.Fatal(err)
	}

	changedPolicy := testWorkspaceRun(sourcePath)
	changedPolicy.Manifest.CachePolicy.NegativeTTLDays = 14
	policyRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config-cache-policy"), changedPolicy, false)
	if err != nil {
		t.Fatal(err)
	}
	var policyPlanID int64
	if err := db.DB.QueryRow("SELECT execution_plan_id FROM pipeline_runs WHERE id=?", policyRunID).Scan(&policyPlanID); err != nil {
		t.Fatal(err)
	}
	if policyPlanID == firstPlanID {
		t.Fatal("changed cache policy reused the prior execution plan")
	}
	if err := db.PipelineRuns.FinishRun(policyRunID, "completed", ""); err != nil {
		t.Fatal(err)
	}

	changedRevision := testWorkspaceRun(sourcePath)
	changedRevision.Manifest.SearchRevision = "r2-query-expansion"
	revisionRunID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config-r2"), changedRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	var revisionPlanID int64
	if err := db.DB.QueryRow("SELECT execution_plan_id FROM pipeline_runs WHERE id=?", revisionRunID).Scan(&revisionPlanID); err != nil {
		t.Fatal(err)
	}
	if revisionPlanID == firstPlanID || revisionPlanID == policyPlanID {
		t.Fatal("new search revision did not create a plan beneath a distinct revision")
	}

	var revisionCount, planCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM search_revisions").Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM execution_plans").Scan(&planCount); err != nil {
		t.Fatal(err)
	}
	if revisionCount != 2 || planCount != 3 {
		t.Fatalf("grouping counts: revisions=%d plans=%d, want 2 revisions and 3 plans", revisionCount, planCount)
	}
}

// TestWorkspacePipelineNormalizesOnlyValidArticles verifies workspace pipeline normalizes only valid articles.
func TestWorkspacePipelineNormalizesOnlyValidArticles(t *testing.T) {
	chdirToRepositoryRoot(t)
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.csv")
	contents := "doi,title,year,authors,publisher,cited_references\n" +
		"10.1234/valid,Valid article,2024,Alice Smith,Example Publisher,10.5555/reference\n" +
		"10.1234/discarded,Discarded article,2024,Bob Jones,,10.5555/reference\n"
	if err := os.WriteFile(sourcePath, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(tempDir, "workspace.db")
	run := testWorkspaceRun(sourcePath)
	run.Manifest.SearchID = "normalize-only-valid"
	run.Manifest.Sources[0].KeepFields = []string{"doi", "title", "year", "authors", "publisher", "cited_references"}
	if err := workspace.RunPipeline(dbPath, []byte("workspace-config"), run, false); err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(dbPath, filepath.Join("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, check := range []struct {
		doi             string
		validateOutcome string
		normalized      bool
	}{
		{doi: "10.1234/valid", validateOutcome: database.OutcomeValid, normalized: true},
		{doi: "10.1234/discarded", validateOutcome: database.OutcomeDiscarded, normalized: false},
	} {
		work, err := db.Works.GetByDOI(check.doi)
		if err != nil || work == nil {
			t.Fatalf("work %s = %+v, %v", check.doi, work, err)
		}
		validationStage, err := db.RunWorkStages.GetByRunAndWork(1, work.ID, database.StageNameValidate)
		if err != nil || validationStage == nil || validationStage.Outcome != check.validateOutcome {
			t.Fatalf("validation stage for %s = %+v, %v", check.doi, validationStage, err)
		}
		if check.validateOutcome == database.OutcomeDiscarded && validationStage.Reason == "" {
			t.Fatalf("discarded work %s has no validation reason", check.doi)
		}
		normalizationStage, err := db.RunWorkStages.GetByRunAndWork(1, work.ID, database.StageNameNormalize)
		if err != nil {
			t.Fatal(err)
		}
		if check.normalized != (normalizationStage != nil) {
			t.Fatalf("normalization stage for %s = %+v, want normalized=%t", check.doi, normalizationStage, check.normalized)
		}
		if check.normalized && normalizationStage.Outcome != database.OutcomeNormalized {
			t.Fatalf("normalization outcome for %s = %q, want %q", check.doi, normalizationStage.Outcome, database.OutcomeNormalized)
		}
	}
	var normalizedCount int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM work_revisions WHERE producer_stage = 'normalize'`).Scan(&normalizedCount); err != nil {
		t.Fatal(err)
	}
	if normalizedCount != 1 {
		t.Fatalf("normalized revisions = %d, want 1 valid work only", normalizedCount)
	}
}

// TestWorkspaceAttemptRejectsConcurrentPlan verifies workspace attempt rejects concurrent plan.
func TestWorkspaceAttemptRejectsConcurrentPlan(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("source\n"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(filepath.Join(tempDir, "workspace.db"), filepath.Join("..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	run := testWorkspaceRun(sourcePath)
	if _, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), run, false); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), run, false); err == nil || !strings.Contains(err.Error(), "already has running attempt") {
		t.Fatalf("concurrent matching plan error = %v", err)
	}
}

// TestWorkspaceAttemptRecordsDeclaredEnrichmentPolicy verifies workspace attempt records declared enrichment policy.
func TestWorkspaceAttemptRecordsDeclaredEnrichmentPolicy(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("source\n"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(filepath.Join(tempDir, "workspace.db"), filepath.Join("..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, enabled := range []bool{false, true} {
		run := testWorkspaceRun(sourcePath)
		run.Manifest.EnrichmentEnabled = enabled
		run.Manifest.SearchRevision = fmt.Sprintf("r-%t", enabled)
		runID, err := workspace.StartWorkspaceAttempt(db, []byte("workspace-config"), run, false)
		if err != nil {
			t.Fatal(err)
		}

		var stored bool
		if err := db.DB.QueryRow(`SELECT ep.enrichment_enabled
			FROM execution_plans ep JOIN pipeline_runs pr ON pr.execution_plan_id = ep.id
			WHERE pr.id = ?`, runID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != enabled {
			t.Fatalf("plan enrichment_enabled = %t, want %t", stored, enabled)
		}
		metric, err := db.Metrics.Get(runID, "enrichment_enabled", "")
		if err != nil {
			t.Fatal(err)
		}
		expectedMetric := 0
		if enabled {
			expectedMetric = 1
		}
		if metric == nil || metric.Value != expectedMetric {
			t.Fatalf("enrichment metric = %+v, want %d", metric, expectedMetric)
		}
		events, err := db.AuditEvents.ListByRun(runID)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 || !strings.Contains(events[0].MetadataJSON, fmt.Sprintf(`"enrichment_enabled":%t`, enabled)) {
			t.Fatalf("run audit does not record declared enrichment=%t: %+v", enabled, events)
		}
	}
}

// TestWorkspacePipelineRecordsUnreadableSourcePreflightFailure verifies workspace pipeline records unreadable source preflight failure.
func TestWorkspacePipelineRecordsUnreadableSourcePreflightFailure(t *testing.T) {
	chdirToRepositoryRoot(t)
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "workspace.db")
	missingPath := filepath.Join(tempDir, "missing.csv")
	run := testWorkspaceRun(missingPath)

	err := workspace.RunPipeline(dbPath, []byte("workspace-config"), run, false)
	if err == nil {
		t.Fatal("workspace.RunPipeline succeeded for an unreadable source")
	}
	if !strings.Contains(err.Error(), `source "fixture"`) || !strings.Contains(err.Error(), missingPath) {
		t.Fatalf("unreadable source error = %q, want source name and path", err)
	}

	db, err := database.Open(dbPath, filepath.Join("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for table, want := range map[string]int{
		"searches":         1,
		"search_revisions": 1,
		"execution_plans":  1,
		"pipeline_runs":    1,
	} {
		var got int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s rows = %d, want %d", table, got, want)
		}
	}

	var runID int64
	var status, summary string
	if err := db.DB.QueryRow(`SELECT id, status, COALESCE(summary, '') FROM pipeline_runs`).Scan(&runID, &status, &summary); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !strings.Contains(summary, missingPath) {
		t.Fatalf("pipeline run status/summary = %q/%q, want failed with %q", status, summary, missingPath)
	}

	steps, err := db.RunSteps.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].StepName != "preflight" || steps[0].StepStatus != string(manifest.StageFailed) {
		t.Fatalf("preflight steps = %+v, want one failed preflight step", steps)
	}

	events, err := db.AuditEvents.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	foundFailureAudit := false
	for _, event := range events {
		if event.Action == string(manifest.AuditRunFailed) && strings.Contains(event.MetadataJSON, missingPath) {
			foundFailureAudit = true
		}
	}
	if !foundFailureAudit {
		t.Fatalf("missing failed-run audit for unreadable source: %+v", events)
	}

	rows, err := db.DB.Query("SELECT data FROM artifact_blobs WHERE pipeline_run_id=?", runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	foundInputFailure := false
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"read_error"`) && strings.Contains(string(data), missingPath) {
			foundInputFailure = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !foundInputFailure {
		t.Fatalf("input manifest artifact does not record unreadable source %q", missingPath)
	}
}

// TestWorkspacePipelineFailsForEmptyOrMalformedSource verifies workspace pipeline fails for empty or malformed source.
func TestWorkspacePipelineFailsForEmptyOrMalformedSource(t *testing.T) {
	chdirToRepositoryRoot(t)
	tests := []struct {
		name     string
		fileType string
		contents string
		wantErr  string
	}{
		{name: "empty CSV", fileType: "csv", contents: "", wantErr: "is empty"},
		{name: "header-only CSV", fileType: "csv", contents: "doi,title,year\n", wantErr: "has no data records"},
		{name: "malformed CSV", fileType: "csv", contents: "doi,title\n10.1000/one,title,unexpected\n", wantErr: "record on line 2"},
		{name: "empty BibTeX", fileType: "bib", contents: "", wantErr: "has no article entries"},
		{name: "malformed BibTeX", fileType: "bib", contents: "@{", wantErr: "expected token type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			sourcePath := filepath.Join(tempDir, "source."+tc.fileType)
			if err := os.WriteFile(sourcePath, []byte(tc.contents), 0644); err != nil {
				t.Fatal(err)
			}
			dbPath := filepath.Join(tempDir, "workspace.db")
			run := testWorkspaceRun(sourcePath)
			run.Manifest.SearchID = "fatal-" + strings.ReplaceAll(tc.name, " ", "-")
			run.Manifest.Sources[0].FileType = tc.fileType
			run.Manifest.Sources[0].ExpectedResultCount = 1

			err := workspace.RunPipeline(dbPath, []byte("workspace-config"), run, false)
			if err == nil {
				t.Fatal("workspace.RunPipeline succeeded")
			}
			if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), sourcePath) {
				t.Fatalf("fatal source error = %q, want %q and %q", err, tc.wantErr, sourcePath)
			}

			db, err := database.Open(dbPath, filepath.Join("config", "database.something"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if tc.name == "empty CSV" {
				var expected, observed int
				var comparison string
				if err := db.DB.QueryRow(`SELECT expected_result_count, observed_result_count, result_count_comparison FROM run_sources`).Scan(&expected, &observed, &comparison); err != nil {
					t.Fatal(err)
				}
				if expected != 1 || observed != 0 || comparison != "below" {
					t.Fatalf("empty-source count comparison = %d/%d/%q, want 1/0/below", expected, observed, comparison)
				}
			}

			var runID int64
			var status string
			var summary string
			if err := db.DB.QueryRow(`SELECT id, status, COALESCE(summary, '') FROM pipeline_runs`).Scan(&runID, &status, &summary); err != nil {
				t.Fatal(err)
			}
			if status != "failed" || !strings.Contains(summary, tc.wantErr) {
				t.Fatalf("pipeline run status/summary = %q/%q, want failed containing %q", status, summary, tc.wantErr)
			}

			events, err := db.AuditEvents.ListByRun(runID)
			if err != nil {
				t.Fatal(err)
			}
			foundFailureAudit := false
			for _, event := range events {
				if event.Action == string(manifest.AuditRunFailed) && strings.Contains(event.MetadataJSON, tc.wantErr) {
					foundFailureAudit = true
				}
			}
			if !foundFailureAudit {
				t.Fatalf("missing failed-run audit for %q: %+v", tc.wantErr, events)
			}

			steps, err := db.RunSteps.ListByRun(runID)
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range steps {
				if step.StepName == "parse" {
					t.Fatalf("fatal source error wrote parse step: %+v", step)
				}
			}
		})
	}
}

// TestWorkspacePipelineRecordsInformationalExpectedResultCounts verifies workspace pipeline records informational expected result counts.
func TestWorkspacePipelineRecordsInformationalExpectedResultCounts(t *testing.T) {
	chdirToRepositoryRoot(t)
	tests := []struct {
		name       string
		expected   int
		comparison string
	}{
		{name: "below", expected: 3, comparison: "below"},
		{name: "above", expected: 1, comparison: "above"},
		{name: "match", expected: 2, comparison: "match"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			sourcePath := filepath.Join(tempDir, "source.csv")
			contents := "doi,title,year\n10.1000/one,One,2024\n10.1000/two,Two,2025\n"
			if err := os.WriteFile(sourcePath, []byte(contents), 0644); err != nil {
				t.Fatal(err)
			}
			run := testWorkspaceRun(sourcePath)
			run.Manifest.SearchID = "expected-count-" + tc.name
			run.Manifest.Sources[0].ExpectedResultCount = tc.expected
			dbPath := filepath.Join(tempDir, "workspace.db")
			if err := workspace.RunPipeline(dbPath, []byte("workspace-config"), run, false); err != nil {
				t.Fatalf("mismatched expected result count failed run: %v", err)
			}

			db, err := database.Open(dbPath, filepath.Join("config", "database.something"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var expected, observed int
			var comparison, status string
			if err := db.DB.QueryRow(`SELECT expected_result_count, observed_result_count, result_count_comparison FROM run_sources`).Scan(&expected, &observed, &comparison); err != nil {
				t.Fatal(err)
			}
			if err := db.DB.QueryRow(`SELECT status FROM pipeline_runs`).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if expected != tc.expected || observed != 2 || comparison != tc.comparison || status != "completed" {
				t.Fatalf("source result count/status = %d/%d/%q/%q, want %d/2/%q/completed", expected, observed, comparison, status, tc.expected, tc.comparison)
			}
			for metric, want := range map[string]int{"expected_result_count": tc.expected, "observed_result_count": 2} {
				got, err := db.Metrics.Get(1, metric, "fixture")
				if err != nil || got == nil || got.Value != want {
					t.Fatalf("%s metric = %+v, %v; want %d", metric, got, err, want)
				}
			}
		})
	}
}

// TestWorkspacePipelineRetainsSourceRecordsRejectedDuringCanonicalConversion verifies workspace pipeline retains source records rejected during canonical conversion.
func TestWorkspacePipelineRetainsSourceRecordsRejectedDuringCanonicalConversion(t *testing.T) {
	chdirToRepositoryRoot(t)
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("doi,title,year\n,,\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(tempDir, "workspace.db")
	run := testWorkspaceRun(sourcePath)
	run.Manifest.SearchID = "rejected-source-record"

	if err := workspace.RunPipeline(dbPath, []byte("workspace-config"), run, false); err != nil {
		t.Fatalf("workspace.RunPipeline: %v", err)
	}

	db, err := database.Open(dbPath, filepath.Join("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var status string
	if err := db.DB.QueryRow(`SELECT status FROM pipeline_runs`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("pipeline run status = %q, want completed", status)
	}
	var rejected int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM source_records WHERE parse_status = 'rejected'`).Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected != 1 {
		t.Fatalf("rejected source records = %d, want 1", rejected)
	}
}

// TestFrontendAssets verifies frontend asset directory validation.
func TestFrontendAssets(t *testing.T) {
	assetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetDir, "index.html"), []byte("filesystem asset"), 0644); err != nil {
		t.Fatal(err)
	}
	assets, err := frontendAssets(assetDir)
	if err != nil {
		t.Fatalf("frontendAssets filesystem directory: %v", err)
	}
	contents, err := fs.ReadFile(assets, "index.html")
	if err != nil || string(contents) != "filesystem asset" {
		t.Fatalf("filesystem assets index = %q, %v", contents, err)
	}

	if _, err := frontendAssets(filepath.Join(assetDir, "missing")); err == nil {
		t.Fatal("frontendAssets accepted missing directory")
	}
	filePath := filepath.Join(assetDir, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := frontendAssets(filePath); err == nil {
		t.Fatal("frontendAssets accepted regular file")
	}
}

// TestVersion verifies the version command output format and current value.
func TestVersion(t *testing.T) {
	got := version()
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(got) {
		t.Fatalf("version() = %q, want MAJOR.MINOR.PATCH", got)
	}
	if !strings.HasPrefix(got, "1.0.0") {
		t.Errorf("version() = %q, want current version 1.0.0", got)
	}
}

// TestValidateLoopbackAddress verifies writable serving rejects names, wildcards, and remote IPs.
func TestValidateLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8080", "[::1]:0"} {
		if err := validateLoopbackAddress(address); err != nil {
			t.Errorf("validate %q: %v", address, err)
		}
	}
	for _, address := range []string{"localhost:8080", "0.0.0.0:8080", "[::]:8080", "192.0.2.1:8080", "missing-port"} {
		if err := validateLoopbackAddress(address); err == nil {
			t.Errorf("expected %q to be rejected", address)
		}
	}
}

// chdirToRepositoryRoot supports the package test suite's chdir to repository root setup or assertions.
func chdirToRepositoryRoot(t *testing.T) {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(current)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(current); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

// testWorkspaceRun supports the package test suite's test workspace run setup or assertions.
func testWorkspaceRun(sourcePath string) *workspace.Run {
	return &workspace.Run{
		Manifest: &manifest.ResolvedManifest{
			FormatVersion:     2,
			SearchID:          "attempt-fixture",
			SearchRevision:    "r1",
			EnrichmentEnabled: false,
			ReusePolicy:       "reuse_completed",
			CachePolicy: manifest.CachePolicy{
				Reads:  []string{"global"},
				Writes: []string{"active_run"},
			},
			Sources: []manifest.SourceManifest{{
				Name: "fixture", ExpectedFile: sourcePath, FileType: "csv", Query: "fixture",
				RequestedFields: []string{"doi"},
			}},
		},
	}
}
