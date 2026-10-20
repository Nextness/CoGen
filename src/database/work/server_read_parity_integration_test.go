// server_read_parity_integration_test.go verifies the work family reads return
// the same values as the viewer's former raw queries for normalized revisions,
// ordered authors, references, graph edges, evaluation rows, and term-match
// payloads. Comparisons preserve SQL NULL, empty text, and zero independently.
//go:build integration

package work_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"testing"

	"analysis/database/work"
)

// legacyNormalizedPredicate is the viewer's former normalized-revision rule.
const legacyNormalizedPredicate = `wr.producer_stage='normalize'
	AND wr.id=(SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
		WHERE normalized_candidate.pipeline_run_id=wr.pipeline_run_id
		AND normalized_candidate.work_id=wr.work_id AND normalized_candidate.producer_stage='normalize')
	AND EXISTS (SELECT 1 FROM run_work_stages current_validation
		WHERE current_validation.pipeline_run_id=wr.pipeline_run_id
		AND current_validation.work_id=wr.work_id
		AND current_validation.stage_name='validate' AND current_validation.outcome='valid')`

// TestWorkReadParityWithLegacyServerQueries verifies the family reads match the
// viewer's former raw queries on one migrated fixture.
func TestWorkReadParityWithLegacyServerQueries(t *testing.T) {
	fixture := seedRelationshipFixture(t)
	ctx := context.Background()

	// Normalized revision parity: the former article detail query.
	var legacyRevisionID, legacyWorkID int64
	var legacyTitle, legacyDOI string
	if err := fixture.db.RawDBForTest().QueryRowContext(ctx, `SELECT wr.id, wr.work_id, wr.title, w.doi
		FROM work_revisions wr JOIN works w ON w.id=wr.work_id
		WHERE wr.id=? AND wr.pipeline_run_id=? AND (wr.producer_stage!='normalize' OR (`+legacyNormalizedPredicate+`))`,
		fixture.normalizeRevisionID, fixture.runID).Scan(&legacyRevisionID, &legacyWorkID, &legacyTitle, &legacyDOI); err != nil {
		t.Fatal(err)
	}
	revision, err := fixture.store.GetArticleRevision(ctx, fixture.normalizeRevisionID, fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if revision == nil || revision.ID != legacyRevisionID || revision.WorkID != legacyWorkID || !pointsTo(revision.Title, legacyTitle) || !pointsTo(revision.DOI, legacyDOI) {
		t.Fatalf("revision parity: legacy=(%d,%d,%q,%q) family=%+v", legacyRevisionID, legacyWorkID, legacyTitle, legacyDOI, revision)
	}

	// Ordered author parity: the former article author collection query.
	legacyAuthors := queryRowValues(t, fixture.db.RawDBForTest(), `SELECT ao.id, ao.person_id, ao.citation_name, ao.first_name, ao.last_name, ao.orcid, a.author_order, a.affiliation
		FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id
		WHERE a.work_revision_id=? ORDER BY a.id`, fixture.normalizeRevisionID)
	authorPage, err := fixture.store.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{RevisionID: fixture.normalizeRevisionID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	familyAuthors := make([][]string, 0, len(authorPage.Items))
	for _, author := range authorPage.Items {
		familyAuthors = append(familyAuthors, []string{
			formatDatabaseValue(author.ID), familyValue(author.PersonID), familyValue(author.CitationName),
			familyValue(author.FirstName), familyValue(author.LastName), familyValue(author.ORCID),
			formatDatabaseValue(author.AuthorOrder), familyValue(author.Affiliation),
		})
	}
	if !rowsEqual(legacyAuthors, familyAuthors) {
		t.Fatalf("author parity:\nlegacy=%v\nfamily=%v", legacyAuthors, familyAuthors)
	}

	// Reference parity: the former article reference collection query.
	legacyReferences := queryRowValues(t, fixture.db.RawDBForTest(), `SELECT rm.id, rm.work_revision_id, rm.resolved_work_id, rm.mention_order, rm.doi, rm.title, rm.author, rm.year, rm.source, rm.created_at,
			target.id AS resolved_revision_id, target.title AS resolved_title
		FROM reference_mentions rm JOIN work_revisions source ON source.id=rm.work_revision_id
		LEFT JOIN work_revisions target ON target.id=(SELECT candidate.id FROM work_revisions candidate
			WHERE candidate.work_id=rm.resolved_work_id AND candidate.pipeline_run_id=source.pipeline_run_id
			AND candidate.producer_stage='normalize'
			AND candidate.id=(SELECT MAX(normalized_candidate.id) FROM work_revisions normalized_candidate
				WHERE normalized_candidate.pipeline_run_id=candidate.pipeline_run_id
				AND normalized_candidate.work_id=candidate.work_id AND normalized_candidate.producer_stage='normalize')
			AND EXISTS (SELECT 1 FROM run_work_stages current_validation
				WHERE current_validation.pipeline_run_id=candidate.pipeline_run_id
				AND current_validation.work_id=candidate.work_id
				AND current_validation.stage_name='validate' AND current_validation.outcome='valid')
			LIMIT 1)
		WHERE rm.work_revision_id=? ORDER BY rm.id`, fixture.normalizeRevisionID)
	referencePage, err := fixture.store.ListArticleReferences(ctx, work.ArticleReferencePageInput{RevisionID: fixture.normalizeRevisionID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	familyReferences := make([][]string, 0, len(referencePage.Items))
	for _, reference := range referencePage.Items {
		familyReferences = append(familyReferences, []string{
			formatDatabaseValue(reference.ID), formatDatabaseValue(reference.WorkRevisionID), familyValue(reference.ResolvedWorkID),
			formatDatabaseValue(reference.MentionOrder), familyValue(reference.DOI), familyValue(reference.Title), familyValue(reference.Author),
			familyValue(reference.Year), familyValue(reference.Source), familyValue(reference.CreatedAt),
			familyValue(reference.ResolvedRevisionID), familyValue(reference.ResolvedTitle),
		})
	}
	if !rowsEqual(legacyReferences, familyReferences) {
		t.Fatalf("reference parity:\nlegacy=%v\nfamily=%v", legacyReferences, familyReferences)
	}

	// Graph article parity: the former graph article query.
	legacyGraphArticles := queryRowValues(t, fixture.db.RawDBForTest(), `SELECT wr.id, wr.work_id, wr.title, wr.year, wr.source, w.doi
		FROM work_revisions wr JOIN works w ON w.id=wr.work_id
		WHERE wr.pipeline_run_id=? AND `+legacyNormalizedPredicate+` ORDER BY wr.id LIMIT 10`, fixture.runID)
	graphArticles, _, err := fixture.store.ListGraphArticles(ctx, work.GraphFilter{RunID: fixture.runID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	familyGraphArticles := make([][]string, 0, len(graphArticles))
	for _, article := range graphArticles {
		familyGraphArticles = append(familyGraphArticles, []string{
			formatDatabaseValue(article.ID), formatDatabaseValue(article.WorkID), familyValue(article.Title),
			familyValue(article.Year), familyValue(article.Source), familyValue(article.DOI),
		})
	}
	if !rowsEqual(legacyGraphArticles, familyGraphArticles) {
		t.Fatalf("graph article parity:\nlegacy=%v\nfamily=%v", legacyGraphArticles, familyGraphArticles)
	}

	// Graph authorship parity: the former article-author edge query.
	legacyGraphAuthorships := queryRowValues(t, fixture.db.RawDBForTest(), `WITH eligible_authors AS (
            SELECT a.author_occurrence_id FROM authorships a
            WHERE a.work_revision_id IN (?)
            GROUP BY a.author_occurrence_id ORDER BY MIN(a.id), a.author_occurrence_id LIMIT ?
        ) SELECT a.work_revision_id, ao.id AS author_id, ao.citation_name, ao.orcid, a.author_order, a.affiliation
            FROM authorships a JOIN eligible_authors ea ON ea.author_occurrence_id=a.author_occurrence_id
            JOIN author_occurrences ao ON ao.id=a.author_occurrence_id
            WHERE a.work_revision_id IN (?) ORDER BY a.id LIMIT ?`, fixture.normalizeRevisionID, 10, fixture.normalizeRevisionID, 11)
	graphAuthorships, err := fixture.store.ListGraphAuthorships(ctx, []int64{fixture.normalizeRevisionID}, 10, 11)
	if err != nil {
		t.Fatal(err)
	}
	familyGraphAuthorships := make([][]string, 0, len(graphAuthorships))
	for _, authorship := range graphAuthorships {
		familyGraphAuthorships = append(familyGraphAuthorships, []string{
			formatDatabaseValue(authorship.WorkRevisionID), formatDatabaseValue(authorship.AuthorID), familyValue(authorship.CitationName),
			familyValue(authorship.ORCID), formatDatabaseValue(authorship.AuthorOrder), familyValue(authorship.Affiliation),
		})
	}
	if !rowsEqual(legacyGraphAuthorships, familyGraphAuthorships) {
		t.Fatalf("graph authorship parity:\nlegacy=%v\nfamily=%v", legacyGraphAuthorships, familyGraphAuthorships)
	}

	// Evaluation row parity: the former evaluation list query.
	legacyEvaluation := queryRowValues(t, fixture.db.RawDBForTest(), `SELECT wr.id, wr.work_id, wr.title, wr.year, wr.journal, wr.publisher, wr.source, w.doi,
			validation.outcome AS validation_status, wr.citation_count, wr.reference_count, wr.producer_stage, wr.created_at,
			wr.abstract, wr.keywords, wr.keywords_plus,
			(SELECT GROUP_CONCAT(ao.citation_name, '; ') FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id WHERE a.work_revision_id=wr.id ORDER BY a.author_order) AS authors,
			wr.id AS work_revision_id, COALESCE(review.status, 'not_evaluated') AS review_status,
			CASE WHEN review.id IS NOT NULL AND review.created_in_context_id!=? THEN 1 ELSE 0 END AS review_inherited,
			review.id AS review_version_id, review.created_in_context_id AS review_created_in_context_id
		FROM work_revisions wr JOIN works w ON w.id=wr.work_id
		LEFT JOIN run_work_stages validation ON validation.pipeline_run_id=wr.pipeline_run_id
			AND validation.work_id=wr.work_id AND validation.stage_name='validate'
		LEFT JOIN review_context_work_heads review_head ON review_head.review_context_id=? AND review_head.work_id=wr.work_id
		LEFT JOIN work_review_versions review ON review.id=review_head.review_version_id
		WHERE wr.pipeline_run_id=? AND `+legacyNormalizedPredicate+`
		ORDER BY COALESCE(wr.title, '') ASC, wr.id ASC LIMIT ? OFFSET ?`, fixture.contextID, fixture.contextID, fixture.runID, 10, 0)
	evaluationPage, err := fixture.store.ListEvaluation(ctx, work.EvaluationFilter{
		RunID: fixture.runID, ContextID: fixture.contextID, SortField: "title", Order: "ASC", Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	familyEvaluation := make([][]string, 0, len(evaluationPage.Items))
	for _, row := range evaluationPage.Items {
		familyEvaluation = append(familyEvaluation, []string{
			formatDatabaseValue(row.ID), formatDatabaseValue(row.WorkID), familyValue(row.Title), familyValue(row.Year),
			familyValue(row.Journal), familyValue(row.Publisher), familyValue(row.Source), familyValue(row.DOI), familyValue(row.ValidationStatus),
			familyValue(row.CitationCount), familyValue(row.ReferenceCount), familyValue(row.ProducerStage), familyValue(row.CreatedAt),
			familyValue(row.Abstract), familyValue(row.Keywords), familyValue(row.KeywordsPlus), familyValue(row.Authors), formatDatabaseValue(row.WorkRevisionID),
			familyValue(row.ReviewStatus), formatDatabaseValue(boolInt(row.ReviewInherited)), familyValue(row.ReviewVersionID),
			familyValue(row.ReviewCreatedInContextID),
		})
	}
	if !rowsEqual(legacyEvaluation, familyEvaluation) {
		t.Fatalf("evaluation parity:\nlegacy=%v\nfamily=%v", legacyEvaluation, familyEvaluation)
	}

	// Term-match parity: the former per-revision match query. The family groups
	// matches by field, so both sides are sorted by (field, term).
	legacyMatches := queryRowValues(t, fixture.db.RawDBForTest(), `SELECT field, term FROM work_revision_term_matches
		WHERE pipeline_run_id=? AND work_revision_id=? ORDER BY id`, fixture.runID, fixture.normalizeRevisionID)
	familyMatches, err := fixture.store.GetRevisionMatches(ctx, fixture.runID, fixture.normalizeRevisionID)
	if err != nil {
		t.Fatal(err)
	}
	familyMatchRows := make([][]string, 0)
	for _, field := range []string{"title", "abstract", "keywords", "keywords_plus"} {
		for _, term := range familyMatches[field] {
			familyMatchRows = append(familyMatchRows, []string{formatDatabaseValue(field), formatDatabaseValue(term)})
		}
	}
	sortRows(legacyMatches)
	sortRows(familyMatchRows)
	if !rowsEqual(legacyMatches, familyMatchRows) {
		t.Fatalf("term-match parity:\nlegacy=%v\nfamily=%v", legacyMatches, familyMatchRows)
	}
}

// rowsEqual reports whether two rendered row sets are identical.
func rowsEqual(left, right [][]string) bool {
	return slices.EqualFunc(left, right, func(a, b []string) bool {
		return slices.Equal(a, b)
	})
}

// sortRows sorts two-column match rows by field and term for comparison.
func sortRows(rows [][]string) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i][0] != rows[j][0] {
			return rows[i][0] < rows[j][0]
		}
		return rows[i][1] < rows[j][1]
	})
}

// familyValue renders optional or required application values with exact
// JSON null, string, and number identity for the legacy query comparison.
func familyValue(value any) string {
	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return formatDatabaseValue(nil)
		}
		value = reflected.Elem().Interface()
	}
	return formatDatabaseValue(value)
}

// boolInt renders a boolean as the legacy integer representation.
func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

// queryRowValues renders every row of one raw query as ordered comparable text.
func queryRowValues(t *testing.T, db *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	if bytes, ok := value.([]byte); ok {
		value = string(bytes)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
