// details.go provides the article detail endpoint that returns the
// full metadata, authors, and references for a single immutable
// work revision identified by its numeric ID.
package server

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
)

const detailCollectionPreviewLimit = 25

const articleWorkRunRevisionIDsSQL = `SELECT CAST(id AS TEXT)
	FROM work_revisions
	WHERE work_id=? AND pipeline_run_id=?`

const articleWorkRunReviewVersionIDsSQL = `SELECT CAST(review.id AS TEXT)
	FROM work_review_versions review
	JOIN work_revisions revision ON revision.id=review.work_revision_id
	WHERE review.work_id=? AND revision.pipeline_run_id=?`

const articleWorkRunNoteVersionIDsSQL = `SELECT CAST(version.id AS TEXT)
	FROM review_note_versions version
	JOIN review_notes note ON note.id=version.note_id
	JOIN review_contexts context ON context.id=version.created_in_context_id
	WHERE note.work_id=? AND context.pipeline_run_id=?`

const articleWorkRunAnchorVersionIDsSQL = `SELECT CAST(version.id AS TEXT)
	FROM review_anchor_versions version
	JOIN review_anchors anchor ON anchor.id=version.anchor_id
	JOIN review_contexts context ON context.id=version.created_in_context_id
	WHERE anchor.work_id=? AND context.pipeline_run_id=?`

// articleDetail treats the numeric route identifier as an immutable work
// revision ID. It intentionally does not expose the retired mutable articles
// projection.
func (s *Server) articleDetail(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	if err := validateKnownQuery(r, "run_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := requiredQueryID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	id, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	revision, err := s.oneRow(ctx, `SELECT wr.*, w.doi FROM work_revisions wr JOIN works w ON w.id=wr.work_id
		WHERE wr.id=? AND wr.pipeline_run_id=? AND (wr.producer_stage!='normalize' OR (`+currentNormalizedRevisionPredicate("wr")+`))`, id, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if revision == nil {
		s.respond(w, r, nil, notFound("article revision not found"))
		return
	}
	workID := revision["work_id"].(int64)
	authors, err := s.articleDetailCollectionData(ctx, id, workID, runID, "authors", "article_detail_authors_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	references, err := s.articleDetailCollectionData(ctx, id, workID, runID, "references", "article_detail_references_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	stageOutcomes, err := s.articleDetailCollectionData(ctx, id, workID, runID, "stages", "article_detail_stages_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	audit, err := s.articleDetailCollectionData(ctx, id, workID, runID, "audit", "article_detail_audit_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	enrichmentSummary, err := s.articleEnrichmentSummary(ctx, workID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	pdfStatus, err := s.pdfStatusForWork(ctx, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	reviewContext, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	termMatches := map[string]any(nil)
	if revision["producer_stage"] == "normalize" {
		termRows, termTotal, err := s.runSearchTerms(ctx, runID)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		if len(termRows) > 0 {
			revisionMatches, err := s.revisionTermMatches(ctx, runID, id)
			if err != nil {
				s.respond(w, r, nil, err)
				return
			}
			termMatches = detailTermMatches(termRows, termTotal, revisionMatches)
		}
	}
	s.respond(w, r, map[string]any{"article": revision, "authors": authors, "references": references, "stage_outcomes": stageOutcomes, "audit_events": audit, "enrichment_summary": enrichmentSummary, "pdf_status": pdfStatus, "review_context": reviewContext, "review_context_initialized": reviewContext != nil, "term_matches": termMatches}, nil)
}

// authorDetail returns one author occurrence with its articles, audit evidence, and optional run-scoped identity candidates.
func (s *Server) authorDetail(w http.ResponseWriter, r *http.Request) {
	id, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "run_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := requiredQueryID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	author, err := s.oneRow(ctx, `SELECT ao.*, p.orcid AS person_orcid
		FROM author_occurrences ao LEFT JOIN people p ON p.id=ao.person_id
		WHERE ao.id=? AND EXISTS (
			SELECT 1 FROM authorships membership
			JOIN work_revisions revision ON revision.id=membership.work_revision_id
			WHERE membership.author_occurrence_id=ao.id AND revision.pipeline_run_id=?
		)`, id, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if author == nil {
		s.respond(w, r, nil, notFound("author occurrence not found"))
		return
	}
	articles, err := s.authorDetailCollectionData(ctx, id, runID, "articles", "author_detail_articles_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	audit, err := s.authorDetailCollectionData(ctx, id, runID, "audit", "author_detail_audit_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	identityEvidence, err := s.authorDetailCollectionData(ctx, id, runID, "identity", "author_detail_identity_"+stringID(runID)+"_"+stringID(id), 0, detailCollectionPreviewLimit)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]any{"author": author, "articles": articles, "audit_events": audit, "identity_evidence": identityEvidence}, nil)
}

// referenceDetail returns one reference mention with its citing and resolved-work context.
func (s *Server) referenceDetail(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "run_id"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := requiredQueryID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	id, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	mention, err := s.oneRow(ctx, `SELECT rm.*, wr.work_id, wr.title AS citing_title, wr.pipeline_run_id,
        target.id AS resolved_revision_id, target.title AS resolved_title
        FROM reference_mentions rm JOIN work_revisions wr ON wr.id=rm.work_revision_id
		LEFT JOIN work_revisions target ON target.id=(SELECT candidate.id FROM work_revisions candidate
			WHERE candidate.work_id=rm.resolved_work_id AND candidate.pipeline_run_id=wr.pipeline_run_id
			AND `+currentNormalizedRevisionPredicate("candidate")+` LIMIT 1)
		WHERE rm.id=? AND wr.pipeline_run_id=?`, id, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if mention == nil {
		s.respond(w, r, nil, notFound("reference mention not found"))
		return
	}
	s.respond(w, r, map[string]any{"reference": mention}, nil)
}

// articleEnrichmentSummary returns a bounded set of provider and field labels without transferring event payloads.
func (s *Server) articleEnrichmentSummary(ctx context.Context, workID, runID int64) (map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT
		CASE WHEN json_valid(metadata_json) THEN COALESCE(json_extract(metadata_json, '$.provider'), '') ELSE '' END AS provider,
		CASE WHEN json_valid(metadata_json) THEN COALESCE(json_extract(metadata_json, '$.field'), '') ELSE '' END AS field
		FROM audit_events WHERE entity_type='work_revision' AND action='field_enriched'
		AND entity_id IN (`+articleWorkRunRevisionIDsSQL+`)
		ORDER BY provider, field LIMIT 101`, workID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	providers, fields := make([]string, 0), make([]string, 0)
	providerSeen, fieldSeen := make(map[string]bool), make(map[string]bool)
	truncated := false
	count := 0
	for rows.Next() {
		count++
		if count > 100 {
			truncated = true
			break
		}
		var provider, field string
		if err := rows.Scan(&provider, &field); err != nil {
			return nil, err
		}
		if provider != "" && !providerSeen[provider] {
			providerSeen[provider] = true
			providers = append(providers, provider)
		}
		if field != "" && !fieldSeen[field] {
			fieldSeen[field] = true
			fields = append(fields, field)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"providers": providers, "fields": fields, "truncated": truncated, "pair_limit": 100}, nil
}

// articleDetailCollection returns one bounded page of a large article relationship or event collection.
func (s *Server) articleDetailCollection(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "run_id", "limit", "cursor"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := hierarchyRequiredID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	revisionID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	kind := r.PathValue("kind")
	cursorKind := "article_detail_" + kind + "_" + stringID(runID) + "_" + stringID(revisionID)
	cursorID, limit, err := reviewIDPage(r, cursorKind)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	workID, err := s.articleDetailWorkID(ctx, revisionID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	result, err := s.articleDetailCollectionData(ctx, revisionID, workID, runID, kind, cursorKind, cursorID, limit)
	s.respond(w, r, result, err)
}

// authorDetailCollection returns one bounded page of run-owned author relationships or evidence.
func (s *Server) authorDetailCollection(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "run_id", "limit", "cursor"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, err := hierarchyRequiredID(r, "run_id")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	authorID, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	kind := r.PathValue("kind")
	cursorKind := "author_detail_" + kind + "_" + stringID(runID) + "_" + stringID(authorID)
	cursorID, limit, err := reviewIDPage(r, cursorKind)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM author_occurrences occurrence WHERE occurrence.id=? AND EXISTS (
		SELECT 1 FROM authorships membership JOIN work_revisions revision ON revision.id=membership.work_revision_id
		WHERE membership.author_occurrence_id=occurrence.id AND revision.pipeline_run_id=?)`, authorID, runID).Scan(&exists); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("author occurrence not found"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	result, err := s.authorDetailCollectionData(ctx, authorID, runID, kind, cursorKind, cursorID, limit)
	s.respond(w, r, result, err)
}

// articleDetailWorkID validates one visible article revision and returns its owning work.
func (s *Server) articleDetailWorkID(ctx context.Context, revisionID, runID int64) (int64, error) {
	var workID int64
	err := s.db.QueryRowContext(ctx, `SELECT wr.work_id FROM work_revisions wr
		WHERE wr.id=? AND wr.pipeline_run_id=? AND (wr.producer_stage!='normalize' OR (`+currentNormalizedRevisionPredicate("wr")+`))`, revisionID, runID).Scan(&workID)
	if err == sql.ErrNoRows {
		return 0, notFound("article revision not found")
	}
	return workID, err
}

// detailCollectionEnvelope executes one ID-keyset query with an exact count and one-row continuation sentinel.
func (s *Server) detailCollectionEnvelope(ctx context.Context, kind, fromWhere, orderID string, args []any, cursorID int64, descending bool, limit int) (map[string]any, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) "+fromWhere, args...).Scan(&total); err != nil {
		return nil, err
	}
	queryArgs := append([]any(nil), args...)
	operator, direction := ">", "ASC"
	if descending {
		operator, direction = "<", "DESC"
	}
	query := "SELECT " + orderID + " AS collection_cursor_id, detail_rows.* FROM (SELECT * " + fromWhere + ") detail_rows"
	if cursorID > 0 {
		query += " WHERE " + orderID + operator + "?"
		queryArgs = append(queryArgs, cursorID)
	}
	query += " ORDER BY " + orderID + " " + direction + " LIMIT ?"
	queryArgs = append(queryArgs, limit+1)
	items, err := s.rows(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	var nextCursor any
	if hasMore {
		cursor, ok := items[len(items)-1]["collection_cursor_id"].(int64)
		if !ok || cursor < 1 {
			return nil, &apiProblem{Status: http.StatusInternalServerError, Code: "internal_error", Message: "detail collection has an invalid cursor"}
		}
		nextCursor = encodeReviewCursor(reviewCursor{Kind: kind, ID: cursor})
	}
	for _, item := range items {
		delete(item, "collection_cursor_id")
		delete(item, "relation_id")
	}
	return map[string]any{"items": items, "total": total, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil
}

// articleDetailCollectionData defines the fixed projections for article detail subresources.
func (s *Server) articleDetailCollectionData(ctx context.Context, revisionID, workID, runID int64, collection, cursorKind string, cursorID int64, limit int) (map[string]any, error) {
	var fromWhere, orderID string
	var args []any
	descending := false
	switch collection {
	case "authors":
		fromWhere = `FROM (SELECT a.id AS relation_id, ao.id, ao.person_id, ao.citation_name, ao.first_name, ao.last_name, ao.orcid,
			a.author_order, a.affiliation FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id
			WHERE a.work_revision_id=?)`
		orderID, args = "relation_id", []any{revisionID}
	case "references":
		fromWhere = `FROM (SELECT rm.id, rm.work_revision_id, rm.resolved_work_id, rm.mention_order, rm.doi, rm.title, rm.author, rm.year, rm.source, rm.created_at,
			target.id AS resolved_revision_id, target.title AS resolved_title
			FROM reference_mentions rm JOIN work_revisions source ON source.id=rm.work_revision_id
			LEFT JOIN work_revisions target ON target.id=(SELECT candidate.id FROM work_revisions candidate
				WHERE candidate.work_id=rm.resolved_work_id AND candidate.pipeline_run_id=source.pipeline_run_id
				AND ` + currentNormalizedRevisionPredicate("candidate") + ` LIMIT 1)
			WHERE rm.work_revision_id=?)`
		orderID, args = "id", []any{revisionID}
	case "stages":
		fromWhere = `FROM (SELECT id, stage_name, outcome, reason, created_at, updated_at FROM run_work_stages
			WHERE pipeline_run_id=? AND work_id=?)`
		orderID, args = "id", []any{runID, workID}
	case "audit":
		condition, conditionArgs := articleAuditCondition(workID, runID)
		fromWhere = `FROM (SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action,
			before_json, after_json, metadata_json, correlation_id FROM audit_events WHERE ` + condition + `)`
		orderID, args, descending = "id", conditionArgs, true
	default:
		return nil, notFound("article detail collection not found")
	}
	result, err := s.detailCollectionEnvelope(ctx, cursorKind, fromWhere, orderID, args, cursorID, descending, limit)
	if err == nil && collection == "audit" {
		boundAuditEventPayloads(result["items"].([]map[string]any), auditListPayloadBytes)
	}
	return result, err
}

// authorDetailCollectionData defines the fixed projections for author detail subresources.
func (s *Server) authorDetailCollectionData(ctx context.Context, authorID, runID int64, collection, cursorKind string, cursorID int64, limit int) (map[string]any, error) {
	var fromWhere, orderID string
	var args []any
	descending := false
	switch collection {
	case "articles":
		fromWhere = `FROM (SELECT a.id AS relation_id, a.author_order, a.affiliation, wr.id AS work_revision_id, wr.work_id, wr.title, wr.year,
			wr.pipeline_run_id, w.doi FROM authorships a JOIN work_revisions wr ON wr.id=a.work_revision_id
			JOIN works w ON w.id=wr.work_id WHERE a.author_occurrence_id=? AND wr.pipeline_run_id=?)`
		orderID, args = "relation_id", []any{authorID, runID}
	case "audit":
		fromWhere = `FROM (SELECT id, occurred_at, actor, pipeline_run_id, entity_type, entity_id, action,
			before_json, after_json, metadata_json, correlation_id FROM audit_events
			WHERE entity_type='author_occurrence' AND entity_id=? AND pipeline_run_id=?)`
		orderID, args, descending = "id", []any{stringID(authorID), runID}, true
	case "identity":
		if !s.tableHasColumns("author_identity_resolutions", "pipeline_run_id", "author_occurrence_id", "status") {
			return map[string]any{"items": []map[string]any{}, "total": int64(0), "limit": limit, "has_more": false, "next_cursor": nil}, nil
		}
		fromWhere = `FROM (SELECT r.id AS resolution_id, r.id, r.pipeline_run_id, r.status, r.provider, r.queried_citation_name,
			r.error_message, r.resolved_at, COUNT(c.id) AS candidate_count
			FROM author_identity_resolutions r LEFT JOIN author_identity_candidates c ON c.identity_resolution_id=r.id
			WHERE r.author_occurrence_id=? AND r.pipeline_run_id=? GROUP BY r.id)`
		orderID, args, descending = "id", []any{authorID, runID}, true
	default:
		return nil, notFound("author detail collection not found")
	}
	result, err := s.detailCollectionEnvelope(ctx, cursorKind, fromWhere, orderID, args, cursorID, descending, limit)
	if err != nil {
		return nil, err
	}
	if collection == "audit" {
		boundAuditEventPayloads(result["items"].([]map[string]any), auditListPayloadBytes)
	}
	if collection == "identity" {
		if err := s.attachIdentityCandidatePreviews(ctx, result["items"].([]map[string]any)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// articleAuditCondition returns the run-scoped, privacy-safe logical-work event predicate and arguments.
func articleAuditCondition(workID, runID int64) (string, []any) {
	condition := `(entity_type='work_revision' AND entity_id IN (` + articleWorkRunRevisionIDsSQL + `))
		OR (entity_type='work' AND entity_id=? AND (pipeline_run_id=? OR (pipeline_run_id IS NULL AND action LIKE 'pdf_%')))
		OR (entity_type='work_review_version' AND pipeline_run_id=? AND entity_id IN (` + articleWorkRunReviewVersionIDsSQL + `))
		OR (entity_type='review_note_version' AND pipeline_run_id=? AND entity_id IN (` + articleWorkRunNoteVersionIDsSQL + `))
		OR (entity_type='review_anchor_version' AND pipeline_run_id=? AND entity_id IN (` + articleWorkRunAnchorVersionIDsSQL + `))
		OR (entity_type='review_context' AND pipeline_run_id=?)`
	args := []any{
		workID, runID, stringID(workID), runID,
		runID, workID, runID,
		runID, workID, runID,
		runID, workID, runID,
		runID,
	}
	return condition, args
}

// rows executes a read-only query and converts every result row to a field map.
func (s *Server) rows(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsAsMaps(rows)
}

// oneRow returns the first mapped query row, or nil when the query returns no rows.
func (s *Server) oneRow(ctx context.Context, query string, args ...any) (map[string]any, error) {
	rows, err := s.rows(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// stringID formats a numeric database identifier in base 10.
func stringID(id int64) string { return strconv.FormatInt(id, 10) }
