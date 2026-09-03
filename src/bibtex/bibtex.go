// Package bibtex implements a deliberately limited, article-only BibTeX
// parser. It retains only @article entries and maps citation keys to
// field-name/value pairs.
package bibtex

import (
	"analysis/logging"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Entry is a BibTeX entry represented as a map of field names to values.
type Entry map[string]string

// Library is a collection of BibTeX entries keyed by citation key.
type Library map[string]Entry

// Parser parses BibTeX data. Use NewParser to create one.
type Parser struct {
	log *slog.Logger
}

// NewParser creates a Parser that logs to the given logger.
// If log is nil, it uses the process-wide BibTeX component logger.
func NewParser(log *slog.Logger) *Parser {
	if log == nil {
		log = logging.Logger("bibtex")
	}
	return &Parser{log: log}
}

// LoadFile reads and logs a BibTeX input file without parsing it.
//
// Deprecated: workspace ingestion reads source bytes once and calls Parse directly.
// This method remains for external callers that use the public parser API.
func (p *Parser) LoadFile(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", filepath)
	}
	p.log.Info("Loading BibTeX", "file", filepath)
	return string(data), nil
}

// tokenType identifies lexical tokens in the constrained BibTeX grammar.
type tokenType int

const (
	tokAT tokenType = iota
	tokLBRACE
	tokRBRACE
	tokCOMMA
	tokEQUALS
	tokHASH
	tokIDENTIFIER
	tokSTRING
	tokEOF
)

// token carries one BibTeX token category and its optional text value.
type token struct {
	typ   tokenType
	value string
}

// lexicalError identifies malformed constrained BibTeX input by byte position.
type lexicalError struct {
	position int
	message  string
}

// Error returns the lexical diagnostic with its input byte position.
func (e *lexicalError) Error() string {
	return fmt.Sprintf("BibTeX lexical error at byte %d: %s", e.position, e.message)
}

// readBracedContent reads content inside balanced braces.
// When stripBraces is true, the outermost braces are not included in the result.
func readBracedContent(data string, i, n int, stripBraces bool) (string, int, error) {
	depth := 1
	var parts []byte
	for i < n && depth > 0 {
		if i+1 < n && data[i] == '\\' && (data[i+1] == '{' || data[i+1] == '}') {
			parts = append(parts, data[i], data[i+1])
			i += 2
			continue
		}
		c := data[i]
		i++
		if c == '{' {
			depth++
			if !stripBraces || depth > 1 {
				parts = append(parts, c)
			}
		} else if c == '}' {
			depth--
			if depth > 0 {
				parts = append(parts, c)
			}
		} else {
			parts = append(parts, c)
		}
	}
	if depth != 0 {
		return "", i, &lexicalError{position: i, message: "unterminated braced value"}
	}
	return string(parts), i, nil
}

// isIdentChar reports whether a byte is accepted inside a BibTeX identifier.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || strings.ContainsRune("-_:./+", rune(c))
}

// tokenize converts BibTeX source text into the constrained parser token stream.
func tokenize(data string, stripBraces bool) ([]token, error) {
	n := len(data)
	i := 0
	var tokens []token
	afterEquals := false

	for i < n {
		c := data[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if c == '@' {
			tokens = append(tokens, token{typ: tokAT})
			i++
			afterEquals = false
		} else if c == '{' {
			if afterEquals {
				i++
				content, newI, err := readBracedContent(data, i, n, stripBraces)
				if err != nil {
					return nil, err
				}
				i = newI
				tokens = append(tokens, token{typ: tokSTRING, value: content})
			} else {
				tokens = append(tokens, token{typ: tokLBRACE})
				i++
			}
			afterEquals = false
		} else if c == '}' {
			tokens = append(tokens, token{typ: tokRBRACE})
			i++
			afterEquals = false
		} else if c == ',' {
			tokens = append(tokens, token{typ: tokCOMMA})
			i++
			afterEquals = false
		} else if c == '=' {
			tokens = append(tokens, token{typ: tokEQUALS})
			i++
			afterEquals = true
		} else if c == '#' {
			tokens = append(tokens, token{typ: tokHASH})
			i++
			afterEquals = true
		} else if c == '"' && afterEquals {
			start := i
			i++
			var content []byte
			terminated := false
			for i < n {
				if data[i] == '\\' && i+1 < n {
					content = append(content, data[i], data[i+1])
					i += 2
					continue
				}
				if data[i] == '"' {
					i++
					terminated = true
					break
				}
				content = append(content, data[i])
				i++
			}
			if !terminated {
				return nil, &lexicalError{position: start, message: "unterminated quoted value"}
			}
			tokens = append(tokens, token{typ: tokSTRING, value: string(content)})
			afterEquals = false
		} else if isIdentChar(c) {
			start := i
			for i < n && isIdentChar(data[i]) {
				i++
			}
			tokens = append(tokens, token{typ: tokIDENTIFIER, value: data[start:i]})
			afterEquals = false
		} else {
			return nil, &lexicalError{position: i, message: fmt.Sprintf("unsupported character %q", c)}
		}
	}
	tokens = append(tokens, token{typ: tokEOF})
	return tokens, nil
}

// Parse parses raw BibTeX data into a Library.
// Only "article" entries are retained. Duplicate citation keys get a numeric suffix.
// The source parameter is stored in the "article_source" field of each entry.
// A leading UTF-8 byte order mark is stripped because exporters commonly prefix it.
func (p *Parser) Parse(data, source string, stripBraces bool) (Library, error) {
	data = strings.TrimPrefix(data, "\ufeff")
	toks, err := tokenize(data, stripBraces)
	if err != nil {
		return nil, err
	}
	pos := 0
	n := len(toks)
	entries := make(Library)

	peek := func() token {
		if pos < n {
			return toks[pos]
		}
		return token{typ: tokEOF}
	}

	advance := func() token {
		tok := toks[pos]
		pos++
		return tok
	}

	expect := func(expected tokenType) (token, error) {
		tok := peek()
		if tok.typ != expected {
			return token{}, fmt.Errorf("expected token type %d, got %d at position %d", expected, tok.typ, pos)
		}
		return advance(), nil
	}

	for pos < n {
		if peek().typ != tokAT {
			pos++
			continue
		}
		advance() // consume @
		typeTok, err := expect(tokIDENTIFIER)
		if err != nil {
			return nil, err
		}
		isArticle := strings.EqualFold(typeTok.value, "article")
		if _, err := expect(tokLBRACE); err != nil {
			return nil, fmt.Errorf("open %s entry: %w", typeTok.value, err)
		}

		keyTok, err := expect(tokIDENTIFIER)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(keyTok.value)

		if peek().typ == tokCOMMA {
			advance()
		}

		fields := make(Entry)
		fields["entry_type"] = "article"

		for peek().typ == tokIDENTIFIER {
			nameTok := advance()
			fieldName := strings.ToLower(nameTok.value)
			if peek().typ != tokEQUALS {
				fields[fieldName] = ""
				if peek().typ == tokCOMMA {
					advance()
				}
				continue
			}
			advance() // consume =
			var value strings.Builder
			if peek().typ == tokSTRING || peek().typ == tokIDENTIFIER {
				value.WriteString(advance().value)
			}
			for peek().typ == tokHASH {
				advance()
				if peek().typ == tokSTRING || peek().typ == tokIDENTIFIER {
					value.WriteString(advance().value)
				}
			}
			fields[fieldName] = value.String()
			if peek().typ == tokCOMMA {
				advance()
			}
		}

		if _, err := expect(tokRBRACE); err != nil {
			return nil, fmt.Errorf("close entry %q: %w", key, err)
		}

		if !isArticle {
			continue
		}

		fields["article_source"] = source
		storedKey := key
		for suffix := 0; ; suffix++ {
			if _, exists := entries[storedKey]; !exists {
				break
			}
			storedKey = fmt.Sprintf("%s_%d", key, suffix)
		}
		entries[storedKey] = fields
	}

	p.log.Info("Parsed BibTeX entries", "count", len(entries), "source", source)
	return entries, nil
}
