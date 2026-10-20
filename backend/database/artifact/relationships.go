// relationships.go provides the handwritten artifact-primary relationship
// reads over the generated statements. A query whose primary result is an
// artifact belongs to this family even when it joins run steps, cache entries,
// or identity-candidate evidence owned by another family.
package artifact

import (
	"context"
)

// ListRunArtifacts returns every artifact related to a run with its
// relationship evidence, ordered by artifact ID, relationship role, and detail.
// The roles are run_role, step_input, step_output, cache_payload, and
// identity_candidate_payload.
func (s *Store) ListRunArtifacts(ctx context.Context, runID int64) ([]*RelatedArtifact, error) {
	rows, err := s.queries.ListRunArtifacts(ctx, runID)
	if err != nil {
		return nil, err
	}
	var artifacts []*RelatedArtifact
	for _, row := range rows {
		artifacts = append(artifacts, &RelatedArtifact{
			Artifact: Artifact{
				ID:          row.ID,
				ContentHash: row.ContentHash,
				ByteSize:    row.ByteSize,
				ContentType: row.ContentType,
				CreatedAt:   row.CreatedAt,
			},
			Role:   row.RelationshipRole,
			Detail: row.RelationshipDetail,
		})
	}
	return artifacts, nil
}
