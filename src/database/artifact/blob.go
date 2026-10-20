// blob.go provides the handwritten artifact-blob data access layer over the
// generated statements. Artifact and blob creation share CreateWithBlob.
package artifact

import (
	"context"
	"database/sql"
	"errors"

	generated "analysis/database/artifact/internal/sql"
)

// ErrNoBlob reports that an artifact exists but has no stored bytes.
var ErrNoBlob = errors.New("artifact has no blob data")

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
