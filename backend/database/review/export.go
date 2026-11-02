// export.go provides the bounded, immutable review evidence snapshot used by
// comment-enabled PDF exports. It reads only existing active heads in one
// selected context/work and never initializes review state, saves a draft, or
// mutates any head.
package review

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	generated "analysis/backend/database/review/internal/sql"
)

const (
	// MaxExportAnchors is the largest number of application annotations one PDF export may emit.
	MaxExportAnchors = 200
	// MaxExportTextBytes is the largest rendered annotation text one PDF export may emit.
	MaxExportTextBytes = 2_000_000
	// maxExportNotes bounds the saved note candidates loaded for one export.
	maxExportNotes = 200
	// maxExportNoteLinks bounds the saved note PDF references loaded for one export.
	maxExportNoteLinks = 4000
)

var (
	// ErrExportContextNotFound reports a missing review context for comment export.
	ErrExportContextNotFound = errors.New("review context not found")
	// ErrExportLimit reports comment evidence above a bounded export limit.
	ErrExportLimit = errors.New("comment export evidence exceeds its limit")
)

// ExportLimitError identifies one bounded comment-export limit that was exceeded.
type ExportLimitError struct {
	Kind  string
	Count int
	Limit int
}

// Error returns the bounded export-limit diagnostic.
func (e *ExportLimitError) Error() string {
	return fmt.Sprintf("comment export %s limit exceeded: %d exceeds %d", e.Kind, e.Count, e.Limit)
}

// Unwrap exposes the shared export-limit sentinel.
func (e *ExportLimitError) Unwrap() error { return ErrExportLimit }

// ExportAnchor is one immutable active anchor snapshot eligible for PDF output.
type ExportAnchor struct {
	ID              string            `json:"id"`
	Label           string            `json:"label"`
	VersionID       int64             `json:"version_id"`
	WorkRevisionID  int64             `json:"work_revision_id"`
	Page            int               `json:"page"`
	SelectedText    string            `json:"selected_text"`
	Rectangles      []AnchorRectangle `json:"rectangles"`
	ReviewerDisplay string            `json:"reviewer_display"`
	CreatedAt       string            `json:"created_at"`
}

// ExportNote is one immutable active note snapshot with its matching anchor references.
type ExportNote struct {
	ID              int64    `json:"id"`
	VersionID       int64    `json:"version_id"`
	WorkRevisionID  int64    `json:"work_revision_id"`
	Body            string   `json:"body"`
	AnchorIDs       []string `json:"anchor_ids"`
	ReviewerDisplay string   `json:"reviewer_display"`
	CreatedAt       string   `json:"created_at"`
}

// ExportExclusion aggregates excluded saved evidence by kind and reason.
type ExportExclusion struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// ExportEvidence is one bounded immutable snapshot of matching saved comment evidence.
type ExportEvidence struct {
	ContextID       int64             `json:"context_id"`
	WorkID          int64             `json:"work_id"`
	PDFContentHash  string            `json:"pdf_content_hash"`
	Anchors         []ExportAnchor    `json:"anchors"`
	Notes           []ExportNote      `json:"notes"`
	Exclusions      []ExportExclusion `json:"exclusions"`
	ExcludedAnchors int               `json:"excluded_anchors"`
	ExcludedNotes   int               `json:"excluded_notes"`
}

// ExportSnapshot is one bounded review read transaction shared by a ZIP's
// comment-evidence reads so concurrent note edits cannot mix heads.
type ExportSnapshot struct {
	tx      *sql.Tx
	queries *generated.Queries
	closed  bool
}

// BeginExportSnapshot starts one read-only review transaction. Callers must
// close it on every path.
func (s *Store) BeginExportSnapshot(ctx context.Context) (*ExportSnapshot, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin review export snapshot: %w", err)
	}
	return &ExportSnapshot{tx: tx, queries: s.queries.WithTx(tx)}, nil
}

// Close releases the pinned review read transaction.
func (s *ExportSnapshot) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if err := s.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

// ExportEvidence reads bounded current active note and anchor heads for one
// selected context/work and exact source hash through one short read snapshot.
func (s *Store) ExportEvidence(ctx context.Context, contextID, workID int64, pdfContentHash string) (*ExportEvidence, error) {
	snapshot, err := s.BeginExportSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()
	return snapshot.Evidence(ctx, contextID, workID, pdfContentHash)
}

// Evidence reads bounded current active note and anchor heads for one selected
// context/work and exact source hash. It includes inherited heads, excludes
// tombstones, drafts, other contexts/works, page-only/general notes, and notes
// mixing matching and mismatched PDF references, and reports aggregated
// exclusions. The snapshot closes before the caller writes export audit.
func (s *ExportSnapshot) Evidence(ctx context.Context, contextID, workID int64, pdfContentHash string) (*ExportEvidence, error) {
	if s == nil || s.closed {
		return nil, errors.New("review export snapshot is closed")
	}
	if contextID < 1 || workID < 1 {
		return nil, reviewValidation("review context and work must be positive identifiers")
	}
	if !validExportContentHash(pdfContentHash) {
		return nil, reviewValidation("PDF content hash must be a lowercase SHA-256 value")
	}
	queries := s.queries
	if _, err := queries.GetReviewExportContext(ctx, contextID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrExportContextNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read review export context: %w", err)
	}
	totalAnchors, err := queries.CountReviewExportAnchors(ctx, generated.CountReviewExportAnchorsParams{
		ReviewContextID: contextID,
		WorkID:          workID,
	})
	if err != nil {
		return nil, fmt.Errorf("count review export anchors: %w", err)
	}
	totalNotes, err := queries.CountReviewExportNotes(ctx, generated.CountReviewExportNotesParams{
		ReviewContextID: contextID,
		WorkID:          workID,
	})
	if err != nil {
		return nil, fmt.Errorf("count review export notes: %w", err)
	}
	anchorRows, err := queries.ListReviewExportAnchors(ctx, generated.ListReviewExportAnchorsParams{
		ReviewContextID: contextID,
		WorkID:          workID,
		PdfContentHash:  pdfContentHash,
		PageLimit:       MaxExportAnchors + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list review export anchors: %w", err)
	}
	if len(anchorRows) > MaxExportAnchors {
		return nil, &ExportLimitError{Kind: "annotation", Count: len(anchorRows), Limit: MaxExportAnchors}
	}
	noteRows, err := queries.ListReviewExportNotes(ctx, generated.ListReviewExportNotesParams{
		ReviewContextID: contextID,
		WorkID:          workID,
		PageLimit:       maxExportNotes + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list review export notes: %w", err)
	}
	if len(noteRows) > maxExportNotes {
		return nil, &ExportLimitError{Kind: "note", Count: len(noteRows), Limit: maxExportNotes}
	}
	linkRows, err := queries.ListReviewExportNoteLinks(ctx, generated.ListReviewExportNoteLinksParams{
		ReviewContextID: contextID,
		WorkID:          workID,
		PageLimit:       maxExportNoteLinks + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list review export note links: %w", err)
	}
	if len(linkRows) > maxExportNoteLinks {
		return nil, &ExportLimitError{Kind: "note reference", Count: len(linkRows), Limit: maxExportNoteLinks}
	}
	anchors, anchorIndex, err := exportAnchors(anchorRows)
	if err != nil {
		return nil, err
	}
	linksByVersion := make(map[int64][]generated.ListReviewExportNoteLinksRow, len(noteRows))
	for _, link := range linkRows {
		linksByVersion[link.NoteVersionID] = append(linksByVersion[link.NoteVersionID], link)
	}
	notes, excludedNotes, reasons := exportNotes(noteRows, linksByVersion, anchorIndex)
	evidence := &ExportEvidence{
		ContextID: contextID, WorkID: workID, PDFContentHash: pdfContentHash,
		Anchors: anchors, Notes: notes,
		ExcludedAnchors: int(totalAnchors) - len(anchors),
		ExcludedNotes:   int(totalNotes) - len(notes),
	}
	if evidence.ExcludedAnchors < 0 {
		evidence.ExcludedAnchors = 0
	}
	if evidence.ExcludedNotes < 0 {
		evidence.ExcludedNotes = 0
	}
	if evidence.ExcludedAnchors > 0 {
		reasons["anchor:pdf_hash_or_geometry_mismatch"] += evidence.ExcludedAnchors
	}
	if general := evidence.ExcludedNotes - excludedNotes; general > 0 {
		reasons["note:no_pdf_reference"] += general
	}
	evidence.Exclusions = sortedExportExclusions(reasons)
	if err := enforceExportTextLimit(evidence); err != nil {
		return nil, err
	}
	return evidence, nil
}

// exportAnchors maps matching anchor rows and their stored normalized geometry.
func exportAnchors(rows []generated.ListReviewExportAnchorsRow) ([]ExportAnchor, map[string]int, error) {
	anchors := make([]ExportAnchor, 0, len(rows))
	index := make(map[string]int, len(rows))
	for _, row := range rows {
		if !row.Page.Valid || row.Page.Int64 < 1 {
			continue
		}
		var rectangles []AnchorRectangle
		if row.RectanglesJson.Valid {
			if err := json.Unmarshal([]byte(row.RectanglesJson.String), &rectangles); err != nil {
				return nil, nil, fmt.Errorf("decode review export anchor rectangles: %w", err)
			}
		}
		if len(rectangles) == 0 {
			continue
		}
		anchor := ExportAnchor{
			ID: row.ID, Label: row.Label, VersionID: row.VersionID,
			WorkRevisionID: row.WorkRevisionID, Page: int(row.Page.Int64),
			SelectedText: row.SelectedText.String, Rectangles: rectangles,
			ReviewerDisplay: reviewerDisplay(row.ReviewerUsername), CreatedAt: row.VersionCreatedAt,
		}
		index[row.ID] = len(anchors)
		anchors = append(anchors, anchor)
	}
	return anchors, index, nil
}

// exportNotes resolves saved notes whose PDF references all match eligible anchors.
func exportNotes(rows []generated.ListReviewExportNotesRow, linksByVersion map[int64][]generated.ListReviewExportNoteLinksRow, anchorIndex map[string]int) ([]ExportNote, int, map[string]int) {
	notes := make([]ExportNote, 0, len(rows))
	reasons := make(map[string]int)
	excluded := 0
	for _, row := range rows {
		links := linksByVersion[row.VersionID]
		if len(links) == 0 {
			reasons["note:no_pdf_reference"]++
			excluded++
			continue
		}
		if !row.Body.Valid || strings.TrimSpace(row.Body.String) == "" {
			reasons["note:missing_body"]++
			excluded++
			continue
		}
		reason := ""
		anchorIDs := make([]string, 0, len(links))
		seen := make(map[string]bool, len(links))
		for _, link := range links {
			switch link.TargetType {
			case "pdf_page":
				reason = "note:page_only_reference"
			case "anchor":
				if _, ok := anchorIndex[link.RawTarget]; !ok {
					reason = "note:unresolved_or_mismatched_anchor"
					continue
				}
				if seen[link.RawTarget] {
					continue
				}
				seen[link.RawTarget] = true
				anchorIDs = append(anchorIDs, link.RawTarget)
			}
			if reason != "" {
				break
			}
		}
		if reason != "" {
			reasons[reason]++
			excluded++
			continue
		}
		if len(anchorIDs) == 0 {
			reasons["note:no_matching_anchor"]++
			excluded++
			continue
		}
		notes = append(notes, ExportNote{
			ID: row.ID, VersionID: row.VersionID, WorkRevisionID: row.WorkRevisionID,
			Body: row.Body.String, AnchorIDs: anchorIDs,
			ReviewerDisplay: reviewerDisplay(row.ReviewerUsername), CreatedAt: row.VersionCreatedAt,
		})
	}
	return notes, excluded, reasons
}

// enforceExportTextLimit rejects output whose repeated note prose and anchor labels exceed the bound.
func enforceExportTextLimit(evidence *ExportEvidence) error {
	byAnchor := make(map[string][]*ExportNote, len(evidence.Anchors))
	for index := range evidence.Notes {
		note := &evidence.Notes[index]
		for _, anchorID := range note.AnchorIDs {
			byAnchor[anchorID] = append(byAnchor[anchorID], note)
		}
	}
	total := 0
	for index := range evidence.Anchors {
		anchor := &evidence.Anchors[index]
		total += len(anchor.Label) + len(anchor.SelectedText) + len(anchor.ReviewerDisplay) + len(anchor.CreatedAt) + 64
		for _, note := range byAnchor[anchor.ID] {
			total += len(note.Body) + len(note.ReviewerDisplay) + len(note.CreatedAt) + 32
		}
	}
	if total > MaxExportTextBytes {
		return &ExportLimitError{Kind: "annotation text", Count: total, Limit: MaxExportTextBytes}
	}
	return nil
}

// sortedExportExclusions returns deterministic aggregated exclusion evidence.
func sortedExportExclusions(reasons map[string]int) []ExportExclusion {
	keys := make([]string, 0, len(reasons))
	for key, count := range reasons {
		if count > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	items := make([]ExportExclusion, 0, len(keys))
	for _, key := range keys {
		kind, reason, found := strings.Cut(key, ":")
		if !found {
			kind, reason = "note", key
		}
		items = append(items, ExportExclusion{Kind: kind, Reason: reason, Count: reasons[key]})
	}
	return items
}

// validExportContentHash requires one lowercase SHA-256 source identity.
func validExportContentHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}
