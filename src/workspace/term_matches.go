// term_matches.go computes and persists per-run search-term inventories and
// per-revision field matches derived from recorded source queries.
package workspace

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"analysis/article"
	"analysis/database"
	"analysis/searchterms"
)

// computeRunTermMatches derives the per-source term inventory and per-revision
// field matches for the valid articles of one run. revisionIDs maps each
// article DOI to its persisted normalize revision ID.
func computeRunTermMatches(run *Run, articles []*article.Article, revisionIDs map[string]int64) (map[string][]string, map[int64]map[string][]string) {
	queries := make(map[string]string, len(run.Manifest.Sources))
	for _, source := range run.Manifest.Sources {
		queries[source.Name] = source.Query
	}
	terms := searchterms.ParseSources(queries)
	termsBySource := make(map[string][]string)
	for _, term := range terms {
		for _, source := range term.Sources {
			termsBySource[source] = append(termsBySource[source], term.Text)
		}
	}
	matches := make(map[int64]map[string][]string)
	for _, a := range articles {
		revisionID, ok := revisionIDs[a.DOI]
		if !ok || revisionID == 0 {
			continue
		}
		fieldMatches := searchterms.MatchFields(a.Title, a.Abstract, a.Keywords, a.KeywordsAdditional, terms)
		if hasAnyMatch(fieldMatches) {
			matches[revisionID] = fieldMatches
		}
	}
	return termsBySource, matches
}

// hasAnyMatch reports whether any field has at least one matched term.
func hasAnyMatch(fields map[string][]string) bool {
	for _, terms := range fields {
		if len(terms) > 0 {
			return true
		}
	}
	return false
}

// persistRunTermMatches stores the term inventory and revision matches for one run.
func persistRunTermMatches(db *database.Database, runID int64, termsBySource map[string][]string, matches map[int64]map[string][]string) error {
	if err := db.TermMatches.ReplaceRunTermData(runID, termsBySource, matches); err != nil {
		return fmt.Errorf("persist term matches: %w", err)
	}
	return nil
}

// reconcileStoredTermMatchesBestEffort runs the reconciliation pass and logs
// failures without propagating them, so a backfill problem never fails the run
// or the invocation.
func reconcileStoredTermMatchesBestEffort(db *database.Database) {
	if err := reconcileStoredTermMatches(db); err != nil {
		log.Error("term match reconciliation failed", "error", err)
	}
}

// reconcileStoredTermMatches backfills term data for every completed run that
// lacks stored match rows. It reads only stored queries and normalize
// revisions and never reruns pipeline stages.
func reconcileStoredTermMatches(db *database.Database) error {
	rows, err := db.DB.Query(`SELECT id FROM pipeline_runs WHERE status='completed' ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list completed runs for term reconciliation: %w", err)
	}
	var runIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan completed run for term reconciliation: %w", err)
		}
		runIDs = append(runIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate completed runs for term reconciliation: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close completed run rows: %w", err)
	}
	backfilled := 0
	for _, runID := range runIDs {
		reconciled, err := db.TermMatches.HasRunTermData(runID)
		if err != nil {
			return err
		}
		if reconciled {
			continue
		}
		termsBySource, matches, err := computeStoredRunTermMatches(db, runID)
		if err != nil {
			return fmt.Errorf("compute term matches for run %d: %w", runID, err)
		}
		if err := persistRunTermMatches(db, runID, termsBySource, matches); err != nil {
			return fmt.Errorf("persist term matches for run %d: %w", runID, err)
		}
		backfilled++
		log.Info("backfilled term matches", "run_id", runID, "terms", len(termsBySource), "matches", len(matches))
	}
	if backfilled > 0 {
		log.Info("term match reconciliation completed", "runs_backfilled", backfilled)
	}
	return nil
}

// computeStoredRunTermMatches derives term data for one run from stored rows.
func computeStoredRunTermMatches(db *database.Database, runID int64) (map[string][]string, map[int64]map[string][]string, error) {
	sources, err := db.RunSources.ListByRun(runID)
	if err != nil {
		return nil, nil, err
	}
	queries := make(map[string]string)
	for _, source := range sources {
		if strings.TrimSpace(source.Query) != "" {
			queries[source.SourceName] = source.Query
		}
	}
	terms := searchterms.ParseSources(queries)
	termsBySource := make(map[string][]string)
	for _, term := range terms {
		for _, source := range term.Sources {
			termsBySource[source] = append(termsBySource[source], term.Text)
		}
	}
	rows, err := db.DB.Query(`SELECT id, title, abstract, keywords, keywords_plus
		FROM work_revisions WHERE pipeline_run_id=? AND producer_stage='normalize' ORDER BY id`, runID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	matches := make(map[int64]map[string][]string)
	for rows.Next() {
		var revisionID int64
		var title, abstract, keywords, keywordsPlus sql.NullString
		if err := rows.Scan(&revisionID, &title, &abstract, &keywords, &keywordsPlus); err != nil {
			return nil, nil, err
		}
		fieldMatches := searchterms.MatchFields(title.String, abstract.String, parseKeywordArray(keywords), parseKeywordArray(keywordsPlus), terms)
		if hasAnyMatch(fieldMatches) {
			matches[revisionID] = fieldMatches
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return termsBySource, matches, nil
}

// parseKeywordArray decodes a stored keyword TEXT value into an array. JSON
// arrays are used when present; JSON null and empty values become empty arrays;
// otherwise the raw text is treated as a single element.
func parseKeywordArray(raw sql.NullString) []string {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw.String), &arr); err == nil {
		return arr
	}
	return []string{raw.String}
}
