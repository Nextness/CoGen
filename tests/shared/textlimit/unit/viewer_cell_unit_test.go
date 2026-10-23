// viewer_cell_unit_test.go verifies Unicode behavior at the viewer's documented cell-byte boundary.
package textlimit_test

import (
	"analysis/internal/textlimit"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateUTF8BytesPreservesByteLimitAndEncoding verifies table-cell truncation uses bytes safely.
func TestTruncateUTF8BytesPreservesByteLimitAndEncoding(t *testing.T) {
	value := strings.Repeat("x", 1024-1) + "😀"
	truncated := textlimit.UTF8Prefix(value, 1024)
	if len(truncated) > 1024 || !utf8.ValidString(truncated) {
		t.Fatalf("truncated value length=%d valid=%v", len(truncated), utf8.ValidString(truncated))
	}
	if truncated != strings.Repeat("x", 1024-1) {
		t.Fatalf("truncated value = %q", truncated)
	}
}
