// Package enrichment owns the workspace application of observed provider changes and their provenance.
package enrichment

import (
	"analysis/backend/article"
	"analysis/backend/database"

	dbrun "analysis/backend/database/run"

	"analysis/backend/enrich"
	"analysis/backend/manifest"

	"context"
	"encoding/json"
	"fmt"

	"regexp"

	"strconv"
	"strings"
	"time"
)

// AuthorOrigin identifies the persisted article location of one author.
type AuthorOrigin struct {
	DOI   string
	Index int
}

// ApplyProfile fills missing author values and returns exactly the fields
// it changed, using the supplied provenance for audit and metric evidence.
func ApplyProfile(authors []*article.Author, profile *enrich.EnrichedAuthor, matchedORCID string, provenance map[*article.Author]AuthorOrigin) (bool, []Change) {
	changed := false
	changes := make([]Change, 0)
	record := func(author *article.Author, field string) {
		location, ok := provenance[author]
		if ok && location.DOI != "" {
			changes = append(changes, Change{DOI: location.DOI, Field: fmt.Sprintf("author_%s_%d", field, location.Index), Provider: "orcid"})
		}
	}
	for _, author := range authors {
		if author.Orcid == "" && matchedORCID != "" {
			author.Orcid = matchedORCID
			changed = true
			record(author, "orcid")
		}
		if author.FirstName == "" && profile.FirstName != "" {
			author.FirstName = profile.FirstName
			changed = true
			record(author, "first_name")
		}
		if author.LastName == "" && profile.LastName != "" {
			author.LastName = profile.LastName
			changed = true
			record(author, "last_name")
		}
		if author.CitationName == "" && profile.CitationName != "" {
			author.CitationName = profile.CitationName
			changed = true
			record(author, "citation_name")
		}
		if author.Affiliation == "" && profile.Institution != "" {
			author.Affiliation = profile.Institution
			changed = true
			record(author, "affiliation")
		}
	}
	return changed, changes
}

// Change records one enriched field change for audit purposes.
type Change struct {
	DOI      string
	Field    string
	Provider string
}

// applyArticleEnrichment applies article enrichment to the supplied state.
func applyArticleEnrichment(articles map[string]*article.Article, result *enrich.GatherResult) (int, []Change) {
	if result == nil {
		return 0, nil
	}
	updated := 0
	var changes []Change
	for doi, enrichment := range result.Articles {
		a := articles[doi]
		if a == nil || enrichment == nil {
			continue
		}
		changed := false
		allow := func(existing string) bool { return !result.FillMissingOnly || existing == "" }
		if enrichment.Title != "" && enrichment.Title != a.Title && allow(a.Title) {
			a.Title, changed = enrichment.Title, true
			changes = append(changes, Change{DOI: doi, Field: "title", Provider: result.Source})
		}
		if enrichment.Abstract != "" && enrichment.Abstract != a.Abstract && allow(a.Abstract) {
			a.Abstract, changed = enrichment.Abstract, true
			changes = append(changes, Change{DOI: doi, Field: "abstract", Provider: result.Source})
		}
		if enrichment.Publisher != "" && enrichment.Publisher != a.Publisher && allow(a.Publisher) {
			a.Publisher, changed = enrichment.Publisher, true
			changes = append(changes, Change{DOI: doi, Field: "publisher", Provider: result.Source})
		}
		if enrichment.CitationCount > 0 && enrichment.CitationCount != a.CitationCount && (!result.FillMissingOnly || a.CitationCount == 0) {
			a.CitationCount, changed = enrichment.CitationCount, true
			changes = append(changes, Change{DOI: doi, Field: "citation_count", Provider: result.Source})
		}
		if len(enrichment.References) > 0 && (!result.FillMissingOnly || len(a.CitedReferences) == 0) {
			a.CitedReferences = make([]article.Reference, len(enrichment.References))
			for i, ref := range enrichment.References {
				a.CitedReferences[i] = article.Reference{Raw: ref.Raw, DOI: ref.DOI, Title: ref.Title, Author: ref.Author, Year: ref.Year, Source: ref.Source}
			}
			changed = true
			changes = append(changes, Change{DOI: doi, Field: "references", Provider: result.Source})
		}
		if len(enrichment.Authors) > 0 && (!result.FillMissingOnly || len(a.Authors) == 0) {
			authors := usableEnrichedAuthors(enrichment.Authors)
			if len(authors) > 0 {
				a.Authors = authors
				changed = true
				changes = append(changes, Change{DOI: doi, Field: "authors", Provider: result.Source})
			}
		}
		if changed {
			updated++
		}
	}
	return updated, changes
}

// usableEnrichedAuthors converts enriched authors that contain a non-empty citation name.
func usableEnrichedAuthors(enriched []enrich.EnrichedAuthor) []article.Author {
	authors := make([]article.Author, 0, len(enriched))
	for _, author := range enriched {
		citationName := strings.TrimSpace(author.CitationName)
		if citationName == "" {
			continue
		}
		authors = append(authors, article.Author{
			Orcid: author.ORCID, FirstName: author.FirstName, LastName: author.LastName,
			CitationName: citationName, Affiliation: author.Affiliation,
		})
	}
	return authors
}

// ApplyArticles applies configured article enrichment to the supplied state.
func ApplyArticles(articles map[string]*article.Article, source enrich.SourceConfig, result *enrich.GatherResult) (int, []Change) {
	if result != nil {
		result.FillMissingOnly = source.FillMissingOnly
		for _, value := range result.Articles {
			if value == nil {
				continue
			}
			if !source.AllowsField("title") {
				value.Title = ""
			}
			if !source.AllowsField("abstract") {
				value.Abstract = ""
			}
			if !source.AllowsField("publisher") {
				value.Publisher = ""
			}
			if !source.AllowsField("citation_count") {
				value.CitationCount = 0
			}
			if !source.AllowsField("reference_count") {
				value.ReferenceCount = 0
			}
			if !source.AllowsField("references") {
				value.References = nil
			}
			if !source.AllowsField("authors") {
				value.Authors = nil
			}
		}
	}
	return applyArticleEnrichment(articles, result)
}

// RecordAudit records one field_enriched audit event per field
// change. revisionIDs maps DOI -> work_revision ID.
func RecordAudit(ctx context.Context, db *database.Database, runID int64, revisionIDs map[string]int64, changes []Change) error {
	for _, c := range changes {
		revID, ok := revisionIDs[c.DOI]
		if !ok {
			continue
		}
		metadata, err := json.Marshal(map[string]string{
			"field":    c.Field,
			"provider": c.Provider,
		})
		if err != nil {
			return err
		}
		correlationID := fmt.Sprintf("enrich-%d-%d-%s", runID, revID, c.Field)
		if _, err := db.Audit.Insert(ctx, &manifest.AuditEvent{
			OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
			Actor:         c.Provider,
			PipelineRunID: runID,
			EntityType:    "work_revision",
			EntityID:      strconv.FormatInt(revID, 10),
			Action:        manifest.AuditFieldEnriched,
			MetadataJSON:  string(metadata),
			CorrelationID: correlationID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// RecordMetrics records per-field and per-provider enrichment
// metrics to pipeline_run_metrics. Per-author-index fields (e.g.
// author_orcid_0, author_first_name_1) are aggregated into a single count
// per field type (author_orcid, author_first_name, etc.).
func RecordMetrics(ctx context.Context, db *database.Database, runID int64, changes []Change) error {
	fieldCounts := make(map[string]int)
	providerCounts := make(map[string]int)
	authorFieldSuffix := regexp.MustCompile(`_\d+$`)
	for _, c := range changes {
		// Aggregate per-author-index fields by stripping the trailing _N suffix.
		field := authorFieldSuffix.ReplaceAllString(c.Field, "")
		fieldCounts[field]++
		providerCounts[c.Provider]++
	}
	if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "enriched_fields_total", Value: len(changes)}); err != nil {
		return err
	}
	for field, count := range fieldCounts {
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "enriched_fields_" + field, Value: count}); err != nil {
			return err
		}
	}
	for provider, count := range providerCounts {
		if err := db.Run.SetMetric(ctx, dbrun.MetricInput{RunID: runID, Metric: "enriched_fields", Source: provider, Value: count}); err != nil {
			return err
		}
	}
	return nil
}
