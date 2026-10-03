//go:build unit

package cache

import (
	"testing"
)

// completeEntry returns one valid cache entry for validation cases.
func completeEntry() *Entry {
	return &Entry{
		Provider:           "crossref",
		Namespace:          "works",
		RequestFingerprint: "fingerprint",
		ExtractorVersion:   "1",
		FetchedAt:          "2026-01-01T00:00:00Z",
		ResponseStatus:     200,
	}
}

// completeUse returns one valid run cache use for validation cases.
func completeUse() *Use {
	return &Use{PipelineRunID: 1, CacheEntryID: 2, CacheLayer: "global", Outcome: "hit"}
}

// TestValidateEntryRejectsInvalidEntries verifies every entry invariant reports the established message.
func TestValidateEntryRejectsInvalidEntries(t *testing.T) {
	if err := validateEntry(nil); err == nil || err.Error() != "cache entry is required" {
		t.Fatalf("nil entry error = %v, want %q", err, "cache entry is required")
	}
	cases := []struct {
		name   string
		mutate func(*Entry)
		want   string
	}{
		{"provider", func(entry *Entry) { entry.Provider = "  " }, "cache entry provider is required"},
		{"namespace", func(entry *Entry) { entry.Namespace = "" }, "cache entry namespace is required"},
		{"request fingerprint", func(entry *Entry) { entry.RequestFingerprint = " " }, "cache entry request fingerprint is required"},
		{"extractor version", func(entry *Entry) { entry.ExtractorVersion = "" }, "cache entry extractor version is required"},
		{"fetched at", func(entry *Entry) { entry.FetchedAt = " " }, "cache entry fetched at is required"},
		{"status below range", func(entry *Entry) { entry.ResponseStatus = 99 }, "cache entry response status 99 is invalid"},
		{"status above range", func(entry *Entry) { entry.ResponseStatus = 600 }, "cache entry response status 600 is invalid"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			entry := completeEntry()
			test.mutate(entry)
			err := validateEntry(entry)
			if err == nil || err.Error() != test.want {
				t.Fatalf("validateEntry error = %v, want %q", err, test.want)
			}
		})
	}
	if err := validateEntry(completeEntry()); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}
}

// TestValidateUseRejectsInvalidUses verifies every run-cache-use invariant reports the established message.
func TestValidateUseRejectsInvalidUses(t *testing.T) {
	if err := validateUse(nil); err == nil || err.Error() != "pipeline run, cache entry, and cache layer are required" {
		t.Fatalf("nil use error = %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*Use)
		want   string
	}{
		{"pipeline run", func(use *Use) { use.PipelineRunID = 0 }, "pipeline run, cache entry, and cache layer are required"},
		{"cache entry", func(use *Use) { use.CacheEntryID = 0 }, "pipeline run, cache entry, and cache layer are required"},
		{"cache layer", func(use *Use) { use.CacheLayer = " " }, "pipeline run, cache entry, and cache layer are required"},
		{"outcome", func(use *Use) { use.Outcome = "invalid" }, `manifest: invalid cache outcome "invalid"`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			use := completeUse()
			test.mutate(use)
			err := validateUse(use)
			if err == nil || err.Error() != test.want {
				t.Fatalf("validateUse error = %v, want %q", err, test.want)
			}
		})
	}
	if err := validateUse(completeUse()); err != nil {
		t.Fatalf("valid use rejected: %v", err)
	}
}
