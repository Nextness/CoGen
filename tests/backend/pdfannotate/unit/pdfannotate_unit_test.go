// pdfannotate_unit_test.go tests the application-owned annotation boundary in isolation.

package pdfannotate_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

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

// TestAnnotateUnicodePreservesSource verifies Unicode annotation bodies produce a valid PDF without mutating the source.
func TestAnnotateUnicodePreservesSource(t *testing.T) {
	source := pdffixture.Bytes("Fixture page")
	original := append([]byte(nil), source...)
	output, err := subject.Annotate(source, []subject.Annotation{{
		Page:     1,
		Type:     subject.AnnotationText,
		Rect:     subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20},
		Contents: "Résumé naïve 日本語 😀",
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

// TestAnnotateMultipleAnnotationsPreservesSource verifies multiple annotations produce a valid PDF without mutating the source.
func TestAnnotateMultipleAnnotationsPreservesSource(t *testing.T) {
	source := pdffixture.Bytes("Fixture page")
	original := append([]byte(nil), source...)
	output, err := subject.Annotate(source, []subject.Annotation{
		{Page: 1, Type: subject.AnnotationText, Rect: subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: "First note", Title: "Reviewer", Subject: "Comment"},
		{Page: 1, Type: subject.AnnotationHighlight, Rect: subject.Rectangle{X1: 30, Y1: 30, X2: 40, Y2: 40}, Contents: "Highlighted text", Title: "Reviewer", Subject: "Highlight"},
	})
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

// readPageAnnotations parses one annotated PDF and returns its page annotation dictionaries.
func readPageAnnotations(t *testing.T, data []byte, page int) []types.Dict {
	t.Helper()
	ctx, err := api.ReadAndValidate(bytes.NewReader(data), model.NewDefaultConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	dict, _, _, err := ctx.PageDict(page, false)
	if err != nil {
		t.Fatal(err)
	}
	object, found := dict.Find("Annots")
	if !found {
		return nil
	}
	array, err := ctx.DereferenceArray(object)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]types.Dict, 0, len(array))
	for _, item := range array {
		annotation, err := ctx.DereferenceDict(item)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, annotation)
	}
	return items
}

// annotationRect decodes one annotation Rect array.
func annotationRect(t *testing.T, annotation types.Dict) []float64 {
	t.Helper()
	object, found := annotation.Find("Rect")
	if !found {
		t.Fatal("annotation has no Rect")
	}
	array, ok := object.(types.Array)
	if !ok {
		t.Fatalf("Rect value %T is not an array", object)
	}
	values := make([]float64, 0, len(array))
	for _, item := range array {
		number, ok := item.(types.Float)
		if !ok {
			integer, integerOK := item.(types.Integer)
			if !integerOK {
				t.Fatalf("Rect entry %T is not numeric", item)
			}
			values = append(values, float64(integer.Value()))
			continue
		}
		values = append(values, number.Value())
	}
	return values
}

// TestAnnotateMapsNormalizedGeometry verifies crop origins, rotation, and user
// units map stored unrotated rectangles into default user space.
func TestAnnotateMapsNormalizedGeometry(t *testing.T) {
	source := pdffixture.ComplexBytes("Complex fixture page")
	output, err := subject.Annotate(source, []subject.Annotation{{
		Page: 1, Type: subject.AnnotationHighlight,
		Rectangles: []subject.NormalizedRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}},
		Contents:   "Mapped geometry", Title: "Reviewer", Subject: "Highlight",
	}})
	if err != nil {
		t.Fatal(err)
	}
	annotations := readPageAnnotations(t, output, 1)
	if len(annotations) != 2 {
		t.Fatalf("annotations=%d", len(annotations))
	}
	highlight := annotations[1]
	rect := annotationRect(t, highlight)
	want := []float64{69.2, 550.4, 246.8, 627.6}
	for index, value := range want {
		if math.Abs(rect[index]-value) > 0.001 {
			t.Fatalf("Rect=%v want %v", rect, want)
		}
	}
	object, found := highlight.Find("QuadPoints")
	if !found {
		t.Fatal("highlight has no QuadPoints")
	}
	array, ok := object.(types.Array)
	if !ok {
		t.Fatalf("QuadPoints value %T is not an array", object)
	}
	if len(array) != 8 {
		t.Fatalf("QuadPoints entries=%d", len(array))
	}
	if !bytes.Contains(output, []byte("Complex fixture page")) {
		t.Fatal("annotated output lost searchable page text")
	}
}

// TestAnnotateSupportsMultipleRectangles verifies one highlight carries every quad.
func TestAnnotateSupportsMultipleRectangles(t *testing.T) {
	source := pdffixture.Bytes("Multiple rectangle fixture")
	output, err := subject.Annotate(source, []subject.Annotation{{
		Page: 1, Type: subject.AnnotationHighlight,
		Rectangles: []subject.NormalizedRectangle{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.05}, {X: 0.5, Y: 0.2, Width: 0.2, Height: 0.05}},
		Contents:   "Two rectangles", Title: "Reviewer", Subject: "Highlight",
	}})
	if err != nil {
		t.Fatal(err)
	}
	annotations := readPageAnnotations(t, output, 1)
	if len(annotations) != 1 {
		t.Fatalf("annotations=%d", len(annotations))
	}
	object, found := annotations[0].Find("QuadPoints")
	if !found {
		t.Fatal("highlight has no QuadPoints")
	}
	array, ok := object.(types.Array)
	if !ok {
		t.Fatalf("QuadPoints value %T is not an array", object)
	}
	if len(array) != 16 {
		t.Fatalf("QuadPoints entries=%d", len(array))
	}
}

// TestAnnotateRejectsMissingPageAndGeometry verifies invalid placement fails explicitly.
func TestAnnotateRejectsMissingPageAndGeometry(t *testing.T) {
	source := pdffixture.Bytes("Invalid geometry fixture")
	if _, err := subject.Annotate(source, []subject.Annotation{{Page: 2, Type: subject.AnnotationHighlight, Rectangles: []subject.NormalizedRectangle{{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.1}}}}); !errors.Is(err, subject.ErrMissingPage) {
		t.Fatalf("missing page error=%v", err)
	}
	if _, err := subject.Annotate(source, []subject.Annotation{{Page: 1, Type: subject.AnnotationHighlight, Rectangles: []subject.NormalizedRectangle{{X: math.NaN(), Y: 0.1, Width: 0.2, Height: 0.1}}}}); !errors.Is(err, subject.ErrInvalidGeometry) {
		t.Fatalf("invalid geometry error=%v", err)
	}
	if _, err := subject.Annotate(source, []subject.Annotation{{Page: 1, Type: subject.AnnotationHighlight, Rectangles: []subject.NormalizedRectangle{{X: 0.9, Y: 0.1, Width: 0.2, Height: 0.1}}}}); !errors.Is(err, subject.ErrInvalidGeometry) {
		t.Fatalf("overflow geometry error=%v", err)
	}
}

// TestAnnotateRejectsProtectedAndSignedSources verifies transformation restrictions are not bypassed.
func TestAnnotateRejectsProtectedAndSignedSources(t *testing.T) {
	source := pdffixture.Bytes("Protected fixture")
	conf := model.NewAESConfiguration("", "owner", 256)
	conf.Permissions = model.PermissionsNone
	var encrypted bytes.Buffer
	if err := api.Encrypt(bytes.NewReader(source), &encrypted, conf); err != nil {
		t.Fatal(err)
	}
	if _, err := subject.Annotate(encrypted.Bytes(), []subject.Annotation{{Page: 1, Type: subject.AnnotationText, Rect: subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: "protected"}}); !errors.Is(err, subject.ErrProtectedPDF) {
		t.Fatalf("protected source error=%v", err)
	}
	if _, err := subject.Annotate(pdffixture.SignedBytes("Signed fixture"), []subject.Annotation{{Page: 1, Type: subject.AnnotationText, Rect: subject.Rectangle{X1: 10, Y1: 10, X2: 20, Y2: 20}, Contents: "signed"}}); !errors.Is(err, subject.ErrSignedPDF) {
		t.Fatalf("signed source error=%v", err)
	}
}
