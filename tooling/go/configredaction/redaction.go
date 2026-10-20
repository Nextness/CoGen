// Package configredaction removes provable reviewer identities from SOMETHING source and local include trees.
package configredaction

import (
	"analysis/backend/something"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Redact replaces only provable inline reviewer values and fails closed otherwise.
func Redact(source []byte) ([]byte, bool, error) {
	text := string(source)
	type replacement struct{ start, end int }
	replacements := []replacement{}
	for index := 0; index < len(text); {
		next, kind := nextCodeToken(text, index)
		if kind == tokenEnd {
			break
		}
		if kind != tokenIdentifier {
			// Strings and multiline blocks report one position past their end,
			// while punctuation reports its own position. Advance to the start
			// of the next token without consuming a following identifier's
			// first byte.
			if next > index {
				index = next
			} else {
				index = next + 1
			}
			continue
		}
		index = next
		if !hasWordAt(text, index, "reviewer") {
			index++
			continue
		}
		cursor := skipTrivia(text, index+len("reviewer"))
		if cursor >= len(text) || text[cursor] != '=' {
			index += len("reviewer")
			continue
		}
		start := skipTrivia(text, cursor+1)
		if !hasWordAt(text, start, "reviewer_config") {
			return nil, false, fmt.Errorf("reviewer assignment must use an inline reviewer_config value")
		}
		open := skipTrivia(text, start+len("reviewer_config"))
		if open >= len(text) || text[open] != '{' {
			return nil, false, fmt.Errorf("reviewer assignment uses an unsupported macro or reference")
		}
		end, err := matchingBrace(text, open)
		if err != nil {
			return nil, false, err
		}
		if err := validateReviewerLiterals(text[open+1 : end]); err != nil {
			return nil, false, err
		}
		after := skipTrivia(text, end+1)
		if after < len(text) && text[after] != ',' && text[after] != ';' && text[after] != '}' {
			return nil, false, fmt.Errorf("reviewer assignment must be a complete inline literal")
		}
		replacements = append(replacements, replacement{start: start, end: end + 1})
		index = end + 1
	}
	if len(replacements) == 0 {
		return append([]byte(nil), source...), false, nil
	}
	var output strings.Builder
	previous := 0
	for _, item := range replacements {
		output.WriteString(text[previous:item.start])
		output.WriteString(`reviewer_config { username = "", email = "", }`)
		previous = item.end
	}
	output.WriteString(text[previous:])
	return []byte(output.String()), true, nil
}

// validateReviewerLiterals accepts only direct identity strings whose complete source is inside the redacted object.
func validateReviewerLiterals(text string) error {
	for cursor := skipTrivia(text, 0); cursor < len(text); {
		end := cursor
		for end < len(text) && isIdentifierByte(text[end]) {
			end++
		}
		field := text[cursor:end]
		if field != "username" && field != "email" {
			return fmt.Errorf("reviewer fields must be literal username or email assignments")
		}
		cursor = skipTrivia(text, end)
		if cursor >= len(text) || text[cursor] != '=' {
			return fmt.Errorf("reviewer field must use a literal string")
		}
		cursor = skipTrivia(text, cursor+1)
		if cursor >= len(text) || (text[cursor] != '"' && text[cursor] != '\'') {
			return fmt.Errorf("indirect reviewer field values cannot be safely redacted")
		}
		end, err := quotedEnd(text, cursor)
		if err != nil {
			return err
		}
		if strings.ContainsAny(text[cursor+1:end-1], "{}") {
			return fmt.Errorf("interpolated reviewer values cannot be safely redacted")
		}
		cursor = skipTrivia(text, end)
		if cursor == len(text) {
			break
		}
		if text[cursor] != ',' && text[cursor] != ';' {
			return fmt.Errorf("reviewer fields must use complete literal strings")
		}
		cursor = skipTrivia(text, cursor+1)
	}
	return nil
}

// tokenKind distinguishes source identifiers from skipped strings, comments, multiline data, and punctuation.
type tokenKind int

const (
	tokenEnd tokenKind = iota
	tokenIdentifier
	tokenOther
)

// nextCodeToken returns the next executable SOMETHING source position without inspecting data tokens.
func nextCodeToken(text string, start int) (int, tokenKind) {
	index := skipTrivia(text, start)
	if index >= len(text) {
		return index, tokenEnd
	}
	if text[index] == '"' || text[index] == '\'' {
		end, _ := quotedEnd(text, index)
		return end, tokenOther
	}
	if end, ok := somethingMultilineEnd(text, index); ok {
		return end, tokenOther
	}
	if unicode.IsLetter(rune(text[index])) || text[index] == '_' {
		return index, tokenIdentifier
	}
	return index, tokenOther
}

// somethingMultilineEnd skips one SOMETHING multiline literal including its named closing delimiter.
func somethingMultilineEnd(text string, start int) (int, bool) {
	if !strings.HasPrefix(text[start:], "#multiline") || start+len("#multiline") < len(text) && isIdentifierByte(text[start+len("#multiline")]) {
		return 0, false
	}
	headerEnd := strings.IndexByte(text[start:], '\n')
	if headerEnd < 0 {
		return len(text), true
	}
	headerEnd += start
	fields := strings.Fields(text[start:headerEnd])
	if len(fields) < 2 {
		return headerEnd + 1, true
	}
	delimiter := fields[len(fields)-1]
	for lineStart := headerEnd + 1; lineStart <= len(text); {
		lineEnd := strings.IndexByte(text[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += lineStart
		}
		line := strings.TrimSpace(something.StripMultilineComment(text[lineStart:lineEnd]))
		if line == delimiter || line == delimiter+"," || line == delimiter+";" {
			if lineEnd < len(text) {
				return lineEnd + 1, true
			}
			return lineEnd, true
		}
		if lineEnd == len(text) {
			break
		}
		lineStart = lineEnd + 1
	}
	return len(text), true
}

// skipTrivia advances over whitespace and line comments.
func skipTrivia(text string, start int) int {
	for start < len(text) {
		if unicode.IsSpace(rune(text[start])) {
			start++
			continue
		}
		if strings.HasPrefix(text[start:], "//") {
			if newline := strings.IndexByte(text[start:], '\n'); newline >= 0 {
				start += newline + 1
				continue
			}
			return len(text)
		}
		break
	}
	return start
}

// matchingBrace finds an inline reviewer value boundary while ignoring quoted data and comments.
func matchingBrace(text string, open int) (int, error) {
	depth := 0
	for index := open; index < len(text); index++ {
		if strings.HasPrefix(text[index:], "//") {
			newline := strings.IndexByte(text[index:], '\n')
			if newline < 0 {
				return 0, fmt.Errorf("unclosed reviewer value")
			}
			index += newline
			continue
		}
		if text[index] == '"' || text[index] == '\'' {
			end, err := quotedEnd(text, index)
			if err != nil {
				return 0, err
			}
			index = end - 1
			continue
		}
		switch text[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}
	return 0, fmt.Errorf("unclosed reviewer_config value")
}

// quotedEnd returns the position after an escaped single or double quoted token.
func quotedEnd(text string, start int) (int, error) {
	quote := text[start]
	for index := start + 1; index < len(text); index++ {
		if text[index] == '\\' {
			index++
			continue
		}
		if text[index] == quote {
			return index + 1, nil
		}
	}
	return len(text), fmt.Errorf("unclosed quoted value")
}

// hasWordAt matches one identifier without accepting a longer identifier prefix.
func hasWordAt(text string, start int, word string) bool {
	if start < 0 || start+len(word) > len(text) || text[start:start+len(word)] != word {
		return false
	}
	end := start + len(word)
	return (start == 0 || !isIdentifierByte(text[start-1])) && (end == len(text) || !isIdentifierByte(text[end]))
}

// isIdentifierByte reports whether a byte may continue the identifiers used by the sanitizer.
func isIdentifierByte(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

// Copy copies one include tree without permitting lexical or symbolic-link escape.
func Copy(source, destinationRoot string) (string, error) {
	root, err := filepath.Abs(filepath.Dir(source))
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	visited := map[string]bool{}
	var copyOne func(string) (string, error)
	copyOne = func(path string) (string, error) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(root, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("configuration include escapes copied root: %s", path)
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", err
		}
		resolvedRelative, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("configuration include escapes copied root through a symbolic link: %s", path)
		}
		destination := filepath.Join(destinationRoot, "config", relative)
		if visited[resolved] {
			return destination, nil
		}
		visited[resolved] = true
		data, err := os.ReadFile(resolved)
		if err != nil {
			return "", err
		}
		sanitized, _, err := Redact(data)
		if err != nil {
			return "", fmt.Errorf("sanitize configuration %s: %w", absolute, err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(destination, sanitized, 0o644); err != nil {
			return "", err
		}
		includes, err := localIncludes(string(data))
		if err != nil {
			return "", fmt.Errorf("inspect configuration includes in %s: %w", absolute, err)
		}
		for _, include := range includes {
			if _, err := copyOne(filepath.Join(filepath.Dir(absolute), include)); err != nil {
				return "", err
			}
		}
		return destination, nil
	}
	return copyOne(source)
}

// localIncludes extracts literal local include paths outside comments, strings, and multiline data.
func localIncludes(source string) ([]string, error) {
	includes := []string{}
	for offset := 0; offset < len(source); {
		index, kind := nextCodeToken(source, offset)
		if kind == tokenEnd {
			break
		}
		if kind != tokenOther || !strings.HasPrefix(source[index:], "#include") {
			offset = index + 1
			continue
		}
		index = skipTrivia(source, index+len("#include"))
		if index >= len(source) || source[index] != '(' {
			return nil, fmt.Errorf("#include must use a local quoted path")
		}
		index = skipTrivia(source, index+1)
		if index >= len(source) || source[index] != '"' {
			return nil, fmt.Errorf("#include must use a local quoted path")
		}
		end, err := quotedEnd(source, index)
		if err != nil {
			return nil, err
		}
		includes = append(includes, source[index+1:end-1])
		offset = end
	}
	return includes, nil
}
