# AGENTS.md - research_analysis

## 1. Start here

This file is the repository entry point for agents. Read the root [Makefile](Makefile) for the supported command contract, [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for system structure and behavior, [docs/DATABASE.md](docs/DATABASE.md) for the current SQLite schema and relationships, [docs/STANDARDS.md](docs/STANDARDS.md) for change and test rules, [docs/PROJECT-USAGE.md](docs/PROJECT-USAGE.md) for developer and operator workflows, and [docs/PROJECT_CATALOG.md](docs/PROJECT_CATALOG.md) for declarations and source locations.

Before frontend work, read [docs/DESIGN.md](docs/DESIGN.md), [docs/CSS-REFERENCE.md](docs/CSS-REFERENCE.md), and [docs/APP-USAGE.md](docs/APP-USAGE.md). Before SOMETHING language changes, read [docs/something.spec.md](docs/something.spec.md). [docs/DOC-STATE.md](docs/DOC-STATE.md) defines documentation review dependencies and acknowledged hashes. OpenCode planning and execution policies are in [.opencode/agent/developer-plan.md](.opencode/agent/developer-plan.md) and [.opencode/agent/developer-execute.md](.opencode/agent/developer-execute.md).

## 2. Project and stack

The repository builds a Go 1.25.0 research-corpus pipeline and a local read-only viewer. The Go module is `src/`, but the root Makefile is the supported interface and production relative paths assume the repository root.

The stack is Go, SQLite through `modernc.org/sqlite`, the project-specific SOMETHING configuration language, SQL migrations, standard-library HTTP and JSON, vanilla JavaScript ES modules, HTML, and CSS. Frontend tests use Node's built-in test runner with jsdom plus Playwright and axe-core. D3-force is pinned as a frontend development dependency and delivered through a checked-in generated browser bundle.

`modernc.org/sqlite` is the only direct Go runtime dependency. Maintained command-line and repository tools are Go packages under `src/tools/`. Do not add dependencies when the standard library or an existing project capability is adequate.

## 3. Supported commands

Run `make help` from the repository root for current targets, variables, and examples.

```sh
make build
make tools
make run
make run FRESH=1
make serve DB=./corpus.metadata.db
make serve DB=./corpus.metadata.db ASSETS_DIR=./src/server/frontend
make dev
make test
make test-go PACKAGE=./server TEST='^TestGraph$'
make test-race
make coverage
make check
make test-docs
make check-docs
make docs-catalog-update
make docs-state-update
make test-e2e
make test-e2e-live E2E_LIVE=1
make test-frontend-unit
make test-frontend BROWSER=chromium TEST_FILE=tests/viewer.spec.cjs
make test-frontend-visual
```

Run built tools from the repository root:

```sh
./build/analysis run --db ./corpus.metadata.db --config ./config/workspace.something
./build/analysis serve --db ./corpus.metadata.db --addr 127.0.0.1:8080
./build/pdf-store add --db ./corpus.metadata.db --doi 10.1000/example --file ./paper.pdf
./build/something-json ./config/workspace.something
./build/doccheck check
```

There is no separate lint, pre-commit, or CI target. Format changed Go with `gofmt`, run the narrowest relevant tests, and use `make vet` or `make check`. Use `make test-race` for concurrency, cache, HTTP-client, database, or lifecycle changes. Rebuild when `main` or build behavior changes.

## 4. Runtime and configuration essentials

`run` accepts `--db`, `--config`, repeated `--workspace search_id@search_revision`, and `--fresh`. Without selectors it executes every declared workspace in declaration order. A matching completed plan is a no-op by default while normalized DOI inventory reconciliation still runs; `--fresh` creates another attempt.

`serve` accepts `--db`, `--addr`, and `--assets-dir`, defaults to loopback, opens an existing database read-only, and never calls writable `database.Open`, runs migrations, enables WAL, or creates a database. Filesystem assets are explicit; otherwise the embedded frontend is served.

`config/workspace.something` includes `config/baseline.something` and declares format-version-2 workspaces, sources, filters, providers, reuse, and cache policy. `config/database.something` independently selects metadata and PDF migration chains. `config/coverage_policy.something` defines coverage floors.

`src/main.go` moves one directory upward only when its current directory ends in `/src`; do not rely on automatic path discovery elsewhere. The process logging threshold is compile-time `logging.MinLevel`, currently `slog.LevelInfo`, and there is no `--log-level` flag.

## 5. Architecture boundaries

- `src/main.go` is the only production executable entry and delegates pipeline work to `src/workspace/` and viewing to `src/server/`.
- `src/workspace/` owns typed configuration, preflight, attempts, cache policy, ingestion, enrichment orchestration, validation, normalization, metrics, artifacts, audit, and PDF synchronization.
- `src/database/` owns writable metadata SQLite, migrations, repositories, immutable revisions and relationships, cache records, artifacts, metrics, and append-only audit.
- `src/pdfstore/` owns normalized DOI registration, validated manual PDF insertion, content-addressed bytes, companion migrations, and cross-database audit outbox.
- `src/server/` owns existing-only read-only SQLite, bounded GET and HEAD APIs, table browsing, PDF delivery, and embedded frontend; it does not import workspace orchestration or open writable databases.
- `src/enrich/` owns provider HTTP and decoding; gather code has no database calls, and workspace storage does not bypass the rate-limited public client.
- `src/manifest/` owns canonical manifests, fingerprints, lifecycle values, and audit actions without internal application dependencies.
- `src/something/` owns the language compiler and evaluator; [docs/something.spec.md](docs/something.spec.md) is its behavioral specification.
- `src/article/`, `src/bibtex/`, `src/validation/`, and `src/normalization/` own parsing, constrained BibTeX, validation, and canonicalization.

SQLite is the pipeline system of record. Work metadata changes create immutable `work_revisions`; ordered `authorships` and `reference_mentions` are canonical relationships. Validation gates normalization, and discarded work remains inspectable through stage and provenance evidence.

The embedded viewer is a URL-state SPA. `search_id`, `search_revision_id`, `plan_id`, and `run_id` are persistent context. Every internal link uses the shared `link` helper; hard-coded `?view=...` links discard context.

## 6. Data, migration, and security essentials

The metadata chain is V00001-V00021 and the PDF chain is V00001-V00002. Add files under the owning migration directory with `-- ==UP==` and `-- ==DOWN==`, append them to the matching SOMETHING chain, and never rewrite an applied migration.

Migration execution follows configured iteration order, skips applied filenames, records but does not revalidate checksums, ignores `previous` and `upgrade` for ordering, and has no downgrade runner. Keep prerequisite checks before relationship inserts because foreign-key violations are not neutralized by `INSERT OR IGNORE`.

Provider payloads and pipeline artifacts are inline and content-addressed. The pipeline registers normalized DOIs as `not_available`; only `build/pdf-store add` stores validated PDF bytes and changes a DOI to `available`. Manual PDFs are capped at 20,000,000 bytes, require a PDF signature, use SHA-256 identity, and write metadata audit through the outbox.

Keep authentication, authorization, input validation, certificate validation, integrity checks, permission checks, and audit behavior enabled. Do not log or document credentials, tokens, private keys, session identifiers, sensitive personal data, secret environment values, or research content.

## 7. Tests and frontend essentials

Go tests use `unit`, `functional`, and `integration` build tags. Real-database integration tests use temporary SQLite and production migrations. Provider tests use controlled mock servers and never live APIs. Test counts are derived from source and are not maintained here.

Frontend unit tests live under `frontend/tests/unit/` and use Node, jsdom, and `setup.js` where DOM support is required. Playwright tests live under `frontend/tests/`, run against an isolated fixture-backed viewer on an operating-system-assigned loopback port, and write unique output under `build/playwright/`.

The `e2e` Go build tag is reserved for the supported pipeline-to-viewer flow. `make test-e2e` builds and invokes `build/analysis`, keeps deterministic and mocked-provider traffic offline, writes generated database bundles under `build/e2e/`, and runs the guarded Playwright continuity spec. `make test-e2e-live E2E_LIVE=1` is the only test target permitted to contact real Crossref, OpenAlex, and ORCID services; it is opt-in and is not part of default, race, integration, coverage, or frontend targets.

When changing `src/server/frontend/`, run `make test-frontend-unit`, `make test-go PACKAGE=./server`, and the focused Playwright viewer suite. Run `make test-frontend-visual` for visual or accessibility changes and review snapshot changes rather than replacing them blindly.

The checked-in `src/server/frontend/vendor/d3-force.js` is generated by `make frontend-vendor`; edit its dependency or build inputs, not the bundle.

## 8. File and source-control safety

Read repository state and relevant instructions before editing, preserve unrelated changes, modify authoritative sources, and inspect the final diff. Do not add abstractions, configuration, dependencies, compatibility paths, or refactors without a concrete requirement.

`corpus/`, `cache/`, `intermidiate/`, SQLite databases, and `build/` are ignored but may contain real inputs, expensive state, or generated evidence. Treat them as user-owned and read-only by default. The `intermidiate` spelling is part of the current path contract.

Do not commit, push, merge, publish, deploy, release, alter remote state, or use destructive Git or filesystem operations unless explicitly requested.

## 9. Documentation workflow

Keep every prose paragraph, bullet, numbered item, and table row on one physical Markdown line; fenced code and ASCII diagrams may span lines. Documentation describes current supported state only; use Git history for superseded states.

Every maintained catalog declaration requires an attached Go documentation comment or adjacent JavaScript JSDoc; JavaScript test callback titles provide their descriptions. After a declaration or comment changes, run `make docs-catalog-update` and review [docs/PROJECT_CATALOG.md](docs/PROJECT_CATALOG.md). `make check-docs` rejects missing source descriptions.

After a maintained file under `docs/` changes, review the dependencies in [docs/DOC-STATE.md](docs/DOC-STATE.md), then run `make docs-state-update`. Run `make check-docs` before completion; check mode does not write files and validates links, formatting, migrations, exact obsolete references, catalog freshness, and acknowledged hashes.
