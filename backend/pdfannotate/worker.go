// worker.go runs the internal pdf-annotate worker process with bounded input,
// output, and cancellation. The worker receives only the framed JSON request on
// standard input and returns PDF bytes on standard output; it receives no
// database handles, storage paths, HTTP listener, or caller environment.
package pdfannotate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	// MaxRequestBytes bounds the complete JSON request accepted by the worker.
	MaxRequestBytes = 30 * 1024 * 1024
	// MaxPDFBytes bounds the source PDF accepted by the worker.
	MaxPDFBytes = 20_000_000
	// MaxOutputBytes bounds the annotated PDF returned by the worker.
	MaxOutputBytes = 25 * 1024 * 1024
	// MaxAnnotations bounds the application annotations accepted by the worker.
	MaxAnnotations = 200
	// MaxTextBytes bounds the annotation contents, title, and subject text.
	MaxTextBytes = 2_000_000
	// DefaultWorkerTimeout bounds one worker invocation when the caller supplies no earlier deadline.
	DefaultWorkerTimeout = 45 * time.Second
	// maxWorkerStderrBytes bounds retained worker diagnostics.
	maxWorkerStderrBytes = 64 * 1024
)

// Worker process exit codes distinguish stable generation failures.
const (
	// WorkerExitFailure is one unclassified worker failure.
	WorkerExitFailure = 1
	// WorkerExitUsage is one invalid worker invocation.
	WorkerExitUsage = 2
	// WorkerExitProtected is one encrypted or permission-restricted source.
	WorkerExitProtected = 3
	// WorkerExitSigned is one digitally signed source.
	WorkerExitSigned = 4
	// WorkerExitMissingPage is one annotation page outside the source.
	WorkerExitMissingPage = 5
	// WorkerExitInvalidGeometry is one unplaceable annotation rectangle.
	WorkerExitInvalidGeometry = 6
	// WorkerExitLimit is one bounded input or output limit.
	WorkerExitLimit = 7
)

var (
	// ErrLimit reports one bounded worker input or output limit.
	ErrLimit = errors.New("PDF annotation limit exceeded")
)

// LimitError identifies one bounded worker input or output limit that was exceeded.
type LimitError struct {
	Kind  string
	Count int
	Limit int
}

// Error returns the bounded worker-limit diagnostic.
func (e *LimitError) Error() string {
	return fmt.Sprintf("PDF annotation %s limit exceeded: %d exceeds %d", e.Kind, e.Count, e.Limit)
}

// Unwrap exposes the shared worker-limit sentinel.
func (e *LimitError) Unwrap() error { return ErrLimit }

// Worker runs the analysis binary's internal pdf-annotate mode.
type Worker struct {
	// Executable is the analysis binary that provides the pdf-annotate command.
	Executable string
	// Timeout bounds one invocation when the caller context has no earlier deadline.
	Timeout time.Duration
}

// NewWorker returns a worker bound to the current executable.
func NewWorker() (*Worker, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve annotation worker executable: %w", err)
	}
	return &Worker{Executable: executable, Timeout: DefaultWorkerTimeout}, nil
}

// NewWorkerAt returns a worker bound to an explicit executable.
func NewWorkerAt(executable string) *Worker {
	return &Worker{Executable: executable, Timeout: DefaultWorkerTimeout}
}

// workerRequest is the JSON protocol shared with the pdf-annotate command.
type workerRequest struct {
	PDF         []byte       `json:"pdf"`
	Annotations []Annotation `json:"annotations"`
}

// Annotate runs one bounded worker process and returns its validated PDF bytes.
// It kills and reaps the process on timeout or caller cancellation.
func (w *Worker) Annotate(ctx context.Context, source []byte, annotations []Annotation) ([]byte, error) {
	if w == nil || w.Executable == "" {
		return nil, errors.New("annotation worker executable is required")
	}
	if len(source) == 0 {
		return nil, errors.New("source PDF is empty")
	}
	if len(source) > MaxPDFBytes {
		return nil, &LimitError{Kind: "source PDF", Count: len(source), Limit: MaxPDFBytes}
	}
	if len(annotations) > MaxAnnotations {
		return nil, &LimitError{Kind: "annotation count", Count: len(annotations), Limit: MaxAnnotations}
	}
	textBytes := 0
	for _, annotation := range annotations {
		textBytes += len(annotation.Contents) + len(annotation.Title) + len(annotation.Subject)
	}
	if textBytes > MaxTextBytes {
		return nil, &LimitError{Kind: "annotation text", Count: textBytes, Limit: MaxTextBytes}
	}
	payload, err := json.Marshal(workerRequest{PDF: source, Annotations: annotations})
	if err != nil {
		return nil, fmt.Errorf("encode annotation worker request: %w", err)
	}
	if len(payload) > MaxRequestBytes {
		return nil, &LimitError{Kind: "request", Count: len(payload), Limit: MaxRequestBytes}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cancel := func() {}
	if w.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, w.Timeout)
	}
	defer cancel()
	command := exec.CommandContext(ctx, w.Executable, "pdf-annotate")
	configureWorkerProcess(command)
	command.Env = []string{}
	command.Stdin = bytes.NewReader(payload)
	stdout := &limitedBuffer{limit: MaxOutputBytes}
	stderr := &limitedBuffer{limit: maxWorkerStderrBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = 2 * time.Second
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, fmt.Errorf("annotation worker stopped: %w", ctx.Err())
		}
		message := bytes.TrimSpace(stderr.Bytes())
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			switch exit.ExitCode() {
			case WorkerExitProtected:
				return nil, fmt.Errorf("%w: %s", ErrProtectedPDF, message)
			case WorkerExitSigned:
				return nil, fmt.Errorf("%w: %s", ErrSignedPDF, message)
			case WorkerExitMissingPage:
				return nil, fmt.Errorf("%w: %s", ErrMissingPage, message)
			case WorkerExitInvalidGeometry:
				return nil, fmt.Errorf("%w: %s", ErrInvalidGeometry, message)
			case WorkerExitLimit:
				return nil, fmt.Errorf("%w: %s", ErrLimit, message)
			}
		}
		if len(message) != 0 {
			return nil, fmt.Errorf("annotation worker failed: %w: %s", err, message)
		}
		return nil, fmt.Errorf("annotation worker failed: %w", err)
	}
	if stdout.Overflow() {
		return nil, &LimitError{Kind: "output PDF", Count: stdout.Written(), Limit: MaxOutputBytes}
	}
	output := append([]byte(nil), stdout.Bytes()...)
	if len(output) == 0 {
		return nil, errors.New("annotation worker returned no PDF bytes")
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		return nil, errors.New("annotation worker returned a non-PDF response")
	}
	return output, nil
}

// limitedBuffer retains at most limit bytes and records whether more were written.
type limitedBuffer struct {
	data     bytes.Buffer
	limit    int
	overflow bool
	written  int
}

// Write retains a bounded prefix of p and discards the remainder.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.written += len(p)
	remaining := b.limit - b.data.Len()
	if remaining <= 0 {
		b.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.data.Write(p)
	return len(p), nil
}

// Bytes returns the retained bounded prefix.
func (b *limitedBuffer) Bytes() []byte { return b.data.Bytes() }

// Overflow reports whether the writer discarded input.
func (b *limitedBuffer) Overflow() bool { return b.overflow }

// Written returns the total byte count presented to the writer.
func (b *limitedBuffer) Written() int { return b.written }
