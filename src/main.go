// main.go is the sole executable entrypoint. It dispatches to the
// workspace pipeline (run) or the read-only local viewer (serve)
// based on the first command-line argument.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"

	"analysis/database"
	"analysis/dev"
	"analysis/logging"
	"analysis/server"
	"analysis/workspace"
)

var log = logging.Logger("pipeline")

// major, minor, and patch are the semantic version components of the binary.
// Bump them for each release; the version command prints MAJOR.MINOR.PATCH.
const major int = 1
const minor int = 0
const patch int = 0

// version returns the semantic version string, appending "-development" for
// development builds so release and dev binaries are distinguishable.
func version() string {
	v := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	if dev.Mode {
		v += "-development"
	}
	return v
}

// usage writes the supported command syntax to standard error.
func usage() {
	fmt.Fprintf(os.Stderr, `Usage: analysis <command> [options]

analysis runs a research-corpus pipeline ("run"), migrates an existing
metadata database ("migrate"), or starts the local viewer ("serve").

Commands:

  version Print the semantic version (MAJOR.MINOR.PATCH, with a
          "-development" suffix for development builds).

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

  migrate Apply pending metadata migrations to an existing database
          without running a workspace.

          Flags:
            --db <path>        Path to an existing SQLite metadata
                               database (required).

  serve   Start the loopback-only review viewer for an existing workspace
          database. Binds to a loopback address by default; the viewer
          serves the evaluation table, corpus browser, and graph views.

          Flags:
            --db <path>        Path to an existing SQLite workspace
                               database (required).
            --addr <host:port> Local address to listen on
                               (default "127.0.0.1:8080").
            --assets-dir
            <dir>              Serve frontend assets from a filesystem
                               directory instead of embedded assets.
                               Use with "make dev" for hot refresh.

Examples:

  ./analysis run --config config/workspace.something --db corpus.metadata.db
  ./analysis migrate --db corpus.metadata.db
  ./analysis run --config config/workspace.something --db corpus.metadata.db \
      --workspace search_id@search_revision --fresh
  ./analysis serve --db corpus.metadata.db
  ./analysis serve --db corpus.metadata.db --addr 127.0.0.1:8090
  ./analysis serve --db corpus.metadata.db --assets-dir src/server/frontend

`)
}

// main dispatches the analysis command selected by process arguments and exits on command failure.
func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		usage()
		os.Exit(2)
	}
	command := os.Args[1]
	if command == "version" {
		fmt.Println(version())
		return
	}
	if command != "run" && command != "serve" && command != "migrate" {
		fmt.Fprintf(os.Stderr, "unknown command %q; expected run, migrate, serve, or version\n", command)
		os.Exit(2)
	}
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	if command == "run" {
		runPipelineMain()
	} else if command == "migrate" {
		migrateMain()
	} else {
		serveMain()
	}
}

// migrateMain applies pending metadata migrations to one existing database.
func migrateMain() {
	flags := flag.NewFlagSet("migrate", flag.ExitOnError)
	dbPath := flags.String("db", "", "path to an existing SQLite metadata database")
	_ = flags.Parse(os.Args[1:])
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "migrate requires --db")
		os.Exit(2)
	}
	changeToRepositoryRoot()
	if err := database.MigrateExisting(*dbPath, filepath.Join("config", "database.something")); err != nil {
		log.Error("migrate metadata database", "error", err)
		os.Exit(1)
	}
}

// serveMain serves main.
func serveMain() {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := flags.String("db", "", "path to an existing SQLite workspace database")
	addr := flags.String("addr", "127.0.0.1:8080", "local address to listen on")
	assetsDir := flags.String("assets-dir", "", "directory of frontend assets to serve instead of embedded assets")
	_ = flags.Parse(os.Args[1:])
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "serve requires --db")
		os.Exit(2)
	}
	if err := validateLoopbackAddress(*addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	assets, err := frontendAssets(*assetsDir)
	if err != nil {
		log.Error("load frontend assets", "error", err)
		os.Exit(1)
	}
	viewer, err := server.Open(*dbPath)
	if err != nil {
		log.Error("open viewer database", "error", err)
		os.Exit(1)
	}
	defer viewer.Close()
	if assets != nil {
		viewer.AssetsFS = assets
		log.Info("serving frontend from filesystem", "assets_dir", *assetsDir)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("listen viewer", "addr", *addr, "error", err)
		os.Exit(1)
	}
	log.Info("viewer listening", "addr", listener.Addr().String())
	if err := viewer.HTTPServer(listener.Addr().String()).Serve(listener); err != nil {
		log.Error("viewer stopped", "error", err)
		os.Exit(1)
	}
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

// frontendAssets returns either explicit filesystem assets or the embedded production frontend.
func frontendAssets(dir string) (fs.FS, error) {
	if dir == "" {
		return nil, nil
	}
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
func runPipelineMain() {
	configPath := flag.String("config", "", "path to workspace config file")
	dbPath := flag.String("db", "", "path to SQLite database")
	fresh := flag.Bool("fresh", false, "start a new attempt even when a matching plan completed")
	var selectors workspace.StringListFlag
	flag.Var(&selectors, "workspace", "workspace selector search_id@search_revision; repeat to select multiple iterations")
	flag.Parse()
	if *configPath == "" || *dbPath == "" {
		fmt.Fprintln(os.Stderr, "run requires --config and --db")
		os.Exit(2)
	}

	changeToRepositoryRoot()

	config, err := workspace.Load(*configPath)
	if err != nil {
		log.Error("load workspace config", "error", err)
		os.Exit(1)
	}
	runs, err := config.Select(selectors)
	if err != nil {
		log.Error("select workspace iterations", "error", err)
		os.Exit(1)
	}
	for _, run := range runs {
		if err := workspace.RunPipeline(*dbPath, config.OriginalBytes, run, *fresh); err != nil {
			log.Error("workspace pipeline failed", "workspace", workspace.Selector(run.Manifest.SearchID, run.Manifest.SearchRevision), "error", err)
			os.Exit(1)
		}
	}
}

// changeToRepositoryRoot moves one directory upward only when execution starts inside the module directory.
func changeToRepositoryRoot() {
	if cwd, _ := os.Getwd(); strings.HasSuffix(cwd, "/src") || cwd == "src" {
		if err := os.Chdir(".."); err == nil {
			log.Debug("changed working directory to project root")
		}
	}
}
