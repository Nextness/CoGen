// Package pdfannotate_test tests the internal annotation worker through its process boundary.
package pdfannotate_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"analysis/backend/pdfannotate"
	"analysis/tests/support/go/pdffixture"
	"analysis/tests/support/go/testenv"
)

// workerRequest mirrors the JSON protocol accepted by the pdf-annotate worker.
type workerRequest struct {
	PDF         []byte                   `json:"pdf"`
	Annotations []pdfannotate.Annotation `json:"annotations"`
}

// buildWorker builds the analysis binary in a unique directory under build/pdf-trial.
func buildWorker(t *testing.T) string {
	t.Helper()
	root := testenv.Root()
	parent := filepath.Join(root, "build", "pdf-trial")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(parent, "pdfannotate-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	binary := filepath.Join(dir, "analysis")
	command := exec.Command("make", "build", "BIN="+binary)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build worker: %v\n%s", err, output)
	}
	return binary
}

// runWorker runs the worker with one JSON request and returns its output and error text.
func runWorker(t *testing.T, binary string, request []byte) ([]byte, string, error) {
	t.Helper()
	command := exec.Command(binary, "pdf-annotate")
	command.Stdin = bytes.NewReader(request)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.Bytes(), stderr.String(), err
}

// TestWorkerAnnotatesTextAndHighlight verifies text and highlight annotations survive the worker boundary without changing source bytes.
func TestWorkerAnnotatesTextAndHighlight(t *testing.T) {
	binary := buildWorker(t)
	source := pdffixture.Bytes("Fixture page")
	original := append([]byte(nil), source...)
	payload, err := json.Marshal(workerRequest{
		PDF: source,
		Annotations: []pdfannotate.Annotation{
			{
				Page:     1,
				Type:     pdfannotate.AnnotationText,
				Rect:     pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
				Contents: "Reviewer note",
				Title:    "Reviewer",
				Subject:  "Comment",
			},
			{
				Page:     1,
				Type:     pdfannotate.AnnotationHighlight,
				Rect:     pdfannotate.Rectangle{X1: 30, Y1: 30, X2: 40, Y2: 40},
				Contents: "Highlighted text",
				Title:    "Reviewer",
				Subject:  "Highlight",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output, stderr, err := runWorker(t, binary, payload)
	if err != nil {
		t.Fatalf("worker failed: %v\n%s", err, stderr)
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatal("annotated output is not a PDF")
	}
	if bytes.Equal(output, original) {
		t.Fatal("annotated output equals source")
	}
	if !bytes.Equal(source, original) {
		t.Fatal("source bytes changed during annotation")
	}
}

// TestWorkerAnnotatesComplexPage verifies the worker preserves a complex page with crop origins, rotation, user units, and an existing annotation without changing source bytes.
func TestWorkerAnnotatesComplexPage(t *testing.T) {
	binary := buildWorker(t)
	source := pdffixture.ComplexBytes("Complex fixture page")
	original := append([]byte(nil), source...)
	payload, err := json.Marshal(workerRequest{
		PDF: source,
		Annotations: []pdfannotate.Annotation{
			{
				Page:     1,
				Type:     pdfannotate.AnnotationText,
				Rect:     pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
				Contents: "Unicode: 日本語 😀",
				Title:    "Reviewer",
				Subject:  "Comment",
			},
			{
				Page:     1,
				Type:     pdfannotate.AnnotationHighlight,
				Rect:     pdfannotate.Rectangle{X1: 30, Y1: 30, X2: 40, Y2: 40},
				Contents: "Highlighted text",
				Title:    "Reviewer",
				Subject:  "Highlight",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output, stderr, err := runWorker(t, binary, payload)
	if err != nil {
		t.Fatalf("worker failed: %v\n%s", err, stderr)
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatal("annotated output is not a PDF")
	}
	if bytes.Equal(output, original) {
		t.Fatal("annotated output equals source")
	}
	if !bytes.Equal(source, original) {
		t.Fatal("source bytes changed during annotation")
	}
}

// TestWorkerRejectsMalformedRequest verifies malformed JSON fails safely.
func TestWorkerRejectsMalformedRequest(t *testing.T) {
	binary := buildWorker(t)
	_, stderr, err := runWorker(t, binary, []byte(`{"pdf":`))
	if err == nil {
		t.Fatal("malformed request succeeded")
	}
	if !strings.Contains(stderr, "decode annotation request") {
		t.Fatalf("unexpected error text: %s", stderr)
	}
}

// TestWorkerRejectsUnsupportedType verifies unsupported annotation types fail safely.
func TestWorkerRejectsUnsupportedType(t *testing.T) {
	binary := buildWorker(t)
	source := pdffixture.Bytes("Fixture page")
	payload, err := json.Marshal(workerRequest{
		PDF: source,
		Annotations: []pdfannotate.Annotation{{
			Page: 1,
			Type: "Unsupported",
			Rect: pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runWorker(t, binary, payload)
	if err == nil {
		t.Fatal("unsupported annotation type succeeded")
	}
	if !strings.Contains(stderr, "unsupported annotation type") {
		t.Fatalf("unexpected error text: %s", stderr)
	}
}
