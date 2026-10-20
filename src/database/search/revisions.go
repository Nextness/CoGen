// revisions.go provides the handwritten search-revision data access layer over
// the generated queries in this family's private internal/sql package.
package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/search/internal/sql"
)

// UpsertRevision inserts a new search revision or refreshes the
// latest-declaration hashes of an existing (search, label) pair. It returns
// the revision ID and whether its hashes were updated (false on first insert
// or identical hashes).
//
// This allows the same revision label to track the latest configuration for a
// search while immutable historical configuration stays on execution plans and
// attempts.
func (s *Store) UpsertRevision(ctx context.Context, input RevisionInput) (int64, bool, error) {
	result, err := s.queries.InsertRevisionIgnore(ctx, generated.InsertRevisionIgnoreParams{
		SearchID:             input.SearchID,
		RevisionLabel:        input.RevisionLabel,
		ConfigArtifactHash:   input.ConfigArtifactHash,
		ResolvedManifestHash: input.ResolvedManifestHash,
	})
	if err != nil {
		return 0, false, fmt.Errorf("create search revision: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("create search revision: read affected rows: %w", err)
	}
	if rowsAffected > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, false, fmt.Errorf("create search revision: read inserted id: %w", err)
		}
		return id, false, nil
	}

	existing, err := s.GetRevisionBySearchAndLabel(ctx, input.SearchID, input.RevisionLabel)
	if err != nil {
		return 0, false, err
	}
	if existing == nil {
		return 0, false, fmt.Errorf("create search revision: insert skipped but existing row not found")
	}
	if existing.ConfigArtifactHash == input.ConfigArtifactHash && existing.ResolvedManifestHash == input.ResolvedManifestHash {
		return existing.ID, false, nil
	}
	if err := s.queries.UpdateRevisionHashes(ctx, generated.UpdateRevisionHashesParams{
		ConfigArtifactHash:   input.ConfigArtifactHash,
		ResolvedManifestHash: input.ResolvedManifestHash,
		ID:                   existing.ID,
	}); err != nil {
		return 0, false, fmt.Errorf("update search revision hashes: %w", err)
	}
	return existing.ID, true, nil
}

// GetRevisionByID returns a search revision by its primary key, or nil if not found.
func (s *Store) GetRevisionByID(ctx context.Context, id int64) (*Revision, error) {
	row, err := s.queries.GetRevisionByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search revision: %w", err)
	}
	return revisionFromGenerated(row), nil
}

// GetRevisionBySearchAndLabel returns a revision for a given search and label,
// or nil if not found.
func (s *Store) GetRevisionBySearchAndLabel(ctx context.Context, searchID int64, revisionLabel string) (*Revision, error) {
	row, err := s.queries.GetRevisionBySearchAndLabel(ctx, generated.GetRevisionBySearchAndLabelParams{
		SearchID:      searchID,
		RevisionLabel: revisionLabel,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get search revision: %w", err)
	}
	return revisionFromGenerated(row), nil
}

// ListRevisionsBySearch returns all revisions for a search, ordered by ID. An
// absent search returns a nil slice to preserve the current API shape.
func (s *Store) ListRevisionsBySearch(ctx context.Context, searchID int64) ([]*Revision, error) {
	rows, err := s.queries.ListRevisionsBySearch(ctx, searchID)
	if err != nil {
		return nil, fmt.Errorf("list search revisions: %w", err)
	}
	var revisions []*Revision
	for _, row := range rows {
		revisions = append(revisions, revisionFromGenerated(row))
	}
	return revisions, nil
}

// revisionFromGenerated maps one generated revision row into an application
// revision and makes the nullable updated_at zero value explicit.
func revisionFromGenerated(row generated.SearchRevision) *Revision {
	return &Revision{
		ID:                   row.ID,
		SearchID:             row.SearchID,
		RevisionLabel:        row.RevisionLabel,
		ConfigArtifactHash:   row.ConfigArtifactHash,
		ResolvedManifestHash: row.ResolvedManifestHash,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt.String,
	}
}
