// relationships_integration_test.go exercises the work-focused relationship
// reads against a database created by the production migration runner.
//go:build integration

package work_test

import (
	"context"
	"testing"

	"analysis/database"
	"analysis/database/work"
)

// relationshipFixture is one migrated run with works, revisions, stages,
// authorships, references, and a review context.
type relationshipFixture struct {
	store                     *work.Store
	db                        *database.Database
	runID                     int64
	workID                    int64
	parseRevisionID           int64
	supersededRevisionID      int64
	normalizeRevisionID       int64
	discardedWorkID           int64
	discardedRevisionID       int64
	secondWorkID              int64
	secondNormalizeRevisionID int64
	contextID                 int64
	referenceID               int64
}

// seedRelationshipFixture creates one completed planned run with relationship evidence.
func seedRelationshipFixture(t *testing.T) *relationshipFixture {
	t.Helper()
	store, db := openFamilyStore(t)
	ctx := context.Background()
	exec := func(query string, args ...any) int64 {
		t.Helper()
		result, err := db.DB.Exec(query, args...)
		if err != nil {
			t.Fatalf("fixture query %q: %v", query, err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	searchID := exec("INSERT INTO searches (search_id) VALUES ('work-relationship-search')")
	searchRevisionID := exec("INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (?, 'r1', 'config', 'manifest')", searchID)
	planID := exec("INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash) VALUES (?, 'fingerprint', 'manifest', 'input')", searchRevisionID)
	runID := exec("INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number) VALUES ('work-relationship', '2026-01-01 00:00:00', '2026-01-01 00:01:00', 'completed', ?, 1)", planID)

	workID, err := store.CreateWorkByDOI(ctx, "10.1000/relationship")
	if err != nil {
		t.Fatal(err)
	}
	parseRevisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageParse,
		Title: "Parsed article", Year: 2024, Journal: "Journal", Source: "scopus",
	})
	if err != nil {
		t.Fatal(err)
	}
	supersededRevisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize,
		Title: "Superseded article", Year: 2024, Journal: "Journal", Source: "scopus",
	})
	if err != nil {
		t.Fatal(err)
	}
	normalizeRevisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize,
		Title: "Normalized article", Year: 2024, Journal: "Journal", Source: "scopus",
		Abstract: "Abstract text", Keywords: `["kw"]`, CitationCount: 5, ReferenceCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: workID, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: workID, StageName: work.StageNameNormalize, Outcome: work.OutcomeNormalized,
	}); err != nil {
		t.Fatal(err)
	}

	discardedWorkID, err := store.CreateWorkByDOI(ctx, "10.1000/relationship-discarded")
	if err != nil {
		t.Fatal(err)
	}
	discardedRevisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: discardedWorkID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize,
		Title: "Discarded article", Year: 2022, Source: "ieee",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: discardedWorkID, StageName: work.StageNameValidate, Outcome: work.OutcomeDiscarded,
	}); err != nil {
		t.Fatal(err)
	}

	secondWorkID, err := store.CreateWorkByDOI(ctx, "10.1000/relationship-second")
	if err != nil {
		t.Fatal(err)
	}
	secondNormalizeRevisionID, err := store.CreateRevision(ctx, &work.Revision{
		WorkID: secondWorkID, PipelineRunID: runID, ProducerStage: work.ProducerStageNormalize,
		Title: "Second article", Year: 2023, Source: "ieee", CitationCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: secondWorkID, StageName: work.StageNameValidate, Outcome: work.OutcomeValid,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: secondWorkID, StageName: work.StageNameNormalize, Outcome: work.OutcomeNormalized,
	}); err != nil {
		t.Fatal(err)
	}

	firstAuthorID := exec("INSERT INTO author_occurrences (citation_name, first_name, last_name, orcid) VALUES ('Doe, Jane', 'Jane', 'Doe', '0000-0002-1825-0097')")
	secondAuthorID := exec("INSERT INTO author_occurrences (citation_name) VALUES ('Smith, John')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (?, ?, 1, 'University')", normalizeRevisionID, firstAuthorID)
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order) VALUES (?, ?, 2)", normalizeRevisionID, secondAuthorID)

	referenceID, err := store.CreateReference(ctx, &work.Reference{
		WorkRevisionID: normalizeRevisionID, MentionOrder: 1, RawReference: "raw reference",
		DOI: "10.1000/relationship-second", Title: "Second article", Author: "Smith", Year: 2023, Source: "scopus",
	})
	if err != nil {
		t.Fatal(err)
	}

	contextRecord, _, err := db.Reviews.CreateContext(ctx, runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, normalizeRevisionID, nil, "not_approved", []string{"out_of_scope"}, nil); err != nil {
		t.Fatal(err)
	}

	return &relationshipFixture{
		store: store, db: db, runID: runID, workID: workID,
		parseRevisionID: parseRevisionID, supersededRevisionID: supersededRevisionID,
		normalizeRevisionID: normalizeRevisionID, discardedWorkID: discardedWorkID,
		discardedRevisionID: discardedRevisionID, secondWorkID: secondWorkID,
		secondNormalizeRevisionID: secondNormalizeRevisionID, contextID: contextRecord.ID,
		referenceID: referenceID,
	}
}

// TestStoreListArticleAuthorsOrdersAndPaginates verifies ordered authorship
// pages, nullable fields, and the keyset continuation cursor.
func TestStoreListArticleAuthorsOrdersAndPaginates(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	page, err := fixture.store.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{RevisionID: fixture.normalizeRevisionID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 1 || !page.HasMore {
		t.Fatalf("first author page = %+v", page)
	}
	if page.Items[0].CitationName != "Doe, Jane" || page.Items[0].AuthorOrder != 1 || page.Items[0].Affiliation != "University" {
		t.Fatalf("first author = %+v", page.Items[0])
	}
	if page.Items[0].ORCID != "0000-0002-1825-0097" || page.Items[0].FirstName != "Jane" || page.Items[0].LastName != "Doe" {
		t.Fatalf("first author identity = %+v", page.Items[0])
	}
	if page.NextCursorID != page.Items[0].RelationID {
		t.Fatalf("next cursor = %d, want %d", page.NextCursorID, page.Items[0].RelationID)
	}

	second, err := fixture.store.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{
		RevisionID: fixture.normalizeRevisionID, CursorID: page.NextCursorID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 2 || len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second author page = %+v", second)
	}
	if second.Items[0].CitationName != "Smith, John" || second.Items[0].AuthorOrder != 2 || second.Items[0].Affiliation != "" {
		t.Fatalf("second author = %+v", second.Items[0])
	}
	if second.Items[0].PersonID != nil || second.Items[0].ORCID != "" {
		t.Fatalf("second author nullable identity = %+v", second.Items[0])
	}

	empty, err := fixture.store.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{RevisionID: fixture.secondNormalizeRevisionID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 || len(empty.Items) != 0 || empty.HasMore {
		t.Fatalf("empty author page = %+v", empty)
	}
}

// TestStoreListAuthorArticlesResolvesRunScopedArticles verifies the author
// detail article page scopes memberships to one run and maps optional fields.
func TestStoreListAuthorArticlesResolvesRunScopedArticles(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	var authorID int64
	if err := fixture.db.DB.QueryRow("SELECT id FROM author_occurrences WHERE citation_name='Doe, Jane'").Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	page, err := fixture.store.ListAuthorArticles(ctx, work.AuthorArticleFilter{AuthorOccurrenceID: authorID, RunID: fixture.runID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.HasMore {
		t.Fatalf("author articles = %d items total=%d, want 1", len(page.Items), page.Total)
	}
	item := page.Items[0]
	if item.WorkRevisionID != fixture.normalizeRevisionID || item.WorkID != fixture.workID || item.PipelineRunID != fixture.runID {
		t.Fatalf("author article identity = %+v", item)
	}
	if item.Title == nil || *item.Title != "Normalized article" || item.DOI == nil || *item.DOI != "10.1000/relationship" {
		t.Fatalf("author article metadata = %+v", item)
	}
	if item.Affiliation == nil || *item.Affiliation != "University" {
		t.Fatalf("author article affiliation = %+v", item)
	}

	otherRun, err := fixture.store.ListAuthorArticles(ctx, work.AuthorArticleFilter{AuthorOccurrenceID: authorID, RunID: fixture.runID + 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if otherRun.Total != 0 || len(otherRun.Items) != 0 {
		t.Fatalf("cross-run author articles = %+v, want none", otherRun)
	}
	if _, err := fixture.store.ListAuthorArticles(ctx, work.AuthorArticleFilter{AuthorOccurrenceID: authorID, RunID: fixture.runID, Limit: 0}); err == nil {
		t.Fatal("expected a limit validation error")
	}
}

// TestStoreGetArticleRevisionHonorsNormalizedVisibility verifies the
// normalized-revision rule and nullable field mapping.
func TestStoreGetArticleRevisionHonorsNormalizedVisibility(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	visible, err := fixture.store.GetArticleRevision(ctx, fixture.normalizeRevisionID, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if visible == nil {
		t.Fatal("current normalized revision was not visible")
	}
	if visible.Title != "Normalized article" || visible.Year != 2024 || visible.Journal != "Journal" ||
		visible.Source != "scopus" || visible.Abstract != "Abstract text" || visible.Keywords != `["kw"]` {
		t.Fatalf("visible revision = %+v", visible)
	}
	if visible.DOI != "10.1000/relationship" || visible.ProducerStage != work.ProducerStageNormalize {
		t.Fatalf("visible revision identity = %+v", visible)
	}

	superseded, err := fixture.store.GetArticleRevision(ctx, fixture.supersededRevisionID, fixture.runID)
	if err != nil || superseded != nil {
		t.Fatalf("superseded revision = %+v err=%v, want nil without error", superseded, err)
	}
	discarded, err := fixture.store.GetArticleRevision(ctx, fixture.discardedRevisionID, fixture.runID)
	if err != nil || discarded != nil {
		t.Fatalf("discarded revision = %+v err=%v, want nil without error", discarded, err)
	}
	parse, err := fixture.store.GetArticleRevision(ctx, fixture.parseRevisionID, fixture.runID)
	if err != nil || parse == nil {
		t.Fatalf("parse revision = %+v err=%v, want visible", parse, err)
	}
	if parse.Year != 2024 || parse.Abstract != "" || parse.Keywords != "" {
		t.Fatalf("parse revision nullable fields = %+v", parse)
	}
	missing, err := fixture.store.GetArticleRevision(ctx, 99999, fixture.runID)
	if err != nil || missing != nil {
		t.Fatalf("missing revision = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreGetArticleDetailWorkIDReportsVisibility verifies the detail guard
// returns the owning work or the typed not-found error.
func TestStoreGetArticleDetailWorkIDReportsVisibility(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	workID, err := fixture.store.GetArticleDetailWorkID(ctx, fixture.normalizeRevisionID, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if workID != fixture.workID {
		t.Fatalf("detail work id = %d, want %d", workID, fixture.workID)
	}
	if _, err := fixture.store.GetArticleDetailWorkID(ctx, fixture.supersededRevisionID, fixture.runID); err == nil {
		t.Fatal("expected the superseded revision to be rejected")
	}
	if _, err := fixture.store.GetArticleDetailWorkID(ctx, 99999, fixture.runID); err == nil {
		t.Fatal("expected the missing revision to be rejected")
	}
}

// TestStoreListArticleReferencesResolvesTargetAndPaginates verifies ordered
// reference pages with resolved-work context.
func TestStoreListArticleReferencesResolvesTargetAndPaginates(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	page, err := fixture.store.ListArticleReferences(ctx, work.ArticleReferencePageInput{RevisionID: fixture.normalizeRevisionID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.HasMore {
		t.Fatalf("reference page = %+v", page)
	}
	reference := page.Items[0]
	if reference.ID != fixture.referenceID || reference.MentionOrder != 1 || reference.DOI != "10.1000/relationship-second" {
		t.Fatalf("reference = %+v", reference)
	}
	if reference.ResolvedWorkID == nil || *reference.ResolvedWorkID != fixture.secondWorkID {
		t.Fatalf("resolved work = %+v", reference.ResolvedWorkID)
	}
	if reference.ResolvedRevisionID == nil || *reference.ResolvedRevisionID != fixture.secondNormalizeRevisionID {
		t.Fatalf("resolved revision = %+v", reference.ResolvedRevisionID)
	}
	if reference.ResolvedTitle != "Second article" {
		t.Fatalf("resolved title = %q", reference.ResolvedTitle)
	}

	empty, err := fixture.store.ListArticleReferences(ctx, work.ArticleReferencePageInput{RevisionID: fixture.secondNormalizeRevisionID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("empty reference page = %+v", empty)
	}
}

// TestStoreListArticleStagesPaginates verifies ordered stage pages for one work.
func TestStoreListArticleStagesPaginates(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	page, err := fixture.store.ListArticleStages(ctx, work.ArticleStagePageInput{
		RunID: fixture.runID, WorkID: fixture.workID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 1 || !page.HasMore {
		t.Fatalf("first stage page = %+v", page)
	}
	if page.Items[0].StageName != work.StageNameValidate || page.Items[0].Outcome != work.OutcomeValid {
		t.Fatalf("first stage = %+v", page.Items[0])
	}
	second, err := fixture.store.ListArticleStages(ctx, work.ArticleStagePageInput{
		RunID: fixture.runID, WorkID: fixture.workID, CursorID: page.NextCursorID, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 2 || len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second stage page = %+v", second)
	}
	if second.Items[0].StageName != work.StageNameNormalize || second.Items[0].Outcome != work.OutcomeNormalized {
		t.Fatalf("second stage = %+v", second.Items[0])
	}
}

// TestStoreGetReferenceDetailResolvesTarget verifies the reference detail
// projection and its not-found contract.
func TestStoreGetReferenceDetailResolvesTarget(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	detail, err := fixture.store.GetReferenceDetail(ctx, fixture.referenceID, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("reference detail was not found")
	}
	if detail.WorkID != fixture.workID || detail.CitingTitle != "Normalized article" || detail.PipelineRunID != fixture.runID {
		t.Fatalf("reference detail context = %+v", detail)
	}
	if detail.ResolvedRevisionID == nil || *detail.ResolvedRevisionID != fixture.secondNormalizeRevisionID {
		t.Fatalf("reference detail resolved revision = %+v", detail.ResolvedRevisionID)
	}
	if detail.ResolvedTitle != "Second article" || detail.RawReference != "raw reference" {
		t.Fatalf("reference detail = %+v", detail)
	}
	missing, err := fixture.store.GetReferenceDetail(ctx, 99999, fixture.runID)
	if err != nil || missing != nil {
		t.Fatalf("missing reference detail = %+v err=%v, want nil without error", missing, err)
	}
}

// TestStoreEvaluationSummaryAndFacets verifies invariant queue progress and facets.
func TestStoreEvaluationSummaryAndFacets(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	summary, err := fixture.store.EvaluationReviewSummary(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, AvailableDOIsJSON: "[]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Reviewed != 1 || summary.Unreviewed != 1 {
		t.Fatalf("evaluation summary = %+v", summary)
	}
	if summary.PDFAvailable != 0 || summary.PDFNotAvailable != 2 {
		t.Fatalf("evaluation pdf summary = %+v", summary)
	}
	if summary.PercentReviewed == nil || *summary.PercentReviewed != 50 {
		t.Fatalf("percent reviewed = %+v", summary.PercentReviewed)
	}
	if len(summary.Facets.ReviewStatus) != 2 {
		t.Fatalf("review status facets = %+v", summary.Facets.ReviewStatus)
	}
	if len(summary.Facets.Qualifier) != 1 || summary.Facets.Qualifier[0].Value != "out_of_scope" {
		t.Fatalf("qualifier facets = %+v", summary.Facets.Qualifier)
	}
	if len(summary.Facets.PDFStatus) != 2 || summary.Facets.PDFStatus[0].Value != "available" {
		t.Fatalf("pdf facets = %+v", summary.Facets.PDFStatus)
	}
}

// TestStoreListEvaluationFiltersAndOrders verifies the dynamic evaluation list.
func TestStoreListEvaluationFiltersAndOrders(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	page, err := fixture.store.ListEvaluation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, SortField: "title", Order: "ASC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("evaluation page = %+v", page)
	}
	if page.Items[0].Title != "Normalized article" || page.Items[1].Title != "Second article" {
		t.Fatalf("evaluation order = %+v", page.Items)
	}
	if page.Items[0].ReviewStatus != "not_approved" || page.Items[0].ReviewInherited {
		t.Fatalf("evaluation review state = %+v", page.Items[0])
	}
	if len(page.Items[0].ReviewSubStatuses) != 1 || page.Items[0].ReviewSubStatuses[0] != "out_of_scope" {
		t.Fatalf("evaluation sub-statuses = %+v", page.Items[0].ReviewSubStatuses)
	}
	if page.Items[1].ReviewStatus != "not_evaluated" || page.Items[1].ReviewVersionID != nil {
		t.Fatalf("unreviewed evaluation row = %+v", page.Items[1])
	}

	filtered, err := fixture.store.ListEvaluation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, ReviewStatus: "not_approved",
		SortField: "id", Order: "ASC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].Title != "Normalized article" {
		t.Fatalf("filtered evaluation page = %+v", filtered)
	}

	searched, err := fixture.store.ListEvaluation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, Query: "second",
		SortField: "id", Order: "ASC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if searched.Total != 1 || searched.Items[0].Title != "Second article" {
		t.Fatalf("searched evaluation page = %+v", searched)
	}
}

// TestStoreEvaluationQueueNavigation verifies adjacent unreviewed revisions.
func TestStoreEvaluationQueueNavigation(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	navigation, err := fixture.store.EvaluationQueueNavigation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if navigation.NextWorkRevisionID == nil || *navigation.NextWorkRevisionID != fixture.secondNormalizeRevisionID {
		t.Fatalf("queue navigation = %+v", navigation)
	}
	if navigation.PreviousWorkRevisionID == nil || *navigation.PreviousWorkRevisionID != fixture.secondNormalizeRevisionID {
		t.Fatalf("queue navigation previous = %+v", navigation.PreviousWorkRevisionID)
	}

	fromCurrent, err := fixture.store.EvaluationQueueNavigation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC",
		CurrentRevisionID: fixture.secondNormalizeRevisionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fromCurrent.PreviousWorkRevisionID != nil || fromCurrent.NextWorkRevisionID != nil {
		t.Fatalf("queue navigation from current = %+v", fromCurrent)
	}
	if _, err := fixture.store.EvaluationQueueNavigation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC",
		CurrentRevisionID: fixture.discardedRevisionID,
	}); err == nil {
		t.Fatal("expected the discarded revision to be rejected")
	}
}

// TestStoreGraphArticlesFilters verifies bounded graph article selection.
func TestStoreGraphArticlesFilters(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	articles, matches, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if matches != 2 || len(articles) != 2 {
		t.Fatalf("graph articles = %d matches=%d", len(articles), matches)
	}
	if articles[0].Title != "Normalized article" || articles[0].DOI != "10.1000/relationship" {
		t.Fatalf("first graph article = %+v", articles[0])
	}

	filtered, matches, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{
		RunID: fixture.runID, Query: "second", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if matches != 1 || len(filtered) != 1 || filtered[0].Title != "Second article" {
		t.Fatalf("filtered graph articles = %+v matches=%d", filtered, matches)
	}

	byAuthor, matches, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{
		RunID: fixture.runID, Author: "jane", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if matches != 1 || len(byAuthor) != 1 || byAuthor[0].Title != "Normalized article" {
		t.Fatalf("author graph articles = %+v matches=%d", byAuthor, matches)
	}
}

// TestStoreGraphRelationshipRows verifies bounded authorship, citation, and
// reference relationship rows.
func TestStoreGraphRelationshipRows(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	authorships, err := fixture.store.ListGraphAuthorships(ctx, []int64{fixture.normalizeRevisionID}, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(authorships) != 2 || authorships[0].CitationName != "Doe, Jane" || authorships[1].CitationName != "Smith, John" {
		t.Fatalf("graph authorships = %+v", authorships)
	}
	if authorships[0].ORCID != "0000-0002-1825-0097" || authorships[0].Affiliation != "University" {
		t.Fatalf("graph authorship identity = %+v", authorships[0])
	}

	citations, err := fixture.store.ListGraphCitations(ctx, []int64{fixture.normalizeRevisionID}, []int64{fixture.secondWorkID}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(citations) != 1 || citations[0].WorkRevisionID != fixture.normalizeRevisionID || citations[0].ResolvedWorkID != fixture.secondWorkID {
		t.Fatalf("graph citations = %+v", citations)
	}

	references, err := fixture.store.ListGraphReferences(ctx, []int64{fixture.normalizeRevisionID}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 1 || references[0].ID != fixture.referenceID || references[0].Title != "Second article" {
		t.Fatalf("graph references = %+v", references)
	}

	empty, err := fixture.store.ListGraphAuthorships(ctx, nil, 10, 10)
	if err != nil || empty != nil {
		t.Fatalf("empty graph authorships = %+v err=%v, want nil without error", empty, err)
	}
}

// TestStoreCurrentCoverageAndRelationshipTotals verifies run coverage and totals.
func TestStoreCurrentCoverageAndRelationshipTotals(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	coverage, err := fixture.store.CurrentCoverage(ctx, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.WorkRevisions != 5 || coverage.JournalCoverage != 3 {
		t.Fatalf("coverage = %+v", coverage)
	}

	totals, err := fixture.store.RelationshipTotals(ctx, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if totals.WorkRevisions != 5 || totals.AnalysisReadyArticles != 2 {
		t.Fatalf("relationship totals = %+v", totals)
	}
	if totals.Authorships != 2 || totals.ReferenceMentions != 1 || totals.InternalCitations != 1 {
		t.Fatalf("relationship totals relationships = %+v", totals)
	}
}

// TestStoreRunStageSummaries verifies canonical stage ordering and outcome counts.
func TestStoreRunStageSummaries(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	summaries, err := fixture.store.RunStageSummaries(ctx, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("stage summaries = %+v", summaries)
	}
	if summaries[0].StageName != work.StageNameValidate || summaries[0].TotalRecords != 3 {
		t.Fatalf("validate summary = %+v", summaries[0])
	}
	if summaries[0].Outcomes[work.OutcomeValid] != 2 || summaries[0].Outcomes[work.OutcomeDiscarded] != 1 {
		t.Fatalf("validate outcomes = %+v", summaries[0].Outcomes)
	}
	if summaries[1].StageName != work.StageNameNormalize || summaries[1].TotalRecords != 2 {
		t.Fatalf("normalize summary = %+v", summaries[1])
	}
}

// TestStoreListCorpusReferencesSortsAndSearches verifies the dynamic reference page.
func TestStoreListCorpusReferencesSortsAndSearches(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	page, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{
		RunID: fixture.runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != fixture.referenceID {
		t.Fatalf("corpus references = %+v", page)
	}
	if page.Items[0].CitingTitle != "Normalized article" || page.Items[0].ResolvedWorkID == nil {
		t.Fatalf("corpus reference projection = %+v", page.Items[0])
	}

	searched, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{
		RunID: fixture.runID, Query: "second", Sort: "title", Order: "DESC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if searched.Total != 1 || len(searched.Items) != 1 {
		t.Fatalf("searched corpus references = %+v", searched)
	}
	if _, err := fixture.store.ListCorpusReferences(ctx, work.CorpusReferenceFilter{
		RunID: fixture.runID, Sort: "bogus", Order: "ASC", Page: 1, PerPage: 10,
	}); err == nil {
		t.Fatal("expected an unsupported sort field to fail")
	}
}

// TestStoreListRunStagesSortsAndSearches verifies the dynamic run-stage page.
func TestStoreListRunStagesSortsAndSearches(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	page, err := fixture.store.ListRunStages(ctx, work.RunStageFilter{
		RunID: fixture.runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 || len(page.Items) != 5 {
		t.Fatalf("run stages = %+v", page)
	}
	if page.Items[0].StageName != work.StageNameValidate || page.Items[0].Outcome != work.OutcomeValid {
		t.Fatalf("first run stage = %+v", page.Items[0])
	}

	searched, err := fixture.store.ListRunStages(ctx, work.RunStageFilter{
		RunID: fixture.runID, Query: "discarded", Sort: "outcome", Order: "DESC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if searched.Total != 1 || len(searched.Items) != 1 || searched.Items[0].Outcome != work.OutcomeDiscarded {
		t.Fatalf("searched run stages = %+v", searched)
	}
	if _, err := fixture.store.ListRunStages(ctx, work.RunStageFilter{
		RunID: fixture.runID, Sort: "bogus", Order: "ASC", Page: 1, PerPage: 10,
	}); err == nil {
		t.Fatal("expected an unsupported sort field to fail")
	}
}

// TestStoreListEvaluationFilterVariants verifies every supported evaluation
// filter branch and the rejected enum values.
func TestStoreListEvaluationFilterVariants(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()
	base := work.EvaluationFilter{RunID: fixture.runID, ContextID: fixture.contextID, SortField: "id", Order: "ASC", Page: 1, PerPage: 10}

	cases := []struct {
		name  string
		apply func(filter *work.EvaluationFilter)
		total int64
	}{
		{"source", func(filter *work.EvaluationFilter) { filter.Source = "ieee" }, 1},
		{"source not recorded", func(filter *work.EvaluationFilter) { filter.Source = "not_recorded" }, 0},
		{"qualifier", func(filter *work.EvaluationFilter) { filter.Qualifier = "out_of_scope" }, 1},
		{"review source this context", func(filter *work.EvaluationFilter) { filter.ReviewSource = "this_context" }, 1},
		{"review source inherited", func(filter *work.EvaluationFilter) { filter.ReviewSource = "inherited" }, 0},
		{"review source not started", func(filter *work.EvaluationFilter) { filter.ReviewSource = "not_started" }, 1},
		{"reviewed", func(filter *work.EvaluationFilter) { filter.Reviewed = "reviewed" }, 1},
		{"unreviewed", func(filter *work.EvaluationFilter) { filter.Reviewed = "unreviewed" }, 1},
		{"pdf available", func(filter *work.EvaluationFilter) {
			filter.PDFStatus = "available"
			filter.AvailableDOIsJSON = "[]"
		}, 0},
		{"pdf not available", func(filter *work.EvaluationFilter) {
			filter.PDFStatus = "not_available"
			filter.AvailableDOIsJSON = "[]"
		}, 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			filter := base
			test.apply(&filter)
			page, err := fixture.store.ListEvaluation(ctx, filter)
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != test.total {
				t.Fatalf("total = %d, want %d", page.Total, test.total)
			}
		})
	}

	invalid := []struct {
		name  string
		apply func(filter *work.EvaluationFilter)
	}{
		{"invalid review source", func(filter *work.EvaluationFilter) { filter.ReviewSource = "bogus" }},
		{"invalid reviewed", func(filter *work.EvaluationFilter) { filter.Reviewed = "bogus" }},
		{"invalid sort", func(filter *work.EvaluationFilter) { filter.SortField = "bogus" }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			filter := base
			test.apply(&filter)
			if _, err := fixture.store.ListEvaluation(ctx, filter); err == nil {
				t.Fatal("expected an invalid filter to fail")
			}
		})
	}
}

// TestStoreGraphArticleFilterVariants verifies every supported graph filter branch.
func TestStoreGraphArticleFilterVariants(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()
	yearMin, yearMax := int64(2024), int64(2023)
	citationMin, citationMax := int64(3), int64(3)
	referenceMin, referenceMax := int64(1), int64(1)

	cases := []struct {
		name   string
		filter work.GraphFilter
		total  int
	}{
		{"source", work.GraphFilter{RunID: fixture.runID, Source: "scopus", Limit: 10}, 1},
		{"year min", work.GraphFilter{RunID: fixture.runID, YearMin: &yearMin, Limit: 10}, 1},
		{"year max", work.GraphFilter{RunID: fixture.runID, YearMax: &yearMax, Limit: 10}, 1},
		{"citation min", work.GraphFilter{RunID: fixture.runID, CitationMin: &citationMin, Limit: 10}, 1},
		{"citation max", work.GraphFilter{RunID: fixture.runID, CitationMax: &citationMax, Limit: 10}, 1},
		{"reference min", work.GraphFilter{RunID: fixture.runID, ReferenceMin: &referenceMin, Limit: 10}, 1},
		{"reference max", work.GraphFilter{RunID: fixture.runID, ReferenceMax: &referenceMax, Limit: 10}, 1},
		{"orcid", work.GraphFilter{RunID: fixture.runID, ORCID: "0000-0002-1825-0097", Limit: 10}, 1},
		{"reference", work.GraphFilter{RunID: fixture.runID, Reference: "second", Limit: 10}, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			articles, matches, err := fixture.store.ListGraphArticles(ctx, test.filter)
			if err != nil {
				t.Fatal(err)
			}
			if matches != test.total || len(articles) != test.total {
				t.Fatalf("matches = %d articles = %d, want %d", matches, len(articles), test.total)
			}
		})
	}
}

// TestStoreRelationshipReadLimits verifies the page reads reject non-positive limits.
func TestStoreRelationshipReadLimits(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	if _, err := fixture.store.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{RevisionID: fixture.normalizeRevisionID}); err == nil {
		t.Fatal("expected a non-positive author limit to fail")
	}
	if _, err := fixture.store.ListArticleReferences(ctx, work.ArticleReferencePageInput{RevisionID: fixture.normalizeRevisionID}); err == nil {
		t.Fatal("expected a non-positive reference limit to fail")
	}
	if _, err := fixture.store.ListArticleStages(ctx, work.ArticleStagePageInput{RunID: fixture.runID, WorkID: fixture.workID}); err == nil {
		t.Fatal("expected a non-positive stage limit to fail")
	}
	if _, err := fixture.store.ListGraphAuthorships(ctx, []int64{fixture.normalizeRevisionID}, 0, 10); err == nil {
		t.Fatal("expected a non-positive authorship limit to fail")
	}
	if _, err := fixture.store.ListGraphCitations(ctx, []int64{fixture.normalizeRevisionID}, []int64{fixture.secondWorkID}, 0); err == nil {
		t.Fatal("expected a non-positive citation limit to fail")
	}
	if _, err := fixture.store.ListGraphReferences(ctx, []int64{fixture.normalizeRevisionID}, 0); err == nil {
		t.Fatal("expected a non-positive reference limit to fail")
	}
}

// TestStoreEvaluationSummaryEmptyRun verifies the zero-denominator percentage contract.
func TestStoreEvaluationSummaryEmptyRun(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "empty-evaluation")

	summary, err := store.EvaluationReviewSummary(ctx, work.EvaluationFilter{RunID: runID, ContextID: 0, AvailableDOIsJSON: "[]"})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 0 || summary.PercentReviewed != nil {
		t.Fatalf("empty evaluation summary = %+v", summary)
	}
}
