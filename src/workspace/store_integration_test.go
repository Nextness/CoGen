// store_integration_test.go tests the workspace pipeline store
// functions and repository integration with temporary databases
// and real production migrations.
//go:build integration

package workspace

import (
	"analysis/article"
	"analysis/database"
	"analysis/enrich"
	"analysis/manifest"
	"analysis/pdfstore"
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

			if updated, _ := applyConfiguredArticleEnrichment(articles, enrich.SourceConfig{FillMissingOnly: tc.fillMissingOnly}, result); updated != 1 {
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
	registry := filepath.Join("..", "..", "config", "database.something")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corpus.metadata.db")
	db, err := database.Open(dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("inventory-sync", "")
	if err != nil {
		t.Fatal(err)
	}
	for index, doi := range []string{"10.1000/normalized-one", "10.1000/normalized-two"} {
		workID, err := db.Works.CreateByDOI(doi)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.WorkRevisions.Create(&database.WorkRevision{
			WorkID: workID, PipelineRunID: runID,
			ProducerStage: database.ProducerStageNormalize,
			Title:         "Normalized article " + strconv.Itoa(index+1),
		}); err != nil {
			t.Fatal(err)
		}
	}

	registered, flushed, err := syncNormalizedPDFInventory(ctx, db, dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if registered != 2 || flushed != 2 {
		t.Fatalf("first inventory sync registered=%d flushed=%d, want 2 and 2", registered, flushed)
	}
	registered, flushed, err = syncNormalizedPDFInventory(ctx, db, dbPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if registered != 0 || flushed != 0 {
		t.Fatalf("second inventory sync registered=%d flushed=%d, want 0 and 0", registered, flushed)
	}
	store, err := pdfstore.Open(filepath.Join(tempDir, pdfstore.DefaultStoreFilename), registry)
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
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action='pdf_inventory_registered' AND pipeline_run_id=?", runID).Scan(&events); err != nil {
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

	results := normalizeWorkspaceArticles(articles)
	if len(results) != 4 {
		t.Fatalf("normalization results = %d, want 4", len(results))
	}
	got := map[string]string{}
	for _, result := range results {
		got[result.Field] = result.Outcome
	}
	for field, want := range map[string]string{
		"publisher":   normalizationOutcomeAlreadyCanonical,
		"journal":     normalizationOutcomeChanged,
		"author_name": normalizationOutcomeChanged,
		"affiliation": normalizationOutcomeUnavailable,
	} {
		if got[field] != want {
			t.Errorf("%s outcome = %q, want %q", field, got[field], want)
		}
	}

	if articles[0].Publisher != "Elsevier" || articles[0].Authors[0].NormalizedName == "" {
		t.Fatalf("normalized article = %+v", articles[0])
	}
	var extension workspaceRevisionExtension
	if err := json.Unmarshal([]byte(workspaceExtension(articles[0], nil)), &extension); err != nil {
		t.Fatal(err)
	}
	if extension.NormalizedJournal != "Nature" {
		t.Fatalf("normalized journal extension = %q, want Nature", extension.NormalizedJournal)
	}

	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("normalization metrics", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := recordNormalizationMetrics(db, runID, 1, results); err != nil {
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
		metric, err := db.Metrics.Get(runID, check.metric, check.source)
		if err != nil || metric == nil || metric.Value != check.want {
			t.Errorf("metric %s/%s = %+v, %v; want %d", check.metric, check.source, metric, err, check.want)
		}
	}
	auditEvents, err := db.AuditEvents.ListByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(auditEvents) != 0 {
		t.Fatalf("normalization metrics wrote audit events: %+v", auditEvents)
	}
}

// TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName verifies persist workspace stage keeps authors when enrichment has no citation name.
func TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("workspace test", "")
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
	applyConfiguredArticleEnrichment(articleByDOI, enrich.SourceConfig{FillMissingOnly: false}, result)
	got := articleByDOI["10.1000/example"]
	if len(got.Authors) != 1 || got.Authors[0].CitationName != "Export Author" {
		t.Fatalf("authors after malformed enrichment = %+v", got.Authors)
	}
	if _, _, err := persistWorkspaceStage(db, runID, []*article.Article{got}, database.ProducerStageEnrich, database.StageNameEnrich, database.OutcomeEnriched, nil); err != nil {
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
	updated, changes := applyArticleEnrichment(articles, result)
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
	updated, changes := applyArticleEnrichment(articles, nil)
	if updated != 0 || len(changes) != 0 {
		t.Fatalf("nil result: updated=%d changes=%d, want 0", updated, len(changes))
	}
	updated, changes = applyArticleEnrichment(articles, &enrich.GatherResult{Articles: nil})
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
	if !applyAuthorProfile([]*article.Author{author}, profile, profile.ORCID) {
		t.Fatal("applyAuthorProfile reported no change for an empty author")
	}
	if author.Orcid != profile.ORCID || author.FirstName != profile.FirstName || author.LastName != profile.LastName || author.CitationName != profile.CitationName || author.Affiliation != profile.Institution {
		t.Fatalf("author after profile = %+v", author)
	}
	changes := recordAuthorFieldChanges([]*article.Author{author}, profile, map[*article.Author]string{author: "10.1000/profile"})
	if len(changes) != 5 {
		t.Fatalf("field changes = %+v, want five populated fields", changes)
	}
	for _, change := range changes {
		if change.DOI != "10.1000/profile" || change.Provider != "orcid" {
			t.Fatalf("field change = %+v", change)
		}
	}

	if applyAuthorProfile([]*article.Author{author}, profile, profile.ORCID) {
		t.Fatal("applyAuthorProfile changed an already populated author")
	}
	if changes := recordAuthorFieldChanges([]*article.Author{author}, profile, map[*article.Author]string{}); len(changes) != 0 {
		t.Fatalf("field changes without article provenance = %+v, want none", changes)
	}
}

// TestEmitFieldEnrichedAuditEvents verifies emit field enriched audit events.
func TestEmitFieldEnrichedAuditEvents(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("test emit", "")
	if err != nil {
		t.Fatal(err)
	}
	revisionIDs := map[string]int64{"10.1000/one": 42}
	changes := []fieldChange{
		{DOI: "10.1000/one", Field: "title", Provider: "crossref"},
		{DOI: "10.1000/one", Field: "abstract", Provider: "crossref"},
	}
	if err := emitFieldEnrichedAuditEvents(db, runID, revisionIDs, changes); err != nil {
		t.Fatal(err)
	}
	events, err := db.AuditEvents.ListByRun(runID)
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

// TestRecordFieldEnrichmentMetrics verifies record field enrichment metrics.
func TestRecordFieldEnrichmentMetrics(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("test metrics", "")
	if err != nil {
		t.Fatal(err)
	}
	changes := []fieldChange{
		{DOI: "10.1000/one", Field: "title", Provider: "crossref"},
		{DOI: "10.1000/one", Field: "abstract", Provider: "crossref"},
		{DOI: "10.1000/two", Field: "authors", Provider: "openalex"},
	}
	if err := recordFieldEnrichmentMetrics(db, runID, changes); err != nil {
		t.Fatal(err)
	}
	checkMetric := func(name, source string, want int) {
		metric, err := db.Metrics.Get(runID, name, source)
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
	db, err := database.Open(filepath.Join(t.TempDir(), "workspace.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("test skip", "")
	if err != nil {
		t.Fatal(err)
	}
	revisionIDs := map[string]int64{"10.1000/known": 1}
	changes := []fieldChange{
		{DOI: "10.1000/known", Field: "title", Provider: "crossref"},
		{DOI: "10.1000/unknown", Field: "abstract", Provider: "crossref"},
	}
	if err := emitFieldEnrichedAuditEvents(db, runID, revisionIDs, changes); err != nil {
		t.Fatal(err)
	}
	events, err := db.AuditEvents.ListByRun(runID)
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
