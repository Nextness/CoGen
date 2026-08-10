package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"analysis/manifest"
)

// runTrashReasonLimit bounds locally supplied lifecycle explanations.
const runTrashReasonLimit = 1000

// updateRunVisibility moves one terminal run into or out of the reversible trash lifecycle and appends matching audit evidence atomically.
func (s *Server) updateRunVisibility(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := positiveID(r.PathValue("run_id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var request struct {
		VisibilityState string `json:"visibility_state"`
		Reason          string `json:"reason"`
	}
	if err := decodeMutationJSON(w, r, &request); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	request.VisibilityState = strings.TrimSpace(request.VisibilityState)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.VisibilityState != "active" && request.VisibilityState != "trashed" {
		s.respond(w, r, nil, badRequest("visibility_state must be active or trashed"))
		return
	}
	if len([]byte(request.Reason)) > runTrashReasonLimit {
		s.respond(w, r, nil, badRequest("reason must not exceed 1000 UTF-8 bytes"))
		return
	}
	if request.VisibilityState == "trashed" && request.Reason == "" {
		request.Reason = "Moved to trash from the local viewer"
	}

	ctx, cancel := queryContext(r)
	defer cancel()
	tx, err := s.writeDB.DB.BeginTx(ctx, nil)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer tx.Rollback()
	var status, currentVisibility string
	var previousReason sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT status, visibility_state, trash_reason FROM pipeline_runs WHERE id=?", runID).Scan(&status, &currentVisibility, &previousReason); err == sql.ErrNoRows {
		s.respond(w, r, nil, notFound("run not found"))
		return
	} else if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if currentVisibility == request.VisibilityState {
		writeJSON(w, http.StatusOK, map[string]any{"run_id": runID, "visibility_state": currentVisibility, "changed": false})
		return
	}
	if status == string(manifest.AttemptRunning) {
		s.respond(w, r, nil, &apiProblem{Status: http.StatusConflict, Code: "run_active", Message: "a running attempt cannot be moved to or restored from trash"})
		return
	}

	occurredAt := time.Now().UTC().Format(time.RFC3339Nano)
	var action manifest.AuditAction
	if request.VisibilityState == "trashed" {
		action = manifest.AuditRunTrashed
		_, err = tx.ExecContext(ctx, "UPDATE pipeline_runs SET visibility_state='trashed', trashed_at=?, trash_reason=? WHERE id=?", occurredAt, request.Reason, runID)
	} else {
		action = manifest.AuditRunRestored
		_, err = tx.ExecContext(ctx, "UPDATE pipeline_runs SET visibility_state='active', trashed_at=NULL, trash_reason=NULL WHERE id=?", runID)
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	beforeJSON, _ := json.Marshal(map[string]any{"visibility_state": currentVisibility})
	afterJSON, _ := json.Marshal(map[string]any{"visibility_state": request.VisibilityState})
	metadataJSON, _ := json.Marshal(map[string]any{"source": "local_viewer"})
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events
		(occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, before_json, after_json, metadata_json)
		VALUES (?, 'local_user', ?, 'pipeline_run', ?, ?, ?, ?, ?)`,
		occurredAt, runID, strconv.FormatInt(runID, 10), string(action), string(beforeJSON), string(afterJSON), string(metadataJSON)); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run_id": runID, "visibility_state": request.VisibilityState, "changed": true})
}
