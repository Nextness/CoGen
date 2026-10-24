// pdfannotate_unit_test.go tests the application-owned annotation boundary in isolation.

package pdfannotate_test

import (
	"bytes"
	"strings"
	"testing"

	subject "analysis/backend/pdfannotate"
	"analysis/tests/support/go/pdffixture"
)

// TestAnnotateRejectsEmptySource verifies empty input fails safely.
func TestAnnotateRejectsEmptySource(t *testing.T) {
	if _, err := subject.Annotate(nil, nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty source error = %v", err)
	}
}

// TestAnnotateWithoutAnnotationsCopiesSource verifies the source is returned unchanged.
func TestAnnotateWithoutAnnotationsCopiesSource(t *testing.T) {
	source := pdffixture.Bytes("Fixture page")
	output, err := subject.Annotate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, source) {
		t.Fatal("output differs from source when no annotations are requested")
	}
	if &output[0] == &source[0] {
		t.Fatal("output aliases the source buffer")
	}
}

// TestAnnotateRejectsUnsupportedType verifies unknown annotation types fail safely.
func TestAnnotateRejectsUnsupportedType(t *testing.T) {
	source := pdffixture.Bytes("Fixture page")
	_, err := subject.Annotate(source, []subject.Annotation{{
		Page: 1,
		Type: "Unsupported",
		Rect: subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
	}})
	if err == nil || !strings.Contains(err.Error(), "unsupported annotation type") {
		t.Fatalf("unsupported type error = %v", err)
	}
}

// TestAnnotateTextPreservesSource verifies a text annotation produces a valid PDF without mutating the source.
func TestAnnotateTextPreservesSource(t *testing.T) {
	source := pdffixture.Bytes("Fixture page")
	original := append([]byte(nil), source...)
	output, err := subject.Annotate(source, []subject.Annotation{{
		Page:     1,
		Type:     subject.AnnotationText,
		Rect:     subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
		Contents: "Reviewer note",
		Title:    "Reviewer",
		Subject:  "Comment",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatal("annotated output is not a PDF")
	}
	if !bytes.Equal(source, original) {
		t.Fatal("source bytes changed during annotation")
	}
}

// TestAnnotateHighlightPreservesSource verifies a highlight annotation produces a valid PDF without mutating the source.
func TestAnnotateHighlightPreservesSource(t *testing.T) {
	source := pdffixture.Bytes("Fixture page")
	original := append([]byte(nil), source...)
	output, err := subject.Annotate(source, []subject.Annotation{{
		Page:     1,
		Type:     subject.AnnotationHighlight,
		Rect:     subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
		Contents: "Highlighted text",
		Title:    "Reviewer",
		Subject:  "Highlight",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatal("annotated output is not a PDF")
	}
	if !bytes.Equal(source, original) {
		t.Fatal("source bytes changed during annotation")
	}
}
