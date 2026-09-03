package sqliteuri

import (
	"net/url"
	"testing"
)

// TestFileEncodesPathAndRepeatedParameters verifies file paths and repeated pragmas retain their exact values.
func TestFileEncodesPathAndRepeatedParameters(t *testing.T) {
	path := "/tmp/research ?#.db"
	uri := File(path, map[string][]string{
		"mode":    {"ro"},
		"_pragma": {"query_only(1)", "busy_timeout(5000)"},
	})

	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse URI: %v", err)
	}
	if parsed.Scheme != "file" || parsed.Path != path {
		t.Fatalf("URI path = %q, want %q", parsed.Path, path)
	}
	query := parsed.Query()
	if got := query.Get("mode"); got != "ro" {
		t.Fatalf("mode = %q, want ro", got)
	}
	pragmas := query["_pragma"]
	if len(pragmas) != 2 || pragmas[0] != "query_only(1)" || pragmas[1] != "busy_timeout(5000)" {
		t.Fatalf("_pragma = %#v, want both values", pragmas)
	}
}
