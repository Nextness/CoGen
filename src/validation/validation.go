// Package validation provides pure workspace validation and JSON helpers.
package validation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Fields is the workspace-facing validation input. It contains no mutable
// database identity or persistence state.
type Fields struct {
	DOI            string
	Title          string
	Year           int
	Publisher      string
	ReferenceCount int
}

var doiPattern = regexp.MustCompile(`^10\.\d{4,}/\S+$`)

// IsRealDOI checks whether a string looks like a DOI.
func IsRealDOI(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 60 && doiPattern.MatchString(value)
}

// ValidateFields runs the workspace validation rules.
func ValidateFields(a Fields, authorCount int) []string {
	var reasons []string
	if a.Title == "" {
		reasons = append(reasons, "missing title")
	}
	if authorCount == 0 {
		reasons = append(reasons, "missing authors")
	}
	if a.Year <= 0 {
		reasons = append(reasons, fmt.Sprintf("invalid year (%d)", a.Year))
	}
	doi := strings.TrimSpace(a.DOI)
	if !IsRealDOI(doi) {
		if len(doi) > 80 {
			doi = doi[:80]
		}
		reasons = append(reasons, fmt.Sprintf("invalid DOI (%s)", doi))
	}
	if a.Publisher == "" {
		reasons = append(reasons, "missing publisher")
	}
	if a.ReferenceCount == 0 {
		reasons = append(reasons, "missing references")
	}
	return reasons
}

// sortedReasons returns reasons in deterministic order.
func sortedReasons(m map[string]int) []string {
	type kv struct {
		key string
		val int
	}
	sorted := make([]kv, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].val != sorted[j].val {
			return sorted[i].val > sorted[j].val
		}
		return sorted[i].key < sorted[j].key
	})
	out := make([]string, len(sorted))
	for i, kv := range sorted {
		out[i] = kv.key
	}
	return out
}
