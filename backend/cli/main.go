// Package cli implements command dispatch for the corpus pipeline and local viewer.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"analysis/backend/database"
	"analysis/backend/logging"
	"analysis/backend/server"
	"analysis/backend/workspace"
)

var log = logging.Logger("pipeline")

// major, minor, and patch are the semantic version components of the binary.
// Bump them for each release; the version command prints MAJOR.MINOR.PATCH.
const major int = 1
const minor int = 0
const patch int = 0

// version returns the semantic version string.
func version() string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

// usage writes the supported command syntax to standard error.
func usage(stderr io.Writer) {
	fmt.Fprintf(stderr, `Usage: analysis <command> [options]

analysis runs a research-corpus pipeline ("run"), migrates an existing
metadata database ("migrate"), or starts the local viewer ("serve").

Commands:

  version Print the semantic version (MAJOR.MINOR.PATCH).

  run     Execute one or more declared workspace iterations. Reads
          article metadata from CSV and BibTeX sources, normalizes and
          enriches them via configured providers (Crossref, OpenAlex,
          ORCID), and records the results in the SQLite database.

          Flags:
            --config <path>    Path to workspace.something config file.
            --db <path>        Path to SQLite metadata database.
            --fresh            Start a new attempt even if a matching
                               plan already completed.
            --workspace
            <sel>              Workspace selector search_id@search_revision;
                               repeat to select multiple iterations.
                               Without this flag every declared iteration
                               runs in declaration order.

  recover Mark an abandoned running attempt failed, with an audit record.
          Requires exclusive pipeline ownership; refuses an active writer.
          Flags: --db <path> --run-id <positive integer>

  migrate Apply pending metadata migrations to an existing database
          without running a workspace.

          Flags:
            --db <path>        Path to an existing SQLite metadata
                               database (required).

  serve   Start the loopback-only review viewer for an existing workspace
          database. Binds to a loopback address by default; the viewer
          serves the evaluation table, corpus browser, and graph views.
          The binary does not embed frontend assets.

          Flags:
            --db <path>        Path to an existing SQLite workspace
                               database (required).
            --addr <host:port> Local address to listen on
                               (default "127.0.0.1:8080").
            --assets-dir
            <dir>              Directory of frontend assets to serve
                               (required). Use "build/frontend/dist" after
                               "make frontend-build".

Examples:

  ./analysis run --config config/workspace.something --db corpus.metadata.db
  ./analysis migrate --db corpus.metadata.db
  ./analysis run --config config/workspace.something --db corpus.metadata.db \
      --workspace search_id@search_revision --fresh
  ./analysis serve --db corpus.metadata.db --assets-dir build/frontend/dist
  ./analysis serve --db corpus.metadata.db --addr 127.0.0.1:8090 --assets-dir build/frontend/dist

`)
}

// Run dispatches explicit command arguments and returns the process exit status.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version())
		return 0
	case "run":
		return runPipelineMain(args[1:], stderr)
	case "recover":
		return recoverMain(args[1:], stderr)
	case "migrate":
		return migrateMain(args[1:], stderr)
	case "serve":
		return serveMain(args[1:], stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q; expected run, recover, migrate, serve, or version\n", args[0])
		return 2
	}
}

// migrateMain applies pending metadata migrations to one existing database.
func migrateMain(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dbPath := flags.String("db", "", "path to an existing SQLite metadata database")
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *dbPath == "" {
		fmt.Fprintln(stderr, "migrate requires --db")
		return 2
	}
	if err := database.MigrateExisting(*dbPath, filepath.Join("config", "database.something")); err != nil {
		log.Error("migrate metadata database", "error", err)
		return 1
	}
	return 0
}

// serveMain serves main.
func serveMain(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	dbPath := flags.String("db", "", "path to an existing SQLite workspace database")
	addr := flags.String("addr", "127.0.0.1:8080", "local address to listen on")
	assetsDir := flags.String("assets-dir", "", "directory of frontend assets to serve (required)")
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *dbPath == "" {
		fmt.Fprintln(stderr, "serve requires --db")
		return 2
	}
	if *assetsDir == "" {
		fmt.Fprintln(stderr, "serve requires --assets-dir")
		return 2
	}
	if err := validateLoopbackAddress(*addr); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	assets, err := frontendAssets(*assetsDir)
	if err != nil {
		log.Error("load frontend assets", "error", err)
		return 1
	}
	viewer, err := server.Open(*dbPath)
	if err != nil {
		log.Error("open viewer database", "error", err)
		return 1
	}
	defer viewer.Close()
	viewer.AssetsFS = assets
	log.Info("serving frontend from filesystem", "assets_dir", *assetsDir)
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen viewer", "addr", *addr, "error", err)
		return 1
	}
	log.Info("viewer listening", "addr", listener.Addr().String())
	if err := viewer.HTTPServer(listener.Addr().String()).Serve(listener); err != nil {
		log.Error("viewer stopped", "error", err)
		return 1
	}
	return 0
}

// validateLoopbackAddress accepts exact IP loopback authorities and rejects names and wildcard hosts.
func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return fmt.Errorf("serve --addr must be host:port")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("serve --addr must use an IPv4 or IPv6 loopback address")
	}
	return nil
}

// frontendAssets validates one frontend asset directory and exposes it as a file system.
func frontendAssets(dir string) (fs.FS, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("inspect frontend asset directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("frontend asset directory %q is not a directory", dir)
	}
	return os.DirFS(dir), nil
}

// runPipelineMain parses run flags, resolves workspace selections, and executes each pipeline workspace.
func runPipelineMain(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := flags.String("config", "", "path to workspace config file")
	dbPath := flags.String("db", "", "path to SQLite database")
	fresh := flags.Bool("fresh", false, "start a new attempt even when a matching plan completed")
	var selectors workspace.StringListFlag
	flags.Var(&selectors, "workspace", "workspace selector search_id@search_revision; repeat to select multiple iterations")
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *configPath == "" || *dbPath == "" {
		fmt.Fprintln(stderr, "run requires --config and --db")
		return 2
	}

	config, err := workspace.Load(*configPath)
	if err != nil {
		log.Error("load workspace config", "error", err)
		return 1
	}
	runs, err := config.Select(selectors)
	if err != nil {
		log.Error("select workspace iterations", "error", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for _, run := range runs {
		if err := workspace.RunPipelineContext(ctx, *dbPath, config.OriginalBytes, run, *fresh); err != nil {
			log.Error("workspace pipeline failed", "workspace", workspace.Selector(run.Manifest.SearchID, run.Manifest.SearchRevision), "error", err)
			return 1
		}
	}
	return 0
}

// recoverMain recovers one abandoned attempt without migrating or executing a workspace.
func recoverMain(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("recover", flag.ContinueOnError)
	dbPath := flags.String("db", "", "existing metadata database")
	runID := flags.Int64("run-id", 0, "abandoned running attempt ID")
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *dbPath == "" || *runID <= 0 || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "recover requires --db and a positive --run-id")
		return 2
	}
	if err := workspace.RecoverAbandonedRun(context.Background(), *dbPath, *runID); err != nil {
		log.Error("recover abandoned attempt", "error", err)
		return 1
	}
	log.Info("abandoned attempt recovered", "run_id", *runID)
	return 0
}
