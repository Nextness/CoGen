// openalex.go implements the OpenAlex API response decoder for work
// and reference metadata enrichment. It decodes single-work and
// batch-reference responses independently for cache efficiency.
package enrich

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// DecodeOpenAlexResponse converts one raw work response. Reference metadata is
// populated separately by DecodeOpenAlexReferenceResponse so the workspace can
// cache work and reference lookups independently.
func DecodeOpenAlexResponse(body []byte, doi string) (*ArticleEnrichment, []string) {
	entry := extractOpenAlexEntry(body)
	if entry == nil {
		return nil, nil
	}
	return openalexEntryToArticle(entry, doi), openAlexReferenceIDs(entry)
}

// DecodeOpenAlexReferenceResponse converts an OpenAlex batch response to
// references keyed by OpenAlex work ID (for example, W123).
func DecodeOpenAlexReferenceResponse(body []byte) map[string]EnrichedReference {
	var response struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}
	result := make(map[string]EnrichedReference, len(response.Results))
	for _, work := range response.Results {
		id, _ := work["id"].(string)
		id = extractOpenAlexID(id)
		if id == "" {
			continue
		}
		reference := EnrichedReference{Source: "openalex"}
		doi, _ := work["doi"].(string)
		reference.DOI = normalizeOpenAlexDOI(doi)
		reference.Title, _ = work["title"].(string)
		if year, ok := work["publication_year"].(float64); ok {
			reference.Year = int(year)
		}
		result[id] = reference
	}
	return result
}

// extractOpenAlexEntry decodes an OpenAlex work payload, returning nil for malformed JSON.
func extractOpenAlexEntry(body []byte) map[string]any {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	return raw
}

// openalexEntryToArticle converts an OpenAlex work object to article enrichment fields.
func openalexEntryToArticle(entry map[string]any, doi string) *ArticleEnrichment {
	ae := &ArticleEnrichment{DOI: doi}
	if title, ok := entry["title"].(string); ok {
		ae.Title = title
	} else if displayName, ok := entry["display_name"].(string); ok {
		ae.Title = displayName
	}
	if index, ok := entry["abstract_inverted_index"].(map[string]any); ok {
		ae.Abstract = reconstructAbstract(index)
	}
	ae.Publisher = extractOpenAlexPublisher(entry)
	if count, ok := entry["cited_by_count"].(float64); ok {
		ae.CitationCount = int(count)
	}
	if references, ok := entry["referenced_works"].([]any); ok {
		ae.ReferenceCount = len(references)
		ae.References = extractOpenAlexReferences(references)
	}
	if authorships, ok := entry["authorships"].([]any); ok {
		ae.Authors = extractOpenAlexAuthors(authorships)
	}
	return ae
}

// reconstructAbstract rebuilds abstract text from OpenAlex's word-position index.
func reconstructAbstract(index map[string]any) string {
	type wordPosition struct {
		word string
		pos  int
	}
	var positions []wordPosition
	for word, rawPositions := range index {
		positionList, ok := rawPositions.([]any)
		if !ok {
			continue
		}
		for _, rawPosition := range positionList {
			switch position := rawPosition.(type) {
			case float64:
				positions = append(positions, wordPosition{word, int(position)})
			case int:
				positions = append(positions, wordPosition{word, position})
			}
		}
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].pos < positions[j].pos })
	words := make([]string, 0, len(positions))
	for _, position := range positions {
		words = append(words, position.word)
	}
	return strings.Join(words, " ")
}

// extractOpenAlexPublisher returns the direct or primary-source publisher name from a work object.
func extractOpenAlexPublisher(entry map[string]any) string {
	if publisher, ok := entry["publisher"].(string); ok && publisher != "" {
		return publisher
	}
	if location, ok := entry["primary_location"].(map[string]any); ok {
		if source, ok := location["source"].(map[string]any); ok {
			if publisher, ok := source["publisher"].(string); ok && publisher != "" {
				return publisher
			}
			if name, ok := source["display_name"].(string); ok && name != "" {
				return name
			}
		}
	}
	return ""
}

// extractOpenAlexAuthors converts usable OpenAlex authorships to enriched authors in source order.
func extractOpenAlexAuthors(authorships []any) []EnrichedAuthor {
	result := make([]EnrichedAuthor, 0, len(authorships))
	for _, rawAuthorship := range authorships {
		authorship, ok := rawAuthorship.(map[string]any)
		if !ok {
			continue
		}
		authorInfo, ok := authorship["author"].(map[string]any)
		if !ok {
			continue
		}
		author := EnrichedAuthor{Source: "openalex"}
		author.DisplayName, _ = authorInfo["display_name"].(string)
		author.DisplayName = strings.TrimSpace(author.DisplayName)
		author.CitationName = author.DisplayName
		if author.CitationName == "" {
			continue
		}
		if orcid, ok := authorInfo["orcid"].(string); ok && orcid != "" {
			author.ORCID = strings.TrimPrefix(strings.TrimPrefix(orcid, "https://orcid.org/"), "http://orcid.org/")
		}
		if institutions, ok := authorship["institutions"].([]any); ok && len(institutions) > 0 {
			if first, ok := institutions[0].(map[string]any); ok {
				author.Institution, _ = first["display_name"].(string)
				author.Affiliation = author.Institution
			}
		}
		if author.Affiliation == "" {
			if affiliations, ok := authorship["raw_affiliation_strings"].([]any); ok && len(affiliations) > 0 {
				author.Affiliation, _ = affiliations[0].(string)
			}
		}
		result = append(result, author)
	}
	return result
}

// openAlexReferenceIDs returns unique OpenAlex work identifiers from referenced-work URLs.
func openAlexReferenceIDs(entry map[string]any) []string {
	references, ok := entry["referenced_works"].([]any)
	if !ok {
		return nil
	}
	seen := make(map[string]bool, len(references))
	ids := make([]string, 0, len(references))
	for _, rawReference := range references {
		url, _ := rawReference.(string)
		id := extractOpenAlexID(url)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// extractOpenAlexReferences converts referenced-work DOI URLs to enriched references.
func extractOpenAlexReferences(references []any) []EnrichedReference {
	result := make([]EnrichedReference, 0, len(references))
	for _, rawReference := range references {
		url, ok := rawReference.(string)
		if !ok {
			continue
		}
		if doi := extractDOIFromURL(url); doi != "" {
			result = append(result, EnrichedReference{DOI: doi, Source: "openalex"})
		}
	}
	return result
}

// normalizeOpenAlexDOI returns a lowercase DOI extracted from a URL or prefixed value.
func normalizeOpenAlexDOI(value string) string {
	value = strings.TrimSpace(value)
	if doi := extractDOIFromURL(value); doi != "" {
		return doi
	}
	return strings.ToLower(strings.TrimPrefix(value, "doi:"))
}

var (
	openalexIDRe = regexp.MustCompile(`openalex\.org/(W\d+)`)
	doiURLRe     = regexp.MustCompile(`doi\.org/(10\.\d{4,}/[^\s,;]+)`)
)

// extractOpenAlexID returns a work identifier from an OpenAlex URL or bare identifier.
func extractOpenAlexID(url string) string {
	matches := openalexIDRe.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return matches[1]
	}
	if strings.HasPrefix(url, "W") && len(url) > 1 {
		return url
	}
	return ""
}

// extractDOIFromURL returns a lowercase DOI embedded in a DOI URL.
func extractDOIFromURL(url string) string {
	matches := doiURLRe.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return strings.ToLower(strings.TrimSpace(matches[1]))
	}
	return ""
}
