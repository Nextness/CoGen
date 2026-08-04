// validation_unit_test.go tests PDF validation in isolation.
//go:build unit

package pdfstore

import (
	"strings"
	"testing"
)

// TestValidatePDFBoundariesAndHash verifies validate pdf boundaries and hash.
func TestValidatePDFBoundariesAndHash(t *testing.T) {
	data := []byte("%PDF-1.7\nfixture")
	hash, err := ValidatePDF(data, len(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
	if _, err := ValidatePDF(data, len(data)-1); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized PDF error = %v", err)
	}
	if _, err := ValidatePDF([]byte("not a PDF"), 100); err == nil || !strings.Contains(err.Error(), "PDF header") {
		t.Fatalf("invalid PDF error = %v", err)
	}
}
