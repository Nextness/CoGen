// orcid.go implements ORCID and OpenAlex author response decoders for
// author identity enrichment. It also provides ORCID name-search URL
// construction and candidate extraction for the identity pipeline.
package enrich

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// DecodeOpenAlexAuthorResponse converts an OpenAlex author response for an
// exact ORCID lookup.
func DecodeOpenAlexAuthorResponse(body []byte, orcid string) *EnrichedAuthor {
	entry := decodeOpenAlexAuthor(body)
	if entry == nil {
		return nil
	}
	entry["_source"] = "openalex"
	return orcidEntryToAuthor(entry, orcid)
}

// decodeOpenAlexAuthor decodes open alex author from the supplied payload.
func decodeOpenAlexAuthor(body []byte) map[string]any {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	return raw
}

// DecodeORCIDRecordResponse converts an ORCID person record for an exact
// ORCID lookup.
func DecodeORCIDRecordResponse(body []byte, orcid string) *EnrichedAuthor {
	entry := decodeORCIDRecord(body)
	if entry == nil {
		return nil
	}
	entry["_source"] = "orcid"
	return orcidEntryToAuthor(entry, orcid)
}

// decodeORCIDRecord decodes orcid record from the supplied payload.
func decodeORCIDRecord(body []byte) map[string]any {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	out := make(map[string]any)
	if person, ok := raw["person"].(map[string]any); ok {
		if name, ok := person["name"].(map[string]any); ok {
			if givenNames, ok := name["given-names"].(map[string]any); ok {
				out["given_names"], _ = givenNames["value"].(string)
			}
			if familyName, ok := name["family-name"].(map[string]any); ok {
				out["family_name"], _ = familyName["value"].(string)
			}
			if creditName, ok := name["credit-name"].(map[string]any); ok {
				out["credit_name"], _ = creditName["value"].(string)
			}
		}
		if otherNames, ok := person["other-names"].(map[string]any); ok {
			if names, ok := otherNames["other-name"].([]any); ok {
				alternates := make([]string, 0, len(names))
				for _, rawName := range names {
					if name, ok := rawName.(map[string]any); ok {
						if value, ok := name["content"].(string); ok {
							alternates = append(alternates, value)
						}
					}
				}
				out["alternate_names"] = alternates
			}
		}
	}
	if orcidValue, ok := raw["orcid"].(map[string]any); ok {
		out["orcid_path"], _ = orcidValue["path"].(string)
	}
	return out
}

// ORCIDNameSearchURLs returns the ordered exact-name search requests used by
// the workspace pipeline. Candidates from every successful query remain
// evidence for review and are never treated as identity proof.
func ORCIDNameSearchURLs(source SourceConfig, name string) []string {
	searchURL := source.ExtraURLs["search"]
	if searchURL == "" {
		return nil
	}
	var givenName, familyName, creditName string
	if index := strings.Index(name, ","); index >= 0 {
		familyName = strings.TrimSpace(name[:index])
		givenName = strings.TrimSpace(name[index+1:])
		creditName = strings.TrimSpace(givenName + " " + familyName)
	} else {
		creditName = strings.TrimSpace(name)
	}
	queries := make([]string, 0, 3)
	if givenName != "" && familyName != "" {
		queries = append(queries,
			fmt.Sprintf("given-names:%s AND family-name:%s", orcidSearchLiteral(givenName), orcidSearchLiteral(familyName)),
			fmt.Sprintf("credit-name:%s", orcidSearchLiteral(creditName)),
			fmt.Sprintf("credit-name:%s", orcidSearchLiteral(familyName+", "+givenName)),
		)
	} else if creditName != "" {
		queries = append(queries, fmt.Sprintf("credit-name:%s", orcidSearchLiteral(creditName)))
	}
	urls := make([]string, 0, len(queries))
	for _, query := range queries {
		urls = append(urls, searchURL+"?q="+url.QueryEscape(query))
	}
	return urls
}

// ORCIDNameSearchCandidate is provider evidence only, never proof that the
// citation author is the person identified by the returned ORCID.
type ORCIDNameSearchCandidate struct {
	ORCID string
}

// DecodeORCIDNameSearchCandidates returns every usable ORCID in provider
// order. Workspace callers retain this ambiguity for human review.
func DecodeORCIDNameSearchCandidates(body []byte) []ORCIDNameSearchCandidate {
	var response struct {
		Result []struct {
			ORCID struct {
				Path string `json:"path"`
			} `json:"orcid-identifier"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}
	candidates := make([]ORCIDNameSearchCandidate, 0, len(response.Result))
	for _, result := range response.Result {
		if orcid := strings.TrimSpace(result.ORCID.Path); orcid != "" {
			candidates = append(candidates, ORCIDNameSearchCandidate{ORCID: orcid})
		}
	}
	return candidates
}

// orcidEntryToAuthor converts a validated ORCID record to author enrichment fields.
func orcidEntryToAuthor(entry map[string]any, orcid string) *EnrichedAuthor {
	author := &EnrichedAuthor{ORCID: orcid}
	source, _ := entry["_source"].(string)
	author.Source = source
	if source == "" {
		author.Source = "unknown"
	}
	switch source {
	case "openalex":
		author.DisplayName, _ = entry["display_name"].(string)
		if name := author.DisplayName; name != "" {
			parts := strings.SplitN(name, " ", 2)
			author.FirstName = parts[0]
			if len(parts) == 2 {
				author.LastName = parts[1]
			}
		}
		if stats, ok := entry["summary_stats"].(map[string]any); ok {
			if hIndex, ok := stats["h_index"].(float64); ok {
				author.HIndex = int(hIndex)
			}
			if i10Index, ok := stats["i10_index"].(float64); ok {
				author.I10Index = int(i10Index)
			}
		}
		if count, ok := entry["works_count"].(float64); ok {
			author.WorksCount = int(count)
		}
		if count, ok := entry["cited_by_count"].(float64); ok {
			author.CitedByCnt = int(count)
		}
		if institutions, ok := entry["last_known_institutions"].([]any); ok && len(institutions) > 0 {
			if first, ok := institutions[0].(map[string]any); ok {
				author.Institution, _ = first["display_name"].(string)
			}
		}
	case "orcid":
		author.FirstName, _ = entry["given_names"].(string)
		author.LastName, _ = entry["family_name"].(string)
		author.DisplayName, _ = entry["credit_name"].(string)
		if author.DisplayName == "" && author.FirstName != "" && author.LastName != "" {
			author.DisplayName = author.FirstName + " " + author.LastName
		}
		if author.LastName != "" && author.FirstName != "" {
			author.CitationName = author.LastName + ", " + author.FirstName
		}
	}
	return author
}

// orcidSearchLiteral returns a quoted ORCID search literal with query syntax escaped.
func orcidSearchLiteral(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
