# Project and Tool Usage

## 1. Purpose

This document explains how developers and operators use the repository, its Makefile, executables, configuration, fixtures, tests, and documentation workflow. Use [AGENTS.md](../AGENTS.md) for initial orientation, [ARCHITECTURE.md](ARCHITECTURE.md) for system behavior, [DATABASE.md](DATABASE.md) for schema and persistence details, [STANDARDS.md](STANDARDS.md) for normative change rules, and [APP-USAGE.md](APP-USAGE.md) for the viewer experience.

## 2. Prerequisites and working directory

- Go 1.25.0 or a compatible later toolchain is required for backend, tooling, tests, and builds.
- Node.js 22.18 or later and npm are required only for frontend development, unit tests, dependency maintenance, and Playwright.
- Playwright browser binaries are required for the browser projects being tested.
- Supported commands run from the repository root because configuration, migrations, databases, frontend assets, and build output use repository-relative paths.

Install locked frontend dependencies and selected browser binaries when frontend work requires them:

```sh
make frontend-install
make frontend-browsers BROWSERS="chromium firefox"
```

`frontend-browsers` installs browser binaries; platform-native libraries remain an environment responsibility described by Playwright when missing.

## 3. Makefile interface

Run `make help` for the current targets, variables, and examples. The root [Makefile](../Makefile) is the supported interface and runs Go module commands from the repository root. Root `npm ci` installs the frontend, tooling, and testing workspaces from one lockfile.

```sh
make build
make tools
make run
make migrate DB=corpus.metadata.db
make serve DB=corpus.metadata.db
make dev
make prepare-to-osf DB=corpus.metadata.db CONFIG=config/workspace.something OUT=build/osf-export
make test
make check
```

All intentional executables are written beneath `build/`; routine work does not use a bare package-level `go build`. `build/` can contain useful reports and generated evidence, so inspect it before running `make clean`.

## 4. Build outputs and tools

| Output | Build target | Purpose |
|---|---|---|
| `build/analysis` | `make build` | Runs the pipeline, migrates an existing metadata database, serves the loopback-only local review viewer, or runs the internal PDF annotation worker. |
| `build/something-printer` | `make something-printer` or `make tools` | Evaluates one SOMETHING file and prints the evaluated public values as SOMETHING, JSON, or YAML. |
| `build/pdf-store` | `make pdf-store` or `make tools` | Inserts one validated, already registered PDF into the companion store. |
| `build/pdf-export` | `make pdf-export` or `make tools` | Exports stored PDFs selected by DOI to filenames derived from their paper titles, opening both corpus databases read-only. |
| `build/prepare-osf` | `make prepare-osf` or `make tools` | Creates one sanitized copy-only metadata, PDF, and optional configuration export. |
| `build/doccheck` | `make doccheck` or `make tools` | Checks and updates maintained documentation artifacts. |
| `build/coveragecheck` | `make coveragecheck`, `make tools`, or `make coverage` | Enforces configured package and file coverage floors. |
| `build/rawsqlcheck` | `make rawsqlcheck`, `make tools`, `make check-sql`, or `make sql-report` | Checks and reports raw SQL ownership across maintained Go sources. |
| `build/importcheck` | `make importcheck`, `make tools`, `make check-imports`, or `make import-report` | Checks and reports the package import and transaction graph. |
| `build/testselect` | `make testselect`, `make tools`, or ordinary test targets | Discovers domain/component/layer suites and invokes their standard runners with strict focused-selection validation. |
| `build/sqlc` | `make sqlc-install` | Pinned sqlc development generator used only for family SQL code generation and freshness checking; normal builds, tests, and checks never invoke it. |
| `build/gitleaks` | `make gitleaks-install` or `make gitleaks` | Scans repository history and the working tree for secrets; part of `make check` and the CI merge gate. |
| `build/coverage/coverage.out` | `make coverage` | Atomic Go coverage profile used by the policy checker. |
| `build/e2e/<variant>/` | `make test-e2e` or `make test-e2e-live E2E_LIVE=1` | Generated metadata and companion PDF databases, evaluated workspace configuration, and preserved review mutation evidence for pipeline-to-viewer verification. |
| `build/playwright/run-*/` | Browser test targets | Isolated traces, screenshots, results, and HTML reports. |

`build/coveragecheck` is normally executed by `make coverage`; developers should use the Make target so it receives the correct profile and policy paths.

`build/rawsqlcheck` is normally executed by `make check-sql` or `make sql-report`; developers should use the Make targets so it scans the repository root with the committed ownership table.

`build/importcheck` is normally executed by `make check-imports` or `make import-report`; developers should use the Make targets so it scans the repository root with the committed layer rules.

`build/sqlc` is normally executed by `make sqlc-generate` or `make sqlc-check`; both targets pass `-f config/sqlc.yaml` from the repository root. Section 11 documents the generation workflow, maintained versus generated files, and common inference problems.

`build/analysis version` prints the semantic version `MAJOR.MINOR.PATCH` (currently `1.0.0`).

`build/analysis pdf-annotate` is the internal worker mode used by the annotation trial and integration tests. It reads one JSON request containing base64-encoded PDF bytes and application annotation data from standard input and writes annotated PDF bytes to standard output. It does not open databases or accept operator file paths.

## 5. Pipeline execution

Build and run the configured workspaces:

```sh
make run DB=corpus.metadata.db CONFIG=config/workspace.something
make run DB=corpus.metadata.db CONFIG=config/workspace.something WORKSPACE=search_id@revision
make run DB=corpus.metadata.db CONFIG=config/workspace.something FRESH=1
```

The equivalent executable form is:

```sh
./build/analysis run --db ./corpus.metadata.db --config ./config/workspace.something
./build/analysis run --db ./corpus.metadata.db --config ./config/workspace.something --workspace search_id@revision --fresh
```

Without `WORKSPACE` or `--workspace`, every declared workspace runs in declaration order. Repeat `--workspace` to select multiple declarations. A matching completed plan is reused as a no-op by default while DOI inventory and stored search-term match reconciliation still run; the first run after a migration is a full new run because the schema version is part of the execution fingerprint. `FRESH=1` or `--fresh` creates another attempt.

`config/workspace.something` is the normal workspace entry and includes `config/baseline.something`. Each workspace may provide optional `reviewer = reviewer_config { username = "...", email = "..." }`; omitted values default to empty, are trimmed and bounded, are captured for each attempt, and do not affect manifests or execution fingerprints. `config/database.something` independently selects metadata and PDF migration chains. Paths supplied in source declarations must resolve in the repository-root runtime context.

If a process crashes or is killed while an attempt is running, stop any older pipeline binaries and recover that attempt with `make recover DB=corpus.metadata.db RUN_ID=123` or `./build/analysis recover --db ./corpus.metadata.db --run-id 123`, then retry the original run command. Recovery requires exclusive ownership of the metadata file and records an operator `run_failed` event. `--fresh` never bypasses an active attempt. SIGINT and SIGTERM cancel the current pipeline and finalize failure automatically. Pipeline execution and recovery require Linux, macOS, or a BSD system with `flock`; older binaries do not participate in this ownership lock.

Provider `fields` restricts enrichment application and dependent lookups; an omitted or empty list retains all-field behavior. Crossref supports `title`, `authors`, `publisher`, and `references` (`reference` is also accepted). OpenAlex additionally supports `abstract`, `citation_count`, and `reference_count`. ORCID supports `orcid`, `display_name`, `institution`, `works_count`, `cited_by_count`, `h_index`, and `i10_index`; name searches require `orcid`, name updates require `display_name`, and affiliation updates require `institution`. Invalid names are rejected before requests. Raw provider payloads remain complete provenance evidence.

## 6. Viewer operation and development

Apply pending metadata migrations to an existing database before serving it:

```sh
make migrate DB=corpus.metadata.db
./build/analysis migrate --db ./corpus.metadata.db
```

The migration command rejects a missing database and does not run a workspace. When a configured migration explicitly supersedes an equivalent earlier filename already recorded in the database, migration retains the earlier history row and records the canonical filename without rerunning the SQL; operators must not edit migration history manually. The viewer verifies required review tables and append-only triggers at startup but never migrates or repairs the database itself.

Serve an existing metadata database with the assembled production assets:

```sh
make frontend-build
make serve DB=corpus.metadata.db ADDR=127.0.0.1:8080 ASSETS_DIR=build/frontend/dist
./build/analysis serve --db ./corpus.metadata.db --addr 127.0.0.1:8080 --assets-dir ./build/frontend/dist
```

Serve the assembled assets, overriding the default asset directory:

```sh
make serve DB=corpus.metadata.db ASSETS_DIR=build/frontend/dist
make dev
```

`make dev` builds the frontend assets, generates the default fixture pair, copies the ignored generated fixture metadata and PDF pair to a disposable directory, and serves that copy with the assembled assets on the default loopback address. The server writes only run-scoped review evidence to existing metadata, keeps pipeline evidence and the companion PDF database read-only, rejects non-loopback addresses, and rejects requests whose Host authority differs from the bound listener. Frontend work edits the TypeScript sources under `frontend/src`, runs `make frontend-build` to assemble `build/frontend/dist`, and serves that directory; `make check-frontend` verifies CSS class-registry freshness, validates statically visible class uses, and type-checks the sources, while unit tests import them directly without a build step.

When a frontend change adds or removes a CSS selector, update the owning stylesheet and use the supported registry targets:

```sh
make frontend-classes
make frontend-classes-check
make check-frontend
```

`make frontend-classes` rewrites the committed `frontend/src/jsx/classes.ts` registry from the six authoritative stylesheets. `make frontend-classes-check` is the non-mutating proof that the registry is fresh, validates non-JSX class operations, and reports defined tokens without a statically visible authored use. Review the generated diff and do not edit the registry directly.

Use [APP-USAGE.md](APP-USAGE.md) for review-context initialization, status and note history, note syntax, embedded PDF anchors, navigation, data interpretation, privacy, graph limits, and operational expectations.

## 7. Manual PDF inventory

Acquire documents through an authorized source, then use the validated manual write path:

```sh
make pdf-store
./build/pdf-store add --db corpus.metadata.db --doi 10.1000/example --file ./paper.pdf
```

The DOI must already have a normalized work revision and registered inventory row. The tool enforces the 20,000,000-byte limit and PDF signature, content-addresses identical bytes, changes the inventory state to `available`, records `inventoried_at`, and delivers the manual audit event. Re-adding an available DOI leaves its selected bytes unchanged.

Treat `corpus.metadata.db` and its bound `corpus.pdf.db` as one portable bundle. Stop writers for a simple file copy or use SQLite backup facilities for coordinated online backup. The Makefile `database-backup` target copies both configured database filenames to `../backup_databases/` and assumes that destination already exists.

Export existing PDF bytes by DOI without changing either database:

```sh
make pdf-export
./build/pdf-export --db ./corpus.metadata.db --doi 10.1000/example --out ./build/papers
./build/pdf-export --db ./corpus.metadata.db --doi 10.1000/first --doi https://doi.org/10.1000/second --out ./build/papers
```

The export tool requires an existing metadata database and existing bundle-relative PDF binding; it never runs migrations, registers documents, or flushes audit events. Repeated DOI selectors are normalized and deduplicated. Each available PDF must pass the stored byte-size, PDF-signature, and SHA-256 checks before export. Filenames use the latest nonempty normalized revision title, falling back to the latest recorded title when no normalized title exists; unsafe filename characters are replaced and long names are bounded without breaking Unicode. The tool creates the output directory, accepts an identical existing export, and refuses conflicting files or symlinks. Standard output is a JSON array of successful exports with DOI, path, content hash, byte size, and exported or unchanged status. Missing titles, unavailable PDFs, and other per-DOI failures are reported on standard error with exit status 1 while other valid requested exports are retained; invalid command syntax exits with status 2.

## 8. Sanitized OSF export

Create a new sanitized bundle without modifying or locking in changes to the source metadata database, PDF database, configuration, or browser drafts:

```sh
make prepare-to-osf DB=corpus.metadata.db OUT=build/osf-export
make prepare-to-osf DB=corpus.metadata.db CONFIG=config/workspace.something OUT=build/osf-export
./build/prepare-osf --db ./corpus.metadata.db --config ./config/workspace.something --out ./build/osf-export
```

`DB` and `OUT` are required. The Make target includes configuration only when `CONFIG` is explicitly supplied on the command line. `OUT` must not exist. The tool resolves and validates the bound PDF companion, creates WAL-safe snapshots in a temporary sibling directory, redacts copied reviewer rows and syntax-recognized inline reviewer configuration, replaces reviewer-bearing raw configuration artifacts by content hash, regenerates only the copied corpus ID, validates hashes and foreign keys, writes `export-manifest.json`, and atomically publishes the directory.

Configuration includes must remain inside the main configuration directory and must not escape through symbolic links. Macro-backed, referenced, or otherwise unprovable reviewer assignments fail closed. The manifest records source and sanitized schema versions, hashes for every copied file, raw-config hash mappings, and the explicit limitation that browser drafts and arbitrary research text were not scanned. Append-only audit rows remain unchanged, and the PDF snapshot receives no review schema or sanitization.

## 9. SOMETHING inspection

Build and inspect a configuration without running the pipeline:

```sh
make something-printer
./build/something-printer ./config/workspace.something
./build/something-printer --json ./config/workspace.something
./build/something-printer --yaml ./config/workspace.something
```

The tool executes the same public SOMETHING loading boundary as the application and prints evaluated public values. The default and explicit `--something` format renders the evaluated result in SOMETHING setup syntax, while `--json` and `--yaml` select the other supported serializations. The format flags are mutually exclusive. Use [something.spec.md](something.spec.md) for syntax and semantics.

## 10. Focused and broad Go verification

```sh
make fmt
make format-check
make vet
make test-go DOMAIN=backend COMPONENT=server TEST='^TestAPIDetailsGraphModes$'
make test
make test-race
make coverage
make check-sql
make sql-report
make check-imports
make import-report
make sqlc-check
make check
```

`make test-go` enables unit, functional, and integration tags for the selected package and disables result caching. `make test` runs the same tag set across the module. Use `make test-race` for concurrency, cache, HTTP-client, database, or lifecycle changes.

`make coverage` writes `build/coverage/coverage.out` and enforces [config/coverage_policy.something](../config/coverage_policy.something). The checker merges duplicate source ranges using the highest hit count and reports current, required, and delta coverage for tracked packages and high-risk files.

`make check` runs the Go formatting check, `go vet`, the raw SQL ownership check, the import graph check, the complete non-mutating documentation check, and the gitleaks secret scan. It does not run all Go or frontend tests, so it supplements rather than replaces behavior tests. `make gitleaks` installs gitleaks into `build/gitleaks` on first use and scans the repository history and working tree for secrets; the same target gates the CI workflows.

`make check-sql` parses maintained Go files under `backend/`, `tooling/go/`, `internal/`, and `tests/` outside generated `internal/sql/` output and requires each Query, QueryRow, Exec, or prepared-statement call to be classified as platform migrations or connection behavior, viewer query-budget plumbing, dynamic table introspection, sanitized export maintenance, cross-database outbox savepoints, a retained `*_dynamic.go` reader, or a direct integrity test. An unclassified call fails with the file, line, method, and expected remediation, and a stale ownership entry fails when its file disappears or keeps no calls. `make sql-report` prints the same classification as a reviewable ownership report with per-file reasons.

`make check-imports` parses maintained Go packages under `backend/`, `tooling/go/`, `internal/`, and `tests/`, classifies it as a metadata family, the composition root, the companion PDF store, a consumer, or a shared low-level package, and enforces the approved dependency direction. Family production code may depend only on its own subtree and shared internal, logging, manifest, or notes packages, so sibling families cannot call one another inside transactions; test fixtures may compose the composition root and sibling families. Generated `internal/sql` output is imported only inside its owning package tree, and every sqlc generation unit keeps its schema, queries, and generated output under one owner directory. `make import-report` prints the layer graph, family production imports, fixture composition, sqlc units, and verified invariants.

`make sqlc-check` proves committed family generated output is current and every maintained query passes sqlc analysis; section 11 documents generation, maintained versus generated files, and common inference problems.

Ordinary suites are owned by `tests/<domain>/<component>/<layer>/`. `DOMAIN` accepts `all`, `backend`, `frontend`, `tooling`, `shared`, or `contracts`; `LAYER` accepts `all`, `unit`, `functional`, or `integration`. `COMPONENT` selects a relative subtree within one explicit domain. The general default `make test` includes ordinary Go, Node, viewer, HTTP contract, and visual/accessibility suites; E2E remains explicit. A backend-only selection needs Go alone. Unknown scopes, path escapes, empty scopes, incompatible runner filters, and Go `TEST` expressions matching nothing fail.

```sh
make test DOMAIN=backend LAYER=unit COMPONENT=database/run
make test DOMAIN=frontend LAYER=functional COMPONENT=views/corpus
make test DOMAIN=tooling COMPONENT=go/doccheck LAYER=functional
make test DOMAIN=contracts COMPONENT=http LAYER=integration BROWSER=chromium
make test-go COMPONENT=database/run LAYER=integration
make test-go PACKAGE=./tests/backend/server/integration TEST='^TestAPIDetailsGraphModes$'
make test-frontend-functional
GOFLAGS=-p=2 make test-race
```

`make test-go` selects Go suites from backend, tooling, and shared domains; a `COMPONENT` without `DOMAIN` defaults to backend for this convenience target. `PACKAGE` is a single canonical test-package override and cannot be combined with `COMPONENT`. `TEST_FILE` is one canonical browser-spec path within the selected domain/component; general browser selection requires no Go or Node suites in that scope. `BROWSER=all` selects both configured browser projects. Root npm workspaces separately own source, Node tools, and tests; `make check-frontend` checks all three TypeScript scopes, including compile-only JSX assertions.

## 11. Family SQL code generation

Family typed queries are generated by the pinned sqlc development generator, which is installed to `build/sqlc` at the `SQLC_VERSION` declared in the root Makefile and is not a Go module dependency:

```sh
make sqlc-install
make sqlc-generate
make sqlc-check
```

The equivalent direct commands from the repository root are:

```sh
./build/sqlc -f config/sqlc.yaml generate
./build/sqlc -f config/sqlc.yaml diff
./build/sqlc -f config/sqlc.yaml vet
```

Maintained inputs are `config/sqlc.yaml` with one v2 generation unit per owner, the family declarations and named queries under `backend/database/<family>/schema/` and `backend/pdfstore/schema/`, and the private export queries under `tooling/go/prepare-osf/schema/`. Generated outputs are the matching committed `internal/sql/` packages under `backend/database/<family>/`, `backend/pdfstore/`, and `tooling/go/prepare-osf/`; change the maintained inputs, run `make sqlc-generate`, and review the generated diff instead of editing generated files.

`make sqlc-check` runs `diff` to prove committed generated Go matches the current inputs and `vet` to analyze every maintained query without writing files. Run it after any family schema or query change and treat either failure as blocking. Normal `make build`, `make tools`, `make test`, and `make check` consume committed generated output and never invoke sqlc or require network access; `make -n build tools` shows no sqlc command.

Common generation and analysis problems:

- A `diff` failure means committed generated output is stale: run `make sqlc-generate`, review the generated diff, and rerun `make sqlc-check`.
- A `vet` failure names the maintained query file and statement: correct the SQL under the owning `schema/` directory and rerun the focused family tests.
- A missing table or column means the owning `schema/definitions.sql` does not declare the smallest shape the query needs: add the declaration without connecting it to runtime migrations.
- SQLite analysis is weak for expressions, recursive CTEs, and nullable joins, so an output field can become `interface{}` or the wrong nullable form: select explicit columns with aliases, or use `CAST` and `COALESCE`, then inspect the generated signature and cover complex queries with real-database integration tests.
- Keep SQL comments ASCII-only for the pinned generator, and never point sqlc at `migrations/` or `config/database*.something`; those remain the only runtime schema and versioning sources.
- A query that needs discovered identifiers is not a generation problem: retain the explicit `*_dynamic.go` exception and run `make check-sql`.

## 12. End-to-end verification

Run the deterministic and mocked-provider pipeline through persisted SQLite, the viewer APIs, and the Chromium review workflow:

```sh
make test-e2e
```

The target builds `build/analysis`, `build/pdf-store`, and `build/something-printer`, evaluates generated configurations, invokes the real pipeline executable, adds a deterministic valid PDF through the supported manual tool, and writes fresh target-owned bundles under `build/e2e/deterministic/` and `build/e2e/mocked/`. It creates overlapping A1 and A2 attempts, preserves the Playwright-mutated copy under `build/e2e/deterministic/review/`, and runs a Go database verifier over inherited and divergent heads, immutable versions, links, anchors, and review audit rows. Mocked provider URLs are loopback-only and accidental external HTTP and HTTPS are blocked for that subprocess.

Run the bounded real-provider variant only when network access and live calls are explicitly acceptable:

```sh
make test-e2e-live E2E_LIVE=1
```

The live target contacts public Crossref, OpenAlex, and ORCID endpoints, uses structural rather than exact remote-data assertions, writes `build/e2e/live/`, and is not reachable from `make test`, `make test-race`, `make test-integration`, `make test-all`, `make coverage`, or frontend targets. Provider availability, rate limiting, and remote payload changes can make this opt-in target fail independently of offline repository behavior.

## 13. Viewer fixture

Generate the ignored viewer fixture pair before browser work, and regenerate it when its intended data contract changes:

```sh
make fixture
```

The target invokes `go run ./tests/support/go/cmd/viewer-fixture`, uses production migrations, and writes `build/fixtures/viewer/workspace.fixture.db` plus its bound PDF companion. `FIXTURE_DB` overrides the output path; authoritative fixture construction lives under `tests/support/go/viewerfixture/`. The fixture includes a deterministic valid multi-page PDF with selectable text. Both database files are generated and ignored; authoritative changes belong in the fixture code.

## 14. Frontend unit verification

```sh
make test-frontend-unit
```

The target runs the unit suites below `tests/frontend/` through Node's built-in runner and jsdom. It requires installed locked frontend dependencies but does not start a browser or server. Tests are authored in TypeScript and import the TypeScript sources under `frontend/src` directly with explicit `.ts` or `.tsx` specifiers; Node's native type stripping executes `.ts` sources and an esbuild loader hook executes `.tsx` sources, so no build step is required (Node 22.18+).

## 15. Playwright verification

```sh
make test-frontend
make test-frontend-all
make test-frontend BROWSER=chromium WORKERS=4 TEST_FILE=tests/frontend/viewer/integration/viewer.spec.ts
make test-frontend-all WORKERS=1 TEST_FILE=tests/frontend/viewer/integration/review.spec.ts
make test-frontend-headed BROWSER=chromium
make test-frontend-debug BROWSER=chromium TEST_FILE=tests/frontend/viewer/integration/viewer.spec.ts
make test-frontend-visual
```

Each browser-test target builds `build/analysis`, assembles `build/frontend/dist`, copies the generated metadata and PDF fixture pair into a unique target-owned directory, starts an isolated viewer on an operating-system-assigned loopback port with the assembled assets, waits for `/api/health`, gives that exact URL to Playwright, and stops only its own child process. It does not reuse or stop a manually running viewer and never serves the base fixture writable.

Each invocation writes unique output under `build/playwright/run-*/` and prints its report path. Open one report with:

```sh
make frontend-report REPORT_DIR=build/playwright/run-<id>/report
```

`WORKERS` controls Playwright worker count (one in CI, eight locally unless overridden). Use a smaller explicit value when running alongside other CPU-intensive checks. Read-oriented specs may run in parallel. Each browser project and mutation spec receives a fresh fixture pair; `review.spec.ts` and the HTTP contract spec run serially within their own isolated copies. An E2E caller may set `PLAYWRIGHT_MUTATION_DB` only to a target-owned path under `build/e2e/` or `build/playwright/` so the post-browser database verifier can inspect the mutation evidence.

The quality suite launches Linux Chromium with [tests/support/fonts/fonts.conf](../tests/support/fonts/fonts.conf), which limits font discovery to committed assets: the existing PDF.js Liberation Sans faces, Liberation Mono 2.1.5 for machine values, and DejaVu Sans 2.37 for symbol fallback. The additional font fixtures and their licenses live under `tests/support/fonts/`. This fixes DOM and canvas typography across Linux distributions so local screenshot baselines also match CI. Review screenshot changes before accepting them; after preparing the binary, frontend assets, and fixture pair, regenerate the Chromium baselines with:

```sh
node tests/support/node/run-playwright.ts --project=chromium tests/frontend/quality/integration/ui-quality.spec.ts --grep 'Visual regression' --update-snapshots --workers=1
make test-frontend-visual
```

## 16. Frontend dependency and vendor maintenance

The checked-in D3-force browser module means normal Go builds do not invoke npm. Rebuild it only after an approved dependency or frontend entry change:

```sh
make frontend-vendor
```

Edit the dependency declaration or build input, not `frontend/vendor/d3-force.js`. Review the generated bundle and run the graph unit, server, and browser verification required by [STANDARDS.md](STANDARDS.md).

PDF.js is pinned exactly at 4.2.67 for the Node 22.18+ development environment. Rebuild and deterministically check its official prebuilt core, exact matching worker, CMaps, standard fonts, and license assets with:

```sh
make frontend-pdfjs-vendor
make frontend-pdfjs-vendor-check
```

Edit `tooling/node/package.json` or `tooling/node/frontend/pdfjs-vendor.ts`, not files under `frontend/vendor/pdfjs/`. A PDF.js upgrade requires compatibility, core and worker version, license, browser, and security review.

## 17. Documentation workflow

Build the unified documentation tool and run all non-mutating checks:

```sh
make doccheck
make test-docs
make check-docs
```

The direct command interface is:

```sh
./build/doccheck check
./build/doccheck catalog check
./build/doccheck catalog update
./build/doccheck state check
./build/doccheck state update
```

After maintained Go or project-authored JavaScript declarations change, regenerate and review the source catalog:

```sh
make docs-catalog-update
```

After any maintained file under `docs/` changes, review the dependents listed in [DOC-STATE.md](DOC-STATE.md), then explicitly acknowledge the reviewed bytes:

```sh
make docs-state-update
```

Catalog update changes only the generated region in [PROJECT_CATALOG.md](PROJECT_CATALOG.md); catalog check also rejects maintained Go or JavaScript declarations without source descriptions. State update changes only the generated region in [DOC-STATE.md](DOC-STATE.md). There is no combined update because updating state is a review acknowledgement rather than a formatting operation.

`make check-docs` checks local Markdown links, single-line Markdown formatting, exact obsolete references, migration filenames and boundaries, source catalog freshness and description completeness, documentation hashes, and dependency-table integrity. It does not validate external URLs, Markdown anchors, or semantic correctness beyond these executable contracts.

## 18. Output and data safety

`corpus/`, `cache/`, `intermidiate/`, SQLite databases, and `build/` are ignored but may contain real inputs, expensive state, or generated evidence. Inspect them read-only by default and do not clear, migrate, rename, or regenerate them unless the task explicitly requires it. The `intermidiate` spelling is intentional current behavior.

Do not commit, push, merge, publish, deploy, release, change remote state, or run destructive Git or filesystem operations without explicit authorization.

OSF export rewrites configuration links in run artifacts, search revisions, and preflight steps, including artifact fingerprints. Only direct literal reviewer fields can be redacted; variable references, macros, expressions, and interpolation fail closed without publishing an output bundle.
