// validation.go enforces PDF byte bounds and the PDF file signature,
// returning the lowercase SHA-256 content hash of a validated PDF.
package pdfstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const DefaultMaxPDFBytes = 20_000_000

// ValidatePDF enforces the byte bound and the PDF signature, then
// returns the lowercase SHA-256 content hash.
func ValidatePDF(data []byte, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		return "", fmt.Errorf("maximum PDF size must be positive")
	}
	if len(data) > maxBytes {
		return "", fmt.Errorf("PDF exceeds the %d-byte limit", maxBytes)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", fmt.Errorf("document does not start with a PDF header")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
