// reviewer.go provides the handwritten per-run reviewer attribution over the
// generated statements. Reviewer identity is immutable and stays private to
// the owning run.
package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	generated "analysis/database/run/internal/sql"
)

// InsertReviewer records one immutable reviewer identity for a newly created run.
func (s *Store) InsertReviewer(ctx context.Context, input ReviewerInput) error {
	username := strings.TrimSpace(input.Username)
	email := strings.TrimSpace(input.Email)
	if input.RunID < 1 {
		return fmt.Errorf("reviewer pipeline run ID must be positive")
	}
	if utf8.RuneCountInString(username) > 200 {
		return fmt.Errorf("reviewer username exceeds 200 characters")
	}
	if utf8.RuneCountInString(email) > 320 {
		return fmt.Errorf("reviewer email exceeds 320 characters")
	}
	if err := s.queries.InsertReviewer(ctx, generated.InsertReviewerParams{
		PipelineRunID: input.RunID,
		Username:      username,
		Email:         email,
		CreatedAt:     runTimestamp(),
	}); err != nil {
		return fmt.Errorf("insert pipeline run reviewer: %w", err)
	}
	return nil
}

// GetReviewer returns the reviewer captured for a run, or nil when a legacy
// writer omitted it.
func (s *Store) GetReviewer(ctx context.Context, runID int64) (*Reviewer, error) {
	row, err := s.queries.GetReviewerByRun(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pipeline run reviewer: %w", err)
	}
	return &Reviewer{
		PipelineRunID: row.PipelineRunID,
		Username:      row.Username,
		Email:         row.Email,
		CreatedAt:     row.CreatedAt,
	}, nil
}
