// client_functional_test.go tests the enrichment HTTP client with mock servers.
//go:build functional

package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

	client := NewClient(SourceConfig{
		UserAgent: "Agent/1.0", ContactEmail: "test@example.com",
		RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 3,
	})
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

	client := NewClient(SourceConfig{
		UserAgent: "Agent/1.0", ContactEmail: "test@example.com",
		RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 2,
	})
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

	client := NewClient(SourceConfig{
		UserAgent: "Agent/1.0", ContactEmail: "test@example.com",
		RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 2,
	})
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

	client := NewClient(SourceConfig{
		UserAgent: "Agent/1.0", ContactEmail: "test@example.com",
		RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 2,
	})
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err == nil {
		t.Fatal("expected max retries error, got nil")
	}
	if response.Body != nil {
		t.Fatalf("expected no body on retry exhaustion, got %v", response.Body)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests (MaxRetries=2), got %d", requests)
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

	client := NewClient(SourceConfig{
		UserAgent: "Agent/1.0", ContactEmail: "test@example.com",
		RatePerSecond: 1000, Concurrency: 1, TimeoutSecs: 5, MaxRetries: 1,
	})
	defer client.Close()
	response := client.Fetch(context.Background(), server.URL)
	if response.Err != nil || response.StatusCode != http.StatusOK || requests != 1 {
		t.Fatalf("Fetch response = %+v, requests=%d", response, requests)
	}
}
