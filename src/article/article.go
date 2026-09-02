// Package article defines the parsed article model, author and reference
// types, DOI-based deduplication, and text-sanitisation helpers used
// throughout the pipeline.
package article

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Author stores one source-observed or enriched author occurrence before persistence.
type Author struct {
	CitationName   string `json:"citation_name"`
	Orcid          string `json:"orcid"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	NormalizedName string `json:"normalized_name"`
	Affiliation    string `json:"affiliation"`
}

// Reference stores one ordered bibliographic reference parsed or enriched for an article.
type Reference struct {
	Raw    string `json:"raw"`
	DOI    string `json:"doi"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
	Source string `json:"source"`
}

// Article is the pipeline's mutable in-memory work record before immutable revision persistence.
type Article struct {
	DOI                string      `json:"doi"`
	Title              string      `json:"title"`
	Abstract           string      `json:"abstract"`
	Year               int         `json:"year"`
	Keywords           []string    `json:"keywords"`
	KeywordsAdditional []string    `json:"keywords_additional"`
	Journal            string      `json:"journal"`
	Publisher          string      `json:"publisher"`
	Source             string      `json:"source"`
	CitationCount      int         `json:"citation_count"`
	CitedReferences    []Reference `json:"cited_references"`
	Authors            []Author    `json:"authors"`
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)
var doiInlineRe = regexp.MustCompile(`(?:doi\.org/)?(10\.\d{4,}/[^\s,;]+)`)

var sanitizeReplace = map[string]string{
	"\u201c": `"`,
	"\u201d": `"`,
	"\u2018": `'`,
	"\u2019": `'`,
	"\u2013": "--",
	"\u2014": "---",
	"\u00a0": " ",
	"\u00a9": "(c)",
	"\\_":    "_",
}

// SanitizeText cleans problematic characters from raw export text.
// Removes HTML tags, replaces smart quotes/dashes with ASCII equivalents,
// strips BOM.
func SanitizeText(data string) string {
	data = htmlTagRe.ReplaceAllString(data, "")
	for old, new := range sanitizeReplace {
		data = strings.ReplaceAll(data, old, new)
	}
	data = strings.TrimLeft(data, "\ufeff")
	return data
}

// SplitToList splits a string by separator, returning cleaned non-empty parts.
// When separator is "\n", it splits on newlines directly without stripping newlines first.
func SplitToList(value, separator string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var parts []string
	if separator == "\n" {
		for _, line := range strings.Split(value, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				parts = append(parts, line)
			}
		}
		return parts
	}
	cleaned := strings.ReplaceAll(value, "\n", "")
	for _, p := range strings.Split(cleaned, separator) {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// parseInteger parses one export value after normalizing its whitespace.
func parseInteger(value string) (int, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", ""))
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return n, true
}

// ParseInt parses a string as an integer. Returns 0 on failure.
func ParseInt(value string) int {
	n, ok := parseInteger(value)
	if !ok {
		return 0
	}
	return n
}

// ParseOptionalInt parses a string as an integer. Returns nil pointer on failure.
func ParseOptionalInt(value string) *int {
	n, ok := parseInteger(value)
	if !ok {
		return nil
	}
	return &n
}

// ExtractDOI returns the first DOI found in text, or empty string.
func ExtractDOI(text string) string {
	m := doiInlineRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	// Strip trailing punctuation often glued to the end
	return strings.ToLower(strings.TrimRight(m[1], ".,;:)]}"))
}

// ParseAuthors parses semicolon/newline-separated author and affiliation strings.
// Affiliations are matched by position (same index as authors).
// BibTeX uses " and " as separator - this is tried as a fallback when
// semicolons don't produce a useful split.
func ParseAuthors(authorsStr, affiliationsStr string) []Author {
	authorsStr = strings.TrimSpace(authorsStr)
	if authorsStr == "" {
		return nil
	}

	authorList := SplitToList(authorsStr, ";")

	// BibTeX " and " fallback
	if (len(authorList) == 0 || (len(authorList) == 1 && strings.Contains(authorList[0], " and "))) &&
		strings.Contains(authorsStr, " and ") {
		authorList = SplitToList(authorsStr, " and ")
	}

	if len(authorList) == 0 {
		return nil
	}

	affSeparator := "\n"
	if !strings.Contains(affiliationsStr, "\n") && strings.Contains(affiliationsStr, ";") {
		affSeparator = ";"
	}
	affList := SplitToList(affiliationsStr, affSeparator)

	result := make([]Author, len(authorList))
	for i, name := range authorList {
		aff := ""
		if i < len(affList) {
			aff = affList[i]
		}
		result[i] = Author{
			CitationName: strings.TrimSpace(name),
			Affiliation:  strings.TrimSpace(aff),
		}
	}
	return result
}

// ParseReferences parses a newline or semicolon-separated reference list with
// DOI extraction. CSV exports generally use semicolons while BibTeX exports use
// one reference per line.
func ParseReferences(refsStr string) []Reference {
	separator := "\n"
	if !strings.Contains(refsStr, "\n") && strings.Contains(refsStr, ";") {
		separator = ";"
	}
	lines := SplitToList(refsStr, separator)
	if len(lines) == 0 {
		return nil
	}
	result := make([]Reference, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		result[i] = Reference{
			Raw:    line,
			DOI:    ExtractDOI(line),
			Title:  "",
			Author: "",
			Year:   0,
			Source: "",
		}
	}
	return result
}

// RenameFields renames keys in m according to patches (old -> new), in-place.
func RenameFields(m map[string]string, patches map[string]string) {
	// Read every value before changing the map. Without this snapshot, chained
	// renames such as a->b and b->c depend on randomized Go map iteration and
	// can overwrite the original b value.
	type rename struct {
		old, new, value string
	}
	keys := make([]string, 0, len(patches))
	for old := range patches {
		keys = append(keys, old)
	}
	sort.Strings(keys)
	var renames []rename
	for _, old := range keys {
		if value, ok := m[old]; ok {
			renames = append(renames, rename{old: old, new: patches[old], value: value})
		}
	}
	for _, item := range renames {
		delete(m, item.old)
	}
	for _, item := range renames {
		m[item.new] = item.value
	}
}

// KeepFields deletes keys not in the keep set, in-place.
func KeepFields(m map[string]string, keep []string) {
	keepSet := make(map[string]struct{}, len(keep))
	for _, k := range keep {
		keepSet[k] = struct{}{}
	}
	for k := range m {
		if _, ok := keepSet[k]; !ok {
			delete(m, k)
		}
	}
}

// RequiredFieldError is returned when a required field is missing.
type RequiredFieldError struct {
	Missing []string
}

// Error returns the receiver's diagnostic message.
func (e *RequiredFieldError) Error() string {
	return fmt.Sprintf("article missing required fields: %s", strings.Join(e.Missing, ", "))
}

// CheckRequired returns the list of missing required fields.
func CheckRequired(a *Article) []string {
	if a == nil {
		return []string{"article"}
	}
	var missing []string
	if a.DOI == "" {
		missing = append(missing, "doi")
	}
	if a.Title == "" {
		missing = append(missing, "title")
	}
	if a.Year == 0 {
		missing = append(missing, "year")
	}
	return missing
}

// NewFromMap builds an Article from a canonical-field map[string]string.
// All text values are sanitised automatically.
// Returns nil + *RequiredFieldError if required fields are missing.
func NewFromMap(entry map[string]string, source string) (*Article, error) {
	doi := strings.TrimSpace(SanitizeText(entry["doi"]))
	if extracted := ExtractDOI(doi); extracted != "" {
		doi = extracted
	}
	a := &Article{
		DOI:           strings.ToLower(doi),
		Title:         SanitizeText(entry["title"]),
		Abstract:      SanitizeText(entry["abstract"]),
		Year:          ParseInt(entry["year"]),
		Journal:       SanitizeText(entry["journal"]),
		Publisher:     SanitizeText(entry["publisher"]),
		Source:        source,
		CitationCount: 0,
	}

	if c := ParseOptionalInt(entry["citation_count"]); c != nil {
		a.CitationCount = *c
	}

	a.Keywords = lowerAll(SplitToList(entry["keywords"], ";"))
	a.KeywordsAdditional = lowerAll(SplitToList(entry["keywords_additional"], ";"))
	a.CitedReferences = ParseReferences(entry["cited_references"])
	a.Authors = ParseAuthors(entry["authors"], entry["affiliations"])

	if missing := CheckRequired(a); len(missing) > 0 {
		return nil, &RequiredFieldError{Missing: missing}
	}
	return a, nil
}

// ArticleToMap serialises an Article to a plain map for JSON output.
func ArticleToMap(a *Article) map[string]any {
	authors := make([]map[string]any, len(a.Authors))
	for i, au := range a.Authors {
		authors[i] = map[string]any{
			"citation_name":   au.CitationName,
			"orcid":           au.Orcid,
			"first_name":      au.FirstName,
			"last_name":       au.LastName,
			"normalized_name": au.NormalizedName,
			"affiliation":     au.Affiliation,
		}
	}
	refs := make([]map[string]any, len(a.CitedReferences))
	for i, r := range a.CitedReferences {
		refs[i] = map[string]any{
			"raw":    r.Raw,
			"doi":    r.DOI,
			"title":  r.Title,
			"author": r.Author,
			"year":   r.Year,
			"source": r.Source,
		}
	}
	return map[string]any{
		"doi":                 a.DOI,
		"title":               a.Title,
		"abstract":            a.Abstract,
		"year":                a.Year,
		"keywords":            a.Keywords,
		"keywords_additional": a.KeywordsAdditional,
		"journal":             a.Journal,
		"publisher":           a.Publisher,
		"source":              a.Source,
		"citation_count":      a.CitationCount,
		"cited_references":    refs,
		"authors":             authors,
	}
}

// MergeBySource merges articles from multiple sources, deduplicating by DOI.
// Returns (unique, duplicates). Articles without DOI are always kept.
func MergeBySource(articlesBySource map[string][]*Article) (unique, duplicates []*Article) {
	seenDOIs := make(map[string]bool)

	// Sources are processed in the order provided in the config, but
	// map iteration is random. We sort source names to get consistent
	// ordering.
	// Ordering doesn't matter at this stage. We don't care if something is duplicated
	// from one source or another, as long as we remove duplicates and
	// keep track of where they were dupicated from.
	for _, sourceName := range sortedKeys(articlesBySource) {
		for _, article := range articlesBySource[sourceName] {
			if article.DOI == "" {
				unique = append(unique, article)
				continue
			}
			doiLower := strings.ToLower(article.DOI)
			if seenDOIs[doiLower] {
				duplicates = append(duplicates, article)
			} else {
				seenDOIs[doiLower] = true
				unique = append(unique, article)
			}
		}
	}
	return unique, duplicates
}

// lowerAll returns lowercase copies of all input strings.
func lowerAll(ss []string) []string {
	for i, s := range ss {
		ss[i] = strings.ToLower(s)
	}
	return ss
}

// sortedKeys returns keys in deterministic order.
func sortedKeys(m map[string][]*Article) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
