package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

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
	run, err := s.loadReviewRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	contextRecord, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	writable := run.Status == "completed" && run.Visibility != "trashed"
	response := map[string]any{"run_id": runID, "context_initialized": contextRecord != nil, "context": contextRecord, "run_writable": writable}
	if contextRecord == nil && writable {
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
	limit, err := reviewLimit(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	cursor, err := decodeReviewCursor(r.URL.Query().Get("cursor"), "parent_candidates")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if r.URL.Query().Get("cursor") != "" && (cursor.StartedAt == "" || cursor.ID < 1) {
		s.respond(w, r, nil, badRequest("cursor is invalid for this collection"))
		return
	}
	items, err := s.writeDB.Reviews.ListParentCandidates(ctx, runID, scope, cursor.StartedAt, cursor.ID, limit+1, r.URL.Query().Get("q"))
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	var nextCursor *string
	if hasMore {
		value := encodeReviewCursor(reviewCursor{Kind: "parent_candidates", StartedAt: items[len(items)-1].StartedAt, ID: items[len(items)-1].PipelineRunID})
		nextCursor = &value
	}
	s.respond(w, r, map[string]any{"items": items, "rows": items, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
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
	run, err := s.loadReviewRun(ctx, runID)
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
		"editability": map[string]any{"decision": false, "notes": false, "anchors": false},
		"state":       map[string]any{"status": "not_evaluated", "sub_statuses": []string{}, "reason": nil, "version": nil},
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
	writable := run.Status == "completed" && run.Visibility != "trashed"
	base["editable"] = writable
	base["editability"] = map[string]any{"decision": writable, "notes": writable, "anchors": writable && pdfStatus["status"] == "available"}
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
	cursor, limit, err := reviewIDPage(r, "review_versions")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireContextForRead(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versions, err := s.writeDB.Reviews.ListWorkReviewVersions(ctx, contextRecord.ID, workRevisionID, cursor, limit+1)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	items, hasMore, nextCursor := reviewIDItems(versions, limit, "review_versions", func(item database.WorkReviewVersion) int64 { return item.ID })
	s.respond(w, r, map[string]any{"items": items, "versions": items, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
}

// articleReviewVersion returns one full immutable decision version from the selected ancestry.
func (s *Server) articleReviewVersion(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versionID, err := positiveID(r.PathValue("version_id"))
	if err != nil || validateKnownQuery(r) != nil {
		s.respond(w, r, nil, badRequest("version_id must be positive and query parameters are not supported"))
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireContextForRead(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	version, err := s.writeDB.Reviews.GetWorkReviewVersion(ctx, contextRecord.ID, workRevisionID, versionID)
	if err == nil && version == nil {
		err = notFound("review version not found in selected context")
	}
	s.respond(w, r, map[string]any{"version": version}, mapReviewError(err))
}

// articleNotes returns bounded current active note heads.
func (s *Server) articleNotes(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit", "state", "q"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewIDPage(r, "notes")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireContextForRead(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "active"
	}
	notes, err := s.writeDB.Reviews.ListNotesFiltered(ctx, contextRecord.ID, &workRevisionID, cursor, limit+1, state, r.URL.Query().Get("q"))
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	items, hasMore, nextCursor := reviewIDItems(notes, limit, "notes", func(item database.ReviewNote) int64 { return item.ID })
	s.respond(w, r, map[string]any{"items": items, "notes": items, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
}

// runNotes returns a searchable run-scoped index of active and removed note heads.
func (s *Server) runNotes(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := validateKnownQuery(r, "cursor", "limit", "state", "q"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, limit, err := reviewIDPage(r, "run_notes")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireContextForRead(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "all"
	}
	notes, err := s.writeDB.Reviews.ListNotesFiltered(ctx, contextRecord.ID, nil, cursor, limit+1, state, r.URL.Query().Get("q"))
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	items, hasMore, nextCursor := reviewIDItems(notes, limit, "run_notes", func(item database.ReviewNote) int64 { return item.ID })
	s.respond(w, r, map[string]any{"items": items, "notes": items, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
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
	cursor, limit, err := reviewIDPage(r, "note_versions")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versions, err := s.writeDB.Reviews.ListNoteVersions(ctx, contextRecord.ID, noteID, cursor, limit+1)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	items, hasMore, nextCursor := reviewIDItems(versions, limit, "note_versions", func(item database.ReviewNoteVersion) int64 { return item.ID })
	s.respond(w, r, map[string]any{"run_id": runID, "items": items, "versions": items, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
}

// noteVersion returns one full immutable note body and resolved link set from the selected ancestry.
func (s *Server) noteVersion(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, noteID, contextRecord, ctx, cancel, err := s.reviewNoteRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versionID, err := positiveID(r.PathValue("version_id"))
	if err != nil || validateKnownQuery(r) != nil {
		s.respond(w, r, nil, badRequest("version_id must be positive and query parameters are not supported"))
		return
	}
	version, err := s.writeDB.Reviews.GetNoteVersion(ctx, contextRecord.ID, noteID, versionID)
	if err == nil && version == nil {
		err = notFound("note version not found in selected context")
	}
	s.respond(w, r, map[string]any{"run_id": runID, "version": version}, mapReviewError(err))
}

// createNoteVersion creates an active edit or deletion tombstone.
func (s *Server) createNoteVersion(w http.ResponseWriter, r *http.Request) {
	_, noteID, contextRecord, ctx, cancel, err := s.reviewNoteRequest(r)
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
	limit, err := reviewLimit(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireContextForRead(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	cursor, err := decodeReviewCursor(r.URL.Query().Get("cursor"), "anchors")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	anchors, err := s.writeDB.Reviews.ListAnchors(ctx, contextRecord.ID, workRevisionID, cursor.Text, limit+1)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	hasMore := len(anchors) > limit
	if hasMore {
		anchors = anchors[:limit]
	}
	var nextCursor *string
	if hasMore {
		value := encodeReviewCursor(reviewCursor{Kind: "anchors", Text: anchors[len(anchors)-1].ID})
		nextCursor = &value
	}
	s.respond(w, r, map[string]any{"items": anchors, "anchors": anchors, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
}

// createArticleAnchor creates a logical anchor and its first immutable geometry version.
func (s *Server) createArticleAnchor(w http.ResponseWriter, r *http.Request) {
	runID, workRevisionID, err := reviewArticleIDs(r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		Label        string                     `json:"label"`
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
	anchor, err := s.writeDB.Reviews.CreateAnchor(ctx, contextRecord.ID, workRevisionID, request.Label,
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
	cursor, limit, err := reviewIDPage(r, "anchor_versions")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versions, err := s.writeDB.Reviews.ListAnchorVersions(ctx, contextRecord.ID, anchorID, cursor, limit+1)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	items, hasMore, nextCursor := reviewIDItems(versions, limit, "anchor_versions", func(item database.ReviewAnchorVersion) int64 { return item.ID })
	anchor, err := s.writeDB.Reviews.GetAnchor(ctx, contextRecord.ID, anchorID)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	if anchor == nil {
		s.respond(w, r, nil, notFound("review anchor not found in selected context"))
		return
	}
	s.respond(w, r, map[string]any{"run_id": runID, "anchor": anchor, "items": items, "versions": items, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
}

// anchorVersion returns one full immutable anchor version from the selected ancestry.
func (s *Server) anchorVersion(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, anchorID, contextRecord, ctx, cancel, err := s.reviewAnchorRequest(r)
	defer cancel()
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	versionID, err := positiveID(r.PathValue("version_id"))
	if err != nil || validateKnownQuery(r) != nil {
		s.respond(w, r, nil, badRequest("version_id must be positive and query parameters are not supported"))
		return
	}
	version, err := s.writeDB.Reviews.GetAnchorVersion(ctx, contextRecord.ID, anchorID, versionID)
	if err == nil && version == nil {
		err = notFound("anchor version not found in selected context")
	}
	s.respond(w, r, map[string]any{"run_id": runID, "version": version}, mapReviewError(err))
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
		RestoreFromID     *int64                     `json:"restore_from_version_id"`
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
	if request.RestoreFromID != nil {
		if request.State != "active" || *request.RestoreFromID < 1 {
			s.respond(w, r, nil, badRequest("restore_from_version_id requires a positive active anchor version"))
			return
		}
		var restoredHash, rectanglesJSON string
		if err := s.db.QueryRowContext(ctx, `SELECT pdf_content_hash, page, selected_text, rectangles_json
			FROM review_anchor_versions WHERE id=? AND anchor_id=? AND state='active'`, *request.RestoreFromID, anchorID).
			Scan(&restoredHash, &request.Page, &request.SelectedText, &rectanglesJSON); err == sql.ErrNoRows {
			s.respond(w, r, nil, notFound("restorable anchor version not found"))
			return
		} else if err != nil {
			s.respond(w, r, nil, err)
			return
		}
		if restoredHash != contentHash {
			s.respond(w, r, nil, &apiProblem{Status: http.StatusConflict, Code: "anchor_pdf_changed", Message: "the selected anchor version belongs to different PDF content"})
			return
		}
		if err := json.Unmarshal([]byte(rectanglesJSON), &request.Rectangles); err != nil {
			s.respond(w, r, nil, err)
			return
		}
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
	if err := validateKnownQuery(r, "target_type", "target_id", "work_revision_id", "cursor", "limit"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	targetType, targetID := r.URL.Query().Get("target_type"), r.URL.Query().Get("target_id")
	if !map[string]bool{"note": true, "article": true, "pdf_page": true, "anchor": true, "ext": true}[targetType] || targetID == "" {
		s.respond(w, r, nil, badRequest("target_type and target_id are required"))
		return
	}
	if targetType == "pdf_page" && r.URL.Query().Get("work_revision_id") == "" {
		s.respond(w, r, nil, badRequest("work_revision_id is required for PDF page backlinks"))
		return
	}
	cursor, limit, err := reviewIDPage(r, "backlinks")
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	contextRecord, err := s.requireContextForRead(ctx, runID)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var sourceWorkID int64
	if raw := r.URL.Query().Get("work_revision_id"); raw != "" {
		workRevisionID, parseErr := positiveID(raw)
		if parseErr != nil {
			s.respond(w, r, nil, badRequest("work_revision_id must be positive"))
			return
		}
		sourceWorkID, _, err = s.reviewArticlePDF(ctx, runID, workRevisionID)
		if err != nil {
			s.respond(w, r, nil, err)
			return
		}
	}
	items, err := s.writeDB.Reviews.ListBacklinks(ctx, contextRecord.ID, targetType, targetID, sourceWorkID, cursor, limit+1)
	if err != nil {
		s.respond(w, r, nil, mapReviewError(err))
		return
	}
	pageItems, hasMore, nextCursor := reviewIDItems(items, limit, "backlinks", func(item database.ReviewNote) int64 { return item.ID })
	s.respond(w, r, map[string]any{"items": pageItems, "backlinks": pageItems, "limit": limit, "has_more": hasMore, "next_cursor": nextCursor}, nil)
}

// reviewRunRecord contains the lifecycle fields that gate local review.
type reviewRunRecord struct{ Status, Visibility string }

// loadReviewRun returns lifecycle fields without rejecting read-only historical contexts.
func (s *Server) loadReviewRun(ctx context.Context, runID int64) (reviewRunRecord, error) {
	var run reviewRunRecord
	err := s.db.QueryRowContext(ctx, "SELECT status, visibility_state FROM pipeline_runs WHERE id=?", runID).Scan(&run.Status, &run.Visibility)
	if err == sql.ErrNoRows {
		return run, notFound("pipeline run not found")
	}
	return run, err
}

// requireReviewableRun rejects missing, failed, running, or trashed run contexts.
func (s *Server) requireReviewableRun(ctx context.Context, runID int64) (reviewRunRecord, error) {
	run, err := s.loadReviewRun(ctx, runID)
	if err != nil {
		return run, err
	}
	if run.Status != "completed" || run.Visibility == "trashed" {
		return run, &apiProblem{Status: http.StatusConflict, Code: "run_not_reviewable", Message: "only completed non-trashed runs can be reviewed"}
	}
	return run, nil
}

// requireContextForRead returns an existing context for active or read-only historical runs.
func (s *Server) requireContextForRead(ctx context.Context, runID int64) (*database.ReviewContext, error) {
	if _, err := s.loadReviewRun(ctx, runID); err != nil {
		return nil, err
	}
	contextRecord, err := s.writeDB.Reviews.GetContextByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if contextRecord == nil {
		return nil, &apiProblem{Status: http.StatusConflict, Code: "review_context_required", Message: "start review for this run before reading review history"}
	}
	return contextRecord, nil
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
	err := s.db.QueryRowContext(ctx, `SELECT revision.work_id FROM work_revisions revision
		WHERE revision.id=? AND revision.pipeline_run_id=? AND `+currentNormalizedRevisionPredicate("revision"), workRevisionID, runID).Scan(&workID)
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
		return 0, "", &apiProblem{Status: http.StatusConflict, Code: "pdf_unavailable", Message: "an available PDF is required for anchor changes"}
	}
	hash, _ := status["content_hash"].(string)
	return workID, hash, nil
}

// requireAvailableWorkPDF gates PDF anchor mutation on run membership and matching PDF bytes.
func (s *Server) requireAvailableWorkPDF(ctx context.Context, runID, workID int64) (int64, string, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_revisions revision
		WHERE revision.pipeline_run_id=? AND revision.work_id=? AND `+currentNormalizedRevisionPredicate("revision"), runID, workID).Scan(&count); err != nil {
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
		return 0, "", &apiProblem{Status: http.StatusConflict, Code: "pdf_unavailable", Message: "an available PDF is required for anchor changes"}
	}
	hash, _ := status["content_hash"].(string)
	return workID, hash, nil
}

// reviewSummaryCounts returns current note, anchor, and decision-version summary counts.
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
	contextRecord, err := s.requireContextForRead(ctx, runID)
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
	contextRecord, err := s.requireContextForRead(ctx, runID)
	return runID, anchorID, contextRecord, ctx, cancel, err
}

// reviewCursor is the endpoint-bound opaque keyset carried between review collection pages.
type reviewCursor struct {
	Kind      string `json:"k"`
	ID        int64  `json:"i,omitempty"`
	Text      string `json:"s,omitempty"`
	StartedAt string `json:"t,omitempty"`
}

// reviewLimit validates the public page-size boundary.
func reviewLimit(r *http.Request) (int, error) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			return 0, badRequest("limit must be between 1 and 100")
		}
		limit = value
	}
	return limit, nil
}

// reviewIDPage decodes an endpoint-bound numeric keyset cursor and page limit.
func reviewIDPage(r *http.Request, kind string) (int64, int, error) {
	limit, err := reviewLimit(r)
	if err != nil {
		return 0, 0, err
	}
	cursor, err := decodeReviewCursor(r.URL.Query().Get("cursor"), kind)
	if err != nil {
		return 0, 0, err
	}
	if r.URL.Query().Get("cursor") != "" && cursor.ID < 1 {
		return 0, 0, badRequest("cursor is invalid for this collection")
	}
	return cursor.ID, limit, nil
}

// decodeReviewCursor validates an opaque cursor against its owning collection.
func decodeReviewCursor(raw, kind string) (reviewCursor, error) {
	if raw == "" {
		return reviewCursor{Kind: kind}, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return reviewCursor{}, badRequest("cursor is invalid for this collection")
	}
	var cursor reviewCursor
	if err := json.Unmarshal(encoded, &cursor); err != nil || cursor.Kind != kind {
		return reviewCursor{}, badRequest("cursor is invalid for this collection")
	}
	return cursor, nil
}

// encodeReviewCursor serializes one endpoint-bound keyset without exposing its structure.
func encodeReviewCursor(cursor reviewCursor) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// reviewIDItems trims a limit-plus-one result and encodes the last visible numeric key.
func reviewIDItems[T any](items []T, limit int, kind string, id func(T) int64) ([]T, bool, *string) {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	if !hasMore {
		return items, false, nil
	}
	value := encodeReviewCursor(reviewCursor{Kind: kind, ID: id(items[len(items)-1])})
	return items, true, &value
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
	var parentConflict *database.ReviewContextParentConflictError
	if errors.As(err, &parentConflict) {
		return &apiProblem{Status: http.StatusConflict, Code: "context_parent_conflict", Message: parentConflict.Error(), Details: map[string]any{"requested_parent_context_id": parentConflict.Requested, "existing_parent_context_id": parentConflict.Existing}}
	}
	var labelConflict *database.ReviewAnchorLabelConflictError
	if errors.As(err, &labelConflict) {
		return &apiProblem{Status: http.StatusConflict, Code: "anchor_label_conflict", Message: labelConflict.Error(), Details: map[string]any{"label": labelConflict.Label}}
	}
	var syntax *database.NoteSyntaxError
	if errors.As(err, &syntax) {
		return &apiProblem{Status: http.StatusBadRequest, Code: "note_syntax_error", Message: syntax.Error(), Details: map[string]any{"syntax_errors": syntax.Errors}}
	}
	var repositoryError *database.ReviewError
	if errors.As(err, &repositoryError) {
		switch repositoryError.Kind {
		case "validation":
			return &apiProblem{Status: http.StatusBadRequest, Code: "invalid_review_request", Message: repositoryError.Message}
		case "not_found":
			return &apiProblem{Status: http.StatusNotFound, Code: "review_record_not_found", Message: repositoryError.Message}
		case "lifecycle":
			return &apiProblem{Status: http.StatusConflict, Code: "review_read_only", Message: repositoryError.Message}
		}
	}
	return err
}

// setMutableResponseHeaders prevents caching context-sensitive review responses.
func setMutableResponseHeaders(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }
