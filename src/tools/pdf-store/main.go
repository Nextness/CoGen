// Command pdf-store provides controlled maintenance operations for the
// companion PDF database. It never writes through the read-only viewer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"analysis/database"
	"analysis/pdfstore"
)

// main dispatches the analysis command selected by process arguments and exits on command failure.
func main() {
	if len(os.Args) < 2 || os.Args[1] != "add" {
		fmt.Fprintln(os.Stderr, "usage: pdf-store add --db <corpus.metadata.db> --doi <doi> --file <local.pdf>")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("add", flag.ExitOnError)
	dbPath := flags.String("db", "", "path to the metadata corpus database")
	doi := flags.String("doi", "", "DOI belonging to the metadata corpus")
	filePath := flags.String("file", "", "local PDF file")
	_ = flags.Parse(os.Args[2:])
	if *dbPath == "" || *doi == "" || *filePath == "" {
		fmt.Fprintln(os.Stderr, "pdf-store add requires --db, --doi, and --file")
		os.Exit(2)
	}
	if err := add(*dbPath, *doi, *filePath); err != nil {
		fmt.Fprintf(os.Stderr, "pdf-store add: %v\n", err)
		os.Exit(1)
	}
}

// add validates command arguments and adds one manual PDF through the configured database registry.
func add(metadataPath, doi, filePath string) error {
	return addWithRegistry(metadataPath, doi, filePath, "config/database.something")
}

// addWithRegistry opens the metadata and PDF stores, inserts content, and drains the audit outbox.
func addWithRegistry(metadataPath, doi, filePath, registryPath string) error {
	if _, err := os.Stat(metadataPath); err != nil {
		return fmt.Errorf("inspect metadata corpus: %w", err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open PDF: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect PDF: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("PDF path is a directory")
	}
	if info.Size() > int64(pdfstore.DefaultMaxPDFBytes) {
		return fmt.Errorf("PDF exceeds the %d-byte limit", pdfstore.DefaultMaxPDFBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(pdfstore.DefaultMaxPDFBytes)+1))
	if err != nil {
		return fmt.Errorf("read PDF: %w", err)
	}
	if _, err := pdfstore.ValidatePDF(data, pdfstore.DefaultMaxPDFBytes); err != nil {
		return err
	}
	metadata, err := database.Open(metadataPath, registryPath)
	if err != nil {
		return err
	}
	defer metadata.Close()
	work, err := metadata.Works.GetByDOI(doi)
	if err != nil {
		return err
	}
	if work == nil {
		return fmt.Errorf("DOI %q does not belong to the metadata corpus", database.NormalizeDOI(doi))
	}
	ctx := context.Background()
	storePath, err := pdfstore.BoundStorePath(ctx, metadata.DB, metadataPath)
	if err != nil {
		return err
	}
	store, err := pdfstore.Open(storePath, registryPath)
	if err != nil {
		return err
	}
	defer store.Close()
	flushed, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if err != nil {
		return err
	}
	result, err := store.Add(ctx, work.DOI, work.ID, data)
	if err != nil {
		return err
	}
	newlyFlushed, err := store.FlushAuditOutbox(ctx, metadata.DB)
	if err != nil {
		return err
	}
	flushed += newlyFlushed
	status := "unchanged"
	if result.Added {
		status = "inventoried"
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"doi": work.DOI, "status": status, "content_hash": result.ContentHash,
		"byte_size": result.ByteSize, "audit_events_flushed": flushed,
	})
}
