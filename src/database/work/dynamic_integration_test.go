// dynamic_integration_test.go verifies the retained work-family builders
// reject injection attempts, follow their declared ordering, and cap their
// search, filter, identifier-list, and page inputs.
//go:build integration

package work_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/work"
)

// TestStoreCorpusBuildersConstrainDynamicInputs verifies the retained corpus
// reference, author, and run-stage builders bind search values, reject unknown
// sort and order values, follow their declared ordering, and cap oversized
// inputs.
func TestStoreCorpusBuildersConstrainDynamicInputs(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	if _, err := fixture.store.CreateReference(ctx, &work.Reference{
		WorkRevisionID: fixture.normalizeRevisionID, MentionOrder: 2, RawReference: "raw",
		Title: `x' OR '1'='1`, Author: "Injected", Year: 2024, Source: "scopus",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.DB.Exec("INSERT INTO author_occurrences (citation_name) VALUES (?)", `x' OR '1'='1`); err != nil {
		t.Fatal(err)
	}
	var injectedAuthorID int64
	if err := fixture.db.DB.QueryRow("SELECT id FROM author_occurrences WHERE citation_name=?", `x' OR '1'='1`).Scan(&injectedAuthorID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.DB.Exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order) VALUES (?, ?, 3)", fixture.normalizeRevisionID, injectedAuthorID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: fixture.runID, WorkID: fixture.workID, StageName: work.StageNameEnrich, Outcome: work.OutcomeEnriched, Reason: `x' OR '1'='1`,
	}); err != nil {
		t.Fatal(err)
	}

	references, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{RunID: fixture.runID, Sort: "title", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(references.Items) != 2 || references.Items[0].Title != "Second article" || references.Items[1].Title != `x' OR '1'='1` {
		t.Fatalf("ascending reference order = %+v", references.Items)
	}
	literalReferences, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{RunID: fixture.runID, Query: `' OR '1'='1`, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if literalReferences.Total != 1 || len(literalReferences.Items) != 1 || literalReferences.Items[0].Title != `x' OR '1'='1` {
		t.Fatalf("literal reference search = %+v", literalReferences.Items)
	}

	authors, err := fixture.store.ListCorpusAuthors(ctx, work.CorpusAuthorFilter{RunID: fixture.runID, Sort: "citation_name", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(authors.Items) != 3 || authors.Items[0].CitationName != "Doe, Jane" || authors.Items[2].CitationName != `x' OR '1'='1` {
		t.Fatalf("ascending author order = %+v", authors.Items)
	}
	literalAuthors, err := fixture.store.ListCorpusAuthors(ctx, work.CorpusAuthorFilter{RunID: fixture.runID, Query: `' OR '1'='1`, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if literalAuthors.Total != 1 || len(literalAuthors.Items) != 1 || literalAuthors.Items[0].CitationName != `x' OR '1'='1` {
		t.Fatalf("literal author search = %+v", literalAuthors.Items)
	}

	stages, err := fixture.store.ListRunStages(ctx, work.RunStageFilter{RunID: fixture.runID, Sort: "outcome", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(stages.Items) < 2 || stages.Items[0].Outcome > stages.Items[len(stages.Items)-1].Outcome {
		t.Fatalf("ascending stage order = %+v", stages.Items)
	}
	literalStages, err := fixture.store.ListRunStages(ctx, work.RunStageFilter{RunID: fixture.runID, Query: `' OR '1'='1`, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if literalStages.Total != 1 || len(literalStages.Items) != 1 || literalStages.Items[0].Reason != `x' OR '1'='1` {
		t.Fatalf("literal stage search = %+v", literalStages.Items)
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{"injected reference sort", func() error {
			_, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{RunID: fixture.runID, Sort: "id; DROP TABLE reference_mentions; --", Order: "ASC", Page: 1, PerPage: 20})
			return err
		}},
		{"injected reference order", func() error {
			_, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{RunID: fixture.runID, Sort: "id", Order: "ASC; DROP TABLE reference_mentions; --", Page: 1, PerPage: 20})
			return err
		}},
		{"oversized reference query", func() error {
			_, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{RunID: fixture.runID, Query: strings.Repeat("x", 201), Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
			return err
		}},
		{"oversized author page size", func() error {
			_, err := fixture.store.ListCorpusAuthors(ctx, work.CorpusAuthorFilter{RunID: fixture.runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 1001})
			return err
		}},
		{"zero stage page", func() error {
			_, err := fixture.store.ListRunStages(ctx, work.RunStageFilter{RunID: fixture.runID, Sort: "id", Order: "ASC", Page: 0, PerPage: 20})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}

// TestStoreEvaluationBuilderConstrainsDynamicInputs verifies the retained
// evaluation builder binds search values, rejects unknown sort and order
// values, follows its declared ordering, and caps oversized inputs.
func TestStoreEvaluationBuilderConstrainsDynamicInputs(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()
	base := work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, SortField: "title", Order: "ASC", Page: 1, PerPage: 10}

	ascending, err := fixture.store.ListEvaluation(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(ascending.Items) != 2 || ascending.Items[0].Title != "Normalized article" || ascending.Items[1].Title != "Second article" {
		t.Fatalf("ascending evaluation order = %+v", ascending.Items)
	}
	descending := base
	descending.Order = "DESC"
	reversed, err := fixture.store.ListEvaluation(ctx, descending)
	if err != nil {
		t.Fatal(err)
	}
	if len(reversed.Items) != 2 || reversed.Items[0].Title != "Second article" || reversed.Items[1].Title != "Normalized article" {
		t.Fatalf("descending evaluation order = %+v", reversed.Items)
	}

	// A malicious search value is bound as a literal LIKE pattern and matches
	// no stored article.
	literal := base
	literal.Query = `' OR '1'='1`
	none, err := fixture.store.ListEvaluation(ctx, literal)
	if err != nil {
		t.Fatal(err)
	}
	if none.Total != 0 || len(none.Items) != 0 {
		t.Fatalf("literal evaluation search = %+v, want no matches", none.Items)
	}

	for _, test := range []struct {
		name   string
		filter work.EvaluationFilter
	}{
		{"injected sort", work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id; DROP TABLE work_revisions; --", Order: "ASC", Page: 1, PerPage: 10}},
		{"injected order", work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC; DROP TABLE work_revisions; --", Page: 1, PerPage: 10}},
		{"oversized query", work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, Query: strings.Repeat("x", 201), SortField: "id", Order: "ASC", Page: 1, PerPage: 10}},
		{"oversized source", work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, Source: strings.Repeat("x", 201), SortField: "id", Order: "ASC", Page: 1, PerPage: 10}},
		{"oversized page size", work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC", Page: 1, PerPage: 1001}},
		{"zero page", work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC", Page: 0, PerPage: 10}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := fixture.store.ListEvaluation(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}

// TestStoreGraphBuildersConstrainDynamicInputs verifies the retained graph
// builders bind filter values and cap filter strings, identifier lists, and
// row limits.
func TestStoreGraphBuildersConstrainDynamicInputs(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	// A malicious filter value is bound and matches no stored article.
	none, matches, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Query: `' OR '1'='1`, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if matches != 0 || len(none) != 0 {
		t.Fatalf("literal graph search = %+v matches=%d, want no matches", none, matches)
	}

	oversizedIDs := make([]int64, 2001)
	for index := range oversizedIDs {
		oversizedIDs[index] = int64(index + 1)
	}
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"oversized graph query", func() error {
			_, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Query: strings.Repeat("x", 201), Limit: 10})
			return err
		}},
		{"oversized graph author", func() error {
			_, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Author: strings.Repeat("x", 201), Limit: 10})
			return err
		}},
		{"oversized graph orcid", func() error {
			_, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, ORCID: strings.Repeat("x", 201), Limit: 10})
			return err
		}},
		{"oversized graph reference", func() error {
			_, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Reference: strings.Repeat("x", 201), Limit: 10})
			return err
		}},
		{"oversized graph source", func() error {
			_, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Source: strings.Repeat("x", 201), Limit: 10})
			return err
		}},
		{"oversized graph limit", func() error {
			_, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Limit: 2001})
			return err
		}},
		{"oversized authorship ids", func() error {
			_, err := fixture.store.ListGraphAuthorships(ctx, oversizedIDs, 10, 10)
			return err
		}},
		{"oversized authorship author limit", func() error {
			_, err := fixture.store.ListGraphAuthorships(ctx, []int64{fixture.normalizeRevisionID}, 10001, 10)
			return err
		}},
		{"oversized citation row limit", func() error {
			_, err := fixture.store.ListGraphCitations(ctx, []int64{fixture.normalizeRevisionID}, []int64{fixture.secondWorkID}, 25001)
			return err
		}},
		{"oversized reference ids", func() error {
			_, err := fixture.store.ListGraphReferences(ctx, oversizedIDs, 10)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}
