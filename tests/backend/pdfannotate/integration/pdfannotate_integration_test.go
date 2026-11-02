// Package pdfannotate_test tests the internal annotation worker through its process boundary.
package pdfannotate_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestWorkerCancellationTerminates verifies the parent can kill and reap a blocked worker.
func TestWorkerCancellationTerminates(t *testing.T) {
	binary := buildWorker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "pdf-annotate")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	err = command.Wait()
	if err == nil {
		t.Fatal("worker exited before cancellation")
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("context error = %v", ctx.Err())
	}
	if command.ProcessState == nil {
		t.Fatal("worker was not reaped")
	}
}

// TestWorkerRejectsOversizedPDF verifies the worker rejects input above the configured PDF bound.
func TestWorkerRejectsOversizedPDF(t *testing.T) {
	binary := buildWorker(t)
	oversized := make([]byte, 20_000_001)
	payload, err := json.Marshal(workerRequest{
		PDF: oversized,
		Annotations: []pdfannotate.Annotation{{
			Page: 1, Type: pdfannotate.AnnotationText, Rect: pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: "Reviewer note", Title: "Reviewer", Subject: "Comment",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runWorker(t, binary, payload)
	if err == nil {
		t.Fatal("oversized PDF succeeded")
	}
	if !strings.Contains(stderr, "PDF exceeds") {
		t.Fatalf("unexpected error text: %s", stderr)
	}
}

// TestWorkerRejectsExcessiveAnnotationCount verifies the worker rejects requests above the configured annotation count.
func TestWorkerRejectsExcessiveAnnotationCount(t *testing.T) {
	binary := buildWorker(t)
	source := pdffixture.Bytes("Fixture page")
	annotations := make([]pdfannotate.Annotation, 201)
	for index := range annotations {
		annotations[index] = pdfannotate.Annotation{
			Page: 1, Type: pdfannotate.AnnotationText, Rect: pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: "Reviewer note", Title: "Reviewer", Subject: "Comment",
		}
	}
	payload, err := json.Marshal(workerRequest{
		PDF:         source,
		Annotations: annotations,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runWorker(t, binary, payload)
	if err == nil {
		t.Fatal("oversized annotation count succeeded")
	}
	if !strings.Contains(stderr, "annotation count") {
		t.Fatalf("unexpected error text: %s", stderr)
	}
}

// TestWorkerRejectsOversizedAnnotationText verifies the worker rejects requests above the configured annotation text bound.
func TestWorkerRejectsOversizedAnnotationText(t *testing.T) {
	binary := buildWorker(t)
	source := pdffixture.Bytes("Fixture page")
	payload, err := json.Marshal(workerRequest{
		PDF: source,
		Annotations: []pdfannotate.Annotation{{
			Page: 1, Type: pdfannotate.AnnotationText, Rect: pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: strings.Repeat("a", 2_000_001), Title: "Reviewer", Subject: "Comment",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runWorker(t, binary, payload)
	if err == nil {
		t.Fatal("oversized annotation text succeeded")
	}
	if !strings.Contains(stderr, "annotation text exceeds") {
		t.Fatalf("unexpected error text: %s", stderr)
	}
}

// TestWorkerMeasuresExecutionTime verifies a valid request completes within a measured time.
func TestWorkerMeasuresExecutionTime(t *testing.T) {
	binary := buildWorker(t)
	source := pdffixture.Bytes("Fixture page")
	payload, err := json.Marshal(workerRequest{
		PDF: source,
		Annotations: []pdfannotate.Annotation{{
			Page: 1, Type: pdfannotate.AnnotationText, Rect: pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: "Reviewer note", Title: "Reviewer", Subject: "Comment",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "pdf-annotate")
	command.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	start := time.Now()
	if err := command.Run(); err != nil {
		t.Fatalf("worker failed: %v\n%s", err, stderr.String())
	}
	t.Logf("worker elapsed=%s", time.Since(start))
}

// TestWorkerAnnotatesThroughBoundedRunner verifies the application worker runner returns PDF bytes without mutating source.
func TestWorkerAnnotatesThroughBoundedRunner(t *testing.T) {
	binary := buildWorker(t)
	runner := pdfannotate.NewWorkerAt(binary)
	source := pdffixture.Bytes("Runner fixture page")
	original := append([]byte(nil), source...)
	output, err := runner.Annotate(context.Background(), source, []pdfannotate.Annotation{{
		Page:     1,
		Type:     pdfannotate.AnnotationText,
		Rect:     pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
		Contents: "Runner note",
		Title:    "Reviewer",
		Subject:  "Comment",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) || bytes.Equal(output, source) {
		t.Fatal("runner did not return annotated PDF bytes")
	}
	if !bytes.Equal(source, original) {
		t.Fatal("runner mutated source bytes")
	}
}

// TestWorkerRunnerRejectsGenerationFailure verifies malformed sources fail instead of returning unannotated bytes.
func TestWorkerRunnerRejectsGenerationFailure(t *testing.T) {
	binary := buildWorker(t)
	runner := pdfannotate.NewWorkerAt(binary)
	if _, err := runner.Annotate(context.Background(), []byte("%PDF-1.7\nnot a complete document"), []pdfannotate.Annotation{{
		Page:     1,
		Type:     pdfannotate.AnnotationText,
		Rect:     pdfannotate.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
		Contents: "Runner note",
	}}); err == nil {
		t.Fatal("malformed source produced a successful export")
	}
}

// TestWorkerRunnerKillsTimedOutProcess verifies cancellation kills and reaps the worker.
func TestWorkerRunnerKillsTimedOutProcess(t *testing.T) {
	script := filepath.Join(t.TempDir(), "blocked-worker")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := pdfannotate.NewWorkerAt(script)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runner.Annotate(ctx, pdffixture.Bytes("Timeout fixture"), nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
}

// TestWorkerRunnerRejectsOversizedInputs verifies preflight bounds before process start.
func TestWorkerRunnerRejectsOversizedInputs(t *testing.T) {
	runner := pdfannotate.NewWorkerAt("/does/not/exist")
	if _, err := runner.Annotate(context.Background(), make([]byte, pdfannotate.MaxPDFBytes+1), nil); !errors.Is(err, pdfannotate.ErrLimit) {
		t.Fatalf("oversized PDF error=%v", err)
	}
	annotations := make([]pdfannotate.Annotation, pdfannotate.MaxAnnotations+1)
	if _, err := runner.Annotate(context.Background(), []byte("%PDF-1.7"), annotations); !errors.Is(err, pdfannotate.ErrLimit) {
		t.Fatalf("oversized annotation count error=%v", err)
	}
	annotations = []pdfannotate.Annotation{{Contents: strings.Repeat("x", pdfannotate.MaxTextBytes+1)}}
	if _, err := runner.Annotate(context.Background(), []byte("%PDF-1.7"), annotations); !errors.Is(err, pdfannotate.ErrLimit) {
		t.Fatalf("oversized annotation text error=%v", err)
	}
}

// writeWorkerScript writes one executable shell worker for a failure-path test.
func writeWorkerScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestWorkerRunnerRejectsInvalidInputs verifies empty and nil worker boundaries.
func TestWorkerRunnerRejectsInvalidInputs(t *testing.T) {
	var missing *pdfannotate.Worker
	if _, err := missing.Annotate(context.Background(), pdffixture.Bytes("Nil worker"), nil); err == nil {
		t.Fatal("nil worker accepted input")
	}
	if _, err := pdfannotate.NewWorkerAt("/does/not/exist").Annotate(context.Background(), nil, nil); err == nil {
		t.Fatal("empty source accepted")
	}
}

// TestWorkerRunnerReportsStderrAndInvalidOutput verifies explicit process failures.
func TestWorkerRunnerReportsStderrAndInvalidOutput(t *testing.T) {
	stderrWorker := writeWorkerScript(t, "echo 'synthetic worker failure' >&2\nexit 1")
	if _, err := pdfannotate.NewWorkerAt(stderrWorker).Annotate(context.Background(), pdffixture.Bytes("Stderr fixture"), nil); err == nil || !strings.Contains(err.Error(), "synthetic worker failure") {
		t.Fatalf("stderr failure error=%v", err)
	}
	invalidWorker := writeWorkerScript(t, "printf 'not a pdf'")
	if _, err := pdfannotate.NewWorkerAt(invalidWorker).Annotate(context.Background(), pdffixture.Bytes("Invalid output fixture"), nil); err == nil || !strings.Contains(err.Error(), "non-PDF") {
		t.Fatalf("invalid output error=%v", err)
	}
	emptyWorker := writeWorkerScript(t, "exit 0")
	if _, err := pdfannotate.NewWorkerAt(emptyWorker).Annotate(context.Background(), pdffixture.Bytes("Empty output fixture"), nil); err == nil || !strings.Contains(err.Error(), "no PDF bytes") {
		t.Fatalf("empty output error=%v", err)
	}
}

// TestWorkerRunnerMapsExitCodes verifies stable worker failures become typed errors.
func TestWorkerRunnerMapsExitCodes(t *testing.T) {
	cases := []struct {
		code   int
		target error
	}{
		{pdfannotate.WorkerExitProtected, pdfannotate.ErrProtectedPDF},
		{pdfannotate.WorkerExitSigned, pdfannotate.ErrSignedPDF},
		{pdfannotate.WorkerExitMissingPage, pdfannotate.ErrMissingPage},
		{pdfannotate.WorkerExitInvalidGeometry, pdfannotate.ErrInvalidGeometry},
		{pdfannotate.WorkerExitLimit, pdfannotate.ErrLimit},
	}
	for _, test := range cases {
		script := writeWorkerScript(t, fmt.Sprintf("exit %d", test.code))
		if _, err := pdfannotate.NewWorkerAt(script).Annotate(context.Background(), pdffixture.Bytes("Exit code fixture"), nil); !errors.Is(err, test.target) {
			t.Fatalf("exit code %d error=%v", test.code, err)
		}
	}
}

// TestWorkerRunnerRejectsOversizedOutput verifies bounded output retention.
func TestWorkerRunnerRejectsOversizedOutput(t *testing.T) {
	script := writeWorkerScript(t, "head -c 27000000 /dev/zero")
	if _, err := pdfannotate.NewWorkerAt(script).Annotate(context.Background(), pdffixture.Bytes("Output bound fixture"), nil); !errors.Is(err, pdfannotate.ErrLimit) {
		t.Fatalf("oversized output error=%v", err)
	}
}

// TestWorkerRunnerCanceledContext verifies caller cancellation is reported.
func TestWorkerRunnerCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pdfannotate.NewWorkerAt("/does/not/exist").Annotate(ctx, pdffixture.Bytes("Canceled fixture"), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error=%v", err)
	}
}

// TestWorkerLimitDiagnostics verifies bounded limit errors expose their count and sentinel.
func TestWorkerLimitDiagnostics(t *testing.T) {
	limitErr := &pdfannotate.LimitError{Kind: "annotation text", Count: 2, Limit: 1}
	if !errors.Is(limitErr, pdfannotate.ErrLimit) || !strings.Contains(limitErr.Error(), "2 exceeds 1") {
		t.Fatalf("limit diagnostics=%v", limitErr)
	}
	worker, err := pdfannotate.NewWorker()
	if err != nil || worker.Executable == "" {
		t.Fatalf("new worker=%+v err=%v", worker, err)
	}
}

// TestWorkerRunnerAcceptsNilContext verifies the default caller context.
func TestWorkerRunnerAcceptsNilContext(t *testing.T) {
	script := writeWorkerScript(t, "printf '%%PDF-1.7'")
	output, err := pdfannotate.NewWorkerAt(script).Annotate(nil, pdffixture.Bytes("Nil context fixture"), nil)
	if err != nil || !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatalf("nil context output=%q err=%v", output, err)
	}
}

// TestWorkerRunnerRejectsOversizedRequest verifies the framed request bound.
func TestWorkerRunnerRejectsOversizedRequest(t *testing.T) {
	annotations := []pdfannotate.Annotation{{ID: strings.Repeat("x", pdfannotate.MaxRequestBytes+1)}}
	if _, err := pdfannotate.NewWorkerAt("/does/not/exist").Annotate(context.Background(), []byte("%PDF-1.7"), annotations); !errors.Is(err, pdfannotate.ErrLimit) {
		t.Fatalf("oversized request error=%v", err)
	}
}

// TestWorkerRunnerRejectsOutputOneByteOverLimit verifies partial-write overflow detection.
func TestWorkerRunnerRejectsOutputOneByteOverLimit(t *testing.T) {
	script := writeWorkerScript(t, fmt.Sprintf("dd if=/dev/zero bs=%d count=1 2>/dev/null", pdfannotate.MaxOutputBytes+1))
	if _, err := pdfannotate.NewWorkerAt(script).Annotate(context.Background(), pdffixture.Bytes("Partial overflow fixture"), nil); !errors.Is(err, pdfannotate.ErrLimit) {
		t.Fatalf("partial overflow error=%v", err)
	}
}

// TestWorkerRunnerWithoutTimeout verifies the no-timeout path still cleans up.
func TestWorkerRunnerWithoutTimeout(t *testing.T) {
	script := writeWorkerScript(t, "printf '%%PDF-1.7'")
	worker := pdfannotate.NewWorkerAt(script)
	worker.Timeout = 0
	if _, err := worker.Annotate(context.Background(), pdffixture.Bytes("No timeout fixture"), nil); err != nil {
		t.Fatal(err)
	}
}
