// characterization_integration_test.go verifies nullable companion query
// projections and deterministic keyset outbox selection.

package pdfstore_test

import subject "analysis/backend/pdfstore"
import testhelpers "analysis/tests/support/go/backend/pdfstore"

import (
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

// TestCharacterizationPDFStoreTypedQueries verifies the typed inventory lookup
// preserves nullable fields and the outbox selection preserves keyset ordering
// across both batch variants.
func TestCharacterizationPDFStoreTypedQueries(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)

	if _, err := store.Register(ctx, "10.1000/pending", 1, 10); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Document(ctx, "10.1000/pending")
	if err != nil || pending == nil {
		t.Fatalf("pending document = %+v err=%v", pending, err)
	}
	if pending.Status != subject.StatusNotAvailable || pending.ContentHash != "" || pending.InventoriedAt != "" || pending.UpdatedAt == "" || pending.ChangeCounter != 1 {
		t.Fatalf("pending document projection = %+v", pending)
	}

	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/pending", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\ncharacterization")}); err != nil {
		t.Fatal(err)
	}
	available, err := store.Document(ctx, "10.1000/pending")
	if err != nil || available == nil {
		t.Fatalf("available document = %+v err=%v", available, err)
	}
	if available.Status != subject.StatusAvailable || available.ContentHash == "" || available.InventoriedAt == "" || available.ChangeCounter != 2 {
		t.Fatalf("available document projection = %+v", available)
	}

	// Two events share an occurred_at value so the event_key tie-breaker and
	// the keyset cursor are both exercised. The registration and add events
	// are marked delivered first so only the fixtures remain pending.
	if _, err := store.DB.Exec(`UPDATE pdf_audit_outbox SET delivered_at='2026-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ key, occurredAt string }{
		{"event-b", "2026-01-01T00:00:00Z"},
		{"event-a", "2026-01-01T00:00:00Z"},
		{"event-c", "2026-01-01T00:00:01Z"},
	} {
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
			(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (?, ?, 'test', 'work', '1', 'pdf_inventory_registered', '{}', ?)`,
			event.key, event.occurredAt, event.key); err != nil {
			t.Fatal(err)
		}
	}

	// Fill the first batch with equal timestamps; event-a and event-b must be
	// reached by the key tie-breaker on the subsequent public flush batch.
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("event-%03d", i)
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
   (event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
   VALUES (?, '2026-01-01T00:00:00Z', 'test', 'work', '1', 'pdf_inventory_registered', '{}', ?)`, key, key); err != nil {
			t.Fatal(err)
		}
	}
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	delivered, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil || delivered != 103 {
		t.Fatalf("flush=(%d,%v), want 103 deliveries", delivered, err)
	}
	rows, err := metadata.Conn.Query(`SELECT correlation_id FROM audit_events ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actual []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		actual = append(actual, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(actual) != 103 {
		t.Fatalf("delivered keys=%v", actual)
	}
	for i, key := range actual {
		want := fmt.Sprintf("event-%03d", i)
		if i >= 100 {
			want = []string{"event-a", "event-b", "event-c"}[i-100]
		}
		if key != want {
			t.Fatalf("delivery %d=%q, want %q", i, key, want)
		}
	}
	if repeated, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); err != nil || repeated != 0 {
		t.Fatalf("repeat flush=(%d,%v)", repeated, err)
	}
}
