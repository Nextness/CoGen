// note.go provides logical review notes, immutable note versions and
// tombstones, note heads, bounded note reads, and the note summary derivation
// used by list and backlink projections.
package review

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

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
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		runID, workID, _, err := s.mutableWorkHead(ctx, tx, contextID, workRevisionID)
		if err != nil {
			return err
		}
		createdAt := timestamp()
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_notes (work_id, created_at) VALUES (?, ?)`, workID, createdAt)
		if err != nil {
			return fmt.Errorf("insert review note: %w", err)
		}
		noteID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		versionResult, err := tx.ExecContext(ctx, `INSERT INTO review_note_versions
			(note_id, created_in_context_id, state, body, created_at) VALUES (?, ?, 'active', ?, ?)`, noteID, contextID, body, createdAt)
		if err != nil {
			return fmt.Errorf("insert review note version: %w", err)
		}
		versionID, err := versionResult.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertNoteLinks(ctx, tx, versionID, document.Links); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_context_note_heads
			(review_context_id, note_id, note_version_id) VALUES (?, ?, ?)`, contextID, noteID, versionID); err != nil {
			return fmt.Errorf("insert review note head: %w", err)
		}
		metadata := map[string]any{"review_context_id": contextID, "work_id": workID, "work_revision_id": workRevisionID, "new_version_id": versionID}
		if err := insertReviewAudit(ctx, tx, runID, "review_note_version", strconv.FormatInt(versionID, 10), manifest.AuditReviewNoteCreated, metadata); err != nil {
			return err
		}
		version, err := s.getNoteVersion(ctx, tx, contextID, versionID)
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
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var runID, workID, workRevisionID, currentID int64
		var createdAt, runStatus, visibility string
		err := tx.QueryRowContext(ctx, `SELECT rc.pipeline_run_id, logical.work_id, work_head.work_revision_id,
			head.note_version_id, logical.created_at, pr.status, pr.visibility_state
			FROM review_context_note_heads head
			JOIN review_contexts rc ON rc.id=head.review_context_id
			JOIN pipeline_runs pr ON pr.id=rc.pipeline_run_id
			JOIN review_notes logical ON logical.id=head.note_id
			JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
			WHERE head.review_context_id=? AND head.note_id=?`, contextID, noteID).
			Scan(&runID, &workID, &workRevisionID, &currentID, &createdAt, &runStatus, &visibility)
		if err == sql.ErrNoRows {
			return reviewNotFound("note does not belong to review context")
		}
		if err != nil {
			return err
		}
		if runStatus != string(manifest.AttemptCompleted) || visibility == string(manifest.RunTrashed) {
			return reviewLifecycle("review context run is read-only")
		}
		if currentID != expectedVersionID {
			current := currentID
			expected := expectedVersionID
			return &ConflictError{Expected: &expected, Current: &current}
		}
		current, err := s.getNoteVersion(ctx, tx, contextID, currentID)
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
		var bodyValue any
		if state == "active" {
			bodyValue = body
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO review_note_versions
			(note_id, parent_version_id, created_in_context_id, state, body, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`, noteID, currentID, contextID, state, bodyValue, createdVersionAt)
		if err != nil {
			return fmt.Errorf("insert review note version: %w", err)
		}
		versionID, err := inserted.LastInsertId()
		if err != nil {
			return err
		}
		if state == "active" {
			if err := insertNoteLinks(ctx, tx, versionID, document.Links); err != nil {
				return err
			}
		}
		updated, err := tx.ExecContext(ctx, `UPDATE review_context_note_heads SET note_version_id=?
			WHERE review_context_id=? AND note_id=? AND note_version_id=?`, versionID, contextID, noteID, currentID)
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
		if err := insertReviewAudit(ctx, tx, runID, "review_note_version", strconv.FormatInt(versionID, 10), action, metadata); err != nil {
			return err
		}
		version, err := s.getNoteVersion(ctx, tx, contextID, versionID)
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
	return s.getNote(ctx, s.db, contextID, noteID)
}

// getNote reads one selected context head and resolves inherited attribution.
func (s *Store) getNote(ctx context.Context, q reviewQuerier, contextID, noteID int64) (*Note, error) {
	var note Note
	var versionID int64
	err := q.QueryRowContext(ctx, `SELECT logical.id, logical.work_id, work_head.work_revision_id,
		logical.created_at, head.note_version_id
		FROM review_context_note_heads head
		JOIN review_notes logical ON logical.id=head.note_id
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id AND work_head.work_id=logical.work_id
		WHERE head.review_context_id=? AND head.note_id=?`, contextID, noteID).
		Scan(&note.ID, &note.WorkID, &note.WorkRevisionID, &note.CreatedAt, &versionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := s.getNoteVersion(ctx, q, contextID, versionID)
	if err != nil {
		return nil, err
	}
	note.Version = *version
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
	clauses := []string{"head.review_context_id=?", "(?=0 OR head.note_id<?)"}
	args := []any{contextID, cursor, cursor}
	switch state {
	case "active":
		clauses = append(clauses, "version.state='active'")
	case "removed":
		clauses = append(clauses, "version.state='deleted'")
	case "all":
	default:
		return nil, reviewValidation("note state must be active, removed, or all")
	}
	if workRevisionID != nil {
		clauses = append(clauses, "work_head.work_revision_id=?")
		args = append(args, *workRevisionID)
	}
	if query = strings.TrimSpace(query); query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		clauses = append(clauses, `version.body LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escaped+"%")
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT logical.id, logical.work_id, work_head.work_revision_id, logical.created_at,
		version.id, version.note_id, version.parent_version_id, version.created_in_context_id, version.state,
		substr(version.body, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`), COALESCE(length(CAST(version.body AS BLOB)), 0), version.created_at,
		reviewer.username, reviewer.email,
		(SELECT COUNT(*) FROM review_note_links link WHERE link.note_version_id=version.id)
		FROM review_context_note_heads head
		JOIN review_context_work_heads work_head ON work_head.review_context_id=head.review_context_id
		JOIN review_notes logical ON logical.id=head.note_id AND logical.work_id=work_head.work_id
		JOIN review_note_versions version ON version.id=head.note_version_id
		JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY head.note_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	return scanReviewNoteSummaries(rows, contextID)
}

// ListNoteVersions follows only the selected context note head's ancestors.
func (s *Store) ListNoteVersions(ctx context.Context, contextID, noteID, cursor int64, limit int) ([]NoteVersion, error) {
	if limit < 1 || limit > reviewListLimit {
		return nil, reviewValidation("note version fetch limit must be between 1 and 101")
	}
	rows, err := s.db.QueryContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT note_version_id FROM review_context_note_heads WHERE review_context_id=? AND note_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_note_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	)
	SELECT version.id, version.note_id, version.parent_version_id, version.created_in_context_id, version.state,
		substr(version.body, 1, `+strconv.Itoa(reviewTextPreviewBytes)+`), COALESCE(length(CAST(version.body AS BLOB)), 0), version.created_at,
		reviewer.username, reviewer.email,
		(SELECT COUNT(*) FROM review_note_links link WHERE link.note_version_id=version.id)
	FROM ancestry JOIN review_note_versions version ON version.id=ancestry.id
	JOIN review_contexts version_context ON version_context.id=version.created_in_context_id
	LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=version_context.pipeline_run_id
	WHERE (?=0 OR version.id<?) ORDER BY version.id DESC LIMIT ?`, contextID, noteID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	versions := make([]NoteVersion, 0)
	for rows.Next() {
		var item NoteVersion
		var parent sql.NullInt64
		var body boundedText
		var username, email sql.NullString
		if err := rows.Scan(&item.ID, &item.NoteID, &parent, &item.CreatedInContextID, &item.State,
			&body.text, &body.bytes, &item.CreatedAt, &username, &email, &item.LinkCount); err != nil {
			return nil, err
		}
		item.ParentVersionID = nullInt64Pointer(parent)
		item.Body, item.BodyTruncated = body.optional()
		item.BodyBytes = body.bytes
		if item.Body != nil {
			item.Title, item.Excerpt = noteSummary(*item.Body)
		}
		item.Links = []Link{}
		item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
		versions = append(versions, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return versions, nil
}

// GetNoteVersion returns one full body and link set only when it belongs to the selected head ancestry.
func (s *Store) GetNoteVersion(ctx context.Context, contextID, noteID, versionID int64) (*NoteVersion, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `WITH RECURSIVE ancestry(id) AS (
		SELECT note_version_id FROM review_context_note_heads WHERE review_context_id=? AND note_id=?
		UNION ALL
		SELECT version.parent_version_id FROM review_note_versions version JOIN ancestry ON version.id=ancestry.id
		 WHERE version.parent_version_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM ancestry WHERE id=?)`, contextID, noteID, versionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return s.getNoteVersion(ctx, s.db, contextID, versionID)
}

// getNoteVersion reads one immutable note version and resolves its version-scoped links.
func (s *Store) getNoteVersion(ctx context.Context, q reviewQuerier, contextID, versionID int64) (*NoteVersion, error) {
	var item NoteVersion
	var parent sql.NullInt64
	var body, username, email sql.NullString
	err := q.QueryRowContext(ctx, `SELECT version.id, version.note_id, version.parent_version_id,
		version.created_in_context_id, version.state, version.body, version.created_at,
		reviewer.username, reviewer.email
		FROM review_note_versions version
		JOIN review_contexts context ON context.id=version.created_in_context_id
		LEFT JOIN pipeline_run_reviewers reviewer ON reviewer.pipeline_run_id=context.pipeline_run_id
		WHERE version.id=?`, versionID).Scan(&item.ID, &item.NoteID, &parent, &item.CreatedInContextID,
		&item.State, &body, &item.CreatedAt, &username, &email)
	if err != nil {
		return nil, err
	}
	item.ParentVersionID = nullInt64Pointer(parent)
	if body.Valid {
		item.Body = &body.String
		item.BodyBytes = len([]byte(body.String))
		item.Title, item.Excerpt = noteSummary(body.String)
	}
	item.ReviewerDisplay = reviewerDisplay(username.String, email.String)
	links, err := s.linksForVersion(ctx, q, contextID, item.NoteID, versionID)
	if err != nil {
		return nil, err
	}
	item.Links = links
	item.LinkCount = len(links)
	return &item, nil
}

// insertNoteLinks stores parser output against the exact immutable note version.
func insertNoteLinks(ctx context.Context, tx *sql.Tx, versionID int64, links []notes.Link) error {
	for index, link := range links {
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_note_links
			(note_version_id, ordinal, target_type, raw_target, display_text, utf16_position, utf16_length, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, versionID, index+1, link.TargetType, link.RawTarget,
			nullablePointer(link.DisplayText), link.Position, link.Length, timestamp()); err != nil {
			return fmt.Errorf("insert review note link: %w", err)
		}
	}
	return nil
}

// scanReviewNoteSummaries reads bounded list projections without loading full bodies or resolving every link.
func scanReviewNoteSummaries(rows *sql.Rows, contextID int64) ([]Note, error) {
	defer rows.Close()
	items := make([]Note, 0)
	for rows.Next() {
		var item Note
		var version NoteVersion
		var parent sql.NullInt64
		var body boundedText
		var username, email sql.NullString
		if err := rows.Scan(&item.ID, &item.WorkID, &item.WorkRevisionID, &item.CreatedAt,
			&version.ID, &version.NoteID, &parent, &version.CreatedInContextID, &version.State,
			&body.text, &body.bytes, &version.CreatedAt, &username, &email, &version.LinkCount); err != nil {
			return nil, err
		}
		version.ParentVersionID = nullInt64Pointer(parent)
		version.Body, version.BodyTruncated = body.optional()
		version.BodyBytes = body.bytes
		if version.Body != nil {
			version.Title, version.Excerpt = noteSummary(*version.Body)
		}
		version.Links = []Link{}
		version.ReviewerDisplay = reviewerDisplay(username.String, email.String)
		item.Version = version
		if version.CreatedInContextID != contextID {
			value := version.CreatedInContextID
			item.InheritedFromContextID = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
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
