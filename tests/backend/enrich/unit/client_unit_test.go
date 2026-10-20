// client_unit_test.go tests fetchOne and Fetch with no external servers.

package enrich_test

import subject "analysis/backend/enrich"

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestClientContextCancellation verifies client context cancellation.
func TestClientContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := subject.NewClient(subject.SourceConfig{
		Name:          "test",
		BaseURL:       "https://example.invalid/",
		RatePerSecond: 1,
		Concurrency:   1,
		TimeoutSecs:   5,
		MaxRetries:    3,
		BatchSize:     1,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	result := client.Fetch(ctx, "http://unused.example/unused")
	if result.Err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", result.Err)
	}
}

// TestNewClientRejectsInvalidSourceConfig verifies defensive validation for direct callers.
func TestNewClientRejectsInvalidSourceConfig(t *testing.T) {
	valid := subject.SourceConfig{Name: "test", BaseURL: "https://provider.example/api", RatePerSecond: 1, Concurrency: 1, TimeoutSecs: 1, MaxRetries: 1, BatchSize: 1}
	tests := []struct {
		name   string
		mutate func(*subject.SourceConfig)
	}{
		{"relative base URL", func(cfg *subject.SourceConfig) { cfg.BaseURL = "/api" }},
		{"missing rate", func(cfg *subject.SourceConfig) { cfg.RatePerSecond = 0 }},
		{"ticker zero rate", func(cfg *subject.SourceConfig) { cfg.RatePerSecond = int(time.Second) + 1 }},
		{"unbounded concurrency", func(cfg *subject.SourceConfig) { cfg.Concurrency = subject.maxProviderConcurrency + 1 }},
		{"unbounded timeout", func(cfg *subject.SourceConfig) { cfg.TimeoutSecs = subject.maxProviderTimeoutSeconds + 1 }},
		{"unbounded retries", func(cfg *subject.SourceConfig) { cfg.MaxRetries = subject.maxProviderRetries + 1 }},
		{"unbounded batch size", func(cfg *subject.SourceConfig) { cfg.BatchSize = subject.maxProviderBatchSize + 1 }},
		{"missing timeout", func(cfg *subject.SourceConfig) { cfg.TimeoutSecs = 0 }},
		{"missing retries", func(cfg *subject.SourceConfig) { cfg.MaxRetries = 0 }},
		{"missing batch size", func(cfg *subject.SourceConfig) { cfg.BatchSize = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.mutate(&cfg)
			if client, err := subject.NewClient(cfg); err == nil {
				client.Close()
				t.Fatal("NewClient() unexpectedly succeeded")
			}
		})
	}
}

// TestRequestLogTargetExcludesQuery verifies provider logs cannot contain request query data.
func TestRequestLogTargetExcludesQuery(t *testing.T) {
	host, path := subject.requestLogTarget("https://orcid.example/search?q=credit-name%3A%22Ada%20Lovelace%22")
	if host != "orcid.example" || path != "/search" {
		t.Fatalf("requestLogTarget() = (%q, %q)", host, path)
	}
}
