// Package pdfannotate adds application-owned annotations to PDF bytes without
// exposing the underlying PDF library to callers. It never opens databases and
// keeps pdfcpu model types behind its handwritten API.
//
// The pdfcpu dependency is licensed under the Apache License, Version 2.0.
// See https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/LICENSE.txt.
package pdfannotate

import (
	"bytes"
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
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

// Annotation is application-owned annotation data. It never exposes pdfcpu types.
type Annotation struct {
	Page     int
	Type     AnnotationType
	Rect     Rectangle
	Contents string
	Title    string
	Subject  string
}

// Annotate returns a copy of source with annotations added. It does not mutate source.
func Annotate(source []byte, annotations []Annotation) ([]byte, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("source PDF is empty")
	}
	if len(annotations) == 0 {
		return append([]byte(nil), source...), nil
	}

	byPage := make(map[int][]model.AnnotationRenderer)
	for _, a := range annotations {
		renderer, err := rendererFor(a)
		if err != nil {
			return nil, err
		}
		byPage[a.Page] = append(byPage[a.Page], renderer)
	}

	var output bytes.Buffer
	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.ADDANNOTATIONS
	if err := api.AddAnnotationsMap(bytes.NewReader(source), &output, byPage, conf); err != nil {
		return nil, fmt.Errorf("add annotations: %w", err)
	}
	return output.Bytes(), nil
}

// rendererFor maps application-owned annotation data to a pdfcpu renderer.
func rendererFor(a Annotation) (model.AnnotationRenderer, error) {
	rect := types.NewRectangle(a.Rect.X1, a.Rect.Y1, a.Rect.X2, a.Rect.Y2)
	switch a.Type {
	case AnnotationText:
		return model.NewTextAnnotation(*rect, 0, a.Contents, "", "", 0, nil, a.Title, nil, nil, "", a.Subject, 0, 0, 0, false, "Comment"), nil
	case AnnotationHighlight:
		quad := types.QuadPoints{*types.NewQuadLiteralForRect(rect)}
		return model.NewHighlightAnnotation(*rect, 0, a.Contents, "", "", 0, nil, 0, 0, 0, a.Title, nil, nil, "", a.Subject, quad), nil
	default:
		return nil, fmt.Errorf("unsupported annotation type %q", a.Type)
	}
}
