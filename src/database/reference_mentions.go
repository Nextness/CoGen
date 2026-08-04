// reference_mentions.go provides the repository for cited-reference
// mentions on immutable work revisions, linking each mention to its
// resolved work in the same corpus when available.
package database

import (
	"database/sql"
	"fmt"
)

// ReferenceMention is one cited reference observed on an immutable work
// revision. ResolvedWorkID is optional because most citations are external to
// the current workspace.
type ReferenceMention struct {
	ID             int64  `json:"id"`
	WorkRevisionID int64  `json:"work_revision_id"`
	ResolvedWorkID int64  `json:"resolved_work_id"`
	MentionOrder   int    `json:"mention_order"`
	RawReference   string `json:"raw_reference"`
	DOI            string `json:"doi"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	Year           int    `json:"year"`
	Source         string `json:"source"`
	CreatedAt      string `json:"created_at"`
}

// ReferenceMentionRepository persists immutable reference mentions.
type ReferenceMentionRepository struct {
	db *Database
}

// Create stores one ordered reference mention. A DOI is normalized and, when
// it identifies an existing work, linked through ResolvedWorkID automatically.
func (r *ReferenceMentionRepository) Create(mention *ReferenceMention) (int64, error) {
	if mention.WorkRevisionID == 0 {
		return 0, fmt.Errorf("create reference mention: work_revision_id is required")
	}
	if mention.MentionOrder < 1 {
		return 0, fmt.Errorf("create reference mention: mention_order must be positive")
	}

	mention.DOI = NormalizeDOI(mention.DOI)
	resolvedWorkID := mention.ResolvedWorkID
	if resolvedWorkID == 0 && mention.DOI != "" {
		work, err := r.db.Works.GetByDOI(mention.DOI)
		if err != nil {
			return 0, fmt.Errorf("create reference mention: resolve DOI: %w", err)
		}
		if work != nil {
			resolvedWorkID = work.ID
			mention.ResolvedWorkID = work.ID
		}
	}

	res, err := r.db.DB.Exec(`
		INSERT INTO reference_mentions
			(work_revision_id, resolved_work_id, mention_order, raw_reference, doi, title, author, year, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mention.WorkRevisionID, nullInt(resolvedWorkID), mention.MentionOrder,
		nullStr(mention.RawReference), nullStr(mention.DOI), nullStr(mention.Title),
		nullStr(mention.Author), nullInt(int64(mention.Year)), nullStr(mention.Source),
	)
	if err != nil {
		return 0, fmt.Errorf("create reference mention: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetByID returns a mention by primary key, or nil if it does not exist.
func (r *ReferenceMentionRepository) GetByID(id int64) (*ReferenceMention, error) {
	return scanReferenceMention(r.db.DB.QueryRow(`
		SELECT id, work_revision_id, resolved_work_id, mention_order, raw_reference, doi, title, author, year, source, created_at
		FROM reference_mentions WHERE id = ?`, id))
}

// GetByRevisionID returns a revision's references in their source order.
func (r *ReferenceMentionRepository) GetByRevisionID(revisionID int64) ([]*ReferenceMention, error) {
	rows, err := r.db.DB.Query(`
		SELECT id, work_revision_id, resolved_work_id, mention_order, raw_reference, doi, title, author, year, source, created_at
		FROM reference_mentions WHERE work_revision_id = ? ORDER BY mention_order, id`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mentions := make([]*ReferenceMention, 0)
	for rows.Next() {
		mention, err := scanReferenceMention(rows)
		if err != nil {
			return nil, err
		}
		mentions = append(mentions, mention)
	}
	return mentions, rows.Err()
}

// GetByResolvedWorkID returns workspace citations that resolve to one work.
func (r *ReferenceMentionRepository) GetByResolvedWorkID(workID int64) ([]*ReferenceMention, error) {
	rows, err := r.db.DB.Query(`
		SELECT id, work_revision_id, resolved_work_id, mention_order, raw_reference, doi, title, author, year, source, created_at
		FROM reference_mentions WHERE resolved_work_id = ? ORDER BY id`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mentions := make([]*ReferenceMention, 0)
	for rows.Next() {
		mention, err := scanReferenceMention(rows)
		if err != nil {
			return nil, err
		}
		mentions = append(mentions, mention)
	}
	return mentions, rows.Err()
}

// scanReferenceMention decodes reference mention from a database row.
func scanReferenceMention(row scannable) (*ReferenceMention, error) {
	var mention ReferenceMention
	var resolvedWorkID, year sql.NullInt64
	var raw, doi, title, author, source sql.NullString
	if err := row.Scan(&mention.ID, &mention.WorkRevisionID, &resolvedWorkID,
		&mention.MentionOrder, &raw, &doi, &title, &author, &year, &source, &mention.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if resolvedWorkID.Valid {
		mention.ResolvedWorkID = resolvedWorkID.Int64
	}
	if raw.Valid {
		mention.RawReference = raw.String
	}
	if doi.Valid {
		mention.DOI = doi.String
	}
	if title.Valid {
		mention.Title = title.String
	}
	if author.Valid {
		mention.Author = author.String
	}
	if year.Valid {
		mention.Year = int(year.Int64)
	}
	if source.Valid {
		mention.Source = source.String
	}
	return &mention, nil
}
