// e2e_test.go verifies the supported analysis binary from pipeline input
// through persisted evidence, the read-only API, and generated viewer data.
//go:build e2e

package main

import (
	"analysis/database"
	"analysis/pdfstore"
	"analysis/server"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	e2eCandidateDOI   = "10.17116/rosstomat20251801140"
	e2eCandidateORCID = "0000-0003-0779-1055"
	e2eOpenAlexORCID  = "0000-0002-1825-0097"
)

// e2eMode describes one pipeline variant and its expected persisted result.
type e2eMode struct {
	name              string
	enrichment        bool
	live              bool
	providerBaseURL   string
	sources           []e2eSource
	expectedParsed    int
	expectedUnique    int
	expectedValid     int
	expectedDiscarded int
	expectedTitles    []string
}

// e2eSource describes one tracked input used by a generated workspace configuration.
type e2eSource struct {
	name     string
	filename string
	fileType string
	count    int
}

// e2eResult identifies the generated database and rows needed by API assertions.
type e2eResult struct {
	dbPath  string
	runID   int64
	workIDs map[string]int64
	titles  []string
}

// providerMock records requests served by the local deterministic provider.
type providerMock struct {
	mu       sync.Mutex
	requests []string
}

// TestE2EDeterministic verifies the offline pipeline, database, API, and PDF inventory flow.
func TestE2EDeterministic(t *testing.T) {
	root := e2eRepositoryRoot(t)
	mode := e2eMode{
		name: "deterministic",
		sources: []e2eSource{
			{name: "alpha_csv", filename: "offline.csv", fileType: "csv", count: 4},
			{name: "beta_bib", filename: "offline.bib", fileType: "bib", count: 2},
		},
		expectedParsed: 5, expectedUnique: 4, expectedValid: 2, expectedDiscarded: 2,
		expectedTitles: []string{"Offline Complete One", "Offline Complete Two"},
	}
	result := runE2EVariant(t, root, mode, nil)
	assertE2EAPI(t, result)
}

// TestE2EMocked verifies enrichment through loopback providers and cross-layer evidence.
func TestE2EMocked(t *testing.T) {
	root := e2eRepositoryRoot(t)
	mock := &providerMock{}
	provider := newE2EProviderServer(t, mock)
	defer provider.Close()
	mode := e2eMode{
		name: "mocked", enrichment: true, providerBaseURL: provider.URL,
		sources: []e2eSource{
			{name: "alpha_csv", filename: "offline.csv", fileType: "csv", count: 4},
			{name: "beta_bib", filename: "offline.bib", fileType: "bib", count: 2},
		},
		expectedParsed: 5, expectedUnique: 4, expectedValid: 3, expectedDiscarded: 1,
		expectedTitles: []string{"Offline Complete One", "Offline Complete Two", "Provider Enrichment Candidate"},
	}
	result := runE2EVariant(t, root, mode, e2eOfflineEnvironment(provider.URL))
	mock.assertRequests(t)
	assertE2EAPI(t, result)
}

// newE2EProviderServer starts the deterministic provider on an explicit IPv4 loopback listener.
func newE2EProviderServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for E2E provider: %v", err)
	}
	provider := httptest.NewUnstartedServer(handler)
	provider.Listener = listener
	provider.Start()
	return provider
}

// TestE2ELive verifies the explicitly enabled real-provider path with structural assertions.
func TestE2ELive(t *testing.T) {
	if os.Getenv("E2E_LIVE") != "1" {
		t.Skip("set E2E_LIVE=1 to contact real enrichment providers")
	}
	root := e2eRepositoryRoot(t)
	mode := e2eMode{
		name: "live", enrichment: true, live: true,
		sources:        []e2eSource{{name: "live_csv", filename: "live.csv", fileType: "csv", count: 2}},
		expectedParsed: 2, expectedUnique: 2, expectedValid: 2, expectedDiscarded: 0,
		expectedTitles: []string{"Live ORCID Name Search", "Live Provider Enrichment Candidate"},
	}
	result := runE2EVariant(t, root, mode, nil)
	assertE2EAPI(t, result)
}

// runE2EVariant generates configuration, invokes the supported binary, and validates its databases.
func runE2EVariant(t *testing.T, root string, mode e2eMode, environment []string) e2eResult {
	t.Helper()
	outputDir := prepareE2EOutput(t, root, mode.name)
	configPath := writeE2EConfig(t, root, outputDir, mode)
	validateE2EConfig(t, root, configPath)
	dbPath := filepath.Join(outputDir, "corpus.metadata.db")
	command := exec.Command(filepath.Join(root, "build", "analysis"), "run", "--db", dbPath, "--config", configPath, "--fresh")
	command.Dir = root
	if environment != nil {
		command.Env = environment
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run %s E2E pipeline: %v\n%s", mode.name, err, output)
	}
	return assertE2EDatabases(t, root, dbPath, mode)
}

// e2eRepositoryRoot resolves the repository root from the main package working directory.
func e2eRepositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(directory)
	for _, path := range []string{filepath.Join(root, "Makefile"), filepath.Join(root, "config", "database.something"), filepath.Join(root, "build", "analysis"), filepath.Join(root, "build", "something-json")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required E2E input %s is unavailable: %v", path, err)
		}
	}
	return root
}

// prepareE2EOutput recreates one known target-owned variant directory under build/e2e.
func prepareE2EOutput(t *testing.T, root, variant string) string {
	t.Helper()
	allowed := map[string]bool{"deterministic": true, "mocked": true, "live": true}
	if !allowed[variant] {
		t.Fatalf("unsupported E2E output variant %q", variant)
	}
	directory := filepath.Join(root, "build", "e2e", variant)
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative != filepath.Join("build", "e2e", variant) {
		t.Fatalf("unsafe E2E output directory %q", directory)
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatalf("remove stale E2E output %s: %v", directory, err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create E2E output %s: %v", directory, err)
	}
	return directory
}

// writeE2EConfig writes a typed workspace configuration and its relative include.
func writeE2EConfig(t *testing.T, root, outputDir string, mode e2eMode) string {
	t.Helper()
	baseline, err := os.ReadFile(filepath.Join(root, "config", "baseline.something"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "baseline.something"), baseline, 0o644); err != nil {
		t.Fatal(err)
	}
	sourceBlocks := make([]string, 0, len(mode.sources))
	for _, source := range mode.sources {
		path := filepath.Join(root, "src", "testdata", "e2e", source.filename)
		patchFields := "mapping(string, string) {}"
		if source.fileType == "bib" {
			patchFields = `mapping(string, string) { ["author"] => "authors", ["cited-references"] => "cited_references" }`
		}
		sourceBlocks = append(sourceBlocks, fmt.Sprintf(`source_declaration {
                name = %q,
                date = "2026-08-03 00:00:00",
                expected_file = %q,
                file_type = %q,
                query = "E2E(%s)",
                filters = []raw_data_filters { { filters = [.NO_FILTER], count = %d } },
                expected_result_count = %d,
                requested_fields = []string { "doi", "title", "authors", "references" },
                patch_fields = %s,
                keep_fields = []string { "doi", "title", "year", "authors", "affiliations", "publisher", "cited_references" },
            }`, source.name, path, source.fileType, source.name, source.count, source.count, patchFields))
	}
	providers := "[]"
	if mode.enrichment {
		providers = e2eProviderConfig(mode)
	}
	config := fmt.Sprintf(`#include("baseline.something");

#iteration("_workspace"): scope = {
    workspace: workspace_config = {
        format_version = 2,
        search_id = "e2e-%s",
        search_revision = "current",
        enrichment_enabled = %t,
        reuse_policy = reuse_policy_config { policy = "reuse_completed", },
        cache_policy = cache_policy_config {
            reads = []cache_policy_read_options { .GLOBAL, .NETWORK },
            writes = []cache_policy_write_options { .ACTIVE_RUN, .GLOBAL },
            negative_ttl_days = 7,
        },
        sources = [%s
        ],
        enrichment_providers = %s,
    };
}
`, mode.name, mode.enrichment, strings.Join(sourceBlocks, ",\n"), providers)
	configPath := filepath.Join(outputDir, "workspace.something")
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

// e2eProviderConfig returns concrete local or live provider declarations.
func e2eProviderConfig(mode e2eMode) string {
	crossrefBase := "https://api.crossref.org/works/"
	openAlexBase := "https://api.openalex.org/works/"
	openAlexAuthor := "https://api.openalex.org/authors/"
	orcidBase := "https://pub.orcid.org/v3.0/"
	orcidSearch := "https://pub.orcid.org/v3.0/search"
	ratePerSecond := 2
	if !mode.live {
		crossrefBase = mode.providerBaseURL + "/crossref/"
		openAlexBase = mode.providerBaseURL + "/openalex/works/"
		openAlexAuthor = mode.providerBaseURL + "/openalex/authors/"
		orcidBase = mode.providerBaseURL + "/orcid/record/"
		orcidSearch = mode.providerBaseURL + "/orcid/search"
		ratePerSecond = 1000
	}
	return fmt.Sprintf(`[
            enrichment_provider_config {
                name = "crossref", base_url = %q,
                user_agent = "research-analysis-e2e/1.0", contact_email = "e2e@example.invalid",
                rate_per_second = %d, concurrency = 1, timeout_seconds = 20, max_retries = 1,
                fields = []string { "publisher", "references", "authors" }, fill_missing_only = true,
            },
            enrichment_provider_config {
                name = "openalex", base_url = %q,
                user_agent = "research-analysis-e2e/1.0", contact_email = "e2e@example.invalid",
                rate_per_second = %d, concurrency = 1, timeout_seconds = 20, max_retries = 1,
                fields = []string { "abstract", "citation_count" }, fill_missing_only = true,
                extra_urls = mapping(string, string) { ["author"] => %q }, batch_size = 50,
            },
            enrichment_provider_config {
                name = "orcid", base_url = %q,
                user_agent = "research-analysis-e2e/1.0", contact_email = "e2e@example.invalid",
                rate_per_second = %d, concurrency = 1, timeout_seconds = 20, max_retries = 1,
                fields = []string { "orcid", "display_name", "institution" }, fill_missing_only = true,
                extra_urls = mapping(string, string) { ["search"] => %q },
            },
        ]`, crossrefBase, ratePerSecond, openAlexBase, ratePerSecond, openAlexAuthor, orcidBase, ratePerSecond, orcidSearch)
}

// validateE2EConfig evaluates the generated configuration through the maintained tool.
func validateE2EConfig(t *testing.T, root, configPath string) {
	t.Helper()
	command := exec.Command(filepath.Join(root, "build", "something-json"), configPath)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("validate generated E2E config: %v\n%s", err, output)
	}
}

// e2eOfflineEnvironment blocks accidental non-loopback provider traffic from the pipeline subprocess.
func e2eOfflineEnvironment(providerURL string) []string {
	blocked := map[string]bool{
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "http_proxy": true, "https_proxy": true,
		"NO_PROXY": true, "no_proxy": true,
	}
	environment := make([]string, 0, len(os.Environ())+6)
	for _, value := range os.Environ() {
		key := strings.SplitN(value, "=", 2)[0]
		if !blocked[key] {
			environment = append(environment, value)
		}
	}
	noProxy := "127.0.0.1,localhost"
	if strings.Contains(providerURL, "[::1]") {
		noProxy += ",::1"
	}
	return append(environment,
		"HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1",
		"http_proxy=http://127.0.0.1:1", "https_proxy=http://127.0.0.1:1",
		"NO_PROXY="+noProxy, "no_proxy="+noProxy,
	)
}

// ServeHTTP returns deterministic provider envelopes for every supported enrichment path.
func (m *providerMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.requests = append(m.requests, r.URL.RequestURI())
	m.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch {
	case r.URL.Path == "/crossref/"+e2eCandidateDOI:
		_, _ = w.Write([]byte(`{"message":{"title":["Provider Enrichment Candidate"],"publisher":"Mock Publisher","author":[{"given":"Olga","family":"Risovannaya","ORCID":"https://orcid.org/0000-0003-0779-1055"},{"given":"Open","family":"Alex","ORCID":"https://orcid.org/0000-0002-1825-0097"}],"reference":[{"DOI":"10.5555/e2e-mock-reference","article-title":"Mock Reference","year":"2020"}]}}`))
	case r.URL.Path == "/openalex/works/doi:"+e2eCandidateDOI:
		_, _ = w.Write([]byte(`{"id":"https://openalex.org/W123","display_name":"Provider Enrichment Candidate","abstract_inverted_index":{"Mock":[0],"abstract":[1]},"cited_by_count":7,"referenced_works":["https://openalex.org/W456"]}`))
	case r.URL.Path == "/openalex/works" && r.URL.Query().Get("filter") == "openalex_id:W456":
		_, _ = w.Write([]byte(`{"results":[{"id":"https://openalex.org/W456","doi":"https://doi.org/10.5555/e2e-openalex-reference","title":"OpenAlex Reference","publication_year":2021}]}`))
	case r.URL.Path == "/openalex/authors/orcid:"+e2eCandidateORCID:
		w.WriteHeader(http.StatusNotFound)
	case r.URL.Path == "/openalex/authors/orcid:"+e2eOpenAlexORCID:
		_, _ = w.Write([]byte(`{"id":"https://openalex.org/A456","display_name":"Open Alex","orcid":"https://orcid.org/0000-0002-1825-0097","works_count":3,"cited_by_count":5,"summary_stats":{"h_index":2,"i10_index":1},"last_known_institutions":[{"display_name":"Mock University"}]}`))
	case r.URL.Path == "/orcid/record/"+e2eCandidateORCID:
		_, _ = w.Write([]byte(`{"person":{"name":{"given-names":{"value":"Olga"},"family-name":{"value":"Risovannaya"},"credit-name":{"value":"Olga Risovannaya"}}}}`))
	case r.URL.Path == "/orcid/search" && r.URL.Query().Get("q") != "":
		_, _ = w.Write([]byte(`{"result":[{"orcid-identifier":{"path":"0000-0002-1825-0097"}}]}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// assertRequests verifies all expected provider families were exercised through loopback.
func (m *providerMock) assertRequests(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	requests := append([]string(nil), m.requests...)
	m.mu.Unlock()
	for _, prefix := range []string{"/crossref/", "/openalex/works/", "/openalex/works?", "/openalex/authors/orcid:" + e2eCandidateORCID, "/openalex/authors/orcid:" + e2eOpenAlexORCID, "/orcid/record/", "/orcid/search?"} {
		found := false
		for _, request := range requests {
			if strings.HasPrefix(request, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("mock provider did not receive %s request; got %v", prefix, requests)
		}
	}
}

// assertE2EDatabases validates persisted pipeline, audit, cache, and PDF evidence.
func assertE2EDatabases(t *testing.T, root, dbPath string, mode e2eMode) e2eResult {
	t.Helper()
	db, err := database.Open(dbPath, filepath.Join(root, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	var runID int64
	var status string
	if err := db.DB.QueryRow("SELECT id, status FROM pipeline_runs ORDER BY id DESC LIMIT 1").Scan(&runID, &status); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if status != "completed" {
		db.Close()
		t.Fatalf("%s pipeline status = %q, want completed", mode.name, status)
	}
	assertE2ECount(t, db, "run_sources", "pipeline_run_id=?", runID, len(mode.sources))
	assertE2ECount(t, db, "source_records", "run_source_id IN (SELECT id FROM run_sources WHERE pipeline_run_id=?)", runID, e2eRawCount(mode.sources))
	assertE2ECount(t, db, "source_records", "parse_status='parsed' AND run_source_id IN (SELECT id FROM run_sources WHERE pipeline_run_id=?)", runID, mode.expectedParsed)
	assertE2ECount(t, db, "source_records", "parse_status='rejected' AND run_source_id IN (SELECT id FROM run_sources WHERE pipeline_run_id=?)", runID, e2eRawCount(mode.sources)-mode.expectedParsed)
	assertE2ECount(t, db, "works", "1=1", nil, mode.expectedUnique)
	assertE2ECount(t, db, "run_work_stages", "pipeline_run_id=? AND stage_name='validate' AND outcome='valid'", runID, mode.expectedValid)
	assertE2ECount(t, db, "run_work_stages", "pipeline_run_id=? AND stage_name='validate' AND outcome='discarded'", runID, mode.expectedDiscarded)
	assertE2ECount(t, db, "work_revisions", "pipeline_run_id=? AND producer_stage='normalize'", runID, mode.expectedValid)
	assertE2ECount(t, db, "run_artifacts", "pipeline_run_id=?", runID, 3)
	for metric, want := range map[string]int{
		"input_records": e2eRawCount(mode.sources), "parsed_articles": mode.expectedParsed,
		"deduplicated_articles": mode.expectedUnique, "duplicate_articles": mode.expectedParsed - mode.expectedUnique,
		"valid_articles": mode.expectedValid, "discarded_articles": mode.expectedDiscarded,
		"normalized_articles_processed": mode.expectedValid,
	} {
		value, err := db.Metrics.Get(runID, metric, "")
		if err != nil || value == nil || value.Value != want {
			db.Close()
			t.Fatalf("%s metric %s = %+v, %v; want %d", mode.name, metric, value, err, want)
		}
	}
	titles, workIDs := e2ENormalizedWorks(t, db, runID)
	if !equalE2EStrings(titles, mode.expectedTitles) {
		db.Close()
		t.Fatalf("%s normalized titles = %v, want %v", mode.name, titles, mode.expectedTitles)
	}
	assertE2EAuditOrder(t, db, runID)
	assertE2ECount(t, db, "pdf_store_binding", "id=1 AND relative_path='corpus.pdf.db'", nil, 1)
	assertE2ECount(t, db, "pdf_audit_links", "audit_event_id IN (SELECT id FROM audit_events WHERE pipeline_run_id=? AND action='pdf_inventory_registered')", runID, mode.expectedValid)
	if mode.enrichment {
		assertE2EAtLeast(t, db, "cache_entries", "1=1", nil, 1)
		assertE2EAtLeast(t, db, "run_cache_uses", "pipeline_run_id=?", runID, 1)
		assertE2EAtLeast(t, db, "audit_events", "pipeline_run_id=? AND action='network_fetch'", runID, 1)
		assertE2EAtLeast(t, db, "audit_events", "pipeline_run_id=? AND action='field_enriched'", runID, 1)
		assertE2EAtLeast(t, db, "people", "orcid<>''", nil, 1)
		assertE2EAtLeast(t, db, "author_identity_resolutions", "pipeline_run_id=?", runID, 1)
		expectedCitationName := "Alice Example"
		if mode.live {
			expectedCitationName = "Live Name Search Author"
		}
		assertE2EAtLeast(t, db, "author_identity_resolutions", "queried_citation_name=?", expectedCitationName, 1)
		if !mode.live {
			assertE2EAtLeast(t, db, "author_identity_candidates", "1=1", nil, 1)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(filepath.Dir(dbPath), pdfstore.DefaultStoreFilename)
	pdf, err := pdfstore.Open(pdfPath, filepath.Join(root, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	assertE2EPDFCount(t, pdf, "pdf_documents", "status='not_available'", mode.expectedValid)
	assertE2EPDFCount(t, pdf, "pdf_audit_outbox", "delivered_at IS NOT NULL", mode.expectedValid)
	if err := pdf.Close(); err != nil {
		t.Fatal(err)
	}
	return e2eResult{dbPath: dbPath, runID: runID, workIDs: workIDs, titles: titles}
}

// e2eRawCount returns the total declared fixture record count.
func e2eRawCount(sources []e2eSource) int {
	total := 0
	for _, source := range sources {
		total += source.count
	}
	return total
}

// assertE2ECount checks an exact table row count using one optional query argument.
func assertE2ECount(t *testing.T, db *database.Database, table, where string, argument any, want int) {
	t.Helper()
	var got int
	var err error
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + where
	if argument == nil {
		err = db.DB.QueryRow(query).Scan(&got)
	} else {
		err = db.DB.QueryRow(query, argument).Scan(&got)
	}
	if err != nil || got != want {
		t.Fatalf("%s count with %q = %d, %v; want %d", table, where, got, err, want)
	}
}

// assertE2EAtLeast checks a minimum table row count using one optional query argument.
func assertE2EAtLeast(t *testing.T, db *database.Database, table, where string, argument any, minimum int) {
	t.Helper()
	var got int
	var err error
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + where
	if argument == nil {
		err = db.DB.QueryRow(query).Scan(&got)
	} else {
		err = db.DB.QueryRow(query, argument).Scan(&got)
	}
	if err != nil || got < minimum {
		t.Fatalf("%s count with %q = %d, %v; want at least %d", table, where, got, err, minimum)
	}
}

// assertE2EPDFCount checks an exact row count in the companion PDF store.
func assertE2EPDFCount(t *testing.T, store *pdfstore.Store, table, where string, want int) {
	t.Helper()
	var got int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&got); err != nil || got != want {
		t.Fatalf("PDF %s count with %q = %d, %v; want %d", table, where, got, err, want)
	}
}

// e2ENormalizedWorks returns sorted normalized titles and their stable work IDs.
func e2ENormalizedWorks(t *testing.T, db *database.Database, runID int64) ([]string, map[string]int64) {
	t.Helper()
	rows, err := db.DB.Query(`SELECT wr.title, wr.work_id FROM work_revisions wr
        WHERE wr.pipeline_run_id=? AND wr.producer_stage='normalize' ORDER BY wr.title`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var titles []string
	workIDs := make(map[string]int64)
	for rows.Next() {
		var title string
		var workID int64
		if err := rows.Scan(&title, &workID); err != nil {
			t.Fatal(err)
		}
		titles = append(titles, title)
		workIDs[title] = workID
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return titles, workIDs
}

// assertE2EAuditOrder verifies the pipeline's cross-database terminal event order.
func assertE2EAuditOrder(t *testing.T, db *database.Database, runID int64) {
	t.Helper()
	rows, err := db.DB.Query("SELECT action FROM audit_events WHERE pipeline_run_id=? ORDER BY id", runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
	}
	positions := make([]int, 0, 4)
	for _, expected := range []string{"run_started", "validation_changed", "pdf_inventory_registered", "run_completed"} {
		position := -1
		for index, action := range actions {
			if action == expected {
				position = index
				break
			}
		}
		if position < 0 {
			t.Fatalf("audit action %q missing from %v", expected, actions)
		}
		positions = append(positions, position)
	}
	if !(positions[0] < positions[1] && positions[1] < positions[2] && positions[2] < positions[3]) {
		t.Fatalf("audit action order %v does not satisfy run, validation, PDF, completion", actions)
	}
}

// assertE2EAPI compares read-only HTTP responses with the database assertions.
func assertE2EAPI(t *testing.T, result e2eResult) {
	t.Helper()
	viewer, err := server.Open(result.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	health := requestE2EJSON(t, handler, "/api/health")
	if health["readable"] != true {
		t.Fatalf("viewer health = %#v", health)
	}
	searches := requestE2EJSON(t, handler, "/api/searches")
	if !strings.Contains(mustE2EJSON(t, searches), "e2e-") {
		t.Fatalf("viewer searches do not contain E2E context: %#v", searches)
	}
	revisionID := nestedE2EID(t, searches, "searches", "revisions")
	plans := requestE2EJSON(t, handler, "/api/plans?search_revision_id="+strconv.FormatInt(revisionID, 10))
	if !strings.Contains(mustE2EJSON(t, plans), "execution_fingerprint") {
		t.Fatalf("viewer plans = %#v", plans)
	}
	runs := requestE2EJSON(t, handler, "/api/runs?search_revision_id="+strconv.FormatInt(revisionID, 10))
	if !strings.Contains(mustE2EJSON(t, runs), `"status":"completed"`) {
		t.Fatalf("viewer runs = %#v", runs)
	}
	base := "/api/runs/" + strconv.FormatInt(result.runID, 10)
	corpus := requestE2EJSON(t, handler, base+"/corpus/articles?per_page=20&sort=title&order=asc")
	evaluation := requestE2EJSON(t, handler, base+"/evaluation?per_page=20&sort=title&order=asc")
	audit := requestE2EJSON(t, handler, base+"/audit")
	for _, title := range result.titles {
		if !strings.Contains(mustE2EJSON(t, corpus), title) || !strings.Contains(mustE2EJSON(t, evaluation), title) {
			t.Errorf("viewer corpus or evaluation is missing normalized title %q", title)
		}
		workID := result.workIDs[title]
		status := requestE2EJSON(t, handler, "/api/works/"+strconv.FormatInt(workID, 10)+"/pdf-status")
		if status["status"] != "not_available" {
			t.Errorf("viewer PDF status for %q = %#v", title, status)
		}
	}
	for _, action := range []string{"run_started", "validation_changed", "pdf_inventory_registered", "run_completed"} {
		if !strings.Contains(mustE2EJSON(t, audit), action) {
			t.Errorf("viewer audit is missing %q", action)
		}
	}
}

// requestE2EJSON invokes one read-only viewer route and decodes its object response.
func requestE2EJSON(t *testing.T, handler http.Handler, path string) map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d: %s", path, response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode GET %s: %v", path, err)
	}
	return payload
}

// nestedE2EID reads the first nested object ID from a viewer collection response.
func nestedE2EID(t *testing.T, payload map[string]any, collection, nested string) int64 {
	t.Helper()
	items, ok := payload[collection].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("viewer %s collection is empty: %#v", collection, payload)
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("viewer %s item has unexpected shape: %#v", collection, items[0])
	}
	nestedItems, ok := item[nested].([]any)
	if !ok || len(nestedItems) == 0 {
		t.Fatalf("viewer %s nested collection is empty: %#v", nested, item)
	}
	nestedItem, ok := nestedItems[0].(map[string]any)
	if !ok {
		t.Fatalf("viewer %s item has unexpected shape: %#v", nested, nestedItems[0])
	}
	id, ok := nestedItem["id"].(float64)
	if !ok || id < 1 {
		t.Fatalf("viewer %s ID has unexpected value: %#v", nested, nestedItem["id"])
	}
	return int64(id)
}

// mustE2EJSON serializes a decoded payload for compact membership assertions.
func mustE2EJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// equalE2EStrings compares two string sets after sorting copies.
func equalE2EStrings(left, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}
