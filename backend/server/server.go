// Package server exposes an existing workspace database through a local HTTP
// API with read-only research queries and bounded review and lifecycle writes.
// It deliberately does not run migrations while opening the viewer.
package server

import (
	viewrequest "analysis/backend/server/request"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"strings"
	"sync"
	"time"

	"analysis/backend/bundle"
	"analysis/backend/database"
	"analysis/backend/database/artifact"
	"analysis/backend/database/audit"
	"analysis/backend/database/author"
	"analysis/backend/database/cache"
	"analysis/backend/database/pdfbinding"
	"analysis/backend/database/review"
	"analysis/backend/database/run"
	"analysis/backend/database/search"
	"analysis/backend/database/source"
	"analysis/backend/database/work"
	"analysis/backend/logging"
	"analysis/backend/pdfstore"
	"analysis/internal/httpbudget"
	"analysis/internal/querybudget"
	"analysis/internal/sqliteread"

	"modernc.org/sqlite"
)

// queryBudgetDriverName selects the viewer's context-budgeted SQLite connections.
const queryBudgetDriverName = "analysis-viewer-budgeted-sqlite"

// init registers the viewer's production driver once per process.
func init() { sql.Register(queryBudgetDriverName, querybudget.WrapDriver(&sqlite.Driver{})) }

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

// encodeCursor serializes one endpoint-bound keyset without exposing its structure.
func encodeCursor(cursor any) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

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
// Viewer-facing family data-access layers bind to the server's already opened
// statement-budgeted connections: the metadata layers share db, the companion
// PDF layer binds pdfDB for evidence reads and pdfWriteStore for validated
// mutations. Every method receives the handler's context so cancellation and
// query counting continue through generated calls.
// AssetsFS is the frontend asset file system served at the web root; it must
// be set because the binary does not embed frontend assets.
type Server struct {
	db               *sql.DB
	artifact         *artifact.Store
	auditStore       *audit.Store
	workStore        *work.Store
	searchStore      *search.Store
	runStore         *run.Store
	sourceStore      *source.Store
	authorStore      *author.Store
	reviewStore      *review.Store
	cacheStore       *cache.Store
	pdfBindings      *pdfbinding.Store
	writeDB          *database.Database
	pdfDB            *sql.DB
	pdfStore         *pdfstore.Store
	pdfWriteDB       *sql.DB
	pdfWriteStore    *pdfstore.Store
	pdfPath          string
	pdfWritable      bool
	pdfWriteReason   string
	maintenance      *database.MaintenanceLock
	metadataActual   string
	metadataTarget   string
	metadataReady    bool
	pdfActual        string
	pdfTarget        string
	pdfReady         bool
	schemaReady      bool
	pdfCacheMu       sync.Mutex
	pdfCache         *cachedPDF
	pdfUploads       chan struct{}
	pdfExports       chan struct{}
	pdfZIPExports    chan struct{}
	auditRelayMu     sync.Mutex
	auditRelayCancel context.CancelFunc
	auditRelayDone   chan struct{}
	auditRelayError  string
	tables           map[string]tableInfo
	AssetsFS         fs.FS // serves frontend assets from this filesystem
}

// maintenanceWaitTimeout bounds startup waiting for an active schema preparation.
const maintenanceWaitTimeout = 30 * time.Second

// Open opens an existing database without creating it, migrating it, or
// repairing it. It verifies both configured schema targets, acquires shared
// maintenance ownership for the server lifetime, and prepares a separate
// existing-only companion write connection when the PDF store is writable.
func Open(path, registryPath string) (*Server, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if strings.TrimSpace(registryPath) == "" {
		return nil, errors.New("database registry path is required")
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
	maintenanceCtx, cancelMaintenance := context.WithTimeout(context.Background(), maintenanceWaitTimeout)
	maintenance, err := database.AcquireMaintenanceLock(maintenanceCtx, absolute, database.MaintenanceShared)
	cancelMaintenance()
	if err != nil {
		return nil, fmt.Errorf("acquire shared maintenance ownership: %w", err)
	}
	s := &Server{
		maintenance: maintenance, pdfUploads: make(chan struct{}, pdfUploadConcurrency),
		pdfExports: make(chan struct{}, pdfExportConcurrency), pdfZIPExports: make(chan struct{}, 1),
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = s.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	readiness, err := bundle.VerifyMetadataReadiness(ctx, registryPath, absolute)
	if err != nil {
		return nil, fmt.Errorf("verify metadata readiness: %w", err)
	}
	s.metadataActual = readiness.Metadata
	s.metadataTarget = readiness.MetadataTarget
	s.metadataReady = true
	db, err := sqliteread.Open(absolute, queryBudgetDriverName)
	if err != nil {
		return nil, fmt.Errorf("open workspace database read-only: %w", err)
	}
	s.db = db
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("read workspace database: %w", err)
	}
	s.artifact = artifact.New(db)
	s.auditStore = audit.New(db)
	s.workStore = work.New(db)
	s.searchStore = search.New(db)
	s.runStore = run.New(db)
	s.sourceStore = source.New(db)
	s.authorStore = author.New(db)
	s.reviewStore = review.New(db)
	s.cacheStore = cache.New(db)
	s.pdfBindings = pdfbinding.New(db)
	if err := s.discoverTables(ctx); err != nil {
		return nil, err
	}
	if err := s.verifyReviewSchema(ctx); err != nil {
		return nil, err
	}
	writeDB, err := database.OpenExistingWithDriver(absolute, queryBudgetDriverName)
	if err != nil {
		return nil, fmt.Errorf("open metadata mutation connection: %w", err)
	}
	s.writeDB = writeDB
	if err := s.openBoundPDFStore(ctx, absolute, registryPath); err != nil {
		return nil, err
	}
	s.schemaReady = s.metadataReady && s.pdfReady
	cleanup = false
	return s, nil
}

// Close releases resources owned by the receiver.
func (s *Server) Close() error {
	s.StopAuditRelay()
	var first error
	if s.pdfWriteDB != nil {
		if err := s.pdfWriteDB.Close(); err != nil {
			first = err
		}
		s.pdfWriteDB = nil
		s.pdfWriteStore = nil
	}
	if s.pdfDB != nil {
		if err := s.pdfDB.Close(); err != nil && first == nil {
			first = err
		}
		s.pdfDB = nil
		s.pdfStore = nil
	}
	if s.writeDB != nil {
		if err := s.writeDB.Close(); err != nil && first == nil {
			first = err
		}
		s.writeDB = nil
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil && first == nil {
			first = err
		}
		s.db = nil
	}
	if s.maintenance != nil {
		if err := s.maintenance.Close(); err != nil && first == nil {
			first = err
		}
		s.maintenance = nil
	}
	return first
}

// PDFStoreBound reports whether a readable companion PDF database is attached.
func (s *Server) PDFStoreBound() bool { return s.pdfDB != nil }

// PDFStoreWritable reports whether a writable companion connection is attached.
func (s *Server) PDFStoreWritable() bool { return s.pdfWriteDB != nil }

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
	mux.HandleFunc("PUT /api/runs/{run_id}/articles/{work_revision_id}/pdf", s.saveArticlePDF)
	mux.HandleFunc("DELETE /api/runs/{run_id}/articles/{work_revision_id}/pdf", s.removeArticlePDF)
	mux.HandleFunc("POST /api/runs/{run_id}/articles/{work_revision_id}/pdf-export", s.exportArticlePDF)
	mux.HandleFunc("POST /api/runs/{run_id}/pdf-exports", s.exportPDFs)
	mux.HandleFunc("GET /api/runs/{run_id}/articles/{work_revision_id}/pdf", pdfMethodNotAllowed)
	mux.HandleFunc("POST /api/runs/{run_id}/articles/{work_revision_id}/pdf", pdfMethodNotAllowed)
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
		return withPDFExportDeadlines(withPDFUploadDeadlines(withAPIResponseBudgets(withJSONErrors(mux))))
	}
	for path, file := range viewPages {
		mux.HandleFunc("GET "+path, s.serveViewPage(file))
	}
	mux.Handle("GET /", http.FileServer(http.FS(s.AssetsFS)))
	return withPDFExportDeadlines(withPDFUploadDeadlines(withAPIResponseBudgets(withJSONErrors(mux))))
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
	if !strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api/pdf/") || isPDFExportPath(path) || (strings.HasPrefix(path, "/api/artifacts/") && strings.HasSuffix(path, "/content")) {
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
	if isPDFExportPath(path) {
		return detailAPIQueries
	}
	if path == "/api/graph" || strings.HasPrefix(path, "/api/articles/") || strings.HasPrefix(path, "/api/authors/") || strings.HasPrefix(path, "/api/references/") || strings.HasPrefix(path, "/api/tables/") || strings.Contains(path, "/review") || strings.Contains(path, "/notes") || strings.Contains(path, "/anchors") || strings.Contains(path, "/links/backlinks") {
		return detailAPIQueries
	}
	if path == "/api/audit" || strings.HasPrefix(path, "/api/audit/") || path == "/api/hierarchy" || strings.Contains(path, "/corpus/") || strings.Contains(path, "/evaluation") || strings.Contains(path, "/identity-evidence") || strings.Contains(path, "/cache-uses") || strings.Contains(path, "/stages") || strings.Contains(path, "/artifacts") {
		return collectionAPIQueries
	}
	return compactAPIQueries
}

// isPDFExportPath reports explicit binary export resources outside the JSON byte budget.
func isPDFExportPath(path string) bool {
	return strings.HasSuffix(path, "/pdf-export") || strings.HasSuffix(path, "/pdf-exports")
}

// withAPIResponseBudgets applies the viewer's route policy and stable error envelope.
func withAPIResponseBudgets(next http.Handler) http.Handler {
	return httpbudget.Middleware(next, func(path string) httpbudget.Limits {
		return httpbudget.Limits{Bytes: apiResponseByteBudget(path), Queries: apiQueryBudget(path)}
	}, writeDetailedError)
}

// openBoundPDFStore resolves, verifies, and opens the companion PDF database declared by metadata.
func (s *Server) openBoundPDFStore(ctx context.Context, metadataPath, registryPath string) error {
	if !s.hasTable("pdf_store_binding") {
		return fmt.Errorf("metadata schema is missing the PDF store binding table; run analysis migrate")
	}
	relativePath, found, err := s.pdfBindings.Binding(ctx)
	if err != nil {
		return err
	}
	if !found {
		s.pdfTarget = bundle.SupportedPair.PDF
		s.pdfReady = true
		s.pdfWriteReason = "no PDF store binding"
		return nil
	}
	if filepath.IsAbs(relativePath) {
		return fmt.Errorf("bound PDF store path must be relative")
	}
	resolvedStorePath, err := pdfbinding.ResolveStorePath(metadataPath, relativePath)
	if err != nil {
		return fmt.Errorf("resolve bound PDF store: %w", err)
	}
	info, err := os.Stat(resolvedStorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("bound PDF store %q is missing; restore it or update the metadata binding instead of creating an empty replacement", relativePath)
		}
		return fmt.Errorf("inspect bound PDF store: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("bound PDF store path is not a regular file")
	}
	readiness, err := bundle.VerifyPDFReadiness(ctx, registryPath, resolvedStorePath)
	if err != nil {
		return fmt.Errorf("verify PDF store readiness: %w", err)
	}
	s.pdfActual = readiness.PDF
	s.pdfTarget = readiness.PDFTarget
	s.pdfReady = true
	pdfDB, err := sqliteread.Open(resolvedStorePath, queryBudgetDriverName)
	if err != nil {
		return fmt.Errorf("open PDF store read-only: %w", err)
	}
	if err := pdfDB.PingContext(ctx); err != nil {
		_ = pdfDB.Close()
		return fmt.Errorf("read PDF store: %w", err)
	}
	if err := pdfstore.VerifySchema(ctx, pdfDB); err != nil {
		_ = pdfDB.Close()
		return fmt.Errorf("verify PDF store schema: %w", err)
	}
	// Two explicit export snapshots may hold the companion read pool at once.
	// The store remains read-only; this only admits the bounded export concurrency.
	pdfDB.SetMaxOpenConns(pdfExportConcurrency)
	s.pdfDB = pdfDB
	s.pdfStore = pdfstore.New(pdfDB)
	s.pdfPath = resolvedStorePath
	if err := writableFile(resolvedStorePath); err != nil {
		s.pdfWriteReason = fmt.Sprintf("PDF store is read-only: %v", err)
		return nil
	}
	pdfWriteDB, err := database.OpenExistingConnectionWithDriver(resolvedStorePath, queryBudgetDriverName)
	if err != nil {
		s.pdfWriteReason = fmt.Sprintf("PDF store write connection unavailable: %v", err)
		return nil
	}
	if err := pdfstore.VerifyMutationConnection(ctx, pdfWriteDB); err != nil {
		_ = pdfWriteDB.Close()
		s.pdfWriteReason = fmt.Sprintf("PDF store write connection is not safely configured: %v", err)
		return nil
	}
	s.pdfWriteDB = pdfWriteDB
	s.pdfWriteStore = pdfstore.New(pdfWriteDB)
	s.pdfWritable = true
	return nil
}

// writableFile verifies that a companion file can be opened for writing without changing it.
func writableFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return file.Close()
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

// apiError is the stable JSON envelope returned for client-visible failures.
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	} `json:"error"`
}

// withJSONErrors converts handler-returned errors into the server's JSON error response.
func withJSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
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
	var problem *viewrequest.Problem
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
