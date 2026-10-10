// hierarchy.go provides bounded Home discovery for search history and run management.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"analysis/database/run"
	"analysis/database/search"
)

const hierarchyPageLimit = 20

// hierarchyCursor is an endpoint-bound keyset for descending identifier traversal.
type hierarchyCursor struct {
	Kind  string `json:"kind"`
	Scope string `json:"scope"`
	ID    int64  `json:"id"`
}

// hierarchy serves independently recoverable summary, search, revision, and run sections.
func (s *Server) hierarchy(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "section", "q", "visibility", "status", "started_after", "started_before", "search_id", "search_revision_id", "plan_id", "selected_id", "cursor"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	section := r.URL.Query().Get("section")
	if section == "" {
		section = "summary"
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var value any
	var err error
	switch section {
	case "summary":
		value, err = s.hierarchySummary(ctx)
	case "searches":
		value, err = s.hierarchySearches(ctx, r)
	case "revisions":
		value, err = s.hierarchyRevisions(ctx, r)
	case "plans":
		value, err = s.hierarchyPlans(ctx, r)
	case "attempts":
		value, err = s.hierarchyAttempts(ctx, r)
	case "runs":
		value, err = s.hierarchyRuns(ctx, r)
	default:
		err = badRequest("section must be summary, searches, revisions, plans, attempts, or runs")
	}
	if err == nil {
		value.(map[string]any)["version"] = "1"
	}
	s.respond(w, r, value, err)
}

// hierarchySummary returns current workspace totals and the latest planned run.
func (s *Server) hierarchySummary(ctx context.Context) (map[string]any, error) {
	totals, err := s.searchStore.HierarchyTotals(ctx)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestHierarchyRun(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"totals": map[string]any{
			"searches":       totals.Searches,
			"revisions":      totals.Revisions,
			"plans":          totals.Plans,
			"runs":           totals.Runs,
			"completed_runs": totals.CompletedRuns,
		},
		"latest_run": latest,
	}, nil
}

// latestHierarchyRun returns the newest run with complete ancestry when one exists.
func (s *Server) latestHierarchyRun(ctx context.Context) (any, error) {
	ancestry, err := s.runStore.LatestRunWithAncestry(ctx)
	if err != nil {
		return nil, err
	}
	if ancestry == nil {
		return nil, nil
	}
	return runAncestryRow(ancestry), nil
}

// hierarchySearches returns one bounded server-searchable page of search summaries.
func (s *Server) hierarchySearches(ctx context.Context, r *http.Request) (map[string]any, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := hierarchyScope("searches", query)
	cursor, err := decodeHierarchyCursor(r.URL.Query().Get("cursor"), "searches", scope)
	if err != nil {
		return nil, err
	}
	rows, hasMore, err := s.searchStore.ListHierarchySearches(ctx, search.HierarchySearchFilter{
		Query:    query,
		CursorID: cursor.ID,
		Limit:    hierarchyPageLimit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                 row.ID,
			"search_id":          row.SearchID,
			"created_at":         row.CreatedAt,
			"revision_count":     row.RevisionCount,
			"plan_count":         row.PlanCount,
			"run_count":          row.RunCount,
			"latest_run_id":      nullableIDPointer(row.LatestRunID),
			"latest_plan_id":     nullableIDPointer(row.LatestPlanID),
			"latest_revision_id": nullableIDPointer(row.LatestRevisionID),
		})
	}
	page := hierarchyPage("searches", scope, items, hasMore)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		selected, err := s.searchStore.GetSearchByID(ctx, selectedID)
		if err != nil {
			return nil, err
		}
		if selected != nil {
			page["selected_item"] = map[string]any{
				"id":         selected.ID,
				"search_id":  selected.SearchID,
				"created_at": selected.CreatedAt,
			}
		}
	}
	return page, nil
}

// hierarchyRevisions returns one bounded page of revision summaries for a selected search.
func (s *Server) hierarchyRevisions(ctx context.Context, r *http.Request) (map[string]any, error) {
	searchID, err := hierarchyRequiredID(r, "search_id")
	if err != nil {
		return nil, err
	}
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := hierarchyScope("revisions", strconv.FormatInt(searchID, 10), searchQuery)
	cursor, err := decodeHierarchyCursor(r.URL.Query().Get("cursor"), "revisions", scope)
	if err != nil {
		return nil, err
	}
	rows, hasMore, err := s.searchStore.ListHierarchyRevisions(ctx, search.HierarchyRevisionFilter{
		SearchID: searchID,
		Query:    searchQuery,
		CursorID: cursor.ID,
		Limit:    hierarchyPageLimit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":             row.ID,
			"label":          row.Label,
			"created_at":     row.CreatedAt,
			"plan_count":     row.PlanCount,
			"run_count":      row.RunCount,
			"latest_run_id":  nullableIDPointer(row.LatestRunID),
			"latest_plan_id": nullableIDPointer(row.LatestPlanID),
		})
	}
	page := hierarchyPage("revisions", scope, items, hasMore)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		selected, err := s.searchStore.GetRevisionByID(ctx, selectedID)
		if err != nil {
			return nil, err
		}
		if selected != nil && selected.SearchID == searchID {
			page["selected_item"] = map[string]any{
				"id":         selected.ID,
				"label":      selected.RevisionLabel,
				"created_at": selected.CreatedAt,
			}
		}
	}
	return page, nil
}

// hierarchyPlans returns one bounded searchable page of execution plans for a selected revision.
func (s *Server) hierarchyPlans(ctx context.Context, r *http.Request) (map[string]any, error) {
	revisionID, err := hierarchyRequiredID(r, "search_revision_id")
	if err != nil {
		return nil, err
	}
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := hierarchyScope("plans", strconv.FormatInt(revisionID, 10), searchQuery)
	cursor, err := decodeHierarchyCursor(r.URL.Query().Get("cursor"), "plans", scope)
	if err != nil {
		return nil, err
	}
	rows, hasMore, err := s.searchStore.ListHierarchyPlans(ctx, search.HierarchyPlanFilter{
		SearchRevisionID: revisionID,
		Query:            searchQuery,
		CursorID:         cursor.ID,
		Limit:            hierarchyPageLimit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                    row.ID,
			"search_revision_id":    row.SearchRevisionID,
			"execution_fingerprint": row.ExecutionFingerprint,
			"enrichment_enabled":    row.EnrichmentEnabled,
			"created_at":            row.CreatedAt,
		})
	}
	page := hierarchyPage("plans", scope, items, hasMore)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		selected, err := s.searchStore.GetPlanByID(ctx, selectedID)
		if err != nil {
			return nil, err
		}
		if selected != nil && selected.SearchRevisionID == revisionID {
			page["selected_item"] = map[string]any{
				"id":                    selected.ID,
				"execution_fingerprint": selected.ExecutionFingerprint,
			}
		}
	}
	return page, nil
}

// hierarchyAttempts returns one bounded searchable page of non-trashed attempts for a selected plan.
func (s *Server) hierarchyAttempts(ctx context.Context, r *http.Request) (map[string]any, error) {
	planID, err := hierarchyRequiredID(r, "plan_id")
	if err != nil {
		return nil, err
	}
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := hierarchyScope("attempts", strconv.FormatInt(planID, 10), searchQuery)
	cursor, err := decodeHierarchyCursor(r.URL.Query().Get("cursor"), "attempts", scope)
	if err != nil {
		return nil, err
	}
	rows, hasMore, err := s.runStore.ListHierarchyAttempts(ctx, run.HierarchyAttemptFilter{
		ExecutionPlanID: planID,
		Query:           searchQuery,
		CursorID:        cursor.ID,
		Limit:           hierarchyPageLimit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                row.ID,
			"execution_plan_id": nullableIDPointer(row.ExecutionPlanID),
			"attempt_number":    nullableIDPointer(row.AttemptNumber),
			"started_at":        row.StartedAt,
			"finished_at":       nullableText(row.FinishedAt),
			"status":            row.Status,
			"visibility_state":  row.VisibilityState,
		})
	}
	page := hierarchyPage("attempts", scope, items, hasMore)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		attempt, err := s.runStore.SelectedAttempt(ctx, planID, selectedID)
		if err != nil {
			return nil, err
		}
		if attempt != nil {
			page["selected_item"] = selectedAttemptRow(attempt)
		}
	}
	return page, nil
}

// hierarchyRuns returns one bounded filtered page of run attempts with complete ancestry.
func (s *Server) hierarchyRuns(ctx context.Context, r *http.Request) (map[string]any, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	visibility := r.URL.Query().Get("visibility")
	if visibility == "" {
		visibility = "active"
	}
	if visibility != "active" && visibility != "trashed" && visibility != "all" {
		return nil, badRequest("visibility must be active, trashed, or all")
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "all" && status != "running" && status != "completed" && status != "failed" {
		return nil, badRequest("status must be running, completed, failed, or all")
	}
	startedAfter, err := hierarchyDate(r.URL.Query().Get("started_after"), "started_after")
	if err != nil {
		return nil, err
	}
	startedBefore, err := hierarchyDate(r.URL.Query().Get("started_before"), "started_before")
	if err != nil {
		return nil, err
	}
	scope := hierarchyScope("runs", query, visibility, status, startedAfter, startedBefore)
	cursor, err := decodeHierarchyCursor(r.URL.Query().Get("cursor"), "runs", scope)
	if err != nil {
		return nil, err
	}
	rows, hasMore, err := s.runStore.ListHierarchyRuns(ctx, run.HierarchyRunFilter{
		Query:         query,
		Visibility:    visibility,
		Status:        status,
		StartedAfter:  startedAfter,
		StartedBefore: startedBefore,
		CursorID:      cursor.ID,
		Limit:         hierarchyPageLimit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                 row.ID,
			"attempt_number":     nullableIDPointer(row.AttemptNumber),
			"started_at":         row.StartedAt,
			"finished_at":        nullableText(row.FinishedAt),
			"status":             row.Status,
			"visibility_state":   row.VisibilityState,
			"search_id":          nullableIDPointer(row.SearchID),
			"search_name":        row.SearchName,
			"search_revision_id": nullableIDPointer(row.SearchRevisionID),
			"revision_label":     row.RevisionLabel,
			"execution_plan_id":  nullableIDPointer(row.ExecutionPlanID),
		})
	}
	return hierarchyPage("runs", scope, items, hasMore), nil
}

// hierarchyPage trims the lookahead row and emits an opaque continuation cursor.
func hierarchyPage(kind, scope string, items []map[string]any, hasMore bool) map[string]any {
	nextCursor := ""
	if hasMore {
		id, _ := items[len(items)-1]["id"].(int64)
		nextCursor = encodeCursor(hierarchyCursor{
			Kind:  kind,
			Scope: scope,
			ID:    id,
		})
	}
	return map[string]any{"items": items,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
		"limit":       hierarchyPageLimit,
	}
}

// hierarchyDate validates one inclusive-from or exclusive-before calendar boundary.
func hierarchyDate(raw, name string) (string, error) {
	if raw == "" {
		return "", nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return "", badRequest(name + " must use YYYY-MM-DD")
	}
	if name == "started_before" {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return parsed.Format("2006-01-02 15:04:05"), nil
}

// optionalHierarchyID parses an optional selected item identifier for exact membership validation.
func optionalHierarchyID(r *http.Request, name string) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	return positiveID(raw)
}

// hierarchyRequiredID parses one required parent identifier after the handler-level allowlist check.
func hierarchyRequiredID(r *http.Request, name string) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, badRequest(name + " is required")
	}
	return positiveID(raw)
}

// hierarchyScope hashes the section filters so a cursor cannot cross result sets.
func hierarchyScope(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

// decodeHierarchyCursor validates an opaque cursor against its owning filtered collection.
func decodeHierarchyCursor(raw, kind, scope string) (hierarchyCursor, error) {
	if raw == "" {
		return hierarchyCursor{Kind: kind, Scope: scope}, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return hierarchyCursor{}, badRequest("cursor is invalid for this hierarchy query")
	}
	var cursor hierarchyCursor
	if err := json.Unmarshal(encoded, &cursor); err != nil || cursor.Kind != kind || cursor.Scope != scope || cursor.ID < 1 {
		return hierarchyCursor{}, badRequest("cursor is invalid for this hierarchy query")
	}
	return cursor, nil
}

// nullableIDPointer converts an optional family identifier to the invariant JSON null-or-number shape.
func nullableIDPointer(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullableText converts an optional family text value to the invariant JSON null-or-string shape.
func nullableText(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
