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
	for _, path := range []string{"/api/health", "/api/searches", "/api/plans?search_revision_id=1", "/api/runs?search_revision_id=1&plan_id=1", "/api/overview?run_id=" + stringID(runID), "/api/audit", "/api/trash", "/api/tables", "/api/tables/work_revisions?page=1&per_page=20&sort=id&order=asc", "/", "/styles/base.css", "/app.js"} {
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

// TestEmbeddedFrontendResearchWorkspaceContract verifies embedded frontend research workspace contract.
func TestEmbeddedFrontendResearchWorkspaceContract(t *testing.T) {
	path, _, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	index := viewerRequest(t, handler, "/")
	for _, expected := range []string{
		"Overview", "Corpus", "Relationships", "Provenance", "Evaluation", "Advanced",
		"Research selections", "Search revision", "Execution plan", "Run attempt", "Local review", "workspace-breadcrumb",
	} {
		if !strings.Contains(index.Body.String(), expected) {
			t.Errorf("embedded index is missing %q", expected)
		}
	}
	if strings.Contains(index.Body.String(), `data-view-link="trash"`) {
		t.Error("embedded index still exposes Trash as a Deepdive tab")
	}
	for _, check := range []struct {
		path, needle string
	}{
		{"/views/home.js", "Choose a captured run"},
		{"/views/home.js", "Move to trash"},
		{"/router.js", "AbortController"},
		{"/views/overview.js", "Captured during execution"},
		{"/views/overview.js", "Derived from stored run data"},
		{"/components/graph.js", "Relationship table"},
		{"/vendor/d3-force.js", "forceSimulation"},
		{"/views/provenance.js", "Stage outcomes"},
		{"/views/evaluation.js", "Normalized article inventory"},
		{"/views/advanced.js", "Advanced database inspection"},
		{"/router.js", "pushState"},
		{"/router.js", "item.setAttribute('href', link({ view: item.dataset.viewLink }))"},
	} {
		asset := viewerRequest(t, handler, check.path)
		if !strings.Contains(asset.Body.String(), check.needle) {
			t.Errorf("embedded frontend %s is missing %q", check.path, check.needle)
		}
	}
	for _, check := range []struct {
		path, needle string
	}{
		{"/styles/base.css", "prefers-reduced-motion"},
		{"/styles/base.css", ".skip-link"},
		{"/styles/collections.css", ".context-panel"},
	} {
		asset := viewerRequest(t, handler, check.path)
		if !strings.Contains(asset.Body.String(), check.needle) {
			t.Errorf("embedded frontend %s is missing %q", check.path, check.needle)
		}
	}
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
