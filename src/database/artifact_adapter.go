// artifact_adapter.go provides temporary forwarding adapters that keep the
// legacy flat artifact, artifact-blob, and run-artifact repository API working
// over the artifact family store. The adapters contain no SQL or second
// implementation; remove them after every caller migrates to Database.Artifact.
package database

import (
	"context"

	"analysis/database/artifact"
)

// Run artifact role values for content-addressed configuration snapshots.
const (
	RunArtifactWorkspaceConfig  = artifact.RunArtifactWorkspaceConfig
	RunArtifactResolvedManifest = artifact.RunArtifactResolvedManifest
	RunArtifactInputManifest    = artifact.RunArtifactInputManifest
)

// Artifact is a content-addressed immutable payload.
type Artifact struct {
	ID          int64  `json:"id"`
	ContentHash string `json:"content_hash"`
	ByteSize    int64  `json:"byte_size"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
}

// ArtifactBlob stores the raw bytes for an artifact inline in the database.
type ArtifactBlob struct {
	ID            int64  `json:"id"`
	ArtifactID    int64  `json:"artifact_id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	Data          []byte `json:"-"`
	CreatedAt     string `json:"created_at"`
}

// RunArtifact links an attempt to a content-addressed configuration snapshot.
// The role distinguishes the raw workspace file from its resolved and input
// manifests without duplicating immutable artifact payloads.
type RunArtifact struct {
	PipelineRunID int64  `json:"pipeline_run_id"`
	ArtifactID    int64  `json:"artifact_id"`
	ArtifactRole  string `json:"artifact_role"`
	CreatedAt     string `json:"created_at"`
}

// ArtifactRepository forwards the legacy artifact API to the artifact family store.
type ArtifactRepository struct{ db *Database }

// ArtifactBlobRepository forwards the legacy artifact-blob API to the artifact family store.
type ArtifactBlobRepository struct{ db *Database }

// RunArtifactRepository forwards the legacy run-artifact API to the artifact family store.
type RunArtifactRepository struct{ db *Database }

// Create inserts a new artifact. Returns the artifact ID.
// If the content_hash already exists, returns the existing artifact ID.
func (r *ArtifactRepository) Create(contentHash, contentType string, byteSize int64) (int64, error) {
	return r.db.Artifact.Create(context.Background(), artifact.CreateInput{
		ContentHash: contentHash,
		ContentType: contentType,
		ByteSize:    byteSize,
	})
}

// CreateWithBlob atomically records an artifact and its bytes for a pipeline run.
func (r *ArtifactRepository) CreateWithBlob(contentHash, contentType string, byteSize, pipelineRunID int64, data []byte) (int64, error) {
	return r.db.Artifact.CreateWithBlob(context.Background(), artifact.CreateWithBlobInput{
		ContentHash:   contentHash,
		ContentType:   contentType,
		ByteSize:      byteSize,
		PipelineRunID: pipelineRunID,
		Data:          data,
	})
}

// GetByHash returns an artifact by its content hash, or nil if not found.
func (r *ArtifactRepository) GetByHash(contentHash string) (*Artifact, error) {
	found, err := r.db.Artifact.GetByHash(context.Background(), contentHash)
	if err != nil {
		return nil, err
	}
	return artifactFromFamily(found), nil
}

// GetByID returns an artifact by its primary key, or nil if not found.
func (r *ArtifactRepository) GetByID(id int64) (*Artifact, error) {
	found, err := r.db.Artifact.GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return artifactFromFamily(found), nil
}

// Create inserts a new artifact blob. Returns the blob ID.
// If the artifact_id already exists, returns the existing blob ID (deduplicated).
func (r *ArtifactBlobRepository) Create(artifactID, pipelineRunID int64, data []byte) (int64, error) {
	return r.db.Artifact.CreateBlob(context.Background(), artifact.CreateBlobInput{
		ArtifactID:    artifactID,
		PipelineRunID: pipelineRunID,
		Data:          data,
	})
}

// GetByArtifactID returns the blob for a given artifact, or nil if not found.
func (r *ArtifactBlobRepository) GetByArtifactID(artifactID int64) (*ArtifactBlob, error) {
	found, err := r.db.Artifact.GetBlobByArtifactID(context.Background(), artifactID)
	if err != nil {
		return nil, err
	}
	return artifactBlobFromFamily(found), nil
}

// Link records one snapshot role for an attempt. Repeating the same link is
// idempotent; assigning a role to a different artifact is rejected.
func (r *RunArtifactRepository) Link(pipelineRunID, artifactID int64, role string) error {
	return r.db.Artifact.Link(context.Background(), artifact.LinkInput{
		PipelineRunID: pipelineRunID,
		ArtifactID:    artifactID,
		Role:          role,
	})
}

// artifactFromFamily maps an artifact family artifact into the legacy application type.
func artifactFromFamily(found *artifact.Artifact) *Artifact {
	if found == nil {
		return nil
	}
	return &Artifact{
		ID:          found.ID,
		ContentHash: found.ContentHash,
		ByteSize:    found.ByteSize,
		ContentType: found.ContentType,
		CreatedAt:   found.CreatedAt,
	}
}

// artifactBlobFromFamily maps an artifact family blob into the legacy application type.
func artifactBlobFromFamily(found *artifact.Blob) *ArtifactBlob {
	if found == nil {
		return nil
	}
	return &ArtifactBlob{
		ID:            found.ID,
		ArtifactID:    found.ArtifactID,
		PipelineRunID: found.PipelineRunID,
		Data:          found.Data,
		CreatedAt:     found.CreatedAt,
	}
}
