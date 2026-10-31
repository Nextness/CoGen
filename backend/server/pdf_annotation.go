// pdf_annotation.go builds bounded application annotations from one immutable
// review evidence snapshot. Note prose is rendered through the project note
// parser and never carries active markup or application-only link syntax.
package server

import (
	"strings"
	"unicode/utf8"

	"analysis/backend/database/review"
	"analysis/backend/notes"
	"analysis/backend/pdfannotate"
)

// pdfAnnotationTitleLimit bounds generated attribution and label text.
const pdfAnnotationTitleLimit = 512

// pdfExportNoteText is one saved note's safe rendered prose and attribution.
type pdfExportNoteText struct {
	body      string
	reviewer  string
	createdAt string
	versionID int64
	anchorIDs []string
}

// pdfCommentAnnotations builds one highlight annotation per eligible anchor,
// repeating linked note prose at every matching location.
func pdfCommentAnnotations(evidence *review.ExportEvidence) ([]pdfannotate.Annotation, error) {
	if evidence == nil {
		return nil, nil
	}
	notesByAnchor := make(map[string][]pdfExportNoteText, len(evidence.Anchors))
	for _, note := range evidence.Notes {
		body := notes.PlainText(note.Body)
		if strings.TrimSpace(body) == "" {
			continue
		}
		item := pdfExportNoteText{
			body: body, reviewer: note.ReviewerDisplay, createdAt: note.CreatedAt,
			versionID: note.VersionID,
		}
		for _, anchorID := range note.AnchorIDs {
			notesByAnchor[anchorID] = append(notesByAnchor[anchorID], item)
		}
	}
	annotations := make([]pdfannotate.Annotation, 0, len(evidence.Anchors))
	textBytes := 0
	for _, anchor := range evidence.Anchors {
		linked := notesByAnchor[anchor.ID]
		contents, title, subject, modifiedAt := pdfAnchorAnnotationText(anchor, linked)
		rectangles := make([]pdfannotate.NormalizedRectangle, 0, len(anchor.Rectangles))
		for _, rectangle := range anchor.Rectangles {
			rectangles = append(rectangles, pdfannotate.NormalizedRectangle{
				X: rectangle.X, Y: rectangle.Y, Width: rectangle.Width, Height: rectangle.Height,
			})
		}
		textBytes += len(contents) + len(title) + len(subject)
		if textBytes > pdfannotate.MaxTextBytes {
			return nil, &pdfannotate.LimitError{Kind: "annotation text", Count: textBytes, Limit: pdfannotate.MaxTextBytes}
		}
		annotations = append(annotations, pdfannotate.Annotation{
			Page: anchor.Page, Type: pdfannotate.AnnotationHighlight, Rectangles: rectangles,
			Contents: contents, Title: title, Subject: subject, ID: anchor.ID, ModifiedAt: modifiedAt,
		})
	}
	if len(annotations) > pdfannotate.MaxAnnotations {
		return nil, &pdfannotate.LimitError{Kind: "annotation count", Count: len(annotations), Limit: pdfannotate.MaxAnnotations}
	}
	return annotations, nil
}

// pdfAnchorAnnotationText renders complete saved note prose or an anchor quotation.
func pdfAnchorAnnotationText(anchor review.ExportAnchor, linked []pdfExportNoteText) (string, string, string, string) {
	if len(linked) == 0 {
		contents := strings.TrimSpace(anchor.SelectedText)
		if contents != "" {
			contents = "\u201C" + contents + "\u201D"
		}
		if label := strings.TrimSpace(anchor.Label); label != "" {
			if contents != "" {
				contents += "\n\n"
			}
			contents += "Anchor: " + label
		}
		subject := "CoGen anchor"
		if label := strings.TrimSpace(anchor.Label); label != "" {
			subject += ": " + label
		}
		return contents, boundedAnnotationText(anchor.ReviewerDisplay), boundedAnnotationText(subject), anchor.CreatedAt
	}
	bodies := make([]string, 0, len(linked))
	reviewers := make([]string, 0, len(linked))
	seenReviewers := make(map[string]bool, len(linked))
	modifiedAt := anchor.CreatedAt
	seenNotes := make(map[int64]bool, len(linked))
	for _, note := range linked {
		if seenNotes[note.versionID] {
			continue
		}
		seenNotes[note.versionID] = true
		bodies = append(bodies, note.body)
		if !seenReviewers[note.reviewer] {
			seenReviewers[note.reviewer] = true
			reviewers = append(reviewers, note.reviewer)
		}
		if note.createdAt > modifiedAt {
			modifiedAt = note.createdAt
		}
	}
	contents := strings.Join(bodies, "\n\n---\n\n")
	title := strings.Join(reviewers, ", ")
	if strings.TrimSpace(title) == "" {
		title = anchor.ReviewerDisplay
	}
	subject := "CoGen comment"
	if label := strings.TrimSpace(anchor.Label); label != "" {
		subject += ": " + label
	}
	return contents, boundedAnnotationText(title), boundedAnnotationText(subject), modifiedAt
}

// boundedAnnotationText bounds generated attribution without truncating saved note prose.
func boundedAnnotationText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= pdfAnnotationTitleLimit {
		return value
	}
	for len(value) > pdfAnnotationTitleLimit-1 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value + "\u2026"
}
