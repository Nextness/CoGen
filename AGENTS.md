# AGENTS.md - research_analysis

## 1. Start here

This file is the repository entry point for agents. Read the root [Makefile](Makefile) for the supported command contract, [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for system structure and behavior, [docs/DATABASE.md](docs/DATABASE.md) for the current SQLite schema and relationships, [docs/STANDARDS.md](docs/STANDARDS.md) for change and test rules, [docs/PROJECT-USAGE.md](docs/PROJECT-USAGE.md) for developer and operator workflows, and [docs/PROJECT_CATALOG.md](docs/PROJECT_CATALOG.md) for declarations and source locations.

Before frontend work, read [docs/DESIGN.md](docs/DESIGN.md), [docs/CSS-REFERENCE.md](docs/CSS-REFERENCE.md), [docs/APP-USAGE.md](docs/APP-USAGE.md), [docs/JSX-RUNTIME.md](docs/JSX-RUNTIME.md), and [docs/FRONTEND-CODE-STYLE-GUIDE.md](docs/FRONTEND-CODE-STYLE-GUIDE.md); frontend code changes must follow the style guide. Before SOMETHING language changes, read [docs/something.spec.md](docs/something.spec.md). [docs/DOC-STATE.md](docs/DOC-STATE.md) defines documentation review dependencies and acknowledged hashes. OpenCode planning and execution policies are in [.opencode/agent/developer-plan.md](.opencode/agent/developer-plan.md) and [.opencode/agent/developer-execute.md](.opencode/agent/developer-execute.md).

## 2. Project and stack

The repository builds a Go 1.25.0 research-corpus pipeline and a loopback-only local viewer with writable run-scoped review evidence. The Go module and npm workspace are at the repository root; the root Makefile is the supported interface and production relative paths assume the repository root.

The stack is Go, SQLite through `modernc.org/sqlite`, the project-specific SOMETHING configuration language, SQL migrations, standard-library HTTP and JSON, TypeScript ES modules, HTML, and CSS. Frontend tests use Node's built-in test runner with jsdom plus Playwright and axe-core. D3-force and PDF.js are pinned frontend development dependencies delivered through checked-in generated browser assets.

Node.js 22.18 or later and npm are required for frontend development, unit tests, and Playwright. Node's native type stripping executes `.ts` modules in unit tests, and an esbuild loader hook executes `.tsx` modules; type checking is a separate `tsc --noEmit` step via `make check-frontend`.

`modernc.org/sqlite` is the only direct Go runtime dependency. Maintained command-line and repository tools have importable implementations under `tooling/go/` and thin commands under `tooling/go/cmd/`; Node developer scripts live under `tooling/node/`. Do not add dependencies when the standard library or an existing project capability is adequate.

## 3. Supported commands

Run `make help` from the repository root for current targets, variables, and examples.

```sh
make build
make tools
make run
make run FRESH=1
make migrate DB=./corpus.metadata.db
make serve DB=./corpus.metadata.db
make serve DB=./corpus.metadata.db ASSETS_DIR=./build/frontend/dist
make dev
make prepare-to-osf DB=./corpus.metadata.db CONFIG=./config/workspace.something OUT=./build/osf-export
make test
make test-go DOMAIN=backend COMPONENT=server TEST='^TestAPIDetailsGraphModes$'
make test-race
make coverage
make check
make gitleaks
make check-sql
make sql-report
make check-imports
make import-report
make sqlc-install
make sqlc-generate
make sqlc-check
make test-docs
make check-docs
make docs-catalog-update
make docs-state-update
make test-e2e
make test-e2e-live E2E_LIVE=1
make frontend-build
make frontend-classes
make frontend-classes-check
make check-frontend
make test-frontend-unit
make test-frontend-functional
make test-frontend BROWSER=chromium TEST_FILE=tests/frontend/viewer/integration/viewer.spec.ts
make test-frontend-visual
make frontend-pdfjs-vendor-check
```

Run built tools from the repository root:

```sh
./build/analysis run --db ./corpus.metadata.db --config ./config/workspace.something
./build/analysis migrate --db ./corpus.metadata.db
./build/analysis serve --db ./corpus.metadata.db --addr 127.0.0.1:8080
./build/pdf-store add --db ./corpus.metadata.db --doi 10.1000/example --file ./paper.pdf
./build/something-printer --json ./config/workspace.something
./build/doccheck check
./build/rawsqlcheck check
./build/rawsqlcheck report
./build/importcheck check
./build/importcheck report
./build/prepare-osf --db ./corpus.metadata.db --config ./config/workspace.something --out ./build/osf-export
```

There is no separate lint or pre-commit target. CI merge-gate workflows live under `.github/workflows/`; the `ci` workflow runs on every pull request and covers a gitleaks secret scan before backend, race, coverage, documentation, frontend, browser, and offline end-to-end checks, while the manual `e2e-live` workflow runs the opt-in live-provider target after its own gitleaks gate. Format changed Go with `gofmt`, run the narrowest relevant tests, and use `make vet` or `make check`. Use `make test-race` for concurrency, cache, HTTP-client, database, or lifecycle changes. Rebuild when `main` or build behavior changes. Raw SQL calls outside generated `internal/sql/` output must be classified by `make check-sql`, and the package import and transaction graph is enforced by `make check-imports`; `make sql-report` and `make import-report` print the current ownership inventories.

Family SQL is compiled by sqlc, a pinned development generator installed to `build/sqlc` by `make sqlc-install` at the version declared in the root Makefile. Maintained declarations and named queries stay under `backend/database/<family>/schema/` and `backend/pdfstore/schema/`, and generated Go is committed under reserved `internal/sql/` paths, stays private to its owning package tree, and never crosses handwritten family APIs; change the inputs, run `make sqlc-generate` from the repository root with the committed `config/sqlc.yaml` configuration, and review the generated diff instead of editing generated files.

Normal builds and tests stay offline: `make build`, `make tools`, `make test`, and `make check` consume committed generated output without invoking sqlc, while `make sqlc-check` proves generated output is current and every maintained query still passes sqlc analysis after any SQL input change. sqlc is not a Go module dependency and never feeds runtime migrations or SOMETHING database versioning, and `modernc.org/sqlite` remains the only direct runtime database dependency.

## 4. Runtime and configuration essentials

`run` accepts `--db`, `--config`, repeated `--workspace search_id@search_revision`, and `--fresh`. Without selectors it executes every declared workspace in declaration order. A matching completed plan is a no-op by default while normalized DOI inventory and stored search-term match reconciliation still run; the first run after a migration is a full new run because the schema version is part of the execution fingerprint. `--fresh` creates another attempt.

`migrate` prepares both configured stores for an existing metadata database, initializes an unbound default companion, rejects a missing or unsafe bound companion, reconciles normalized DOI inventory, and flushes pending PDF audit delivery without running a workspace or creating a research attempt. `serve` requires `--db`, `--addr`, and `--assets-dir`, requires an exact loopback IP address, holds shared maintenance ownership, verifies both configured schema targets and companion protection contracts without running migrations, opens the existing metadata database through separate read-only and review-write connections, opens the bound PDF companion read-only for evidence plus an optional existing-only write connection, and reports schema and writable capabilities through health. The binary contains no frontend assets; `--assets-dir` is the sole asset source.

`config/workspace.something` includes `config/baseline.something` and declares format-version-2 workspaces, optional reviewer identity, sources, filters, providers, reuse, and cache policy. Reviewer identity is captured per attempt but excluded from manifests and execution fingerprints. `config/database.something` independently selects metadata and PDF migration chains. `config/coverage_policy.something` defines coverage floors.

Runtime commands execute from the repository root and resolve explicit path flags from that working directory. The executable entrypoint is `backend/cmd/analysis/main.go`, which delegates to `backend/cli/`; runtime code does not discover or change the working directory. The process logging threshold is compile-time `logging.MinLevel`, currently `slog.LevelInfo`, and there is no `--log-level` flag.

## 5. Architecture boundaries

- `backend/cmd/analysis/main.go` delegates command dispatch to `backend/cli/`, which delegates pipeline work to `backend/workspace/` and viewing to `backend/server/`.
- `backend/workspace/` owns typed configuration, preflight, attempts, cache policy, ingestion, enrichment orchestration, validation, normalization, metrics, artifacts, audit, and PDF synchronization.
- `backend/database/` owns writable metadata SQLite, migrations, repositories, immutable revisions and relationships, run-scoped review contexts and immutable review versions, cache records, artifacts, metrics, and append-only audit.
- `backend/pdfstore/` owns normalized DOI registration, validated manual PDF insertion, content-addressed bytes, companion migrations, and cross-database audit outbox.
- `backend/server/` owns existing-only metadata and companion connections, paired schema readiness, bounded read and review-mutation APIs, table browsing, read-only PDF evidence delivery, and frontend asset serving; it does not import workspace orchestration, run migrations, create databases, or acquire exclusive maintenance ownership.
- `backend/enrich/` owns provider HTTP and decoding; gather code has no database calls, and workspace storage does not bypass the rate-limited public client.
- `backend/manifest/` owns canonical manifests, fingerprints, lifecycle values, and audit actions without internal application dependencies.
- `backend/something/` owns the language compiler and evaluator; [docs/something.spec.md](docs/something.spec.md) is its behavioral specification.
- `backend/article/`, `backend/bibtex/`, `backend/validation/`, and `backend/normalization/` own parsing, constrained BibTeX, validation, and canonicalization.

SQLite is the pipeline system of record. Work metadata changes create immutable `work_revisions`; ordered `authorships` and `reference_mentions` are canonical relationships. Validation gates normalization, and discarded work remains inspectable through stage and provenance evidence.

The viewer is a URL-state SPA assembled into `build/frontend/dist` from `frontend/src` and served with `serve --assets-dir`. `search_id`, `search_revision_id`, `plan_id`, `run_id`, `note_id`, `anchor_id`, and `pdf_page` are persistent context. Every internal link uses the shared `link` helper; hard-coded `?view=...` links discard context.

## 6. Data, migration, and security essentials

The metadata chain is V00001-V00028 and the PDF chain is V00001-V00003. Add files under the owning migration directory with `-- ==UP==` and `-- ==DOWN==`, append them to the matching SOMETHING chain, and never rewrite an applied migration.

Migration execution follows configured iteration order, skips applied filenames, records but does not revalidate checksums, ignores `previous` and `upgrade` for ordering, and has no downgrade runner. A configured `supersedes` filename adopts an already applied renamed migration by recording the canonical filename without rerunning its SQL; retain both history rows and use this only for an explicitly verified equivalent migration. Keep prerequisite checks before relationship inserts because foreign-key violations are not neutralized by `INSERT OR IGNORE`.

Provider payloads and pipeline artifacts are inline and content-addressed. The pipeline registers normalized DOIs as `not_available`; only `build/pdf-store add` stores validated PDF bytes and changes a DOI to `available`. Manual PDFs are capped at 20,000,000 bytes, require a PDF signature, use SHA-256 identity, and write metadata audit through the outbox.

Keep authentication, authorization, input validation, certificate validation, integrity checks, permission checks, and audit behavior enabled. Do not log or document credentials, tokens, private keys, session identifiers, sensitive personal data, secret environment values, or research content.

## 7. Tests and frontend essentials

Ordinary tests live under `tests/<domain>/<component>/{unit,functional,integration}/`; directories select layers without build tags. `backend/`, `frontend/`, `tooling/`, and `internal/` contain no test suites or test-only hooks. Production packages never import `tests/`. `make test` runs all ordinary Go, Node, and browser suites; `make test-go` selects Go only, and `DOMAIN`, `COMPONENT`, and `LAYER` narrow supported selections. Invalid or empty selections and unmatched Go `TEST` expressions fail. Real-database integration tests use temporary SQLite and production migrations. Provider tests use controlled mock servers and never live APIs. Test counts are derived from source and are not maintained here.

Frontend unit and functional tests live below `tests/frontend/` and use Node, jsdom, and `tests/support/node/setup.ts` where DOM support is required. Playwright viewer and quality suites live under `tests/frontend/`, shared API contracts under `tests/contracts/http/integration/`, and configuration and harnesses under `tests/` and `tests/support/node/`; browser suites run against isolated fixture copies on an operating-system-assigned loopback port, and write unique output under `build/playwright/`. Review mutation specs are serial and never write the generated base fixture.

The `e2e` Go build tag selects `tests/e2e/offline/`; live-provider tests additionally require `e2e_live` and `E2E_LIVE=1`. `make test-e2e` builds and invokes `build/analysis`, keeps deterministic and mocked-provider traffic offline, writes generated database bundles under `build/e2e/`, and runs the guarded Playwright continuity spec. `make test-e2e-live E2E_LIVE=1` is the only test target permitted to contact real Crossref, OpenAlex, and ORCID services; it is opt-in and is not part of default, race, integration, coverage, or frontend targets.

When changing `frontend/`, follow the code style in [docs/FRONTEND-CODE-STYLE-GUIDE.md](docs/FRONTEND-CODE-STYLE-GUIDE.md), then run `make test-frontend-unit`, `make test-go DOMAIN=backend COMPONENT=server`, and the focused Playwright viewer suite. Run `make test-frontend-visual` for visual or accessibility changes and review snapshot changes rather than replacing them blindly.

The checked-in `frontend/vendor/d3-force.js` is generated by `make frontend-vendor`. The checked-in PDF.js core, worker, CMaps, standard fonts, and license assets under `frontend/vendor/pdfjs/` are generated by `make frontend-pdfjs-vendor`. Edit dependency or generation inputs, not the generated assets. `build/frontend/dist` is assembled by `make frontend-build` and is required by `serve`, `dev`, and Playwright targets.

## 8. File and source-control safety

Read repository state and relevant instructions before editing, preserve unrelated changes, modify authoritative sources, and inspect the final diff. Do not add abstractions, configuration, dependencies, compatibility paths, or refactors without a concrete requirement.

`corpus/`, `cache/`, `intermidiate/`, SQLite databases, and `build/` are ignored but may contain real inputs, expensive state, or generated evidence. Treat them as user-owned and read-only by default. The `intermidiate` spelling is part of the current path contract.

Do not commit, push, merge, publish, deploy, release, alter remote state, or use destructive Git or filesystem operations unless explicitly requested.

## 9. Documentation workflow

Keep every prose paragraph, bullet, numbered item, and table row on one physical Markdown line; fenced code and ASCII diagrams may span lines. Documentation describes current supported state only; use Git history for superseded states.

Every maintained catalog declaration requires an attached Go documentation comment or adjacent JavaScript JSDoc; JavaScript test callback titles provide their descriptions. After a declaration or comment changes, run `make docs-catalog-update` and review [docs/PROJECT_CATALOG.md](docs/PROJECT_CATALOG.md). `make check-docs` rejects missing source descriptions.

After a maintained file under `docs/` changes, review the dependencies in [docs/DOC-STATE.md](docs/DOC-STATE.md), then run `make docs-state-update`. Run `make check-docs` before completion; check mode does not write files and validates links, formatting, migrations, exact obsolete references, catalog freshness, and acknowledged hashes.
