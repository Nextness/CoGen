// viewer.go provides the artifact family's run-scoped artifact evidence page.
// It aggregates every run role, step input/output, cache payload, and identity
// candidate payload relationship into one bounded artifact projection.
package artifact

import (
	"context"
	"fmt"
)

// RunArtifactFilter identifies one bounded page of run artifact evidence in
// either cursor mode or page mode.
type RunArtifactFilter struct {
	RunID    int64
	Query    string
	Role     string
	CursorID int64
	FocusID  int64
	Page     int
	PerPage  int
	PageMode bool
	Order    string
	Limit    int
}

// RunArtifactEvidence is one artifact with its run relationship evidence.
type RunArtifactEvidence struct {
	ID                int64
	ContentHash       string
	ByteSize          int64
	ContentType       string
	CreatedAt         string
	HasBlob           bool
	ArtifactRoles     string
	RelationshipRoles string
	ProducedBySteps   string
	ConsumedBySteps   string
}

// RunArtifactEvidencePage is one bounded page of run artifact evidence.
type RunArtifactEvidencePage struct {
	Items        []*RunArtifactEvidence
	Total        int64
	HasMore      bool
	NextCursorID int64
	Page         int
}

// ListRunArtifactEvidence returns one bounded page of artifact evidence linked
// to a run. The exact count is read only in page mode, matching the viewer's
// cursor mode.
func (s *Store) ListRunArtifactEvidence(ctx context.Context, filter RunArtifactFilter) (*RunArtifactEvidencePage, error) {
	if filter.Limit < 1 {
		return nil, fmt.Errorf("list run artifact evidence: limit must be positive")
	}
	if filter.Order != "ASC" && filter.Order != "DESC" {
		return nil, fmt.Errorf("list run artifact evidence: invalid order %q", filter.Order)
	}
	where, filterArgs := runArtifactFilterClauses(filter)
	page := filter.Page
	var total int64
	if filter.PageMode {
		countArgs := append(runArtifactRunArgs(filter.RunID, 5), filterArgs...)
		countQuery := runArtifactRelationshipCTE + `
			SELECT COUNT(*) FROM selected_artifacts selected
			JOIN artifacts a ON a.id=selected.artifact_id` + where
		if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
			return nil, fmt.Errorf("count run artifact evidence: %w", err)
		}
		page = clampPage(filter.Page, filter.PerPage, total)
	}
	args := append(runArtifactRunArgs(filter.RunID, 8), filterArgs...)
	query := runArtifactRelationshipCTE + `
		SELECT a.id, a.content_hash, a.byte_size, a.content_type, a.created_at,
		       (ab.id IS NOT NULL) AS has_blob,
		       COALESCE((SELECT GROUP_CONCAT(role.artifact_role, ', ') FROM (
		           SELECT DISTINCT artifact_role FROM run_artifacts
		           WHERE pipeline_run_id=? AND artifact_id=a.id ORDER BY artifact_role
		       ) role), '') AS artifact_roles,
		       COALESCE((SELECT GROUP_CONCAT(relationship_role, ', ') FROM (
		           SELECT DISTINCT relationship_role FROM artifact_relationships
		           WHERE artifact_id=a.id ORDER BY relationship_role
		       )), '') AS relationship_roles,
		       COALESCE((SELECT GROUP_CONCAT(step_name, ', ') FROM (
		           SELECT DISTINCT step_name FROM run_steps
                   WHERE pipeline_run_id=? AND output_artifact_id=a.id
                   ORDER BY step_name
               )), '') AS produced_by_steps,
               COALESCE((SELECT GROUP_CONCAT(step_name, ', ') FROM (
                   SELECT DISTINCT step_name FROM run_steps
                   WHERE pipeline_run_id=? AND input_artifact_id=a.id
                   ORDER BY step_name
               )), '') AS consumed_by_steps
		FROM selected_artifacts selected
		JOIN artifacts a ON a.id=selected.artifact_id
		LEFT JOIN artifact_blobs ab ON ab.artifact_id=a.id` + where
	if !filter.PageMode {
		query += " AND a.id>?"
		args = append(args, filter.CursorID)
	}
	if filter.CursorID > 0 && filter.FocusID > 0 {
		query += " AND a.id!=?"
		args = append(args, filter.FocusID)
	}
	query += ` GROUP BY a.id, a.content_hash, a.byte_size, a.content_type, a.created_at, ab.id
		ORDER BY CASE WHEN a.id=? THEN 0 ELSE 1 END, a.id ` + filter.Order + ` LIMIT ?`
	args = append(args, filter.FocusID)
	if filter.PageMode {
		args = append(args, filter.PerPage, (page-1)*filter.PerPage)
		query += " OFFSET ?"
	} else {
		args = append(args, filter.Limit+1)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list run artifact evidence: %w", err)
	}
	defer rows.Close()
	items := make([]*RunArtifactEvidence, 0)
	for rows.Next() {
		var item RunArtifactEvidence
		if err := rows.Scan(&item.ID, &item.ContentHash, &item.ByteSize, &item.ContentType, &item.CreatedAt, &item.HasBlob, &item.ArtifactRoles, &item.RelationshipRoles, &item.ProducedBySteps, &item.ConsumedBySteps); err != nil {
			return nil, fmt.Errorf("list run artifact evidence: %w", err)
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list run artifact evidence: %w", err)
	}
	result := &RunArtifactEvidencePage{Items: items, Total: total, Page: page}
	if filter.PageMode {
		result.HasMore = int64(page*filter.PerPage) < total
		return result, nil
	}
	result.HasMore = len(items) > filter.Limit
	if result.HasMore {
		items = items[:filter.Limit]
		result.Items = items
		result.NextCursorID = items[len(items)-1].ID
	}
	return result, nil
}

// clampPage maps an offset request past the end to the final populated page.
func clampPage(page, perPage int, total int64) int {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		return 1
	}
	if int64(page) > totalPages {
		return int(totalPages)
	}
	return page
}
