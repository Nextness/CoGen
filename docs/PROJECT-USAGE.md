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
make frontend-browsers BROWSERS="chromium firefox webkit"
```

`frontend-browsers` installs browser binaries; platform-native libraries remain an environment responsibility described by Playwright when missing.

## 3. Makefile interface

Run `make help` for the current targets, variables, and examples. The root [Makefile](../Makefile) is the supported interface and changes into `src/` for Go module commands.

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
| `build/analysis` | `make build` | Runs the pipeline, migrates an existing metadata database, or serves the loopback-only local review viewer. |
| `build/something-json` | `make something-json` or `make tools` | Evaluates one SOMETHING file and prints its value as JSON. |
| `build/pdf-store` | `make pdf-store` or `make tools` | Inserts one validated, already registered PDF into the companion store. |
| `build/prepare-osf` | `make prepare-osf` or `make tools` | Creates one sanitized copy-only metadata, PDF, and optional configuration export. |
| `build/doccheck` | `make doccheck` or `make tools` | Checks and updates maintained documentation artifacts. |
| `build/coveragecheck` | `make coveragecheck`, `make tools`, or `make coverage` | Enforces configured package and file coverage floors. |
| `build/coverage/coverage.out` | `make coverage` | Atomic Go coverage profile used by the policy checker. |
| `build/e2e/<variant>/` | `make test-e2e` or `make test-e2e-live E2E_LIVE=1` | Generated metadata and companion PDF databases, evaluated workspace configuration, and preserved review mutation evidence for pipeline-to-viewer verification. |
| `build/playwright/run-*/` | Browser test targets | Isolated traces, screenshots, results, and HTML reports. |

`build/coveragecheck` is normally executed by `make coverage`; developers should use the Make target so it receives the correct profile and policy paths.

`build/analysis version` prints the semantic version `MAJOR.MINOR.PATCH` (currently `1.0.0`), with a `-development` suffix for development builds.

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

Without `WORKSPACE` or `--workspace`, every declared workspace runs in declaration order. Repeat `--workspace` to select multiple declarations. A matching completed plan is reused as a no-op by default while DOI inventory reconciliation still runs; `FRESH=1` or `--fresh` creates another attempt.

`config/workspace.something` is the normal workspace entry and includes `config/baseline.something`. Each workspace may provide optional `reviewer = reviewer_config { username = "...", email = "..." }`; omitted values default to empty, are trimmed and bounded, are captured for each attempt, and do not affect manifests or execution fingerprints. `config/database.something` independently selects metadata and PDF migration chains. Paths supplied in source declarations must resolve in the repository-root runtime context.

## 6. Viewer operation and development

Apply pending metadata migrations to an existing database before serving it:

```sh
make migrate DB=corpus.metadata.db
./build/analysis migrate --db ./corpus.metadata.db
```

The migration command rejects a missing database and does not run a workspace. The viewer verifies required review tables and append-only triggers at startup but never migrates or repairs the database itself.

Serve an existing metadata database with the assembled production assets:

```sh
make frontend-build
make serve DB=corpus.metadata.db ADDR=127.0.0.1:8080 ASSETS_DIR=frontend/dist
./build/analysis serve --db ./corpus.metadata.db --addr 127.0.0.1:8080 --assets-dir ./frontend/dist
```

Serve the assembled assets, overriding the default asset directory:

```sh
make serve DB=corpus.metadata.db ASSETS_DIR=frontend/dist
make dev
```

`make dev` builds the frontend assets, copies the ignored generated fixture metadata and PDF pair to a disposable directory, and serves that copy with the assembled assets on the default loopback address. The server writes only run-scoped review evidence to existing metadata, keeps pipeline evidence and the companion PDF database read-only, rejects non-loopback addresses, and rejects requests whose Host authority differs from the bound listener. Frontend work edits the TypeScript sources under `frontend/src`, runs `make frontend-build` to assemble `frontend/dist`, and serves that directory; `make check-frontend` type-checks the sources, and unit tests import them directly without a build step.

Use [APP-USAGE.md](APP-USAGE.md) for review-context initialization, status and note history, note syntax, embedded PDF anchors, navigation, data interpretation, privacy, graph limits, and operational expectations.

## 7. Manual PDF inventory

Acquire documents through an authorized source, then use the validated manual write path:

```sh
make pdf-store
./build/pdf-store add --db corpus.metadata.db --doi 10.1000/example --file ./paper.pdf
```

The DOI must already have a normalized work revision and registered inventory row. The tool enforces the 20,000,000-byte limit and PDF signature, content-addresses identical bytes, changes the inventory state to `available`, records `inventoried_at`, and delivers the manual audit event. Re-adding an available DOI leaves its selected bytes unchanged.

Treat `corpus.metadata.db` and its bound `corpus.pdf.db` as one portable bundle. Stop writers for a simple file copy or use SQLite backup facilities for coordinated online backup. The Makefile `database-backup` target copies both configured database filenames to `../backup_databases/` and assumes that destination already exists.

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
make something-json
./build/something-json ./config/workspace.something
```

The tool executes the same public SOMETHING loading boundary as the application and prints evaluated values as JSON. Use [something.spec.md](something.spec.md) for syntax and semantics.

## 10. Focused and broad Go verification

```sh
make fmt
make format-check
make vet
make test-go PACKAGE=./server TEST='^TestGraph$'
make test
make test-race
make coverage
make check
```

`make test-go` enables unit, functional, and integration tags for the selected package and disables result caching. `make test` runs the same tag set across the module. Use `make test-race` for concurrency, cache, HTTP-client, database, or lifecycle changes.

`make coverage` writes `build/coverage/coverage.out` and enforces [config/coverage_policy.something](../config/coverage_policy.something). The checker merges duplicate source ranges using the highest hit count and reports current, required, and delta coverage for tracked packages and high-risk files.

`make check` runs the Go formatting check, `go vet`, and the complete non-mutating documentation check. It does not run all Go or frontend tests, so it supplements rather than replaces behavior tests.

## 11. End-to-end verification

Run the deterministic and mocked-provider pipeline through persisted SQLite, the viewer APIs, and the Chromium review workflow:

```sh
make test-e2e
```

The target builds `build/analysis`, `build/pdf-store`, and `build/something-json`, evaluates generated configurations, invokes the real pipeline executable, adds a deterministic valid PDF through the supported manual tool, and writes fresh target-owned bundles under `build/e2e/deterministic/` and `build/e2e/mocked/`. It creates overlapping A1 and A2 attempts, preserves the Playwright-mutated copy under `build/e2e/deterministic/review/`, and runs a Go database verifier over inherited and divergent heads, immutable versions, links, anchors, and review audit rows. Mocked provider URLs are loopback-only and accidental external HTTP and HTTPS are blocked for that subprocess.

Run the bounded real-provider variant only when network access and live calls are explicitly acceptable:

```sh
make test-e2e-live E2E_LIVE=1
```

The live target contacts public Crossref, OpenAlex, and ORCID endpoints, uses structural rather than exact remote-data assertions, writes `build/e2e/live/`, and is not reachable from `make test`, `make test-race`, `make test-integration`, `make test-all`, `make coverage`, or frontend targets. Provider availability, rate limiting, and remote payload changes can make this opt-in target fail independently of offline repository behavior.

## 12. Viewer fixture

Regenerate the ignored viewer fixture pair only when the intended test data contract changes:

```sh
make fixture
```

The target runs `TestGenerateFixture` with `FORCE_FIXTURE=1`, uses production migrations, and rewrites `src/server/testdata/workspace.fixture.db` plus its bound PDF companion. The fixture includes a deterministic valid multi-page PDF with selectable text. Both database files are generated and ignored; authoritative changes belong in the fixture code.

## 13. Frontend unit verification

```sh
make test-frontend-unit
```

The target runs every `frontend/tests/unit/**/*.test.ts` file through Node's built-in runner and jsdom. It requires installed locked frontend dependencies but does not start a browser or server. Tests are authored in TypeScript and import the TypeScript sources under `frontend/src` directly with explicit `.ts` or `.tsx` specifiers; Node's native type stripping executes `.ts` sources and an esbuild loader hook executes `.tsx` sources, so no build step is required (Node 22.18+). `make test-frontend-unit-tsx` runs only the tests for migrated `.tsx` modules.

## 14. Playwright verification

```sh
make test-frontend
make test-frontend-all
make test-frontend BROWSER=chromium WORKERS=4 TEST_FILE=tests/viewer.spec.cjs
make test-frontend-all WORKERS=1 TEST_FILE=tests/review.spec.cjs
make test-frontend-headed BROWSER=chromium
make test-frontend-debug BROWSER=chromium TEST_FILE=tests/viewer.spec.cjs
make test-frontend-visual
```

Each browser-test target builds `build/analysis`, assembles `frontend/dist`, copies the generated metadata and PDF fixture pair into a unique target-owned directory, starts an isolated viewer on an operating-system-assigned loopback port with the assembled assets, waits for `/api/health`, gives that exact URL to Playwright, and stops only its own child process. It does not reuse or stop a manually running viewer and never serves the base fixture writable.

Each invocation writes unique output under `build/playwright/run-*/` and prints its report path. Open one report with:

```sh
make frontend-report REPORT_DIR=build/playwright/run-<id>/report
```

`WORKERS` controls Playwright worker count. Read-oriented specs may run in parallel. `review.spec.cjs` is serial because it mutates only its invocation's isolated copy. An E2E caller may set `PLAYWRIGHT_MUTATION_DB` only to a target-owned path under `build/e2e/` or `build/playwright/` so the post-browser database verifier can inspect the mutation evidence.

## 15. Frontend dependency and vendor maintenance

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

Edit `frontend/package.json` or `frontend/scripts/pdfjs-vendor.mjs`, not files under `frontend/vendor/pdfjs/`. A PDF.js upgrade requires compatibility, core and worker version, license, browser, and security review.

## 16. Documentation workflow

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

## 17. Output and data safety

`corpus/`, `cache/`, `intermidiate/`, SQLite databases, and `build/` are ignored but may contain real inputs, expensive state, or generated evidence. Inspect them read-only by default and do not clear, migrate, rename, or regenerate them unless the task explicitly requires it. The `intermidiate` spelling is intentional current behavior.

Do not commit, push, merge, publish, deploy, release, change remote state, or run destructive Git or filesystem operations without explicit authorization.
