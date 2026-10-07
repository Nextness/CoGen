// errors.go centralizes the review family's stable error categories, validation
// helpers, and persisted review vocabulary shared by every table workflow.
package review

import (
	"errors"

	"analysis/notes"
)

// validReviewStatuses is the closed persisted work-review status vocabulary.
var validReviewStatuses = map[string]bool{"not_evaluated": true, "in_progress": true, "approved": true, "not_approved": true, "removed": true}

// validReviewSubstatuses is the closed persisted work-review sub-status vocabulary.
var validReviewSubstatuses = map[string]bool{
	"redacted": true, "unrelated": true, "out_of_scope": true, "duplicate": true,
	"retracted": true, "withdrawn": true, "superseded": true, "predatory_low_quality": true,
	"copyright_licensing": true, "not_peer_reviewed": true,
}

// ConflictError reports that a context head changed after the caller read it.
type ConflictError struct {
	Expected *int64
	Current  *int64
}

// Error returns a safe optimistic-concurrency diagnostic.
func (e *ConflictError) Error() string { return "review head changed; reload before saving" }

// Error is a safe repository error with a stable client-visible category.
type Error struct {
	Kind    string
	Message string
}

// Error returns the safe repository diagnostic.
func (e *Error) Error() string { return e.Message }

// ContextParentConflictError reports conflicting idempotent initialization choices.
type ContextParentConflictError struct {
	Requested *int64
	Existing  *int64
}

// Error returns a safe immutable-lineage diagnostic.
func (e *ContextParentConflictError) Error() string {
	return "review context was already initialized with a different parent"
}

// AnchorLabelConflictError reports a duplicate human anchor label within one work.
type AnchorLabelConflictError struct {
	Label string
}

// Error returns a safe work-scoped label diagnostic.
func (e *AnchorLabelConflictError) Error() string {
	return "anchor label is already used for this article"
}

// NoteSyntaxError reports parser diagnostics that make a note version unsaveable.
type NoteSyntaxError struct{ Errors []notes.SyntaxError }

// Error returns a safe note-language diagnostic.
func (e *NoteSyntaxError) Error() string { return "note contains syntax errors" }

// reviewValidation reports invalid caller-controlled review input.
func reviewValidation(message string) error {
	return &Error{Kind: "validation", Message: message}
}

// reviewNotFound reports a missing review-scoped record.
func reviewNotFound(message string) error { return &Error{Kind: "not_found", Message: message} }

// reviewLifecycle reports a valid request rejected by immutable run lifecycle.
func reviewLifecycle(message string) error { return &Error{Kind: "lifecycle", Message: message} }

// IsConflict reports whether an error is an optimistic head conflict.
func IsConflict(err error) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict)
}
