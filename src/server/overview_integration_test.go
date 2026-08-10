// overview_integration_test.go tests the overview endpoint and frontend contract.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"analysis/database"
)

// TestOverviewReportsUnavailableMetricsAndFrontendContract verifies overview reports unavailable metrics and frontend contract.
func TestOverviewReportsUnavailableMetricsAndFrontendContract(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	response := viewerRequest(t, viewer.Handler(), "/api/overview?run_id="+stringID(runID))
	if response.Code != http.StatusOK {
		t.Fatalf("overview status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		CapturedMetrics []struct {
			Metric    string `json:"metric"`
			Available bool   `json:"available"`
		} `json:"captured_metrics"`
		RetentionFunnel map[string]struct {
			Available bool `json:"available"`
		} `json:"retention_funnel"`
		EnrichmentFieldBreakdown    map[string]any `json:"enrichment_field_breakdown"`
		EnrichmentProviderBreakdown map[string]any `json:"enrichment_provider_breakdown"`
		NormalizationFieldBreakdown map[string]map[string]struct {
			Available   bool  `json:"available"`
			Value       int64 `json:"value"`
			Denominator int64 `json:"denominator"`
		} `json:"normalization_field_breakdown"`
		SourceResultCounts []struct {
			SourceName string `json:"source_name"`
			Expected   int    `json:"expected_result_count"`
			Observed   int    `json:"observed_result_count"`
			Comparison string `json:"result_count_comparison"`
		} `json:"source_result_counts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	var parsedUnavailable bool
	for _, metric := range payload.CapturedMetrics {
		if metric.Metric == "parsed_articles" && !metric.Available {
			parsedUnavailable = true
		}
	}
	if !parsedUnavailable || payload.RetentionFunnel["parsed_articles"].Available {
		t.Fatalf("missing run metrics must be explicitly unavailable: %#v", payload)
	}
	if payload.EnrichmentFieldBreakdown == nil {
		t.Fatal("enrichment_field_breakdown is missing from overview response")
	}
	if payload.EnrichmentProviderBreakdown == nil {
		t.Fatal("enrichment_provider_breakdown is missing from overview response")
	}
	if _, ok := payload.EnrichmentFieldBreakdown["title"]; !ok {
		t.Fatal("enrichment_field_breakdown missing 'title' field")
	}
	if _, ok := payload.EnrichmentFieldBreakdown["abstract"]; !ok {
		t.Fatal("enrichment_field_breakdown missing 'abstract' field")
	}
	if _, ok := payload.EnrichmentProviderBreakdown["crossref"]; !ok {
		t.Fatal("enrichment_provider_breakdown missing 'crossref' provider")
	}
	if journal := payload.NormalizationFieldBreakdown["journal"]; journal["changed"].Value != 1 || journal["changed"].Denominator != 1 {
		t.Fatalf("normalization journal breakdown = %#v", journal)
	}
	if len(payload.SourceResultCounts) != 1 || payload.SourceResultCounts[0].SourceName != "fixture" || payload.SourceResultCounts[0].Expected != 4 || payload.SourceResultCounts[0].Observed != 1 || payload.SourceResultCounts[0].Comparison != "below" {
		t.Fatalf("source result count summary = %#v", payload.SourceResultCounts)
	}
	viewer.AssetsFS = os.DirFS(filepath.Join("..", "..", "frontend", "src"))
	handler := viewer.Handler()
	for _, check := range []struct {
		path, needle string
	}{
		{"/views/overview.ts", "/api/overview"},
		{"/views/overview.ts", "Enriched fields"},
		{"/views/overview.ts", "Enrichment by provider"},
		{"/views/overview.ts", "Normalization field outcomes"},
		{"/views/relationships.ts", "/api/graph"},
		{"/views/relationships.ts", "graph-form"},
		{"/router.ts", "pushState"},
		{"/views/overview.ts", "Not recorded for this run"},
		{"/state.ts", "Source export counts"},
		{"/views/corpus.ts", "source_result_counts"},
	} {
		asset := viewerRequest(t, handler, check.path)
		if !strings.Contains(asset.Body.String(), check.needle) {
			t.Fatalf("frontend %s is missing %q", check.path, check.needle)
		}
	}
}

// TestOverviewSupportsPreResultCountRunSources verifies overview supports pre result count run sources.
func TestOverviewSupportsPreResultCountRunSources(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	db, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"result_count_comparison", "observed_result_count", "expected_result_count"} {
		if _, err := db.DB.Exec("ALTER TABLE run_sources DROP COLUMN " + column); err != nil {
			db.Close()
			t.Fatalf("drop %s: %v", column, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	response := viewerRequest(t, viewer.Handler(), "/api/overview?run_id="+stringID(runID))
	if response.Code != http.StatusOK {
		t.Fatalf("overview status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		SourceResultCounts []map[string]any `json:"source_result_counts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.SourceResultCounts) != 1 || payload.SourceResultCounts[0]["expected_result_count"] != nil || payload.SourceResultCounts[0]["observed_result_count"] != nil || payload.SourceResultCounts[0]["result_count_comparison"] != nil {
		t.Fatalf("legacy source count summary = %#v, want unavailable values", payload.SourceResultCounts)
	}
}
