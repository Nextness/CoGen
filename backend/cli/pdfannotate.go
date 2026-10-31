package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"analysis/backend/pdfannotate"
)

// pdfannotateRequest is the JSON request for the internal annotation worker.
type pdfannotateRequest struct {
	PDF         []byte                   `json:"pdf"`
	Annotations []pdfannotate.Annotation `json:"annotations"`
}

// pdfAnnotateMain runs the internal PDF annotation worker. It reads one bounded JSON request from standard input and writes annotated PDF bytes to standard output. The caller owns cancellation and must kill and reap this process on timeout.
func pdfAnnotateMain(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "pdf-annotate takes no arguments")
		return 2
	}
	limited := io.LimitReader(os.Stdin, pdfannotate.MaxRequestBytes+1)
	var request pdfannotateRequest
	if err := json.NewDecoder(limited).Decode(&request); err != nil {
		fmt.Fprintf(stderr, "decode annotation request: %v\n", err)
		return 1
	}
	if len(request.PDF) > pdfannotate.MaxPDFBytes {
		fmt.Fprintf(stderr, "PDF exceeds %d bytes\n", pdfannotate.MaxPDFBytes)
		return pdfannotate.WorkerExitLimit
	}
	if len(request.Annotations) > pdfannotate.MaxAnnotations {
		fmt.Fprintf(stderr, "annotation count %d exceeds %d\n", len(request.Annotations), pdfannotate.MaxAnnotations)
		return pdfannotate.WorkerExitLimit
	}
	var textBytes int
	for _, annotation := range request.Annotations {
		textBytes += len(annotation.Contents) + len(annotation.Title) + len(annotation.Subject)
	}
	if textBytes > pdfannotate.MaxTextBytes {
		fmt.Fprintf(stderr, "annotation text exceeds %d bytes\n", pdfannotate.MaxTextBytes)
		return pdfannotate.WorkerExitLimit
	}
	output, err := pdfannotate.Annotate(request.PDF, request.Annotations)
	if err != nil {
		fmt.Fprintf(stderr, "annotate PDF: %v\n", err)
		return pdfannotateWorkerExitCode(err)
	}
	if len(output) > pdfannotate.MaxOutputBytes {
		fmt.Fprintf(stderr, "annotated PDF exceeds %d bytes\n", pdfannotate.MaxOutputBytes)
		return pdfannotate.WorkerExitLimit
	}
	if _, err := stdout.Write(output); err != nil {
		fmt.Fprintf(stderr, "write annotated PDF: %v\n", err)
		return 1
	}
	return 0
}

// pdfannotateWorkerExitCode maps one annotation failure to a stable worker exit code.
func pdfannotateWorkerExitCode(err error) int {
	switch {
	case errors.Is(err, pdfannotate.ErrProtectedPDF):
		return pdfannotate.WorkerExitProtected
	case errors.Is(err, pdfannotate.ErrSignedPDF):
		return pdfannotate.WorkerExitSigned
	case errors.Is(err, pdfannotate.ErrMissingPage):
		return pdfannotate.WorkerExitMissingPage
	case errors.Is(err, pdfannotate.ErrInvalidGeometry):
		return pdfannotate.WorkerExitInvalidGeometry
	case errors.Is(err, pdfannotate.ErrLimit):
		return pdfannotate.WorkerExitLimit
	default:
		return pdfannotate.WorkerExitFailure
	}
}
