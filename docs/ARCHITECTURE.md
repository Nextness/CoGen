# Research Analysis Architecture

## 1. Purpose and authority

This document describes the implemented architecture, runtime workflows, state, storage, configuration, protocols, boundaries, decisions, assumptions, constraints, and current known issues. Start with [AGENTS.md](../AGENTS.md), use [DATABASE.md](DATABASE.md) for the current SQLite schema and relationships, use [STANDARDS.md](STANDARDS.md) for change rules, use [PROJECT-USAGE.md](PROJECT-USAGE.md) for supported commands, use [PROJECT_CATALOG.md](PROJECT_CATALOG.md) for source declarations and line locations, use [DESIGN.md](DESIGN.md) for the frontend contract, and use [something.spec.md](something.spec.md) for the SOMETHING language.

The implementation, migrations, configuration, and executable tests are authoritative when prose and code disagree. Documentation describes only the supported current state; Git history owns superseded designs and workflows.

## 2. System summary

The repository builds a Go command that turns Scopus and IEEE Xplore CSV exports plus Web of Science BibTeX exports into an immutable, provenance-rich research corpus in SQLite. A workspace run captures exact SOMETHING configuration and source identities, parses and deduplicates articles, optionally enriches them through Crossref, OpenAlex, and ORCID, validates them, normalizes accepted metadata, registers normalized DOIs in a companion PDF inventory, and records artifacts, metrics, stage outcomes, cache decisions, and append-only audit events.

The same binary serves a loopback-only local viewer over an existing migrated metadata database and its bundle-relative PDF store. Pipeline evidence remains immutable; a separate metadata write connection supports append-only review versions and reversible run visibility with lifecycle audit, while PDF evidence reads remain read-only and a separate existing-only companion connection is prepared for validated PDF mutations. The viewer is a framework-free TypeScript multi-page application with URL-backed state, assembled into `build/frontend/dist` and served from filesystem assets; the binary contains no frontend assets, and `serve` requires `--assets-dir`.

The project stack is Go 1.25.0, SQLite through `modernc.org/sqlite`, SOMETHING configuration, SQL migrations, standard-library HTTP and JSON, TypeScript ES modules, HTML, and CSS. Frontend development uses Node.js, `node:test`, jsdom, Playwright, axe-core, D3-force, esbuild, and TypeScript. `modernc.org/sqlite` is the only direct Go runtime dependency.

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
 filesystem-assembled TypeScript multi-page viewer
```

## 3. Repository and build layout

| Path | Responsibility |
|---|---|
| [Makefile](../Makefile) | Supported repository-root build, execution, test, documentation, coverage, fixture, viewer, and frontend interface. |
| `backend/cmd/analysis/`, `backend/cli/` | Thin executable entry and importable dispatch for `run`, `recover`, `migrate`, `pdf-annotate`, `serve`, and `version`. |
| `go.mod`, `go.sum` | Go module and dependency lock; Go commands execute from the repository-root module. |
| `backend/article/` | Parsed article model, text cleanup, DOI extraction, field projection, and DOI deduplication. |
| `backend/bibtex/` | Deliberately constrained article-only BibTeX tokenizer and parser. |
| `backend/bundle/` | Focused schema-pair consumer that derives configured metadata and PDF migration endpoints, validates complete history and readable migration inputs, and coordinates paired preparation, readiness, maintenance ownership, normalized inventory reconciliation, and forward retry; it does not import workspace or server orchestration. |
| `backend/database/` | Metadata composition root: writable and existing-only connections, SQLite URI and pragma configuration, the migration registry and runner, schema version, health, close, and one handwritten store per table family bound to the opened connection. |
| `backend/database/<family>/` | Handwritten table-family DALs for search, run, source, artifact, cache, work, author, review, audit, and PDF binding; each owns its `schema/` SQL inputs, private generated `internal/sql/` package, application models, validation, error mapping, and transactions. |
| `backend/enrich/` | Rate-limited provider HTTP client, payload-envelope validation, provider URL construction, and response decoding without database access. |
| `backend/logging/` | Process-wide structured logger and compile-time logging threshold. |
| `backend/manifest/` | Standard-library-only canonical manifests, fingerprints, lifecycle vocabulary, cache layout, and audit types. |
| `backend/normalization/` | Deterministic author, affiliation, publisher, and journal normalization. |
| `backend/pdfannotate/` | Application-owned PDF annotation boundary over the retained pdfcpu v0.15.0 dependency: accepts bounded source bytes and annotation data, returns bounded annotated bytes without opening databases or exposing library model types, and runs in a killable worker process for cancellation. |
| `backend/pdfstore/` | Companion PDF SQLite store with its own `schema/` inputs and generated `internal/sql/` package: normalized DOI registration, PDF validation, content blobs, read-only delivery, and the transactional audit outbox. |
| `backend/searchterms/` | Search-term extraction from source query strings and deterministic case-insensitive field matching that stems every word with the English snowball stemmer before whole-word matching, with wildcard support. |
| `backend/server/` | Existing-only metadata read and bounded-mutation connections, a separate companion PDF write connection, paired schema readiness, evidence and lifecycle APIs, bounded run/article PDF upload and removal, details, audit, artifact access, read-only PDF delivery, and frontend asset serving. |
| `backend/something/` | Lexer, ordered parser, directive expansion, type checker, evaluator, error boundary, and typed result accessors. |
| `backend/notes/` | Authoritative bounded note grammar, link extraction, diagnostics, and shared Go and JavaScript conformance fixtures. |
| `tooling/go/coveragecheck/` | Coverage policy parser and enforcement command used by `make coverage`. |
| `tooling/go/doccheck/` | Documentation links, format, migration, source catalog, hash state, dependency, and obsolete-reference validation. |
| `tooling/go/importcheck/` | Package import and transaction graph scanning, classification, enforcement, and reporting. |
| `tooling/go/pdf-store/` | Validated manual PDF insertion command. |
| `tooling/go/pdf-export/` | Read-only DOI-selected PDF export with title-derived filenames, stored-content integrity checks, and exclusive output creation. |
| `tooling/go/prepare-osf/` | Copy-only metadata, PDF, and optional configuration sanitization for OSF export. |
| `tooling/go/rawsqlcheck/` | Raw SQL ownership scanning, classification, enforcement, and reporting for maintained Go sources. |
| `tooling/go/something-printer/` | SOMETHING evaluation and inspection command with SOMETHING, JSON, and YAML output. |
| `backend/validation/` | Shared article field validation rules. |
| `backend/workspace/` | Typed configuration, attempt preflight, cache policy, pipeline orchestration, artifacts, and source loaders. |
| `config/` | Workspace, baseline types, database registry and chains, and coverage policy in SOMETHING. |
| `migrations/corpus.metadata/` | Metadata migrations V00001-V00028. |
| `migrations/corpus.pdf/` | Companion PDF migrations V00001-V00003. |
| `frontend/` | Browser production source, stylesheets, HTML shell, generated class declarations, pinned D3-force and PDF.js assets, licenses, and source build metadata. |
| `tooling/node/` | Frontend build, class, vendor, and development commands in the root npm workspace. |
| `internal/` | Shared production primitives for paths, migrations, hashing, query and HTTP budgets, read-only SQLite pools, text limits, and JSON/media projection. |
| `tests/` | Backend, frontend, Go and Node tooling, shared, HTTP contract, and explicitly gated E2E suites; support owns fixture construction and runner lifecycle. |
| `package.json`, `package-lock.json` | One npm workspace and lockfile for frontend, tooling, and tests, with explicit dependency ownership. |
| `build/frontend/dist/` | Ignored assembled browser served root. |
| `build/fixtures/viewer/` | Ignored generated viewer metadata and PDF fixture pair produced from authoritative Go fixture code. |
| `tests/e2e/offline/testdata/`, `tests/e2e/live/testdata/` | Small tracked CSV and BibTeX inputs for deterministic, mocked-provider, and opt-in live pipeline-to-viewer verification. |
| `docs/` | Maintained architecture, design, standards, usage, language, catalog, CSS, and documentation-state references. |

The root Makefile runs Go commands from the repository-root module and writes binaries and generated reports beneath `build/`. `make build` produces `build/analysis`; `make tools` produces `build/something-printer`, `build/pdf-store`, `build/prepare-osf`, `build/doccheck`, `build/coveragecheck`, `build/rawsqlcheck`, `build/importcheck`, and the ordinary-suite selector `build/testselect`. Runtime relative paths assume the repository root; process entrypoints do not discover or change the working directory.

## 4. Dependency direction and component boundaries

The internal production import direction is intentionally narrow. Shared low-level packages such as `manifest`, `something`, `logging`, `article`, `notes`, `normalization`, `validation`, and `internal/` helpers do not depend on orchestration. `backend/database/` is the metadata composition root and must not import consumers or the companion store; table-family packages under `backend/database/<family>/` own their SQL inputs, private generated output, application models, validation, error mapping, and transactions and depend only on their own subtree plus those shared packages, so sibling families never call one another inside a transaction. `workspace`, `server`, `tooling/go/`, and `cli` are consumers that import the composition root and the family packages whose application types they need. `enrich` owns provider HTTP without database access, `workspace` composes pipeline concerns, `server` uses family stores but does not import workspace orchestration or call migration-running `database.Open`, and `pdfstore` is the companion store that must not import consumers. `bundle` is a focused consumer that coordinates paired schema preparation, readiness, and normalized inventory reconciliation without importing workspace or server orchestration.

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
              |--> enrich (HTTP only)       |
              |                             |
              +--------------+--------------+
                             v
                   +-------------------+        +-------------------+
                   | database (root)   |<-------| pdfstore          |
                   | composition root  |        | companion store   |
                   +---------+---------+        +---------+---------+
                             |                            |
                             v                            v
        database/<family> handwritten DALs       pdfstore/internal/sql
        artifact, audit, author, cache,
        pdfbinding, review, run, search,
        source, work
                             |
                             v
             <family>/internal/sql generated queries
                             |
                             v
            database/sql + modernc.org/sqlite
```

`backend/cmd/analysis/main.go` is the application process entry, delegating to importable `backend/cli/`; maintained tools have separate thin entries under `tooling/go/cmd/`. Pipeline behavior belongs in `backend/workspace/`; provider gatherers do not call family stores, family and companion store functions do not perform provider HTTP requests, and `backend/server/` does not import workspace orchestration or call migration-running `database.Open`.

SQLite is the sole persisted pipeline record. Immutable `work_revisions`, ordered `authorships`, and ordered `reference_mentions` are the canonical corpus representation. `run_work_stages` records per-run, per-work outcomes independently from revision rows, so rejected work remains inspectable even though only valid normalized revisions enter the analysis-ready corpus.

Provider responses live in content-addressed `artifacts` and inline `artifact_blobs`; immutable response versions, request identity, and expiry live in `cache_entries`, and per-run use lives in `run_cache_uses`. The PDF store is a companion database rather than a metadata table family. The viewer opens metadata once in read-only mode and once in existing-only bounded-mutation mode, never creates a database or runs migrations, and opens the companion PDF store read-only for evidence plus an optional existing-only mutation connection.

### 4.1 Table-family data access

`backend/database/database.go` is the metadata composition root. It opens or creates the metadata database, configures the SQLite URI and pragmas, applies the configured migration chain behind an immediate lock, exposes platform methods for close, health, writability, and schema version, and binds one handwritten store per table family to that single connection. Consumers that need only the stores use the root fields; consumers that need family application types, filters, or errors import the owning family package directly, which is why `workspace` and `server` appear as importers of both the root and individual families.

| Family | Package | Primary tables and relationships |
|---|---|---|
| Search | `backend/database/search/` | `searches`, `search_revisions`, `execution_plans`; search revision and reusable plan hierarchy. |
| Run | `backend/database/run/` | `pipeline_runs`, `pipeline_run_reviewers`, `run_steps`, `pipeline_run_metrics`; attempt lifecycle, reviewer attribution, steps, and counters. |
| Source | `backend/database/source/` | `run_sources`, `source_records`, `source_filter_counts`; source acquisition, raw records, parse outcomes, and ordered filter evidence. |
| Artifact | `backend/database/artifact/` | `artifacts`, `artifact_blobs`, `run_artifacts`; content identity, bytes, and run, step, cache, and candidate links. |
| Cache | `backend/database/cache/` | `cache_entries`, `run_cache_uses`; response versioning, selection, expiry, and per-run use evidence. |
| Work | `backend/database/work/` | `works`, `work_identifiers`, `work_revisions`, `run_work_stages`, `reference_mentions`, `run_search_terms`, `work_revision_term_matches`, `run_term_match_reconciliations`; stable works, immutable revisions, stage outcomes, references, term coverage, and corpus, detail, evaluation, graph, and server-read projections. |
| Author | `backend/database/author/` | `people`, `author_occurrences`, `authorships`, `author_identity_resolutions`, `author_identity_candidates`; confirmed identity, observed names, ordered authorship, and uncertain candidate evidence. |
| Review | `backend/database/review/` | Review settings and contexts, immutable work-review versions and version children, mutable context heads, logical notes, note versions, note links, logical anchors, and anchor versions; compare-and-swap review mutation workflows. |
| Audit | `backend/database/audit/` | `audit_events`; append-only insertion, filtering, and provenance relationships whose primary result is audit evidence. |
| PDF binding | `backend/database/pdfbinding/` | Metadata `pdf_store_binding` and `pdf_audit_links`; companion binding and delivered-event links. |

Each family owns its maintained declarations and named queries under `backend/database/<family>/schema/`, its committed sqlc-generated package under the reserved `backend/database/<family>/internal/sql/` path, and its handwritten models, validation, error mapping, and transactions under `backend/database/<family>/`; generated row types are mapped to application types inside the family, never cross a handwritten family API, and neither the schema declarations nor the generated output feeds runtime migrations or SOMETHING versioning. The companion store follows the same shape with `backend/pdfstore/schema/` and `backend/pdfstore/internal/sql/`, and `tooling/go/prepare-osf/` keeps a private generated export package because its statements are maintenance orchestration.

Relationship SQL lives with the family that owns the returned resource or the mutation, in a specifically named Go and SQL file such as `cache/cache_use.go` with `cache/schema/cache_use.sql`, `artifact/run_artifact.go` with `artifact/schema/run_artifact.sql`, or `work/reference.go` with `work/schema/reference.sql`. A family may declare the external table shapes its relationship queries need to compile without taking ownership of those tables.

There is no generic read-model package. Every cross-table read belongs to the family owning its primary result: hierarchy and overview to `search` or `run`; corpus, details, evaluation, graph, references, and term matches to `work`; identity evidence to `author`; review navigation to `review`; audit and artifact evidence to `audit` or `artifact`; and companion inventory to `pdfstore`. Cross-table writes and transactions stay in the family owning the use case; they may run a specifically named relationship query that touches another family's table but never call a sibling DAL method, so abandoned-run recovery, artifact linking, cache-use recording, and review mutations with audit insertion each keep one transaction. `make check-imports` enforces this graph: family production code depends only on its own subtree and shared `internal`, `logging`, `manifest`, or `notes` packages, generated `internal/sql` output is imported only inside its owning package tree, and every generation unit keeps its schema, queries, and output under one owner directory.

Every public family operation takes `context.Context` as its first parameter and passes it to the generated context-aware method, so handler deadlines and cancellation reach SQLite and the viewer's statement-budget driver keeps counting generated calls. Multi-statement mutations remain handwritten Go: a family `withTx` helper opens `s.db.BeginTx(ctx, nil)`, binds the same generated queries with `WithTx(tx)`, runs validation, current-head checks, inserts, compare-and-swap updates, and audit evidence, rolls back on error or panic, and commits only after every invariant succeeds. No transaction spans two family stores or two databases.

### 4.2 Metadata and companion PDF coordination

Metadata and PDF state remain two independently migrated SQLite databases because document bytes and manual inventory have a different lifecycle from research metadata and SQLite cannot provide one transaction across separate files. `backend/pdfstore/` owns the companion connection, its `schema/` inputs, its generated `internal/sql` package, PDF validation, content-addressed blobs, and the audit outbox, while `backend/database/pdfbinding/` owns the metadata-side `pdf_store_binding` and `pdf_audit_links` tables and the typed operations that resolve and validate the relative binding, begin the metadata transaction, and record delivered events.

The cross-database protocol stays deliberately non-atomic. A flush loads a bounded companion outbox batch, isolates each event with a per-event savepoint, writes metadata audit plus `pdf_audit_links` in one metadata transaction through the binding store, commits metadata before marking companion events delivered, and retries idempotently after a crash between those commits. Section 13 describes the full inventory, binding, and outbox behavior, and [DATABASE.md](DATABASE.md) lists both schemas.

Workspace orchestration composes stage packages under `backend/workspace/`: `providercache` owns policy/provenance, `termindex` owns stored search reconciliation, `providers` owns provider application evidence, `revisions` persists immutable stage snapshots, `enrichment` applies and audits field changes, `validator` partitions articles, `normalizer` normalizes and records metrics, and `pdfsync` delegates companion inventory reconciliation to the bundle consumer. Server components under `backend/server/` separate request validation, graph assembly, and metric interpretation; shared production primitives remain under root `internal/`. These components are used by runtime callers as well as external tests; no source package exposes a test-only bridge.

## 5. Executable boundaries

`build/analysis run` accepts `--db`, `--config`, repeated `--workspace search_id@search_revision`, and `--fresh`. Without selectors it executes every declared workspace in declaration order. A matching completed execution plan is a no-op by default, but DOI inventory and stored search-term match reconciliation still run; the first run after a migration is a full new run because the schema version is part of the execution fingerprint. `--fresh` creates another attempt and records the rerun reason.

`build/analysis migrate --db <metadata.db>` prepares both configured stores for an existing metadata database, binds and initializes the default companion when none exists, and reconciles normalized DOI inventory and pending PDF audit delivery without running a workspace or creating a research attempt. It does not create a missing metadata database and rejects an existing binding whose companion file is missing or unsafe.

`build/analysis serve` accepts `--db`, `--addr`, and requires `--assets-dir`. It defaults to `corpus.metadata.db` and `127.0.0.1:8080`, rejects addresses whose host is not an exact loopback IP, acquires shared maintenance ownership, verifies both configured schema targets and companion protection contracts without migrating, opens metadata through separate read and bounded-mutation connections plus a companion read connection and an optional existing-only write connection, serves the assembled frontend assets from the given directory, and applies conservative HTTP timeouts.

`build/analysis version` prints the semantic version `MAJOR.MINOR.PATCH` from the compile-time `major`, `minor`, and `patch` constants in `backend/cli/main.go`.

`build/pdf-store add --db <metadata.db> --doi <doi> --file <pdf>` resolves the bound companion database from metadata. It requires an already normalized and registered DOI, accepts at most 20,000,000 bytes with a `%PDF-` signature, hashes content with SHA-256, stores identical bytes once, leaves an available DOI unchanged, and flushes idempotent PDF audit events.

`build/prepare-osf --db <metadata.db> [--config <workspace.something>] --out <new-directory>` creates a WAL-safe, sanitized, self-contained copy without overwriting output or changing any source. `build/something-printer [--json|--yaml|--something] <path>` evaluates a SOMETHING file and writes the evaluated public values in SOMETHING setup syntax by default, or JSON or YAML when selected by a flag. `build/doccheck` implements the documentation workflow. `build/coveragecheck` is built by `make coveragecheck` or `make tools` and is normally invoked through `make coverage` with the configured policy. `build/rawsqlcheck check` scans maintained Go sources for unclassified raw SQL calls, and `build/rawsqlcheck report` prints the per-file ownership inventory; both are normally invoked through `make check-sql` and `make sql-report`. `build/importcheck check` enforces the package import and transaction graph, and `build/importcheck report` prints the layer graph, family imports, fixture composition, and sqlc units; both are normally invoked through `make check-imports` and `make import-report`.

The process logger is installed as the standard `slog` default and package loggers come from `logging.Logger(component)`. The compile-time minimum is `slog.LevelInfo`; there is no runtime `--log-level` option.

## 6. Configuration architecture

`config/workspace.something` includes `config/baseline.something`, declares format-version-2 workspaces, optional reviewer username and email, source queries and filter counts, rename and keep mappings, enrichment settings, reuse policy, and cache policy. `workspace.Load` reads the selected file once, evaluates the exact bytes, ignores iteration scopes without a `workspace` value, rejects duplicate selectors, trims and bounds reviewer values, and produces typed `manifest.ResolvedManifest` and `enrich.Config` values. Reviewer identity is persisted for every attempt but excluded from resolved and input manifests and execution fingerprints.

Workspace selectors are stable `search_id@search_revision` strings. Supported source file types are `csv` and `bib`. Supported reuse policies are `reuse_completed`, `fresh`, and `retry`; command-line `--fresh` forces a new attempt.

Provider endpoint values, requested fields, fill policy, rate, concurrency, timeout, retries, batch size, and extra URLs are configured, but provider keys and orchestration order are implemented code: Crossref, OpenAlex metadata and references, then ORCID exact profiles and name-search evidence. `max_retries` counts retries after the initial request.

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

After normalization, the pipeline derives per-run search-term coverage: it parses the run's recorded source queries into a deduplicated term inventory, matches each term against the normalized revision's title, abstract, keywords, and keywords plus, and stores both the inventory and the per-revision matches in derived tables keyed by `pipeline_run_id`. The derivation is deterministic and uses the same in-memory fields that were just persisted.

Before successful completion, normalized revisions are grouped by work and synchronized into the companion PDF inventory. Missing rows become `not_available`, and outbox events reach metadata audit. A reused completed plan performs the same PDF and search-term reconciliation without rerunning the stages: every completed run that lacks stored term data is backfilled from its stored queries and normalize revisions, and reconciliation failures are logged without failing the run.

```text
preflight
   |
   v
parse -> deduplicate -> enrich metadata -> enrich identity -> validate -> normalize -> term matches
   |          |                |                 |              |          |              |
   +----------+----------------+-----------------+--------------+----------+--------------+
                                       |
                                       v
                     revisions + stages + artifacts + metrics + audit
                                       |
                                       v
                     DOI inventory sync + term-match reconciliation
```

Stages use family store operations and scoped transactions rather than one transaction for an entire run. A late failure marks the attempt failed but intentionally preserves earlier immutable evidence.

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

Author identity resolution statuses are `orcid_is_unclear`, `no_orcid_candidate`, `provider_failed`, `confirmed`, and `rejected`. Candidates retain provider rank, ORCID, display name, raw payload artifact, and evidence context. A name-search candidate remains review evidence and does not silently change confirmed identity.

Validation checks title, authors, year, DOI, publisher, and reference-count conditions in `backend/validation/validation.go`. Discarded work remains visible through stage and provenance records but receives no normalized revision.

Normalization is deterministic and heuristic. Author processing separates given and family names and derives initials; affiliation and publisher processing expands known abbreviations and applies local title casing; journal canonical forms are stored in revision extension JSON while the main journal field preserves the revision value.

Reference mentions are immutable and ordered within a revision. DOI resolution targets a work identity when known. The viewer resolves a work to one selected-run revision using stage precedence `normalize`, `validate`, `enrich_identity`, `enrich_metadata`, `enrich`, `deduplicate`, then `parse`, with newest revision ID breaking ties.

## 11. Persistence and migrations

Writable database opening creates the parent directory, configures SQLite foreign keys and a five-second busy timeout on every connection, enables WAL, and applies the configured chain. Tracking-table creation and each migration run behind `BEGIN IMMEDIATE`, which serializes schema changes across independent processes.

Paired preparation is owned by `backend/bundle/`. It validates both configured chains, declared supersedes aliases, and every migration input before changing either store, acquires exclusive maintenance ownership, applies metadata migrations followed by PDF migrations, and verifies both targets. Each migration retains its own transaction, so a failed PDF upgrade after a committed metadata upgrade reports the failed store and a later preparation resumes forward without downgrading, wiping, or rewriting either store. Readiness verification opens both stores read-only, checks the companion protection contracts, and never creates files or applies migrations. Inventory reconciliation and pending audit delivery are separate from schema readiness and never create a research attempt.

The metadata chain is V00001-V00028 and the PDF chain is V00001-V00003. Migration execution follows SOMETHING iteration order, skips applied filenames, records checksums without later revalidation, does not use `previous` or `upgrade` to order execution, and has no downgrade runner even though files retain `-- ==DOWN==` sections. A migration may declare a verified equivalent earlier filename in `supersedes`; when that filename is already recorded, opening adopts the canonical filename in the same migration transaction without rerunning the SQL and retains both history rows.

The metadata schema includes pipeline planning and evidence, corpus revisions and relationships, cache and PDF links, `pipeline_run_reviewers`, singleton `review_settings`, run-scoped `review_contexts`, immutable review, note, link, and anchor version tables, mutable context head tables, and derived per-run search-term inventories and revision matches. [DATABASE.md](DATABASE.md) lists every table and relationship.

The companion schema includes `schema_migrations`, `pdf_blobs`, `pdf_documents`, `pdf_gather_runs`, `pdf_download_attempts`, and `pdf_audit_outbox`. Current runtime behavior uses normalized DOI registration and validated manual inventory. [DATABASE.md](DATABASE.md) is the detailed reference for every table, column, default, relationship, constraint, index, trigger, and repository-level expectation in both databases.

Append-only triggers reject updates and deletes from audit events, work revisions, author occurrences, authorships, reference mentions, review contexts, logical notes and anchors, all immutable review versions and version children, and PDF download attempts. Supported code inserts reviewer rows once and does not update them. PDF blobs reject updates, and the runtime exposes no PDF-blob deletion path. Only review head tables move through family transactions. Foreign keys protect reviewed runs and their immutable review lineage.

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

PDF content is addressed by SHA-256 in `pdf_blobs`. `pdf_documents` has two states: `not_available` has no content hash or inventory timestamp, and `available` has both. Registration is idempotent. Conditional save through the manual tool or serving API validates size and signature before storage, requires prior normalized registration and the current document change counter, and uploads, replaces, or reports identical bytes unchanged. Removal clears the selection and inventory timestamp while retaining the registration. Outgoing bytes are deleted only when no current document selects them.

Manual and serving save and removal run in one immediate companion transaction: the store validates the current document counter, inserts or reuses a fully verified blob, moves the selection with compare-and-swap, deletes outgoing bytes only when no current document selects them, and appends the outbox event. Any pre-commit failure rolls back all of those effects and leaves the previous selection intact.

PDF events enter `pdf_audit_outbox` in the same transaction as the PDF mutation. Uploads record `pdf_document_inventoried`, replacements record `pdf_document_replaced`, and removals record `pdf_document_removed`. Changed events carry bounded nullable before/after state: status, selected hash, byte size when known, inventory timestamp, change counter, and whether outgoing bytes were deleted or retained. Manual events remain globally work-scoped with a null owning `pipeline_run_id`; initiating run and revision identifiers stay in bounded metadata. Flushing writes metadata `audit_events` plus `pdf_audit_links` in one metadata transaction and then marks the outbox row delivered. Repeated delivery is idempotent across interruption.

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

`server.Open` rejects empty, missing, and directory paths; acquires shared maintenance ownership for the server lifetime; verifies the configured metadata target and, when a companion is bound, the PDF target and its protection contracts without migrating; opens metadata SQLite using `mode=ro` and `query_only` for browsing; opens the same existing file with a single `mode=rw` connection for bounded review and lifecycle transactions; verifies the review schema and its append-only triggers; discovers non-system tables and columns; and opens the bound PDF store read-only for evidence plus an existing-only `mode=rw` companion connection when the file is writable, and verifies its immediate-transaction, busy-timeout, foreign-key, and secure-deletion settings before advertising mutation capability. A read-only companion still serves evidence with an explicit non-writable capability. Startup closes every opened connection and releases maintenance ownership if a later validation step fails. `serveMain` starts the bounded PDF audit relay only after the listener is ready; `server.Open` performs no data writes.

The server accepts GET and HEAD for evidence reads, narrowly routed POST, PUT, and DELETE review or lifecycle mutations, and one raw `application/pdf` PUT plus empty-body DELETE for run/article PDF changes. JSON mutations require JSON content type, reject unknown fields and trailing values, cap request bodies, enforce same-origin requests when `Origin` is present, return structured status and conflict codes, and use no-store caching. PDF uploads require one strong mutation ETag, positive `X-PDF-Size`, lowercase `X-PDF-SHA256`, at most 20,000,000 complete bytes, and matching media, size, digest, and PDF signature. Upload routes have a 60-second body deadline, a finite response deadline, and admission for at most two retained bodies; the ordinary five-second query timeout resumes after ingestion. Mutation preconditions are rechecked inside the companion transaction, so interrupted or stale uploads never create a replacement event or delete old bytes. Every JSON API response has a serialized-byte ceiling of 512 KiB for compact routes, 2 MiB for collection routes, or 4 MiB for detail routes; an overrun returns the stable `response_budget_exceeded` error instead of a partial document. Every API request has a deterministic SQLite statement ceiling of 64 for compact routes, 256 for collection routes, or 1,024 for detail and review routes; an overrun returns `query_budget_exceeded` for JSON routes. `X-Response-Byte-Limit`, `X-Query-Count-Limit`, and `X-Query-Count-Used` expose the applicable contract, with query use delivered as a trailer for streamed PDF and artifact content. Table and sort identifiers are validated against discovered schema, artifact and PDF downloads use conservative headers, and API request contexts time out after five seconds. HTTP server timeouts are five seconds for headers, ten seconds for reads, fifteen seconds for writes, and thirty seconds idle; upload routes extend read and write deadlines through the response controller before the JSON response buffer. The server rejects Host authorities that differ from its exact loopback listener to limit rebinding attacks. PDF evidence reads recheck the current selection before reusing process-local cache bytes, and committed mutations invalidate that cache. The embedded reader pins its URL to the selected content hash, and a changed selection returns `pdf_changed` rather than combining ranges from two files.

| Route | Responsibility and bounds |
|---|---|
| `/api/health` | Reports database health, review capability, and the opaque corpus ID used to namespace browser drafts. |
| `/api/hierarchy`, `/api/runs/{id}/context` | Returns bounded searchable hierarchy pages and one canonical complete run ancestry. Legacy `/api/searches`, `/api/plans`, and `/api/runs` discovery responses are explicitly deprecated and hard-bounded. |
| `/api/runs/{id}/visibility` | Moves a terminal attempt between `active` and `trashed` in one transaction and appends the corresponding lifecycle audit event. |
| `/api/overview` | Reports captured execution metrics and current derived coverage. |
| `/api/audit`, `/api/audit/{id}/recorded-data` | Returns bounded cursor audit pages, first-page facets, structured review filters, and lazy privacy-scrubbed recorded data. |
| `/api/runs/{id}/artifacts`, `/api/artifacts/{id}/inspect`, `/api/artifacts/{id}/content` | Lists artifacts through numbered page and page-size metadata while retaining the bounded cursor contract for existing callers, previews a bounded text prefix, or downloads stored artifacts. |
| `/api/runs/{id}/cache-uses` | Returns cache evidence for one run. |
| `/api/runs/{id}/corpus/{kind}` | Returns bounded articles, authors, references, or sources for the selected run; article rows include metadata, stored search-term matches, PDF inventory, review state, filter facets, progress, and adjacent unreviewed navigation through the same projection as `/evaluation`. |
| `/api/runs/{id}/evaluation`, `/api/runs/{id}/identity-evidence`, `/api/runs/{id}/stages` | Returns the same article/review projection as Corpus, uncertain identity evidence, or stage outcomes. |
| `/api/runs/{id}/review-context`, `/api/runs/{id}/review-context-candidates` | Returns, proposes, explicitly initializes, or lists bounded eligible parents for one completed non-trashed run. |
| `/api/runs/{id}/articles/{revision}/review`, `/api/runs/{id}/articles/{revision}/review/versions` | Reads or appends complete review state and returns bounded immutable ancestry with optimistic concurrency. |
| `/api/runs/{id}/articles/{revision}/notes`, `/api/runs/{id}/notes/{note}`, `/api/runs/{id}/notes/{note}/versions` | Creates, reads, edits, tombstones, restores, and lists bounded immutable note versions and resolved links. |
| `/api/runs/{id}/articles/{revision}/anchors`, `/api/runs/{id}/anchors/{anchor}/versions`, `/api/runs/{id}/links/backlinks` | Creates and reads bounded PDF anchors, tombstones, ancestry, and current-version backlinks. Creation, active edits, and restoration require the reader-supplied PDF content hash; tombstones retain historical hash evidence and remain possible after PDF removal. |
| `/api/tables`, `/api/tables/{table}` | Discovers tables and browses an allowlisted schema with bounded page sizes. |
| `/api/articles/{id}`, `/api/authors/{id}`, `/api/references/{id}`, `/api/articles/{id}/collections/{kind}`, `/api/authors/{id}/collections/{kind}`, `/api/identity-resolutions/{id}/candidates` | Requires selected-run membership, returns bounded detail summaries, and exposes counted run-owned cursor subresources for related evidence and identity candidates. Article summary retains stored search-term coverage for its exact normalized revision. |
| `/api/works/{work_id}/pdf-status`, `/api/pdf/{work_id}` | Reports registration, change counter, mutation ETag, upload limit, storage capability, and current inventory status, or delivers validated available PDF bytes. An embedded reader may pin the response with a validated `content_hash`; a changed pin returns `pdf_changed` instead of mixed ranges. |
| `/api/runs/{run_id}/articles/{work_revision_id}/pdf` | Accepts one conditional raw PDF save as PUT or an empty-body DELETE, resolves the exact selected-run normalized revision and DOI, and returns `changed`, `operation`, current `pdf_status`, nullable `event_key`, and `audit_delivery`. |
| `/api/graph` | Returns one bounded graph model with explicit truncation evidence. |

Run-scoped articles contain valid normalize-stage revisions. Article browsing remains available for historical runs without execution-plan lineage, with review initialization unavailable. Identity evidence projects one title/DOI pair per resolution and work, chooses the current normalized article when available, and retains the captured revision and producer stage. Author detail follows earlier same-run, same-work snapshots only when author order, recorded names, and observed ORCID are identical; candidate evidence never creates a confirmed identity. Author and reference collections include relationships attached to revisions produced during the run, so consumers use revision and producer-stage identity when uniqueness matters.

Audit supports run context, text search, category, action, actor, entity, stage, outcome, review status, review reason, and review substatus filters. Categories accepted by the server are pipeline, enrichment, validation, PDF, and review. Run-scoped PDF history includes only works that belong to that run, while workspace-global PDF history is an explicit scope. The frontend presents UTC date-grouped audit rows and cursor-based loading without discarding already-loaded evidence; expanding a row loads readable changes and separately disclosed recorded JSON under fixed privacy and byte budgets. Artifact preview defaults to 64 KiB and caps at 256 KiB; only recognized text, JSON, and SOMETHING media are previewed.

Graph modes are `research_network`, `article_author`, `citation`, and `article_reference`. The endpoint caps article nodes at 2,000, related nodes at 10,000, and edges at 20,000, and reports truncation and its reason.

Every review mutation validates the selected completed active run, context head, work revision, vocabulary, and expected version before one metadata transaction inserts a child version, compare-and-swap moves only that context head, and appends safe audit evidence. Decision and Note mutations require exact work membership; anchor creation and restoration additionally require current matching PDF availability. The companion PDF availability check remains outside the metadata transaction; PDF replacement and removal are separate companion transactions, so review mutations validate anchors against the current document state.

PDF save and removal resolve the exact current normalized revision for the selected run, require a completed active run and a registered DOI, validate transport and the strong mutation ETag before retaining a body, then delegate to one immediate companion transaction. The transaction rechecks the document change counter, verifies stored bytes, moves the selection with compare-and-swap, deletes outgoing bytes only when unreferenced, and writes one outbox event before commit. A committed mutation whose response is lost is an uncertain outcome; the client must reconcile `pdf-status` instead of retrying a stale ETag. Metadata audit delivery remains asynchronous and outside the request query budget.

Anchor creation, active edits, and restoration acquire a companion-owned immediate write reservation before the metadata transaction and retain it through anchor commit, so PDF replacement or removal cannot interleave between hash validation and metadata evidence. The lock order is always PDF reservation before metadata. Audit delivery commits its metadata transaction before marking companion delivery, so it never holds a metadata transaction while waiting for a PDF lock. Tombstones do not require current PDF bytes and retain the anchor version's historical hash; creation and restoration require the reader's exact current hash.

```text
browser GET/HEAD -> read-only metadata or PDF query -> bounded response
browser POST/PUT -> transport and context validation -> optional anchor PDF check -> metadata version + head CAS + audit transaction
```

## 15. Frontend architecture

The frontend is a framework-free native ES-module multi-page application authored in TypeScript, including its build scripts, Playwright configuration, and browser specs. `make check-frontend` first verifies the generated CSS token registry and statically visible class uses, then type-checks sources, scripts, configuration, and tests with `tsc --noEmit`; `make frontend-build` compiles each production source file with esbuild into the `build/frontend/dist` served root, rewrites relative `.ts` and `.tsx` import specifiers to `.js`, and asserts that the assembled root contains no `.ts`, `.tsx`, `.d.ts`, or `.map` files and no dot- or underscore-prefixed entries. Unit tests are authored in TypeScript under `tests/frontend/<component>/<layer>/` and type-checked by `tsc --noEmit`; `make test-frontend-unit` runs them through Node's native type stripping for `.ts` sources and an esbuild loader hook for `.tsx` sources, so no build step is required. `frontend/index.html` is the authoritative accessible shell template, and `tooling/node/frontend/build.ts` generates `index.html` plus one identified HTML document for each Deepdive and detail view. `app.tsx` is a side-effect entry that binds selector changes, context-preserving navigation, history, notices, loading controls, health state, mobile navigation, and initial rendering in every document.

Project branding lives under [assets/](../assets/). The frontend build copies `CoGen_Header_Logo_2400x600.png` and `CoGen_favicon_2048.png` into the served `assets/` directory for the shared header and browser icon. The [README](../README.md) uses `CoGen_GitHub_Banner_3000x1000.png` directly; the master image `Teal Fox Constellation Emblem.png` is excluded from the frontend build.

The viewer state is the source of truth for `search_id`, `search_revision_id`, `plan_id`, `run_id`, view, section, filters, sort, page, expanded row, selected graph node, artifact inspector state, focused `note_id`, focused `anchor_id`, and `pdf_page` where applicable. `state.tsx` owns a module-level `viewerState` object mirrored to `sessionStorage` under `rw-viewer-state` and carried in `history.state` on every push or replace. The view derives from the pathname (`/overview` maps to `overview`), and `state.link` returns a clean path-only URL preserving only canonical context plus destination-owned keys. Cross-view links and `router.setURL` transitions persist the destination state and perform native document navigation; same-view filters, pagination, cursors, graph controls, and section changes keep `pushState` or `replaceState` plus an in-document render. Anchors that carry state beyond the view path render `data-state` JSON consumed by the click handler. Native fragment links within the current document, including Skip to content, bypass route interception. Initial document rendering preserves normal browser focus on every view; explicit same-view navigation may focus the rendered page title. Legacy `?view=...` query parameters are ignored at boot and no longer supported.

`router.render` increments a request sequence, aborts the prior controller, refreshes context selectors, dispatches the selected view, updates the document title, ignores stale or aborted responses, and clears loading state only for the current request. Views fetch JSON, assign a built JSX tree to a descriptive binding, pass that binding to the project-owned `render` runtime documented in [JSX-RUNTIME.md](JSX-RUNTIME.md), and then bind interactions that require the new DOM.

```text
page load or same-view history event
      |
      v
app.tsx -> router.render -> abort prior request -> hydrate selectors -> dispatch view
      |                                                   |
      |                                                   v
      +---------------------------------------------- API fetch
                                                          |
                                                          v
                                    render(viewMarkup, app) into #app
                                                          |
                                                          v
                                              bind view interactions
```

### 15.1 Frontend file ownership

| Path | Responsibility |
|---|---|
| `index.html` | Authoritative shell template for the generated home, Deepdive, and detail HTML documents. |
| [tooling/node/frontend/build.ts](../tooling/node/frontend/build.ts) | Per-file TypeScript compilation, HTML page generation, static-asset assembly, and served-root validation. |
| [tooling/node/frontend/generate-classes.ts](../tooling/node/frontend/generate-classes.ts) and [tooling/node/frontend/check-classes.ts](../tooling/node/frontend/check-classes.ts) | Generated CSS token registry, freshness verification, statically visible use validation, and unused-token reporting. |
| `app.tsx` | Global event binding, same-view history interception, cross-view native-navigation protection, shell initialization, and first render. |
| `jsx/classes.ts` | Committed generated `ClassName` union derived from the six authoritative stylesheets. |
| `jsx/jsx-runtime.ts` | Project-owned classic-mode JSX runtime: `h`, `Fragment`, typed class composition and DOM class helpers, `render`, `renderToString`, and the controlled `raw` escape hatch. |
| `jsx/jsx.d.ts` | Ambient global `JSX` namespace with class names narrowed to generated tokens or branded combinations. |
| `state.tsx` | URL values, view-to-page ownership, DOM state, escaping, formatting, shared JSX panels, tables, flows, links, statuses, and global UI behavior. |
| `api.tsx` | Abort-aware JSON reads, JSON and binary PDF mutations, conditional request headers, endpoint construction, structured API errors, and table discovery cache. |
| `router.tsx` | Request sequence, abort lifecycle, selector hydration, view dispatch, native cross-view navigation, same-view history state, document title, and page-title focus. |
| `components/context-selector.tsx` | Dependent bounded searchable single-select hydration, skeletons, keyboard listbox interaction, exact selected options, and sole-child auto-selection. |
| `components/data-table.tsx` | Shared rows, sorting, search, page size, expansion, and control binding. |
| `components/pagination.tsx` | First, Previous, numbered, Next, and Last controls plus result ranges. |
| `components/audit-events.tsx` | Audit classification, summaries, metadata disclosures, stream markup, and investigation export. |
| `components/graph.tsx` | Query state, connected components, canvas lifecycle, simulation, drawing, interactions, export, selection, and edge table. |
| `components/shell.tsx` | Health state and responsive navigation toggle. |
| `components/note-parser.tsx` | Safe bounded note preview, link diagnostics, unresolved states, and context-preserving link rendering. |
| `components/note-editor.tsx` | Immutable note edits, tombstones, restoration, history comparison, and corpus-scoped browser drafts. |
| `components/pdf-management.tsx` | Conditional PDF upload, replacement, removal, local file identity, capability reasons, confirmation, uncertain outcomes, and targeted reader/evidence refresh. |
| `components/pdf-viewer.tsx` | Custom vendored PDF.js lifecycle, single-page canvas and selectable text rendering, boundary-aware navigation, rotation, zoom, selection geometry, anchor highlights, the inverted theme toggle, and the fullscreen toolbar toggle. |
| `components/pdf-fullscreen.tsx` | Browser-fullscreen reading mode with a CSS fallback class, the collapsible review drawer, the drawer edge control, and selection-driven auto-expand. |
| `components/review-panel.tsx` | Explicit context initialization, lineage selection, complete status saves, history, notes, anchors, PDF integration, and the fullscreen selection handoff event. |
| `views/home.tsx` | Context-independent hierarchy metrics, Explore links, and reversible run-visibility dialog behavior. |
| `views/*.tsx` | Page-level fetching, rendering, and post-render binding for Deepdive and detail routes. |
| `styles/tokens.css` | Theme, spacing, type, status, graph colors, focus, radius, and elevation tokens. |
| `styles/base.css` | Reset, document layout, typography, links, landmarks, and reduced motion. |
| `styles/elements.css` | Buttons, labels, messages, headers, loaders, and segments. |
| `styles/collections.css` | Navigation, selectors, grids, forms, tables, pagination, breadcrumbs, and responsive collections. |
| `styles/views.css` | Overview, details, provenance, audit, artifacts, article review, and stage presentation. |
| `styles/graph.css` | Relationship layout, controls, canvas, overview, legend, selection, and edge table. |
| `vendor/d3-force.js` | Generated pinned force-simulation implementation; it is changed only through `make frontend-vendor`. |
| `vendor/pdfjs/` | Generated PDF.js 4.2.67 core, matching worker, CMaps, standard fonts, and license assets; it is changed only through `make frontend-pdfjs-vendor`. |

### 15.2 Frontend module graph

```text
generated *.html -> app.tsx
  +-- state.tsx                    no project imports
  +-- router.tsx
  |     +-- state.tsx
  |     +-- context-selector.tsx --+-- state.tsx
  |     |                          +-- api.tsx -> state.tsx
  |     |                          +-- router.tsx
  |     +-- home.tsx --------------+-- state.tsx, api.tsx, router.tsx
  |     +-- overview.tsx ----------+-- state.tsx, api.tsx, router.tsx
  |     +-- corpus.tsx ------------+-- state.tsx, api.tsx, data-table.tsx, pagination.tsx
  |     |   +-- evaluation.tsx ----+-- state.tsx, api.tsx, article-table.tsx, data-table.tsx
  |     +-- relationships.tsx -----+-- state.tsx, api.tsx, graph.tsx, router.tsx
  |     +-- provenance.tsx --------+-- state.tsx, api.tsx, data-table.tsx, router.tsx
  |     +-- advanced.tsx ----------+-- state.tsx, api.tsx, data-table.tsx, router.tsx
  |     +-- detail.tsx ------------+-- state.tsx, api.tsx, pagination.tsx, review-panel.tsx, pdf-fullscreen.tsx
  +-- api.tsx
  +-- context-selector.tsx
  +-- shell.tsx -> api.tsx

graph.tsx -> state.tsx, router.tsx, pagination.tsx, vendor/d3-force.js
data-table.tsx -> state.tsx, router.tsx, pagination.tsx
review-panel.tsx -> api.tsx, state.tsx, note-editor.tsx, pdf-viewer.tsx
pdf-fullscreen.tsx -> jsx-runtime.ts
note-editor.tsx -> api.tsx, state.tsx, note-parser.tsx
```

`state.tsx` is the shared leaf and imports only the project-owned JSX runtime. Components do not import views. `router.tsx` and `context-selector.tsx` form one intentional ES-module cycle because selector interactions call `setURL` while rendering calls `hydrateSelectors`; bindings run only after module initialization, and new modules must not expand the cycle. Every generated HTML document loads CSS in tokens, base, elements, collections, views, and graph order inherited from the authoritative `index.html` template.

Home summarizes the search/revision/plan/run hierarchy, establishes complete Deepdive context, and owns reversible run-visibility actions. Overview separates captured execution metrics from current derived coverage. Corpus selects articles, authors, references, source records, or identity evidence without nested tabs; article rows expand to show the stored search-term coverage for the run. Relationships provides four bounded graph models plus a table equivalent. Provenance covers audit, artifacts, cache, stages, and run detail. Shared audit presentation groups compact rows by UTC date and expands each event inline into recorded context and readable before/after evidence; privacy-scrubbed payloads are fetched on demand, with JSON exposed separately. Row expansion remains available through keyboard focus and touch, and cursor appends preserve already-open evidence. The Corpus article collection also owns PDF inventory, current review state, review initialization, filtered unreviewed navigation, and the run Notes index. `/evaluation` navigation normalizes to `/corpus` while preserving saved article filters. Article detail provides conditional PDF upload, replacement, and removal controls, a side-by-side PDF/review workspace, complete status history, note and link versions, content-hash-bound anchors, and a search term coverage panel; author detail owns candidate ORCID evidence. Advanced exposes discovered tables, and detail views remain within Corpus navigation.

The graph uses the generated checked-in D3-force bundle without a CDN. Canvas rendering is supplemental to DOM controls, legend, cluster overview, search, selection details, paginated edge table, fit, zoom, drag, expansion, and PNG export. Shape encodes entity type and color encodes connected component.

The rendering and frontend extension rules live in [STANDARDS.md](STANDARDS.md), the visual and interaction contract lives in [DESIGN.md](DESIGN.md), and active stylesheet selectors and tokens live in [CSS-REFERENCE.md](CSS-REFERENCE.md).

## 16. Testing architecture

Ordinary Go tests use `testing` in external packages under `tests/<domain>/<component>/{unit,functional,integration}/`, with directory-based selection and no ordinary layer tags. `make test` selects all ordinary Go, Node, and browser suites, while `make test-go` selects backend, tooling, and shared Go suites without cached results. `DOMAIN`, `LAYER`, and `COMPONENT` narrow that scope; invalid or empty scopes and unmatched focused Go expressions fail. The isolated `e2e` tag gates offline E2E and `e2e_live` additionally gates live tests; `make test-e2e` alone selects the offline E2E tests, builds and invokes `build/analysis`, evaluates generated SOMETHING configuration through `build/something-printer`, and writes deterministic and mocked database bundles beneath `build/e2e/`. Real-database integration and E2E tests use production migrations.

`make test-race` runs all Go tests under the race detector and applies to concurrency, cache, HTTP-client, database, and lifecycle changes. `make coverage` explicitly instruments backend, tooling, and shared production packages, including uncovered packages, writes an atomic profile, and enforces the SOMETHING package/file floors without counting test support. Extracted components retain their former owner floors; thin process wrappers and documented inspection tools are explicitly excluded. `GOFLAGS=-p=2 make test-race` bounds package parallelism for comparable SQLite-heavy runs. `make format-check`, `make vet`, `make check-sql`, `make check-imports`, and `make check` cover formatting, static checks, raw SQL ownership, the import and transaction graph, and complete read-only documentation consistency.

Frontend unit tests use `node:test`, `node:assert`, and jsdom without a browser or server. Playwright configuration at `tests/playwright.config.ts` and browser specs under `tests/frontend/` and `tests/contracts/` are authored in TypeScript and split each invocation into a read-only suite and a serial mutation suite, with a fresh copied fixture pair and isolated viewer for each browser project and mutation spec on an operating-system-assigned loopback port, and write unique output under `build/playwright/`; `viewer.spec.ts` covers fixture evidence, serial `review.spec.ts` covers local review mutations and PDF rendering, serial `pdf-management.spec.ts` covers visible PDF upload, replacement, removal, download, failure, stale-tab, and draft-preservation behavior, `ui-quality.spec.ts` covers axe, responsive behavior, and reviewed visual snapshots, and the environment-guarded `e2e.spec.ts` mutates and verifies A1 and A2 review lineage over a database generated by the real pipeline binary.

Current test counts are derived from source rather than maintained in prose. The normative test and verification policy is [STANDARDS.md](STANDARDS.md).

## 17. Security and data-integrity boundaries

The viewer has no authentication or authorization, so writable serving requires an exact loopback IP listener and exact Host authority. It writes append-only review evidence, mutable review heads, and audited reversible run visibility in existing migrated metadata, keeps pipeline evidence and PDF evidence reads read-only while preparing a separate existing-only companion write connection for validated run/article PDF saves and removals, validates bounded inputs, strong mutation preconditions, origins, and upload identity, escapes frontend and note content, and makes no production CDN request.

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
- The frontend uses page-file navigation, URL state, and native modules so research context is shareable, reload-safe, and independent from a framework runtime; cross-view transitions load a document, same-view state changes re-render in place, and sources plus generated pages are assembled into `build/frontend/dist` by `make frontend-build` before serving.
- D3-force is pinned and generated as one checked-in browser asset so the assembled frontend requires no CDN; Node is needed only to build assets and run frontend tests, not while the viewer serves them.
- PDF.js 4.2.67 is pinned and generated with its matching worker, CMaps, fonts, and license so PDF rendering requires neither a CDN nor the default PDF.js viewer UI.
- Documentation generation and validation use one Go tool so maintained project tooling does not introduce another runtime language.
- Documentation state updates are explicit acknowledgements because a hash detects changed bytes but cannot prove semantic accuracy.

## 19. Assumptions and constraints

- Commands and runtime relative paths assume execution from the repository root.
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
- Frontend rendering uses the project-owned JSX runtime documented in [JSX-RUNTIME.md](JSX-RUNTIME.md); text and attributes are escaped automatically, and the controlled `raw` escape hatch accepts only trusted, already-escaped markup.
- The router and context selector have one intentional module cycle that must not expand.
- Canonical application links are clean per-view paths with no query parameters; state lives in `history.state` mirrored to `sessionStorage`, and legacy `?view=...` URLs are rejected.
- Each cross-view document load repeats health, hierarchy hydration, and view requests, and `http.FileServer` relies on `Last-Modified` revalidation without a project cache policy.
- Research context and filters in copied URLs or browser history may reveal search and run identifiers.
- Fixture databases and visual snapshots prove behavior only for controlled data and the tested browser and platform.
- The process-wide logging threshold is compile-time and cannot be changed by a command-line option.

## 21. Safe extension points

- Add a metadata or PDF migration file with both section markers, append it to the correct SOMETHING chain, and never rewrite an applied migration.
- Add an enrichment provider by implementing provider HTTP and decoding in `enrich`, registering ordering and storage orchestration in `workspace`, preserving cache evidence, and using controlled server tests.
- Add an evidence endpoint with GET or HEAD, read-only SQLite, allowlisted identifiers, bounded input and output, context timeouts, and integration coverage. Add a mutation only through the existing transport guards, a narrow family transaction, required state checks, and identifier-only audit metadata; review-decision mutations additionally record the bounded complete previous and new decision in audit before/after fields while preserving the PDF availability boundary, optimistic head compare-and-swap, and immutable version model. Add a PDF mutation only for an exact selected-run normalized revision through a strong mutation ETag, bounded raw body, and the companion store transaction.
- Add a frontend view by preserving URL context, registering router dispatch, primary navigation, `state.viewPage`, and the static page definition in [tooling/node/frontend/build.ts](../tooling/node/frontend/build.ts), rendering through established state and component helpers, binding after DOM replacement, and adding proportional tests.
- Add reusable frontend behavior to a component without importing view modules or expanding the router and selector cycle.
- Add an attached Go documentation comment or adjacent JavaScript JSDoc with every maintained declaration, then regenerate [PROJECT_CATALOG.md](PROJECT_CATALOG.md); JavaScript test callback titles supply test descriptions, and documentation checks reject missing source descriptions.

## 22. Documentation architecture

[AGENTS.md](../AGENTS.md) is the entry point, this document owns system architecture, [STANDARDS.md](STANDARDS.md) owns normative change rules, [DESIGN.md](DESIGN.md) owns frontend behavior and visual contracts, [CSS-REFERENCE.md](CSS-REFERENCE.md) owns active styles, [FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) owns frontend code style and naming, [APP-USAGE.md](APP-USAGE.md) owns user experience and operating expectations, [PROJECT-USAGE.md](PROJECT-USAGE.md) owns developer and operator workflows, [PROJECT_CATALOG.md](PROJECT_CATALOG.md) is generated source navigation, [something.spec.md](something.spec.md) owns the language, and [DOC-STATE.md](DOC-STATE.md) records reviewed documentation hashes and dependency guidance.

`doccheck check` validates local links, exact obsolete references, migration configuration and documented boundaries, single-line Markdown, catalog freshness, complete maintained declaration descriptions, and acknowledged documentation state. `doccheck catalog update` changes only the generated project-catalog region. `doccheck state update` changes only the generated hash region and is run after affected documents have been reviewed.

Pipeline execution holds an exclusive operating-system lock on the metadata file, released automatically on process exit. SIGINT and SIGTERM cancel provider requests and finalize the attempt as failed. `analysis recover --db <path> --run-id <id>` acquires the same lock and atomically marks an abandoned running attempt failed with operator audit evidence; it refuses active pipeline ownership and does not migrate the database. Paired schema preparation uses a separate shared or exclusive maintenance lock beside the metadata file. Viewers and pipeline attempts hold shared ownership, so multiple current-schema owners may coexist while an upgrade requires exclusive ownership; the pipeline-attempt lock remains independent. Pipeline execution and recovery support Linux, macOS, and BSD systems with `flock`.
