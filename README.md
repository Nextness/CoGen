# CoGen - Corpus Generator

CoGen is a Go-based research-corpus pipeline that turns literature exports from Scopus, IEEE Xplore, and Web of Science into an immutable, provenance-rich SQLite corpus. It deduplicates articles, enriches them through Crossref, OpenAlex, and ORCID, validates and normalizes metadata, and records every step as content-addressed artifacts and append-only audit events. A loopback-only local viewer lets you browse the corpus and record run-scoped reviews, versioned notes, links, and PDF anchors.

## Quick start

Requirements: Go 1.25+ and Node.js 22.18+ (the viewer and frontend need Node and npm).

```sh
make build           # build the pipeline binary
make run             # run the configured workspaces (creates corpus.metadata.db)
make frontend-build  # assemble the viewer assets
make serve DB=corpus.metadata.db
```

Open `http://127.0.0.1:8080` in a browser. To preview the viewer without pipeline data, run `make dev` instead.

`make run` reads the export files declared in `config/workspace.something`. Put your Scopus and IEEE Xplore CSV exports and your Web of Science BibTeX export in `corpus/`, and point each source's `expected_file` at them. Run `make help` for every target, variable, and example.

## What CoGen does

- Ingests Scopus and IEEE Xplore CSV exports and Web of Science BibTeX exports.
- Deduplicates, enriches, validates, and normalizes article metadata.
- Persists an immutable SQLite corpus with content-addressed artifacts and append-only audit events.
- Serves a loopback-only viewer for browsing evidence and recording run-scoped reviews.

## Documentation

In-depth guides are planned for the project wiki. Until it exists, the authoritative documents live under `docs/`:

- [docs/PROJECT-USAGE.md](docs/PROJECT-USAGE.md) - developer and operator workflows.
- [docs/APP-USAGE.md](docs/APP-USAGE.md) - viewer usage, note syntax, and review workflow.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - system structure and behavior.

## Requirements

- Go 1.25.0 or a compatible later toolchain for the backend, tooling, tests, and builds.
- Node.js 22.18 or later and npm for the frontend, unit tests, and Playwright browser tests.

## License

MIT. See [LICENSE](LICENSE).
