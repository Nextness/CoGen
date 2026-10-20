package cache_test

import subject "analysis/backend/database/cache"

import (
	"analysis/tests/support/go/testfixture"
	"context"
	"strings"
	"testing"
)

// completeEntry returns one valid cache entry for validation cases.
func completeEntry() *subject.Entry {
	return &subject.Entry{
		Provider:           "crossref",
		Namespace:          "works",
		RequestFingerprint: "fingerprint",
		ExtractorVersion:   "1",
		FetchedAt:          "2026-01-01T00:00:00Z",
		ResponseStatus:     200,
	}
}

// completeUse returns one valid run cache use for validation cases.
func completeUse() *subject.Use {
	return &subject.Use{PipelineRunID: 1, CacheEntryID: 2, CacheLayer: "global", Outcome: "hit"}
}

// TestValidateEntryRejectsInvalidEntries verifies every entry invariant reports the established message.
func TestValidateEntryRejectsInvalidEntries(t *testing.T) {
	if err := entryValidationError(nil); err == nil || err.Error() != "cache entry is required" {
		t.Fatalf("nil entry error = %v, want %q", err, "cache entry is required")
	}
	cases := []struct {
		name   string
		mutate func(*subject.Entry)
		want   string
	}{
		{"provider", func(entry *subject.Entry) { entry.Provider = "  " }, "cache entry provider is required"},
		{"namespace", func(entry *subject.Entry) { entry.Namespace = "" }, "cache entry namespace is required"},
		{"request fingerprint", func(entry *subject.Entry) { entry.RequestFingerprint = " " }, "cache entry request fingerprint is required"},
		{"extractor version", func(entry *subject.Entry) { entry.ExtractorVersion = "" }, "cache entry extractor version is required"},
		{"fetched at", func(entry *subject.Entry) { entry.FetchedAt = " " }, "cache entry fetched at is required"},
		{"status below range", func(entry *subject.Entry) { entry.ResponseStatus = 99 }, "cache entry response status 99 is invalid"},
		{"status above range", func(entry *subject.Entry) { entry.ResponseStatus = 600 }, "cache entry response status 600 is invalid"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			entry := completeEntry()
			test.mutate(entry)
			err := entryValidationError(entry)
			if err == nil || err.Error() != test.want {
				t.Fatalf("validateEntry error = %v, want %q", err, test.want)
			}
		})
	}
	if err := entryValidationError(completeEntry()); err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("valid entry rejected: %v", err)
	}
}

// TestValidateUseRejectsInvalidUses verifies every run-cache-use invariant reports the established message.
func TestValidateUseRejectsInvalidUses(t *testing.T) {
	if err := useValidationError(nil); err == nil || err.Error() != "pipeline run, cache entry, and cache layer are required" {
		t.Fatalf("nil use error = %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*subject.Use)
		want   string
	}{
		{"pipeline run", func(use *subject.Use) { use.PipelineRunID = 0 }, "pipeline run, cache entry, and cache layer are required"},
		{"cache entry", func(use *subject.Use) { use.CacheEntryID = 0 }, "pipeline run, cache entry, and cache layer are required"},
		{"cache layer", func(use *subject.Use) { use.CacheLayer = " " }, "pipeline run, cache entry, and cache layer are required"},
		{"outcome", func(use *subject.Use) { use.Outcome = "invalid" }, `manifest: invalid cache outcome "invalid"`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			use := completeUse()
			test.mutate(use)
			err := useValidationError(use)
			if err == nil || err.Error() != test.want {
				t.Fatalf("validateUse error = %v, want %q", err, test.want)
			}
		})
	}
	if err := useValidationError(completeUse()); err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("valid use rejected: %v", err)
	}
}

// entryValidationError observes entry validation through the supported append operation.
func entryValidationError(entry *subject.Entry) error {
	_, err := subject.New(testfixture.ClosedDB()).AppendEntry(context.Background(), entry)
	return err
}

// useValidationError observes use validation through the supported append operation.
func useValidationError(use *subject.Use) error {
	_, err := subject.New(testfixture.ClosedDB()).AppendUse(context.Background(), use)
	return err
}
