// characterization_integration_test.go pins the typed companion query
// projections and the keyset outbox selection after the sqlc migration.
//go:build integration

package pdfstore

import (
	"context"
	"testing"
)

// TestCharacterizationPDFStoreTypedQueries verifies the typed inventory lookup
// preserves nullable fields and the outbox selection preserves keyset ordering
// across both batch variants.
func TestCharacterizationPDFStoreTypedQueries(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	if _, err := store.Register(ctx, "10.1000/pending", 1, 10); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Document(ctx, "10.1000/pending")
	if err != nil || pending == nil {
		t.Fatalf("pending document = %+v err=%v", pending, err)
	}
	if pending.Status != StatusNotAvailable || pending.ContentHash != "" || pending.InventoriedAt != "" || pending.UpdatedAt == "" {
		t.Fatalf("pending document projection = %+v", pending)
	}

	if _, err := store.Add(ctx, "10.1000/pending", 1, []byte("%PDF-1.7\ncharacterization")); err != nil {
		t.Fatal(err)
	}
	available, err := store.Document(ctx, "10.1000/pending")
	if err != nil || available == nil {
		t.Fatalf("available document = %+v err=%v", available, err)
	}
	if available.Status != StatusAvailable || available.ContentHash == "" || available.InventoriedAt == "" {
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

	first, err := store.pendingAuditEvents(ctx, 2, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].EventKey != "event-a" || first[1].EventKey != "event-b" {
		t.Fatalf("first pending batch = %+v", first)
	}
	second, err := store.pendingAuditEvents(ctx, 2, first[1].OccurredAt, first[1].EventKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].EventKey != "event-c" {
		t.Fatalf("cursor pending batch = %+v", second)
	}
}
