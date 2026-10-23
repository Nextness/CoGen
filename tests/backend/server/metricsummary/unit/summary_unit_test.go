// summary_unit_test.go verifies metric availability, population denominators, percentages, and presentation filtering.
package metricsummary_test

import (
	"analysis/backend/server/metricsummary"
	"math"
	"testing"
)

// TestHelper_metricGroup verifies helper metric group.
func TestHelper_metricGroup(t *testing.T) {
	metrics := map[string]map[string]any{
		"input_records":   {"available": true, "value": int64(10)},
		"parsed_articles": {"available": true, "value": int64(20)},
	}
	result := summaryNamed(metrics)["retention_funnel"].(map[string]any)
	if v := result["input_records"].(map[string]any)["value"]; v != int64(10) {
		t.Errorf("alpha value = %v, want 10", v)
	}
	if v := result["parsed_articles"].(map[string]any)["value"]; v != int64(20) {
		t.Errorf("beta value = %v, want 20", v)
	}
	gamma := result["valid_articles"].(map[string]any)
	if gamma["available"] != false {
		t.Errorf("gamma.available = %v, want false", gamma["available"])
	}
	if _, ok := result["delta"]; ok {
		t.Error("unexpected key delta in result")
	}
	if len(result) != 5 {
		t.Errorf("result has %d keys, want 5", len(result))
	}
}

// TestHelper_sourceBreakdown verifies helper source breakdown.
func TestHelper_sourceBreakdown(t *testing.T) {
	metrics := []map[string]any{
		{"source": "scopus", "metric": "input_records", "value": int64(100)},
		{"source": "pubmed", "metric": "input_records", "value": int64(50)},
		{"source": "", "metric": "input_records", "value": int64(200)},
		{"source": "scopus", "metric": "another_metric", "value": int64(999)},
	}
	result := metricsummary.Build(metrics)["source_breakdown"].(map[string]any)
	scopus := result["scopus"].(map[string]any)
	if scopus["value"] != int64(100) {
		t.Errorf("scopus value = %v, want 100", scopus["value"])
	}
	if scopus["percentage"].(float64) != 50.0 {
		t.Errorf("scopus percentage = %v, want 50.0", scopus["percentage"])
	}
	pubmed := result["pubmed"].(map[string]any)
	if pubmed["value"] != int64(50) {
		t.Errorf("pubmed value = %v, want 50", pubmed["value"])
	}
	if _, ok := result[""]; ok {
		t.Error("empty source should be excluded")
	}
	if result["unknown"] != nil {
		t.Error("unknown source should be absent")
	}
}

// TestHelper_sourceBreakdown_noDenominator verifies helper source breakdown no denominator.
func TestHelper_sourceBreakdown_noDenominator(t *testing.T) {
	metrics := []map[string]any{
		{"source": "scopus", "metric": "input_records", "value": int64(100)},
	}
	result := metricsummary.Build(metrics)["source_breakdown"].(map[string]any)
	scopus := result["scopus"].(map[string]any)
	if _, ok := scopus["denominator"]; ok {
		t.Error("denominator should be absent when totals is 0")
	}
	if _, ok := scopus["percentage"]; ok {
		t.Error("percentage should be absent when denominator is 0")
	}
}

// TestHelper_enrichmentFieldBreakdown verifies helper enrichment field breakdown.
func TestHelper_enrichmentFieldBreakdown(t *testing.T) {
	byName := map[string]map[string]any{
		"enriched_fields_title":       {"available": true, "value": int64(10)},
		"enriched_fields_abstract":    {"available": true, "value": int64(5)},
		"enriched_fields_total":       {"available": true, "value": int64(15)},
		"unrelated_metric":            {"available": true, "value": int64(99)},
		"enriched_fields_author_name": {"available": true, "value": int64(8)},
	}
	result := summaryNamed(byName)["enrichment_field_breakdown"].(map[string]any)
	if v := result["title"].(map[string]any)["value"]; v != int64(10) {
		t.Errorf("title value = %v, want 10", v)
	}
	if v := result["abstract"].(map[string]any)["value"]; v != int64(5) {
		t.Errorf("abstract value = %v, want 5", v)
	}
	if v := result["author_name"].(map[string]any)["value"]; v != int64(8) {
		t.Errorf("author_name value = %v, want 8", v)
	}
	if _, ok := result["total"]; ok {
		t.Error("total should be excluded")
	}
	if _, ok := result["unrelated_metric"]; ok {
		t.Error("non-enriched_fields metrics should be excluded")
	}
	if len(result) != 3 {
		t.Errorf("result has %d keys, want 3", len(result))
	}
}

// TestHelper_enrichmentProviderBreakdown verifies helper enrichment provider breakdown.
func TestHelper_enrichmentProviderBreakdown(t *testing.T) {
	metrics := []map[string]any{
		{"metric": "enriched_fields", "source": "crossref", "value": int64(30)},
		{"metric": "enriched_fields", "source": "openalex", "value": int64(20)},
		{"metric": "enriched_fields", "source": "", "value": int64(50)},
		{"metric": "other", "source": "crossref", "value": int64(99)},
	}
	result := metricsummary.Build(metrics)["enrichment_provider_breakdown"].(map[string]any)
	crossref := result["crossref"].(map[string]any)
	if crossref["value"] != int64(30) {
		t.Errorf("crossref value = %v, want 30", crossref["value"])
	}
	openalex := result["openalex"].(map[string]any)
	if openalex["value"] != int64(20) {
		t.Errorf("openalex value = %v, want 20", openalex["value"])
	}
	if result[""] != nil {
		t.Error("empty source should be excluded")
	}
	if len(result) != 2 {
		t.Errorf("result has %d keys, want 2", len(result))
	}
}

// TestHelper_normalizationFieldBreakdown verifies helper normalization field breakdown.
func TestHelper_normalizationFieldBreakdown(t *testing.T) {
	metrics := []map[string]any{
		{"source": "publisher", "metric": "normalization_fields_processed", "value": int64(100)},
		{"source": "publisher", "metric": "normalization_fields_changed", "value": int64(30)},
		{"source": "publisher", "metric": "normalization_fields_already_canonical", "value": int64(60)},
		{"source": "publisher", "metric": "normalization_fields_unavailable", "value": int64(10)},
		{"source": "journal", "metric": "normalization_fields_processed", "value": int64(50)},
		{"source": "journal", "metric": "normalization_fields_changed", "value": int64(20)},
	}
	result := metricsummary.Build(metrics)["normalization_field_breakdown"].(map[string]map[string]any)
	pub := result["publisher"]
	if p := pub["processed"].(map[string]any)["value"]; p != int64(100) {
		t.Errorf("publisher processed = %v, want 100", p)
	}
	if c := pub["changed"].(map[string]any)["value"]; c != int64(30) {
		t.Errorf("publisher changed = %v, want 30", c)
	}
	if d := pub["changed"].(map[string]any)["denominator"]; d != int64(100) {
		t.Errorf("publisher changed denominator = %v, want 100", d)
	}
	jour := result["journal"]
	if p := jour["processed"].(map[string]any)["value"]; p != int64(50) {
		t.Errorf("journal processed = %v, want 50", p)
	}
	if c := jour["changed"].(map[string]any)["value"]; c != int64(20) {
		t.Errorf("journal changed = %v, want 20", c)
	}
	// author_name and affiliation should have default false entries
	for _, field := range []string{"author_name", "affiliation"} {
		for _, status := range []string{"processed", "changed", "already_canonical", "unavailable"} {
			item := result[field][status].(map[string]any)
			if item["available"] != false {
				t.Errorf("%s.%s.available = %v, want false", field, status, item["available"])
			}
		}
	}
	if _, ok := result["unknown"]; ok {
		t.Error("unknown field should not be in result")
	}
}

// TestHelper_normalizationFieldBreakdown_emptyProcessed verifies helper normalization field breakdown empty processed.
func TestHelper_normalizationFieldBreakdown_emptyProcessed(t *testing.T) {
	metrics := []map[string]any{
		{"source": "publisher", "metric": "normalization_fields_changed", "value": int64(10)},
	}
	result := metricsummary.Build(metrics)["normalization_field_breakdown"].(map[string]map[string]any)
	pub := result["publisher"]
	changed := pub["changed"].(map[string]any)
	if changed["available"] != true {
		t.Error("changed should be marked available even when processed is 0")
	}
	if changed["value"] != int64(10) {
		t.Errorf("changed value = %v, want 10", changed["value"])
	}
	if _, ok := changed["denominator"]; ok {
		t.Error("denominator should not be set when processed is 0")
	}
	if _, ok := changed["percentage"]; ok {
		t.Error("percentage should not be set when processed is 0")
	}
}

// TestHelper_normalizationFieldBreakdown_skipsMissingStatuses verifies helper normalization field breakdown skips missing statuses.
func TestHelper_normalizationFieldBreakdown_skipsMissingStatuses(t *testing.T) {
	// Statuses not in the known list should be skipped
	metrics := []map[string]any{
		{"source": "publisher", "metric": "normalization_fields_processed", "value": int64(100)},
		{"source": "publisher", "metric": "normalization_fields_unknown_status", "value": int64(99)},
	}
	result := metricsummary.Build(metrics)["normalization_field_breakdown"].(map[string]map[string]any)
	pub := result["publisher"]
	if _, ok := pub["unknown_status"]; ok {
		t.Error("unknown status should be skipped")
	}
}

// TestHelper_metricDenominator verifies helper metric denominator.
func TestHelper_metricDenominator(t *testing.T) {
	values := map[string]int64{
		"input_records":                  200,
		"deduplicated_articles":          150,
		"valid_articles":                 120,
		"normalization_fields_processed": 500,
	}
	tests := []struct {
		metric  string
		wantDen int64
		wantOk  bool
	}{
		{"input_records", 200, true},
		{"parsed_articles", 200, true},
		{"deduplicated_articles", 200, true},
		{"valid_articles", 150, true},
		{"discarded_articles", 150, true},
		{"enrichment_candidates", 150, true},
		{"enriched_article_updates", 150, true},
		{"normalized_articles_processed", 120, true},
		{"normalization_fields_changed", 500, true},
		{"normalization_fields_already_canonical", 500, true},
		{"normalization_fields_unavailable", 500, true},
		{"unknown_metric", 0, false},
	}
	for _, tc := range tests {
		den, ok := capturedDenominator(tc.metric, values)
		if ok != tc.wantOk {
			t.Errorf("metricDenominator(%q) ok = %v, want %v", tc.metric, ok, tc.wantOk)
		}
		if den != tc.wantDen {
			t.Errorf("metricDenominator(%q) = %d, want %d", tc.metric, den, tc.wantDen)
		}
	}
}

// TestHelper_metricDenominator_zeroThreshold verifies helper metric denominator zero threshold.
func TestHelper_metricDenominator_zeroThreshold(t *testing.T) {
	// When the candidate denominator is 0, ok should be false
	values := map[string]int64{
		"input_records": 0,
	}
	den, ok := capturedDenominator("input_records", values)
	if ok {
		t.Error("expected ok=false when denominator value is 0")
	}
	if den != 0 {
		t.Errorf("expected den=0, got %d", den)
	}
}

// TestHelper_percent verifies helper percent.
func TestHelper_percent(t *testing.T) {
	tests := []struct {
		value, denominator int64
		want               *float64
	}{
		{50, 100, ptr(50.0)},
		{1, 3, ptr(100.0 / 3.0)},
		{0, 100, ptr(0.0)},
		{100, 100, ptr(100.0)},
		{50, 0, nil},
		{0, 0, nil},
	}
	for _, tc := range tests {
		got := capturedPercentage(tc.value, tc.denominator)
		if tc.want == nil {
			if got != nil {
				t.Errorf("percent(%d,%d) = %v, want nil", tc.value, tc.denominator, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("percent(%d,%d) = nil, want %f", tc.value, tc.denominator, *tc.want)
			continue
		}
		if math.Abs(*got-*tc.want) > 1e-9 {
			t.Errorf("percent(%d,%d) = %f, want %f", tc.value, tc.denominator, *got, *tc.want)
		}
	}
}

// ptr returns a floating-point expectation without changing report inputs.
func ptr(v float64) *float64 { return &v }

// summaryNamed converts fixture captures into the public report builder's raw metric input.
func summaryNamed(named map[string]map[string]any) map[string]any {
	rows := make([]map[string]any, 0, len(named))
	for name, item := range named {
		rows = append(rows, map[string]any{"metric": name, "source": "", "value": item["value"]})
	}
	return metricsummary.Build(rows)
}

// capturedDenominator observes a selected metric's population through the public captured-metrics report.
func capturedDenominator(name string, totals map[string]int64) (int64, bool) {
	named := map[string]map[string]any{}
	for key, value := range totals {
		named[key] = map[string]any{"value": value}
	}
	if _, exists := named[name]; !exists {
		named[name] = map[string]any{"value": int64(1)}
	}
	for _, item := range summaryNamed(named)["captured_metrics"].([]map[string]any) {
		if item["metric"] == name {
			value, ok := item["denominator"].(int64)
			return value, ok
		}
	}
	return 0, false
}

// capturedPercentage observes the same percentage arithmetic and absent-population behavior through a retained metric.
func capturedPercentage(value, denominator int64) *float64 {
	report := summaryNamed(map[string]map[string]any{"input_records": {"value": denominator}, "parsed_articles": {"value": value}})
	item := report["retention_funnel"].(map[string]any)["parsed_articles"].(map[string]any)
	percentage, ok := item["percentage"].(float64)
	if !ok {
		return nil
	}
	return &percentage
}
