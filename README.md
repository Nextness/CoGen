# Research Analysis

A Go-based research-corpus pipeline that turns Scopus, IEEE Xplore, and Web of Science exports into an immutable, provenance-rich SQLite corpus, with a local read-only web viewer for browsing works, relationships, provenance, and evaluation evidence.

The pipeline parses and deduplicates articles, optionally enriches them through Crossref, OpenAlex, and ORCID, validates and normalizes metadata, registers normalized DOIs in a companion PDF inventory, and records artifacts, metrics, and append-only audit events. The same binary serves the embedded viewer over an existing metadata database.

## Requirements

- Go 1.25.0 or a compatible later toolchain for the backend, tooling, tests, and builds.
- Node.js 18 or later and npm for frontend development, unit tests, and Playwright browser tests.
- Playwright browser binaries for the browser test projects.

## Developer setup

Install locked frontend dependencies and browser binaries when frontend work requires them:

```sh
make frontend-install
make frontend-browsers BROWSERS="chromium firefox webkit"
```

Build the pipeline binary and the maintained tools:

```sh
make build
make tools
```

Run `make help` for the full list of targets, variables, and examples. See [docs/PROJECT-USAGE.md](docs/PROJECT-USAGE.md) for developer and operator workflows and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for system structure.

## Data input

The pipeline reads raw literature exports from the `corpus/` directory at the repository root. Each configured source expects one export file in a specific format:

- Scopus: a CSV export saved as `corpus/scopus.raw.csv`.
- Web of Science: a BibTeX export saved as `corpus/wos.raw.bib`.
- IEEE Xplore: a CSV export saved as `corpus/ieeexplore.raw.csv`.

The expected file name, format, and location are declared per source in the `sources` list of `config/workspace.something`. To point a source at a different file, update its `expected_file` field, and set `file_type` to `"csv"` or `"bib"` to match the export format. Paths are resolved relative to the configuration directory, so `../corpus/scopus.raw.csv` in the config refers to `corpus/scopus.raw.csv` at the repository root.

## Usage (non-developers)

Run the configured workspaces to build the corpus:

```sh
make run
```

Serve the viewer over an existing metadata database:

```sh
make serve DB=corpus.metadata.db
```

Open http://127.0.0.1:8080 in a browser. `make dev` serves a bundled fixture database with filesystem assets for local exploration. See [docs/APP-USAGE.md](docs/APP-USAGE.md) for viewer navigation and data interpretation.

## License

MIT. See [LICENSE](LICENSE).
