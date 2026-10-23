// Package metricsummary builds the viewer's captured-metric presentation without database or HTTP dependencies.
package metricsummary

import "strings"

var knownRunMetrics = []string{
	"input_records",
	"parsed_articles",
	"deduplicated_articles",
	"duplicate_articles",
	"valid_articles",
	"discarded_articles",
	"enrichment_skipped",
	"enrichment_candidates",
	"enriched_article_updates",
	"enriched_fields_total",
	"enriched_fields_title",
	"enriched_fields_abstract",
	"enriched_fields_publisher",
	"enriched_fields_citation_count",
	"enriched_fields_references",
	"enriched_fields_authors",
	"enriched_fields_author_orcid",
	"enriched_fields_author_first_name",
	"enriched_fields_author_last_name",
	"enriched_fields_author_citation_name",
	"enriched_fields_author_affiliation",
	"normalized_articles_processed",
	"normalization_fields_processed",
	"normalization_fields_changed",
	"normalization_fields_already_canonical",
	"normalization_fields_unavailable",
	"cache_hits",
	"cache_misses",
	"cache_negative",
	"cache_stale",
	"cache_network_fetches",
	"cache_invalid_payloads",
}

// Build returns captured metrics and their source, enrichment, validation, cache, and normalization breakdowns.
func Build(metrics []map[string]any) map[string]any {
	metricValues := map[string]int64{}
	for _, metric := range metrics {
		if source, _ := metric["source"].(string); source == "" {
			switch v := metric["value"].(type) {
			case int64:
				metricValues[metric["metric"].(string)] = v
			case int:
				metricValues[metric["metric"].(string)] = int64(v)
			}
		}
	}
	byName := make(map[string]map[string]any, len(metrics))
	for _, metric := range metrics {
		item := map[string]any{
			"metric":    metric["metric"],
			"source":    metric["source"],
			"available": true,
			"state":     "recorded",
			"value":     metric["value"],
		}
		if metric["source"] == "" {
			denominator, ok := metricDenominator(metric["metric"].(string), metricValues)
			if ok && denominator > 0 {
				item["denominator"] = denominator
				item["percentage"] = float64(metric["value"].(int64)) * 100 / float64(denominator)
			}
		}
		if metric["source"] == "" {
			byName[metric["metric"].(string)] = item
		}
	}
	metricSummary := make([]map[string]any, 0, len(knownRunMetrics)+len(metrics))
	for _, name := range knownRunMetrics {
		if item, ok := byName[name]; ok {
			metricSummary = append(metricSummary, item)
		} else {
			metricSummary = append(metricSummary, map[string]any{
				"metric":    name,
				"source":    "",
				"available": false,
				"state":     "unavailable",
			})
		}
	}
	for _, metric := range metrics {
		if metric["source"] != "" {
			metricSummary = append(metricSummary, map[string]any{
				"metric":    metric["metric"],
				"source":    metric["source"],
				"available": true,
				"state":     "recorded",
				"value":     metric["value"],
			})
		}
	}
	return map[string]any{
		"captured_metrics":              metricSummary,
		"retention_funnel":              metricGroup(byName, "input_records", "parsed_articles", "deduplicated_articles", "valid_articles", "discarded_articles"),
		"validation_breakdown":          metricGroup(byName, "valid_articles", "discarded_articles"),
		"cache_breakdown":               metricGroup(byName, "cache_hits", "cache_misses", "cache_negative", "cache_stale", "cache_network_fetches", "cache_invalid_payloads"),
		"enrichment_breakdown":          metricGroup(byName, "enrichment_skipped", "enrichment_candidates", "enriched_article_updates"),
		"normalization_breakdown":       metricGroup(byName, "normalized_articles_processed", "normalization_fields_processed", "normalization_fields_changed", "normalization_fields_already_canonical", "normalization_fields_unavailable"),
		"source_breakdown":              sourceBreakdown(metrics, metricValues),
		"enrichment_field_breakdown":    enrichmentFieldBreakdown(byName),
		"enrichment_provider_breakdown": enrichmentProviderBreakdown(metrics),
		"normalization_field_breakdown": normalizationFieldBreakdown(metrics),
	}
}

// metricGroup selects named metrics and marks absent captures as unavailable.
func metricGroup(metrics map[string]map[string]any, names ...string) map[string]any {
	result := make(map[string]any, len(names))
	for _, name := range names {
		if metric, ok := metrics[name]; ok {
			result[name] = metric
		} else {
			result[name] = map[string]any{
				"available": false,
				"state":     "unavailable",
			}
		}
	}
	return result
}

// sourceBreakdown calculates each source's share of captured input records.
func sourceBreakdown(metrics []map[string]any, totals map[string]int64) map[string]any {
	result := map[string]any{}
	for _, metric := range metrics {
		source, _ := metric["source"].(string)
		if source == "" || metric["metric"] != "input_records" {
			continue
		}
		value, _ := metric["value"].(int64)
		item := map[string]any{
			"available": true,
			"state":     "recorded",
			"value":     value,
		}
		if denominator := totals["input_records"]; denominator > 0 {
			item["denominator"] = denominator
			item["percentage"] = float64(value) * 100 / float64(denominator)
		}
		result[source] = item
	}
	return result
}

// enrichmentFieldBreakdown extracts per-field enrichment counts from the
// byName metric map. Metrics named "enriched_fields_<field>" with no source
// are included.
func enrichmentFieldBreakdown(byName map[string]map[string]any) map[string]any {
	result := make(map[string]any)
	prefix := "enriched_fields_"
	for name, item := range byName {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		field := strings.TrimPrefix(name, prefix)
		if field == "total" {
			continue
		}
		result[field] = item
	}
	return result
}

// enrichmentProviderBreakdown extracts per-provider enrichment counts from the
// raw metrics list. Metrics named "enriched_fields" with a non-empty source
// are included.
func enrichmentProviderBreakdown(metrics []map[string]any) map[string]any {
	result := make(map[string]any)
	for _, metric := range metrics {
		m, _ := metric["metric"].(string)
		source, _ := metric["source"].(string)
		if m != "enriched_fields" || source == "" {
			continue
		}
		result[source] = map[string]any{
			"available": true,
			"state":     "recorded",
			"value":     metric["value"],
		}
	}
	return result
}

// normalizationFieldBreakdown groups normalization outcome metrics by field and derives percentages.
func normalizationFieldBreakdown(metrics []map[string]any) map[string]map[string]any {
	fields := []string{"publisher", "journal", "author_name", "affiliation"}
	statuses := []string{"processed", "changed", "already_canonical", "unavailable"}
	result := make(map[string]map[string]any, len(fields))
	for _, field := range fields {
		result[field] = make(map[string]any, len(statuses))
		for _, status := range statuses {
			result[field][status] = map[string]any{
				"available": false,
				"state":     "unavailable",
			}
		}
	}
	for _, metric := range metrics {
		field, _ := metric["source"].(string)
		if _, ok := result[field]; !ok {
			continue
		}
		name, _ := metric["metric"].(string)
		const prefix = "normalization_fields_"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		status := strings.TrimPrefix(name, prefix)
		if _, ok := result[field][status]; !ok {
			continue
		}
		result[field][status] = map[string]any{
			"available": true,
			"state":     "recorded",
			"value":     metric["value"],
		}
	}
	for _, field := range fields {
		processed, _ := result[field]["processed"].(map[string]any)
		processedValue, _ := processed["value"].(int64)
		if processedValue == 0 {
			continue
		}
		for _, status := range statuses[1:] {
			item := result[field][status].(map[string]any)
			if item["available"] != true {
				continue
			}
			item["denominator"] = processedValue
			item["percentage"] = float64(item["value"].(int64)) * 100 / float64(processedValue)
		}
	}
	return result
}

// metricDenominator returns the captured population against which a metric is measured.
func metricDenominator(metric string, values map[string]int64) (int64, bool) {
	switch metric {
	case "input_records", "parsed_articles", "deduplicated_articles":
		return values["input_records"], values["input_records"] > 0
	case "valid_articles", "discarded_articles", "enrichment_candidates", "enriched_article_updates":
		return values["deduplicated_articles"], values["deduplicated_articles"] > 0
	case "normalized_articles_processed":
		return values["valid_articles"], values["valid_articles"] > 0
	case "normalization_fields_changed", "normalization_fields_already_canonical", "normalization_fields_unavailable":
		return values["normalization_fields_processed"], values["normalization_fields_processed"] > 0
	}
	return 0, false
}
