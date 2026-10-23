// hierarchy_integration_test.go exercises the search family's viewer hierarchy
// reads against a database created by the production migration runner.

package search_test

import (
	"context"
	"testing"

	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/tests/support/go/testfixture"
)

// TestStoreHierarchyTotalsCountsWorkspaceIdentities verifies the Home summary
// totals planned and completed attempts without changing ownership.
func TestStoreHierarchyTotalsCountsWorkspaceIdentities(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	firstSearch, err := db.Search.FindOrCreateSearch(ctx, "hierarchy-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Search.FindOrCreateSearch(ctx, "hierarchy-beta"); err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: firstSearch, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: firstSearch, RevisionLabel: "r2", ConfigArtifactHash: "config2", ResolvedManifestHash: "manifest2",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-1", ResolvedManifestHash: "manifest", InputManifestHash: "input",
	}); err != nil {
		t.Fatal(err)
	}
	completedRun, err := testfixture.Run(ctx, db.Conn, "hierarchy", "query")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.FinishRun(ctx, completedRun, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := testfixture.Run(ctx, db.Conn, "hierarchy", "query"); err != nil {
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

	alpha, err := db.Search.FindOrCreateSearch(ctx, "legacy-alpha")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := db.Search.FindOrCreateSearch(ctx, "legacy-beta")
	if err != nil {
		t.Fatal(err)
	}
	var alphaRevisions []int64
	for _, label := range []string{"a1", "a2", "a3"} {
		revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
			SearchID: alpha, RevisionLabel: label, ConfigArtifactHash: "config-" + label, ResolvedManifestHash: "manifest-" + label,
		})
		if err != nil {
			t.Fatal(err)
		}
		alphaRevisions = append(alphaRevisions, revisionID)
	}
	var betaRevisions []int64
	for _, label := range []string{"b1", "b2"} {
		revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
			SearchID: beta, RevisionLabel: label, ConfigArtifactHash: "config-" + label, ResolvedManifestHash: "manifest-" + label,
		})
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

	searchID, err := db.Search.FindOrCreateSearch(ctx, "legacy-plans")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest",
	})
	if err != nil {
		t.Fatal(err)
	}
	var planIDs []int64
	for _, fingerprint := range []string{"fp-a", "fp-b", "fp-c"} {
		planID, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
			SearchRevisionID: revisionID, ExecutionFingerprint: fingerprint, ResolvedManifestHash: "manifest",
			InputManifestHash: "input", EnrichmentEnabled: true,
		})
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

// TestStoreListHierarchySectionsFilterOrderAndPage verifies the filtered Home
// sections return descending identifiers, lookahead flags, cursor exclusion,
// searchable predicates, counts, and nullable newest planned identifiers.
func TestStoreListHierarchySectionsFilterOrderAndPage(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	alpha, err := db.Search.FindOrCreateSearch(ctx, "section-alpha")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := db.Search.FindOrCreateSearch(ctx, "section-beta")
	if err != nil {
		t.Fatal(err)
	}
	gamma, err := db.Search.FindOrCreateSearch(ctx, "section-gamma")
	if err != nil {
		t.Fatal(err)
	}
	alphaRevision, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: alpha, RevisionLabel: "alpha-r1", ConfigArtifactHash: "config-a", ResolvedManifestHash: "manifest-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	alphaPlan, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: alphaRevision, ExecutionFingerprint: "alpha-fp", ResolvedManifestHash: "manifest-a", InputManifestHash: "input-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Run.StartAttemptIfIdle(ctx, run.StartAttemptInput{ExecutionPlanID: alphaPlan, Step: "workspace"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: gamma, RevisionLabel: "gamma-r1", ConfigArtifactHash: "config-g", ResolvedManifestHash: "manifest-g",
	}); err != nil {
		t.Fatal(err)
	}

	searches, hasMore, err := store.ListHierarchySearches(ctx, search.HierarchySearchFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(searches) != 2 {
		t.Fatalf("search page = %d items hasMore=%v, want 2 items with lookahead", len(searches), hasMore)
	}
	if searches[0].ID != gamma || searches[1].ID != beta {
		t.Fatalf("search order = [%d %d], want [%d %d]", searches[0].ID, searches[1].ID, gamma, beta)
	}
	if searches[0].RevisionCount != 1 || searches[0].LatestRunID != nil || searches[0].LatestPlanID != nil {
		t.Fatalf("gamma summary = %+v, want one revision and no planned identifiers", searches[0])
	}

	filtered, _, err := store.ListHierarchySearches(ctx, search.HierarchySearchFilter{Query: "alpha-r1", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ID != alpha {
		t.Fatalf("revision-label filter = %+v, want search %d", filtered, alpha)
	}
	if filtered[0].RunCount != 1 || filtered[0].LatestRunID == nil || filtered[0].LatestPlanID == nil || filtered[0].LatestRevisionID == nil {
		t.Fatalf("alpha summary = %+v, want planned identifiers and one run", filtered[0])
	}
	if *filtered[0].LatestRevisionID != alphaRevision || *filtered[0].LatestPlanID != alphaPlan {
		t.Fatalf("alpha newest identifiers = %d/%d, want %d/%d", *filtered[0].LatestRevisionID, *filtered[0].LatestPlanID, alphaRevision, alphaPlan)
	}

	afterCursor, _, err := store.ListHierarchySearches(ctx, search.HierarchySearchFilter{CursorID: beta, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(afterCursor) != 1 || afterCursor[0].ID != alpha {
		t.Fatalf("cursor page = %+v, want only search %d", afterCursor, alpha)
	}

	revisions, revisionHasMore, err := store.ListHierarchyRevisions(ctx, search.HierarchyRevisionFilter{SearchID: alpha, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if revisionHasMore || len(revisions) != 1 || revisions[0].ID != alphaRevision {
		t.Fatalf("revision page = %+v hasMore=%v, want only revision %d", revisions, revisionHasMore, alphaRevision)
	}
	if revisions[0].RunCount != 1 || revisions[0].LatestRunID == nil || revisions[0].LatestPlanID == nil {
		t.Fatalf("revision summary = %+v, want planned identifiers", revisions[0])
	}

	plans, planHasMore, err := store.ListHierarchyPlans(ctx, search.HierarchyPlanFilter{SearchRevisionID: alphaRevision, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if planHasMore || len(plans) != 1 || plans[0].ID != alphaPlan || plans[0].EnrichmentEnabled {
		t.Fatalf("plan page = %+v hasMore=%v, want plan %d", plans, planHasMore, alphaPlan)
	}

	missing, _, err := store.ListHierarchyPlans(ctx, search.HierarchyPlanFilter{SearchRevisionID: alphaRevision, Query: "no-match", Limit: 1})
	if err != nil || len(missing) != 0 {
		t.Fatalf("unmatched plan filter = %+v err=%v, want empty", missing, err)
	}
	if _, _, err := store.ListHierarchySearches(ctx, search.HierarchySearchFilter{Limit: 0}); err == nil {
		t.Fatal("expected a limit validation error")
	}
}

// TestStoreListHierarchySectionsLookaheadAndQueryFilters verifies revision and
// plan lookahead flags and their searchable predicates.
func TestStoreListHierarchySectionsLookaheadAndQueryFilters(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()

	searchID, err := db.Search.FindOrCreateSearch(ctx, "lookahead-search")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "first-revision", ConfigArtifactHash: "config-1", ResolvedManifestHash: "manifest-1",
	}); err != nil {
		t.Fatal(err)
	}
	secondRevision, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
		SearchID: searchID, RevisionLabel: "second-revision", ConfigArtifactHash: "config-2", ResolvedManifestHash: "manifest-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: secondRevision, ExecutionFingerprint: "first-fingerprint", ResolvedManifestHash: "manifest", InputManifestHash: "input",
	}); err != nil {
		t.Fatal(err)
	}
	secondPlan, err := db.Search.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: secondRevision, ExecutionFingerprint: "second-fingerprint", ResolvedManifestHash: "manifest",
		InputManifestHash: "input", EnrichmentEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	revisions, hasMore, err := store.ListHierarchyRevisions(ctx, search.HierarchyRevisionFilter{SearchID: searchID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(revisions) != 1 || revisions[0].ID != secondRevision {
		t.Fatalf("revision lookahead = %+v hasMore=%v, want newest revision %d", revisions, hasMore, secondRevision)
	}
	filteredRevisions, _, err := store.ListHierarchyRevisions(ctx, search.HierarchyRevisionFilter{SearchID: searchID, Query: "first-revision", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(filteredRevisions) != 1 || filteredRevisions[0].Label != "first-revision" {
		t.Fatalf("filtered revisions = %+v", filteredRevisions)
	}

	plans, planHasMore, err := store.ListHierarchyPlans(ctx, search.HierarchyPlanFilter{SearchRevisionID: secondRevision, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !planHasMore || len(plans) != 1 || plans[0].ID != secondPlan {
		t.Fatalf("plan lookahead = %+v hasMore=%v, want newest plan %d", plans, planHasMore, secondPlan)
	}
	filteredPlans, _, err := store.ListHierarchyPlans(ctx, search.HierarchyPlanFilter{SearchRevisionID: secondRevision, Query: "first-fingerprint", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(filteredPlans) != 1 || filteredPlans[0].ExecutionFingerprint != "first-fingerprint" {
		t.Fatalf("filtered plans = %+v", filteredPlans)
	}
}
