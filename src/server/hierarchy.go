// hierarchy.go provides bounded Home discovery for search history and run management.
package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	var searches, revisions, plans, runs, completed int64
	err := s.db.QueryRowContext(ctx, `SELECT
        (SELECT COUNT(*) FROM searches),
        (SELECT COUNT(*) FROM search_revisions),
        (SELECT COUNT(*) FROM execution_plans),
        (SELECT COUNT(*) FROM pipeline_runs),
        (SELECT COUNT(*) FROM pipeline_runs WHERE status='completed')`).Scan(&searches, &revisions, &plans, &runs, &completed)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestHierarchyRun(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"totals": map[string]any{
			"searches": searches, "revisions": revisions, "plans": plans, "runs": runs, "completed_runs": completed,
		},
		"latest_run": latest,
	}, nil
}

// latestHierarchyRun returns the newest run with complete ancestry when one exists.
func (s *Server) latestHierarchyRun(ctx context.Context) (any, error) {
	row := s.db.QueryRowContext(ctx, `SELECT pr.id, pr.attempt_number, pr.started_at, pr.finished_at,
        pr.status, pr.visibility_state, s.id, s.search_id, sr.id, sr.revision_label, ep.id
		FROM pipeline_runs pr
		LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		LEFT JOIN search_revisions sr ON sr.id=ep.search_revision_id
		LEFT JOIN searches s ON s.id=sr.search_id
        ORDER BY pr.id DESC LIMIT 1`)
	item, err := scanHierarchyRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// hierarchySearches returns one bounded server-searchable page of search summaries.
func (s *Server) hierarchySearches(ctx context.Context, r *http.Request) (map[string]any, error) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := hierarchyScope("searches", query)
	cursor, err := decodeHierarchyCursor(r.URL.Query().Get("cursor"), "searches", scope)
	if err != nil {
		return nil, err
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if query != "" {
		pattern := "%" + strings.ToLower(query) + "%"
		clauses = append(clauses, `(LOWER(s.search_id) LIKE ? OR EXISTS (
            SELECT 1 FROM search_revisions matched_sr
            WHERE matched_sr.search_id=s.id AND LOWER(matched_sr.revision_label) LIKE ?))`)
		args = append(args, pattern, pattern)
	}
	if cursor.ID > 0 {
		clauses = append(clauses, "s.id < ?")
		args = append(args, cursor.ID)
	}
	sqlQuery := `SELECT s.id, s.search_id, s.created_at,
        COUNT(DISTINCT sr.id) AS revision_count,
        COUNT(DISTINCT ep.id) AS plan_count,
        COUNT(DISTINCT pr.id) AS run_count,
        MAX(pr.id) AS latest_run_id,
        (SELECT latest_pr.execution_plan_id FROM pipeline_runs latest_pr
         JOIN execution_plans latest_ep ON latest_ep.id=latest_pr.execution_plan_id
         JOIN search_revisions latest_sr ON latest_sr.id=latest_ep.search_revision_id
         WHERE latest_sr.search_id=s.id ORDER BY latest_pr.id DESC LIMIT 1) AS latest_plan_id,
        (SELECT latest_ep.search_revision_id FROM pipeline_runs latest_pr
         JOIN execution_plans latest_ep ON latest_ep.id=latest_pr.execution_plan_id
         JOIN search_revisions latest_sr ON latest_sr.id=latest_ep.search_revision_id
         WHERE latest_sr.search_id=s.id ORDER BY latest_pr.id DESC LIMIT 1) AS latest_revision_id
        FROM searches s
        LEFT JOIN search_revisions sr ON sr.search_id=s.id
        LEFT JOIN execution_plans ep ON ep.search_revision_id=sr.id
        LEFT JOIN pipeline_runs pr ON pr.execution_plan_id=ep.id`
	if len(clauses) > 0 {
		sqlQuery += " WHERE " + strings.Join(clauses, " AND ")
	}
	sqlQuery += " GROUP BY s.id ORDER BY s.id DESC LIMIT ?"
	args = append(args, hierarchyPageLimit+1)
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, hierarchyPageLimit+1)
	for rows.Next() {
		var id, revisions, plans, runs int64
		var searchID, createdAt string
		var latestRunID, latestPlanID, latestRevisionID sql.NullInt64
		if err := rows.Scan(&id, &searchID, &createdAt, &revisions, &plans, &runs, &latestRunID, &latestPlanID, &latestRevisionID); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "search_id": searchID, "created_at": createdAt,
			"revision_count": revisions, "plan_count": plans, "run_count": runs,
			"latest_run_id": nullableInt64(latestRunID), "latest_plan_id": nullableInt64(latestPlanID), "latest_revision_id": nullableInt64(latestRevisionID),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	page := hierarchyPage("searches", scope, items)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		var id int64
		var searchID, createdAt string
		err := s.db.QueryRowContext(ctx, "SELECT id, search_id, created_at FROM searches WHERE id=?", selectedID).Scan(&id, &searchID, &createdAt)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil {
			page["selected_item"] = map[string]any{"id": id, "search_id": searchID, "created_at": createdAt}
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
	query := `SELECT sr.id, sr.revision_label, sr.created_at,
        COUNT(DISTINCT ep.id), COUNT(DISTINCT pr.id), MAX(pr.id),
        (SELECT latest_pr.execution_plan_id FROM pipeline_runs latest_pr
         JOIN execution_plans latest_ep ON latest_ep.id=latest_pr.execution_plan_id
         WHERE latest_ep.search_revision_id=sr.id ORDER BY latest_pr.id DESC LIMIT 1)
        FROM search_revisions sr
        LEFT JOIN execution_plans ep ON ep.search_revision_id=sr.id
        LEFT JOIN pipeline_runs pr ON pr.execution_plan_id=ep.id
		WHERE sr.search_id=?`
	args := []any{searchID}
	if searchQuery != "" {
		query += " AND (LOWER(sr.revision_label) LIKE ? OR CAST(sr.id AS TEXT) LIKE ?)"
		pattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, pattern, pattern)
	}
	if cursor.ID > 0 {
		query += " AND sr.id < ?"
		args = append(args, cursor.ID)
	}
	query += " GROUP BY sr.id ORDER BY sr.id DESC LIMIT ?"
	args = append(args, hierarchyPageLimit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, hierarchyPageLimit+1)
	for rows.Next() {
		var id, plans, runs int64
		var label, createdAt string
		var latestRunID, latestPlanID sql.NullInt64
		if err := rows.Scan(&id, &label, &createdAt, &plans, &runs, &latestRunID, &latestPlanID); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "label": label, "created_at": createdAt, "plan_count": plans, "run_count": runs,
			"latest_run_id": nullableInt64(latestRunID), "latest_plan_id": nullableInt64(latestPlanID),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	page := hierarchyPage("revisions", scope, items)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		var id int64
		var label, createdAt string
		err := s.db.QueryRowContext(ctx, "SELECT id, revision_label, created_at FROM search_revisions WHERE id=? AND search_id=?", selectedID, searchID).Scan(&id, &label, &createdAt)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil {
			page["selected_item"] = map[string]any{"id": id, "label": label, "created_at": createdAt}
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
	query := `SELECT id, search_revision_id, execution_fingerprint, enrichment_enabled, created_at
        FROM execution_plans WHERE search_revision_id=?`
	args := []any{revisionID}
	if searchQuery != "" {
		query += " AND (LOWER(execution_fingerprint) LIKE ? OR CAST(id AS TEXT) LIKE ?)"
		pattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, pattern, pattern)
	}
	if cursor.ID > 0 {
		query += " AND id < ?"
		args = append(args, cursor.ID)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, hierarchyPageLimit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, hierarchyPageLimit+1)
	for rows.Next() {
		var id, parentID int64
		var fingerprint, createdAt string
		var enrichmentEnabled bool
		if err := rows.Scan(&id, &parentID, &fingerprint, &enrichmentEnabled, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "search_revision_id": parentID, "execution_fingerprint": fingerprint,
			"enrichment_enabled": enrichmentEnabled, "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	page := hierarchyPage("plans", scope, items)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		var id int64
		var fingerprint string
		err := s.db.QueryRowContext(ctx, "SELECT id, execution_fingerprint FROM execution_plans WHERE id=? AND search_revision_id=?", selectedID, revisionID).Scan(&id, &fingerprint)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil {
			page["selected_item"] = map[string]any{"id": id, "execution_fingerprint": fingerprint}
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
	query := `SELECT id, execution_plan_id, attempt_number, started_at, finished_at, status, visibility_state
        FROM pipeline_runs WHERE execution_plan_id=? AND visibility_state!='trashed'`
	args := []any{planID}
	if searchQuery != "" {
		query += " AND (CAST(id AS TEXT) LIKE ? OR LOWER(status) LIKE ? OR LOWER(started_at) LIKE ?)"
		pattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if cursor.ID > 0 {
		query += " AND id < ?"
		args = append(args, cursor.ID)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, hierarchyPageLimit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, hierarchyPageLimit+1)
	for rows.Next() {
		var id, parentID int64
		var attempt sql.NullInt64
		var startedAt, status, visibility string
		var finishedAt sql.NullString
		if err := rows.Scan(&id, &parentID, &attempt, &startedAt, &finishedAt, &status, &visibility); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "execution_plan_id": parentID, "attempt_number": nullableInt64(attempt), "started_at": startedAt,
			"finished_at": nullableString(finishedAt), "status": status, "visibility_state": visibility,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	page := hierarchyPage("attempts", scope, items)
	selectedID, err := optionalHierarchyID(r, "selected_id")
	if err != nil {
		return nil, err
	}
	if selectedID > 0 {
		var id int64
		var attempt sql.NullInt64
		var startedAt, status, visibility string
		err := s.db.QueryRowContext(ctx, `SELECT id, attempt_number, started_at, status, visibility_state
            FROM pipeline_runs WHERE id=? AND execution_plan_id=? AND visibility_state!='trashed'`, selectedID, planID).Scan(&id, &attempt, &startedAt, &status, &visibility)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil {
			page["selected_item"] = map[string]any{
				"id": id, "attempt_number": nullableInt64(attempt), "started_at": startedAt,
				"status": status, "visibility_state": visibility,
			}
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
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 10)
	if query != "" {
		pattern := "%" + strings.ToLower(query) + "%"
		clauses = append(clauses, "(LOWER(s.search_id) LIKE ? OR LOWER(sr.revision_label) LIKE ? OR CAST(pr.id AS TEXT) LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}
	if visibility != "all" {
		if visibility == "active" {
			clauses = append(clauses, "pr.visibility_state!='trashed'")
		} else {
			clauses = append(clauses, "pr.visibility_state='trashed'")
		}
	}
	if status != "" && status != "all" {
		clauses = append(clauses, "pr.status=?")
		args = append(args, status)
	}
	if startedAfter != "" {
		clauses = append(clauses, "datetime(pr.started_at) >= datetime(?)")
		args = append(args, startedAfter)
	}
	if startedBefore != "" {
		clauses = append(clauses, "datetime(pr.started_at) < datetime(?)")
		args = append(args, startedBefore)
	}
	if cursor.ID > 0 {
		clauses = append(clauses, "pr.id < ?")
		args = append(args, cursor.ID)
	}
	sqlQuery := `SELECT pr.id, pr.attempt_number, pr.started_at, pr.finished_at,
        pr.status, pr.visibility_state, s.id, s.search_id, sr.id, sr.revision_label, ep.id
		FROM pipeline_runs pr
		LEFT JOIN execution_plans ep ON ep.id=pr.execution_plan_id
		LEFT JOIN search_revisions sr ON sr.id=ep.search_revision_id
		LEFT JOIN searches s ON s.id=sr.search_id`
	if len(clauses) > 0 {
		sqlQuery += " WHERE " + strings.Join(clauses, " AND ")
	}
	sqlQuery += " ORDER BY pr.id DESC LIMIT ?"
	args = append(args, hierarchyPageLimit+1)
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, hierarchyPageLimit+1)
	for rows.Next() {
		item, err := scanHierarchyRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hierarchyPage("runs", scope, items), nil
}

// hierarchyScanner is the shared Scan contract for one row or rows cursor.
type hierarchyScanner interface {
	Scan(...any) error
}

// scanHierarchyRun scans one run and its complete search ancestry.
func scanHierarchyRun(scanner hierarchyScanner) (map[string]any, error) {
	var id int64
	var attempt, searchID, revisionID, planID sql.NullInt64
	var startedAt, status, visibility, searchName, revisionLabel string
	var nullableSearchName, nullableRevisionLabel sql.NullString
	var finishedAt sql.NullString
	if err := scanner.Scan(&id, &attempt, &startedAt, &finishedAt, &status, &visibility, &searchID, &nullableSearchName, &revisionID, &nullableRevisionLabel, &planID); err != nil {
		return nil, err
	}
	searchName = nullableSearchName.String
	revisionLabel = nullableRevisionLabel.String
	return map[string]any{
		"id": id, "attempt_number": nullableInt64(attempt), "started_at": startedAt, "finished_at": nullableString(finishedAt),
		"status": status, "visibility_state": visibility,
		"search_id": nullableInt64(searchID), "search_name": searchName, "search_revision_id": nullableInt64(revisionID), "revision_label": revisionLabel, "execution_plan_id": nullableInt64(planID),
	}, nil
}

// hierarchyPage trims the lookahead row and emits an opaque continuation cursor.
func hierarchyPage(kind, scope string, items []map[string]any) map[string]any {
	hasMore := len(items) > hierarchyPageLimit
	if hasMore {
		items = items[:hierarchyPageLimit]
	}
	nextCursor := ""
	if hasMore {
		id, _ := items[len(items)-1]["id"].(int64)
		nextCursor = encodeHierarchyCursor(hierarchyCursor{Kind: kind, Scope: scope, ID: id})
	}
	return map[string]any{"items": items, "has_more": hasMore, "next_cursor": nextCursor, "limit": hierarchyPageLimit}
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

// encodeHierarchyCursor serializes one endpoint-bound keyset without exposing its structure.
func encodeHierarchyCursor(cursor hierarchyCursor) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// nullableInt64 converts a nullable identifier to the invariant JSON null-or-number shape.
func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

// nullableString converts an optional stored string to the invariant JSON null-or-string shape.
func nullableString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}
