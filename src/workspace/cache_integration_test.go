// cache_integration_test.go tests the workspace cache layer with mock
// HTTP servers and temporary databases, covering policy resolution,
// cache hits/misses, negative entries, and stale-entry behaviour.
//go:build integration

package workspace

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"analysis/article"
	"analysis/database"
	"analysis/enrich"
	"analysis/manifest"
)

// openWorkspaceCacheTest supports the package test suite's open workspace cache test setup or assertions.
func openWorkspaceCacheTest(t *testing.T, policy manifest.CachePolicy) (*database.Database, *workspaceCache, int64) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "cache.db"), filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.PipelineRuns.StartRun("cache-test", "")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db, &workspaceCache{db: db, runID: runID, policy: policy}, runID
}

// testCacheRequest supports the package test suite's test cache request setup or assertions.
func testCacheRequest() cacheRequest {
	return cacheRequest{Provider: "crossref", Namespace: "work_by_doi", Identity: "10.1000/example", URL: "https://provider.test/works/10.1000/example"}
}

// testCachePayload supports the package test suite's test cache payload setup or assertions.
func testCachePayload(title string) []byte {
	return []byte(`{"message":{"title":["` + title + `"]}}`)
}

// TestWorkspaceCacheNamedPriorRunReproduction verifies workspace cache named prior run reproduction.
func TestWorkspaceCacheNamedPriorRunReproduction(t *testing.T) {
	db, first, firstRunID := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run"}})
	defer db.Close()
	request := testCacheRequest()
	calls := 0
	response, err := first.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: []byte(`{"message":{"title":["first"]}}`)}
	}, nil)
	if err != nil || response.Layer != "network" || calls != 1 {
		t.Fatalf("first response = %+v, %v, calls=%d", response, err, calls)
	}
	secondRunID, err := db.PipelineRuns.StartRun("cache-test", "")
	if err != nil {
		t.Fatal(err)
	}
	second := &workspaceCache{db: db, runID: secondRunID, policy: manifest.CachePolicy{Reads: []string{"run:" + strconv.FormatInt(firstRunID, 10)}, Writes: []string{"active_run"}}}
	response, err = second.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: []byte("unexpected")}
	}, nil)
	if err != nil || response.Layer != "run:"+strconv.FormatInt(firstRunID, 10) || string(response.Body) == "unexpected" || calls != 1 {
		t.Fatalf("prior-run response = %+v, %v, calls=%d", response, err, calls)
	}
	uses, err := db.RunCacheUses.ListByRun(secondRunID)
	if err != nil || len(uses) != 1 || uses[0].CacheLayer != "run:"+strconv.FormatInt(firstRunID, 10) || uses[0].Outcome != string(manifest.CacheHit) {
		t.Fatalf("prior-run use = %+v, %v", uses, err)
	}
}

// TestWorkspaceCacheGlobalReuseAcrossRuns verifies workspace cache global reuse across runs.
func TestWorkspaceCacheGlobalReuseAcrossRuns(t *testing.T) {
	db, first, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run", "global"}})
	defer db.Close()
	request := testCacheRequest()
	calls := 0
	if _, err := first.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: []byte(`{"message":{"title":["global"]}}`)}
	}, nil); err != nil {
		t.Fatal(err)
	}
	_, second, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"global", "network"}, Writes: []string{"active_run"}})
	response, err := second.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: []byte("unexpected")}
	}, nil)
	if err != nil || response.Layer != "global" || string(response.Body) == "unexpected" || calls != 1 {
		t.Fatalf("global reuse = %+v, %v, calls=%d", response, err, calls)
	}
}

// TestWorkspaceCacheOfflineMiss verifies workspace cache offline miss.
func TestWorkspaceCacheOfflineMiss(t *testing.T) {
	db, cache, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"active_run", "global"}, Writes: []string{"active_run"}})
	defer db.Close()
	_, err := cache.resolve(context.Background(), testCacheRequest(), func(context.Context) *enrich.FetchResult {
		t.Fatal("offline policy attempted network")
		return nil
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "no network layer") {
		t.Fatalf("offline miss error = %v", err)
	}
}

// TestWorkspaceCacheFreshBypassesGlobal verifies workspace cache fresh bypasses global.
func TestWorkspaceCacheFreshBypassesGlobal(t *testing.T) {
	db, first, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}})
	defer db.Close()
	request := testCacheRequest()
	calls := 0
	if _, err := first.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: testCachePayload("global")}
	}, nil); err != nil {
		t.Fatal(err)
	}
	_, fresh, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run"}})
	response, err := fresh.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: testCachePayload("fresh")}
	}, nil)
	if err != nil || response.Layer != "network" || string(response.Body) != string(testCachePayload("fresh")) || calls != 2 {
		t.Fatalf("fresh response = %+v, %v, calls=%d", response, err, calls)
	}
	_, globalReader, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"global"}, Writes: []string{"active_run"}})
	response, err = globalReader.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: []byte("unexpected")}
	}, nil)
	if err != nil || response.Layer != "global" || string(response.Body) != string(testCachePayload("global")) || calls != 2 {
		t.Fatalf("global entry after fresh private write = %+v, %v, calls=%d", response, err, calls)
	}
}

// TestWorkspaceCacheStaleNegativeRelooksUp verifies workspace cache stale negative relooks up.
func TestWorkspaceCacheStaleNegativeRelooksUp(t *testing.T) {
	db, first, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}, NegativeTTLDays: 1})
	defer db.Close()
	request := testCacheRequest()
	calls := 0
	response, err := first.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 404}
	}, nil)
	if err != nil || response.Outcome != manifest.CacheNegative {
		t.Fatalf("negative response = %+v, %v", response, err)
	}
	entry, err := db.CacheEntries.Get("crossref", "work_by_doi", cacheFingerprint(request), cacheExtractorVersion)
	if err != nil || entry == nil {
		t.Fatalf("negative entry = %+v, %v", entry, err)
	}
	if _, err := db.DB.Exec("UPDATE cache_entries SET expires_at='2000-01-01T00:00:00Z' WHERE id=?", entry.ID); err != nil {
		t.Fatal(err)
	}
	_, second, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"global", "network"}, Writes: []string{"global"}, NegativeTTLDays: 1})
	response, err = second.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: testCachePayload("relooked")}
	}, nil)
	if err != nil || response.Layer != "network" || string(response.Body) != string(testCachePayload("relooked")) || calls != 2 {
		t.Fatalf("stale negative relookup = %+v, %v, calls=%d", response, err, calls)
	}
}

// TestWorkspaceCacheNetworkFallback verifies workspace cache network fallback.
func TestWorkspaceCacheNetworkFallback(t *testing.T) {
	db, cache, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"global", "network"}, Writes: []string{"active_run", "global"}})
	defer db.Close()
	calls := 0
	response, err := cache.resolve(context.Background(), testCacheRequest(), func(context.Context) *enrich.FetchResult {
		calls++
		return &enrich.FetchResult{StatusCode: 200, Body: testCachePayload("network")}
	}, nil)
	if err != nil || response.Layer != "network" || string(response.Body) != string(testCachePayload("network")) || calls != 1 {
		t.Fatalf("network fallback = %+v, %v, calls=%d", response, err, calls)
	}
	uses, err := db.RunCacheUses.ListByRun(cache.runID)
	if err != nil || len(uses) != 2 || uses[0].CacheLayer != "active_run" || uses[1].CacheLayer != "global" {
		t.Fatalf("network cache writes = %+v, %v", uses, err)
	}
	entry, err := db.CacheEntries.Get("crossref", "work_by_doi", cacheFingerprint(testCacheRequest()), cacheExtractorVersion)
	if err != nil || entry == nil || entry.PayloadArtifactID == nil {
		t.Fatalf("network cache payload linkage = %+v, %v", entry, err)
	}
	for _, metric := range []struct {
		name   string
		source string
		want   int
	}{
		{"cache_misses", "", 1}, {"cache_misses", "crossref", 1},
		{"cache_network_fetches", "", 1}, {"cache_network_fetches", "crossref", 1},
	} {
		got, err := db.Metrics.Get(cache.runID, metric.name, metric.source)
		if err != nil || got == nil || got.Value != metric.want {
			t.Fatalf("metric %s/%s = %+v, %v; want %d", metric.name, metric.source, got, err, metric.want)
		}
	}
}

// TestWorkspaceCacheRejectsMalformedSuccessAndRecovers verifies workspace cache rejects malformed success and recovers.
func TestWorkspaceCacheRejectsMalformedSuccessAndRecovers(t *testing.T) {
	db, cache, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}})
	defer db.Close()
	request := testCacheRequest()
	if _, err := cache.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		return &enrich.FetchResult{StatusCode: 200, Body: []byte(`{"error":"temporary provider response"}`)}
	}, nil); err == nil || !strings.Contains(err.Error(), "invalid provider payload") {
		t.Fatalf("malformed provider response error = %v", err)
	}
	entry, err := db.CacheEntries.Get("crossref", "work_by_doi", cacheFingerprint(request), cacheExtractorVersion)
	if err != nil || entry != nil {
		t.Fatalf("malformed provider response was cached: entry=%+v err=%v", entry, err)
	}
	metric, err := db.Metrics.Get(cache.runID, "cache_invalid_payloads", "crossref")
	if err != nil || metric == nil || metric.Value != 1 {
		t.Fatalf("invalid payload metric = %+v, %v", metric, err)
	}

	valid := testCachePayload("recovered")
	response, err := cache.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		return &enrich.FetchResult{StatusCode: 200, Body: valid}
	}, nil)
	if err != nil || response.Layer != "network" || string(response.Body) != string(valid) {
		t.Fatalf("valid retry response = %+v, %v", response, err)
	}
	entry, err = db.CacheEntries.Get("crossref", "work_by_doi", cacheFingerprint(request), cacheExtractorVersion)
	if err != nil || entry == nil || entry.PayloadArtifactID == nil {
		t.Fatalf("valid retry cache entry = %+v, %v", entry, err)
	}
}

// TestWorkspaceCacheSkipsStoredMalformedPayload verifies workspace cache skips stored malformed payload.
func TestWorkspaceCacheSkipsStoredMalformedPayload(t *testing.T) {
	db, writer, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}})
	defer db.Close()
	request := testCacheRequest()
	artifactID, err := persistArtifact(db, writer.runID, []byte(`{"error":"legacy malformed payload"}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	entryID, err := db.CacheEntries.Upsert(&database.CacheEntry{
		Provider: request.Provider, Namespace: request.Namespace, RequestFingerprint: cacheFingerprint(request),
		ResponseStatus: 200, PayloadArtifactID: &artifactID, FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), ExtractorVersion: cacheExtractorVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunCacheUses.Create(&database.RunCacheUse{PipelineRunID: writer.runID, CacheEntryID: entryID, CacheLayer: "global", Outcome: string(manifest.CacheHit)}); err != nil {
		t.Fatal(err)
	}

	_, reader, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"global", "network"}, Writes: []string{"active_run"}})
	valid := testCachePayload("recovered from legacy entry")
	response, err := reader.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		return &enrich.FetchResult{StatusCode: 200, Body: valid}
	}, nil)
	if err != nil || response.Layer != "network" || string(response.Body) != string(valid) {
		t.Fatalf("legacy malformed cache response = %+v, %v", response, err)
	}
	metric, err := db.Metrics.Get(reader.runID, "cache_invalid_payloads", "crossref")
	if err != nil || metric == nil || metric.Value != 1 {
		t.Fatalf("legacy invalid payload metric = %+v, %v", metric, err)
	}
}

// TestWorkspaceCacheORCIDNameFailureAndEmptyMatchPolicies verifies workspace cache orcid name failure and empty match policies.
func TestWorkspaceCacheORCIDNameFailureAndEmptyMatchPolicies(t *testing.T) {
	db, cache, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}, NegativeTTLDays: 14})
	defer db.Close()
	request := cacheRequest{Provider: "orcid", Namespace: "author_name_search", Identity: "ada lovelace|query-one", URL: "https://provider.test/search?q=ada"}
	negative := func(body []byte) bool { return len(enrich.DecodeORCIDNameSearchCandidates(body)) == 0 }
	if _, err := cache.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		return &enrich.FetchResult{Err: errors.New("temporary ORCID outage")}
	}, negative); err == nil || !strings.Contains(err.Error(), "temporary ORCID outage") {
		t.Fatalf("transient ORCID error = %v", err)
	}
	entry, err := db.CacheEntries.Get("orcid", "author_name_search", cacheFingerprint(request), cacheExtractorVersion)
	if err != nil || entry != nil {
		t.Fatalf("transient ORCID error was cached: entry=%+v err=%v", entry, err)
	}

	response, err := cache.resolve(context.Background(), request, func(context.Context) *enrich.FetchResult {
		return &enrich.FetchResult{StatusCode: 200, Body: []byte(`{"result":[{"orcid-identifier":{"path":"0000-0001-2345-6789"}}]}`)}
	}, negative)
	if err != nil || response.Status != 200 {
		t.Fatalf("ORCID retry response = %+v, %v", response, err)
	}

	emptyRequest := cacheRequest{Provider: "orcid", Namespace: "author_name_search", Identity: "no match|query-two", URL: "https://provider.test/search?q=none"}
	response, err = cache.resolve(context.Background(), emptyRequest, func(context.Context) *enrich.FetchResult {
		return &enrich.FetchResult{StatusCode: 200, Body: []byte(`{"result":null,"num-found":0}`)}
	}, negative)
	if err != nil || response.Status != 404 || response.Outcome != manifest.CacheNegative {
		t.Fatalf("empty ORCID result = %+v, %v", response, err)
	}
	emptyEntry, err := db.CacheEntries.Get("orcid", "author_name_search", cacheFingerprint(emptyRequest), cacheExtractorVersion)
	if err != nil || emptyEntry == nil || emptyEntry.ExpiresAt == "" || emptyEntry.PayloadArtifactID != nil {
		t.Fatalf("empty ORCID negative cache entry = %+v, %v", emptyEntry, err)
	}
}

// TestWorkspaceMetricUnavailableForOlderRun verifies workspace metric unavailable for older run.
func TestWorkspaceMetricUnavailableForOlderRun(t *testing.T) {
	db, cache, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"active_run"}})
	defer db.Close()
	metric, err := db.Metrics.Get(cache.runID, "cache_hits", "")
	if err != nil || metric != nil {
		t.Fatalf("unrecorded cache metric = %+v, %v", metric, err)
	}
}

// TestWorkspaceCrossrefCacheUsesSQLitePolicy verifies workspace crossref cache uses sq lite policy.
func TestWorkspaceCrossrefCacheUsesSQLitePolicy(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/works/10.1000/example" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"message":{"title":["Cached title"],"publisher":"Cached publisher"}}`))
	}))
	defer server.Close()
	db, first, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}})
	defer db.Close()
	source := enrich.SourceConfig{Name: "crossref", BaseURL: server.URL + "/works/", RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 1, BatchSize: 1}
	articles := []*article.Article{{DOI: "10.1000/example"}}
	result, err := gatherCachedCrossref(context.Background(), first, source, articles)
	if err != nil || result.Articles["10.1000/example"] == nil || result.Articles["10.1000/example"].Title != "Cached title" || requests != 1 {
		t.Fatalf("first cached gather = %+v, %v, requests=%d", result, err, requests)
	}
	_, second, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"global", "network"}, Writes: []string{"active_run"}})
	result, err = gatherCachedCrossref(context.Background(), second, source, articles)
	if err != nil || result.Articles["10.1000/example"] == nil || requests != 1 {
		t.Fatalf("global cached gather = %+v, %v, requests=%d", result, err, requests)
	}
}

// TestWorkspaceOpenAlexReferenceCacheUsesSQLitePolicy verifies the OpenAlex reference cache uses SQLite policy.
func TestWorkspaceOpenAlexReferenceCacheUsesSQLitePolicy(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case r.URL.Path == "/works/doi:10.1000/example":
			_, _ = w.Write([]byte(`{"id":"https://openalex.org/W1","title":"OpenAlex title","referenced_works":["https://openalex.org/W123"]}`))
		case r.URL.Path == "/works" && strings.Contains(r.URL.Query().Get("filter"), "W123"):
			_, _ = w.Write([]byte(`{"results":[{"id":"https://openalex.org/W123","doi":"https://doi.org/10.1000/reference","title":"Reference","publication_year":2024}]}`))
		default:
			t.Fatalf("unexpected OpenAlex request %q?%s", r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	db, first, _ := openWorkspaceCacheTest(t, manifest.CachePolicy{Reads: []string{"network"}, Writes: []string{"global"}})
	defer db.Close()
	source := enrich.SourceConfig{Name: "openalex", BaseURL: server.URL + "/works/", RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 1, BatchSize: 1}
	articles := []*article.Article{{DOI: "10.1000/example"}}
	result, err := gatherCachedOpenAlex(context.Background(), first, source, articles)
	if err != nil || result.Articles["10.1000/example"] == nil || len(result.Articles["10.1000/example"].References) != 1 || result.Articles["10.1000/example"].References[0].DOI != "10.1000/reference" || requests != 2 {
		t.Fatalf("first OpenAlex gather = %+v, %v, requests=%d", result, err, requests)
	}

	_, second, _ := openWorkspaceCacheTestForDB(t, db, manifest.CachePolicy{Reads: []string{"global", "network"}, Writes: []string{"active_run"}})
	result, err = gatherCachedOpenAlex(context.Background(), second, source, articles)
	if err != nil || result.Articles["10.1000/example"] == nil || len(result.Articles["10.1000/example"].References) != 1 || requests != 2 {
		t.Fatalf("global OpenAlex gather = %+v, %v, requests=%d", result, err, requests)
	}
}

// openWorkspaceCacheTestForDB supports the package test suite's open workspace cache test for db setup or assertions.
func openWorkspaceCacheTestForDB(t *testing.T, db *database.Database, policy manifest.CachePolicy) (*database.Database, *workspaceCache, int64) {
	t.Helper()
	runID, err := db.PipelineRuns.StartRun("cache-test", "")
	if err != nil {
		t.Fatal(err)
	}
	return db, &workspaceCache{db: db, runID: runID, policy: policy}, runID
}
