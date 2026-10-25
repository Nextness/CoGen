// store_integration_test.go tests PDF store operations against real
// temporary SQLite databases.

package pdfstore_test

import "analysis/tests/support/go/testenv"

import subject "analysis/backend/pdfstore"
import testhelpers "analysis/tests/support/go/backend/pdfstore"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"analysis/tests/support/go/testfixture"
)

// TestSaveNormalizesDOIDeduplicatesBlobsAndHandlesNoOpAndReplacement verifies
// save normalizes DOIs, shares content-addressed blobs, treats identical bytes
// as a verified no-op, and deletes outgoing bytes only after the final selection changes.
func TestSaveNormalizesDOIDeduplicatesBlobsAndHandlesNoOpAndReplacement(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	data := []byte("%PDF-1.7\nsame content")
	for index, doi := range []string{"https://doi.org/10.1000/ONE", "10.1000/two"} {
		registered, err := store.Register(ctx, doi, int64(index+1), 10)
		if err != nil || !registered {
			t.Fatalf("register %d: registered=%v err=%v", index, registered, err)
		}
		result, err := store.Save(ctx, subject.SaveInput{
			DOI: doi, WorkID: int64(index + 1), ExpectedChangeCounter: 1, Data: data,
		})
		if err != nil || !result.Changed || result.Operation != subject.OperationUploaded || result.Document.ChangeCounter != 2 {
			t.Fatalf("save %d: %+v err=%v", index, result, err)
		}
	}
	document, err := store.Document(ctx, "10.1000/one")
	if err != nil {
		t.Fatal(err)
	}
	if document == nil || document.DOI != "10.1000/one" || document.Status != subject.StatusAvailable || document.InventoriedAt == "" || document.ChangeCounter != 2 {
		t.Fatalf("unexpected document: %+v", document)
	}
	firstHash := document.ContentHash
	unchanged, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/one", WorkID: 1, ExpectedChangeCounter: 2, Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Changed || unchanged.Operation != subject.OperationUnchanged || unchanged.Document.ContentHash != firstHash || unchanged.Document.ChangeCounter != 2 || unchanged.ByteSize != int64(len(data)) || unchanged.EventKey != "" {
		t.Fatalf("identical save was not a verified no-op: %+v", unchanged)
	}
	if unchanged.Document.UpdatedAt != document.UpdatedAt {
		t.Fatalf("no-op changed updated_at from %q to %q", document.UpdatedAt, unchanged.Document.UpdatedAt)
	}
	replacement := []byte("%PDF-1.7\nreplacement content")
	replaced, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/one", WorkID: 1, ExpectedChangeCounter: 2, Data: replacement,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replaced.Changed || replaced.Operation != subject.OperationReplaced || replaced.Document.ChangeCounter != 3 || replaced.Document.ContentHash == firstHash || replaced.OutgoingDeleted || replaced.EventKey == "" {
		t.Fatalf("replacement result = %+v", replaced)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", firstHash); got != 1 {
		t.Fatalf("shared outgoing blob count = %d, want 1", got)
	}
	removed, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/two", WorkID: 2, ExpectedChangeCounter: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Changed || removed.Operation != subject.OperationRemoved || removed.Document.Status != subject.StatusNotAvailable || removed.Document.ChangeCounter != 3 || !removed.OutgoingDeleted || removed.EventKey == "" {
		t.Fatalf("removal result = %+v", removed)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", firstHash); got != 0 {
		t.Fatalf("outgoing blob count after final removal = %d, want 0", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs"); got != 1 {
		t.Fatalf("remaining blob count = %d, want 1", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_documents WHERE status = 'available'"); got != 1 {
		t.Fatalf("available document count = %d, want 1", got)
	}
	for action, want := range map[string]int{
		"pdf_inventory_registered": 2, "pdf_document_inventoried": 2,
		"pdf_document_replaced": 1, "pdf_document_removed": 1,
	} {
		if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = ?", action); got != want {
			t.Fatalf("audit action %s count = %d, want %d", action, got, want)
		}
	}
}

// TestSaveAndAuditOutboxAreTransactionalAndIdempotent verifies save and audit outbox are transactional and idempotent.
func TestSaveAndAuditOutboxAreTransactionalAndIdempotent(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	workID, err := metadata.Work.CreateWorkByDOI(ctx, "10.1000/audit")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := testfixture.Run(ctx, metadata.Conn, "pdf-inventory-test", "")
	if err != nil {
		t.Fatal(err)
	}
	registered, err := store.Register(ctx, "10.1000/audit", workID, runID)
	if err != nil || !registered {
		t.Fatalf("register audit inventory: registered=%v err=%v", registered, err)
	}
	registered, err = store.Register(ctx, "10.1000/audit", workID, runID)
	if err != nil || registered {
		t.Fatalf("duplicate registration: registered=%v err=%v", registered, err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/audit", WorkID: workID, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\naudit"),
	}); err != nil {
		t.Fatal(err)
	}
	first, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil {
		t.Fatal(err)
	}
	if first != 2 || second != 0 {
		t.Fatalf("flush counts = %d then %d, want 2 then 0", first, second)
	}
	var events, links int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action IN ('pdf_inventory_registered', 'pdf_document_inventoried')").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 2 || links != 2 {
		t.Fatalf("metadata PDF audit events=%d links=%d, want two each", events, links)
	}
	var registeredRunID int64
	if err := metadata.Conn.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&registeredRunID); err != nil {
		t.Fatal(err)
	}
	if registeredRunID != runID {
		t.Fatalf("registration audit run ID = %d, want %d", registeredRunID, runID)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/audit", WorkID: workID, ExpectedChangeCounter: 2, Data: []byte("%PDF-1.7\naudit replacement"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/audit", WorkID: workID, ExpectedChangeCounter: 3,
	}); err != nil {
		t.Fatal(err)
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil || flushed != 2 {
		t.Fatalf("replacement/removal flush=(%d,%v), want 2", flushed, err)
	}
	for action, want := range map[string]int{
		"pdf_document_replaced": 1, "pdf_document_removed": 1,
	} {
		if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action=?", action).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if events != want {
			t.Fatalf("metadata %s events = %d, want %d", action, events, want)
		}
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 4 {
		t.Fatalf("metadata PDF audit links = %d, want 4", links)
	}
	if repeated, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); err != nil || repeated != 0 {
		t.Fatalf("repeated replacement/removal flush=(%d,%v), want 0", repeated, err)
	}
}

// TestSaveRejectsCorruptIdenticalBlobAndReplacesDifferentBytes verifies a
// corrupt stored blob cannot be blessed by a no-op, while different valid bytes
// replace it without reading the corrupt outgoing content.
func TestSaveRejectsCorruptIdenticalBlobAndReplacesDifferentBytes(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/corrupt", 1, 1); err != nil {
		t.Fatal(err)
	}
	original := []byte("%PDF-1.7\noriginal")
	saved, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/corrupt", WorkID: 1, ExpectedChangeCounter: 1, Data: original,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`DROP TRIGGER pdf_blobs_abort_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE pdf_blobs SET data=? WHERE content_hash=?`, []byte("%PDF-1.7\nchanged!"), saved.Document.ContentHash); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/corrupt", WorkID: 1, ExpectedChangeCounter: 2, Data: original,
	}); !errors.Is(err, subject.ErrPDFIntegrity) {
		t.Fatalf("corrupt identical save error = %v", err)
	}
	document, err := store.Document(ctx, "10.1000/corrupt")
	if err != nil || document == nil || document.ChangeCounter != 2 || document.ContentHash != saved.Document.ContentHash {
		t.Fatalf("corrupt no-op changed document: %+v err=%v", document, err)
	}
	replaced, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/corrupt", WorkID: 1, ExpectedChangeCounter: 2, Data: []byte("%PDF-1.7\nretry"),
	})
	if err != nil || !replaced.Changed || replaced.Operation != subject.OperationReplaced || replaced.Document.ChangeCounter != 3 {
		t.Fatalf("different-byte replacement = %+v err=%v", replaced, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", saved.Document.ContentHash); got != 0 {
		t.Fatalf("corrupt outgoing blob count = %d, want 0", got)
	}
}

// TestSaveAndRemoveRequireCurrentChangeCounter verifies stale preconditions are
// rejected and repeated removal with the current counter is a no-op.
func TestSaveAndRemoveRequireCurrentChangeCounter(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/counter", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\ncounter"),
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/counter")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\nstale"),
	}); !errors.Is(err, subject.ErrPDFStateConflict) {
		t.Fatalf("stale save error = %v", err)
	}
	after, err := store.Document(ctx, "10.1000/counter")
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentHash != before.ContentHash || after.ChangeCounter != before.ChangeCounter || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("stale save changed state from %+v to %+v", before, after)
	}
	var conflict *subject.StateConflictError
	if _, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 1,
	}); !errors.As(err, &conflict) || conflict.State.ChangeCounter != 2 {
		t.Fatalf("stale remove error = %v", err)
	}
	after, err = store.Document(ctx, "10.1000/counter")
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != subject.StatusAvailable || after.ContentHash != before.ContentHash || after.ChangeCounter != before.ChangeCounter {
		t.Fatalf("stale remove changed state from %+v to %+v", before, after)
	}
	removed, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 2,
	})
	if err != nil || !removed.Changed || removed.Document.ChangeCounter != 3 {
		t.Fatalf("removal = %+v err=%v", removed, err)
	}
	unchanged, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 3,
	})
	if err != nil || unchanged.Changed || unchanged.Operation != subject.OperationUnchanged || unchanged.Document.ChangeCounter != 3 || unchanged.Document.UpdatedAt != removed.Document.UpdatedAt {
		t.Fatalf("repeated removal = %+v err=%v", unchanged, err)
	}
	if _, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 2,
	}); !errors.Is(err, subject.ErrPDFStateConflict) {
		t.Fatalf("stale repeated removal error = %v", err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_removed'"); got != 1 {
		t.Fatalf("removal audit count = %d, want 1", got)
	}
	restored, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/counter", WorkID: 1, ExpectedChangeCounter: 3, Data: []byte("%PDF-1.7\nafter removal"),
	})
	if err != nil || !restored.Changed || restored.Operation != subject.OperationUploaded || restored.Document.ChangeCounter != 4 || restored.Document.Status != subject.StatusAvailable {
		t.Fatalf("save after removal = %+v err=%v", restored, err)
	}
}

// TestRemoveRetainsSharedBlobUntilFinalSelection verifies removal deletes bytes
// only after no current document selects the same content.
func TestRemoveRetainsSharedBlobUntilFinalSelection(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	data := []byte("%PDF-1.7\nshared removal")
	for index, doi := range []string{"10.1000/shared-one", "10.1000/shared-two"} {
		if _, err := store.Register(ctx, doi, int64(index+1), 1); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Save(ctx, subject.SaveInput{
			DOI: doi, WorkID: int64(index + 1), ExpectedChangeCounter: 1, Data: data,
		}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/shared-one", WorkID: 1, ExpectedChangeCounter: 2,
	})
	if err != nil || !first.Changed || first.OutgoingDeleted {
		t.Fatalf("first shared removal = %+v err=%v", first, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs"); got != 1 {
		t.Fatalf("shared blob count after first removal = %d, want 1", got)
	}
	second, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/shared-two", WorkID: 2, ExpectedChangeCounter: 2,
	})
	if err != nil || !second.Changed || !second.OutgoingDeleted {
		t.Fatalf("second shared removal = %+v err=%v", second, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs"); got != 0 {
		t.Fatalf("shared blob count after final removal = %d, want 0", got)
	}
}

// TestSaveConcurrentCounterAllowsOneWinner verifies concurrent saves using the
// same expected counter produce exactly one committed change.
func TestSaveConcurrentCounterAllowsOneWinner(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/race-save", 1, 1); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result subject.SaveResult
		err    error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})
	for _, data := range [][]byte{[]byte("%PDF-1.7\nrace one"), []byte("%PDF-1.7\nrace two")} {
		go func(data []byte) {
			<-start
			result, err := store.Save(ctx, subject.SaveInput{
				DOI: "10.1000/race-save", WorkID: 1, ExpectedChangeCounter: 1, Data: data,
			})
			results <- outcome{result: result, err: err}
		}(data)
	}
	close(start)
	changed, conflicts := 0, 0
	for index := 0; index < 2; index++ {
		outcome := <-results
		switch {
		case outcome.err == nil && outcome.result.Changed:
			changed++
		case errors.Is(outcome.err, subject.ErrPDFStateConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent save outcome: %+v err=%v", outcome.result, outcome.err)
		}
	}
	if changed != 1 || conflicts != 1 {
		t.Fatalf("concurrent save changed=%d conflicts=%d, want one each", changed, conflicts)
	}
	document, err := store.Document(ctx, "10.1000/race-save")
	if err != nil || document == nil || document.ChangeCounter != 2 {
		t.Fatalf("concurrent save document = %+v err=%v", document, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_inventoried'"); got != 1 {
		t.Fatalf("concurrent save audit count = %d, want 1", got)
	}
}

// TestRemoveConcurrentCounterAllowsOneWinner verifies concurrent removals using
// the same expected counter produce exactly one committed change.
func TestRemoveConcurrentCounterAllowsOneWinner(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/race-remove", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/race-remove", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\nrace remove"),
	}); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result subject.RemoveResult
		err    error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			result, err := store.Remove(ctx, subject.RemoveInput{
				DOI: "10.1000/race-remove", WorkID: 1, ExpectedChangeCounter: 2,
			})
			results <- outcome{result: result, err: err}
		}()
	}
	close(start)
	changed, conflicts := 0, 0
	for index := 0; index < 2; index++ {
		outcome := <-results
		switch {
		case outcome.err == nil && outcome.result.Changed:
			changed++
		case errors.Is(outcome.err, subject.ErrPDFStateConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent removal outcome: %+v err=%v", outcome.result, outcome.err)
		}
	}
	if changed != 1 || conflicts != 1 {
		t.Fatalf("concurrent removal changed=%d conflicts=%d, want one each", changed, conflicts)
	}
	document, err := store.Document(ctx, "10.1000/race-remove")
	if err != nil || document == nil || document.ChangeCounter != 3 || document.Status != subject.StatusNotAvailable {
		t.Fatalf("concurrent removal document = %+v err=%v", document, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_removed'"); got != 1 {
		t.Fatalf("concurrent removal audit count = %d, want 1", got)
	}
}

// TestRemoveDoesNotReadCorruptOutgoingBlob verifies removal remains possible
// when the current blob is corrupt.
func TestRemoveDoesNotReadCorruptOutgoingBlob(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/corrupt-remove", 1, 1); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/corrupt-remove", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\noriginal"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`DROP TRIGGER pdf_blobs_abort_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE pdf_blobs SET data=? WHERE content_hash=?`, []byte("%PDF-1.7\nchanged!"), saved.Document.ContentHash); err != nil {
		t.Fatal(err)
	}
	removed, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/corrupt-remove", WorkID: 1, ExpectedChangeCounter: 2,
	})
	if err != nil || !removed.Changed || removed.Document.Status != subject.StatusNotAvailable || removed.Document.ChangeCounter != 3 {
		t.Fatalf("corrupt removal = %+v err=%v", removed, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", saved.Document.ContentHash); got != 0 {
		t.Fatalf("corrupt outgoing blob count = %d, want 0", got)
	}
}

// TestRegisterDoesNotRestoreRemovedSelection verifies pipeline registration
// remains idempotent after a manual removal and cannot resurrect old bytes.
func TestRegisterDoesNotRestoreRemovedSelection(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if registered, err := store.Register(ctx, "10.1000/reregister", 1, 1); err != nil || !registered {
		t.Fatalf("first registration = %v err=%v", registered, err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/reregister", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\nreregister"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/reregister", WorkID: 1, ExpectedChangeCounter: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if registered, err := store.Register(ctx, "10.1000/reregister", 1, 2); err != nil || registered {
		t.Fatalf("re-registration = %v err=%v, want idempotent false", registered, err)
	}
	document, err := store.Document(ctx, "10.1000/reregister")
	if err != nil || document == nil || document.Status != subject.StatusNotAvailable || document.ContentHash != "" || document.ChangeCounter != 3 {
		t.Fatalf("re-registered document = %+v err=%v", document, err)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs"); got != 0 {
		t.Fatalf("blob count after re-registration = %d, want 0", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_inventory_registered'"); got != 1 {
		t.Fatalf("registration audit count = %d, want 1", got)
	}
}

// TestFlushAuditOutboxDropsStalePipelineRun verifies a flush preserves an
// outbox event whose pipeline run no longer exists in the bound metadata
// database, matching the durable PDF store across metadata iterations.
func TestFlushAuditOutboxDropsStalePipelineRun(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	workID, err := metadata.Work.CreateWorkByDOI(ctx, "10.1000/stale")
	if err != nil {
		t.Fatal(err)
	}
	// Register with a run ID that does not exist in this metadata database,
	// simulating an outbox event carried over from an older metadata iteration.
	registered, err := store.Register(ctx, "10.1000/stale", workID, 999)
	if err != nil || !registered {
		t.Fatalf("register stale inventory: registered=%v err=%v", registered, err)
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil {
		t.Fatal(err)
	}
	if flushed != 1 {
		t.Fatalf("flush count = %d, want 1", flushed)
	}
	var events, links int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 1 {
		t.Fatalf("metadata PDF audit events=%d links=%d, want one each", events, links)
	}
	var runID any
	if err := metadata.Conn.QueryRow("SELECT pipeline_run_id FROM audit_events WHERE action='pdf_inventory_registered'").Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if runID != nil {
		t.Fatalf("stale registration audit run ID = %v, want NULL", runID)
	}
}

// TestFlushAuditOutboxDrainsBoundedBatches verifies a full flush processes ordered batches without retaining the backlog.
func TestFlushAuditOutboxDrainsBoundedBatches(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	for index := 0; index < 100+2; index++ {
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
			(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (?, ?, 'test', 'work', ?, 'pdf_inventory_registered', '{}', ?)`,
			fmt.Sprintf("batch-%03d", index), fmt.Sprintf("2026-01-01T00:00:%02dZ", index%60), index, fmt.Sprintf("correlation-%03d", index)); err != nil {
			t.Fatal(err)
		}
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil {
		t.Fatal(err)
	}
	if flushed != 100+2 {
		t.Fatalf("flushed=%d, want %d", flushed, 100+2)
	}
	var remaining, events int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE actor='test'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 || events != 100+2 {
		t.Fatalf("batch drain remaining=%d events=%d", remaining, events)
	}
}

// TestFlushAuditOutboxContinuesAfterOneBadEvent verifies a failed event does not block later evidence.
func TestFlushAuditOutboxContinuesAfterOneBadEvent(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if _, err := metadata.Conn.Exec(`CREATE TRIGGER reject_bad_pdf_audit BEFORE INSERT ON audit_events
		WHEN NEW.actor='bad' BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ key, actor string }{{"bad-event", "bad"}, {"good-event", "good"}} {
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
			(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (?, '2026-01-01T00:00:00Z', ?, 'work', '1', 'pdf_inventory_registered', '{}', ?)`, event.key, event.actor, event.key); err != nil {
			t.Fatal(err)
		}
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if flushed != 1 || err == nil || !strings.Contains(err.Error(), "injected audit failure") {
		t.Fatalf("failure-isolating flush=%d err=%v", flushed, err)
	}
	var goodEvents, remaining int
	if err := metadata.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE actor='good'`).Scan(&goodEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if goodEvents != 1 || remaining != 1 {
		t.Fatalf("good events=%d remaining=%d", goodEvents, remaining)
	}
}

// TestFlushAuditOutboxRollsBackMetadataAuditWhenLinkFails verifies the metadata
// audit row and its delivery link commit or roll back together and remain
// retryable after a failed delivery.
func TestFlushAuditOutboxRollsBackMetadataAuditWhenLinkFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
		(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES ('link-failure', '2026-01-01T00:00:00Z', 'pipeline', 'work', '1', 'pdf_inventory_registered', '{}', 'link-failure')`); err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.Conn.Exec(`CREATE TRIGGER reject_pdf_audit_link BEFORE INSERT ON pdf_audit_links
		BEGIN SELECT RAISE(ABORT, 'injected link failure'); END`); err != nil {
		t.Fatal(err)
	}

	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if flushed != 0 || err == nil || !strings.Contains(err.Error(), "injected link failure") {
		t.Fatalf("link-failure flush=%d err=%v", flushed, err)
	}
	var events, links, remaining int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if events != 0 || links != 0 || remaining != 1 {
		t.Fatalf("failed delivery events=%d links=%d remaining=%d, want 0, 0, 1", events, links, remaining)
	}

	if _, err := metadata.Conn.Exec("DROP TRIGGER reject_pdf_audit_link"); err != nil {
		t.Fatal(err)
	}
	flushed, err = store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil || flushed != 1 {
		t.Fatalf("retry flush=%d err=%v, want one delivered event", flushed, err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 1 || remaining != 0 {
		t.Fatalf("retried delivery events=%d links=%d remaining=%d, want 1, 1, 0", events, links, remaining)
	}
}

// TestFlushAuditOutboxRetryAfterMetadataCommitIsIdempotent verifies a crash
// between the metadata commit and the companion marking leaves the committed
// audit row reusable, so the retry marks the event delivered without
// duplicating append-only evidence.
func TestFlushAuditOutboxRetryAfterMetadataCommitIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
		(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
		VALUES ('marking-failure', '2026-01-01T00:00:00Z', 'pipeline', 'work', '1', 'pdf_inventory_registered', '{}', 'marking-failure')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_outbox_marking BEFORE UPDATE ON pdf_audit_outbox
		WHEN NEW.delivered_at IS NOT NULL BEGIN SELECT RAISE(ABORT, 'injected marking failure'); END`); err != nil {
		t.Fatal(err)
	}

	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if flushed != 0 || err == nil || !strings.Contains(err.Error(), "injected marking failure") {
		t.Fatalf("marking-failure flush=%d err=%v", flushed, err)
	}
	var events, links, remaining int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 1 || remaining != 1 {
		t.Fatalf("committed-but-unmarked events=%d links=%d remaining=%d, want 1, 1, 1", events, links, remaining)
	}

	if _, err := store.DB.Exec("DROP TRIGGER reject_pdf_outbox_marking"); err != nil {
		t.Fatal(err)
	}
	flushed, err = store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if err != nil || flushed != 1 {
		t.Fatalf("retry flush=%d err=%v, want one delivered event", flushed, err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 1 || remaining != 0 {
		t.Fatalf("retried delivery events=%d links=%d remaining=%d, want 1, 1, 0", events, links, remaining)
	}
}

// TestFlushAuditOutboxIsolatesFailuresBetweenSuccessfulEvents verifies a failed
// event rolls back only its own savepoint and leaves earlier and later events
// delivered in the same batch.
func TestFlushAuditOutboxIsolatesFailuresBetweenSuccessfulEvents(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if _, err := metadata.Conn.Exec(`CREATE TRIGGER reject_bad_pdf_audit BEFORE INSERT ON audit_events
		WHEN NEW.actor='bad' BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ key, actor string }{
		{"event-a", "good"}, {"event-b", "bad"}, {"event-c", "good"},
	} {
		if _, err := store.DB.Exec(`INSERT INTO pdf_audit_outbox
			(event_key, occurred_at, actor, entity_type, entity_id, action, metadata_json, correlation_id)
			VALUES (?, '2026-01-01T00:00:00Z', ?, 'work', '1', 'pdf_inventory_registered', '{}', ?)`, event.key, event.actor, event.key); err != nil {
			t.Fatal(err)
		}
	}
	flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings)
	if flushed != 2 || err == nil || !strings.Contains(err.Error(), "injected audit failure") {
		t.Fatalf("failure-isolating flush=%d err=%v", flushed, err)
	}
	var goodEvents, remaining int
	if err := metadata.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE actor='good'`).Scan(&goodEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_audit_outbox WHERE delivered_at IS NULL`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if goodEvents != 2 || remaining != 1 {
		t.Fatalf("good events=%d remaining=%d, want 2 and 1", goodEvents, remaining)
	}
}

// TestSaveRollsBackWhenAuditOutboxWriteFails verifies save rolls back when the audit outbox write fails.
func TestSaveRollsBackWhenAuditOutboxWriteFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/rollback", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_outbox
		BEFORE INSERT ON pdf_audit_outbox BEGIN
			SELECT RAISE(ABORT, 'injected outbox failure');
		END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/rollback", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\nrollback"),
	}); err == nil || !strings.Contains(err.Error(), "injected outbox failure") {
		t.Fatalf("outbox failure error = %v", err)
	}
	var blobs, documents int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_blobs").Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM pdf_documents").Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if blobs != 0 || documents != 1 {
		t.Fatalf("failed save left blobs=%d documents=%d", blobs, documents)
	}
	document, err := store.Document(ctx, "10.1000/rollback")
	if err != nil || document == nil || document.Status != subject.StatusNotAvailable || document.ChangeCounter != 1 {
		t.Fatalf("failed save inventory document=%+v err=%v", document, err)
	}
}

// TestReplaceRollsBackWhenBlobInsertFails verifies a failed incoming blob
// insert leaves the old selection, bytes, timestamps, counter, and outbox unchanged.
func TestReplaceRollsBackWhenBlobInsertFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/blob-failure", 1, 1); err != nil {
		t.Fatal(err)
	}
	original := []byte("%PDF-1.7\noriginal")
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/blob-failure", WorkID: 1, ExpectedChangeCounter: 1, Data: original,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/blob-failure")
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte("%PDF-1.7\nreplacement")
	replacementHash, err := subject.ValidatePDF(replacement, subject.DefaultMaxPDFBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(fmt.Sprintf(`CREATE TRIGGER reject_pdf_blob_insert
		BEFORE INSERT ON pdf_blobs
		WHEN NEW.content_hash = '%s'
		BEGIN SELECT RAISE(ABORT, 'injected blob failure'); END`, replacementHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/blob-failure", WorkID: 1, ExpectedChangeCounter: 2, Data: replacement,
	}); err == nil || !strings.Contains(err.Error(), "injected blob failure") {
		t.Fatalf("blob failure error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/blob-failure", before)
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", replacementHash); got != 0 {
		t.Fatalf("speculative blob count = %d, want 0", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_replaced'"); got != 0 {
		t.Fatalf("replacement audit count = %d, want 0", got)
	}
}

// TestReplaceRollsBackWhenPointerUpdateFails verifies a failed selection update
// removes the speculative incoming blob and preserves the old selection.
func TestReplaceRollsBackWhenPointerUpdateFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/update-failure", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/update-failure", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\noriginal"),
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/update-failure")
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte("%PDF-1.7\nreplacement")
	replacementHash, err := subject.ValidatePDF(replacement, subject.DefaultMaxPDFBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_document_update
		BEFORE UPDATE ON pdf_documents
		BEGIN SELECT RAISE(ABORT, 'injected update failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/update-failure", WorkID: 1, ExpectedChangeCounter: 2, Data: replacement,
	}); err == nil || !strings.Contains(err.Error(), "injected update failure") {
		t.Fatalf("update failure error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/update-failure", before)
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", replacementHash); got != 0 {
		t.Fatalf("speculative blob count = %d, want 0", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_replaced'"); got != 0 {
		t.Fatalf("replacement audit count = %d, want 0", got)
	}
}

// TestReplaceRollsBackWhenOutgoingDeletionFails verifies a failed outgoing blob
// deletion rolls back the selection update and speculative incoming blob.
func TestReplaceRollsBackWhenOutgoingDeletionFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/delete-failure", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/delete-failure", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\noriginal"),
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/delete-failure")
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte("%PDF-1.7\nreplacement")
	replacementHash, err := subject.ValidatePDF(replacement, subject.DefaultMaxPDFBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_blob_delete
		BEFORE DELETE ON pdf_blobs
		BEGIN SELECT RAISE(ABORT, 'injected delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/delete-failure", WorkID: 1, ExpectedChangeCounter: 2, Data: replacement,
	}); err == nil || !strings.Contains(err.Error(), "injected delete failure") {
		t.Fatalf("delete failure error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/delete-failure", before)
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", replacementHash); got != 0 {
		t.Fatalf("speculative blob count = %d, want 0", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", before.ContentHash); got != 1 {
		t.Fatalf("outgoing blob count = %d, want 1", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_replaced'"); got != 0 {
		t.Fatalf("replacement audit count = %d, want 0", got)
	}
}

// TestRemoveRollsBackWhenOutgoingDeletionFails verifies a failed removal
// deletion preserves the current selection, counter, and outbox.
func TestRemoveRollsBackWhenOutgoingDeletionFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/remove-delete-failure", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/remove-delete-failure", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\noriginal"),
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/remove-delete-failure")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_blob_delete
		BEFORE DELETE ON pdf_blobs
		BEGIN SELECT RAISE(ABORT, 'injected delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Remove(ctx, subject.RemoveInput{
		DOI: "10.1000/remove-delete-failure", WorkID: 1, ExpectedChangeCounter: 2,
	}); err == nil || !strings.Contains(err.Error(), "injected delete failure") {
		t.Fatalf("removal delete failure error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/remove-delete-failure", before)
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", before.ContentHash); got != 1 {
		t.Fatalf("outgoing blob count = %d, want 1", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_removed'"); got != 0 {
		t.Fatalf("removal audit count = %d, want 0", got)
	}
}

// TestReplaceRollsBackWhenAuditOutboxWriteFails verifies a failed replacement
// preserves the previous selection, bytes, timestamp, and counter.
func TestReplaceRollsBackWhenAuditOutboxWriteFails(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/replace-rollback", 1, 1); err != nil {
		t.Fatal(err)
	}
	original := []byte("%PDF-1.7\noriginal")
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/replace-rollback", WorkID: 1, ExpectedChangeCounter: 1, Data: original,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/replace-rollback")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_pdf_outbox
		BEFORE INSERT ON pdf_audit_outbox BEGIN
			SELECT RAISE(ABORT, 'injected outbox failure');
		END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/replace-rollback", WorkID: 1, ExpectedChangeCounter: 2, Data: []byte("%PDF-1.7\nreplacement"),
	}); err == nil || !strings.Contains(err.Error(), "injected outbox failure") {
		t.Fatalf("replacement outbox failure error = %v", err)
	}
	after, err := store.Document(ctx, "10.1000/replace-rollback")
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentHash != before.ContentHash || after.ChangeCounter != before.ChangeCounter || after.UpdatedAt != before.UpdatedAt || after.InventoriedAt != before.InventoriedAt {
		t.Fatalf("failed replacement changed state from %+v to %+v", before, after)
	}
	stored, err := store.BlobData(ctx, before.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, original) {
		t.Fatalf("failed replacement changed stored bytes: %q", stored)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs"); got != 1 {
		t.Fatalf("failed replacement left %d blobs, want 1", got)
	}
}

// TestStoreIntegrationInvalidInputs verifies typed save and removal validation errors.
func TestStoreIntegrationInvalidInputs(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	valid := []byte("%PDF-1.7\nvalid")
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "", WorkID: 1, ExpectedChangeCounter: 1, Data: valid}); !errors.Is(err, subject.ErrPDFInvalid) {
		t.Fatalf("empty DOI save error = %v", err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/input", WorkID: 1, ExpectedChangeCounter: 0, Data: valid}); !errors.Is(err, subject.ErrPDFInvalid) {
		t.Fatalf("nonpositive counter save error = %v", err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/input", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("not a PDF")}); !errors.Is(err, subject.ErrPDFInvalid) {
		t.Fatalf("invalid PDF save error = %v", err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/not-normalized", WorkID: 1, ExpectedChangeCounter: 1, Data: valid}); !errors.Is(err, subject.ErrPDFNotRegistered) {
		t.Fatalf("unregistered save error = %v", err)
	}
	if _, err := store.Remove(ctx, subject.RemoveInput{DOI: "", WorkID: 1, ExpectedChangeCounter: 1}); !errors.Is(err, subject.ErrPDFInvalid) {
		t.Fatalf("empty DOI remove error = %v", err)
	}
	if _, err := store.Remove(ctx, subject.RemoveInput{DOI: "10.1000/not-normalized", WorkID: 1, ExpectedChangeCounter: 1}); !errors.Is(err, subject.ErrPDFNotRegistered) {
		t.Fatalf("unregistered remove error = %v", err)
	}
}

// assertDocumentUnchanged verifies one document still matches the supplied state.
func assertDocumentUnchanged(t *testing.T, store *subject.Store, doi string, want *subject.Document) {
	t.Helper()
	got, err := store.Document(context.Background(), doi)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Status != want.Status || got.ContentHash != want.ContentHash || got.InventoriedAt != want.InventoriedAt || got.UpdatedAt != want.UpdatedAt || got.ChangeCounter != want.ChangeCounter {
		t.Fatalf("document changed from %+v to %+v", want, got)
	}
}

// queryCount runs one bounded count query against the companion store.
func queryCount(t *testing.T, store *subject.Store, query string, args ...any) int {
	t.Helper()
	var count int
	if err := store.DB.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
