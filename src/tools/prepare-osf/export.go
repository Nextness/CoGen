// export.go provides the private typed export adapter used by the prepare-osf
// sanitization workflow. The adapter binds generated queries to the copied
// metadata connection or transaction; the copy, pragma, integrity, and
// publication orchestration stays in main.go.
package main

import (
	"context"
	"database/sql"
	"fmt"

	generated "analysis/tools/prepare-osf/internal/sql"
)

// exportArtifact is one workspace-configuration artifact awaiting sanitization.
type exportArtifact struct {
	ID            int64
	ContentHash   string
	ContentType   string
	PipelineRunID int64
	Data          []byte
}

// exportBlob is one copied artifact blob awaiting integrity verification.
type exportBlob struct {
	ContentHash string
	ByteSize    int64
	Data        []byte
}

// exportStore binds the generated export queries to one copied metadata connection.
type exportStore struct {
	queries *generated.Queries
}

// newExportStore returns an export adapter over an already configured connection.
func newExportStore(db *sql.DB) *exportStore {
	return &exportStore{queries: generated.New(db)}
}

// withTx returns an export adapter bound to the caller's transaction.
func (s *exportStore) withTx(tx *sql.Tx) *exportStore {
	return &exportStore{queries: s.queries.WithTx(tx)}
}

// binding returns the singleton relative companion path.
func (s *exportStore) binding(ctx context.Context) (string, error) {
	relativePath, err := s.queries.GetPDFStoreBinding(ctx)
	if err != nil {
		return "", fmt.Errorf("read PDF binding: %w", err)
	}
	return relativePath, nil
}

// redactReviewers blanks every reviewer identity row.
func (s *exportStore) redactReviewers(ctx context.Context) error {
	if err := s.queries.RedactReviewers(ctx); err != nil {
		return fmt.Errorf("redact reviewers: %w", err)
	}
	return nil
}

// regenerateCorpusID replaces the copied review corpus identity.
func (s *exportStore) regenerateCorpusID(ctx context.Context) error {
	if err := s.queries.RegenerateReviewCorpusID(ctx); err != nil {
		return fmt.Errorf("regenerate review corpus ID: %w", err)
	}
	return nil
}

// workspaceConfigArtifacts returns the distinct workspace-configuration artifacts in artifact ID order.
func (s *exportStore) workspaceConfigArtifacts(ctx context.Context) ([]exportArtifact, error) {
	rows, err := s.queries.ListWorkspaceConfigArtifacts(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]exportArtifact, 0, len(rows))
	for _, row := range rows {
		items = append(items, exportArtifact{
			ID: row.ID, ContentHash: row.ContentHash, ContentType: row.ContentType,
			PipelineRunID: row.PipelineRunID, Data: row.Data,
		})
	}
	return items, nil
}

// artifactByHash returns the artifact ID for a content hash and whether it exists.
func (s *exportStore) artifactByHash(ctx context.Context, contentHash string) (int64, bool, error) {
	id, err := s.queries.GetArtifactByHash(ctx, contentHash)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// insertArtifact records one sanitized content identity and returns its ID.
func (s *exportStore) insertArtifact(ctx context.Context, contentHash string, byteSize int64, contentType string) (int64, error) {
	result, err := s.queries.InsertArtifact(ctx, generated.InsertArtifactParams{
		ContentHash: contentHash, ByteSize: byteSize, ContentType: contentType,
	})
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// insertArtifactBlob records one sanitized blob for an artifact.
func (s *exportStore) insertArtifactBlob(ctx context.Context, artifactID, pipelineRunID int64, data []byte) error {
	return s.queries.InsertArtifactBlob(ctx, generated.InsertArtifactBlobParams{
		ArtifactID: artifactID, PipelineRunID: pipelineRunID, Data: data,
	})
}

// rewireRunArtifacts points run-artifact links at the sanitized artifact.
func (s *exportStore) rewireRunArtifacts(ctx context.Context, newArtifactID, oldArtifactID int64) error {
	return s.queries.RewireRunArtifacts(ctx, generated.RewireRunArtifactsParams{
		NewArtifactID: newArtifactID, OldArtifactID: oldArtifactID,
	})
}

// rewireSearchRevisions points search revisions at the sanitized configuration hash.
func (s *exportStore) rewireSearchRevisions(ctx context.Context, newHash, oldHash string) error {
	return s.queries.RewireSearchRevisions(ctx, generated.RewireSearchRevisionsParams{
		NewHash: newHash, OldHash: oldHash,
	})
}

// rewireRunStepInputs points step inputs at the sanitized artifact identity.
func (s *exportStore) rewireRunStepInputs(ctx context.Context, newArtifactID int64, newHash string, oldArtifactID int64) error {
	return s.queries.RewireRunStepInputs(ctx, generated.RewireRunStepInputsParams{
		NewArtifactID: sql.NullInt64{Int64: newArtifactID, Valid: true},
		NewHash:       newHash,
		OldArtifactID: sql.NullInt64{Int64: oldArtifactID, Valid: true},
	})
}

// rewireRunStepOutputs points step outputs at the sanitized artifact identity.
func (s *exportStore) rewireRunStepOutputs(ctx context.Context, newArtifactID int64, newHash string, oldArtifactID int64) error {
	return s.queries.RewireRunStepOutputs(ctx, generated.RewireRunStepOutputsParams{
		NewArtifactID: sql.NullInt64{Int64: newArtifactID, Valid: true},
		NewHash:       newHash,
		OldArtifactID: sql.NullInt64{Int64: oldArtifactID, Valid: true},
	})
}

// artifactReferenceCount counts every remaining reference to an original artifact.
func (s *exportStore) artifactReferenceCount(ctx context.Context, artifactID int64) (int64, error) {
	return s.queries.CountArtifactReferences(ctx, artifactID)
}

// deleteArtifactBlob removes one original artifact blob.
func (s *exportStore) deleteArtifactBlob(ctx context.Context, artifactID int64) error {
	return s.queries.DeleteArtifactBlob(ctx, artifactID)
}

// deleteArtifact removes one original artifact row.
func (s *exportStore) deleteArtifact(ctx context.Context, artifactID int64) error {
	return s.queries.DeleteArtifact(ctx, artifactID)
}

// artifactBlobs returns every copied artifact and blob pair for integrity verification.
func (s *exportStore) artifactBlobs(ctx context.Context) ([]exportBlob, error) {
	rows, err := s.queries.ListArtifactBlobs(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]exportBlob, 0, len(rows))
	for _, row := range rows {
		items = append(items, exportBlob{ContentHash: row.ContentHash, ByteSize: row.ByteSize, Data: row.Data})
	}
	return items, nil
}
