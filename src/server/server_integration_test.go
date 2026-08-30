// server_integration_test.go tests server-level endpoints, frontend asset
// serving, the read-only query connection, and bounded local mutations.
//
//go:build integration

package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunContextReturnsCanonicalAncestryAndLifecycle verifies one run determines every visible parent identifier.
func TestRunContextReturnsCanonicalAncestryAndLifecycle(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	response := viewerRequest(t, viewer.Handler(), "/api/runs/"+stringID(runID)+"/context")
	if response.Code != http.StatusOK {
		t.Fatalf("run context status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Search struct {
			ID       int64  `json:"id"`
			SearchID string `json:"search_id"`
		} `json:"search"`
		Revision struct {
			ID       int64 `json:"id"`
			SearchID int64 `json:"search_id"`
		} `json:"revision"`
		Plan struct {
			ID               int64 `json:"id"`
			SearchRevisionID int64 `json:"search_revision_id"`
		} `json:"plan"`
		Run struct {
			ID              int64  `json:"id"`
			ExecutionPlanID int64  `json:"execution_plan_id"`
			Status          string `json:"status"`
			VisibilityState string `json:"visibility_state"`
		} `json:"run"`
		Lifecycle struct {
			ReviewWritable bool `json:"review_writable"`
		} `json:"lifecycle"`
		Review struct {
			Initialized bool `json:"initialized"`
		} `json:"review"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	data := payload
	if data.Search.ID < 1 || data.Search.SearchID != "research" || data.Revision.SearchID != data.Search.ID {
		t.Fatalf("canonical search ancestry = %+v %+v", data.Search, data.Revision)
	}
	if data.Plan.SearchRevisionID != data.Revision.ID || data.Run.ExecutionPlanID != data.Plan.ID || data.Run.ID != runID {
		t.Fatalf("canonical run ancestry = %+v %+v %+v", data.Revision, data.Plan, data.Run)
	}
	if data.Run.Status != "completed" || data.Run.VisibilityState != "active" || !data.Lifecycle.ReviewWritable || data.Review.Initialized {
		t.Fatalf("canonical lifecycle = run %+v lifecycle %+v review %+v", data.Run, data.Lifecycle, data.Review)
	}

	missing := viewerRequest(t, viewer.Handler(), "/api/runs/999999/context")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"code":"not_found"`) {
		t.Fatalf("missing run context status=%d body=%s", missing.Code, missing.Body.String())
	}
}

// TestHealthReportsIndependentCapabilities verifies absent PDF storage is not reported as readable.
func TestHealthReportsIndependentCapabilities(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	status, payload := requestJSON(t, viewer.Handler(), "/api/health")
	if status != http.StatusOK {
		t.Fatalf("health status=%d payload=%+v", status, payload)
	}
	data := payload
	if data["metadata_readable"] != true || data["review_writable"] != true {
		t.Fatalf("metadata capabilities = %+v", data)
	}
	if data["pdf_store_bound"] != false || data["pdf_store_readable"] != false {
		t.Fatalf("unbound PDF capabilities = %+v", data)
	}
	review := data["review"].(map[string]any)
	if review["pdf_store_read_only"] != false {
		t.Fatalf("legacy PDF capability = %+v", review)
	}
}

// TestOpenIsReadOnlyAndDoesNotCreateMissingDatabase verifies open is read only and does not create missing database.
func TestOpenIsReadOnlyAndDoesNotCreateMissingDatabase(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err := Open(missing); err == nil {
		t.Fatal("Open missing database succeeded")
	}
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	if _, err := viewer.db.Exec("INSERT INTO searches (search_id) VALUES ('must-not-write')"); err == nil {
		t.Fatal("read-only database accepted write")
	}
	var count int
	if err := viewer.db.QueryRow("SELECT COUNT(*) FROM searches").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("search count changed through read-only viewer: %d", count)
	}
}

// TestAPIWorkspaceDiscoveryAndSafePagination verifies api workspace discovery and safe pagination.
func TestAPIWorkspaceDiscoveryAndSafePagination(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	for _, path := range []string{"/api/health", "/api/searches", "/api/plans?search_revision_id=1", "/api/runs?search_revision_id=1&plan_id=1", "/api/overview?run_id=" + stringID(runID), "/api/audit", "/api/trash", "/api/tables", "/api/tables/work_revisions?page=1&per_page=20&sort=id&order=asc"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/runs?bad=1", "/api/unknown"} {
		response := viewerRequest(t, handler, path)
		if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
			t.Errorf("GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"error"`) {
			t.Errorf("GET %s did not return JSON API error", path)
		}
	}
}

// TestDiskServedFrontendContract verifies frontend assets served from a filesystem directory.
func TestDiskServedFrontendContract(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	assetDir := t.TempDir()
	index := `<!doctype html><html lang="en"><head><title>Research workspace</title></head><body>
<header><span id="health-status"></span><span>Local review</span></header>
<div id="workspace-breadcrumb"></div>
<nav><a data-view-link="overview">Overview</a><a data-view-link="corpus">Corpus</a><a data-view-link="provenance">Provenance</a></nav>
<script type="module" src="/app.js"></script></body></html>`
	if err := os.WriteFile(filepath.Join(assetDir, "index.html"), []byte(index), 0644); err != nil {
		t.Fatal(err)
	}
	for _, viewFile := range []string{"overview", "corpus", "relationships", "provenance", "evaluation", "advanced", "article", "author", "reference"} {
		viewDocument := strings.Replace(index, "<title>Research workspace</title>", "<title>"+strings.Title(viewFile)+" · Research workspace</title>", 1)
		if err := os.WriteFile(filepath.Join(assetDir, viewFile+".html"), []byte(viewDocument), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(assetDir, "app.js"), []byte("export const marker = 'forceSimulation';\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("serves index with content type and sentinels", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		handler := viewer.Handler()
		index := viewerRequest(t, handler, "/")
		if index.Code != http.StatusOK {
			t.Fatalf("index status=%d body=%s", index.Code, index.Body.String())
		}
		if contentType := index.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Errorf("index content type = %q, want text/html", contentType)
		}
		for _, expected := range []string{
			"Overview", "Corpus", "Provenance",
			"Local review", "workspace-breadcrumb",
		} {
			if !strings.Contains(index.Body.String(), expected) {
				t.Errorf("served index is missing %q", expected)
			}
		}
		if strings.Contains(index.Body.String(), `data-view-link="trash"`) {
			t.Error("served index still exposes Trash as a Deepdive tab")
		}
	})

	t.Run("serves a view page with an HTML content type", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		page := viewerRequest(t, viewer.Handler(), "/overview")
		if page.Code != http.StatusOK {
			t.Fatalf("overview page status=%d body=%s", page.Code, page.Body.String())
		}
		if contentType := page.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Errorf("overview page content type = %q, want text/html", contentType)
		}
		if !strings.Contains(page.Body.String(), "Overview · Research workspace") {
			t.Error("served overview page is missing its page title")
		}
	})

	t.Run("serves every extensionless view path with its owning document", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		handler := viewer.Handler()
		for _, viewPath := range []string{"/overview", "/corpus", "/relationships", "/provenance", "/evaluation", "/advanced", "/article", "/author", "/reference", "/trash"} {
			page := viewerRequest(t, handler, viewPath)
			if page.Code != http.StatusOK {
				t.Errorf("GET %s: status=%d body=%s", viewPath, page.Code, page.Body.String())
			}
			if contentType := page.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
				t.Errorf("GET %s: content type = %q, want text/html", viewPath, contentType)
			}
		}
	})

	t.Run("serves the physical view documents through the filesystem fallback", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		page := viewerRequest(t, viewer.Handler(), "/overview.html")
		if page.Code != http.StatusOK {
			t.Fatalf("overview.html status=%d body=%s", page.Code, page.Body.String())
		}
		if !strings.Contains(page.Body.String(), "Overview · Research workspace") {
			t.Error("served overview.html is missing its page title")
		}
	})

	t.Run("serves module assets with sentinels", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		handler := viewer.Handler()
		asset := viewerRequest(t, handler, "/app.js")
		if asset.Code != http.StatusOK {
			t.Fatalf("asset status=%d body=%s", asset.Code, asset.Body.String())
		}
		if !strings.Contains(asset.Body.String(), "forceSimulation") {
			t.Errorf("served asset is missing %q", "forceSimulation")
		}
	})

	t.Run("returns 404 for a missing file", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		response := viewerRequest(t, viewer.Handler(), "/missing.js")
		if response.Code != http.StatusNotFound {
			t.Errorf("missing asset status=%d body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("cleans raw traversal paths through the mux", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		handler := viewer.Handler()
		for _, traversal := range []string{"/../", "/../app.js"} {
			response := viewerRequest(t, handler, traversal)
			if response.Code < 300 || response.Code >= 400 {
				t.Errorf("GET %s: status=%d body=%s; want redirect", traversal, response.Code, response.Body.String())
			}
			if location := response.Header().Get("Location"); location == "" {
				t.Errorf("GET %s: redirect without Location header", traversal)
			}
			if strings.Contains(response.Body.String(), "Research workspace") {
				t.Errorf("GET %s: redirect leaks served content: %q", traversal, response.Body.String())
			}
		}
	})

	t.Run("sanitizes escaped traversal paths", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		viewer.AssetsFS = os.DirFS(assetDir)
		response := viewerRequest(t, viewer.Handler(), "/%2e%2e/%2e%2e/etc/passwd")
		if response.Code != http.StatusNotFound {
			t.Errorf("escaped traversal status=%d body=%s; want 404", response.Code, response.Body.String())
		}
	})

	t.Run("returns JSON 503 when assets are not configured", func(t *testing.T) {
		viewer, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer viewer.Close()
		response := viewerRequest(t, viewer.Handler(), "/app.js")
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("unconfigured assets status=%d body=%s", response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"assets_not_configured"`) {
			t.Errorf("unconfigured assets body is missing error code:\n%s", response.Body.String())
		}
	})
}

// TestHandlerServesFilesystemAssets verifies handler serves filesystem assets.
func TestHandlerServesFilesystemAssets(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	assetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetDir, "index.html"), []byte("filesystem viewer"), 0644); err != nil {
		t.Fatal(err)
	}
	viewer.AssetsFS = os.DirFS(assetDir)

	response := viewerRequest(t, viewer.Handler(), "/")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "filesystem viewer") {
		t.Fatalf("filesystem frontend response: status=%d body=%q", response.Code, response.Body.String())
	}
}
