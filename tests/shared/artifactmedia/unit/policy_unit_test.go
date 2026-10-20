// policy_unit_test.go verifies normalized downloads and the artifact preview safety policy.
package artifactmedia_test

import (
	"analysis/internal/artifactmedia"
	"testing"
)

// TestHelper_normalizedArtifactContentType verifies helper normalized artifact content type.
func TestHelper_normalizedArtifactContentType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"application/json", "application/json"},
		{"text/plain; charset=utf-8", "text/plain"},
		{"APPLICATION/JSON", "application/json"},
		{"application/json; charset=utf-8", "application/json"},
		{"application/vnd.api+json", "application/vnd.api+json"},
		{"", "application/octet-stream"},
		{"not/a/valid/mime; invalid=;;", "application/octet-stream"},
		{"text/html", "text/html"},
	}
	for _, tc := range tests {
		got := artifactmedia.Inspect(tc.input).MediaType
		if got != tc.want {
			t.Errorf("normalizedArtifactContentType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestHelper_jsonArtifactContentType verifies helper json artifact content type.
func TestHelper_jsonArtifactContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/vnd.api+json", true},
		{"text/plain", false},
		{"text/html", false},
		{"", false},
	}
	for _, tc := range tests {
		got := artifactmedia.Inspect(tc.input).JSON
		if got != tc.want {
			t.Errorf("jsonArtifactContentType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestHelper_inlineArtifactContentType verifies helper inline artifact content type.
func TestHelper_inlineArtifactContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"text/plain", true},
		{"text/plain; charset=utf-8", true},
		{"text/csv", true},
		{"application/json", true},
		{"application/vnd.api+json", true},
		{"application/x-something-config", true},
		{"text/html", false},
		{"application/xhtml+xml", false},
		{"application/octet-stream", false},
		{"image/png", false},
		{"", false},
	}
	for _, tc := range tests {
		got := artifactmedia.Inspect(tc.input).Inline
		if got != tc.want {
			t.Errorf("inlineArtifactContentType(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
