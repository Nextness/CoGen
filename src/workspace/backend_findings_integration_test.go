//go:build integration

// Package workspace_test verifies backend review regressions through public pipeline and repository boundaries.
package workspace_test

import (
	"analysis/article"
	"analysis/database"
	"analysis/enrich"
	"analysis/manifest"
	"analysis/notes"
	"analysis/workspace"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestReviewDOIRequestPath checks DOI punctuation remains identifier data in provider requests.
func TestReviewDOIRequestPath(t *testing.T) {
	_, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	sourcePath := run.Manifest.Sources[0].ExpectedFile
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	doi := "10.1000/review?part#section"
	if err := os.WriteFile(sourcePath, []byte(strings.Replace(string(data), "10.1000/review", doi, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	reviewProvider(t, run, `{"message":{"title":["Synthetic response"]}}`)
	transport := http.DefaultTransport
	var observed *http.Request
	http.DefaultTransport = reviewTransport(func(request *http.Request) (*http.Response, error) {
		observed = request
		return transport.RoundTrip(request)
	})
	if err := workspace.RunPipeline(path, []byte("// synthetic DOI transport"), run, false); err != nil {
		t.Fatal(err)
	}
	if observed == nil {
		t.Fatal("provider request missing")
	}
	if observed.URL.Path != "/works/"+doi || observed.URL.RawQuery != "" || observed.URL.Fragment != "" {
		t.Errorf("DOI was interpreted as URL syntax: path=%q query=%q fragment=%q", observed.URL.Path, observed.URL.RawQuery, observed.URL.Fragment)
	}
}

// TestReviewRetryExhaustion checks an exhausted request does not enter another backoff.
func TestReviewRetryExhaustion(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	attempts := 0
	http.DefaultTransport = reviewTransport(func(request *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})
	client, err := enrich.NewClient(enrich.SourceConfig{Name: "crossref", BaseURL: "https://provider.invalid/", RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 1, BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := client.Fetch(ctx, "https://provider.invalid/")
	if attempts != 2 {
		t.Fatalf("expected two attempts before deadline, got %d", attempts)
	}
	if errors.Is(result.Err, context.DeadlineExceeded) {
		t.Errorf("both allowed attempts finished after about 2 seconds, but the unnecessary final backoff ran into the 3-second deadline: %v", result.Err)
	}
}

// reviewRoot locates the repository from either the Makefile or Go test working directory.
func reviewRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "config/database.something")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

// reviewTransport intercepts provider traffic without opening a socket or contacting live services.
type reviewTransport func(*http.Request) (*http.Response, error)

// RoundTrip supplies a synthetic response to the real rate-limited provider client.
func (transport reviewTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

// reviewProvider configures a bounded synthetic Crossref client and records its manifest fields.
func reviewProvider(t *testing.T, run *workspace.Run, body string) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = reviewTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	source := enrich.SourceConfig{Name: "crossref", BaseURL: "https://provider.invalid/works/", RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 1, BatchSize: 1, Fields: []string{"title"}}
	run.Enrichment = &enrich.Config{Sources: map[string]enrich.SourceConfig{"crossref": source}}
	run.Manifest.EnrichmentEnabled = true
	run.Manifest.CachePolicy = manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run"}}
	run.Manifest.EnrichmentProviders = []manifest.EnrichmentProvider{{Name: source.Name, BaseURL: source.BaseURL, Fields: source.Fields}}
}

// TestReviewProviderFieldSelection checks that requesting only titles does not overwrite publishers.
func TestReviewProviderFieldSelection(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	reviewProvider(t, run, `{"message":{"title":["Updated title"],"publisher":"Replacement Publisher"}}`)
	if err := workspace.RunPipeline(path, []byte("// synthetic field policy"), run, false); err != nil {
		t.Fatal(err)
	}
	var publisher string
	if err := db.DB.QueryRow("SELECT publisher FROM work_revisions WHERE producer_stage='normalize'").Scan(&publisher); err != nil {
		t.Fatal(err)
	}
	if publisher != "IEEE" {
		t.Errorf("title-only enrichment changed publisher to %q", publisher)
	}
}

// TestReviewUnstructuredReferencePreserved checks provider reference text survives decoding and persistence.
func TestReviewUnstructuredReferencePreserved(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	reviewProvider(t, run, `{"message":{"reference":[{"key":"r1","unstructured":"Synthetic reference text"}]}}`)
	source := run.Enrichment.Sources["crossref"]
	source.Fields = []string{"reference"}
	run.Enrichment.Sources["crossref"] = source
	run.Manifest.EnrichmentProviders[0].Fields = source.Fields
	if err := workspace.RunPipeline(path, []byte("// synthetic unstructured reference"), run, false); err != nil {
		t.Fatal(err)
	}
	var emptyReferences int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM reference_mentions reference JOIN work_revisions revision ON revision.id=reference.work_revision_id
		WHERE revision.producer_stage='normalize' AND reference.raw_reference IS NULL AND reference.doi IS NULL AND reference.title IS NULL AND reference.author IS NULL AND reference.source IS NULL AND reference.year IS NULL`).Scan(&emptyReferences); err != nil {
		t.Fatal(err)
	}
	if emptyReferences > 0 {
		t.Errorf("normalized corpus contains %d completely empty reference mentions after replacing a real source reference", emptyReferences)
	}
}

// TestReviewOversizedLinkAllocation checks oversized link rejection occurs before excessive copying.
func TestReviewOversizedLinkAllocation(t *testing.T) {
	body := "[[ext:https://example.invalid/" + strings.Repeat("x", 16*1024) + "]]"
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	document := notes.Parse(body)
	runtime.ReadMemStats(&after)
	if len(document.Errors) == 0 {
		t.Fatal("expected oversized target rejection")
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 50*1024*1024 {
		t.Errorf("rejecting %d-byte note allocated %d bytes before target length validation", len(body), allocated)
	}
}

// TestReviewRawSourcePreserved checks raw source evidence is captured before text sanitization.
func TestReviewRawSourcePreserved(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	source := run.Manifest.Sources[0].ExpectedFile
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "Synthetic review record", "Synthetic <b>review</b> record", 1))
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := workspace.RunPipeline(path, []byte("// synthetic raw-source preservation"), run, false); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.DB.QueryRow("SELECT raw_payload FROM source_records LIMIT 1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `\u003cb\u003e`) && !strings.Contains(raw, "<b>") {
		t.Errorf("raw source payload already lost the original markup: %s", raw)
	}
}

// reviewDB creates a synthetic database using production migrations in an isolated test directory.
func reviewDB(t *testing.T) (*database.Database, string) {
	t.Helper()
	t.Chdir(reviewRoot(t))
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.metadata.db")
	db, err := database.Open(path, filepath.Join(reviewRoot(t), "config/database.something"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

// reviewRun creates a synthetic source and minimal supported pipeline configuration.
func reviewRun(t *testing.T, dir, prefix string) *workspace.Run {
	t.Helper()
	path := filepath.Join(dir, "source.csv")
	data := prefix + "doi,title,year,publisher,authors,cited_references\n10.1000/review,Synthetic review record,2024,Institute of Electrical and Electronics Engineers (IEEE),Example Author,Reference DOI 10.1000/ref\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return &workspace.Run{Manifest: &manifest.ResolvedManifest{
		FormatVersion: 2, SearchID: "backend-review", SearchRevision: "r1", ReusePolicy: "reuse_completed",
		CachePolicy: manifest.CachePolicy{Reads: []string{"global"}, Writes: []string{"active_run"}},
		Sources: []manifest.SourceManifest{{Name: "synthetic", FileType: "csv", ExpectedFile: path, ExpectedResultCount: 1,
			KeepFields: []string{"doi", "title", "year", "publisher", "authors", "cited_references"}}},
	}}
}

// TestReviewStageInputPreserved reproduces F-01 against a real pipeline run.
func TestReviewStageInputPreserved(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	if err := workspace.RunPipeline(path, []byte("// synthetic review configuration"), run, false); err != nil {
		t.Fatal(err)
	}
	var input, output int64
	if err := db.DB.QueryRow("SELECT input_artifact_id, output_artifact_id FROM run_steps WHERE step_name='normalize'").Scan(&input, &output); err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err := db.DB.QueryRow("SELECT publisher FROM work_revisions WHERE producer_stage='validate'").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT publisher FROM work_revisions WHERE producer_stage='normalize'").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("reproduction did not transform publisher")
	}
	for id, publisher := range map[int64]string{input: before, output: after} {
		blob, err := db.ArtifactBlobs.GetByArtifactID(id)
		if err != nil || blob == nil {
			t.Fatalf("stage artifact missing: %v", err)
		}
		var articles []article.Article
		if err := json.Unmarshal(blob.Data, &articles); err != nil || len(articles) != 1 || articles[0].Publisher != publisher {
			t.Fatalf("stage artifact does not preserve publisher: %+v %v", articles, err)
		}
		var hash string
		if err := db.DB.QueryRow("SELECT content_hash FROM artifacts WHERE id=?", id).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(blob.Data)) != hash {
			t.Fatal("artifact hash mismatch")
		}
	}
	if input == output {
		t.Errorf("publisher changed %q -> %q, but normalization input/output both reference artifact %d", before, after, input)
	}
}

// TestReviewNoteSingleColumnTable checks that a valid single-column table is accepted.
func TestReviewNoteSingleColumnTable(t *testing.T) {
	body := "| Status |\n| --- |\n| Good |"
	document := notes.Parse(body)
	for _, problem := range document.Errors {
		if problem.Message == "malformed table" {
			t.Errorf("valid single-column table was rejected as a malformed table: %v", problem)
		}
	}
}

// TestReviewNotePipeParagraphNotTable checks that prose containing pipes is not rejected as a table.
func TestReviewNotePipeParagraphNotTable(t *testing.T) {
	body := "First line with | pipe\nSecond line also | has pipe"
	document := notes.Parse(body)
	for _, problem := range document.Errors {
		if problem.Message == "malformed table" {
			t.Errorf("ordinary paragraph containing pipes was rejected as a malformed table: %v", problem)
		}
	}
}

// TestReviewPriorRunCacheStable reproduces F-02 with a private snapshot and a global refresh.
func TestReviewPriorRunCacheStable(t *testing.T) {
	db, _ := reviewDB(t)
	runID, err := db.PipelineRuns.StartRun("review", "")
	if err != nil {
		t.Fatal(err)
	}
	artifacts := make([]int64, 2)
	for index, title := range []string{"original", "refreshed"} {
		data := []byte(fmt.Sprintf(`{"message":{"title":[%q]}}`, title))
		artifacts[index], err = db.Artifacts.CreateWithBlob(fmt.Sprintf("%x", sha256.Sum256(data)), "application/json", int64(len(data)), runID, data)
		if err != nil {
			t.Fatal(err)
		}
	}
	entry := &database.CacheEntry{Provider: "crossref", Namespace: "work_by_doi", RequestFingerprint: "synthetic-key", ResponseStatus: 200, PayloadArtifactID: &artifacts[0], FetchedAt: "2026-09-13T00:00:00Z"}
	for _, layer := range []string{"active_run", "global"} {
		entry.ExtractorVersion = "workspace-cache-v1"
		if layer == "active_run" {
			entry.ExtractorVersion += fmt.Sprintf(":run:%d", runID)
		}
		id, err := db.CacheEntries.Upsert(entry)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.RunCacheUses.Create(&database.RunCacheUse{PipelineRunID: runID, CacheEntryID: id, CacheLayer: layer, Outcome: "hit"}); err != nil {
			t.Fatal(err)
		}
	}
	entry.PayloadArtifactID = &artifacts[1]
	if _, err := db.CacheEntries.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	replayed, err := db.RunCacheUses.FindAnyEntry(runID, entry.Provider, entry.Namespace, entry.RequestFingerprint, "workspace-cache-v1")
	if err != nil {
		t.Fatal(err)
	}
	if replayed == nil || replayed.PayloadArtifactID == nil {
		t.Fatal("replay missing")
	}
	if *replayed.PayloadArtifactID != artifacts[0] {
		t.Errorf("prior run consumed artifact %d, replay returned refreshed artifact %d despite its private snapshot", artifacts[0], *replayed.PayloadArtifactID)
	}
}

// TestReviewStructuredDOIPreserved checks punctuation in an explicit DOI field.
func TestReviewStructuredDOIPreserved(t *testing.T) {
	for _, doi := range []string{"10.1000/example(1)", "10.1000/example;part", "10.1000/example<part>"} {
		a, err := article.NewFromMap(map[string]string{"doi": doi, "title": "Synthetic", "year": "2024"}, "review")
		if err != nil {
			t.Fatal(err)
		}
		if a.DOI != doi {
			t.Errorf("explicit DOI %q was silently changed to %q", doi, a.DOI)
		}
	}
}

// TestReviewCSVByteOrderMark checks normal UTF-8 BOM handling in the first header.
func TestReviewCSVByteOrderMark(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "\ufeff")
	if err := workspace.RunPipeline(path, []byte("// synthetic BOM review"), run, false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM work_revisions WHERE producer_stage='normalize'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("valid BOM-prefixed CSV completed with %d normalized articles; expected 1", count)
	}
}

// TestReviewOpenAlexPublisherIsNotJournal checks the distinct publisher and journal fields.
func TestReviewOpenAlexPublisherIsNotJournal(t *testing.T) {
	data := []byte(`{"id":"https://openalex.org/W1","primary_location":{"source":{"display_name":"Synthetic Journal","host_organization_name":"Synthetic Publisher"}}}`)
	a, _ := enrich.DecodeOpenAlexResponse(data, "10.1000/review")
	if a.Publisher == "Synthetic Journal" {
		t.Errorf("journal name is incorrectly decoded as publisher: %q", a.Publisher)
	}
}

// TestReviewInterruptedAttemptRecoverable checks whether fresh can recover an abandoned attempt.
func TestReviewInterruptedAttemptRecoverable(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	runID, err := workspace.StartWorkspaceAttempt(db, []byte("// synthetic abandoned attempt"), run, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.StartWorkspaceAttempt(db, []byte("// synthetic abandoned attempt"), run, true); err == nil {
		t.Fatal("fresh must not bypass a running attempt")
	}
	if err := workspace.RecoverAbandonedRun(context.Background(), path, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.StartWorkspaceAttempt(db, []byte("// synthetic abandoned attempt"), run, true); err != nil {
		t.Fatal(err)
	}
	var failed, audits int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM pipeline_runs WHERE id=? AND status='failed' AND finished_at IS NOT NULL", runID).Scan(&failed); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE pipeline_run_id=? AND action='run_failed' AND actor='operator'", runID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if failed != 1 || audits != 1 {
		t.Fatalf("recovery failed=%d audits=%d", failed, audits)
	}

}

// TestReviewCancellationAndActiveRecovery verifies active writers block recovery and cancellation leaves an audited retryable attempt.
func TestReviewCancellationAndActiveRecovery(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	reviewProvider(t, run, `{"message":{"title":["unused"]}}`)
	entered := make(chan struct{})
	http.DefaultTransport = reviewTransport(func(request *http.Request) (*http.Response, error) {
		close(entered)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- workspace.RunPipelineContext(ctx, path, []byte("// cancel regression"), run, false)
	}()
	select {
	case <-entered:
	case err := <-finished:
		t.Fatalf("pipeline stopped before request: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("pipeline never entered provider request")
	}
	var runID int64
	if err := db.DB.QueryRow("SELECT id FROM pipeline_runs WHERE status='running'").Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if err := workspace.RecoverAbandonedRun(context.Background(), path, runID); err == nil || !strings.Contains(err.Error(), "active pipeline") {
		t.Fatalf("active recovery was not refused: %v", err)
	}
	if err := workspace.RunPipeline(path, []byte("// cancel regression"), run, true); err == nil {
		t.Fatal("concurrent pipeline was accepted")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled pipeline did not finish")
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM pipeline_runs WHERE id=? AND status='failed' AND finished_at IS NOT NULL", runID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cancelled attempt not failed: %d %v", count, err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE pipeline_run_id=? AND action='run_failed'", runID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cancellation audit missing: %d %v", count, err)
	}
	http.DefaultTransport = reviewTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":{"title":["Retry"]}}`)), Request: request}, nil
	})
	if err := workspace.RunPipeline(path, []byte("// cancel regression"), run, true); err != nil {
		t.Fatalf("retry after cancellation: %v", err)
	}
}

// TestReviewMissingCSVHeaderFails verifies a missing required mapped column cannot complete an empty corpus.
func TestReviewMissingCSVHeaderFails(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	csvPath := run.Manifest.Sources[0].ExpectedFile
	if err := os.WriteFile(csvPath, []byte("wrong,title,year\n10.1000/example,Synthetic,2024\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := workspace.RunPipeline(path, []byte("// missing header"), run, false); err == nil || !strings.Contains(err.Error(), "required mapped header") {
		t.Fatalf("missing header: %v", err)
	}
	var status string
	if err := db.DB.QueryRow("SELECT status FROM pipeline_runs").Scan(&status); err != nil || status != "failed" {
		t.Fatalf("status=%s err=%v", status, err)
	}
}

// TestReviewProviderIdentityMismatch verifies another work's response is rejected before cache publication.
func TestReviewProviderIdentityMismatch(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	reviewProvider(t, run, `{"message":{"DOI":"10.1000/another","title":["Wrong work"]}}`)
	if err := workspace.RunPipeline(path, []byte("// identity mismatch"), run, false); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched DOI response accepted: %v", err)
	}
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM cache_entries").Scan(&count); err != nil || count != 0 {
		t.Fatalf("wrong work cached: %d %v", count, err)
	}
}

// TestReviewMetadataAndIdentityInputs verifies independent immutable inputs for both enrichment stages.
func TestReviewMetadataAndIdentityInputs(t *testing.T) {
	db, path := reviewDB(t)
	run := reviewRun(t, filepath.Dir(path), "")
	reviewProvider(t, run, "")
	crossref := run.Enrichment.Sources["crossref"]
	crossref.Fields = []string{"authors"}
	run.Enrichment.Sources["crossref"] = crossref
	orcid := crossref
	orcid.Name, orcid.BaseURL, orcid.Fields = "orcid", "https://identity.invalid/", []string{"display_name"}
	run.Enrichment.Sources["orcid"] = orcid
	http.DefaultTransport = reviewTransport(func(request *http.Request) (*http.Response, error) {
		body := `{"message":{"author":[{"family":"Example","ORCID":"https://orcid.org/0000-0001-2345-6789"}]}}`
		if request.URL.Host == "identity.invalid" {
			body = `{"person":{"name":{"given-names":{"value":"Synthetic"},"family-name":{"value":"Example"}}}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	if err := workspace.RunPipeline(path, []byte("// metadata and identity snapshots"), run, false); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"enrich_metadata", "enrich_identity"} {
		var input, output []byte
		var beforeHash, afterHash string
		if err := db.DB.QueryRow(`SELECT before.data, after.data, step.input_fingerprint, step.output_fingerprint FROM run_steps step JOIN artifact_blobs before ON before.artifact_id=step.input_artifact_id JOIN artifact_blobs after ON after.artifact_id=step.output_artifact_id WHERE step.step_name=?`, stage).Scan(&input, &output, &beforeHash, &afterHash); err != nil {
			t.Fatal(err)
		}
		var before, after []article.Article
		if err := json.Unmarshal(input, &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(output, &after); err != nil {
			t.Fatal(err)
		}
		if len(before) != 1 || len(after) != 1 || len(before[0].Authors) != 1 || len(after[0].Authors) != 1 {
			t.Fatal("missing stage authors")
		}
		if stage == "enrich_metadata" && (before[0].Authors[0].Orcid != "" || after[0].Authors[0].Orcid == "") {
			t.Fatal("metadata input was mutated")
		}
		if stage == "enrich_identity" && (before[0].Authors[0].FirstName != "" || after[0].Authors[0].FirstName != "Synthetic") {
			t.Fatal("identity input was mutated")
		}
		if beforeHash == afterHash || fmt.Sprintf("%x", sha256.Sum256(input)) != beforeHash || fmt.Sprintf("%x", sha256.Sum256(output)) != afterHash {
			t.Fatal("stage fingerprint mismatch")
		}
	}
}
