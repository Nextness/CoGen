// artifact.go provides the handwritten content-identity data access layer over
// the generated queries in this family's private internal/sql package. It owns
// content-addressed creation, atomic artifact-plus-blob creation, and identity
// lookup; blobs, run links, and relationship reads live in their specifically
// named files.
package artifact

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/artifact/internal/sql"
)

// Store binds the generated artifact queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns an artifact family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// Create inserts a new artifact. Returns the artifact ID.
// If the content_hash already exists, returns the existing artifact ID.
func (s *Store) Create(ctx context.Context, input CreateInput) (int64, error) {
	result, err := s.queries.InsertArtifact(ctx, generated.InsertArtifactParams{
		ContentHash: input.ContentHash,
		ByteSize:    input.ByteSize,
		ContentType: input.ContentType,
	})
	if err != nil {
		return 0, fmt.Errorf("create artifact: %w", err)
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

	// Already exists - return existing ID after verifying compatible metadata.
	existing, err := s.GetByHash(ctx, input.ContentHash)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		return 0, fmt.Errorf("create artifact: insert skipped but existing row not found")
	}
	if existing.ByteSize != input.ByteSize || existing.ContentType != input.ContentType {
		return 0, fmt.Errorf("create artifact: content hash %q conflicts with stored metadata", input.ContentHash)
	}
	return existing.ID, nil
}

// CreateWithBlob atomically records an artifact and its bytes for a pipeline
// run. An existing content identity is reused only when its stored metadata and
// bytes match; every partial write rolls back on mismatch.
func (s *Store) CreateWithBlob(ctx context.Context, input CreateWithBlobInput) (int64, error) {
	expectedSize := input.ByteSize
	expectedType := input.ContentType
	var artifactID int64
	err := s.withTx(ctx, func(queries *generated.Queries) error {
		result, err := queries.InsertArtifact(ctx, generated.InsertArtifactParams{
			ContentHash: input.ContentHash,
			ByteSize:    input.ByteSize,
			ContentType: input.ContentType,
		})
		if err != nil {
			return fmt.Errorf("create artifact: %w", err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read artifact insert result: %w", err)
		}
		if inserted > 0 {
			artifactID, err = result.LastInsertId()
			if err != nil {
				return fmt.Errorf("read artifact ID: %w", err)
			}
		} else {
			existing, err := queries.GetArtifactMetadataByHash(ctx, input.ContentHash)
			if err != nil {
				return fmt.Errorf("read existing artifact: %w", err)
			}
			artifactID = existing.ID
			if existing.ByteSize != expectedSize || existing.ContentType != expectedType {
				return fmt.Errorf("create artifact: content hash %q conflicts with stored metadata", input.ContentHash)
			}
		}
		stored, err := queries.GetArtifactMetadataByID(ctx, artifactID)
		if err != nil {
			return fmt.Errorf("read stored artifact metadata: %w", err)
		}
		if stored.ByteSize != expectedSize || stored.ContentType != expectedType || expectedSize != int64(len(input.Data)) {
			return fmt.Errorf("create artifact: content hash %q conflicts with stored metadata", input.ContentHash)
		}
		result, err = queries.InsertArtifactBlob(ctx, generated.InsertArtifactBlobParams{
			ArtifactID:    artifactID,
			PipelineRunID: input.PipelineRunID,
			Data:          input.Data,
		})
		if err != nil {
			return fmt.Errorf("create artifact blob: %w", err)
		}
		inserted, err = result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read artifact blob insert result: %w", err)
		}
		if inserted == 0 {
			existing, err := queries.GetArtifactBlobDataByArtifactID(ctx, artifactID)
			if err != nil {
				return fmt.Errorf("read existing artifact blob: %w", err)
			}
			if !bytes.Equal(existing, input.Data) {
				return fmt.Errorf("create artifact blob: artifact %d conflicts with stored bytes", artifactID)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return artifactID, nil
}

// GetByHash returns an artifact by its content hash, or nil if not found.
func (s *Store) GetByHash(ctx context.Context, contentHash string) (*Artifact, error) {
	row, err := s.queries.GetArtifactByHash(ctx, contentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return artifactFromGenerated(row), nil
}

// GetByID returns an artifact by its primary key, or nil if not found.
func (s *Store) GetByID(ctx context.Context, id int64) (*Artifact, error) {
	row, err := s.queries.GetArtifactByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return artifactFromGenerated(row), nil
}

// artifactFromGenerated maps one generated artifact row into an application artifact.
func artifactFromGenerated(row generated.Artifact) *Artifact {
	return &Artifact{
		ID:          row.ID,
		ContentHash: row.ContentHash,
		ByteSize:    row.ByteSize,
		ContentType: row.ContentType,
		CreatedAt:   row.CreatedAt,
	}
}

// withTx runs fn inside one transaction bound to the generated queries and
// commits only when every statement succeeds.
func (s *Store) withTx(ctx context.Context, fn func(*generated.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		// Rollback on panic; if fn returned an error the tx is already rolled back.
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
