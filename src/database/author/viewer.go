// viewer.go provides the author family's run-scoped identity evidence reads
// over the family connection and the generated candidate statements. It owns
// the filtered evidence page, the bounded candidate previews, and the ranked
// candidate cursor page.
package author

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	generated "analysis/database/author/internal/sql"
)

// maxIdentityCandidateLimit bounds one viewer candidate page.
const maxIdentityCandidateLimit = 1000

// IdentityEvidenceFilter identifies one bounded, searchable page of run-scoped
// identity resolutions with their current work evidence.
type IdentityEvidenceFilter struct {
	RunID   int64
	Query   string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// IdentityEvidenceRow is one run-scoped identity resolution with the current
// normalized revision of its evidence work. Optional fields stay nil when the
// underlying column is NULL.
type IdentityEvidenceRow struct {
	ResolutionID        int64
	Status              string
	Provider            string
	QueriedCitationName string
	ErrorMessage        *string
	ResolvedAt          string
	AuthorOccurrenceID  int64
	ObservedORCID       *string
	PersonID            *int64
	ArticleTitle        *string
	DOI                 *string
	WorkRevisionID      *int64
	EvidenceRevisionID  *int64
	EvidenceStage       *string
	CandidateCount      int64
}

// IdentityEvidencePage is one bounded page of run-scoped identity evidence.
type IdentityEvidencePage struct {
	Items []*IdentityEvidenceRow
	Total int64
}

// IdentityCandidateFilter identifies one ranked candidate cursor page for a
// run-owned resolution.
type IdentityCandidateFilter struct {
	ResolutionID int64
	CursorRank   int64
	CursorID     int64
	Limit        int
}

// IdentityCandidateRow is one ranked provider candidate in the viewer cursor
// page. Optional fields stay nil when the underlying column is NULL.
type IdentityCandidateRow struct {
	ID                  int64
	CandidateORCID      string
	ProviderDisplayName *string
	QueryURL            string
	PayloadArtifactID   *int64
	ProviderRank        *int64
	CreatedAt           string
}

// IdentityCandidatePreview is one ranked candidate preview attached to a
// visible resolution.
type IdentityCandidatePreview struct {
	ResolutionID        int64
	ID                  int64
	CandidateORCID      string
	ProviderDisplayName *string
	QueryURL            string
	PayloadArtifactID   *int64
	ProviderRank        *int64
	CreatedAt           string
}

// ListIdentityEvidence returns one bounded page of run-scoped identity
// resolutions with the current normalized revision of their evidence work.
func (s *Store) ListIdentityEvidence(ctx context.Context, filter IdentityEvidenceFilter) (*IdentityEvidencePage, error) {
	if _, ok := identityEvidenceSortFields[filter.Sort]; !ok {
		return nil, fmt.Errorf("list identity evidence: unsupported sort field %q", filter.Sort)
	}
	from, where, args := identityEvidenceQuery(filter)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT r.id "+from+" WHERE "+where+" GROUP BY r.id, evidence.work_id)", args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count identity evidence: %w", err)
	}
	page := clampPage(filter.Page, filter.PerPage, total)
	orderSQL := identityEvidenceSortFields[filter.Sort] + " " + sqlDirection(filter.Order)
	if identityEvidenceSortFields[filter.Sort] != "r.id" {
		orderSQL += ", r.id " + sqlDirection(filter.Order)
	}
	orderSQL += ", evidence.work_id " + sqlDirection(filter.Order)
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, identityEvidenceSelect+" "+from+" WHERE "+where+" GROUP BY r.id, evidence.work_id ORDER BY "+orderSQL+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list identity evidence: %w", err)
	}
	defer rows.Close()
	items := make([]*IdentityEvidenceRow, 0)
	for rows.Next() {
		var item IdentityEvidenceRow
		var errorMessage, observedORCID, articleTitle, doi, evidenceStage sql.NullString
		var personID, workRevisionID, evidenceRevisionID sql.NullInt64
		if err := rows.Scan(
			&item.ResolutionID, &item.Status, &item.Provider, &item.QueriedCitationName,
			&errorMessage, &item.ResolvedAt, &item.AuthorOccurrenceID, &observedORCID,
			&personID, &articleTitle, &doi, &workRevisionID, &evidenceRevisionID,
			&evidenceStage, &item.CandidateCount,
		); err != nil {
			return nil, fmt.Errorf("list identity evidence: %w", err)
		}
		item.ErrorMessage = nullableStringPointer(errorMessage)
		item.ObservedORCID = nullableStringPointer(observedORCID)
		item.PersonID = nullableInt64Pointer(personID)
		item.ArticleTitle = nullableStringPointer(articleTitle)
		item.DOI = nullableStringPointer(doi)
		item.WorkRevisionID = nullableInt64Pointer(workRevisionID)
		item.EvidenceRevisionID = nullableInt64Pointer(evidenceRevisionID)
		item.EvidenceStage = nullableStringPointer(evidenceStage)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list identity evidence: %w", err)
	}
	return &IdentityEvidencePage{Items: items, Total: total}, nil
}

// ListCandidatePreviews returns a bounded ranked preview for every requested
// resolution, ordered by resolution, rank, and ID. An empty request returns no
// rows without querying.
func (s *Store) ListCandidatePreviews(ctx context.Context, resolutionIDs []int64, limit int) ([]*IdentityCandidatePreview, error) {
	if len(resolutionIDs) == 0 || limit < 1 {
		return nil, nil
	}
	encoded, err := json.Marshal(resolutionIDs)
	if err != nil {
		return nil, fmt.Errorf("list identity candidate previews: encode resolutions: %w", err)
	}
	rows, err := s.queries.ListCandidatePreviews(ctx, generated.ListCandidatePreviewsParams{
		ResolutionIdsJson: string(encoded),
		Limit:             int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list identity candidate previews: %w", err)
	}
	items := make([]*IdentityCandidatePreview, 0, len(rows))
	for _, row := range rows {
		item := &IdentityCandidatePreview{
			ResolutionID:   row.IdentityResolutionID,
			ID:             row.ID,
			CandidateORCID: row.CandidateOrcid,
			QueryURL:       row.QueryUrl,
			CreatedAt:      row.CreatedAt,
		}
		item.ProviderDisplayName = nullableStringPointer(row.ProviderDisplayName)
		item.PayloadArtifactID = nullableInt64Pointer(row.PayloadArtifactID)
		rank := row.ProviderRank
		item.ProviderRank = &rank
		items = append(items, item)
	}
	return items, nil
}

// ListIdentityCandidates returns up to Limit+1 ranked candidates after an
// optional rank and ID cursor, so the caller can detect a further page.
func (s *Store) ListIdentityCandidates(ctx context.Context, filter IdentityCandidateFilter) ([]*IdentityCandidateRow, error) {
	if filter.Limit < 1 || filter.Limit > maxIdentityCandidateLimit {
		return nil, fmt.Errorf("list identity candidates: limit must be between 1 and %d", maxIdentityCandidateLimit)
	}
	rows, err := s.queries.ListIdentityCandidates(ctx, generated.ListIdentityCandidatesParams{
		ResolutionID: filter.ResolutionID,
		CursorRank:   filter.CursorRank,
		CursorID:     filter.CursorID,
		Limit:        int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list identity candidates: %w", err)
	}
	items := make([]*IdentityCandidateRow, 0, len(rows))
	for _, row := range rows {
		item := &IdentityCandidateRow{
			ID:             row.ID,
			CandidateORCID: row.CandidateOrcid,
			QueryURL:       row.QueryUrl,
			CreatedAt:      row.CreatedAt,
		}
		item.ProviderDisplayName = nullableStringPointer(row.ProviderDisplayName)
		item.PayloadArtifactID = nullableInt64Pointer(row.PayloadArtifactID)
		rank := row.ProviderRank
		item.ProviderRank = &rank
		items = append(items, item)
	}
	return items, nil
}

// AuthorIdentityEvidenceFilter identifies one bounded page of identity
// evidence for one author occurrence in a run.
type AuthorIdentityEvidenceFilter struct {
	RunID              int64
	AuthorOccurrenceID int64
	CursorID           int64
	Limit              int
}

// AuthorIdentityEvidenceRow is one run-scoped resolution visible for an author
// occurrence, with the same-name evidence membership rule.
type AuthorIdentityEvidenceRow struct {
	ResolutionID        int64
	ID                  int64
	PipelineRunID       int64
	Status              string
	Provider            string
	QueriedCitationName string
	ErrorMessage        *string
	ResolvedAt          string
	CandidateCount      int64
}

// AuthorIdentityEvidencePage is one bounded page of author identity evidence.
type AuthorIdentityEvidencePage struct {
	Items        []*AuthorIdentityEvidenceRow
	Total        int64
	HasMore      bool
	NextCursorID int64
}

// ListAuthorIdentityEvidence returns one bounded page of run-scoped identity
// evidence for one author occurrence, ordered by descending resolution ID.
func (s *Store) ListAuthorIdentityEvidence(ctx context.Context, filter AuthorIdentityEvidenceFilter) (*AuthorIdentityEvidencePage, error) {
	if filter.Limit < 1 || filter.Limit > maxIdentityCandidateLimit {
		return nil, fmt.Errorf("list author identity evidence: limit must be between 1 and %d", maxIdentityCandidateLimit)
	}
	total, err := s.queries.CountAuthorIdentityEvidence(ctx, generated.CountAuthorIdentityEvidenceParams{
		PipelineRunID:      filter.RunID,
		AuthorOccurrenceID: filter.AuthorOccurrenceID,
	})
	if err != nil {
		return nil, fmt.Errorf("count author identity evidence: %w", err)
	}
	rows, err := s.queries.ListAuthorIdentityEvidence(ctx, generated.ListAuthorIdentityEvidenceParams{
		PipelineRunID:      filter.RunID,
		AuthorOccurrenceID: filter.AuthorOccurrenceID,
		CursorID:           filter.CursorID,
		RowLimit:           int64(filter.Limit + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("list author identity evidence: %w", err)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items := make([]*AuthorIdentityEvidenceRow, 0, len(rows))
	for _, row := range rows {
		item := &AuthorIdentityEvidenceRow{
			ResolutionID:        row.ResolutionID,
			ID:                  row.ID,
			PipelineRunID:       row.PipelineRunID,
			Status:              row.Status,
			Provider:            row.Provider,
			QueriedCitationName: row.QueriedCitationName,
			ResolvedAt:          row.ResolvedAt,
			CandidateCount:      row.CandidateCount,
		}
		item.ErrorMessage = nullableStringPointer(row.ErrorMessage)
		items = append(items, item)
	}
	var nextCursorID int64
	if hasMore && len(items) > 0 {
		nextCursorID = items[len(items)-1].ID
	}
	return &AuthorIdentityEvidencePage{Items: items, Total: total, HasMore: hasMore, NextCursorID: nextCursorID}, nil
}

// clampPage maps an offset request past the end to the final populated page.
func clampPage(page, perPage int, total int64) int {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		return 1
	}
	if int64(page) > totalPages {
		return int(totalPages)
	}
	return page
}
