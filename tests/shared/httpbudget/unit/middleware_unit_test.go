// middleware_unit_test.go verifies response buffering, SQL ceilings, and stable error reporting through the production middleware.
package httpbudget_test

import (
	"analysis/internal/httpbudget"
	"analysis/internal/querybudget"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// bounded applies a fixed policy so tests isolate the generic middleware from viewer routing.
func bounded(next http.Handler) http.Handler {
	return httpbudget.Middleware(next, func(string) httpbudget.Limits { return httpbudget.Limits{Bytes: 512 * 1024, Queries: 64} }, func(w http.ResponseWriter, status int, code, message string, details any) {
		w.WriteHeader(status)
		writeJSON(w, status, map[string]any{"code": code, "message": message, "details": details})
	})
}

// writeJSON writes a controlled handler response for middleware scenarios.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// TestResponseCeilingAndSuccessfulBuffering verifies oversized JSON fails atomically and bounded responses retain status and bytes.
func TestResponseCeilingAndSuccessfulBuffering(t *testing.T) {
	handler := bounded(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"payload": strings.Repeat("x", (512 * 1024))})
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"response_budget_exceeded"`) {
		t.Fatalf("oversized response code=%d body=%s", response.Code, response.Body.String())
	}
	if response.Body.Len() > (512 * 1024) {
		t.Fatalf("budget error response size=%d, limit=%d", response.Body.Len(), (512 * 1024))
	}
	if got := response.Header().Get("X-Response-Byte-Limit"); got != fmt.Sprint((512 * 1024)) {
		t.Fatalf("response budget header=%q", got)
	}

	response = httptest.NewRecorder()
	bounded(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]bool{"bounded": true})
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/audit", nil))
	if response.Code != http.StatusAccepted || response.Body.String() != "{\"bounded\":true}\n" {
		t.Fatalf("bounded response code=%d body=%q", response.Code, response.Body.String())
	}
}

// TestStatementCeilingRejectsSuccessfulHandlerResult verifies a handler cannot conceal budget exhaustion with a success response.
func TestStatementCeilingRejectsSuccessfulHandlerResult(t *testing.T) {
	handler := bounded(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for index := 0; index <= 64; index++ {
			_ = querybudget.Consume(r.Context())
		}
		writeJSON(w, http.StatusOK, map[string]bool{"unreachable": true})
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"query_budget_exceeded"`) {
		t.Fatalf("query budget response code=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Query-Count-Limit"); got != fmt.Sprint(64) {
		t.Fatalf("query budget limit header=%q", got)
	}
	if got := response.Header().Get("X-Query-Count-Used"); got != fmt.Sprint(64+1) {
		t.Fatalf("query budget used header=%q", got)
	}
}
