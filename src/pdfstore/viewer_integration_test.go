// viewer_integration_test.go exercises the companion store's viewer-facing
// available-inventory and blob reads against a database created by the
// production migration runner.
//go:build integration

package pdfstore

import (
	"bytes"
	"context"
	"testing"
)

// TestStoreViewerReadsReturnAvailableInventoryAndBlob verifies the viewer's
// available-DOI, document metadata, and blob projections.
func TestStoreViewerReadsReturnAvailableInventoryAndBlob(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	first := []byte("%PDF-1.7\nfirst content")
	second := []byte("%PDF-1.7\nsecond content")
	for index, item := range []struct {
		doi  string
		data []byte
	}{
		{"10.1000/zeta", first},
		{"10.1000/alpha", second},
	} {
		if _, err := store.Register(ctx, item.doi, int64(index+1), 10); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Add(ctx, item.doi, int64(index+1), item.data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Register(ctx, "10.1000/pending", 3, 10); err != nil {
		t.Fatal(err)
	}

	dois, err := store.AvailablePDFDOIs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dois) != 2 || dois[0] != "10.1000/alpha" || dois[1] != "10.1000/zeta" {
		t.Fatalf("available DOIs = %v", dois)
	}

	document, err := store.AvailableDocument(ctx, "https://doi.org/10.1000/ALPHA")
	if err != nil || document == nil {
		t.Fatalf("available document = %+v err=%v", document, err)
	}
	if document.ByteSize != int64(len(second)) || document.ContentHash == "" || document.InventoriedAt == "" {
		t.Fatalf("available document = %+v", document)
	}

	data, err := store.BlobData(ctx, document.ContentHash)
	if err != nil || !bytes.Equal(data, second) {
		t.Fatalf("blob data = %q err=%v, want %q", data, err, second)
	}

	if document, err := store.AvailableDocument(ctx, "10.1000/pending"); err != nil || document != nil {
		t.Fatalf("pending document = %+v err=%v, want nil", document, err)
	}
	if document, err := store.AvailableDocument(ctx, "10.1000/missing"); err != nil || document != nil {
		t.Fatalf("missing document = %+v err=%v, want nil", document, err)
	}
	if data, err := store.BlobData(ctx, "missing"); err != nil || data != nil {
		t.Fatalf("missing blob = %q err=%v, want nil", data, err)
	}
}

// TestStoreInventoryForDOIsFiltersAvailableContent verifies the bounded
// evaluation inventory projection returns only requested available DOIs in DOI
// order and skips empty requests.
func TestStoreInventoryForDOIsFiltersAvailableContent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	for index, doi := range []string{"10.1000/zeta", "10.1000/alpha", "10.1000/beta"} {
		if _, err := store.Register(ctx, doi, int64(index+1), 10); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Add(ctx, doi, int64(index+1), []byte("%PDF-1.7\ncontent")); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := store.InventoryForDOIs(ctx, []string{"10.1000/zeta", "10.1000/missing", "10.1000/alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].DOI != "10.1000/alpha" || entries[1].DOI != "10.1000/zeta" {
		t.Fatalf("inventory entries = %+v, want available DOIs in order", entries)
	}
	if entries[0].InventoriedAt == nil || *entries[0].InventoriedAt == "" {
		t.Fatalf("inventory timestamp = %+v, want recorded value", entries[0])
	}

	empty, err := store.InventoryForDOIs(ctx, nil)
	if err != nil || empty != nil {
		t.Fatalf("empty inventory = %+v err=%v, want nil without error", empty, err)
	}
}
