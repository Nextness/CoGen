// run_artifact.go provides the handwritten run-artifact link data access layer
// over the generated statements. Role scoping and idempotent replay stay in Go
// so one role cannot silently point at a different artifact.
package artifact

import (
	"context"
	"fmt"

	generated "analysis/database/artifact/internal/sql"
)

// Link records one snapshot role for an attempt. Repeating the same link is
// idempotent; assigning a role to a different artifact is rejected.
func (s *Store) Link(ctx context.Context, input LinkInput) error {
	result, err := s.queries.InsertRunArtifact(ctx, generated.InsertRunArtifactParams{
		PipelineRunID: input.PipelineRunID,
		ArtifactID:    input.ArtifactID,
		ArtifactRole:  input.Role,
	})
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
	existingID, err := s.queries.GetRunArtifactByRunAndRole(ctx, generated.GetRunArtifactByRunAndRoleParams{
		PipelineRunID: input.PipelineRunID,
		ArtifactRole:  input.Role,
	})
	if err != nil {
		return fmt.Errorf("read existing run artifact link: %w", err)
	}
	if existingID != input.ArtifactID {
		return fmt.Errorf("run %d already links %s to artifact %d", input.PipelineRunID, input.Role, existingID)
	}
	return nil
}
