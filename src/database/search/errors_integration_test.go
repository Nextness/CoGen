// errors_integration_test.go verifies the search family propagates connection
// failures as errors instead of panicking or returning partial results.
//go:build integration

package search_test

import (
	"context"
	"testing"

	"analysis/database/search"
)

// TestStoreOperationsPropagateConnectionErrors verifies every public store
// operation reports a closed connection as an error rather than a value.
func TestStoreOperationsPropagateConnectionErrors(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"FindOrCreateSearch", func() error { _, err := store.FindOrCreateSearch(ctx, "closed"); return err }},
		{"GetSearchByID", func() error { _, err := store.GetSearchByID(ctx, 1); return err }},
		{"GetSearchBySearchID", func() error { _, err := store.GetSearchBySearchID(ctx, "closed"); return err }},
		{"ListSearches", func() error { _, err := store.ListSearches(ctx); return err }},
		{"UpsertRevision", func() error {
			_, _, err := store.UpsertRevision(ctx, search.RevisionInput{SearchID: 1, RevisionLabel: "v1"})
			return err
		}},
		{"GetRevisionByID", func() error { _, err := store.GetRevisionByID(ctx, 1); return err }},
		{"GetRevisionBySearchAndLabel", func() error { _, err := store.GetRevisionBySearchAndLabel(ctx, 1, "v1"); return err }},
		{"ListRevisionsBySearch", func() error { _, err := store.ListRevisionsBySearch(ctx, 1); return err }},
		{"FindOrCreatePlan", func() error {
			_, err := store.FindOrCreatePlan(ctx, search.PlanInput{SearchRevisionID: 1, ExecutionFingerprint: "fp"})
			return err
		}},
		{"FindOrCreatePlanWithInputManifest", func() error {
			_, err := store.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{SearchRevisionID: 1, ExecutionFingerprint: "fp", InputManifestHash: "input"})
			return err
		}},
		{"GetPlanByID", func() error { _, err := store.GetPlanByID(ctx, 1); return err }},
		{"GetPlanByFingerprint", func() error { _, err := store.GetPlanByFingerprint(ctx, 1, "fp"); return err }},
		{"ListPlansBySearchRevision", func() error { _, err := store.ListPlansBySearchRevision(ctx, 1); return err }},
		{"HierarchyTotals", func() error { _, err := store.HierarchyTotals(ctx); return err }},
		{"ListLegacySearches", func() error { _, err := store.ListLegacySearches(ctx, 10); return err }},
		{"ListLegacyPlans", func() error { _, err := store.ListLegacyPlans(ctx, 1, 10); return err }},
		{"ListHierarchySearches", func() error {
			_, _, err := store.ListHierarchySearches(ctx, search.HierarchySearchFilter{Limit: 10})
			return err
		}},
		{"ListHierarchyRevisions", func() error {
			_, _, err := store.ListHierarchyRevisions(ctx, search.HierarchyRevisionFilter{SearchID: 1, Limit: 10})
			return err
		}},
		{"ListHierarchyPlans", func() error {
			_, _, err := store.ListHierarchyPlans(ctx, search.HierarchyPlanFilter{SearchRevisionID: 1, Limit: 10})
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("closed connection did not produce an error")
			}
		})
	}
}
