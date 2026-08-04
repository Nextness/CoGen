// Package enrich provides provider configuration, HTTP clients, decoders, and
// DTOs for Crossref, OpenAlex, and ORCID. Database persistence and cache
// policy belong to the workspace pipeline.
package enrich

import "analysis/logging"

var log = logging.Logger("enrich")

// SourceConfig describes an enrichment source as declared in a SOMETHING
// configuration.
type SourceConfig struct {
	Name            string
	BaseURL         string
	UserAgent       string
	ContactEmail    string
	RatePerSecond   int
	Concurrency     int
	TimeoutSecs     int
	MaxRetries      int
	Fields          []string
	ExtraURLs       map[string]string
	BatchSize       int
	FillMissingOnly bool
}

// Config holds all enrichment sources keyed by their configuration name.
type Config struct {
	Sources map[string]SourceConfig
}

// EnrichedAuthor is the per-author result of enrichment.
type EnrichedAuthor struct {
	ORCID        string
	FirstName    string
	LastName     string
	CitationName string
	Affiliation  string
	DisplayName  string
	WorksCount   int
	CitedByCnt   int
	HIndex       int
	I10Index     int
	Institution  string
	Source       string
}

// EnrichedReference is enriched metadata for one cited reference.
type EnrichedReference struct {
	DOI    string `json:"doi"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
	Source string `json:"source"`
}

// ArticleEnrichment holds gathered data for one article from one source.
type ArticleEnrichment struct {
	DOI            string
	Title          string
	Abstract       string
	Authors        []EnrichedAuthor
	Publisher      string
	References     []EnrichedReference
	CitationCount  int
	ReferenceCount int
}

// GatherResult is the pure output of one gather step.
type GatherResult struct {
	Source          string
	FillMissingOnly bool
	Articles        map[string]*ArticleEnrichment
	Authors         map[string]*EnrichedAuthor
	AuthorMatches   map[string]string
	DOINotFound     []string
}
