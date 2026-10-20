// Package pdffixture builds deterministic selectable PDFs for isolated viewer and end-to-end fixtures.
package pdffixture

import (
	"bytes"
	"fmt"
	"strings"
)

// Bytes returns a structurally valid multi-page PDF with selectable Helvetica text.
func Bytes(pageText ...string) []byte {
	if len(pageText) == 0 {
		pageText = []string{"Selectable fixture PDF text"}
	}
	objects := make([]string, 0, 3+len(pageText)*2)
	pageIDs := make([]int, len(pageText))
	contentIDs := make([]int, len(pageText))
	fontID := 3 + len(pageText)
	for index := range pageText {
		pageIDs[index] = 3 + index
		contentIDs[index] = fontID + 1 + index
	}
	kids := make([]string, len(pageIDs))
	for index, id := range pageIDs {
		kids[index] = fmt.Sprintf("%d 0 R", id)
	}
	objects = append(objects,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageIDs)),
	)
	for index := range pageText {
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", fontID, contentIDs[index]))
	}
	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	for _, text := range pageText {
		escaped := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(text)
		stream := "BT /F1 18 Tf 72 720 Td (" + escaped + ") Tj ET\n"
		objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
	}
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
