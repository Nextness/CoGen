// store_unit_test.go tests PDF store pure functions with no database required.
//go:build unit

package pdfstore

import (
	"strings"
	"testing"
	"time"
)

// TestNewCorrelationIDProducesUniqueNonEmptyValues verifies new correlation id produces unique non empty values.
func TestNewCorrelationIDProducesUniqueNonEmptyValues(t *testing.T) {
	id1, err := newCorrelationID()
	if err != nil {
		t.Fatalf("newCorrelationID returned error: %v", err)
	}
	if id1 == "" {
		t.Fatal("newCorrelationID returned empty string")
	}
	id2, err := newCorrelationID()
	if err != nil {
		t.Fatalf("newCorrelationID returned error: %v", err)
	}
	if id1 == id2 {
		t.Fatal("successive correlation IDs should be different")
	}
}

// TestNewCorrelationIDFormatMatchesUUID verifies new correlation id format matches uuid.
func TestNewCorrelationIDFormatMatchesUUID(t *testing.T) {
	id, err := newCorrelationID()
	if err != nil {
		t.Fatalf("newCorrelationID returned error: %v", err)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("correlation ID %q has %d segments, want 5", id, len(parts))
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Fatalf("correlation ID %q has wrong segment lengths (want 8-4-4-4-12)", id)
	}
}

// TestTimestampReturnsNonEmptyRFC3339NanoFormat verifies timestamp returns non empty rfc3339 nano format.
func TestTimestampReturnsNonEmptyRFC3339NanoFormat(t *testing.T) {
	ts := timestamp(time.Date(2025, 1, 15, 10, 30, 0, 123456789, time.UTC))
	if ts == "" {
		t.Fatal("timestamp returned empty string")
	}
	if !strings.Contains(ts, "T") {
		t.Fatalf("timestamp %q does not contain 'T' separator", ts)
	}
	if !strings.HasSuffix(ts, "Z") {
		t.Fatalf("timestamp %q does not end with 'Z'", ts)
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Fatalf("timestamp %q is not valid RFC3339Nano: %v", ts, err)
	}
	if !parsed.Equal(time.Date(2025, 1, 15, 10, 30, 0, 123456789, time.UTC)) {
		t.Fatalf("timestamp %q parsed to %v, want 2025-01-15T10:30:00.123456789Z", ts, parsed)
	}
}
