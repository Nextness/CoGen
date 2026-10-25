// failure_integration_test.go proves PDF storage and audit delivery leave the
// previous selection intact across injected pre-commit and post-commit failures.

package pdfstore_test

import (
	"analysis/backend/database"
	subject "analysis/backend/pdfstore"
	"analysis/internal/migrations"
	"analysis/tests/support/go/backend/pdfstore"
	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// openDeferredTestStore opens a migrated companion store on one connection so
// deferred foreign-key failures surface at commit time.
func openDeferredTestStore(t *testing.T) *subject.Store {
	t.Helper()
	registry := testenv.Path("config", "database.something")
	path := filepath.Join(t.TempDir(), "corpus.pdf.db")
	conn, err := database.OpenWritable(path)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	config, err := database.ResolveMigrationConfig(registry, database.StoreCorpusPDF)
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if _, err := migrations.ApplyContext(context.Background(), conn, config.MigrationsDir, config.ConfigPath); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	store := subject.New(conn)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestReplaceCommitFailurePreservesPreviousState verifies a deferred foreign-key
// failure at commit rolls back the pointer move and speculative incoming blob.
func TestReplaceCommitFailurePreservesPreviousState(t *testing.T) {
	ctx := context.Background()
	store := openDeferredTestStore(t)
	if _, err := store.Register(ctx, "10.1000/commit-failure", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/commit-failure", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\noriginal"),
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/commit-failure")
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte("%PDF-1.7\nreplacement")
	replacementHash, err := subject.ValidatePDF(replacement, subject.DefaultMaxPDFBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER inject_deferred_failure
		AFTER UPDATE ON pdf_documents
		BEGIN
			INSERT INTO pdf_documents (doi, status, content_hash, inventoried_at, updated_at)
			VALUES ('10.1000/trigger-invalid', 'available', 'missing-hash', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
		END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec("PRAGMA defer_foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/commit-failure", WorkID: 1, ExpectedChangeCounter: 2, Data: replacement,
	}); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("commit failure error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/commit-failure", before)
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", replacementHash); got != 0 {
		t.Fatalf("speculative blob count = %d, want 0", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_blobs WHERE content_hash = ?", before.ContentHash); got != 1 {
		t.Fatalf("previous blob count = %d, want 1", got)
	}
	if got := queryCount(t, store, "SELECT COUNT(*) FROM pdf_audit_outbox WHERE action = 'pdf_document_replaced'"); got != 0 {
		t.Fatalf("replacement audit count = %d, want 0", got)
	}
}

// TestStaleCounterRejectsABASelection verifies a stale counter cannot win after
// the selection returns to the same content hash it previously held.
func TestStaleCounterRejectsABASelection(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	if _, err := store.Register(ctx, "10.1000/aba", 1, 1); err != nil {
		t.Fatal(err)
	}
	dataA := []byte("%PDF-1.7\naba-a")
	dataB := []byte("%PDF-1.7\naba-b")
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/aba", WorkID: 1, ExpectedChangeCounter: 1, Data: dataA}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/aba", WorkID: 1, ExpectedChangeCounter: 2, Data: dataB}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/aba", WorkID: 1, ExpectedChangeCounter: 3, Data: dataA}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Document(ctx, "10.1000/aba")
	if err != nil {
		t.Fatal(err)
	}
	if before.ChangeCounter != 4 {
		t.Fatalf("ABA counter = %d, want 4", before.ChangeCounter)
	}
	if _, err := store.Save(ctx, subject.SaveInput{DOI: "10.1000/aba", WorkID: 1, ExpectedChangeCounter: 2, Data: []byte("%PDF-1.7\nstale")}); !errors.Is(err, subject.ErrPDFStateConflict) {
		t.Fatalf("stale ABA save error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/aba", before)
	if _, err := store.Remove(ctx, subject.RemoveInput{DOI: "10.1000/aba", WorkID: 1, ExpectedChangeCounter: 2}); !errors.Is(err, subject.ErrPDFStateConflict) {
		t.Fatalf("stale ABA removal error = %v", err)
	}
	assertDocumentUnchanged(t, store, "10.1000/aba", before)
}

// TestAuditReplaySurvivesStoreRestart verifies committed outbox events remain
// deliverable after the companion store is reopened.
func TestAuditReplaySurvivesStoreRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	registry := testenv.Path("config", "database.something")
	metadata, err := testfixture.Open(filepath.Join(root, "corpus.metadata.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	pdfPath := filepath.Join(root, "corpus.pdf.db")
	store, err := subject.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Register(ctx, "10.1000/restart", 1, 1); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/restart", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\nrestart"),
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = subject.Open(pdfPath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); err != nil || flushed != 2 {
		t.Fatalf("restart flush=(%d,%v), want 2", flushed, err)
	}
	if repeated, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); err != nil || repeated != 0 {
		t.Fatalf("restart repeat flush=(%d,%v), want 0", repeated, err)
	}
	var events, links int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 2 || links != 2 {
		t.Fatalf("restart audit events=%d links=%d, want two each", events, links)
	}
}

// TestMetadataMirrorFailureAfterPDFCommitKeepsSelectionAndRetries verifies a
// failed metadata mirror does not undo a committed PDF replacement and remains
// retryable.
func TestMetadataMirrorFailureAfterPDFCommitKeepsSelectionAndRetries(t *testing.T) {
	ctx := context.Background()
	store := testhelpers.OpenTestStore(t)
	metadata, err := testfixture.Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	if _, err := store.Register(ctx, "10.1000/mirror", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/mirror", WorkID: 1, ExpectedChangeCounter: 1, Data: []byte("%PDF-1.7\nmirror original"),
	}); err != nil {
		t.Fatal(err)
	}
	if flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); err != nil || flushed != 2 {
		t.Fatalf("initial flush=(%d,%v), want 2", flushed, err)
	}
	if _, err := metadata.Conn.Exec(`CREATE TRIGGER reject_pdf_mirror
		BEFORE INSERT ON audit_events
		BEGIN SELECT RAISE(ABORT, 'injected mirror failure'); END`); err != nil {
		t.Fatal(err)
	}
	replacement := []byte("%PDF-1.7\nmirror replacement")
	replaced, err := store.Save(ctx, subject.SaveInput{
		DOI: "10.1000/mirror", WorkID: 1, ExpectedChangeCounter: 2, Data: replacement,
	})
	if err != nil {
		t.Fatal(err)
	}
	if flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); flushed != 0 || err == nil || !strings.Contains(err.Error(), "injected mirror failure") {
		t.Fatalf("mirror failure flush=(%d,%v)", flushed, err)
	}
	document, err := store.Document(ctx, "10.1000/mirror")
	if err != nil || document == nil || document.ContentHash != replaced.Document.ContentHash || document.ChangeCounter != 3 {
		t.Fatalf("committed PDF state after mirror failure = %+v err=%v", document, err)
	}
	if pending, err := store.PendingAuditCount(ctx); err != nil || pending != 1 {
		t.Fatalf("pending after mirror failure=(%d,%v), want 1", pending, err)
	}
	if _, err := metadata.Conn.Exec("DROP TRIGGER reject_pdf_mirror"); err != nil {
		t.Fatal(err)
	}
	if flushed, err := store.FlushAuditOutbox(ctx, metadata.PDFBindings); err != nil || flushed != 1 {
		t.Fatalf("mirror retry flush=(%d,%v), want 1", flushed, err)
	}
	document, err = store.Document(ctx, "10.1000/mirror")
	if err != nil || document == nil || document.ContentHash != replaced.Document.ContentHash || document.ChangeCounter != 3 {
		t.Fatalf("PDF state after mirror retry = %+v err=%v", document, err)
	}
	var events, links int
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action = 'pdf_document_replaced'").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Conn.QueryRow("SELECT COUNT(*) FROM pdf_audit_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if events != 1 || links != 3 {
		t.Fatalf("mirror retry replacement events=%d links=%d, want 1 and 3", events, links)
	}
}
