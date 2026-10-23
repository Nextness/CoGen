// Package providers owns the workspace rate-limited cached provider gathering and identity-search evidence.
package providers

import (
	"analysis/backend/article"
	"analysis/backend/database"
	dbauthor "analysis/backend/database/author"

	"analysis/backend/enrich"
	"analysis/backend/logging"

	"analysis/backend/workspace/enrichment"
	"analysis/backend/workspace/providercache"

	"context"

	"fmt"
	"net/url"

	"sort"

	"strings"
	"time"
)

// Crossref gathers cached crossref from the supplied inputs.
func Crossref(ctx context.Context, cache *providercache.Cache, source enrich.SourceConfig, articles []*article.Article) (*enrich.GatherResult, error) {
	client, err := enrich.NewClient(source)
	if err != nil {
		return nil, fmt.Errorf("create crossref client: %w", err)
	}
	defer client.Close()
	result := &enrich.GatherResult{Source: "crossref", Articles: make(map[string]*enrich.ArticleEnrichment)}
	for _, a := range articles {
		if a.DOI == "" {
			continue
		}
		response, err := cache.Resolve(ctx, providercache.Request{Provider: "crossref", Namespace: "work_by_doi", Identity: strings.ToLower(a.DOI), URL: source.BaseURL + url.PathEscape(a.DOI)}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, source.BaseURL+url.PathEscape(a.DOI))
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve crossref DOI %q: %w", a.DOI, err)
		}
		if response.Status == 404 {
			result.DOINotFound = append(result.DOINotFound, a.DOI)
			continue
		}
		if enrichment := enrich.DecodeCrossrefResponse(response.Body, a.DOI); enrichment != nil {
			result.Articles[a.DOI] = enrichment
		}
	}
	return result, nil
}

// OpenAlex gathers cached open alex from the supplied inputs.
func OpenAlex(ctx context.Context, cache *providercache.Cache, source enrich.SourceConfig, articles []*article.Article) (*enrich.GatherResult, error) {
	client, err := enrich.NewClient(source)
	if err != nil {
		return nil, fmt.Errorf("create openalex client: %w", err)
	}
	defer client.Close()
	result := &enrich.GatherResult{Source: "openalex", Articles: make(map[string]*enrich.ArticleEnrichment)}
	refIDsByDOI := make(map[string][]string)
	for _, a := range articles {
		if a.DOI == "" {
			continue
		}
		url := source.BaseURL + "doi:" + url.PathEscape(a.DOI)
		response, err := cache.Resolve(ctx, providercache.Request{Provider: "openalex", Namespace: "work_by_doi", Identity: strings.ToLower(a.DOI), URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, url)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve openalex DOI %q: %w", a.DOI, err)
		}
		if response.Status == 404 {
			result.DOINotFound = append(result.DOINotFound, a.DOI)
			continue
		}
		if enrichment, referenceIDs := enrich.DecodeOpenAlexResponse(response.Body, a.DOI); enrichment != nil {
			result.Articles[a.DOI] = enrichment
			if source.AllowsField("references") {
				refIDsByDOI[a.DOI] = referenceIDs
			}
		}
	}
	references, err := resolveCachedOpenAlexReferences(ctx, cache, client, source, refIDsByDOI)
	if err != nil {
		return nil, err
	}
	for doi, ids := range refIDsByDOI {
		enrichment := result.Articles[doi]
		if enrichment == nil {
			continue
		}
		for _, id := range ids {
			if reference, ok := references[id]; ok {
				enrichment.References = append(enrichment.References, reference)
			}
		}
	}
	return result, nil
}

// resolveCachedOpenAlexReferences resolves cached open alex references from the supplied context.
func resolveCachedOpenAlexReferences(ctx context.Context, cache *providercache.Cache, client *enrich.Client, source enrich.SourceConfig, refIDsByDOI map[string][]string) (map[string]enrich.EnrichedReference, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, articleIDs := range refIDsByDOI {
		for _, id := range articleIDs {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	result := make(map[string]enrich.EnrichedReference)
	batchSize := source.BatchSize
	if batchSize < 1 {
		batchSize = 50
	}
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		identity := strings.Join(batch, "|")
		url := fmt.Sprintf("%s?filter=openalex_id:%s&select=id,doi,title,publication_year&per_page=%d", strings.TrimRight(source.BaseURL, "/"), identity, batchSize)
		response, err := cache.Resolve(ctx, providercache.Request{Provider: "openalex", Namespace: "work_references", Identity: identity, URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, url)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve openalex reference batch: %w", err)
		}
		if response.Status == 404 {
			continue
		}
		for id, reference := range enrich.DecodeOpenAlexReferenceResponse(response.Body) {
			result[id] = reference
		}
	}
	return result, nil
}

// Candidate stores one unconfirmed ORCID name-search result and its provenance.
type Candidate struct {
	ORCID             string
	ProviderDisplay   string
	QueryURL          string
	PayloadArtifactID int64
	ProviderRank      int
}

// IdentitySearchEvidence records candidate-search results or provider failure for one author occurrence.
type IdentitySearchEvidence struct {
	DOI          string
	AuthorIndex  int
	CitationName string
	Candidates   []Candidate
	Status       string
	ErrorMessage string
}

// ORCID gathers confirmed ORCID records and uncertain name-search evidence through the workspace cache.
func ORCID(ctx context.Context, cache *providercache.Cache, orcidSource, openAlexSource enrich.SourceConfig, hasOpenAlex bool, articles []*article.Article) (int, []enrichment.Change, []IdentitySearchEvidence, error) {
	orcidClient, err := enrich.NewClient(orcidSource)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("create ORCID client: %w", err)
	}
	defer orcidClient.Close()
	var openAlexClient *enrich.Client
	if hasOpenAlex {
		openAlexClient, err = enrich.NewClient(openAlexSource)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("create OpenAlex client: %w", err)
		}
		defer openAlexClient.Close()
	}
	updated := 0
	var changes []enrichment.Change

	// Build per-author provenance so mutation and audit evidence share one source.
	authorProvenance := make(map[*article.Author]enrichment.AuthorOrigin)
	for _, a := range articles {
		for i := range a.Authors {
			authorProvenance[&a.Authors[i]] = enrichment.AuthorOrigin{DOI: a.DOI, Index: i}
		}
	}

	byORCID := make(map[string][]*article.Author)
	for _, a := range articles {
		for index := range a.Authors {
			author := &a.Authors[index]
			if author.Orcid != "" {
				byORCID[author.Orcid] = append(byORCID[author.Orcid], author)
			}
		}
	}
	for orcid, authors := range byORCID {
		profile, err := resolveCachedORCIDProfile(ctx, cache, orcidSource, orcidClient, openAlexSource, openAlexClient, hasOpenAlex, orcid)
		if err != nil {
			return 0, nil, nil, err
		}
		if profile != nil {
			if !orcidSource.AllowsField("display_name") {
				profile.FirstName, profile.LastName, profile.CitationName, profile.DisplayName = "", "", "", ""
			}
			if !orcidSource.AllowsField("institution") {
				profile.Institution, profile.Affiliation = "", ""
			}
			changed, observedChanges := enrichment.ApplyProfile(authors, profile, "", authorProvenance)
			if changed {
				updated++
				changes = append(changes, observedChanges...)
			}
		}
	}
	var evidence []IdentitySearchEvidence
	for _, a := range articles {
		for authorIndex := range a.Authors {
			author := &a.Authors[authorIndex]
			if !orcidSource.AllowsField("orcid") || author.Orcid != "" || author.CitationName == "" {
				continue
			}
			candidates, err := resolveCachedORCIDNameCandidates(ctx, cache, orcidSource, orcidClient, author.CitationName)
			searchEvidence := IdentitySearchEvidence{DOI: a.DOI, AuthorIndex: authorIndex, CitationName: author.CitationName, Candidates: candidates}
			if err != nil {
				searchEvidence.Status = dbauthor.IdentityStatusProviderFailed
				searchEvidence.ErrorMessage = err.Error()
				evidence = append(evidence, searchEvidence)
				return updated, changes, evidence, err
			}
			evidence = append(evidence, searchEvidence)
		}
	}
	return updated, changes, evidence, nil
}

// resolveCachedORCIDProfile resolves cached orcid profile from the supplied context.
func resolveCachedORCIDProfile(ctx context.Context, cache *providercache.Cache, orcidSource enrich.SourceConfig, orcidClient *enrich.Client, openAlexSource enrich.SourceConfig, openAlexClient *enrich.Client, hasOpenAlex bool, orcid string) (*enrich.EnrichedAuthor, error) {
	if hasOpenAlex {
		baseURL := openAlexSource.ExtraURLs["author"]
		if baseURL == "" {
			baseURL = "https://api.openalex.org/authors/"
		}
		url := baseURL + "orcid:" + orcid
		response, err := cache.Resolve(ctx, providercache.Request{Provider: "openalex", Namespace: "author_by_orcid", Identity: strings.ToLower(orcid), URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return openAlexClient.Fetch(ctx, url)
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve openalex ORCID %q: %w", orcid, err)
		}
		if response.Status == 200 {
			return enrich.DecodeOpenAlexAuthorResponse(response.Body, orcid), nil
		}
	}
	url := orcidSource.BaseURL + orcid
	response, err := cache.Resolve(ctx, providercache.Request{Provider: "orcid", Namespace: "author_by_orcid", Identity: strings.ToLower(orcid), URL: url}, func(ctx context.Context) *enrich.FetchResult {
		return orcidClient.Fetch(ctx, url)
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("resolve ORCID %q: %w", orcid, err)
	}
	if response.Status == 404 {
		return nil, nil
	}
	return enrich.DecodeORCIDRecordResponse(response.Body, orcid), nil
}

// resolveCachedORCIDNameCandidates resolves cached orcid name candidates from the supplied context.
func resolveCachedORCIDNameCandidates(ctx context.Context, cache *providercache.Cache, source enrich.SourceConfig, client *enrich.Client, name string) ([]Candidate, error) {
	seen := make(map[string]struct{})
	candidates := make([]Candidate, 0)
	for _, url := range enrich.ORCIDNameSearchURLs(source, name) {
		response, err := cache.Resolve(ctx, providercache.Request{Provider: "orcid", Namespace: "author_name_search", Identity: normalizeCacheName(name) + "|" + url, URL: url}, func(ctx context.Context) *enrich.FetchResult {
			return client.Fetch(ctx, url)
		}, func(body []byte) bool { return len(enrich.DecodeORCIDNameSearchCandidates(body)) == 0 })
		if err != nil {
			return candidates, fmt.Errorf("resolve ORCID name %q: %w", name, err)
		}
		if response.Status != 200 {
			continue
		}
		for _, decoded := range enrich.DecodeORCIDNameSearchCandidates(response.Body) {
			orcid := strings.TrimSpace(decoded.ORCID)
			if orcid == "" {
				continue
			}
			key := strings.ToLower(orcid)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, Candidate{
				ORCID: orcid, QueryURL: url, PayloadArtifactID: response.PayloadArtifactID,
				ProviderRank: len(candidates) + 1,
			})
		}
	}
	return candidates, nil
}

// PersistIdentityEvidence persists uncertain orcid evidence through the owning repository.
func PersistIdentityEvidence(ctx context.Context, db *database.Database, runID int64, revisionIDs map[string]int64, evidence []IdentitySearchEvidence) error {
	for _, search := range evidence {
		revisionID, ok := revisionIDs[search.DOI]
		if !ok {
			return fmt.Errorf("persist uncertain ORCID evidence: missing metadata-enriched revision for DOI %q", search.DOI)
		}
		authorships, err := db.Author.ListAuthorshipsByRevisionID(ctx, revisionID)
		if err != nil {
			return err
		}
		if search.AuthorIndex < 0 || search.AuthorIndex >= len(authorships) {
			return fmt.Errorf("persist uncertain ORCID evidence: author index %d is missing from metadata-enriched revision %d", search.AuthorIndex, revisionID)
		}
		status := search.Status
		if status == "" && len(search.Candidates) > 0 {
			status = dbauthor.IdentityStatusORCIDUnclear
		}
		if status == "" {
			status = dbauthor.IdentityStatusNoORCIDCandidate
		}
		resolutionID, err := db.Author.CreateIdentityResolution(ctx, &dbauthor.IdentityResolution{
			PipelineRunID: runID, AuthorOccurrenceID: authorships[search.AuthorIndex].AuthorOccurrenceID,
			Status: status, Provider: "orcid", QueriedCitationName: search.CitationName,
			ErrorMessage: search.ErrorMessage,
			ResolvedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return err
		}
		for _, candidate := range search.Candidates {
			if _, err := db.Author.CreateIdentityCandidate(ctx, &dbauthor.IdentityCandidate{
				IdentityResolutionID: resolutionID, CandidateORCID: candidate.ORCID,
				ProviderDisplayName: candidate.ProviderDisplay, QueryURL: candidate.QueryURL,
				PayloadArtifactID: candidate.PayloadArtifactID, ProviderRank: candidate.ProviderRank,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// normalizeCacheName normalizes cache name.
func normalizeCacheName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// log retains the owning pipeline operational identity.
var log = logging.Logger("workspace")
