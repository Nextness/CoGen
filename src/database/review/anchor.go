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
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	generated "analysis/database/review/internal/sql"
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
	err = s.withTx(ctx, func(queries *generated.Queries) error {
		runID, workID, _, err := s.mutableWorkHead(ctx, queries, contextID, workRevisionID)
		if err != nil {
			return err
		}
		labelExists, err := queries.ReviewAnchorLabelExists(ctx, generated.ReviewAnchorLabelExistsParams{
			WorkID: workID,
			Label:  sql.NullString{String: label, Valid: true},
		})
		if err != nil {
			return err
		}
		if labelExists {
			return &AnchorLabelConflictError{Label: label}
		}
		createdAt := timestamp()
		if err := queries.InsertReviewAnchor(ctx, generated.InsertReviewAnchorParams{
			ID:        anchorID,
			WorkID:    workID,
			Label:     sql.NullString{String: label, Valid: true},
			CreatedAt: createdAt,
		}); err != nil {
			return fmt.Errorf("insert review anchor: %w", err)
		}
		rectanglesJSON, _ := json.Marshal(rectangles)
		inserted, err := queries.InsertReviewAnchorFirstVersion(ctx, generated.InsertReviewAnchorFirstVersionParams{
			AnchorID:           anchorID,
			CreatedInContextID: contextID,
			WorkRevisionID:     workRevisionID,
			PdfContentHash:     contentHash,
			Page:               sql.NullInt64{Int64: int64(page), Valid: true},
			SelectedText:       sql.NullString{String: selectedText, Valid: true},
			RectanglesJson:     sql.NullString{String: string(rectanglesJSON), Valid: true},
			CreatedAt:          createdAt,
		})
		if err != nil {
			return fmt.Errorf("insert review anchor version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		if err := queries.InsertReviewAnchorHead(ctx, generated.InsertReviewAnchorHeadParams{
			ReviewContextID: contextID,
			AnchorID:        anchorID,
			AnchorVersionID: versionID,
		}); err != nil {
			return fmt.Errorf("insert review anchor head: %w", err)
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, queries, runID, "review_anchor_version", strconv.FormatInt(versionID, 10), manifest.AuditReviewAnchorCreated, metadata); err != nil {
			return err
		}
		version, err := s.getAnchorVersion(ctx, queries, versionID)
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
	err := s.withTx(ctx, func(queries *generated.Queries) error {
		row, err := queries.GetReviewAnchorHeadForUpdate(ctx, generated.GetReviewAnchorHeadForUpdateParams{
			ReviewContextID: contextID,
			AnchorID:        anchorID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return reviewNotFound("anchor does not belong to review context")
		}
		if err != nil {
			return err
		}
		runID, workID, workRevisionID, currentID := row.PipelineRunID, row.WorkID, row.WorkRevisionID, row.AnchorVersionID
		label, createdAt := row.Label, row.CreatedAt
		if row.Status != string(manifest.AttemptCompleted) || row.VisibilityState == string(manifest.RunTrashed) {
			return reviewLifecycle("review context run is read-only")
		}
		if currentID != expectedVersionID {
			current, expected := currentID, expectedVersionID
			return &ConflictError{Expected: &expected, Current: &current}
		}
		current, err := s.getAnchorVersion(ctx, queries, currentID)
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
		var pageValue sql.NullInt64
		var textValue, rectanglesValue sql.NullString
		if state == "active" {
			encoded, _ := json.Marshal(rectangles)
			pageValue = sql.NullInt64{Int64: int64(page), Valid: true}
			textValue = sql.NullString{String: selectedText, Valid: true}
			rectanglesValue = sql.NullString{String: string(encoded), Valid: true}
		}
		inserted, err := queries.InsertReviewAnchorVersion(ctx, generated.InsertReviewAnchorVersionParams{
			AnchorID:           anchorID,
			ParentVersionID:    sql.NullInt64{Int64: currentID, Valid: true},
			CreatedInContextID: contextID,
			WorkRevisionID:     workRevisionID,
			PdfContentHash:     contentHash,
			State:              state,
			Page:               pageValue,
			SelectedText:       textValue,
			RectanglesJson:     rectanglesValue,
			CreatedAt:          createdVersionAt,
		})
		if err != nil {
			return fmt.Errorf("insert review anchor version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		updated, err := queries.UpdateReviewAnchorHeadCAS(ctx, generated.UpdateReviewAnchorHeadCASParams{
			AnchorVersionID:   versionID,
			ReviewContextID:   contextID,
			AnchorID:          anchorID,
			ExpectedVersionID: currentID,
		})
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
		if err := insertReviewAudit(ctx, queries, runID, "review_anchor_version", strconv.FormatInt(versionID, 10), action, metadata); err != nil {
			return err
		}
		version, err := s.getAnchorVersion(ctx, queries, versionID)
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
	return s.getAnchor(ctx, s.queries, contextID, anchorID)
}

// ListAnchors returns bounded active current anchors for one context work.
func (s *Store) ListAnchors(ctx context.Context, contextID, workRevisionID int64, cursor string, limit int) ([]Anchor, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("anchor fetch limit must be between 1 and 101")
	}
	rows, err := s.queries.ListReviewAnchors(ctx, generated.ListReviewAnchorsParams{
		PreviewBytes:    anchorTextPreviewBytes,
		ReviewContextID: contextID,
		WorkRevisionID:  workRevisionID,
		Cursor:          cursor,
		PageLimit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]Anchor, 0, len(rows))
	for _, row := range rows {
		version, err := anchorVersionFromProjection(anchorVersionProjectionFromListRow(row))
		if err != nil {
			return nil, err
		}
		item := Anchor{ID: row.ID, Label: row.Label, WorkID: row.WorkID, CreatedAt: row.CreatedAt, Version: version}
		if version.CreatedInContextID != contextID {
			value := version.CreatedInContextID
			item.InheritedFromContextID = &value
		}
		items = append(items, item)
	}
	return items, nil
}

// getAnchor reads one selected context anchor head with inherited attribution.
func (s *Store) getAnchor(ctx context.Context, queries *generated.Queries, contextID int64, anchorID string) (*Anchor, error) {
	row, err := queries.GetReviewAnchorHead(ctx, generated.GetReviewAnchorHeadParams{
		ReviewContextID: contextID,
		AnchorID:        anchorID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := s.getAnchorVersion(ctx, queries, row.AnchorVersionID)
	if err != nil {
		return nil, err
	}
	item := Anchor{ID: row.ID, Label: row.Label, WorkID: row.WorkID, CreatedAt: row.CreatedAt, Version: *version}
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
	rows, err := s.queries.ListReviewAnchorVersionAncestry(ctx, generated.ListReviewAnchorVersionAncestryParams{
		PreviewBytes:    anchorTextPreviewBytes,
		Cursor:          cursor,
		PageLimit:       int64(limit),
		ReviewContextID: contextID,
		AnchorID:        anchorID,
	})
	if err != nil {
		return nil, err
	}
	items := make([]AnchorVersion, 0, len(rows))
	for _, row := range rows {
		version, err := anchorVersionFromProjection(anchorVersionProjectionFromAncestryRow(row))
		if err != nil {
			return nil, err
		}
		items = append(items, version)
	}
	return items, nil
}

// GetAnchorVersion returns one full geometry version only when it belongs to the selected head ancestry.
func (s *Store) GetAnchorVersion(ctx context.Context, contextID int64, anchorID string, versionID int64) (*AnchorVersion, error) {
	exists, err := s.queries.ReviewAnchorVersionInAncestry(ctx, generated.ReviewAnchorVersionInAncestryParams{
		VersionID:       versionID,
		ReviewContextID: contextID,
		AnchorID:        anchorID,
	})
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return s.getAnchorVersion(ctx, s.queries, versionID)
}

// anchorVersionProjection is the shared bounded anchor-version projection for current-head and history reads.
type anchorVersionProjection struct {
	ID                  int64
	AnchorID            string
	ParentVersionID     sql.NullInt64
	CreatedInContextID  int64
	WorkRevisionID      int64
	PDFContentHash      string
	State               string
	Page                sql.NullInt64
	SelectedTextPreview string
	SelectedTextBytes   int64
	HasSelectedText     int64
	RectanglesJSON      sql.NullString
	CreatedAt           string
	ReviewerUsername    string
	ReviewerEmail       string
}

// anchorVersionProjectionFromListRow maps one generated current-anchor list row.
func anchorVersionProjectionFromListRow(row generated.ListReviewAnchorsRow) anchorVersionProjection {
	return anchorVersionProjection{
		ID:                  row.VersionID,
		AnchorID:            row.AnchorID,
		ParentVersionID:     row.ParentVersionID,
		CreatedInContextID:  row.CreatedInContextID,
		WorkRevisionID:      row.WorkRevisionID,
		PDFContentHash:      row.PdfContentHash,
		State:               row.State,
		Page:                row.Page,
		SelectedTextPreview: row.SelectedTextPreview,
		SelectedTextBytes:   row.SelectedTextBytes,
		HasSelectedText:     row.HasSelectedText,
		RectanglesJSON:      row.RectanglesJson,
		CreatedAt:           row.VersionCreatedAt,
		ReviewerUsername:    row.ReviewerUsername,
		ReviewerEmail:       row.ReviewerEmail,
	}
}

// anchorVersionProjectionFromAncestryRow maps one generated anchor-history row.
func anchorVersionProjectionFromAncestryRow(row generated.ListReviewAnchorVersionAncestryRow) anchorVersionProjection {
	return anchorVersionProjection{
		ID:                  row.ID,
		AnchorID:            row.AnchorID,
		ParentVersionID:     row.ParentVersionID,
		CreatedInContextID:  row.CreatedInContextID,
		WorkRevisionID:      row.WorkRevisionID,
		PDFContentHash:      row.PdfContentHash,
		State:               row.State,
		Page:                row.Page,
		SelectedTextPreview: row.SelectedTextPreview,
		SelectedTextBytes:   row.SelectedTextBytes,
		HasSelectedText:     row.HasSelectedText,
		RectanglesJSON:      row.RectanglesJson,
		CreatedAt:           row.CreatedAt,
		ReviewerUsername:    row.ReviewerUsername,
		ReviewerEmail:       row.ReviewerEmail,
	}
}

// anchorVersionFromProjection builds one bounded immutable anchor-version projection.
func anchorVersionFromProjection(projection anchorVersionProjection) (AnchorVersion, error) {
	item := AnchorVersion{
		ID:                 projection.ID,
		AnchorID:           projection.AnchorID,
		ParentVersionID:    nullInt64Pointer(projection.ParentVersionID),
		CreatedInContextID: projection.CreatedInContextID,
		WorkRevisionID:     projection.WorkRevisionID,
		PDFContentHash:     projection.PDFContentHash,
		State:              projection.State,
		CreatedAt:          projection.CreatedAt,
		ReviewerDisplay:    reviewerDisplay(projection.ReviewerUsername, projection.ReviewerEmail),
	}
	if projection.Page.Valid {
		value := int(projection.Page.Int64)
		item.Page = &value
	}
	item.SelectedText, item.SelectedTextTruncated = boundedPreview(projection.SelectedTextPreview, projection.SelectedTextBytes, projection.HasSelectedText != 0).optional()
	if projection.RectanglesJSON.Valid {
		if err := json.Unmarshal([]byte(projection.RectanglesJSON.String), &item.Rectangles); err != nil {
			return item, fmt.Errorf("decode anchor rectangles: %w", err)
		}
	}
	return item, nil
}

// getAnchorVersion reads one immutable geometry snapshot or tombstone.
func (s *Store) getAnchorVersion(ctx context.Context, queries *generated.Queries, versionID int64) (*AnchorVersion, error) {
	row, err := queries.GetReviewAnchorVersionByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	item := AnchorVersion{
		ID:                 row.ID,
		AnchorID:           row.AnchorID,
		ParentVersionID:    nullInt64Pointer(row.ParentVersionID),
		CreatedInContextID: row.CreatedInContextID,
		WorkRevisionID:     row.WorkRevisionID,
		PDFContentHash:     row.PdfContentHash,
		State:              row.State,
		CreatedAt:          row.CreatedAt,
		ReviewerDisplay:    reviewerDisplay(row.ReviewerUsername, row.ReviewerEmail),
	}
	if row.Page.Valid {
		value := int(row.Page.Int64)
		item.Page = &value
	}
	if row.SelectedText.Valid {
		item.SelectedText = &row.SelectedText.String
	}
	if row.RectanglesJson.Valid {
		if err := json.Unmarshal([]byte(row.RectanglesJson.String), &item.Rectangles); err != nil {
			return nil, fmt.Errorf("decode anchor rectangles: %w", err)
		}
	}
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
