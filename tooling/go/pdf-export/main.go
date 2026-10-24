// Package pdfexport exports validated companion PDFs by DOI without changing the corpus databases.
package pdfexport

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"analysis/backend/database/pdfbinding"
	"analysis/backend/database/work"
	"analysis/backend/pdfstore"
	"analysis/internal/pathpolicy"
	"analysis/internal/sqliteread"

	_ "modernc.org/sqlite"
)

// Result identifies one exported PDF, including its validated content identity and output path.
type Result struct {
	DOI         string `json:"doi"`
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`
	ByteSize    int    `json:"byte_size"`
	Status      string `json:"status"`
}

// Run parses repeatable DOI selectors, exports available PDFs, and reports partial failures with a nonzero exit status.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("pdf-export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	metadataPath := flags.String("db", "", "existing metadata corpus database")
	outputDir := flags.String("out", "", "directory for paper-title PDF filenames")
	var dois []string
	flags.Func("doi", "DOI or doi.org URL; may be repeated", func(doi string) error {
		dois = append(dois, doi)
		return nil
	})
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *metadataPath == "" || *outputDir == "" || len(dois) == 0 || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: pdf-export --db <corpus.metadata.db> --doi <doi> [--doi <doi> ...] --out <directory>")
		return 2
	}
	results, exportErr := Export(context.Background(), *metadataPath, dois, *outputDir)
	if results == nil {
		results = []Result{}
	}
	if err := json.NewEncoder(stdout).Encode(results); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if exportErr != nil {
		fmt.Fprintln(stderr, exportErr)
		return 1
	}
	return 0
}

// Export reads an existing bound corpus, exports each unique DOI, and preserves successful results when another DOI fails.
func Export(ctx context.Context, metadataPath string, dois []string, outputDir string) ([]Result, error) {
	if metadataPath == "" || outputDir == "" || len(dois) == 0 {
		return nil, fmt.Errorf("metadata database, DOI selectors, and output directory are required")
	}
	metadata, err := sqliteread.Open(metadataPath, "sqlite")
	if err != nil {
		return nil, fmt.Errorf("open metadata database: %w", err)
	}
	defer metadata.Close()
	if err := metadata.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("read metadata database: %w", err)
	}
	relative, found, err := pdfbinding.New(metadata).Binding(ctx)
	if err != nil {
		return nil, err
	}
	if !found || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("an existing bundle-relative PDF store binding is required")
	}
	absoluteMetadata, err := filepath.Abs(metadataPath)
	if err != nil {
		return nil, err
	}
	storePath, err := pathpolicy.ResolveExistingWithin(filepath.Dir(absoluteMetadata), relative)
	if err != nil {
		return nil, fmt.Errorf("resolve PDF companion: %w", err)
	}
	resolvedMetadata, err := filepath.EvalSymlinks(absoluteMetadata)
	if err != nil {
		return nil, err
	}
	if storePath == resolvedMetadata {
		return nil, fmt.Errorf("PDF companion must differ from the metadata database")
	}
	companion, err := sqliteread.Open(storePath, "sqlite")
	if err != nil {
		return nil, fmt.Errorf("open PDF companion: %w", err)
	}
	defer companion.Close()
	if err := companion.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("read PDF companion: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}
	works := work.New(metadata)
	pdfs := pdfstore.New(companion)
	results := make([]Result, 0, len(dois))
	seen := make(map[string]bool)
	var failures []error
	for _, raw := range dois {
		doi := work.NormalizeDOI(raw)
		if seen[doi] {
			continue
		}
		seen[doi] = true
		result, err := exportOne(ctx, works, pdfs, doi, outputDir)
		if err != nil {
			failures = append(failures, fmt.Errorf("export DOI %q: %w", doi, err))
			continue
		}
		results = append(results, result)
	}
	return results, errors.Join(failures...)
}

// exportOne resolves a paper title, validates stored PDF identity, and writes or recognizes its existing export.
func exportOne(ctx context.Context, works *work.Store, pdfs *pdfstore.Store, doi, outputDir string) (Result, error) {
	result := Result{DOI: doi}
	paper, err := works.GetWorkByDOI(ctx, doi)
	if err != nil {
		return result, err
	}
	if paper == nil {
		return result, fmt.Errorf("DOI does not belong to the metadata corpus")
	}
	revisions, err := works.ListRevisionsByWorkID(ctx, paper.ID)
	if err != nil {
		return result, err
	}
	var title string
	var normalized bool
	for _, revision := range revisions {
		if strings.TrimSpace(revision.Title) == "" {
			continue
		}
		if revision.ProducerStage == work.ProducerStageNormalize {
			title, normalized = revision.Title, true
		} else if !normalized {
			title = revision.Title
		}
	}
	filename, err := titleFilename(title)
	if err != nil {
		return result, err
	}
	document, err := pdfs.AvailableDocument(ctx, doi)
	if err != nil {
		return result, err
	}
	if document == nil {
		return result, fmt.Errorf("PDF is not available in the companion database")
	}
	if document.ByteSize <= 0 || document.ByteSize > pdfstore.DefaultMaxPDFBytes {
		return result, fmt.Errorf("stored PDF size is outside the supported byte limit")
	}
	data, err := pdfs.BlobData(ctx, document.ContentHash)
	if err != nil {
		return result, err
	}
	digest, err := pdfstore.ValidatePDF(data, pdfstore.DefaultMaxPDFBytes)
	if err != nil {
		return result, fmt.Errorf("stored PDF failed validation: %w", err)
	}
	if digest != document.ContentHash || int64(len(data)) != document.ByteSize {
		return result, fmt.Errorf("stored PDF failed content-hash or byte-size validation")
	}
	result.Path = filepath.Join(outputDir, filename)
	result.ContentHash, result.ByteSize = digest, len(data)
	result.Status, err = writePDF(result.Path, data, digest)
	return result, err
}

// titleFilename preserves readable Unicode titles while removing path separators, unsafe characters, and excessive filename bytes.
func titleFilename(title string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("paper title is not recorded")
	}
	name := strings.Map(func(character rune) rune {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return ' '
		}
		if strings.ContainsRune(`/\:*?"<>|`, character) {
			return '-'
		}
		return character
	}, title)
	name = strings.Join(strings.Fields(name), " ")
	name = strings.Trim(name, " .")
	for len(name) > 236 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	name = strings.TrimRight(name, " .")
	if name == "" {
		return "", fmt.Errorf("paper title has no usable filename characters")
	}
	return name + ".pdf", nil
}

// writePDF creates a new export exclusively, accepts identical existing bytes, and never overwrites a different file or symlink.
func writePDF(path string, data []byte, digest string) (string, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return "", statErr
		}
		if info.Mode().IsRegular() && info.Size() == int64(len(data)) {
			existing, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			existingDigest, validationErr := pdfstore.ValidatePDF(existing, pdfstore.DefaultMaxPDFBytes)
			if validationErr == nil && existingDigest == digest {
				return "unchanged", nil
			}
		}
		return "", fmt.Errorf("output filename already exists with different content or is not a regular file")
	}
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("write PDF: %w", err)
	}
	return "exported", nil
}
