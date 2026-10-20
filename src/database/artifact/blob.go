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

// ErrNoBlob reports that an artifact exists but has no stored bytes.
var ErrNoBlob = errors.New("artifact has no blob data")

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

// GetContent returns one artifact's stored bytes, media type, and first run
// role. It returns nil, nil when the artifact does not exist and ErrNoBlob when
// the artifact exists without stored bytes.
func (s *Store) GetContent(ctx context.Context, artifactID int64) (*Content, error) {
	row, err := s.queries.GetArtifactContent(ctx, artifactID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Data == nil {
		return nil, ErrNoBlob
	}
	return &Content{
		ContentType: row.ContentType,
		Role:        row.ArtifactRole,
		Data:        row.Data,
	}, nil
}

// GetPreview returns a bounded artifact prefix with its stored metadata. It
// returns nil, nil when the artifact does not exist and ErrNoBlob when the
// artifact exists without stored bytes.
func (s *Store) GetPreview(ctx context.Context, artifactID int64, previewBytes int) (*Preview, error) {
	row, err := s.queries.GetArtifactPreview(ctx, generated.GetArtifactPreviewParams{
		PreviewBytes: int64(previewBytes),
		ID:           artifactID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.HasBlob == 0 {
		return nil, ErrNoBlob
	}
	return &Preview{
		ContentType: row.ContentType,
		ByteSize:    row.ByteSize,
		BlobSize:    row.BlobSize,
		Data:        row.Data,
	}, nil
}
