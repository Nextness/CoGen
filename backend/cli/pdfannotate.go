package cli

import (
	"encoding/json"
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

// pdfAnnotateMain runs the internal PDF annotation worker. It reads one JSON request from standard input and writes annotated PDF bytes to standard output.
func pdfAnnotateMain(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "pdf-annotate takes no arguments")
		return 2
	}
	var request pdfannotateRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintf(stderr, "decode annotation request: %v\n", err)
		return 1
	}
	output, err := pdfannotate.Annotate(request.PDF, request.Annotations)
	if err != nil {
		fmt.Fprintf(stderr, "annotate PDF: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(output); err != nil {
		fmt.Fprintf(stderr, "write annotated PDF: %v\n", err)
		return 1
	}
	return 0
}
