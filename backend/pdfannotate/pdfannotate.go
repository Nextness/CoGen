// Package pdfannotate adds application-owned annotations to PDF bytes without
// exposing the underlying PDF library to callers. It never opens databases and
// keeps pdfcpu model types behind its handwritten API.
//
// The pdfcpu dependency is licensed under the Apache License, Version 2.0.
// See https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/LICENSE.txt.
package pdfannotate

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

var (
	// ErrProtectedPDF reports an encrypted or permission-restricted source that must not be transformed.
	ErrProtectedPDF = errors.New("PDF is encrypted or modification-restricted")
	// ErrSignedPDF reports a digitally signed source whose signature a transformation would invalidate.
	ErrSignedPDF = errors.New("PDF is digitally signed")
	// ErrMissingPage reports an annotation page outside the source document.
	ErrMissingPage = errors.New("annotation page does not exist")
	// ErrInvalidGeometry reports finite, normalized geometry outside the page.
	ErrInvalidGeometry = errors.New("annotation geometry is invalid")
)

// init disables pdfcpu's user configuration directory so annotation work never
// reads or writes operator configuration or user fonts.
func init() {
	api.DisableConfigDir()
}

// AnnotationType identifies the kind of annotation to add.
type AnnotationType string

const (
	// AnnotationText adds a sticky-note text annotation.
	AnnotationText AnnotationType = "Text"
	// AnnotationHighlight adds a text highlight annotation.
	AnnotationHighlight AnnotationType = "Highlight"
)

// Rectangle is an application-owned annotation rectangle in PDF user space.
type Rectangle struct {
	X1, Y1, X2, Y2 float64
}

// NormalizedRectangle is a top-left-origin rectangle normalized to the page's
// unrotated crop box. Stored review anchors use this orientation.
type NormalizedRectangle struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Annotation is application-owned annotation data. It never exposes pdfcpu types.
type Annotation struct {
	Page       int
	Type       AnnotationType
	Rect       Rectangle
	Rectangles []NormalizedRectangle
	Contents   string
	Title      string
	Subject    string
	ID         string
	ModifiedAt string
}

// pageGeometry describes the unrotated crop box and page transform fields used
// to place annotations in default user space.
type pageGeometry struct {
	originX  float64
	originY  float64
	width    float64
	height   float64
	rotation int
	userUnit float64
}

// Annotate returns a copy of source with annotations added. It does not mutate
// source, preserves existing page content and annotations, and rejects encrypted,
// permission-restricted, or signed documents instead of bypassing restrictions.
func Annotate(source []byte, annotations []Annotation) ([]byte, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("source PDF is empty")
	}
	if len(annotations) == 0 {
		return append([]byte(nil), source...), nil
	}
	if len(annotations) > MaxAnnotations {
		return nil, &LimitError{Kind: "annotation count", Count: len(annotations), Limit: MaxAnnotations}
	}
	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.ADDANNOTATIONS
	ctx, err := api.ReadAndValidate(bytes.NewReader(source), conf)
	if err != nil {
		return nil, fmt.Errorf("read source PDF: %w", err)
	}
	if ctx.Encrypt != nil || len(ctx.EncKey) != 0 {
		return nil, ErrProtectedPDF
	}
	if ctx.SignatureExist || len(ctx.Signatures) != 0 {
		return nil, ErrSignedPDF
	}
	existing := countPageAnnotations(ctx)
	byPage := make(map[int][]model.AnnotationRenderer)
	for _, annotation := range annotations {
		renderer, err := rendererFor(ctx, annotation)
		if err != nil {
			return nil, err
		}
		byPage[annotation.Page] = append(byPage[annotation.Page], renderer)
	}
	ok, err := pdfcpu.AddAnnotationsMap(ctx, byPage, false)
	if err != nil {
		return nil, fmt.Errorf("add annotations: %w", err)
	}
	if !ok {
		return nil, errors.New("add annotations: no annotations were added")
	}
	var output bytes.Buffer
	if err := api.Write(ctx, &output, conf); err != nil {
		return nil, fmt.Errorf("write annotated PDF: %w", err)
	}
	data := output.Bytes()
	if len(data) > MaxOutputBytes {
		return nil, &LimitError{Kind: "output PDF", Count: len(data), Limit: MaxOutputBytes}
	}
	if err := validateAnnotatedOutput(data, ctx.PageCount, existing+len(annotations)); err != nil {
		return nil, err
	}
	return append([]byte(nil), data...), nil
}

// rendererFor maps application-owned annotation data and page geometry to a pdfcpu renderer.
func rendererFor(ctx *model.Context, annotation Annotation) (model.AnnotationRenderer, error) {
	if annotation.Page < 1 || annotation.Page > ctx.PageCount {
		return nil, fmt.Errorf("%w: page %d of %d", ErrMissingPage, annotation.Page, ctx.PageCount)
	}
	geometry, err := pageGeometryFor(ctx, annotation.Page)
	if err != nil {
		return nil, err
	}
	rectangles, err := mapAnnotationRectangles(annotation, geometry)
	if err != nil {
		return nil, err
	}
	rect := enclosingRectangle(rectangles)
	modDate, err := annotationModificationDate(annotation.ModifiedAt)
	if err != nil {
		return nil, err
	}
	switch annotation.Type {
	case AnnotationText:
		return model.NewTextAnnotation(rect, 0, annotation.Contents, annotation.ID, modDate, 0, nil, annotation.Title, nil, nil, "", annotation.Subject, 0, 0, 0, false, "Comment"), nil
	case AnnotationHighlight:
		quads := make(types.QuadPoints, 0, len(rectangles))
		for _, rectangle := range rectangles {
			quad := types.NewQuadLiteralForRect(&rectangle)
			quads.AddQuadLiteral(*quad)
		}
		return model.NewHighlightAnnotation(rect, 0, annotation.Contents, annotation.ID, modDate, 0, nil, 0, 0, 0, annotation.Title, nil, nil, "", annotation.Subject, quads), nil
	default:
		return nil, fmt.Errorf("unsupported annotation type %q", annotation.Type)
	}
}

// mapAnnotationRectangles converts normalized unrotated rectangles into default
// user space using the actual crop-box origin and dimensions. Page rotation is
// already removed from stored review geometry, and UserUnit scales physical
// display rather than default user-space annotation coordinates.
func mapAnnotationRectangles(annotation Annotation, geometry pageGeometry) ([]types.Rectangle, error) {
	if len(annotation.Rectangles) != 0 {
		rectangles := make([]types.Rectangle, 0, len(annotation.Rectangles))
		for _, rectangle := range annotation.Rectangles {
			mapped, err := mapNormalizedRectangle(rectangle, geometry)
			if err != nil {
				return nil, err
			}
			rectangles = append(rectangles, *mapped)
		}
		return rectangles, nil
	}
	if !finiteRectangle(annotation.Rect) {
		return nil, ErrInvalidGeometry
	}
	return []types.Rectangle{*types.NewRectangle(annotation.Rect.X1, annotation.Rect.Y1, annotation.Rect.X2, annotation.Rect.Y2)}, nil
}

// mapNormalizedRectangle converts one top-left-origin normalized rectangle.
func mapNormalizedRectangle(rectangle NormalizedRectangle, geometry pageGeometry) (*types.Rectangle, error) {
	values := []float64{rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, ErrInvalidGeometry
		}
	}
	if rectangle.X < 0 || rectangle.Y < 0 || rectangle.Width <= 0 || rectangle.Height <= 0 ||
		rectangle.X+rectangle.Width > 1.000001 || rectangle.Y+rectangle.Height > 1.000001 {
		return nil, ErrInvalidGeometry
	}
	x1 := geometry.originX + rectangle.X*geometry.width
	x2 := geometry.originX + (rectangle.X+rectangle.Width)*geometry.width
	y2 := geometry.originY + (1-rectangle.Y)*geometry.height
	y1 := geometry.originY + (1-rectangle.Y-rectangle.Height)*geometry.height
	return types.NewRectangle(x1, y1, x2, y2), nil
}

// enclosingRectangle returns the smallest rectangle containing every quad.
func enclosingRectangle(rectangles []types.Rectangle) types.Rectangle {
	first := rectangles[0]
	minX, minY, maxX, maxY := first.LL.X, first.LL.Y, first.UR.X, first.UR.Y
	for _, rectangle := range rectangles[1:] {
		minX = math.Min(minX, rectangle.LL.X)
		minY = math.Min(minY, rectangle.LL.Y)
		maxX = math.Max(maxX, rectangle.UR.X)
		maxY = math.Max(maxY, rectangle.UR.Y)
	}
	return *types.NewRectangle(minX, minY, maxX, maxY)
}

// pageGeometryFor reads the actual unrotated crop box and page transform fields.
func pageGeometryFor(ctx *model.Context, page int) (pageGeometry, error) {
	dict, _, attrs, err := ctx.PageDict(page, false)
	if err != nil {
		return pageGeometry{}, fmt.Errorf("read page %d geometry: %w", page, err)
	}
	box := attrs.CropBox
	if box == nil || !box.Visible() {
		box = attrs.MediaBox
	}
	if box == nil || !box.Visible() {
		return pageGeometry{}, fmt.Errorf("page %d has no visible crop or media box", page)
	}
	userUnit := 1.0
	if value, found := dict.Find("UserUnit"); found {
		number, err := ctx.DereferenceNumber(value)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
			return pageGeometry{}, fmt.Errorf("page %d has invalid UserUnit", page)
		}
		userUnit = number
	}
	rotation := ((attrs.Rotate % 360) + 360) % 360
	return pageGeometry{
		originX: box.LL.X, originY: box.LL.Y, width: box.Width(), height: box.Height(),
		rotation: rotation, userUnit: userUnit,
	}, nil
}

// annotationModificationDate validates and converts one recorded timestamp to a PDF date.
func annotationModificationDate(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return types.DateString(parsed.UTC()), nil
		}
	}
	if parsed, ok := types.DateTime(value, true); ok {
		return types.DateString(parsed.UTC()), nil
	}
	return "", fmt.Errorf("annotation modification date %q is invalid", value)
}

// finiteRectangle reports whether one explicit user-space rectangle is finite and positive.
func finiteRectangle(rectangle Rectangle) bool {
	values := []float64{rectangle.X1, rectangle.Y1, rectangle.X2, rectangle.Y2}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return rectangle.X2 > rectangle.X1 && rectangle.Y2 > rectangle.Y1
}

// countPageAnnotations counts existing annotations across every page.
func countPageAnnotations(ctx *model.Context) int {
	total := 0
	for page := 1; page <= ctx.PageCount; page++ {
		dict, _, _, err := ctx.PageDict(page, false)
		if err != nil {
			continue
		}
		object, found := dict.Find("Annots")
		if !found {
			continue
		}
		array, err := ctx.DereferenceArray(object)
		if err != nil {
			continue
		}
		total += len(array)
	}
	return total
}

// validateAnnotatedOutput verifies the written document keeps its page count and
// contains exactly the existing plus requested application annotations.
func validateAnnotatedOutput(data []byte, pages, expected int) error {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return errors.New("annotated output is not a PDF")
	}
	ctx, err := api.ReadAndValidate(bytes.NewReader(data), model.NewDefaultConfiguration())
	if err != nil {
		return fmt.Errorf("read annotated PDF: %w", err)
	}
	if ctx.PageCount != pages {
		return fmt.Errorf("annotated PDF page count changed from %d to %d", pages, ctx.PageCount)
	}
	if actual := countPageAnnotations(ctx); actual != expected {
		return fmt.Errorf("annotated PDF contains %d annotations, expected %d", actual, expected)
	}
	return nil
}
