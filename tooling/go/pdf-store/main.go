// Command pdf-store provides controlled maintenance operations for the
// companion PDF database. It never writes through the read-only viewer.
package pdfstorecmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"analysis/backend/database"
	dbwork "analysis/backend/database/work"
	"analysis/backend/pdfstore"
)

// Run dispatches the analysis command selected by process arguments and exits on command failure.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "add" {
		fmt.Fprintln(stderr, "usage: pdf-store add --db <corpus.metadata.db> --doi <doi> --file <local.pdf>")
		return 2
	}
	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	dbPath := flags.String("db", "", "path to the metadata corpus database")
	doi := flags.String("doi", "", "DOI belonging to the metadata corpus")
	filePath := flags.String("file", "", "local PDF file")
	flags.SetOutput(stderr)
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *dbPath == "" || *doi == "" || *filePath == "" {
		fmt.Fprintln(stderr, "pdf-store add requires --db, --doi, and --file")
		return 2
	}
	if err := add(*dbPath, *doi, *filePath, stdout); err != nil {
		fmt.Fprintf(stderr, "pdf-store add: %v\n", err)
		return 1
	}
	return 0
}

// add validates and inventories one manual PDF through the configured stores.
func add(metadataPath, doi, filePath string, stdout io.Writer) error {
	const registryPath = "config/database.something"
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
	ctx := context.Background()
	metadata, err := database.Open(metadataPath, registryPath)
	if err != nil {
		return err
	}
	defer metadata.Close()
	found, err := metadata.Work.GetWorkByDOI(ctx, doi)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("DOI %q does not belong to the metadata corpus", dbwork.NormalizeDOI(doi))
	}
	bindings := metadata.PDFBindings
	storePath, err := bindings.ResolveBinding(ctx, metadataPath)
	if err != nil {
		return err
	}
	store, err := pdfstore.Open(storePath, registryPath)
	if err != nil {
		return err
	}
	defer store.Close()
	flushed, err := store.FlushAuditOutbox(ctx, bindings)
	if err != nil {
		return err
	}
	result, err := store.Add(ctx, found.DOI, found.ID, data)
	if err != nil {
		return err
	}
	newlyFlushed, err := store.FlushAuditOutbox(ctx, bindings)
	if err != nil {
		return err
	}
	flushed += newlyFlushed
	status := "unchanged"
	if result.Added {
		status = "inventoried"
	}
	return json.NewEncoder(stdout).Encode(map[string]any{
		"doi": found.DOI, "status": status, "content_hash": result.ContentHash,
		"byte_size": result.ByteSize, "audit_events_flushed": flushed,
	})
}
