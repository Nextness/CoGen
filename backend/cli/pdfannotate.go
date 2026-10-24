package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"analysis/backend/pdfannotate"
)

// Worker request and response bounds. The JSON request is capped so a hostile
// or accidental upload cannot exhaust memory before validation. The PDF and
// output caps mirror the companion store and derived-export limits.
const (
	maxWorkerRequestBytes = 30 * 1024 * 1024
	maxWorkerPDFBytes     = 20_000_000
	maxWorkerOutputBytes  = 25 * 1024 * 1024
	maxWorkerAnnotations  = 200
	maxWorkerTextBytes    = 2_000_000
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
	limited := io.LimitReader(os.Stdin, maxWorkerRequestBytes+1)
	var request pdfannotateRequest
	if err := json.NewDecoder(limited).Decode(&request); err != nil {
		fmt.Fprintf(stderr, "decode annotation request: %v\n", err)
		return 1
	}
	if len(request.PDF) > maxWorkerPDFBytes {
		fmt.Fprintf(stderr, "PDF exceeds %d bytes\n", maxWorkerPDFBytes)
		return 1
	}
	if len(request.Annotations) > maxWorkerAnnotations {
		fmt.Fprintf(stderr, "annotation count %d exceeds %d\n", len(request.Annotations), maxWorkerAnnotations)
		return 1
	}
	var textBytes int
	for _, annotation := range request.Annotations {
		textBytes += len(annotation.Contents) + len(annotation.Title) + len(annotation.Subject)
	}
	if textBytes > maxWorkerTextBytes {
		fmt.Fprintf(stderr, "annotation text exceeds %d bytes\n", maxWorkerTextBytes)
		return 1
	}
	output, err := pdfannotate.Annotate(request.PDF, request.Annotations)
	if err != nil {
		fmt.Fprintf(stderr, "annotate PDF: %v\n", err)
		return 1
	}
	if len(output) > maxWorkerOutputBytes {
		fmt.Fprintf(stderr, "annotated PDF exceeds %d bytes\n", maxWorkerOutputBytes)
		return 1
	}
	if _, err := stdout.Write(output); err != nil {
		fmt.Fprintf(stderr, "write annotated PDF: %v\n", err)
		return 1
	}
	return 0
}
