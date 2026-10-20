// note.go provides logical review notes, immutable note versions and
// tombstones, note heads, bounded note reads, and the note summary derivation
// used by list and backlink projections.
package review

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	generated "analysis/database/review/internal/sql"
	"analysis/manifest"
	"analysis/notes"
)

// CreateNote creates one logical note, immutable first version, head, links, and audit atomically.
func (s *Store) CreateNote(ctx context.Context, contextID, workRevisionID int64, body string) (*Note, error) {
	document := notes.Parse(body)
	if len(document.Errors) != 0 {
		return nil, &NoteSyntaxError{Errors: document.Errors}
	}
	if strings.TrimSpace(body) == "" {
		return nil, reviewValidation("active note body must not be blank")
	}
	var note *Note
	err := s.withTx(ctx, func(queries *generated.Queries) error {
		runID, workID, _, err := s.mutableWorkHead(ctx, queries, contextID, workRevisionID)
		if err != nil {
			return err
		}
		createdAt := timestamp()
		inserted, err := queries.InsertReviewNote(ctx, generated.InsertReviewNoteParams{WorkID: workID, CreatedAt: createdAt})
		if err != nil {
			return fmt.Errorf("insert review note: %w", err)
		}
		noteID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		versionResult, err := queries.InsertReviewNoteFirstVersion(ctx, generated.InsertReviewNoteFirstVersionParams{
			NoteID:             noteID,
			CreatedInContextID: contextID,
			Body:               sql.NullString{String: body, Valid: true},
			CreatedAt:          createdAt,
		})
		if err != nil {
			return fmt.Errorf("insert review note version: %w", err)
		}
		versionID, err := versionResult.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertNoteLinks(ctx, queries, versionID, document.Links); err != nil {
			return err
		}
		if err := queries.InsertReviewNoteHead(ctx, generated.InsertReviewNoteHeadParams{
			ReviewContextID: contextID,
			NoteID:          noteID,
			NoteVersionID:   versionID,
		}); err != nil {
			return fmt.Errorf("insert review note head: %w", err)
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, queries, runID, "review_note_version", strconv.FormatInt(versionID, 10), manifest.AuditReviewNoteCreated, metadata); err != nil {
			return err
		}
		version, err := s.getNoteVersion(ctx, queries, contextID, versionID)
		if err != nil {
			return err
		}
		note = &Note{ID: noteID, WorkID: workID, WorkRevisionID: workRevisionID, CreatedAt: createdAt, Version: *version}
		return nil
	})
	return note, err
}

// AppendNoteVersion appends an active edit or deletion tombstone with optimistic concurrency.
func (s *Store) AppendNoteVersion(ctx context.Context, contextID, noteID int64, expectedVersionID int64, state, body string) (*Note, bool, error) {
	if state != "active" && state != "deleted" {
		return nil, false, reviewValidation("note state must be active or deleted")
	}
	var document notes.Document
	if state == "active" {
		document = notes.Parse(body)
		if len(document.Errors) != 0 {
			return nil, false, &NoteSyntaxError{Errors: document.Errors}
		}
		if strings.TrimSpace(body) == "" {
			return nil, false, reviewValidation("active note body must not be blank")
		}
	} else if body != "" {
		return nil, false, reviewValidation("deleted note version must not contain a body")
	}
	var note *Note
	changed := false
	err := s.withTx(ctx, func(queries *generated.Queries) error {
		row, err := queries.GetReviewNoteHeadForUpdate(ctx, generated.GetReviewNoteHeadForUpdateParams{
			ReviewContextID: contextID,
			NoteID:          noteID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return reviewNotFound("note does not belong to review context")
		}
		if err != nil {
			return err
		}
		runID, workID, workRevisionID, currentID := row.PipelineRunID, row.WorkID, row.WorkRevisionID, row.NoteVersionID
		createdAt := row.CreatedAt
		if row.Status != string(manifest.AttemptCompleted) || row.VisibilityState == string(manifest.RunTrashed) {
			return reviewLifecycle("review context run is read-only")
		}
		if currentID != expectedVersionID {
			current := currentID
			expected := expectedVersionID
			return &ConflictError{Expected: &expected, Current: &current}
		}
		current, err := s.getNoteVersion(ctx, queries, contextID, currentID)
		if err != nil {
			return err
		}
		if current.State == state && ((state == "deleted") || (current.Body != nil && *current.Body == body)) {
			note = &Note{ID: noteID, WorkID: workID, WorkRevisionID: workRevisionID, CreatedAt: createdAt, Version: *current}
			if current.CreatedInContextID != contextID {
				value := current.CreatedInContextID
				note.InheritedFromContextID = &value
			}
			return nil
		}
		createdVersionAt := timestamp()
		var bodyValue sql.NullString
		if state == "active" {
			bodyValue = sql.NullString{String: body, Valid: true}
		}
		inserted, err := queries.InsertReviewNoteVersion(ctx, generated.InsertReviewNoteVersionParams{
			NoteID:             noteID,
			ParentVersionID:    sql.NullInt64{Int64: currentID, Valid: true},
			CreatedInContextID: contextID,
			State:              state,
			Body:               bodyValue,
			CreatedAt:          createdVersionAt,
		})
		if err != nil {
			return fmt.Errorf("insert review note version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		if state == "active" {
			if err := insertNoteLinks(ctx, queries, versionID, document.Links); err != nil {
				return err
			}
		}
		updated, err := queries.UpdateReviewNoteHeadCAS(ctx, generated.UpdateReviewNoteHeadCASParams{
			NoteVersionID:     versionID,
			ReviewContextID:   contextID,
			NoteID:            noteID,
			ExpectedVersionID: currentID,
		})
		if err != nil {
			return err
		}
		if count, _ := updated.RowsAffected(); count != 1 {
			current := currentID
			expected := expectedVersionID
			return &ConflictError{Expected: &expected, Current: &current}
		}
		action := manifest.AuditReviewNoteVersionCreated
		if state == "deleted" {
			action = manifest.AuditReviewNoteTombstoned
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "parent_version_id": currentID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, queries, runID, "review_note_version", strconv.FormatInt(versionID, 10), action, metadata); err != nil {
			return err
		}
		version, err := s.getNoteVersion(ctx, queries, contextID, versionID)
		if err != nil {
			return err
		}
		note = &Note{ID: noteID, WorkID: workID, WorkRevisionID: workRevisionID, CreatedAt: createdAt, Version: *version}
		changed = true
		return nil
	})
	return note, changed, err
}

// GetNote returns an explicitly addressed current note head, including tombstones.
func (s *Store) GetNote(ctx context.Context, contextID, noteID int64) (*Note, error) {
	return s.getNote(ctx, s.queries, contextID, noteID)
}

// getNote reads one selected context head and resolves inherited attribution.
func (s *Store) getNote(ctx context.Context, queries *generated.Queries, contextID, noteID int64) (*Note, error) {
	row, err := queries.GetReviewNoteHead(ctx, generated.GetReviewNoteHeadParams{
		ReviewContextID: contextID,
		NoteID:          noteID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := s.getNoteVersion(ctx, queries, contextID, row.NoteVersionID)
	if err != nil {
		return nil, err
	}
	note := Note{ID: row.ID, WorkID: row.WorkID, WorkRevisionID: row.WorkRevisionID, CreatedAt: row.CreatedAt, Version: *version}
	if version.CreatedInContextID != contextID {
		value := version.CreatedInContextID
		note.InheritedFromContextID = &value
	}
	return &note, nil
}

// ListNotes returns bounded current note heads for one context work, excluding tombstones unless requested.
func (s *Store) ListNotes(ctx context.Context, contextID, workRevisionID, cursor int64, limit int, includeDeleted bool) ([]Note, error) {
	state := "active"
	if includeDeleted {
		state = "all"
	}
	return s.ListNotesFiltered(ctx, contextID, &workRevisionID, cursor, limit, state, "")
}

// ListNotesFiltered returns bounded current note-head summaries for one work or the complete run context.
func (s *Store) ListNotesFiltered(ctx context.Context, contextID int64, workRevisionID *int64, cursor int64, limit int, state, query string) ([]Note, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("note fetch limit must be between 1 and 101")
	}
	switch state {
	case "active", "removed", "all":
	default:
		return nil, reviewValidation("note state must be active, removed, or all")
	}
	params := generated.ListReviewNoteHeadsParams{
		PreviewBytes:    reviewTextPreviewBytes,
		ReviewContextID: contextID,
		Cursor:          cursor,
		StateFilter:     state,
		PageLimit:       int64(limit),
	}
	if workRevisionID != nil {
		params.HasWorkRevision = 1
		params.WorkRevisionID = *workRevisionID
	}
	if query = strings.TrimSpace(query); query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		params.HasQuery = 1
		params.QueryPattern = sql.NullString{String: "%" + escaped + "%", Valid: true}
	}
	rows, err := s.queries.ListReviewNoteHeads(ctx, params)
	if err != nil {
		return nil, err
	}
	projections := make([]noteSummaryProjection, 0, len(rows))
	for _, row := range rows {
		projections = append(projections, noteSummaryFromHeadRow(row))
	}
	return noteSummaries(projections, contextID), nil
}

// ListNoteVersions follows only the selected context note head's ancestors.
func (s *Store) ListNoteVersions(ctx context.Context, contextID, noteID, cursor int64, limit int) ([]NoteVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("note version fetch limit must be between 1 and 101")
	}
	rows, err := s.queries.ListReviewNoteVersionAncestry(ctx, generated.ListReviewNoteVersionAncestryParams{
		PreviewBytes:    reviewTextPreviewBytes,
		Cursor:          cursor,
		PageLimit:       int64(limit),
		ReviewContextID: contextID,
		NoteID:          noteID,
	})
	if err != nil {
		return nil, err
	}
	versions := make([]NoteVersion, 0, len(rows))
	for _, row := range rows {
		item := NoteVersion{
			ID:                 row.ID,
			NoteID:             row.NoteID,
			ParentVersionID:    nullInt64Pointer(row.ParentVersionID),
			CreatedInContextID: row.CreatedInContextID,
			State:              row.State,
			CreatedAt:          row.CreatedAt,
			ReviewerDisplay:    reviewerDisplay(row.ReviewerUsername, row.ReviewerEmail),
		}
		item.Body, item.BodyTruncated = boundedPreview(row.BodyPreview, row.BodyBytes, row.HasBody != 0).optional()
		item.BodyBytes = int(row.BodyBytes)
		if item.Body != nil {
			item.Title, item.Excerpt = noteSummary(*item.Body)
		}
		item.Links = []Link{}
		versions = append(versions, item)
	}
	return versions, nil
}

// GetNoteVersion returns one full body and link set only when it belongs to the selected head ancestry.
func (s *Store) GetNoteVersion(ctx context.Context, contextID, noteID, versionID int64) (*NoteVersion, error) {
	exists, err := s.queries.ReviewNoteVersionInAncestry(ctx, generated.ReviewNoteVersionInAncestryParams{
		VersionID:       versionID,
		ReviewContextID: contextID,
		NoteID:          noteID,
	})
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return s.getNoteVersion(ctx, s.queries, contextID, versionID)
}

// getNoteVersion reads one immutable note version and resolves its version-scoped links.
func (s *Store) getNoteVersion(ctx context.Context, queries *generated.Queries, contextID, versionID int64) (*NoteVersion, error) {
	row, err := queries.GetReviewNoteVersionByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	item := NoteVersion{
		ID:                 row.ID,
		NoteID:             row.NoteID,
		ParentVersionID:    nullInt64Pointer(row.ParentVersionID),
		CreatedInContextID: row.CreatedInContextID,
		State:              row.State,
		CreatedAt:          row.CreatedAt,
		ReviewerDisplay:    reviewerDisplay(row.ReviewerUsername, row.ReviewerEmail),
	}
	if row.Body.Valid {
		item.Body = &row.Body.String
		item.BodyBytes = len([]byte(row.Body.String))
		item.Title, item.Excerpt = noteSummary(row.Body.String)
	}
	links, err := s.linksForVersion(ctx, queries, contextID, item.NoteID, versionID)
	if err != nil {
		return nil, err
	}
	item.Links = links
	item.LinkCount = len(links)
	return &item, nil
}

// insertNoteLinks stores parser output against the exact immutable note version.
func insertNoteLinks(ctx context.Context, queries *generated.Queries, versionID int64, links []notes.Link) error {
	for index, link := range links {
		if err := queries.InsertReviewNoteLink(ctx, generated.InsertReviewNoteLinkParams{
			NoteVersionID: versionID,
			Ordinal:       int64(index + 1),
			TargetType:    link.TargetType,
			RawTarget:     link.RawTarget,
			DisplayText:   optionalString(link.DisplayText),
			Utf16Position: int64(link.Position),
			Utf16Length:   int64(link.Length),
			CreatedAt:     timestamp(),
		}); err != nil {
			return fmt.Errorf("insert review note link: %w", err)
		}
	}
	return nil
}

// noteSummaryProjection is the shared bounded note-head projection for note list and backlink reads.
type noteSummaryProjection struct {
	ID                 int64
	WorkID             int64
	WorkRevisionID     int64
	CreatedAt          string
	VersionID          int64
	NoteID             int64
	ParentVersionID    sql.NullInt64
	CreatedInContextID int64
	State              string
	BodyPreview        string
	BodyBytes          int64
	HasBody            int64
	VersionCreatedAt   string
	ReviewerUsername   string
	ReviewerEmail      string
	LinkCount          int64
}

// noteSummaryFromHeadRow maps one generated note-head list row.
func noteSummaryFromHeadRow(row generated.ListReviewNoteHeadsRow) noteSummaryProjection {
	return noteSummaryProjection{
		ID:                 row.ID,
		WorkID:             row.WorkID,
		WorkRevisionID:     row.WorkRevisionID,
		CreatedAt:          row.CreatedAt,
		VersionID:          row.VersionID,
		NoteID:             row.NoteID,
		ParentVersionID:    row.ParentVersionID,
		CreatedInContextID: row.CreatedInContextID,
		State:              row.State,
		BodyPreview:        row.BodyPreview,
		BodyBytes:          row.BodyBytes,
		HasBody:            row.HasBody,
		VersionCreatedAt:   row.VersionCreatedAt,
		ReviewerUsername:   row.ReviewerUsername,
		ReviewerEmail:      row.ReviewerEmail,
		LinkCount:          row.LinkCount,
	}
}

// noteSummaryFromBacklinkRow maps one generated backlink list row.
func noteSummaryFromBacklinkRow(row generated.ListReviewBacklinksRow) noteSummaryProjection {
	return noteSummaryProjection{
		ID:                 row.ID,
		WorkID:             row.WorkID,
		WorkRevisionID:     row.WorkRevisionID,
		CreatedAt:          row.CreatedAt,
		VersionID:          row.VersionID,
		NoteID:             row.NoteID,
		ParentVersionID:    row.ParentVersionID,
		CreatedInContextID: row.CreatedInContextID,
		State:              row.State,
		BodyPreview:        row.BodyPreview,
		BodyBytes:          row.BodyBytes,
		HasBody:            row.HasBody,
		VersionCreatedAt:   row.VersionCreatedAt,
		ReviewerUsername:   row.ReviewerUsername,
		ReviewerEmail:      row.ReviewerEmail,
		LinkCount:          row.LinkCount,
	}
}

// noteSummaries builds bounded list projections without loading full bodies or resolving every link.
func noteSummaries(projections []noteSummaryProjection, contextID int64) []Note {
	items := make([]Note, 0, len(projections))
	for _, projection := range projections {
		version := NoteVersion{
			ID:                 projection.VersionID,
			NoteID:             projection.NoteID,
			ParentVersionID:    nullInt64Pointer(projection.ParentVersionID),
			CreatedInContextID: projection.CreatedInContextID,
			State:              projection.State,
			BodyBytes:          int(projection.BodyBytes),
			LinkCount:          int(projection.LinkCount),
			CreatedAt:          projection.VersionCreatedAt,
			ReviewerDisplay:    reviewerDisplay(projection.ReviewerUsername, projection.ReviewerEmail),
		}
		version.Body, version.BodyTruncated = boundedPreview(projection.BodyPreview, projection.BodyBytes, projection.HasBody != 0).optional()
		if version.Body != nil {
			version.Title, version.Excerpt = noteSummary(*version.Body)
		}
		version.Links = []Link{}
		item := Note{ID: projection.ID, WorkID: projection.WorkID, WorkRevisionID: projection.WorkRevisionID, CreatedAt: projection.CreatedAt, Version: version}
		if projection.CreatedInContextID != contextID {
			value := projection.CreatedInContextID
			item.InheritedFromContextID = &value
		}
		items = append(items, item)
	}
	return items
}

// noteSummary derives a safe title and excerpt from one stored body or bounded prefix.
func noteSummary(body string) (string, string) {
	lines := strings.Split(body, "\n")
	title := "Untitled note"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		trimmed := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if trimmed != "" {
			title = trimmed
			break
		}
	}
	return truncateRunes(title, 80), truncateRunes(strings.Join(strings.Fields(body), " "), 180)
}

// truncateRunes returns a Unicode-safe bounded label with an explicit truncation marker.
func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
