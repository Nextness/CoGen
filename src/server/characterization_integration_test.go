// characterization_integration_test.go pins viewer request-context behavior
// that the migration must preserve across the HTTP-to-repository boundary.
//go:build integration

package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCharacterizationRequestContextCancellationDoesNotMutate verifies a
// canceled request context fails the mutation before any row is committed.
func TestCharacterizationRequestContextCancellationDoesNotMutate(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	handler := fixture.server.Handler()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.runID),
		bytes.NewBufferString(`{"parent_context_id":null}`)).WithContext(canceled)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("canceled mutation status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var contexts int
	if err := fixture.server.writeDB.DB.QueryRow("SELECT COUNT(*) FROM review_contexts WHERE pipeline_run_id=?", fixture.runID).Scan(&contexts); err != nil {
		t.Fatal(err)
	}
	if contexts != 0 {
		t.Fatalf("canceled mutation wrote %d review contexts", contexts)
	}
}
