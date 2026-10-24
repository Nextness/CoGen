// Package bundle verifies that the configured metadata and PDF migration chains
// reach the exact schema pair supported by this application. It is a focused
// consumer that application entry points compose; it never opens databases or
// imports workspace/server orchestration.
package bundle

import (
	"fmt"

	"analysis/backend/something"
	"analysis/internal/migrations"
)

// Pair identifies the final migration filenames for the metadata and PDF stores.
type Pair struct {
	Metadata string
	PDF      string
}

// SupportedPair is the schema pair this application requires before normal use.
var SupportedPair = Pair{
	Metadata: "V00028_immutable_cache_responses.sql",
	PDF:      "V00003_pdf_document_management.sql",
}

// chainLink records one configured migration link without applying it.
type chainLink struct {
	Filename   string
	Previous   string
	Upgrade    string
	Supersedes []string
}

// VerifySupportedPair derives the configured endpoints from the metadata and PDF
// chains and verifies they match SupportedPair with complete prior history.
func VerifySupportedPair(metadataConfigPath, pdfConfigPath string) error {
	return VerifyPair(metadataConfigPath, pdfConfigPath, SupportedPair)
}

// VerifyPair derives the configured endpoints from the metadata and PDF chains
// and verifies they match supported with complete prior history.
func VerifyPair(metadataConfigPath, pdfConfigPath string, supported Pair) error {
	metadataLinks, err := loadChainLinks(metadataConfigPath)
	if err != nil {
		return fmt.Errorf("metadata chain: %w", err)
	}
	pdfLinks, err := loadChainLinks(pdfConfigPath)
	if err != nil {
		return fmt.Errorf("pdf chain: %w", err)
	}
	if err := verifyChain("metadata", metadataLinks, supported.Metadata); err != nil {
		return err
	}
	if err := verifyChain("pdf", pdfLinks, supported.PDF); err != nil {
		return err
	}
	return nil
}

// loadChainLinks reads one migration configuration's configured links without applying them.
func loadChainLinks(configPath string) ([]chainLink, error) {
	cfg, err := something.LoadSomethingFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", configPath, err)
	}
	structs, err := something.GetStructAll(cfg, "[iteration]_db_migration")
	if err != nil {
		return nil, fmt.Errorf("get migration structs from %s: %w", configPath, err)
	}
	links := make([]chainLink, 0, len(structs))
	for _, entry := range structs {
		filename, ok := entry["filename"].(string)
		if !ok || !migrations.ValidFilename(filename) {
			return nil, fmt.Errorf("migration filename must be in V00001_description.sql form")
		}
		link := chainLink{Filename: filename}
		if previous, ok := entry["previous"].(string); ok {
			link.Previous = previous
		}
		if upgrade, ok := entry["upgrade"].(string); ok {
			link.Upgrade = upgrade
		}
		if raw, ok := entry["supersedes"].([]any); ok {
			for _, value := range raw {
				alias, ok := value.(string)
				if !ok || !migrations.ValidFilename(alias) {
					return nil, fmt.Errorf("migration %s supersedes must contain filenames in V00001_description.sql form", filename)
				}
				if alias == filename {
					return nil, fmt.Errorf("migration %s cannot supersede itself", filename)
				}
				link.Supersedes = append(link.Supersedes, alias)
			}
		}
		links = append(links, link)
	}
	return links, nil
}

// verifyChain checks one chain's prior history and final endpoint.
func verifyChain(name string, links []chainLink, endpoint string) error {
	if len(links) == 0 {
		return fmt.Errorf("%s chain is empty", name)
	}
	for index, link := range links {
		if index == 0 {
			if link.Previous != "ROOT" && link.Previous != "" {
				return fmt.Errorf("%s first migration %q previous = %q, want ROOT", name, link.Filename, link.Previous)
			}
			continue
		}
		if link.Previous != links[index-1].Filename {
			return fmt.Errorf("%s migration %q previous = %q, want %q", name, link.Filename, link.Previous, links[index-1].Filename)
		}
	}
	if links[len(links)-1].Filename != endpoint {
		return fmt.Errorf("%s endpoint = %q, want %q", name, links[len(links)-1].Filename, endpoint)
	}
	return nil
}
