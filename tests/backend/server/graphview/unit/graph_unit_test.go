// Package graphview_test verifies graph failures through the production assembler API.
package graphview_test

import (
	"analysis/backend/database/work"
	"analysis/backend/server/graphview"
	"analysis/tests/support/go/testfixture"
	"testing"
)

// TestGraphReadFailureReturnsNoPartialGraph verifies each relationship mode propagates storage failure atomically.
func TestGraphReadFailureReturnsNoPartialGraph(t *testing.T) {
	store := work.New(testfixture.ClosedDB())
	articles := []map[string]any{{"id": int64(1), "work_id": int64(1), "title": "Fixture", "doi": nil}}
	for _, mode := range []string{"article_author", "article_reference", "citation", "research_network"} {
		t.Run(mode, func(t *testing.T) {
			nodes, edges, _, err := graphview.Build(t.Context(), store, mode, articles, 10, 10)
			if err == nil || nodes != nil || edges != nil {
				t.Fatalf("nodes=%v edges=%v err=%v", nodes, edges, err)
			}
		})
	}
}
