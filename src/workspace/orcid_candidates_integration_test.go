// orcid_integration_test.go tests the ORCID candidate-search
// pipeline path, including name-derived candidate extraction and
// integration with the enrichment cache.
//go:build integration

package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/article"
	"analysis/database"
	"analysis/enrich"
	"analysis/manifest"
)

// TestORCIDNameSearchRetainsCandidatesWithoutCreatingIdentity verifies orcid name search retains candidates without creating identity.
func TestORCIDNameSearchRetainsCandidatesWithoutCreatingIdentity(t *testing.T) {
	requests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"orcid-identifier":{"path":"0000-0001-2345-6789"}},{"orcid-identifier":{"path":"0000-0002-1825-0097"}},{"orcid-identifier":{"path":"0000-0001-2345-6789"}}]}`))
	}))
	defer provider.Close()

	db, cache, runID := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run"}})
	defer db.Close()
	articles := []*article.Article{{DOI: "10.1000/orcid-candidate", Authors: []article.Author{{CitationName: "Lovelace, Ada"}}}}
	source := enrich.SourceConfig{Name: "orcid", BaseURL: provider.URL + "/record/", ExtraURLs: map[string]string{"search": provider.URL + "/search"}, Concurrency: 1, BatchSize: 1, TimeoutSecs: 5, MaxRetries: 1, RatePerSecond: 1000}
	updated, changes, evidence, err := enrichCachedORCID(context.Background(), cache, source, enrich.SourceConfig{}, false, articles)
	if err != nil {
		t.Fatal(err)
	}
	if updated != 0 || len(changes) != 0 || requests != 3 {
		t.Fatalf("name search must not enrich the author: updated=%d changes=%#v requests=%d", updated, changes, requests)
	}
	if articles[0].Authors[0].Orcid != "" || articles[0].Authors[0].FirstName != "" || articles[0].Authors[0].LastName != "" {
		t.Fatalf("name-search candidate changed observed author: %#v", articles[0].Authors[0])
	}
	if len(evidence) != 1 || len(evidence[0].Candidates) != 2 || evidence[0].Candidates[0].ProviderRank != 1 || evidence[0].Candidates[1].ProviderRank != 2 {
		t.Fatalf("candidate evidence = %#v", evidence)
	}
	if evidence[0].Candidates[0].PayloadArtifactID == 0 {
		t.Fatalf("candidate evidence omitted raw provider payload reference: %#v", evidence[0])
	}

	_, revisionID, err := persistWorkSnapshot(db, runID, articles[0], database.ProducerStageEnrich, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := persistUncertainORCIDEvidence(db, runID, map[string]int64{articles[0].DOI: revisionID}, evidence); err != nil {
		t.Fatal(err)
	}
	var status, observedORCID string
	var personID any
	var candidates int
	err = db.DB.QueryRow(`SELECT r.status, COALESCE(ao.orcid, ''), ao.person_id, COUNT(c.id)
		FROM author_identity_resolutions r
		JOIN author_occurrences ao ON ao.id=r.author_occurrence_id
		LEFT JOIN author_identity_candidates c ON c.identity_resolution_id=r.id
		GROUP BY r.id`).Scan(&status, &observedORCID, &personID, &candidates)
	if err != nil {
		t.Fatal(err)
	}
	if status != database.AuthorIdentityStatusORCIDUnclear || observedORCID != "" || personID != nil || candidates != 2 {
		t.Fatalf("uncertain candidate became an identity: status=%q observed_orcid=%q person=%v candidates=%d", status, observedORCID, personID, candidates)
	}
	var people int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM people").Scan(&people); err != nil {
		t.Fatal(err)
	}
	if people != 0 {
		t.Fatalf("name-search candidates created people rows: %d", people)
	}
}

// TestWorkspacePipelinePreservesMetadataAndProviderFailureEvidence verifies workspace pipeline preserves metadata and provider failure evidence.
func TestWorkspacePipelinePreservesMetadataAndProviderFailureEvidence(t *testing.T) {
	request := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request++
		w.Header().Set("Content-Type", "application/json")
		if request == 1 {
			_, _ = w.Write([]byte(`{"result":[{"orcid-identifier":{"path":"0000-0001-2345-6789"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer provider.Close()

	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.csv")
	if err := os.WriteFile(sourcePath, []byte("doi,title,year,authors\n10.1000/orcid-provider-failure,Provider failure,2024,\"Lovelace, Ada\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := testWorkspaceRun(sourcePath)
	run.Manifest.EnrichmentEnabled = true
	run.Manifest.CachePolicy = manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run"}}
	run.Manifest.Sources[0].KeepFields = []string{"doi", "title", "year", "authors"}
	run.Manifest.EnrichmentProviders = []manifest.EnrichmentProvider{{Name: "orcid", BaseURL: provider.URL + "/record/", ExtraURLs: map[string]string{"search": provider.URL + "/search"}}}
	run.Enrichment = &enrich.Config{Sources: map[string]enrich.SourceConfig{
		"orcid": {Name: "orcid", BaseURL: provider.URL + "/record/", ExtraURLs: map[string]string{"search": provider.URL + "/search"}, Concurrency: 1, BatchSize: 1, TimeoutSecs: 5, MaxRetries: 1, RatePerSecond: 1000},
	}}

	dbPath := filepath.Join(tempDir, "workspace.db")
	err := RunPipeline(dbPath, []byte("workspace-config"), run, false)
	if err == nil || !strings.Contains(err.Error(), "invalid provider payload") {
		t.Fatalf("pipeline error = %v, want ORCID payload failure", err)
	}
	db, err := database.Open(dbPath, databaseRegistryPath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var runStatus string
	if err := db.DB.QueryRow(`SELECT status FROM pipeline_runs ORDER BY id DESC LIMIT 1`).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != "failed" {
		t.Fatalf("run status = %q, want failed", runStatus)
	}
	var metadataRevisionCount, identityRevisionCount int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM work_revisions WHERE producer_stage=?`, database.ProducerStageEnrichMetadata).Scan(&metadataRevisionCount); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM work_revisions WHERE producer_stage=?`, database.ProducerStageEnrichIdentity).Scan(&identityRevisionCount); err != nil {
		t.Fatal(err)
	}
	if metadataRevisionCount != 1 || identityRevisionCount != 0 {
		t.Fatalf("metadata/identity revisions = %d/%d, want 1/0", metadataRevisionCount, identityRevisionCount)
	}
	work, err := db.Works.GetByDOI("10.1000/orcid-provider-failure")
	if err != nil || work == nil {
		t.Fatalf("failed identity work = %+v, %v", work, err)
	}
	identityStage, err := db.RunWorkStages.GetByRunAndWork(1, work.ID, database.StageNameEnrichIdentity)
	if err != nil || identityStage == nil || identityStage.Outcome != database.OutcomeFailed || !strings.Contains(identityStage.Reason, "invalid provider payload") {
		t.Fatalf("identity stage = %+v, %v", identityStage, err)
	}
	var status, errorMessage, snapshotStage string
	var candidateCount int
	if err := db.DB.QueryRow(`SELECT r.status, r.error_message, wr.producer_stage, COUNT(c.id)
		FROM author_identity_resolutions r
		JOIN author_occurrences ao ON ao.id=r.author_occurrence_id
		JOIN authorships a ON a.author_occurrence_id=ao.id
		JOIN work_revisions wr ON wr.id=a.work_revision_id
		LEFT JOIN author_identity_candidates c ON c.identity_resolution_id=r.id
		GROUP BY r.id`).Scan(&status, &errorMessage, &snapshotStage, &candidateCount); err != nil {
		t.Fatal(err)
	}
	if status != database.AuthorIdentityStatusProviderFailed || !strings.Contains(errorMessage, "invalid provider payload") || snapshotStage != database.ProducerStageEnrichMetadata || candidateCount != 1 {
		t.Fatalf("provider failure evidence = status=%q error=%q snapshot=%q candidates=%d", status, errorMessage, snapshotStage, candidateCount)
	}
}

// testWorkspaceRun supports the package test suite's test workspace run setup or assertions.
func testWorkspaceRun(sourcePath string) *Run {
	return &Run{
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
