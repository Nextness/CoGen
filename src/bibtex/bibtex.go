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

// EntryType identifies the supported BibTeX entry categories.
type EntryType int

const (
	EntryArticle EntryType = iota
	EntryBook
	EntryInProceedings
	EntryMisc
	EntryUnknown
)

// String returns the receiver's textual representation.
func (e EntryType) String() string {
	switch e {
	case EntryArticle:
		return "article"
	case EntryBook:
		return "book"
	case EntryInProceedings:
		return "inproceedings"
	case EntryMisc:
		return "misc"
	default:
		return "unknown"
	}
}

// entryTypeFromString maps a case-insensitive BibTeX entry name to its category.
func entryTypeFromString(s string) EntryType {
	switch strings.ToLower(s) {
	case "article":
		return EntryArticle
	case "book":
		return EntryBook
	case "inproceedings":
		return EntryInProceedings
	case "misc":
		return EntryMisc
	default:
		return EntryUnknown
	}
}

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

// readBracedContent reads content inside balanced braces.
// When stripBraces is true, the outermost braces are not included in the result.
func readBracedContent(data string, i, n int, stripBraces bool) (string, int) {
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
	return string(parts), i
}

// isIdentChar reports whether a byte is accepted inside a BibTeX identifier.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || strings.ContainsRune("-_:./+", rune(c))
}

// tokenize converts BibTeX source text into the constrained parser token stream.
func tokenize(data string, stripBraces bool) []token {
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
				content, newI := readBracedContent(data, i, n, stripBraces)
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
			i++
			var content []byte
			for i < n {
				if data[i] == '\\' && i+1 < n {
					content = append(content, data[i], data[i+1])
					i += 2
					continue
				}
				if data[i] == '"' {
					i++
					break
				}
				content = append(content, data[i])
				i++
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
			i++
		}
	}
	tokens = append(tokens, token{typ: tokEOF})
	return tokens
}

// Parse parses raw BibTeX data into a Library.
// Only "article" entries are retained. Duplicate citation keys get a numeric suffix.
// The source parameter is stored in the "article_source" field of each entry.
func (p *Parser) Parse(data, source string, stripBraces bool) (Library, error) {
	toks := tokenize(data, stripBraces)
	pos := 0
	n := len(toks)
	entries := make(Library)
	dupCounts := make(map[string]int)

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
		entryType := entryTypeFromString(typeTok.value)

		if peek().typ != tokLBRACE {
			continue
		}
		advance()

		keyTok, err := expect(tokIDENTIFIER)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(keyTok.value)

		if peek().typ == tokCOMMA {
			advance()
		}

		fields := make(Entry)
		fields["entry_type"] = entryType.String()

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

		if peek().typ == tokRBRACE {
			advance()
		}

		if entryType != EntryArticle {
			continue
		}

		fields["article_source"] = source
		if _, exists := entries[key]; exists {
			dupKey := fmt.Sprintf("%s_%d", key, dupCounts[key])
			dupCounts[key]++
			entries[dupKey] = fields
		} else {
			entries[key] = fields
		}
	}

	p.log.Info("Parsed BibTeX entries", "count", len(entries), "source", source)
	return entries, nil
}
