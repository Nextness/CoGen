// server_integration_test.go tests server-level endpoints, frontend asset
// serving, the read-only query connection, and bounded local mutations.
//
//go:build integration

package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
