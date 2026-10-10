// types.go defines the review family application types shared by the context,
// decision, note, and anchor workflows. Generated query types, once this family
// adopts sqlc, stay inside the private internal/sql package and never cross
// this family boundary.
package review

// Context is one explicitly initialized interpretation context for a completed run.
type Context struct {
	ID              int64  `json:"id"`
	PipelineRunID   int64  `json:"pipeline_run_id"`
	ParentContextID *int64 `json:"parent_context_id,omitempty"`
	CreatedAt       string `json:"created_at"`
}

// ContextCandidate describes one eligible parent context without materializing inherited state.
type ContextCandidate struct {
	ContextID          int64  `json:"context_id"`
	PipelineRunID      int64  `json:"pipeline_run_id"`
	SearchID           string `json:"search_id"`
	SearchRevision     string `json:"search_revision"`
	ExecutionPlanID    int64  `json:"execution_plan_id"`
	AttemptNumber      int    `json:"attempt_number"`
	StartedAt          string `json:"started_at"`
	InheritedWorkCount int    `json:"inherited_work_count"`
}

// WorkReviewVersion is one immutable complete article-review snapshot.
type WorkReviewVersion struct {
	ID                 int64    `json:"id"`
	WorkID             int64    `json:"work_id"`
	WorkRevisionID     int64    `json:"work_revision_id"`
	CreatedInContextID int64    `json:"created_in_context_id"`
	ParentVersionID    *int64   `json:"parent_version_id,omitempty"`
	Status             string   `json:"status"`
	Substatuses        []string `json:"sub_statuses"`
	Reason             *string  `json:"reason"`
	ReasonTruncated    bool     `json:"reason_truncated"`
	CreatedAt          string   `json:"created_at"`
	ReviewerDisplay    string   `json:"reviewer_display"`
}

// WorkReviewState is the complete current state for one context work head.
type WorkReviewState struct {
	ContextID              int64              `json:"context_id"`
	WorkID                 int64              `json:"work_id"`
	WorkRevisionID         int64              `json:"work_revision_id"`
	Version                *WorkReviewVersion `json:"version,omitempty"`
	InheritedFromContextID *int64             `json:"inherited_from_context_id,omitempty"`
}

// workReviewAuditState is the bounded decision payload stored in audit before/after state.
type workReviewAuditState struct {
	Status      string   `json:"status"`
	Reason      *string  `json:"reason"`
	Substatuses []string `json:"sub_statuses"`
}

// NoteVersion is one immutable active note snapshot or deletion tombstone.
type NoteVersion struct {
	ID                 int64   `json:"id"`
	NoteID             int64   `json:"note_id"`
	ParentVersionID    *int64  `json:"parent_version_id,omitempty"`
	CreatedInContextID int64   `json:"created_in_context_id"`
	State              string  `json:"state"`
	Body               *string `json:"body"`
	BodyBytes          int     `json:"body_bytes"`
	BodyTruncated      bool    `json:"body_truncated"`
	Title              string  `json:"title"`
	Excerpt            string  `json:"excerpt"`
	LinkCount          int     `json:"link_count"`
	CreatedAt          string  `json:"created_at"`
	ReviewerDisplay    string  `json:"reviewer_display"`
	Links              []Link  `json:"links"`
}

// Note is one logical note with the selected context's current immutable head.
type Note struct {
	ID                     int64       `json:"id"`
	WorkID                 int64       `json:"work_id"`
	WorkRevisionID         int64       `json:"work_revision_id"`
	CreatedAt              string      `json:"created_at"`
	Version                NoteVersion `json:"version"`
	InheritedFromContextID *int64      `json:"inherited_from_context_id,omitempty"`
}

// Link is a version-scoped custom link plus its context-sensitive resolution.
type Link struct {
	Ordinal        int     `json:"ordinal"`
	TargetType     string  `json:"target_type"`
	RawTarget      string  `json:"raw_target"`
	DisplayText    *string `json:"display_text,omitempty"`
	UTF16Position  int     `json:"utf16_position"`
	UTF16Length    int     `json:"utf16_length"`
	Resolved       bool    `json:"resolved"`
	WorkRevisionID *int64  `json:"work_revision_id,omitempty"`
	NoteID         *int64  `json:"note_id,omitempty"`
	AnchorID       *string `json:"anchor_id,omitempty"`
	Page           *int    `json:"page,omitempty"`
	URL            *string `json:"url,omitempty"`
}

// AnchorRectangle is one normalized highlight rectangle on a single PDF page.
type AnchorRectangle struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// AnchorVersion is one immutable active highlight snapshot or deletion tombstone.
type AnchorVersion struct {
	ID                    int64             `json:"id"`
	AnchorID              string            `json:"anchor_id"`
	ParentVersionID       *int64            `json:"parent_version_id,omitempty"`
	CreatedInContextID    int64             `json:"created_in_context_id"`
	WorkRevisionID        int64             `json:"work_revision_id"`
	PDFContentHash        string            `json:"pdf_content_hash"`
	State                 string            `json:"state"`
	Page                  *int              `json:"page,omitempty"`
	SelectedText          *string           `json:"selected_text,omitempty"`
	SelectedTextTruncated bool              `json:"selected_text_truncated"`
	Rectangles            []AnchorRectangle `json:"rectangles,omitempty"`
	CreatedAt             string            `json:"created_at"`
	ReviewerDisplay       string            `json:"reviewer_display"`
}

// Anchor is one stable corpus-wide anchor with the selected context's current head.
type Anchor struct {
	ID                     string        `json:"id"`
	Label                  string        `json:"label"`
	WorkID                 int64         `json:"work_id"`
	CreatedAt              string        `json:"created_at"`
	Version                AnchorVersion `json:"version"`
	InheritedFromContextID *int64        `json:"inherited_from_context_id,omitempty"`
}

// AnchorRestore is one active anchor version's restorable geometry.
type AnchorRestore struct {
	PDFContentHash string
	Page           int
	SelectedText   string
	RectanglesJSON string
}

// SummaryCounts is the current note, anchor, and decision-version summary for
// one work in one review context.
type SummaryCounts struct {
	NoteCount          int64
	AnchorCount        int64
	ReviewVersionCount int64
}
