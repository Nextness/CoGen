// Package pdffixture builds deterministic selectable PDFs for isolated viewer and end-to-end fixtures.
package pdffixture

import (
	"bytes"
	"fmt"
	"strings"
)

// SignedBytes returns a structurally valid one-page PDF whose catalog declares
// signature flags, for exercising protected-document rejection without signing.
func SignedBytes(pageText ...string) []byte {
	if len(pageText) == 0 {
		pageText = []string{"Signed fixture page"}
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 5 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 7 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /SigFlags 1 /Fields [6 0 R] >>",
		"<< /Type /Annot /Subtype /Widget /FT /Sig /T (Signature1) /Rect [0 0 0 0] /F 132 /P 3 0 R >>",
	}
	escaped := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(pageText[0])
	stream := "BT /F1 18 Tf 72 720 Td (" + escaped + ") Tj ET\n"
	objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
	var output bytes.Buffer
	output.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = output.Len()
		fmt.Fprintf(&output, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for index := 1; index < len(offsets); index++ {
		fmt.Fprintf(&output, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&output, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return output.Bytes()
}
