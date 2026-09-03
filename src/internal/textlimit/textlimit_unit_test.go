// textlimit_unit_test.go verifies byte-bounded UTF-8 truncation.
package textlimit

import (
	"testing"
	"unicode/utf8"
)

// TestUTF8PrefixPreservesByteLimitsAndEncoding verifies multibyte prefixes never produce invalid output.
func TestUTF8PrefixPreservesByteLimitsAndEncoding(t *testing.T) {
	for _, value := range []string{"é", "€", "😀"} {
		input := value + value
		got := UTF8Prefix(input, len(value)+1)
		if len(got) > len(value)+1 || !utf8.ValidString(got) || got != value {
			t.Fatalf("UTF8Prefix(%q) = %q", input, got)
		}
	}
}

// TestUTF8PrefixBytesPreservesByteLimitsAndEncoding verifies byte callers receive the same safe prefix.
func TestUTF8PrefixBytesPreservesByteLimitsAndEncoding(t *testing.T) {
	input := []byte("prefix 😀")
	got := UTF8PrefixBytes(input, len(input)-1)
	if len(got) > len(input)-1 || !utf8.Valid(got) || string(got) != "prefix " {
		t.Fatalf("UTF8PrefixBytes(%q) = %q", input, got)
	}
}
