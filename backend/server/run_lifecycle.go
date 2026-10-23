package server

import (
	viewrequest "analysis/backend/server/request"
	"errors"
	"net/http"
	"strings"

	"analysis/backend/database/run"
)

// runTrashReasonLimit bounds locally supplied lifecycle explanations.
const runTrashReasonLimit = 1000

// updateRunVisibility moves one terminal run into or out of the reversible trash lifecycle and appends matching audit evidence atomically.
func (s *Server) updateRunVisibility(w http.ResponseWriter, r *http.Request) {
	setMutableResponseHeaders(w)
	runID, err := viewrequest.PositiveID(r.PathValue("run_id"))
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
	if request.VisibilityState != run.VisibilityActive && request.VisibilityState != run.VisibilityTrashed {
		s.respond(w, r, nil, viewrequest.BadRequest("visibility_state must be active or trashed"))
		return
	}
	if len([]byte(request.Reason)) > runTrashReasonLimit {
		s.respond(w, r, nil, viewrequest.BadRequest("reason must not exceed 1000 UTF-8 bytes"))
		return
	}
	if request.VisibilityState == run.VisibilityTrashed && request.Reason == "" {
		request.Reason = "Moved to trash from the local viewer"
	}

	ctx, cancel := queryContext(r)
	defer cancel()
	result, err := s.writeDB.Run.UpdateVisibility(ctx, run.VisibilityInput{
		RunID:           runID,
		VisibilityState: request.VisibilityState,
		Reason:          request.Reason,
	})
	if errors.Is(err, run.ErrRunActive) {
		s.respond(w, r, nil, &viewrequest.Problem{Status: http.StatusConflict, Code: "run_active", Message: err.Error()})
		return
	}
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if result == nil {
		s.respond(w, r, nil, viewrequest.NotFound("run not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":           runID,
		"visibility_state": result.VisibilityState,
		"changed":          result.Changed,
	})
}
