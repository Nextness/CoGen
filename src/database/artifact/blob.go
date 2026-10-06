// blob.go provides the handwritten artifact-blob data access layer over the
// generated statements. Content-addressed deduplication and byte comparison
// stay in Go so a repeated identity cannot conceal different bytes.
package artifact

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/artifact/internal/sql"
)

// CreateBlob inserts a new artifact blob. Returns the blob ID.
// If the artifact_id already exists, returns the existing blob ID (deduplicated).
func (s *Store) CreateBlob(ctx context.Context, input CreateBlobInput) (int64, error) {
	result, err := s.queries.InsertArtifactBlob(ctx, generated.InsertArtifactBlobParams{
		ArtifactID:    input.ArtifactID,
		PipelineRunID: input.PipelineRunID,
		Data:          input.Data,
	})
	if err != nil {
		return 0, fmt.Errorf("create artifact blob: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if inserted > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		return id, nil
	}

	// Already exists - return existing ID after verifying identical bytes.
	existing, err := s.GetBlobByArtifactID(ctx, input.ArtifactID)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		return 0, fmt.Errorf("create artifact blob: insert skipped but existing row not found")
	}
	if !bytes.Equal(existing.Data, input.Data) {
		return 0, fmt.Errorf("create artifact blob: artifact %d conflicts with stored bytes", input.ArtifactID)
	}
	return existing.ID, nil
}

// GetBlobByArtifactID returns the blob for a given artifact, or nil if not found.
func (s *Store) GetBlobByArtifactID(ctx context.Context, artifactID int64) (*Blob, error) {
	row, err := s.queries.GetArtifactBlobByArtifactID(ctx, artifactID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Blob{
		ID:            row.ID,
		ArtifactID:    row.ArtifactID,
		PipelineRunID: row.PipelineRunID,
		Data:          row.Data,
		CreatedAt:     row.CreatedAt,
	}, nil
}
