// errors_integration_test.go verifies the work family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package work_test

import (
	"context"
	"testing"

	"analysis/database/work"
)

// TestStoreOperationsPropagateConnectionErrors verifies every public store
// operation reports a closed connection as an error rather than a value.
func TestStoreOperationsPropagateConnectionErrors(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"CreateWorkByDOI", func() error {
			_, err := store.CreateWorkByDOI(ctx, "10.1000/closed")
			return err
		}},
		{"CreateWorkWithoutDOI", func() error { _, err := store.CreateWorkWithoutDOI(ctx); return err }},
		{"GetWorkByID", func() error { _, err := store.GetWorkByID(ctx, 1); return err }},
		{"GetWorkByDOI", func() error { _, err := store.GetWorkByDOI(ctx, "10.1000/closed"); return err }},
		{"ListWorksByIDs", func() error { _, err := store.ListWorksByIDs(ctx, []int64{1}); return err }},
		{"CountWorks", func() error { _, err := store.CountWorks(ctx); return err }},
		{"InsertIdentifier", func() error {
			_, err := store.InsertIdentifier(ctx, work.InsertIdentifierInput{WorkID: 1, Namespace: "closed", Identifier: "closed"})
			return err
		}},
		{"GetIdentifierByID", func() error { _, err := store.GetIdentifierByID(ctx, 1); return err }},
		{"ListIdentifiersByWorkID", func() error { _, err := store.ListIdentifiersByWorkID(ctx, 1); return err }},
		{"GetIdentifierByNamespaceAndIdentifier", func() error {
			_, err := store.GetIdentifierByNamespaceAndIdentifier(ctx, "closed", "closed")
			return err
		}},
		{"CountIdentifiersByWorkID", func() error { _, err := store.CountIdentifiersByWorkID(ctx, 1); return err }},
		{"CreateRevision", func() error {
			_, err := store.CreateRevision(ctx, &work.Revision{WorkID: 1, PipelineRunID: 1, ProducerStage: work.ProducerStageParse})
			return err
		}},
		{"GetRevisionByID", func() error { _, err := store.GetRevisionByID(ctx, 1); return err }},
		{"ListRevisionsByWorkID", func() error { _, err := store.ListRevisionsByWorkID(ctx, 1); return err }},
		{"ListRevisionsByRunID", func() error { _, err := store.ListRevisionsByRunID(ctx, 1); return err }},
		{"CountRevisionsByWorkID", func() error { _, err := store.CountRevisionsByWorkID(ctx, 1); return err }},
		{"SetStageOutcome", func() error {
			return store.SetStageOutcome(ctx, work.StageOutcomeInput{
				RunID: 1, WorkID: 1, StageName: work.StageNameParse, Outcome: work.OutcomeParsed,
			})
		}},
		{"GetStageOutcome", func() error {
			_, err := store.GetStageOutcome(ctx, 1, 1, work.StageNameParse)
			return err
		}},
		{"ListStageOutcomesByRun", func() error { _, err := store.ListStageOutcomesByRun(ctx, 1); return err }},
		{"ListStageOutcomesByWork", func() error { _, err := store.ListStageOutcomesByWork(ctx, 1); return err }},
		{"CountStageOutcomes", func() error {
			_, err := store.CountStageOutcomes(ctx, 1, work.StageNameParse, work.OutcomeParsed)
			return err
		}},
		{"CreateReference", func() error {
			_, err := store.CreateReference(ctx, &work.Reference{WorkRevisionID: 1, MentionOrder: 1})
			return err
		}},
		{"GetReferenceByID", func() error { _, err := store.GetReferenceByID(ctx, 1); return err }},
		{"ListReferencesByRevisionID", func() error { _, err := store.ListReferencesByRevisionID(ctx, 1); return err }},
		{"ListReferencesByResolvedWorkID", func() error { _, err := store.ListReferencesByResolvedWorkID(ctx, 1); return err }},
		{"ReplaceRunTermData", func() error {
			return store.ReplaceRunTermData(ctx, work.ReplaceTermDataInput{RunID: 1, TermsBySource: map[string][]string{"closed": {"closed"}}})
		}},
		{"HasRunTermData", func() error { _, err := store.HasRunTermData(ctx, 1); return err }},
		{"ListRunTerms", func() error { _, err := store.ListRunTerms(ctx, 1); return err }},
		{"GetRevisionMatches", func() error { _, err := store.GetRevisionMatches(ctx, 1, 1); return err }},
		{"GetRevisionMatchesBulk", func() error { _, err := store.GetRevisionMatchesBulk(ctx, 1, []int64{1}); return err }},
		{"ListArticleAuthors", func() error {
			_, err := store.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{RevisionID: 1, Limit: 10})
			return err
		}},
		{"GetArticleRevision", func() error { _, err := store.GetArticleRevision(ctx, 1, 1); return err }},
		{"GetArticleDetailWorkID", func() error { _, err := store.GetArticleDetailWorkID(ctx, 1, 1); return err }},
		{"ListArticleReferences", func() error {
			_, err := store.ListArticleReferences(ctx, work.ArticleReferencePageInput{RevisionID: 1, Limit: 10})
			return err
		}},
		{"ListArticleStages", func() error {
			_, err := store.ListArticleStages(ctx, work.ArticleStagePageInput{RunID: 1, WorkID: 1, Limit: 10})
			return err
		}},
		{"GetReferenceDetail", func() error { _, err := store.GetReferenceDetail(ctx, 1, 1); return err }},
		{"EvaluationReviewSummary", func() error {
			_, err := store.EvaluationReviewSummary(ctx, work.EvaluationFilter{RunID: 1, ContextID: 1})
			return err
		}},
		{"ListEvaluation", func() error {
			_, err := store.ListEvaluation(ctx, work.EvaluationFilter{RunID: 1, ContextID: 1, SortField: "id", Order: "ASC", Page: 1, PerPage: 10})
			return err
		}},
		{"EvaluationQueueNavigation", func() error {
			_, err := store.EvaluationQueueNavigation(ctx, work.EvaluationFilter{RunID: 1, ContextID: 1, SortField: "id", Order: "ASC"})
			return err
		}},
		{"ListGraphArticles", func() error {
			_, _, err := store.ListGraphArticles(ctx, work.GraphFilter{RunID: 1, Limit: 10})
			return err
		}},
		{"ListGraphAuthorships", func() error {
			_, err := store.ListGraphAuthorships(ctx, []int64{1}, 10, 10)
			return err
		}},
		{"ListGraphCitations", func() error {
			_, err := store.ListGraphCitations(ctx, []int64{1}, []int64{1}, 10)
			return err
		}},
		{"ListGraphReferences", func() error {
			_, err := store.ListGraphReferences(ctx, []int64{1}, 10)
			return err
		}},
		{"CurrentCoverage", func() error { _, err := store.CurrentCoverage(ctx, 1); return err }},
		{"RelationshipTotals", func() error { _, err := store.RelationshipTotals(ctx, 1); return err }},
		{"RunStageSummaries", func() error { _, err := store.RunStageSummaries(ctx, 1); return err }},
		{"ListCorpusReferences", func() error {
			_, err := store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{RunID: 1, Sort: "id", Order: "ASC", Page: 1, PerPage: 10})
			return err
		}},
		{"ListRunStages", func() error {
			_, err := store.ListRunStages(ctx, work.RunStageFilter{RunID: 1, Sort: "id", Order: "ASC", Page: 1, PerPage: 10})
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed connection did not produce an error")
			}
		})
	}
}
