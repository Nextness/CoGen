// plaintext.go renders saved note bodies as safe plain text for PDF annotation
// contents. It uses the project note parser and never emits active markup,
// application-only link syntax, or executable link targets.
package notes

import (
	"regexp"
	"strings"
)

var customLinkPattern = regexp.MustCompile(`\[\[[^\[\]]+\]\]`)

// PlainText returns the safe plain-text rendering of one saved note body. It
// returns an empty string when the body is not a valid saved note.
func PlainText(body string) string {
	document := Parse(body)
	if len(document.Errors) != 0 {
		return ""
	}
	lines := make([]string, 0, len(document.Blocks))
	for _, block := range document.Blocks {
		switch block.Type {
		case "heading", "paragraph":
			if strings.TrimSpace(block.Text) != "" {
				lines = append(lines, block.Text)
			}
		case "list":
			for _, item := range block.Items {
				lines = append(lines, item)
			}
		case "code":
			if strings.TrimSpace(block.Text) != "" {
				lines = append(lines, block.Text)
			}
		case "table":
			if len(block.Header) != 0 {
				lines = append(lines, strings.Join(block.Header, " | "))
			}
			for _, row := range block.Rows {
				lines = append(lines, strings.Join(row, " | "))
			}
		}
	}
	text := strings.Join(lines, "\n")
	text = customLinkPattern.ReplaceAllStringFunc(text, plainLinkText)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.Map(func(character rune) rune {
		if character < 0x20 && character != '\n' && character != '\t' {
			return -1
		}
		return character
	}, text)
	return strings.TrimSpace(text)
}

// plainLinkText replaces one custom link with its display text or safe target.
func plainLinkText(match string) string {
	if len(match) < 4 {
		return ""
	}
	link, message := decodeLink(match[2 : len(match)-2])
	if message != "" {
		return ""
	}
	if link.DisplayText != nil && strings.TrimSpace(*link.DisplayText) != "" {
		return *link.DisplayText
	}
	switch link.TargetType {
	case "note":
		return "note " + link.RawTarget
	case "pdf_page":
		return "page " + link.RawTarget
	default:
		return link.RawTarget
	}
}
