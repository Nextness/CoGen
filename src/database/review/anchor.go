// anchor.go provides stable PDF anchors, immutable anchor versions and
// tombstones, anchor heads, geometry validation, and the bounded anchor reads
// used by the viewer and review mutations.
package review

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"analysis/manifest"
	"analysis/notes"
)

// CreateAnchor creates one generated logical anchor with a work-scoped label and immutable first version atomically.
func (s *Store) CreateAnchor(ctx context.Context, contextID, workRevisionID int64, label, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) (*Anchor, error) {
	if !notes.ValidAnchorID(label) {
		return nil, reviewValidation("anchor label has an invalid format")
	}
	anchorID, err := newAnchorID()
	if err != nil {
		return nil, fmt.Errorf("generate review anchor identity: %w", err)
	}
	if err := validateAnchorVersion(anchorID, contentHash, "active", page, selectedText, rectangles); err != nil {
		return nil, err
	}
	var anchor *Anchor
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		runID, workID, _, err := s.mutableWorkHead(ctx, tx, contextID, workRevisionID)
		if err != nil {
			return err
		}
		var labelExists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM review_anchors WHERE work_id=? AND label=?)`, workID, label).Scan(&labelExists); err != nil {
			return err
		}
		if labelExists {
			return &AnchorLabelConflictError{Label: label}
		}
		createdAt := timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_anchors (id, work_id, label, created_at) VALUES (?, ?, ?, ?)`, anchorID, workID, label, createdAt); err != nil {
			return fmt.Errorf("insert review anchor: %w", err)
		}
		rectanglesJSON, _ := json.Marshal(rectangles)
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_anchor_versions
			(anchor_id, created_in_context_id, work_revision_id, pdf_content_hash, state, page, selected_text, rectangles_json, created_at)
			VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?)`, anchorID, contextID, workRevisionID, contentHash, page, selectedText, string(rectanglesJSON), createdAt)
		if err != nil {
			return fmt.Errorf("insert review anchor version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_context_anchor_heads
			(review_context_id, anchor_id, anchor_version_id) VALUES (?, ?, ?)`, contextID, anchorID, versionID); err != nil {
			return fmt.Errorf("insert review anchor head: %w", err)
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_anchor_version", strconv.FormatInt(versionID, 10), manifest.AuditReviewAnchorCreated, metadata); err != nil {
			return err
		}
		version, err := s.getAnchorVersion(ctx, tx, versionID)
		if err != nil {
			return err
		}
		anchor = &Anchor{ID: anchorID, Label: label, WorkID: workID, CreatedAt: createdAt, Version: *version}
		return nil
	})
	return anchor, err
}

// AppendAnchorVersion appends an active replacement or tombstone using optimistic concurrency.
func (s *Store) AppendAnchorVersion(ctx context.Context, contextID int64, anchorID string, expectedVersionID int64, state, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) (*Anchor, bool, error) {
	if state != "active" && state != "deleted" {
		return nil, false, reviewValidation("anchor state must be active or deleted")
	}
	if err := validateAnchorVersion(anchorID, contentHash, state, page, selectedText, rectangles); err != nil {
		return nil, false, err
	}
	var anchor *Anchor
	changed := false
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var runID, workID, workRevisionID, currentID int64
		var label, createdAt, runStatus, visibility string
		err := tx.QueryRowContext(ctx, `SELECT rc.pipeline_run_id, logical.work_id, work_head.work_revision_id,
			head.anchor_version_id, COALESCE(logical.label, logical.id), logical.created_at, pr.status, pr.visibility_state
			FROM review_context_anchor_heads head
			JOIN review_contexts rc ON rc.id=head.review_context_id
			JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
			JOIN review_anchors logical ON logical.id=head.anchor_id
			JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
			WHERE head.review_context_id=? AND head.anchor_id=?`, contextID, anchorID).
			Scan(&runID, &workID, &workRevisionID, &currentID, &label, &createdAt, &runStatus, &visibility)
		if err == sql.ErrNoRows {
			return reviewNotFound("anchor does not belong to review context")
		}
		if err != nil {
			return err
		}
		if runStatus != string(manifest.AttemptCompleted) || visibility == string(manifest.RunTrashed) {
			return reviewLifecycle("review context run is read-only")
		}
		if currentID != expectedVersionID {
			current, expected := currentID, expectedVersionID
			return &ConflictError{Expected: &expected, Current: &current}
		}
		current, err := s.getAnchorVersion(ctx, tx, currentID)
		if err != nil {
			return err
		}
		if anchorsEqual(current, state, contentHash, page, selectedText, rectangles) {
			anchor = &Anchor{ID: anchorID, Label: label, WorkID: workID, CreatedAt: createdAt, Version: *current}
			if current.CreatedInContextID != contextID {
				value := current.CreatedInContextID
				anchor.InheritedFromContextID = &value
			}
			return nil
		}
		createdVersionAt := timestamp()
		var pageValue, textValue, rectanglesValue any
		if state == "active" {
			encoded, _ := json.Marshal(rectangles)
			pageValue, textValue, rectanglesValue = page, selectedText, string(encoded)
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_anchor_versions
			(anchor_id, parent_version_id, created_in_context_id, work_revision_id, pdf_content_hash, state, page, selected_text, rectangles_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, anchorID, currentID, contextID, workRevisionID, contentHash,
			state, pageValue, textValue, rectanglesValue, createdVersionAt)
		if err != nil {
			return fmt.Errorf("insert review anchor version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		updated, err := tx.ExecContext(ctx, `UPDATE review_context_anchor_heads SET anchor_version_id=?
			WHERE review_context_id=? AND anchor_id=? AND anchor_version_id=?`, versionID, contextID, anchorID, currentID)
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			currentValue, expected := currentID, expectedVersionID
			return &ConflictError{Expected: &expected, Current: &currentValue}
		}
		action := manifest.AuditReviewAnchorVersionCreated
		if state == "deleted" {
			action = manifest.AuditReviewAnchorTombstoned
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": currentID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_anchor_version", strconv.FormatInt(versionID, 10), action, metadata); err != nil {
			return err
		}
		version, err := s.getAnchorVersion(ctx, tx, versionID)
		if err != nil {
			return err
		}
		anchor = &Anchor{ID: anchorID, Label: label, WorkID: workID, CreatedAt: createdAt, Version: *version}
		changed = true
		return nil
	})
	return anchor, changed, err
}

// GetAnchor returns one selected-context logical anchor and its current head.
func (s *Store) GetAnchor(ctx context.Context, contextID int64, anchorID string) (*Anchor, error) {
	return s.getAnchor(ctx, contextID, anchorID)
}

// ListAnchors returns bounded active current anchors for one context work.
func (s *Store) ListAnchors(ctx context.Context, contextID, workRevisionID int64, cursor string, limit int) ([]Anchor, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("anchor fetch limit must be between 1 and 101")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT logical.id, COALESCE(logical.label, logical.id), logical.work_id, logical.created_at,
		version.id, version.anchor_id, version.parent_version_id, version.created_in_context_id,
		version.work_revision_id, version.pdf_content_hash, version.state, version.page,
		substr(version.selected_text, 1, `+strconv.Itoa(anchorTextPreviewBytes)+`), COALESCE(length(CAST(version.selected_text AS BLOB)), 0),
		version.rectangles_json, version.created_at, reviewer.username, reviewer.email
		FROM review_context_anchor_heads head
		JOIN review_anchors logical ON logical.id=head.anchor_id
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
		JOIN review_anchor_versions version ON version.id=head.anchor_version_id AND version.state='active'
		JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
		WHERE head.review_context_id=? AND work_head.work_revision_id=? AND (?='' OR head.anchor_id>?)
		ORDER BY head.anchor_id LIMIT ?`, contextID, workRevisionID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	items := make([]Anchor, 0)
	for rows.Next() {
		item, err := scanReviewAnchor(rows, contextID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return items, nil
}

// getAnchor reads one selected context anchor head with inherited attribution.
func (s *Store) getAnchor(ctx context.Context, contextID int64, anchorID string) (*Anchor, error) {
	var item Anchor
	var versionID int64
	err := s.db.QueryRowContext(ctx, `SELECT logical.id, COALESCE(logical.label, logical.id), logical.work_id, logical.created_at, head.anchor_version_id
		FROM review_context_anchor_heads head JOIN review_anchors logical ON logical.id=head.anchor_id
		WHERE head.review_context_id=? AND head.anchor_id=?`, contextID, anchorID).
		Scan(&item.ID, &item.Label, &item.WorkID, &item.CreatedAt, &versionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := s.getAnchorVersion(ctx, s.db, versionID)
	if err != nil {
		return nil, err
	}
	item.Version = *version
	if version.CreatedInContextID != contextID {
		value := version.CreatedInContextID
		item.InheritedFromContextID = &value
	}
	return &item, nil
}

// ListAnchorVersions follows only the selected context anchor head's ancestors.
func (s *Store) ListAnchorVersions(ctx context.Context, contextID int64, anchorID string, cursor int64, limit int) ([]AnchorVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("anchor version fetch limit must be between 1 and 101")
	}
	rows, err := s.db.QueryContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT anchor_version_id FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_anchor_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	)
	SELECT version.id, version.anchor_id, version.parent_version_id, version.created_in_context_id,
		version.work_revision_id, version.pdf_content_hash, version.state, version.page,
		substr(version.selected_text, 1, `+strconv.Itoa(anchorTextPreviewBytes)+`), COALESCE(length(CAST(version.selected_text AS BLOB)), 0),
		version.rectangles_json, version.created_at, reviewer.username, reviewer.email
	FROM ancestry JOIN review_anchor_versions version ON version.id=ancestry.id
	JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
	LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
	WHERE (?=0 OR version.id<?) ORDER BY version.id DESC LIMIT ?`, contextID, anchorID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	items := make([]AnchorVersion, 0)
	for rows.Next() {
		item, err := scanReviewAnchorVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetAnchorVersion returns one full geometry version only when it belongs to the selected head ancestry.
func (s *Store) GetAnchorVersion(ctx context.Context, contextID int64, anchorID string, versionID int64) (*AnchorVersion, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT anchor_version_id FROM review_context_anchor_heads WHERE review_context_id=? AND anchor_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_anchor_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM ancestry WHERE id=?)`, contextID, anchorID, versionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return s.getAnchorVersion(ctx, s.db, versionID)
}

// scanReviewAnchor reads one bounded logical-anchor list projection.
func scanReviewAnchor(scanner interface{ Scan(...any) error }, contextID int64) (Anchor, error) {
	var item Anchor
	version, err := scanReviewAnchorVersionWithPrefix(scanner, &item)
	if err != nil {
		return item, err
	}
	item.Version = version
	if version.CreatedInContextID != contextID {
		value := version.CreatedInContextID
		item.InheritedFromContextID = &value
	}
	return item, nil
}

// scanReviewAnchorVersion reads one bounded immutable anchor-version projection.
func scanReviewAnchorVersion(scanner interface{ Scan(...any) error }) (AnchorVersion, error) {
	return scanReviewAnchorVersionWithPrefix(scanner, nil)
}

// scanReviewAnchorVersionWithPrefix shares decoding for logical-head and history projections.
func scanReviewAnchorVersionWithPrefix(scanner interface{ Scan(...any) error }, anchor *Anchor) (AnchorVersion, error) {
	var item AnchorVersion
	var parent, page sql.NullInt64
	var selectedText boundedText
	var rectanglesJSON, username, email sql.NullString
	targets := make([]any, 0, 18)
	if anchor != nil {
		targets = append(targets, &anchor.ID, &anchor.Label, &anchor.WorkID, &anchor.CreatedAt)
	}
	targets = append(targets, &item.ID, &item.AnchorID, &parent, &item.CreatedInContextID,
		&item.WorkRevisionID, &item.PDFContentHash, &item.State, &page, &selectedText.text,
		&selectedText.bytes, &rectanglesJSON, &item.CreatedAt, &username, &email)
	if err := scanner.Scan(targets...); err != nil {
		return item, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if page.Valid {
		value := int(page.Int64)
		item.Page = &value
	}
	item.SelectedText, item.SelectedTextTruncated = selectedText.optional()
	if rectanglesJSON.Valid {
		if err := json.Unmarshal([]byte(rectanglesJSON.String), &item.Rectangles); err != nil {
			return item, fmt.Errorf("decode anchor rectangles: %w", err)
		}
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	return item, nil
}

// getAnchorVersion reads one immutable geometry snapshot or tombstone.
func (s *Store) getAnchorVersion(ctx context.Context, q queryRower, versionID int64) (*AnchorVersion, error) {
	var item AnchorVersion
	var parent sql.NullInt64
	var page sql.NullInt64
	var selectedText, rectanglesJSON, username, email sql.NullString
	err := q.QueryRowContext(ctx, `SELECT version.id, version.anchor_id, version.parent_version_id,
		version.created_in_context_id, version.work_revision_id, version.pdf_content_hash, version.state,
		version.page, version.selected_text, version.rectangles_json, version.created_at,
		reviewer.username, reviewer.email
		FROM review_anchor_versions version
		JOIN review_contexts context ON context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=context.pipeline_run_id
		WHERE version.id=?`, versionID).Scan(&item.ID, &item.AnchorID, &parent, &item.CreatedInContextID,
		&item.WorkRevisionID, &item.PDFContentHash, &item.State, &page, &selectedText, &rectanglesJSON,
		&item.CreatedAt, &username, &email)
	if err != nil {
		return nil, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if page.Valid {
		value := int(page.Int64)
		item.Page = &value
	}
	if selectedText.Valid {
		item.SelectedText = &selectedText.String
	}
	if rectanglesJSON.Valid {
		if err := json.Unmarshal([]byte(rectanglesJSON.String), &item.Rectangles); err != nil {
			return nil, fmt.Errorf("decode anchor rectangles: %w", err)
		}
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	return &item, nil
}

// validateAnchorVersion enforces safe identity, PDF binding, state, and normalized geometry.
func validateAnchorVersion(anchorID, contentHash, state string, page int, selectedText string, rectangles []AnchorRectangle) error {
	if !notes.ValidAnchorID(anchorID) {
		return reviewValidation("anchor ID has an invalid format")
	}
	decoded, err := hex.DecodeString(contentHash)
	if err != nil || len(decoded) != 32 {
		return reviewValidation("PDF content hash must be a lowercase SHA-256 value")
	}
	if contentHash != strings.ToLower(contentHash) {
		return reviewValidation("PDF content hash must be lowercase")
	}
	if state == "deleted" {
		if page != 0 || selectedText != "" || len(rectangles) != 0 {
			return reviewValidation("deleted anchor version must not contain replacement geometry")
		}
		return nil
	}
	if page < 1 {
		return reviewValidation("anchor page must be positive")
	}
	if strings.TrimSpace(selectedText) == "" {
		return reviewValidation("anchor selected text must not be blank")
	}
	if len(selectedText) > 16384 {
		return reviewValidation("anchor selected text exceeds 16384 bytes")
	}
	if len(rectangles) < 1 || len(rectangles) > 64 {
		return reviewValidation("anchor requires between 1 and 64 rectangles")
	}
	for _, rectangle := range rectangles {
		values := []float64{rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
				return reviewValidation("anchor rectangle coordinates must be finite and normalized")
			}
		}
		if rectangle.Width <= 0 || rectangle.Height <= 0 || rectangle.X+rectangle.Width > 1 || rectangle.Y+rectangle.Height > 1 {
			return reviewValidation("anchor rectangle dimensions must be positive and remain within the page")
		}
	}
	return nil
}

// newAnchorID returns an opaque global identifier compatible with the note-language anchor grammar.
func newAnchorID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "a" + hex.EncodeToString(random[:]), nil
}

// anchorsEqual detects an identical save so the repository can avoid redundant history.
func anchorsEqual(current *AnchorVersion, state, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) bool {
	if current.State != state || current.PDFContentHash != contentHash {
		return false
	}
	if state == "deleted" {
		return true
	}
	if current.Page == nil || *current.Page != page || current.SelectedText == nil || *current.SelectedText != selectedText || len(current.Rectangles) != len(rectangles) {
		return false
	}
	for index := range rectangles {
		if current.Rectangles[index] != rectangles[index] {
			return false
		}
	}
	return true
}
