// Shared helpers for manifest test files.
package manifest

// baseResolvedManifest supports the package test suite's base resolved manifest setup or assertions.
func baseResolvedManifest() *ResolvedManifest {
	return &ResolvedManifest{
		FormatVersion:     2,
		SearchID:          "bpmn-optimisation",
		SearchRevision:    "2026-07-query-expansion",
		EnrichmentEnabled: true,
		ReusePolicy:       "reuse_completed",
		CachePolicy: CachePolicy{
			Reads:           []string{"global", "network"},
			Writes:          []string{"active_run", "global"},
			NegativeTTLDays: 14,
		},
		Sources: []SourceManifest{
			{
				Name:            "scopus",
				ExpectedFile:    "corpus/scopus.raw.csv",
				FileType:        "csv",
				Query:           "TITLE-ABS-KEY(BPMN AND optimisation)",
				RequestedFields: []string{"title", "doi", "authors", "references"},
				PatchFields:     map[string]string{"dc:title": "title", "prism:doi": "doi"},
				KeepFields:      []string{"title", "doi", "authors", "references"},
			},
			{
				Name:            "ieeexplore",
				ExpectedFile:    "corpus/ieeexplore.raw.csv",
				FileType:        "csv",
				Query:           "BPMN AND optimisation",
				RequestedFields: []string{"title", "doi", "authors", "references"},
				PatchFields:     map[string]string{"title": "title", "doi": "doi"},
				KeepFields:      []string{"title", "doi", "authors", "references"},
			},
		},
		EnrichmentProviders: []EnrichmentProvider{
			{
				Name:            "crossref",
				BaseURL:         "https://api.crossref.org/works/",
				Fields:          []string{"title", "authors", "references", "publisher"},
				FillMissingOnly: false,
				RatePerSecond:   10,
				Concurrency:     10,
				TimeoutSeconds:  30,
				MaxRetries:      5,
				BatchSize:       50,
			},
			{
				Name:            "openalex",
				BaseURL:         "https://api.openalex.org/works/",
				Fields:          []string{"authors", "cited_by_count", "references"},
				FillMissingOnly: true,
				RatePerSecond:   10,
				Concurrency:     10,
				TimeoutSeconds:  30,
				MaxRetries:      5,
				BatchSize:       50,
			},
		},
		SchemaVersion: "V00004",
	}
}

// baseInputManifest supports the package test suite's base input manifest setup or assertions.
func baseInputManifest(rm *ResolvedManifest) *InputManifest {
	im, err := NewInputManifest(rm, map[string]SourceFileInfo{
		"scopus": {
			Path:   "corpus/scopus.raw.csv",
			SHA256: "abc123",
			Size:   1024,
		},
		"ieeexplore": {
			Path:   "corpus/ieeexplore.raw.csv",
			SHA256: "def456",
			Size:   512,
		},
	})
	if err != nil {
		panic(err)
	}
	return im
}

// cloneMap returns a shallow copy of a string-keyed map.
func cloneMap[V any](m map[string]V) map[string]V {
	out := make(map[string]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneStrings returns a copy of a string slice.
func cloneStrings(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	return out
}
