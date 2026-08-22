//go:build unit

package server

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"modernc.org/sqlite"
)

// TestAPIResponseByteBudgets verifies route classification and hard serialized limits.
func TestAPIResponseByteBudgets(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/api/health", compactAPIResponseBytes},
		{"/api/hierarchy", collectionAPIResponseBytes},
		{"/api/runs/1/articles/1/notes", collectionAPIResponseBytes},
		{"/api/audit/1/recorded-data", collectionAPIResponseBytes},
		{"/api/graph", detailAPIResponseBytes},
		{"/api/articles/1", detailAPIResponseBytes},
		{"/api/tables/work_revisions", detailAPIResponseBytes},
		{"/api/pdf/1", 0},
		{"/api/artifacts/1/content", 0},
		{"/styles/base.css", 0},
	}
	for _, test := range tests {
		if got := apiResponseByteBudget(test.path); got != test.want {
			t.Errorf("apiResponseByteBudget(%q)=%d, want %d", test.path, got, test.want)
		}
	}

	handler := withAPIResponseBudgets(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"payload": strings.Repeat("x", compactAPIResponseBytes)})
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"response_budget_exceeded"`) {
		t.Fatalf("oversized response code=%d body=%s", response.Code, response.Body.String())
	}
	if response.Body.Len() > compactAPIResponseBytes {
		t.Fatalf("budget error response size=%d, limit=%d", response.Body.Len(), compactAPIResponseBytes)
	}
	if got := response.Header().Get("X-Response-Byte-Limit"); got != fmt.Sprint(compactAPIResponseBytes) {
		t.Fatalf("response budget header=%q", got)
	}

	response = httptest.NewRecorder()
	withAPIResponseBudgets(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]bool{"bounded": true})
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/audit", nil))
	if response.Code != http.StatusAccepted || response.Body.String() != "{\"bounded\":true}\n" {
		t.Fatalf("bounded response code=%d body=%q", response.Code, response.Body.String())
	}
}

// TestAPIQueryBudgets verifies deterministic route ceilings, driver accounting, and hard rejection.
func TestAPIQueryBudgets(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/api/health", compactAPIQueries},
		{"/api/pdf/1", compactAPIQueries},
		{"/api/audit", collectionAPIQueries},
		{"/api/runs/1/cache-uses", collectionAPIQueries},
		{"/api/graph", detailAPIQueries},
		{"/api/runs/1/articles/1/notes", detailAPIQueries},
		{"/styles/base.css", 0},
	}
	for _, test := range tests {
		if got := apiQueryBudget(test.path); got != test.want {
			t.Errorf("apiQueryBudget(%q)=%d, want %d", test.path, got, test.want)
		}
	}

	db, err := sql.Open(queryBudgetDriverName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, state := withQueryBudget(t.Context(), 2)
	if _, err := db.ExecContext(ctx, "CREATE TABLE evidence (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO evidence DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM evidence").Scan(&count); !errors.Is(err, errQueryBudgetExceeded) {
		t.Fatalf("third query error=%v, want query budget error", err)
	}
	if used := state.used.Load(); used != 3 || !state.exceeded.Load() {
		t.Fatalf("query budget used=%d exceeded=%v", used, state.exceeded.Load())
	}

	handler := withAPIResponseBudgets(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for index := 0; index <= compactAPIQueries; index++ {
			_ = consumeQuery(r.Context())
		}
		writeJSON(w, http.StatusOK, map[string]bool{"unreachable": true})
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"query_budget_exceeded"`) {
		t.Fatalf("query budget response code=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Query-Count-Limit"); got != fmt.Sprint(compactAPIQueries) {
		t.Fatalf("query budget limit header=%q", got)
	}
	if got := response.Header().Get("X-Query-Count-Used"); got != fmt.Sprint(compactAPIQueries+1) {
		t.Fatalf("query budget used header=%q", got)
	}
}

// TestQueryBudgetDriverInterfaces verifies statement, transaction, and optional driver paths.
func TestQueryBudgetDriverInterfaces(t *testing.T) {
	wrapped := &queryBudgetDriver{inner: &sqlite.Driver{}}
	connection, err := wrapped.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	budgeted := connection.(*queryBudgetConn)
	ctx, state := withQueryBudget(t.Context(), 8)
	if err := budgeted.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if !budgeted.IsValid() {
		t.Fatal("new SQLite connection is not valid")
	}
	if err := budgeted.ResetSession(ctx); err != nil {
		t.Fatal(err)
	}
	value := &driver.NamedValue{Ordinal: 1, Value: int64(1)}
	if err := budgeted.CheckNamedValue(value); err != nil && !errors.Is(err, driver.ErrSkip) {
		t.Fatal(err)
	}
	if _, err := budgeted.ExecContext(ctx, "CREATE TABLE evidence (id INTEGER PRIMARY KEY, value TEXT)", nil); err != nil {
		t.Fatal(err)
	}
	insert, err := budgeted.PrepareContext(ctx, "INSERT INTO evidence (value) VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}
	defer insert.Close()
	if _, err := insert.(*queryBudgetStmt).ExecContext(ctx, []driver.NamedValue{{Ordinal: 1, Value: "prepared"}}); err != nil {
		t.Fatal(err)
	}
	query, err := budgeted.Prepare("SELECT value FROM evidence ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer query.Close()
	rows, err := query.(*queryBudgetStmt).QueryContext(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	transaction, err := budgeted.BeginTx(ctx, driver.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatal(err)
	}
	if used := state.used.Load(); used != 3 {
		t.Fatalf("query budget used=%d, want 3", used)
	}
}

// TestHelper_quoteIdentifier verifies helper quote identifier.
func TestHelper_quoteIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", `"simple"`},
		{"has spaces", `"has spaces"`},
		{"tab	chars", `"tab	chars"`},
		{`has"quote`, `"has""quote"`},
		{`"leading`, `"""leading"`},
		{`trailing"`, `"trailing"""`},
		{`double""double`, `"double""""double"`},
		{``, `""`},
		{`_underscore_123`, `"_underscore_123"`},
		{`unicode→`, `"unicode→"`},
	}
	for _, tc := range tests {
		got := quoteIdentifier(tc.input)
		if got != tc.want {
			t.Errorf("quoteIdentifier(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestHelper_positiveID verifies helper positive id.
func TestHelper_positiveID(t *testing.T) {
	tests := []struct {
		raw     string
		wantID  int64
		wantErr bool
	}{
		{"1", 1, false},
		{"42", 42, false},
		{"9223372036854775807", math.MaxInt64, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"-42", 0, true},
		{"abc", 0, true},
		{"", 0, true},
		{"1.0", 0, true},
		{" 1", 0, true},
	}
	for _, tc := range tests {
		id, err := positiveID(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("positiveID(%q) = %d, nil; want error", tc.raw, id)
			}
			continue
		}
		if err != nil {
			t.Errorf("positiveID(%q) returned error: %v", tc.raw, err)
		}
		if id != tc.wantID {
			t.Errorf("positiveID(%q) = %d, want %d", tc.raw, id, tc.wantID)
		}
		var problem *apiProblem
		if tc.wantErr && err != nil {
			if !errors.As(err, &problem) {
				t.Errorf("positiveID(%q) error type = %T, want *apiProblem", tc.raw, err)
			}
			if problem.Code != "invalid_request" {
				t.Errorf("positiveID(%q) error code = %q, want %q", tc.raw, problem.Code, "invalid_request")
			}
			if problem.Status != http.StatusBadRequest {
				t.Errorf("positiveID(%q) error status = %d, want %d", tc.raw, problem.Status, http.StatusBadRequest)
			}
		}
	}
}

// TestHelper_stringID verifies helper string id.
func TestHelper_stringID(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{math.MaxInt64, "9223372036854775807"},
		{-1, "-1"},
	}
	for _, tc := range tests {
		got := stringID(tc.input)
		if got != tc.want {
			t.Errorf("stringID(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestHelper_nullableValue verifies helper nullable value.
func TestHelper_nullableValue(t *testing.T) {
	tests := []struct {
		input sql.NullString
		want  any
	}{
		{sql.NullString{String: "hello", Valid: true}, "hello"},
		{sql.NullString{String: "", Valid: true}, ""},
		{sql.NullString{String: "anything", Valid: false}, nil},
		{sql.NullString{Valid: false}, nil},
	}
	for _, tc := range tests {
		got := nullableValue(tc.input)
		if tc.want == nil {
			if got != nil {
				t.Errorf("nullableValue(%+v) = %v, want nil", tc.input, got)
			}
		} else if got != tc.want {
			t.Errorf("nullableValue(%+v) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestHelper_badRequest verifies helper bad request.
func TestHelper_badRequest(t *testing.T) {
	err := badRequest("test message")
	var problem *apiProblem
	if !errors.As(err, &problem) {
		t.Fatal("badRequest did not return *apiProblem")
	}
	if problem.Code != "invalid_request" {
		t.Errorf("code = %q, want %q", problem.Code, "invalid_request")
	}
	if problem.Message != "test message" {
		t.Errorf("message = %q, want %q", problem.Message, "test message")
	}
	if problem.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", problem.Status, http.StatusBadRequest)
	}
	if problem.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", problem.Error(), "test message")
	}
}

// TestHelper_notFound verifies helper not found.
func TestHelper_notFound(t *testing.T) {
	err := notFound("missing")
	var problem *apiProblem
	if !errors.As(err, &problem) {
		t.Fatal("notFound did not return *apiProblem")
	}
	if problem.Code != "not_found" {
		t.Errorf("code = %q, want %q", problem.Code, "not_found")
	}
	if problem.Message != "missing" {
		t.Errorf("message = %q, want %q", problem.Message, "missing")
	}
	if problem.Status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", problem.Status, http.StatusNotFound)
	}
}

// TestHelper_metricGroup verifies helper metric group.
func TestHelper_metricGroup(t *testing.T) {
	metrics := map[string]map[string]any{
		"alpha": {"available": true, "value": int64(10)},
		"beta":  {"available": true, "value": int64(20)},
	}
	result := metricGroup(metrics, "alpha", "beta", "gamma")
	if v := result["alpha"].(map[string]any)["value"]; v != int64(10) {
		t.Errorf("alpha value = %v, want 10", v)
	}
	if v := result["beta"].(map[string]any)["value"]; v != int64(20) {
		t.Errorf("beta value = %v, want 20", v)
	}
	gamma := result["gamma"].(map[string]any)
	if gamma["available"] != false {
		t.Errorf("gamma.available = %v, want false", gamma["available"])
	}
	if _, ok := result["delta"]; ok {
		t.Error("unexpected key delta in result")
	}
	if len(result) != 3 {
		t.Errorf("result has %d keys, want 3", len(result))
	}
}

// TestHelper_sourceBreakdown verifies helper source breakdown.
func TestHelper_sourceBreakdown(t *testing.T) {
	metrics := []map[string]any{
		{"source": "scopus", "metric": "input_records", "value": int64(100)},
		{"source": "pubmed", "metric": "input_records", "value": int64(50)},
		{"source": "", "metric": "input_records", "value": int64(200)},
		{"source": "scopus", "metric": "another_metric", "value": int64(999)},
	}
	totals := map[string]int64{"input_records": 200}
	result := sourceBreakdown(metrics, totals)
	scopus := result["scopus"].(map[string]any)
	if scopus["value"] != int64(100) {
		t.Errorf("scopus value = %v, want 100", scopus["value"])
	}
	if scopus["percentage"].(float64) != 50.0 {
		t.Errorf("scopus percentage = %v, want 50.0", scopus["percentage"])
	}
	pubmed := result["pubmed"].(map[string]any)
	if pubmed["value"] != int64(50) {
		t.Errorf("pubmed value = %v, want 50", pubmed["value"])
	}
	if _, ok := result[""]; ok {
		t.Error("empty source should be excluded")
	}
	if result["unknown"] != nil {
		t.Error("unknown source should be absent")
	}
}

// TestHelper_sourceBreakdown_noDenominator verifies helper source breakdown no denominator.
func TestHelper_sourceBreakdown_noDenominator(t *testing.T) {
	metrics := []map[string]any{
		{"source": "scopus", "metric": "input_records", "value": int64(100)},
	}
	result := sourceBreakdown(metrics, map[string]int64{})
	scopus := result["scopus"].(map[string]any)
	if _, ok := scopus["denominator"]; ok {
		t.Error("denominator should be absent when totals is 0")
	}
	if _, ok := scopus["percentage"]; ok {
		t.Error("percentage should be absent when denominator is 0")
	}
}

// TestHelper_enrichmentFieldBreakdown verifies helper enrichment field breakdown.
func TestHelper_enrichmentFieldBreakdown(t *testing.T) {
	byName := map[string]map[string]any{
		"enriched_fields_title":       {"available": true, "value": int64(10)},
		"enriched_fields_abstract":    {"available": true, "value": int64(5)},
		"enriched_fields_total":       {"available": true, "value": int64(15)},
		"unrelated_metric":            {"available": true, "value": int64(99)},
		"enriched_fields_author_name": {"available": true, "value": int64(8)},
	}
	result := enrichmentFieldBreakdown(byName)
	if v := result["title"].(map[string]any)["value"]; v != int64(10) {
		t.Errorf("title value = %v, want 10", v)
	}
	if v := result["abstract"].(map[string]any)["value"]; v != int64(5) {
		t.Errorf("abstract value = %v, want 5", v)
	}
	if v := result["author_name"].(map[string]any)["value"]; v != int64(8) {
		t.Errorf("author_name value = %v, want 8", v)
	}
	if _, ok := result["total"]; ok {
		t.Error("total should be excluded")
	}
	if _, ok := result["unrelated_metric"]; ok {
		t.Error("non-enriched_fields metrics should be excluded")
	}
	if len(result) != 3 {
		t.Errorf("result has %d keys, want 3", len(result))
	}
}

// TestHelper_enrichmentProviderBreakdown verifies helper enrichment provider breakdown.
func TestHelper_enrichmentProviderBreakdown(t *testing.T) {
	metrics := []map[string]any{
		{"metric": "enriched_fields", "source": "crossref", "value": int64(30)},
		{"metric": "enriched_fields", "source": "openalex", "value": int64(20)},
		{"metric": "enriched_fields", "source": "", "value": int64(50)},
		{"metric": "other", "source": "crossref", "value": int64(99)},
	}
	result := enrichmentProviderBreakdown(metrics)
	crossref := result["crossref"].(map[string]any)
	if crossref["value"] != int64(30) {
		t.Errorf("crossref value = %v, want 30", crossref["value"])
	}
	openalex := result["openalex"].(map[string]any)
	if openalex["value"] != int64(20) {
		t.Errorf("openalex value = %v, want 20", openalex["value"])
	}
	if result[""] != nil {
		t.Error("empty source should be excluded")
	}
	if len(result) != 2 {
		t.Errorf("result has %d keys, want 2", len(result))
	}
}

// TestHelper_normalizationFieldBreakdown verifies helper normalization field breakdown.
func TestHelper_normalizationFieldBreakdown(t *testing.T) {
	metrics := []map[string]any{
		{"source": "publisher", "metric": "normalization_fields_processed", "value": int64(100)},
		{"source": "publisher", "metric": "normalization_fields_changed", "value": int64(30)},
		{"source": "publisher", "metric": "normalization_fields_already_canonical", "value": int64(60)},
		{"source": "publisher", "metric": "normalization_fields_unavailable", "value": int64(10)},
		{"source": "journal", "metric": "normalization_fields_processed", "value": int64(50)},
		{"source": "journal", "metric": "normalization_fields_changed", "value": int64(20)},
	}
	result := normalizationFieldBreakdown(metrics)
	pub := result["publisher"]
	if p := pub["processed"].(map[string]any)["value"]; p != int64(100) {
		t.Errorf("publisher processed = %v, want 100", p)
	}
	if c := pub["changed"].(map[string]any)["value"]; c != int64(30) {
		t.Errorf("publisher changed = %v, want 30", c)
	}
	if d := pub["changed"].(map[string]any)["denominator"]; d != int64(100) {
		t.Errorf("publisher changed denominator = %v, want 100", d)
	}
	jour := result["journal"]
	if p := jour["processed"].(map[string]any)["value"]; p != int64(50) {
		t.Errorf("journal processed = %v, want 50", p)
	}
	if c := jour["changed"].(map[string]any)["value"]; c != int64(20) {
		t.Errorf("journal changed = %v, want 20", c)
	}
	// author_name and affiliation should have default false entries
	for _, field := range []string{"author_name", "affiliation"} {
		for _, status := range []string{"processed", "changed", "already_canonical", "unavailable"} {
			item := result[field][status].(map[string]any)
			if item["available"] != false {
				t.Errorf("%s.%s.available = %v, want false", field, status, item["available"])
			}
		}
	}
	if _, ok := result["unknown"]; ok {
		t.Error("unknown field should not be in result")
	}
}

// TestHelper_normalizationFieldBreakdown_emptyProcessed verifies helper normalization field breakdown empty processed.
func TestHelper_normalizationFieldBreakdown_emptyProcessed(t *testing.T) {
	metrics := []map[string]any{
		{"source": "publisher", "metric": "normalization_fields_changed", "value": int64(10)},
	}
	result := normalizationFieldBreakdown(metrics)
	pub := result["publisher"]
	changed := pub["changed"].(map[string]any)
	if changed["available"] != true {
		t.Error("changed should be marked available even when processed is 0")
	}
	if changed["value"] != int64(10) {
		t.Errorf("changed value = %v, want 10", changed["value"])
	}
	if _, ok := changed["denominator"]; ok {
		t.Error("denominator should not be set when processed is 0")
	}
	if _, ok := changed["percentage"]; ok {
		t.Error("percentage should not be set when processed is 0")
	}
}

// TestHelper_normalizationFieldBreakdown_skipsMissingStatuses verifies helper normalization field breakdown skips missing statuses.
func TestHelper_normalizationFieldBreakdown_skipsMissingStatuses(t *testing.T) {
	// Statuses not in the known list should be skipped
	metrics := []map[string]any{
		{"source": "publisher", "metric": "normalization_fields_processed", "value": int64(100)},
		{"source": "publisher", "metric": "normalization_fields_unknown_status", "value": int64(99)},
	}
	result := normalizationFieldBreakdown(metrics)
	pub := result["publisher"]
	if _, ok := pub["unknown_status"]; ok {
		t.Error("unknown status should be skipped")
	}
}

// TestHelper_metricDenominator verifies helper metric denominator.
func TestHelper_metricDenominator(t *testing.T) {
	values := map[string]int64{
		"input_records":                  200,
		"deduplicated_articles":          150,
		"valid_articles":                 120,
		"normalization_fields_processed": 500,
	}
	tests := []struct {
		metric  string
		wantDen int64
		wantOk  bool
	}{
		{"input_records", 200, true},
		{"parsed_articles", 200, true},
		{"deduplicated_articles", 200, true},
		{"valid_articles", 150, true},
		{"discarded_articles", 150, true},
		{"enrichment_candidates", 150, true},
		{"enriched_article_updates", 150, true},
		{"normalized_articles_processed", 120, true},
		{"normalization_fields_changed", 500, true},
		{"normalization_fields_already_canonical", 500, true},
		{"normalization_fields_unavailable", 500, true},
		{"unknown_metric", 0, false},
	}
	for _, tc := range tests {
		den, ok := metricDenominator(tc.metric, values)
		if ok != tc.wantOk {
			t.Errorf("metricDenominator(%q) ok = %v, want %v", tc.metric, ok, tc.wantOk)
		}
		if den != tc.wantDen {
			t.Errorf("metricDenominator(%q) = %d, want %d", tc.metric, den, tc.wantDen)
		}
	}
}

// TestHelper_metricDenominator_zeroThreshold verifies helper metric denominator zero threshold.
func TestHelper_metricDenominator_zeroThreshold(t *testing.T) {
	// When the candidate denominator is 0, ok should be false
	values := map[string]int64{
		"input_records": 0,
	}
	den, ok := metricDenominator("input_records", values)
	if ok {
		t.Error("expected ok=false when denominator value is 0")
	}
	if den != 0 {
		t.Errorf("expected den=0, got %d", den)
	}
}

// TestHelper_percent verifies helper percent.
func TestHelper_percent(t *testing.T) {
	tests := []struct {
		value, denominator int64
		want               *float64
	}{
		{50, 100, ptr(50.0)},
		{1, 3, ptr(100.0 / 3.0)},
		{0, 100, ptr(0.0)},
		{100, 100, ptr(100.0)},
		{50, 0, nil},
		{0, 0, nil},
	}
	for _, tc := range tests {
		got := percent(tc.value, tc.denominator)
		if tc.want == nil {
			if got != nil {
				t.Errorf("percent(%d,%d) = %v, want nil", tc.value, tc.denominator, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("percent(%d,%d) = nil, want %f", tc.value, tc.denominator, *tc.want)
			continue
		}
		if math.Abs(*got-*tc.want) > 1e-9 {
			t.Errorf("percent(%d,%d) = %f, want %f", tc.value, tc.denominator, *got, *tc.want)
		}
	}
}

// TestHelper_normalizedArtifactContentType verifies helper normalized artifact content type.
func TestHelper_normalizedArtifactContentType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"application/json", "application/json"},
		{"text/plain; charset=utf-8", "text/plain"},
		{"APPLICATION/JSON", "application/json"},
		{"application/json; charset=utf-8", "application/json"},
		{"application/vnd.api+json", "application/vnd.api+json"},
		{"", "application/octet-stream"},
		{"not/a/valid/mime; invalid=;;", "application/octet-stream"},
		{"text/html", "text/html"},
	}
	for _, tc := range tests {
		got := normalizedArtifactContentType(tc.input)
		if got != tc.want {
			t.Errorf("normalizedArtifactContentType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestMapReviewErrorPreservesUnknownFailures verifies unclassified repository failures reach the safe internal-error responder.
func TestMapReviewErrorPreservesUnknownFailures(t *testing.T) {
	unexpected := errors.New("database schema detail")
	if mapped := mapReviewError(unexpected); !errors.Is(mapped, unexpected) {
		t.Fatalf("mapped error=%v, want original internal failure", mapped)
	}
}

// TestHelper_jsonArtifactContentType verifies helper json artifact content type.
func TestHelper_jsonArtifactContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/vnd.api+json", true},
		{"text/plain", false},
		{"text/html", false},
		{"", false},
	}
	for _, tc := range tests {
		got := jsonArtifactContentType(tc.input)
		if got != tc.want {
			t.Errorf("jsonArtifactContentType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestHelper_inlineArtifactContentType verifies helper inline artifact content type.
func TestHelper_inlineArtifactContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"text/plain", true},
		{"text/plain; charset=utf-8", true},
		{"text/csv", true},
		{"application/json", true},
		{"application/vnd.api+json", true},
		{"application/x-something-config", true},
		{"text/html", false},
		{"application/xhtml+xml", false},
		{"application/octet-stream", false},
		{"image/png", false},
		{"", false},
	}
	for _, tc := range tests {
		got := inlineArtifactContentType(tc.input)
		if got != tc.want {
			t.Errorf("inlineArtifactContentType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestHelper_auditMultiValues verifies helper audit multi values.
func TestHelper_auditMultiValues(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		vals, err := auditMultiValues("", "param")
		if err != nil {
			t.Fatal(err)
		}
		if vals != nil {
			t.Errorf("got %v, want nil", vals)
		}
	})
	t.Run("single value", func(t *testing.T) {
		vals, err := auditMultiValues("abc", "param")
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 1 || vals[0] != "abc" {
			t.Errorf("got %v, want [abc]", vals)
		}
	})
	t.Run("multiple values", func(t *testing.T) {
		vals, err := auditMultiValues("a, b, c", "param")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"a", "b", "c"}
		if len(vals) != len(want) {
			t.Fatalf("got %v, want %v", vals, want)
		}
		for i := range want {
			if vals[i] != want[i] {
				t.Errorf("vals[%d] = %q, want %q", i, vals[i], want[i])
			}
		}
	})
	t.Run("deduplicates values", func(t *testing.T) {
		vals, err := auditMultiValues("x, x, y, x", "param")
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 2 || vals[0] != "x" || vals[1] != "y" {
			t.Errorf("got %v, want [x y]", vals)
		}
	})
	t.Run("whitespace trimmed", func(t *testing.T) {
		vals, err := auditMultiValues("  hello , world ", "param")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"hello", "world"}
		if len(vals) != len(want) {
			t.Fatalf("got %v, want %v", vals, want)
		}
		for i := range want {
			if vals[i] != want[i] {
				t.Errorf("vals[%d] = %q, want %q", i, vals[i], want[i])
			}
		}
	})
	t.Run("rejects empty item", func(t *testing.T) {
		_, err := auditMultiValues("a,,b", "param")
		if err == nil {
			t.Error("expected error for empty item")
		}
	})
	t.Run("rejects long value", func(t *testing.T) {
		long := make([]byte, 201)
		for i := range long {
			long[i] = 'x'
		}
		_, err := auditMultiValues("a,"+string(long), "param")
		if err == nil {
			t.Error("expected error for value >200 chars")
		}
	})
	t.Run("rejects more than 100 values", func(t *testing.T) {
		parts := make([]string, 101)
		for i := 0; i < 101; i++ {
			parts[i] = fmt.Sprintf("v%03d", i)
		}
		_, err := auditMultiValues(strings.Join(parts, ","), "param")
		if err == nil {
			t.Error("expected error for >100 values")
		}
	})
}

// TestHelper_auditInClause verifies helper audit in clause.
func TestHelper_auditInClause(t *testing.T) {
	t.Run("single value", func(t *testing.T) {
		clause, args := auditInClause("col", []string{"val1"})
		if clause != "col IN (?)" {
			t.Errorf("clause = %q, want %q", clause, "col IN (?)")
		}
		if len(args) != 1 || args[0] != "val1" {
			t.Errorf("args = %v, want [val1]", args)
		}
	})
	t.Run("multiple values", func(t *testing.T) {
		clause, args := auditInClause("actor", []string{"a", "b", "c"})
		if clause != "actor IN (?,?,?)" {
			t.Errorf("clause = %q, want %q", clause, "actor IN (?,?,?)")
		}
		if len(args) != 3 {
			t.Fatalf("len(args) = %d, want 3", len(args))
		}
		for i, want := range []string{"a", "b", "c"} {
			if args[i] != want {
				t.Errorf("args[%d] = %v, want %v", i, args[i], want)
			}
		}
	})
	t.Run("empty", func(t *testing.T) {
		clause, args := auditInClause("col", []string{})
		if clause != "col IN ()" {
			t.Errorf("clause = %q, want %q", clause, "col IN ()")
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})
}

// TestHelper_auditWhere verifies helper audit where.
func TestHelper_auditWhere(t *testing.T) {
	tests := []struct {
		clauses []string
		want    string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"a = 1"}, " WHERE a = 1"},
		{[]string{"a = 1", "b = 2"}, " WHERE a = 1 AND b = 2"},
		{[]string{"x IN (?,?,?)", "y IS NULL"}, " WHERE x IN (?,?,?) AND y IS NULL"},
	}
	for _, tc := range tests {
		got := auditWhere(tc.clauses)
		if got != tc.want {
			t.Errorf("auditWhere(%v) = %q, want %q", tc.clauses, got, tc.want)
		}
	}
}

// TestHelper_corpusSelectColumns verifies helper corpus select columns.
func TestHelper_corpusSelectColumns(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"articles", "wr.id, wr.work_id, wr.title, wr.year, wr.journal, wr.publisher, wr.source, w.doi, validation.outcome AS validation_status, wr.citation_count, wr.reference_count, wr.producer_stage, wr.created_at, wr.abstract, wr.keywords, wr.keywords_plus, (SELECT GROUP_CONCAT(ao.citation_name, '; ') FROM authorships a JOIN author_occurrences ao ON ao.id=a.author_occurrence_id WHERE a.work_revision_id=wr.id ORDER BY a.author_order) AS authors"},
		{"authors", "ao.id, ao.citation_name, ao.first_name, ao.last_name, ao.orcid, ao.person_id, COUNT(DISTINCT a.work_revision_id) AS article_count, COUNT(DISTINCT NULLIF(a.affiliation, '')) AS affiliation_count, ao.created_at"},
		{"references", "rm.id, rm.work_revision_id, rm.mention_order, rm.doi, rm.title, rm.author, rm.year, rm.source, rm.resolved_work_id, wr.title AS citing_title, rm.created_at"},
		{"sources", "sr.id, sr.run_source_id, rs.source_name, rs.source_type, sr.record_index, sr.parse_status, sr.reject_reason, sr.content_hash, sr.created_at"},
		{"unknown", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := corpusSelectColumns(tc.kind)
		if got != tc.want {
			t.Errorf("corpusSelectColumns(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// TestHelper_scopedPagination verifies helper scoped pagination.
func TestHelper_scopedPagination(t *testing.T) {
	tests := []struct {
		page, perPage int
		total         int64
		sort, order   string
		want          map[string]any
	}{
		{1, 50, 100, "id", "asc", map[string]any{"page": 1, "per_page": 50, "total_rows": int64(100), "total_pages": int64(2), "has_next": true, "sort": "id", "order": "asc"}},
		{2, 50, 100, "id", "DESC", map[string]any{"page": 2, "per_page": 50, "total_rows": int64(100), "total_pages": int64(2), "has_next": false, "sort": "id", "order": "desc"}},
		{1, 20, 0, "name", "ASC", map[string]any{"page": 1, "per_page": 20, "total_rows": int64(0), "total_pages": int64(0), "has_next": false, "sort": "name", "order": "asc"}},
		{1, 50, 1, "id", "asc", map[string]any{"page": 1, "per_page": 50, "total_rows": int64(1), "total_pages": int64(1), "has_next": false, "sort": "id", "order": "asc"}},
		{1, 100, 250, "year", "DeSc", map[string]any{"page": 1, "per_page": 100, "total_rows": int64(250), "total_pages": int64(3), "has_next": true, "sort": "year", "order": "desc"}},
	}
	for _, tc := range tests {
		got := scopedPagination(tc.page, tc.perPage, tc.total, tc.sort, tc.order)
		for key, wantVal := range tc.want {
			gotVal, ok := got[key]
			if !ok {
				t.Errorf("scopedPagination(%d,%d,%d,%q,%q) missing key %q", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key)
				continue
			}
			switch w := wantVal.(type) {
			case int:
				if g, ok2 := gotVal.(int); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %d", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			case int64:
				if g, ok2 := gotVal.(int64); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %d", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			case bool:
				if g, ok2 := gotVal.(bool); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %v", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			case string:
				if g, ok2 := gotVal.(string); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %q", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			default:
				t.Errorf("unexpected type for key %q: %T", key, wantVal)
			}
		}
	}
}

// TestHelper_placeholders verifies helper placeholders.
func TestHelper_placeholders(t *testing.T) {
	t.Run("single id", func(t *testing.T) {
		clause, args := placeholders([]int64{42})
		if clause != "?" {
			t.Errorf("clause = %q, want ?", clause)
		}
		if len(args) != 1 || args[0] != int64(42) {
			t.Errorf("args = %v, want [42]", args)
		}
	})
	t.Run("multiple ids", func(t *testing.T) {
		clause, args := placeholders([]int64{1, 2, 3})
		if clause != "?,?,?" {
			t.Errorf("clause = %q, want ?,?,?", clause)
		}
		if len(args) != 3 {
			t.Fatalf("len(args) = %d, want 3", len(args))
		}
		for i, want := range []int64{1, 2, 3} {
			if args[i] != want {
				t.Errorf("args[%d] = %v, want %v", i, args[i], want)
			}
		}
	})
	t.Run("empty", func(t *testing.T) {
		clause, args := placeholders([]int64{})
		if clause != "" {
			t.Errorf("clause = %q, want empty", clause)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})
}

// TestHelper_parseOptionalInt verifies helper parse optional int.
func TestHelper_parseOptionalInt(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		v, err := parseOptionalInt("", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != 0 {
			t.Errorf("got %d, want 0", v)
		}
	})
	t.Run("valid int", func(t *testing.T) {
		v, err := parseOptionalInt("42", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != 42 {
			t.Errorf("got %d, want 42", v)
		}
	})
	t.Run("negative int allowed", func(t *testing.T) {
		v, err := parseOptionalInt("-5", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != -5 {
			t.Errorf("got %d, want -5", v)
		}
	})
	t.Run("zero valid", func(t *testing.T) {
		v, err := parseOptionalInt("0", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != 0 {
			t.Errorf("got %d, want 0", v)
		}
	})
	t.Run("invalid string", func(t *testing.T) {
		_, err := parseOptionalInt("abc", "param")
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("max int64", func(t *testing.T) {
		v, err := parseOptionalInt("9223372036854775807", "big")
		if err != nil {
			t.Fatal(err)
		}
		if v != math.MaxInt64 {
			t.Errorf("got %d, want %d", v, math.MaxInt64)
		}
	})
}

// ptr supports the package test suite's ptr setup or assertions.
func ptr(v float64) *float64 {
	return &v
}
