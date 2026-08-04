# Database Reference

## 1. Purpose and authority

This document describes the current SQLite schema after every configured migration has been applied. It covers every maintained table in the metadata database and companion PDF database, their columns and SQL defaults, foreign-key and logical relationships, repository expectations, integrity controls, and operational role. Use [ARCHITECTURE.md](ARCHITECTURE.md) for end-to-end system behavior, [STANDARDS.md](STANDARDS.md) for migration and persistence rules, [PROJECT-USAGE.md](PROJECT-USAGE.md) for supported commands, and [PROJECT_CATALOG.md](PROJECT_CATALOG.md) for repository functions and source locations.

The authoritative schema sources are [config/database.something](../config/database.something), [config/database.corpus.metadata.something](../config/database.corpus.metadata.something), [config/database.corpus.pdf.something](../config/database.corpus.pdf.something), [migrations/corpus.metadata/](../migrations/corpus.metadata/), and [migrations/corpus.pdf/](../migrations/corpus.pdf/). Repository code under [src/database/](../src/database/) and [src/pdfstore/](../src/pdfstore/) is authoritative for application-level validation and update behavior that SQLite does not encode.

The metadata database contains 27 application tables plus `schema_migrations`. The PDF database contains five application tables plus its independent `schema_migrations`. SQLite also creates the internal `sqlite_sequence` table for `AUTOINCREMENT` keys; it is implementation state rather than a project table and is not part of the documented application contract.

In the column summaries below, `NULL` means the column is nullable, and a default is shown only when the DDL declares one. A column without an explicit `DEFAULT` has no SQL default even when repository code normally supplies a value. `PK` means primary key, `FK` means foreign key, and all foreign keys use SQLite's default `NO ACTION` behavior because the migrations declare no cascading action.

## 2. Database bundle and opening behavior

| Database | Normal filename | Migration chain | Owner | Current role |
|---|---|---|---|---|
| Metadata | `corpus.metadata.db` | V00001-V00021 under `migrations/corpus.metadata/` | `src/database/` and `src/workspace/` | System of record for configuration identity, attempts, source evidence, immutable corpus revisions and relationships, artifacts, cache, metrics, audit, and the PDF-store binding. |
| PDF | `corpus.pdf.db` | V00001-V00002 under `migrations/corpus.pdf/` | `src/pdfstore/` | Portable companion inventory for normalized DOIs, content-addressed validated PDF bytes, and cross-database audit delivery. |

`config/database.something` selects the two migration configurations independently. Writable opening creates the parent directory, enables WAL, sets a 5,000 millisecond busy timeout, enables foreign keys on every pooled connection, and applies configured migrations in SOMETHING declaration order. Tracking-table creation and each individual migration use `BEGIN IMMEDIATE` to serialize concurrent schema changes.

Migration identity is the filename. An already recorded filename is skipped, and its stored checksum is not revalidated on later opens. The `previous` and `upgrade` configuration fields document chain adjacency but do not determine execution order. There is no runtime downgrade command even though migration files contain `-- ==DOWN==` sections.

The viewer opens an existing metadata database with `mode=ro` and `query_only`, never creates a database, and never applies migrations. If `pdf_store_binding` exists and is valid, the viewer opens the companion PDF database read-only as well.

Metadata and PDF files form one portable bundle after PDF inventory begins. `pdf_store_binding.relative_path` is resolved relative to the metadata database directory, while `works.doi`, `pdf_documents.doi`, and `pdf_audit_outbox.pipeline_run_id` are cross-file logical references that SQLite cannot enforce as foreign keys.

## 3. Relationship overview

### 3.1 Metadata planning, execution, and evidence

```text
searches 1 ----< search_revisions 1 ----< execution_plans 1 ----< pipeline_runs
pipeline_runs 1 ----< run_sources 1 ----< source_records
pipeline_runs 1 ----< source_filter_counts ... logical (run, source_name) ... run_sources
pipeline_runs 1 ----< run_steps ---- input/output artifact ----> artifacts 1 ---- 0..1 artifact_blobs
pipeline_runs <----- run_steps ---- reused_from_run_id
pipeline_runs 1 ----< run_artifacts >---- 1 artifacts
pipeline_runs 1 ----< pipeline_run_metrics
pipeline_runs 1 ----< audit_events
pipeline_runs 1 ----< run_cache_uses >---- 1 cache_entries >---- 0..1 artifacts
```

`searches`, `search_revisions`, and `execution_plans` define reusable intent and exact execution identity. `pipeline_runs` records numbered attempts. Every run-owned evidence table points back to the attempt directly or through another run-owned row, while content-addressed artifacts and cache entries may be shared across attempts.

### 3.2 Corpus and identity model

```text
pipeline_runs 1 ----< work_revisions >---- 1 works
      |                    |                    |
      |                    +----< authorships   +----< work_identifiers
      |                    |          |
      |                    |          v
      |                    |    author_occurrences >---- 0..1 people
      |                    |
      |                    +----< reference_mentions >---- 0..1 resolved works
      |
      +----< run_work_stages >---- works
      |
      +----< author_identity_resolutions >---- author_occurrences
                                |
                                +----< author_identity_candidates >---- 0..1 artifacts
```

`works` supplies stable identity, while `work_revisions` supplies immutable stage snapshots. Authorships and reference mentions belong to a specific revision rather than directly to a work. Stage outcomes belong to the run and work so discarded or failed processing remains inspectable without manufacturing an accepted revision.

### 3.3 Companion PDF inventory and audit delivery

```text
metadata database                                      PDF database
-----------------                                      ------------
works.doi ........................ logical DOI .......> pdf_documents.doi
                                                            |
pdf_store_binding.relative_path ---- opens file ------------+
                                                            |
                                                            v
                                                       pdf_blobs

pipeline_runs.id ................ logical run ......... pdf_audit_outbox.pipeline_run_id
audit_events <---- pdf_audit_links <---- event_key .... pdf_audit_outbox
     ^                                                        |
     +---------------- flush undelivered event ----------------+
```

PDF registration and byte storage commit with a `pdf_audit_outbox` row in the PDF transaction. Flushing creates one metadata `audit_events` row and one `pdf_audit_links` delivery record in a metadata transaction, then marks the PDF outbox row delivered. The event key makes retries idempotent across a crash between those transactions.

## 4. Shared storage conventions

- Integer `AUTOINCREMENT` primary keys are generated by SQLite; text primary keys are supplied by the application.
- Timestamps are stored as `TEXT`. SQL defaults use UTC `datetime('now')`; repository-supplied timestamps normally use UTC RFC 3339 with nanosecond support. Consumers must treat them as timestamps rather than assume one textual precision.
- Hash columns hold lowercase hexadecimal SHA-256 values when written through supported code. Only fields with an explicit length check enforce 64 characters in SQLite, and length checks do not independently prove hexadecimal content or recompute the digest.
- Metadata JSON and serialized fields are `TEXT`. The PDF schema applies `json_valid` to `settings_json`, `selected_workspaces`, and `pdf_audit_outbox.metadata_json`; metadata-side JSON fields and `source_filter_counts.filter_data` rely on supported writers and do not have SQL JSON checks.
- Boolean configuration is stored as SQLite `INTEGER`; `execution_plans.enrichment_enabled` defaults to `0`, and supported code writes `0` or `1`, but the schema does not add a boolean check.
- Foreign keys are enforced on writable pooled connections. No table declares `ON DELETE CASCADE`, so parent removal must be explicitly ordered and will fail while dependent rows remain.
- Tables described as immutable or append-only have the exact enforcement stated in their entry. Some content-addressed tables are treated as immutable by repository behavior but do not have update or delete triggers.

## 5. Metadata database tables

### 5.1 Migration and execution identity

#### `schema_migrations`

- Purpose: Records which configured metadata migrations have been applied.
- Columns and defaults: `filename TEXT PK`; `applied_at TEXT NOT NULL DEFAULT datetime('now')`; `checksum TEXT NOT NULL`.
- Relationships and expectations: The table has no foreign keys. Migration lookup uses `filename`, and `SchemaVersion` returns the most recently inserted filename by row order. The checksum is recorded at application time but is not revalidated when that filename is encountered again.

#### `searches`

- Purpose: Stores one stable research-search identity independent of revisions and attempts.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `search_id TEXT NOT NULL UNIQUE`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: One search owns many `search_revisions`. Repeating a supported create with the same `search_id` returns the existing row.

#### `search_revisions`

- Purpose: Stores one researcher-managed revision label and the latest observed declaration hashes for a search.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `search_id INTEGER NOT NULL FK searches.id`; `revision_label TEXT NOT NULL`; `config_artifact_hash TEXT NOT NULL`; `resolved_manifest_hash TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `updated_at TEXT NULL`; `UNIQUE(search_id, revision_label)`.
- Relationships and expectations: One revision owns many `execution_plans`. A repeated label with changed configuration or manifest hashes updates this row and sets `updated_at`; exact historical inputs remain frozen through execution plans, run artifact links, and attempt evidence. A newly inserted current row normally has `updated_at` as `NULL` until its hashes change.

#### `execution_plans`

- Purpose: Stores a reusable, immutable execution identity for one search revision and input policy.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `search_revision_id INTEGER NOT NULL FK search_revisions.id`; `execution_fingerprint TEXT NOT NULL`; `resolved_manifest_hash TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `input_manifest_hash TEXT NOT NULL DEFAULT ''`; `enrichment_enabled INTEGER NOT NULL DEFAULT 0`; `UNIQUE(search_revision_id, execution_fingerprint)`.
- Relationships and expectations: One plan owns numbered `pipeline_runs`. Supported workspace creation requires a nonempty input-manifest hash and treats a reused fingerprint with a different resolved-manifest hash as an integrity error. The lower-level `ExecutionPlanRepository.Create` method can write the empty SQL default when no input manifest is supplied.

#### `pipeline_runs`

- Purpose: Stores one execution attempt, its lifecycle, and its reversible visibility state.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `step TEXT NOT NULL`; `started_at TEXT NOT NULL`; `finished_at TEXT NULL`; `status TEXT NOT NULL DEFAULT 'running'`; `summary TEXT NULL`; `search_query TEXT NULL`; `execution_plan_id INTEGER NULL FK execution_plans.id`; `attempt_number INTEGER NULL`; `visibility_state TEXT NOT NULL DEFAULT 'active'`; `trashed_at TEXT NULL`; `trash_reason TEXT NULL`; `created_at TEXT NULL DEFAULT datetime('now')`; `UNIQUE(execution_plan_id, attempt_number)` through an explicit unique index.
- Relationships and expectations: Current attempts link to a plan and receive a positive per-plan attempt number atomically; nullable plan and attempt fields support the lower-level unplanned run entry point. Repository-accepted statuses are `running`, `completed`, and `failed`; terminal states set `finished_at`. Visibility vocabulary is `active`, `archived`, and `trashed`; trash and restore update the timestamp and reason consistently. The schema does not check those vocabularies.

### 5.2 Source ingestion evidence

#### `run_sources`

- Purpose: Captures each declared source as used by one attempt, including file and export-count provenance.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `source_name TEXT NOT NULL`; `source_type TEXT NOT NULL`; `expected_file TEXT NOT NULL`; `query TEXT NULL`; `requested_fields TEXT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `expected_result_count INTEGER NULL CHECK >= 0`; `observed_result_count INTEGER NULL CHECK >= 0`; `result_count_comparison TEXT NULL CHECK IN ('match', 'below', 'above')`; `export_date TEXT NULL`; `UNIQUE(pipeline_run_id, source_name)`.
- Relationships and expectations: One row owns many `source_records`. Expected count and export date come from configuration; observed count and comparison are written after reading the export. The comparison is informational provenance and does not by itself accept or reject the run.

#### `source_records`

- Purpose: Preserves one raw source record and its parse result.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `run_source_id INTEGER NOT NULL FK run_sources.id`; `record_index INTEGER NOT NULL`; `raw_payload TEXT NOT NULL`; `content_hash TEXT NOT NULL`; `parse_status TEXT NOT NULL DEFAULT 'pending'`; `reject_reason TEXT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: Records are listed in `record_index` order. Supported processing writes statuses such as `parsed`, `accepted`, or `rejected` according to the ingest path and supplies a rejection reason when applicable; neither the vocabulary nor index uniqueness is constrained by SQL. Raw evidence remains available even when parsing rejects the record.

#### `source_filter_counts`

- Purpose: Stores the ordered cumulative filter stages and retained article counts for one named source in one attempt.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `source_name TEXT NOT NULL`; `filter_data TEXT NOT NULL DEFAULT '[]'`; `UNIQUE(pipeline_run_id, source_name)`.
- Relationships and expectations: `source_name` logically matches the corresponding `run_sources.source_name`, but there is no composite foreign key. Supported writers upsert a JSON array of objects shaped as `{filters: string[], count: int}`; SQLite does not validate that JSON.

### 5.3 Artifacts, steps, metrics, cache, and audit

#### `artifacts`

- Purpose: Stores content identity and media metadata for a unique pipeline artifact.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `content_hash TEXT NOT NULL UNIQUE`; `byte_size INTEGER NOT NULL`; `content_type TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: Artifact bytes live in `artifact_blobs`; plans refer to manifest hashes, while attempts and stages link artifact IDs through `run_artifacts`, `run_steps`, cache entries, and identity candidates. Supported writers use SHA-256, return the existing ID for duplicate content, and expect `byte_size` and `content_type` to describe the blob. The table has no hash-length, size, update, or delete trigger.

#### `artifact_blobs`

- Purpose: Stores artifact bytes inline and records the attempt that first persisted them.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `artifact_id INTEGER NOT NULL UNIQUE FK artifacts.id`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `data BLOB NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: There is at most one blob per artifact. Because content is deduplicated, `pipeline_run_id` identifies the first writer rather than every consumer; consumers are represented by step, run-artifact, cache, or candidate links. Repository methods only create and read blobs, but SQL has no immutability trigger and no length check against `artifacts.byte_size`.

#### `run_artifacts`

- Purpose: Links each attempt to its exact configuration snapshots without duplicating content-addressed artifacts.
- Columns and defaults: `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `artifact_id INTEGER NOT NULL FK artifacts.id`; `artifact_role TEXT NOT NULL CHECK IN ('workspace_config', 'resolved_manifest', 'input_manifest')`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `PK(pipeline_run_id, artifact_role)`.
- Relationships and expectations: A run has at most one artifact for each role. Repeating the same link is idempotent, while trying to assign an existing role to a different artifact is rejected by repository code.

#### `run_steps`

- Purpose: Records execution, artifacts, fingerprints, and reuse for one named stage in an attempt.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `step_name TEXT NOT NULL`; `step_status TEXT NOT NULL DEFAULT 'pending'`; `input_artifact_id INTEGER NULL FK artifacts.id`; `output_artifact_id INTEGER NULL FK artifacts.id`; `reused_from_run_id INTEGER NULL FK pipeline_runs.id`; `started_at TEXT NULL`; `finished_at TEXT NULL`; `input_fingerprint TEXT NOT NULL DEFAULT ''`; `output_fingerprint TEXT NOT NULL DEFAULT ''`; `UNIQUE(pipeline_run_id, step_name)`.
- Relationships and expectations: Repository status vocabulary is `pending`, `running`, `completed`, `skipped`, `reused`, and `failed`; terminal statuses set `finished_at`, and reuse also links the prior run. The schema does not check status vocabulary or require artifacts and fingerprints for a particular status.

#### `pipeline_run_metrics`

- Purpose: Stores nonnegative counter snapshots by attempt, metric name, and optional source dimension.
- Columns and defaults: `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `metric TEXT NOT NULL`; `source TEXT NOT NULL DEFAULT ''`; `value INTEGER NOT NULL CHECK >= 0`; `PK(pipeline_run_id, metric, source)`.
- Relationships and expectations: Empty `source` means a whole-run metric; provider, input source, or field names supply scoped dimensions. Setting the same key replaces its value, so rows represent the latest recorded counter rather than an append-only series.

#### `cache_entries`

- Purpose: Stores a versioned provider-response cache entry shared across attempts.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `provider TEXT NOT NULL`; `namespace TEXT NOT NULL`; `request_fingerprint TEXT NOT NULL`; `response_status INTEGER NOT NULL`; `payload_artifact_id INTEGER NULL FK artifacts.id`; `fetched_at TEXT NOT NULL`; `expires_at TEXT NULL`; `extractor_version TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `updated_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(provider, namespace, request_fingerprint, extractor_version)`.
- Relationships and expectations: Upsert updates the response in place so `run_cache_uses` retains a stable row ID. Repository validation requires nonblank identity fields and an HTTP status from 100 through 599. A missing payload artifact is valid for negative responses such as 404; cache policy, not SQL, interprets expiry and reusability.

#### `run_cache_uses`

- Purpose: Records which concrete cache entry an attempt consulted or consumed and how that lookup resolved.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `cache_entry_id INTEGER NOT NULL FK cache_entries.id`; `cache_layer TEXT NOT NULL`; `outcome TEXT NOT NULL`; `used_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: Repository-accepted outcomes are `hit`, `miss`, `negative`, and `stale`; layers include current configured names such as `active_run` and `global`. SQL does not constrain either vocabulary. Global availability is established by a recorded `global` use rather than a flag on `cache_entries`.

#### `audit_events`

- Purpose: Stores the append-only audit stream for pipeline, enrichment, validation, lifecycle, and delivered PDF events.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `occurred_at TEXT NOT NULL`; `actor TEXT NOT NULL`; `pipeline_run_id INTEGER NULL FK pipeline_runs.id`; `entity_type TEXT NOT NULL`; `entity_id TEXT NOT NULL`; `action TEXT NOT NULL`; `before_json TEXT NULL`; `after_json TEXT NULL`; `metadata_json TEXT NULL`; `correlation_id TEXT NULL`.
- Relationships and expectations: Update and delete triggers make every row append-only. Repository insertion validates actions against the manifest vocabulary; the JSON-shaped columns are not checked with `json_valid`. `pipeline_run_id` may be `NULL` for events outside an attempt, and correlation IDs support tracing but are not unique.

### 5.4 Corpus revisions and stage state

#### `works`

- Purpose: Stores stable work identity separately from changing metadata snapshots.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `doi TEXT NULL UNIQUE`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: One work owns identifiers, revisions, run-stage outcomes, and inbound resolved references. Supported DOI creation trims whitespace, lowercases, removes `http://doi.org/` or `https://doi.org/`, and reuses an existing normalized DOI. A work without a DOI is always inserted as a distinct row and is never globally merged by title alone.

#### `work_identifiers`

- Purpose: Stores non-DOI identifiers scoped by namespace for a work.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `work_id INTEGER NOT NULL FK works.id`; `namespace TEXT NOT NULL`; `identifier TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(namespace, identifier)`.
- Relationships and expectations: The namespace and identifier pair has one owner across the corpus. Repeating the pair for the same work returns the existing row; attempting to assign it to another work is rejected.

#### `work_revisions`

- Purpose: Stores an immutable typed metadata snapshot produced by one pipeline stage in one attempt.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `work_id INTEGER NOT NULL FK works.id`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `field_schema_version TEXT NOT NULL DEFAULT '1'`; `payload_hash TEXT NOT NULL`; `title TEXT NULL`; `abstract TEXT NULL`; `year INTEGER NULL`; `journal TEXT NULL`; `publisher TEXT NULL`; `source TEXT NULL`; `keywords TEXT NULL`; `keywords_plus TEXT NULL`; `citation_count INTEGER NULL`; `reference_count INTEGER NULL`; `extension_data TEXT NULL`; `producer_stage TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: Update and delete triggers make revisions append-only. Supported new producer stages are `parse`, `deduplicate`, `validate`, `enrich`, `enrich_metadata`, `enrich_identity`, and `normalize`; repository code rejects an empty or unknown value. `keywords` and `keywords_plus` are JSON arrays and `extension_data` is a JSON object by application contract, without SQL JSON checks. The payload hash covers content fields and excludes producer-stage provenance.

#### `run_work_stages`

- Purpose: Stores the current outcome of each work at each stage in a particular attempt, including failures and discarded work that may have no accepted normalized revision.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `work_id INTEGER NOT NULL FK works.id`; `stage_name TEXT NOT NULL`; `outcome TEXT NOT NULL`; `reason TEXT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `updated_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(pipeline_run_id, work_id, stage_name)`.
- Relationships and expectations: Repository upsert preserves `id` and `created_at` while replacing `outcome`, `reason`, and `updated_at`. Valid pairs are `parse` with `parsed`, `skipped`, or `pending`; `deduplicate` with `duplicate`, `deduplicated`, `skipped`, or `pending`; `validate` with `valid`, `discarded`, `skipped`, or `pending`; `enrich` and `enrich_metadata` with `enriched`, `skipped`, or `pending`; `enrich_identity` with `enriched`, `failed`, `skipped`, or `pending`; and `normalize` with `normalized`, `skipped`, or `pending`. SQL does not encode this matrix.

### 5.5 Authors, identity evidence, and references

#### `people`

- Purpose: Stores confirmed global author identities keyed by a strong ORCID signal.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `orcid TEXT UNIQUE`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: Insert and ORCID-update triggers reject `NULL`, empty, or whitespace ORCIDs even though the column declaration lacks `NOT NULL`. Repository creation normalizes case and whitespace and validates the `XXXX-XXXX-XXXX-XXXX` shape plus ISO 7064 MOD 11-2 checksum. Author observations link here only when that validation succeeds.

#### `author_occurrences`

- Purpose: Preserves author data as observed at one point in pipeline processing without merging uncertain names globally.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `person_id INTEGER NULL FK people.id`; `citation_name TEXT NOT NULL`; `first_name TEXT NULL`; `last_name TEXT NULL`; `orcid TEXT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`.
- Relationships and expectations: Update and delete triggers make occurrences append-only. A valid observed ORCID creates or reuses a `people` row and sets `person_id`; an invalid raw ORCID may remain in `orcid` as evidence but does not establish identity. ORCID-less occurrences with the same name remain distinct.

#### `authorships`

- Purpose: Connects an immutable work revision to an observed author while preserving order and affiliation.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `work_revision_id INTEGER NOT NULL FK work_revisions.id`; `author_occurrence_id INTEGER NOT NULL FK author_occurrences.id`; `author_order INTEGER NOT NULL`; `affiliation TEXT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(work_revision_id, author_occurrence_id)`; `UNIQUE(work_revision_id, author_order)`.
- Relationships and expectations: Update and delete triggers make authorships append-only. Repository creation requires an order of at least one, but SQL does not check positivity. Corrections require a new work revision and a new authorship set.

#### `author_identity_resolutions`

- Purpose: Records the outcome of evaluating one author occurrence against an identity provider for one attempt.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `pipeline_run_id INTEGER NOT NULL FK pipeline_runs.id`; `author_occurrence_id INTEGER NOT NULL FK author_occurrences.id`; `status TEXT NOT NULL CHECK IN ('orcid_is_unclear', 'no_orcid_candidate', 'provider_failed', 'confirmed', 'rejected')`; `provider TEXT NOT NULL`; `queried_citation_name TEXT NOT NULL`; `error_message TEXT NULL`; `resolved_at TEXT NOT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(pipeline_run_id, author_occurrence_id, provider)`.
- Relationships and expectations: Name-search results are evidence rather than proof and remain separate from `people`. Repository code validates required IDs, status, provider, citation name, and resolution timestamp. The supported repository only inserts and reads these rows, but SQL has no append-only trigger.

#### `author_identity_candidates`

- Purpose: Stores each provider-returned possible ORCID attached to one identity-resolution record.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `identity_resolution_id INTEGER NOT NULL FK author_identity_resolutions.id`; `candidate_orcid TEXT NOT NULL`; `provider_display_name TEXT NULL`; `query_url TEXT NOT NULL`; `payload_artifact_id INTEGER NULL FK artifacts.id`; `provider_rank INTEGER NOT NULL CHECK >= 1`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(identity_resolution_id, candidate_orcid)`; `UNIQUE(identity_resolution_id, provider_rank)`.
- Relationships and expectations: Candidates deliberately have no `person_id`; confirmation is a separate decision. The optional artifact preserves the provider payload, while unique candidate and rank constraints preserve a deterministic result set. The supported repository only inserts these rows, but SQL has no append-only trigger.

#### `reference_mentions`

- Purpose: Stores the ordered immutable citation snapshot attached to one work revision.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `work_revision_id INTEGER NOT NULL FK work_revisions.id`; `resolved_work_id INTEGER NULL FK works.id`; `mention_order INTEGER NOT NULL`; `raw_reference TEXT NULL`; `doi TEXT NULL`; `title TEXT NULL`; `author TEXT NULL`; `year INTEGER NULL`; `source TEXT NULL`; `created_at TEXT NOT NULL DEFAULT datetime('now')`; `UNIQUE(work_revision_id, mention_order)`.
- Relationships and expectations: Update and delete triggers make mentions append-only. Repository creation requires a positive order, normalizes DOI, and sets `resolved_work_id` when the DOI already identifies a work. Identical external references may remain separate ordered mentions; SQL does not check order positivity.

### 5.6 PDF bundle and delivery links

#### `pdf_store_binding`

- Purpose: Binds one metadata corpus to one portable companion PDF database.
- Columns and defaults: `id INTEGER PK CHECK id = 1`; `relative_path TEXT NOT NULL CHECK nonblank`; `configured_at TEXT NOT NULL`; `config_fingerprint TEXT NOT NULL CHECK length = 64`.
- Relationships and expectations: The fixed key permits at most one binding. Supported writers default to `corpus.pdf.db`, preserve an existing binding, reject absolute paths, reject traversal outside the metadata directory, and reject the metadata file itself as the companion. The fingerprint is SHA-256 over the binding identity and path.

#### `pdf_audit_links`

- Purpose: Records the metadata audit row created for each delivered PDF outbox event.
- Columns and defaults: `event_key TEXT PK`; `audit_event_id INTEGER NOT NULL FK audit_events.id`; `created_at TEXT NOT NULL`.
- Relationships and expectations: `event_key` logically equals `pdf_audit_outbox.event_key` in the companion database, but cross-file foreign keys are impossible. The primary key prevents duplicate audit insertion when delivery is retried; link insertion and the referenced metadata audit event commit together.

## 6. PDF database tables

### 6.1 Migration state

#### `schema_migrations`

- Purpose: Records which configured PDF migrations have been applied independently of metadata migrations.
- Columns and defaults: `filename TEXT PK`; `applied_at TEXT NOT NULL DEFAULT datetime('now')`; `checksum TEXT NOT NULL`.
- Relationships and expectations: Behavior matches the metadata tracking table. Applying one database's chain does not imply that the other file has been opened or migrated.

### 6.2 Inventory and content

#### `pdf_documents`

- Purpose: Stores one normalized DOI's current PDF availability state.
- Columns and defaults: `doi TEXT PK CHECK nonblank`; `status TEXT NOT NULL CHECK IN ('not_available', 'available')`; `content_hash TEXT NULL FK pdf_blobs.content_hash`; `inventoried_at TEXT NULL`; `updated_at TEXT NOT NULL`; row check requires `available` to have both content hash and inventory timestamp and requires `not_available` to have neither.
- Relationships and expectations: Pipeline normalization registers a DOI idempotently as `not_available`. Manual addition requires prior registration, validates the bytes, inserts or reuses a blob, and atomically changes the row to `available`. An already available DOI is left unchanged, so supported code does not replace its selected PDF.

#### `pdf_blobs`

- Purpose: Stores validated PDF bytes once by content hash.
- Columns and defaults: `content_hash TEXT PK CHECK length = 64`; `byte_size INTEGER NOT NULL CHECK > 0`; `data BLOB NOT NULL CHECK length(data) = byte_size`; `created_at TEXT NOT NULL`.
- Relationships and expectations: `pdf_documents` and the retained download-attempt evidence can reference a blob. An update trigger makes bytes and metadata immutable; there is no delete trigger, and current runtime exposes no blob deletion path. Supported manual insertion validates a maximum of 20,000,000 bytes, a `%PDF-` signature, and SHA-256 identity before writing.

### 6.3 Audit outbox

#### `pdf_audit_outbox`

- Purpose: Durably queues PDF mutations for idempotent delivery into metadata audit.
- Columns and defaults: `event_key TEXT PK`; `occurred_at TEXT NOT NULL`; `actor TEXT NOT NULL`; `entity_type TEXT NOT NULL`; `entity_id TEXT NOT NULL`; `action TEXT NOT NULL`; `metadata_json TEXT NOT NULL CHECK json_valid`; `correlation_id TEXT NOT NULL`; `delivered_at TEXT NULL`; `pipeline_run_id INTEGER NULL`.
- Relationships and expectations: Registration and inventory transactions insert the outbox row before commit. `pipeline_run_id` logically references metadata `pipeline_runs.id` but cannot have a cross-file foreign key. Undelivered rows have `delivered_at IS NULL`; flushing uses `pdf_audit_links.event_key` to reuse an already inserted metadata event, then sets the delivery timestamp in the PDF database.

### 6.4 Present schema tables without a current production writer

#### `pdf_gather_runs`

- Purpose: Stores bounded settings and lifecycle metadata for a PDF gathering run when such rows are supplied.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `correlation_id TEXT NOT NULL UNIQUE`; `started_at TEXT NOT NULL`; `completed_at TEXT NULL`; `status TEXT NOT NULL CHECK IN ('running', 'completed', 'failed')`; `settings_fingerprint TEXT NOT NULL CHECK length = 64`; `settings_json TEXT NOT NULL CHECK json_valid`; `selected_workspaces TEXT NOT NULL CHECK json_valid`.
- Relationships and expectations: One row can own many `pdf_download_attempts`. The current production PDF workflow is normalized DOI registration plus validated manual insertion and does not create, read, or update gathering-run rows. The table remains part of the current migrated schema and therefore remains visible to the read-only table browser.

#### `pdf_download_attempts`

- Purpose: Stores one append-only download-attempt result associated with a gathering run when such evidence is supplied.
- Columns and defaults: `id INTEGER PK AUTOINCREMENT`; `gather_run_id INTEGER NOT NULL FK pdf_gather_runs.id`; `doi TEXT NOT NULL`; `source TEXT NULL`; `resolved_url TEXT NULL CHECK length <= 2048`; `started_at TEXT NOT NULL`; `completed_at TEXT NULL`; `outcome TEXT NOT NULL`; `http_status INTEGER NULL`; `error_class TEXT NULL`; `error_message TEXT NULL CHECK length <= 1000`; `content_hash TEXT NULL FK pdf_blobs.content_hash`.
- Relationships and expectations: Update and delete triggers make attempt rows append-only. The schema does not constrain outcome or HTTP status vocabulary. The current production PDF workflow does not create or read these rows, but existing rows remain part of the current schema and can reference retained content blobs.

## 7. Explicit indexes and triggers

Primary keys and `UNIQUE` constraints create SQLite autoindexes. The following table lists only explicitly named secondary indexes and project triggers, which are part of the current query and integrity behavior.

| Database table | Explicit secondary indexes | Triggers |
|---|---|---|
| Metadata `pipeline_runs` | Unique `(execution_plan_id, attempt_number)` | None |
| Metadata `search_revisions` | `search_id` | None |
| Metadata `execution_plans` | `execution_fingerprint`, `input_manifest_hash`, `enrichment_enabled` | None |
| Metadata `source_records` | `run_source_id`, `content_hash` | None |
| Metadata `artifacts` | None | None |
| Metadata `artifact_blobs` | `pipeline_run_id`, `artifact_id` | None |
| Metadata `run_artifacts` | `artifact_id` | None |
| Metadata `run_steps` | `input_artifact_id`, `output_artifact_id`, `input_fingerprint` | None |
| Metadata `audit_events` | `pipeline_run_id`, `(entity_type, entity_id)`, `action`, `correlation_id` | Reject update and delete |
| Metadata `work_identifiers` | `work_id` | None |
| Metadata `work_revisions` | `work_id`, `pipeline_run_id` | Reject update and delete |
| Metadata `run_work_stages` | `pipeline_run_id`, `work_id` | None |
| Metadata `author_occurrences` | `person_id`, `orcid` | Reject update and delete |
| Metadata `people` | None | Reject blank or null ORCID insert and ORCID update |
| Metadata `authorships` | `work_revision_id`, `author_occurrence_id` | Reject update and delete |
| Metadata `author_identity_resolutions` | `pipeline_run_id`, `author_occurrence_id` | None |
| Metadata `author_identity_candidates` | `identity_resolution_id`, `candidate_orcid` | None |
| Metadata `reference_mentions` | `work_revision_id`, `resolved_work_id` | Reject update and delete |
| Metadata `cache_entries` | `expires_at`, `payload_artifact_id` | None |
| Metadata `run_cache_uses` | `pipeline_run_id`, `cache_entry_id` | None |
| PDF `pdf_blobs` | None | Reject update |
| PDF `pdf_documents` | None | None |
| PDF `pdf_download_attempts` | `(gather_run_id, id)`, `(doi, id)`, `(source, outcome)` | Reject update and delete |
| PDF `pdf_audit_outbox` | `(delivered_at, occurred_at)` | None |

Tables omitted from this index table have no explicit named secondary index and no project trigger beyond their primary-key and unique-constraint autoindexes.

## 8. End-to-end expectations

1. Preflight resolves configuration and source identities, creates or reuses `searches`, `search_revisions`, and `execution_plans`, links exact configuration artifacts, and starts a numbered `pipeline_runs` attempt only when execution is required.
2. Ingestion creates `run_sources`, `source_records`, `source_filter_counts`, parse revisions, stage outcomes, step rows, artifacts, metrics, and audit evidence. Later stages add immutable revisions and relationships rather than overwriting earlier snapshots.
3. Validation determines whether a work proceeds to normalized corpus state. `run_work_stages` retains invalid and discarded outcomes, while analysis-ready viewer queries select valid normalized revisions.
4. Exact observed ORCID values may establish `people`; raw author observations remain in `author_occurrences`, and uncertain provider name matches remain isolated in resolution and candidate evidence until explicitly confirmed or rejected.
5. Cache payloads use the shared artifact store, cache identity remains stable across upserts, and `run_cache_uses` records which attempt and layer used each result.
6. A normalized work DOI is registered in `pdf_documents`; a validated manual PDF is stored in `pdf_blobs`; outbox delivery mirrors the mutation into metadata `audit_events` without requiring a transaction spanning both database files.
7. Normal operators and the viewer treat stored corpus evidence as read-only. Schema migrations are append-only files, immutable table corrections create new rows, and parent deletion requires explicit dependency-aware repository behavior because no foreign key cascades.

## 9. Schema change checklist

- Add a new migration under the database-owning directory with both `-- ==UP==` and `-- ==DOWN==` markers, then append its filename to the matching SOMETHING migration chain without editing an applied migration.
- Put table constraints at the database boundary when they are universally true, and retain repository validation when the rule requires project logic such as DOI normalization, ORCID checksum validation, lifecycle pairing, content hashing, or cross-file path safety.
- Preserve foreign keys, append-only triggers, content identity, immutable revision semantics, audit delivery idempotence, and bundle-relative PDF path validation.
- Update this document, review related architecture and standards, run the focused database tests, run `make docs-state-update`, and then run `make check`, which includes the read-only documentation consistency check.
