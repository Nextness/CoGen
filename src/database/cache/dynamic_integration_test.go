// dynamic_integration_test.go verifies the retained cache-use builder rejects
// injection attempts, follows its declared ordering, and caps its search and
// page inputs.
//go:build integration

package cache_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/cache"
)

// TestStoreListCacheUsesForRunConstrainsDynamicInputs verifies the retained
// cache-use builder binds search values, rejects unknown sort and order
// values, follows its declared ordering, and caps oversized inputs.
func TestStoreListCacheUsesForRunConstrainsDynamicInputs(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID, err := db.Run.StartRun(ctx, "cache-viewer-dynamic", "")
	if err != nil {
		t.Fatal(err)
	}
	alphaEntry := appendTestEntry(t, store, cache.Key{Provider: "crossref", Namespace: "doi", RequestFingerprint: "alpha", ExtractorVersion: "v1"}, 200)
	betaEntry := appendTestEntry(t, store, cache.Key{Provider: "openalex", Namespace: `x' OR '1'='1`, RequestFingerprint: "beta", ExtractorVersion: "v1"}, 404)
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: alphaEntry, CacheLayer: "global", Outcome: "hit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: betaEntry, CacheLayer: "global", Outcome: "negative"}); err != nil {
		t.Fatal(err)
	}

	ascending, err := store.ListCacheUsesForRun(ctx, cache.CacheUseFilter{RunID: runID, Sort: "provider", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(ascending.Items) != 2 || ascending.Items[0].Provider != "crossref" || ascending.Items[1].Provider != "openalex" {
		t.Fatalf("ascending provider order = %+v", ascending.Items)
	}
	descending, err := store.ListCacheUsesForRun(ctx, cache.CacheUseFilter{RunID: runID, Sort: "provider", Order: "DESC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(descending.Items) != 2 || descending.Items[0].Provider != "openalex" || descending.Items[1].Provider != "crossref" {
		t.Fatalf("descending provider order = %+v", descending.Items)
	}

	// A malicious search value is bound as a literal LIKE pattern and matches
	// only the entry that stores the same text.
	literal, err := store.ListCacheUsesForRun(ctx, cache.CacheUseFilter{RunID: runID, Query: `' OR '1'='1`, Sort: "id", Order: "ASC", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if literal.Total != 1 || len(literal.Items) != 1 || literal.Items[0].CacheEntryID != betaEntry {
		t.Fatalf("literal search = %+v, want only the stored malicious text", literal.Items)
	}

	for _, test := range []struct {
		name   string
		filter cache.CacheUseFilter
	}{
		{"injected sort", cache.CacheUseFilter{RunID: runID, Sort: "id; DROP TABLE run_cache_uses; --", Order: "ASC", Page: 1, PerPage: 20}},
		{"injected order", cache.CacheUseFilter{RunID: runID, Sort: "id", Order: "ASC; DROP TABLE run_cache_uses; --", Page: 1, PerPage: 20}},
		{"oversized query", cache.CacheUseFilter{RunID: runID, Query: strings.Repeat("x", 201), Sort: "id", Order: "ASC", Page: 1, PerPage: 20}},
		{"oversized page size", cache.CacheUseFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 1, PerPage: 1001}},
		{"zero page", cache.CacheUseFilter{RunID: runID, Sort: "id", Order: "ASC", Page: 0, PerPage: 20}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.ListCacheUsesForRun(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}
