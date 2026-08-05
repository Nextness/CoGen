package database

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

// PipelineRunReviewer is the immutable reviewer identity captured for one run.
type PipelineRunReviewer struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	CreatedAt     string `json:"created_at"`
}

// PipelineRunReviewerRepository stores and reads per-run reviewer attribution.
type PipelineRunReviewerRepository struct{ db *Database }

// Insert records one immutable reviewer identity for a newly created run.
func (r *PipelineRunReviewerRepository) Insert(runID int64, username, email string) error {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if runID < 1 {
		return fmt.Errorf("reviewer pipeline run ID must be positive")
	}
	if utf8.RuneCountInString(username) > 200 {
		return fmt.Errorf("reviewer username exceeds 200 characters")
	}
	if utf8.RuneCountInString(email) > 320 {
		return fmt.Errorf("reviewer email exceeds 320 characters")
	}
	_, err := r.db.DB.Exec(`INSERT INTO pipeline_run_reviewers
		(pipeline_run_id, username, email, created_at) VALUES (?, ?, ?, ?)`, runID, username, email, timestamp())
	if err != nil {
		return fmt.Errorf("insert pipeline run reviewer: %w", err)
	}
	return nil
}

// Get returns the reviewer captured for a run, or nil when a legacy writer omitted it.
func (r *PipelineRunReviewerRepository) Get(runID int64) (*PipelineRunReviewer, error) {
	var reviewer PipelineRunReviewer
	err := r.db.DB.QueryRow(`SELECT pipeline_run_id, username, email, created_at
		FROM pipeline_run_reviewers WHERE pipeline_run_id=?`, runID).Scan(
		&reviewer.PipelineRunID, &reviewer.Username, &reviewer.Email, &reviewer.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pipeline run reviewer: %w", err)
	}
	return &reviewer, nil
}
