// client_unit_test.go tests fetchOne and Fetch with no external servers.
//go:build unit

package enrich

import (
	"context"
	"errors"
	"testing"
)

// TestClientContextCancellation verifies client context cancellation.
func TestClientContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(SourceConfig{
		Name:          "test",
		RatePerSecond: 1,
		Concurrency:   1,
		TimeoutSecs:   5,
		MaxRetries:    3,
	})
	defer client.Close()

	result := client.Fetch(ctx, "http://unused.example/unused")
	if result.Err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", result.Err)
	}
}
