# Automated Code Graph and RAG Pipeline for Agents

This document captures an early discussion about improving agent workflows by maintaining a structured, queryable representation of the repository. The storage decision is now settled: LadybugDB. The document records the problem, the chosen architecture, technical choices, and open questions so we can validate the schema and design before building anything.

## 1. Motivation

Current agents rely on documentation, generated catalogs, and file reads. Those are useful but have limits:

- Documentation describes intended behavior, not always the actual code graph.
- File reads require guessing which files matter and navigating manually.
- Large repositories exceed context windows, so agents miss cross-file relationships.
- Generated artifacts and tools already produce structured data, but that data is not connected in a queryable way.

The goal is to give agents a persistent, up-to-date graph of the codebase, augmented with retrieval and semantic context. Agents should be able to ask structural questions ("what calls this function?", "which handlers depend on this table?") and semantic questions ("how do we usually validate input?") without reading the entire repository.

## 2. Proposed Architecture

The system has four logical layers. The schema contract and query catalog are cross-cutting concerns that sit on top of them. Each part can be implemented incrementally.

### 2.1 Extraction Layer (AST Parsing)

Target languages and formats for the first version: Go, TypeScript, SQL, SOMETHING configuration, Makefile, JSON, and YAML. Parsing strategy per format:

- **Go**: Tree-sitter or Go's native `go/ast` parser. Native parsing gives types and package structure for free. Tree-sitter is more uniform if we already use it for other languages.
- **TypeScript**: Tree-sitter or the TypeScript compiler API. The compiler API gives type information but is heavier. Tree-sitter is lighter and sufficient for structural extraction.
- **SQL**: Tree-sitter has SQL grammars. For repository-specific SQL, we also want to extract table names, columns, and named queries from the sqlc inputs under `backend/database/`.
- **SOMETHING configuration**: This is a project-specific language. Tree-sitter likely does not have a grammar. Options are a dedicated parser using the existing `backend/something/` compiler, or regex-based extraction for declarations. The existing compiler is the most reliable source.
- **Makefile**: Tree-sitter or a simple line-oriented parser. We mainly need targets, variables, and dependencies.
- **JSON and YAML**: Structural parsing is enough. Extract keys, nested objects, and references to file paths or configuration values. These are useful for configuration and dependency graphs.

Candidate tools:

- **Tree-sitter** for multi-language support. It is incremental, deterministic, and has grammars for Go, TypeScript, JavaScript, SQL, Markdown, and many others. It does not call the network and keeps source local during parsing. This aligns with the repository's offline-first constraints.
- Language-native parsers for cases where more semantic detail is needed. Go's `go/ast` and TypeScript's compiler API provide richer type and declaration information than Tree-sitter. Tree-sitter is faster and more uniform across languages; native parsers are more accurate for specific languages.

What to extract initially:

- Files and packages/modules.
- Declarations: functions, methods, types, structs, interfaces, classes, constants.
- Imports and dependencies between files and packages.
- Call relationships where detectable.
- Documentation comments and JSDoc.
- SQL schema declarations and named queries.
- SOMETHING configuration declarations.

Line numbers and file paths should be stored on nodes so agents can cite exact locations. This directly addresses the requirement for file paths and line numbers.

#### Call Stack Requirement

The graph should expose a call stack for each function so agents can move up or down the call graph easily. This means the extraction layer must capture call edges, not only declarations. For each function or method, the graph should support:

- **Upward traversal**: which functions call this function (callers).
- **Downward traversal**: which functions this function calls (callees).
- **Path queries**: the chain from an entry point to a target function.
- **Cross-language edges**: TypeScript calls into an HTTP API that a Go handler serves, or SQL queries that a Go repository method executes. These are harder to detect statically and may require conventions or configuration.

Call edges are one of the strongest arguments for a dedicated graph database. LadybugDB's Cypher treats them as native path queries, which makes upward and downward traversal straightforward. This is part of why LadybugDB was chosen.

### 2.2 Graph Storage Layer

LadybugDB is the chosen graph storage engine. It is an embedded columnar property graph database with Cypher, full-text search, and vector indices. It ships an official Go binding, `github.com/LadybugDB/go-ladybug`, runs in-process, uses a single database file, and supports ACID transactions. The main cost is that the Go binding uses CGO and links against a C/C++ library, which complicates cross-compilation and static builds. That cost is acceptable because this is separate developer tooling, not a change to the core pipeline binary.

The deciding factors are Cypher queries, built-in vector indices, full-text search, and call-stack traversal. The graph data lives in its own database file, separate from the pipeline metadata database and its migrations.

Build implications to design for:

- The tool should be a separate command, not part of the `analysis` binary. This keeps CGO out of the main pipeline build and out of the main test path.
- The tool should be built on demand through a dedicated Make target, not as part of `make build` or `make check`.
- The C/C++ library should be downloaded or built once into `build/` and ignored by Git. The repository already ignores `build/`.
- The tool is local-only. It is not integrated into CI.
- Cross-compilation is out of scope for the first version. The tool runs on the developer platform where it is built.

### 2.3 RAG Layer (Semantic Context)

Graph topology handles structural queries well. It does not handle semantic similarity. A RAG layer can store embeddings for:

- Documentation paragraphs and headings.
- Doc comments and JSDoc.
- Function bodies and selected code chunks.
- Commit messages or issue descriptions if useful.

Decision: generate embeddings locally only. This preserves the offline build and test constraint. Candidate approaches include a local embedding model served through a local process, a local Python or Node embedding library, or a small on-device model. Because LadybugDB provides native vector indices and full-text search, the graph and vector storage share one engine. Embedding generation should be an explicit, optional step in the update pipeline so that default builds do not require a model download or a long-running process.

Graph and vector retrieval can be combined: first find candidate nodes by graph traversal or keyword search, then rank by embedding similarity. This hybrid approach is more useful than either alone for agent navigation.

### 2.4 Orchestration Layer

The orchestrator detects repository changes and updates the graph and RAG indexes. It should be cheap when nothing changed and incremental when files changed. The core algorithm is checksum-based:

1. Compute checksums for tracked source files and relevant documentation.
2. Compare against a stored manifest in the graph store or a sidecar file.
3. If checksums match, skip the run.
4. If checksums differ, identify added, modified, and deleted files.
5. Re-parse only affected files and update graph nodes and edges.
6. Re-embed only changed documentation and code chunks.
7. Store the new manifest.

This is similar to how `make docs-catalog-update` and `make docs-state-update` already work. The difference is that this pipeline would cover source code relationships rather than prose declarations.

#### Trigger Mechanism

The chosen trigger is a mixed approach: an OpenCode startup check plus an explicit Make target. This combination keeps the workflow local and developer-driven without relying on hidden or automatic processes.

- **OpenCode startup check**: when an OpenCode session starts, the tool runs only the checksum comparison. If the checksums match, it does nothing. If they differ, it either runs a fast incremental update or prompts the user to run the explicit target. This gives a low-friction freshness check without mandatory background work.
- **Explicit Make target**: provide `make code-graph-update` as the authoritative update command. It is what documentation references and what developers run when they want a guaranteed refresh. The target is not part of `make build` or `make check`, so routine commands stay fast.

Git hooks and CI integration are intentionally excluded. Git hooks are easy to bypass and add friction to commit and merge operations. A CI workflow is unnecessary because the graph is optional local tooling and not mandatory for the build. A long-lived file watcher is also excluded from the first version because it adds lifecycle and resource management without a clear need.

This mix preserves predictability and offline operation while giving agents a low-friction freshness check.

### 2.5 Proposed Core Graph Schema

The storage backend is LadybugDB. The graph needs a schema. The first version should cover structural relationships and call stacks. The following is a starting point for discussion.

**Node types:**

- `Repository`: the root node. Properties: name, path.
- `Directory`: a folder. Properties: path.
- `File`: a source or configuration file. Properties: path, language, content hash, last parsed timestamp.
- `Package`: a Go package or TypeScript module. Properties: import path, name.
- `Declaration`: a named declaration. Properties: name, kind (function, method, type, interface, class, constant), file path, start line, end line, documentation.
- `Function`: a specialization of declaration for callable code. Properties: signature, receiver, exported flag, cyclomatic complexity.
- `Type`: a specialization of declaration for types, structs, interfaces, and classes. Properties: underlying type, fields.
- `Table`: a SQL table. Properties: name, schema, columns.
- `Query`: a named SQL query. Properties: name, text, source file, line.
- `Configuration`: a configuration key or block from SOMETHING, YAML, JSON, or Makefile. Properties: key, value, source file, line.

**Edge types:**

- `CONTAINS`: Repository to Directory, Directory to File, File to Declaration. Supports hierarchical navigation.
- `DEFINES`: File to Declaration. Identifies where a declaration lives.
- `IMPORTS`: File to Package or File to File. Captures dependency edges.
- `CALLS`: Function to Function. Captures call stacks. Properties: call site line, call type (direct, method, indirect).
- `IMPLEMENTS`: Type to Interface. Captures interface satisfaction.
- `EXTENDS`: Type to Type. Captures inheritance or embedding.
- `USES`: Declaration to Type. Captures type dependencies.
- `QUERIES`: Function to Table or Function to Query. Captures SQL usage.
- `CONFIGURED_BY`: Declaration or Package to Configuration. Captures configuration dependencies.
- `DOCUMENTED_IN`: Declaration to documentation file or section. Links code to prose.

The `CALLS` edge is the key to the requested call-stack feature. With it, an agent can traverse upward to find callers or downward to find callees. Path queries become a matter of following `CALLS` edges.

This schema is intentionally language-agnostic. Each parser maps its language constructs into these node and edge types. Language-specific details can be stored in properties or as additional node and edge types later.

### 2.6 Schema Evolution and Change Management

The schema will change over time as questions change. The key design question is how to handle that without turning the graph into a maintenance burden. The proposal below separates three concerns that are easy to conflate.

1. **Storage schema**: the node tables and relationship tables in LadybugDB. Changes are DDL and require reindexing.
2. **Extraction contract**: the mapping from parser output to storage schema. Changes require parser or mapper changes and reindexing.
3. **Query catalog**: the questions agents ask and the Cypher that answers them. Changes do not require reindexing.

The central principle is that **the graph is a derived index, not a system of record**. Source files remain authoritative. This means incompatible schema changes do not need migration code. The graph can be deleted and rebuilt from source. The only requirement is that a full rebuild is bounded and acceptable.

#### Approaches Considered

**Approach A: Typed core schema with additive evolution (recommended)**

Keep a small set of stable core node and edge tables. Add new tables or columns for new concepts. Deprecate old tables instead of deleting them. Rebuild on incompatible changes.

- Best for: teams that want readable Cypher, strong typing, and predictable query performance.
- Strengths: Cypher queries are clear and type-checked by the database; indexes and constraints are explicit; schema changes are reviewable as DDL.
- Weaknesses: every new concept needs a DDL change and a parser mapping; too many tables can become unwieldy.
- Risk: schema churn if we add a table for every new question. The query catalog reduces this risk.

**Approach B: Generic property graph**

Use very few node types, such as `Entity`, and edge types, such as `RELATES_TO`, with a `kind` property. Store everything else as properties.

- Best for: rapid experimentation where the data model is not yet known.
- Strengths: no schema churn; new concepts are just new `kind` values.
- Weaknesses: Cypher becomes stringly typed and error-prone; queries cannot rely on type constraints; performance suffers without careful indexing; agents must know the `kind` conventions.
- Risk: the graph becomes an untyped bag of nodes and loses the structural clarity that makes it useful.

**Approach C: Schema-as-data with generated DDL**

Store node and edge type definitions in a SOMETHING or JSON configuration file. Generate the DDL and parser mappings from that file.

- Best for: teams that want schema changes to be reviewable, repeatable, and documented in one place.
- Strengths: one source of truth for node and edge types; generated DDL matches the repository's existing generated-artifact patterns; diffs are easy to review.
- Weaknesses: requires a generator and a meta-schema; adds a layer between the configuration and the database; debugging generated DDL can be harder.
- Risk: the generator itself becomes a maintenance burden if the schema is simple.

**Approach D: Hybrid (recommended)**

Combine a typed core schema with schema-as-data for the type registry, a query catalog, and a rebuild policy.

- The core schema stays typed and stable.
- The type registry declares node and edge types in a configuration file and generates DDL. This makes changes reviewable.
- The query catalog holds agent questions and Cypher separately from the schema.
- The rebuild policy treats the database as disposable and rebuilds on incompatible change.

This gives type safety where it matters, reviewable schema changes, and a clean separation between "what is stored" and "what is asked".

#### Change Categories and Handling

- **New question, existing types**: add a query to the catalog. No reindex.
- **New question, new edge type**: add the edge type to the registry, update the parser or mapper, and reindex affected files. If the change touches cross-file edges, prefer a full rebuild.
- **New question, new node type**: same as above.
- **Changed question**: edit the query in the catalog. No reindex.
- **Removed question**: remove or archive the query in the catalog. No reindex. Leave unused schema types in place.
- **Removed node or edge type**: stop writing it and mark it deprecated in the registry. Keep the table so old queries do not break until the next rebuild. Or bump the schema version and rebuild.
- **Renamed type or property**: avoid in-place rename. Introduce the new name, dual-write if needed, then rebuild and drop the old name. Prefer a full rebuild over in-place rename.
- **New non-queryable attribute**: add a property or store it in an `extras` JSON or string property rather than creating a new type. This prevents schema churn for cosmetic metadata.

#### Schema Versioning

- Store a `SchemaVersion` record in the graph with fields: schema version, parser versions per language, extraction contract version, build timestamp, and source revision.
- The update tool compares the stored version with the expected version. If they differ, it runs a full rebuild instead of an incremental update. This is the same idea as the existing execution fingerprint after migrations in the pipeline.
- Incremental updates are only valid within the same schema and parser version.
- Keep the previous database file until the rebuild succeeds, then swap atomically. This avoids leaving agents with a broken or partial graph.

#### Query Catalog

- Store questions and Cypher in a versioned file under `config/` or `tooling/`, for example `config/code-graph.queries.something` or JSON.
- Each entry has an identifier, the natural-language question, the Cypher text or template, required parameters, and the schema version it targets.
- Agents call a CLI command with the question identifier and parameters. The CLI runs the stored query and returns JSON. This keeps query logic out of prompts and lets questions evolve without schema churn.
- A query can be marked deprecated or removed without touching the database.
- This also gives us a way to test queries: run the catalog against a fixture graph and assert results.

#### Extensibility Rules

- Prefer properties over new node types for descriptive data.
- Prefer new edge types over new node types when the concept is a relationship.
- Use a generic `Annotation` node plus an `ANNOTATED_WITH` edge for experimental or language-specific metadata that is not yet query-critical. Promote it to a typed table only when queries need it.
- Keep core identifiers stable: file path, qualified declaration name, table name, and configuration key. These are the join keys across languages.
- Store source locations as structured properties: file path, start line, start column, end line, end column.

#### Rebuild Policy

- The graph database is disposable. If a schema change is incompatible, delete or move the old database and rebuild from source. Do not invest in in-place migration until rebuild time becomes a real problem.
- Measure full rebuild time early. If it is under a few minutes for this repository, the rebuild policy is sufficient. If it grows, add incremental updates within a schema version and keep rebuild as the fallback.
- LadybugDB supports `ALTER TABLE ... ADD COLUMN`, so additive property changes may be possible without a full rebuild. Additive node or edge tables are also possible. Renames and removals should still use a rebuild.

## 3. Query Interface for Agents

The graph and RAG indexes are only useful if agents can query them. Options:

- **CLI tool**: extend `tooling/go/` or add a new command under `build/` to query nodes, edges, and search results. Agents can run it through Bash. This fits the current tooling pattern.
- **MCP server**: expose the graph as a Model Context Protocol server so agents can query it natively. This is more integrated but adds a protocol dependency and lifecycle management.
- **HTTP API**: reuse the existing server patterns. Heavier than necessary for local use.
- **Direct Cypher access**: agents run Cypher queries against LadybugDB directly. Simple but fragile because query logic and schema details leak into prompts.

Initial recommendation for discussion: start with a small CLI that prints JSON or text summaries. If the workflow proves valuable, wrap it in an MCP server later.

## 4. Incremental Update Design Details

The checksum manifest should include:

- File path.
- File content hash or a hash of the parsed representation.
- Parser version or grammar version, so parser upgrades invalidate old results.
- Timestamp or run identifier for debugging.

When a file changes, the update should handle:

- Deleted nodes and edges from the old version.
- New or changed declarations.
- Changed imports that affect cross-file edges.
- Changed line numbers for unchanged declarations. This matters because moving a function changes its location even if its body is identical. Agents should see current line numbers.

Handling line numbers correctly suggests that the manifest should store both content hashes and position metadata, or the update should refresh positions for files whose line structure changed. A full re-parse of changed files is simpler and likely fast enough.

Schema and parser version changes invalidate incremental updates. The manifest should also store the schema version, extraction contract version, and parser versions. When any of these differ from the expected values, the update tool performs a full rebuild instead of an incremental update. This mirrors the pipeline's behavior where a migration changes the execution fingerprint and forces a new run. See Section 2.6 for the full schema evolution policy.

## 5. Technical Choices and Trade-offs

| Decision | Options | Current direction | Notes |
| --- | --- | --- | --- |
| Parser | Tree-sitter vs native parsers | Mixed per language | Tree-sitter is uniform and fast; native parsers give richer type information. Go and TypeScript can use either; SOMETHING should use the existing compiler. |
| Graph store | LadybugDB | Chosen | Embedded property graph with Cypher, vector indices, and full-text search. CGO and a C/C++ library are acceptable because this is separate tooling. See Section 2.2. |
| Embeddings | Local model vs API | Local only | Preserves offline guarantees. Adds a model dependency and possible startup cost. |
| Trigger | OpenCode startup check plus explicit Make target | Chosen | No git hooks and no CI integration. See Section 2.4. |
| Granularity | File-level vs declaration-level | Declaration-level preferred | Declaration-level is more useful for agents but more complex to maintain. Start with file-level if declaration-level proves too complex. |
| Update strategy | Full rebuild vs incremental | Incremental within a schema version; full rebuild on schema or parser change | Necessary for large repositories but needs careful deletion handling. Rebuild is the fallback for incompatible changes. See Section 2.6. |
| Schema evolution | Typed core, generic property graph, schema-as-data, hybrid | Hybrid proposed | Typed core plus a generated type registry plus a query catalog plus a rebuild policy. See Section 2.6. |

## 6. Relationship to Existing Tooling

The repository already has:

- `docs/PROJECT_CATALOG.md` generated from declarations and comments.
- `make docs-catalog-update` and `make docs-state-update`.
- Go tooling under `tooling/go/` with thin commands.
- Strict offline build and test constraints.
- The pipeline's metadata database as the system of record.

The proposed pipeline should complement these, not replace them. The catalog answers "what exists and where is it documented?" The graph would answer "how are these things connected and what depends on what?" The RAG layer would answer "where is similar logic or guidance?"

The graph database is separate from the pipeline's metadata database. It is disposable and can be rebuilt from source. This keeps the pipeline's system-of-record guarantees intact while allowing the agent tooling schema to evolve independently.

## 7. Open Questions

1. Language scope is decided for now: Go, TypeScript, SQL, SOMETHING, Makefile, JSON, YAML. Which of these should be in the very first prototype, and which can wait?
2. How should the graph handle generated files and ignored directories? Parsing generated code may add noise. The first version should probably exclude generated directories and vendored assets.
3. What is the acceptable latency for a startup checksum check on the full repository? This determines whether a synchronous startup update is viable or whether the mixed approach should always defer to the explicit target.
4. Local embeddings are chosen. Should they cover code, documentation, or both? Code embeddings require chunking strategies that respect declaration boundaries. Documentation embeddings are simpler and may provide most of the value initially.
5. Which schema evolution approach should we adopt: typed core with additive evolution, generic property graph, schema-as-data, or the hybrid? Section 2.6 proposes the hybrid. Do we agree?
6. What query patterns do agents actually need? We should collect real questions before designing the schema. Examples include "what calls this function?", "which files import this package?", and "where is this configuration key used?".
7. Should this be a standalone tool or integrated into the existing `analysis` binary? Standalone keeps the pipeline binary focused; integration reduces duplication.
8. How do we handle SOMETHING parsing? Reuse the existing compiler in `backend/something/` or write a lightweight extraction layer that only captures declarations and references.
9. Where should the query catalog live: SOMETHING configuration, JSON, or YAML? How should queries be versioned and tested?
10. What is the full rebuild time for this repository? This determines whether the rebuild policy is acceptable or whether incremental updates within a schema version are necessary.
11. Should schema changes be generated from a SOMETHING or JSON type registry, or hand-written as DDL? The hybrid approach suggests generated DDL, but hand-written may be simpler for a small schema.

## 8. Next Steps for Discussion

The following proposal is ordered so that each step produces something usable and testable. LadybugDB is chosen, so the storage decision is settled.

1. Validate the schema against real agent questions. Collect 10 to 15 questions an agent would actually ask. Examples: "What calls this function?", "Which files import this package?", "Which handlers use this table?", "Where is this configuration key used?". Map each question to the node and edge types in Section 2.5. If the schema cannot answer a question without a new edge type, adjust the schema now.
2. Prove LadybugDB in this environment. Build or download the C/C++ library into `build/`, write a minimal Go program that opens a database, creates a node table and a relationship table, and runs a one-hop and multi-hop Cypher query. Confirm the build and query path works before writing the extraction layer.
3. Prototype the extraction layer for one language. Start with Go because the repository's native parser and tooling make it the easiest to validate. Produce node and edge records in memory and print them as JSON. Do not write to LadybugDB yet. This proves the schema and parser contract without storage complexity.
4. Load the Go extraction output into LadybugDB. Create the Cypher schema from the JSON records and verify that call-stack queries return the expected results. This closes the loop from parser to graph query.
5. Implement the checksum manifest and incremental update logic. Start with a simple manifest file and a full-rebuild fallback. Add schema and parser version checks so incompatible changes force a rebuild. Add incremental updates only after the full rebuild works.
6. Define the query catalog. Store the validated agent questions and their Cypher templates in a versioned configuration file. Add tests that run each query against a fixture graph and assert results.
7. Build the query CLI. A small command that accepts a catalog entry identifier and parameters and prints results as JSON. This is the agent-facing interface.
8. Add local embeddings and RAG retrieval. Start with documentation paragraphs only. Add code embeddings after the graph query path is stable. Use LadybugDB's native vector index.
9. Integrate the mixed trigger approach. Add the explicit Make target first. Add the OpenCode startup checksum check second. Do not add git hooks or CI integration.

Steps 1 through 4 are the critical path. They validate the schema, prove the storage, and close the parser-to-query loop. The remaining steps can follow incrementally.

## 9. Decision Log

| Date | Topic | Decision | Rationale |
| --- | --- | --- | --- |
| 2026-11-01 | Initial scope | Go, TypeScript, SQL, SOMETHING, Makefile, JSON, YAML | Covers the languages and formats used in the repository. |
| 2026-11-01 | Storage backend | LadybugDB | Cypher queries and vector indices. CGO and a C/C++ library are acceptable for separate tooling. See Section 2.2. |
| 2026-11-01 | Embedding strategy | Local only | Preserves offline build and test constraints. |
| 2026-11-01 | Trigger mechanism | Mixed approach | Startup checksum check plus explicit Make target. See Section 2.4. |
| 2026-11-01 | Schema evolution | Hybrid proposed | Typed core plus generated type registry plus query catalog plus rebuild policy. Treats the graph as a disposable derived index. See Section 2.6. |
