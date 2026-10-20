// main_unit_test.go provides unit tests for the command entry point that do
// not require a built binary or a migrated database.
package cli_test

import subject "analysis/backend/cli"

import (
	"bytes"

	"regexp"
	"strings"
	"testing"
)

// TestVersion verifies the version command output format and current value.
func TestVersion(t *testing.T) {
	var output, errors bytes.Buffer
	if code := subject.Run([]string{"version"}, &output, &errors); code != 0 {
		t.Fatalf("version status=%d stderr=%s", code, &errors)
	}
	got := strings.TrimSpace(output.String())
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(got) {
		t.Fatalf("version() = %q, want MAJOR.MINOR.PATCH", got)
	}
	if !strings.HasPrefix(got, "1.0.0") {
		t.Errorf("version() = %q, want current version 1.0.0", got)
	}
}

// TestValidateLoopbackAddress verifies writable serving rejects names, wildcards, and remote IPs.
func TestValidateLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8080", "[::1]:0"} {
		var output, errors bytes.Buffer
		if code := subject.Run([]string{"serve", "--db", "missing.db", "--assets-dir", "missing-assets", "--addr", address}, &output, &errors); code != 1 {
			t.Errorf("accepted loopback address %q returned status=%d", address, code)
		}
	}
	for _, address := range []string{"localhost:8080", "0.0.0.0:8080", "[::]:8080", "192.0.2.1:8080", "missing-port"} {
		var output, errors bytes.Buffer
		if code := subject.Run([]string{"serve", "--db", "missing.db", "--assets-dir", "missing-assets", "--addr", address}, &output, &errors); code != 2 {
			t.Errorf("invalid loopback address %q returned status=%d", address, code)
		}
	}
}
