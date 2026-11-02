//go:build e2e

package offline_test

import (
	"analysis/backend/pdfstore"
	flow "analysis/tests/support/go/e2eflow"
	"analysis/tests/support/go/testfixture"

	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"testing"
)

// TestE2EDeterministic verifies the offline pipeline, database, API, and PDF inventory flow.
func TestE2EDeterministic(t *testing.T) {
	root := flow.E2ERepositoryRoot(t)
	mode := flow.Mode{
		Name: "deterministic",
		Sources: []flow.Source{
			{Name: "alpha_csv", Filename: "offline.csv", FileType: "csv", Count: 4},
			{Name: "beta_bib", Filename: "offline.bib", FileType: "bib", Count: 2},
		},
		ExpectedParsed: 5, ExpectedUnique: 4, ExpectedValid: 2, ExpectedDiscarded: 2,
		ExpectedTitles: []string{"Offline Complete One", "Offline Complete Two"},
	}
	result := flow.RunE2EVariant(t, root, mode, nil)
	flow.AssertE2EAPI(t, result)
	flow.PrepareE2EReviewRuns(t, root, result)
}

// TestE2EReviewEvidence verifies browser mutations against the isolated database after Playwright completes.
func TestE2EReviewEvidence(t *testing.T) {
	root := flow.E2ERepositoryRoot(t)
	path := filepath.Join(root, "build", "e2e", "deterministic", "review", "corpus.metadata.db")
	db, err := testfixture.OpenExistingPlatform(t, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	flow.AssertE2ECount(t, db, "review_contexts", "1=1", nil, 2)
	var a1Context, a2Context, a1Review, a2Review int64
	var a1Status, a2Status string
	if err := db.Conn.QueryRow(`SELECT context.id, head.review_version_id, version.status FROM review_contexts context
		JOIN pipeline_runs run ON run.id=context.pipeline_run_id JOIN review_context_work_heads head ON head.review_context_id=context.id
		JOIN works work ON work.id=head.work_id AND work.doi='10.5555/e2e-complete-one'
		JOIN work_review_versions version ON version.id=head.review_version_id WHERE run.attempt_number=1`).Scan(&a1Context, &a1Review, &a1Status); err != nil {
		t.Fatal(err)
	}
	if err := db.Conn.QueryRow(`SELECT context.id, head.review_version_id, version.status FROM review_contexts context
		JOIN pipeline_runs run ON run.id=context.pipeline_run_id JOIN review_context_work_heads head ON head.review_context_id=context.id
		JOIN works work ON work.id=head.work_id AND work.doi='10.5555/e2e-complete-one'
		JOIN work_review_versions version ON version.id=head.review_version_id WHERE run.attempt_number=2`).Scan(&a2Context, &a2Review, &a2Status); err != nil {
		t.Fatal(err)
	}
	if a1Status != "approved" || a2Status != "not_approved" || a1Review == a2Review {
		t.Fatalf("review heads A1=%d/%s A2=%d/%s", a1Review, a1Status, a2Review, a2Status)
	}
	var parentVersion, parentContext int64
	if err := db.Conn.QueryRow("SELECT parent_version_id, created_in_context_id FROM work_review_versions WHERE id=?", a2Review).Scan(&parentVersion, &parentContext); err != nil {
		t.Fatal(err)
	}
	if parentVersion != a1Review || parentContext != a2Context {
		t.Fatalf("A2 review parent=%d context=%d, want %d/%d", parentVersion, parentContext, a1Review, a2Context)
	}
	var a1NoteHead, a2NoteHead int64
	if err := db.Conn.QueryRow("SELECT note_version_id FROM review_context_note_heads WHERE review_context_id=? AND note_id=1", a1Context).Scan(&a1NoteHead); err != nil {
		t.Fatal(err)
	}
	if err := db.Conn.QueryRow("SELECT note_version_id FROM review_context_note_heads WHERE review_context_id=? AND note_id=1", a2Context).Scan(&a2NoteHead); err != nil {
		t.Fatal(err)
	}
	if a1NoteHead == a2NoteHead {
		t.Fatal("A2 note edit moved the A1 note head")
	}
	var anchorHeads, auditEvents int
	if err := db.Conn.QueryRow(`SELECT COUNT(*) FROM review_context_anchor_heads head
		JOIN review_anchors anchor ON anchor.id=head.anchor_id WHERE anchor.label='e2e-methods'`).Scan(&anchorHeads); err != nil || anchorHeads != 2 {
		t.Fatalf("anchor heads=%d err=%v", anchorHeads, err)
	}
	if err := db.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action LIKE 'review_%' OR action='work_review_version_created'").Scan(&auditEvents); err != nil || auditEvents < 8 {
		t.Fatalf("review audit events=%d err=%v", auditEvents, err)
	}
	assertE2EPDFLifecycleEvidence(t, root, path, db)
}

// assertE2EPDFLifecycleEvidence verifies final selections, deleted blobs, counters, and exactly-once mutation audit.
func assertE2EPDFLifecycleEvidence(t *testing.T, root, metadataPath string, metadata *testfixture.Database) {
	t.Helper()
	outputDir := filepath.Dir(metadataPath)
	fixturePath := filepath.Join(outputDir, "review.fixture.pdf")
	replacementPath := filepath.Join(outputDir, "review.replacement.pdf")
	if _, err := os.Stat(fixturePath); err != nil {
		fixturePath = filepath.Join(filepath.Dir(outputDir), "review.fixture.pdf")
		replacementPath = filepath.Join(filepath.Dir(outputDir), "review.replacement.pdf")
	}
	finalBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	replacementBytes, err := os.ReadFile(replacementPath)
	if err != nil {
		t.Fatal(err)
	}
	finalHash := sha256Hex(finalBytes)
	replacementHash := sha256Hex(replacementBytes)
	if finalHash == replacementHash {
		t.Fatal("E2E replacement fixture has the original identity")
	}
	ctx := context.Background()
	pdfPath, err := metadata.PDFBindings.ResolveBinding(ctx, metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := pdfstore.Open(pdfPath, filepath.Join(root, "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var status, contentHash string
	var counter int64
	if err := store.DB.QueryRow(`SELECT status, COALESCE(content_hash, ''), change_counter FROM pdf_documents WHERE doi=?`, "10.5555/e2e-complete-one").Scan(&status, &contentHash, &counter); err != nil {
		t.Fatal(err)
	}
	if status != pdfstore.StatusAvailable || contentHash != finalHash || counter != 5 {
		t.Fatalf("final PDF selection status=%s hash=%s counter=%d", status, contentHash, counter)
	}
	var finalBlobs, replacementBlobs int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_blobs WHERE content_hash=?`, finalHash).Scan(&finalBlobs); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM pdf_blobs WHERE content_hash=?`, replacementHash).Scan(&replacementBlobs); err != nil {
		t.Fatal(err)
	}
	if finalBlobs != 1 || replacementBlobs != 0 {
		t.Fatalf("PDF blobs final=%d replacement=%d", finalBlobs, replacementBlobs)
	}
	var inventoried, replaced, removed, prepared int
	if err := metadata.Conn.QueryRow(`SELECT
		SUM(CASE WHEN action='pdf_document_inventoried' THEN 1 ELSE 0 END),
		SUM(CASE WHEN action='pdf_document_replaced' THEN 1 ELSE 0 END),
		SUM(CASE WHEN action='pdf_document_removed' THEN 1 ELSE 0 END),
		SUM(CASE WHEN action='pdf_document_export_prepared' THEN 1 ELSE 0 END)
		FROM audit_events`).Scan(&inventoried, &replaced, &removed, &prepared); err != nil {
		t.Fatal(err)
	}
	if inventoried != 2 || replaced != 1 || removed != 1 || prepared < 3 {
		t.Fatalf("PDF audit inventoried=%d replaced=%d removed=%d prepared=%d", inventoried, replaced, removed, prepared)
	}
	var delivered, pending int
	if err := store.DB.QueryRow(`SELECT
		SUM(CASE WHEN delivered_at IS NOT NULL THEN 1 ELSE 0 END),
		SUM(CASE WHEN delivered_at IS NULL THEN 1 ELSE 0 END)
		FROM pdf_audit_outbox WHERE action IN ('pdf_document_inventoried','pdf_document_replaced','pdf_document_removed')`).Scan(&delivered, &pending); err != nil {
		t.Fatal(err)
	}
	if delivered != 4 || pending != 0 {
		t.Fatalf("PDF mutation outbox delivered=%d pending=%d", delivered, pending)
	}
	var links int
	if err := metadata.Conn.QueryRow(`SELECT COUNT(*) FROM pdf_audit_links link
		JOIN audit_events event ON event.id=link.audit_event_id
		WHERE event.action IN ('pdf_document_inventoried','pdf_document_replaced','pdf_document_removed')`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 4 {
		t.Fatalf("PDF mutation audit links=%d", links)
	}
}

// sha256Hex returns the lowercase SHA-256 identity of one fixture file.
func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// TestE2EMocked verifies enrichment through loopback providers and cross-layer evidence.
func TestE2EMocked(t *testing.T) {
	root := flow.E2ERepositoryRoot(t)
	mock := &flow.ProviderMock{}
	provider := flow.NewE2EProviderServer(t, mock)
	defer provider.Close()
	mode := flow.Mode{
		Name: "mocked", Enrichment: true, ProviderBaseURL: provider.URL,
		Sources: []flow.Source{
			{Name: "alpha_csv", Filename: "offline.csv", FileType: "csv", Count: 4},
			{Name: "beta_bib", Filename: "offline.bib", FileType: "bib", Count: 2},
		},
		ExpectedParsed: 5, ExpectedUnique: 4, ExpectedValid: 3, ExpectedDiscarded: 1,
		ExpectedTitles: []string{"Offline Complete One", "Offline Complete Two", "Provider Enrichment Candidate"},
	}
	result := flow.RunE2EVariant(t, root, mode, flow.E2EOfflineEnvironment(provider.URL))
	mock.AssertRequests(t)
	flow.AssertE2EAPI(t, result)
}
