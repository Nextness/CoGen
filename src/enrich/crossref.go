// crossref.go implements the Crossref API response decoder for work
// metadata enrichment. It extracts fields from the Crossref JSON
// envelope into the generic ArticleEnrichment representation.
package enrich

import (
	"encoding/json"
	"strconv"
	"strings"
)

// extractCrossrefEntry parses the Crossref API response and returns a generic
// map of extracted fields.
func extractCrossrefEntry(body []byte) map[string]any {
	var raw struct {
		Message map[string]any `json:"message"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	if raw.Message == nil {
		return nil
	}
	return raw.Message
}

// DecodeCrossrefResponse converts a raw Crossref work response into the
// workspace enrichment representation. The workspace owns cache policy and
// persistence; this package remains responsible only for decoding.
func DecodeCrossrefResponse(body []byte, doi string) *ArticleEnrichment {
	entry := extractCrossrefEntry(body)
	if entry == nil {
		return nil
	}
	return crossrefEntryToArticle(entry, doi)
}

// crossrefEntryToArticle converts a Crossref API message map to an
// ArticleEnrichment.
func crossrefEntryToArticle(entry map[string]any, doi string) *ArticleEnrichment {
	ae := &ArticleEnrichment{
		DOI: doi,
	}

	// Title
	if titles, ok := entry["title"].([]any); ok && len(titles) > 0 {
		ae.Title, _ = titles[0].(string)
	}

	// Publisher
	if pub, ok := entry["publisher"].(string); ok {
		ae.Publisher = pub
	}

	// Authors
	if authors, ok := entry["author"].([]any); ok {
		ae.Authors = extractCrossrefAuthors(authors)
	}

	// References
	if refs, ok := entry["reference"].([]any); ok {
		ae.References = extractCrossrefReferences(refs)
	}

	return ae
}

// extractCrossrefAuthors parses the Crossref author array.
func extractCrossrefAuthors(authors []any) []EnrichedAuthor {
	var result []EnrichedAuthor
	for _, a := range authors {
		am, ok := a.(map[string]any)
		if !ok {
			continue
		}

		ea := EnrichedAuthor{}

		given, _ := am["given"].(string)
		family, _ := am["family"].(string)
		ea.FirstName = given
		ea.LastName = family
		if given != "" && family != "" {
			ea.CitationName = family + ", " + given
		} else if family != "" {
			ea.CitationName = family
			ea.LastName = family
		} else if name, ok := am["name"].(string); ok {
			ea.CitationName = name
		}
		ea.CitationName = strings.TrimSpace(ea.CitationName)
		if ea.CitationName == "" {
			continue
		}

		// ORCID
		if orcid, ok := am["ORCID"].(string); ok {
			ea.ORCID = strings.TrimPrefix(orcid, "http://orcid.org/")
			ea.ORCID = strings.TrimPrefix(ea.ORCID, "https://orcid.org/")
		}

		// Affiliations
		if affs, ok := am["affiliation"].([]any); ok && len(affs) > 0 {
			if firstAff, ok := affs[0].(map[string]any); ok {
				ea.Affiliation, _ = firstAff["name"].(string)
			}
		}

		result = append(result, ea)
	}
	return result
}

// extractCrossrefReferences parses the Crossref reference array.
func extractCrossrefReferences(refs []any) []EnrichedReference {
	var result []EnrichedReference
	for _, r := range refs {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}

		er := EnrichedReference{}
		if doi, ok := rm["DOI"].(string); ok {
			er.DOI = strings.ToLower(strings.TrimSpace(doi))
		}
		if t, ok := rm["article-title"].(string); ok {
			er.Title = t
		}
		if a, ok := rm["author"].(string); ok {
			er.Author = a
		}
		switch y := rm["year"].(type) {
		case string:
			if year, err := strconv.Atoi(strings.TrimSpace(y)); err == nil {
				er.Year = year
			}
		case float64:
			er.Year = int(y)
		case json.Number:
			if year, err := strconv.Atoi(string(y)); err == nil {
				er.Year = year
			}
		}
		if s, ok := rm["journal-title"].(string); ok {
			er.Source = s
		} else if s, ok := rm["series-title"].(string); ok {
			er.Source = s
		}

		result = append(result, er)
	}
	return result
}
