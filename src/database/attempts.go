// attempts.go provides the repository for content-addressed artifacts and
// their inline bytes that track the per-attempt lifecycle of workspace
// iterations.
package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
)

// Artifact is a content-addressed immutable payload.
type Artifact struct {
	ID          int64  `json:"id"`
	ContentHash string `json:"content_hash"`
	ByteSize    int64  `json:"byte_size"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
}

// ArtifactRepository provides CRUD for the artifacts table.
type ArtifactRepository struct {
	db *Database
}

// Create inserts a new artifact. Returns the artifact ID.
// If the content_hash already exists, returns the existing artifact ID.
func (r *ArtifactRepository) Create(contentHash, contentType string, byteSize int64) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO artifacts (content_hash, byte_size, content_type)
		 VALUES (?, ?, ?)`,
		contentHash, byteSize, contentType,
	)
	if err != nil {
		lg.Debug("artifact creation failed", "hash", contentHash, "error", err)
		return 0, fmt.Errorf("create artifact: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("artifact creation result read failed", "hash", contentHash, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("artifact inserted ID read failed", "hash", contentHash, "error", err)
			return 0, err
		}
		lg.Debug("artifact creation successful", "hash", contentHash, "id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - return existing ID
	existing, err := r.GetByHash(contentHash)
	if err != nil {
		lg.Debug("artifact existing lookup failed", "hash", contentHash, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("artifact creation failed", "hash", contentHash, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create artifact: insert skipped but existing row not found")
	}
	if existing.ByteSize != byteSize || existing.ContentType != contentType {
		return 0, fmt.Errorf("create artifact: content hash %q conflicts with stored metadata", contentHash)
	}
	lg.Debug("artifact creation successful", "hash", contentHash, "id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// CreateWithBlob atomically records an artifact and its bytes for a pipeline run.
func (r *ArtifactRepository) CreateWithBlob(contentHash, contentType string, byteSize, pipelineRunID int64, data []byte) (int64, error) {
	expectedSize := byteSize
	expectedType := contentType
	var artifactID int64
	err := r.db.withTx(context.Background(), func(tx *sql.Tx) error {
		result, err := tx.Exec(`INSERT OR IGNORE INTO artifacts (content_hash, byte_size, content_type) VALUES (?, ?, ?)`, contentHash, byteSize, contentType)
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
			var existingSize int64
			var existingType string
			if err := tx.QueryRow(`SELECT id, byte_size, content_type FROM artifacts WHERE content_hash=?`, contentHash).Scan(&artifactID, &existingSize, &existingType); err != nil {
				return fmt.Errorf("read existing artifact: %w", err)
			}
			if existingSize != expectedSize || existingType != expectedType {
				return fmt.Errorf("create artifact: content hash %q conflicts with stored metadata", contentHash)
			}
		}
		var storedSize int64
		var storedType string
		if err := tx.QueryRow(`SELECT byte_size, content_type FROM artifacts WHERE id=?`, artifactID).Scan(&storedSize, &storedType); err != nil {
			return fmt.Errorf("read stored artifact metadata: %w", err)
		}
		if storedSize != expectedSize || storedType != expectedType || expectedSize != int64(len(data)) {
			return fmt.Errorf("create artifact: content hash %q conflicts with stored metadata", contentHash)
		}
		result, err = tx.Exec(`INSERT OR IGNORE INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, ?)`, artifactID, pipelineRunID, data)
		if err != nil {
			return fmt.Errorf("create artifact blob: %w", err)
		}
		inserted, err = result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read artifact blob insert result: %w", err)
		}
		if inserted == 0 {
			var existing []byte
			if err := tx.QueryRow(`SELECT data FROM artifact_blobs WHERE artifact_id=?`, artifactID).Scan(&existing); err != nil {
				return fmt.Errorf("read existing artifact blob: %w", err)
			}
			if !bytes.Equal(existing, data) {
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
func (r *ArtifactRepository) GetByHash(contentHash string) (*Artifact, error) {
	var a Artifact
	err := r.db.DB.QueryRow(
		`SELECT id, content_hash, byte_size, content_type, created_at
		 FROM artifacts WHERE content_hash = ?`, contentHash,
	).Scan(&a.ID, &a.ContentHash, &a.ByteSize, &a.ContentType, &a.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("artifact query successful", "hash", contentHash, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("artifact query failed", "hash", contentHash, "error", err)
		return nil, err
	}
	lg.Debug("artifact query successful", "hash", contentHash, "id", a.ID, "result", "found")
	return &a, nil
}

// GetByID returns an artifact by its primary key, or nil if not found.
func (r *ArtifactRepository) GetByID(id int64) (*Artifact, error) {
	var a Artifact
	err := r.db.DB.QueryRow(
		`SELECT id, content_hash, byte_size, content_type, created_at
		 FROM artifacts WHERE id = ?`, id,
	).Scan(&a.ID, &a.ContentHash, &a.ByteSize, &a.ContentType, &a.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("artifact query successful", "id", id, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("artifact query failed", "id", id, "error", err)
		return nil, err
	}
	lg.Debug("artifact query successful", "id", id, "hash", a.ContentHash, "result", "found")
	return &a, nil
}

// ArtifactBlob stores the raw bytes for an artifact inline in the database.
type ArtifactBlob struct {
	ID            int64  `json:"id"`
	ArtifactID    int64  `json:"artifact_id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	Data          []byte `json:"-"`
	CreatedAt     string `json:"created_at"`
}

// ArtifactBlobRepository provides CRUD for the artifact_blobs table.
type ArtifactBlobRepository struct {
	db *Database
}

// Create inserts a new artifact blob. Returns the blob ID.
// If the artifact_id already exists, returns the existing blob ID (deduplicated).
func (r *ArtifactBlobRepository) Create(artifactID, pipelineRunID int64, data []byte) (int64, error) {
	res, err := r.db.DB.Exec(
		`INSERT OR IGNORE INTO artifact_blobs (artifact_id, pipeline_run_id, data)
		 VALUES (?, ?, ?)`,
		artifactID, pipelineRunID, data,
	)
	if err != nil {
		lg.Debug("artifact blob creation failed", "artifact_id", artifactID, "pipeline_run_id", pipelineRunID, "error", err)
		return 0, fmt.Errorf("create artifact blob: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		lg.Debug("artifact blob creation result read failed", "artifact_id", artifactID, "error", err)
		return 0, err
	}
	if rowsAffected > 0 {
		id, err := res.LastInsertId()
		if err != nil {
			lg.Debug("artifact blob inserted ID read failed", "artifact_id", artifactID, "error", err)
			return 0, err
		}
		lg.Debug("artifact blob creation successful", "artifact_id", artifactID, "id", id, "result", "inserted")
		return id, nil
	}

	// Already exists - return existing ID
	existing, err := r.GetByArtifactID(artifactID)
	if err != nil {
		lg.Debug("artifact blob existing lookup failed", "artifact_id", artifactID, "error", err)
		return 0, err
	}
	if existing == nil {
		lg.Debug("artifact blob creation failed", "artifact_id", artifactID, "reason", "insert_skipped_but_not_found")
		return 0, fmt.Errorf("create artifact blob: insert skipped but existing row not found")
	}
	if !bytes.Equal(existing.Data, data) {
		return 0, fmt.Errorf("create artifact blob: artifact %d conflicts with stored bytes", artifactID)
	}
	lg.Debug("artifact blob creation successful",
		"artifact_id", artifactID, "id", existing.ID, "result", "already_existing")
	return existing.ID, nil
}

// GetByArtifactID returns the blob for a given artifact, or nil if not found.
func (r *ArtifactBlobRepository) GetByArtifactID(artifactID int64) (*ArtifactBlob, error) {
	var b ArtifactBlob
	err := r.db.DB.QueryRow(
		`SELECT id, artifact_id, pipeline_run_id, data, created_at
		 FROM artifact_blobs WHERE artifact_id = ?`, artifactID,
	).Scan(&b.ID, &b.ArtifactID, &b.PipelineRunID, &b.Data, &b.CreatedAt)
	if err == sql.ErrNoRows {
		lg.Debug("artifact blob query successful", "artifact_id", artifactID, "result", "not_found")
		return nil, nil
	}
	if err != nil {
		lg.Debug("artifact blob query failed", "artifact_id", artifactID, "error", err)
		return nil, err
	}
	lg.Debug("artifact blob query successful", "artifact_id", artifactID, "id", b.ID, "result", "found")
	return &b, nil
}
