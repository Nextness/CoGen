// normalized.go defines the authoritative current analysis-ready revision predicate.
package server

import "analysis/database"

// currentNormalizedRevisionPredicate selects the latest valid normalize revision for each run and work.
func currentNormalizedRevisionPredicate(alias string) string {
	return database.CurrentNormalizedRevisionPredicate(alias)
}
