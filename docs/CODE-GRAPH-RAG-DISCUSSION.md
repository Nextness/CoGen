# Automated Code Graph and RAG Pipeline for Agents

**Epic:** [RAG-01](https://github.com/users/Nextness/projects/5) — Improve agent workflows with a local code graph and selective retrieval. **Status:** tracking published; implementation not started. Sections 1–11 describe the design; Sections 12–15 evaluate implementation, define the story/task backlog, and record verification requirements and the published GitHub tracker. Existing code references are evidence; new paths, commands, schemas, and snippets are proposed examples, not implemented or compiled features.

This document records the current design discussion for local developer tooling that improves OpenCode and Codex navigation through structural queries, selective retrieval, and agent hooks. LadybugDB, optional local embeddings, isolated worktrees, and stacked PRs for dependent work are the chosen directions. The graph, query CLI, hooks, and lifecycle automation remain proposals; the repository currently provides the `worktrees/` directory convention with a tracked placeholder and ignored workspace contents described in Section 7.

One implementation agent executes this epic in dependency order. The delivered workflow targets Linux with Codex CLI `0.166.0` and OpenCode `1.19.0`, English project knowledge, and 2–3 writing agents after the setup is complete. The agreed resource target is 1 GB aggregate RAM for RAG-owned processes. Retain the newest three successful graph publications per worktree and collect eligible older generations at a project-wide 10 GB generation-storage threshold. Query growth, task-boundary publication, retention safety, and the planned shared worktree skill are specified below. Planning/project publication does not install the skill or runtime tooling.

## 1. Motivation and Scope

Agents already have documentation, generated catalogs, repository tools, file reads, and `rg`. The proposed tool connects those sources so an agent can answer a focused question without repeatedly rediscovering the same relationships or loading the entire repository into context.

- Structural questions: what calls this declaration, which handlers use this table, and what depends on this configuration key?
- Semantic questions: where is similar validation implemented, and which guidance applies to this change?
- Workflow questions: which source owns this generated file, which tests are relevant, and which other task owns a dependency?

Correctness comes before reducing tokens or lookup time. The graph must expose incomplete extraction, uncertain relationships, stale locations, and bounded results. An empty result must never imply that a declaration has no callers or that a change has no impact unless the relevant coverage supports that conclusion.

This is separate developer tooling, with its own disposable derived index. It does not become part of the research pipeline, viewer, metadata database, PDF companion, or their migration chains. The broader extraction scope is Go, TypeScript/TSX, SQL, SOMETHING, Makefile, JSON, YAML, and selected Markdown; a Go-first prototype validates the contract before expanding coverage.

### 1.1 Initial Questions Are a Seed Corpus

The first questions define acceptance examples, not a limit on what the project can ask. A parameterized capability such as `symbol.context` applies to every supported symbol, not just the PDF function used by a fixture. New questions may reuse existing capabilities, compose them, or reveal missing facts that require extractor/schema work. The maintained question corpus and query implementation catalog are separate artifacts; Section 3.5 defines how they grow.

| Initial agent question | Initial capability or evidence needed |
| --- | --- |
| What directly calls `pdfstore.Store.Save`? | Resolved Go callers and unresolved call sites |
| What does `saveArticlePDF` call? | Go callees with current source evidence |
| Which HTTP entry point reaches that store operation? | Route-to-handler mapping and bounded call paths |
| Which packages depend on `backend/pdfstore`? | Import/dependency inventory |
| Which SQL queries read or modify `pdf_documents`? | Maintained query/table relationships |
| What maintained SQL generates `MarkPDFDocumentAvailable`? | sqlc ownership and generator mappings |
| How does frontend `uploadPDF` reach the Go handler? | TypeScript request, route inference, and Go resolution |
| Which tests are relevant to changing PDF replacement? | Suite inventory and explained candidate associations |
| Which supported Make commands regenerate affected output? | Generator ownership and supported command inventory |
| Which documents need review after this change? | Declared review dependencies and source relationships |
| Which instructions apply to this path and task? | Explicit guidance mappings with static fallback |
| Which agent task owns a related interface or file scope? | Plan/task references and local ownership records |

These 12 questions are approved as the starting corpus. Later additions should cover real work across ingestion, enrichment/provider behavior, normalization/validation, caching/reuse, search revisions, attempts/manifests, review lineage, PDF audit delivery, migrations, exports, SOMETHING, frontend state, and developer tooling. Coverage expands through reviewed examples from actual stories rather than trying to invent every future question upfront.

## 2. Proposed Architecture

The design separates extraction, graph storage, optional semantic retrieval, and orchestration. A versioned extraction contract and query catalog connect those layers to thin agent-specific adapters.

### 2.1 Extraction and Semantic Resolution

| Language or format | Proposed extraction | Important limit |
| --- | --- | --- |
| Go | Native syntax parsing plus package loading and type information where required | `go/ast` alone does not resolve types, imported symbols, or dynamic dispatch. Record the build context. |
| TypeScript and TSX | TypeScript compiler API for resolved declarations and references; Tree-sitter where syntax alone is sufficient | Include compiler configuration and module resolution; syntax matching cannot prove a call target. |
| SQL | Maintained schema and named queries, sqlc configuration, and existing raw SQL inventory | Distinguish owning inputs, generated wrappers, migrations, and unresolved dynamic SQL. |
| SOMETHING | Reuse the existing compiler in [backend/something/](../backend/something/) | Do not replace the language's semantics with regular expressions. |
| Makefile | Targets, prerequisites, variables, and known repository command conventions | Shell recipes and dynamic expansions can leave relationships unresolved. |
| JSON and YAML | Structural parsing of selected configuration, manifests, and known path references | A matching string alone is not a resolved code relationship. |
| Markdown | Headings, sections, repository links, and selected guidance | Prose references and semantic similarity are weaker evidence than resolved symbols. |

Tree-sitter is useful for uniform syntax extraction, while language-native tooling supplies semantics where the queries require them. Go's [package loader](https://pkg.go.dev/golang.org/x/tools/go/packages) can provide syntax, imports, and type information according to its load mode. The prototype must demonstrate which relationships it actually resolves rather than assuming every parsed reference is accurate.

Initial records cover files, packages/modules, declarations, imports, documentation comments, SQL tables and named queries, and configuration declarations. Source locations include path, start/end line and column, and source hash. Qualified semantic identities support joins; locations are evidence attached to an indexed source version, not permanent identities.

The requested upward, downward, and entry-point traversal is a static call graph, not a runtime call stack. Direct calls may be resolved precisely; interface dispatch, callbacks, reflection, generated code, and unavailable dependencies can leave candidate or missing edges. Cross-language paths such as TypeScript request to Go route to repository method to SQL query require explicit extractors or repository conventions, with their evidence retained.

| Evidence category | Meaning | Required presentation |
| --- | --- | --- |
| Semantically resolved | A language resolver or authoritative repository mapping identifies the target | Show the call/reference location, source version, and resolver. |
| Syntactic | Syntax identifies a reference without proving its target | Label it as unresolved or a candidate. |
| Convention-inferred | A route, path, name, or configuration convention suggests a relationship | State the convention and supporting locations. |
| Unresolved or unsupported | Extraction cannot determine the relationship | Report the coverage gap instead of silently dropping it. |

Every extraction reports language/file coverage and failures. An incomplete package load or parse error must not produce an apparently complete graph. Whether uncertainty needs a numeric score is open; evidence categories and provenance are required first.

### 2.2 Graph Storage and Process Ownership

LadybugDB is the chosen embedded graph engine for Cypher traversal and the planned full-text/vector retrieval path. Its native Go integration introduces CGO and a C/C++ library, which are acceptable for separately built local tooling. Store graph data beneath the owning worktree's ignored `build/` directory, separate from application databases.

Ladybug's [concurrency contract](https://docs.ladybugdb.com/concurrency/) permits one read-write `Database` object or multiple read-only database objects for a database file. Multiple connections within one owner differ from independent processes opening the same file. A writer lock alone therefore does not make separate CLI readers safe while that database remains open for writing.

The proposed CLI design publishes immutable generations so readers never open the database being updated:

1. Acquire a publisher lock scoped to the repository and worktree; concurrent requests coalesce or return a bounded busy result.
2. Build a private candidate generation. Start with full rebuilds; incremental construction can follow after correctness and rebuild cost are measured.
3. Validate schema, source/build fingerprints, extraction coverage, and representative queries, then finalize and close the writer before publication.
4. Atomically publish a small manifest pointing to the completed generation, checking that the expected predecessor and indexed source snapshot are still valid.
5. Each lookup pins one generation, opens it read-only, and uses that generation for the entire request. Already-running readers may finish against an older generation with its actual fingerprint reported.
6. On failure, retain the previous usable generation and report its freshness honestly. Retire old generations only through a protocol that prevents deletion between reader selection and pinning or while any reader still holds them.

This is a proposed application protocol, not a claim that atomic rename alone solves database concurrency. Publication, reader pins, crash recovery, cancellation, and safe retirement must be proven on the supported developer platform. If source changes during extraction, the result must not be labelled current for the new bytes. A global mutable "latest graph" shared by all worktrees would violate this contract.

Retain the newest three successfully published generations per worktree. Older generations can remain while total generation storage stays below 10 GB across the repository's worktrees. At the threshold, collect eligible older generations oldest first toward an 8 GB low-water target. Active reader pins and task-baseline pins protect additional generations; never delete a protected generation to satisfy the target. Implement the Linux selection/pin/retirement protocol before enabling automatic deletion, as specified in Section 12.7 and RAG-S06-T04.

Build isolation also needs proof. Adding a separate `main` under the root Go module does not keep CGO out of `go vet ./...` or the production-package coverage scan in the [Makefile](../Makefile). Choose a separate module or another demonstrated boundary that leaves normal root builds, checks, and tests offline and independent of Ladybug's native dependency. No dependency or module is added by this design update.

Pin the selected database/binding versions and explicitly provision native libraries under ignored local output. Model/native downloads are setup actions, never hidden effects of a hook or ordinary build. A dedicated build/update target is proposed, outside `make build`, `make tools`, `make test`, and `make check`. Graph CI integration, cross-compilation, a daemon, and a long-lived watcher are outside the first prototype.

### 2.3 Optional Semantic Retrieval

Embeddings are local only and optional. Begin with selected documentation sections and source comments; declaration-aware code chunks can follow if evaluation shows an improvement. Commit messages and issue descriptions are possible later inputs, with explicit scope and provenance rather than automatic ingestion.

Retrieval should combine exact symbol/path matching, lexical search, bounded graph traversal, and optional vector ranking. Semantic similarity must not remove exact matches or replace evidence from a resolver. Deduplicate source comments and their generated catalog copies so repeated text does not dominate ranking. Return the reason each result was selected and a source location the agent can inspect.

Record model identifier/version, dimensions, tokenizer/chunking policy, normalization, and content hashes. A change to these inputs invalidates affected embeddings. If the model is absent, lexical and graph queries still work; no hook downloads a model or starts an unexpected service. Local indexing does not imply the coding agent's model inference also runs locally.

### 2.4 Updates and Agent Triggers

Use an explicit task-boundary update command plus lifecycle freshness checks in both OpenCode and Codex. `make code-graph-update` is a proposed command name, not an implemented Make target. It should be the documented way to publish a verified refresh at a checkpoint and should report indexing failures directly.

At startup/resume, compare repository identity, source fingerprints, and the targeted tool versions, then pin the task's baseline generation. Before selected lookups, inspect freshness and retrieve only relevant evidence. During edits, keep that baseline unchanged and record a task-local dirty inventory; do not rebuild after every change. Checksums remain authoritative because an edit may occur outside either agent. Include tracked eligible files and explicitly allowed new untracked source files in checkpoint capture, so newly created declarations appear before staging is required.

The default lookup hook is advisory: a missing index, timeout, unavailable model, or extraction failure leaves normal agent navigation available and reports a compact status. It does not approve tool use, rewrite commands, bypass repository instructions, or trigger unbounded indexing. Whether a small update fits a synchronous hook budget is measured; otherwise direct the agent to the explicit update command.

Git hooks and CI are excluded from the initial trigger design. Agents should not depend on a commit to discover dirty edits, and normal repository checks should not require an optional local graph. A continuous watcher can be reconsidered only if explicit updates and bounded freshness checks prove inadequate.

### 2.5 Proposed Core Graph Schema

Start with a small typed core driven by real questions:

| Entity | Minimum useful properties |
| --- | --- |
| Repository/worktree snapshot | Repository identity, canonical worktree identity/root, base and HEAD commits, dirty fingerprint, generation, schema and extraction versions |
| File | Relative path, language, source hash, parse status, generated/maintained classification |
| Package/module | Import/module identity, build context, source files |
| Declaration | Stable language-qualified identity where possible, name, kind, signature/receiver, visibility, location, documentation |
| SQL table/query | Owning family, schema or query name, source location, maintained/generated relationship |
| Configuration | Owning format, qualified key/block, source location, resolved references where supported |

`Function`, `Type`, and `Interface` are conceptual declaration kinds; the Ladybug DDL must explicitly choose kind properties or dedicated tables and relationship endpoints. Do not assume database table inheritance from an object-oriented diagram. Directories and documentation sections can become explicit entities when navigation requires them.

Core relationships include `CONTAINS`, `DEFINES`, `IMPORTS`, `CALLS`, `USES`, `IMPLEMENTS`, `QUERIES`, `CONFIGURED_BY`, and `DOCUMENTED_IN`. Use a distinct embedding/composition relationship for Go rather than treating it as class inheritance. Each relationship retains its extractor, evidence category, source site/hash, and applicable build context.

Candidate extensions include routes, tests, commands, and generators, with relationships such as `SERVES`, `REQUESTS`, `TESTS`, and `GENERATED_FROM`. Add them when validated questions require them. Test relevance inferred from imports or names is a recommendation, not proof of runtime coverage. Cyclomatic complexity and similar descriptive metrics are optional properties rather than prerequisites for navigation.

### 2.6 Schema Evolution and Change Management

Keep three contracts separate: storage schema, extractor-to-schema mappings, and query catalog. A new question over existing facts changes the catalog without reindexing. A new relationship, resolver change, or schema change may require re-extraction, including unchanged consumers of changed declarations.

| Approach | Strength | Cost and current position |
| --- | --- | --- |
| Typed core with additive evolution | Readable relationships and explicit endpoints/properties | Every new concept changes DDL and mappings; suitable for a small first schema. |
| Generic entities and relationships with kind properties | Easy experimentation | Weaker constraints and more convention-dependent queries; avoid making the entire core generic. |
| Schema-as-data with generated DDL | Reviewable registry and repeatable generation | Adds a generator and meta-schema before their value is proven. |
| Hybrid | Typed core, versioned registry, separate query catalog, disposable generations | Preferred longer-term direction; handwritten DDL is sufficient for the prototype. |

The graph is derived data, so incompatible changes should build a new generation from source. Do not add application-style migrations, permanent deprecated tables, or dual writes without a demonstrated rebuild problem. An experimental annotation mechanism can hold non-query-critical metadata; promote a concept into typed storage when real queries need its constraints.

Store schema, extraction contract, per-language resolver/parser, query compatibility, and optional embedding versions. Full rebuild is the safe fallback when compatibility is uncertain. Query-only changes do not require a rebuild when they use the same stored facts. Incremental updates require dependency invalidation, not just a list of modified files. Measure full rebuild time early before investing in incremental publication or generated DDL.

## 3. Lookup Contract and Agent Adapters

### 3.1 Common Interface and Query Catalog

Start with one local CLI shared by both agents and an internal versioned interface. It accepts a catalog query identifier, typed parameters, an explicit worktree root, and result bounds; it returns versioned JSON or a compact text view of the same response. Keep Cypher in the maintained query catalog rather than asking agents to invent arbitrary queries. Bash-based invocation and thin host adapters are sufficient for the agreed scope. MCP and a new HTTP service are deferred; project-specific knowledge and retrieval quality take priority.

Each catalog entry declares its purpose, parameter schema, supported graph/schema versions, required extraction coverage, limits, and expected evidence shape. Query IDs below illustrate the contract; they are not implemented commands or a finalized catalog format.

| Proposed query | Parameters | Useful result |
| --- | --- | --- |
| `symbol.context` | Qualified symbol or path/location | Declaration, callers/callees, relevant documentation, and resolution gaps |
| `change.impact` | Paths/symbols and bounded traversal depth | Dependents, routes, configuration, and candidate tests with supporting paths |
| `owner.sources` | Maintained or generated path | Owning package, authoritative inputs, generator, and supported command |
| `tests.for_path` | Source path or symbol | Relevant suites and supported Make selectors, with the reason for each association |
| `route.trace` | Route or request location | Client, handler, repository, and SQL relationships, including inferred links |
| `guidance.for_path` | Task/path and optional terms | Applicable guidance and source sections without promoting retrieved prose into policy |

Validate parameters and constrain paths, traversal depth, row count, execution time, and output size. Avoid shell interpolation and query-text interpolation from tool arguments. The explicit CLI must remain useful without hooks or embeddings, with ordinary file reads and `rg` as available fallbacks.

### 3.2 Hook Lifecycle and Capabilities

| Lifecycle point | Intended action |
| --- | --- |
| Session start or resume | Discover the actual worktree, check fingerprints, and provide a small index/capability status. |
| New task or recognized lookup | Select a relevant catalog query using explicit paths/symbols and simple intent rules. |
| Before a supported tool call | Add bounded evidence when it helps the pending lookup or edit; avoid retrieving for every command. |
| After edits or external changes | Record dirty files/affected relationships against the pinned baseline and recheck evidence; defer publication to the task checkpoint. |
| After compaction | Restore a small workspace/generation summary and essential references, not the entire previous retrieval payload. |

The [official Codex hook documentation](https://learn.chatgpt.com/docs/hooks) describes `SessionStart`, `PreToolUse`, `PostToolUse`, and compaction events. For `PreToolUse`, plain stdout is ignored; model-visible context uses `hookSpecificOutput.additionalContext` in JSON. The adapter must use the event's supported output shape and the configured context limit, without returning tool-approval or input-rewrite decisions.

[OpenCode plugins](https://opencode.ai/docs/plugins/) provide `tool.execute.before`, `tool.execute.after`, file events, and session events. A callback's ability to inspect or modify tool arguments does not prove it can inject model-visible context at the desired point. Verify a supported delivery mechanism in the pinned version rather than assuming it behaves like Codex or printing text that the model never receives.

Record tested agent versions, supported events, canonical tool names and aliases, input/output schemas, delivery timing, trust/setup requirements, timeout behavior, and proof that the model receives the intended context. Install hooks only through the agent's supported configuration and trust flow. If an event cannot deliver context reliably, use explicit CLI/tool retrieval for that path.

Prevent recursion when the lookup itself invokes tools. Deduplicate retrieval by session/task, query parameters, worktree, generation, and source fingerprint, with invalidation after relevant changes. Parse known lookup shapes conservatively; do not use a model call to classify every shell command or attempt to understand arbitrary shell programs. Deduplication must not suppress a needed refresh after an edit.

### 3.3 Results, Budgets, and Explanation

Every response includes the query/version, repository and worktree identity, generation, indexed source fingerprint, freshness state, extraction coverage, and any failures. Each item carries its symbol/path/location, source hash, evidence category, reason for inclusion, and originating resolver or document. Bounded results report truncation, omitted counts where known, and a follow-up query or file location.

Prefer exact matches and short summaries first, then allow deliberate expansion. Establish measured limits for hook latency, update duration, traversal, memory, rows, and context tokens before enabling automatic injection. Report cold and warm timings separately. Cancellation and timeouts must release database handles, reader pins, and locks. A lookup failure must not look like a successful empty answer.

### 3.4 Input Scope and Trust

Index eligible repository source and selected guidance through an explicit inventory. Exclude runtime corpus content, caches, application SQLite files, `corpus/`, `intermidiate/`, generated build output, vendored assets, credentials, environment secrets, and agent transcripts. Allow selected new source files without accepting arbitrary ignored inputs. Do not log source content or sensitive values; operational diagnostics should use identifiers, timings, and counts.

The primary checkout must never recursively index `worktrees/`. A lookup inside a linked worktree indexes that worktree's own root and excludes any nested workspace container. Keep path resolution within the selected root, including symlink handling. Classify useful committed generated code, such as sqlc wrappers, and connect it to authoritative inputs rather than either indexing all generated/vendor output indiscriminately or dropping relationships needed for tracing.

Retrieved documentation, comments, issue text, and generated summaries are evidence, not privileged instructions. Preserve provenance and separate adapter control metadata from source excerpts. Retrieval must not turn a source comment into permission to run a command or override the repository's instruction hierarchy.

### 3.5 Question Discovery and Capability Evolution

Maintain `config/codegraph/questions.json` as the reviewed acceptance corpus and `config/codegraph/queries.json` as the executable capability catalog. The first contains real questions, domain/intent, expected evidence, fixture links, coverage requirements, and capability references. The second contains typed query implementations and bounds. Natural-language variations and new symbol/path parameters do not each need a new Cypher query.

```json
{"id":"workspace.reuse-after-migration","domain":"workspace","question":"Why does a schema migration invalidate a completed workspace plan?","capabilities":["symbol.context","change.impact","guidance.for_path"],"required_evidence":["execution fingerprint construction","schema-version input","workspace no-op decision"],"fixture":"workspace-reuse-schema-change","status":"proposed","source_task":"<story-or-task-id>"}
```

The example extends coverage beyond PDF navigation without claiming that the present extractors already answer it. A gap such as missing manifest/configuration evidence stays visible until the owning extractor and regression fixture exist. Each new or materially changed story reviews its knowledge needs and adds/updates an acceptance case, or records why existing cases cover them. The coordinator reviews unresolved retrieval gaps at story review and before a release.

```text
new feature, unanswered lookup, or incorrect/missing evidence
  -> sanitized local gap record
  -> coordinator review and proposed question/fixture
  -> reuse/compose a capability, or add missing extraction/query support
  -> positive, negative, uncertainty, and dirty-worktree regression cases
  -> reviewed corpus/catalog change and version compatibility check
  -> close the gap only after its acceptance case passes
```

Local feedback under `build/codegraph/feedback/` records structural intent/capability IDs, relevant repository paths, failure/coverage category, and a task reference. It does not automatically log raw prompts, source excerpts, transcripts, research content, or secrets. Promotion into tracked questions is reviewed; hooks do not silently edit the catalog or create GitHub issues. A query over existing facts requires no reindex; new extractor facts or schema changes rebuild the affected derived index under the established compatibility rules.

Corpus coverage reports group cases by domain and capability, distinguish supported/partial/unimplemented cases, and show unresolved gaps and regression results. A growing list of unanswered question strings is not progress by itself. Maintain a small fast core fixture plus domain-specific cases; do not preload the full corpus or plan into every agent prompt.

## 4. Freshness and Incremental Updates

The manifest identifies the repository and canonical worktree, declared base/parent and HEAD commits, eligible tracked and allowlisted untracked file hashes, relevant build configuration, schema/extraction/parser versions, query compatibility, and optional embedding versions. A commit hash alone cannot identify a dirty agent workspace. Timestamps may help diagnostics but do not prove freshness.

| Change | Required handling |
| --- | --- |
| Added or modified file | Parse its current bytes, replace owned records, and invalidate affected dependents. |
| Deleted file or declaration | Remove obsolete nodes, edges, and retrieval chunks, including incoming references that need resolution again. |
| Rename or moved declaration | Refresh paths and locations; preserve identity only when supported, otherwise use delete/add semantics. |
| Position-only edit | Refresh source locations and hashes even when declaration meaning is unchanged. |
| Signature, type, or import change | Re-resolve affected consumers, including unchanged files; changed-file parsing alone is insufficient. |
| Build tags, module/compiler configuration, or tool version change | Invalidate the relevant semantic context and rebuild when the affected set cannot be proven. |
| Parent update, checkout, merge, or rebase | Recompute source/build identity before reusing an index or retrieval cache. |
| Parse/load failure | Expose partial coverage or retain an older generation with stale status; never silently claim completeness. |

Before an edit relies on a graph location, compare the current source hash with the indexed hash and re-read or refresh when they differ. A source edit after lookup can invalidate the result even if it was fresh when returned. Graph evidence helps navigation; it does not replace reading the source being changed.

Start with correct full rebuilds and no-op fingerprint checks. Incremental extraction and embedding updates follow measured need and must obey the same immutable publication contract. Shared immutable artifacts may be reused only for a complete matching fingerprint, with source identity checked independently of their physical cache location. Never share mutable dirty-worktree state under a repository-only cache key.

### 4.1 Task Baseline, Dirty Inventory, and Checkpoint Publication

The task works against a pinned original graph and publishes an updated generation when its updates are complete. Source files change as the agent edits them; the original graph remains a record of the earlier source snapshot. Agent knowledge and planned edits can guide lookup and invalidation, but cannot make an old database represent new source bytes.

An initial setup can explicitly bootstrap the first index outside an active writing task. If no compatible baseline exists, report unindexed/unavailable state and use normal source tools until an explicit checkpoint creates one; startup does not invent a baseline or silently rebuild a task's graph.

1. At assignment, record the plan/task reference, worktree, baseline generation, source fingerprint, parent commit, and write scope. Keep the baseline pinned for the task, including interruption/resume.
2. During editing, append a local change journal and verify it against source hashes, Git tracked/index differences, and eligible untracked files. Record added, modified, deleted, and renamed paths plus affected dependency scopes. Planned-but-unwritten changes remain hypotheses in the plan.
3. Query the original graph as baseline evidence. Results touching dirty files or invalidated consumers are marked baseline/stale/uncertain and verified with live file reads, `rg`, or bounded live syntax inspection. New symbols are discoverable through those live tools before the graph checkpoint. An unchanged caller file is not sufficient proof that its resolved relationships remain valid after a callee/import/type change.
4. At ready-for-handoff, finish source, generated output, and the explicit documentation review/acknowledgement workflow where applicable. Capture actual source/configuration/build inputs and compute the full delta from the baseline, including changes omitted from the journal. Run relevant checks against that captured snapshot, or keep the owned worktree quiescent through capture and verification with matching before/after fingerprints. Bind every check record to the exact source/build fingerprint it verified; any source/build drift invalidates that evidence and requires recapture/revalidation.
5. Build one candidate generation from the verified snapshot, require its source/build fingerprint to match the check evidence, and recheck live identity before verified publication/handoff. Use changed inputs plus dependency invalidation when proven; initially a full rebuild remains the correctness fallback. On drift, do not relabel old checks as verification of new bytes. Publish atomically and record the generation/fingerprint/check evidence. A no-change checkpoint need not create another generation.
6. On failure or interruption, keep the original graph and task record. Resume with actual dirty status; do not label the baseline current or erase changes. Rebases and coordinated parent updates create explicit checkpoints rather than silently switching a writer's baseline.

```json
{"task_id":"RAG-S05-T01","baseline_generation":"g17","baseline_fingerprint":"sha256:<original>","live_fingerprint":"sha256:<edited>","freshness":"baseline-with-local-changes","dirty_paths":["frontend/src/api.tsx"],"affected_capabilities":["route.trace"],"publication":"deferred-until-checkpoint"}
```

The journal is a useful hint and recovery aid, not the update's source of truth. Hash-based capture also covers manual user edits, formatter/generator output, deletions, and changes made outside hook events. DOC-STATE acknowledgement remains a separate reviewed action; a graph checkpoint never creates that acknowledgement automatically.

## 5. Technical Choices and Trade-offs

| Topic | Current direction | Remaining validation |
| --- | --- | --- |
| Storage | LadybugDB, isolated from application stores | Native build, supported platform, query behavior, and generation lifecycle |
| Extraction | Mixed native semantics and syntax parsing; Go-first prototype | Exact coverage, dynamic-call limits, and cross-language mappings |
| Retrieval | Exact/lexical plus graph; optional local embeddings | Model/chunk policy and measured benefit over the simpler baseline |
| Agent integration | Codex CLI `0.166.0` and OpenCode `1.19.0`, shared CLI and planned worktree skill | Verify discovery/context delivery and update targets explicitly if future needs change |
| Trigger | Pin a task baseline, track dirty changes, publish at completion/handoff checkpoint | Verify live evidence and compute deltas independently of agent memory |
| Publication | Immutable generations, protected newest three per worktree, 10 GB project-wide cleanup threshold | Linux reader/task pins, atomic publication, crash recovery, and pressure handling |
| Schema evolution | Typed core, disposable rebuilds, hybrid registry later if useful | Handwritten DDL first; query catalog and registry format remain open |
| Build boundary | Portable root facts plus optional production/test modules proposed in Section 12 | Prove module-aware discovery, native setup, and independent verification |
| Concurrent editing | One agent implements this epic; delivered setup supports 2–3 independently writing agents afterward | Shared skill, plan/dependency references, and aggregate resource scheduling |
| Related changes | Shallow stacked PRs with explicit parent commits and handoffs | Restacking and retargeting policy under the repository's merge strategy |
| Resources/platform | Linux only; 1 GB aggregate RAG RAM target | Measure owned-process RSS and serialize heavy work; other platforms are deferred |
| Human language/interface | English knowledge/retrieval and internal versioned Bash/CLI interface | English embedding benchmark; MCP and shared artifact service are deferred |

## 6. Relationship to Existing Tooling

Reuse authoritative repository inventories and command contracts rather than rebuilding competing versions:

- [PROJECT_CATALOG.md](PROJECT_CATALOG.md) supplies declarations, source locations, and comments; retrieval should avoid duplicating its copied prose.
- The import and transaction inventory in [tooling/go/importcheck/](../tooling/go/importcheck/) supplies enforced package boundaries.
- [tooling/go/rawsqlcheck/](../tooling/go/rawsqlcheck/) and [config/sqlc.yaml](../config/sqlc.yaml) identify SQL ownership and generation inputs.
- [tooling/go/testselect/](../tooling/go/testselect/) and the [Makefile](../Makefile) define supported suite selection and commands; test recommendations should preserve these interfaces.
- [DOC-STATE.md](DOC-STATE.md) supplies explicit documentation review dependencies; [AGENTS.md](../AGENTS.md) and applicable repository instructions remain authoritative guidance.
- [backend/something/](../backend/something/) owns configuration language semantics; the graph should consume its capabilities without creating a second language definition.

The catalog answers what exists and where it is described; the graph adds relationships and provenance; semantic retrieval finds related guidance. None becomes a second application system of record. Inferred test relevance is not measured coverage, and graph reachability is not proof that parallel edits are conflict-free.

## 7. Concurrent Agents, Worktrees, and Stacked PRs

### 7.1 Workspace Convention

Use both mechanisms: worktrees isolate concurrent filesystem edits and local output; stacked PRs express the dependency and review order of related changes. Each independently writing task owns one branch and one linked worktree. Read-only reviewers can share a pinned snapshot, provided no writer changes that snapshot while they rely on it.

The primary checkout includes `worktrees/` in Git through [worktrees/.gitkeep](../worktrees/.gitkeep), so fresh checkouts contain the directory. The root [.gitignore](../.gitignore) uses `/worktrees/*` and `!/worktrees/.gitkeep` to ignore every other file and subdirectory. Linked worktrees, their metadata pointers, task records, and generated artifacts stay local; the parent checkout tracks only `.gitkeep` inside this directory.

```text
cogen/                         primary checkout
  .gitignore                   ignores workspace contents except .gitkeep
  worktrees/                   included in Git through the placeholder
    .gitkeep                   tracked; all other contents stay local
    extractor/                 branch agent/code-graph-extractor
      .git                     Git-managed pointer to worktree metadata
      build/                   task-owned output and graph generations
    lookup-cli/                branch agent/code-graph-lookup
    evaluation/                branch agent/code-graph-evaluation
```

Ignoring local workspace contents does not prevent normal source commits from inside each linked worktree: those use that worktree's own index and branch. Such commits still require authorization. Ignore rules concern untracked paths and are not a protection against force-adding or already tracked files; never force-add ignored workspace contents. See Git's [ignore rules](https://git-scm.com/docs/gitignore).

The following commands illustrate the layout and dependency choices; they are not an instruction to create these example tasks now:

```sh
git worktree add -b agent/code-graph-extractor worktrees/extractor main
git worktree add -b agent/code-graph-lookup worktrees/lookup-cli agent/code-graph-extractor
git worktree add -b agent/code-graph-evaluation worktrees/evaluation main
git worktree list --porcelain
git check-ignore -v worktrees/extractor
```

Discover the actual root and Git administrative paths through Git, including `git rev-parse --show-toplevel`, `--git-dir`, and `--git-common-dir`; do not assume `.git` is a directory. Linked worktrees share repository objects and branch references while keeping separate HEAD/index state. Do not force the same branch into multiple writing worktrees. Use Git's [worktree lifecycle commands](https://git-scm.com/docs/git-worktree) for management rather than treating worktrees as ordinary copied folders.

### 7.2 Related Parallel Work and Stacked PRs

For dependent related tasks, branch the child from an explicit committed parent version and target its PR at the parent branch. For example, extractor PR A targets `main`, lookup PR B targets A's branch, and independent evaluation PR C targets `main`. A and B can progress concurrently once the required interface is committed or a clear interface contract allows preparatory work; C can proceed independently.

A child cannot see the parent's uncommitted edits, and later parent commits do not appear automatically in the child. Record the parent commit used by each task. Coordinate interface changes through a handoff: finish or pause the affected writers, agree on the new parent version, deliberately update/restack the child, resolve conflicts, regenerate affected output, invalidate its index, and rerun relevant checks. Never rebase or otherwise rewrite a branch beneath an active agent.

Keep stacks shallow and related to a concrete dependency. Worktrees remain useful for unrelated tasks, which can use independent PRs. A shared epic alone does not establish a code dependency. After a parent merges, inspect the merge result and deliberately retarget/restack descendants; a squash merge changes ancestry and may otherwise make parent changes reappear in a child diff. Validate the final combined state, not just each task in isolation.

One coordinator owns integration, dependency updates, and task assignment; each writing task owns its branch/worktree. Parallel work does not authorize agents to publish PRs, push, merge, or rewrite shared branches. The worktree convention also does not itself authorize spawning additional agents.

### 7.3 Ownership, Resources, and Recovery

The proposed coordinator keeps a small local task record in ignored state, separate from disposable graph generations. It identifies the task/owner, branch, worktree, base and parent commit, dependencies, intended file/package scope, status, relevant validation, and graph generation. Rebuilding an index must not erase task ownership or recovery information. Use simple manual records before adding a scheduling service or workflow database.

File/package scopes help avoid collisions but remain advisory: separate files can still share a contract, generated output, or schema. Give shared interfaces and generation inputs one owner, coordinate handoffs, and use graph impact queries as evidence rather than a guarantee of independence. Git refs and repository-level configuration remain shared, so agents must not independently alter common configuration or perform overlapping integration operations.

Builds, frontend assets, fixture databases, reports, browser evidence, logs, and mutable graph files belong to the owning worktree. Run commands from that root and assign isolated output paths within it when multiple jobs also run inside one worktree. Use operating-system-assigned ports for concurrent servers. Existing immutable download/compiler caches may be shared where their own concurrency rules permit it; never share writable application fixtures or a mutable dirty graph. Bound the number of expensive indexing, build, and browser jobs so parallel agents do not exhaust the machine.

A useful lifecycle is assigned, active, ready for integration, integrated, or interrupted. On interruption, preserve the worktree, source changes, task record, and useful evidence so the task can resume. A coordinator may inspect ownership and stale locks but must not delete another active task's workspace. Cleanup is deliberate after integration and inspection, through Git's worktree commands with the required authorization; no broad recursive deletion, forced removal, or indiscriminate `git clean` is part of startup or recovery.

### 7.4 Planned Shared Codex/OpenCode Worktree Skill

Plan one instruction-only `project-worktrees` skill at `.agents/skills/project-worktrees/SKILL.md`, a repository discovery location documented for both [Codex](https://learn.chatgpt.com/docs/build-skills) and [OpenCode](https://opencode.ai/docs/skills/). Verify discovery and behavior in the targeted versions during RAG-S10-T04. Do not maintain divergent copies for the two hosts. This is a planned skill specification; the current work does not create or install it.

The GitHub Project represents the epic; its stories/tasks and the versioned repository plan describe scope and dependencies. Each agent receives the relevant task, plan path/version, dependency IDs, parent commit, owned worktree/branch, and write scope. The full plan remains available for reference, with only the relevant story/task and dependency contracts loaded for routine work in addition to current mandatory repository instructions. Local task records bind those references to the agent's environment.

```markdown
---
name: project-worktrees
description: Follow this repository's plan, ownership, baseline, and handoff rules when assigned a task in an isolated agent worktree.
---

Read current AGENTS.md and the assigned story/task in the versioned plan. Resolve dependency contracts and record their parent commits before editing.

Confirm the assigned branch/worktree and write scope against Git and the coordinator's task record. Each writing task owns one environment; keep worktrees/.gitkeep and leave other container contents ignored.

Pin the task's original graph generation. Record edits and treat affected graph evidence as baseline/stale; verify it against live source. Planned changes and agent memory are not source facts.

Finish source, generated output, documentation review, and relevant checks before the handoff checkpoint. Capture actual file changes, publish the verified graph generation, and record its fingerprint and check evidence.

Coordinate parent updates and restacks with affected writers paused. Preserve interrupted work, never reclaim by heartbeat age, and follow existing authorization for Git publication or cleanup.

Use Bash/CLI retrieval and the current plan when optional tooling is unavailable. A failed graph refresh does not authorize ignoring source changes, bypassing checks, or rewriting another task's branch.
```

Keep the skill concise and route to the maintained plan instead of copying the epic or whole policies into it. A short future AGENTS/OpenCode routing instruction should require loading it for assigned worktree tasks; implicit host selection alone does not guarantee compliance. Include its files in narrowly scoped Markdown/frontmatter/link checks and input fingerprints without broadening DOC-STATE acknowledgement to arbitrary repository files.

## 8. Evaluation and Rollout

Use representative repository tasks and compare repeated runs on the same starting snapshots, model/agent configuration, permissions, and acceptance criteria. Include navigation, impact analysis, bug fixes, new code, generated-source ownership, and concurrent dirty-worktree scenarios. Correct task completion and regression results take priority over retrieval scores or fewer tokens.

Compare the ordinary documentation/catalog/`rg` workflow, structural plus lexical CLI retrieval, optional embeddings, and advisory hooks. This separates the value of the graph from the value of vectors and automatic injection. Record evidence precision and omissions, source reads, repeated searches, context tokens, wall time, cold/warm and p95 lookup latency, update/rebuild duration, memory/storage, and fallback frequency.

Test missing/corrupt indexes, absent models, partial extraction, stale line numbers, renamed/deleted files, build-configuration changes, missed edit events, overlapping update requests, source changes during indexing, interrupted publication, reader retirement, and rebases. Include two worktrees with different dirty versions of the same path to prove isolation. Adapter tests must prove that the intended context reaches the model and that timeouts preserve ordinary tool behavior.

Existing work informs the design without proving its benefit here: [Aider's repository map](https://aider.chat/docs/repomap.html) demonstrates selecting relevant code structure within a token budget, while [Repoformer](https://proceedings.mlr.press/v235/wu24a.html) studies selective retrieval for repository-level code completion. These support testing compact context and selective retrieval; their results do not establish gains for this repository's full agent workflows.

Roll out an explicit CLI first, then observe which hooks would retrieve without injecting context, then enable small advisory responses on proven events. Keep embeddings optional until they improve the baseline. No automatic PR publication, required graph CI job, or always-running coordinator is implied by this rollout.

## 9. Agreed Policies and Remaining Validation

| Topic | Agreed starting policy | Engineering validation |
| --- | --- | --- |
| Questions | Approve the initial 12 as seed cases; grow a reviewed domain/capability corpus from real tasks and retrieval gaps | Demonstrate promotion of a new non-PDF case, extractor/query support, and regression coverage |
| Agents | Target Codex CLI `0.166.0` and OpenCode `1.19.0`; change targets deliberately later | Verify events, model-visible delivery, skill discovery, and failure behavior on those releases |
| RAM | 1 GB aggregate target for RAG-owned processes serving 2–3 agents | Measure native RSS/peaks, apply admission control, and serialize indexing/model work where needed |
| Retention | Keep newest three successful graph publications per worktree; collect eligible older ones at 10 GB project-wide generation storage toward 8 GB | Protect task baselines/readers, verify Linux lock ordering and crash recovery, and report pressure if protected data exceeds the target |
| Updates | Preserve the original task graph during edits; publish one verified checkpoint after work is ready for handoff | Compare actual source with the baseline, catch unrecorded changes, and invalidate affected consumers |
| Interface | Internal versioned CLI and JSON catalog | Preserve bounded typed parameters and explicit compatibility failures |
| Platform | Linux only initially | Prove the module/scanner/test/coverage split and native build on the development architecture |
| Human language | English only; local optional embeddings | Benchmark a compact English model/chunker within the aggregate RAM budget |
| Concurrency | One implementation agent now; enable 2–3 agents with the shared skill and versioned plan/dependency references after acceptance | Use controlled process/worktree fixtures to test independence, handoffs, restacking, interruption, and fallback |
| Integration scope | Bash tools and project-specific knowledge first; no MCP now | Measure retrieval usefulness before adding protocol or shared-cache services |

Remaining technical work is to pin a compatible Ladybug/binding/toolchain combination, prove safe bounded native cancellation and offline extension reopening, measure rebuild/query/memory/storage costs, validate retention races, define exact versioned parameter contracts, and compare English embedding candidates against lexical/graph retrieval. These are trial/acceptance tasks, not missing product decisions. The starting policies above remain in force until evidence and an explicit plan revision justify changing them.

## 10. Next Steps

1. Collect real queries and acceptance tasks, map them onto the proposed schema, and define evidence and incomplete-result expectations.
2. Prototype Go extraction to JSON with native semantic resolution, source hashes, build context, and explicit coverage gaps.
3. Prove Ladybug's isolated native build, representative traversal queries, and immutable-generation concurrency protocol.
4. Implement an explicit catalog-driven CLI, fingerprint/no-op checks, and correct full rebuilds before optimizing updates.
5. Specify the shared worktree skill and exercise 2–3 agents' plan/ownership/checkpoint and shallow-stack workflow with isolated output before broad lifecycle automation.
6. Add one small agent adapter, verify actual context delivery and failure behavior, and then add the other adapter against the same response contract.
7. Measure task correctness, latency, retrieval noise, and context use against ordinary repository navigation before expanding automatic hooks.
8. Add optional local embeddings and incremental updates only when measured results justify them.
9. Introduce registry generation, shared immutable caches, or broader lifecycle automation only for a demonstrated need; MCP and other platforms/languages remain outside the current scope.

These are implementation proposals, not authorization to spawn tasks, create example branches, add native dependencies, publish changes, or modify application databases. The current repository change establishes the local worktree convention and records the design.

## 11. Decision Log

| Date | Topic | Current decision | Rationale |
| --- | --- | --- | --- |
| 2026-11-01 | Language scope | Go, TypeScript, SQL, SOMETHING, Makefile, JSON, YAML; selected Markdown for retrieval | Covers repository code/configuration and the guidance agents need. |
| 2026-11-01 | Storage | LadybugDB in separate developer tooling | Graph traversal and planned full-text/vector retrieval; native setup is kept out of application builds. |
| 2026-11-01 | Embeddings | Optional and local only | Preserve explicit provisioning and offline ordinary commands. |
| 2026-11-01 | Schema evolution | Typed core and disposable rebuilds; hybrid registry remains proposed | Keep facts, questions, and schema generation separate. |
| 2026-11-02 | Agent workflow | Shared CLI, explicit update, and selective advisory hooks for Codex and OpenCode | Reduce repeated navigation while preserving normal agent tools and permissions. |
| 2026-11-02 | Evidence and freshness | Source hashes, provenance, coverage gaps, and bounded responses | Avoid presenting partial, inferred, or stale results as complete facts. |
| 2026-11-02 | Database concurrency | Immutable generations proposed; one publisher per worktree | Respect Ladybug's process ownership contract without requiring a daemon. |
| 2026-11-02 | Concurrent editing | Owned linked worktrees under `worktrees/`, kept in Git through `.gitkeep`; all other contents ignored | Isolate source edits and local artifacts while sharing repository history and preserving the directory in fresh checkouts. |
| 2026-11-02 | Related parallel work | Stacked PRs with explicit parents and coordinated handoffs | Represent dependencies and review order alongside filesystem isolation. |
| 2026-11-02 | Evaluation | CLI baseline, then observed hooks, then bounded injection; embeddings evaluated separately | Demonstrate task benefit before adding automation or retrieval cost. |
| 2026-11-02 | Implementation boundary | Portable owning exports plus optional semantic/native and external test modules proposed | Reuse existing policy without native prerequisites leaking into root checks. |
| 2026-11-02 | Documentation simplification | Collect once, render/validate/export the same snapshot, preserve explicit review acknowledgement | Remove duplicate work while retaining deterministic docs and human review. |
| 2026-11-02 | Publication/update lifecycle | Pin original task graph, track dirty changes, publish a verified completion/handoff checkpoint | Preserve a stable baseline while deriving updates from actual source rather than agent memory. |
| 2026-11-02 | Retention | Newest three successful generations per worktree protected; collect eligible older ones at 10 GB toward 8 GB across project generation storage | Bound growth without deleting active readers, task baselines, or the protected minimum. |
| 2026-11-02 | Targets/resources | Linux, Codex `0.166.0`, OpenCode `1.19.0`, 2–3 agents, 1 GB aggregate RAG RAM | Start with the current environment and a small measured resource envelope. |
| 2026-11-02 | Knowledge/interface scope | English knowledge, internal Bash/CLI interface, evolving reviewed question corpus | Improve repository-specific retrieval first; no MCP or multilingual work now. |
| 2026-11-02 | Worktree skill | Plan one shared Codex/OpenCode skill with plan/task/dependency references | Establish common rules for independent environments; implementation remains a later task. |
| 2026-11-02 | Implementation staffing | One agent executes the epic; parallel agent operation begins after the setup is accepted | Preserve technical dependencies and concurrency tests without requiring multiple implementation agents now. |

## 12. Technical Implementation Evaluation

### 12.1 Repository Findings and Simplification Opportunities

| Confirmed repository behavior | Implementation consequence | Recommended simplification |
| --- | --- | --- |
| [projectlayout.WalkGo](../internal/projectlayout/layout.go) walks `backend/`, `tooling/go/`, `internal/`, and `tests/`; it does not currently stop at nested `go.mod` files. | A nested module can still be read by repository scanners even though root Go package patterns exclude it. | Make module ownership explicit; do not manufacture root-module import paths for optional-module files. |
| [testselect.Select](../tooling/go/testselect/select.go) discovers suites under root `tests/` and invokes Go from the root module. | A native test module placed under `tests/` would otherwise enter ordinary suite discovery incorrectly. | Skip nested modules by default and add one explicit optional-module selection path. |
| [checkCatalog](../tooling/go/doccheck/catalog.go) renders declarations and then `checkCatalogDescriptions` collects them again. | Both Go and TypeScript/JavaScript syntax collections repeat within one check. | Collect once per command and feed the same immutable snapshot to rendering, comment validation, and JSON export. |
| [javascript_catalog.go](../tooling/go/doccheck/javascript_catalog.go) performs a Go-hosted syntax scan without running Node. | Replacing it with the TypeScript compiler would add a runtime prerequisite to ordinary documentation checks. | Preserve the portable catalog extractor; put richer TypeScript resolution in an optional enrichment step. |
| The [catalog introduction](PROJECT_CATALOG.md) says Go scans `src/` and JavaScript scans only `frontend/`, while actual roots include backend, tooling, internal, and tests. | The current prose understates coverage and contains an obsolete root. | Correct the owning introduction when the shared inventory is implemented; do not rename every generated heading merely for terminology. |
| [importcheck](../tooling/go/importcheck/main.go), [rawsqlcheck](../tooling/go/rawsqlcheck/ownership/analysis.go), and `testselect` already own rules and classifications. | Parsing their human-readable reports or recreating their rules would create competing authorities. | Expose narrow versioned data from their owning packages; keep enforcement in the existing checkers. |
| [documentationFiles](../tooling/go/doccheck/state.go) hashes every maintained regular file under `docs/`, excluding `DOC-STATE.md` and `docs/ref/`. | Putting generated index manifests or snapshots in `docs/` would create unnecessary acknowledgement churn. | Keep ephemeral JSON, indexes, reports, and task state in ignored output; commit only contracts, source, fixtures, and reviewed prose. |
| `AGENTS.md` currently requests several large documents before task-specific work; the catalog is a full declaration listing. | An agent may spend substantial context before its first relevant lookup. | Evaluate a short mandatory instruction core plus a task/path reading matrix after retrieval proves reliable; preserve a static fallback and every substantive rule. |

The first useful cleanup is eliminating duplicate collection and rule copies. Removing the catalog, documentation acknowledgement, ownership checks, or existing source comments would remove guarantees that a derived graph does not replace. Any later retirement needs a named replacement, equivalent coverage, reviewed callers, and an explicit migration task.

### 12.2 Recommended Build and Ownership Boundary

Prefer a portable root-module fact exporter and an optional semantic/native module. The portable layer uses existing root dependencies and does not import Ladybug or require Node; it is CGO-free, not necessarily transitively standard-library-only. The optional module consumes a versioned JSON boundary and owns `go/packages`, native database setup, semantic resolution, and query execution. Do not import private `internal/sql` or other handwritten private implementation APIs across that boundary.

```text
root Go module: analysis
  internal/projectlayout/                   source/module discovery policy
  tooling/go/codefacts/                     portable facts and workspace identity
  tooling/go/cmd/code-facts/main.go          thin JSON-export command
  tooling/go/doccheck/                      Markdown/check/state ownership
  tests/tooling/go/codefacts/{unit,functional}/

optional production module:
  tooling/agentgraph/go.mod                 module analysis/tooling/agentgraph
  tooling/agentgraph/cmd/code-graph/main.go  thin CLI
  tooling/agentgraph/{extract,store,query,hook,workspace}/

optional external test module:
  tests/tooling/agentgraph/go.mod           module analysis/tests/tooling/agentgraph
  tests/tooling/agentgraph/{unit,functional,integration}/
  tests/tooling/agentgraph/testdata/         synthetic sources and expected facts

host-specific TypeScript:
  tooling/node/agent-workflow/              bounded OpenCode subprocess client
  .opencode/tools/code-graph.ts             thin explicit-tool registration
  tests/tooling/node/agent-workflow/{unit,functional}/

committed contracts:
  config/codegraph/{questions,queries,policy}.json
                                             question corpus, query catalog, and policy
  .agents/skills/project-worktrees/SKILL.md   planned shared host-discovered skill

local output:
  build/code-facts                          portable binary
  build/code-graph                          optional native binary
  build/codegraph/{current.json,generations/,locks/,reports/}
  worktrees/.gitkeep                       tracked placeholder
  worktrees/.state/tasks/                  ignored coordination records
```

This is a proposed file map, not permission to create empty packages. Start with cohesive files and split only for actual responsibilities. There is no root `go.work` and no root dependency on the optional module. The Go [module model](https://go.dev/ref/mod#module-path) distinguishes module/package identity; source scanning, formatting, catalog coverage, test selection, and coverage policy must also respect those identities explicitly.

The optional test module can require `analysis/tooling/agentgraph v0.0.0` with `replace analysis/tooling/agentgraph => ../../../tooling/agentgraph`. Add a narrowly scoped `--go-module tests/tooling/agentgraph` selector mode and an explicit `make test-code-graph` wrapper. Ordinary selections skip this module; explicit selection derives package arguments relative to that module and retains strict empty-selection and unmatched-test failures. Root formatting and source-comment checks can inspect optional source syntactically without loading CGO. Native vet/race/coverage stay explicit and must not disappear from the plan merely because root checks skip the module.

### 12.3 One Source Snapshot, Several Consumers

```text
eligible source bytes + owning checker inventories + review dependency metadata
                       |
                 portable fact snapshot
                  /             \
     deterministic catalog       JSON export under build/
     and comment validation             |
                                  optional resolvers
                                         |
                               graph candidate generation
                                         |
                              validated immutable publication
```

Keep the portable syntax snapshot distinct from build-selected semantic enrichment. A catalog can list an inactive build-tag file while the Go call graph resolves only the selected build profile. Both are useful, but they must not claim the same completeness. Share file identity, locations, comments, and classification; do not force all tools into one generic AST or replace domain rules with graph queries.

Keep the import direction acyclic: doccheck owns reusable catalog/review snapshots, existing checker packages own their inventories, and code-facts aggregates those exports. Doccheck must not import the aggregator that already imports it. The native module consumes JSON rather than checker internals, so changing storage does not change the portable documentation contract.

Read eligible bytes once for a snapshot, hash those bytes, and ensure each extractor consumes that version. For Go semantics, use a controlled source snapshot or `packages.Config.Overlay` for captured files and a recorded build environment. A before/after timestamp or HEAD check alone does not prevent mixed-version facts if a file changes while parsers reread it. Recheck identity before publication and report current disk differences as stale rather than relabelling a finished generation.

### 12.4 Documentation State and Cleanup Boundaries

| State | Authority | What the graph may do |
| --- | --- | --- |
| Source declaration/comment facts | Source bytes and portable extraction | Import with hashes and provenance. |
| Generated Markdown catalog | Pure rendering of current source facts | Link to it, deduplicate copied prose, and avoid embedding it as another independent source comment. |
| Acknowledged documentation hashes | Human review recorded by `docs-state-update` | Report changed documents and declared review dependents; never acknowledge them automatically. |
| Graph/index freshness | Source/build/tool fingerprints and generation manifest | Refresh disposable local output without changing tracked acknowledgements. |
| Agent task ownership | Local coordinator record and Git worktree metadata | Explain ownership/dependencies; never infer permission to reclaim a workspace. |

There is no generation cycle if the graph reads documentation state but never writes it, and catalog rendering never reads graph output. The catalog must not contain graph generation IDs, timestamps, model text, or inferred descriptions. Index acknowledgement metadata as structured evidence; exclude generated hash-table prose from semantic ranking to reduce noise.

A later instruction cleanup may replace blanket catalog loading with focused lookup and a reading matrix such as `database changes -> DATABASE + owning schema + STANDARDS`, `frontend changes -> DESIGN + JSX/CSS/style + affected usage`, and `docs-only changes -> STANDARDS + DOC-STATE + changed dependencies`. This requires an explicit `AGENTS.md`/OpenCode policy revision with equivalence tests and a no-index fallback. A hook cannot silently narrow today's required reading.

### 12.5 Native Feasibility, Resources, and Extensions

The inspected [Go binding configuration](https://github.com/LadybugDB/go-ladybug/blob/master/database.go) documents an 80%-of-system-memory default buffer pool and a CPU-count default thread limit. Multiple independent CLI readers must use explicit per-process budgets. Its [connection API](https://github.com/LadybugDB/go-ladybug/blob/master/connection.go) exposes `SetTimeout` and `Interrupt` rather than a Go context parameter. Prove cancellation and safe handle closure with the selected pinned engine/binding pair; a blocking C call is not bounded merely because its caller has a context.

```go
// Illustrative settings; the trial must verify supported limits and API versions.
cfg := lbug.DefaultSystemConfig()
cfg.ReadOnly = true
cfg.BufferPoolSize = 64 << 20
cfg.MaxNumThreads = 1
db, err := lbug.OpenDatabase(generationPath, cfg)
// On success: open one request connection, set a finite millisecond timeout,
// interrupt on cancellation, wait for query completion, then close in order.
```

The 64 MiB pool is an experiment, not a process RSS limit. Measure native allocations and total concurrent cost. If interruption cannot safely finish within the required deadline, isolate queries in a killable worker rather than closing a connection still in use. Pin engine, binding, extension, compiler/platform, artifact checksums, and licenses after the feasibility trial; upstream `master` links are inspected API references, not dependency pins.

Full-text and vector search are [Ladybug extensions](https://docs.ladybugdb.com/extensions/). Prove explicit offline provisioning, index creation, finalization, and reopening from independent read-only processes before relying on them. If a selected extension cannot operate within immutable generations, keep exact/portable lexical retrieval available and revise that part of the plan before implementation. No hook runs an extension installer or model download.

### 12.6 Proposed Initial Budgets and Failure Contract

| Operation | Initial experimental target | Required behavior when unavailable or exceeded |
| --- | --- | --- |
| Warm status/fingerprint lookup | p95 at most 150 ms on a recorded developer machine | Report unavailable/stale without a hidden full rebuild. |
| Warm bounded graph query | p95 at most 500 ms | Return an explicit timeout/error, not successful empty evidence. |
| Advisory hook subprocess | Hard deadline at most 2 seconds | Preserve normal agent tool behavior and release resources. |
| Injected context | Approximate 800-token target/host spill threshold plus an enforced hard byte cap | Truncate at item boundaries and expose omitted results; claim an exact token ceiling only if a verified tokenizer enforces it. |
| Default response/traversal | At most 20 evidence items and 3 hops | Offer explicit expansion through a catalog query. |
| Rebuild/model startup | Measured separately at explicit checkpoints/setup | Keep them outside the pre-tool critical path and serialize heavy work within the aggregate RAM budget. |
| Aggregate RAG memory | 1 GB across owned readers, metadata processes, indexer, and any embedding worker serving 2–3 agents | Measure total resident/native memory, bound concurrency, and disable/defer optional work if the budget cannot be met. |

The timing/context numbers are hypotheses to benchmark, not current measurements or promised performance; 1 GB is the agreed aggregate memory target to validate. Correctness gates remain strict: no dirty-worktree leakage, no stale result labelled current, no unsupported extraction labelled complete, no hidden downloads, bounded output, and no hook approval/input mutation. Distinguish CLI statuses such as `ok`, `stale`, `baseline-with-local-changes`, `unavailable`, `partial`, and `error` from host-hook exit behavior; an advisory host failure can continue the agent while still reporting lookup failure honestly.

### 12.7 Resource Accounting and Retention Protocol

Treat the 1 GB RAM target as an aggregate RAG-tool budget, excluding Codex/OpenCode clients, browsers, application processes, and ordinary builds. Count owned native readers, portable capture/export processes, the checkpoint indexer, and any local embedding worker/model. Measure native/process RSS and concurrent peaks; `GOMEMLIMIT` and Ladybug buffer-pool settings alone do not cap C/C++ memory. Start with 2–3 agents and one heavy indexing/model job at a time. Reuse one embedding worker or run it sequentially; do not start one model per agent. Report evidence before proposing a higher budget.

The generation-storage threshold is project-wide across the registered worktrees, not 10 GB independently for each agent. Count complete generation artifacts and private candidate generations; separately report model/native installation artifacts and reports rather than silently treating unrelated `build/` content as disposable graph storage. A run here means a successfully published RAG graph generation, never an application `pipeline_runs` row or research database. No retention action touches source, task records, corpus/SQLite data, or arbitrary build output.

```json
{"format_version":1,"targets":{"os":"linux","codex_cli":"0.166.0","opencode":"1.19.0","human_language":"en","writing_agents":3},"memory":{"aggregate_target_bytes":1000000000,"heavy_jobs":1},"retention":{"minimum_successful_generations_per_worktree":3,"project_high_water_bytes":10000000000,"project_low_water_bytes":8000000000,"protect_reader_pins":true,"protect_task_baselines":true},"updates":{"mode":"task-checkpoint","verify_actual_delta":true},"interface":{"scope":"internal","transport":"cli","mcp":false}}
```

The policy example uses decimal GB explicitly. For a worktree with generations `g1` through `g10`, the protected newest three are `g10`, `g9`, and `g8`. Older ones may stay below the threshold. At 10 GB, remove eligible older generations oldest first toward 8 GB to avoid repeatedly collecting a tiny amount. A reader or active/interrupted task can also protect an older generation such as `g2`; its age does not make it eligible.

1. Use one project lifecycle/retention lock to coordinate generation selection, task-pin changes, publication metadata, and retirement decisions across the registered worktrees. Keep heavy candidate construction outside this short critical section and retain per-worktree publisher ownership.
2. A reader acquires the lifecycle lock, selects a validated generation, obtains a shared OS-backed lock on its persistent per-generation pin file keyed by worktree and generation identity, then releases the lifecycle lock. Hold the pin until all database handles are closed. Generation IDs are never reused; do not unlink/recreate a pin file in a way that lets readers lock different inodes for the same generation.
3. The collector acquires the same lifecycle lock, computes each worktree's newest-three set and durable task-baseline pins, skips active candidates/current publications, and attempts a nonblocking exclusive pin lock on each otherwise eligible older generation. Only proven unpinned tool-owned generations may be retired. The same lock ordering prevents a reader from selecting a generation while it is being deleted.
4. Reader locks release when a process exits, but task-baseline pins remain in durable task records across interruption. Release or move those pins only through a coordinated checkpoint/handoff/integration transition; do not reclaim them using heartbeat age or PID absence.
5. If protected generations exceed the target or active readers prevent sufficient cleanup, report `storage-pressure` with protected/eligible byte counts and defer further heavy updates where necessary. Preserve the newest three and pins. The 10 GB threshold triggers collection; it cannot also be an unconditional hard cap when the protected minimum alone is larger. Never silently delete a protected snapshot to claim compliance.

Candidate-build admission and progress monitoring account for temporary disk use, with safe cancellation/owned-candidate cleanup on pressure. Filesystem usage accounting, lock ordering, crash recovery, pin lifetime, and simultaneous publishers/readers must be proven on Linux before enabling the collector. Per-worktree owner verification and path containment prevent a global collector from treating another directory's contents as its own cache.

Task-pin records change atomically through the coordinated ownership API, including when a human performs the manual workflow; direct unsynchronized JSON edits cannot be the basis of safe collection. Malformed or unknown ownership/pin metadata makes the collector skip the affected scope and report the problem, rather than guessing that no task uses an older generation.

## 13. Epic RAG-01: Implementation Stories and Tasks

### 13.1 Epic Contract and Backlog Map

**Outcome:** A developer using Codex or OpenCode can obtain bounded, verifiable context for a repository task, keep it aligned with dirty source in the owning worktree, and coordinate related parallel work without duplicating documentation or enforcement systems. **Delivery boundary:** the backlog covers the tool and its local workflow, not application behavior or research data. **Status:** all tasks below are planned; this evaluation does not mark them implemented.

**Epic acceptance:** Portable checks run without native/model/agent prerequisites; explicit retrieval answers the approved seed cases and demonstrates adding a new domain question with provenance and honest gaps; native publication/retention is safe under concurrent processes; targeted Codex/OpenCode releases have proven retrieval and shared-skill discovery; hooks are bounded; 2–3 agents use isolated worktrees and task checkpoints within the measured 1 GB aggregate RAG target; newest-three and reader/task protections survive 10 GB storage pressure; cleanup removes only demonstrated duplication; evaluation records correctness and cost before enabling optional features.

| Story | Description | Depends on | Delivery gate |
| --- | --- | --- | --- |
| `RAG-S01` | Prove native capabilities and freeze the first query contracts | None | Feasibility and baseline fixtures |
| `RAG-S02` | Establish portable facts and optional-module ownership | S01 contracts | Root/native isolation |
| `RAG-S03` | Simplify catalog collection and expose review state safely | S02 | Documentation parity |
| `RAG-S04` | Resolve Go declarations and calls with evidence | S01, S02 | First semantic vertical slice |
| `RAG-S05` | Connect TypeScript, routes, SQL, configuration, and guidance | S02, S04 contract | Repository workflow coverage |
| `RAG-S06` | Publish task checkpoints and retain immutable graph generations safely | S01, S02, S04 | Process/concurrency and retention correctness |
| `RAG-S07` | Deliver bounded retrieval and evolve the question/capability corpus | S03, S04, S06 | Explicit CLI MVP and growth path |
| `RAG-S08` | Add dependency-safe incremental updates where justified | S04, S06, S07 measurements | Optional optimization |
| `RAG-S09` | Integrate Codex and OpenCode through thin adapters | S07 | Proven model-visible retrieval |
| `RAG-S10` | Coordinate 2–3 worktrees through one Codex/OpenCode skill and shared plan | S02; S07 for graph status | Concurrent workflow readiness |
| `RAG-S11` | Add optional local embeddings and evaluated rank fusion | S05, S07, S12 baseline | Optional semantic improvement |
| `RAG-S12` | Run acceptance evaluation, simplify bootstrap, and finish docs | S03–S07, S09, S10; rerun for S08/S11 | Reviewed release and cleanup |

S12's baseline task can start after S07 and before S11; S12's final acceptance waits for whichever optional stories are selected. This avoids a circular dependency between evaluating embeddings and completing the release. Stories are not all sequential: fixtures and metadata contracts unlock independent extraction, documentation, and publication work. Shared contracts, source scopes, and parent commits govern parallelization, as detailed in Section 15.

**Task format:** Each task below specifies objective/changes, files, an example, expected behavior/acceptance, and verification. Examples are deliberately small interface or failure cases, not complete implementations. Paths that do not yet exist are proposed. Every command in the backlog is a planned verification command; actual results belong to execution evidence, never to an unchecked planning box.

### RAG-S01: Prove Capabilities and Define the First Acceptance Queries

**Story:** As a maintainer, I need evidence that the chosen engine and query contract work on the supported machine before building adapters around them. **Technical target:** a pinned, bounded native trial and synthetic acceptance fixtures. **Dependencies:** none. **Acceptance:** supported versions/platforms, offline setup, resource limits, query results, and failure behavior have recorded outcomes; a failed gate revises downstream tasks rather than changing storage silently.

#### RAG-S01-T01: Define Representative Questions and Golden Evidence

**Objective and changes:** Implement the approved 12 seed questions in Section 1.1 across callers, impact, ownership, tests, routes, configuration, guidance, and task coordination. Pair each with a small synthetic fixture and expected resolved, inferred, unresolved, and empty results. Capture the ordinary catalog/`rg` baseline without adding an engine dependency. These cases start a growing corpus; they are not an exhaustive project boundary.

**Files:** proposed canonical `config/codegraph/questions.json`, fixtures under `tests/tooling/agentgraph/testdata/{repository,expected}/`, and capability references; inspect existing [PDF routes](../backend/server/server.go) and [test selector](../tooling/go/testselect/select.go). Do not maintain a divergent second question list in testdata.

```json
{"id":"go-direct-callers","query":"symbol.context","parameters":{"symbol":"go:fixture/store#(*Store).Save","direction":"callers"},"expect":{"resolved_callers":1,"unresolved_sites":1,"coverage":"partial"}}
```

**Expected result and acceptance:** The fixture distinguishes a direct call from an interface call whose concrete target is unknown. Empty results state scope and completeness. Synthetic IDs/hashes are not presented as repository measurements. **Verification:** fixture-schema validation and manually reviewed baseline answers; S04/S07 later run the same expected cases through extraction and queries.

#### RAG-S01-T02: Pin and Prove the Native Build and Offline Extensions

**Objective and changes:** Select a mutually compatible Ladybug engine/binding version and Go 1.25-compatible `x/tools` for Linux on the development architecture. Keep Codex CLI `0.166.0` and OpenCode `1.19.0` as the host targets; changes require explicit plan/compatibility revision. Record native ABI/compiler, artifact checksums, license notices, setup prerequisites, and exact extension versions. Prove create/load/query/close/reopen with no network after explicit setup. Pin native dependencies only after this trial succeeds.

**Files:** an isolated trial module under ignored `build/codegraph/trial/`, tracked synthetic inputs under `tests/tooling/agentgraph/testdata/`, and reviewed version/license/build findings. S02 promotes the proven module/command/test structure into maintained paths; do not add a discoverable optional test module before its selector isolation exists. No root native dependency or automatic downloader.

```sh
# After explicit provisioning of the trial's pinned native/module prerequisites:
GOWORK=off GOPROXY=off GOSUMDB=off go -C build/codegraph/trial test ./...
# S02 later provides supported setup/build/test targets for maintained modules.
```

**Expected result and acceptance:** One-hop prepared queries work, completed files reopen independently read-only, and optional FTS/vector provisioning is installation-free during lookup. Missing setup produces an actionable error. Root `go.mod`/`go.sum` remain independent. **Verification:** record actual engine/binding/platform/build flags, setup access, and offline test outcomes; unavailable optional extensions remain explicitly unavailable.

#### RAG-S01-T03: Prove Cancellation and Concurrent Resource Bounds

**Objective and changes:** Exercise finite pool/thread limits, per-query timeouts, cancellation via `Interrupt`, handle cleanup, and 2–3 independent readers sharing the 1 GB aggregate RAG target. Measure capture/export, readers, one heavy index/model job, and native peaks rather than counting only configured buffers. Include an expensive traversal and interrupted candidate build. Use a killable worker only if the targeted binding cannot meet the cancellation contract safely; optional work queues or disables itself before requesting more RAM.

**Files:** initial resource/cancellation probes in the isolated trial and reports under `build/codegraph/reports/`; promote into `tooling/agentgraph/store/connection.go` and `tests/tooling/agentgraph/integration/resources_test.go` with S02's module isolation.

```go
conn.SetTimeout(200) // Example milliseconds inside a larger CLI deadline.
// Cancellation must interrupt and join the active query before closing conn/db.
// A timer firing in Go alone does not stop native execution.
```

**Expected result and acceptance:** Cancellation releases every owned handle; no connection closes while native work or its cancellation callback still uses it. Measured concurrent RSS/CPU is recorded separately from configured buffer sizes. **Verification:** native integration and race suites, repeated interruption, and process-exit checks; retain failure evidence rather than treating a skipped native run as passed.

### RAG-S02: Establish Portable Facts and Explicit Module Ownership

**Story:** As a developer, I need shared repository facts and optional native tooling without adding prerequisites to normal builds and checks. **Technical target:** an acyclic portable JSON bridge, two explicitly owned optional modules, and strict suite selection. **Dependencies:** S01's contract and build decisions. **Acceptance:** ordinary root commands remain independent of CGO/agents/models, while native source and tests have their own maintained checks.

#### RAG-S02-T01: Add the Portable Fact Envelope and Owning Exports

**Objective and changes:** Define a versioned deterministic envelope containing workspace identity, eligible file hashes, syntax declarations, imports, SQL ownership/generation mappings, suite inventory, and review metadata. Reuse the owning analysis behind APIs such as `ownership.Analyze` and `testselect.Select`, but add export paths that accept the captured inventory/bytes through a read-only source interface such as `fs.ReadFileFS`, or a proven frozen source root. Calling today's root-path APIs independently after hashing would reread live files and is insufficient. Expose narrow read-only DTOs for private catalog/import/state data. Pure JSON goes to stdout, diagnostics to stderr.

**Files:** proposed `tooling/go/codefacts/{snapshot,export}.go`, thin `tooling/go/cmd/code-facts/main.go`, and narrow exports in existing doccheck/importcheck/rawsqlcheck/testselect packages; ordinary tests under `tests/tooling/go/codefacts/`.

```json
{"format_version":1,"producer":"code-facts","workspace":{"repository_id":"<local-id>","worktree_id":"<git-admin-id>","input_fingerprint":"sha256:<digest>"},"declarations":[{"module":"analysis","path":"backend/pdfstore/mutation.go","name":"(*Store).Save","start_line":103,"source_hash":"sha256:<digest>","evidence":"syntactic"}]}
```

**Expected result and acceptance:** Same captured bytes and policy produce deterministically ordered facts, even if the live tree changes during owner extraction; publication later reports any live-tree mismatch. `doccheck` owns the reusable catalog snapshot; `codefacts` imports the owner packages, and no owner imports `codefacts`, avoiding a dependency cycle. It never scrapes Markdown or human reports. **Verification:** ordinary unit/functional tests for stable output, concurrent edits between owner scans, versions, exclusions, escaped paths, and missing comments; existing checker tests remain unchanged in meaning.

#### RAG-S02-T02: Make Discovery and Suite Selection Module-Aware

**Objective and changes:** Stop root Go/import/test discovery at nested `go.mod` boundaries. Explicitly declare the optional production and test roots for syntax/catalog coverage, storing their actual module identities. Add the narrowly allowed `--go-module tests/tooling/agentgraph` selection mode; reject arbitrary directories or traversal and run selected Go packages from that module.

**Files:** [internal/projectlayout/layout.go](../internal/projectlayout/layout.go), [testselect/select.go](../tooling/go/testselect/select.go), importcheck's root discovery, proposed optional `go.mod` files, and regression fixtures in the existing selector/importcheck suites.

```go
module analysis/tests/tooling/agentgraph

go 1.25.0

require analysis/tooling/agentgraph v0.0.0

replace analysis/tooling/agentgraph => ../../../tooling/agentgraph
```

**Expected result and acceptance:** Default `make test` does not discover native tests. Explicit selection keeps repository-relative evidence paths but invokes package arguments relative to the chosen module. Unknown modules, empty selections, and unmatched Go test expressions fail. No root `go.work` changes module behavior. **Verification:** `make test-go DOMAIN=tooling COMPONENT=go/testselect`, importcheck tests, synthetic nested-module cases, and offline ordinary/native selection smoke runs.

#### RAG-S02-T03: Provide Separate Build and Verification Commands

**Objective and changes:** Add explicit code-facts and code-graph build targets, optional native test/vet/race/coverage targets, and native syntax-format coverage. Keep native targets out of root `build`, `tools`, `test`, and `check`. Include portable additions in normal coverage ownership; give optional code a separate reported profile rather than hiding it with broad exclusions.

**Files:** [Makefile](../Makefile), [coverage policy](../config/coverage_policy.something), optional module checks, and future command documentation.

```make
# Proposed sketch; add .PHONY/help, selector validation, and pinned build flags.
code-graph-build:
	mkdir -p build
	GOWORK=off go -C tooling/agentgraph build -o ../../build/code-graph ./cmd/code-graph
```

**Expected result and acceptance:** Every binary/report belongs to the current worktree's `build/`; setup is separate from build and never implicit. Root checks work with native libraries, models, and agent installations absent. **Verification:** ordinary build/tools/check/test discovery plus explicit native format/vet/test/race/coverage commands; inspect root dependency diff and coverage inventories.

### RAG-S03: Simplify Documentation Collection and Expose Review State

**Story:** As a maintainer, I need a single source-derived catalog and useful review guidance without automatic acknowledgement. **Technical target:** collect-once rendering/export, read-only state reporting, and a stable generation DAG. **Dependencies:** S02. **Acceptance:** documentation guarantees remain equivalent and all generated output is reproducible without the graph engine or Node.

#### RAG-S03-T01: Collect Catalog Entries Once per Command

**Objective and changes:** Refactor catalog collection into a typed immutable snapshot consumed by Markdown rendering, missing-description validation, and export. Preserve existing scope/filter rules and source comments. Correct the stale catalog introduction to describe actual Go and TypeScript/JavaScript roots; expose discovery scope so prose does not invent a second inventory.

**Files:** [catalog.go](../tooling/go/doccheck/catalog.go), [javascript_catalog.go](../tooling/go/doccheck/javascript_catalog.go), proposed `doccheck/snapshot.go`, catalog introduction, and existing catalog functional fixtures.

```go
// Illustrative owner API: one collection, three read-only consumers.
snapshot, err := CollectCatalog(root)
// RenderCatalog(snapshot); ValidateDescriptions(snapshot); ExportCatalog(snapshot)
// No consumer recollects files or depends on a graph generation.
```

**Expected result and acceptance:** Unchanged fixture source yields identical rows/descriptions; no inferred comments appear. Optional module declarations are documented syntactically without CGO compilation, while generated `internal/sql` exclusions remain intentional. **Verification:** `make test-docs`, portable export parity fixtures, `make docs-catalog-update`, reviewed generated diff, and `make check-docs`; measure collection work rather than adding a permanent cache.

#### RAG-S03-T02: Report Review Dependencies Without Writing Acknowledgements

**Objective and changes:** Expose current/acknowledged document hashes and manually declared review dependents as structured data. Graph status may recommend review while remaining fresh for unacknowledged bytes. Keep `state update` an explicit post-review operation, and ensure hooks/queries have no code path that invokes it.

**Files:** [doccheck/state.go](../tooling/go/doccheck/state.go), code-facts aggregation, state functional tests, and the dependency guidance in [DOC-STATE.md](DOC-STATE.md).

```json
{"path":"docs/ARCHITECTURE.md","current_hash":"sha256:<current>","acknowledged_hash":"sha256:<reviewed>","review_state":"changed","review_dependents":["docs/DATABASE.md","docs/STANDARDS.md"]}
```

**Expected result and acceptance:** Reporting does not mutate any tracked bytes. Graph output stays in `build/`; catalogue/state generation never reads it. Generated hash-table text does not crowd embeddings. **Verification:** read-only report functional tests comparing files before/after; state mismatch, missing dependency, and generated-output cycle fixtures; `make test-docs` and reviewed state updates only.

### RAG-S04: Extract Go Semantics with Explicit Resolution Limits

**Story:** As an agent, I need callers and impact evidence tied to actual Go symbols and build context. **Technical target:** native package/type loading, deterministic symbol IDs, and call-site evidence over captured source. **Dependencies:** S01 and S02. **Acceptance:** direct-call fixtures are exact, uncertainty is visible, and package errors never become complete empty answers.

#### RAG-S04-T01: Load Captured Source Under a Recorded Build Profile

**Objective and changes:** Use pinned `go/packages` with controlled worktree root, captured source overlays, build tags, GOOS/GOARCH/CGO/toolchain/module locks, and offline dependency policy. Check both the top-level error and per-package errors. Do not run generators, package initialization, application binaries, or automatic toolchain/module downloads.

**Files:** proposed `tooling/agentgraph/extract/goextract/load.go`, snapshot/build manifest, and optional unit/integration fixtures.

```go
cfg := &packages.Config{
	Context: ctx, Dir: worktreeRoot, Overlay: capturedGoBytes,
	Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
		packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
		packages.NeedSyntax | packages.NeedModule,
}
pkgs, err := packages.Load(cfg, "./backend/...", "./internal/...", "./tooling/go/...")
```

**Expected result and acceptance:** Manifest records the actual environment; package errors produce coverage gaps. Source references and hashes come from captured bytes. Test variants and inactive build-tag files do not masquerade as duplicate production declarations. **Verification:** offline loader fixtures with build tags, missing modules, modified overlays, and package errors; all-file catalog coverage remains separately reported.

#### RAG-S04-T02: Emit Canonical Declarations and Call-Site Edges

**Objective and changes:** Resolve static callees using type information and a verified helper such as [typeutil.StaticCallee](https://pkg.go.dev/golang.org/x/tools/go/types/typeutil#StaticCallee). Separate builtins/conversions, direct calls, interface dispatch, function values, and unresolved sites. Canonicalize generic declarations and receiver identities; give anonymous scopes snapshot-local identities rather than attributing their calls to the enclosing function.

**Files:** proposed `extract/goextract/{declarations,calls}.go`, fact DTOs, and synthetic generic/method/interface/closure fixtures.

```go
callee := typeutil.StaticCallee(pkg.TypesInfo, call)
// nil requires classification; it is not automatically "no call".
// A resolved ID is position-independent, e.g.:
// go:analysis/backend/pdfstore#(*Store).Save
```

**Expected result and acceptance:** Distinct call sites remain distinct edges, even for the same caller/callee. Each edge contains source path/hash, line/column, resolver, evidence category, and build context. No whole-program runtime-call claim is made. **Verification:** optional unit/functional golden facts for direct and generic calls, methods, callbacks, interfaces, closures, and moved declarations; compare IDs independently from locations.

#### RAG-S04-T03: Validate One Real Repository Vertical Slice

**Objective and changes:** Trace the PDF save path using current source and the synthetic contract. Resolve the Go handler-to-store and store-to-generated-query calls; classify generator mappings separately. Record unsupported references and useful test candidates without claiming runtime coverage.

**Files:** existing [saveArticlePDF](../backend/server/pdf_mutation.go), [pdfstore.Save](../backend/pdfstore/mutation.go), [maintained PDF queries](../backend/pdfstore/schema/queries.sql), and proposed query fixtures; no application code change is required by this task.

```text
(*Server).saveArticlePDF
  -- CALLS [Go-resolved] --> (*pdfstore.Store).Save
  -- through Save --> (*generated.Queries).MarkPDFDocumentAvailable
  -- GENERATED_FROM [sqlc mapping] --> backend/pdfstore/schema/queries.sql
```

**Expected result and acceptance:** The result cites current source hashes and actual call sites, not copied line numbers frozen in this plan. Generated SQL evidence links to maintained inputs without changing private generated package boundaries. **Verification:** optional integration lookup on a captured repository snapshot plus a synthetic regression independent of future application refactors.

### RAG-S05: Connect Repository Languages and Authoritative Guidance

**Story:** As an agent, I need to follow a workflow across frontend requests, backend routes, SQL, configuration, and relevant documentation. **Technical target:** language-specific enrichment over shared source identities, with conventions clearly separated from semantic proof. **Dependencies:** S02 and S04's record contract. **Acceptance:** useful cross-language paths retain the evidence and incompleteness of every hop.

#### RAG-S05-T01: Resolve TypeScript and Match HTTP Boundaries

**Objective and changes:** Use the pinned TypeScript compiler through optional Node enrichment for imports/references and selected API request templates. Extract Go route method/pattern/handler mappings. Normalize route variables conservatively, distinguish methods, and label cross-language route matching as convention-inferred; do not infer arbitrary constructed URLs.

**Files:** proposed `tooling/node/agent-workflow/extract-typescript.ts`, native `extract/routes.go`, TypeScript fixtures under `tests/tooling/node/agent-workflow/`, and read-only use of [api.tsx](../frontend/src/api.tsx) and [server.go](../backend/server/server.go).

```text
uploadPDF(runID, revisionID, request)
  -- REQUESTS [template inference] --> PUT /api/runs/{run_id}/articles/{work_revision_id}/pdf
  -- SERVED_BY [registered Go route] --> (*Server).saveArticlePDF
```

**Expected result and acceptance:** The real PDF path can be explained without marking the TS-to-route hop as type-resolved Go evidence. Missing Node/compiler prerequisites affect only this enrichment and are visible in coverage. Root doccheck remains Go-only. **Verification:** method mismatch, ambiguous routes, parameter renaming, TS module/compiler configuration, and unsupported dynamic URL fixtures; no server or real corpus is needed.

#### RAG-S05-T02: Import SQL Ownership and Generator Mappings

**Objective and changes:** Consume owning SQL/sqlc inventory records, named query declarations, and generated wrapper mapping. Associate table use with maintained queries and expose unresolved raw/dynamic SQL. Keep application SQL ownership enforcement independent from native Cypher calls.

**Files:** owning exports from [importcheck](../tooling/go/importcheck/main.go) and [rawsqlcheck](../tooling/go/rawsqlcheck/main.go), [config/sqlc.yaml](../config/sqlc.yaml), proposed `tooling/agentgraph/extract/sql.go`, and fixtures.

```json
{"from":"sqlc:pdfstore#MarkPDFDocumentAvailable","relationship":"QUERIES","to":"sql:pdfstore#pdf_documents","evidence":"named-query-input","path":"backend/pdfstore/schema/queries.sql"}
```

**Expected result and acceptance:** A generated wrapper leads to its real maintained input and owning store. The graph does not import private generated packages or replace SQL classifiers with a second allowlist. **Verification:** generated-file renames, named query/table joins, unknown dynamic SQL, and two families with similarly named queries; existing SQL/import checker suites remain authoritative.

#### RAG-S05-T03: Extract Configuration, Commands, and Documentation Chunks

**Objective and changes:** Use SOMETHING's owning compiler through the portable bridge where needed; parse selected JSON/YAML/Make relationships and Markdown headings/links. Extract source/configuration-to-guidance evidence and source-owned command/test references. Preserve uncertainty around Make expansions and prose mentions.

**Files:** portable compiler adapter near `tooling/go/codefacts/`, existing [SOMETHING compiler](../backend/something/), proposed native `extract/{configuration,markdown,commands}.go`, and `config/codegraph/policy.json`.

```json
{"kind":"doc-section","path":"docs/STANDARDS.md","heading":"Documentation standards","source_hash":"sha256:<digest>","applies_to":["docs/"],"evidence":"explicit-policy-mapping"}
```

**Expected result and acceptance:** Documentation chunks retain original paths, headings, hashes, and trust classification. Catalog copies and generated hash tables are deduplicated; secrets, runtime corpus data, ignored worktrees, and symlink escapes stay excluded. **Verification:** compiler failure, include dependencies, renamed headings, command expansions, generated-source ownership, excluded paths, and malicious instruction text as data.

### RAG-S06: Publish Correct Immutable Graph Generations

**Story:** As concurrent agents, we need a stable task baseline and consistent read-only results while a checkpoint builds a new graph. **Technical target:** typed schema, source-verified checkpoints, immutable publication, and threshold retention. **Dependencies:** S01, S02, and S04. **Acceptance:** readers never open writable candidates, failed updates preserve the task baseline, and cleanup respects newest-three, reader, and durable task protections under the aggregate resource limits.

#### RAG-S06-T01: Implement the Small Typed Schema and Full Loader

**Objective and changes:** Write explicit Ladybug DDL for the first queries and load versioned facts in a deterministic full build. Validate duplicate IDs, relationship endpoints, source hashes, coverage, and schema compatibility. Add extensions only after their S01 gate passes; no inheritance assumption or schema generator is required.

**Files:** proposed `tooling/agentgraph/store/{schema,load,validate}.go`, `store/core.cypher`, and native fixtures.

```cypher
CREATE NODE TABLE SourceFile(path STRING, sha256 STRING, PRIMARY KEY(path));
CREATE NODE TABLE Declaration(id STRING, package_path STRING, name STRING, kind STRING, PRIMARY KEY(id));
CREATE REL TABLE DECLARES(FROM SourceFile TO Declaration, start_line INT64, end_line INT64);
CREATE REL TABLE CALLS(FROM Declaration TO Declaration, path STRING, source_sha256 STRING, line INT64, column INT64, resolution STRING);
```

**Expected result and acceptance:** A fresh synthetic graph answers the golden direct-call questions and rejects dangling or incompatible facts. IDs belong to one generation's source snapshot; separate call sites are preserved. **Verification:** create/load/query/reopen integration tests against the pinned engine, schema incompatibility, invalid input, and duplicate-site cases.

#### RAG-S06-T02: Build Privately and Publish a Validated Manifest

**Objective and changes:** At an explicit completion/handoff checkpoint, verify the actual source delta against the task baseline and journal, capture source/build inputs, and bind relevant check evidence to that fingerprint. Serialize publishers per worktree, build a unique candidate, validate identity/queries and matching check evidence, close/finalize the writer, and publish atomically. Recheck the predecessor/live snapshot before claiming verified current status; source/build drift requires recapture/revalidation. Record the baseline and publication in handoff evidence. Do not rebuild after every edit or rely only on an agent-reported list; use full rebuild as the initial fallback. Apply S06-T04 retention only after its safety gate passes.

**Files:** proposed `tooling/agentgraph/generation/{build,publish,manifest}.go`, `build/codegraph/current.json`, and per-worktree lock/generation directories.

```json
{"format_version":1,"generation":"g17","schema_version":1,"facts_version":1,"repository_id":"<repo>","worktree_id":"<worktree>","head":"<commit>","input_fingerprint":"sha256:<digest>","build_context":"<profile>","verification":{"source_build_fingerprint":"sha256:<digest>","evidence_record":"<matching-check-record>"},"coverage":{"go":"partial"},"database":"generations/g17/graph.lbdb"}
```

**Expected result and acceptance:** A reader sees either the previous complete generation or the new complete generation, never partial state. Verified handoff evidence matches exactly the published source/build inputs. A failed refresh retains older output with honest stale status. Publication is tested for crash/durability; candidate names are never reused. **Verification:** concurrent publishers/readers, source edits before/during/after checks and extraction, stale check-evidence rejection, crash before/after replacement, corrupt candidate, and repository/worktree mismatch cases.

#### RAG-S06-T03: Pin Readers and Task Baselines and Bound Queries

**Objective and changes:** Select one manifest/generation per request under the project lifecycle lock, obtain the per-generation reader pin before releasing that lock, verify containment/compatibility, open only read-only, and enforce S01 limits until handles close. Preserve task baselines in durable ownership records through interruption. Implement the lock ordering and unique pin-file lifetime required before S06-T04 can retire generations.

**Files:** proposed `generation/{read,pins}.go`, `store/connection.go`, durable task-pin integration, and native concurrent-process tests.

```text
Reader A pins g17 and opens it read-only; a task may also pin it durably.
Publisher builds g18 privately, closes its writer, then publishes current -> g18.
Reader B opens g18; A finishes against g17 and reports g17's fingerprint.
Collection skips g17 while any reader/task protects it, even if it is older than three runs.
```

**Expected result and acceptance:** Readers observe consistent generations, and cancellation closes handles before releasing pins. A crashed reader releases its OS lock; an interrupted task retains its durable baseline pin. Age, PID absence, or manifest replacement alone cannot authorize deletion. **Verification:** reader-selection/collector races, held readers across publication, cancellation, interrupted/resumed tasks, pin-file reuse prevention, and lifetime tests in separate processes.

#### RAG-S06-T04: Implement Newest-Three and Threshold Retention

**Objective and changes:** Keep the newest three successful publications per worktree and all reader/task pins. Below the project-wide 10 GB generation threshold, allow eligible older history to remain. At the threshold, use the lifecycle lock and nonblocking exclusive pin acquisition to collect eligible older generations oldest first toward 8 GB. Account for candidates and stop/defer heavy updates on unresolved storage pressure; do not scan or delete arbitrary build/model/source data.

**Files:** proposed `tooling/agentgraph/generation/{retention,usage}.go`, explicit policy fields in `config/codegraph/policy.json`, task-pin metadata, and scaled native integration fixtures. Collection runs at checkpoints or explicit maintenance, outside pre-tool lookup latency.

```json
{"retention":"storage-pressure","high_water_bytes":10000000000,"low_water_bytes":8000000000,"protected_generations":["g10","g9","g8","g2"],"protected_bytes":9000000000,"eligible_bytes":0,"action":"preserve-protected-and-defer-heavy-update"}
```

**Expected result and acceptance:** The example includes an older task-pinned `g2`. The collector never trades safety for a nominal byte target; it reports when the protected set prevents reaching the low-water mark. Newest-three means successful publication order, not lexical IDs, timestamps alone, application runs, or failed candidates. **Verification:** lower byte thresholds in controlled fixtures; test oldest-first order, per-worktree floors, total accounting, task/reader pins, concurrent publication, interrupted collection, symlink/ownership rejection, and disk-pressure recovery without filling a real 10 GB cache.

### RAG-S07: Deliver Catalog Queries and Bounded Retrieval

**Story:** As an agent, I need a stable question interface with short answers and inspectable evidence. **Technical target:** typed catalog parameters, exact/lexical/graph retrieval, and a versioned CLI response. **Dependencies:** S03, S04, and S06. **Acceptance:** the explicit CLI is useful before hooks or embeddings exist and distinguishes unavailable, partial, stale, and complete results.

#### RAG-S07-T01: Implement a Versioned JSON Query Catalog

**Objective and changes:** Start with JSON for the small query/parameter registry, using existing JSON support rather than adding a schema DSL/generator. Keep Cypher prepared and source-controlled. Validate IDs, parameter types, allowed bounds, schema compatibility, and required extraction coverage before opening the database; use finite audited templates for non-parameterizable depth/label variants.

**Files:** proposed `config/codegraph/queries.json`, `tooling/agentgraph/query/{catalog,parameters}.go`, and optional unit/functional fixtures.

```cypher
MATCH (caller:Declaration)-[site:CALLS]->(callee:Declaration)
WHERE callee.id = $symbol
RETURN caller.id AS symbol, site.path AS path, site.line AS line,
       site.source_sha256 AS source_hash, site.resolution AS evidence
ORDER BY symbol, path, line
LIMIT $limit;
```

**Expected result and acceptance:** `$symbol` and `$limit` are bound parameters; prompt text never becomes query syntax. Unknown IDs, invalid bounds, incompatible schema, and unsupported required coverage have explicit responses. **Verification:** prepared-query fixtures, quotes/control characters in identifiers, stable ordering, truncation, and no-reindex catalog-only changes.

#### RAG-S07-T02: Add Exact, Lexical, and Graph Retrieval with Explanations

**Objective and changes:** Preserve exact symbol/path anchors, add lexical candidates and bounded relationship expansion, deduplicate authoritative sources, and render reasons/coverage. Portable lexical matching remains available if optional FTS is unavailable. Do not compare unrelated raw search scores or infer test coverage from name matches.

**Files:** proposed `query/{retrieve,rank,evidence}.go`, optional catalog/test metadata adapters, and golden response fixtures.

```json
{"format_version":1,"query_id":"symbol.context","status":"partial","generation":"g17","freshness":"current","workspace":{"worktree_id":"<id>","input_fingerprint":"sha256:<digest>"},"coverage":{"go":{"unresolved_call_sites":1}},"items":[{"symbol":"go:analysis/backend/server#(*Server).saveArticlePDF","path":"backend/server/pdf_mutation.go","line":103,"source_hash":"sha256:<digest>","evidence":"semantically_resolved","reason":"direct caller of pdfstore.Save"}],"truncated":false}
```

**Expected result and acceptance:** This illustrative line/hash is replaced with current extraction evidence during execution. Freshness can be current while coverage is partial; the response does not conflate them. Results retain exact matches and expose follow-up paths. **Verification:** deterministic rank/merge cases, duplicate catalog prose, unresolved sites, unavailable FTS, partial coverage, and wrong-worktree rejection.

#### RAG-S07-T03: Expose Explicit Status, Update, and Query Commands

**Objective and changes:** Add thin CLI dispatch with `status`, `update`, and `query`, JSON and compact text rendering, explicit root/task resolution, bounded stdout/stderr, meaningful exit codes, and cancellation. `status` reports the pinned baseline and verified dirty state without rebuilding; `update` publishes a task checkpoint and is the only generation builder. Do not infer the primary checkout from a `.git` directory assumption.

**Files:** proposed `cmd/code-graph/main.go`, importable CLI dispatch, `query/render.go`, and process-level functional tests.

```sh
./build/code-graph status --root . --format json
./build/code-graph update --root . --task RAG-S05-T01
./build/code-graph query symbol.context --root . --symbol 'go:analysis/backend/pdfstore#(*Store).Save' --limit 20 --format json
```

**Expected result and acceptance:** A subdirectory invocation uses its real owning worktree; missing indexes and malformed output never become successful empty evidence. Queries make no network request or acknowledgement update. **Verification:** explicit native functional suites with paths containing spaces/quotes, missing/corrupt index, unknown query, timeout, oversized output, and two dirty worktrees.

#### RAG-S07-T04: Evolve Questions and Capabilities from Real Work

**Objective and changes:** Implement the separate reviewed question corpus, sanitized local gap records, domain/capability coverage reports, and a story-review rule that adds/updates a case or explains existing coverage. Map new questions to reusable/combined capabilities first; add extraction/query/schema work when facts are missing. Close a gap only when its evidence/uncertainty regression passes, not when its wording is added to a file.

**Files:** proposed `config/codegraph/questions.json`, `query/{questions,gaps,coverage}.go`, synthetic domain fixtures, ignored `build/codegraph/feedback/`, and the story/task review template. Reuse the existing query catalog; do not duplicate executable Cypher into question records.

```json
{"gap_id":"gap-17","task_id":"<workspace-story-task>","domain":"workspace","capabilities":["change.impact"],"reason":"missing-schema-fingerprint-edge","paths":["backend/workspace/"],"status":"needs-reviewed-fixture","raw_prompt_recorded":false}
```

**Expected result and acceptance:** Demonstrate adding a non-PDF question about schema-driven workspace reuse, including required new evidence and a regression; existing capabilities remain compatible. The corpus grows beyond the seed without silently self-modifying tracked policy or recording raw prompts. **Verification:** question/capability ID/version validation, unsupported-to-supported promotion, missing/incorrect evidence and dirty-worktree cases, domain coverage reports, and query-only changes requiring no reindex.

### RAG-S08: Optimize Updates Only After Correct Rebuilds

**Story:** As a developer, I need task checkpoints to reuse unchanged work when rebuild measurements show a material cost. **Technical target:** dependency-safe invalidation of a source-verified delta with a full-rebuild oracle. **Dependencies:** S04, S06, and S07 measurements. **Acceptance:** incremental output is equivalent to a clean rebuild for supported edits; uncertain changes fall back to rebuilding. Checkpoint publication and dirty-state reporting are core behavior even if this optimization is deferred.

#### RAG-S08-T01: Define the Dependency Invalidation Closure

**Objective and changes:** Reconcile the agent's task-local change journal with hashes of the actual baseline and checkpoint source, including eligible new files, generators, manual edits, renames, and deletions. Track file/package/configuration dependencies and compute affected consumers for declaration/signature/import/type/configuration changes. Separate semantic invalidation from position/embedding refresh. Treat build/tool/schema changes as full rebuilds until a narrower closure is proven; do not rebuild the persistent baseline during intermediate edits.

**Files:** proposed `tooling/agentgraph/generation/invalidate.go`, extraction dependency records, and edit-sequence fixtures.

```text
store.Save signature changes
  -> store package facts
  -> unchanged callers that must be resolved again
  -> route/impact query projections
  -> changed source chunks only for embeddings
```

**Expected result and acceptance:** Re-resolve unchanged consumers; remove obsolete incoming/outgoing edges and refresh locations. Unsupported dependency reasoning selects full rebuild instead of guessing. **Verification:** differential fixtures for edits, renames, deletes, imports, generic/type changes, and parent updates; compare facts and queries against a clean rebuild.

#### RAG-S08-T02: Build Incremental Candidates Without Mutating Readers

**Objective and changes:** Reuse parsed immutable facts or construct a new candidate from the previous generation through an engine-supported safe path. Never mutate a published graph. Reuse embeddings only when source/model/chunk fingerprints match, and retain full rebuild as a user-visible fallback.

**Files:** proposed `generation/incremental.go`, immutable fact cache, embedding invalidation, and explicit update reports.

```json
{"update_mode":"incremental-candidate","changed_files":2,"invalidated_packages":4,"fallback_reason":null,"published_generation":"g18"}
```

**Expected result and acceptance:** An interrupted incremental build leaves published readers intact; output matches the rebuild oracle and the measured update cost improves. If no material improvement exists, retain full rebuilds and close this story as deferred with evidence. **Verification:** long edit sequences, random small fixture edits, crash injection, concurrent readers, and before/after timing reports.

### RAG-S09: Integrate Codex and OpenCode with Thin Adapters

**Story:** As a developer, I need both agents to receive relevant evidence through supported interfaces without new prompts or permission changes. **Technical target:** one query/intent/rendering implementation in the CLI, a bounded host client, and tested capability-specific wrappers. **Dependencies:** S07. **Acceptance:** explicit retrieval works in both hosts; automatic context delivery is enabled only after a controlled smoke test proves it.

#### RAG-S09-T01: Implement the Shared Bounded Hook Translator

**Objective and changes:** Add a CLI `hook codex` translator that reads supported event JSON, resolves workspace identity, conservatively maps recognized lookups to catalog queries, and renders bounded informational context. Skip arbitrary shell syntax and the graph's own commands. Deduplicate by logical agent/task, turn/tool event, query, generation, and source fingerprint, not session ID alone.

**Files:** proposed `tooling/agentgraph/hook/{input,intent,render,dedup}.go` and synthetic host fixtures under the optional test module. Keep query/ranking policy out of host wrappers.

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"Repository evidence: generation g17, current source, partial Go coverage. pdfstore.Save has a resolved caller in backend/server/pdf_mutation.go. Verify the returned source hash before editing; unresolved interface calls remain."}}
```

**Expected result and acceptance:** Supported [Codex JSON context output](https://learn.chatgpt.com/docs/hooks) reaches the model; plain stdout is not treated as a context channel. Output has no approval, input rewrite, continuation prompt, or acknowledgement update. Missing/stale/index/model states are advisory and explicit. **Verification:** versioned event fixtures, concurrent callbacks/subagents, dirty-edit invalidation, recursion, unknown tools/events, malformed input, and output/deadline limits.

#### RAG-S09-T02: Register and Prove Codex Hooks Without Overwriting Setup

**Objective and changes:** Target Codex CLI `0.166.0` and capture that release's event/tool aliases, trust flow, input shape, output timing, and failure behavior. Begin with observed lookup selection, then one small proven pre-tool registration; add startup/resume/edit/compaction only when verified. Merge user-managed configuration deliberately rather than replacing existing hooks. A future target upgrade is an explicit compatibility/plan change.

**Files:** proposed setup examples/compatibility fixtures under `tooling/agentgraph/hook/` and later operator documentation; actual user/agent configuration changes occur during explicit installation, not during ordinary build.

```json
{"hooks":{"PreToolUse":[{"matcher":"^Bash$","hooks":[{"type":"command","command":"\"$(git rev-parse --show-toplevel)/build/code-graph\" hook codex","timeout":2,"additionalContextLimit":800}]}]}}
```

**Expected result and acceptance:** This proposed registration is validated against the pinned host and preserves normal commands on timeout/failure. The handler enforces its own byte/deadline bounds; the host's approximate token limit is additional protection. No runtime compilation, indexing, model startup, or download occurs in this path. **Verification:** installed-host smoke evidence showing model-visible context from a real lookup plus missing binary/index and disabled/trust-denied cases; ordinary tests use synthetic fixtures and need no agent installation.

#### RAG-S09-T03: Expose an OpenCode Tool and Gate Optional Injection

**Objective and changes:** Target OpenCode `1.19.0` with a uniquely named explicit custom tool, using [OpenCode's supported tool interface](https://opencode.ai/docs/custom-tools/). Invoke the shared CLI through a capped subprocess client with an argument array and no shell. Validate the matching SDK and host cancellation signal in an explicit fixture. Treat pre-tool or experimental compaction injection as a separate capability gate; keep retrieval independent from [auto-continue.js](../.opencode/plugins/auto-continue.js), which submits continuation prompts. Do not silently upgrade the host to match newer documentation.

**Files:** proposed `.opencode/tools/code-graph.ts`, `tooling/node/agent-workflow/client.ts`, Node unit/functional tests, and narrow references in the existing OpenCode policies.

```ts
import { tool } from '@opencode-ai/plugin';
import { invokeGraph } from '../../tooling/node/agent-workflow/client.ts';

export default tool({
  description: 'Retrieve bounded repository evidence',
  args: {
    query: tool.schema.enum(['symbol.context', 'change.impact', 'owner.sources']),
    parameters: tool.schema.string(),
  },
  async execute(args, context) {
    return invokeGraph({
      root: context.worktree, query: args.query, parameters: args.parameters,
      signal: context.abort,
    });
  },
});
```

**Expected result and acceptance:** The example targets the SDK context exposing `abort`; verify the installed interface and map another cancellation field if that pinned release differs. The client uses `spawn(binary, argumentArray, {cwd: root, shell: false})`, combines the host signal with its hard deadline, terminates/reaps cancelled children, and validates/caps output; it does not duplicate ranking. The explicit result is model-visible even where `tool.execute.before` cannot inject. SDK wrappers have pinned optional checks, not unchecked ambient types. **Verification:** `make test DOMAIN=tooling COMPONENT=node/agent-workflow LAYER=unit`, functional fake-binary cancellation/deadline tests, `make check-frontend` for maintained TypeScript, optional SDK smoke tests, and installed-host context evidence.

### RAG-S10: Coordinate Worktrees and Stacked Dependencies Safely

**Story:** As a coordinator, I need 2–3 agents to follow the same versioned plan and worktree rules while owning independent source/output. **Technical target:** local task/dependency records, one shared Codex/OpenCode skill, and read-only status before broad lifecycle automation. **Dependencies:** S02; S07 for graph status. **Acceptance:** both targeted hosts discover the skill, ownership/baselines survive interruption, parent changes are deliberate, and stale metadata never causes source deletion or branch rewriting.

#### RAG-S10-T01: Define Workspace Identity and Local Ownership Records

**Objective and changes:** Derive local repository/worktree identity from canonical Git common/admin directories and verify entries against `git worktree list --porcelain`. Keep task records in ignored `worktrees/.state/tasks/`, outside graph generations. Bind each task to the GitHub story/task identifiers, versioned repository plan path/commit or content hash, dependency IDs/contracts, parent commit, graph baseline/published generation, write scope, and check evidence. Change durable pins atomically through the lifecycle-coordinated API; malformed metadata blocks collection for that scope. Validate containment/names/symlinks and reserve `.state` from worktree names.

**Files:** proposed portable workspace metadata near `tooling/go/codefacts/`, native `workspace/status.go`, task-record schema/fixtures, and the existing `worktrees/.gitkeep`/ignore convention.

```json
{"format_version":1,"task_id":"RAG-S07-T01","owner":"query-agent","status":"active","plan":{"path":"docs/CODE-GRAPH-RAG-DISCUSSION.md","commit":"<plan-commit>","story":"RAG-S07"},"worktree":"worktrees/query-cli","branch":"agent/rag-query","base_commit":"<full-commit>","parent":{"branch":"agent/rag-store","commit":"<full-parent-commit>"},"dependencies":["RAG-S06-T02"],"write_scope":["tooling/agentgraph/query/"],"baseline_generation":"g17","published_generation":null,"checks":[]}
```

**Expected result and acceptance:** The example's checks are synthetic record shape, not claims of completed task testing. Root/branch identity does not come from a directory name alone. `.gitkeep` remains the only committable container file; every real workspace/task record stays ignored. **Verification:** temporary-Git-repository cases, mismatched metadata, moved roots, invalid names, symlink escape, and read-only status preserving every source file.

#### RAG-S10-T02: Exercise Assignment and Parent Handoffs Before Automating Them

**Objective and changes:** Use manual `git worktree add` and one-owner assignment, with explicit parent commits and write scopes. Record old/new parent cutoffs, pause affected writers, integrate/restack deliberately, finish generated/docs output, capture the source/build snapshot, run matching checks, and publish its graph checkpoint before handoff. Evidence from the old parent or different bytes cannot verify the new generation. Implement a creation wrapper only after this protocol is demonstrated; no coordinator service is needed.

**Files:** task-record transitions, proposed `workspace/handoff.go` validation, synthetic repository fixtures, and the workflow in Section 7.

```sh
# Illustrative authorized handoff; all affected writers are paused.
git -C worktrees/query-cli status --short
git -C worktrees/query-cli rebase --onto main OLD_PARENT_COMMIT agent/rag-query
```

**Expected result and acceptance:** `OLD_PARENT_COMMIT` is the recorded ancestor cutoff, not necessarily the parent's latest tip. Parent uncommitted changes never appear in the child; a squash-merged parent does not reappear in the child diff after a correct restack. Conflicts preserve resumable interrupted state. **Verification:** temporary repositories covering parent advancement, unrelated parallel changes, squash merge, conflicts, and validation tied to the resulting fingerprint; no automated rebase beneath a writer.

#### RAG-S10-T03: Preserve Interrupted Work and Isolate Resources

**Objective and changes:** Define interrupted/owner-unconfirmed recovery, explicit inspected cleanup, and bounded expensive jobs. Keep build/frontend assets, writable fixtures, logs, and graphs within each task's worktree; use unique sub-output and assigned ports for jobs sharing a tree. Share only caches whose own concurrency contract permits it.

**Files:** proposed `workspace/recovery.go`, task-status validation, concurrent fixture helpers under the optional test module, and later usage documentation. Do not add a second browser/server manager when existing fixture/port isolation suffices.

```text
heartbeat expires -> owner-unconfirmed/interrupted -> inspect and contact coordinator
heartbeat expires -/-> reclaim branch, reset files, remove worktree, or delete evidence
```

**Expected result and acceptance:** A crash preserves source/task state and useful evidence. Reindexing never erases ownership. Cleanup follows existing authorization and Git worktree lifecycle commands; it is never triggered by age alone. **Verification:** terminate a fixture agent, resume it, run different dirty versions of the same relative path in two worktrees, and verify independent graph/fixture/build results with no force-add/reset/removal.

#### RAG-S10-T04: Implement One Shared Codex/OpenCode Worktree Skill

**Objective and changes:** In this future implementation task, create the instruction-only skill specified in Section 7.4 at the shared repository discovery path and add concise host/AGENTS routing. Require the assigned plan/story/task, dependencies, parent commit, write scope, baseline/dirty-state protocol, checkpoint evidence, and safe handoff/recovery. Preserve all existing task/source-control authorization. Do not duplicate the full plan or maintain separate host copies.

**Files:** proposed `.agents/skills/project-worktrees/SKILL.md`, narrow AGENTS/OpenCode routing references, explicit skill Markdown/frontmatter checks, and discovery/behavior fixtures for Codex `0.166.0` and OpenCode `1.19.0`. Include skill hashes in workflow inputs; DOC-STATE remains scoped to maintained `docs/` files. No skill is created or installed during the present planning discussion.

```yaml
name: project-worktrees
description: Follow the assigned project plan, dependency contracts, isolated worktree ownership, and graph-checkpoint handoff rules.
```

**Expected result and acceptance:** Both targeted hosts load the same skill from primary/linked-worktree subdirectories, with static plan/CLI fallback when graph tooling is absent. Agents cannot skip dependency checks, rewrite another active writer's branch, acknowledge docs through a graph refresh, or reclaim interrupted work. The skill stays within the aggregate resource/context policy. **Verification:** frontmatter/name/link checks, targeted host discovery/smoke cases, skill-routing cases, and temporary-repository task scenarios; no new service/MCP or model dependency.

### RAG-S11: Add Local Embeddings Only When They Improve Retrieval

**Story:** As an agent, I need related guidance when exact/lexical/graph retrieval is insufficient, while keeping local operation and simple fallbacks. **Technical target:** explicit local model provisioning, source-aware chunks, and evaluated vector rank fusion. **Dependencies:** S05, S07, and S12-T01's baseline. **Acceptance:** the model is optional, its artifacts are pinned and reviewed, and it improves selected tasks without reducing exact-match reliability.

#### RAG-S11-T01: Provision a Pinned Local Model and Deterministic Chunks

**Objective and changes:** Trial a compact English-only retrieval model, with [BGE-small-en-v1.5](https://huggingface.co/BAAI/bge-small-en-v1.5) as an initial candidate rather than a proven winner. Record artifact/version/license/checksum, tokenizer, dimensions, normalization, and measured peak/native/runtime memory. Keep the embedding worker inside the shared 1 GB RAG budget through serialization; no multilingual evaluation is required. Provision explicitly outside ordinary commands. Begin with heading/paragraph chunks around 200–350 model tokens, within the model's full input limit, plus source comments; deduplicate catalog copies, invalidate changed policy, and validate finite vectors/dimensions.

**Files:** proposed `tooling/agentgraph/embedding/{model,chunks,manifest}.go`, explicit setup policy, synthetic retrieval fixtures, and ignored model/artifact output.

```json
{"chunk_id":"docs/STANDARDS.md#documentation-standards:paragraph-1","source_hash":"sha256:<digest>","model_artifact_sha256":"<digest>","dimensions":384,"chunker_version":1,"normalization":"l2","embedding_status":"pending"}
```

**Expected result and acceptance:** `384` illustrates a dimension field, not a selected model. No model means normal graph/lexical operation. No source data is sent to an external embedding API, and no hook downloads or initializes a model. **Verification:** missing/corrupt artifact, incompatible license/configuration, NaN/infinite/wrong-dimension vectors, changed chunk boundaries, model-version invalidation, and offline operation.

#### RAG-S11-T02: Fuse Ranks and Measure Semantic Benefit

**Objective and changes:** Add vector candidates only through the proven read-only extension path. Fuse rank positions or another evaluated normalization instead of comparing raw lexical/cosine scores. Preserve exact anchors, deduplicate authoritative chunks, and include selection reasons. Compare with the same baseline tasks and snapshots.

**Files:** proposed `embedding/search.go`, `query/rank.go`, optional native vector fixtures, and evaluation report schema.

```text
exact symbol/path anchors are retained first
other candidate score = sum(1 / (k + rank_in_each_available_retriever))
deduplicate by authoritative source/chunk identity; return reason + provenance
```

**Expected result and acceptance:** The fusion constant/weights are benchmark configuration, not unexplained defaults. Optional vectors demonstrably help selected semantic tasks; absent vectors preserve the same structural results. If evidence is weak or resource cost dominates, leave this feature disabled/deferred. **Verification:** ablations, exact-match retention, duplicate-source/noise cases, model absence, and offline read-only reopen with measured cost.

### RAG-S12: Evaluate Acceptance, Simplify Bootstrap, and Finish Documentation

**Story:** As a maintainer, I need proof that the workflow improves real work and remains understandable when optional tooling is absent. **Technical target:** reproducible evaluation, evidence-based instruction cleanup, and complete ownership/verification documentation. **Dependencies:** baseline after S07; final acceptance after the selected core and optional stories. **Acceptance:** recorded results separate correctness from performance, and no retired behavior lacks a verified replacement.

#### RAG-S12-T01: Run Comparable Workflow Ablations and Failure Tests

**Objective and changes:** Define repeatable repository tasks and baseline report schema; compare ordinary navigation, explicit graph/lexical CLI, advisory hooks, and optional embeddings using the same source/model/permissions. Record task acceptance/regressions, evidence precision/omissions, source reads, context size, wall time, cold/warm p50/p95 latency, indexing resources, fallbacks, and variance.

**Files:** proposed acceptance fixtures under `tests/tooling/agentgraph/testdata/`, optional evaluation runner, and reports under `build/codegraph/reports/`.

```json
{"task":"trace-pdf-upload","variant":"graph-lexical-cli","snapshot":"sha256:<digest>","planned_runs":5,"completed_runs":0,"task_acceptance_passes":null,"warm_lookup_p95_ms":null,"context_tokens":null,"limitations":["synthetic report example; measurements not collected"]}
```

**Expected result and acceptance:** Null/unavailable measurements stay explicit; the example is not performance evidence. Deterministic correctness failures block rollout; a small pilot does not establish general productivity claims. **Verification:** repeat tasks and Section 8's corrupt/stale/concurrent/crash cases, preserve raw reports, and compare against the proposed budgets on a recorded machine.

#### RAG-S12-T02: Simplify Agent Bootstrap After Equivalence Review

**Objective and changes:** Propose a concise mandatory safety/command core and task/path-specific reading matrix, replace whole-catalog preload with focused lookup, and preserve direct document/`rg` fallbacks. Review all existing AGENTS/OpenCode requirements for equivalence before modifying instructions. Remove repeated discovery guidance only where the new interface demonstrably replaces it.

**Files:** [AGENTS.md](../AGENTS.md), [developer-plan.md](../.opencode/agent/developer-plan.md), [developer-execute.md](../.opencode/agent/developer-execute.md), maintained usage guidance, and adapter/bootstrap fixtures.

```json
{"task_kind":"database-change","mandatory_guidance":["repository safety and command rules","docs/STANDARDS.md","docs/DATABASE.md","owning schema inputs"],"lookup":"guidance.for_path","fallback":"read the same authoritative documents directly"}
```

**Expected result and acceptance:** A failed/missing lookup does not omit required security, ownership, migration, test, or review rules. Retrieval evidence remains data rather than instruction authority. No runtime model-generated policy replaces reviewed guidance. **Verification:** representative task-matrix/equivalence cases, no-index agent smoke runs, and reviewed policy diffs with source references for every removed duplication.

#### RAG-S12-T03: Finish Verification, Supported Commands, and Justified Cleanup

**Objective and changes:** Reconcile supported commands, module/test ownership, offline setup, capabilities, error/recovery behavior, and rollback-to-ordinary-navigation documentation. Delete only code/report parsing or repeated collection proven obsolete by the implemented shared path. Keep source comments, human-readable catalog, DOC-STATE acknowledgement, and owning enforcement tools unless a separately reviewed replacement is accepted.

**Files:** Makefile, coverage ownership, AGENTS, architecture/standards/usage/catalog prose where implementation changes them, DOC-STATE dependencies/hashes, and existing CI only where portable checks need registration. Native setup/tests remain explicit local targets under the chosen scope.

```sh
make docs-catalog-update
# Review changed documents and every DOC-STATE dependent before acknowledgement.
make docs-state-update
make test-docs
make check-docs
make check
```

**Expected result and acceptance:** Commands describe implemented behavior only; prospective examples graduate to supported usage after verification. Ordinary CI never needs Ladybug, a model, or an agent installation. No broad deletion touches ignored databases, worktrees, models, or build evidence. **Verification:** the matrix in Section 14, final diff/dependency/license review, and recorded task completion evidence; optional features remain explicitly disabled if their gates are unmet.

## 14. Verification Matrix and Completion Evidence

All native/new-component commands below are proposed additions, not targets available today. Existing root checks are run for this planning-document change; implementation verification happens with the corresponding stories. Setup may need explicitly provisioned artifacts, but subsequent ordinary checks and controlled query tests remain offline.

| Area | Planned verification | What it proves |
| --- | --- | --- |
| Portable source facts | `make test-go DOMAIN=tooling COMPONENT=go/codefacts` | Deterministic versioned exports, exclusions, owner reuse, and read-only behavior |
| Catalog/review state | `make test-docs` and `make check-docs` | Rendering/comment parity, links/format, migration contracts, and reviewed state hashes |
| Module/import ownership | `make test-go DOMAIN=tooling COMPONENT=go/importcheck` plus `make check-imports` | Correct root-module identities and preserved family boundaries |
| SQL inventory | `make test-go DOMAIN=tooling COMPONENT=go/rawsqlcheck` plus `make check-sql` | Owning SQL classifications remain authoritative and Cypher does not enter application SQL rules |
| Suite discovery | `make test-go DOMAIN=tooling COMPONENT=go/testselect` | Default exclusion and explicit optional-module selection, including unmatched selections |
| Optional native format/vet | `make check-code-graph` | Maintained optional production and test modules have syntax, formatting, and vet coverage |
| Optional unit/functional/integration | `make test-code-graph LAYER=unit`, `LAYER=functional`, and `LAYER=integration` | Fact/schema/query correctness, process protocol, and real native publication/cancellation |
| Optional races/resources | `make test-code-graph-race` and S01's process measurements | Concurrent Go ownership, native interruption, independent readers, and observed resource cost |
| Aggregate memory/retention | 2–3 reader/task fixtures, one heavy worker, and scaled disk thresholds | Measure the 1 GB RAG target; protect each newest-three set, readers, and interrupted-task baselines under 10 GB/8 GB threshold policy |
| Optional coverage | `make coverage-code-graph` | A separate native production coverage profile with explicit ownership/floors, not implied root coverage |
| Shared TypeScript client/enrichment | `make test DOMAIN=tooling COMPONENT=node/agent-workflow LAYER=unit` and `LAYER=functional`, then `make check-frontend` | Typed maintained Node logic with fake binaries and controlled compiler fixtures |
| Installed host/SDK integration | Explicit pinned Codex/OpenCode smoke runs | Correct event/output compatibility and actual model-visible context; no real service/API credential required for ordinary fixture tests |
| Worktree lifecycle | Optional integration fixtures using temporary Git repositories | Distinct dirty sources, correct parents, squash/restack handling, and preservation after interruption |
| Shared skill/plan | Frontmatter/link checks and Codex `0.166.0`/OpenCode `1.19.0` discovery/behavior fixtures | One planned skill routes each task to the correct plan/dependencies, independent worktree, dirty baseline, and checkpoint workflow |
| Question evolution | Corpus/catalog validation and domain promotion regression | Seed cases expand to a real non-PDF question with implemented evidence; unanswered gaps stay visible and raw prompts remain unlogged |
| Embeddings | Optional controlled model/vector tests after explicit setup | Hash/license/dimension checks, offline fallback, exact-match retention, and measured semantic benefit |
| Ordinary repository regression | `make build`, `make tools`, `make check`, relevant ordinary tests/races, and `make coverage` after portable code changes | No native prerequisite leakage or weakening of existing ownership/coverage guarantees |

Go race detection does not prove the safety of opaque C/C++ allocations or process-level file ownership. Retain independent-process, cancellation, and resource tests alongside Go race results. Use synthetic repositories and disposable controlled fixtures; no test needs a user's corpus, real application databases, live providers, or a network embedding endpoint.

Each implementation task records exact command, source fingerprint, prerequisite/platform/version, result, and any specific limitation. Missing native/agent/model prerequisites leave the corresponding gate incomplete; they are not passing tests. Completion requires relevant source comments, generated output, documentation review, and final diff inspection in addition to test results.

```json
{"task_id":"RAG-S06-T02","source_fingerprint":"sha256:<digest>","command":"make test-code-graph LAYER=integration TEST='^TestPublication$'","platform":"<os/arch>","prerequisites":{"engine":"<pinned-version>","binding":"<pinned-version>"},"result":"not-run","evidence_path":null,"limitation":"illustrative execution record"}
```

**Epic completion checklist:**

- [ ] All selected story/task outcomes have reviewed evidence and resolved dependencies.
- [ ] Root offline build/check/test paths do not require the native module, models, or agent installations.
- [ ] The explicit CLI answers acceptance questions with hashes, coverage, provenance, bounded output, and truthful failure states.
- [ ] Concurrent readers/publishers and different dirty worktrees satisfy isolation and interruption tests.
- [ ] Task checkpoints verify actual deltas, retain baseline evidence during edits, and publish once the handoff work is ready.
- [ ] Newest-three, reader/task pins, 10 GB/8 GB generation pressure, and measured 1 GB aggregate RAG memory satisfy the resource policy.
- [ ] Codex and OpenCode have verified explicit retrieval; enabled hook paths prove delivery and advisory failure behavior.
- [ ] The shared worktree skill is discovered in both targeted hosts and binds tasks to the versioned plan and dependency contracts.
- [ ] A new non-PDF question progresses from an explicit gap to an implemented capability/fixture with reviewed regression evidence.
- [ ] `.gitkeep` remains tracked/committable while every workspace/task artifact is ignored; source commits happen inside owning worktrees.
- [ ] Documentation generation has no graph/state cycle and no hook acknowledges review hashes.
- [ ] Simplification removes only demonstrated duplication; static guidance and existing enforcement remain supported.
- [ ] Optional incremental/vector features have their own evidence or remain explicitly deferred.
- [ ] Final documented commands, module/dependency/license inventories, source comments/catalog, and reviewed hashes match implemented behavior.

## 15. Dependency Plan and GitHub Project Tracking

### 15.1 Single-Agent Execution and Future Parallel Review Order

Execute one task at a time with one implementation agent. The graph below describes technical prerequisites, not permission to launch parallel agents during this epic. Keep independent tasks independent in native dependency links, and use a separate execution-order field for the single agent's schedule. Controlled concurrent readers/processes and disposable worktree fixtures remain required tests of the capability being delivered. Enable live multi-agent writing only after the setup and shared skill are accepted.

```text
S01: contracts and native feasibility
  |
  v
S02: portable exports and module ownership
  |------------------|------------------|
  v                  v                  v
S03: docs         S04: Go facts       S10: workspace records/protocol
                     |\
                     | \-> S05: cross-language enrichment
                     v
                  S06: generations
                     |
S03 -----------------+--> S07: explicit CLI
                              |--------------------|
                              v                    v
                           S09: adapters        S12-T01: baseline
                                                   |
                                  S05 + S07 -------+--> S11: vectors (optional)
S04 + S06 + S07 measurements --------------------------> S08: incremental (optional)
core story evidence + selected optional evidence ------> S12 final acceptance
```

The summary in Section 13.1 and concrete native task blockers in Section 15.6 control where this compact diagram omits an edge. Shared contracts must be committed before dependent branches consume them. A possible future shallow stack is S02 contract branch -> S04 extractor branch -> S06 storage branch; S03 documentation and S05 enrichment are technically independent once their required records exist. The single implementation agent still executes one task at a time. S07 can prepare fixtures against the committed DTO, but its final acceptance waits for real S06 integration.

Do not create a 12-branch stack simply because there are 12 stories. Batch small related tasks on one story branch, split independent ownership when it helps, and keep the stack depth small enough to review and restack safely. One coordinator owns shared Makefile, module/discovery, DTO, and query-contract changes; other tasks should avoid concurrent edits to those inputs. Record parent commits and verification fingerprints, and validate the final combined state after integration.

### 15.2 GitHub Hierarchy and Fields

The published [RAG-01 project](https://github.com/users/Nextness/projects/5) follows the existing [PDF story](https://github.com/Nextness/CoGen/issues/92) and [task](https://github.com/Nextness/CoGen/issues/138) structure: the project is the epic, story issues own shared contracts, and task issues are native sub-issues. Full issue bodies and the README preserve the reviewed plan, with native blocking links and an explicit single-agent execution order. Publication creates tracking only; no implementation, skill installation, commit, or push is marked complete.

| Level | Published title/identity | Body and fields |
| --- | --- | --- |
| Epic/project | [RAG-01: Improve agent workflows with a local code graph and selective retrieval](https://github.com/users/Nextness/projects/5) | Problem, scope, architecture/choices, acceptance, story index, dependencies, verification, and deferred features |
| Story issue | [RAG-S04: Extract Go semantics with explicit resolution limits](https://github.com/Nextness/CoGen/issues/146) | Epic link, Story ID, user story, technical target, references, dependencies/gate, task map, shared contracts, acceptance, completion evidence |
| Task sub-issue | [RAG-S04-T02: Emit canonical declarations and call-site edges](https://github.com/Nextness/CoGen/issues/164) | Epic/parent links, Task ID, objective/technical changes, owning files, example, dependencies, expected outcome, verification, documentation impact |
| Project fields | Plan ID, Kind, Story, Stage, Order, Gate, Priority, Status | Ready requires satisfied prerequisites; required successors start Blocked, optional work starts Backlog, and the executor updates status as evidence changes. Dependencies use native issue links. |

Native parent/sub-issue and blocking relationships are populated and verified, while textual IDs/dependency lists keep the plan portable. The All work, Single-agent tasks, and Stories views expose the issue hierarchy and fields. Story/task completion remains execution evidence, separate from tracker creation; no item starts In Progress or Done.

### 15.3 Copyable Task Issue Template

The following template is the body shape for a future import, with task-specific content supplied from Section 13. Preserve actual fenced examples and real newlines when creating issues; do not collapse task bodies into one-line summaries.

```markdown
**Epic:** RAG-01 — <project link when created>
**Task ID:** RAG-S04-T02. **Parent story:** <RAG-S04 issue link>.
**Repository standards:** AGENTS.md, STANDARDS.md, supported Make commands, and owning module/test contracts apply.

## Objective and technical work
Emit canonical Go declarations and distinct call-site edges; preserve unresolved dynamic sites.

## Story context and source references
Story: an agent needs build-aware callers with source evidence.
Files: tooling/agentgraph/extract/goextract/{declarations,calls}.go and optional fixture suites.
Example: typeutil.StaticCallee(pkg.TypesInfo, call); nil requires classification.

## Dependencies
RAG-S04-T01 and the versioned S02 fact contract; use the recorded parent commit.

## Expected result and acceptance
Stable symbol identity, distinct call sites, source hashes, evidence categories, and explicit coverage gaps.

## Verification and documentation
Run the task's unit/functional fixtures and record commands, versions, source fingerprint, and results.
Update source comments/catalog and review affected documentation before acknowledging hashes.
Review knowledge/retrieval needs; add or update a question case, or explain why existing cases cover this story.

## Completion evidence

- [ ] Technical work and failure cases are independently reviewable.
- [ ] Applicable verification has recorded outcomes and limitations.
- [ ] Generated output, owning documentation, and dependency impacts are reviewed.
```

### 15.4 Task-Level Sequencing and Export Completeness

Within each story, T01 normally establishes the contract/input used by later tasks; T02/T03 may overlap only after that contract is committed. Concrete cross-story task prerequisites include S06-T01 after S04-T02, S06-T02 after its loader validation, S07-T02 after S07-T01 and S06-T02, S09-T02/T03 after S09-T01's host-independent contract, and S11-T02 after S11-T01 plus S12-T01's baseline. S12-T03 waits for all selected implementation outcomes and their documentation reviews.

S10-T01's durable task/plan/pin record contract can complete after S02 without waiting for S07's status integration. S06-T03/T04 consume that early contract; retention is not enabled before its reader/task-pin tests pass. S07-T04 then adds question growth over the stable catalog, and S10-T04 implements the shared skill against the approved plan/workflow. These task-level gates avoid making core storage depend cyclically on the completed workflow story. S11 depends on the S12-T01 baseline task, not completion of the whole S12 story.

The initial publication verified all 12 story IDs, 36 task IDs, 36 native parent links, 91 native blockers, full issue contents, project fields/views, repository association, and an acyclic dependency graph including parent progress. Future additions must preserve those checks and export each task's full objective, files, example, acceptance, and verification. Store planning/issue content in Git; keep local ownership, model/index artifacts, and execution reports ignored unless a sanitized report is intentionally selected for review.

### 15.5 Published Tracking Index

The epic is [RAG-01, GitHub Project #5](https://github.com/users/Nextness/projects/5), linked to `Nextness/CoGen`. It contains 12 story issues, 36 native task sub-issues, and 91 native blocking relationships. The initial tracker is unimplemented: only RAG-S01-T01 is Ready among tasks; required successors are Blocked and measured optional work is Backlog. One implementation agent follows the task order below.

| Story | Native task sub-issues |
| --- | --- |
| [RAG-S01](https://github.com/Nextness/CoGen/issues/143) | [RAG-S01-T01](https://github.com/Nextness/CoGen/issues/155), [RAG-S01-T02](https://github.com/Nextness/CoGen/issues/156), [RAG-S01-T03](https://github.com/Nextness/CoGen/issues/157) |
| [RAG-S02](https://github.com/Nextness/CoGen/issues/144) | [RAG-S02-T01](https://github.com/Nextness/CoGen/issues/158), [RAG-S02-T02](https://github.com/Nextness/CoGen/issues/159), [RAG-S02-T03](https://github.com/Nextness/CoGen/issues/160) |
| [RAG-S03](https://github.com/Nextness/CoGen/issues/145) | [RAG-S03-T01](https://github.com/Nextness/CoGen/issues/161), [RAG-S03-T02](https://github.com/Nextness/CoGen/issues/162) |
| [RAG-S04](https://github.com/Nextness/CoGen/issues/146) | [RAG-S04-T01](https://github.com/Nextness/CoGen/issues/163), [RAG-S04-T02](https://github.com/Nextness/CoGen/issues/164), [RAG-S04-T03](https://github.com/Nextness/CoGen/issues/165) |
| [RAG-S05](https://github.com/Nextness/CoGen/issues/147) | [RAG-S05-T01](https://github.com/Nextness/CoGen/issues/166), [RAG-S05-T02](https://github.com/Nextness/CoGen/issues/167), [RAG-S05-T03](https://github.com/Nextness/CoGen/issues/168) |
| [RAG-S06](https://github.com/Nextness/CoGen/issues/148) | [RAG-S06-T01](https://github.com/Nextness/CoGen/issues/170), [RAG-S06-T02](https://github.com/Nextness/CoGen/issues/171), [RAG-S06-T03](https://github.com/Nextness/CoGen/issues/172), [RAG-S06-T04](https://github.com/Nextness/CoGen/issues/173) |
| [RAG-S07](https://github.com/Nextness/CoGen/issues/149) | [RAG-S07-T01](https://github.com/Nextness/CoGen/issues/174), [RAG-S07-T02](https://github.com/Nextness/CoGen/issues/175), [RAG-S07-T03](https://github.com/Nextness/CoGen/issues/176), [RAG-S07-T04](https://github.com/Nextness/CoGen/issues/177) |
| [RAG-S08](https://github.com/Nextness/CoGen/issues/150) | [RAG-S08-T01](https://github.com/Nextness/CoGen/issues/185), [RAG-S08-T02](https://github.com/Nextness/CoGen/issues/186) |
| [RAG-S09](https://github.com/Nextness/CoGen/issues/151) | [RAG-S09-T01](https://github.com/Nextness/CoGen/issues/178), [RAG-S09-T02](https://github.com/Nextness/CoGen/issues/179), [RAG-S09-T03](https://github.com/Nextness/CoGen/issues/180) |
| [RAG-S10](https://github.com/Nextness/CoGen/issues/152) | [RAG-S10-T01](https://github.com/Nextness/CoGen/issues/169), [RAG-S10-T02](https://github.com/Nextness/CoGen/issues/181), [RAG-S10-T03](https://github.com/Nextness/CoGen/issues/182), [RAG-S10-T04](https://github.com/Nextness/CoGen/issues/183) |
| [RAG-S11](https://github.com/Nextness/CoGen/issues/153) | [RAG-S11-T01](https://github.com/Nextness/CoGen/issues/187), [RAG-S11-T02](https://github.com/Nextness/CoGen/issues/188) |
| [RAG-S12](https://github.com/Nextness/CoGen/issues/154) | [RAG-S12-T01](https://github.com/Nextness/CoGen/issues/184), [RAG-S12-T02](https://github.com/Nextness/CoGen/issues/189), [RAG-S12-T03](https://github.com/Nextness/CoGen/issues/190) |

Publication source snapshot SHA-256: `21394607d07116d123990a4fed116408d15b45c6c763725c8a39ee70ab2f7497`. The GitHub README and full issue bodies preserve the reviewed execution contracts independently of whether the expanded local plan has been pushed. Tracking links added here do not mark implementation complete.

### 15.6 Single-Agent Execution Order and Native Task Blockers

The Order field supplies a valid single-agent schedule without turning every independent task into a technical dependency. Native blocking links are authoritative for readiness; status fields are updated by the executor as evidence and blockers change. S08/S11 remain measurement-gated: implement them when justified or record their deferral before final acceptance.

| Order | Task | Native task blockers | Gate |
| --- | --- | --- | --- |
| 1 | [RAG-S01-T01](https://github.com/Nextness/CoGen/issues/155) | None | Required |
| 2 | [RAG-S01-T02](https://github.com/Nextness/CoGen/issues/156) | [RAG-S01-T01](https://github.com/Nextness/CoGen/issues/155) | Required |
| 3 | [RAG-S01-T03](https://github.com/Nextness/CoGen/issues/157) | [RAG-S01-T02](https://github.com/Nextness/CoGen/issues/156) | Required |
| 4 | [RAG-S02-T01](https://github.com/Nextness/CoGen/issues/158) | [RAG-S01-T03](https://github.com/Nextness/CoGen/issues/157) | Required |
| 5 | [RAG-S02-T02](https://github.com/Nextness/CoGen/issues/159) | [RAG-S02-T01](https://github.com/Nextness/CoGen/issues/158) | Required |
| 6 | [RAG-S02-T03](https://github.com/Nextness/CoGen/issues/160) | [RAG-S02-T02](https://github.com/Nextness/CoGen/issues/159) | Required |
| 7 | [RAG-S03-T01](https://github.com/Nextness/CoGen/issues/161) | [RAG-S02-T03](https://github.com/Nextness/CoGen/issues/160) | Required |
| 8 | [RAG-S03-T02](https://github.com/Nextness/CoGen/issues/162) | [RAG-S03-T01](https://github.com/Nextness/CoGen/issues/161) | Required |
| 9 | [RAG-S04-T01](https://github.com/Nextness/CoGen/issues/163) | [RAG-S02-T03](https://github.com/Nextness/CoGen/issues/160) | Required |
| 10 | [RAG-S04-T02](https://github.com/Nextness/CoGen/issues/164) | [RAG-S04-T01](https://github.com/Nextness/CoGen/issues/163) | Required |
| 11 | [RAG-S04-T03](https://github.com/Nextness/CoGen/issues/165) | [RAG-S04-T02](https://github.com/Nextness/CoGen/issues/164) | Required |
| 12 | [RAG-S05-T01](https://github.com/Nextness/CoGen/issues/166) | [RAG-S04-T03](https://github.com/Nextness/CoGen/issues/165) | Required |
| 13 | [RAG-S05-T02](https://github.com/Nextness/CoGen/issues/167) | [RAG-S04-T03](https://github.com/Nextness/CoGen/issues/165), [RAG-S02-T01](https://github.com/Nextness/CoGen/issues/158) | Required |
| 14 | [RAG-S05-T03](https://github.com/Nextness/CoGen/issues/168) | [RAG-S02-T03](https://github.com/Nextness/CoGen/issues/160), [RAG-S04-T02](https://github.com/Nextness/CoGen/issues/164) | Required |
| 15 | [RAG-S10-T01](https://github.com/Nextness/CoGen/issues/169) | [RAG-S02-T03](https://github.com/Nextness/CoGen/issues/160) | Required |
| 16 | [RAG-S06-T01](https://github.com/Nextness/CoGen/issues/170) | [RAG-S04-T03](https://github.com/Nextness/CoGen/issues/165), [RAG-S01-T03](https://github.com/Nextness/CoGen/issues/157) | Required |
| 17 | [RAG-S06-T02](https://github.com/Nextness/CoGen/issues/171) | [RAG-S06-T01](https://github.com/Nextness/CoGen/issues/170), [RAG-S03-T02](https://github.com/Nextness/CoGen/issues/162) | Required |
| 18 | [RAG-S06-T03](https://github.com/Nextness/CoGen/issues/172) | [RAG-S06-T02](https://github.com/Nextness/CoGen/issues/171), [RAG-S10-T01](https://github.com/Nextness/CoGen/issues/169) | Required |
| 19 | [RAG-S06-T04](https://github.com/Nextness/CoGen/issues/173) | [RAG-S06-T03](https://github.com/Nextness/CoGen/issues/172) | Required |
| 20 | [RAG-S07-T01](https://github.com/Nextness/CoGen/issues/174) | [RAG-S06-T04](https://github.com/Nextness/CoGen/issues/173), [RAG-S03-T02](https://github.com/Nextness/CoGen/issues/162) | Required |
| 21 | [RAG-S07-T02](https://github.com/Nextness/CoGen/issues/175) | [RAG-S07-T01](https://github.com/Nextness/CoGen/issues/174), [RAG-S03-T01](https://github.com/Nextness/CoGen/issues/161) | Required |
| 22 | [RAG-S07-T03](https://github.com/Nextness/CoGen/issues/176) | [RAG-S07-T02](https://github.com/Nextness/CoGen/issues/175) | Required |
| 23 | [RAG-S07-T04](https://github.com/Nextness/CoGen/issues/177) | [RAG-S07-T03](https://github.com/Nextness/CoGen/issues/176), [RAG-S05-T03](https://github.com/Nextness/CoGen/issues/168) | Required |
| 24 | [RAG-S09-T01](https://github.com/Nextness/CoGen/issues/178) | [RAG-S07-T03](https://github.com/Nextness/CoGen/issues/176) | Required |
| 25 | [RAG-S09-T02](https://github.com/Nextness/CoGen/issues/179) | [RAG-S09-T01](https://github.com/Nextness/CoGen/issues/178) | Required |
| 26 | [RAG-S09-T03](https://github.com/Nextness/CoGen/issues/180) | [RAG-S09-T01](https://github.com/Nextness/CoGen/issues/178) | Required |
| 27 | [RAG-S10-T02](https://github.com/Nextness/CoGen/issues/181) | [RAG-S10-T01](https://github.com/Nextness/CoGen/issues/169), [RAG-S07-T03](https://github.com/Nextness/CoGen/issues/176) | Required |
| 28 | [RAG-S10-T03](https://github.com/Nextness/CoGen/issues/182) | [RAG-S10-T02](https://github.com/Nextness/CoGen/issues/181), [RAG-S06-T04](https://github.com/Nextness/CoGen/issues/173) | Required |
| 29 | [RAG-S10-T04](https://github.com/Nextness/CoGen/issues/183) | [RAG-S10-T03](https://github.com/Nextness/CoGen/issues/182), [RAG-S09-T02](https://github.com/Nextness/CoGen/issues/179), [RAG-S09-T03](https://github.com/Nextness/CoGen/issues/180) | Required |
| 30 | [RAG-S12-T01](https://github.com/Nextness/CoGen/issues/184) | [RAG-S07-T04](https://github.com/Nextness/CoGen/issues/177), [RAG-S05-T01](https://github.com/Nextness/CoGen/issues/166), [RAG-S05-T02](https://github.com/Nextness/CoGen/issues/167), [RAG-S09-T02](https://github.com/Nextness/CoGen/issues/179), [RAG-S09-T03](https://github.com/Nextness/CoGen/issues/180), [RAG-S10-T04](https://github.com/Nextness/CoGen/issues/183) | Required |
| 31 | [RAG-S08-T01](https://github.com/Nextness/CoGen/issues/185) | [RAG-S12-T01](https://github.com/Nextness/CoGen/issues/184), [RAG-S04-T03](https://github.com/Nextness/CoGen/issues/165), [RAG-S06-T04](https://github.com/Nextness/CoGen/issues/173) | Measured optional |
| 32 | [RAG-S08-T02](https://github.com/Nextness/CoGen/issues/186) | [RAG-S08-T01](https://github.com/Nextness/CoGen/issues/185) | Measured optional |
| 33 | [RAG-S11-T01](https://github.com/Nextness/CoGen/issues/187) | [RAG-S12-T01](https://github.com/Nextness/CoGen/issues/184), [RAG-S05-T03](https://github.com/Nextness/CoGen/issues/168) | Measured optional |
| 34 | [RAG-S11-T02](https://github.com/Nextness/CoGen/issues/188) | [RAG-S11-T01](https://github.com/Nextness/CoGen/issues/187), [RAG-S07-T04](https://github.com/Nextness/CoGen/issues/177) | Measured optional |
| 35 | [RAG-S12-T02](https://github.com/Nextness/CoGen/issues/189) | [RAG-S12-T01](https://github.com/Nextness/CoGen/issues/184) | Required |
| 36 | [RAG-S12-T03](https://github.com/Nextness/CoGen/issues/190) | [RAG-S12-T02](https://github.com/Nextness/CoGen/issues/189), [RAG-S03-T02](https://github.com/Nextness/CoGen/issues/162), [RAG-S06-T04](https://github.com/Nextness/CoGen/issues/173), [RAG-S07-T04](https://github.com/Nextness/CoGen/issues/177), [RAG-S09-T02](https://github.com/Nextness/CoGen/issues/179), [RAG-S09-T03](https://github.com/Nextness/CoGen/issues/180), [RAG-S10-T04](https://github.com/Nextness/CoGen/issues/183) | Required |
