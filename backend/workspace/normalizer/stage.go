// Package normalizer owns the workspace field normalization and its metric evidence.
package normalizer

import (
	"analysis/backend/article"
	"analysis/backend/database"

	dbrun "analysis/backend/database/run"

	"analysis/backend/normalization"

	"context"

	"strings"
)

// FieldOutcome records one field's normalization outcome for an article DOI.
type FieldOutcome struct {
	DOI     string
	Field   string
	Outcome string
}

const (
	normalizationOutcomeChanged          = "changed"
	normalizationOutcomeAlreadyCanonical = "already_canonical"
	normalizationOutcomeUnavailable      = "unavailable"
)

var normalizationFields = []string{"publisher", "journal", "author_name", "affiliation"}

var normalizationOutcomes = []string{
	normalizationOutcomeChanged,
	normalizationOutcomeAlreadyCanonical,
	normalizationOutcomeUnavailable,
}

// Normalize applies each normalizer and records one outcome
// for every checked field. Journal normalization is retained in revision
// extension data, so its result is measured here but not assigned to Journal.
func Normalize(articles []*article.Article) []FieldOutcome {
	results := make([]FieldOutcome, 0, len(articles)*2)
	for _, a := range articles {
		publisher := normalization.NormalizePublisher(a.Publisher)
		results = append(results, normalizationResult(a.DOI, "publisher", a.Publisher, publisher))
		a.Publisher = publisher

		journal := normalization.NormalizeJournal(a.Journal)
		results = append(results, normalizationResult(a.DOI, "journal", a.Journal, journal))

		for index := range a.Authors {
			author := &a.Authors[index]
			name := normalization.NormalizeAuthorName(author.CitationName)
			results = append(results, normalizationResult(a.DOI, "author_name", author.CitationName, name))
			author.NormalizedName = name

			affiliation := normalization.NormalizeAffiliation(author.Affiliation)
			results = append(results, normalizationResult(a.DOI, "affiliation", author.Affiliation, affiliation))
			author.Affiliation = affiliation
		}
	}
	return results
}

// normalizationResult classifies a field as changed, already canonical, or unavailable.
func normalizationResult(doi, field, input, output string) FieldOutcome {
	outcome := normalizationOutcomeChanged
	if strings.TrimSpace(input) == "" {
		outcome = normalizationOutcomeUnavailable
	} else if input == output {
		outcome = normalizationOutcomeAlreadyCanonical
	}
	return FieldOutcome{DOI: doi, Field: field, Outcome: outcome}
}

// RecordMetrics stores mutually exclusive outcomes for each
// checked field. No per-field audit events are emitted: the immutable
// normalized revision already records the output, and the prior revision is
// retained as the corresponding input evidence.
func RecordMetrics(ctx context.Context, db *database.Database, runID int64, processedArticles int, results []FieldOutcome) error {
	totals := make(map[string]int, len(normalizationOutcomes))
	byField := make(map[string]map[string]int, len(normalizationFields))
	for _, field := range normalizationFields {
		byField[field] = make(map[string]int, len(normalizationOutcomes))
	}
	for _, result := range results {
		totals[result.Outcome]++
		byField[result.Field][result.Outcome]++
	}
	for _, metric := range []dbrun.MetricInput{
		{RunID: runID, Metric: "normalized_articles_processed", Value: processedArticles},
		{RunID: runID, Metric: "normalization_fields_processed", Value: len(results)},
		{RunID: runID, Metric: "normalization_fields_changed", Value: totals[normalizationOutcomeChanged]},
		{RunID: runID, Metric: "normalization_fields_already_canonical", Value: totals[normalizationOutcomeAlreadyCanonical]},
		{RunID: runID, Metric: "normalization_fields_unavailable", Value: totals[normalizationOutcomeUnavailable]},
	} {
		if err := db.Run.SetMetric(ctx, metric); err != nil {
			return err
		}
	}
	for _, field := range normalizationFields {
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "normalization_fields_processed", Source: field, Value: sumNormalizationOutcomes(byField[field])}); err != nil {
			return err
		}
		for _, outcome := range normalizationOutcomes {
			if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "normalization_fields_" + outcome, Source: field, Value: byField[field][outcome]}); err != nil {
				return err
			}
		}
	}
	return nil
}

// sumNormalizationOutcomes totals the recognized mutually exclusive normalization outcomes.
func sumNormalizationOutcomes(outcomes map[string]int) int {
	total := 0
	for _, outcome := range normalizationOutcomes {
		total += outcomes[outcome]
	}
	return total
}
