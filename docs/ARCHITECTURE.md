# Research Analysis Architecture

## 1. Purpose and authority

This document describes the implemented architecture, runtime workflows, state, storage, configuration, protocols, boundaries, decisions, assumptions, constraints, and current known issues. Start with [AGENTS.md](../AGENTS.md), use [DATABASE.md](DATABASE.md) for the current SQLite schema and relationships, use [STANDARDS.md](STANDARDS.md) for change rules, use [PROJECT-USAGE.md](PROJECT-USAGE.md) for supported commands, use [PROJECT_CATALOG.md](PROJECT_CATALOG.md) for source declarations and line locations, use [DESIGN.md](DESIGN.md) for the frontend contract, and use [something.spec.md](something.spec.md) for the SOMETHING language.

The implementation, migrations, configuration, and executable tests are authoritative when prose and code disagree. Documentation describes only the supported current state; Git history owns superseded designs and workflows.

## 2. System summary

The repository builds a Go command that turns Scopus and IEEE Xplore CSV exports plus Web of Science BibTeX exports into an immutable, provenance-rich research corpus in SQLite. A workspace run captures exact SOMETHING configuration and source identities, parses and deduplicates articles, optionally enriches them through Crossref, OpenAlex, and ORCID, validates them, normalizes accepted metadata, registers normalized DOIs in a companion PDF inventory, and records artifacts, metrics, stage outcomes, cache decisions, and append-only audit events.

The same binary serves a loopback-only local viewer over an existing migrated metadata database and its bundle-relative PDF store. Pipeline evidence remains immutable; a separate metadata write connection supports append-only review versions and reversible run visibility with lifecycle audit, while the PDF store remains read-only. The viewer is a vanilla JavaScript URL-state SPA embedded in the Go binary by default; `--assets-dir` changes only the frontend asset source for development.

The project stack is Go 1.25.0, SQLite through `modernc.org/sqlite`, SOMETHING configuration, SQL migrations, standard-library HTTP and JSON, vanilla JavaScript ES modules, HTML, and CSS. Frontend development uses Node.js, `node:test`, jsdom, Playwright, axe-core, D3-force, and esbuild. `modernc.org/sqlite` is the only direct Go runtime dependency.

```text
Source exports + SOMETHING configuration
                    |
                    v
        Go workspace pipeline
          |                |
          v                v
 metadata SQLite <----> companion PDF SQLite
          |
          v
 loopback-only Go HTTP server
          |
          v
 embedded URL-state JavaScript SPA
```

## 3. Repository and build layout

| Path | Responsibility |
|---|---|
| [Makefile](../Makefile) | Supported repository-root build, execution, test, documentation, coverage, fixture, viewer, and frontend interface. |
| `src/main.go` | Only production executable entry point; dispatches `run`, `migrate`, `serve`, and `version`. |
| `src/go.mod`, `src/go.sum` | Go module and dependency lock; Go commands execute with `src/` as the module directory. |
| `src/article/` | Parsed article model, text cleanup, DOI extraction, field projection, and DOI deduplication. |
| `src/bibtex/` | Deliberately constrained article-only BibTeX tokenizer and parser. |
| `src/database/` | Writable and existing-only metadata SQLite connections, migrations, repositories, immutable relationships and review versions, mutable review heads, attempts, artifacts, cache, metrics, and audit. |
| `src/enrich/` | Rate-limited provider HTTP client, payload-envelope validation, provider URL construction, and response decoding without database access. |
| `src/logging/` | Process-wide structured logger and compile-time logging threshold. |
| `src/manifest/` | Standard-library-only canonical manifests, fingerprints, lifecycle vocabulary, cache layout, and audit types. |
| `src/normalization/` | Deterministic author, affiliation, publisher, and journal normalization. |
| `src/pdfstore/` | Companion PDF SQLite store, normalized DOI registration, PDF validation, content blobs, and transactional audit outbox. |
| `src/server/` | Existing-only metadata read and bounded-mutation connections, evidence and lifecycle APIs, details, audit, artifact access, read-only PDF delivery, and embedded assets. |
| `src/something/` | Lexer, ordered parser, directive expansion, type checker, evaluator, error boundary, and typed result accessors. |
| `src/notes/` | Authoritative bounded note grammar, link extraction, diagnostics, and shared Go and JavaScript conformance fixtures. |
| `src/tools/coveragecheck/` | Coverage policy parser and enforcement command used by `make coverage`. |
| `src/tools/doccheck/` | Documentation links, format, migration, source catalog, hash state, dependency, and obsolete-reference validation. |
| `src/tools/pdf-store/` | Validated manual PDF insertion command. |
| `src/tools/prepare-osf/` | Copy-only metadata, PDF, and optional configuration sanitization for OSF export. |
| `src/tools/something-json/` | SOMETHING evaluation and JSON inspection command. |
| `src/validation/` | Shared article field validation rules. |
| `src/workspace/` | Typed configuration, attempt preflight, cache policy, pipeline orchestration, artifacts, and source loaders. |
| `config/` | Workspace, baseline types, database registry and chains, and coverage policy in SOMETHING. |
| `migrations/corpus.metadata/` | Metadata migrations V00001-V00024. |
| `migrations/corpus.pdf/` | Companion PDF migrations V00001-V00002. |
| `src/server/frontend/` | Embedded SPA source, stylesheets, generated pinned D3-force bundle, and generated pinned PDF.js assets. |
| `frontend/` | Node lock data, Playwright runner and configuration, browser tests, unit tests, and snapshots. |
| `src/server/testdata/` | Ignored generated viewer metadata and PDF fixture pair produced from authoritative Go fixture code. |
| `src/testdata/e2e/` | Small tracked CSV and BibTeX inputs for deterministic, mocked-provider, and opt-in live pipeline-to-viewer verification. |
| `docs/` | Maintained architecture, design, standards, usage, language, catalog, CSS, and documentation-state references. |

The root Makefile changes into `src/` for Go commands and writes binaries and generated reports beneath `build/`. `make build` produces `build/analysis`; `make tools` produces `build/something-json`, `build/pdf-store`, `build/prepare-osf`, `build/doccheck`, and `build/coveragecheck`. Runtime relative paths assume the repository root, except that `src/main.go` moves one directory upward when its current directory ends in `/src`.

## 4. Dependency direction and component boundaries

The internal production import direction is intentionally narrow: foundation packages such as `manifest`, `something`, `logging`, `article`, `notes`, `normalization`, and `validation` do not depend on orchestration; `database`, `enrich`, and `pdfstore` own infrastructure boundaries; `workspace` composes pipeline concerns; `server` uses database repositories but does not import workspace orchestration; and `main` imports `database`, `logging`, `server`, and `workspace`.

```text
                          +------------------+
                          |     main.go      |
                          +--------+---------+
                                   |
                    +--------------+--------------+
                    v                             v
             +-------------+                +-----------+
             |  workspace  |                |  server   |
             +------+------+                +-----+-----+
                    |                             |
       +------------+-------------+               v
       v            v             v        existing metadata SQLite
  database       enrich       pdfstore
       ^            ^             ^
       +------------+-------------+
                    |
   article, bibtex, manifest, normalization,
   validation, something, logging
```

`src/main.go` is the only production entry point. Pipeline behavior belongs in `src/workspace/`; provider gatherers do not call database repositories, repository and store functions do not perform provider HTTP requests, and `src/server/` does not import workspace orchestration or call migration-running `database.Open`.

SQLite is the sole persisted pipeline record. Immutable `work_revisions`, ordered `authorships`, and ordered `reference_mentions` are the canonical corpus representation. `run_work_stages` records per-run, per-work outcomes independently from revision rows, so rejected work remains inspectable even though only valid normalized revisions enter the analysis-ready corpus.

Provider responses live in content-addressed `artifacts` and inline `artifact_blobs`; global cache identity and expiry live in `cache_entries`, and per-run use lives in `run_cache_uses`. The PDF store is a companion database rather than a metadata table family. The viewer opens metadata once in read-only mode and once in existing-only bounded-mutation mode, never creates a database or runs migrations, and keeps the companion PDF connection read-only.

## 5. Executable boundaries

`build/analysis run` accepts `--db`, `--config`, repeated `--workspace search_id@search_revision`, and `--fresh`. Without selectors it executes every declared workspace in declaration order. A matching completed execution plan is a no-op by default, but DOI inventory reconciliation still runs; `--fresh` creates another attempt and records the rerun reason.

`build/analysis migrate --db <metadata.db>` opens an existing metadata database and applies the configured metadata migration chain without running a workspace. It does not create a missing database.

`build/analysis serve` accepts `--db`, `--addr`, and `--assets-dir`. It defaults to `corpus.metadata.db` and `127.0.0.1:8080`, rejects addresses whose host is not an exact loopback IP, opens an existing migrated database through separate metadata read and bounded-mutation connections, verifies required append-only review triggers, serves embedded assets unless a directory is supplied, and applies conservative HTTP timeouts.

`build/analysis version` prints the semantic version `MAJOR.MINOR.PATCH` from the compile-time `major`, `minor`, and `patch` constants in `src/main.go`, appending `-development` when the compile-time `dev.Mode` constant marks a development build.

`build/pdf-store add --db <metadata.db> --doi <doi> --file <pdf>` resolves the bound companion database from metadata. It requires an already normalized and registered DOI, accepts at most 20,000,000 bytes with a `%PDF-` signature, hashes content with SHA-256, stores identical bytes once, leaves an available DOI unchanged, and flushes idempotent PDF audit events.

`build/prepare-osf --db <metadata.db> [--config <workspace.something>] --out <new-directory>` creates a WAL-safe, sanitized, self-contained copy without overwriting output or changing any source. `build/something-json <path>` evaluates a SOMETHING file and writes JSON. `build/doccheck` implements the documentation workflow. `build/coveragecheck` is built by `make coveragecheck` or `make tools` and is normally invoked through `make coverage` with the configured policy.

The process logger is installed as the standard `slog` default and package loggers come from `logging.Logger(component)`. The compile-time minimum is `slog.LevelInfo`; there is no runtime `--log-level` option.

## 6. Configuration architecture

`config/workspace.something` includes `config/baseline.something`, declares format-version-2 workspaces, optional reviewer username and email, source queries and filter counts, rename and keep mappings, enrichment settings, reuse policy, and cache policy. `workspace.Load` reads the selected file once, evaluates the exact bytes, ignores iteration scopes without a `workspace` value, rejects duplicate selectors, trims and bounds reviewer values, and produces typed `manifest.ResolvedManifest` and `enrich.Config` values. Reviewer identity is persisted for every attempt but excluded from resolved and input manifests and execution fingerprints.

Workspace selectors are stable `search_id@search_revision` strings. Supported source file types are `csv` and `bib`. Supported reuse policies are `reuse_completed`, `fresh`, and `retry`; command-line `--fresh` forces a new attempt.

Provider endpoint values, requested fields, fill policy, rate, concurrency, timeout, retries, batch size, and extra URLs are configured, but provider keys and orchestration order are implemented code: Crossref, OpenAlex metadata and references, then ORCID exact profiles and name-search evidence.

Cache read layers are ordered and may be `active_run`, `global`, `network`, or resolved `run:N`; writes may target `active_run` and `global`. A `run_specific` declaration becomes `run:N` after requiring a positive `read_run_id`. Fresh policy cannot write global cache data, and negative TTL must be nonnegative.

`config/database.something` selects the metadata and PDF chain files independently from the workspace configuration path. Each chain declares its migration directory and ordered `db_migration` iterations. `config/coverage_policy.something` defines the versioned whole-module and high-risk-file thresholds used by `make coverage`.

```text
command --config
      |
      v
config/workspace.something ---> config/baseline.something
      |
      v
SOMETHING evaluation ---> typed workspace manifests

repository runtime location
         |
         v
   config/database.something
    |                    |
    v                    v
metadata chain         PDF chain
```

## 7. SOMETHING compiler and runtime

The public recovery boundary is `something.LoadSomethingBytes` or `something.LoadSomethingFile`. Lexer, parser, directive-generator, type-checker, and evaluator failures intentionally panic with `*SomethingError`; the public loaders recover that type as an error and re-panic unexpected values.

Compilation is strict and source ordered: tokenize, parse an ordered AST, expand structural directives, type-check the expanded AST, then evaluate it. Structural directives include includes, loops, insertions, iteration labels, dynamic lvalues, and macro calls. `#assert`, `#if`, `#error`, boolean and comparison operators, `#match`, and `#len` survive structural expansion and execute during later phases.

Iteration scopes publish keys such as `iteration_0000000000_workspace`; typed consumers use the Once, Index, and All accessor families. Mapping, setup, struct-like scope, and namespace values evaluate to `map[string]any`; arrays evaluate to `[]any`; enum accessors preserve enum-name and ordinal validation.

```text
source bytes
   |
   v
 lexer -> parser -> structural directive generation -> type checker -> evaluator
                                                                           |
                                                                           v
                                                               typed published values
```

The authoritative syntax and semantic contract is [something.spec.md](something.spec.md).

## 8. Pipeline workflow

`workspace.RunPipeline` opens the metadata store with migrations and starts or reuses an attempt. Preflight resolves and hashes the manifest and source bytes, records unreadable-source failures in the input manifest, creates or reuses search, revision, and plan records, persists exact configuration and manifest artifacts, and starts an atomic attempt only when execution is needed.

Ingestion records each source, expected and observed export counts, ordered filter counts, raw records, parse outcomes, a parse artifact and run step, immutable parse revisions, and parse stage outcomes. CSV uses `encoding/csv` with `LazyQuotes`; headers are trimmed and lowercased. BibTeX accepts article entries only.

Deduplication merges parsed articles by DOI across sources, records unique and duplicate metrics, persists a deduplication artifact and step, creates immutable deduplicate revisions, and records stage outcomes. Normal workspace parsing requires DOI, title, and year.

When enabled, Crossref and OpenAlex enrich article metadata in that order. OpenAlex references become immutable active-run relationships. The metadata boundary creates an `enrich_metadata` revision before ORCID identity work. Exact observed ORCIDs may enrich profiles; name searches create uncertain candidate evidence and never assign identity by themselves.

Every applied field change creates one `field_enriched` audit event. Validation records `valid` or `discarded` and gates normalization. Normalization processes only valid work, classifies supported fields as changed, already canonical, or unavailable, persists normalized values and extension metadata in a new revision, and records metrics.

Before successful completion, normalized revisions are grouped by work and synchronized into the companion PDF inventory. Missing rows become `not_available`, and outbox events reach metadata audit. A reused completed plan performs the same reconciliation without rerunning the stages.

```text
preflight
   |
   v
parse -> deduplicate -> enrich metadata -> enrich identity -> validate -> normalize
   |          |                |                 |              |          |
   +----------+----------------+-----------------+--------------+----------+
                                      |
                                      v
                    revisions + stages + artifacts + metrics + audit
                                      |
                                      v
                              DOI inventory sync
```

Stages use repository operations and scoped transactions rather than one transaction for an entire run. A late failure marks the attempt failed but intentionally preserves earlier immutable evidence.

## 9. Lifecycle and work states

An execution plan identifies deterministic resolved-manifest and input-manifest fingerprints. Attempts are separate lifecycle records, allowing retries and explicit fresh runs without changing plan identity.

```text
                     +----------+
                     | planned  |
                     +----+-----+
                          |
                          v
                     +----------+
                     | running  |
                     +----+-----+
                          |
                 +--------+--------+
                 v                 v
            +---------+       +--------+
            | success |       | failed |
            +---------+       +--------+

matching successful plan + no --fresh -> reused no-op + DOI reconciliation
matching successful plan + --fresh    -> new running attempt
```

Work stage rows retain observable outcomes independently from revisions. Parse and deduplication create revisions; configured enrichment stages may create later revisions or a skipped outcome; validation creates valid or discarded outcomes; normalization creates a final analysis-ready revision only for valid work.

```text
parse revision
     |
     v
deduplicate revision
     |
     +--> enrich_metadata revision
     |          |
     |          +--> enrich_identity revision or skipped
     |                         |
     +-------------------------+
                               v
                         valid | discarded
                           |         |
                           v         +--> inspectable provenance only
                    normalize revision
```

## 10. Identity, validation, normalization, and references

People represent confirmed identity and require a nonblank ORCID. Author occurrences preserve observed citation names and provider profile fields; authorships attach ordered occurrences and affiliations to immutable revisions. Prerequisite checks remain before join inserts because SQLite foreign-key violations are not neutralized by `INSERT OR IGNORE`.

Author identity resolution statuses are `matched`, `no_match`, `unclear`, and `provider_error`. Candidates retain provider rank, ORCID, display name, raw payload artifact, and evidence context. A name-search candidate remains review evidence and does not silently change confirmed identity.

Validation checks title, authors, year, DOI, publisher, and reference-count conditions in `src/validation/validation.go`. Discarded work remains visible through stage and provenance records but receives no normalized revision.

Normalization is deterministic and heuristic. Author processing separates given and family names and derives initials; affiliation and publisher processing expands known abbreviations and applies local title casing; journal canonical forms are stored in revision extension JSON while the main journal field preserves the revision value.

Reference mentions are immutable and ordered within a revision. DOI resolution targets a work identity when known. The viewer resolves a work to one selected-run revision using stage precedence `normalize`, `validate`, `enrich_identity`, `enrich_metadata`, `enrich`, `deduplicate`, then `parse`, with newest revision ID breaking ties.

## 11. Persistence and migrations

Writable database opening creates the parent directory, configures SQLite foreign keys and a five-second busy timeout on every connection, enables WAL, and applies the configured chain. Tracking-table creation and each migration run behind `BEGIN IMMEDIATE`, which serializes schema changes across independent processes.

The metadata chain is V00001-V00024 and the PDF chain is V00001-V00002. Migration execution follows SOMETHING iteration order, skips applied filenames, records checksums without later revalidation, does not use `previous` or `upgrade` to order execution, and has no downgrade runner even though files retain `-- ==DOWN==` sections.

The metadata schema includes pipeline planning and evidence, corpus revisions and relationships, cache and PDF links, `pipeline_run_reviewers`, singleton `review_settings`, run-scoped `review_contexts`, immutable review, note, link, and anchor version tables, and mutable context head tables. [DATABASE.md](DATABASE.md) lists every table and relationship.

The companion schema includes `schema_migrations`, `pdf_blobs`, `pdf_documents`, `pdf_gather_runs`, `pdf_download_attempts`, and `pdf_audit_outbox`. Current runtime behavior uses normalized DOI registration and validated manual inventory. [DATABASE.md](DATABASE.md) is the detailed reference for every table, column, default, relationship, constraint, index, trigger, and repository-level expectation in both databases.

Append-only triggers reject updates and deletes from audit events, work revisions, author occurrences, authorships, reference mentions, review contexts, logical notes and anchors, all immutable review versions and version children, and PDF download attempts. Supported code inserts reviewer rows once and does not update them. PDF blobs reject updates, and the runtime exposes no PDF-blob deletion path. Only review head tables move through repository transactions. A reviewed run or a run used in review lineage is purge-ineligible.

## 12. Cache and network behavior

`enrich.Client` owns the shared rate ticker, worker pool, HTTP timeout, request headers, and retries. Public `Fetch` delegates to `FetchAll`, so single and batch requests share rate limiting. HTTP 429 responses use exponential retry delay; context cancellation and request or read failures become `FetchResult` errors.

The workspace cache evaluates each configured read layer in order. Active-run and named-run layers use recorded run uses, global uses provider and request identity, and network delegates to the rate-limited client. Expired negative entries are stale and continue to later layers. Successful responses and deliberate negative results may be written to declared layers; malformed successful payloads are rejected and never cached.

```text
request identity
      |
      v
active_run -> named run:N -> global -> network
      |             |          |         |
      +-------------+----------+---------+
                    |
                    v
      valid hit | negative hit | miss | stale | invalid | fetch
                    |
                    v
           run_cache_uses + metrics
```

Cache metrics distinguish hits, misses, negative hits, stale entries, invalid payloads, and network fetches at run and provider scope. `run_cache_uses` preserves the concrete entry and outcome used by an attempt.

## 13. PDF inventory and audit outbox

The metadata database stores one `pdf_store_binding` row with a relative companion path. Writable pipeline binding and read-only viewer binding reject absolute paths and traversal outside the metadata database directory.

PDF content is addressed by SHA-256 in `pdf_blobs`. `pdf_documents` has two states: `not_available` has no content hash or inventory timestamp, and `available` has both. Registration is idempotent. Manual addition validates size and signature before storage, requires prior normalized registration, and does not replace an available selection.

PDF events enter `pdf_audit_outbox` in the same transaction as the PDF mutation. Flushing writes metadata `audit_events` plus `pdf_audit_links` in one metadata transaction and then marks the outbox row delivered. Repeated delivery is idempotent across interruption.

```text
PDF transaction                        metadata transaction
----------------                       --------------------
pdf_documents change
pdf_audit_outbox row ---- flush ----> audit_events row
                                      pdf_audit_links row
         ^                                      |
         +---------- mark delivered ------------+
```

## 14. Viewer backend and protocol

`server.Open` rejects empty, missing, and directory paths; opens metadata SQLite using `mode=ro` and `query_only` for browsing; opens the same existing file with a single `mode=rw` connection for bounded review and lifecycle transactions; verifies the review schema and its protection triggers without migrating; discovers non-system tables and columns; and optionally opens the bound PDF store read-only after path and schema validation. Startup closes every opened connection if a later validation step fails.

The server accepts GET and HEAD for evidence reads and narrowly routed POST and PUT review or lifecycle mutations. Mutations require JSON content type, reject unknown fields and trailing values, cap request bodies, enforce same-origin requests when `Origin` is present, return structured status and conflict codes, and use no-store caching. Collection inputs and response sizes are bounded, table and sort identifiers are validated against discovered schema, artifact and PDF downloads use conservative headers, and API request contexts time out after five seconds. HTTP server timeouts are five seconds for headers, ten seconds for reads, fifteen seconds for writes, and thirty seconds idle. The server rejects Host authorities that differ from its exact loopback listener to limit rebinding attacks.

| Route | Responsibility and bounds |
|---|---|
| `/api/health` | Reports database health, review capability, and the opaque corpus ID used to namespace browser drafts. |
| `/api/searches`, `/api/plans`, `/api/runs` | Resolves hierarchical research context. |
| `/api/runs/{id}/visibility` | Moves a terminal attempt between `active` and `trashed` in one transaction and appends the corresponding lifecycle audit event. |
| `/api/overview` | Reports captured execution metrics and current derived coverage. |
| `/api/runs/{id}/audit`, `/api/audit` | Returns bounded cursor audit pages, facets, and supported filters. |
| `/api/runs/{id}/artifacts`, `/api/artifacts/{id}/inspect`, `/api/artifacts/{id}/content` | Lists, previews a bounded text prefix, or downloads stored artifacts. |
| `/api/runs/{id}/cache-uses` | Returns cache evidence for one run. |
| `/api/runs/{id}/corpus/{kind}` | Returns bounded articles, authors, references, or sources for the selected run. |
| `/api/runs/{id}/evaluation`, `/api/runs/{id}/identity-evidence`, `/api/runs/{id}/stages` | Returns evaluation, uncertain identity evidence, or stage outcomes. |
| `/api/runs/{id}/review-context`, `/api/runs/{id}/review-context-candidates` | Returns, proposes, explicitly initializes, or lists bounded eligible parents for one completed non-trashed run. |
| `/api/runs/{id}/articles/{revision}/review`, `/api/runs/{id}/articles/{revision}/review/versions` | Reads or appends complete review state and returns bounded immutable ancestry with optimistic concurrency. |
| `/api/runs/{id}/articles/{revision}/notes`, `/api/runs/{id}/notes/{note}`, `/api/runs/{id}/notes/{note}/versions` | Creates, reads, edits, tombstones, restores, and lists bounded immutable note versions and resolved links. |
| `/api/runs/{id}/articles/{revision}/anchors`, `/api/runs/{id}/anchors/{anchor}/versions`, `/api/runs/{id}/review-backlinks` | Creates and reads bounded PDF anchors, tombstones, ancestry, and current-version backlinks. |
| `/api/tables`, `/api/tables/{table}` | Discovers tables and browses an allowlisted schema with bounded page sizes. |
| `/api/articles/{id}`, `/api/authors/{id}`, `/api/references/{id}` | Returns detail records and selected-run provenance; author detail includes bounded run-scoped identity candidates when `run_id` is supplied. |
| `/api/works/{work_id}/pdf-status`, `/api/pdf/{work_id}` | Reports inventory status or delivers validated available PDF bytes. |
| `/api/graph` | Returns one bounded graph model with explicit truncation evidence. |
| `/api/trash` | Returns read-only trashed-run and restore history. |

Run-scoped articles contain valid normalize-stage revisions. Author and reference collections include relationships attached to revisions produced during the run, so consumers use revision and producer-stage identity when uniqueness matters.

Audit supports run context, text search, category, action, actor, entity, stage, and outcome filters. Categories accepted by the server are pipeline, enrichment, validation, and PDF; review actions have their own visual classification and remain filterable by action. The frontend presents date-grouped timeline cards and cursor-based loading without discarding already-loaded evidence. Artifact preview defaults to 64 KiB and caps at 256 KiB; only recognized text, JSON, and SOMETHING media are previewed.

Graph modes are `research_network`, `article_author`, `citation`, and `article_reference`. The endpoint caps article nodes at 2,000, related nodes at 10,000, and edges at 20,000, and reports truncation and its reason.

Every review mutation validates the selected completed run, context head, work revision, vocabulary, expected version, and current PDF availability before one metadata transaction inserts a child version, compare-and-swap moves only that context head, and appends an audit event containing identifiers rather than note bodies, selected text, or reviewer email. The companion PDF availability check remains outside the metadata transaction because supported PDF inventory never removes or replaces an available document.

```text
browser GET/HEAD -> read-only metadata or PDF query -> bounded response
browser POST/PUT -> transport and context validation -> PDF availability read -> metadata version + head CAS + audit transaction
```

## 15. Frontend architecture

The frontend is a framework-free native ES-module SPA. `index.html` owns the accessible persistent header, breadcrumb, Deepdive navigation, searchable hierarchical selectors, status, loading, notice, and content regions. `app.js` is a side-effect entry that binds selector changes, context-preserving navigation, history, notices, loading controls, health state, mobile navigation, and initial rendering.

The URL is the source of truth for `search_id`, `search_revision_id`, `plan_id`, `run_id`, view, section, filters, sort, page, expanded row, selected graph node, artifact inspector state, focused `note_id`, focused `anchor_id`, and `pdf_page` where applicable. Every internal link uses `state.link`; hard-coded `?view=...` links lose research context.

`router.render` increments a request sequence, aborts the prior controller, refreshes context selectors, dispatches the selected view, updates the document title, ignores stale or aborted responses, and clears loading state only for the current request. Views fetch JSON, replace `app.innerHTML`, and then bind interactions that require the new DOM.

```text
URL/history event
      |
      v
router.render -> abort prior request -> hydrate selectors -> dispatch view
      |                                                   |
      |                                                   v
      +---------------------------------------------- API fetch
                                                          |
                                                          v
                                            replace app.innerHTML
                                                          |
                                                          v
                                              bind view interactions
```

### 15.1 Frontend file ownership

| Path | Responsibility |
|---|---|
| `index.html` | Persistent header, breadcrumb, Deepdive navigation, searchable context selectors, status, loading, notice, and app mount point. |
| `app.js` | Global event binding, history interception, shell initialization, and first render. |
| `state.js` | URL values, DOM state, escaping, formatting, shared panels, tables, flows, links, statuses, and global UI behavior. |
| `api.js` | Abort-aware JSON reads and mutations, endpoint construction, structured API errors, and table discovery cache. |
| `router.js` | Request sequence, abort lifecycle, selector hydration, view dispatch, navigation state, and document title. |
| `components/context-selector.js` | Dependent searchable single-select hydration, skeletons, clear controls, keyboard listbox interaction, and auto-selection. |
| `components/data-table.js` | Shared rows, sorting, search, page size, expansion, and control binding. |
| `components/pagination.js` | First, Previous, numbered, Next, and Last controls plus result ranges. |
| `components/audit-events.js` | Audit classification, summaries, metadata disclosures, stream markup, and investigation export. |
| `components/graph.js` | Query state, connected components, canvas lifecycle, simulation, drawing, interactions, export, selection, and edge table. |
| `components/shell.js` | Health state and responsive navigation toggle. |
| `components/note-parser.js` | Safe bounded note preview, link diagnostics, unresolved states, and context-preserving link rendering. |
| `components/note-editor.js` | Immutable note edits, tombstones, restoration, history comparison, and corpus-scoped browser drafts. |
| `components/pdf-viewer.js` | Custom vendored PDF.js lifecycle, single-page canvas and selectable text rendering, boundary-aware navigation, rotation, zoom, selection geometry, and anchor highlights. |
| `components/review-panel.js` | Explicit context initialization, lineage selection, complete status saves, history, notes, anchors, and PDF integration. |
| `views/home.js` | Context-independent hierarchy metrics, Explore links, and reversible run-visibility dialog behavior. |
| `views/*.js` | Page-level fetching, rendering, and post-render binding for Deepdive and detail routes. |
| `styles/tokens.css` | Theme, spacing, type, status, graph colors, focus, radius, and elevation tokens. |
| `styles/base.css` | Reset, document layout, typography, links, landmarks, and reduced motion. |
| `styles/elements.css` | Buttons, labels, messages, headers, loaders, and segments. |
| `styles/collections.css` | Navigation, selectors, grids, forms, tables, pagination, breadcrumbs, and responsive collections. |
| `styles/views.css` | Overview, details, provenance, audit, artifacts, evaluation, and stage presentation. |
| `styles/graph.css` | Relationship layout, controls, canvas, overview, legend, selection, and edge table. |
| `vendor/d3-force.js` | Generated pinned force-simulation implementation; it is changed only through `make frontend-vendor`. |
| `vendor/pdfjs/` | Generated PDF.js 4.2.67 core, matching worker, CMaps, standard fonts, and license assets; it is changed only through `make frontend-pdfjs-vendor`. |

### 15.2 Frontend module graph

```text
app.js
  +-- state.js                    no project imports
  +-- router.js
  |     +-- state.js
  |     +-- context-selector.js --+-- state.js
  |     |                         +-- api.js -> state.js
  |     |                         +-- router.js
  |     +-- home.js --------------+-- state.js, api.js, router.js
  |     +-- overview.js ----------+-- state.js, api.js, router.js
  |     +-- corpus.js ------------+-- state.js, api.js, data-table.js, pagination.js
  |     +-- relationships.js -----+-- state.js, api.js, graph.js, router.js
  |     +-- provenance.js --------+-- state.js, api.js, data-table.js, router.js
  |     +-- evaluation.js --------+-- state.js, api.js, data-table.js, router.js
  |     +-- advanced.js ----------+-- state.js, api.js, data-table.js, router.js
  |     +-- detail.js ------------+-- state.js, api.js, pagination.js, review-panel.js
  +-- api.js
  +-- context-selector.js
  +-- shell.js -> api.js

graph.js -> state.js, router.js, pagination.js, vendor/d3-force.js
data-table.js -> state.js, router.js, pagination.js
review-panel.js -> api.js, state.js, note-editor.js, pdf-viewer.js
note-editor.js -> api.js, state.js, note-parser.js
```

`state.js` is the shared leaf and imports no project module. Components do not import views. `router.js` and `context-selector.js` form one intentional ES-module cycle because selector interactions call `setURL` while rendering calls `hydrateSelectors`; bindings run only after module initialization, and new modules must not expand the cycle. CSS files load independently through `index.html` in tokens, base, elements, collections, views, and graph order.

Home summarizes the search/revision/plan/run hierarchy, establishes complete Deepdive context, and owns reversible run-visibility actions. Overview separates captured execution metrics from current derived coverage. Corpus selects articles, authors, references, source records, or identity evidence without nested tabs. Relationships provides four bounded graph models plus a table equivalent. Provenance covers audit, artifacts, cache, stages, and run detail. Evaluation covers normalized DOI, PDF inventory, and current review status and can explicitly start a run review context. Article detail provides a side-by-side PDF/review workspace, complete status history, note and link versions, and content-hash-bound anchors; author detail owns candidate ORCID evidence. Advanced exposes discovered tables, and detail views remain within Corpus navigation.

The graph uses the generated checked-in D3-force bundle without a CDN. Canvas rendering is supplemental to DOM controls, legend, cluster overview, search, selection details, paginated edge table, fit, zoom, drag, expansion, and PNG export. Shape encodes entity type and color encodes connected component.

The rendering and frontend extension rules live in [STANDARDS.md](STANDARDS.md), the visual and interaction contract lives in [DESIGN.md](DESIGN.md), and active stylesheet selectors and tokens live in [CSS-REFERENCE.md](CSS-REFERENCE.md).

## 16. Testing architecture

Go tests use `testing` with `unit`, `functional`, `integration`, and isolated `e2e` build tags. `make test` and `make test-go` enable the first three tag families without cached results; `make test-e2e` alone selects the offline E2E tests, builds and invokes `build/analysis`, evaluates generated SOMETHING configuration through `build/something-json`, and writes deterministic and mocked database bundles beneath `build/e2e/`. Real-database integration and E2E tests use production migrations.

`make test-race` runs all Go tests under the race detector and applies to concurrency, cache, HTTP-client, database, and lifecycle changes. `make coverage` writes an atomic whole-module profile and enforces the SOMETHING coverage policy. `make format-check`, `make vet`, and `make check` cover formatting, static checks, and complete read-only documentation consistency.

Frontend unit tests use `node:test`, `node:assert`, and jsdom without a browser or server. Playwright copies the ignored generated fixture pair for each invocation, uses an isolated viewer on an operating-system-assigned loopback port, and writes unique output under `build/playwright/`; `viewer.spec.cjs` covers fixture evidence, serial `review.spec.cjs` covers local mutations and PDF rendering, `ui-quality.spec.cjs` covers axe, responsive behavior, and reviewed visual snapshots, and the environment-guarded `e2e.spec.cjs` mutates and verifies A1 and A2 review lineage over a database generated by the real pipeline binary.

Current test counts are derived from source rather than maintained in prose. The normative test and verification policy is [STANDARDS.md](STANDARDS.md).

## 17. Security and data-integrity boundaries

The viewer has no authentication or authorization, so writable serving requires an exact loopback IP listener and exact Host authority. It writes append-only review evidence, mutable review heads, and audited reversible run visibility in existing migrated metadata, keeps all other pipeline evidence and the PDF companion read-only, validates bounded inputs and origins, escapes frontend and note content, and makes no production CDN request.

Provider requests use configured timeouts, retries, rate limits, controlled headers, and payload validation. Credentials and sensitive environment values are not stored in documentation or emitted deliberately through logs. Routine provider tests use controlled mock servers, while `make test-e2e-live E2E_LIVE=1` is the only explicit test exception and uses a bounded public corpus with structural assertions.

Persisted revisions, relationships, artifacts, and audit evidence are immutable or append-only at their owning boundary. Content-addressed blobs are verified by SHA-256. Cross-database PDF audit uses an idempotent outbox because SQLite cannot provide one transaction across independent files.

## 18. Architectural decisions

- SQLite is the sole pipeline system of record so retries, provenance, cache reuse, and offline inspection share one transactional substrate.
- Work revisions and relationship rows are immutable so stages remain reproducible without destructive overwrites.
- Execution plans use deterministic manifest fingerprints while attempts retain independent lifecycle and retry evidence.
- Metadata and PDF stores are separate portable databases linked by one relative binding because document bytes and manual inventory have a different lifecycle from research metadata.
- PDF audit delivery uses an outbox because one atomic transaction cannot span two SQLite databases.
- Provider gather and storage are separated so HTTP behavior remains mockable and database provenance remains centralized.
- SOMETHING preserves ordered evaluation and panic-based internal phase failures so compiler behavior remains consistent across phases.
- The viewer is local and review-writable, with bounded mutation endpoints and no authentication layer; exact loopback binding and Host authority are mandatory privacy boundaries.
- Review contexts freeze selected parent heads by stable work identity, and later child versions move only the selected context so historical runs remain stable.
- The frontend uses URL state and native modules so research context is shareable, reload-safe, and independent from a framework build step.
- D3-force is pinned and generated as one checked-in browser asset so production serving requires neither Node nor a CDN.
- PDF.js 4.2.67 is pinned and generated with its matching worker, CMaps, fonts, and license so PDF rendering requires neither a CDN nor the default PDF.js viewer UI.
- Documentation generation and validation use one Go tool so maintained project tooling does not introduce another runtime language.
- Documentation state updates are explicit acknowledgements because a hash detects changed bytes but cannot prove semantic accuracy.

## 19. Assumptions and constraints

- Commands and runtime relative paths assume execution from the repository root unless a documented command changes into `src/` itself.
- Workspace source exports exist locally and match their declared source type; the pipeline does not acquire proprietary exports.
- Provider keys and execution order are code, so adding configuration alone does not register a provider.
- The metadata and companion PDF databases move and back up together after PDF inventory begins.
- SQLite foreign keys, WAL for writable metadata, append-only triggers, and immutable revision semantics are required integrity behavior.
- The viewer requires an existing fully migrated database and is not a migration or repair path; `analysis migrate` is the separate explicit migration command.
- The graph, table browser, audit, artifact preview, and collection endpoints remain bounded even for local use.
- The `intermidiate` spelling is part of the current filesystem path contract.

## 20. Current known issues

- The viewer has no authentication or authorization and therefore rejects non-loopback listeners; local processes and users with host access remain inside its trust boundary.
- Author and reference collections may contain repeated conceptual values across revision snapshots and must retain occurrence, mention, revision, or producer-stage labels.
- Large graph responses are truncated at fixed bounds and are not streamed beyond those limits.
- Frontend templates are strings, so every dynamic value requires deliberate use of `esc`; there is no framework-level automatic escaping.
- The router and context selector have one intentional module cycle that must not expand.
- Research context and filters in copied URLs or browser history may reveal search and run identifiers.
- Fixture databases and visual snapshots prove behavior only for controlled data and the tested browser and platform.
- The process-wide logging threshold is compile-time and cannot be changed by a command-line option.

## 21. Safe extension points

- Add a metadata or PDF migration file with both section markers, append it to the correct SOMETHING chain, and never rewrite an applied migration.
- Add an enrichment provider by implementing provider HTTP and decoding in `enrich`, registering ordering and storage orchestration in `workspace`, preserving cache evidence, and using controlled server tests.
- Add an evidence endpoint with GET or HEAD, read-only SQLite, allowlisted identifiers, bounded input and output, context timeouts, and integration coverage. Add a mutation only through the existing transport guards, a narrow repository transaction, required state checks, and identifier-only audit metadata; review mutations additionally preserve the PDF availability boundary, optimistic head compare-and-swap, and immutable version model.
- Add a frontend view by preserving URL context, registering router dispatch and primary navigation, rendering through established state and component helpers, binding after DOM replacement, and adding proportional tests.
- Add reusable frontend behavior to a component without importing view modules or expanding the router and selector cycle.
- Add an attached Go documentation comment or adjacent JavaScript JSDoc with every maintained declaration, then regenerate [PROJECT_CATALOG.md](PROJECT_CATALOG.md); JavaScript test callback titles supply test descriptions, and documentation checks reject missing source descriptions.

## 22. Documentation architecture

[AGENTS.md](../AGENTS.md) is the entry point, this document owns system architecture, [STANDARDS.md](STANDARDS.md) owns normative change rules, [DESIGN.md](DESIGN.md) owns frontend behavior and visual contracts, [CSS-REFERENCE.md](CSS-REFERENCE.md) owns active styles, [APP-USAGE.md](APP-USAGE.md) owns user experience and operating expectations, [PROJECT-USAGE.md](PROJECT-USAGE.md) owns developer and operator workflows, [PROJECT_CATALOG.md](PROJECT_CATALOG.md) is generated source navigation, [something.spec.md](something.spec.md) owns the language, and [DOC-STATE.md](DOC-STATE.md) records reviewed documentation hashes and dependency guidance.

`doccheck check` validates local links, exact obsolete references, migration configuration and documented boundaries, single-line Markdown, catalog freshness, complete maintained declaration descriptions, and acknowledged documentation state. `doccheck catalog update` changes only the generated project-catalog region. `doccheck state update` changes only the generated hash region and is run after affected documents have been reviewed.
