// details.go provides the article detail endpoint that returns the
// full metadata, authors, and references for a single immutable
// work revision identified by its numeric ID.
package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"analysis/database/audit"
	"analysis/database/author"
	"analysis/database/work"
)

const detailCollectionPreviewLimit = 25

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
	revision, err := s.workStore.GetArticleRevision(ctx, id, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if revision == nil {
		s.respond(w, r, nil, notFound("article revision not found"))
		return
	}
	workID := revision.WorkID
	article := articleRevisionRow(revision)
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
	if revision.ProducerStage == "normalize" {
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
	s.respond(w, r, map[string]any{
		"article":                    article,
		"authors":                    authors,
		"references":                 references,
		"stage_outcomes":             stageOutcomes,
		"audit_events":               audit,
		"enrichment_summary":         enrichmentSummary,
		"pdf_status":                 pdfStatus,
		"review_context":             reviewContext,
		"review_context_initialized": reviewContext != nil,
		"term_matches":               termMatches,
	}, nil)
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
	author, err := s.authorStore.GetOccurrenceForRun(ctx, id, runID)
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
	s.respond(w, r, map[string]any{"author": authorOccurrenceRow(author), "articles": articles, "audit_events": audit, "identity_evidence": identityEvidence}, nil)
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
	detail, err := s.workStore.GetReferenceDetail(ctx, id, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if detail == nil {
		s.respond(w, r, nil, notFound("reference mention not found"))
		return
	}
	s.respond(w, r, map[string]any{"reference": referenceDetailRow(detail)}, nil)
}

// articleEnrichmentSummary returns a bounded set of provider and field labels without transferring event payloads.
func (s *Server) articleEnrichmentSummary(ctx context.Context, workID, runID int64) (map[string]any, error) {
	summary, err := s.auditStore.EnrichmentSummary(ctx, workID, runID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"providers":  summary.Providers,
		"fields":     summary.Fields,
		"truncated":  summary.Truncated,
		"pair_limit": 100,
	}, nil
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
	author, err := s.authorStore.GetOccurrenceForRun(ctx, authorID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if author == nil {
		s.respond(w, r, nil, notFound("author occurrence not found"))
		return
	}
	result, err := s.authorDetailCollectionData(ctx, authorID, runID, kind, cursorKind, cursorID, limit)
	s.respond(w, r, result, err)
}

// articleDetailWorkID validates one visible article revision and returns its owning work.
func (s *Server) articleDetailWorkID(ctx context.Context, revisionID, runID int64) (int64, error) {
	workID, err := s.workStore.GetArticleDetailWorkID(ctx, revisionID, runID)
	if errors.Is(err, work.ErrArticleRevisionNotFound) {
		return 0, notFound("article revision not found")
	}
	return workID, err
}

// articleDetailCollectionData defines the fixed projections for article detail subresources.
func (s *Server) articleDetailCollectionData(ctx context.Context, revisionID, workID, runID int64, collection, cursorKind string, cursorID int64, limit int) (map[string]any, error) {
	switch collection {
	case "authors":
		page, err := s.workStore.ListArticleAuthors(ctx, work.ArticleAuthorPageInput{RevisionID: revisionID, CursorID: cursorID, Limit: limit})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, articleAuthorRow(item))
		}
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	case "references":
		page, err := s.workStore.ListArticleReferences(ctx, work.ArticleReferencePageInput{RevisionID: revisionID, CursorID: cursorID, Limit: limit})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, articleReferenceRow(item))
		}
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	case "stages":
		page, err := s.workStore.ListArticleStages(ctx, work.ArticleStagePageInput{RunID: runID, WorkID: workID, CursorID: cursorID, Limit: limit})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, articleStageRow(item))
		}
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	case "audit":
		page, err := s.auditStore.ListArticleDetailEvents(ctx, audit.ArticleDetailEventFilter{
			WorkID: workID, RunID: runID, CursorID: cursorID, Limit: limit,
		})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, event := range page.Items {
			items = append(items, auditEventMap(event))
		}
		boundAuditEventPayloads(items, auditListPayloadBytes)
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	default:
		return nil, notFound("article detail collection not found")
	}
}

// detailCollectionPage builds one bounded detail collection envelope from a family page.
func detailCollectionPage(items []map[string]any, total int64, limit int, hasMore bool, nextCursorID int64, cursorKind string) map[string]any {
	var nextCursor any
	if hasMore {
		nextCursor = encodeCursor(reviewCursor{Kind: cursorKind, ID: nextCursorID})
	}
	return map[string]any{"items": items, "total": total, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}
}

// authorDetailCollectionData defines the fixed projections for author detail subresources.
func (s *Server) authorDetailCollectionData(ctx context.Context, authorID, runID int64, collection, cursorKind string, cursorID int64, limit int) (map[string]any, error) {
	switch collection {
	case "articles":
		page, err := s.workStore.ListAuthorArticles(ctx, work.AuthorArticleFilter{
			AuthorOccurrenceID: authorID, RunID: runID, CursorID: cursorID, Limit: limit,
		})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, authorArticleRow(item))
		}
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	case "audit":
		page, err := s.auditStore.ListAuthorDetailEvents(ctx, audit.AuthorDetailEventFilter{
			AuthorOccurrenceID: authorID, RunID: runID, CursorID: cursorID, Limit: limit,
		})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, event := range page.Items {
			items = append(items, auditEventMap(event))
		}
		boundAuditEventPayloads(items, auditListPayloadBytes)
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	case "identity":
		if !s.tableHasColumns("author_identity_resolutions", "pipeline_run_id", "author_occurrence_id", "status") {
			return map[string]any{"items": []map[string]any{}, "total": int64(0), "limit": limit, "has_more": false, "next_cursor": nil}, nil
		}
		page, err := s.authorStore.ListAuthorIdentityEvidence(ctx, author.AuthorIdentityEvidenceFilter{
			RunID: runID, AuthorOccurrenceID: authorID, CursorID: cursorID, Limit: limit,
		})
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, authorIdentityEvidenceRow(item))
		}
		if err := s.attachIdentityCandidatePreviews(ctx, items); err != nil {
			return nil, err
		}
		return detailCollectionPage(items, page.Total, limit, page.HasMore, page.NextCursorID, cursorKind), nil
	default:
		return nil, notFound("author detail collection not found")
	}
}

// authorArticleRow renders one author article projection as the viewer's JSON row shape.
func authorArticleRow(item *work.AuthorArticle) map[string]any {
	return map[string]any{
		"author_order":     item.AuthorOrder,
		"affiliation":      nullableText(item.Affiliation),
		"work_revision_id": item.WorkRevisionID,
		"work_id":          item.WorkID,
		"title":            nullableText(item.Title),
		"year":             nullableIDPointer(item.Year),
		"pipeline_run_id":  item.PipelineRunID,
		"doi":              nullableText(item.DOI),
	}
}

// authorIdentityEvidenceRow renders one author identity evidence projection as the viewer's JSON row shape.
func authorIdentityEvidenceRow(item *author.AuthorIdentityEvidenceRow) map[string]any {
	return map[string]any{
		"resolution_id":         item.ResolutionID,
		"id":                    item.ID,
		"pipeline_run_id":       item.PipelineRunID,
		"status":                item.Status,
		"provider":              item.Provider,
		"queried_citation_name": item.QueriedCitationName,
		"error_message":         nullableText(item.ErrorMessage),
		"resolved_at":           item.ResolvedAt,
		"candidate_count":       item.CandidateCount,
	}
}

// stringID formats a numeric database identifier in base 10.
func stringID(id int64) string { return strconv.FormatInt(id, 10) }
