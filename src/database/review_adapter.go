// review_adapter.go provides temporary forwarding aliases and adapters that
// keep the legacy flat review repository API working over the review family
// store. The adapters contain no SQL or second implementation; remove them
// after every caller migrates to Database.Review.
package database

import (
	"context"

	"analysis/database/review"
)

// ReviewContext is one explicitly initialized interpretation context for a completed run.
type ReviewContext = review.Context

// ReviewContextCandidate describes one eligible parent context without materializing inherited state.
type ReviewContextCandidate = review.ContextCandidate

// WorkReviewVersion is one immutable complete article-review snapshot.
type WorkReviewVersion = review.WorkReviewVersion

// WorkReviewState is the complete current state for one context work head.
type WorkReviewState = review.WorkReviewState

// ReviewNoteVersion is one immutable active note snapshot or deletion tombstone.
type ReviewNoteVersion = review.NoteVersion

// ReviewNote is one logical note with the selected context's current immutable head.
type ReviewNote = review.Note

// ReviewLink is a version-scoped custom link plus its context-sensitive resolution.
type ReviewLink = review.Link

// AnchorRectangle is one normalized highlight rectangle on a single PDF page.
type AnchorRectangle = review.AnchorRectangle

// ReviewAnchorVersion is one immutable active highlight snapshot or deletion tombstone.
type ReviewAnchorVersion = review.AnchorVersion

// ReviewAnchor is one stable corpus-wide anchor with the selected context's current head.
type ReviewAnchor = review.Anchor

// ReviewConflictError reports that a context head changed after the caller read it.
type ReviewConflictError = review.ConflictError

// ReviewError is a safe repository error with a stable client-visible category.
type ReviewError = review.Error

// ReviewContextParentConflictError reports conflicting idempotent initialization choices.
type ReviewContextParentConflictError = review.ContextParentConflictError

// ReviewAnchorLabelConflictError reports a duplicate human anchor label within one work.
type ReviewAnchorLabelConflictError = review.AnchorLabelConflictError

// NoteSyntaxError reports parser diagnostics that make a note version unsaveable.
type NoteSyntaxError = review.NoteSyntaxError

// ReviewRepository forwards the legacy flat review API to the review family store.
type ReviewRepository struct{ db *Database }

// IsReviewConflict reports whether an error is an optimistic head conflict.
func IsReviewConflict(err error) bool { return review.IsConflict(err) }

// CorpusID returns the opaque corpus identity used to namespace browser-local drafts.
func (r *ReviewRepository) CorpusID(ctx context.Context) (string, error) {
	return r.db.Review.CorpusID(ctx)
}

// GetContextByRun returns the one initialized review context for a run, if present.
func (r *ReviewRepository) GetContextByRun(ctx context.Context, runID int64) (*ReviewContext, error) {
	return r.db.Review.GetContextByRun(ctx, runID)
}

// ProposeParent selects the latest initialized context from the same plan, then the same search.
func (r *ReviewRepository) ProposeParent(ctx context.Context, runID int64) (*ReviewContextCandidate, error) {
	return r.db.Review.ProposeParent(ctx, runID)
}

// ListParentCandidates returns bounded earlier contexts in stable descending run order.
func (r *ReviewRepository) ListParentCandidates(ctx context.Context, runID int64, scope, cursorStartedAt string, cursorRunID int64, limit int, query string) ([]ReviewContextCandidate, error) {
	return r.db.Review.ListParentCandidates(ctx, runID, scope, cursorStartedAt, cursorRunID, limit, query)
}

// CreateContext initializes one run context and freezes matching parent heads without copying version bodies.
func (r *ReviewRepository) CreateContext(ctx context.Context, runID int64, parentContextID *int64) (*ReviewContext, bool, error) {
	return r.db.Review.CreateContext(ctx, runID, parentContextID)
}

// GetWorkReview returns the complete current state for one context work revision.
func (r *ReviewRepository) GetWorkReview(ctx context.Context, contextID, workRevisionID int64) (*WorkReviewState, error) {
	return r.db.Review.GetWorkReview(ctx, contextID, workRevisionID)
}

// AppendWorkReview appends a complete immutable state and compare-and-swaps only the selected context head.
func (r *ReviewRepository) AppendWorkReview(ctx context.Context, contextID, workRevisionID int64, expectedVersionID *int64, status string, substatuses []string, reason *string) (*WorkReviewState, bool, error) {
	return r.db.Review.AppendWorkReview(ctx, contextID, workRevisionID, expectedVersionID, status, substatuses, reason)
}

// ListWorkReviewVersions follows only the selected head's ancestor chain.
func (r *ReviewRepository) ListWorkReviewVersions(ctx context.Context, contextID, workRevisionID, cursor int64, limit int) ([]WorkReviewVersion, error) {
	return r.db.Review.ListWorkReviewVersions(ctx, contextID, workRevisionID, cursor, limit)
}

// GetWorkReviewVersion returns one full version only when it belongs to the selected head ancestry.
func (r *ReviewRepository) GetWorkReviewVersion(ctx context.Context, contextID, workRevisionID, versionID int64) (*WorkReviewVersion, error) {
	return r.db.Review.GetWorkReviewVersion(ctx, contextID, workRevisionID, versionID)
}

// CreateNote creates one logical note, immutable first version, head, links, and audit atomically.
func (r *ReviewRepository) CreateNote(ctx context.Context, contextID, workRevisionID int64, body string) (*ReviewNote, error) {
	return r.db.Review.CreateNote(ctx, contextID, workRevisionID, body)
}

// AppendNoteVersion appends an active edit or deletion tombstone with optimistic concurrency.
func (r *ReviewRepository) AppendNoteVersion(ctx context.Context, contextID, noteID int64, expectedVersionID int64, state, body string) (*ReviewNote, bool, error) {
	return r.db.Review.AppendNoteVersion(ctx, contextID, noteID, expectedVersionID, state, body)
}

// GetNote returns an explicitly addressed current note head, including tombstones.
func (r *ReviewRepository) GetNote(ctx context.Context, contextID, noteID int64) (*ReviewNote, error) {
	return r.db.Review.GetNote(ctx, contextID, noteID)
}

// ListNotes returns bounded current note heads for one context work, excluding tombstones unless requested.
func (r *ReviewRepository) ListNotes(ctx context.Context, contextID, workRevisionID, cursor int64, limit int, includeDeleted bool) ([]ReviewNote, error) {
	return r.db.Review.ListNotes(ctx, contextID, workRevisionID, cursor, limit, includeDeleted)
}

// ListNotesFiltered returns bounded current note-head summaries for one work or the complete run context.
func (r *ReviewRepository) ListNotesFiltered(ctx context.Context, contextID int64, workRevisionID *int64, cursor int64, limit int, state, query string) ([]ReviewNote, error) {
	return r.db.Review.ListNotesFiltered(ctx, contextID, workRevisionID, cursor, limit, state, query)
}

// ListNoteVersions follows only the selected context note head's ancestors.
func (r *ReviewRepository) ListNoteVersions(ctx context.Context, contextID, noteID, cursor int64, limit int) ([]ReviewNoteVersion, error) {
	return r.db.Review.ListNoteVersions(ctx, contextID, noteID, cursor, limit)
}

// GetNoteVersion returns one full body and link set only when it belongs to the selected head ancestry.
func (r *ReviewRepository) GetNoteVersion(ctx context.Context, contextID, noteID, versionID int64) (*ReviewNoteVersion, error) {
	return r.db.Review.GetNoteVersion(ctx, contextID, noteID, versionID)
}

// ListBacklinks returns links from current note heads only.
func (r *ReviewRepository) ListBacklinks(ctx context.Context, contextID int64, targetType, targetID string, sourceWorkID, cursor int64, limit int) ([]ReviewNote, error) {
	return r.db.Review.ListBacklinks(ctx, contextID, targetType, targetID, sourceWorkID, cursor, limit)
}

// CreateAnchor creates one generated logical anchor with a work-scoped label and immutable first version atomically.
func (r *ReviewRepository) CreateAnchor(ctx context.Context, contextID, workRevisionID int64, label, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) (*ReviewAnchor, error) {
	return r.db.Review.CreateAnchor(ctx, contextID, workRevisionID, label, contentHash, page, selectedText, rectangles)
}

// AppendAnchorVersion appends an active replacement or tombstone using optimistic concurrency.
func (r *ReviewRepository) AppendAnchorVersion(ctx context.Context, contextID int64, anchorID string, expectedVersionID int64, state, contentHash string, page int, selectedText string, rectangles []AnchorRectangle) (*ReviewAnchor, bool, error) {
	return r.db.Review.AppendAnchorVersion(ctx, contextID, anchorID, expectedVersionID, state, contentHash, page, selectedText, rectangles)
}

// GetAnchor returns one selected-context logical anchor and its current head.
func (r *ReviewRepository) GetAnchor(ctx context.Context, contextID int64, anchorID string) (*ReviewAnchor, error) {
	return r.db.Review.GetAnchor(ctx, contextID, anchorID)
}

// ListAnchors returns bounded active current anchors for one context work.
func (r *ReviewRepository) ListAnchors(ctx context.Context, contextID, workRevisionID int64, cursor string, limit int) ([]ReviewAnchor, error) {
	return r.db.Review.ListAnchors(ctx, contextID, workRevisionID, cursor, limit)
}

// ListAnchorVersions follows only the selected context anchor head's ancestors.
func (r *ReviewRepository) ListAnchorVersions(ctx context.Context, contextID int64, anchorID string, cursor int64, limit int) ([]ReviewAnchorVersion, error) {
	return r.db.Review.ListAnchorVersions(ctx, contextID, anchorID, cursor, limit)
}

// GetAnchorVersion returns one full geometry version only when it belongs to the selected head ancestry.
func (r *ReviewRepository) GetAnchorVersion(ctx context.Context, contextID int64, anchorID string, versionID int64) (*ReviewAnchorVersion, error) {
	return r.db.Review.GetAnchorVersion(ctx, contextID, anchorID, versionID)
}
