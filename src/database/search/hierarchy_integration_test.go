// hierarchy_integration_test.go exercises the search family's viewer hierarchy
// reads against a database created by the production migration runner.
//go:build integration

package search_test

import (
	"context"
	"testing"

	"analysis/database/search"
)

// TestStoreHierarchyTotalsCountsWorkspaceIdentities verifies the Home summary
// totals planned and completed attempts without changing ownership.
func TestStoreHierarchyTotalsCountsWorkspaceIdentities(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	firstSearch, err := db.Searches.Create("hierarchy-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Searches.Create("hierarchy-beta"); err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := db.Revisions.Create(firstSearch, "r1", "config", "manifest")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Revisions.Create(firstSearch, "r2", "config2", "manifest2"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Plans.CreateWithInputManifest(revisionID, "fp-1", "manifest", "input", false); err != nil {
		t.Fatal(err)
	}
	completedRun, err := db.PipelineRuns.StartRun("hierarchy", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PipelineRuns.FinishRun(completedRun, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PipelineRuns.StartRun("hierarchy", "query"); err != nil {
		t.Fatal(err)
	}

	totals, err := store.HierarchyTotals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Searches != 2 || totals.Revisions != 2 || totals.Plans != 1 || totals.Runs != 2 || totals.CompletedRuns != 1 {
		t.Fatalf("hierarchy totals = %+v", totals)
	}
}

// TestStoreListLegacySearchesBoundsNewestRevisionsPerSearch verifies the
// deprecated discovery projection orders searches and revisions descending and
// keeps at most the requested revisions per search.
func TestStoreListLegacySearchesBoundsNewestRevisionsPerSearch(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	alpha, err := db.Searches.Create("legacy-alpha")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := db.Searches.Create("legacy-beta")
	if err != nil {
		t.Fatal(err)
	}
	var alphaRevisions []int64
	for _, label := range []string{"a1", "a2", "a3"} {
		revisionID, _, err := db.Revisions.Create(alpha, label, "config-"+label, "manifest-"+label)
		if err != nil {
			t.Fatal(err)
		}
		alphaRevisions = append(alphaRevisions, revisionID)
	}
	var betaRevisions []int64
	for _, label := range []string{"b1", "b2"} {
		revisionID, _, err := db.Revisions.Create(beta, label, "config-"+label, "manifest-"+label)
		if err != nil {
			t.Fatal(err)
		}
		betaRevisions = append(betaRevisions, revisionID)
	}

	rows, err := store.ListLegacySearches(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("legacy search rows = %d, want 4", len(rows))
	}
	bySearch := map[int64][]*search.LegacySearchRow{}
	for _, row := range rows {
		bySearch[row.ID] = append(bySearch[row.ID], row)
	}
	alphaRows := bySearch[alpha]
	if len(alphaRows) != 2 {
		t.Fatalf("alpha revision rows = %d, want 2", len(alphaRows))
	}
	if alphaRows[0].RevisionID == nil || *alphaRows[0].RevisionID != alphaRevisions[2] ||
		alphaRows[1].RevisionID == nil || *alphaRows[1].RevisionID != alphaRevisions[1] {
		t.Fatalf("alpha revisions = %+v %+v, want newest first", alphaRows[0].RevisionID, alphaRows[1].RevisionID)
	}
	if alphaRows[0].RevisionLabel != "a3" || alphaRows[0].ConfigArtifactHash != "config-a3" {
		t.Fatalf("alpha newest revision = %+v", alphaRows[0])
	}
	betaRows := bySearch[beta]
	if len(betaRows) != 2 || betaRows[0].RevisionID == nil || *betaRows[0].RevisionID != betaRevisions[1] {
		t.Fatalf("beta revisions = %+v, want newest first", betaRows)
	}
	if rows[0].ID != beta || rows[len(rows)-1].ID != alpha {
		t.Fatalf("legacy search order = %d..%d, want search %d first", rows[0].ID, rows[len(rows)-1].ID, beta)
	}
}

// TestStoreListLegacyPlansOrdersDescendingWithLimit verifies the deprecated
// plan discovery projection returns newest plans first and honors its bound.
func TestStoreListLegacyPlansOrdersDescendingWithLimit(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	searchID, err := db.Searches.Create("legacy-plans")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := db.Revisions.Create(searchID, "r1", "config", "manifest")
	if err != nil {
		t.Fatal(err)
	}
	var planIDs []int64
	for _, fingerprint := range []string{"fp-a", "fp-b", "fp-c"} {
		planID, err := db.Plans.CreateWithInputManifest(revisionID, fingerprint, "manifest", "input", true)
		if err != nil {
			t.Fatal(err)
		}
		planIDs = append(planIDs, planID)
	}

	plans, err := store.ListLegacyPlans(ctx, revisionID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("legacy plans = %d, want 2", len(plans))
	}
	if plans[0].ID != planIDs[2] || plans[1].ID != planIDs[1] {
		t.Fatalf("legacy plan ids = [%d %d], want [%d %d]", plans[0].ID, plans[1].ID, planIDs[2], planIDs[1])
	}
	if plans[0].ExecutionFingerprint != "fp-c" || !plans[0].EnrichmentEnabled {
		t.Fatalf("legacy newest plan = %+v", plans[0])
	}
}
