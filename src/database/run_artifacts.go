// run_artifacts.go provides the repository for content-addressed
// configuration and manifest snapshots linked to individual pipeline
// run attempts.
package database

import "fmt"

const (
	RunArtifactWorkspaceConfig  = "workspace_config"
	RunArtifactResolvedManifest = "resolved_manifest"
	RunArtifactInputManifest    = "input_manifest"
)

// RunArtifact links an attempt to a content-addressed configuration snapshot.
// The role distinguishes the raw workspace file from its resolved and input
// manifests without duplicating immutable artifact payloads.
type RunArtifact struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	ArtifactID    int64  `json:"artifact_id"`
	ArtifactRole  string `json:"artifact_role"`
	CreatedAt     string `json:"created_at"`
}

// RunArtifactRepository manages attempt-specific configuration artifact links.
type RunArtifactRepository struct {
	db *Database
}

// Link records one snapshot role for an attempt. Repeating the same link is
// idempotent; assigning a role to a different artifact is rejected.
func (r *RunArtifactRepository) Link(pipelineRunID, artifactID int64, role string) error {
	result, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role)
		 VALUES (?, ?, ?)`,
		pipelineRunID, artifactID, role,
	)
	if err != nil {
		return fmt.Errorf("link run artifact: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read run artifact link result: %w", err)
	}
	if inserted > 0 {
		return nil
	}
	var existingID int64
	if err := r.db.DB.QueryRow(
		"SELECT artifact_id FROM run_artifacts WHERE pipeline_run_id=? AND artifact_role=?",
		pipelineRunID, role,
	).Scan(&existingID); err != nil {
		return fmt.Errorf("read existing run artifact link: %w", err)
	}
	if existingID != artifactID {
		return fmt.Errorf("run %d already links %s to artifact %d", pipelineRunID, role, existingID)
	}
	return nil
}
