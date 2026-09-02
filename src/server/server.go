// Package server exposes an existing workspace database through a local HTTP
// API with read-only research queries and bounded review and lifecycle writes.
// It deliberately does not run migrations while opening the viewer.
package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"analysis/database"
	"analysis/logging"

	_ "modernc.org/sqlite"
)

const requestTimeout = 5 * time.Second

const (
	compactAPIResponseBytes    = 512 * 1024
	collectionAPIResponseBytes = 2 * 1024 * 1024
	detailAPIResponseBytes     = 4 * 1024 * 1024
	compactAPIQueries          = 64
	collectionAPIQueries       = 256
	detailAPIQueries           = 1024
)

var log = logging.Logger("viewer")

// viewPages maps extensionless view paths to the HTML documents that own
// their navigation entries. The trash view shares the Home document.
var viewPages = map[string]string{
	"/overview":      "overview.html",
	"/corpus":        "corpus.html",
	"/relationships": "relationships.html",
	"/provenance":    "provenance.html",
	"/evaluation":    "evaluation.html",
	"/advanced":      "advanced.html",
	"/article":       "article.html",
	"/author":        "author.html",
	"/reference":     "reference.html",
	"/trash":         "index.html",
}

// Server serves one existing workspace database. db remains a query-only
// connection while writeDB owns bounded local review and lifecycle mutations.
// AssetsFS is the frontend asset file system served at the web root; it must
// be set because the binary does not embed frontend assets.
type Server struct {
	db         *sql.DB
	writeDB    *database.Database
	pdfDB      *sql.DB
	pdfPath    string
	pdfCacheMu sync.Mutex
	pdfCache   *cachedPDF
	tables     map[string]tableInfo
	AssetsFS   fs.FS // serves frontend assets from this filesystem
}

// tableInfo stores the discovered columns for one browsable SQLite table.
type tableInfo struct {
	Name    string       `json:"name"`
	Columns []columnInfo `json:"columns"`
}

// columnInfo records a SQLite column's name, declared type, and primary-key position.
type columnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	PrimaryKey bool   `json:"primary_key"`
}

// tableHasColumns reports whether a discovered table contains every requested column.
func (s *Server) tableHasColumns(table string, required ...string) bool {
	info, ok := s.tables[table]
	if !ok {
		return false
	}
	available := make(map[string]struct{}, len(info.Columns))
	for _, column := range info.Columns {
		available[column.Name] = struct{}{}
	}
	for _, name := range required {
		if _, ok := available[name]; !ok {
			return false
		}
	}
	return true
}

// Open opens an existing database without creating it or modifying it.
func Open(path string) (*Server, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workspace database does not exist")
		}
		return nil, fmt.Errorf("inspect workspace database: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("workspace database path is a directory")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace database path: %w", err)
	}
	uri := (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro&_pragma=query_only(1)"}).String()
	db, err := sql.Open(queryBudgetDriverName, uri)
	if err != nil {
		return nil, fmt.Errorf("open workspace database read-only: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("read workspace database: %w", err)
	}
	s := &Server{db: db}
	if err := s.discoverTables(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.verifyReviewSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	writeDB, err := database.OpenExistingWithDriver(absolute, queryBudgetDriverName)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open metadata mutation connection: %w", err)
	}
	s.writeDB = writeDB
	if err := s.openBoundPDFStore(ctx, filepath.Dir(absolute)); err != nil {
		writeDB.Close()
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases resources owned by the receiver.
func (s *Server) Close() error {
	var first error
	if s.pdfDB != nil {
		if err := s.pdfDB.Close(); err != nil {
			first = err
		}
	}
	if s.writeDB != nil {
		if err := s.writeDB.Close(); err != nil && first == nil {
			first = err
		}
	}
	if err := s.db.Close(); err != nil && first == nil {
		first = err
	}
	return first
}

// PDFStoreBound reports whether a readable companion PDF database is attached.
func (s *Server) PDFStoreBound() bool { return s.pdfDB != nil }

// Handler returns the local API and frontend handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/hierarchy", s.hierarchy)
	mux.HandleFunc("GET /api/searches", s.searches)
	mux.HandleFunc("GET /api/plans", s.plans)
	mux.HandleFunc("GET /api/runs", s.runs)
	mux.HandleFunc("GET /api/runs/{id}/context", s.runContext)
	mux.HandleFunc("PUT /api/runs/{run_id}/visibility", s.updateRunVisibility)
	mux.HandleFunc("GET /api/overview", s.overview)
	mux.HandleFunc("GET /api/runs/{id}/artifacts", s.runArtifacts)
	mux.HandleFunc("GET /api/artifacts/{id}/inspect", s.artifactInspection)
	mux.HandleFunc("GET /api/artifacts/{id}/content", s.artifactContent)
	mux.HandleFunc("GET /api/runs/{id}/cache-uses", s.runCacheUses)
	mux.HandleFunc("GET /api/runs/{id}/corpus/{kind}", s.runCorpus)
	mux.HandleFunc("GET /api/runs/{id}/evaluation", s.runEvaluation)
	mux.HandleFunc("GET /api/runs/{run_id}/review-context", s.runReviewContext)
	mux.HandleFunc("GET /api/runs/{run_id}/review-context-candidates", s.reviewContextCandidates)
	mux.HandleFunc("POST /api/runs/{run_id}/review-context", s.createReviewContext)
	mux.HandleFunc("GET /api/runs/{run_id}/articles/{work_revision_id}/review", s.articleReview)
	mux.HandleFunc("PUT /api/runs/{run_id}/articles/{work_revision_id}/review", s.updateArticleReview)
	mux.HandleFunc("GET /api/runs/{run_id}/articles/{work_revision_id}/review/versions", s.articleReviewVersions)
	mux.HandleFunc("GET /api/runs/{run_id}/articles/{work_revision_id}/review/versions/{version_id}", s.articleReviewVersion)
	mux.HandleFunc("GET /api/runs/{run_id}/articles/{work_revision_id}/notes", s.articleNotes)
	mux.HandleFunc("POST /api/runs/{run_id}/articles/{work_revision_id}/notes", s.createArticleNote)
	mux.HandleFunc("GET /api/runs/{run_id}/notes", s.runNotes)
	mux.HandleFunc("GET /api/runs/{run_id}/notes/{note_id}", s.note)
	mux.HandleFunc("GET /api/runs/{run_id}/notes/{note_id}/versions", s.noteVersions)
	mux.HandleFunc("GET /api/runs/{run_id}/notes/{note_id}/versions/{version_id}", s.noteVersion)
	mux.HandleFunc("POST /api/runs/{run_id}/notes/{note_id}/versions", s.createNoteVersion)
	mux.HandleFunc("GET /api/runs/{run_id}/articles/{work_revision_id}/anchors", s.articleAnchors)
	mux.HandleFunc("POST /api/runs/{run_id}/articles/{work_revision_id}/anchors", s.createArticleAnchor)
	mux.HandleFunc("GET /api/runs/{run_id}/anchors/{anchor_id}/versions", s.anchorVersions)
	mux.HandleFunc("GET /api/runs/{run_id}/anchors/{anchor_id}/versions/{version_id}", s.anchorVersion)
	mux.HandleFunc("POST /api/runs/{run_id}/anchors/{anchor_id}/versions", s.createAnchorVersion)
	mux.HandleFunc("GET /api/runs/{run_id}/links/backlinks", s.reviewBacklinks)
	mux.HandleFunc("GET /api/runs/{id}/identity-evidence", s.runIdentityEvidence)
	mux.HandleFunc("GET /api/identity-resolutions/{id}/candidates", s.identityCandidates)
	mux.HandleFunc("GET /api/runs/{id}/stages", s.runStages)
	mux.HandleFunc("GET /api/audit", s.audit)
	mux.HandleFunc("GET /api/audit/{id}/recorded-data", s.auditRecordedData)
	mux.HandleFunc("GET /api/tables", s.tablesHandler)
	mux.HandleFunc("GET /api/tables/{table}", s.tableRows)
	mux.HandleFunc("GET /api/articles/{id}", s.articleDetail)
	mux.HandleFunc("GET /api/articles/{id}/collections/{kind}", s.articleDetailCollection)
	mux.HandleFunc("GET /api/works/{work_id}/pdf-status", s.workPDFStatus)
	mux.HandleFunc("GET /api/pdf/{work_id}", s.workPDF)
	mux.HandleFunc("GET /api/authors/{id}", s.authorDetail)
	mux.HandleFunc("GET /api/authors/{id}/collections/{kind}", s.authorDetailCollection)
	mux.HandleFunc("GET /api/references/{id}", s.referenceDetail)
	mux.HandleFunc("GET /api/graph", s.graph)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
	})
	if s.AssetsFS == nil {
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "assets_not_configured", "frontend assets not configured; serve requires --assets-dir")
		})
		return withAPIResponseBudgets(withJSONErrors(mux))
	}
	for path, file := range viewPages {
		mux.HandleFunc("GET "+path, s.serveViewPage(file))
	}
	mux.Handle("GET /", http.FileServer(http.FS(s.AssetsFS)))
	return withAPIResponseBudgets(withJSONErrors(mux))
}

// serveViewPage serves one view HTML document from the assets filesystem.
func (s *Server) serveViewPage(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, s.AssetsFS, file)
	}
}

// apiResponseByteBudget returns the serialized JSON budget for one API route.
// Binary PDF and artifact-content routes stream outside this JSON contract.
func apiResponseByteBudget(path string) int {
	if !strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api/pdf/") || (strings.HasPrefix(path, "/api/artifacts/") && strings.HasSuffix(path, "/content")) {
		return 0
	}
	if path == "/api/graph" || strings.HasPrefix(path, "/api/articles/") || strings.HasPrefix(path, "/api/authors/") || strings.HasPrefix(path, "/api/references/") || strings.HasPrefix(path, "/api/tables/") {
		return detailAPIResponseBytes
	}
	if path == "/api/audit" || strings.HasPrefix(path, "/api/audit/") || path == "/api/hierarchy" || strings.Contains(path, "/review") || strings.Contains(path, "/notes") || strings.Contains(path, "/anchors") || strings.Contains(path, "/links/backlinks") || strings.Contains(path, "/corpus/") || strings.Contains(path, "/evaluation") || strings.Contains(path, "/identity-evidence") || strings.Contains(path, "/cache-uses") || strings.Contains(path, "/stages") || strings.Contains(path, "/artifacts") {
		return collectionAPIResponseBytes
	}
	return compactAPIResponseBytes
}

// apiQueryBudget returns the maximum SQL statements one API request may execute.
func apiQueryBudget(path string) int {
	if !strings.HasPrefix(path, "/api/") {
		return 0
	}
	if path == "/api/graph" || strings.HasPrefix(path, "/api/articles/") || strings.HasPrefix(path, "/api/authors/") || strings.HasPrefix(path, "/api/references/") || strings.HasPrefix(path, "/api/tables/") || strings.Contains(path, "/review") || strings.Contains(path, "/notes") || strings.Contains(path, "/anchors") || strings.Contains(path, "/links/backlinks") {
		return detailAPIQueries
	}
	if path == "/api/audit" || strings.HasPrefix(path, "/api/audit/") || path == "/api/hierarchy" || strings.Contains(path, "/corpus/") || strings.Contains(path, "/evaluation") || strings.Contains(path, "/identity-evidence") || strings.Contains(path, "/cache-uses") || strings.Contains(path, "/stages") || strings.Contains(path, "/artifacts") {
		return collectionAPIQueries
	}
	return compactAPIQueries
}

// responseBudgetWriter buffers one bounded JSON response before it is committed.
type responseBudgetWriter struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	limit    int
	exceeded bool
}

// Header implements http.ResponseWriter.
func (w *responseBudgetWriter) Header() http.Header { return w.header }

// WriteHeader records the first response status without committing it.
func (w *responseBudgetWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

// Write retains at most the configured byte budget and reports a complete write.
func (w *responseBudgetWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	remaining := w.limit - w.body.Len()
	if remaining < len(data) {
		w.exceeded = true
		if remaining > 0 {
			_, _ = w.body.Write(data[:remaining])
		}
		return len(data), nil
	}
	_, _ = w.body.Write(data)
	return len(data), nil
}

// withAPIResponseBudgets enforces route-specific serialized JSON limits.
func withAPIResponseBudgets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := apiResponseByteBudget(r.URL.Path)
		queryLimit := apiQueryBudget(r.URL.Path)
		if limit == 0 && queryLimit == 0 {
			next.ServeHTTP(w, r)
			return
		}
		ctx, queryState := withQueryBudget(r.Context(), queryLimit)
		r = r.WithContext(ctx)
		if limit == 0 {
			w.Header().Set("X-Query-Count-Limit", strconv.Itoa(queryLimit))
			w.Header().Add("Trailer", "X-Query-Count-Used")
			next.ServeHTTP(w, r)
			w.Header().Set("X-Query-Count-Used", strconv.FormatInt(queryState.used.Load(), 10))
			return
		}
		buffered := &responseBudgetWriter{header: make(http.Header), limit: limit}
		next.ServeHTTP(buffered, r)
		if limit > 0 {
			w.Header().Set("X-Response-Byte-Limit", strconv.Itoa(limit))
		}
		w.Header().Set("X-Query-Count-Limit", strconv.Itoa(queryLimit))
		w.Header().Set("X-Query-Count-Used", strconv.FormatInt(queryState.used.Load(), 10))
		if queryState.exceeded.Load() {
			writeDetailedError(w, http.StatusInternalServerError, "query_budget_exceeded", "the request exceeded its SQL statement budget", map[string]any{"query_limit": queryLimit})
			return
		}
		if buffered.exceeded {
			writeDetailedError(w, http.StatusInternalServerError, "response_budget_exceeded", "the bounded response exceeded its serialized byte limit", map[string]any{"byte_limit": limit})
			return
		}
		for key, values := range buffered.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		status := buffered.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(buffered.body.Bytes())
	})
}

// verifyReviewSchema rejects an unmigrated metadata database before writable controls are served.
func (s *Server) verifyReviewSchema(ctx context.Context) error {
	requiredTables := []string{
		"pipeline_run_reviewers", "review_settings", "review_contexts", "work_review_versions",
		"work_review_version_substatuses", "review_context_work_heads", "review_notes",
		"review_note_versions", "review_context_note_heads", "review_note_links", "review_anchors",
		"review_anchor_versions", "review_context_anchor_heads",
	}
	for _, table := range requiredTables {
		if !s.hasTable(table) {
			return fmt.Errorf("metadata database is missing review migration table %q; run analysis migrate --db <metadata.db>", table)
		}
	}
	requiredTriggers := []string{
		"review_contexts_abort_update", "review_contexts_abort_delete", "work_review_versions_abort_update",
		"work_review_versions_abort_delete", "review_note_versions_abort_update", "review_note_versions_abort_delete",
		"review_anchor_versions_abort_update", "review_anchor_versions_abort_delete",
	}
	for _, trigger := range requiredTriggers {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?`, trigger).Scan(&count); err != nil {
			return fmt.Errorf("inspect review migration triggers: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("metadata database is missing review migration trigger %q; run analysis migrate --db <metadata.db>", trigger)
		}
	}
	return nil
}

// openBoundPDFStore resolves and opens the companion PDF database declared by metadata.
func (s *Server) openBoundPDFStore(ctx context.Context, metadataDir string) error {
	if !s.hasTable("pdf_store_binding") {
		return nil
	}
	var relativePath string
	err := s.db.QueryRowContext(ctx, "SELECT relative_path FROM pdf_store_binding WHERE id=1").Scan(&relativePath)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read PDF store binding: %w", err)
	}
	if filepath.IsAbs(relativePath) {
		return fmt.Errorf("bound PDF store path must be relative")
	}
	resolvedMetadataDir, err := filepath.EvalSymlinks(metadataDir)
	if err != nil {
		return fmt.Errorf("resolve metadata database directory: %w", err)
	}
	absolute := filepath.Clean(filepath.Join(resolvedMetadataDir, relativePath))
	resolvedStorePath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return fmt.Errorf("resolve bound PDF store: %w", err)
	}
	relative, err := filepath.Rel(resolvedMetadataDir, resolvedStorePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("bound PDF store path escapes the metadata database directory")
	}
	info, err := os.Stat(resolvedStorePath)
	if err != nil {
		return fmt.Errorf("inspect bound PDF store: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("bound PDF store path is not a regular file")
	}
	uri := (&url.URL{Scheme: "file", Path: resolvedStorePath, RawQuery: "mode=ro&_pragma=query_only(1)"}).String()
	pdfDB, err := sql.Open(queryBudgetDriverName, uri)
	if err != nil {
		return fmt.Errorf("open PDF store read-only: %w", err)
	}
	pdfDB.SetMaxOpenConns(1)
	pdfDB.SetConnMaxLifetime(0)
	if err := pdfDB.PingContext(ctx); err != nil {
		pdfDB.Close()
		return fmt.Errorf("read PDF store: %w", err)
	}
	requiredTables := []struct {
		name    string
		columns []string
	}{
		{"pdf_blobs", []string{"content_hash", "byte_size", "data"}},
		{"pdf_documents", []string{"doi", "status", "content_hash", "inventoried_at"}},
	}
	for _, required := range requiredTables {
		var found int
		if err := pdfDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name=?`, required.name).Scan(&found); err != nil {
			pdfDB.Close()
			return fmt.Errorf("inspect PDF store schema: %w", err)
		}
		if found != 1 {
			pdfDB.Close()
			return fmt.Errorf("bound PDF store is missing required table %q", required.name)
		}
		columns, err := pdfTableColumns(ctx, pdfDB, required.name)
		if err != nil {
			pdfDB.Close()
			return err
		}
		for _, column := range required.columns {
			if !columns[column] {
				pdfDB.Close()
				return fmt.Errorf("bound PDF store table %q is missing required column %q; run the workspace pipeline to apply companion migrations", required.name, column)
			}
		}
	}
	s.pdfDB, s.pdfPath = pdfDB, absolute
	return nil
}

// pdfTableColumns returns the discovered columns for a companion PDF table.
func pdfTableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("inspect PDF store table %q: %w", table, err)
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("inspect PDF store table %q: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inspect PDF store table %q: %w", table, err)
	}
	return columns, nil
}

// HTTPServer returns a conservatively configured local HTTP server.
func (s *Server) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           enforceLoopbackAuthority(addr, s.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

// enforceLoopbackAuthority rejects invalid Host authorities before routing local viewer requests.
func enforceLoopbackAuthority(authority string, next http.Handler) http.Handler {
	expectedHost, expectedPort, expectedErr := net.SplitHostPort(authority)
	expectedIP := net.ParseIP(expectedHost)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, port, err := net.SplitHostPort(r.Host)
		ip := net.ParseIP(host)
		if expectedErr != nil || expectedIP == nil || !expectedIP.IsLoopback() || err != nil || ip == nil ||
			!ip.IsLoopback() || !ip.Equal(expectedIP) || port != expectedPort {
			writeError(w, http.StatusBadRequest, "invalid_host", "request Host must match the bound loopback authority")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// discoverTables reads the SQLite schema and returns tables eligible for read-only browsing.
func (s *Server) discoverTables(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM sqlite_master
        WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return fmt.Errorf("discover workspace tables: %w", err)
	}
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	s.tables = make(map[string]tableInfo)
	for _, name := range names {
		columns, err := s.columns(ctx, name)
		if err != nil {
			return err
		}
		s.tables[name] = tableInfo{Name: name, Columns: columns}
	}
	return nil
}

// columns returns ordered metadata for the requested table's columns.
func (s *Server) columns(ctx context.Context, table string) ([]columnInfo, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+quoteIdentifier(table)+")")
	if err != nil {
		return nil, fmt.Errorf("read schema for %q: %w", table, err)
	}
	defer rows.Close()
	var result []columnInfo
	for rows.Next() {
		var cid, pk int
		var name, typ string
		var notNull int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		result = append(result, columnInfo{Name: name, Type: typ, PrimaryKey: pk > 0})
	}
	return result, rows.Err()
}

// quoteIdentifier quotes a validated SQLite identifier and escapes embedded quotes.
func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// hasTable reports whether a table was discovered as browsable.
func (s *Server) hasTable(name string) bool { _, ok := s.tables[name]; return ok }

// hasColumn reports whether a discovered table contains a named column.
func (s *Server) hasColumn(table, column string) bool {
	t, ok := s.tables[table]
	if !ok {
		return false
	}
	for _, c := range t.Columns {
		if c.Name == column {
			return true
		}
	}
	return false
}

// apiError is the stable JSON envelope returned for client-visible failures.
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	} `json:"error"`
}

// apiProblem carries an HTTP status and safe client-facing error message.
type apiProblem struct {
	Code, Message string
	Status        int
	Details       any
}

// Error returns the receiver's diagnostic message.
func (e *apiProblem) Error() string { return e.Message }

// badRequest constructs an API problem with HTTP status 400.
func badRequest(message string) error {
	return &apiProblem{Code: "invalid_request", Message: message, Status: http.StatusBadRequest}
}

// notFound constructs an API problem with HTTP status 404.
func notFound(message string) error {
	return &apiProblem{Code: "not_found", Message: message, Status: http.StatusNotFound}
}

// withJSONErrors converts handler-returned errors into the server's JSON error response.
func withJSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost && r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "request method is not supported")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response with the supplied HTTP status.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// writeError writes the stable JSON error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeDetailedError(w, status, code, message, nil)
}

// writeDetailedError writes the stable JSON error envelope with optional structured details.
func writeDetailedError(w http.ResponseWriter, status int, code, message string, details any) {
	var response apiError
	response.Error.Code = code
	response.Error.Message = message
	response.Error.Details = details
	writeJSON(w, status, response)
}

// respond maps successful values, client problems, and internal failures to safe JSON responses.
func (s *Server) respond(w http.ResponseWriter, r *http.Request, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	var problem *apiProblem
	if errors.As(err, &problem) {
		writeDetailedError(w, problem.Status, problem.Code, problem.Message, problem.Details)
		return
	}
	log.Error("viewer request failed", "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "unable to read workspace data")
}

// queryContext derives a request context bounded by the server query timeout.
func queryContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), requestTimeout)
}

// positiveID parses a strictly positive decimal identifier or returns a bad-request problem.
func positiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, badRequest("id must be a positive integer")
	}
	return id, nil
}

// rowsAsMaps scans SQL rows into maps keyed by result-column name.
func rowsAsMaps(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[columns[i]] = string(bytes)
			} else {
				row[columns[i]] = value
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
