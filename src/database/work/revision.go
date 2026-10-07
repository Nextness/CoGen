// revision.go provides the handwritten immutable work-revision data access
// layer over the generated statements. Producer-stage validation and payload
// hashing stay in Go around the generated insert.
package work

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	generated "analysis/database/work/internal/sql"
)

// CreateRevision inserts a new immutable work revision and returns its ID.
// The payload hash is computed from the supplied core fields and extension
// data. If FieldSchemaVersion is empty, it defaults to "1". ProducerStage must
// be a known pipeline stage; the legacy_unknown sentinel is rejected.
func (s *Store) CreateRevision(ctx context.Context, revision *Revision) (int64, error) {
	if revision == nil {
		return 0, fmt.Errorf("create work revision: value is required")
	}
	if revision.ProducerStage == "" {
		return 0, fmt.Errorf("create work revision: producer_stage is required")
	}
	if !validProducerStage(revision.ProducerStage) {
		return 0, fmt.Errorf("create work revision: invalid producer_stage %q", revision.ProducerStage)
	}
	if revision.FieldSchemaVersion == "" {
		revision.FieldSchemaVersion = "1"
	}
	hash := computeRevisionPayloadHash(revision)

	result, err := s.queries.InsertWorkRevision(ctx, generated.InsertWorkRevisionParams{
		WorkID:             revision.WorkID,
		PipelineRunID:      revision.PipelineRunID,
		ProducerStage:      revision.ProducerStage,
		FieldSchemaVersion: revision.FieldSchemaVersion,
		PayloadHash:        hash,
		Title:              nullableString(revision.Title),
		Abstract:           nullableString(revision.Abstract),
		Year:               nullableInt64(int64(revision.Year)),
		Journal:            nullableString(revision.Journal),
		Publisher:          nullableString(revision.Publisher),
		Source:             nullableString(revision.Source),
		Keywords:           nullableString(revision.Keywords),
		KeywordsPlus:       nullableString(revision.KeywordsPlus),
		CitationCount:      nullableInt64(int64(revision.CitationCount)),
		ReferenceCount:     nullableInt64(int64(revision.ReferenceCount)),
		ExtensionData:      nullableString(revision.ExtensionData),
	})
	if err != nil {
		return 0, fmt.Errorf("create work revision: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	revision.PayloadHash = hash
	return id, nil
}

// GetRevisionByID returns a work revision by its primary key, or nil if not found.
func (s *Store) GetRevisionByID(ctx context.Context, id int64) (*Revision, error) {
	row, err := s.queries.GetWorkRevisionByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return revisionFromGenerated(row), nil
}

// ListRevisionsByWorkID returns all revisions for a given work in
// chronological ID order.
func (s *Store) ListRevisionsByWorkID(ctx context.Context, workID int64) ([]*Revision, error) {
	rows, err := s.queries.ListWorkRevisionsByWorkID(ctx, workID)
	if err != nil {
		return nil, err
	}
	var revisions []*Revision
	for _, row := range rows {
		revisions = append(revisions, revisionFromGenerated(row))
	}
	return revisions, nil
}

// ListRevisionsByRunID returns all revisions created by a given pipeline run.
func (s *Store) ListRevisionsByRunID(ctx context.Context, runID int64) ([]*Revision, error) {
	rows, err := s.queries.ListWorkRevisionsByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	var revisions []*Revision
	for _, row := range rows {
		revisions = append(revisions, revisionFromGenerated(row))
	}
	return revisions, nil
}

// ListNormalizeRevisionsByRunID returns the normalize revisions created by a
// given pipeline run, in ID order.
func (s *Store) ListNormalizeRevisionsByRunID(ctx context.Context, runID int64) ([]*Revision, error) {
	rows, err := s.queries.ListNormalizeRevisionsByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	var revisions []*Revision
	for _, row := range rows {
		revisions = append(revisions, revisionFromGenerated(row))
	}
	return revisions, nil
}

// CountRevisionsByWorkID returns the number of revisions for a given work.
func (s *Store) CountRevisionsByWorkID(ctx context.Context, workID int64) (int, error) {
	count, err := s.queries.CountWorkRevisionsByWorkID(ctx, workID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetCurrentNormalizedRevisionWorkID returns the owning work of one current
// normalized revision. It returns sql.ErrNoRows when the revision is not the
// current valid normalize revision for its run and work.
func (s *Store) GetCurrentNormalizedRevisionWorkID(ctx context.Context, revisionID, runID int64) (int64, error) {
	return s.queries.GetCurrentNormalizedRevisionWorkID(ctx, generated.GetCurrentNormalizedRevisionWorkIDParams{
		ID:            revisionID,
		PipelineRunID: runID,
	})
}

// CountCurrentNormalizedRevisionsByWork counts the current normalized
// revisions for one work in one run.
func (s *Store) CountCurrentNormalizedRevisionsByWork(ctx context.Context, runID, workID int64) (int64, error) {
	return s.queries.CountCurrentNormalizedRevisionsByWork(ctx, generated.CountCurrentNormalizedRevisionsByWorkParams{
		PipelineRunID: runID,
		WorkID:        workID,
	})
}

// revisionFromGenerated maps one generated revision row into an application
// revision and makes every nullable core field explicit.
func revisionFromGenerated(row generated.WorkRevision) *Revision {
	revision := &Revision{
		ID:                 row.ID,
		WorkID:             row.WorkID,
		PipelineRunID:      row.PipelineRunID,
		ProducerStage:      row.ProducerStage,
		FieldSchemaVersion: row.FieldSchemaVersion,
		PayloadHash:        row.PayloadHash,
		CreatedAt:          row.CreatedAt,
	}
	if row.Title.Valid {
		revision.Title = row.Title.String
	}
	if row.Abstract.Valid {
		revision.Abstract = row.Abstract.String
	}
	if row.Year.Valid {
		revision.Year = int(row.Year.Int64)
	}
	if row.Journal.Valid {
		revision.Journal = row.Journal.String
	}
	if row.Publisher.Valid {
		revision.Publisher = row.Publisher.String
	}
	if row.Source.Valid {
		revision.Source = row.Source.String
	}
	if row.Keywords.Valid {
		revision.Keywords = row.Keywords.String
	}
	if row.KeywordsPlus.Valid {
		revision.KeywordsPlus = row.KeywordsPlus.String
	}
	if row.CitationCount.Valid {
		revision.CitationCount = int(row.CitationCount.Int64)
	}
	if row.ReferenceCount.Valid {
		revision.ReferenceCount = int(row.ReferenceCount.Int64)
	}
	if row.ExtensionData.Valid {
		revision.ExtensionData = row.ExtensionData.String
	}
	return revision
}

// Valid producer stages for new work revisions.
var validProducerStages = map[string]bool{
	ProducerStageParse:          true,
	ProducerStageDeduplicate:    true,
	ProducerStageValidate:       true,
	ProducerStageEnrich:         true,
	ProducerStageEnrichMetadata: true,
	ProducerStageEnrichIdentity: true,
	ProducerStageNormalize:      true,
}

// validProducerStage reports whether the supplied producer stage is supported.
func validProducerStage(stage string) bool {
	return validProducerStages[stage]
}

// computeRevisionPayloadHash computes a deterministic SHA-256 hex hash from
// the typed core fields and extension data of a revision. ProducerStage is
// excluded because it is provenance metadata, not content.
// FieldSchemaVersion is included because it affects how the fields should be
// interpreted.
func computeRevisionPayloadHash(revision *Revision) string {
	// Collect key-value pairs and sort by key for determinism.
	m := map[string]string{
		"title":                revision.Title,
		"abstract":             revision.Abstract,
		"year":                 fmt.Sprintf("%d", revision.Year),
		"journal":              revision.Journal,
		"publisher":            revision.Publisher,
		"source":               revision.Source,
		"keywords":             revision.Keywords,
		"keywords_plus":        revision.KeywordsPlus,
		"citation_count":       fmt.Sprintf("%d", revision.CitationCount),
		"reference_count":      fmt.Sprintf("%d", revision.ReferenceCount),
		"extension_data":       revision.ExtensionData,
		"field_schema_version": revision.FieldSchemaVersion,
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hash := sha256.New()
	for _, key := range keys {
		hash.Write([]byte(key))
		hash.Write([]byte{0})
		hash.Write([]byte(m[key]))
		hash.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
