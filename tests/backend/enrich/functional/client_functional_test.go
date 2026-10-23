// client_functional_test.go tests the enrichment HTTP client with mock servers.

package enrich_test

import subject "analysis/backend/enrich"

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// newTestClient creates a valid client for one controlled provider server.
func newTestClient(t *testing.T, server *httptest.Server, retries int) *subject.Client {
	t.Helper()
	client, err := subject.NewClient(subject.SourceConfig{
		Name: "test", BaseURL: server.URL, UserAgent: "Agent/1.0", ContactEmail: "test@example.com",
		RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 5, MaxRetries: retries, BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

// TestClientRetriesOn429 verifies client retries on429.
func TestClientRetriesOn429(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer server.Close()

	client := newTestClient(t, server, 2)
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err != nil {
		t.Fatalf("expected success after retries, got error: %v", response.Err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}
	if requests != 3 {
		t.Fatalf("expected 3 requests (2 retries + 1 success), got %d", requests)
	}
}

// TestClientReturns404WithoutError verifies client returns404 without error.
func TestClientReturns404WithoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(t, server, 2)
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err != nil {
		t.Fatalf("expected no error for 404, got %v", response.Err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", response.StatusCode)
	}
}

// TestClientReturnsErrorOnServerError verifies client returns error on server error.
func TestClientReturnsErrorOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(t, server, 2)
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", response.StatusCode)
	}
}

// TestClientExhaustsRetries verifies client exhausts retries.
func TestClientExhaustsRetries(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newTestClient(t, server, 2)
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err == nil {
		t.Fatal("expected max retries error, got nil")
	}
	if response.Body != nil {
		t.Fatalf("expected no body on retry exhaustion, got %v", response.Body)
	}
	if requests != 3 {
		t.Fatalf("expected 3 requests (1 initial attempt + MaxRetries=2), got %d", requests)
	}
}

// TestClientFetchUsesPublicRateLimitedPath verifies client fetch uses public rate limited path.
func TestClientFetchUsesPublicRateLimitedPath(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("User-Agent") != "Agent/1.0" {
			t.Fatalf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("From") != "test@example.com" {
			t.Fatalf("From = %q", r.Header.Get("From"))
		}
		_, _ = w.Write([]byte(`{"message":{"title":["Test"]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server, 1)
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err != nil || response.StatusCode != http.StatusOK || requests != 1 {
		t.Fatalf("Fetch response = %+v, requests=%d", response, requests)
	}
}

// TestClientRejectsOversizedProviderPayload verifies untrusted provider bodies are bounded.
func TestClientRejectsOversizedProviderPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, strings.Repeat("x", subject.MaxProviderPayloadBytes+1))
	}))
	defer server.Close()

	client := newTestClient(t, server, 1)
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err == nil || !strings.Contains(response.Err.Error(), "exceeds") {
		t.Fatalf("Fetch() error = %v, want payload-size error", response.Err)
	}
}

// TestClientLogsDoNotExposeRequestQueries verifies real request failure logs retain the path while omitting queries and credentials.
func TestClientLogsDoNotExposeRequestQueries(t *testing.T) {
	if os.Getenv("COGEN_QUERY_LOG_CHILD") == "1" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
		}))
		defer server.Close()
		client := newTestClient(t, server, 1)
		defer client.Close()
		response := client.Fetch(context.Background(), server.URL+"/search?q=credit-name%3A%22Ada%20Lovelace%22&token=synthetic-query-token")
		if response.Err == nil {
			t.Fatal("expected controlled transport failure")
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestClientLogsDoNotExposeRequestQueries$")
	cmd.Env = append(os.Environ(), "COGEN_QUERY_LOG_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("log subprocess: %v: %s", err, output)
	}
	logged := string(output)
	for _, secret := range []string{"Ada", "Lovelace", "q=", "credit-name", "synthetic-query-token", "token="} {
		if strings.Contains(logged, secret) {
			t.Fatalf("request log exposed query content: %q", logged)
		}
	}
	if !strings.Contains(logged, "path=/search") || !strings.Contains(logged, "http request failed") {
		t.Fatalf("missing real query-free failure log: %q", logged)
	}
}
