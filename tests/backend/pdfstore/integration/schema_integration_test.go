// schema_integration_test.go verifies companion schema readiness checks the
// document-management columns, index, and protection triggers required before writes.

package pdfstore_test

import (
	"context"
	"strings"
	"testing"

	subject "analysis/backend/pdfstore"
	testhelpers "analysis/tests/support/go/backend/pdfstore"
)

// TestVerifySchemaRequiresDocumentManagementContracts verifies current schema passes and missing contracts fail.
func TestVerifySchemaRequiresDocumentManagementContracts(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		change  string
		wantErr string
	}{
		{name: "current"},
		{name: "missing counter trigger", change: "DROP TRIGGER pdf_documents_advance_counter", wantErr: "pdf_documents_advance_counter"},
		{name: "missing content hash index", change: "DROP INDEX idx_pdf_documents_content_hash", wantErr: "content-hash index"},
		{name: "missing outbox table", change: "DROP TABLE pdf_audit_outbox", wantErr: "pdf_audit_outbox"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := testhelpers.OpenTestStore(t)
			if testCase.change != "" {
				if _, err := store.DB.Exec(testCase.change); err != nil {
					t.Fatal(err)
				}
			}
			err := subject.VerifySchema(context.Background(), store.DB)
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %v, want %q", err, testCase.wantErr)
			}
		})
	}
}
