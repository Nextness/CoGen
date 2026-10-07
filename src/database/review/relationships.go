// relationships.go provides link resolution and backlink navigation across
// note versions, work revisions, anchors, and external targets without
// rewriting persisted link identity.
package review

import (
	"context"
	"database/sql"
	"net/url"
	"strconv"
)

// linksForVersion reads links in source order and resolves them against the selected context.
func (s *Store) linksForVersion(ctx context.Context, q reviewQuerier, contextID, noteID, versionID int64) ([]Link, error) {
	rows, err := q.QueryContext(ctx, `SELECT ordinal, target_type, raw_target, display_text, utf16_position, utf16_length
		FROM review_note_links WHERE note_version_id=? ORDER BY ordinal`, versionID)
	if err != nil {
		return nil, err
	}
	links := make([]Link, 0)
	for rows.Next() {
		var link Link
		var display sql.NullString
		if err := rows.Scan(&link.Ordinal, &link.TargetType, &link.RawTarget, &display, &link.UTF16Position, &link.UTF16Length); err != nil {
			return nil, err
		}
		if display.Valid {
			link.DisplayText = &display.String
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range links {
		if err := s.resolveLink(ctx, q, contextID, noteID, &links[index]); err != nil {
			return nil, err
		}
	}
	return links, nil
}

// resolveLink enriches a syntactically valid link without rewriting persisted link identity.
func (s *Store) resolveLink(ctx context.Context, q queryRower, contextID, sourceNoteID int64, link *Link) error {
	switch link.TargetType {
	case "note":
		noteID, _ := strconv.ParseInt(link.RawTarget, 10, 64)
		var revisionID int64
		err := q.QueryRowContext(ctx, `SELECT work_head.work_revision_id
			FROM review_context_note_heads note_head
			JOIN review_notes logical ON logical.id=note_head.note_id
			JOIN review_context_work_heads work_head ON work_head.review_context_id=note_head.review_context_id AND work_head.work_id=logical.work_id
			WHERE note_head.review_context_id=? AND note_head.note_id=?`, contextID, noteID).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID, link.NoteID = true, &revisionID, &noteID
	case "article":
		var revisionID int64
		err := q.QueryRowContext(ctx, `SELECT head.work_revision_id FROM review_context_work_heads head
			JOIN works work ON work.id=head.work_id WHERE head.review_context_id=? AND work.doi=?`, contextID, link.RawTarget).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID = true, &revisionID
	case "pdf_page":
		page, _ := strconv.Atoi(link.RawTarget)
		var revisionID int64
		err := q.QueryRowContext(ctx, `SELECT work_head.work_revision_id
			FROM review_notes logical JOIN review_context_work_heads work_head
			 ON work_head.review_context_id=? AND work_head.work_id=logical.work_id
			WHERE logical.id=?`, contextID, sourceNoteID).Scan(&revisionID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		link.Resolved, link.WorkRevisionID, link.Page = true, &revisionID, &page
	case "anchor":
		var revisionID int64
		var page int
		err := q.QueryRowContext(ctx, `SELECT work_head.work_revision_id, version.page
			FROM review_context_anchor_heads anchor_head
			JOIN review_anchors logical ON logical.id=anchor_head.anchor_id
			JOIN review_anchor_versions version ON version.id=anchor_head.anchor_version_id AND version.state='active'
			JOIN review_context_work_heads work_head ON work_head.review_context_id=anchor_head.review_context_id AND work_head.work_id=logical.work_id
			WHERE anchor_head.review_context_id=? AND anchor_head.anchor_id=?`, contextID, link.RawTarget).Scan(&revisionID, &page)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		anchorID := link.RawTarget
		link.Resolved, link.WorkRevisionID, link.AnchorID, link.Page = true, &revisionID, &anchorID, &page
	case "ext":
		parsed, err := url.Parse(link.RawTarget)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			value := parsed.String()
			link.Resolved, link.URL = true, &value
		}
	}
	return nil
}

// ListBacklinks returns links from current note heads only.
func (s *Store) ListBacklinks(ctx context.Context, contextID int64, targetType, targetID string, sourceWorkID, cursor int64, limit int) ([]Note, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("backlink fetch limit must be between 1 and 101")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT logical.id, logical.work_id, work_head.work_revision_id, logical.created_at,
		version.id, version.note_id, version.parent_version_id, version.created_in_context_id, version.state,
		substr(version.body, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`), COALESCE(length(CAST(version.body AS BLOB)), 0), version.created_at,
		reviewer.username, reviewer.email,
		(SELECT COUNT(*) FROM review_note_links all_links WHERE all_links.note_version_id=version.id)
		FROM review_context_note_heads head
		JOIN review_note_versions version ON version.id=head.note_version_id AND version.state='active'
		JOIN review_notes logical ON logical.id=head.note_id
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
		JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
		JOIN review_note_links link ON link.note_version_id=head.note_version_id
		WHERE head.review_context_id=? AND link.target_type=? AND link.raw_target=?
		AND (?=0 OR logical.work_id=?)
		AND (?=0 OR head.note_id<?)
		GROUP BY logical.id, version.id
		ORDER BY head.note_id DESC LIMIT ?`, contextID, targetType, targetID, sourceWorkID, sourceWorkID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	return scanReviewNoteSummaries(rows, contextID)
}
