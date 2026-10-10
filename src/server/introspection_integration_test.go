// introspection_integration_test.go tests the retained schema-discovery and
// advanced table browser adapter: discovery allowlisting, forbidden tables,
// identifier quoting, row/cell/projection/response limits, query budgets, and
// injection rejection.
//
//go:build integration

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"analysis/database"
)

// TestAPITableBrowserErrors verifies api table browser errors.
func TestAPITableBrowserErrors(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{"/api/tables/nope", "/api/tables/works?per_page=21", "/api/tables/works?sort=id;DROP%20TABLE%20works", "/api/tables/works?order=sideways"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"error"`) {
			t.Errorf("GET %s did not return JSON API error", path)
		}
	}
}

// TestAPITableBrowserTruncatesMultibyteCellsByBytes verifies UTF-8 table cells honour the advertised byte limit.
func TestAPITableBrowserTruncatesMultibyteCellsByBytes(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	ids := make(map[int64]struct{})
	for _, value := range []string{
		strings.Repeat("é", advancedCellBytes/2+1),
		strings.Repeat("€", advancedCellBytes/3+1),
		strings.Repeat("😀", advancedCellBytes/4+1),
	} {
		result, err := viewer.writeDB.DB.Exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES (?, 0, 'text/plain')", value)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		ids[id] = struct{}{}
	}
	status, payload := requestJSON(t, viewer.Handler(), "/api/tables/artifacts?per_page=20&sort=id&order=desc")
	if status != http.StatusOK {
		t.Fatalf("artifact table status=%d body=%v", status, payload)
	}
	seen := 0
	for _, raw := range payload["rows"].([]any) {
		row := raw.(map[string]any)
		id := int64(row["id"].(float64))
		if _, ok := ids[id]; !ok {
			continue
		}
		value, ok := row["content_hash"].(string)
		if !ok || len(value) > advancedCellBytes || !utf8.ValidString(value) {
			t.Fatalf("artifact %d value bytes=%d valid=%v truncation=%v", id, len(value), utf8.ValidString(value), payload["truncated_fields"])
		}
		seen++
	}
	if seen != len(ids) {
		t.Fatalf("multibyte artifacts rendered=%d, want %d", seen, len(ids))
	}
	reasons := payload["truncated_fields"].(map[string]any)["content_hash"].([]any)
	if len(reasons) == 0 || reasons[0] != "cell_byte_limit" {
		t.Fatalf("multibyte truncation reasons = %v", reasons)
	}
}

// TestAPITableBrowserUsesSafeBoundedProjection verifies request-time counts, redaction, binary omission, and page canonicalization.
func TestAPITableBrowserUsesSafeBoundedProjection(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	status, discovery := requestJSON(t, handler, "/api/tables")
	if status != http.StatusOK {
		t.Fatalf("table discovery status=%d body=%v", status, discovery)
	}
	encodedDiscovery, err := json.Marshal(discovery)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedDiscovery), "row_count") {
		t.Fatalf("table discovery must not expose cached startup counts: %s", encodedDiscovery)
	}
	if strings.Contains(string(encodedDiscovery), `"name":"data"`) {
		t.Fatalf("binary artifact data must not be discoverable as a browsable field: %s", encodedDiscovery)
	}

	status, page := requestJSON(t, handler, "/api/tables/source_records?page=999&per_page=20&sort=parse_status&order=asc")
	if status != http.StatusOK {
		t.Fatalf("table page status=%d body=%v", status, page)
	}
	pagination := page["pagination"].(map[string]any)
	if pagination["page"] != pagination["total_pages"] {
		t.Fatalf("out-of-range page was not canonicalized: %v", pagination)
	}
	rows := page["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["raw_payload"] != "[redacted]" {
		t.Fatalf("sensitive payload was not redacted: %v", rows)
	}
	truncated := page["truncated_fields"].(map[string]any)
	if _, ok := truncated["raw_payload"]; !ok {
		t.Fatalf("redaction metadata is missing: %v", truncated)
	}
}

// TestIntrospectionDiscoveryExcludesForbiddenTables verifies SQLite-internal
// tables never become browsable through discovery or direct lookup.
func TestIntrospectionDiscoveryExcludesForbiddenTables(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	status, discovery := requestJSON(t, handler, "/api/tables")
	if status != http.StatusOK {
		t.Fatalf("table discovery status=%d body=%v", status, discovery)
	}
	names := make(map[string]bool)
	for _, raw := range discovery["tables"].([]any) {
		names[raw.(map[string]any)["name"].(string)] = true
	}
	if !names["works"] {
		t.Fatalf("discovery omitted the works table: %v", names)
	}
	for _, forbidden := range []string{"sqlite_sequence", "sqlite_master"} {
		if names[forbidden] {
			t.Errorf("discovery exposed forbidden table %q", forbidden)
		}
		response := viewerRequest(t, handler, "/api/tables/"+forbidden)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET /api/tables/%s status=%d, want 404", forbidden, response.Code)
		}
	}
}

// TestIntrospectionDiscoveryIsDeterministic verifies discovery returns tables
// in sorted order rather than map iteration order.
func TestIntrospectionDiscoveryIsDeterministic(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	status, discovery := requestJSON(t, viewer.Handler(), "/api/tables")
	if status != http.StatusOK {
		t.Fatalf("table discovery status=%d body=%v", status, discovery)
	}
	names := make([]string, 0)
	for _, raw := range discovery["tables"].([]any) {
		names = append(names, raw.(map[string]any)["name"].(string))
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("discovered tables are not sorted: %v", names)
	}
}

// TestIntrospectionProjectionLimitOmitsWideColumns verifies a wide table keeps
// its primary key and caps the projected columns at the documented limit.
func TestIntrospectionProjectionLimitOmitsWideColumns(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	db, err := database.OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	columns := make([]string, 0, 41)
	for index := 1; index <= 40; index++ {
		columns = append(columns, fmt.Sprintf("column_%02d TEXT", index))
	}
	columns = append(columns, "id INTEGER PRIMARY KEY")
	if _, err := db.DB.Exec("CREATE TABLE wide_evidence (" + strings.Join(columns, ", ") + ")"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO wide_evidence (column_01, column_40) VALUES ('first', 'last')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	status, page := requestJSON(t, viewer.Handler(), "/api/tables/wide_evidence?per_page=20")
	if status != http.StatusOK {
		t.Fatalf("wide table status=%d body=%v", status, page)
	}
	table := page["table"].(map[string]any)
	projected := table["columns"].([]any)
	if len(projected) != advancedProjectionLimit+1 {
		t.Fatalf("projected columns=%d, want %d", len(projected), advancedProjectionLimit+1)
	}
	last := projected[len(projected)-1].(map[string]any)
	if last["name"] != "id" || last["primary_key"] != true {
		t.Fatalf("primary key was not retained: %v", last)
	}
	omitted := table["omitted_columns"].(map[string]any)
	if len(omitted) != 8 {
		t.Fatalf("omitted columns=%d, want 8: %v", len(omitted), omitted)
	}
	for name, reason := range omitted {
		if reason != "projection_column_limit" {
			t.Errorf("omitted column %s reason=%v", name, reason)
		}
	}
}

// TestIntrospectionResponseByteBudgetOmitsExcessValues verifies the browser
// replaces values beyond the response byte budget instead of returning an
// oversized document.
func TestIntrospectionResponseByteBudgetOmitsExcessValues(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	db, err := database.OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("CREATE TABLE large_evidence (id INTEGER PRIMARY KEY, alpha TEXT, beta TEXT, gamma TEXT)"); err != nil {
		t.Fatal(err)
	}
	value := strings.Repeat("x", advancedCellBytes)
	for index := 0; index < 100; index++ {
		if _, err := db.DB.Exec("INSERT INTO large_evidence (alpha, beta, gamma) VALUES (?, ?, ?)", value, value, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	status, page := requestJSON(t, viewer.Handler(), "/api/tables/large_evidence?per_page=100")
	if status != http.StatusOK {
		t.Fatalf("large table status=%d body=%v", status, page)
	}
	omitted := 0
	for _, raw := range page["rows"].([]any) {
		row := raw.(map[string]any)
		for _, column := range []string{"alpha", "beta", "gamma"} {
			if row[column] == "[omitted: response byte budget]" {
				omitted++
			}
		}
	}
	if omitted == 0 {
		t.Fatal("response byte budget did not omit any value")
	}
	reasons := page["truncated_fields"].(map[string]any)
	found := false
	for _, column := range []string{"alpha", "beta", "gamma"} {
		for _, reason := range reasons[column].([]any) {
			if reason == "response_byte_limit" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("response byte limit reason missing: %v", reasons)
	}
}

// TestIntrospectionTableBrowserChargesQueryBudget verifies the browser's count
// and page statements flow through the request statement budget.
func TestIntrospectionTableBrowserChargesQueryBudget(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	ctx, budget := withQueryBudget(context.Background(), 1)
	request := httptest.NewRequest(http.MethodGet, "/api/tables/works?per_page=20", nil).WithContext(ctx)
	request.SetPathValue("table", "works")
	recorder := httptest.NewRecorder()
	viewer.tableRows(recorder, request)
	if used := budget.used.Load(); used != 2 {
		t.Fatalf("table browser used %d budgeted statements, want 2", used)
	}
	if !budget.exceeded.Load() {
		t.Fatal("table browser did not record the exceeded statement budget")
	}
}

// TestIntrospectionRejectsInjectionVectors verifies every dynamic identifier
// and bound value is validated before it reaches SQL.
func TestIntrospectionRejectsInjectionVectors(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()

	for _, route := range []string{
		`/api/tables/works%22%3B%20DROP%20TABLE%20works`,
		`/api/tables/works?sort=id%22%3B%20DROP%20TABLE%20works`,
		`/api/tables/works?sort=id%20DESC%3B%20DROP%20TABLE%20works`,
		`/api/tables/works?order=ASC%3B%20DROP%20TABLE%20works`,
		`/api/tables/works?page=1%3B%20DROP%20TABLE%20works`,
		`/api/tables/works?per_page=20%3B%20DROP%20TABLE%20works`,
		`/api/tables/works?evil=1`,
		`/api/tables/works?sort=id;DROP`,
	} {
		response := viewerRequest(t, handler, route)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s status=%d body=%s", route, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"error"`) {
			t.Errorf("GET %s did not return a JSON API error", route)
		}
	}

	var count int
	if err := viewer.db.QueryRow("SELECT COUNT(*) FROM works").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("works row count=%d after injection attempts, want 3", count)
	}
}

// TestIntrospectionQuotesDiscoveredIdentifiers verifies a discovered table and
// column containing quote characters are browsed through escaped identifiers.
func TestIntrospectionQuotesDiscoveredIdentifiers(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	db, err := database.OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`CREATE TABLE "weird""table" ("weird""column" TEXT, id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`INSERT INTO "weird""table" ("weird""column") VALUES ('quoted value')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	status, page := requestJSON(t, viewer.Handler(), `/api/tables/weird%22table?sort=weird%22column`)
	if status != http.StatusOK {
		t.Fatalf("quoted table status=%d body=%v", status, page)
	}
	rows := page["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)[`weird"column`] != "quoted value" {
		t.Fatalf("quoted table rows=%v", rows)
	}
}
