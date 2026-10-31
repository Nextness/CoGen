// store_integration_test.go tests the workspace pipeline store
// functions and repository integration with temporary databases
// and real production migrations.

package workspace_test

import "analysis/tests/support/go/testenv"

import (
	"analysis/backend/workspace/enrichment"
	"analysis/backend/workspace/normalizer"
	"analysis/backend/workspace/pdfsync"
	"analysis/backend/workspace/revisions"
	"analysis/backend/workspace/validator"

	"analysis/backend/article"

	"analysis/backend/database/pdfbinding"
	dbwork "analysis/backend/database/work"
	"analysis/backend/enrich"
	"analysis/backend/manifest"
	"analysis/backend/pdfstore"
	"analysis/tests/support/go/testfixture"
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestApplyConfiguredArticleEnrichmentHonorsFillMissingOnly verifies apply configured article enrichment honors fill missing only.
func TestApplyConfiguredArticleEnrichmentHonorsFillMissingOnly(t *testing.T) {
	for _, tc := range []struct {
		name            string
		fillMissingOnly bool
		wantTitle       string
		wantCitation    int
		wantAuthor      string
	}{
		{
			name: "fill missing fields only", fillMissingOnly: true,
			wantTitle: "export title", wantCitation: 1, wantAuthor: "Export Author",
		},
		{
			name: "refresh configured metadata", fillMissingOnly: false,
			wantTitle: "provider title", wantCitation: 7, wantAuthor: "Provider Author",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			articles := map[string]*article.Article{
				"10.1000/example": {
					DOI:           "10.1000/example",
					Title:         "export title",
					CitationCount: 1,
					Authors:       []article.Author{{CitationName: "Export Author"}},
				},
			}
			result := &enrich.GatherResult{Articles: map[string]*enrich.ArticleEnrichment{
				"10.1000/example": {
					Title:         "provider title",
					Abstract:      "provider abstract",
					CitationCount: 7,
					Authors:       []enrich.EnrichedAuthor{{CitationName: "Provider Author"}},
				},
			}}

			if updated, _ := enrichment.ApplyArticles(articles, enrich.SourceConfig{FillMissingOnly: tc.fillMissingOnly}, result); updated != 1 {
				t.Fatalf("updated = %d, want 1", updated)
			}
			got := articles["10.1000/example"]
			if got.Title != tc.wantTitle || got.CitationCount != tc.wantCitation || got.Authors[0].CitationName != tc.wantAuthor {
				t.Fatalf("article = %+v, want title=%q citation=%d author=%q", got, tc.wantTitle, tc.wantCitation, tc.wantAuthor)
			}
			if got.Abstract != "provider abstract" {
				t.Fatalf("abstract = %q, want provider abstract", got.Abstract)
			}
		})
	}
}

// TestSyncNormalizedPDFInventoryRegistersOnceAndFlushesAudit verifies sync normalized pdf inventory registers once and flushes audit.
func TestSyncNormalizedPDFInventoryRegistersOnceAndFlushesAudit(t *testing.T) {
	ctx := context.Background()
	registry := testenv.Path("config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	db, err := testfixture.Open(dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(context.Background(), db.Conn, "inventory-sync", "")
	if err != nil {
		t.Fatal(err)
	}
	for index, doi := range []string{"10.1000/normalized-one", "10.1000/normalized-two"} {
		workID, err := db.Work.CreateWorkByDOI(context.Background(), doi)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Work.CreateRevision(context.Background(), &dbwork.Revision{
			WorkID: workID, PipelineRunID: runID,
			ProducerStage: dbwork.ProducerStageNormalize,
			Title:         "Normalized article " + strconv.Itoa(index+1),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.PDFBindings.EnsureBinding(ctx, pdfbinding.DefaultStoreFilename); err != nil {
		t.Fatal(err)
	}
	prepared, err := pdfstore.Open(filepath.Join(tempDir, pdfbinding.DefaultStoreFilename), registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}

	registered, flushed, err := pdfsync.Sync(ctx, db.Database, dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if registered != 2 || flushed != 2 {
		t.Fatalf("first inventory sync registered=%d flushed=%d, want 2 and 2", registered, flushed)
	}
	registered, flushed, err = pdfsync.Sync(ctx, db.Database, dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if registered != 0 || flushed != 0 {
		t.Fatalf("second inventory sync registered=%d flushed=%d, want 0 and 0", registered, flushed)
	}
	store, err := pdfstore.Open(filepath.Join(tempDir, pdfbinding.DefaultStoreFilename), registry)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var documents int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents WHERE status='not_available'").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if documents != 2 {
		t.Fatalf("not-available inventory documents = %d, want 2", documents)
	}
	var events int
	if err := db.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action='pdf_inventory_registered' AND pipeline_run_id=?", runID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Fatalf("inventory registration audit events = %d, want 2", events)
	}
}

// TestNormalizeWorkspaceArticlesRecordsExplicitFieldOutcomes verifies normalize workspace articles records explicit field outcomes.
func TestNormalizeWorkspaceArticlesRecordsExplicitFieldOutcomes(t *testing.T) {
	articles := []*article.Article{{
		DOI:       "10.1000/normalization",
		Publisher: "Elsevier",
		Journal:   "nature",
		Authors: []article.Author{{
			CitationName: "Ada Lovelace",
			Affiliation:  "",
		}},
	}}

	results := normalizer.Normalize(articles)
	if len(results) != 4 {
		t.Fatalf("normalization results = %d, want 4", len(results))
	}
	got := map[string]string{}
	for _, result := range results {
		got[result.Field] = result.Outcome
	}
	for field, want := range map[string]string{
		"publisher":   "already_canonical",
		"journal":     "changed",
		"author_name": "changed",
		"affiliation": "unavailable",
	} {
		if got[field] != want {
			t.Errorf("%s outcome = %q, want %q", field, got[field], want)
		}
	}

	if articles[0].Publisher != "Elsevier" || articles[0].Authors[0].NormalizedName == "" {
		t.Fatalf("normalized article = %+v", articles[0])
	}
	var extension revisionExtension
	if err := json.Unmarshal([]byte(persistedExtension(t, articles[0], nil)), &extension); err != nil {
		t.Fatal(err)
	}
	if extension.NormalizedJournal != "Nature" {
		t.Fatalf("normalized journal extension = %q, want Nature", extension.NormalizedJournal)
	}

	db, err := testfixture.Open(filepath.Join(t.TempDir(), "workspace.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(context.Background(), db.Conn, "normalization metrics", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := normalizer.RecordMetrics(context.Background(), db.Database, runID, 1, results); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		metric string
		source string
		want   int
	}{
		{"normalized_articles_processed", "", 1},
		{"normalization_fields_processed", "", 4},
		{"normalization_fields_changed", "", 2},
		{"normalization_fields_already_canonical", "", 1},
		{"normalization_fields_unavailable", "", 1},
		{"normalization_fields_processed", "journal", 1},
		{"normalization_fields_changed", "journal", 1},
		{"normalization_fields_already_canonical", "publisher", 1},
		{"normalization_fields_unavailable", "affiliation", 1},
	} {
		metric, err := db.Run.GetMetric(context.Background(), runID, check.metric, check.source)
		if err != nil || metric == nil || metric.Value != check.want {
			t.Errorf("metric %s/%s = %+v, %v; want %d", check.metric, check.source, metric, err, check.want)
		}
	}
	auditEvents, err := db.Audit.ListByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(auditEvents) != 0 {
		t.Fatalf("normalization metrics wrote audit events: %+v", auditEvents)
	}
}

// TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName verifies persist workspace stage keeps authors when enrichment has no citation name.
func TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName(t *testing.T) {
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "workspace.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(context.Background(), db.Conn, "workspace test", "")
	if err != nil {
		t.Fatal(err)
	}
	articleByDOI := map[string]*article.Article{
		"10.1000/example": {
			DOI: "10.1000/example", Title: "export title", Year: 2024,
			Authors: []article.Author{{CitationName: "Export Author"}},
		},
	}
	result := &enrich.GatherResult{Articles: map[string]*enrich.ArticleEnrichment{
		"10.1000/example": {
			Title:   "provider title",
			Authors: []enrich.EnrichedAuthor{{ORCID: "0000-0002-8765-4321"}},
		},
	}}
	enrichment.ApplyArticles(articleByDOI, enrich.SourceConfig{FillMissingOnly: false}, result)
	got := articleByDOI["10.1000/example"]
	if len(got.Authors) != 1 || got.Authors[0].CitationName != "Export Author" {
		t.Fatalf("authors after malformed enrichment = %+v", got.Authors)
	}
	if _, _, err := revisions.Persist(context.Background(), db.Database, runID, []*article.Article{got}, dbwork.ProducerStageEnrich, dbwork.StageNameEnrich, dbwork.OutcomeEnriched, nil); err != nil {
		t.Fatalf("persist enriched work: %v", err)
	}
}

// TestApplyArticleEnrichmentReturnsFieldChanges verifies apply article enrichment returns field changes.
func TestApplyArticleEnrichmentReturnsFieldChanges(t *testing.T) {
	articles := map[string]*article.Article{
		"10.1000/one": {
			DOI:           "10.1000/one",
			Title:         "original title",
			Abstract:      "",
			Publisher:     "",
			CitationCount: 0,
		},
	}
	result := &enrich.GatherResult{
		Source: "crossref",
		Articles: map[string]*enrich.ArticleEnrichment{
			"10.1000/one": {
				Title:         "enriched title",
				Abstract:      "new abstract",
				Publisher:     "Test Publisher",
				CitationCount: 42,
			},
		},
	}
	updated, changes := applyArticleResults(articles, result)
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}
	if len(changes) != 4 {
		t.Fatalf("changes = %d, want 4 (title, abstract, publisher, citation_count)", len(changes))
	}
	expected := []struct{ field, provider string }{
		{"title", "crossref"},
		{"abstract", "crossref"},
		{"publisher", "crossref"},
		{"citation_count", "crossref"},
	}
	for i, exp := range expected {
		if changes[i].DOI != "10.1000/one" || changes[i].Field != exp.field || changes[i].Provider != exp.provider {
			t.Fatalf("change[%d] = %+v, want DOI=10.1000/one field=%q provider=%q", i, changes[i], exp.field, exp.provider)
		}
	}
}

// TestApplyArticleEnrichmentReturnsNoChangesWhenNoEnrichment verifies apply article enrichment returns no changes when no enrichment.
func TestApplyArticleEnrichmentReturnsNoChangesWhenNoEnrichment(t *testing.T) {
	articles := map[string]*article.Article{
		"10.1000/one": {DOI: "10.1000/one", Title: "title"},
	}
	updated, changes := applyArticleResults(articles, nil)
	if updated != 0 || len(changes) != 0 {
		t.Fatalf("nil result: updated=%d changes=%d, want 0", updated, len(changes))
	}
	updated, changes = applyArticleResults(articles, &enrich.GatherResult{Articles: nil})
	if updated != 0 || len(changes) != 0 {
		t.Fatalf("nil articles: updated=%d changes=%d, want 0", updated, len(changes))
	}
}

// TestApplyAuthorProfileRecordsOnlyObservedFieldChanges verifies apply author profile records only observed field changes.
func TestApplyAuthorProfileRecordsOnlyObservedFieldChanges(t *testing.T) {
	author := &article.Author{}
	profile := &enrich.EnrichedAuthor{
		ORCID:        "0000-0002-1825-0097",
		FirstName:    "Ada",
		LastName:     "Lovelace",
		CitationName: "Ada Lovelace",
		Institution:  "Analytical Society",
	}
	provenance := map[*article.Author]enrichment.AuthorOrigin{author: {DOI: "10.1000/profile", Index: 2}}
	changed, changes := enrichment.ApplyProfile([]*article.Author{author}, profile, profile.ORCID, provenance)
	if !changed {
		t.Fatal("applyAuthorProfile reported no change for an empty author")
	}
	if author.Orcid != profile.ORCID || author.FirstName != profile.FirstName || author.LastName != profile.LastName || author.CitationName != profile.CitationName || author.Affiliation != profile.Institution {
		t.Fatalf("author after profile = %+v", author)
	}
	if len(changes) != 5 {
		t.Fatalf("field changes = %+v, want five populated fields", changes)
	}
	for _, change := range changes {
		if change.DOI != "10.1000/profile" || change.Provider != "orcid" {
			t.Fatalf("field change = %+v", change)
		}
	}

	changed, changes = enrichment.ApplyProfile([]*article.Author{author}, profile, profile.ORCID, provenance)
	if changed {
		t.Fatal("applyAuthorProfile changed an already populated author")
	}
	if len(changes) != 0 {
		t.Fatalf("populated author produced field changes = %+v, want none", changes)
	}

	prepopulated := &article.Author{Orcid: "different", FirstName: "Existing", LastName: "Name", CitationName: "Existing Name", Affiliation: "Existing institution"}
	changed, changes = enrichment.ApplyProfile([]*article.Author{prepopulated}, profile, profile.ORCID, map[*article.Author]enrichment.AuthorOrigin{prepopulated: {DOI: "10.1000/prepopulated", Index: 0}})
	if changed || len(changes) != 0 {
		t.Fatalf("prepopulated author produced false changes: changed=%t changes=%+v", changed, changes)
	}
}

// TestEmitFieldEnrichedAuditEvents verifies emit field enriched audit events.
func TestEmitFieldEnrichedAuditEvents(t *testing.T) {
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "workspace.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(context.Background(), db.Conn, "test emit", "")
	if err != nil {
		t.Fatal(err)
	}
	revisionIDs := map[string]int64{"10.1000/one": 42}
	changes := []enrichment.Change{
		{DOI: "10.1000/one", Field: "title", Provider: "crossref"},
		{DOI: "10.1000/one", Field: "abstract", Provider: "crossref"},
	}
	if err := enrichment.RecordAudit(context.Background(), db.Database, runID, revisionIDs, changes); err != nil {
		t.Fatal(err)
	}
	events, err := db.Audit.ListByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	for i, event := range events {
		if event.Action != string(manifest.AuditFieldEnriched) {
			t.Fatalf("event[%d] action = %q, want %q", i, event.Action, manifest.AuditFieldEnriched)
		}
		if event.EntityType != "work_revision" || event.EntityID != "42" {
			t.Fatalf("event[%d] entity = %s/%s, want work_revision/42", i, event.EntityType, event.EntityID)
		}
		if event.Actor != "crossref" {
			t.Fatalf("event[%d] actor = %q, want crossref", i, event.Actor)
		}
		var meta map[string]string
		if err := json.Unmarshal([]byte(event.MetadataJSON), &meta); err != nil {
			t.Fatalf("event[%d] metadata unmarshal: %v", i, err)
		}
		if meta["provider"] != "crossref" {
			t.Fatalf("event[%d] metadata provider = %q", i, meta["provider"])
		}
	}
	if events[0].CorrelationID != "enrich-1-42-title" || events[1].CorrelationID != "enrich-1-42-abstract" {
		t.Fatalf("correlation IDs: %q, %q", events[0].CorrelationID, events[1].CorrelationID)
	}
}

// TestRecordValidationAuditPreservesStateAndReasons verifies the validation
// workflow records standalone audit evidence with the current actor, state,
// reason metadata, run linkage, and correlation identity.
func TestRecordValidationAuditPreservesStateAndReasons(t *testing.T) {
	ctx := context.Background()
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "workspace.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(ctx, db.Conn, "validation-audit", "")
	if err != nil {
		t.Fatal(err)
	}
	validWorkID, err := db.Work.CreateWorkByDOI(context.Background(), "10.1000/valid")
	if err != nil {
		t.Fatal(err)
	}
	discardedWorkID, err := db.Work.CreateWorkByDOI(context.Background(), "10.1000/discarded")
	if err != nil {
		t.Fatal(err)
	}
	articles := []*article.Article{{DOI: "10.1000/valid"}, {DOI: "10.1000/discarded"}}
	reasons := map[string][]string{"10.1000/discarded": {"missing title", "invalid year"}}
	if err := validator.RecordAudit(ctx, db.Database, runID, articles, reasons); err != nil {
		t.Fatal(err)
	}
	events, err := db.Audit.ListByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("validation audit events = %d, want 2", len(events))
	}
	valid, discarded := events[0], events[1]
	if valid.EntityType != "work" || valid.EntityID != strconv.FormatInt(validWorkID, 10) ||
		valid.Actor != "pipeline" || valid.Action != string(manifest.AuditValidationChanged) {
		t.Fatalf("valid event identity = %+v", valid)
	}
	if valid.AfterJSON != `{"status":"valid"}` || valid.MetadataJSON != "{}" {
		t.Fatalf("valid event state = after %q metadata %q", valid.AfterJSON, valid.MetadataJSON)
	}
	if discarded.EntityID != strconv.FormatInt(discardedWorkID, 10) ||
		discarded.AfterJSON != `{"status":"discarded"}` ||
		discarded.MetadataJSON != `{"reasons":["missing title","invalid year"]}` {
		t.Fatalf("discarded event = %+v", discarded)
	}
	for index, event := range events {
		if event.PipelineRunID == nil || *event.PipelineRunID != runID {
			t.Fatalf("event[%d] run linkage = %v, want %d", index, event.PipelineRunID, runID)
		}
		if event.CorrelationID != "validation-"+strconv.FormatInt(runID, 10) {
			t.Fatalf("event[%d] correlation = %q", index, event.CorrelationID)
		}
	}

	missing := []*article.Article{{DOI: "10.1000/missing"}}
	if err := validator.RecordAudit(ctx, db.Database, runID, missing, nil); err == nil || !strings.Contains(err.Error(), "work missing for validation audit") {
		t.Fatalf("missing work error = %v", err)
	}
	after, err := db.Audit.ListByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 {
		t.Fatalf("missing work wrote %d additional events", len(after)-2)
	}
}

// TestRecordFieldEnrichmentMetrics verifies record field enrichment metrics.
func TestRecordFieldEnrichmentMetrics(t *testing.T) {
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "workspace.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(context.Background(), db.Conn, "test metrics", "")
	if err != nil {
		t.Fatal(err)
	}
	changes := []enrichment.Change{
		{DOI: "10.1000/one", Field: "title", Provider: "crossref"},
		{DOI: "10.1000/one", Field: "abstract", Provider: "crossref"},
		{DOI: "10.1000/two", Field: "authors", Provider: "openalex"},
	}
	if err := enrichment.RecordMetrics(context.Background(), db.Database, runID, changes); err != nil {
		t.Fatal(err)
	}
	checkMetric := func(name, source string, want int) {
		metric, err := db.Run.GetMetric(context.Background(), runID, name, source)
		if err != nil || metric == nil || metric.Value != want {
			t.Fatalf("metric %s/%s = %+v, %v; want %d", name, source, metric, err, want)
		}
	}
	checkMetric("enriched_fields_total", "", 3)
	checkMetric("enriched_fields_title", "", 1)
	checkMetric("enriched_fields_abstract", "", 1)
	checkMetric("enriched_fields_authors", "", 1)
	checkMetric("enriched_fields", "crossref", 2)
	checkMetric("enriched_fields", "openalex", 1)
}

// TestEmitFieldEnrichedSkipsUnknownDOI verifies emit field enriched skips unknown doi.
func TestEmitFieldEnrichedSkipsUnknownDOI(t *testing.T) {
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "workspace.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(context.Background(), db.Conn, "test skip", "")
	if err != nil {
		t.Fatal(err)
	}
	revisionIDs := map[string]int64{"10.1000/known": 1}
	changes := []enrichment.Change{
		{DOI: "10.1000/known", Field: "title", Provider: "crossref"},
		{DOI: "10.1000/unknown", Field: "abstract", Provider: "crossref"},
	}
	if err := enrichment.RecordAudit(context.Background(), db.Database, runID, revisionIDs, changes); err != nil {
		t.Fatal(err)
	}
	events, err := db.Audit.ListByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1 (unknown DOI should be skipped)", len(events))
	}
	if !strings.Contains(events[0].MetadataJSON, `"field":"title"`) {
		t.Fatalf("event metadata = %q, want title field", events[0].MetadataJSON)
	}
}

// applyArticleResults supplies the existing result policy to the public configured enrichment operation.
func applyArticleResults(articles map[string]*article.Article, result *enrich.GatherResult) (int, []enrichment.Change) {
	source := enrich.SourceConfig{}
	if result != nil {
		source.FillMissingOnly = result.FillMissingOnly
	}
	return enrichment.ApplyArticles(articles, source, result)
}

// revisionExtension decodes the persisted revision evidence contract.
type revisionExtension struct {
	NormalizedJournal string            `json:"normalized_journal"`
	ValidationReasons []string          `json:"validation_reasons"`
	NormalizedAuthors map[string]string `json:"normalized_authors"`
}

// persistedExtension reads extension data produced by real immutable revision persistence.
func persistedExtension(t *testing.T, entry *article.Article, reasons []string) string {
	t.Helper()
	db, err := testfixture.Open(filepath.Join(t.TempDir(), "extension.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := testfixture.Run(t.Context(), db.Conn, "extension", "")
	if err != nil {
		t.Fatal(err)
	}
	_, ids, err := revisions.Persist(t.Context(), db.Database, runID, []*article.Article{entry}, dbwork.ProducerStageNormalize, dbwork.StageNameNormalize, dbwork.OutcomeNormalized, map[string][]string{entry.DOI: reasons})
	if err != nil {
		t.Fatal(err)
	}
	var extension string
	if err := db.Conn.QueryRow("SELECT extension_data FROM work_revisions WHERE id=?", ids[entry.DOI]).Scan(&extension); err != nil {
		t.Fatal(err)
	}
	return extension
}

// persistTestSnapshot prepares the revision identity needed by identity-search evidence assertions.
func persistTestSnapshot(t *testing.T, db *testfixture.Database, runID int64, entry *article.Article, producer, extension string) (int64, int64, error) {
	t.Helper()
	works, revisionsByDOI, err := revisions.Persist(t.Context(), db.Database, runID, []*article.Article{entry}, producer, dbwork.StageNameEnrich, dbwork.OutcomeEnriched, nil)
	return works[entry.DOI], revisionsByDOI[entry.DOI], err
}
