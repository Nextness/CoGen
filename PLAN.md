# Epic PDF-01: Manage the PDF lifecycle through the research application

Status: The user reviewed the feature plan and requested its GitHub stories and tasks. This revision incorporates a repository-standards review and changes only PLAN.md. Individual approvals remain preserved; existing trial code and untracked test work are not treated as completed acceptance evidence by this review.

## Epic outcome, scope, and decisions

Users will upload, replace, download, export, and remove PDFs through the application. The frontend supplies files and export selections and presents results; the HTTP backend validates requests, owns storage transactions, deletes unused bytes, prepares downloads, and records audit evidence. Metadata and PDF storage remain separate SQLite databases.

This is the first step toward moving operator workflows into the application. This phase retires both direct-database PDF tools, `pdf-store` and `pdf-export`, and replaces their user workflows with frontend controls and backend APIs. Every user-facing download must originate in the frontend and receive its data from the backend; read-only database access is not an exception to that boundary. Other export types, including complete sanitized OSF bundles, require separate frontend integration in later plans. Pipeline execution, recovery, migrations, and operational backups remain separate administrative work in this phase.

The feature requirements and tracker structure have been reviewed. Remaining implementation follows the story gates below; this standards review does not execute them. The original investigation used repository source and existing tests without exercising uploads against a real corpus or running application tests. This review inspected current standards, file ownership, test selection, CI, and documentation tooling and ran the documentation consistency check.

### Confirmed requirements and decisions

- The application must provide PDF upload, replacement, and removal, with backend-owned persistence and audit logs for changes.
- PDF downloads and exports are also in scope. Users must not need a database-access command to download one PDF or export a selection of PDFs.
- Single-PDF and ZIP exports must offer an Include comments toggle. The user chose comments embedded inside the exported PDF as annotations; only saved application comments that match the exact source PDF may be included.
- Replacement must receive and validate the complete new upload before changing the current selection. Any failure before commit must preserve the old file and leave no partially applied replacement.
- The PDF database remains separate from the metadata database.
- Both databases should reach the latest schemas supported by the installed application before normal use. Routine upgrade recovery must move forward; users must not have to downgrade either database to match the other.
- The user explicitly chose permanent deletion of unused old bytes when replacing or removing a PDF. Retaining old file versions for restoration is therefore outside this design.
- The user allows exactly `github.com/pdfcpu/pdfcpu v0.15.0` during validation. Keep it as a project dependency if the agreed tests demonstrate the required behavior; if it fails, revise this plan instead of silently substituting another release/library or upgrading Go.
- The current task is plan validation only; source, tests, configuration, migrations, generated documentation, and user databases remain unchanged during this review.

### Recommended behavior included in approval

- APPROVED: Manage one current PDF per normalized DOI, using the existing stable work identity. Selection remains shared across all runs containing that work; it does not become a run-specific attachment.
- APPROVED: Put management controls on article detail, inside the existing Full-text PDF panel. A completed active run and exact normalized article membership are required to initiate changes, but initializing a review context is unnecessary.
- APPROVED: Retain notes, anchors, decisions, audit events, and document registration when removing a file. Old anchors remain historical evidence and cannot be drawn on different content.
- APPROVED: Use the existing 20,000,000-byte size limit and PDF signature/hash validation. No new runtime dependency is needed.
- APPROVED: Provide individual PDF downloads from article detail and a bounded ZIP export of selected articles from Corpus. Use safe readable filenames and report per-file failures explicitly.
- APPROVED: Keep database preparation and migration in an explicit backend administrative command. Serving and ordinary HTTP requests do not create, migrate, or repair databases.

The approved no-new-runtime-dependency note applies to the base storage/plain-export workflow. The user's later instruction explicitly permits the pdfcpu v0.15.0 trial for embedded comments. Go remains at the project's declared 1.25 toolchain. Passing the trial permits retaining that dependency; a failed trial requires a revised plan for review. No broader dependency or toolchain change is authorized by that exception.

### Investigation completed

- [x] Review repository architecture, storage contracts, frontend conventions, and documentation rules.
- [x] Trace PDF registration, manual insertion, serving, audit delivery, and anchor creation.
- [x] Inspect affected integration, frontend, browser, contract, and end-to-end test paths.
- [x] Resolve whether replaced and removed bytes should be retained.
- [x] Define the proposed implementation, migration, failure behavior, verification, and rollout.
- [x] Obtain user review of the plan and create the epic's story/task tracking.
- [x] Validate the plan against current repository standards and record execution constraints and corrections.

## Repository findings

| Area | Confirmed current behavior | Required change |
|---|---|---|
| Operator entry point | [tooling/go/pdf-store/main.go](tooling/go/pdf-store/main.go) accepts `add --db --doi --file`, opens writable databases, can apply migrations, and flushes PDF audit events. | Replace the operator workflow with frontend controls and HTTP mutations; remove this command after its callers move. |
| Export entry point | [tooling/go/pdf-export/main.go](tooling/go/pdf-export/main.go) opens both databases read-only, accepts repeated DOI selectors, validates stored bytes, writes title-based filenames, and preserves successful exports when other selectors fail. | Move useful selection, validation, naming, and partial-result behavior into frontend-initiated backend downloads; retire the executable and its direct database-opening workflow. |
| Backend storage | [backend/pdfstore/store.go](backend/pdfstore/store.go) registers normalized DOIs and implements `Add`. An available selection is left unchanged even if different bytes are supplied. | Implement explicit conditional save and removal operations instead of immutable-selection behavior. |
| Validation | [backend/pdfstore/validation.go](backend/pdfstore/validation.go) enforces size, the leading `%PDF-` signature, and SHA-256 identity. | Reuse it behind bounded HTTP ingestion; do not treat the file extension or browser MIME value as proof of validity. |
| Companion schema | [V00002_normalized_inventory.sql](migrations/corpus.pdf/V00002_normalized_inventory.sql) supports `not_available` and `available`, with a mutable document-to-blob pointer. | Preserve these states; add a monotonic document change counter for concurrency. Replacement does not require updating blob contents. |
| Legacy evidence | [V00001_pdf_store_baseline.sql](migrations/corpus.pdf/V00001_pdf_store_baseline.sql) gives append-only `pdf_download_attempts.content_hash` a foreign key to blobs. | Preserve historical attempt hashes while removing this blob-retention constraint through a new migration. Otherwise some permanent deletions fail. |
| Server opening | [backend/server/server.go](backend/server/server.go) opens metadata through separate read and write connections, but the companion only through a read-only connection. | Add a narrowly owned existing-only PDF writer while retaining protected read connections. Verify the writable companion schema before allowing changes. |
| HTTP API | [backend/server/pdf.go](backend/server/pdf.go) provides status and GET/HEAD delivery, including ranges, hash validation, ETags, and one cached document. | Add save/removal endpoints, mutable capabilities, concurrency errors, and coherent reads during changes. |
| Transport guards | [backend/server/reviews.go](backend/server/reviews.go) couples Origin checks to JSON decoding with a 512 KiB body limit; the method wrapper rejects DELETE. | Share applicable guards with binary uploads, give PDFs their own bounds, and permit DELETE only on the new route. |
| Audit | [backend/pdfstore/audit.go](backend/pdfstore/audit.go) writes an outbox in the PDF transaction; [backend/database/pdfbinding/delivery.go](backend/database/pdfbinding/delivery.go) mirrors events idempotently to metadata. Delivery currently depends on pipeline/tool execution. | Preserve the protocol, add replacement/removal evidence and bounded before/after values, and provide delivery/retry during server operation. |
| Frontend | [frontend/src/views/detail.tsx](frontend/src/views/detail.tsx) renders status and a generic `work-ID.pdf` download; [frontend/src/api.tsx](frontend/src/api.tsx) sends JSON mutation bodies and decodes JSON responses only. | Add accessible upload/download/export workflows, binary bodies and responses, safe readable filenames, and shared structured error handling. |
| Anchors and reader | [frontend/src/components/review-panel.tsx](frontend/src/components/review-panel.tsx) already filters anchors by hash, but anchor creation sends geometry without the reader's PDF hash. The server checks availability before a separate metadata transaction. | Carry the reader's hash and close the check/commit race now that PDF selection can change. Refresh the reader without discarding review drafts. |
| Exportable comments | [backend/database/review/types.go](backend/database/review/types.go) stores notes as run-context version heads and PDF anchors as exact content-hash-bound geometry. Notes can reference anchors, but general notes and page-only links have no independent PDF hash binding. The current checkout now has an isolated [annotation trial boundary](backend/pdfannotate/pdfannotate.go) and the exact dependency pin; comment export is not integrated. | Validate and extend the existing trial without duplicating it; add the toggle, matching saved-evidence selection, bounded worker, and derived export output. |
| Database preparation | [backend/cli/main.go](backend/cli/main.go) currently migrates metadata only. The pipeline's [pdfsync.Sync](backend/workspace/pdfsync/stage.go) prepares and reconciles companion inventory. | Provide an explicit way to prepare the companion migration and registrations without rerunning a research workspace. |
| Bundle readiness | [config/database.something](config/database.something) selects two independent chains, and [internal/migrations/runner.go](internal/migrations/runner.go) commits migrations independently per store. Pipeline PDF preparation currently occurs after attempt processing or completed-plan reuse. | Define one required schema pair, prepare both before a research attempt, and reject normal serving of a partial/unsupported pair. |
| Tests and builds | [tests/support/go/e2eflow/flow.go](tests/support/go/e2eflow/flow.go) and the [Makefile](Makefile) require the PDF mutation executable. Browser mutation isolation recognizes a fixed set of spec names. | Remove the executable dependency and exercise PDF input through the application; register any new mutation spec in both browser runners. |

The existing content-addressed storage and outbox are suitable foundations. Moving the CLI handler into HTTP unchanged would retain silent replacement no-ops, migration side effects, incomplete deletion behavior, and assumptions that available PDFs never change.

## Repository standards and execution constraints

The governing references are [AGENTS.md](AGENTS.md), [STANDARDS.md](docs/STANDARDS.md), [ARCHITECTURE.md](docs/ARCHITECTURE.md), [DATABASE.md](docs/DATABASE.md), [PROJECT-USAGE.md](docs/PROJECT-USAGE.md), the [Makefile](Makefile), and [PROJECT_CATALOG.md](docs/PROJECT_CATALOG.md). Frontend work additionally follows [FRONTEND-CODE-STYLE-GUIDE.md](docs/FRONTEND-CODE-STYLE-GUIDE.md), [JSX-RUNTIME.md](docs/JSX-RUNTIME.md), [DESIGN.md](docs/DESIGN.md), [CSS-REFERENCE.md](docs/CSS-REFERENCE.md), and [APP-USAGE.md](docs/APP-USAGE.md). These rules apply from the first implementation task, including the dependency trial and test helpers.

### Languages, ownership, and source layout

All newly authored frontend, Node tooling, and Node/browser test code must be TypeScript. Use `.ts` for modules without JSX and `.tsx` for JSX components; preserve the existing filenames of edited modules. Do not add handwritten `.js`, `.mjs`, or `.cjs` modules, inline JavaScript programs passed to Node, or a parallel JavaScript implementation for a trial/helper. PDF.js is the existing library's name; generated browser JavaScript and pinned vendor assets remain build outputs, not authoring formats. Some existing documentation/catalog headings use the term JavaScript for TypeScript declarations; this terminology does not authorize JavaScript source or require an unrelated catalog-generator rewrite.

| Area | Required owner and shape | Execution constraint |
|---|---|---|
| Backend application behavior | Go under the owning `backend/` package; process entry in `backend/cmd/analysis/main.go`, importable dispatch in `backend/cli/`. | The annotation worker is a production backend mode, not a new database-access tool or test hook. Keep pdfcpu types/imports inside `backend/pdfannotate/`. |
| Bundle coordination | Focused Go consumer under `backend/bundle/`, composed by CLI, workspace, and server. | Explicitly classify it as a consumer in the import checker before database imports are added. It must not import `backend/workspace/`; readiness used by the server must have no transitive pipeline orchestration dependency. |
| Persistence | Handwritten APIs and transactions in `backend/database/<family>/` or `backend/pdfstore/`; named SQL and declarations in that owner's `schema/`. | Generated `internal/sql/` types remain private. Keep migrations under the matching `migrations/` directory and SOMETHING chain; do not put family SQL in handlers or the bundle coordinator. |
| Frontend transport and payloads | Extend `frontend/src/api.tsx`; request/response interfaces belong in `frontend/src/api/types.ts`. | Use explicit `.ts`/`.tsx` imports and `import type`, typed exports, camelCase bindings, unchanged snake_case wire keys, and accurate `null`/optional fields. Add no `any`, framework, alternate transport stack, or new npm dependency. |
| Frontend rendering and interaction | Views under `frontend/src/views/`, reusable components under `frontend/src/components/`, project-owned classic JSX runtime. | Components do not import views. Return `JSX.Element`, type props, derive data/classes before markup, use `cx` for compound classes, rely on JSX escaping, preserve `link()` context, and follow render-then-bind/abort ownership. Do not add `raw()`, HTML-string rendering, or a new router/context cycle. |
| Maintained developer tooling | TypeScript under `tooling/node/`; importable Go tools under `tooling/go/` with thin `tooling/go/cmd/` entries only when an actual tool is needed. | This epic adds no replacement operator PDF executable. Test-only PDF inspection belongs in test support rather than developer or product tooling. |
| Shared test construction | Go helpers under `tests/support/go/`, TypeScript helpers under `tests/support/node/`, fixtures owned by the corresponding test component. | Reuse `testenv.Root`/`testenv.Path` for Go input/root resolution instead of counting parent directories. Production packages never import test support or contain fixtures, suites, test-only hooks, or source copies. |
| Test executable and report output | Unique target-owned paths under `build/`; temporary input/database files remain private to their tests. | Build the actual backend with `make build BIN=build/pdf-trial/<unique-run>/analysis` from the repository root; do not use a stale shared binary, bare `go build`, an unrelated ignored output directory, or a user corpus. Clean only evidence owned by the current test. |

### Test layers and independent PDF inspection

Ordinary suites follow `tests/<domain>/<component>/{unit,functional,integration}/`. Go suites use external `*_test` packages and `*_test.go`; Node unit/functional/integration suites use `.test.ts`, `node:test`, and `node:assert`; browser suites use Playwright `.spec.ts`. Unit cases isolate deterministic validation, mapping, and rendering without a database, HTTP service, browser, or child process. Functional cases cover complete in-process behavior; integration cases own SQLite, HTTP, subprocess, PDF.js, and worker boundaries. E2E remains under `tests/e2e/offline/` with the existing build-tag and opt-in rules.

Keep ordinary Go, race, and coverage suites Go-only. Place the independent writer/PDF.js output checks in `tests/contracts/pdf/integration/pdf-annotations.test.ts`, with reusable typed inspection in `tests/support/node/pdf-inspector.ts` if needed; the cross-language contract requires Go plus the locked Node/PDF.js environment. The suite builds/invokes the actual annotation worker, owns its fixture inputs and outputs, and validates annotations, geometry, searchable text, and source immutability independently. Reuse the deterministic Go PDF fixture definitions where applicable and record provenance for additional test-owned fixture files; do not copy production source into the harness or depend on artifacts left by a previous test command. Go integration tests cover the real worker and cancellation without launching Node. Browser output checks remain in the PDF management/review suites.

Wire the new TypeScript contract suite into the existing Node-capable frontend CI job in [.github/workflows/ci.yml](.github/workflows/ci.yml) with `make test DOMAIN=contracts COMPONENT=pdf LAYER=integration`; ensure the required Go toolchain has already been set up there. `make check-frontend` must type-check it and its support through [tests/tsconfig.json](tests/tsconfig.json). Missing Node, locked PDF.js, fonts, or browser binaries must produce an explicit missing-prerequisite result for the relevant gate, not a skipped check reported as a passing pdfcpu trial. Do not add Node/npm/browser requirements to the Go-only backend, race, coverage, or documentation jobs.

### Documentation and generated files

Every new or changed maintained Go declaration needs an attached Go documentation comment beginning with its symbol name; TypeScript declarations need adjacent JSDoc and anonymous test cases need descriptive titles. Keep each prose paragraph, bullet, numbered item, and table row on one physical Markdown line. Planning artifacts describe proposed work; maintained product documents under `docs/` describe supported behavior only after the corresponding implementation and verification exist. Update affected documentation with each story, then reconcile the complete set in S09 instead of postponing every documentation change to the last phase.

Regenerate SQL only from owning `schema/` inputs plus `config/sqlc.yaml`, CSS classes only from owning stylesheets, and the source catalog only from maintained declarations/comments. Review their generated diffs. Review each changed `docs/` file and the dependents in [DOC-STATE.md](docs/DOC-STATE.md) before `make docs-state-update`, then run `make check-docs`. A root PLAN.md-only revision needs Markdown/link validation and `make check-docs`; it does not justify regenerating the catalog or acknowledging unrelated changed documentation. Do not mark a check complete merely because it was listed in this plan.

### Standards review findings and disposition

| Finding confirmed by inspection | Required correction in this plan | Verification during execution |
|---|---|---|
| The trial specified PDF.js checks without fixing their language, runner, or CI ownership. Go-only jobs do not install locked npm dependencies. | Require TypeScript shared inspection and a separately selected PDF contract integration suite; keep Node out of Go-only suites. | Run the Go-only annotation suites separately from the TypeScript contract, type-check both Node scopes, and prove the contract runs in the Node-capable CI job. |
| [packageGraph.layerOf](tooling/go/importcheck/main.go) currently classifies an unrecognized `backend/bundle/` path as shared low-level code, which cannot import databases or consumers. | Add only the concrete bundle subtree to consumer classification in S02; preserve restrictions on family/root/companion imports and avoid a bundle-to-workspace dependency. | Extend [import graph unit cases](tests/tooling/go/importcheck/unit/main_unit_test.go), run the import-checker suites, and run `make check-imports`. |
| `make test-frontend-all` hardcodes the frontend viewer scope, while `api-shapes.spec.ts` belongs to contracts/http. | Select that suite through `make test DOMAIN=contracts COMPONENT=http LAYER=integration BROWSER=all`. | The selector must resolve the contract spec within its owning domain; then run the isolated Chromium/Firefox contract suites during implementation. |
| The expected-change list did not consistently distinguish new, required, conditional, generated, and retired files. | Make these states explicit below, keep harnesses in `tests/support/`, and attach generators to generated paths. | Review the file inventory against the final diff, without speculative files or direct generated edits. |
| Before this PLAN.md edit, `make check-docs` failed because `docs/PROJECT_CATALOG.md` was stale. Existing untracked annotation integration/support files were also present. | Preserve that work and report the baseline failure; do not regenerate unrelated catalog/state files during plan validation or mark their implementation accepted. | Recheck documentation and validate this plan's links, anchors, formatting, story IDs, and task IDs independently; disclose any remaining preexisting catalog failure. |

Review validation confirmed nine stories, 38 unique task IDs, and all six original approval notes preserved. Repository-local links and internal anchors resolve, referenced Make targets exist, and the diff whitespace check passes. The full documentation check reports the same stale source catalog before and after this review; catalog regeneration and acknowledgement of unrelated work remain outside this task. These results validate the planning artifact, not unimplemented application behavior or the pdfcpu acceptance gate.

## Story map and execution rules

Each story is one implementation phase within epic PDF-01. Task IDs identify executable work; the shared contracts later in this document supply the common API, storage, audit, limits, and failure rules. The examples below describe planned interfaces or pseudocode, not code already added to the project. Every task remains pending until implementation is approved and its evidence is recorded.

| Story / phase | Target | Dependencies | Exit evidence |
|---|---|---|---|
| [PDF-S01](#pdf-s01-pdfcpu-trial) | Validate the allowed pdfcpu v0.15.0 dependency on Go 1.25. | Execution-plan approval. | Recorded pass/keep or fail/revise decision, generated PDF fixtures, compatibility and cancellation results. |
| [PDF-S02](#pdf-s02-schema-and-bundle-readiness) | Add the companion migration and coordinate both required schema targets. | S01 passes; approved migration entry-point policy remains in force. | Upgrade/retry/readiness tests pass with metadata V00028 and PDF V00003. |
| [PDF-S03](#pdf-s03-atomic-storage-and-audit) | Provide atomic save/remove operations, safe byte deletion, and durable audit. | S02. | Transaction, deduplication, stale-counter, and outbox replay tests pass. |
| [PDF-S04](#pdf-s04-http-mutation-api) | Expose bounded, guarded PDF upload/replacement/removal APIs. | S03. | Public HTTP contracts and incomplete-upload failure tests pass. |
| [PDF-S05](#pdf-s05-reader-and-review-consistency) | Keep PDF readers, caches, and anchors correct while files change. | S04. | Range/cache and cross-process anchor-race tests pass. |
| [PDF-S06](#pdf-s06-frontend-pdf-management) | Make upload, replace, and remove usable from article detail. | S05. | Accessible UI workflow, draft preservation, and focused browser checks pass. |
| [PDF-S07](#pdf-s07-frontend-downloads-and-zip-exports) | Replace command-based PDF export with frontend single/ZIP downloads. | S06. | Download bytes, safe names, ZIP partial results, and export audits verified. |
| [PDF-S08](#pdf-s08-embedded-comment-exports) | Add optional matching comments inside exported PDF copies. | S01, S05, and S07. | Original bytes unchanged; comment selection, placement, limits, and independent PDF.js checks pass. |
| [PDF-S09](#pdf-s09-tool-retirement-and-epic-verification) | Retire both PDF tools and complete documentation and system verification. | S01-S08. | Build/CI callers migrated; full required verification and final diff review complete. |

Do not mark a story complete solely because its internal code exists. Its public integration, failure handling, focused tests, and documentation impact must be reviewable. Record commands and actual outcomes beside the tasks during execution. Preserve unrelated user edits and the existing approval notes.

The trial is an explicit decision gate. If v0.15.0 passes the criteria below, retain it as authorized and proceed. If it fails materially, retain the evidence, revise the affected stories/contracts, and obtain review of the revised approach before substituting another dependency or changing Go. Do not ship a fake Include comments success or silently omit requested comments to pass the gate.

Use `change_counter` for the PDF document concurrency field throughout this epic. It stores only the current change sequence and is independent of immutable review/note/anchor versions; it does not preserve old PDF files.

## Stories and technical tasks

### PDF-S01: pdfcpu trial

**Story:** As a researcher, I need confidence that the chosen PDF writer can export readable comments without damaging the document or changing stored data. **Technical target:** a small annotation component using exactly pdfcpu v0.15.0, with real Go 1.25 execution and a proven bounded processing boundary. **Dependencies:** approval to execute this plan; the dependency itself is already allowed for this trial.

**References:** [Go and dependency standards](docs/STANDARDS.md), [supported command interface](Makefile), [upstream v0.15.0 module](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/go.mod), [annotation API](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/pkg/api/annotation.go), and [license](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/LICENSE.txt).

- [ ] **PDF-S01-T01: Pin and isolate the trial dependency.** Verify the current exact `github.com/pdfcpu/pdfcpu@v0.15.0` pin and existing `backend/pdfannotate/` boundary before making further changes; use Go module tooling only if the allowed pin needs correction. Review go.mod/go.sum and the transitive diff without unrelated upgrades or removing other work's dependencies. Keep Go 1.25 and application-owned annotation types; the component must not open databases or expose pdfcpu models across its handwritten API. Document new declarations and required license notices, and accept this task only on recorded verification rather than the presence of files.
- [ ] **PDF-S01-T02: Prove annotation correctness on controlled fixtures.** Keep deterministic Go validation/mapping tests under `tests/backend/pdfannotate/unit/` and real writer/worker integration under `tests/backend/pdfannotate/integration/`; reuse [PDF fixtures](tests/support/go/pdffixture/pdf.go). Cover text/highlight comments, full Unicode bodies, multiple rectangles, crop origins, rotation, user units, existing annotations, searchable text, and source byte immutability. Add independent TypeScript checks in `tests/contracts/pdf/integration/pdf-annotations.test.ts` with typed shared inspection under `tests/support/node/pdf-inspector.ts` when needed; follow the [test ownership contract](#test-layers-and-independent-pdf-inspection). This suite builds/invokes the actual worker, owns its inputs/outputs, and runs explicitly in the Node-capable CI job. Do not launch Node from Go-only tests, accept skipped PDF.js verification as a passing trial, or rely only on a writer reading its own output. Inspect overlapping untracked tests and integrate with existing work without overwriting or discarding it; this review does not modify them.
- [x] **PDF-S01-T03: Prove cancellation and resource containment.** The v0.15.0 API has no request context parameter. Exercise the writer in a killable backend-owned worker process with bounded input/output, finite processing time, and measured/enforceable resource limits. Put process entry in `backend/cmd/analysis/main.go`, importable worker dispatch/protocol handling in `backend/cli/`, and annotation behavior in `backend/pdfannotate/`; its internal mode receives only PDF bytes and bounded annotation data, never database paths, handles, or an operator file interface. Go integration tests build the real executable from the repository root through `make build BIN=build/pdf-trial/<unique-run>/analysis`, use `testenv.Root`/`testenv.Path` for root/input resolution, and own cleanup of that unique output. Prove timeout/cancellation terminates and reaps the child and cleans temporary output. Do not use a stale shared binary, production test hooks, or a timed-out goroutine that continues processing.
- [ ] **PDF-S01-T04: Record the dependency decision.** Run the trial with an actual Go 1.25 toolchain and automatic toolchain switching disabled, inspect parser/dependency advisories and configuration/font side effects, and record output samples, supported/unsupported cases, time/memory observations, and pass/fail results in target-owned evidence under `build/`. Pass means the required fixtures, containment, and compatibility behavior work without an unapproved Go/runtime change. Keep v0.15.0 on pass; on failure, revise this plan and describe the concrete incompatibility instead of choosing another library silently.

For T01, the following are future execution commands, not commands run during this review. The allowed dependency is already pinned in the current checkout; run `go get` only if that pin needs correction after inspecting the worktree:

```sh
go get github.com/pdfcpu/pdfcpu@v0.15.0
GOTOOLCHAIN=local make test-go DOMAIN=backend COMPONENT=pdfannotate
GOTOOLCHAIN=local make test DOMAIN=contracts COMPONENT=pdf LAYER=integration
```

For T02, this illustrates the upstream stream-based boundary inside the isolated trial; the caller supplies a valid fixture annotation and controlled configuration. The handwritten application API should map its own types to these library types internally.

```go
// trialAnnotate exercises the upstream stream API on one controlled fixture.
func trialAnnotate(source []byte, annotation model.AnnotationRenderer, conf *model.Configuration) ([]byte, error) {
	var output bytes.Buffer
	if err := api.AddAnnotations(bytes.NewReader(source), &output, []string{"1"}, annotation, conf); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
```

**Acceptance:** The trial pin is exact; tests actually run with Go 1.25; the Go-only worker checks and independently selected TypeScript/PDF.js contract both pass; valid fixtures preserve page content and existing annotations; malformed/unsupported input fails safely; cancelled work stops; source bytes never change; the keep/revise decision is documented. A declaration in upstream go.mod alone does not count as project compatibility evidence.

### PDF-S02: Schema and bundle readiness

**Story:** As an operator, I need one preparation operation to bring both databases to the schemas expected by this application, with forward recovery after a partial upgrade. **Technical target:** metadata V00028 / PDF V00003, full-history verification, and safe maintenance ownership. **Dependencies:** S01 trial accepted; no change to the approved explicit-migration policy.

**References:** [database contract](docs/DATABASE.md), [migration registry](config/database.something), [migration runner](internal/migrations/runner.go), [registry resolver](backend/database/migration_config.go), and [shared migration contract](#shared-contract-persistence-and-migrations).

- [ ] **PDF-S02-T01: Define the supported schema pair.** Derive configured endpoints from the two existing chains and compare them with the pair supported by a focused `backend/bundle/` consumer. First extend `packageGraph.layerOf` in `tooling/go/importcheck/main.go` to classify only that concrete subtree as a consumer; add cases to `TestImportRulesThroughSourceTrees` and retain rejections of family/root/companion dependencies on consumers. The bundle must not import workspace or server orchestration. Verify complete required history and supersedes aliases; reject unknown/newer history and mismatched code/config. Do not introduce historical SyncN declarations, a third database, another migration engine, or a broad shared-layer exception.
- [ ] **PDF-S02-T02: Add companion V00003.** Add `migrations/corpus.pdf/V00003_pdf_document_management.sql` and append it to [the PDF chain](config/database.corpus.pdf.something). Add a positive `change_counter` to document registrations, nullable validated before/after JSON to the outbox, and the content-hash index. Protect registration identity and exactly-once counter advancement. Rebuild legacy download attempts only to remove their blob-retention foreign key, retaining values, IDs, other constraints, indexes, triggers, and sequence behavior. Keep blob-update protection and all applied migration files unchanged.
- [ ] **PDF-S02-T03: Coordinate maintenance and forward retry.** Compose per-store migrators under one maintenance ownership protocol that excludes active serving/pipeline operations while allowing multiple already-current viewers. Validate migration inputs/bindings before changing data; apply metadata then PDF; verify both targets afterward. Retain committed progress on failure and resume forward. Keep the existing exclusive pipeline-attempt ownership separate from maintenance ownership, and preserve cleanup on cancellation and startup failure.
- [ ] **PDF-S02-T04: Wire application entry points.** Extend `migrateMain` to prepare both stores and normalized registrations without a research attempt. Extract the existing `pdfsync.Sync` inventory coordination into the bundle consumer where needed, leaving the workspace stage as a caller; never make the bundle import `backend/workspace/pdfsync` or make metadata/companion owners import the bundle. Prepare both schemas in `RunPipelineContext` before `StartWorkspaceAttemptContext` or reuse; keep `serveMain`/`server.Open` as readiness checks without migrations. Initialize an unbound companion only through preparation; reject missing bound files and path escapes instead of recreating lost data. Add actual/target schema and readiness information to health.
- [ ] **PDF-S02-T05: Regenerate and prove upgrades.** Update owning SQL declarations and queries, run `make sqlc-generate`, review the generated diff, and run `make sqlc-check`. Add production-chain integration tests for existing populated databases, current pairs, supersedes aliases, missing/future steps, a failed second-store upgrade after first-store commit, and successful forward retry. Verify maintenance changes create no research attempt and no user rollback task.

For T01, the derived target is conceptually the following; it is not an additional SOMETHING configuration block to maintain:

```json
{
  "metadata": "V00028_immutable_cache_responses.sql",
  "pdf": "V00003_pdf_document_management.sql"
}
```

**Acceptance:** Both schemas must be current before normal use; a partial upgrade is blocked and resumable; no automatic downgrade exists; current-schema read-only PDF evidence remains readable; family constructors stay side-effect-free; import ownership is explicit without a server-to-workspace dependency. **Verification:** `make test-go DOMAIN=backend COMPONENT=bundle`, `make test-go DOMAIN=tooling COMPONENT=go/importcheck`, `make check-imports`, relevant database/PDF/CLI/workspace integration tests, and `make sqlc-check`.

### PDF-S03: Atomic storage and audit

**Story:** As a researcher, I need replacement/removal to apply completely or leave the existing PDF untouched, with durable audit evidence. **Technical target:** owning-store transactions, a current-state change counter, safe shared-blob deletion, and idempotent audit delivery. **Dependencies:** S02.

**References:** [PDF store](backend/pdfstore/store.go), [validation](backend/pdfstore/validation.go), [outbox](backend/pdfstore/audit.go), [metadata delivery](backend/database/pdfbinding/delivery.go), [shared audit contract](#shared-contract-audit), and [transaction standards](docs/STANDARDS.md).

- [ ] **PDF-S03-T01: Implement conditional save and remove.** Replace the add-only contract with typed save/removal results and errors. Save requires complete validated bytes and an expected `change_counter`; unavailable registrations become available, different bytes replace the selection, and identical bytes are a verified no-op. Remove clears selection/timestamp and advances the counter while retaining registration; repeated removal with the current counter is a no-op. No history of old PDF files is introduced.
- [ ] **PDF-S03-T02: Keep all storage effects in one transaction.** Under an immediate companion transaction, validate expected state, insert/reuse a verified blob, move the selection with compare-and-swap, delete outgoing bytes only when unreferenced by every current document, and append the outbox event. Every pre-commit failure rolls back all these effects. Removal and different-byte repair must not require reading a corrupt outgoing blob; deduplicated incoming bytes must pass full size/hash validation.
- [ ] **PDF-S03-T03: Extend audit vocabulary and delivery.** Add replacement/removal actions and before/after operational data to the outbox and metadata delivery types/queries. Keep manual PDF events globally work-scoped, with initiating run/revision context and `actor: user`. Keep bodies, quotations, filenames, paths, and email out of audit. Add a bounded relay API for server use; retain atomic metadata event/link insertion and retry-safe delivery marking across interruption and concurrent relays.
- [ ] **PDF-S03-T04: Prove unchanged state on failures.** Extend [store integration tests](tests/backend/pdfstore/integration/store_integration_test.go) and PDF-binding integration coverage. Inject failures at blob insert, pointer update, outgoing deletion, outbox insert, and commit using test-owned SQLite fixtures/constraints. Assert old hash/bytes/timestamp/counter and replacement-event count are unchanged and no speculative blob survives. Cover shared bytes, stale/ABA counters, pipeline re-registration, crash/restart replay, and metadata-mirror failure after a successful PDF commit.

For T02, this illustrates the conditional SQL in the owning named-query source, not a standalone transaction or handler SQL:

```sql
UPDATE pdf_documents
SET status = 'available',
    content_hash = sqlc.arg(new_hash),
    inventoried_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    change_counter = change_counter + 1
WHERE doi = sqlc.arg(doi)
  AND change_counter = sqlc.arg(expected_counter);

DELETE FROM pdf_blobs
WHERE content_hash = sqlc.arg(outgoing_hash)
  AND NOT EXISTS (
      SELECT 1 FROM pdf_documents
      WHERE content_hash = sqlc.arg(outgoing_hash)
  );
```

**Acceptance:** A failed replacement never loses the old file; concurrent changes cannot both win the same counter; no-op operations do not change timestamps/counters/audit; unreferenced outgoing bytes are deleted; audit replay never duplicates an event. **Verification:** focused PDF-store and PDF-binding suites, SQL generation checks, then the required race suite after integration.

### PDF-S04: HTTP mutation API

**Story:** As a frontend client, I need bounded upload, replacement, and removal endpoints with clear capabilities and errors. **Technical target:** validated HTTP input connected to S03 without database or filesystem paths supplied by the browser. **Dependencies:** S03.

**References:** [server lifecycle and routing](backend/server/server.go), [PDF delivery](backend/server/pdf.go), [existing mutation decoding](backend/server/reviews.go), [health](backend/server/overview.go), and [shared HTTP contract](#shared-contract-http).

- [ ] **PDF-S04-T01: Open only the required connections.** Retain read-only metadata/PDF pools, add the single existing-only PDF mutation connection with immediate transactions, busy timeout, foreign keys, secure deletion, and the statement-budget driver. Verify S02 readiness and protection triggers before capability reporting. Keep opening/handlers migration-free, close every resource on partial failure, and start/stop audit relay ownership explicitly from serving lifecycle rather than `server.Open`.
- [ ] **PDF-S04-T02: Add guarded binary save and empty-body DELETE.** Register the exact run/article PDF resource, permit DELETE only there, and factor applicable Origin/query checks out of JSON-only decoding. Resolve normalized work/DOI from exact selected-run revision membership; require a completed active run for mutation without requiring review-context initialization. Enforce media type, `If-Match`, declared size/digest, actual complete body, and storage preconditions before committing.
- [ ] **PDF-S04-T03: Bound body, deadline, and response behavior.** Allow at most two retained upload bodies, cap each at 20,000,000 bytes including chunked transfers, and validate the digest against the fully received bytes. Use finite upload-specific deadlines before the buffering wrapper and the normal query timeout after ingestion. Map statuses/errors as specified below, use no-store responses, and keep arbitrary audit backlog work out of request query budgets. Treat lost responses after commit as uncertain outcomes requiring reconciliation.
- [ ] **PDF-S04-T04: Test the public boundary.** Extend server integration and [HTTP shape tests](tests/contracts/http/integration/api-shapes.spec.ts). Use real HTTPServer tests for Host/Origin, method/media/precondition rejection, truncated uploads after a valid signature, size/digest mismatch, timeout/cancellation, capacity release, schema capabilities, and pool/worker cleanup. Prove the old PDF remains readable during a replacement upload and after every rejection.

For T02, the planned save request looks like this; the values are illustrative and the digest must match the actual body:

```http
PUT /api/runs/12/articles/57/pdf HTTP/1.1
Content-Type: application/pdf
If-Match: "pdf-42-c7"
X-PDF-Size: 12345
X-PDF-SHA256: <64 lowercase hexadecimal characters>

<complete PDF bytes>
```

**Acceptance:** Only valid complete requests reach storage; stale saves/removals are rejected; interrupted uploads never create a replacement event or delete old bytes; the stable JSON envelope and capability fields match frontend contracts. **Verification:** `make test-go DOMAIN=backend COMPONENT=server` with focused new PDF cases first, followed by the isolated contract suite when the frontend fixture is updated.

### PDF-S05: Reader and review consistency

**Story:** As a reviewer, I need the document reader and saved anchors to remain trustworthy when another tab replaces or removes a PDF. **Technical target:** coherent reads, hash-pinned ranges, correct cache invalidation, and cross-process anchor guards. **Dependencies:** S04.

**References:** [PDF endpoint/cache](backend/server/pdf.go), [anchor mutation handlers](backend/server/reviews.go), [review component](frontend/src/components/review-panel.tsx), [reader](frontend/src/components/pdf-viewer.tsx), [fullscreen lifecycle](frontend/src/components/pdf-fullscreen.tsx), and [shared consistency contract](#shared-contract-reader-and-anchor-consistency).

- [ ] **PDF-S05-T01: Preserve correct reads and ranges.** Read selection/bytes coherently, invalidate affected local cache entries after commit, and recheck current selection before cache reuse in every server process. Add a validated content-hash pin to embedded-reader requests. Preserve GET/HEAD, conditional byte ETags, ranges, safe headers, and hash integrity checks; a changed pin returns a deliberate refresh/conflict result rather than mixed file segments.
- [ ] **PDF-S05-T02: Bind anchor writes to the displayed PDF.** Carry `pdf_content_hash` from the reader in creation/active-edit/restoration requests. Acquire a companion-owned write reservation before the metadata transaction and retain it until anchor commit, so PDF replacement/removal cannot interleave between validation and save. Keep one lock order: PDF reservation before metadata; audit delivery commits metadata before updating PDF delivery state. Do not use an in-process mutex as the only cross-process protection.
- [ ] **PDF-S05-T03: Preserve review evidence and editor input.** Let metadata anchor tombstoning work without available PDF bytes while creation/restoration require matching bytes. Refresh reader and anchor availability without replacing unsaved decision/note DOM state, losing drafts, or changing review heads. Tear down stale PDF.js documents/workers/fullscreen state and pending selections; clear only PDF page/focus state made invalid by the new content. Keep canonical research navigation context.
- [ ] **PDF-S05-T04: Prove races and lifecycle behavior.** Run replacement/removal against a second server connection while creating/restoring anchors and reading ranges. Assert no wrong-hash anchor commits or deadlocks. Extend reader/fullscreen/review tests for missing/mismatched content, page boundaries, stale-tab focus, teardown, draft preservation, and anchor removal after PDF removal.

**Acceptance:** The next request after a committed change observes the new selection/absence; stale reader requests cannot combine files; anchors always refer to the selected bytes they were created against; review history and drafts survive refresh. **Verification:** focused server/review integration, PDF viewer/fullscreen unit tests, and the review browser suite; race detection is mandatory for this story.

### PDF-S06: Frontend PDF management

**Story:** As a researcher, I can upload, replace, and remove PDFs from article detail without running a database-access tool. **Technical target:** usable controls in Full-text PDF, structured transport, explicit confirmation, and safe failure feedback. **Dependencies:** S05.

**References:** [PDFStatusPanel/detail lifecycle](frontend/src/views/detail.tsx), [API transport](frontend/src/api.tsx), [wire types](frontend/src/api/types.ts), [frontend style guide](docs/FRONTEND-CODE-STYLE-GUIDE.md), [JSX runtime](docs/JSX-RUNTIME.md), [design](docs/DESIGN.md), and [CSS ownership](docs/CSS-REFERENCE.md).

- [ ] **PDF-S06-T01: Extend typed transport and upload preparation.** Extend the existing TypeScript `frontend/src/api.tsx` for File/Blob bodies and conditional headers while sharing JSON error decoding with existing mutations. Put request, status, result, and capability interfaces in `frontend/src/api/types.ts`, imported with `import type`; add no JavaScript helper, untyped payload copy in a view, or `any`. Compute File.size and SHA-256 through browser APIs before upload and capture the current change counter/ETag. Preserve JSON callers, explicit import specifiers, nullable/optional wire semantics, and abort/navigation behavior; no source filename or client path enters backend storage/audit.
- [ ] **PDF-S06-T02: Add management controls and confirmation.** Extend PDFStatusPanel, with a focused `pdf-management.tsx` component if needed. Provide one labeled file input, explicit submit, selected size/name shown locally, capability reasons, and actions appropriate to current availability. Confirm replacement/removal with article identity, global work scope, and permanent unused-byte deletion. Keep file selection and other review input on recoverable failures; avoid duplicate submission and implicit retries with refreshed preconditions.
- [ ] **PDF-S06-T03: Integrate targeted refresh and accessibility.** Apply S05 reader/anchor refresh, reload article audit/current status after commit, distinguish saved/audit-pending/unchanged/error/uncertain outcomes, and preserve Corpus filters on return. Reuse existing JSX/control styles, accessible labels/live regions/focus, and narrow-screen layout. Modify the owning stylesheet only when needed, then regenerate CSS classes; do not edit the registry or vendor assets directly.
- [ ] **PDF-S06-T04: Add isolated UI verification.** Extend [API unit tests](tests/frontend/api/unit/api.test.ts), [detail functional tests](tests/frontend/views/detail/functional/detail.test.ts), and focused component tests. Create `tests/frontend/viewer/integration/pdf-management.spec.ts`, register it as a serial mutation spec in both browser configuration and Node runner, and verify upload/success/failure/replacement/removal/reload through visible controls on copied fixtures. Cover keyboard confirmation, stale tabs, and preserved note/decision drafts.

For T01, these are the conceptual request options after computing the complete-file digest; this is not a replacement implementation of the shared API helper:

```ts
const uploadOptions: RequestInit = {
  method: "PUT",
  headers: {
    "Content-Type": "application/pdf",
    "If-Match": pdfStatus.mutation_etag,
    "X-PDF-Size": String(file.size),
    "X-PDF-SHA256": fileDigest,
  },
  body: file,
};
```

**Acceptance:** Every PDF mutation is usable in article detail and persists across reload; errors preserve existing bytes and editor input; local read-only capabilities are explained accurately; both browser projects and relevant accessibility states work. **Verification:** frontend unit/functional, `make check-frontend`, backend server tests, and the focused serial PDF management browser suite.

### PDF-S07: Frontend downloads and ZIP exports

**Story:** As a researcher, I can download one PDF or a selected set through the frontend with readable names and clear partial results. **Technical target:** backend-prepared original-byte attachments, bounded ZIP streaming, and explicit export-preparation audit. **Dependencies:** S06; this story establishes original-byte export before S08 adds annotations.

**References:** [existing export behavior to migrate](tooling/go/pdf-export/main.go), [its tests](tests/tooling/go/pdf-export/integration/export_test.go), [batched inventory overlay](backend/server/evaluation.go), [audit family](backend/database/audit/audit.go), [Corpus view](frontend/src/views/corpus.tsx), and [shared export contract](#shared-contract-frontend-and-export).

- [ ] **PDF-S07-T01: Implement download/export resources.** Add the two POST export routes below with bounded JSON selectors and expected source hashes. Resolve exact selected-run article membership without imposing mutation-only active-run/context requirements. Single export returns a validated PDF attachment; a selection of at most 20 unique revisions returns a ZIP with a result manifest. Preserve valid files when other valid selectors have unavailable/changed/corrupt content; reject invalid outside-run selectors, all-failed output, and exceeded caps before headers.
- [ ] **PDF-S07-T02: Implement safe names, snapshots, and audit.** Reuse useful filename validation behavior, deriving names from selected revisions, bounding Unicode bytes, and using stable work-ID suffixes/fallbacks. Use archive/zip stored entries with pinned source snapshots, aggregate output/admission/deadline limits, and explicit binary response policies outside the JSON serialization cap. Write required per-work export-prepared audit events through one metadata-owned batch transaction before headers; do not write the PDF outbox or claim a file was saved to disk.
- [ ] **PDF-S07-T03: Add frontend download actions and selection.** Add a binary helper sharing structured JSON failure decoding, bounded body consumption, and object-URL cleanup. Keep Download PDF in article detail and add current-page Corpus selection with count/limit feedback and Download selected PDFs. Clear selection on context/page/filter changes; do not interpret a page selection as hidden results. Extend batched InventoryForDOIs/overlayPDFInventory with hash/size and wire types instead of adding one status fetch per row. The `include_comments` field is defined now; S08 enables the functional comments path.
- [ ] **PDF-S07-T04: Port export coverage to the application boundary.** Move CLI integrity/naming/deduplication/partial-result cases to server/frontend tests before tool retirement. Verify attachment headers, exact bytes, Unicode/colliding/missing titles, ZIP paths/manifest, count/byte/time/admission bounds, read-only companion support, required-audit failure, stream interruption, and replacement during export. Use Playwright download events to inspect actual files and preserve PDF storage/review state apart from intended metadata audit additions.

For T01, a planned batch request is:

```json
{
  "articles": [
    {"work_revision_id": 57, "content_hash": "<source SHA-256>"},
    {"work_revision_id": 61, "content_hash": "<source SHA-256>"}
  ],
  "include_comments": false
}
```

**Acceptance:** Both download paths work without pdf-export; original output is byte-for-byte source content; partial results are explicit; browser destinations replace backend output directories; no PDF version/selection changes occur. **Verification:** focused server exports, frontend transport/Corpus tests, isolated download browser cases, and API contract tests.

### PDF-S08: Embedded comment exports

**Story:** As a reviewer, I can choose whether an export includes matching saved comments inside the PDF. **Technical target:** Include comments on single/ZIP exports using validated pdfcpu v0.15.0, exact hash/context matching, and derived copies only. **Dependencies:** S01 accepted, S05 consistency guarantees, and S07 export plumbing.

**References:** [review types](backend/database/review/types.go), [note/anchor relationships](backend/database/review/relationships.go), [owning SQL](backend/database/review/schema/relationships.sql), [annotation examples](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/pkg/api/test/annotation_test.go), and [embedded-comment contract](#embedded-comments-matching-snapshots-and-output).

- [ ] **PDF-S08-T01: Fetch matching saved evidence.** Add owning `review/export.go` and `schema/export.sql` queries for bounded current active note/anchor heads in the selected context/work, including inherited heads. Filter anchors by the exact source hash, use full saved bodies rather than list previews, and exclude tombstones, drafts, other contexts/works, unbound/page-only notes, and notes mixing matching and mismatched/unresolved PDF references. Snapshot immutable IDs/content, report exclusions, and never initialize review or save a draft during export.
- [ ] **PDF-S08-T02: Integrate the bounded annotation worker.** Use the S01-validated component and internal backend worker mode with framed/capped byte-and-annotation input and capped output. The worker gets no database credentials, paths, handles, or HTTP listener. Kill/reap it on timeout/cancellation and apply the proven memory/processing controls. Keep pdfcpu imports and library-specific models inside the annotation component. Unsupported files or generation failures become explicit per-file outcomes, never an unannotated success pretending to include comments.
- [ ] **PDF-S08-T03: Map geometry and preserve document content.** Convert normalized unrotated rectangles using actual page crop/media origins, rotation, and user units; validate page existence. Add standard highlight/comment annotations with complete safe note text, labels/quotations, display attribution, and recorded time. Preserve searchable text, pages, and existing annotations; respect modification/signature restrictions. Deduplicate identities and enforce annotation/text/output caps without silently truncating saved prose.
- [ ] **PDF-S08-T04: Add toggle, exact output retention, and provenance.** Enable Include comments, default off, on both frontend download surfaces. Off returns untouched original bytes and performs no comment lookup; on derives a separate copy or clearly reports no matching comments. For ZIPs, prepare/validate each derivative once and retain exact output in bounded private temporary spools until emission; clean them on every exit. Audit and manifest use the actual source/output hashes, toggle, bounded immutable IDs and counts, never comment prose or email. Do not change PDF selection/counter/blobs or review heads.
- [ ] **PDF-S08-T05: Validate independently and decide retention.** Parse downloaded PDFs with pinned PDF.js and review rendered placement on representative rotated/cropped/Unicode fixtures. Test original annotations, inherited comments, excluded evidence, output growth, parser/writer/temporary-disk failure, cancellation cleanup, signature/permission failure, and concurrent comment/PDF changes. Verify original storage hashes and review heads remain unchanged and repeated exports do not accumulate annotations. Confirm the trial passes end to end and retain v0.15.0; if it fails, revise the plan with evidence before changing the chosen approach.

For T01, the eligibility rule is conceptually:

```text
context = existing review context for selected run
anchor is eligible when:
    it is the context's active head for the selected stable work
    AND anchor.pdf_content_hash == exported_source_hash
note is eligible when:
    it is that context/work's saved active head
    AND its PDF references resolve only to matching eligible evidence
```

**Acceptance:** The toggle is consistent for single/ZIP output; only matching saved evidence is embedded and readable; no source/review mutation occurs; failures cannot leak partial derivatives or leave worker processes running. The generated file, not a database update, contains the requested comment prose. **Verification:** annotation component and review/server integration, PDF.js/browser annotation checks, targeted visual review, API shapes, and race coverage.

### PDF-S09: Tool retirement and epic verification

**Story:** As a project maintainer, I need the supported commands, tests, and documentation to make the frontend/backend PDF workflow the sole user workflow. **Technical target:** remove both direct-database PDF tools after replacement coverage exists and deliver one reviewed, verified epic. **Dependencies:** S01-S08 accepted.

**References:** [Makefile](Makefile), [coverage policy](config/coverage_policy.something), [offline E2E flow](tests/support/go/e2eflow/flow.go), [browser runner](tests/support/node/run-playwright.ts), [documentation dependencies](docs/DOC-STATE.md), and [verification matrix](#verification-matrix-and-commands).

- [ ] **PDF-S09-T01: Retire command sources and maintained callers.** Remove the pdf-store and pdf-export implementations/thin commands and obsolete CLI-only tests after the application coverage is in place. Remove their Make variables/targets, tools/E2E prerequisites, and stale coverage/ownership entries without lowering surviving floors. Update every maintained caller and example; do not broadly clean ignored build evidence or claim old binaries elsewhere were removed.
- [ ] **PDF-S09-T02: Move complete E2E behavior onto the app.** Replace Go setup's pdf-store invocation with a visible browser upload on a real-pipeline-generated corpus. Preserve A1/A2 review lineage and add successful/failed replacement, removal, single/ZIP download, and comments-toggle checks. Post-browser verification inspects both databases for selections, deleted unused blobs, counters, immutable review evidence, and exactly-once mutation audit. Keep all fixture copies, ports, and outputs isolated and all provider traffic offline.
- [ ] **PDF-S09-T03: Finish source and product documentation.** Reconcile the documentation updated with each implemented story: AGENTS and architecture/database/standards/design/application/operator documents must describe current paired readiness, forward recovery, writable PDFs, exports/comments, and retired commands. Add required Go documentation and TypeScript JSDoc, regenerate SQL/classes/catalog only from authoritative inputs, inspect generated diffs, review each document and every declared dependent, and then update acknowledged hashes. Keep implementation details out of normal UI copy unless they affect a user decision; do not rewrite generated catalog headings just to rename its existing JavaScript terminology.
- [ ] **PDF-S09-T04: Run the complete verification and review release readiness.** Run the focused suites first and then the matrix below, including Go 1.25 dependency verification, server/review/worker races, coverage, both browser projects, accessibility/visual checks, offline E2E, SQL freshness, builds/tools, and documentation/static checks. Record actual results and specific environmental limitations. Review a disposable migrated bundle, worker cleanup, partial-upgrade forward retry, restored startup, and the final diff; do not migrate or delete a user's real corpus as a development test.

**Acceptance:** No maintained user workflow depends on either PDF CLI; the exact allowed dependency has passed and remains justified; epic behavior works end to end; required evidence is recorded; docs and generated artifacts are current; no implementation/source-control/deployment action outside user authorization is taken.

## Shared contract: user behavior

| Action or condition | Result |
|---|---|
| Upload to a registered `not_available` document | Validate bytes, select the blob, set the inventory timestamp, advance the document change counter, and enqueue one upload event. |
| Replace an available document with different bytes | Receive the complete upload, verify its declared size/digest and all requirements, then atomically select the new blob, advance the counter, enqueue one replacement event, and delete the previous blob if no document still selects it. The old file stays available throughout upload and validation. |
| Upload interrupted, incomplete, invalid, or rejected; transaction fails | Preserve the old selection, bytes, timestamps, and counter. Commit no replacement audit event and leave no speculative new blob. |
| Save identical bytes with the current expected counter | Return `changed: false`; preserve timestamps and counter, and create no duplicate audit event. Verify any reused stored blob rather than trusting its hash label. |
| Remove an available document | Confirm removal, change the row to `not_available`, clear its selected hash and inventory timestamp, advance its counter, enqueue one removal event, and delete the outgoing blob when unreferenced. |
| Remove an already unavailable registered document with the current counter | Return `changed: false` without a second audit event. |
| Upload after removal | Accept a new explicit upload using the current document change counter. There is no restore-old-PDF action. |
| Another document selects identical bytes | Keep the shared blob; remove only this document's selection. Delete the blob when its final current selection is removed or replaced. |
| A stale tab attempts a change | Reject its precondition, return the current state, preserve the selected browser file and review inputs, and require review before retrying. |
| DOI missing, inventory unregistered, run ineligible, or writer unavailable | Explain the specific unavailable capability and reject changes in the backend as well as the UI. |
| PDF replacement or removal affects existing anchors | Preserve anchor text, hashes, geometry, lineage, and history. Mark mismatched or missing document evidence unavailable; do not relocate anchors or change decisions automatically. |
| Download one PDF | Use a frontend action to request validated bytes from the backend with a readable, sanitized attachment filename. The browser controls the destination. |
| Export several PDFs | Select a bounded set of Corpus articles and download one ZIP containing validated files and a result manifest. Report skipped unavailable/corrupt files without silently discarding the successful ones. |
| Export with Include comments disabled | Return the original stored PDF bytes. Do not query or append application review comments. Annotations already embedded in the uploaded original remain part of those original bytes. |
| Export with Include comments enabled | Add matching saved highlights/comments from the selected run's review context to a derived PDF copy. Leave stored bytes, selection, hash, and review heads unchanged. Report included and excluded evidence counts. |

Replacement and removal affect PDF availability in every run for the work, including historical runs. Confirmation copy must state that scope and that unused old file bytes cannot be restored through the application. A run's review status of `removed` remains a separate review decision and does not delete its PDF.

Permanent deletion means removing the unused blob row and its bytes from the live logical database, with no application recovery path or old-hash download endpoint. SQLite can retain old pages in WAL files or existing reader snapshots, and deletion need not shrink the file. Configure secure deletion on companion mutation connections, but do not promise forensic erasure from storage media, old backups, exports, browser memory, or files already downloaded. Do not run VACUUM or delete user backups during an HTTP request.

## Shared contract: HTTP

Retain current read URLs. Use the selected run and article revision for new mutations so the backend resolves the stable work and DOI itself. The browser never supplies a database path, server filesystem path, or arbitrary DOI registration request.

| Method and route | Contract |
|---|---|
| `GET /api/works/{work_id}/pdf-status` | Preserve existing fields and report registration, current document change counter, an opaque mutation ETag, maximum upload bytes, and storage capability/reason. Use `Cache-Control: no-store`. |
| `GET /api/articles/{revision_id}?run_id=...` | Include the same current PDF state plus action availability for the exact selected run/article. Keep storage capability distinct from review-context editability. |
| `PUT /api/runs/{run_id}/articles/{work_revision_id}/pdf` | Save one raw `application/pdf` body, at most 20,000,000 bytes. Require `If-Match`, `X-PDF-Size`, and `X-PDF-SHA256` for the selected file. An unavailable selection becomes an upload; an available selection becomes a replacement or no-op. |
| `DELETE /api/runs/{run_id}/articles/{work_revision_id}/pdf` | Remove the current selection using the same required `If-Match` precondition. Require an empty body; no JSON content type is necessary. |
| `GET` or `HEAD /api/pdf/{work_id}` | Preserve inline reading, content-hash ETag, ranges, and conditional reads. The embedded reader additionally supplies `content_hash` as a validated query parameter to pin its requests to the bytes it opened. These are evidence reads, not explicit export actions. |
| `POST /api/runs/{run_id}/articles/{work_revision_id}/pdf-export` | A frontend download action sends a bounded JSON request containing the expected `content_hash` and optional boolean `include_comments` (default false); return the validated original or annotated PDF as an attachment. Return a structured conflict if selection changed. |
| `POST /api/runs/{run_id}/pdf-exports` | A frontend export action sends up to 20 selected article revision IDs with expected content hashes and one optional `include_comments` boolean (default false) for the selection; return PDFs plus a bounded result manifest. Enforce a 100,000,000-byte aggregate exported-PDF limit after any annotation generation. |
| `GET /api/health` | Add actual companion write capability and a safe disabled reason. Correct the existing `review.pdf_store_read_only` report to reflect storage capability rather than assuming every bound store is read-only. |

The document mutation ETag is separate from the PDF byte ETag. It includes work identity and a monotonic document change counter, for example `"pdf-42-c7"`; content hash alone cannot detect a selection that changed and later returned to identical bytes. Do not accept wildcard or weak mutation preconditions.

Successful mutations return HTTP 200 and a typed JSON object containing `changed`, `operation` (`uploaded`, `replaced`, `removed`, or `unchanged`), current `pdf_status`, nullable `event_key`, and `audit_delivery` (`delivered`, `pending`, or `not_applicable`). A saved change whose metadata audit mirror is pending is still a committed save, and the UI must say so.

| Failure | Proposed status and code |
|---|---|
| Invalid identifiers, unexpected query keys, invalid conditional-header syntax, invalid/missing upload size or digest headers, invalid export selection, or nonempty DELETE body | 400, using the existing structured bad-request envelope. |
| Origin rejected | 403 `origin_rejected`. Preserve exact loopback listener and Host enforcement. |
| Missing work or article revision outside the selected run | 404, using the existing not-found envelope. |
| Run running, failed, or trashed; missing DOI; or missing normalized registration | 409 with distinct lifecycle, eligibility, or inventory codes. |
| Missing mutation precondition | 428 `pdf_precondition_required`. |
| Stale mutation precondition | 412 `pdf_state_conflict`, including bounded current state. A stale identical-body request still fails its precondition. |
| Body exceeds the byte limit | 413 `request_too_large`, including chunked requests without a declared length. |
| Upload media type is unsupported | 415 `unsupported_media_type`. |
| Signature invalid or reused stored content fails integrity checks | 422 with distinct PDF validation/integrity codes. |
| Completed upload byte count or SHA-256 differs from the selected file's declared size/digest | 422 `pdf_upload_mismatch`; an interrupted HTTP body is a failed upload and never reaches the mutation transaction. |
| Export selection exceeds file-count or aggregate-byte limits | 413 with an explicit export-limit code; do not start an archive. |
| Explicit export's expected content hash no longer matches | 409 `pdf_changed` for a single file; a per-file result for a mixed ZIP selection. |
| Comment-enabled export cannot safely transform the PDF or match a note's PDF references | Report a stable reason and excluded count. Invalid annotation geometry, unsupported transformation, or writer failure rejects a single export or skips that file in a ZIP; never silently deliver an unannotated file while claiming comments were included. |
| Companion writes unavailable, admission capacity exhausted, or transient SQLite contention | 503 with a safe capability/busy code and bounded retry guidance. |
| Storage/outbox failure before commit | A safe storage error; roll back the entire PDF transaction and preserve the prior selection. |

Use raw binary upload because the form needs exactly one file and no filename persistence. This avoids base64 inflation and multipart temporary files. Extend the existing API transport's body type to support a browser `File`/`Blob`, retaining one JSON response/error decoder and existing JSON mutation behavior.

The existing validator checks the file envelope and content identity, not complete PDF renderability. A damaged or encrypted PDF may pass those checks and fail to render in PDF.js; preserve explicit reader-error feedback and the ability to download, replace, or remove that file. Adding a full backend PDF parser is outside this phase.

Before upload, the frontend obtains `File.size` and computes the file's SHA-256 using the existing browser platform APIs. Require a positive bounded decimal `X-PDF-Size` and a lowercase 64-character hexadecimal `X-PDF-SHA256`; compare both with the bytes actually received. A `%PDF-` prefix alone must not let an incomplete transfer replace a document. Read errors, premature EOF, inconsistent framing, cancellation before commit, size mismatch, or digest mismatch abort the replacement.

Reject invalid transport and preconditions before retaining an upload body where practical; recheck document change counter inside the write transaction. Read and validate the complete bounded body before holding database locks. Limit simultaneous retained upload bodies to two per server and release capacity on every exit path.

Give upload routes a finite 60-second body deadline and sufficient finite response time for the subsequent bounded storage operation; retain the existing short query timeout after body ingestion. Apply upload-specific deadlines before the JSON response-budget wrapper, which currently buffers and does not expose the underlying response writer. Preserve ordinary-route deadlines and JSON/query budgets, and keep audit backlog draining outside request budgets.

Export routes use the existing same-origin and bounded-JSON transport guards. Their successful binary responses need explicit byte, query, concurrency, and time limits rather than the JSON serialization ceiling; error responses still use the shared JSON envelope. Downloads do not require a writable PDF companion, a completed active run, or initialized review context; they require exact selected-run article membership and readable valid bytes. Their metadata audit requirement is described below.

## Shared contract: persistence and migrations

### Companion migration

Add `migrations/corpus.pdf/V00003_pdf_document_management.sql` and append it to [config/database.corpus.pdf.something](config/database.corpus.pdf.something). Do not edit V00001 or V00002. No metadata migration is expected because metadata audit already has before/after columns and the delivery link table already supplies idempotence.

The new migration must:

1. Add `pdf_documents.change_counter` as a positive integer with initial/default value 1 for both existing and newly registered documents. Every real selection change advances it; no-op operations leave it unchanged.
2. Preserve the existing `not_available`/`available` consistency checks, DOI registration rows, selected hashes, timestamps, and blob bytes.
3. Add database protection against deleting document registrations or updating a selection without advancing its counter exactly once. This prevents counter reuse and makes old add-only writers fail instead of changing selection without concurrency evidence.
4. Add an index on document `content_hash` for bounded checks of remaining current selections during deletion.
5. Add nullable JSON-validated `before_json` and `after_json` columns to `pdf_audit_outbox`; existing pending events retain null values and remain deliverable.
6. Rebuild only `pdf_download_attempts` as needed to turn its historical `content_hash` into a logical hash reference. Preserve every existing value, ID, remaining foreign key, index, append-only trigger, and AUTOINCREMENT behavior. Historical attempts must not force retention of unused bytes.
7. Preserve the PDF blob update-protection trigger and metadata audit immutability. Do not remove unrelated integrity controls or purge preexisting orphan blobs as a migration side effect.

Retain the required DOWN section markers and document their limits, but do not introduce a downgrade workflow. Deleted bytes cannot be reconstructed, and restoring the legacy attempt-to-blob foreign key must fail safely when historical hashes no longer have blobs. Normal upgrade recovery resumes forward from committed migrations. Restoring a consistent backup pair is exceptional disaster recovery, never the automatic response to one store failing to upgrade.

### One required schema pair

Treat metadata and PDF as one application bundle with independently numbered schemas. Current configured targets are metadata V00028 / PDF V00002; this feature proposes metadata V00028 / PDF V00003. Different numbers do not mean the stores are out of sync: readiness means each store has reached its own required target. Latest means the schemas shipped with the installed application and matching configuration, not an arbitrary newer release found elsewhere.

Use [config/database.something](config/database.something) as the configuration entry point and derive the required pair from the endpoints of its two selected migration chains. Do not add a separate manually maintained `Sync1`, `Sync2`, or minimum-version history in this phase. The current chain endpoints already identify the required pair, and Git retains previous release configurations. Only introduce ordered intermediate pair checkpoints when a future migration actually needs to read/transform data across both databases at specific intermediate schemas; the inspected feature migrations have no such dependency.

Pin the pair supported by the backend code in the focused bundle coordinator, and test/check that it matches the configured chain endpoints. This small code/config compatibility check prevents an older binary from blindly accepting a newer runtime configuration. Use canonical migration filenames, verify complete required history and declared supersedes aliases, and reject unknown/newer history or missing required steps. A last-applied row or `actual >= minimum` comparison alone does not establish compatibility. No new generator, dependency, bundle-version table, third database, or SOMETHING language feature is required.

The existing migration ledgers remain authoritative for progress. The coordinator validates both configurations and readable migration inputs before changing either store, applies metadata migrations in configured order, applies PDF migrations in configured order, and then verifies the whole target pair and required protection contracts. Serialize preparation and require maintenance ownership so a schema upgrade cannot run underneath active serving or pipeline processes. Preserve the existing exclusive pipeline-attempt lock and keep normal concurrency between already-current server instances.

If metadata reaches its target and PDF migration fails, retain that committed progress, report the failed store/migration, and block normal use. Rerunning the same operation skips committed metadata migrations and retries the pending PDF steps. Never automatically downgrade the completed store, wipe the other store, edit history, or restore only one database. Each migration retains its own SQLite transaction; this is a resumable upgrade of a bundle, not an atomic transaction across both files.

Expected non-current states are an older corpus/backup opened by a newer application, a failed/interrupted upgrade, or previous tooling that upgraded only one store. These are upgrade/recovery states, not supported normal serving states. Unknown newer schemas require the matching newer application, not an automatic downgrade. A missing bound database or an outdated database without write permission must fail readiness with a clear reason.

The currently approved entry-point policy keeps migrations in explicit backend preparation and pipeline operations; serving and ordinary handlers do not migrate. Preserve that approval while adding startup verification of both targets. Automatically invoking the same preparation operation before serving is a possible later policy change, but it must not be implied by requiring current schemas. Current-schema PDF files with restricted write permission remain compatible with the evidence/download capability policy.

### Storage operations

Extend the handwritten API in [backend/pdfstore/store.go](backend/pdfstore/store.go), with a focused sibling mutation file if needed. Update authoritative [schema/queries.sql](backend/pdfstore/schema/queries.sql) and [schema/definitions.sql](backend/pdfstore/schema/definitions.sql), then generate the private SQL implementation with the existing sqlc workflow.

A changed save or removal uses one companion transaction for all of: expected-version validation, blob insertion/reuse when applicable, document compare-and-swap, outgoing unreferenced-blob deletion, and outbox insertion. Failure at any point rolls back all of them. Keep blob contents immutable; replacement moves the document pointer rather than overwriting the previous blob.

Replacement has one commit point. No database write, old-pointer clearing, old-byte deletion, or success/replacement audit record occurs while the body is arriving or being validated. Only after the full upload passes transfer, signature, size, digest, eligibility, and concurrency checks may the transaction stage the new blob, update selection, delete unreferenced outgoing bytes, and insert its outbox event. Those staged writes become visible together at commit; any pre-commit error rolls them all back. Readers continue to use the old file until the replacement transaction commits.

Failures to send the HTTP response after a successful commit are a different case: the replacement may already have completed. Return or display an uncertain outcome and reconcile through fresh status/audit evidence rather than reporting that the old file is guaranteed unchanged, deleting the new file as compensation, or blindly retrying. Metadata audit mirroring can also remain pending after commit because the durable outbox is already the committed event record.

Validate the complete size and digest of a deduplicated existing blob before reuse. Removal must remain possible for a corrupt current blob, and replacing it with a different valid blob must not require reading the corrupt old bytes. Do not silently repair a corrupt shared blob or report it as a successful no-op.

Keep the document registration after removal so pipeline reconciliation cannot reset the counter or recreate the file. Existing `Register` and pipeline synchronization remain idempotent and preserve manual selections and removals. A later pipeline run never restores deleted bytes automatically.

### Explicit preparation and opening

Extend `migrateMain` in [backend/cli/main.go](backend/cli/main.go) so the supported `analysis migrate --db ...` / `make migrate DB=...` operation prepares both parts of the bundle: apply metadata migrations, prepare/migrate the companion, and reconcile existing normalized DOI registrations without creating a new pipeline attempt. Preserve the existing `pdfsync.Sync` algorithm by moving shared inventory coordination into the bundle consumer and making the workspace stage call it. The bundle must not import workspace orchestration; the metadata composition root and companion store must not import the bundle consumer.

Put preparation/readiness composition in a focused `backend/bundle/` consumer shared by the application entry points, with explicit narrow consumer classification and regression coverage in the import checker. `migrateMain` runs the coordinated upgrade, while `serveMain`/`server.Open` verify readiness without migration side effects. In [RunPipelineContext](backend/workspace/store.go), prepare both schemas before `StartWorkspaceAttemptContext` or completed-plan reuse; PDF migration must not first happen at the end of a research run. Preserve family-level migration APIs and focused tests without forcing every temporary single-store fixture through bundle preparation.

Verify both targets after migration and before declaring success. Keep schema readiness, normalized inventory reconciliation, and pending audit delivery distinct; a queued audit mirror does not mean a schema is outdated. Registration and outbox delivery retain their existing idempotence and can be retried without creating an extra research attempt.

For first preparation with no binding, bind and initialize the default separate companion through that explicit administrative command. If metadata already binds a companion but that file is missing, reject preparation and serving with a restore/repair explanation rather than silently creating an empty replacement. Preserve relative-path, traversal, symlink, and distinct-file checks.

In `server.Open`/`openBoundPDFStore`, retain the read-only PDF connection and add a single existing-only `mode=rw` companion connection with foreign keys, busy timeout, immediate write transactions, secure deletion, and the server's statement-budget driver. Do not use migration-running `pdfstore.Open` from server paths. Keep `pdfstore.New` free of connection and migration side effects.

Make `server.Open` verify both schema targets without migrating, then check the new columns, relevant protection triggers, and outbox before enabling writes. An old-schema companion fails readiness instead of silently serving a partial application; a current-schema permission-read-only companion can still serve evidence with `pdf_store_writable: false` and an explicit reason. Missing or unsafe bound files remain failures; ordinary upload requests do not initialize storage. Close both companion connections, maintenance ownership resources, and background workers on shutdown and partial-startup failure. Health output reports both actual/target schemas and bundle readiness after startup.

## Shared contract: audit

Keep `pdf_document_inventoried` for uploads and add `pdf_document_replaced` and `pdf_document_removed` to [backend/manifest/lifecycle.go](backend/manifest/lifecycle.go) and action validation. Preserve support for existing historical PDF action names.

Also add `pdf_document_export_prepared` for explicit frontend downloads/exports. Record one work-scoped event per included PDF with a shared request correlation ID for a ZIP. The event means validated bytes were prepared for a response, not that the browser saved a file to disk. GET/HEAD reader, range, and cache requests do not create export events.

Each changed operation records UTC time, a unique event key and correlation ID, `actor: user`, stable work identity, normalized DOI, and the frontend initiation source. Record the initiating run and article revision as context, while keeping manual PDF events globally work-scoped with a null owning `pipeline_run_id`; the existing run-membership projection must continue to show the event from every relevant run. The application has no authenticated user identity, so the audit must not pretend the run's configured reviewer proves who clicked the control.

Before/after evidence contains only bounded operational facts: status, selected hash, byte size when known, inventory timestamp, and document change counter. Record whether the outgoing blob was deleted or retained because another document still selects it. Do not record PDF content, selected text, note bodies, client paths, original filenames, or reviewer email.

Extend [backend/pdfstore/audit.go](backend/pdfstore/audit.go), the delivered-event type and query under [backend/database/pdfbinding/](backend/database/pdfbinding/), and their generated SQL inputs to copy the new nullable before/after fields into existing metadata audit columns. Existing events remain compatible.

Export preparation does not mutate PDF storage and must not need a writable companion outbox. Append its bounded operational evidence directly through the metadata audit family, using a family-owned transaction for a ZIP's event batch. Record requested work/revision identifiers, included hashes/sizes, and correlation IDs without filename/title prose or PDF bytes. If this required audit insertion fails, do not begin the explicit export response. Preserve readable inline evidence separately. No metadata schema migration is needed.

For comments-enabled exports, audit the toggle value, selected context, included immutable note/anchor version IDs and counts, source PDF hash, and generated output hash. Keep payloads bounded and omit comment bodies, selected quotations, and reviewer email. The exported PDF is the deliberate destination for requested review prose; neither audit records nor application logs duplicate it.

The PDF state change and durable outbox event commit together. Metadata `audit_events` and `pdf_audit_links` still commit together in a separate transaction. Failure of the metadata mirror after the PDF commit must never be reported as a failed PDF save or trigger an automatic compensating deletion.

Add a server-owned relay that processes bounded batches, retries pending delivery during operation, replays pending events after startup, and stops before its connections close. Start it explicitly from the serving lifecycle in `serveMain` after successful opening/listening; `server.Open` itself remains free of data writes. Keep retries cancellable and bounded, serialize relay ownership inside one server, and preserve idempotent delivery across multiple server processes. Do not synchronously drain an arbitrary backlog inside an upload request. Surface pending audit delivery and safe relay failures through health/status feedback without logging research content.

Update [frontend/src/components/audit-events.tsx](frontend/src/components/audit-events.tsx) so replacement and removal have accurate summaries and readable before/after changes. Verify both the article timeline and Provenance PDF category; the current generic document summary would incorrectly describe removal as recording a document.

## Shared contract: reader and anchor consistency

### Selection and deletion

Use immediate companion transactions and document compare-and-swap to serialize conflicting writes across server processes. Hash reuse, pointer changes, and deletion checks occur inside the same transaction, so a shared blob cannot be deleted between another document's reference check and commit. An in-memory mutex alone is insufficient.

Read current document metadata and bytes from a coherent companion snapshot. During a concurrent change, a request may complete using the snapshot it already opened; a new request after commit must observe the new selection or absence. Handle a changed selection explicitly rather than turning an expected race into an internal storage error.

Invalidate matching server cache entries after a committed change, and check the current selection before using any cached bytes, including in another server process. The embedded PDF.js URL carries its selected content hash so later range requests cannot accidentally combine bytes from two PDFs. A hash mismatch returns a stable `pdf_changed` response and offers a reader refresh. Preserve existing GET/HEAD, conditional-request, and Range behavior for unchanged documents.

### Anchor correctness

Add the reader's `pdf_content_hash` to anchor creation requests and require the matching hash for active anchor edits/restoration. The current server chooses the latest hash itself, which could otherwise attach old selected text and geometry to newly uploaded bytes.

Coordinate the availability check and metadata anchor commit using a companion write reservation acquired before the metadata transaction and held until that transaction finishes. Put the reservation lifecycle in the companion owner and orchestration in `backend/server`; do not expose generated query types or create a distributed transaction. This is required so replacement/removal cannot interleave between validating availability and committing an anchor, including across separate server processes.

Use one lock order whenever both stores are involved: acquire the PDF reservation before starting the metadata mutation. The audit relay must commit its metadata transaction before marking delivery in the companion, with no metadata transaction held while waiting for a PDF lock. Test this ordering against a second server/store connection and pipeline registration traffic.

Keep anchor tombstoning possible when its original PDF is unavailable; deletion of a metadata anchor does not require current PDF bytes. Derive the frontend's anchor-removal eligibility from run/context/head writability separately from PDF-dependent creation/restoration eligibility. Creation and restoration still require current matching bytes. Preserve work/run membership checks, review lifecycle guards, expected anchor-version checks, and immutable anchor history.

After a committed replacement/removal, cancel old PDF rendering, destroy the old PDF.js document/worker, clear pending text selections and stale highlights, update anchor availability, and remount or clear the reader. Reset invalid `pdf_page`/focused-anchor state when content changes while preserving the selected article and research context. Do not discard unsaved decision or note input by rerendering the entire review workspace.

A second tab may still display bytes it already loaded. Revalidate PDF state on relevant focus/reader actions and before submitting anchors; the backend remains authoritative. Preserve the existing navigation guard and browser-draft behavior.

## Shared contract: frontend and export

Extend `PDFStatusPanel` and the detail lifecycle in [frontend/src/views/detail.tsx](frontend/src/views/detail.tsx), using a focused `frontend/src/components/pdf-management.tsx` component for the form and mutation state if needed to keep the view manageable.

- Show Upload PDF when the document is registered and unavailable. Keep Download PDF independent from PDF mutation eligibility and companion write permission; require its own readable-byte and metadata-audit capabilities. Enable Upload, Replace, and Remove only when storage and selected-run action eligibility permit them.
- Use one labeled file input with `accept="application/pdf,.pdf"`, show the selected filename and size locally, and require an explicit submit action. Do not persist or transmit the filename.
- Show the byte limit and specific unavailable reason. The existing JSX input types already support `file` and `accept`; no JSX runtime expansion is required.
- Confirm replacement/removal with article identity, shared-run scope, and permanent-deletion wording. Explain shared-blob retention when applicable rather than claiming every removal deletes the underlying blob.
- Present pending, success, unchanged, validation failure, conflict, storage failure, and saved-with-audit-pending states near the form. Disable duplicate submission while work is pending and preserve the selected file on a recoverable failure.
- On success, refresh current PDF status, relevant audit evidence, and derived PDF availability used by Corpus/Evaluation. Preserve filtering and navigation context on return to the list.
- On a lost response, fetch current state and explain an uncertain outcome before retrying. Do not blindly repeat a replacement/removal with a fresh precondition. If a user cancels a request, do not claim cancellation proves that no commit occurred.
- Keep controls keyboard-operable with labeled inputs, visible focus, status announcements, and usable narrow-screen layout. Follow the existing JSX, camelCase, typed payload, and render-then-bind conventions.

Update [frontend/src/api/types.ts](frontend/src/api/types.ts) for status, capabilities, mutation responses, and anchor request contracts. Extend [frontend/src/api.tsx](frontend/src/api.tsx) narrowly for binary bodies and conditional headers. Reuse existing control primitives and [frontend/styles/views.css](frontend/styles/views.css); regenerate CSS classes only if selectors change. Vendored PDF.js and D3 assets need no changes.

### PDF download and export workflow

Keep Download PDF in article detail and add Download selected PDFs to the Corpus article collection in [frontend/src/views/corpus.tsx](frontend/src/views/corpus.tsx). Start with an explicitly selected set on the current page, capped at 20 articles; clear selection when run/page/filter context changes and never interpret it as every matching article across hidden pages. Show selected count, known unavailable entries, limits, and preparation/failure feedback. Export is separate from PDF replacement/removal and does not change availability, counter, or review state.

Both download surfaces offer a labeled Include comments checkbox, default off, before starting the request. Explain that it adds matching saved PDF highlights and their linked notes from the selected run. Show the selected review context and included/excluded counts, distinguish no matching saved comments from export failure, and leave unsaved notes in the editor. The toggle applies consistently to every PDF in a ZIP and never triggers an implicit note save or context initialization.

Extend the existing batched `InventoryForDOIs` projection and `overlayPDFInventory` in [backend/server/evaluation.go](backend/server/evaluation.go) to provide the current selected hash and byte size for article rows. Use those bounded page results for export selection and expected hashes; do not add one status request per row. A current-schema read-only companion still supports readable export metadata; an old schema requires successful preparation before normal use.

The backend resolves requested revision IDs within the selected run, deduplicates repeated identities, and checks expected hashes. Reject invalid or outside-run selectors before preparing an archive. For a valid selection containing unavailable, changed, or corrupt files, preserve successful entries and include skipped entries with stable reasons in `export-manifest.json`. If no file qualifies, return a structured error instead of an empty ZIP. Report included/skipped counts to the frontend in bounded response headers; the ZIP manifest contains the per-file details.

Use the selected article revision's title for its attachment name, sanitize path separators, control characters, unsafe filename characters, and leading/trailing dots/spaces, and bound UTF-8 length. Use a stable work-ID suffix to prevent duplicate titles colliding inside a ZIP; use `work-ID.pdf` when a title is absent or unusable. This deliberately replaces the CLI's corpus-wide latest-title lookup and missing-title failure with naming tied to the article the user selected. Format attachment headers with existing standard-library MIME support; never use a client-provided path or filename.

Generate ZIPs with Go's standard `archive/zip`, using stored entries because PDFs are already compressed. Cap aggregate final PDF bytes at 100,000,000 and archive metadata at 1,000,000 bytes, allow at most two active explicit export responses per server including at most one ZIP or annotation transformation, and use a finite 60-second export deadline. Keep at most one PDF and its bounded derived output in memory per transformation; do not buffer a whole archive. Count output growth from comments before committing response headers. Avoid an export job system or persistent/user-selected server output directories.

Pin preflight and emission to one bounded companion read snapshot. Validate all included bytes, output sizes, and names before committing binary response headers, then reread the same snapshot while emitting the archive. Finish the read snapshot before or on cancellation/timeout. A replacement/removal may proceed concurrently, but the archive must not mix different versions of one file or resurrect an old selection for subsequent requests. In-flight snapshots have the same permanent-deletion limitation already documented for readers. A streaming failure after headers begin must abort the response rather than append a JSON error to a PDF/ZIP.

For comments-enabled ZIPs, prepare and validate each derived PDF once before starting the archive and retain its exact output for emission/audit. Use a bounded private temporary spool for those derivatives if necessary, capped at the same 100,000,000-byte aggregate. Use server-created 0600 files with no client-controlled paths, prefer immediately unlinked temporary files on the supported Unix platforms, and close/remove them on success, failure, cancellation, and shutdown. Do not assume regenerating annotations in a second pass will produce identical bytes; source/output hashes must describe the exact files delivered. Plain ZIP exports retain the existing snapshot/stream approach.

Add a bounded binary download helper alongside the existing API transport. Share structured JSON error decoding, check expected content type and response bounds (20,000,000 bytes for an original PDF; 25,000,000 bytes for an annotated PDF; 101,000,000 bytes for a ZIP), detect truncated/failed response consumption, and create/revoke a browser object URL only for a completed response. Hand the prepared PDF/ZIP to the browser's download mechanism and announce that the download was prepared or started; the application cannot prove that a user saved it on disk. Do not accept backend output-directory flags or reproduce server-side overwrite/symlink rules in the UI; the browser owns destination and filename-conflict behavior. Upload/review drafts and navigation context remain intact after downloads.

### Embedded comments: matching, snapshots, and output

Resolve the selected run's existing review context on the backend. Export current active heads visible in that context, including inherited heads frozen into it; do not merge comments from other runs, historical superseded versions, tombstones, or browser drafts. No initialized context means no saved comments to add, not permission to initialize one as an export side effect.

An eligible PDF anchor belongs to the selected stable work and context, is active, and has `pdf_content_hash` equal to the exact source hash being exported. Include its page, complete selected text, label, and geometry. A saved note is eligible as a PDF comment only when it belongs to that work/context and links to such an anchor. General article notes and page-only references have no verifiable hash binding and are excluded from this PDF-comment export. If a note mixes matching and mismatched/unresolved PDF references, exclude the whole note with a reason rather than guessing which prose applies. Keep the underlying note unchanged. These rules are a proposed interpretation of the current data model and must be visible in export help/reporting.

Place matching anchors as standard PDF highlight annotations and attach linked saved note text as readable annotation comments at those anchors. Anchor labels/quotations remain meaningful when an anchor has no linked note. Deduplicate note/anchor identities during selection; a note attached to multiple matching anchors may appear at each referenced location, with output limits accounting for repeated text. Preserve full selected note bodies rather than the truncated list previews. Use the project note parser to obtain safe text; do not embed active HTML, scripts, or application-only executable links. Include existing reviewer display attribution and recorded time, omitting reviewer email.

Read the selected metadata heads and full matching evidence through one bounded metadata snapshot, materialize immutable IDs/content within limits, and close that read transaction before writing export audit. Keep the source PDF snapshot/hash pinned independently. Concurrent note edits or PDF replacement must not mix heads during one export or project old geometry onto different bytes; do not claim one distributed transaction across the two files.

Convert stored unrotated normalized rectangles using the source PDF's actual page geometry, including crop origins, rotation, and user units. Validate page existence and finite in-page geometry before writing. Do not assume that CSS pixels, the current browser zoom, or a top-left origin can be written directly as PDF annotation coordinates. Preserve existing page content, searchable text, and original embedded annotations; do not flatten pages into screenshots.

The original download remains byte-for-byte identical when the toggle is off. When the toggle is on and matching annotations exist, generate a derived PDF, validate the resulting document and expected annotations before export, and use a distinguishable filename plus separate source/output hashes in response metadata and the ZIP manifest. Never insert the derived bytes into `pdf_blobs`, move document selection, increment its change counter, or rewrite review records. Re-exporting always starts from the stored original, preventing duplicate application annotations from accumulating. Reuploading an annotated export is a new file identity and follows the ordinary replacement/hash-matching rules.

Cap emitted application annotations at 200 and their total rendered UTF-8 text at 2,000,000 bytes per PDF; use limit-plus-one detection so exports fail clearly rather than silently truncate comments. Cap a derived PDF at 25,000,000 bytes and the ZIP's aggregate final PDF bytes at 100,000,000. Report excluded nonmatching evidence separately from resource/processing failures. If annotation generation fails or the source cannot be modified safely, reject that single export or record a failed ZIP entry; the user can explicitly retry without comments. Do not bypass encryption/modification permissions or claim to preserve a digital signature invalidated by transformation.

### Allowed PDF-writing dependency and trial decision

The original investigation found PDF envelope/hash validation without an annotation writer. The current checkout has an isolated pdfcpu trial boundary, which still needs the S01 containment/output evidence and S08 application integration. The existing PDF.js dependency remains the reader and independent output verifier. Complete the existing boundary rather than creating a duplicate writer; do not patch PDF byte strings, install a database-access CLI, or shift export generation into a frontend-only path.

The selected trial dependency is exactly `github.com/pdfcpu/pdfcpu v0.15.0`. Its upstream [annotation API](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/pkg/api/annotation.go) writes annotations to a separate output stream, and its upstream [annotation examples](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/pkg/api/test/annotation_test.go) are the version-specific reference for constructing annotations. Its [license](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/LICENSE.txt) is Apache 2.0. This selection authorizes validation and retention on success; it is not a claim that the project integration has been tested.

Run PDF-S01 before depending on annotated exports. Validate actual Go 1.25 compilation, parser/resource behavior, cancellation, Unicode, preservation of page content/existing annotations, license obligations, and transitive dependency impact. Upstream [v0.15.0 declares Go 1.25](https://github.com/pdfcpu/pdfcpu/blob/v0.15.0/go.mod), but its inspected annotation API lacks context cancellation, so request timeout alone is not sufficient. The trial must prove a bounded execution mechanism and record its implementation constraints; do not leave a transformation running after timeout or introduce an unapproved Go upgrade.

During implementation, verify the existing v0.15.0 pin and annotation boundary, use Go module tooling only for a needed correction, review any go.mod/go.sum and license/build diff, and keep library types behind the component. Retain it when PDF-S01 and the integrated PDF-S08 acceptance checks pass. If either fails materially, record the evidence, revise this plan, and keep replacement libraries, version changes, and toolchain changes subject to review. No source, dependency installation, toolchain, or schema changes occur during this plan review.

## Expected source and documentation changes

Required means necessary when the owning story is implemented, not permission to edit during this review. Conditional means edit only if that specific integration requires it. Existing files are authoritative unless explicitly marked generated. Proposed filenames describe new responsibilities; where only a package is named, choose the smallest cohesive split after inspecting that package instead of manufacturing speculative files.

| Source or artifact | Change status | Ownership and responsibility |
|---|---|---|
| `backend/pdfstore/store.go`, `backend/pdfstore/validation.go`, `backend/pdfstore/audit.go`, and focused companion-owned mutation/connection helpers | Required edits; new helper files only for a concrete owning responsibility. | Conditional save/remove, reservations, existing-only writer, typed errors/results, safe deletion, and bounded outbox relay support. |
| `backend/pdfstore/schema/queries.sql`, `backend/pdfstore/schema/definitions.sql` | Required authoritative SQL edits. | Named queries and declared shapes for change counters, inventory reads, mutations, deletion checks, and outbox fields. |
| `backend/database/pdfbinding/delivery.go`, `backend/database/pdfbinding/types.go`, `backend/database/pdfbinding/schema/delivery.sql` | Required authoritative edits. | Deliver nullable before/after evidence without changing metadata schema or delivery idempotence. |
| `backend/database/audit/audit.go` and its owning `schema/` queries | Required API/batch behavior; SQL edits where existing queries are insufficient. | Family-owned metadata export-preparation audit transactions using existing columns; no companion outbox write for export. |
| `backend/pdfannotate/pdfannotate.go`, `backend/cli/` worker dispatch, `backend/cmd/analysis/main.go` where process wiring requires it | Required extension of the existing trial boundary; process-entry edit conditional on existing dispatch. | Application-owned annotation data, isolated pdfcpu imports, bounded data-only internal worker, cancellation and cleanup. |
| `backend/database/review/export.go`, `backend/database/review/schema/export.sql` | Required new authoritative relationship reader/query, unless an existing equivalent can be extended. | Bounded saved note/anchor heads, exact work/hash eligibility, and immutable export snapshots without leaking generated types. |
| `backend/server/server.go`, `backend/server/pdf.go`, `backend/server/reviews.go`, `backend/server/overview.go`, `backend/server/evaluation.go` | Required authoritative edits. | Lifecycle, guarded binary routes, capabilities, cache/reader consistency, inventory overlay, status, and audit reporting. |
| `backend/server/pdf_export.go`, `backend/server/pdf_annotations.go` | New focused files if the confirmed export/annotation responsibilities warrant this split. | PDF/ZIP response preparation, annotation mapping, bounded snapshots/spools; no server-owned family SQL or library model leakage. |
| `backend/manifest/lifecycle.go` | Required authoritative edit. | Validated PDF upload/replacement/removal/export-prepared action vocabulary with historical compatibility. |
| `backend/bundle/` plus schema/maintenance ownership helpers in existing owners | Required new consumer; exact file split remains an implementation choice. | Target pair, coordinated preparation/readiness, shared inventory coordination, maintenance ownership, and forward retry. |
| `tooling/go/importcheck/main.go`, `tests/tooling/go/importcheck/unit/main_unit_test.go`, `tests/tooling/go/importcheck/functional/main_functional_test.go` | Required narrow classification edit and regression coverage. | Recognize the bundle consumer; retain rejection of reverse dependencies and private generated imports rather than weakening checks. |
| `tooling/go/rawsqlcheck/main.go` and `tests/tooling/go/rawsqlcheck/` | Conditional, only if approved connection/platform/outbox SQL locations move or are added/retired. | Keep `approvedLocations` exact and justified; no blanket handler/bundle SQL exception. |
| `backend/cli/main.go`, `backend/workspace/store.go`, `backend/workspace/pdfsync/stage.go` | Required authoritative edits; additional lock wrappers only where existing ownership needs integration. | Preparation before attempt/reuse, serving readiness only, shared inventory extraction, and preserved attempt locking. |
| `migrations/corpus.pdf/V00003_pdf_document_management.sql`, `config/database.corpus.pdf.something` | Required new runtime migration and append to the authoritative PDF chain. | Contiguous V00003 with UP/DOWN markers, existing-record preservation, forward recovery, and protection contracts; never rewrite V00001/V00002. |
| `frontend/src/views/detail.tsx`, `frontend/src/views/corpus.tsx`, `frontend/src/components/review-panel.tsx`, `frontend/src/components/pdf-viewer.tsx`, `frontend/src/components/audit-events.tsx`, `frontend/src/api.tsx`, `frontend/src/api/types.ts` | Required authoritative TypeScript edits. | Management, downloads, bounded selection, typed transport, reader identity, draft-preserving refresh, and audit presentation. |
| `frontend/src/components/pdf-management.tsx` | Conditional new TypeScript component. | Concrete management form/request lifecycle if detail integration warrants extraction; no attachment framework or view import. |
| `frontend/styles/views.css` | Conditional authoritative CSS edit. | Only missing file-control/feedback layout; shared primitives stay in their existing owning stylesheet and use existing theme tokens. |
| `Makefile`, `config/coverage_policy.something` | Required tool-retirement edits; new-package coverage entries based on measured meaningful coverage. | Remove both PDF tools and update maintained callers/ownership without lowering surviving floors. |
| `go.mod`, `go.sum`, applicable license notices | Verify existing exact pin; edits only if the allowed trial requires correction or notices. | Preserve Go 1.25 and unrelated dependency work; no unapproved substitution, unrelated upgrade, or additional npm dependency. |
| `.github/workflows/ci.yml` | Required Node-capable PDF contract gate; other edits only for concrete retired callers. | Run the independent TypeScript/PDF.js suite with Go, Node, and locked dependencies while keeping Go-only jobs independent of Node. |
| `tests/backend/pdfannotate/{unit,integration}/`, `tests/backend/pdfstore/{unit,integration}/`, `tests/backend/bundle/{unit,integration}/`, `tests/backend/server/integration/`, `tests/backend/cli/integration/` | Required focused suites under their real layers. | Pure validation/mapping unit checks; real store, migration, HTTP, and Go worker integration; no tests under production roots. |
| `tests/backend/database/pdfbinding/integration/`, `tests/backend/database/audit/integration/`, `tests/backend/database/review/integration/`, existing workspace integration suites | Required edits where the owning behavior changes. | Audit delivery, export batches, comment selection, immutable histories, pipeline registration/readiness, and concurrency. |
| `tests/contracts/pdf/integration/pdf-annotations.test.ts`, `tests/support/node/pdf-inspector.ts` | Required new independent contract suite; reuse/extend shared inspection only as needed. | Typed PDF.js assertions through the actual annotation worker, owned fixtures/output, explicit CI gate; no Node launch in Go suites. Preserve overlapping untracked work until deliberately reconciled. |
| `tests/frontend/api/unit/`, `tests/frontend/components/<affected-component>/{unit,functional}/`, `tests/frontend/views/detail/functional/`, `tests/frontend/views/corpus/functional/` | Required TypeScript tests scoped to affected behavior. | Binary transport, DOM interaction, file preparation, input preservation, selection, draft/reader lifecycle, and accessible feedback. |
| `tests/frontend/viewer/integration/pdf-management.spec.ts`, `tests/contracts/http/integration/api-shapes.spec.ts`, `tests/frontend/quality/integration/ui-quality.spec.ts` | New PDF management browser spec; required contract updates; quality edits/snapshots when visual states change. | Isolated visible mutations/downloads, typed HTTP shapes, keyboard/axe/responsive checks, and reviewed snapshots. |
| `tests/playwright.config.ts`, `tests/support/node/run-playwright.ts`, shared Go/Node test support, `tests/e2e/offline/` | Required mutation-spec registration and E2E edits; helper changes scoped to reuse. | Register serial isolation in both runners, preserve fixture copies and A1/A2 lineage, replace CLI setup with visible frontend upload, and verify persisted evidence. |
| `tooling/go/pdf-store/`, `tooling/go/cmd/pdf-store/`, `tooling/go/pdf-export/`, `tooling/go/cmd/pdf-export/`, `tests/tooling/go/pdf-store/`, `tests/tooling/go/pdf-export/` | Retire only after equivalent public application coverage exists. | Intentional supported-command removal; preserve useful naming/integrity/partial-result cases in backend/frontend suites first. |
| `tests/tooling/go/prepare-osf/` | Extend only the relevant integration cases. | Preserve schema compatibility and sanitization without presenting the command as the frontend PDF download workflow. |
| `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/DATABASE.md`, `docs/STANDARDS.md`, `docs/DESIGN.md`, `docs/APP-USAGE.md`, `docs/PROJECT-USAGE.md` | Required authored documentation alongside owning stories and final S09 reconciliation. | Describe implemented paired readiness, mutation/reader/export behavior, permanent deletion, supported commands, and tests; remove superseded current-state claims only when the replacement exists. |
| `docs/CSS-REFERENCE.md` | Conditional authored documentation when selectors change. | Describe active layout/style contracts; do not document speculative selectors. |

| Generated path | Authoritative input and generator | When to refresh |
|---|---|---|
| Owning `backend/database/<family>/internal/sql/` and `backend/pdfstore/internal/sql/` | Owner's `schema/` declarations/queries plus `config/sqlc.yaml`; `make sqlc-generate`. | Required after SQL input changes; inspect committed output and run `make sqlc-check`. |
| `frontend/src/jsx/classes.ts` | Six owning stylesheets; `make frontend-classes`. | Only when selectors/tokens change; review output and run `make frontend-classes-check`. |
| `docs/PROJECT_CATALOG.md` | Maintained Go comments, TypeScript JSDoc, and test titles; `make docs-catalog-update`. | After maintained declarations/comments move/change; review generated descriptions rather than hand-editing rows. |
| `docs/DOC-STATE.md` acknowledged hashes | Reviewed maintained `docs/` bytes and their dependency table; `make docs-state-update`. | Only after inspecting each changed document and its listed dependents; do not acknowledge unrelated changes. |
| `build/frontend/dist`, test database pairs, worker executables/reports | Supported build/fixture/test targets using isolated owned output. | During relevant verification; never hand-edit generated assets or reuse/mutate a user's corpus as a fixture. |

Leave `frontend/src/jsx/jsx-runtime.ts`, `frontend/src/jsx/jsx.d.ts`, the TypeScript compiler configurations, npm manifests/lockfile, and PDF.js/D3 vendor assets unchanged unless inspection proves a missing required capability; the current JSX file-input surface, locked dependencies, and TypeScript source/test scopes already support this work. No SOMETHING language feature, additional metadata migration for comments, arbitrary database editing API, or separate attachment framework is planned. The pdfcpu v0.15.0 trial remains the only additional runtime dependency authorized by this epic; annotated copies do not change the schema-pair requirement.

## Verification matrix and commands

### Existing coverage to preserve or adapt

- [tests/backend/pdfstore/unit/validation_unit_test.go](tests/backend/pdfstore/unit/validation_unit_test.go) covers byte bounds and PDF signatures; retain these checks and add any needed transport-boundary cases in server tests.
- [tests/backend/pdfstore/integration/store_integration_test.go](tests/backend/pdfstore/integration/store_integration_test.go) covers normalized DOI handling, deduplication, transaction rollback, and outbox delivery/idempotence. Replace the old expectation that different replacement bytes are silently ignored with the approved conditional replacement behavior.
- [tests/backend/server/integration/pdf_integration_test.go](tests/backend/server/integration/pdf_integration_test.go) covers status, ranges, manual PDF audit visibility, corruption, and read-only connections. Preserve evidence-read protection and add public mutation tests; a dedicated mutation connection does not make read-only reader assertions obsolete.
- Existing server review and review-family tests cover lifecycle, membership, immutable histories, and hash-bound anchors. Extend them for changing/missing PDFs and anchor removal after PDF removal.
- Existing frontend detail, API, PDF viewer, fullscreen, audit, and review tests remain regression coverage for navigation, lifecycle teardown, draft behavior, and evidence presentation.
- [tests/tooling/go/pdf-export/integration/export_test.go](tests/tooling/go/pdf-export/integration/export_test.go) covers title naming, Unicode/bounds, deduplication, partial results, corruption, and filesystem safety. Move naming, selection, integrity, and partial-result assertions to server/browser tests; retire command syntax and output-directory/symlink cases with the command. Replace whole-database-file unchanged assertions with unchanged PDF selections/counters and pipeline/review evidence, allowing the new explicit export audit events.

### Required new or expanded cases

| Layer and location | Cases and evidence |
|---|---|
| Go pdfcpu trial under `tests/backend/pdfannotate/{unit,integration}/` | Compile and run exactly v0.15.0 with Go 1.25; keep pure validation/mapping in unit and writer/real-worker boundaries in integration. Cover parser/resource failures, source immutability, configuration/font isolation, bounded termination/cleanup, and pass/keep or fail/revise evidence without launching Node. |
| TypeScript PDF output contract under `tests/contracts/pdf/integration/pdf-annotations.test.ts` | Build/invoke the real backend worker and independently inspect output with pinned PDF.js through typed test support. Verify text/highlight contents, Unicode, multiple rectangles, crop/rotation/user units, searchable text, original annotations, input/output identity, fixture ownership, and reliable cleanup. Run in the existing Node-capable CI job, not by spawning Node from a Go test or depending on a prior run's files. |
| Companion migration integration under `tests/backend/pdfstore/integration/` | Upgrade from V00002 with available and unavailable documents, shared blobs, pending legacy outbox events, and populated legacy attempts; preserve values/IDs/triggers; validate foreign keys; repeat preparation safely; prove historical attempts no longer prevent byte deletion. |
| Companion mutation integration | Upload, different-byte replacement, identical save, removal, repeated removal, reupload, monotonically advancing counters, stale/ABA preconditions, normalized DOI identity, unregistered documents, corruption, and preservation across pipeline registration. |
| Blob lifetime integration | Shared outgoing blob retained until the final document releases it; outgoing unshared blob row absent after commit and restart; incoming duplicate blob reused only after integrity validation; no speculative blob survives a failed transaction. |
| Transaction failure integration | Force blob insertion, selection update, outgoing-byte deletion, outbox insertion, and commit failures using test-owned SQLite fixtures/triggers/constraints; verify the old document hash, bytes, timestamp, counter, and replacement-event count are unchanged, with no speculative new blob. Cover cancellation and storage-full/permission failures where reproducible; add no production test hooks. |
| Outbox/delivery integration | Upload/replacement/removal before/after fields, replay after restart, legacy null fields, failure before metadata commit, failure after metadata commit but before delivery marking, concurrent relays, bounded batches, and exactly one metadata row/link per event. |
| Server integration | Exact run/revision membership, no review-context prerequisite, active completed run mutation policy, unavailable capability reasons, raw PDF handling, empty/invalid bodies, declared size/digest verification, exactly-at-limit and over-limit uploads, chunked bounds, DELETE routing, and every documented error/status. |
| Complete replacement transport integration | Interrupt/truncate a raw upload after a valid PDF prefix, send a completed shorter body with the original size/digest, send mismatched digests, and trigger timeout/cancellation. Prove the old PDF is readable during upload and after each rejection, with no new blob/counter advance/replacement event. A successful retry must produce exactly one replacement event and one counter advance. |
| Server transport/lifecycle integration | Real HTTPServer Host/Origin enforcement, unsupported media/methods, missing/weak/stale preconditions, deadline behavior, two-body admission bound, cancellation cleanup, connection/relay shutdown, no binding, missing bound file, and opening/serving that checks readiness without changing schemas. Old schemas fail readiness and require explicit preparation. |
| Server concurrency integration | Two independent servers/connections compete to replace/remove the same version; exactly one wins. Race shared-blob adoption with deletion, anchor create/restore with replacement/removal, and audit replay with mutations; verify no wrong-hash anchors or deadlocks. |
| Server read regression | GET/HEAD, range and conditional responses before/after change; hash-pinned requests reject a changed document; cache entries cannot expose a removed selection to a subsequent request; invalid stored content still fails safely. |
| Bundle preparation integration under `tests/backend/bundle/integration/` and CLI/workspace integration | Older/current pairs, either store pending, interrupted upgrades, second-store failure after first-store commit, and retry that skips completed steps and finishes forward. Prove no downgrade, no extra maintenance attempt, and PDF readiness before pipeline attempt creation/reuse. |
| Target/readiness integration | Supported pair matches configured endpoints; reject unknown/newer history, missing migration steps, mismatched code/config, missing bound file, and an outdated unwritable store. Accept verified supersedes history and current-schema read-only PDF evidence. `server.Open` rejects partial pairs; health reports actual/target schemas. |
| Maintenance ownership integration | Concurrent preparations serialize; upgrades cannot alter schemas beneath active serving/pipeline processes; failure/cancellation releases maintenance ownership; multiple current servers still work. Preserve existing exclusive-attempt tests and stop nonparticipating legacy binaries before maintenance. |
| Import ownership under `tests/tooling/go/importcheck/{unit,functional}/` | Explicitly classify only `backend/bundle/` as a consumer. Permit its family/composition-root/companion imports, reject family/root/companion imports of the bundle, retain private generated-package boundaries, and preserve shared low-level restrictions. Inspect that server readiness cannot depend on workspace orchestration. |
| CLI/preparation edge cases | Missing metadata rejected; unbound companion initialized by preparation; missing existing binding target rejected; repeated migration safe; no arbitrary path accepted. Keep inventory reconciliation and audit delivery distinct from schema readiness. |
| Frontend TypeScript unit/functional under `tests/frontend/<affected-component>/{unit,functional}/` | File size/hash computation and binary/If-Match headers, structured errors, failure states preserving the old reader/file/input, identical-save feedback, saved-with-pending-audit feedback, confirmation cancellation, reader destruction, page reset, stale selection rejection, draft preservation, bounded export selection, binary error decoding, and object-URL cleanup. Use `tests/support/node/setup.ts` for DOM shims and mount runtime Nodes for interaction tests. |
| HTTP contract under `tests/contracts/http/integration/api-shapes.spec.ts` | New status/capability/mutation fields and nullability, upload size/digest headers, raw binary requests, anchor hash requests, and export request/error/manifest shapes and binary headers. Keep the suite serial on its copied fixture. |
| Serial browser spec at `tests/frontend/viewer/integration/pdf-management.spec.ts` | Through visible controls: upload, download/verify PDF bytes and filename, ZIP export/verify entries and manifest, replace successfully and unsuccessfully, confirm/cancel removal, remove, reupload, observe audit changes, check second-tab conflict, verify inherited/mismatched anchors, preserve drafts, and reload persisted state in Chromium and Firefox. |
| Quality browser suites | Labeled file controls, keyboard confirmation/focus, live feedback, axe checks, responsive layout, and reviewed light/dark snapshots for available, unavailable, and failure states. |
| Offline E2E and post-browser database verifier | Generate the corpus with the real pipeline, upload through the browser instead of `build/pdf-store`, retain A1/A2 review-lineage checks, exercise replacement/removal and a failed replacement on an isolated eligible work, and download single/ZIP outputs through the frontend. Verify bytes, final selections, absent unused blobs, preserved review history, and exactly-once mutation audit evidence in both databases. |
| Backend export integration | Individual replacement bytes and attachment headers; removed/missing/corrupt files; Unicode, absent/duplicate/unsafe titles; duplicate selectors, outside-run IDs, expected-hash conflicts, partial/all-failed ZIP selections, count/byte/query/deadline/admission limits, unchanged PDF storage, and required export audit insertion failure. Verify archives are valid, contain no unsafe paths, and preserve one pinned snapshot during replacement/removal. |
| Comment selection integration under `tests/backend/database/review/integration/` | Toggle off does not load review content; toggle on selects active current/inherited heads in the chosen run/context and exact work/source hash. Include complete linked note bodies; exclude tombstones, old heads, other contexts/works, mismatched/mixed PDF references, page-only/general notes, and drafts. Test empty/uninitialized contexts and limit-plus-one overflow without silent truncation. |
| Annotated PDF integration under `tests/backend/server/integration/` | Exported annotation contents and highlights match saved evidence and page geometry; cover Unicode, full note text, multiple rectangles, inherited heads, rotated/cropped pages and user units, existing embedded annotations, unavailable pages, malformed input, signatures/permissions, writer failures, deadlines, and output growth limits. Verify the stored original hash/bytes, document counter, and review heads never change. |
| Annotated output lifecycle | Generate each derivative once, validate it before headers/audit, and verify emitted output hashes against retained exact bytes. Cover temporary spool permissions, byte/admission limits, disk/write failures, cancellation, and cleanup with no leaked files or work after timeout. Plain downloads remain byte-for-byte original and preserve annotations already in the source. |
| Comment export browser/contract tests | The labeled toggle defaults off and sends the same flag for single/ZIP exports. Parse downloaded PDFs using pinned PDF.js to verify text/highlight annotations, page locations, original annotation preservation, plain/annotated output hashes, visible counts/reasons, and no silent fallback on writer failure. Render representative annotated pages and review visual placement; changing comments/PDFs mid-export must not mix snapshots. |
| Comment export audit/privacy | Record toggle, source/output hashes, bounded immutable IDs, context, and counts; exclude prose, quotations, and reviewer email from audit/logs/manifest. The requested prose exists only in the derived PDF annotations. Repeated exports do not accumulate annotations in source storage. |
| Download lifecycle integration | A response interrupted after binary headers aborts cleanly; the frontend does not create a completed-file download from failed consumption, display it as saved, or append a JSON error to a PDF/ZIP. Verify only explicit exports create preparation audit events; ordinary PDF.js requests do not. |
| Existing OSF backend compatibility | Sanitized copies remain valid on the new companion schema and preserve audit/history without resurrecting deleted blob rows. Frontend bundle export remains separate work under the same required download boundary. |

Tests use production migrations, temporary/copied database pairs, deterministic PDF fixtures, and controlled local HTTP. Production databases and the generated base browser fixture remain untouched. Direct SQL remains appropriate inside integrity-focused tests and database verifiers; it must not substitute for the upload workflow in browser/E2E tests.

Register `pdf-management.spec.ts` in both the Playwright mutation matching and the Node runner's mutation-spec classification/default list. Otherwise the new write tests would execute in the read suite and violate fixture isolation.

### Verification prerequisites and scope

| Scope | Required environment | How to handle missing prerequisites |
|---|---|---|
| Go annotation trial, store/server integration, race, coverage | Actual Go 1.25 for trial compatibility with `GOTOOLCHAIN=local`, locked Go modules, writable test-owned temporary/build output, and race support for the race gate. | Record the unavailable toolchain/resource explicitly; do not let automatic switching, a Node-dependent Go test, or a skipped parser check masquerade as a passed Go gate. |
| Independent PDF.js contract and TypeScript checks | Node 22.18+, npm, installed root-lockfile workspaces, exact existing PDF.js, and Go for building the actual worker. | Prepare locked dependencies through the supported `make frontend-install` workflow when needed; preserve the existing npm dependency versions. Missing inspection prerequisites leave the trial gate incomplete. |
| Browser/quality/E2E | Assembled frontend assets, installed Chromium/Firefox binaries for their selected projects, and platform libraries. | Use supported frontend/browser preparation when needed and record environmental limits; never broaden live-provider access or write the base fixture to obtain a passing result. |
| SQL freshness | Pinned `build/sqlc` matching the Makefile, authoritative SQL inputs, and committed generated output. | Use the documented `make sqlc-install` preparation if required; ordinary builds/tests stay offline and consume generated code. |
| Documentation | Current source comments/catalog and reviewed documentation hashes. | Distinguish preexisting stale catalog/state from this plan's Markdown checks; update only the implementation's relevant generated content after review. |

Install/download prerequisites only when needed through the existing workflows and session permissions. Normal validation uses installed locked dependencies and controlled offline inputs; a missing dependency is an environmental limitation rather than permission to change dependency versions or weaken assertions. The TypeScript PDF contract adds a concrete CI command, not a new public target or test framework.

### Verification commands after implementation

Run focused cases first, then their affected suites. These are planned commands, not results from this planning task.

```sh
GOTOOLCHAIN=local make test-go DOMAIN=backend COMPONENT=pdfannotate
GOTOOLCHAIN=local make test DOMAIN=contracts COMPONENT=pdf LAYER=integration
make test-go DOMAIN=backend COMPONENT=pdfstore
make test-go DOMAIN=backend COMPONENT=bundle
make test-go DOMAIN=backend COMPONENT=database/pdfbinding
make test-go DOMAIN=backend COMPONENT=database/audit
make test-go DOMAIN=backend COMPONENT=database/review
make test-go DOMAIN=backend COMPONENT=server
make test-go DOMAIN=backend COMPONENT=cli
make test-go DOMAIN=backend COMPONENT=workspace
make test-go DOMAIN=tooling COMPONENT=go/importcheck
make test-go DOMAIN=tooling COMPONENT=go/prepare-osf
make test-frontend-unit
make test-frontend-functional
make check-frontend
make frontend-build
make test-frontend-all WORKERS=1 TEST_FILE=tests/frontend/viewer/integration/pdf-management.spec.ts
make test-frontend-all WORKERS=1 TEST_FILE=tests/frontend/viewer/integration/review.spec.ts
make test-frontend BROWSER=chromium TEST_FILE=tests/frontend/viewer/integration/viewer.spec.ts
make test DOMAIN=contracts COMPONENT=http LAYER=integration BROWSER=all WORKERS=1 TEST_FILE=tests/contracts/http/integration/api-shapes.spec.ts
make test-frontend-visual
make test-e2e
make test-race
make coverage
make build
make tools
make sqlc-check
make test-docs
make check
```

Format changed Go with `gofmt` and run `make format-check`. Run `make check-imports` after changing the graph and `make check-sql` after adding/moving SQL calls; if the SQL ownership inventory changes, also run `make test-go DOMAIN=tooling COMPONENT=go/rawsqlcheck`. Run `make sqlc-generate` after SQL input changes, inspect generated output, and run `make sqlc-check`. Run `make frontend-classes` only when CSS selectors change; `make check-frontend` checks classes and all three TypeScript workspace scopes. After maintained comments/declarations move/change, run `make docs-catalog-update` and review the catalog; review changed documents and their [DOC-STATE.md](docs/DOC-STATE.md) dependents before `make docs-state-update` and `make check-docs`. `make check` includes formatting, vet, SQL/import ownership, documentation, and secrets checks. Review visual differences rather than accepting snapshots blindly. Live-provider E2E is unnecessary for this local PDF feature and remains opt-in.

## Rollout and epic acceptance

Before implementation rollout, stop old processes and take a consistent backup of the metadata/PDF pair, including a WAL-safe snapshot where applicable. Never use a real corpus to develop or test deletion. Run the one coordinated bundle migration, then start the new backend with matching configuration and frontend assets. Verify pair readiness, writable capability, and audit replay on a disposable migrated copy first. Retry a partial upgrade forward rather than downgrading one database to match the other; serving refuses the pair until it reaches both targets.

The existing OSF backend snapshots the two databases sequentially. With mutable PDF selection, a live bundle snapshot cannot promise consistency across those reads. Preserve backend schema compatibility and the requirement to quiesce writers for coordinated bundle snapshots/backups; a frontend OSF bundle download requires a separate plan. This is not an exemption for command-based user downloads, and two independent SQLite backups must not be described as an atomic snapshot.

Removing `pdf-store` and `pdf-export` is an intentional command compatibility break. Update every maintained caller and documented example to the frontend/backend PDF workflow. Old binaries left in ignored `build/` or installed elsewhere are not removed automatically and must not be treated as supported PDF workflows; do not run broad cleanup commands against user-owned build evidence. Downgrading a binary does not restore permanently deleted PDFs, and existing backups/exports retain their own copies.

The broader move away from database-access tools remains future work. This phase establishes the frontend/backend path for the complete PDF lifecycle, including downloads and selected-PDF exports, without adding arbitrary metadata mutation, automated external PDF acquisition, multiple attachments per DOI, in-browser PDF page editing, file-version restore, remote access, or a new authentication system. Future user-facing downloads must follow this same boundary.

The implementation is ready for review when all of the following are demonstrated:

- A user can upload, replace, remove, and reupload a PDF entirely through article detail, with persistence across restart and separate database files.
- Both schemas reach the installed application's configured/supported target pair before normal serving or a research attempt. Interrupted upgrades resume forward without separate rollback work; partial or unknown/newer schemas fail readiness instead of being used silently.
- The old PDF remains selected and readable throughout upload/validation, and every pre-commit failure leaves its bytes, selection, timestamp, counter, and replacement audit count unchanged. Only a complete valid replacement commits the new file and its event together.
- A user can download one PDF and export a bounded selection as a ZIP entirely through the frontend, with safe readable names, validated bytes, explicit partial results, and accurate export-preparation audit evidence.
- Single/ZIP exports offer Include comments. When enabled, matching saved review comments appear inside the exported PDFs as standard annotations; when disabled, original bytes are preserved. Other PDF hashes/contexts and unsaved drafts cannot leak into the output, and original storage never changes during annotation export.
- Old unused blob rows are deleted, shared current bytes remain valid, and removal preserves document registration and immutable research/review history.
- Every changed operation has durable audit evidence, eventual exactly-once metadata delivery, and accurate visible outcomes even if delivery temporarily fails.
- Stale tabs cannot overwrite newer selection, and anchors cannot be created against a different document than the one the user selected text from.
- Reader ranges, cache behavior, note/decision drafts, unavailable anchors, corpus availability, and audit timelines behave correctly after changes.
- The supported build and E2E workflow no longer relies on either direct-database PDF executable, and documentation accurately describes the approved mutation and frontend download/export behavior.
- Required focused, race, SQL generation, frontend, browser, accessibility/visual, documentation, and offline E2E checks pass or have specific reviewable environmental limitations.

The writer version is now selected for trial: pdfcpu v0.15.0. Record its validation outcome in PDF-S01 and its integration outcome in PDF-S08. Keep it when those outcomes satisfy the agreed requirements; otherwise revise this plan before proceeding with a replacement approach. This does not authorize a Go toolchain upgrade.

This revision completes the requested plan-standards review only. Execute remaining stories in dependency order with recorded acceptance evidence; if the pdfcpu gate fails or a proposed implementation changes approved behavior, revise the affected plan before proceeding with that approach. No source/test change, user-data migration, or PDF deletion occurs during this review.
