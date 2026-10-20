// Package viewerfixture_test verifies generated database pairs through the actual viewer opener.
package viewerfixture_test

import (
	"analysis/backend/server"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/viewerfixture"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestGeneratedPairRetainsBindingAtCustomPaths verifies both supported metadata filename forms produce a usable portable bundle.
func TestGeneratedPairRetainsBindingAtCustomPaths(t *testing.T) {
	for _, filename := range []string{"workspace.fixture.db", "corpus.metadata.db"} {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bundle", filename)
			if err := viewerfixture.Generate(path, testenv.Path("config/database.something")); err != nil {
				t.Fatal(err)
			}
			viewer, err := server.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer viewer.Close()
			response := httptest.NewRecorder()
			viewer.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
			var health struct {
				Metadata bool `json:"metadata_readable"`
				Review   bool `json:"review_writable"`
				PDF      bool `json:"pdf_store_readable"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &health); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || !health.Metadata || !health.Review || !health.PDF {
				t.Fatalf("status=%d health=%s", response.Code, response.Body.String())
			}
		})
	}
}
