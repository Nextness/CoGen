package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"analysis/database"
)

const reviewMutationBodyLimit int64 = 524288

// runReviewContext returns the initialized context or the deterministic proposed parent.
func (s *Server) runReviewContext(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	if _, err := s.requireReviewableRun(ctx, runID); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	contextRecord, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	response := map[string]any{"run_id": runID, "context_initialized": contextRecord != nil, "context": contextRecord}
	if contextRecord == nil {
		response["proposed_parent"], err = s.writeDB.Reviews.ProposeParent(ctx, runID)
	}
	s.respond(w, r, response, err)
}

// reviewContextCandidates returns bounded eligible parent contexts.
func (s *Server) reviewContextCandidates(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "scope", "cursor", "limit", "q"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "same_search"
	}
	cursor, limit, err := reviewPage(r, false)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	items, err := s.writeDB.Reviews.ListParentCandidates(ctx, runID, scope, cursor, limit, r.URL.Query().Get("q"))
	s.respond(w, r, map[string]any{"rows": items, "limit": limit}, mapReviewError(err))
}

// createReviewContext explicitly initializes a completed run's review context.
func (s *Server) createReviewContext(w http.ResponseWriter, r *http.Request) {
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		ParentContextID *int64 `json:"parent_context_id"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if request.ParentContextID != nil && *request.ParentContextID < 1 {
		s.respond(w, r, nil, badRequest("parent_context_id must be positive or null"))
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	created, wasCreated, err := s.writeDB.Reviews.CreateContext(ctx, runID, request.ParentContextID)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	status := http.StatusOK
	if wasCreated {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"context_initialized": true, "context": created})
}

// articleReview returns current review state or an uninitialized default without manufacturing a head.
func (s *Server) articleReview(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	run, err := s.requireReviewableRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	workID, pdfStatus, err := s.reviewArticlePDF(ctx, runID, workRevisionID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	contextRecord, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	base := map[string]any{
		"run_id": runID, "work_id": workID, "work_revision_id": workRevisionID,
		"context_initialized": contextRecord != nil, "editable": false, "pdf_status": pdfStatus,
		"state": map[string]any{"status": "not_evaluated", "sub_statuses": []string{}, "reason": nil, "version": nil},
	}
	if contextRecord == nil {
		s.respond(w, r, base, nil)
		return
	}
	state, err := s.writeDB.Reviews.GetWorkReview(ctx, contextRecord.ID, workRevisionID)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	base["context"] = contextRecord
	base["review"] = state
	base["editable"] = run.Status == "completed" && run.Visibility != "trashed" && pdfStatus["status"] == "available"
	counts, err := s.reviewSummaryCounts(ctx, contextRecord.ID, workID)
	if err == nil {
		base["summary_counts"] = counts
	}
	s.respond(w, r, base, err)
}

// updateArticleReview appends one complete immutable article-review state.
func (s *Server) updateArticleReview(w http.ResponseWriter, r *http.Request) {
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		ExpectedVersionID *int64   `json:"expected_version_id"`
		Status            string   `json:"status"`
		Substatuses       []string `json:"sub_statuses"`
		Reason            *string  `json:"reason"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if _, _, err := s.requireAvailableArticlePDF(ctx, runID, workRevisionID); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	state, changed, err := s.writeDB.Reviews.AppendWorkReview(ctx, contextRecord.ID, workRevisionID,
		request.ExpectedVersionID, request.Status, request.Substatuses, request.Reason)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": state, "changed": changed})
}

// articleReviewVersions returns bounded immutable ancestors from the selected context head.
func (s *Server) articleReviewVersions(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewPage(r, false)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versions, err := s.writeDB.Reviews.ListWorkReviewVersions(ctx, contextRecord.ID, workRevisionID, cursor, limit)
	s.respond(w, r, map[string]any{"versions": versions, "limit": limit}, mapReviewError(err))
}

// articleNotes returns bounded current active note heads.
func (s *Server) articleNotes(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewPage(r, false)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	notes, err := s.writeDB.Reviews.ListNotes(ctx, contextRecord.ID, workRevisionID, cursor, limit, false)
	s.respond(w, r, map[string]any{"notes": notes, "limit": limit}, mapReviewError(err))
}

// createArticleNote creates a logical note and first immutable version.
func (s *Server) createArticleNote(w http.ResponseWriter, r *http.Request) {
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		Body string `json:"body"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if _, _, err := s.requireAvailableArticlePDF(ctx, runID, workRevisionID); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	note, err := s.writeDB.Reviews.CreateNote(ctx, contextRecord.ID, workRevisionID, request.Body)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"note": note})
}

// note returns an explicitly addressed current head, including a tombstone.
func (s *Server) note(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	runID, noteID, contextRecord, ctx, cancel, err := s.reviewNoteRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	note, err := s.writeDB.Reviews.GetNote(ctx, contextRecord.ID, noteID)
	if err == nil && note == nil {
		err = notFound("review note not found in selected context")
	}
	s.respond(w, r, map[string]any{"run_id": runID, "note": note}, mapReviewError(err))
}

// noteVersions returns bounded immutable note ancestors.
func (s *Server) noteVersions(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, noteID, contextRecord, ctx, cancel, err := s.reviewNoteRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewPage(r, false)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versions, err := s.writeDB.Reviews.ListNoteVersions(ctx, contextRecord.ID, noteID, cursor, limit)
	s.respond(w, r, map[string]any{"run_id": runID, "versions": versions, "limit": limit}, mapReviewError(err))
}

// createNoteVersion creates an active edit or deletion tombstone.
func (s *Server) createNoteVersion(w http.ResponseWriter, r *http.Request) {
	runID, noteID, contextRecord, ctx, cancel, err := s.reviewNoteRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		ExpectedVersionID int64  `json:"expected_version_id"`
		State             string `json:"state"`
		Body              string `json:"body"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if request.ExpectedVersionID < 1 {
		s.respond(w, r, nil, badRequest("expected_version_id must be positive"))
		return
	}
	current, err := s.writeDB.Reviews.GetNote(ctx, contextRecord.ID, noteID)
	if err != nil || current == nil {
		s.respond(w, r, nil, notFound("review note not found in selected context"))
		return
	}
	if _, _, err := s.requireAvailableWorkPDF(ctx, runID, current.WorkID); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	note, changed, err := s.writeDB.Reviews.AppendNoteVersion(ctx, contextRecord.ID, noteID,
		request.ExpectedVersionID, request.State, request.Body)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	status := http.StatusCreated
	if !changed {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"note": note, "changed": changed})
}

// articleAnchors returns bounded current active PDF anchors.
func (s *Server) articleAnchors(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	_, limit, err := reviewPage(r, true)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	anchors, err := s.writeDB.Reviews.ListAnchors(ctx, contextRecord.ID, workRevisionID, r.URL.Query().Get("cursor"), limit)
	s.respond(w, r, map[string]any{"anchors": anchors, "limit": limit}, mapReviewError(err))
}

// createArticleAnchor creates a logical anchor and its first immutable geometry version.
func (s *Server) createArticleAnchor(w http.ResponseWriter, r *http.Request) {
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		AnchorID     string                     `json:"anchor_id"`
		Page         int                        `json:"page"`
		SelectedText string                     `json:"selected_text"`
		Rectangles   []database.AnchorRectangle `json:"rectangles"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	_, contentHash, err := s.requireAvailableArticlePDF(ctx, runID, workRevisionID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	anchor, err := s.writeDB.Reviews.CreateAnchor(ctx, contextRecord.ID, workRevisionID, request.AnchorID,
		contentHash, request.Page, request.SelectedText, request.Rectangles)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"anchor": anchor})
}

// anchorVersions returns bounded immutable anchor ancestors.
func (s *Server) anchorVersions(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, anchorID, contextRecord, ctx, cancel, err := s.reviewAnchorRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewPage(r, false)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versions, err := s.writeDB.Reviews.ListAnchorVersions(ctx, contextRecord.ID, anchorID, cursor, limit)
	s.respond(w, r, map[string]any{"run_id": runID, "versions": versions, "limit": limit}, mapReviewError(err))
}

// createAnchorVersion creates a replacement anchor version or tombstone using the currently selected PDF hash.
func (s *Server) createAnchorVersion(w http.ResponseWriter, r *http.Request) {
	runID, anchorID, contextRecord, ctx, cancel, err := s.reviewAnchorRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		ExpectedVersionID int64                      `json:"expected_version_id"`
		State             string                     `json:"state"`
		Page              int                        `json:"page"`
		SelectedText      string                     `json:"selected_text"`
		Rectangles        []database.AnchorRectangle `json:"rectangles"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if request.ExpectedVersionID < 1 {
		s.respond(w, r, nil, badRequest("expected_version_id must be positive"))
		return
	}
	var workID int64
	if err := s.db.QueryRowContext(ctx, `SELECT logical.work_id FROM review_context_anchor_heads head
		JOIN review_anchors logical ON logical.id=head.anchor_id WHERE head.review_context_id=? AND head.anchor_id=?`,
		contextRecord.ID, anchorID).Scan(&workID); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("review anchor not found in selected context"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	_, contentHash, err := s.requireAvailableWorkPDF(ctx, runID, workID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	anchor, changed, err := s.writeDB.Reviews.AppendAnchorVersion(ctx, contextRecord.ID, anchorID,
		request.ExpectedVersionID, request.State, contentHash, request.Page, request.SelectedText, request.Rectangles)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	status := http.StatusCreated
	if !changed {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"anchor": anchor, "changed": changed})
}

// reviewBacklinks returns bounded current-version backlinks.
func (s *Server) reviewBacklinks(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "target_type", "target_id", "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	targetType, targetID := r.URL.Query().Get("target_type"), r.URL.Query().Get("target_id")
	if !map[string]bool{"note": true, "article": true, "pdf_page": true, "anchor": true, "ext": true}[targetType] || targetID == "" {
		s.respond(w, r, nil, badRequest("target_type and target_id are required"))
		return
	}
	cursor, limit, err := reviewPage(r, false)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items, err := s.writeDB.Reviews.ListBacklinks(ctx, contextRecord.ID, targetType, targetID, cursor, limit)
	s.respond(w, r, map[string]any{"backlinks": items, "limit": limit}, mapReviewError(err))
}

// reviewRunRecord contains the lifecycle fields that gate local review.
type reviewRunRecord struct{ Status, Visibility string }

// requireReviewableRun rejects missing, failed, running, or trashed run contexts.
func (s *Server) requireReviewableRun(ctx context.Context, runID int64) (reviewRunRecord, error) {
	var run reviewRunRecord
	err := s.db.QueryRowContext(ctx, "SELECT status, visibility_state FROM pipeline_runs WHERE id=?", runID).Scan(&run.Status, &run.Visibility)
	if err == sql.ErrNoRows {
		return run, notFound("pipeline run not found")
	}
	if err != nil {
		return run, err
	}
	if run.Status != "completed" || run.Visibility == "trashed" {
		return run, &apiProblem{Status: http.StatusConflict, Code: "run_not_reviewable", Message: "only completed non-trashed runs can be reviewed"}
	}
	return run, nil
}

// requireInitializedContext returns the explicitly created context for an eligible run.
func (s *Server) requireInitializedContext(ctx context.Context, runID int64) (*database.ReviewContext, error) {
	if _, err := s.requireReviewableRun(ctx, runID); err != nil {
		return nil, err
	}
	contextRecord, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if contextRecord == nil {
		return nil, &apiProblem{Status: http.StatusConflict, Code: "review_context_required", Message: "start review for this run before saving"}
	}
	return contextRecord, nil
}

// reviewArticlePDF validates exact run ownership and returns stable work plus inventory state.
func (s *Server) reviewArticlePDF(ctx context.Context, runID, workRevisionID int64) (int64, map[string]any, error) {
	var workID int64
	err := s.db.QueryRowContext(ctx, `SELECT work_id FROM work_revisions
		WHERE id=? AND pipeline_run_id=? AND producer_stage='normalize'`, workRevisionID, runID).Scan(&workID)
	if err == sql.ErrNoRows {
		return 0, nil, notFound("article revision does not belong to selected run")
	}
	if err != nil {
		return 0, nil, err
	}
	status, err := s.pdfStatusForWork(ctx, workID)
	return workID, status, err
}

// requireAvailableArticlePDF gates an exact revision mutation on selected PDF bytes.
func (s *Server) requireAvailableArticlePDF(ctx context.Context, runID, workRevisionID int64) (int64, string, error) {
	workID, status, err := s.reviewArticlePDF(ctx, runID, workRevisionID)
	if err != nil {
		return 0, "", err
	}
	if status["status"] != "available" {
		return 0, "", &apiProblem{Status: http.StatusConflict, Code: "pdf_unavailable", Message: "an available PDF is required for review changes"}
	}
	hash, _ := status["content_hash"].(string)
	return workID, hash, nil
}

// requireAvailableWorkPDF gates logical note or anchor mutation on run membership and PDF bytes.
func (s *Server) requireAvailableWorkPDF(ctx context.Context, runID, workID int64) (int64, string, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_revisions
		WHERE pipeline_run_id=? AND work_id=? AND producer_stage='normalize'`, runID, workID).Scan(&count); err != nil {
		return 0, "", err
	}
	if count == 0 {
		return 0, "", notFound("work does not belong to selected run")
	}
	status, err := s.pdfStatusForWork(ctx, workID)
	if err != nil {
		return 0, "", err
	}
	if status["status"] != "available" {
		return 0, "", &apiProblem{Status: http.StatusConflict, Code: "pdf_unavailable", Message: "an available PDF is required for review changes"}
	}
	hash, _ := status["content_hash"].(string)
	return workID, hash, nil
}

// reviewSummaryCounts returns bounded current note, anchor, and backlink summary counts.
func (s *Server) reviewSummaryCounts(ctx context.Context, contextID, workID int64) (map[string]int, error) {
	counts := map[string]int{}
	queries := map[string]string{
		"note_count": `SELECT COUNT(*) FROM review_context_note_heads head JOIN review_notes logical ON logical.id=head.note_id
			JOIN review_note_versions version ON version.id=head.note_version_id WHERE head.review_context_id=? AND logical.work_id=? AND version.state='active'`,
		"anchor_count": `SELECT COUNT(*) FROM review_context_anchor_heads head JOIN review_anchors logical ON logical.id=head.anchor_id
			JOIN review_anchor_versions version ON version.id=head.anchor_version_id WHERE head.review_context_id=? AND logical.work_id=? AND version.state='active'`,
		"review_version_count": `WITH RECURSIVE ancestry(id) AS (SELECT review_version_id FROM review_context_work_heads
			WHERE review_context_id=? AND work_id=? UNION ALL SELECT version.parent_version_id FROM work_review_versions version
			JOIN ancestry ON ancestry.id=version.id WHERE version.parent_version_id IS NOT NULL) SELECT COUNT(*) FROM ancestry WHERE id IS NOT NULL`,
	}
	for name, query := range queries {
		var count int
		if err := s.db.QueryRowContext(ctx, query, contextID, workID).Scan(&count); err != nil {
			return nil, err
		}
		counts[name] = count
	}
	return counts, nil
}

// reviewArticleIDs parses the positive run and revision identifiers from one review route.
func reviewArticleIDs(r *http.Request) (int64, int64, error) {
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		return 0, 0, err
	}
	revisionID, err := positiveID(r.PathValue("work_revision_id"))
	return runID, revisionID, err
}

// reviewNoteRequest prepares a bounded request context and resolves one logical note route.
func (s *Server) reviewNoteRequest(r *http.Request) (int64, int64, *database.ReviewContext, context.Context, context.CancelFunc, error) {
	ctx, cancel := queryContext(r)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		return 0, 0, nil, ctx, cancel, err
	}
	noteID, err := positiveID(r.PathValue("note_id"))
	if err != nil {
		return 0, 0, nil, ctx, cancel, err
	}
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	return runID, noteID, contextRecord, ctx, cancel, err
}

// reviewAnchorRequest prepares a bounded request context and resolves one safe anchor route.
func (s *Server) reviewAnchorRequest(r *http.Request) (int64, string, *database.ReviewContext, context.Context, context.CancelFunc, error) {
	ctx, cancel := queryContext(r)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		return 0, "", nil, ctx, cancel, err
	}
	anchorID := r.PathValue("anchor_id")
	if anchorID == "" || len(anchorID) > 64 {
		return 0, "", nil, ctx, cancel, badRequest("anchor_id has an invalid format")
	}
	contextRecord, err := s.requireInitializedContext(ctx, runID)
	return runID, anchorID, contextRecord, ctx, cancel, err
}

// reviewPage validates bounded review pagination and optional numeric cursors.
func reviewPage(r *http.Request, stringCursor bool) (int64, int, error) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			return 0, 0, badRequest("limit must be between 1 and 100")
		}
		limit = value
	}
	if stringCursor {
		return 0, limit, nil
	}
	var cursor int64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1 {
			return 0, 0, badRequest("cursor must be a positive integer")
		}
		cursor = value
	}
	return cursor, limit, nil
}

// decodeMutationJSON enforces media type, body bound, single value, and known JSON fields.
func decodeMutationJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if err := validateKnownQuery(r); err != nil {
		return err
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &apiProblem{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type must be application/json"}
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		return &apiProblem{Status: http.StatusForbidden, Code: "origin_rejected", Message: "request Origin must match the local viewer origin"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, reviewMutationBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &apiProblem{Status: http.StatusRequestEntityTooLarge, Code: "request_too_large", Message: "request body exceeds 524288 bytes"}
		}
		return badRequest("request body must be one valid JSON object with known fields")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return badRequest("request body must contain exactly one JSON value")
	}
	return nil
}

// mapReviewError converts repository conflicts and validation failures into stable API problems.
func mapReviewError(err error) error {
	if err == nil {
		return nil
	}
	var conflict *database.ReviewConflictError
	if errors.As(err, &conflict) {
		return &apiProblem{Status: http.StatusConflict, Code: "version_conflict", Message: conflict.Error(), Details: map[string]any{"expected_version_id": conflict.Expected, "current_version_id": conflict.Current}}
	}
	var syntax *database.NoteSyntaxError
	if errors.As(err, &syntax) {
		return &apiProblem{Status: http.StatusBadRequest, Code: "note_syntax_error", Message: syntax.Error(), Details: map[string]any{"syntax_errors": syntax.Errors}}
	}
	if strings.Contains(err.Error(), "does not belong") || strings.Contains(err.Error(), "not found") {
		return notFound(err.Error())
	}
	return badRequest(err.Error())
}

// setMutableResponseHeaders prevents caching context-sensitive review responses.
func setMutableResponseHeaders(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }
