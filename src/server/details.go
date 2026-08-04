// details.go provides the article detail endpoint that returns the
// full metadata, authors, and references for a single immutable
// work revision identified by its numeric ID.
package server

import (
	"context"
	"net/http"
	"strconv"
)

const articleWorkRunRevisionIDsSQL = `SELECT CAST(id AS TEXT)
	FROM work_revisions
	WHERE work_id=? AND pipeline_run_id=?`

// articleDetail treats the numeric route identifier as an immutable work
// revision ID. It intentionally does not expose the retired mutable articles
// projection.
func (s *Server) articleDetail(w http.ResponseWriter, r *http.Request) {
	id, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	revision, err := s.oneRow(ctx, `SELECT wr.*, w.doi FROM work_revisions wr JOIN works w ON w.id=wr.work_id WHERE wr.id=?`, id)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if revision == nil {
		s.respond(w, r, nil, notFound("article revision not found"))
		return
	}
	workID := revision["work_id"].(int64)
	runID := revision["pipeline_run_id"].(int64)
	authors, err := s.rows(ctx, `SELECT ao.id, ao.person_id, ao.citation_name, ao.first_name, ao.last_name, ao.orcid,
        a.author_order, a.affiliation FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id
        WHERE a.work_revision_id=? ORDER BY a.author_order, a.id`, id)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	references, err := s.rows(ctx, `SELECT rm.*, target.id AS resolved_revision_id, target.title AS resolved_title
		FROM reference_mentions rm JOIN work_revisions source ON source.id=rm.work_revision_id
		LEFT JOIN work_revisions target ON target.id=(
			SELECT candidate.id FROM work_revisions candidate
			WHERE candidate.work_id=rm.resolved_work_id AND candidate.pipeline_run_id=source.pipeline_run_id
			ORDER BY CASE candidate.producer_stage
				WHEN 'normalize' THEN 7
				WHEN 'validate' THEN 6
				WHEN 'enrich_identity' THEN 5
				WHEN 'enrich_metadata' THEN 4
				WHEN 'enrich' THEN 3
				WHEN 'deduplicate' THEN 2
				WHEN 'parse' THEN 1
				ELSE 0
			END DESC, candidate.id DESC
			LIMIT 1
		)
		WHERE rm.work_revision_id=? ORDER BY rm.mention_order, rm.id`, id)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	stageOutcomes, err := s.rows(ctx, `SELECT id, stage_name, outcome, reason, created_at, updated_at
		FROM run_work_stages
		WHERE pipeline_run_id=? AND work_id=?
		ORDER BY created_at ASC, id ASC`, runID, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	audit, err := s.auditRows(ctx, `(entity_type='work_revision' AND entity_id IN (`+articleWorkRunRevisionIDsSQL+`))
		OR (entity_type='work' AND entity_id=? AND (
			pipeline_run_id=? OR (pipeline_run_id IS NULL AND action LIKE 'pdf_%')
		))`, workID, runID, stringID(workID), runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	enrichedFields, err := s.rows(ctx,
		`SELECT metadata_json FROM audit_events
		 WHERE entity_type='work_revision' AND action='field_enriched'
		   AND entity_id IN (`+articleWorkRunRevisionIDsSQL+`)
		 ORDER BY id`, workID, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	pdfStatus, err := s.pdfStatusForWork(ctx, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]any{"article": revision, "authors": authors, "references": references, "stage_outcomes": stageOutcomes, "audit_events": audit, "enriched_fields": enrichedFields, "pdf_status": pdfStatus}, nil)
}

// authorDetail returns one author occurrence with its articles and audit evidence.
func (s *Server) authorDetail(w http.ResponseWriter, r *http.Request) {
	id, err := positiveID(r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	author, err := s.oneRow(ctx, `SELECT ao.*, p.orcid AS person_orcid FROM author_occurrences ao LEFT JOIN people p ON p.id=ao.person_id WHERE ao.id=?`, id)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if author == nil {
		s.respond(w, r, nil, notFound("author occurrence not found"))
		return
	}
	articles, err := s.rows(ctx, `SELECT a.author_order, a.affiliation, wr.id AS work_revision_id, wr.work_id, wr.title, wr.year,
        wr.pipeline_run_id, w.doi FROM authorships a JOIN work_revisions wr ON wr.id=a.work_revision_id
        JOIN works w ON w.id=wr.work_id WHERE a.author_occurrence_id=? ORDER BY wr.id, a.author_order`, id)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	audit, err := s.auditRows(ctx, "entity_type='author_occurrence' AND entity_id=?", stringID(id))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]any{"author": author, "articles": articles, "audit_events": audit}, nil)
}

// referenceDetail returns one reference mention with its citing and resolved-work context.
func (s *Server) referenceDetail(w http.ResponseWriter, r *http.Request) {
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
        LEFT JOIN work_revisions target ON target.id=(
            SELECT candidate.id FROM work_revisions candidate
            WHERE candidate.work_id=rm.resolved_work_id AND candidate.pipeline_run_id=wr.pipeline_run_id
            ORDER BY CASE candidate.producer_stage
                WHEN 'normalize' THEN 7
                WHEN 'validate' THEN 6
                WHEN 'enrich_identity' THEN 5
                WHEN 'enrich_metadata' THEN 4
                WHEN 'enrich' THEN 3
                WHEN 'deduplicate' THEN 2
                WHEN 'parse' THEN 1
                ELSE 0
            END DESC, candidate.id DESC
            LIMIT 1
        )
        WHERE rm.id=?`, id)
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
