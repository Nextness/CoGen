// Package notes parses the bounded review-note language and extracts immutable link evidence.
package notes

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	// MaxBodyBytes is the largest UTF-8 note accepted by the parser and review API.
	MaxBodyBytes = 262144
	// MaxTargetBytes bounds one stored custom-link target.
	MaxTargetBytes = 2048
	// MaxDisplayBytes bounds optional custom-link display text.
	MaxDisplayBytes = 1024
)

var anchorPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,63}$`)

// ValidAnchorID reports whether an anchor identifier is safe for storage, links, and URLs.
func ValidAnchorID(value string) bool { return anchorPattern.MatchString(value) }

// SyntaxError identifies one recoverable note-language error using UTF-16 offsets.
type SyntaxError struct {
	Position int    `json:"position"`
	Length   int    `json:"length"`
	Message  string `json:"message"`
}

// Link is one syntactically valid custom link extracted from a note version.
type Link struct {
	TargetType  string  `json:"target_type"`
	RawTarget   string  `json:"raw_target"`
	DisplayText *string `json:"display_text,omitempty"`
	Position    int     `json:"position"`
	Length      int     `json:"length"`
}

// Block is a normalized note block used by cross-language conformance fixtures.
type Block struct {
	Kind  string     `json:"kind"`
	Level int        `json:"level,omitempty"`
	Text  string     `json:"text,omitempty"`
	Items []string   `json:"items,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
}

// Document is the normalized parser result. Any syntax error makes the document unsaveable.
type Document struct {
	Blocks []Block       `json:"blocks"`
	Links  []Link        `json:"links"`
	Errors []SyntaxError `json:"errors"`
}

// Parse returns all recognized blocks, links, and recoverable syntax errors.
func Parse(body string) Document {
	doc := Document{Blocks: make([]Block, 0), Links: make([]Link, 0), Errors: make([]SyntaxError, 0)}
	if !utf8.ValidString(body) {
		doc.Errors = append(doc.Errors, SyntaxError{Position: 0, Length: utf16Length(body), Message: "note body must be valid UTF-8"})
		return doc
	}
	if len(body) > MaxBodyBytes {
		doc.Errors = append(doc.Errors, SyntaxError{Position: 0, Length: utf16Length(body), Message: fmt.Sprintf("note body exceeds %d bytes", MaxBodyBytes)})
		return doc
	}
	doc.Blocks, doc.Errors = parseBlocks(body, doc.Errors)
	doc.Links, doc.Errors = parseLinks(body, doc.Errors)
	return doc
}

// parseBlocks recognizes the bounded block grammar while suppressing links inside code fences.
func parseBlocks(body string, problems []SyntaxError) ([]Block, []SyntaxError) {
	type line struct {
		text  string
		start int
	}
	raw := strings.Split(body, "\n")
	lines := make([]line, len(raw))
	offset := 0
	for i, text := range raw {
		text = strings.TrimSuffix(text, "\r")
		lines[i] = line{text: text, start: offset}
		offset += len(raw[i]) + 1
	}
	blocks := make([]Block, 0)
	for i := 0; i < len(lines); {
		current := lines[i]
		if current.text == "" {
			i++
			continue
		}
		if current.text == "```" {
			start := current.start
			i++
			contents := make([]string, 0)
			for i < len(lines) && lines[i].text != "```" {
				contents = append(contents, lines[i].text)
				i++
			}
			if i == len(lines) {
				problems = append(problems, SyntaxError{Position: utf16Offset(body, start), Length: 3, Message: "unclosed code fence"})
				blocks = append(blocks, Block{Kind: "code", Text: strings.Join(contents, "\n")})
				continue
			}
			i++
			blocks = append(blocks, Block{Kind: "code", Text: strings.Join(contents, "\n")})
			continue
		}
		if level, text, ok := heading(current.text); ok {
			blocks = append(blocks, Block{Kind: "heading", Level: level, Text: text})
			i++
			continue
		}
		if strings.HasPrefix(current.text, "> ") {
			items := make([]string, 0)
			for i < len(lines) && strings.HasPrefix(lines[i].text, "> ") {
				items = append(items, strings.TrimPrefix(lines[i].text, "> "))
				i++
			}
			blocks = append(blocks, Block{Kind: "blockquote", Text: strings.Join(items, "\n")})
			continue
		}
		if kind, text, ok := listItem(current.text); ok {
			items := []string{text}
			i++
			for i < len(lines) {
				nextKind, nextText, nextOK := listItem(lines[i].text)
				if !nextOK || nextKind != kind {
					break
				}
				items = append(items, nextText)
				i++
			}
			blocks = append(blocks, Block{Kind: kind, Items: items})
			continue
		}
		if hasUnescapedPipe(current.text) && i+1 < len(lines) && hasUnescapedPipe(lines[i+1].text) {
			header := splitTableRow(current.text)
			delimiter := splitTableRow(lines[i+1].text)
			valid := len(header) >= 2 && len(delimiter) == len(header)
			for _, cell := range delimiter {
				valid = valid && regexp.MustCompile(`^-+$`).MatchString(strings.TrimSpace(cell))
			}
			if !valid || i+2 >= len(lines) || lines[i+2].text == "" || len(splitTableRow(lines[i+2].text)) != len(header) {
				problems = append(problems, SyntaxError{Position: utf16Offset(body, current.start), Length: utf16Length(current.text), Message: "malformed table"})
				blocks = append(blocks, Block{Kind: "paragraph", Text: current.text})
				i++
				continue
			}
			rows := [][]string{header}
			i += 2
			for i < len(lines) && lines[i].text != "" && hasUnescapedPipe(lines[i].text) {
				row := splitTableRow(lines[i].text)
				if len(row) != len(header) {
					problems = append(problems, SyntaxError{Position: utf16Offset(body, lines[i].start), Length: utf16Length(lines[i].text), Message: "table row has the wrong number of cells"})
					break
				}
				rows = append(rows, row)
				i++
			}
			blocks = append(blocks, Block{Kind: "table", Rows: rows})
			continue
		}
		paragraph := []string{current.text}
		i++
		for i < len(lines) && lines[i].text != "" && lines[i].text != "```" {
			if _, _, ok := heading(lines[i].text); ok {
				break
			}
			if strings.HasPrefix(lines[i].text, "> ") {
				break
			}
			if _, _, ok := listItem(lines[i].text); ok {
				break
			}
			paragraph = append(paragraph, lines[i].text)
			i++
		}
		blocks = append(blocks, Block{Kind: "paragraph", Text: strings.Join(paragraph, "\n")})
	}
	return blocks, problems
}

// heading recognizes project headings from level one through four.
func heading(line string) (int, string, bool) {
	for level := 4; level >= 1; level-- {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(line, prefix) {
			return level, strings.TrimPrefix(line, prefix), true
		}
	}
	return 0, "", false
}

// listItem recognizes bullet and deliberately simple ordered-list lines.
func listItem(line string) (string, string, bool) {
	if strings.HasPrefix(line, "- ") {
		return "unordered_list", strings.TrimPrefix(line, "- "), true
	}
	if strings.HasPrefix(line, "1. ") {
		return "ordered_list", strings.TrimPrefix(line, "1. "), true
	}
	return "", "", false
}

// hasUnescapedPipe reports whether a line may begin the simple table grammar.
func hasUnescapedPipe(line string) bool {
	escaped := false
	for _, r := range line {
		if r == '|' && !escaped {
			return true
		}
		if r == '\\' && !escaped {
			escaped = true
		} else {
			escaped = false
		}
	}
	return false
}

// splitTableRow returns safely unescaped cells or nil for a malformed row.
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range line {
		if escaped {
			if r == '|' || r == '\\' {
				cell.WriteRune(r)
			} else {
				cell.WriteRune('\\')
				cell.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '|' {
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
			continue
		}
		cell.WriteRune(r)
	}
	if escaped {
		cell.WriteRune('\\')
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	return cells
}

// parseLinks extracts custom links outside fenced code while retaining UTF-16 positions.
func parseLinks(body string, problems []SyntaxError) ([]Link, []SyntaxError) {
	links := make([]Link, 0)
	inFence := false
	lineStart := 0
	for lineStart <= len(body) {
		lineEnd := strings.IndexByte(body[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(body)
		} else {
			lineEnd += lineStart
		}
		line := strings.TrimSuffix(body[lineStart:lineEnd], "\r")
		if line == "```" {
			inFence = !inFence
		} else if !inFence {
			var lineLinks []Link
			lineLinks, problems = parseLineLinks(body, line, lineStart, problems)
			links = append(links, lineLinks...)
		}
		if lineEnd == len(body) {
			break
		}
		lineStart = lineEnd + 1
	}
	return links, problems
}

// parseLineLinks extracts and validates each custom link from one source line.
func parseLineLinks(body, line string, base int, problems []SyntaxError) ([]Link, []SyntaxError) {
	links := make([]Link, 0)
	for cursor := 0; cursor < len(line); {
		open := strings.Index(line[cursor:], "[[")
		if open < 0 {
			break
		}
		open += cursor
		close := findLinkEnd(line, open+2)
		if close < 0 {
			problems = append(problems, SyntaxError{Position: utf16Offset(body, base+open), Length: utf16Length(line[open:]), Message: "unclosed custom link"})
			break
		}
		whole := line[open : close+2]
		inside := line[open+2 : close]
		link, message := decodeLink(inside)
		position := utf16Offset(body, base+open)
		length := utf16Length(whole)
		if message != "" {
			problems = append(problems, SyntaxError{Position: position, Length: length, Message: message})
		} else {
			link.Position = position
			link.Length = length
			links = append(links, link)
		}
		cursor = close + 2
	}
	return links, problems
}

// findLinkEnd locates the next unescaped closing delimiter.
func findLinkEnd(line string, start int) int {
	escaped := false
	for i := start; i+1 < len(line); i++ {
		if line[i] == '\\' && !escaped {
			escaped = true
			continue
		}
		if line[i] == ']' && line[i+1] == ']' && !escaped {
			return i
		}
		escaped = false
	}
	return -1
}

// decodeLink converts one custom-link payload into canonical persisted identity.
func decodeLink(input string) (Link, string) {
	parts, err := splitEscaped(input)
	if err != nil {
		return Link{}, err.Error()
	}
	if len(parts) > 2 {
		return Link{}, "custom link contains more than one display separator"
	}
	scheme, target, found := strings.Cut(parts[0], ":")
	if !found || target == "" {
		return Link{}, "custom link target is empty"
	}
	if len(target) > MaxTargetBytes {
		return Link{}, fmt.Sprintf("custom link target exceeds %d bytes", MaxTargetBytes)
	}
	var display *string
	if len(parts) == 2 {
		if len(parts[1]) > MaxDisplayBytes {
			return Link{}, fmt.Sprintf("custom link display text exceeds %d bytes", MaxDisplayBytes)
		}
		text := parts[1]
		display = &text
	}
	link := Link{RawTarget: target, DisplayText: display}
	switch scheme {
	case "note":
		id, err := strconv.ParseInt(target, 10, 64)
		if err != nil || id < 1 {
			return Link{}, "note target must be a positive integer"
		}
		link.TargetType = "note"
	case "article":
		target = normalizeDOI(target)
		if target == "" {
			return Link{}, "article target must be a DOI"
		}
		link.TargetType, link.RawTarget = "article", target
	case "pdf":
		pageText, ok := strings.CutPrefix(target, "page=")
		page, err := strconv.Atoi(pageText)
		if !ok || err != nil || page < 1 {
			return Link{}, "PDF target must use page=<positive integer>"
		}
		link.TargetType, link.RawTarget = "pdf_page", strconv.Itoa(page)
	case "anchor":
		if !anchorPattern.MatchString(target) {
			return Link{}, "anchor target has an invalid identifier"
		}
		link.TargetType = "anchor"
	case "ext":
		if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" {
			if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return Link{}, "external URL must use absolute http or https"
			}
		}
		link.TargetType = "ext"
	default:
		return Link{}, fmt.Sprintf("unknown custom link scheme %q", scheme)
	}
	return link, ""
}

// splitEscaped separates link fields while preserving supported escaped delimiters.
func splitEscaped(input string) ([]string, error) {
	parts := []string{""}
	escaped := false
	for _, r := range input {
		if escaped {
			if r != ']' && r != '|' && r != '\\' {
				return nil, fmt.Errorf("malformed custom link escape")
			}
			parts[len(parts)-1] += string(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '|' {
			parts = append(parts, "")
			continue
		}
		parts[len(parts)-1] += string(r)
	}
	if escaped {
		return nil, fmt.Errorf("malformed custom link escape")
	}
	return parts, nil
}

// normalizeDOI canonicalizes article-link DOI targets without database access.
func normalizeDOI(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, prefix := range []string{"https://doi.org/", "http://doi.org/", "doi:"} {
		value = strings.TrimPrefix(value, prefix)
	}
	return strings.TrimSpace(value)
}

// utf16Offset converts a UTF-8 byte position into a browser-compatible code-unit offset.
func utf16Offset(body string, byteOffset int) int {
	if byteOffset < 0 {
		return 0
	}
	if byteOffset > len(body) {
		byteOffset = len(body)
	}
	return utf16Length(body[:byteOffset])
}

// utf16Length returns the browser-compatible code-unit length of a string.
func utf16Length(value string) int { return len(utf16.Encode([]rune(value))) }
