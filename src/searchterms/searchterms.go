// Package searchterms extracts search terms from source query strings and
// matches them against article fields. It has no database or HTTP dependencies.
package searchterms

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kljensen/snowball"
)

// Term is one distinct search term and the sources that declared it.
type Term struct {
	// Text is the original term spelling, including any wildcard characters.
	Text string
	// Sources lists the source names whose queries declared this term.
	Sources []string
}

// Parse extracts the ordered distinct search terms from one query string.
// Quoted phrases become single terms; operators, field prefixes, and quoted
// field labels are skipped. Terms are deduplicated case-insensitively and the
// first occurrence's spelling and position win.
func Parse(query string) []string {
	return dedupe(parseTerms(query))
}

// ParseSources combines per-source queries into one deduplicated term list
// with source attribution. Source names are processed in sorted order so the
// result is deterministic; a failing or empty query contributes no terms and
// never returns an error.
func ParseSources(queries map[string]string) []Term {
	names := make([]string, 0, len(queries))
	for name := range queries {
		names = append(names, name)
	}
	sort.Strings(names)
	index := make(map[string]int)
	var result []Term
	for _, name := range names {
		for _, text := range Parse(queries[name]) {
			key := strings.ToLower(text)
			if i, exists := index[key]; exists {
				result[i].Sources = append(result[i].Sources, name)
				continue
			}
			index[key] = len(result)
			result = append(result, Term{Text: text, Sources: []string{name}})
		}
	}
	return result
}

// Match reports whether term matches text with case-insensitive whole-word
// semantics after stemming every word in both the term and the text. A
// trailing * is a prefix wildcard matching zero or more word characters; a
// leading * is a suffix wildcard; * at both ends is a substring match. Terms
// without wildcards must match as a whole word or phrase.
func Match(text, term string) bool {
	if term == "" {
		return false
	}
	return matchStemmed(stemText(text), stemTerm(term))
}

// MatchFields returns the terms that match each of the four article fields in
// declaration order title, abstract, keywords, and keywords_plus. Keywords and
// keywords plus are matched per element: a term matches the field when it
// matches any single element, and phrases never span elements. Both the terms
// and the field values are stemmed before matching so that inflected forms
// (for example plural "Notations" against singular "Notation") match.
func MatchFields(title, abstract string, keywords, keywordsPlus []string, terms []Term) map[string][]string {
	stemmedTitle := stemText(title)
	stemmedAbstract := stemText(abstract)
	stemmedKeywords := stemEach(keywords)
	stemmedKeywordsPlus := stemEach(keywordsPlus)
	result := map[string][]string{
		"title":         {},
		"abstract":      {},
		"keywords":      {},
		"keywords_plus": {},
	}
	for _, term := range terms {
		stemmedTerm := stemTerm(term.Text)
		if matchStemmed(stemmedTitle, stemmedTerm) {
			result["title"] = append(result["title"], term.Text)
		}
		if matchStemmed(stemmedAbstract, stemmedTerm) {
			result["abstract"] = append(result["abstract"], term.Text)
		}
		if matchesAnyStemmed(stemmedKeywords, stemmedTerm) {
			result["keywords"] = append(result["keywords"], term.Text)
		}
		if matchesAnyStemmed(stemmedKeywordsPlus, stemmedTerm) {
			result["keywords_plus"] = append(result["keywords_plus"], term.Text)
		}
	}
	return result
}

// matchesAnyStemmed reports whether a stemmed term matches any single stemmed
// keyword element.
func matchesAnyStemmed(elements []string, stemmedTerm string) bool {
	for _, element := range elements {
		if matchStemmed(element, stemmedTerm) {
			return true
		}
	}
	return false
}

// wordRunRe matches a run of ASCII word characters for in-place stemming.
var wordRunRe = regexp.MustCompile(`\w+`)

// stemWord stems a single word with the English snowball stemmer, lowercasing
// it. Words the stemmer cannot process are returned unchanged.
func stemWord(word string) string {
	stemmed, err := snowball.Stem(word, "english", true)
	if err != nil {
		return word
	}
	return stemmed
}

// stemText stems every word run in text in place, preserving all non-word
// characters so that word boundaries and phrase structure are retained.
func stemText(text string) string {
	return wordRunRe.ReplaceAllStringFunc(text, stemWord)
}

// stemEach stems every element of a keyword list.
func stemEach(elements []string) []string {
	stemmed := make([]string, len(elements))
	for i, element := range elements {
		stemmed[i] = stemText(element)
	}
	return stemmed
}

// stemTerm stems every word in a term while preserving wildcard markers.
func stemTerm(term string) string {
	parts := strings.Split(term, "*")
	for i, part := range parts {
		parts[i] = stemText(part)
	}
	return strings.Join(parts, "*")
}

// matchStemmed reports whether a stemmed term matches stemmed text using the
// same whole-word and wildcard semantics as compileTerm.
func matchStemmed(stemmedText, stemmedTerm string) bool {
	if stemmedTerm == "" {
		return false
	}
	re, err := compileTerm(stemmedTerm)
	if err != nil {
		return false
	}
	return re.MatchString(stemmedText)
}

// parseTerms scans one query string and returns terms in declaration order
// without deduplication. Scanning is quote-aware: quoted substrings are
// extracted first, then the remaining text is tokenized.
func parseTerms(query string) []string {
	var terms []string
	for i := 0; i < len(query); {
		switch c := query[i]; {
		case c == '"':
			end := strings.IndexByte(query[i+1:], '"')
			if end < 0 {
				// Unterminated quote: treat the remainder as one term.
				if rest := query[i+1:]; rest != "" {
					terms = append(terms, rest)
				}
				return terms
			}
			text := query[i+1 : i+1+end]
			next := i + 1 + end + 1
			// A quoted field label is immediately followed by a colon.
			if next < len(query) && query[next] == ':' {
				i = next + 1
				continue
			}
			if text != "" {
				terms = append(terms, text)
			}
			i = next
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		default:
			start := i
			for i < len(query) && !isStopChar(query[i]) {
				i++
			}
			token := query[start:i]
			if token == "" {
				// A bare stop character such as ), =, or :.
				i++
				continue
			}
			// A field prefix is a token immediately followed by =, (, or :.
			if i < len(query) && (query[i] == '=' || query[i] == '(' || query[i] == ':') {
				i++
				continue
			}
			if isOperator(token) || isPunctuationOnly(token) {
				continue
			}
			terms = append(terms, token)
		}
	}
	return terms
}

// isStopChar reports whether a byte terminates a bare token.
func isStopChar(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' || c == ')' || c == '=' || c == ':'
}

// isOperator reports whether a bare token is a boolean or proximity operator.
func isOperator(token string) bool {
	lower := strings.ToLower(token)
	switch lower {
	case "and", "or", "not", "near", "w", "pre":
		return true
	}
	if strings.HasPrefix(lower, "near/") || strings.HasPrefix(lower, "w/") {
		digits := strings.TrimPrefix(strings.TrimPrefix(lower, "near/"), "w/")
		if digits != "" {
			for _, r := range digits {
				if r < '0' || r > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}

// isPunctuationOnly reports whether a bare token contains no letter or digit.
func isPunctuationOnly(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// dedupe removes case-insensitive duplicates while preserving first spelling
// and declaration order.
func dedupe(terms []string) []string {
	seen := make(map[string]struct{}, len(terms))
	result := make([]string, 0, len(terms))
	for _, term := range terms {
		key := strings.ToLower(term)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, term)
	}
	return result
}

// compileTerm builds the case-insensitive expression for one term. Wildcards
// become word-character sequences and boundaries are anchored only when the
// term edge is a word character.
func compileTerm(term string) (*regexp.Regexp, error) {
	parts := strings.Split(term, "*")
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = regexp.QuoteMeta(part)
	}
	body := strings.Join(escaped, `\w*`)
	prefix := ""
	if firstRuneIsWord(parts[0]) {
		prefix = `\b`
	}
	suffix := ""
	if lastRuneIsWord(parts[len(parts)-1]) {
		suffix = `\b`
	}
	return regexp.Compile(`(?i)` + prefix + body + suffix)
}

// firstRuneIsWord reports whether the first rune is a word character.
func firstRuneIsWord(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// lastRuneIsWord reports whether the last rune is a word character.
func lastRuneIsWord(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
