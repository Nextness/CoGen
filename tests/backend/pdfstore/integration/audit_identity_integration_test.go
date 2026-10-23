// audit_identity_integration_test.go verifies persisted audit identities and timestamps through registration.
package pdfstore_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	testhelpers "analysis/tests/support/go/backend/pdfstore"
)

// TestRegistrationCreatesUniqueAuditIdentities verifies public writes persist unique UUID-shaped correlations and event keys.
func TestRegistrationCreatesUniqueAuditIdentities(t *testing.T) {
	store := testhelpers.OpenTestStore(t)
	before := time.Now().UTC()
	for i := 0; i < 2; i++ {
		if _, err := store.Register(t.Context(), fmt.Sprintf("10.1000/identity-%d", i), int64(i+1), 1); err != nil {
			t.Fatal(err)
		}
	}
	after := time.Now().UTC()
	rows, err := store.DB.Query(`SELECT event_key, correlation_id, occurred_at FROM pdf_audit_outbox ORDER BY event_key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	keys, correlations := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var key, correlation, timestamp string
		if err := rows.Scan(&key, &correlation, &timestamp); err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(key) || !pattern.MatchString(correlation) || keys[key] || correlations[correlation] {
			t.Fatalf("invalid or duplicate identities: %q %q", key, correlation)
		}
		keys[key] = true
		correlations[correlation] = true
		parsed, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil || !strings.HasSuffix(timestamp, "Z") || parsed.Before(before) || parsed.After(after) || parsed.Format(time.RFC3339Nano) != timestamp {
			t.Fatalf("timestamp=%q parsed=%v error=%v", timestamp, parsed, err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || len(correlations) != 2 {
		t.Fatalf("keys=%v correlations=%v", keys, correlations)
	}
}
