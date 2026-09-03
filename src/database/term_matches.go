// term_matches.go provides the repository for per-run search-term inventories
// and per-revision field matches derived from recorded source queries.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// RunSearchTerm is one stored search term and the source that declared it.
type RunSearchTerm struct {
	ID            int64  `json:"id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	SourceName    string `json:"source_name"`
	Term          string `json:"term"`
	CreatedAt     string `json:"created_at"`
}

// TermMatchesRepository provides replace and read access to the derived
// run_search_terms and work_revision_term_matches tables.
type TermMatchesRepository struct {
	db *Database
}

// ReplaceRunTermData replaces both the term inventory and the revision matches
// for one run in a single transaction, preserving per-run atomicity.
func (r *TermMatchesRepository) ReplaceRunTermData(runID int64, termsBySource map[string][]string, matches map[int64]map[string][]string) error {
	return r.db.withTx(context.Background(), func(tx *sql.Tx) error {
		if err := r.replaceRunTermsTx(tx, runID, termsBySource); err != nil {
			return err
		}
		if err := r.replaceRunMatchesTx(tx, runID, matches); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO run_term_match_reconciliations (pipeline_run_id)
			VALUES (?) ON CONFLICT(pipeline_run_id) DO UPDATE SET reconciled_at=datetime('now')`, runID); err != nil {
			return fmt.Errorf("record term reconciliation: %w", err)
		}
		return nil
	})
}

// HasRunTermData reports whether term reconciliation completed, including valid empty results.
func (r *TermMatchesRepository) HasRunTermData(runID int64) (bool, error) {
	var found bool
	if err := r.db.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM run_term_match_reconciliations WHERE pipeline_run_id = ?)", runID,
	).Scan(&found); err != nil {
		return false, fmt.Errorf("check run term reconciliation: %w", err)
	}
	return found, nil
}

// GetRunTerms returns the stored term inventory for one run ordered by id.
func (r *TermMatchesRepository) GetRunTerms(runID int64) ([]RunSearchTerm, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, pipeline_run_id, source_name, term, created_at
		 FROM run_search_terms WHERE pipeline_run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list run search terms: %w", err)
	}
	defer rows.Close()
	var result []RunSearchTerm
	for rows.Next() {
		var item RunSearchTerm
		if err := rows.Scan(&item.ID, &item.PipelineRunID, &item.SourceName, &item.Term, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan run search term: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run search terms: %w", err)
	}
	return result, nil
}

// GetRevisionMatches returns the per-field matched terms for one revision.
func (r *TermMatchesRepository) GetRevisionMatches(runID, revisionID int64) (map[string][]string, error) {
	rows, err := r.db.DB.Query(
		`SELECT field, term FROM work_revision_term_matches
		 WHERE pipeline_run_id = ? AND work_revision_id = ? ORDER BY id`, runID, revisionID)
	if err != nil {
		return nil, fmt.Errorf("list revision term matches: %w", err)
	}
	defer rows.Close()
	return scanMatches(rows)
}

// GetRevisionMatchesBulk returns per-field matched terms for a page of
// revisions. It short-circuits when the revision list is empty.
func (r *TermMatchesRepository) GetRevisionMatchesBulk(runID int64, revisionIDs []int64) (map[int64]map[string][]string, error) {
	if len(revisionIDs) == 0 {
		return map[int64]map[string][]string{}, nil
	}
	placeholders := strings.Repeat("?,", len(revisionIDs)-1) + "?"
	args := make([]any, 0, len(revisionIDs)+1)
	args = append(args, runID)
	for _, id := range revisionIDs {
		args = append(args, id)
	}
	rows, err := r.db.DB.Query(
		`SELECT work_revision_id, field, term FROM work_revision_term_matches
		 WHERE pipeline_run_id = ? AND work_revision_id IN (`+placeholders+`) ORDER BY id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list revision term matches bulk: %w", err)
	}
	defer rows.Close()
	result := make(map[int64]map[string][]string)
	for rows.Next() {
		var revisionID int64
		var field, term string
		if err := rows.Scan(&revisionID, &field, &term); err != nil {
			return nil, fmt.Errorf("scan revision term match: %w", err)
		}
		if result[revisionID] == nil {
			result[revisionID] = map[string][]string{}
		}
		result[revisionID][field] = append(result[revisionID][field], term)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revision term matches: %w", err)
	}
	return result, nil
}

// replaceRunTermsTx deletes and reinserts the term inventory for one run.
func (r *TermMatchesRepository) replaceRunTermsTx(tx *sql.Tx, runID int64, termsBySource map[string][]string) error {
	if _, err := tx.Exec("DELETE FROM run_search_terms WHERE pipeline_run_id = ?", runID); err != nil {
		return fmt.Errorf("clear run search terms: %w", err)
	}
	sources := make([]string, 0, len(termsBySource))
	for source := range termsBySource {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	for _, source := range sources {
		for _, term := range termsBySource[source] {
			if _, err := tx.Exec(
				"INSERT OR IGNORE INTO run_search_terms (pipeline_run_id, source_name, term) VALUES (?, ?, ?)",
				runID, source, term); err != nil {
				return fmt.Errorf("insert run search term: %w", err)
			}
		}
	}
	return nil
}

// replaceRunMatchesTx deletes and reinserts the revision matches for one run.
func (r *TermMatchesRepository) replaceRunMatchesTx(tx *sql.Tx, runID int64, matches map[int64]map[string][]string) error {
	if _, err := tx.Exec("DELETE FROM work_revision_term_matches WHERE pipeline_run_id = ?", runID); err != nil {
		return fmt.Errorf("clear revision term matches: %w", err)
	}
	revisionIDs := make([]int64, 0, len(matches))
	for revisionID := range matches {
		revisionIDs = append(revisionIDs, revisionID)
	}
	sort.Slice(revisionIDs, func(i, j int) bool { return revisionIDs[i] < revisionIDs[j] })
	for _, revisionID := range revisionIDs {
		fields := matches[revisionID]
		fieldNames := make([]string, 0, len(fields))
		for field := range fields {
			fieldNames = append(fieldNames, field)
		}
		sort.Strings(fieldNames)
		for _, field := range fieldNames {
			terms := append([]string(nil), fields[field]...)
			sort.Strings(terms)
			for _, term := range terms {
				if _, err := tx.Exec(
					"INSERT OR IGNORE INTO work_revision_term_matches (pipeline_run_id, work_revision_id, field, term) VALUES (?, ?, ?, ?)",
					runID, revisionID, field, term); err != nil {
					return fmt.Errorf("insert revision term match: %w", err)
				}
			}
		}
	}
	return nil
}

// scanMatches groups (field, term) rows into per-field term lists.
func scanMatches(rows *sql.Rows) (map[string][]string, error) {
	result := map[string][]string{}
	for rows.Next() {
		var field, term string
		if err := rows.Scan(&field, &term); err != nil {
			return nil, fmt.Errorf("scan term match: %w", err)
		}
		result[field] = append(result[field], term)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate term matches: %w", err)
	}
	return result, nil
}
