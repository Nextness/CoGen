// Package e2eflow verifies the supported analysis binary from pipeline input
// through persisted evidence, the read-only API, and generated viewer data.

package e2eflow

import (
	"analysis/backend/database/pdfbinding"
	"analysis/backend/pdfstore"
	"analysis/backend/server"
	"analysis/tests/support/go/pdffixture"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"

	"context"
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
	CandidateDOI   = "10.17116/rosstomat20251801140"
	CandidateORCID = "0000-0003-0779-1055"
	OpenAlexORCID  = "0000-0002-1825-0097"
)

// Mode describes one pipeline variant and its expected persisted result.
type Mode struct {
	Name              string
	Enrichment        bool
	Live              bool
	ProviderBaseURL   string
	Sources           []Source
	ExpectedParsed    int
	ExpectedUnique    int
	ExpectedValid     int
	ExpectedDiscarded int
	ExpectedTitles    []string
}

// Source describes one tracked input used by a generated workspace configuration.
type Source struct {
	Name     string
	Filename string
	FileType string
	Count    int
}

// Result identifies the generated database and rows needed by API assertions.
type Result struct {
	DBPath  string
	RunID   int64
	WorkIDs map[string]int64
	Titles  []string
}

// ProviderMock records requests served by the local deterministic provider.
type ProviderMock struct {
	Mu       sync.Mutex
	Requests []string
}

// NewE2EProviderServer starts the deterministic provider on an explicit IPv4 loopback listener.
func NewE2EProviderServer(t *testing.T, handler http.Handler) *httptest.Server {
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

// RunE2EVariant generates configuration, invokes the supported binary, and validates its databases.
func RunE2EVariant(t *testing.T, root string, mode Mode, environment []string) Result {
	if mode.Live && os.Getenv("E2E_LIVE") != "1" {
		t.Fatal("live provider traffic requires E2E_LIVE=1")
	}
	t.Helper()
	outputDir := prepareE2EOutput(t, root, mode.Name)
	configPath := writeE2EConfig(t, root, outputDir, mode)
	validateE2EConfig(t, root, configPath)
	DBPath := filepath.Join(outputDir, "corpus.metadata.db")
	command := exec.Command(filepath.Join(root, "build", "analysis"), "run", "--db", DBPath, "--config", configPath, "--fresh")
	command.Dir = root
	if environment != nil {
		command.Env = environment
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run %s E2E pipeline: %v\n%s", mode.Name, err, output)
	}
	return assertE2EDatabases(t, root, DBPath, mode)
}

// E2ERepositoryRoot resolves the repository root from the centralized test environment.
func E2ERepositoryRoot(t *testing.T) string {
	t.Helper()
	root := testenv.Root()
	for _, path := range []string{filepath.Join(root, "Makefile"), filepath.Join(root, "config", "database.something"), filepath.Join(root, "build", "analysis"), filepath.Join(root, "build", "something-printer"), filepath.Join(root, "build", "pdf-store")} {
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
func writeE2EConfig(t *testing.T, root, outputDir string, mode Mode) string {
	t.Helper()
	baseline, err := os.ReadFile(filepath.Join(root, "config", "baseline.something"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "baseline.something"), baseline, 0o644); err != nil {
		t.Fatal(err)
	}
	sourceBlocks := make([]string, 0, len(mode.Sources))
	for _, source := range mode.Sources {
		domain := "offline"
		if mode.Live {
			domain = "live"
		}
		path := filepath.Join(root, "tests", "e2e", domain, "testdata", source.Filename)
		patchFields := "mapping(string, string) {}"
		if source.FileType == "bib" {
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
            }`, source.Name, path, source.FileType, source.Name, source.Count, source.Count, patchFields))
	}
	providers := "[]"
	if mode.Enrichment {
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
        reviewer = reviewer_config { username = "E2E Reviewer", email = "e2e-reviewer@example.invalid", },
        sources = [%s
        ],
        enrichment_providers = %s,
    };
}
`, mode.Name, mode.Enrichment, strings.Join(sourceBlocks, ",\n"), providers)
	configPath := filepath.Join(outputDir, "workspace.something")
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

// PrepareE2EReviewRuns inventories a valid PDF through the supported tool and creates A2 as a fresh overlapping run.
func PrepareE2EReviewRuns(t *testing.T, root string, result Result) {
	t.Helper()
	pdfPath := filepath.Join(filepath.Dir(result.DBPath), "review.fixture.pdf")
	if err := os.WriteFile(pdfPath, pdffixture.Bytes("Selectable E2E methods on page one", "Selectable E2E results on page two"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(root, "build", "pdf-store"), "add", "--db", result.DBPath, "--doi", "10.5555/e2e-complete-one", "--file", pdfPath)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("inventory deterministic E2E PDF: %v\n%s", err, output)
	}
	configPath := filepath.Join(filepath.Dir(result.DBPath), "workspace.something")
	command = exec.Command(filepath.Join(root, "build", "analysis"), "run", "--db", result.DBPath, "--config", configPath, "--fresh")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create deterministic E2E A2 run: %v\n%s", err, output)
	}
	db, err := testfixture.OpenExistingPlatform(t, result.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var completed int
	if err := db.Conn.QueryRow("SELECT COUNT(*) FROM pipeline_runs WHERE status='completed'").Scan(&completed); err != nil || completed != 2 {
		t.Fatalf("completed deterministic review runs=%d err=%v", completed, err)
	}
}

// e2eProviderConfig returns concrete local or live provider declarations.
func e2eProviderConfig(mode Mode) string {
	crossrefBase := "https://api.crossref.org/works/"
	openAlexBase := "https://api.openalex.org/works/"
	openAlexAuthor := "https://api.openalex.org/authors/"
	orcidBase := "https://pub.orcid.org/v3.0/"
	orcidSearch := "https://pub.orcid.org/v3.0/search"
	ratePerSecond := 2
	if !mode.Live {
		crossrefBase = mode.ProviderBaseURL + "/crossref/"
		openAlexBase = mode.ProviderBaseURL + "/openalex/works/"
		openAlexAuthor = mode.ProviderBaseURL + "/openalex/authors/"
		orcidBase = mode.ProviderBaseURL + "/orcid/record/"
		orcidSearch = mode.ProviderBaseURL + "/orcid/search"
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
                fields = []string { "abstract", "citation_count", "references", "authors" }, fill_missing_only = true,
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
	command := exec.Command(filepath.Join(root, "build", "something-printer"), "--json", configPath)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("validate generated E2E config: %v\n%s", err, output)
	}
}

// E2EOfflineEnvironment blocks accidental non-loopback provider traffic from the pipeline subprocess.
func E2EOfflineEnvironment(providerURL string) []string {
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
func (m *ProviderMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Mu.Lock()
	m.Requests = append(m.Requests, r.URL.RequestURI())
	m.Mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch {
	case r.URL.Path == "/crossref/"+CandidateDOI:
		_, _ = w.Write([]byte(`{"message":{"title":["Provider Enrichment Candidate"],"publisher":"Mock Publisher","author":[{"given":"Olga","family":"Risovannaya","ORCID":"https://orcid.org/0000-0003-0779-1055"},{"given":"Open","family":"Alex","ORCID":"https://orcid.org/0000-0002-1825-0097"}],"reference":[{"DOI":"10.5555/e2e-mock-reference","article-title":"Mock Reference","year":"2020"}]}}`))
	case r.URL.Path == "/openalex/works/doi:"+CandidateDOI:
		_, _ = w.Write([]byte(`{"id":"https://openalex.org/W123","display_name":"Provider Enrichment Candidate","abstract_inverted_index":{"Mock":[0],"abstract":[1]},"cited_by_count":7,"referenced_works":["https://openalex.org/W456"]}`))
	case r.URL.Path == "/openalex/works" && r.URL.Query().Get("filter") == "openalex_id:W456":
		_, _ = w.Write([]byte(`{"results":[{"id":"https://openalex.org/W456","doi":"https://doi.org/10.5555/e2e-openalex-reference","title":"OpenAlex Reference","publication_year":2021}]}`))
	case r.URL.Path == "/openalex/authors/orcid:"+CandidateORCID:
		w.WriteHeader(http.StatusNotFound)
	case r.URL.Path == "/openalex/authors/orcid:"+OpenAlexORCID:
		_, _ = w.Write([]byte(`{"id":"https://openalex.org/A456","display_name":"Open Alex","orcid":"https://orcid.org/0000-0002-1825-0097","works_count":3,"cited_by_count":5,"summary_stats":{"h_index":2,"i10_index":1},"last_known_institutions":[{"display_name":"Mock University"}]}`))
	case r.URL.Path == "/orcid/record/"+CandidateORCID:
		_, _ = w.Write([]byte(`{"person":{"name":{"given-names":{"value":"Olga"},"family-name":{"value":"Risovannaya"},"credit-name":{"value":"Olga Risovannaya"}}}}`))
	case r.URL.Path == "/orcid/search" && r.URL.Query().Get("q") != "":
		_, _ = w.Write([]byte(`{"result":[{"orcid-identifier":{"path":"0000-0002-1825-0097"}}]}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// AssertRequests verifies all expected provider families were exercised through loopback.
func (m *ProviderMock) AssertRequests(t *testing.T) {
	t.Helper()
	m.Mu.Lock()
	Requests := append([]string(nil), m.Requests...)
	m.Mu.Unlock()
	for _, prefix := range []string{"/crossref/", "/openalex/works/", "/openalex/works?", "/openalex/authors/orcid:" + CandidateORCID, "/openalex/authors/orcid:" + OpenAlexORCID, "/orcid/record/", "/orcid/search?"} {
		found := false
		for _, request := range Requests {
			if strings.HasPrefix(request, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("mock provider did not receive %s request; got %v", prefix, Requests)
		}
	}
}

// assertE2EDatabases validates persisted pipeline, audit, cache, and PDF evidence.
func assertE2EDatabases(t *testing.T, root, DBPath string, mode Mode) Result {
	t.Helper()
	db, err := testfixture.OpenPlatform(t, DBPath, filepath.Join(root, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	var RunID int64
	var status string
	if err := db.Conn.QueryRow("SELECT id, status FROM pipeline_runs ORDER BY id DESC LIMIT 1").Scan(&RunID, &status); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if status != "completed" {
		db.Close()
		t.Fatalf("%s pipeline status = %q, want completed", mode.Name, status)
	}
	AssertE2ECount(t, db, "run_sources", "pipeline_run_id=?", RunID, len(mode.Sources))
	AssertE2ECount(t, db, "source_records", "run_source_id IN (SELECT id FROM run_sources WHERE pipeline_run_id=?)", RunID, e2eRawCount(mode.Sources))
	AssertE2ECount(t, db, "source_records", "parse_status='parsed' AND run_source_id IN (SELECT id FROM run_sources WHERE pipeline_run_id=?)", RunID, mode.ExpectedParsed)
	AssertE2ECount(t, db, "source_records", "parse_status='rejected' AND run_source_id IN (SELECT id FROM run_sources WHERE pipeline_run_id=?)", RunID, e2eRawCount(mode.Sources)-mode.ExpectedParsed)
	AssertE2ECount(t, db, "works", "1=1", nil, mode.ExpectedUnique)
	AssertE2ECount(t, db, "run_work_stages", "pipeline_run_id=? AND stage_name='validate' AND outcome='valid'", RunID, mode.ExpectedValid)
	AssertE2ECount(t, db, "run_work_stages", "pipeline_run_id=? AND stage_name='validate' AND outcome='discarded'", RunID, mode.ExpectedDiscarded)
	AssertE2ECount(t, db, "work_revisions", "pipeline_run_id=? AND producer_stage='normalize'", RunID, mode.ExpectedValid)
	AssertE2ECount(t, db, "run_artifacts", "pipeline_run_id=?", RunID, 3)
	for metric, want := range map[string]int{
		"input_records": e2eRawCount(mode.Sources), "parsed_articles": mode.ExpectedParsed,
		"deduplicated_articles": mode.ExpectedUnique, "duplicate_articles": mode.ExpectedParsed - mode.ExpectedUnique,
		"valid_articles": mode.ExpectedValid, "discarded_articles": mode.ExpectedDiscarded,
		"normalized_articles_processed": mode.ExpectedValid,
	} {
		value, err := db.Run.GetMetric(context.Background(), RunID, metric, "")
		if err != nil || value == nil || value.Value != want {
			db.Close()
			t.Fatalf("%s metric %s = %+v, %v; want %d", mode.Name, metric, value, err, want)
		}
	}
	Titles, WorkIDs := e2ENormalizedWorks(t, db, RunID)
	if !equalE2EStrings(Titles, mode.ExpectedTitles) {
		db.Close()
		t.Fatalf("%s normalized titles = %v, want %v", mode.Name, Titles, mode.ExpectedTitles)
	}
	assertE2EAuditOrder(t, db, RunID)
	AssertE2ECount(t, db, "pdf_store_binding", "id=1 AND relative_path='corpus.pdf.db'", nil, 1)
	AssertE2ECount(t, db, "pdf_audit_links", "audit_event_id IN (SELECT id FROM audit_events WHERE pipeline_run_id=? AND action='pdf_inventory_registered')", RunID, mode.ExpectedValid)
	if mode.Enrichment {
		assertE2EAtLeast(t, db, "cache_entries", "1=1", nil, 1)
		assertE2EAtLeast(t, db, "run_cache_uses", "pipeline_run_id=?", RunID, 1)
		assertE2EAtLeast(t, db, "audit_events", "pipeline_run_id=? AND action='network_fetch'", RunID, 1)
		assertE2EAtLeast(t, db, "audit_events", "pipeline_run_id=? AND action='field_enriched'", RunID, 1)
		assertE2EAtLeast(t, db, "people", "orcid<>''", nil, 1)
		assertE2EAtLeast(t, db, "author_identity_resolutions", "pipeline_run_id=?", RunID, 1)
		expectedCitationName := "Alice Example"
		if mode.Live {
			expectedCitationName = "Live Name Search Author"
		}
		assertE2EAtLeast(t, db, "author_identity_resolutions", "queried_citation_name=?", expectedCitationName, 1)
		if !mode.Live {
			assertE2EAtLeast(t, db, "author_identity_candidates", "1=1", nil, 1)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(filepath.Dir(DBPath), pdfbinding.DefaultStoreFilename)
	pdf, err := pdfstore.Open(pdfPath, filepath.Join(root, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	assertE2EPDFCount(t, pdf, "pdf_documents", "status='not_available'", mode.ExpectedValid)
	assertE2EPDFCount(t, pdf, "pdf_audit_outbox", "delivered_at IS NOT NULL", mode.ExpectedValid)
	if err := pdf.Close(); err != nil {
		t.Fatal(err)
	}
	return Result{DBPath: DBPath, RunID: RunID, WorkIDs: WorkIDs, Titles: Titles}
}

// e2eRawCount returns the total declared fixture record count.
func e2eRawCount(Sources []Source) int {
	total := 0
	for _, source := range Sources {
		total += source.Count
	}
	return total
}

// AssertE2ECount checks an exact table row count using one optional query argument.
func AssertE2ECount(t *testing.T, db *testfixture.Database, table, where string, argument any, want int) {
	t.Helper()
	var got int
	var err error
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + where
	if argument == nil {
		err = db.Conn.QueryRow(query).Scan(&got)
	} else {
		err = db.Conn.QueryRow(query, argument).Scan(&got)
	}
	if err != nil || got != want {
		t.Fatalf("%s count with %q = %d, %v; want %d", table, where, got, err, want)
	}
}

// assertE2EAtLeast checks a minimum table row count using one optional query argument.
func assertE2EAtLeast(t *testing.T, db *testfixture.Database, table, where string, argument any, minimum int) {
	t.Helper()
	var got int
	var err error
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + where
	if argument == nil {
		err = db.Conn.QueryRow(query).Scan(&got)
	} else {
		err = db.Conn.QueryRow(query, argument).Scan(&got)
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
func e2ENormalizedWorks(t *testing.T, db *testfixture.Database, RunID int64) ([]string, map[string]int64) {
	t.Helper()
	rows, err := db.Conn.Query(`SELECT wr.title, wr.work_id FROM work_revisions wr
        WHERE wr.pipeline_run_id=? AND wr.producer_stage='normalize' ORDER BY wr.title`, RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var Titles []string
	WorkIDs := make(map[string]int64)
	for rows.Next() {
		var title string
		var workID int64
		if err := rows.Scan(&title, &workID); err != nil {
			t.Fatal(err)
		}
		Titles = append(Titles, title)
		WorkIDs[title] = workID
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return Titles, WorkIDs
}

// assertE2EAuditOrder verifies the pipeline's cross-database terminal event order.
func assertE2EAuditOrder(t *testing.T, db *testfixture.Database, RunID int64) {
	t.Helper()
	rows, err := db.Conn.Query("SELECT action FROM audit_events WHERE pipeline_run_id=? ORDER BY id", RunID)
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

// AssertE2EAPI compares read-only HTTP responses with the database assertions.
func AssertE2EAPI(t *testing.T, result Result) {
	t.Helper()
	viewer, err := server.Open(result.DBPath, testenv.Path("config", "database.something"))
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
	base := "/api/runs/" + strconv.FormatInt(result.RunID, 10)
	corpus := requestE2EJSON(t, handler, base+"/corpus/articles?per_page=20&sort=title&order=asc")
	evaluation := requestE2EJSON(t, handler, base+"/evaluation?per_page=20&sort=title&order=asc")
	auditActions := requestE2EAuditActions(t, handler, result.RunID)
	for _, title := range result.Titles {
		if !strings.Contains(mustE2EJSON(t, corpus), title) || !strings.Contains(mustE2EJSON(t, evaluation), title) {
			t.Errorf("viewer corpus or evaluation is missing normalized title %q", title)
		}
		workID := result.WorkIDs[title]
		status := requestE2EJSON(t, handler, "/api/works/"+strconv.FormatInt(workID, 10)+"/pdf-status")
		if status["status"] != "not_available" {
			t.Errorf("viewer PDF status for %q = %#v", title, status)
		}
	}
	for _, action := range []string{"run_started", "validation_changed", "pdf_inventory_registered", "run_completed"} {
		if !auditActions[action] {
			t.Errorf("viewer audit is missing %q", action)
		}
	}
}

// requestE2EAuditActions traverses the bounded canonical audit route for one generated run.
func requestE2EAuditActions(t *testing.T, handler http.Handler, RunID int64) map[string]bool {
	t.Helper()
	actions := make(map[string]bool)
	cursor := ""
	for page := 0; page < 100; page++ {
		path := "/api/audit?run_id=" + strconv.FormatInt(RunID, 10) + "&limit=100"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		payload := requestE2EJSON(t, handler, path)
		for _, raw := range payload["events"].([]any) {
			event := raw.(map[string]any)
			actions[event["action"].(string)] = true
		}
		if payload["has_more"] != true {
			return actions
		}
		next, ok := payload["next_cursor"].(float64)
		if !ok || next < 1 {
			t.Fatalf("viewer audit page %d has invalid cursor %#v", page, payload["next_cursor"])
		}
		cursor = strconv.FormatInt(int64(next), 10)
	}
	t.Fatal("viewer audit traversal exceeded 100 pages")
	return nil
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
