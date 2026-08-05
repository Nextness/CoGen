# Project Source Catalog

## Purpose

This generated catalog maps maintained Go and JavaScript declarations and tests to exact repository-relative source locations. Use [ARCHITECTURE.md](ARCHITECTURE.md) for system behavior and [STANDARDS.md](STANDARDS.md) for source documentation conventions.

Go coverage includes named functions, methods, tests, benchmarks, types, structs, interfaces, and aliases under `src/`. JavaScript coverage includes named function declarations, classes, methods, and `test` or `it` titles under `src/server/frontend/` and `frontend/`. Generated vendor code, installed dependencies, anonymous implementation callbacks, and non-Go or non-JavaScript utilities are outside the catalog.

Declarations and signatures are source-derived and show syntactic inputs and outputs. Descriptions come only from attached Go documentation comments, adjacent JavaScript JSDoc, or JavaScript test titles. Catalog update exposes a missing comment as `No source description` without inferring behavior from a symbol name, and `make check-docs` rejects that incomplete state.

Run `make docs-catalog-update` after maintained declarations or source comments change and review the generated region. `make check-docs` verifies freshness without writing.

<!-- BEGIN GENERATED PROJECT CATALOG -->

## Go declarations

### [`src/article/article.go`](../src/article/article.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Author`](../src/article/article.go#L15) | struct | 15-22 | ``type Author struct { CitationName string `json:"citation_name"` Orcid string `json:"orcid"` FirstName string `json:"first_name"` LastName string `json:"last_name"` NormalizedName string `json:"normalized_name"` Affiliation string `json:"affiliation"` }`` | Author stores one source-observed or enriched author occurrence before persistence. |
| [`Reference`](../src/article/article.go#L25) | struct | 25-32 | ``type Reference struct { Raw string `json:"raw"` DOI string `json:"doi"` Title string `json:"title"` Author string `json:"author"` Year int `json:"year"` Source string `json:"source"` }`` | Reference stores one ordered bibliographic reference parsed or enriched for an article. |
| [`Article`](../src/article/article.go#L35) | struct | 35-48 | ``type Article struct { DOI string `json:"doi"` Title string `json:"title"` Abstract string `json:"abstract"` Year int `json:"year"` Keywords []string `json:"keywords"` KeywordsAdditional []string `json:"keywords_additional"` Journal string `json:"journal"` Publisher string `json:"publisher"` Source string `json:"source"` CitationCount int `json:"citation_count"` CitedReferences []Reference `json:"cited_references"` Authors []Author `json:"authors"` }`` | Article is the pipeline's mutable in-memory work record before immutable revision persistence. |
| [`SanitizeText`](../src/article/article.go#L67) | function | 67-74 | `func SanitizeText(data string) string` | SanitizeText cleans problematic characters from raw export text. Removes HTML tags, replaces smart quotes/dashes with ASCII equivalents, strips BOM. |
| [`SplitToList`](../src/article/article.go#L78) | function | 78-101 | `func SplitToList(value, separator string) []string` | SplitToList splits a string by separator, returning cleaned non-empty parts. When separator is "\n", it splits on newlines directly without stripping newlines first. |
| [`ParseInt`](../src/article/article.go#L104) | function | 104-114 | `func ParseInt(value string) int` | ParseInt parses a string as an integer. Returns 0 on failure. |
| [`ParseOptionalInt`](../src/article/article.go#L117) | function | 117-127 | `func ParseOptionalInt(value string) *int` | ParseOptionalInt parses a string as an integer. Returns nil pointer on failure. |
| [`ExtractDOI`](../src/article/article.go#L130) | function | 130-138 | `func ExtractDOI(text string) string` | ExtractDOI returns the first DOI found in text, or empty string. |
| [`ParseAuthors`](../src/article/article.go#L144) | function | 144-180 | `func ParseAuthors(authorsStr, affiliationsStr string) []Author` | ParseAuthors parses semicolon/newline-separated author and affiliation strings. Affiliations are matched by position (same index as authors). BibTeX uses " and " as separator - this is tried as a fallback when semicolons don't produce a useful split. |
| [`ParseReferences`](../src/article/article.go#L185) | function | 185-207 | `func ParseReferences(refsStr string) []Reference` | ParseReferences parses a newline or semicolon-separated reference list with DOI extraction. CSV exports generally use semicolons while BibTeX exports use one reference per line. |
| [`RenameFields`](../src/article/article.go#L210) | function | 210-234 | `func RenameFields(m map[string]string, patches map[string]string)` | RenameFields renames keys in m according to patches (old -> new), in-place. |
| [`KeepFields`](../src/article/article.go#L237) | function | 237-247 | `func KeepFields(m map[string]string, keep []string)` | KeepFields deletes keys not in the keep set, in-place. |
| [`RequiredFieldError`](../src/article/article.go#L250) | struct | 250-252 | `type RequiredFieldError struct { Missing []string }` | RequiredFieldError is returned when a required field is missing. |
| [`(*RequiredFieldError).Error`](../src/article/article.go#L255) | method | 255-257 | `func (*RequiredFieldError).Error() string` | Error returns the receiver's diagnostic message. |
| [`CheckRequired`](../src/article/article.go#L260) | function | 260-272 | `func CheckRequired(a *Article) []string` | CheckRequired returns the list of missing required fields. |
| [`NewFromMap`](../src/article/article.go#L277) | function | 277-306 | `func NewFromMap(entry map[string]string, source string) (*Article, error)` | NewFromMap builds an Article from a canonical-field map[string]string. All text values are sanitised automatically. Returns nil + *RequiredFieldError if required fields are missing. |
| [`ArticleToMap`](../src/article/article.go#L309) | function | 309-346 | `func ArticleToMap(a *Article) map[string]any` | ArticleToMap serialises an Article to a plain map for JSON output. |
| [`MergeBySource`](../src/article/article.go#L350) | function | 350-375 | `func MergeBySource(articlesBySource map[string][]*Article) (unique, duplicates []*Article)` | MergeBySource merges articles from multiple sources, deduplicating by DOI. Returns (unique, duplicates). Articles without DOI are always kept. |
| [`lowerAll`](../src/article/article.go#L378) | function | 378-383 | `func lowerAll(ss []string) []string` | lowerAll returns lowercase copies of all input strings. |
| [`sortedKeys`](../src/article/article.go#L386) | function | 386-393 | `func sortedKeys(m map[string][]*Article) []string` | sortedKeys returns keys in deterministic order. |

### [`src/article/article_unit_test.go`](../src/article/article_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestSanitizeTextRemovesHTML`](../src/article/article_unit_test.go#L12) | test | 12-17 | `func TestSanitizeTextRemovesHTML(t *testing.T)` | TestSanitizeTextRemovesHTML verifies sanitize text removes html. |
| [`TestSanitizeTextSmartQuotes`](../src/article/article_unit_test.go#L20) | test | 20-25 | `func TestSanitizeTextSmartQuotes(t *testing.T)` | TestSanitizeTextSmartQuotes verifies sanitize text smart quotes. |
| [`TestSanitizeTextBOM`](../src/article/article_unit_test.go#L28) | test | 28-33 | `func TestSanitizeTextBOM(t *testing.T)` | TestSanitizeTextBOM verifies sanitize text bom. |
| [`TestSanitizeTextDashes`](../src/article/article_unit_test.go#L36) | test | 36-41 | `func TestSanitizeTextDashes(t *testing.T)` | TestSanitizeTextDashes verifies sanitize text dashes. |
| [`TestSplitToListBySemicolon`](../src/article/article_unit_test.go#L44) | test | 44-50 | `func TestSplitToListBySemicolon(t *testing.T)` | TestSplitToListBySemicolon verifies split to list by semicolon. |
| [`TestSplitToListByNewline`](../src/article/article_unit_test.go#L53) | test | 53-59 | `func TestSplitToListByNewline(t *testing.T)` | TestSplitToListByNewline verifies split to list by newline. |
| [`TestSplitToListEmpty`](../src/article/article_unit_test.go#L62) | test | 62-67 | `func TestSplitToListEmpty(t *testing.T)` | TestSplitToListEmpty verifies split to list empty. |
| [`TestSplitToListNoNewlineCollapse`](../src/article/article_unit_test.go#L70) | test | 70-77 | `func TestSplitToListNoNewlineCollapse(t *testing.T)` | TestSplitToListNoNewlineCollapse verifies split to list no newline collapse. |
| [`TestParseInt`](../src/article/article_unit_test.go#L80) | test | 80-96 | `func TestParseInt(t *testing.T)` | TestParseInt verifies parse int. |
| [`TestParseOptionalInt`](../src/article/article_unit_test.go#L99) | test | 99-106 | `func TestParseOptionalInt(t *testing.T)` | TestParseOptionalInt verifies parse optional int. |
| [`TestExtractDOIBare`](../src/article/article_unit_test.go#L109) | test | 109-114 | `func TestExtractDOIBare(t *testing.T)` | TestExtractDOIBare verifies extract doi bare. |
| [`TestExtractDOIFromURL`](../src/article/article_unit_test.go#L117) | test | 117-122 | `func TestExtractDOIFromURL(t *testing.T)` | TestExtractDOIFromURL verifies extract doi from url. |
| [`TestExtractDOITrailingPunct`](../src/article/article_unit_test.go#L125) | test | 125-130 | `func TestExtractDOITrailingPunct(t *testing.T)` | TestExtractDOITrailingPunct verifies extract doi trailing punct. |
| [`TestExtractDOINone`](../src/article/article_unit_test.go#L133) | test | 133-138 | `func TestExtractDOINone(t *testing.T)` | TestExtractDOINone verifies extract doi none. |
| [`TestParseAuthorsSemicolon`](../src/article/article_unit_test.go#L141) | test | 141-152 | `func TestParseAuthorsSemicolon(t *testing.T)` | TestParseAuthorsSemicolon verifies parse authors semicolon. |
| [`TestParseAuthorsSemicolonAffiliations`](../src/article/article_unit_test.go#L155) | test | 155-160 | `func TestParseAuthorsSemicolonAffiliations(t *testing.T)` | TestParseAuthorsSemicolonAffiliations verifies parse authors semicolon affiliations. |
| [`TestParseAuthorsAndFallback`](../src/article/article_unit_test.go#L163) | test | 163-171 | `func TestParseAuthorsAndFallback(t *testing.T)` | TestParseAuthorsAndFallback verifies parse authors and fallback. |
| [`TestParseAuthorsEmpty`](../src/article/article_unit_test.go#L174) | test | 174-179 | `func TestParseAuthorsEmpty(t *testing.T)` | TestParseAuthorsEmpty verifies parse authors empty. |
| [`TestParseReferences`](../src/article/article_unit_test.go#L182) | test | 182-193 | `func TestParseReferences(t *testing.T)` | TestParseReferences verifies parse references. |
| [`TestParseReferencesSemicolonSeparated`](../src/article/article_unit_test.go#L196) | test | 196-201 | `func TestParseReferencesSemicolonSeparated(t *testing.T)` | TestParseReferencesSemicolonSeparated verifies parse references semicolon separated. |
| [`TestRenameFields`](../src/article/article_unit_test.go#L204) | test | 204-216 | `func TestRenameFields(t *testing.T)` | TestRenameFields verifies rename fields. |
| [`TestRenameFieldsChainedRenamesAreDeterministic`](../src/article/article_unit_test.go#L219) | test | 219-225 | `func TestRenameFieldsChainedRenamesAreDeterministic(t *testing.T)` | TestRenameFieldsChainedRenamesAreDeterministic verifies rename fields chained renames are deterministic. |
| [`TestKeepFields`](../src/article/article_unit_test.go#L228) | test | 228-240 | `func TestKeepFields(t *testing.T)` | TestKeepFields verifies keep fields. |
| [`TestNewFromMapMinimal`](../src/article/article_unit_test.go#L243) | test | 243-269 | `func TestNewFromMapMinimal(t *testing.T)` | TestNewFromMapMinimal verifies new from map minimal. |
| [`TestNewFromMapNormalizesDOIURL`](../src/article/article_unit_test.go#L272) | test | 272-284 | `func TestNewFromMapNormalizesDOIURL(t *testing.T)` | TestNewFromMapNormalizesDOIURL verifies article construction normalizes a DOI URL. |
| [`TestNewFromMapMissingRequired`](../src/article/article_unit_test.go#L287) | test | 287-296 | `func TestNewFromMapMissingRequired(t *testing.T)` | TestNewFromMapMissingRequired verifies new from map missing required. |
| [`TestCheckRequired`](../src/article/article_unit_test.go#L299) | test | 299-312 | `func TestCheckRequired(t *testing.T)` | TestCheckRequired verifies check required. |
| [`TestArticleToMap`](../src/article/article_unit_test.go#L315) | test | 315-336 | `func TestArticleToMap(t *testing.T)` | TestArticleToMap verifies article to map. |
| [`TestMergeBySource`](../src/article/article_unit_test.go#L339) | test | 339-357 | `func TestMergeBySource(t *testing.T)` | TestMergeBySource verifies merge by source. |
| [`TestMergeBySourceNoDOIIsUnique`](../src/article/article_unit_test.go#L360) | test | 360-373 | `func TestMergeBySourceNoDOIIsUnique(t *testing.T)` | TestMergeBySourceNoDOIIsUnique verifies merge by source no doi is unique. |

### [`src/article/helpers_test.go`](../src/article/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`stringSliceEqual`](../src/article/helpers_test.go#L5) | function | 5-15 | `func stringSliceEqual(a, b []string) bool` | stringSliceEqual supports the package test suite's string slice equal setup or assertions. |
| [`as`](../src/article/helpers_test.go#L18) | function | 18-24 | `func as[T error](err error, target *T) bool` | as supports the package test suite's as setup or assertions. |

### [`src/bibtex/bibtex.go`](../src/bibtex/bibtex.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`EntryType`](../src/bibtex/bibtex.go#L15) | type | 15 | `type EntryType int` | EntryType identifies the supported BibTeX entry categories. |
| [`(EntryType).String`](../src/bibtex/bibtex.go#L26) | method | 26-39 | `func (EntryType).String() string` | String returns the receiver's textual representation. |
| [`entryTypeFromString`](../src/bibtex/bibtex.go#L42) | function | 42-55 | `func entryTypeFromString(s string) EntryType` | entryTypeFromString maps a case-insensitive BibTeX entry name to its category. |
| [`Entry`](../src/bibtex/bibtex.go#L58) | type | 58 | `type Entry map[string]string` | Entry is a BibTeX entry represented as a map of field names to values. |
| [`Library`](../src/bibtex/bibtex.go#L61) | type | 61 | `type Library map[string]Entry` | Library is a collection of BibTeX entries keyed by citation key. |
| [`Parser`](../src/bibtex/bibtex.go#L64) | struct | 64-66 | `type Parser struct { log *slog.Logger }` | Parser parses BibTeX data. Use NewParser to create one. |
| [`NewParser`](../src/bibtex/bibtex.go#L70) | function | 70-75 | `func NewParser(log *slog.Logger) *Parser` | NewParser creates a Parser that logs to the given logger. If log is nil, it uses the process-wide BibTeX component logger. |
| [`(*Parser).LoadFile`](../src/bibtex/bibtex.go#L78) | method | 78-85 | `func (*Parser).LoadFile(filepath string) (string, error)` | LoadFile reads and logs a BibTeX input file without parsing it. |
| [`tokenType`](../src/bibtex/bibtex.go#L88) | type | 88 | `type tokenType int` | tokenType identifies lexical tokens in the constrained BibTeX grammar. |
| [`token`](../src/bibtex/bibtex.go#L103) | struct | 103-106 | `type token struct { typ tokenType value string }` | token carries one BibTeX token category and its optional text value. |
| [`readBracedContent`](../src/bibtex/bibtex.go#L110) | function | 110-136 | `func readBracedContent(data string, i, n int, stripBraces bool) (string, int)` | readBracedContent reads content inside balanced braces. When stripBraces is true, the outermost braces are not included in the result. |
| [`isIdentChar`](../src/bibtex/bibtex.go#L139) | function | 139-142 | `func isIdentChar(c byte) bool` | isIdentChar reports whether a byte is accepted inside a BibTeX identifier. |
| [`tokenize`](../src/bibtex/bibtex.go#L145) | function | 145-219 | `func tokenize(data string, stripBraces bool) []token` | tokenize converts BibTeX source text into the constrained parser token stream. |
| [`(*Parser).Parse`](../src/bibtex/bibtex.go#L224) | method | 224-329 | `func (*Parser).Parse(data, source string, stripBraces bool) (Library, error)` | Parse parses raw BibTeX data into a Library. Only "article" entries are retained. Duplicate citation keys get a numeric suffix. The source parameter is stored in the "article_source" field of each entry. |

### [`src/bibtex/bibtex_functional_test.go`](../src/bibtex/bibtex_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestLoadFile`](../src/bibtex/bibtex_functional_test.go#L14) | test | 14-38 | `func TestLoadFile(t *testing.T)` | TestLoadFile verifies load file. |

### [`src/bibtex/bibtex_unit_test.go`](../src/bibtex/bibtex_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEntryType`](../src/bibtex/bibtex_unit_test.go#L12) | test | 12-51 | `func TestEntryType(t *testing.T)` | TestEntryType verifies entry type. |
| [`TestParse`](../src/bibtex/bibtex_unit_test.go#L54) | test | 54-279 | `func TestParse(t *testing.T)` | TestParse verifies parse. |
| [`TestNewParserNilLogger`](../src/bibtex/bibtex_unit_test.go#L282) | test | 282-291 | `func TestNewParserNilLogger(t *testing.T)` | TestNewParserNilLogger verifies new parser nil logger. |

### [`src/bibtex/helpers_test.go`](../src/bibtex/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`parser`](../src/bibtex/helpers_test.go#L15) | function | 15-17 | `func parser() *Parser` | parser supports the package test suite's parser setup or assertions. |

### [`src/database/attempts.go`](../src/database/attempts.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`RunSource`](../src/database/attempts.go#L14) | struct | 14-27 | ``type RunSource struct { ID int64 `json:"id"` PipelineRunID int64 `json:"pipeline_run_id"` SourceName string `json:"source_name"` SourceType string `json:"source_type"` ExpectedFile string `json:"expected_file"` Query string `json:"query,omitempty"` RequestedFields string `json:"requested_fields,omitempty"` ExpectedResultCount *int `json:"expected_result_count,omitempty"` ObservedResultCount *int `json:"observed_result_count,omitempty"` ResultCountComparison string `json:"result_count_comparison,omitempty"` ExportDate string `json:"export_date,omitempty"` CreatedAt string `json:"created_at"` }`` | RunSource links a pipeline run to one of its declared sources. |
| [`SourceRecord`](../src/database/attempts.go#L30) | struct | 30-39 | ``type SourceRecord struct { ID int64 `json:"id"` RunSourceID int64 `json:"run_source_id"` RecordIndex int `json:"record_index"` RawPayload string `json:"raw_payload"` ContentHash string `json:"content_hash"` ParseStatus string `json:"parse_status"` RejectReason string `json:"reject_reason,omitempty"` CreatedAt string `json:"created_at"` }`` | SourceRecord represents a single raw record loaded from a source. |
| [`Artifact`](../src/database/attempts.go#L42) | struct | 42-48 | ``type Artifact struct { ID int64 `json:"id"` ContentHash string `json:"content_hash"` ByteSize int64 `json:"byte_size"` ContentType string `json:"content_type"` CreatedAt string `json:"created_at"` }`` | Artifact is a content-addressed immutable payload. |
| [`RunStep`](../src/database/attempts.go#L52) | struct | 52-64 | ``type RunStep struct { ID int64 `json:"id"` PipelineRunID int64 `json:"pipeline_run_id"` StepName string `json:"step_name"` StepStatus string `json:"step_status"` InputArtifactID *int64 `json:"input_artifact_id,omitempty"` OutputArtifactID *int64 `json:"output_artifact_id,omitempty"` ReusedFromRunID *int64 `json:"reused_from_run_id,omitempty"` InputFingerprint string `json:"input_fingerprint,omitempty"` OutputFingerprint string `json:"output_fingerprint,omitempty"` StartedAt string `json:"started_at,omitempty"` FinishedAt string `json:"finished_at,omitempty"` }`` | RunStep records a stage's execution within a pipeline run, including its input/output artifacts and optional reuse from a prior run. |
| [`RunSourceRepository`](../src/database/attempts.go#L67) | struct | 67-69 | `type RunSourceRepository struct { db *Database }` | RunSourceRepository provides CRUD for the run_sources table. |
| [`(*RunSourceRepository).Create`](../src/database/attempts.go#L72) | method | 72-93 | `func (*RunSourceRepository).Create(pipelineRunID int64, sourceName, sourceType, expectedFile, query, requestedFields string, expectedResultCount int, exportDate string) (int64, error)` | Create inserts a new run source link. Returns the source ID. |
| [`(*RunSourceRepository).ListByRun`](../src/database/attempts.go#L96) | method | 96-148 | `func (*RunSourceRepository).ListByRun(pipelineRunID int64) ([]*RunSource, error)` | ListByRun returns all sources for a given pipeline run, ordered by ID. |
| [`(*RunSourceRepository).SetObservedResultCount`](../src/database/attempts.go#L152) | method | 152-163 | `func (*RunSourceRepository).SetObservedResultCount(runSourceID int64, observedResultCount int, comparison string) error` | SetObservedResultCount records the raw export count observed for a source and its informational comparison with the count declared in the config. |
| [`SourceRecordRepository`](../src/database/attempts.go#L166) | struct | 166-168 | `type SourceRecordRepository struct { db *Database }` | SourceRecordRepository provides CRUD for the source_records table. |
| [`(*SourceRecordRepository).Create`](../src/database/attempts.go#L171) | method | 171-192 | `func (*SourceRecordRepository).Create(runSourceID int64, recordIndex int, rawPayload, contentHash string) (int64, error)` | Create inserts a new source record. Returns the record ID. |
| [`(*SourceRecordRepository).UpdateParseStatus`](../src/database/attempts.go#L195) | method | 195-208 | `func (*SourceRecordRepository).UpdateParseStatus(recordID int64, status, rejectReason string) error` | UpdateParseStatus updates the parse status and optional reject reason for a source record. |
| [`(*SourceRecordRepository).ListBySource`](../src/database/attempts.go#L211) | method | 211-244 | `func (*SourceRecordRepository).ListBySource(runSourceID int64) ([]*SourceRecord, error)` | ListBySource returns all records for a given run source, ordered by record index. |
| [`(*SourceRecordRepository).CountBySource`](../src/database/attempts.go#L247) | method | 247-258 | `func (*SourceRecordRepository).CountBySource(runSourceID int64) (int, error)` | CountBySource returns the number of records for a given run source. |
| [`ArtifactRepository`](../src/database/attempts.go#L261) | struct | 261-263 | `type ArtifactRepository struct { db *Database }` | ArtifactRepository provides CRUD for the artifacts table. |
| [`(*ArtifactRepository).Create`](../src/database/attempts.go#L267) | method | 267-305 | `func (*ArtifactRepository).Create(contentHash, contentType string, byteSize int64) (int64, error)` | Create inserts a new artifact. Returns the artifact ID. If the content_hash already exists, returns the existing artifact ID. |
| [`(*ArtifactRepository).GetByHash`](../src/database/attempts.go#L308) | method | 308-324 | `func (*ArtifactRepository).GetByHash(contentHash string) (*Artifact, error)` | GetByHash returns an artifact by its content hash, or nil if not found. |
| [`(*ArtifactRepository).GetByID`](../src/database/attempts.go#L327) | method | 327-343 | `func (*ArtifactRepository).GetByID(id int64) (*Artifact, error)` | GetByID returns an artifact by its primary key, or nil if not found. |
| [`ArtifactBlob`](../src/database/attempts.go#L346) | struct | 346-352 | ``type ArtifactBlob struct { ID int64 `json:"id"` ArtifactID int64 `json:"artifact_id"` PipelineRunID int64 `json:"pipeline_run_id"` Data []byte `json:"-"` CreatedAt string `json:"created_at"` }`` | ArtifactBlob stores the raw bytes for an artifact inline in the database. |
| [`ArtifactBlobRepository`](../src/database/attempts.go#L355) | struct | 355-357 | `type ArtifactBlobRepository struct { db *Database }` | ArtifactBlobRepository provides CRUD for the artifact_blobs table. |
| [`(*ArtifactBlobRepository).Create`](../src/database/attempts.go#L361) | method | 361-406 | `func (*ArtifactBlobRepository).Create(artifactID, pipelineRunID int64, data []byte) (int64, error)` | Create inserts a new artifact blob. Returns the blob ID. If the artifact_id already exists, returns the existing blob ID (deduplicated). |
| [`(*ArtifactBlobRepository).GetByArtifactID`](../src/database/attempts.go#L409) | method | 409-425 | `func (*ArtifactBlobRepository).GetByArtifactID(artifactID int64) (*ArtifactBlob, error)` | GetByArtifactID returns the blob for a given artifact, or nil if not found. |
| [`(*ArtifactBlobRepository).ListByRun`](../src/database/attempts.go#L428) | method | 428-455 | `func (*ArtifactBlobRepository).ListByRun(pipelineRunID int64) ([]*ArtifactBlob, error)` | ListByRun returns all blobs written during a given pipeline run, ordered by ID. |
| [`(*ArtifactBlobRepository).ExistsByArtifactID`](../src/database/attempts.go#L458) | method | 458-469 | `func (*ArtifactBlobRepository).ExistsByArtifactID(artifactID int64) (bool, error)` | ExistsByArtifactID checks whether a blob already exists for the given artifact. |
| [`RunStepRepository`](../src/database/attempts.go#L472) | struct | 472-474 | `type RunStepRepository struct { db *Database }` | RunStepRepository provides CRUD for the run_steps table. |
| [`(*RunStepRepository).Create`](../src/database/attempts.go#L477) | method | 477-497 | `func (*RunStepRepository).Create(pipelineRunID int64, stepName string) (int64, error)` | Create inserts a new run step record. Returns the step ID. |
| [`(*RunStepRepository).UpdateStatus`](../src/database/attempts.go#L502) | method | 502-525 | `func (*RunStepRepository).UpdateStatus(stepID int64, status string) error` | UpdateStatus updates the status and optional finish time of a run step. The status must be a valid manifest.StageOutcome value. finished_at is only set for terminal statuses (completed, skipped, reused, failed). |
| [`(*RunStepRepository).LinkReuse`](../src/database/attempts.go#L530) | method | 530-541 | `func (*RunStepRepository).LinkReuse(stepID int64, reusedFromRunID int64) error` | LinkReuse records that a step reused output from a prior run. It sets both the status to "reused" and the finished_at timestamp since reuse is a terminal stage outcome. |
| [`(*RunStepRepository).LinkInputArtifact`](../src/database/attempts.go#L544) | method | 544-555 | `func (*RunStepRepository).LinkInputArtifact(stepID, artifactID int64) error` | LinkInputArtifact records that a step consumed a specific artifact as input. |
| [`(*RunStepRepository).LinkOutputArtifact`](../src/database/attempts.go#L558) | method | 558-569 | `func (*RunStepRepository).LinkOutputArtifact(stepID, artifactID int64) error` | LinkOutputArtifact records that a step produced a specific artifact as output. |
| [`(*RunStepRepository).SetFingerprints`](../src/database/attempts.go#L573) | method | 573-582 | `func (*RunStepRepository).SetFingerprints(stepID int64, inputFingerprint, outputFingerprint string) error` | SetFingerprints records the immutable stage input and output fingerprints used to decide whether this stage may be reused by a later attempt. |
| [`(*RunStepRepository).ListByRun`](../src/database/attempts.go#L585) | method | 585-633 | `func (*RunStepRepository).ListByRun(pipelineRunID int64) ([]*RunStep, error)` | ListByRun returns all steps for a given pipeline run, ordered by ID. |
| [`SourceFilterCount`](../src/database/attempts.go#L636) | struct | 636-641 | ``type SourceFilterCount struct { ID int64 `json:"id"` PipelineRunID int64 `json:"pipeline_run_id"` SourceName string `json:"source_name"` FilterData string `json:"filter_data"` // JSON array of {filters: string[], count: int} }`` | SourceFilterCount holds per-source filter stage article counts for a run. |
| [`SourceFilterCountRepository`](../src/database/attempts.go#L644) | struct | 644-646 | `type SourceFilterCountRepository struct { db *Database }` | SourceFilterCountRepository provides CRUD for the source_filter_counts table. |
| [`(*SourceFilterCountRepository).SetFilterData`](../src/database/attempts.go#L649) | method | 649-664 | `func (*SourceFilterCountRepository).SetFilterData(pipelineRunID int64, sourceName, filterData string) error` | SetFilterData upserts the filter data for a source in a run. |
| [`(*SourceFilterCountRepository).ListByRun`](../src/database/attempts.go#L667) | method | 667-695 | `func (*SourceFilterCountRepository).ListByRun(pipelineRunID int64) ([]*SourceFilterCount, error)` | ListByRun returns filter data for all sources in a run, ordered by source name. |
| [`(*SourceFilterCountRepository).GetByRunAndSource`](../src/database/attempts.go#L698) | method | 698-714 | `func (*SourceFilterCountRepository).GetByRunAndSource(pipelineRunID int64, sourceName string) (*SourceFilterCount, error)` | GetByRunAndSource returns filter data for a specific source in a run. |

### [`src/database/attempts_integration_test.go`](../src/database/attempts_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestExecutionPlanCreateAndGetByFingerprint`](../src/database/attempts_integration_test.go#L9) | test | 9-51 | `func TestExecutionPlanCreateAndGetByFingerprint(t *testing.T)` | TestExecutionPlanCreateAndGetByFingerprint verifies execution plan create and get by fingerprint. |
| [`TestExecutionPlanWithInputManifestAndEnrichmentPolicy`](../src/database/attempts_integration_test.go#L54) | test | 54-74 | `func TestExecutionPlanWithInputManifestAndEnrichmentPolicy(t *testing.T)` | TestExecutionPlanWithInputManifestAndEnrichmentPolicy verifies execution plan with input manifest and enrichment policy. |
| [`TestExecutionPlanDuplicateSameHashReusesID`](../src/database/attempts_integration_test.go#L77) | test | 77-97 | `func TestExecutionPlanDuplicateSameHashReusesID(t *testing.T)` | TestExecutionPlanDuplicateSameHashReusesID: same fingerprint + same manifest hash reuses the existing plan. |
| [`TestExecutionPlanDuplicateDifferentHashRejected`](../src/database/attempts_integration_test.go#L100) | test | 100-116 | `func TestExecutionPlanDuplicateDifferentHashRejected(t *testing.T)` | TestExecutionPlanDuplicateDifferentHashRejected: same fingerprint but different manifest hash returns an error. |
| [`TestExecutionPlanDistinctRevisions`](../src/database/attempts_integration_test.go#L120) | test | 120-152 | `func TestExecutionPlanDistinctRevisions(t *testing.T)` | TestExecutionPlanDistinctRevisions: distinct search revisions may have distinct plans for the same source files (same fingerprint, different revision). |
| [`TestExecutionPlanListBySearchRevision`](../src/database/attempts_integration_test.go#L155) | test | 155-172 | `func TestExecutionPlanListBySearchRevision(t *testing.T)` | TestExecutionPlanListBySearchRevision verifies execution plan list by search revision. |
| [`TestRunSourceCreateAndList`](../src/database/attempts_integration_test.go#L175) | test | 175-222 | `func TestRunSourceCreateAndList(t *testing.T)` | TestRunSourceCreateAndList verifies creating run sources and listing them. |
| [`TestSourceRecordParseAndReject`](../src/database/attempts_integration_test.go#L225) | test | 225-279 | `func TestSourceRecordParseAndReject(t *testing.T)` | TestSourceRecordParseAndReject verifies source record creation and parse status updates. |
| [`TestSourceRecordCount`](../src/database/attempts_integration_test.go#L282) | test | 282-300 | `func TestSourceRecordCount(t *testing.T)` | TestSourceRecordCount verifies the CountBySource method. |
| [`TestRunStepCreateAndUpdate`](../src/database/attempts_integration_test.go#L303) | test | 303-349 | `func TestRunStepCreateAndUpdate(t *testing.T)` | TestRunStepCreateAndUpdate verifies run step lifecycle. |
| [`TestRunStepReuseLink`](../src/database/attempts_integration_test.go#L352) | test | 352-379 | `func TestRunStepReuseLink(t *testing.T)` | TestRunStepReuseLink verifies the reuse linking mechanism. |
| [`TestRunStepArtifactLinks`](../src/database/attempts_integration_test.go#L382) | test | 382-414 | `func TestRunStepArtifactLinks(t *testing.T)` | TestRunStepArtifactLinks verifies artifact linking to steps. |
| [`TestRunStepFingerprints`](../src/database/attempts_integration_test.go#L417) | test | 417-443 | `func TestRunStepFingerprints(t *testing.T)` | TestRunStepFingerprints verifies run step fingerprints. |
| [`TestRunStepUpdateStatusInvalidStatus`](../src/database/attempts_integration_test.go#L447) | test | 447-473 | `func TestRunStepUpdateStatusInvalidStatus(t *testing.T)` | TestRunStepUpdateStatusInvalidStatus verifies that UpdateStatus rejects invalid stage outcomes. |
| [`TestRunStepUpdateStatusNonTerminalDoesNotSetFinishedAt`](../src/database/attempts_integration_test.go#L477) | test | 477-497 | `func TestRunStepUpdateStatusNonTerminalDoesNotSetFinishedAt(t *testing.T)` | TestRunStepUpdateStatusNonTerminalDoesNotSetFinishedAt verifies that setting a non-terminal status (pending, running) does not set finished_at. |
| [`TestSourceFilterCountSetAndGet`](../src/database/attempts_integration_test.go#L500) | test | 500-530 | `func TestSourceFilterCountSetAndGet(t *testing.T)` | TestSourceFilterCountSetAndGet verifies SetFilterData then GetByRunAndSource returns matching data. |
| [`TestSourceFilterCountSetAndList`](../src/database/attempts_integration_test.go#L534) | test | 534-565 | `func TestSourceFilterCountSetAndList(t *testing.T)` | TestSourceFilterCountSetAndList verifies SetFilterData for two sources then ListByRun returns both ordered by source name. |
| [`TestSourceFilterCountUpsert`](../src/database/attempts_integration_test.go#L569) | test | 569-596 | `func TestSourceFilterCountUpsert(t *testing.T)` | TestSourceFilterCountUpsert verifies that calling SetFilterData twice for the same run+source replaces the previous filter data. |
| [`TestSourceFilterCountGetNotFound`](../src/database/attempts_integration_test.go#L600) | test | 600-611 | `func TestSourceFilterCountGetNotFound(t *testing.T)` | TestSourceFilterCountGetNotFound verifies GetByRunAndSource returns nil, nil for a non-existent run. |
| [`TestSourceFilterCountListEmpty`](../src/database/attempts_integration_test.go#L615) | test | 615-631 | `func TestSourceFilterCountListEmpty(t *testing.T)` | TestSourceFilterCountListEmpty verifies ListByRun returns an empty slice for a run with no filter counts. |

### [`src/database/author_identity_candidates.go`](../src/database/author_identity_candidates.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`AuthorIdentityResolution`](../src/database/author_identity_candidates.go#L22) | struct | 22-32 | ``type AuthorIdentityResolution struct { ID int64 `json:"id"` PipelineRunID int64 `json:"pipeline_run_id"` AuthorOccurrenceID int64 `json:"author_occurrence_id"` Status string `json:"status"` Provider string `json:"provider"` QueriedCitationName string `json:"queried_citation_name"` ErrorMessage string `json:"error_message"` ResolvedAt string `json:"resolved_at"` CreatedAt string `json:"created_at"` }`` | AuthorIdentityResolution records the result of evaluating one observed author occurrence against an identity provider. It is separate from people and author_occurrences because a name search alone is not identity proof. |
| [`AuthorIdentityCandidate`](../src/database/author_identity_candidates.go#L37) | struct | 37-46 | ``type AuthorIdentityCandidate struct { ID int64 `json:"id"` IdentityResolutionID int64 `json:"identity_resolution_id"` CandidateORCID string `json:"candidate_orcid"` ProviderDisplayName string `json:"provider_display_name"` QueryURL string `json:"query_url"` PayloadArtifactID int64 `json:"payload_artifact_id"` ProviderRank int `json:"provider_rank"` CreatedAt string `json:"created_at"` }`` | AuthorIdentityCandidate is one provider-returned possible identity. It deliberately stores no person_id: a later reviewer may confirm or reject it without changing the evidence captured by this run. |
| [`AuthorIdentityResolutionRepository`](../src/database/author_identity_candidates.go#L49) | struct | 49-51 | `type AuthorIdentityResolutionRepository struct { db *Database }` | AuthorIdentityResolutionRepository owns uncertain identity evidence. |
| [`(*AuthorIdentityResolutionRepository).Create`](../src/database/author_identity_candidates.go#L54) | method | 54-77 | `func (*AuthorIdentityResolutionRepository).Create(resolution *AuthorIdentityResolution) (int64, error)` | Create validates and inserts one author identity resolution record. |
| [`validAuthorIdentityStatus`](../src/database/author_identity_candidates.go#L80) | function | 80-88 | `func validAuthorIdentityStatus(status string) bool` | validAuthorIdentityStatus reports whether the supplied author identity status is supported. |
| [`AuthorIdentityCandidateRepository`](../src/database/author_identity_candidates.go#L92) | struct | 92-94 | `type AuthorIdentityCandidateRepository struct { db *Database }` | AuthorIdentityCandidateRepository owns the immutable candidates attached to one uncertain identity resolution. |
| [`(*AuthorIdentityCandidateRepository).Create`](../src/database/author_identity_candidates.go#L97) | method | 97-117 | `func (*AuthorIdentityCandidateRepository).Create(candidate *AuthorIdentityCandidate) (int64, error)` | Create validates and inserts one uncertain author identity candidate. |

### [`src/database/author_identity_candidates_integration_test.go`](../src/database/author_identity_candidates_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAuthorIdentityResolutionCreate`](../src/database/author_identity_candidates_integration_test.go#L11) | test | 11-40 | `func TestAuthorIdentityResolutionCreate(t *testing.T)` | TestAuthorIdentityResolutionCreate verifies creating an identity resolution record. |
| [`TestAuthorIdentityResolutionRejectsMissingFields`](../src/database/author_identity_candidates_integration_test.go#L43) | test | 43-55 | `func TestAuthorIdentityResolutionRejectsMissingFields(t *testing.T)` | TestAuthorIdentityResolutionRejectsMissingFields verifies validation. |
| [`TestAuthorIdentityCandidateCreate`](../src/database/author_identity_candidates_integration_test.go#L58) | test | 58-92 | `func TestAuthorIdentityCandidateCreate(t *testing.T)` | TestAuthorIdentityCandidateCreate verifies creating an identity candidate. |

### [`src/database/author_identity_candidates_unit_test.go`](../src/database/author_identity_candidates_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAuthorIdentityStatusConstantsAreValid`](../src/database/author_identity_candidates_unit_test.go#L11) | test | 11-24 | `func TestAuthorIdentityStatusConstantsAreValid(t *testing.T)` | TestAuthorIdentityStatusConstantsAreValid verifies author identity status constants are valid. |

### [`src/database/authors_new.go`](../src/database/authors_new.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Person`](../src/database/authors_new.go#L14) | struct | 14-18 | ``type Person struct { ID int64 `json:"id"` ORCID string `json:"orcid"` CreatedAt string `json:"created_at"` }`` | Person represents an optional strong global identity for an author. ORCID is the canonical strong identity signal. |
| [`AuthorOccurrence`](../src/database/authors_new.go#L24) | struct | 24-32 | ``type AuthorOccurrence struct { ID int64 `json:"id"` PersonID int64 `json:"person_id"` CitationName string `json:"citation_name"` FirstName string `json:"first_name"` LastName string `json:"last_name"` ORCID string `json:"orcid"` CreatedAt string `json:"created_at"` }`` | AuthorOccurrence represents observed author data at a point in time. An occurrence may optionally link to a global Person record when the ORCID is a known strong identity. ORCID-less occurrences with the same name are never merged globally. |
| [`Authorship`](../src/database/authors_new.go#L39) | struct | 39-46 | ``type Authorship struct { ID int64 `json:"id"` WorkRevisionID int64 `json:"work_revision_id"` AuthorOccurrenceID int64 `json:"author_occurrence_id"` AuthorOrder int `json:"author_order"` Affiliation string `json:"affiliation"` CreatedAt string `json:"created_at"` }`` | Authorship links an immutable work_revision to an author_occurrence, preserving author order and optional affiliation. The authorships table is append-only (database-level triggers enforce this), so a historical revision's authorship set is immutable. Corrections create a new work_revision with a new authorship set. |
| [`PersonRepository`](../src/database/authors_new.go#L49) | struct | 49-51 | `type PersonRepository struct { db *Database }` | PersonRepository provides CRUD for the people table. |
| [`(*PersonRepository).CreateByORCID`](../src/database/authors_new.go#L59) | method | 59-107 | `func (*PersonRepository).CreateByORCID(orcid string) (int64, error)` | CreateByORCID inserts a new person by ORCID. If the ORCID already exists, returns the existing person ID (INSERT OR IGNORE semantics). The ORCID is normalized (lowercased, whitespace trimmed) before storage. A malformed ORCID or one that fails the ISO 7064 MOD 11-2 checksum is rejected — the people table is a strong identity registry, not a raw observation store. |
| [`(*PersonRepository).GetByID`](../src/database/authors_new.go#L110) | method | 110-129 | `func (*PersonRepository).GetByID(id int64) (*Person, error)` | GetByID returns a person by their primary key, or nil if not found. |
| [`(*PersonRepository).GetByORCID`](../src/database/authors_new.go#L133) | method | 133-158 | `func (*PersonRepository).GetByORCID(orcid string) (*Person, error)` | GetByORCID returns a person by their ORCID, or nil if not found. The ORCID is normalized the same way as CreateByORCID. |
| [`AuthorOccurrenceRepository`](../src/database/authors_new.go#L161) | struct | 161-163 | `type AuthorOccurrenceRepository struct { db *Database }` | AuthorOccurrenceRepository provides CRUD for the author_occurrences table. |
| [`(*AuthorOccurrenceRepository).Create`](../src/database/authors_new.go#L171) | method | 171-217 | `func (*AuthorOccurrenceRepository).Create(ao *AuthorOccurrence) (int64, error)` | Create inserts a new author occurrence. If the ORCID is non-empty and passes format-and-checksum validation, the method looks up or creates a Person record and links the occurrence to it. Invalid or malformed ORCIDs are stored as raw observed values on the occurrence but do not create or link to a person record — the design requires a strong identity signal before global merging, and an unvalidated string is not a strong signal. |
| [`(*AuthorOccurrenceRepository).GetByID`](../src/database/authors_new.go#L220) | method | 220-224 | `func (*AuthorOccurrenceRepository).GetByID(id int64) (*AuthorOccurrence, error)` | GetByID returns an author occurrence by its primary key, or nil if not found. |
| [`(*AuthorOccurrenceRepository).GetByPersonID`](../src/database/authors_new.go#L228) | method | 228-257 | `func (*AuthorOccurrenceRepository).GetByPersonID(personID int64) ([]*AuthorOccurrence, error)` | GetByPersonID returns all author occurrences linked to a given person, in ID order. |
| [`AuthorshipRepository`](../src/database/authors_new.go#L260) | struct | 260-262 | `type AuthorshipRepository struct { db *Database }` | AuthorshipRepository provides CRUD for the authorships table. |
| [`(*AuthorshipRepository).Create`](../src/database/authors_new.go#L266) | method | 266-301 | `func (*AuthorshipRepository).Create(a *Authorship) (int64, error)` | Create inserts a new authorship linking a work revision to an author occurrence with the given order and optional affiliation. |
| [`(*AuthorshipRepository).GetByRevisionID`](../src/database/authors_new.go#L305) | method | 305-334 | `func (*AuthorshipRepository).GetByRevisionID(revisionID int64) ([]*Authorship, error)` | GetByRevisionID returns all authorships for a given work revision, ordered by author_order. |
| [`(*AuthorshipRepository).GetByOccurrenceID`](../src/database/authors_new.go#L338) | method | 338-367 | `func (*AuthorshipRepository).GetByOccurrenceID(occurrenceID int64) ([]*Authorship, error)` | GetByOccurrenceID returns all authorships for a given author occurrence, ordered by ID. |
| [`scanAuthorOccurrence`](../src/database/authors_new.go#L370) | function | 370-398 | `func scanAuthorOccurrence(row scannable) (*AuthorOccurrence, error)` | scanAuthorOccurrence decodes author occurrence from a database row. |
| [`scanAuthorship`](../src/database/authors_new.go#L401) | function | 401-415 | `func scanAuthorship(row scannable) (*Authorship, error)` | scanAuthorship decodes authorship from a database row. |
| [`normalizeORCID`](../src/database/authors_new.go#L419) | function | 419-421 | `func normalizeORCID(orcid string) string` | normalizeORCID lowercases and trims whitespace from an ORCID string. It does not validate the ORCID format; callers should use isValidORCID. |
| [`orcidDigit`](../src/database/authors_new.go#L425) | function | 425-433 | `func orcidDigit(b byte) int` | orcidDigit converts a byte to its integer value for checksum computation. '0'-'9' map to 0-9; 'x' and 'X' map to 10. |
| [`isValidORCID`](../src/database/authors_new.go#L440) | function | 440-486 | `func isValidORCID(orcid string) bool` | isValidORCID checks whether the given normalized string is a valid ORCID identifier. It must match the pattern XXXX-XXXX-XXXX-XXXX, where the last group ends with a digit or X that matches the ISO 7064 MOD 11-2 checksum. Hyphens are required for the pattern match but are stripped for checksum computation. |

### [`src/database/authors_new_integration_test.go`](../src/database/authors_new_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestPersonCreateByORCID`](../src/database/authors_new_integration_test.go#L13) | test | 13-57 | `func TestPersonCreateByORCID(t *testing.T)` | TestPersonCreateByORCID verifies that a person can be created by ORCID and retrieved by ID and ORCID, and that duplicate ORCIDs return the same ID. |
| [`TestPersonByORCIDNormalizesInput`](../src/database/authors_new_integration_test.go#L61) | test | 61-78 | `func TestPersonByORCIDNormalizesInput(t *testing.T)` | TestPersonByORCIDNormalizesInput verifies that ORCID normalization is consistent between CreateByORCID and GetByORCID. |
| [`TestPersonEmptyORCID`](../src/database/authors_new_integration_test.go#L81) | test | 81-89 | `func TestPersonEmptyORCID(t *testing.T)` | TestPersonEmptyORCID verifies that empty ORCID is rejected for person creation. |
| [`TestPersonMalformedORCIDRejected`](../src/database/authors_new_integration_test.go#L92) | test | 92-100 | `func TestPersonMalformedORCIDRejected(t *testing.T)` | TestPersonMalformedORCIDRejected verifies that a malformed ORCID is rejected. |
| [`TestPersonInvalidChecksumORCIDRejected`](../src/database/authors_new_integration_test.go#L104) | test | 104-112 | `func TestPersonInvalidChecksumORCIDRejected(t *testing.T)` | TestPersonInvalidChecksumORCIDRejected verifies that a well-formed ORCID with a wrong checksum is rejected. |
| [`TestPeopleORCIDGuardTriggers`](../src/database/authors_new_integration_test.go#L116) | test | 116-158 | `func TestPeopleORCIDGuardTriggers(t *testing.T)` | TestPeopleORCIDGuardTriggers verifies that direct SQL cannot insert or update a person to a null, empty, or whitespace-only ORCID. |
| [`TestAuthorOccurrenceValidORCIDLinksToPerson`](../src/database/authors_new_integration_test.go#L162) | test | 162-193 | `func TestAuthorOccurrenceValidORCIDLinksToPerson(t *testing.T)` | TestAuthorOccurrenceValidORCIDLinksToPerson verifies that an occurrence with a valid format-and-checksum ORCID creates or links to a Person record. |
| [`TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson`](../src/database/authors_new_integration_test.go#L198) | test | 198-232 | `func TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson(t *testing.T)` | TestAuthorOccurrenceInvalidORCIDDoesNotLinkToPerson verifies that an occurrence with a malformed or checksum-invalid ORCID stores the raw value but does not create or link to a Person record. |
| [`TestAuthorOccurrenceAppendOnlyTriggerUpdate`](../src/database/authors_new_integration_test.go#L236) | test | 236-264 | `func TestAuthorOccurrenceAppendOnlyTriggerUpdate(t *testing.T)` | TestAuthorOccurrenceAppendOnlyTriggerUpdate verifies that direct UPDATE on author_occurrences is rejected by the database trigger. |
| [`TestAuthorOccurrenceAppendOnlyTriggerDelete`](../src/database/authors_new_integration_test.go#L268) | test | 268-293 | `func TestAuthorOccurrenceAppendOnlyTriggerDelete(t *testing.T)` | TestAuthorOccurrenceAppendOnlyTriggerDelete verifies that direct DELETE on author_occurrences is rejected by the database trigger. |
| [`TestAuthorOccurrenceCreateAndRetrieve`](../src/database/authors_new_integration_test.go#L297) | test | 297-337 | `func TestAuthorOccurrenceCreateAndRetrieve(t *testing.T)` | TestAuthorOccurrenceCreateAndRetrieve verifies basic creation and retrieval of an author occurrence without an ORCID. |
| [`TestAuthorOccurrenceRejectsEmptyCitationName`](../src/database/authors_new_integration_test.go#L341) | test | 341-349 | `func TestAuthorOccurrenceRejectsEmptyCitationName(t *testing.T)` | TestAuthorOccurrenceRejectsEmptyCitationName verifies that Create requires a non-empty citation_name. |
| [`TestAuthorOccurrenceWithORCIDLinksToPerson`](../src/database/authors_new_integration_test.go#L353) | test | 353-391 | `func TestAuthorOccurrenceWithORCIDLinksToPerson(t *testing.T)` | TestAuthorOccurrenceWithORCIDLinksToPerson verifies that an occurrence with a non-empty ORCID automatically creates or links to a Person record. |
| [`TestAuthorOccurrenceSameORCIDSharesPerson`](../src/database/authors_new_integration_test.go#L395) | test | 395-439 | `func TestAuthorOccurrenceSameORCIDSharesPerson(t *testing.T)` | TestAuthorOccurrenceSameORCIDSharesPerson verifies that two occurrences with the same ORCID link to the same Person record. |
| [`TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct`](../src/database/authors_new_integration_test.go#L444) | test | 444-473 | `func TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct(t *testing.T)` | TestAuthorOccurrenceSameNameNoORCIDRemainsDistinct verifies that two occurrences with the same citation name but no ORCID stay as separate rows with no person link, and cannot be merged by name alone. |
| [`TestAuthorshipAppendOnlyTriggerUpdate`](../src/database/authors_new_integration_test.go#L477) | test | 477-511 | `func TestAuthorshipAppendOnlyTriggerUpdate(t *testing.T)` | TestAuthorshipAppendOnlyTriggerUpdate verifies that direct UPDATE on authorships is rejected by the database trigger. |
| [`TestAuthorshipAppendOnlyTriggerDelete`](../src/database/authors_new_integration_test.go#L515) | test | 515-549 | `func TestAuthorshipAppendOnlyTriggerDelete(t *testing.T)` | TestAuthorshipAppendOnlyTriggerDelete verifies that direct DELETE on authorships is rejected by the database trigger. |
| [`TestAuthorshipCreateAndRetrieveByRevision`](../src/database/authors_new_integration_test.go#L554) | test | 554-623 | `func TestAuthorshipCreateAndRetrieveByRevision(t *testing.T)` | TestAuthorshipCreateAndRetrieveByRevision verifies that an authorship can be created linking a work revision and an author occurrence, and retrieved by revision ID in author order. |
| [`TestAuthorshipRejectsMissingWorkRevisionID`](../src/database/authors_new_integration_test.go#L626) | test | 626-637 | `func TestAuthorshipRejectsMissingWorkRevisionID(t *testing.T)` | TestAuthorshipRejectsMissingWorkRevisionID verifies authorship rejects missing work revision id. |
| [`TestAuthorshipRejectsMissingOccurrenceID`](../src/database/authors_new_integration_test.go#L640) | test | 640-651 | `func TestAuthorshipRejectsMissingOccurrenceID(t *testing.T)` | TestAuthorshipRejectsMissingOccurrenceID verifies authorship rejects missing occurrence id. |
| [`TestAuthorshipRejectsInvalidOrder`](../src/database/authors_new_integration_test.go#L654) | test | 654-666 | `func TestAuthorshipRejectsInvalidOrder(t *testing.T)` | TestAuthorshipRejectsInvalidOrder verifies authorship rejects invalid order. |
| [`TestAuthorshipUniqueOrderPerRevision`](../src/database/authors_new_integration_test.go#L669) | test | 669-696 | `func TestAuthorshipUniqueOrderPerRevision(t *testing.T)` | TestAuthorshipUniqueOrderPerRevision verifies authorship unique order per revision. |
| [`TestAuthorshipUniqueOccurrencePerRevision`](../src/database/authors_new_integration_test.go#L699) | test | 699-725 | `func TestAuthorshipUniqueOccurrencePerRevision(t *testing.T)` | TestAuthorshipUniqueOccurrencePerRevision verifies authorship unique occurrence per revision. |
| [`TestAuthorshipFkRejectsNonexistentRevision`](../src/database/authors_new_integration_test.go#L728) | test | 728-740 | `func TestAuthorshipFkRejectsNonexistentRevision(t *testing.T)` | TestAuthorshipFkRejectsNonexistentRevision verifies authorship fk rejects nonexistent revision. |
| [`TestAuthorshipFkRejectsNonexistentOccurrence`](../src/database/authors_new_integration_test.go#L743) | test | 743-760 | `func TestAuthorshipFkRejectsNonexistentOccurrence(t *testing.T)` | TestAuthorshipFkRejectsNonexistentOccurrence verifies authorship fk rejects nonexistent occurrence. |
| [`TestAuthorshipFkRejectsNonexistentPerson`](../src/database/authors_new_integration_test.go#L763) | test | 763-774 | `func TestAuthorshipFkRejectsNonexistentPerson(t *testing.T)` | TestAuthorshipFkRejectsNonexistentPerson verifies authorship fk rejects nonexistent person. |
| [`TestTwoRevisionsDistinctAuthorshipSets`](../src/database/authors_new_integration_test.go#L777) | test | 777-850 | `func TestTwoRevisionsDistinctAuthorshipSets(t *testing.T)` | TestTwoRevisionsDistinctAuthorshipSets verifies two revisions distinct authorship sets. |
| [`TestAuthorshipGetByOccurrenceID`](../src/database/authors_new_integration_test.go#L853) | test | 853-886 | `func TestAuthorshipGetByOccurrenceID(t *testing.T)` | TestAuthorshipGetByOccurrenceID verifies authorship get by occurrence id. |

### [`src/database/authors_new_unit_test.go`](../src/database/authors_new_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestORCIDValidFormat`](../src/database/authors_new_unit_test.go#L11) | test | 11-23 | `func TestORCIDValidFormat(t *testing.T)` | TestORCIDValidFormat verifies orcid valid format. |
| [`TestORCIDInvalidFormat`](../src/database/authors_new_unit_test.go#L26) | test | 26-44 | `func TestORCIDInvalidFormat(t *testing.T)` | TestORCIDInvalidFormat verifies orcid invalid format. |
| [`TestORCIDInvalidChecksum`](../src/database/authors_new_unit_test.go#L47) | test | 47-65 | `func TestORCIDInvalidChecksum(t *testing.T)` | TestORCIDInvalidChecksum verifies orcid invalid checksum. |
| [`TestORCIDNormalizedStillValid`](../src/database/authors_new_unit_test.go#L68) | test | 68-75 | `func TestORCIDNormalizedStillValid(t *testing.T)` | TestORCIDNormalizedStillValid verifies orcid normalized still valid. |

### [`src/database/cache.go`](../src/database/cache.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`CacheEntry`](../src/database/cache.go#L16) | struct | 16-28 | ``type CacheEntry struct { ID int64 `json:"id"` Provider string `json:"provider"` Namespace string `json:"namespace"` RequestFingerprint string `json:"request_fingerprint"` ResponseStatus int `json:"response_status"` PayloadArtifactID *int64 `json:"payload_artifact_id,omitempty"` FetchedAt string `json:"fetched_at"` ExpiresAt string `json:"expires_at,omitempty"` ExtractorVersion string `json:"extractor_version"` CreatedAt string `json:"created_at"` UpdatedAt string `json:"updated_at"` }`` | CacheEntry is a versioned raw provider response. A nil payload artifact is valid for negative responses such as an HTTP 404. |
| [`RunCacheUse`](../src/database/cache.go#L31) | struct | 31-38 | ``type RunCacheUse struct { ID int64 `json:"id"` PipelineRunID int64 `json:"pipeline_run_id"` CacheEntryID int64 `json:"cache_entry_id"` CacheLayer string `json:"cache_layer"` Outcome string `json:"outcome"` UsedAt string `json:"used_at"` }`` | RunCacheUse records that a run consulted or consumed a global cache entry. |
| [`CacheEntryRepository`](../src/database/cache.go#L41) | struct | 41 | `type CacheEntryRepository struct{ db *Database }` | CacheEntryRepository provides persistence operations for cache entry records. |
| [`RunCacheUseRepository`](../src/database/cache.go#L44) | struct | 44 | `type RunCacheUseRepository struct{ db *Database }` | RunCacheUseRepository provides persistence operations for run cache use records. |
| [`validateCacheEntry`](../src/database/cache.go#L47) | function | 47-64 | `func validateCacheEntry(entry *CacheEntry) error` | validateCacheEntry enforces provider, namespace, fingerprint, status, and payload invariants before persistence. |
| [`(*CacheEntryRepository).Upsert`](../src/database/cache.go#L68) | method | 68-98 | `func (*CacheEntryRepository).Upsert(entry *CacheEntry) (int64, error)` | Upsert atomically replaces the response for one provider request and extractor version. The stable row ID preserves references from run_cache_uses. |
| [`(*CacheEntryRepository).Get`](../src/database/cache.go#L102) | method | 102-108 | `func (*CacheEntryRepository).Get(provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error)` | Get returns the exact versioned cache entry, regardless of expiry. Policy execution decides whether an expired entry is stale or may be reused. |
| [`(*CacheEntryRepository).GetGlobal`](../src/database/cache.go#L112) | method | 112-120 | `func (*CacheEntryRepository).GetGlobal(provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error)` | GetGlobal returns an entry only after a run explicitly published it to the global layer. Entries written only to active_run remain private to that run. |
| [`(*CacheEntryRepository).get`](../src/database/cache.go#L123) | method | 123-144 | `func (*CacheEntryRepository).get(query string, args ...any) (*CacheEntry, error)` | get executes a cache-entry query and returns its nullable payload and expiry fields. |
| [`(*RunCacheUseRepository).Create`](../src/database/cache.go#L147) | method | 147-160 | `func (*RunCacheUseRepository).Create(use *RunCacheUse) (int64, error)` | Create validates and inserts one run-scoped cache-use evidence record. |
| [`(*RunCacheUseRepository).ListByRun`](../src/database/cache.go#L163) | method | 163-182 | `func (*RunCacheUseRepository).ListByRun(runID int64) ([]*RunCacheUse, error)` | ListByRun returns cache-use evidence for a run in insertion order. |
| [`(*RunCacheUseRepository).FindEntry`](../src/database/cache.go#L186) | method | 186-188 | `func (*RunCacheUseRepository).FindEntry(runID int64, layer, provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error)` | FindEntry returns the latest entry with this exact versioned key recorded for a run and cache layer. It supports active-run and named-prior-run reads. |
| [`(*RunCacheUseRepository).FindAnyEntry`](../src/database/cache.go#L193) | method | 193-220 | `func (*RunCacheUseRepository).FindAnyEntry(runID int64, provider, namespace, fingerprint, extractorVersion string) (*CacheEntry, error)` | FindAnyEntry returns the latest exact cache key used by a prior run. Named prior-run policy reads use this because their source layer is provenance, not a restriction on how the source run originally obtained the response. |
| [`(*RunCacheUseRepository).findEntry`](../src/database/cache.go#L223) | method | 223-251 | `func (*RunCacheUseRepository).findEntry(layerClause string, args []any) (*CacheEntry, error)` | findEntry returns the latest cache entry matching a run-layer predicate. |

### [`src/database/cache_integration_test.go`](../src/database/cache_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestCacheEntryUpsertAndKeySeparation`](../src/database/cache_integration_test.go#L16) | test | 16-60 | `func TestCacheEntryUpsertAndKeySeparation(t *testing.T)` | TestCacheEntryUpsertAndKeySeparation verifies upsert identity preservation and key separation across provider/request/version. |
| [`TestCacheEntryConcurrentUpsertAndRunUse`](../src/database/cache_integration_test.go#L63) | test | 63-113 | `func TestCacheEntryConcurrentUpsertAndRunUse(t *testing.T)` | TestCacheEntryConcurrentUpsertAndRunUse verifies cache entry concurrent upsert and run use. |
| [`TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity`](../src/database/cache_integration_test.go#L116) | test | 116-197 | `func TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity(t *testing.T)` | TestConcurrentDatabaseInstancesPreserveCacheAndAttemptIntegrity verifies concurrent database instances preserve cache and attempt integrity. |

### [`src/database/config.go`](../src/database/config.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`StoreKind`](../src/database/config.go#L14) | type | 14 | `type StoreKind string` | StoreKind identifies one database in the corpus bundle registry. |
| [`MigrationConfig`](../src/database/config.go#L24) | struct | 24-27 | `type MigrationConfig struct { ConfigPath string MigrationsDir string }` | MigrationConfig is one fully resolved database-specific migration source. ConfigPath and MigrationsDir are absolute so callers do not depend on their current working directory after configuration has been loaded. |
| [`ResolveMigrationConfig`](../src/database/config.go#L31) | function | 31-78 | `func ResolveMigrationConfig(registryPath string, kind StoreKind) (MigrationConfig, error)` | ResolveMigrationConfig loads the database registry and resolves the database-specific configuration and migration directory for kind. |
| [`resolveSpecificMigrationConfig`](../src/database/config.go#L81) | function | 81-102 | `func resolveSpecificMigrationConfig(configPath string, cfg map[string]any, legacy bool) (MigrationConfig, error)` | resolveSpecificMigrationConfig resolves specific migration config from the supplied context. |
| [`loadSomethingConfig`](../src/database/config.go#L105) | function | 105-113 | `func loadSomethingConfig(path string) (map[string]any, error)` | loadSomethingConfig loads a .something file using the existing parser. |
| [`getMigrationStructs`](../src/database/config.go#L117) | function | 117-146 | `func getMigrationStructs(cfg map[string]any) ([]migrationEntry, error)` | getMigrationStructs reads #iteration("_db_migration") entries from the config. Returns them in iteration-counter order (as they appear in the file). |

### [`src/database/config_integration_test.go`](../src/database/config_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestBaselineAppliesCorrectly`](../src/database/config_integration_test.go#L17) | test | 17-193 | `func TestBaselineAppliesCorrectly(t *testing.T)` | TestBaselineAppliesCorrectly verifies that a new temporary database opens with the single workspace baseline, has all required tables, indexes, and triggers, and reopens without reapplying the baseline. |
| [`TestFutureMigrationRollsBackAtomically`](../src/database/config_integration_test.go#L199) | test | 199-280 | `func TestFutureMigrationRollsBackAtomically(t *testing.T)` | TestFutureMigrationRollsBackAtomically verifies that a deliberately broken future migration (V00002) applies its schema changes atomically: if the migration SQL fails, the new table does not persist and the tracking row is not recorded. |
| [`TestV00003MigrationApplies`](../src/database/config_integration_test.go#L284) | test | 284-315 | `func TestV00003MigrationApplies(t *testing.T)` | TestV00003MigrationApplies verifies that the V00003 migration is applied after V00002, creating the four append-only triggers. |
| [`TestV00002ToV00003Upgrade`](../src/database/config_integration_test.go#L321) | test | 321-456 | `func TestV00002ToV00003Upgrade(t *testing.T)` | TestV00002ToV00003Upgrade verifies that an existing database with V00001 and V00002 upgrades correctly when V00003 is added to the config. This is a true upgrade regression test, distinct from TestV00003MigrationApplies which opens a fresh database with all three migrations already configured. |
| [`TestV00003ToV00004Upgrade`](../src/database/config_integration_test.go#L460) | test | 460-571 | `func TestV00003ToV00004Upgrade(t *testing.T)` | TestV00003ToV00004Upgrade verifies that an existing V00003 database gains the non-blank people.orcid guards when V00004 is added to the config. |
| [`TestV00002MigrationApplies`](../src/database/config_integration_test.go#L576) | test | 576-621 | `func TestV00002MigrationApplies(t *testing.T)` | TestV00002MigrationApplies verifies that the V00002 migration is applied after the baseline, creating the people, author_occurrences, and authorships tables with their indexes. |
| [`TestProductionDatabaseRegistryResolvesIndependentMigrationChains`](../src/database/config_integration_test.go#L628) | test | 628-644 | `func TestProductionDatabaseRegistryResolvesIndependentMigrationChains(t *testing.T)` | TestProductionDatabaseRegistryResolvesIndependentMigrationChains verifies production database registry resolves independent migration chains. |
| [`TestProductionRegistryCreatesMetadataAndPDFStores`](../src/database/config_integration_test.go#L647) | test | 647-692 | `func TestProductionRegistryCreatesMetadataAndPDFStores(t *testing.T)` | TestProductionRegistryCreatesMetadataAndPDFStores verifies production registry creates metadata and pdf stores. |
| [`TestPDFInventoryMigrationPreservesLegacyDocumentStates`](../src/database/config_integration_test.go#L695) | test | 695-761 | `func TestPDFInventoryMigrationPreservesLegacyDocumentStates(t *testing.T)` | TestPDFInventoryMigrationPreservesLegacyDocumentStates verifies pdf inventory migration preserves legacy document states. |
| [`TestProductionMetadataUpgradePreservesAppliedBasenames`](../src/database/config_integration_test.go#L764) | test | 764-803 | `func TestProductionMetadataUpgradePreservesAppliedBasenames(t *testing.T)` | TestProductionMetadataUpgradePreservesAppliedBasenames verifies production metadata upgrade preserves applied basenames. |

### [`src/database/database.go`](../src/database/database.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Database`](../src/database/database.go#L26) | struct | 26-56 | `type Database struct { DB *sql.DB PipelineRuns *PipelineRunRepository Searches *SearchRepository Revisions *SearchRevisionRepository Plans *ExecutionPlanRepository RunSources *RunSourceRepository SourceRecords *SourceRecordRepository Artifacts *ArtifactRepository RunSteps *RunStepRepository Metrics *MetricsRepository AuditEvents *AuditEventRepository Works *WorkRepository WorkIdentifiers *WorkIdentifierRepository WorkRevisions *WorkRevisionRepository RunWorkStages *RunWorkStageRepository People *PersonRepository AuthorOccs *AuthorOccurrenceRepository Authorships *AuthorshipRepository IdentityResolutions *AuthorIdentityResolutionRepository IdentityCandidates *AuthorIdentityCandidateRepository ReferenceMentions *ReferenceMentionRepository CacheEntries *CacheEntryRepository RunCacheUses *RunCacheUseRepository ArtifactBlobs *ArtifactBlobRepository RunArtifacts *RunArtifactRepository SourceFilterCounts *SourceFilterCountRepository dbPath string migrations string // migration SQL directory }` | Database wraps a SQLite connection and exposes per-table repositories. |
| [`Open`](../src/database/database.go#L60) | function | 60-74 | `func Open(dbPath, configPath string) (*Database, error)` | Open opens (or creates) the SQLite database at dbPath, runs pending migrations, and initialises repositories. Call Close when done. |
| [`OpenConfigured`](../src/database/database.go#L80) | function | 80-128 | `func OpenConfigured(dbPath, registryPath string, kind StoreKind) (*sql.DB, error)` | OpenConfigured opens a writable SQLite database, configures its connection pool, and applies the migration chain selected from the database registry. It is used by the metadata repositories and the independently owned PDF store. |
| [`(*Database).initRepositories`](../src/database/database.go#L131) | method | 131-157 | `func (*Database).initRepositories()` | initRepositories binds every repository facade to the opened database. |
| [`configurePragma`](../src/database/database.go#L163) | function | 163-177 | `func configurePragma(db *sql.DB, pragma string) error` | configurePragma retries startup-only locking around journal-mode changes. The connection URI covers normal busy handling, but two processes enabling WAL on an uninitialised database can still race before either has completed its first pragma sequence. |
| [`sqliteBusy`](../src/database/database.go#L180) | function | 180-186 | `func sqliteBusy(err error) bool` | sqliteBusy reports whether an error represents SQLite busy or locked contention. |
| [`(*Database).Close`](../src/database/database.go#L189) | method | 189-197 | `func (*Database).Close() error` | Close closes the database connection. |
| [`(*Database).SchemaVersion`](../src/database/database.go#L202) | method | 202-212 | `func (*Database).SchemaVersion() (string, error)` | SchemaVersion returns the most recently applied migration filename. It is recorded in each resolved manifest so plan fingerprints describe the schema that interpreted the input. |
| [`(*Database).runMigrations`](../src/database/database.go#L217) | method | 217-301 | `func (*Database).runMigrations(configPath string) error` | runMigrations applies unapplied configured migrations in declaration order and records their checksums. |
| [`(*Database).withMigrationLock`](../src/database/database.go#L307) | method | 307-330 | `func (*Database).withMigrationLock(ctx context.Context, action func(*sql.Conn) error) (err error)` | withMigrationLock serializes each migration transaction across independent processes. BEGIN IMMEDIATE obtains SQLite's write lock before checking the tracking table, preventing two openers from both observing a migration as pending and applying it twice. |
| [`migrationEntry`](../src/database/database.go#L333) | struct | 333-337 | `type migrationEntry struct { filename string previous string upgrade string }` | migrationEntry stores one configured migration filename and its descriptive linkage fields. |
| [`loadMigrationChain`](../src/database/database.go#L340) | function | 340-353 | `func loadMigrationChain(configPath string) ([]migrationEntry, error)` | loadMigrationChain evaluates the database registry and returns its migrations in declaration order. |
| [`extractUpSQL`](../src/database/database.go#L359) | function | 359-381 | `func extractUpSQL(filepath string) (string, error)` | extractUpSQL returns the SQL between a migration's required UP and DOWN markers. |
| [`fileChecksum`](../src/database/database.go#L384) | function | 384-391 | `func fileChecksum(path string) (string, error)` | fileChecksum returns the lowercase hexadecimal SHA-256 digest of a file. |
| [`timestamp`](../src/database/database.go#L394) | function | 394-396 | `func timestamp() string` | timestamp returns the current UTC time in the repository's persisted format. |

### [`src/database/database_integration_test.go`](../src/database/database_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestSchemaSmokePhase14`](../src/database/database_integration_test.go#L16) | test | 16-253 | `func TestSchemaSmokePhase14(t *testing.T)` | TestSchemaSmokePhase14 verifies Phase 1.4: open a fully migrated database, create one of every workspace entity, then verify FK integrity and expected indexes exist. This is the schema smoke test that validates the complete V00001-V00008 migration chain works together. |
| [`TestCorpusModelSmokePhase25`](../src/database/database_integration_test.go#L257) | test | 257-344 | `func TestCorpusModelSmokePhase25(t *testing.T)` | TestCorpusModelSmokePhase25 proves that two search workspaces can retain independent provenance around one globally identified DOI. |

### [`src/database/helpers_test.go`](../src/database/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`openTestDB`](../src/database/helpers_test.go#L16) | function | 16-24 | `func openTestDB(t *testing.T) *Database` | openTestDB supports the package test suite's open test db setup or assertions. |
| [`createReferenceMentionTestRevision`](../src/database/helpers_test.go#L27) | function | 27-44 | `func createReferenceMentionTestRevision(t *testing.T, db *Database, doi string) int64` | createReferenceMentionTestRevision supports the package test suite's create reference mention test revision setup or assertions. |

### [`src/database/metrics_audit.go`](../src/database/metrics_audit.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`PipelineRunMetric`](../src/database/metrics_audit.go#L14) | struct | 14-19 | ``type PipelineRunMetric struct { PipelineRunID int64 `json:"pipeline_run_id"` Metric string `json:"metric"` Source string `json:"source"` Value int `json:"value"` }`` | PipelineRunMetric is a single counter snapshot for a pipeline run. |
| [`AuditEventRecord`](../src/database/metrics_audit.go#L22) | struct | 22-34 | ``type AuditEventRecord struct { ID int64 `json:"id"` OccurredAt string `json:"occurred_at"` Actor string `json:"actor"` PipelineRunID *int64 `json:"pipeline_run_id,omitempty"` EntityType string `json:"entity_type"` EntityID string `json:"entity_id"` Action string `json:"action"` BeforeJSON string `json:"before_json,omitempty"` AfterJSON string `json:"after_json,omitempty"` MetadataJSON string `json:"metadata_json,omitempty"` CorrelationID string `json:"correlation_id,omitempty"` }`` | AuditEventRecord is the persisted representation of an audit event. |
| [`MetricsRepository`](../src/database/metrics_audit.go#L37) | struct | 37-39 | `type MetricsRepository struct { db *Database }` | MetricsRepository provides CRUD for the pipeline_run_metrics table. |
| [`(*MetricsRepository).Set`](../src/database/metrics_audit.go#L43) | method | 43-58 | `func (*MetricsRepository).Set(runID int64, metric, source string, value int) error` | Set inserts or replaces a metric value for a given run, metric name, and source. If source is empty, it records a whole-run metric. |
| [`(*MetricsRepository).Get`](../src/database/metrics_audit.go#L62) | method | 62-82 | `func (*MetricsRepository).Get(runID int64, metric, source string) (*PipelineRunMetric, error)` | Get returns a single metric for a given run, metric name, and source. Returns nil, nil if the metric is not recorded for this run. |
| [`(*MetricsRepository).ListByRun`](../src/database/metrics_audit.go#L85) | method | 85-112 | `func (*MetricsRepository).ListByRun(runID int64) ([]*PipelineRunMetric, error)` | ListByRun returns all metrics for a given pipeline run, ordered by metric name then source. |
| [`(*MetricsRepository).ListByRunAndSource`](../src/database/metrics_audit.go#L115) | method | 115-142 | `func (*MetricsRepository).ListByRunAndSource(runID int64, source string) ([]*PipelineRunMetric, error)` | ListByRunAndSource returns all metrics for a given run and source. |
| [`AuditEventRepository`](../src/database/metrics_audit.go#L145) | struct | 145-147 | `type AuditEventRepository struct { db *Database }` | AuditEventRepository provides CRUD for the audit_events table. |
| [`(*AuditEventRepository).Insert`](../src/database/metrics_audit.go#L151) | method | 151-186 | `func (*AuditEventRepository).Insert(event *manifest.AuditEvent) (int64, error)` | Insert stores a new audit event. The event's action is validated against the manifest lifecycle vocabulary before insertion. |
| [`(*AuditEventRepository).ListByRun`](../src/database/metrics_audit.go#L189) | method | 189-202 | `func (*AuditEventRepository).ListByRun(runID int64) ([]*AuditEventRecord, error)` | ListByRun returns all audit events for a given pipeline run, ordered by ID. |
| [`(*AuditEventRepository).ListByEntity`](../src/database/metrics_audit.go#L205) | method | 205-218 | `func (*AuditEventRepository).ListByEntity(entityType, entityID string) ([]*AuditEventRecord, error)` | ListByEntity returns all audit events for a given entity type and ID, ordered by ID. |
| [`(*AuditEventRepository).ListByAction`](../src/database/metrics_audit.go#L221) | method | 221-238 | `func (*AuditEventRepository).ListByAction(action manifest.AuditAction) ([]*AuditEventRecord, error)` | ListByAction returns all audit events for a given action, ordered by ID. |
| [`(*AuditEventRepository).ListAll`](../src/database/metrics_audit.go#L242) | method | 242-259 | `func (*AuditEventRepository).ListAll(limit int) ([]*AuditEventRecord, error)` | ListAll returns all audit events ordered by ID, with an optional limit. A limit of 0 returns all events. |
| [`scanAuditEvents`](../src/database/metrics_audit.go#L262) | function | 262-299 | `func scanAuditEvents(rows *sql.Rows) ([]*AuditEventRecord, error)` | scanAuditEvents decodes audit events from a database row. |
| [`PurgeEligibility`](../src/database/metrics_audit.go#L302) | struct | 302-312 | ``type PurgeEligibility struct { // Eligible is true when no other run references this run's artifacts or // reusable outputs. Eligible bool `json:"eligible"` // SharedArtifactCount is the number of artifacts from this run that are // referenced by other runs. SharedArtifactCount int `json:"shared_artifact_count"` // ReusedByCount is the number of other runs that reuse one or more stages // from this run. ReusedByCount int `json:"reused_by_count"` }`` | PurgeEligibility describes whether a pipeline run can be safely purged. |
| [`(*PipelineRunRepository).CheckPurgeEligibility`](../src/database/metrics_audit.go#L317) | method | 317-374 | `func (*PipelineRunRepository).CheckPurgeEligibility(runID int64) (*PurgeEligibility, error)` | CheckPurgeEligibility verifies that no other run shares artifacts or reusable stage outputs from the given run. It is the safety check before purge. Returns an error if no pipeline run with the given ID exists. |

### [`src/database/metrics_audit_integration_test.go`](../src/database/metrics_audit_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestMetricsSetAndGet`](../src/database/metrics_audit_integration_test.go#L13) | test | 13-43 | `func TestMetricsSetAndGet(t *testing.T)` | TestMetricsSetAndGet verifies that a metric can be set and retrieved. |
| [`TestMetricsGetNotFound`](../src/database/metrics_audit_integration_test.go#L46) | test | 46-59 | `func TestMetricsGetNotFound(t *testing.T)` | TestMetricsGetNotFound verifies that Get returns nil for a missing metric. |
| [`TestMetricsSetReplacesValue`](../src/database/metrics_audit_integration_test.go#L62) | test | 62-78 | `func TestMetricsSetReplacesValue(t *testing.T)` | TestMetricsSetReplacesValue verifies that Set replaces an existing value. |
| [`TestMetricsListByRun`](../src/database/metrics_audit_integration_test.go#L81) | test | 81-97 | `func TestMetricsListByRun(t *testing.T)` | TestMetricsListByRun verifies ListByRun returns all metrics for a run. |
| [`TestMetricsListByRunAndSource`](../src/database/metrics_audit_integration_test.go#L100) | test | 100-119 | `func TestMetricsListByRunAndSource(t *testing.T)` | TestMetricsListByRunAndSource verifies source-filtered listing. |
| [`TestAuditEventInsertAndListAll`](../src/database/metrics_audit_integration_test.go#L122) | test | 122-165 | `func TestAuditEventInsertAndListAll(t *testing.T)` | TestAuditEventInsertAndListAll verifies inserting an audit event and listing all. |
| [`TestAuditEventInsertRejectsInvalidAction`](../src/database/metrics_audit_integration_test.go#L168) | test | 168-184 | `func TestAuditEventInsertRejectsInvalidAction(t *testing.T)` | TestAuditEventInsertRejectsInvalidAction verifies that an invalid action is rejected. |
| [`TestAuditEventInsertWithZeroRunID`](../src/database/metrics_audit_integration_test.go#L187) | test | 187-218 | `func TestAuditEventInsertWithZeroRunID(t *testing.T)` | TestAuditEventInsertWithZeroRunID verifies that a zero PipelineRunID is stored as null. |
| [`TestAuditEventListByRun`](../src/database/metrics_audit_integration_test.go#L221) | test | 221-251 | `func TestAuditEventListByRun(t *testing.T)` | TestAuditEventListByRun verifies listing events by pipeline run. |
| [`TestAuditEventListByEntity`](../src/database/metrics_audit_integration_test.go#L254) | test | 254-281 | `func TestAuditEventListByEntity(t *testing.T)` | TestAuditEventListByEntity verifies listing events by entity type and ID. |
| [`TestAuditEventListByAction`](../src/database/metrics_audit_integration_test.go#L284) | test | 284-308 | `func TestAuditEventListByAction(t *testing.T)` | TestAuditEventListByAction verifies listing events by action. |
| [`TestAuditEventListByActionRejectsInvalid`](../src/database/metrics_audit_integration_test.go#L311) | test | 311-319 | `func TestAuditEventListByActionRejectsInvalid(t *testing.T)` | TestAuditEventListByActionRejectsInvalid verifies that an invalid action is rejected. |
| [`TestAuditEventListAllLimit`](../src/database/metrics_audit_integration_test.go#L322) | test | 322-340 | `func TestAuditEventListAllLimit(t *testing.T)` | TestAuditEventListAllLimit verifies the limit parameter on ListAll. |
| [`TestAuditEventRejectsUpdate`](../src/database/metrics_audit_integration_test.go#L344) | test | 344-359 | `func TestAuditEventRejectsUpdate(t *testing.T)` | TestAuditEventRejectsUpdate verifies that UPDATE on audit_events is rejected by the append-only trigger. |
| [`TestAuditEventRejectsDelete`](../src/database/metrics_audit_integration_test.go#L363) | test | 363-392 | `func TestAuditEventRejectsDelete(t *testing.T)` | TestAuditEventRejectsDelete verifies that DELETE on audit_events is rejected by the append-only trigger. |

### [`src/database/pipeline_runs.go`](../src/database/pipeline_runs.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`PipelineRun`](../src/database/pipeline_runs.go#L17) | struct | 17-30 | ``type PipelineRun struct { ID int64 `json:"id"` Step string `json:"step"` StartedAt string `json:"started_at"` FinishedAt *string `json:"finished_at,omitempty"` Status string `json:"status"` Summary *string `json:"summary,omitempty"` SearchQuery *string `json:"search_query,omitempty"` ExecutionPlanID *int64 `json:"execution_plan_id,omitempty"` AttemptNumber *int `json:"attempt_number,omitempty"` VisibilityState string `json:"visibility_state"` TrashedAt *string `json:"trashed_at,omitempty"` TrashReason *string `json:"trash_reason,omitempty"` }`` | PipelineRun represents a row in the pipeline_runs table. |
| [`AttemptAlreadyRunningError`](../src/database/pipeline_runs.go#L34) | struct | 34-37 | `type AttemptAlreadyRunningError struct { ExecutionPlanID int64 PipelineRunID int64 }` | AttemptAlreadyRunningError reports the active attempt that prevents another attempt for the same execution plan from starting. |
| [`(*AttemptAlreadyRunningError).Error`](../src/database/pipeline_runs.go#L40) | method | 40-42 | `func (*AttemptAlreadyRunningError).Error() string` | Error returns the receiver's diagnostic message. |
| [`PipelineRunRepository`](../src/database/pipeline_runs.go#L45) | struct | 45-47 | `type PipelineRunRepository struct { db *Database }` | PipelineRunRepository provides CRUD for the pipeline_runs table. |
| [`(*PipelineRunRepository).StartRun`](../src/database/pipeline_runs.go#L51) | method | 51-67 | `func (*PipelineRunRepository).StartRun(step, searchQuery string) (int64, error)` | StartRun records the start of a pipeline step. Returns the run ID. This is the legacy entry point; new code should use StartAttempt. |
| [`(*PipelineRunRepository).StartAttempt`](../src/database/pipeline_runs.go#L73) | method | 73-75 | `func (*PipelineRunRepository).StartAttempt(executionPlanID int64, step, searchQuery string) (int64, int, error)` | StartAttempt records the start of a pipeline run attempt linked to an execution plan. It atomically computes the next attempt_number for the given plan and retries on transient UNIQUE constraint conflicts or SQLITE_BUSY. Returns the run ID and attempt number. |
| [`(*PipelineRunRepository).StartAttemptIfIdle`](../src/database/pipeline_runs.go#L80) | method | 80-82 | `func (*PipelineRunRepository).StartAttemptIfIdle(executionPlanID int64, step, searchQuery string) (int64, int, error)` | StartAttemptIfIdle starts a new attempt only when this execution plan has no running attempt. The check and insert share one transaction so callers cannot race from a separate read into duplicate active work. |
| [`(*PipelineRunRepository).startAttempt`](../src/database/pipeline_runs.go#L85) | method | 85-167 | `func (*PipelineRunRepository).startAttempt(executionPlanID int64, step, searchQuery string, rejectRunning bool) (int64, int, error)` | startAttempt atomically starts the next plan attempt, optionally rejecting an already-running attempt. |
| [`(*PipelineRunRepository).FinishRun`](../src/database/pipeline_runs.go#L173) | method | 173-196 | `func (*PipelineRunRepository).FinishRun(runID int64, status string, summary string) error` | FinishRun marks a pipeline run as completed (or failed). Supports both legacy runs and new attempt-based runs. It validates the status against the manifest lifecycle vocabulary and only sets finished_at for terminal statuses (completed, failed). |
| [`(*PipelineRunRepository).Trash`](../src/database/pipeline_runs.go#L199) | method | 199-210 | `func (*PipelineRunRepository).Trash(runID int64, reason string) error` | Trash marks a pipeline run as trashed (soft-deleted). |
| [`(*PipelineRunRepository).Restore`](../src/database/pipeline_runs.go#L213) | method | 213-224 | `func (*PipelineRunRepository).Restore(runID int64) error` | Restore sets a trashed pipeline run back to active visibility. |
| [`(*PipelineRunRepository).GetByID`](../src/database/pipeline_runs.go#L227) | method | 227-271 | `func (*PipelineRunRepository).GetByID(runID int64) (*PipelineRun, error)` | GetByID returns a pipeline run by its primary key, or nil if not found. |
| [`(*PipelineRunRepository).ListByPlan`](../src/database/pipeline_runs.go#L274) | method | 274-288 | `func (*PipelineRunRepository).ListByPlan(executionPlanID int64) ([]*PipelineRun, error)` | ListByPlan returns all runs for a given execution plan, ordered by attempt number. |
| [`(*PipelineRunRepository).ListByVisibility`](../src/database/pipeline_runs.go#L291) | method | 291-305 | `func (*PipelineRunRepository).ListByVisibility(visibilityState string) ([]*PipelineRun, error)` | ListByVisibility returns all runs with a given visibility state, ordered by ID. |
| [`scanPipelineRuns`](../src/database/pipeline_runs.go#L308) | function | 308-352 | `func scanPipelineRuns(rows *sql.Rows) ([]*PipelineRun, error)` | scanPipelineRuns decodes pipeline runs from a database row. |
| [`scanAll`](../src/database/pipeline_runs.go#L355) | function | 355-384 | `func scanAll(rows *sql.Rows) ([]map[string]any, error)` | scanAll converts arbitrary SQL rows to maps keyed by column name. |
| [`isRetryableError`](../src/database/pipeline_runs.go#L388) | function | 388-404 | `func isRetryableError(err error) bool` | isRetryableError returns true if the error is a transient SQLite error that can be retried: UNIQUE constraint violation or SQLITE_BUSY (database locked). |
| [`(*Database).withTx`](../src/database/pipeline_runs.go#L407) | method | 407-424 | `func (*Database).withTx(ctx context.Context, fn func(*sql.Tx) error) error` | withTx runs fn inside a transaction, rolling back on error and committing on success. |
| [`nullStrPtr`](../src/database/pipeline_runs.go#L427) | function | 427-432 | `func nullStrPtr(s *string) any` | nullStrPtr returns nil if s is nil, otherwise returns a sql.NullString with the value. |

### [`src/database/pipeline_runs_integration_test.go`](../src/database/pipeline_runs_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestPipelineRuns`](../src/database/pipeline_runs_integration_test.go#L11) | test | 11-27 | `func TestPipelineRuns(t *testing.T)` | TestPipelineRuns verifies pipeline runs. |
| [`TestPipelineRunAttemptNumbering`](../src/database/pipeline_runs_integration_test.go#L31) | test | 31-82 | `func TestPipelineRunAttemptNumbering(t *testing.T)` | TestPipelineRunAttemptNumbering verifies that StartAttempt auto-increments the attempt_number per execution plan. |
| [`TestPipelineRunStartAttemptIfIdleRejectsRunningAttempt`](../src/database/pipeline_runs_integration_test.go#L85) | test | 85-112 | `func TestPipelineRunStartAttemptIfIdleRejectsRunningAttempt(t *testing.T)` | TestPipelineRunStartAttemptIfIdleRejectsRunningAttempt verifies pipeline run start attempt if idle rejects running attempt. |
| [`TestPipelineRunStartAttemptIfIdleRejectsConcurrentAttempts`](../src/database/pipeline_runs_integration_test.go#L115) | test | 115-147 | `func TestPipelineRunStartAttemptIfIdleRejectsConcurrentAttempts(t *testing.T)` | TestPipelineRunStartAttemptIfIdleRejectsConcurrentAttempts verifies pipeline run start attempt if idle rejects concurrent attempts. |
| [`TestPipelineRunListByPlan`](../src/database/pipeline_runs_integration_test.go#L150) | test | 150-179 | `func TestPipelineRunListByPlan(t *testing.T)` | TestPipelineRunListByPlan verifies that ListByPlan returns all attempts for a plan. |
| [`TestPipelineRunTrashAndRestore`](../src/database/pipeline_runs_integration_test.go#L182) | test | 182-231 | `func TestPipelineRunTrashAndRestore(t *testing.T)` | TestPipelineRunTrashAndRestore verifies the trash/restore lifecycle. |
| [`TestPipelineRunListByVisibility`](../src/database/pipeline_runs_integration_test.go#L234) | test | 234-271 | `func TestPipelineRunListByVisibility(t *testing.T)` | TestPipelineRunListByVisibility verifies filtering by visibility state. |
| [`TestPipelineRunAttemptFKReachability`](../src/database/pipeline_runs_integration_test.go#L275) | test | 275-284 | `func TestPipelineRunAttemptFKReachability(t *testing.T)` | TestPipelineRunAttemptFKReachability verifies that a run source referencing a non-existent pipeline run is rejected by the foreign key constraint. |
| [`TestLegacyStartRunBackwardCompat`](../src/database/pipeline_runs_integration_test.go#L288) | test | 288-327 | `func TestLegacyStartRunBackwardCompat(t *testing.T)` | TestLegacyStartRunBackwardCompat verifies that the legacy StartRun/FinishRun methods still work and leave the new columns as NULL. |
| [`TestPipelineRunConcurrentStartAttempt`](../src/database/pipeline_runs_integration_test.go#L332) | test | 332-374 | `func TestPipelineRunConcurrentStartAttempt(t *testing.T)` | TestPipelineRunConcurrentStartAttempt verifies that concurrent callers to StartAttempt each receive a unique attempt_number, enforced by the UNIQUE constraint and the retry loop. |
| [`TestPipelineRunFinishRunInvalidStatus`](../src/database/pipeline_runs_integration_test.go#L378) | test | 378-407 | `func TestPipelineRunFinishRunInvalidStatus(t *testing.T)` | TestPipelineRunFinishRunInvalidStatus verifies that FinishRun rejects invalid attempt statuses. |
| [`TestPurgeEligibilityNoSharedData`](../src/database/pipeline_runs_integration_test.go#L411) | test | 411-430 | `func TestPurgeEligibilityNoSharedData(t *testing.T)` | TestPurgeEligibilityNoSharedData verifies that a run with no shared data is eligible for purge. |
| [`TestPurgeEligibilitySharedArtifact`](../src/database/pipeline_runs_integration_test.go#L434) | test | 434-466 | `func TestPurgeEligibilitySharedArtifact(t *testing.T)` | TestPurgeEligibilitySharedArtifact verifies that sharing an artifact makes the source run ineligible for purge. |
| [`TestPurgeEligibilityReusedBy`](../src/database/pipeline_runs_integration_test.go#L470) | test | 470-496 | `func TestPurgeEligibilityReusedBy(t *testing.T)` | TestPurgeEligibilityReusedBy verifies that another run reusing a stage from this run makes the source run ineligible for purge. |
| [`TestPurgeEligibilitySharedOutputArtifact`](../src/database/pipeline_runs_integration_test.go#L500) | test | 500-529 | `func TestPurgeEligibilitySharedOutputArtifact(t *testing.T)` | TestPurgeEligibilitySharedOutputArtifact verifies that sharing an artifact as another run's output makes the source run ineligible for purge. |
| [`TestPurgeEligibilityNonexistentRun`](../src/database/pipeline_runs_integration_test.go#L533) | test | 533-544 | `func TestPurgeEligibilityNonexistentRun(t *testing.T)` | TestPurgeEligibilityNonexistentRun verifies that checking eligibility for a non-existent run returns an error. |
| [`TestPurgeEligibilitySelfReuseDoesNotBlock`](../src/database/pipeline_runs_integration_test.go#L548) | test | 548-568 | `func TestPurgeEligibilitySelfReuseDoesNotBlock(t *testing.T)` | TestPurgeEligibilitySelfReuseDoesNotBlock verifies that a self-reuse record (a step reusing from the same run) does not falsely make the run ineligible. |

### [`src/database/reference_mentions.go`](../src/database/reference_mentions.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`ReferenceMention`](../src/database/reference_mentions.go#L14) | struct | 14-26 | ``type ReferenceMention struct { ID int64 `json:"id"` WorkRevisionID int64 `json:"work_revision_id"` ResolvedWorkID int64 `json:"resolved_work_id"` MentionOrder int `json:"mention_order"` RawReference string `json:"raw_reference"` DOI string `json:"doi"` Title string `json:"title"` Author string `json:"author"` Year int `json:"year"` Source string `json:"source"` CreatedAt string `json:"created_at"` }`` | ReferenceMention is one cited reference observed on an immutable work revision. ResolvedWorkID is optional because most citations are external to the current workspace. |
| [`ReferenceMentionRepository`](../src/database/reference_mentions.go#L29) | struct | 29-31 | `type ReferenceMentionRepository struct { db *Database }` | ReferenceMentionRepository persists immutable reference mentions. |
| [`(*ReferenceMentionRepository).Create`](../src/database/reference_mentions.go#L35) | method | 35-72 | `func (*ReferenceMentionRepository).Create(mention *ReferenceMention) (int64, error)` | Create stores one ordered reference mention. A DOI is normalized and, when it identifies an existing work, linked through ResolvedWorkID automatically. |
| [`(*ReferenceMentionRepository).GetByID`](../src/database/reference_mentions.go#L75) | method | 75-79 | `func (*ReferenceMentionRepository).GetByID(id int64) (*ReferenceMention, error)` | GetByID returns a mention by primary key, or nil if it does not exist. |
| [`(*ReferenceMentionRepository).GetByRevisionID`](../src/database/reference_mentions.go#L82) | method | 82-100 | `func (*ReferenceMentionRepository).GetByRevisionID(revisionID int64) ([]*ReferenceMention, error)` | GetByRevisionID returns a revision's references in their source order. |
| [`(*ReferenceMentionRepository).GetByResolvedWorkID`](../src/database/reference_mentions.go#L103) | method | 103-121 | `func (*ReferenceMentionRepository).GetByResolvedWorkID(workID int64) ([]*ReferenceMention, error)` | GetByResolvedWorkID returns workspace citations that resolve to one work. |
| [`scanReferenceMention`](../src/database/reference_mentions.go#L124) | function | 124-157 | `func scanReferenceMention(row scannable) (*ReferenceMention, error)` | scanReferenceMention decodes reference mention from a database row. |

### [`src/database/reference_mentions_integration_test.go`](../src/database/reference_mentions_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestReferenceMentionExternalReferencesRemainDistinct`](../src/database/reference_mentions_integration_test.go#L12) | test | 12-45 | `func TestReferenceMentionExternalReferencesRemainDistinct(t *testing.T)` | TestReferenceMentionExternalReferencesRemainDistinct verifies reference mention external references remain distinct. |
| [`TestReferenceMentionResolvesKnownWork`](../src/database/reference_mentions_integration_test.go#L48) | test | 48-74 | `func TestReferenceMentionResolvesKnownWork(t *testing.T)` | TestReferenceMentionResolvesKnownWork verifies reference mention resolves known work. |
| [`TestReferenceMentionValidationAndUniqueness`](../src/database/reference_mentions_integration_test.go#L77) | test | 77-94 | `func TestReferenceMentionValidationAndUniqueness(t *testing.T)` | TestReferenceMentionValidationAndUniqueness verifies reference mention validation and uniqueness. |
| [`TestReferenceMentionSnapshotsAreAppendOnly`](../src/database/reference_mentions_integration_test.go#L97) | test | 97-127 | `func TestReferenceMentionSnapshotsAreAppendOnly(t *testing.T)` | TestReferenceMentionSnapshotsAreAppendOnly verifies reference mention snapshots are append only. |
| [`TestV00005ReferenceMentionsMigrationApplies`](../src/database/reference_mentions_integration_test.go#L130) | test | 130-155 | `func TestV00005ReferenceMentionsMigrationApplies(t *testing.T)` | TestV00005ReferenceMentionsMigrationApplies verifies v00005 reference mentions migration applies. |

### [`src/database/run_artifacts.go`](../src/database/run_artifacts.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`RunArtifact`](../src/database/run_artifacts.go#L17) | struct | 17-22 | ``type RunArtifact struct { PipelineRunID int64 `json:"pipeline_run_id"` ArtifactID int64 `json:"artifact_id"` ArtifactRole string `json:"artifact_role"` CreatedAt string `json:"created_at"` }`` | RunArtifact links an attempt to a content-addressed configuration snapshot. The role distinguishes the raw workspace file from its resolved and input manifests without duplicating immutable artifact payloads. |
| [`RunArtifactRepository`](../src/database/run_artifacts.go#L25) | struct | 25-27 | `type RunArtifactRepository struct { db *Database }` | RunArtifactRepository manages attempt-specific configuration artifact links. |
| [`(*RunArtifactRepository).Link`](../src/database/run_artifacts.go#L31) | method | 31-58 | `func (*RunArtifactRepository).Link(pipelineRunID, artifactID int64, role string) error` | Link records one snapshot role for an attempt. Repeating the same link is idempotent; assigning a role to a different artifact is rejected. |

### [`src/database/run_artifacts_integration_test.go`](../src/database/run_artifacts_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestArtifactCreateAndLookup`](../src/database/run_artifacts_integration_test.go#L11) | test | 11-86 | `func TestArtifactCreateAndLookup(t *testing.T)` | TestArtifactCreateAndLookup verifies artifact creation and content-hash dedup. |
| [`TestRunArtifactLinksAreRoleScopedAndImmutable`](../src/database/run_artifacts_integration_test.go#L89) | test | 89-113 | `func TestRunArtifactLinksAreRoleScopedAndImmutable(t *testing.T)` | TestRunArtifactLinksAreRoleScopedAndImmutable verifies run artifact links are role scoped and immutable. |

### [`src/database/sql_helpers.go`](../src/database/sql_helpers.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`nullStr`](../src/database/sql_helpers.go#L7) | function | 7-12 | `func nullStr(value string) *string` | nullStr represents optional text consistently across workspace repositories. |
| [`nullInt`](../src/database/sql_helpers.go#L15) | function | 15-20 | `func nullInt(value int64) any` | nullInt represents optional integer values consistently across workspace repositories. |

### [`src/database/sql_helpers_unit_test.go`](../src/database/sql_helpers_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestNullStr`](../src/database/sql_helpers_unit_test.go#L11) | test | 11-18 | `func TestNullStr(t *testing.T)` | TestNullStr verifies null str. |
| [`TestNullInt`](../src/database/sql_helpers_unit_test.go#L21) | test | 21-28 | `func TestNullInt(t *testing.T)` | TestNullInt verifies null int. |

### [`src/database/works.go`](../src/database/works.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Work`](../src/database/works.go#L17) | struct | 17-21 | ``type Work struct { ID int64 `json:"id"` DOI string `json:"doi"` CreatedAt string `json:"created_at"` }`` | Work represents a single work with a globally unique DOI. Title-only records each get their own Work row; they are never merged by title alone (uncertain identity). |
| [`WorkIdentifier`](../src/database/works.go#L25) | struct | 25-31 | ``type WorkIdentifier struct { ID int64 `json:"id"` WorkID int64 `json:"work_id"` Namespace string `json:"namespace"` Identifier string `json:"identifier"` CreatedAt string `json:"created_at"` }`` | WorkIdentifier is an alternative identifier for a work, scoped by namespace. Examples: ("scopus", "2-s2.0-84912345678"), ("openalex", "W1234567890"). |
| [`WorkRepository`](../src/database/works.go#L34) | struct | 34-36 | `type WorkRepository struct { db *Database }` | WorkRepository provides CRUD for the works table. |
| [`(*WorkRepository).CreateByDOI`](../src/database/works.go#L41) | method | 41-86 | `func (*WorkRepository).CreateByDOI(doi string) (int64, error)` | CreateByDOI inserts a new work by DOI. The DOI is normalized (lowercased, URL prefix stripped) before storage. If the normalized DOI already exists, returns the existing work ID (INSERT OR IGNORE semantics). |
| [`(*WorkRepository).CreateWithoutDOI`](../src/database/works.go#L90) | method | 90-105 | `func (*WorkRepository).CreateWithoutDOI() (int64, error)` | CreateWithoutDOI inserts a new work without a DOI (e.g. a title-only record). Each call creates a distinct row; uncertain records are never globally merged. |
| [`(*WorkRepository).GetByID`](../src/database/works.go#L108) | method | 108-127 | `func (*WorkRepository).GetByID(id int64) (*Work, error)` | GetByID returns a work by its primary key, or nil if not found. |
| [`(*WorkRepository).GetByDOI`](../src/database/works.go#L133) | method | 133-158 | `func (*WorkRepository).GetByDOI(doi string) (*Work, error)` | GetByDOI returns a work by its DOI, or nil if not found. The DOI is normalized the same way as CreateByDOI so that "10.1000/x", "10.1000/X", and "https://doi.org/10.1000/x" all resolve correctly. |
| [`(*WorkRepository).ListByIDs`](../src/database/works.go#L161) | method | 161-205 | `func (*WorkRepository).ListByIDs(ids []int64) ([]*Work, error)` | ListByIDs returns works matching the given IDs, in ID order. |
| [`(*WorkRepository).Count`](../src/database/works.go#L208) | method | 208-217 | `func (*WorkRepository).Count() (int, error)` | Count returns the total number of works. |
| [`WorkIdentifierRepository`](../src/database/works.go#L220) | struct | 220-222 | `type WorkIdentifierRepository struct { db *Database }` | WorkIdentifierRepository provides CRUD for the work_identifiers table. |
| [`(*WorkIdentifierRepository).Insert`](../src/database/works.go#L227) | method | 227-298 | `func (*WorkIdentifierRepository).Insert(workID int64, namespace, identifier string) (int64, error)` | Insert adds a new identifier for a work. If the (namespace, identifier) pair already exists for the same work, returns the existing ID. If it belongs to a different work, returns an error to prevent silent ownership conflicts. |
| [`(*WorkIdentifierRepository).GetByID`](../src/database/works.go#L301) | method | 301-317 | `func (*WorkIdentifierRepository).GetByID(id int64) (*WorkIdentifier, error)` | GetByID returns a work identifier by its primary key, or nil if not found. |
| [`(*WorkIdentifierRepository).GetByWorkID`](../src/database/works.go#L320) | method | 320-347 | `func (*WorkIdentifierRepository).GetByWorkID(workID int64) ([]*WorkIdentifier, error)` | GetByWorkID returns all identifiers for a given work, ordered by ID. |
| [`(*WorkIdentifierRepository).GetByNamespaceAndIdentifier`](../src/database/works.go#L351) | method | 351-372 | `func (*WorkIdentifierRepository).GetByNamespaceAndIdentifier(namespace, identifier string) (*WorkIdentifier, error)` | GetByNamespaceAndIdentifier returns the work identifier record for the given namespace and identifier pair, or nil if not found. |
| [`(*WorkIdentifierRepository).CountByWorkID`](../src/database/works.go#L375) | method | 375-386 | `func (*WorkIdentifierRepository).CountByWorkID(workID int64) (int, error)` | CountByWorkID returns the number of identifiers for a given work. |
| [`NormalizeDOI`](../src/database/works.go#L390) | function | 390-396 | `func NormalizeDOI(doi string) string` | NormalizeDOI applies the canonical DOI representation used by the works table and the companion PDF store. |
| [`WorkRevision`](../src/database/works.go#L402) | struct | 402-421 | ``type WorkRevision struct { ID int64 `json:"id"` WorkID int64 `json:"work_id"` PipelineRunID int64 `json:"pipeline_run_id"` ProducerStage string `json:"producer_stage"` FieldSchemaVersion string `json:"field_schema_version"` PayloadHash string `json:"payload_hash"` Title string `json:"title"` Abstract string `json:"abstract"` Year int `json:"year"` Journal string `json:"journal"` Publisher string `json:"publisher"` Source string `json:"source"` Keywords string `json:"keywords"` // JSON array KeywordsPlus string `json:"keywords_plus"` // JSON array CitationCount int `json:"citation_count"` ReferenceCount int `json:"reference_count"` ExtensionData string `json:"extension_data"` // JSON object CreatedAt string `json:"created_at"` }`` | WorkRevision is an immutable snapshot of a work's typed core metadata and extension data at the point it was produced by a pipeline run stage. producer_stage records which pipeline stage created the revision (e.g. "parse", "enrich"). |
| [`RunWorkStage`](../src/database/works.go#L428) | struct | 428-437 | ``type RunWorkStage struct { ID int64 `json:"id"` PipelineRunID int64 `json:"pipeline_run_id"` WorkID int64 `json:"work_id"` StageName string `json:"stage_name"` Outcome string `json:"outcome"` Reason string `json:"reason"` CreatedAt string `json:"created_at"` UpdatedAt string `json:"updated_at"` }`` | RunWorkStage records what happened to one work at one pipeline stage within a single pipeline run. The (pipeline_run_id, work_id, stage_name) triplet is unique so the same stage cannot report two different outcomes for the same work in the same run. created_at is the first time the outcome was set; updated_at is the most recent time it changed (via ON CONFLICT DO UPDATE). |
| [`WorkRevisionRepository`](../src/database/works.go#L486) | struct | 486-488 | `type WorkRevisionRepository struct { db *Database }` | WorkRevisionRepository provides CRUD for the work_revisions table. |
| [`(*WorkRevisionRepository).Create`](../src/database/works.go#L494) | method | 494-542 | `func (*WorkRevisionRepository).Create(rev *WorkRevision) (int64, error)` | Create inserts a new immutable work revision and returns its ID. The payload hash is computed from the supplied core fields and extension data. If FieldSchemaVersion is empty, it defaults to "1". ProducerStage must be a known pipeline stage; legacy_unknown is rejected. |
| [`(*WorkRevisionRepository).GetByID`](../src/database/works.go#L545) | method | 545-553 | `func (*WorkRevisionRepository).GetByID(id int64) (*WorkRevision, error)` | GetByID returns a work revision by its primary key, or nil if not found. |
| [`(*WorkRevisionRepository).GetByWorkID`](../src/database/works.go#L557) | method | 557-586 | `func (*WorkRevisionRepository).GetByWorkID(workID int64) ([]*WorkRevision, error)` | GetByWorkID returns all revisions for a given work, ordered by ID ascending (chronological order). |
| [`(*WorkRevisionRepository).GetByRunID`](../src/database/works.go#L589) | method | 589-618 | `func (*WorkRevisionRepository).GetByRunID(runID int64) ([]*WorkRevision, error)` | GetByRunID returns all revisions created by a given pipeline run. |
| [`(*WorkRevisionRepository).CountByWorkID`](../src/database/works.go#L621) | method | 621-632 | `func (*WorkRevisionRepository).CountByWorkID(workID int64) (int, error)` | CountByWorkID returns the number of revisions for a given work. |
| [`RunWorkStageRepository`](../src/database/works.go#L635) | struct | 635-637 | `type RunWorkStageRepository struct { db *Database }` | RunWorkStageRepository provides CRUD for the run_work_stages table. |
| [`(*RunWorkStageRepository).SetOutcome`](../src/database/works.go#L644) | method | 644-678 | `func (*RunWorkStageRepository).SetOutcome(runID, workID int64, stageName, outcome, reason string) error` | SetOutcome inserts or updates a stage outcome for a given work in a run. Uses INSERT ... ON CONFLICT DO UPDATE so the row identity (id, created_at) is preserved across progressive outcome updates (e.g. "pending" -> "parsed"). Stage names and outcomes are validated against known stage/outcome pairs; impossible combinations (e.g. parse/valid) are rejected. |
| [`validProducerStage`](../src/database/works.go#L746) | function | 746 | `func validProducerStage(s string) bool` | validProducerStage reports whether the supplied producer stage is supported. |
| [`validStageName`](../src/database/works.go#L749) | function | 749 | `func validStageName(s string) bool` | validStageName reports whether the supplied stage name is supported. |
| [`validStageOutcomeForStage`](../src/database/works.go#L752) | function | 752-758 | `func validStageOutcomeForStage(stageName, outcome string) bool` | validStageOutcomeForStage returns true if outcome is allowed for the given stage. |
| [`(*RunWorkStageRepository).GetByRunAndWork`](../src/database/works.go#L762) | method | 762-768 | `func (*RunWorkStageRepository).GetByRunAndWork(runID, workID int64, stageName string) (*RunWorkStage, error)` | GetByRunAndWork returns the stage outcome for a specific work, run, and stage name, or nil if not found. |
| [`(*RunWorkStageRepository).GetByRunID`](../src/database/works.go#L771) | method | 771-795 | `func (*RunWorkStageRepository).GetByRunID(runID int64) ([]*RunWorkStage, error)` | GetByRunID returns all stage outcomes for a given pipeline run, ordered by ID. |
| [`(*RunWorkStageRepository).GetByWorkID`](../src/database/works.go#L798) | method | 798-822 | `func (*RunWorkStageRepository).GetByWorkID(workID int64) ([]*RunWorkStage, error)` | GetByWorkID returns all stage outcomes across runs for a given work, ordered by ID. |
| [`(*RunWorkStageRepository).CountByStageAndOutcome`](../src/database/works.go#L826) | method | 826-841 | `func (*RunWorkStageRepository).CountByStageAndOutcome(runID int64, stageName, outcome string) (int, error)` | CountByStageAndOutcome returns the number of works that reached a given stage outcome within a pipeline run. Useful for dashboard funnel counts. |
| [`scannable`](../src/database/works.go#L844) | interface | 844-846 | `type scannable interface { Scan(dest ...any) error }` | scannable defines the behavior required of scannable implementations. |
| [`scanWorkRevision`](../src/database/works.go#L849) | function | 849-885 | `func scanWorkRevision(row scannable) (*WorkRevision, error)` | scanWorkRevision decodes work revision from a database row. |
| [`scanRunWorkStage`](../src/database/works.go#L888) | function | 888-902 | `func scanRunWorkStage(row scannable) (*RunWorkStage, error)` | scanRunWorkStage decodes run work stage from a database row. |
| [`nullStrPtrVal`](../src/database/works.go#L905) | function | 905-910 | `func nullStrPtrVal(ns sql.NullString) string` | nullStrPtrVal returns a nullable SQL string's value or an empty string. |
| [`computeRevisionPayloadHash`](../src/database/works.go#L917) | function | 917-947 | `func computeRevisionPayloadHash(rev *WorkRevision) string` | computeRevisionPayloadHash computes a deterministic SHA-256 hex hash from the typed core fields and extension data of a WorkRevision. producer_stage is excluded because it is provenance metadata, not content. field_schema_version is included because it affects how the fields should be interpreted. |

### [`src/database/works_integration_test.go`](../src/database/works_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestWorkCreateByDOI`](../src/database/works_integration_test.go#L13) | test | 13-62 | `func TestWorkCreateByDOI(t *testing.T)` | TestWorkCreateByDOI verifies that inserting by DOI creates a work row and that the same normalized DOI returns the existing ID. |
| [`TestWorkEmptyDOIReturnsError`](../src/database/works_integration_test.go#L65) | test | 65-73 | `func TestWorkEmptyDOIReturnsError(t *testing.T)` | TestWorkEmptyDOIReturnsError verifies that CreateByDOI with an empty string fails. |
| [`TestWorkCreateWithoutDOI`](../src/database/works_integration_test.go#L76) | test | 76-116 | `func TestWorkCreateWithoutDOI(t *testing.T)` | TestWorkCreateWithoutDOI verifies that title-only records get distinct work rows. |
| [`TestWorkGetByIDAndDOI`](../src/database/works_integration_test.go#L119) | test | 119-188 | `func TestWorkGetByIDAndDOI(t *testing.T)` | TestWorkGetByIDAndDOI verifies GetByID and GetByDOI lookups. |
| [`TestWorkListByIDs`](../src/database/works_integration_test.go#L191) | test | 191-218 | `func TestWorkListByIDs(t *testing.T)` | TestWorkListByIDs verifies batch lookup of works. |
| [`TestWorkCount`](../src/database/works_integration_test.go#L221) | test | 221-244 | `func TestWorkCount(t *testing.T)` | TestWorkCount verifies the count of works. |
| [`TestWorkIdentifierInsert`](../src/database/works_integration_test.go#L247) | test | 247-282 | `func TestWorkIdentifierInsert(t *testing.T)` | TestWorkIdentifierInsert verifies adding identifiers to a work. |
| [`TestWorkIdentifierInsertOwnershipConflict`](../src/database/works_integration_test.go#L287) | test | 287-314 | `func TestWorkIdentifierInsertOwnershipConflict(t *testing.T)` | TestWorkIdentifierInsertOwnershipConflict verifies that inserting a (namespace, identifier) pair that already belongs to a different work returns an error. |
| [`TestWorkIdentifierEmptyArgs`](../src/database/works_integration_test.go#L317) | test | 317-335 | `func TestWorkIdentifierEmptyArgs(t *testing.T)` | TestWorkIdentifierEmptyArgs verifies that empty namespace or identifier fails. |
| [`TestWorkIdentifierGetByWorkID`](../src/database/works_integration_test.go#L338) | test | 338-373 | `func TestWorkIdentifierGetByWorkID(t *testing.T)` | TestWorkIdentifierGetByWorkID verifies listing identifiers for a work. |
| [`TestWorkIdentifierGetByNamespaceAndIdentifier`](../src/database/works_integration_test.go#L376) | test | 376-412 | `func TestWorkIdentifierGetByNamespaceAndIdentifier(t *testing.T)` | TestWorkIdentifierGetByNamespaceAndIdentifier verifies lookup by namespace+identifier. |
| [`TestWorkIdentifierCountByWorkID`](../src/database/works_integration_test.go#L415) | test | 415-442 | `func TestWorkIdentifierCountByWorkID(t *testing.T)` | TestWorkIdentifierCountByWorkID verifies the identifier count for a work. |
| [`TestWorkIdentifierGetByID`](../src/database/works_integration_test.go#L445) | test | 445-481 | `func TestWorkIdentifierGetByID(t *testing.T)` | TestWorkIdentifierGetByID verifies lookup by primary key. |
| [`TestWorkSameDOIMultipleSearches`](../src/database/works_integration_test.go#L486) | test | 486-516 | `func TestWorkSameDOIMultipleSearches(t *testing.T)` | TestWorkSameDOIMultipleSearches verifies that the same DOI used across different searches always resolves to the same global work identity. This is the foundation for per-search membership in Phase 2.2. |
| [`TestWorkRevisionCreate`](../src/database/works_integration_test.go#L520) | test | 520-592 | `func TestWorkRevisionCreate(t *testing.T)` | TestWorkRevisionCreate verifies basic creation and retrieval of a work revision with producer_stage, field_schema_version defaulting, and payload hash computation. |
| [`TestWorkRevisionRejectsEmptyProducerStage`](../src/database/works_integration_test.go#L596) | test | 596-616 | `func TestWorkRevisionRejectsEmptyProducerStage(t *testing.T)` | TestWorkRevisionRejectsEmptyProducerStage verifies that Create rejects a revision without a producer_stage. |
| [`TestWorkRevisionImmutability`](../src/database/works_integration_test.go#L620) | test | 620-700 | `func TestWorkRevisionImmutability(t *testing.T)` | TestWorkRevisionImmutability verifies that two revisions for the same work coexist without overwriting each other. |
| [`TestWorkRevisionGetByRunID`](../src/database/works_integration_test.go#L703) | test | 703-741 | `func TestWorkRevisionGetByRunID(t *testing.T)` | TestWorkRevisionGetByRunID verifies listing revisions by pipeline run. |
| [`TestWorkRevisionAbortUpdate`](../src/database/works_integration_test.go#L745) | test | 745-765 | `func TestWorkRevisionAbortUpdate(t *testing.T)` | TestWorkRevisionAbortUpdate verifies that the append-only trigger on work_revisions rejects UPDATE statements. |
| [`TestWorkRevisionAbortDelete`](../src/database/works_integration_test.go#L769) | test | 769-789 | `func TestWorkRevisionAbortDelete(t *testing.T)` | TestWorkRevisionAbortDelete verifies that the append-only trigger on work_revisions rejects DELETE statements. |
| [`TestWorkRevisionRejectsUnknownProducerStage`](../src/database/works_integration_test.go#L792) | test | 792-812 | `func TestWorkRevisionRejectsUnknownProducerStage(t *testing.T)` | TestWorkRevisionRejectsUnknownProducerStage verifies work revision rejects unknown producer stage. |
| [`TestWorkRevisionRejectsLegacyUnknown`](../src/database/works_integration_test.go#L816) | test | 816-836 | `func TestWorkRevisionRejectsLegacyUnknown(t *testing.T)` | TestWorkRevisionRejectsLegacyUnknown verifies that legacy_unknown is rejected for new revisions (it is only for pre-existing migrated rows). |
| [`TestWorkRevisionAcceptsValidProducerStages`](../src/database/works_integration_test.go#L840) | test | 840-870 | `func TestWorkRevisionAcceptsValidProducerStages(t *testing.T)` | TestWorkRevisionAcceptsValidProducerStages verifies that all known pipeline stages are accepted as producer_stage. |
| [`TestRunWorkStageSetOutcome`](../src/database/works_integration_test.go#L873) | test | 873-934 | `func TestRunWorkStageSetOutcome(t *testing.T)` | TestRunWorkStageSetOutcome verifies setting and getting stage outcomes. |
| [`TestRunWorkStageReplaceOutcome`](../src/database/works_integration_test.go#L938) | test | 938-983 | `func TestRunWorkStageReplaceOutcome(t *testing.T)` | TestRunWorkStageReplaceOutcome verifies that INSERT OR REPLACE updates an existing stage outcome (e.g., moving from "pending" to "parsed"). |
| [`TestRunWorkStageCountByStageAndOutcome`](../src/database/works_integration_test.go#L986) | test | 986-1019 | `func TestRunWorkStageCountByStageAndOutcome(t *testing.T)` | TestRunWorkStageCountByStageAndOutcome verifies funnel counting. |
| [`TestRunWorkStageCrossRunScoping`](../src/database/works_integration_test.go#L1024) | test | 1024-1112 | `func TestRunWorkStageCrossRunScoping(t *testing.T)` | TestRunWorkStageCrossRunScoping verifies that two runs can record different stage outcomes for the same work without interfering with each other, and that revisions from different runs are independently stored. |
| [`TestRunWorkStageCrossWorkScoping`](../src/database/works_integration_test.go#L1116) | test | 1116-1168 | `func TestRunWorkStageCrossWorkScoping(t *testing.T)` | TestRunWorkStageCrossWorkScoping verifies that two different works in the same run can record different validation reasons without cross-contamination. |
| [`TestRunWorkStageInvalidStageName`](../src/database/works_integration_test.go#L1171) | test | 1171-1182 | `func TestRunWorkStageInvalidStageName(t *testing.T)` | TestRunWorkStageInvalidStageName verifies SetOutcome rejects unknown stages. |
| [`TestRunWorkStageInvalidOutcome`](../src/database/works_integration_test.go#L1185) | test | 1185-1196 | `func TestRunWorkStageInvalidOutcome(t *testing.T)` | TestRunWorkStageInvalidOutcome verifies SetOutcome rejects unknown outcomes. |
| [`TestRunWorkStageEmptyStageName`](../src/database/works_integration_test.go#L1199) | test | 1199-1210 | `func TestRunWorkStageEmptyStageName(t *testing.T)` | TestRunWorkStageEmptyStageName verifies SetOutcome rejects empty stage name. |
| [`TestRunWorkStageEmptyOutcome`](../src/database/works_integration_test.go#L1213) | test | 1213-1224 | `func TestRunWorkStageEmptyOutcome(t *testing.T)` | TestRunWorkStageEmptyOutcome verifies SetOutcome rejects empty outcome. |
| [`TestRunWorkStageReplacePreservesIdentity`](../src/database/works_integration_test.go#L1229) | test | 1229-1299 | `func TestRunWorkStageReplacePreservesIdentity(t *testing.T)` | TestRunWorkStageReplacePreservesIdentity verifies that ON CONFLICT DO UPDATE preserves the original created_at and row ID across progressive updates, and that updated_at is set to a later timestamp. |
| [`TestRunWorkStageInvalidCombination`](../src/database/works_integration_test.go#L1303) | test | 1303-1332 | `func TestRunWorkStageInvalidCombination(t *testing.T)` | TestRunWorkStageInvalidCombination verifies that impossible stage/outcome pairs are rejected (e.g. parse/valid, validate/enriched). |
| [`TestRunWorkStageValidCombinations`](../src/database/works_integration_test.go#L1336) | test | 1336-1367 | `func TestRunWorkStageValidCombinations(t *testing.T)` | TestRunWorkStageValidCombinations verifies that all valid stage/outcome pairs are accepted. There are 17 pairs: 3 + 4 + 4 + 3 + 3. |
| [`TestRunWorkStageUpdatedAtProgression`](../src/database/works_integration_test.go#L1371) | test | 1371-1428 | `func TestRunWorkStageUpdatedAtProgression(t *testing.T)` | TestRunWorkStageUpdatedAtProgression verifies that updated_at advances when SetOutcome progressively replaces an outcome. |

### [`src/database/works_unit_test.go`](../src/database/works_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestRevisionPayloadHashDeterminism`](../src/database/works_unit_test.go#L11) | test | 11-71 | `func TestRevisionPayloadHashDeterminism(t *testing.T)` | TestRevisionPayloadHashDeterminism verifies revision payload hash determinism. |

### [`src/database/workspace.go`](../src/database/workspace.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Search`](../src/database/workspace.go#L12) | struct | 12-16 | ``type Search struct { ID int64 `json:"id"` SearchID string `json:"search_id"` CreatedAt string `json:"created_at"` }`` | Search represents a single research question (stable identity). |
| [`SearchRevision`](../src/database/workspace.go#L21) | struct | 21-29 | ``type SearchRevision struct { ID int64 `json:"id"` SearchID int64 `json:"search_id"` RevisionLabel string `json:"revision_label"` ConfigArtifactHash string `json:"config_artifact_hash"` ResolvedManifestHash string `json:"resolved_manifest_hash"` CreatedAt string `json:"created_at"` UpdatedAt string `json:"updated_at,omitempty"` }`` | SearchRevision is a researcher-managed grouping for one version of a search's query intent. Its hashes describe the latest observed declaration; immutable historical configuration belongs to execution plans and attempts. |
| [`ExecutionPlan`](../src/database/workspace.go#L32) | struct | 32-40 | ``type ExecutionPlan struct { ID int64 `json:"id"` SearchRevisionID int64 `json:"search_revision_id"` ExecutionFingerprint string `json:"execution_fingerprint"` ResolvedManifestHash string `json:"resolved_manifest_hash"` InputManifestHash string `json:"input_manifest_hash"` EnrichmentEnabled bool `json:"enrichment_enabled"` CreatedAt string `json:"created_at"` }`` | ExecutionPlan represents a unique fingerprint per search revision and input policy. |
| [`SearchRepository`](../src/database/workspace.go#L43) | struct | 43-45 | `type SearchRepository struct { db *Database }` | SearchRepository provides CRUD for the searches table. |
| [`(*SearchRepository).Create`](../src/database/workspace.go#L49) | method | 49-83 | `func (*SearchRepository).Create(searchID string) (int64, error)` | Create inserts a new search by search_id. Returns the search ID. If the search_id already exists, returns the existing ID. |
| [`(*SearchRepository).GetByID`](../src/database/workspace.go#L86) | method | 86-101 | `func (*SearchRepository).GetByID(id int64) (*Search, error)` | GetByID returns a search by its primary key, or nil if not found. |
| [`(*SearchRepository).GetBySearchID`](../src/database/workspace.go#L104) | method | 104-119 | `func (*SearchRepository).GetBySearchID(searchID string) (*Search, error)` | GetBySearchID returns a search by its string identifier, or nil if not found. |
| [`(*SearchRepository).List`](../src/database/workspace.go#L122) | method | 122-145 | `func (*SearchRepository).List() ([]*Search, error)` | List returns all searches ordered by ID. |
| [`SearchRevisionRepository`](../src/database/workspace.go#L148) | struct | 148-150 | `type SearchRevisionRepository struct { db *Database }` | SearchRevisionRepository provides CRUD for the search_revisions table. |
| [`(*SearchRevisionRepository).Create`](../src/database/workspace.go#L160) | method | 160-230 | `func (*SearchRevisionRepository).Create(searchID int64, revisionLabel string, configArtifactHash, resolvedManifestHash string) (int64, bool, error)` | Create inserts a new search revision. Returns the revision ID and whether its latest-declaration hashes were updated (false on first insert or identical hashes). If the same (search_id, revision_label) already exists with identical config and manifest hashes, returns the existing ID with updated=false. If the hashes differ, the existing row is updated with the new hashes and updated_at, and updated=true is returned. This allows the same revision label to track the latest configuration for a search. |
| [`(*SearchRevisionRepository).GetByID`](../src/database/workspace.go#L233) | method | 233-252 | `func (*SearchRevisionRepository).GetByID(id int64) (*SearchRevision, error)` | GetByID returns a search revision by its primary key, or nil if not found. |
| [`(*SearchRevisionRepository).GetBySearchAndRevision`](../src/database/workspace.go#L255) | method | 255-275 | `func (*SearchRevisionRepository).GetBySearchAndRevision(searchID int64, revisionLabel string) (*SearchRevision, error)` | GetBySearchAndRevision returns a revision for a given search and label, or nil if not found. |
| [`(*SearchRevisionRepository).ListBySearch`](../src/database/workspace.go#L278) | method | 278-308 | `func (*SearchRevisionRepository).ListBySearch(searchID int64) ([]*SearchRevision, error)` | ListBySearch returns all revisions for a search, ordered by ID. |
| [`ExecutionPlanRepository`](../src/database/workspace.go#L311) | struct | 311-313 | `type ExecutionPlanRepository struct { db *Database }` | ExecutionPlanRepository provides CRUD for the execution_plans table. |
| [`(*ExecutionPlanRepository).Create`](../src/database/workspace.go#L320) | method | 320-322 | `func (*ExecutionPlanRepository).Create(searchRevisionID int64, fingerprint, manifestHash string) (int64, error)` | Create inserts a new execution plan. Returns the plan ID. If an identical plan (same search_revision_id + execution_fingerprint) already exists with the same resolved_manifest_hash, returns the existing ID. If the manifest hash differs, returns an error: the fingerprint is reserved for a different resolved configuration and cannot be reused. |
| [`(*ExecutionPlanRepository).CreateWithInputManifest`](../src/database/workspace.go#L326) | method | 326-331 | `func (*ExecutionPlanRepository).CreateWithInputManifest(searchRevisionID int64, fingerprint, manifestHash, inputManifestHash string, enrichmentEnabled bool) (int64, error)` | CreateWithInputManifest creates a plan linked to the frozen input manifest used to calculate its execution fingerprint. |
| [`(*ExecutionPlanRepository).createWithPolicy`](../src/database/workspace.go#L334) | method | 334-388 | `func (*ExecutionPlanRepository).createWithPolicy(searchRevisionID int64, fingerprint, manifestHash, inputManifestHash string, enrichmentEnabled bool) (int64, error)` | createWithPolicy inserts or reuses an execution plan with the supplied manifest and enrichment policy hashes. |
| [`(*ExecutionPlanRepository).GetByID`](../src/database/workspace.go#L391) | method | 391-408 | `func (*ExecutionPlanRepository).GetByID(id int64) (*ExecutionPlan, error)` | GetByID returns an execution plan by its primary key, or nil if not found. |
| [`(*ExecutionPlanRepository).GetByFingerprint`](../src/database/workspace.go#L411) | method | 411-429 | `func (*ExecutionPlanRepository).GetByFingerprint(searchRevisionID int64, fingerprint string) (*ExecutionPlan, error)` | GetByFingerprint returns an execution plan matching the given search revision and fingerprint, or nil if not found. |
| [`(*ExecutionPlanRepository).ListBySearchRevision`](../src/database/workspace.go#L432) | method | 432-460 | `func (*ExecutionPlanRepository).ListBySearchRevision(searchRevisionID int64) ([]*ExecutionPlan, error)` | ListBySearchRevision returns all execution plans for a given search revision, ordered by ID. |

### [`src/database/workspace_integration_test.go`](../src/database/workspace_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestSearchCreateAndGet`](../src/database/workspace_integration_test.go#L11) | test | 11-44 | `func TestSearchCreateAndGet(t *testing.T)` | TestSearchCreateAndGet verifies search create and get. |
| [`TestSearchCreateDuplicateReturnsSameID`](../src/database/workspace_integration_test.go#L47) | test | 47-64 | `func TestSearchCreateDuplicateReturnsSameID(t *testing.T)` | TestSearchCreateDuplicateReturnsSameID verifies search create duplicate returns same id. |
| [`TestSearchCreateDistinctIDs`](../src/database/workspace_integration_test.go#L67) | test | 67-77 | `func TestSearchCreateDistinctIDs(t *testing.T)` | TestSearchCreateDistinctIDs verifies search creation returns distinct IDs. |
| [`TestSearchList`](../src/database/workspace_integration_test.go#L80) | test | 80-95 | `func TestSearchList(t *testing.T)` | TestSearchList verifies search list. |
| [`TestSearchRevisionCreateAndLookup`](../src/database/workspace_integration_test.go#L98) | test | 98-139 | `func TestSearchRevisionCreateAndLookup(t *testing.T)` | TestSearchRevisionCreateAndLookup verifies search revision create and lookup. |
| [`TestSearchRevisionDuplicateSameHashReturnsSameID`](../src/database/workspace_integration_test.go#L142) | test | 142-161 | `func TestSearchRevisionDuplicateSameHashReturnsSameID(t *testing.T)` | TestSearchRevisionDuplicateSameHashReturnsSameID verifies search revision duplicate same hash returns same id. |
| [`TestSearchRevisionDuplicateDifferentHashUpdated`](../src/database/workspace_integration_test.go#L164) | test | 164-200 | `func TestSearchRevisionDuplicateDifferentHashUpdated(t *testing.T)` | TestSearchRevisionDuplicateDifferentHashUpdated verifies search revision duplicate different hash updated. |
| [`TestSearchRevisionDistinctLabels`](../src/database/workspace_integration_test.go#L203) | test | 203-223 | `func TestSearchRevisionDistinctLabels(t *testing.T)` | TestSearchRevisionDistinctLabels verifies search revision distinct labels. |

### [`src/e2e_test.go`](../src/e2e_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`e2eMode`](../src/e2e_test.go#L33) | struct | 33-44 | `type e2eMode struct { name string enrichment bool live bool providerBaseURL string sources []e2eSource expectedParsed int expectedUnique int expectedValid int expectedDiscarded int expectedTitles []string }` | e2eMode describes one pipeline variant and its expected persisted result. |
| [`e2eSource`](../src/e2e_test.go#L47) | struct | 47-52 | `type e2eSource struct { name string filename string fileType string count int }` | e2eSource describes one tracked input used by a generated workspace configuration. |
| [`e2eResult`](../src/e2e_test.go#L55) | struct | 55-60 | `type e2eResult struct { dbPath string runID int64 workIDs map[string]int64 titles []string }` | e2eResult identifies the generated database and rows needed by API assertions. |
| [`providerMock`](../src/e2e_test.go#L63) | struct | 63-66 | `type providerMock struct { mu sync.Mutex requests []string }` | providerMock records requests served by the local deterministic provider. |
| [`TestE2EDeterministic`](../src/e2e_test.go#L69) | test | 69-82 | `func TestE2EDeterministic(t *testing.T)` | TestE2EDeterministic verifies the offline pipeline, database, API, and PDF inventory flow. |
| [`TestE2EMocked`](../src/e2e_test.go#L85) | test | 85-102 | `func TestE2EMocked(t *testing.T)` | TestE2EMocked verifies enrichment through loopback providers and cross-layer evidence. |
| [`newE2EProviderServer`](../src/e2e_test.go#L105) | function | 105-115 | `func newE2EProviderServer(t *testing.T, handler http.Handler) *httptest.Server` | newE2EProviderServer starts the deterministic provider on an explicit IPv4 loopback listener. |
| [`TestE2ELive`](../src/e2e_test.go#L118) | test | 118-131 | `func TestE2ELive(t *testing.T)` | TestE2ELive verifies the explicitly enabled real-provider path with structural assertions. |
| [`runE2EVariant`](../src/e2e_test.go#L134) | function | 134-149 | `func runE2EVariant(t *testing.T, root string, mode e2eMode, environment []string) e2eResult` | runE2EVariant generates configuration, invokes the supported binary, and validates its databases. |
| [`e2eRepositoryRoot`](../src/e2e_test.go#L152) | function | 152-165 | `func e2eRepositoryRoot(t *testing.T) string` | e2eRepositoryRoot resolves the repository root from the main package working directory. |
| [`prepareE2EOutput`](../src/e2e_test.go#L168) | function | 168-186 | `func prepareE2EOutput(t *testing.T, root, variant string) string` | prepareE2EOutput recreates one known target-owned variant directory under build/e2e. |
| [`writeE2EConfig`](../src/e2e_test.go#L189) | function | 189-247 | `func writeE2EConfig(t *testing.T, root, outputDir string, mode e2eMode) string` | writeE2EConfig writes a typed workspace configuration and its relative include. |
| [`e2eProviderConfig`](../src/e2e_test.go#L250) | function | 250-287 | `func e2eProviderConfig(mode e2eMode) string` | e2eProviderConfig returns concrete local or live provider declarations. |
| [`validateE2EConfig`](../src/e2e_test.go#L290) | function | 290-297 | `func validateE2EConfig(t *testing.T, root, configPath string)` | validateE2EConfig evaluates the generated configuration through the maintained tool. |
| [`e2eOfflineEnvironment`](../src/e2e_test.go#L300) | function | 300-321 | `func e2eOfflineEnvironment(providerURL string) []string` | e2eOfflineEnvironment blocks accidental non-loopback provider traffic from the pipeline subprocess. |
| [`(*providerMock).ServeHTTP`](../src/e2e_test.go#L324) | method | 324-351 | `func (*providerMock).ServeHTTP(w http.ResponseWriter, r *http.Request)` | ServeHTTP returns deterministic provider envelopes for every supported enrichment path. |
| [`(*providerMock).assertRequests`](../src/e2e_test.go#L354) | method | 354-371 | `func (*providerMock).assertRequests(t *testing.T)` | assertRequests verifies all expected provider families were exercised through loopback. |
| [`assertE2EDatabases`](../src/e2e_test.go#L374) | function | 374-449 | `func assertE2EDatabases(t *testing.T, root, dbPath string, mode e2eMode) e2eResult` | assertE2EDatabases validates persisted pipeline, audit, cache, and PDF evidence. |
| [`e2eRawCount`](../src/e2e_test.go#L452) | function | 452-458 | `func e2eRawCount(sources []e2eSource) int` | e2eRawCount returns the total declared fixture record count. |
| [`assertE2ECount`](../src/e2e_test.go#L461) | function | 461-474 | `func assertE2ECount(t *testing.T, db *database.Database, table, where string, argument any, want int)` | assertE2ECount checks an exact table row count using one optional query argument. |
| [`assertE2EAtLeast`](../src/e2e_test.go#L477) | function | 477-490 | `func assertE2EAtLeast(t *testing.T, db *database.Database, table, where string, argument any, minimum int)` | assertE2EAtLeast checks a minimum table row count using one optional query argument. |
| [`assertE2EPDFCount`](../src/e2e_test.go#L493) | function | 493-499 | `func assertE2EPDFCount(t *testing.T, store *pdfstore.Store, table, where string, want int)` | assertE2EPDFCount checks an exact row count in the companion PDF store. |
| [`e2ENormalizedWorks`](../src/e2e_test.go#L502) | function | 502-525 | `func e2ENormalizedWorks(t *testing.T, db *database.Database, runID int64) ([]string, map[string]int64)` | e2ENormalizedWorks returns sorted normalized titles and their stable work IDs. |
| [`assertE2EAuditOrder`](../src/e2e_test.go#L528) | function | 528-560 | `func assertE2EAuditOrder(t *testing.T, db *database.Database, runID int64)` | assertE2EAuditOrder verifies the pipeline's cross-database terminal event order. |
| [`assertE2EAPI`](../src/e2e_test.go#L563) | function | 563-607 | `func assertE2EAPI(t *testing.T, result e2eResult)` | assertE2EAPI compares read-only HTTP responses with the database assertions. |
| [`requestE2EJSON`](../src/e2e_test.go#L610) | function | 610-623 | `func requestE2EJSON(t *testing.T, handler http.Handler, path string) map[string]any` | requestE2EJSON invokes one read-only viewer route and decodes its object response. |
| [`nestedE2EID`](../src/e2e_test.go#L626) | function | 626-649 | `func nestedE2EID(t *testing.T, payload map[string]any, collection, nested string) int64` | nestedE2EID reads the first nested object ID from a viewer collection response. |
| [`mustE2EJSON`](../src/e2e_test.go#L652) | function | 652-659 | `func mustE2EJSON(t *testing.T, value any) string` | mustE2EJSON serializes a decoded payload for compact membership assertions. |
| [`equalE2EStrings`](../src/e2e_test.go#L662) | function | 662-668 | `func equalE2EStrings(left, right []string) bool` | equalE2EStrings compares two string sets after sorting copies. |

### [`src/enrich/client.go`](../src/enrich/client.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Client`](../src/enrich/client.go#L17) | struct | 17-21 | `type Client struct { cfg SourceConfig ticker *time.Ticker client *http.Client }` | Client fetches URLs concurrently with configurable rate limiting and exponential backoff on 429 responses. |
| [`NewClient`](../src/enrich/client.go#L29) | function | 29-42 | `func NewClient(cfg SourceConfig) *Client` | NewClient creates a Client for the given source config. |
| [`(*Client).Close`](../src/enrich/client.go#L45) | method | 45-47 | `func (*Client).Close()` | Close stops the rate-limit ticker. |
| [`FetchResult`](../src/enrich/client.go#L50) | struct | 50-54 | `type FetchResult struct { Body []byte StatusCode int Err error }` | FetchResult carries the response body (or error) for one URL fetch. |
| [`(*Client).FetchAll`](../src/enrich/client.go#L59) | method | 59-176 | `func (*Client).FetchAll(ctx context.Context, urls []string) map[string]*FetchResult` | FetchAll fetches all URLs concurrently using a goroutine pool. It respects rate limiting across all workers and applies exponential backoff on 429. Returns a map of original URL -> FetchResult in the same order as input. |
| [`(*Client).Fetch`](../src/enrich/client.go#L181) | method | 181-183 | `func (*Client).Fetch(ctx context.Context, url string) *FetchResult` | Fetch retrieves one URL through the same rate-limited path as FetchAll. Workspace cache policy uses it so individual cache misses do not bypass the provider's configured request limit. |
| [`(*Client).fetchOne`](../src/enrich/client.go#L186) | method | 186-279 | `func (*Client).fetchOne(ctx context.Context, url string) *FetchResult` | fetchOne performs a single HTTP GET with retries and exponential backoff. |
| [`truncateStr`](../src/enrich/client.go#L282) | function | 282-288 | `func truncateStr(value string, limit int) string` | truncateStr truncates str to the requested limit. |
| [`(*Client).sourceName`](../src/enrich/client.go#L291) | method | 291-296 | `func (*Client).sourceName() string` | sourceName returns the configured provider name used in logging and evidence. |
| [`logFetchProgress`](../src/enrich/client.go#L299) | function | 299-311 | `func logFetchProgress(message, source string, started time.Time, completed, total, succeeded, notFound, failed int)` | logFetchProgress emits structured provider progress and elapsed-time fields. |

### [`src/enrich/client_functional_test.go`](../src/enrich/client_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestClientRetriesOn429`](../src/enrich/client_functional_test.go#L14) | test | 14-41 | `func TestClientRetriesOn429(t *testing.T)` | TestClientRetriesOn429 verifies client retries on429. |
| [`TestClientReturns404WithoutError`](../src/enrich/client_functional_test.go#L44) | test | 44-62 | `func TestClientReturns404WithoutError(t *testing.T)` | TestClientReturns404WithoutError verifies client returns404 without error. |
| [`TestClientReturnsErrorOnServerError`](../src/enrich/client_functional_test.go#L65) | test | 65-83 | `func TestClientReturnsErrorOnServerError(t *testing.T)` | TestClientReturnsErrorOnServerError verifies client returns error on server error. |
| [`TestClientExhaustsRetries`](../src/enrich/client_functional_test.go#L86) | test | 86-109 | `func TestClientExhaustsRetries(t *testing.T)` | TestClientExhaustsRetries verifies client exhausts retries. |
| [`TestClientFetchUsesPublicRateLimitedPath`](../src/enrich/client_functional_test.go#L112) | test | 112-135 | `func TestClientFetchUsesPublicRateLimitedPath(t *testing.T)` | TestClientFetchUsesPublicRateLimitedPath verifies client fetch uses public rate limited path. |

### [`src/enrich/client_unit_test.go`](../src/enrich/client_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestClientContextCancellation`](../src/enrich/client_unit_test.go#L13) | test | 13-33 | `func TestClientContextCancellation(t *testing.T)` | TestClientContextCancellation verifies client context cancellation. |

### [`src/enrich/crossref.go`](../src/enrich/crossref.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`extractCrossrefEntry`](../src/enrich/crossref.go#L14) | function | 14-25 | `func extractCrossrefEntry(body []byte) map[string]any` | extractCrossrefEntry parses the Crossref API response and returns a generic map of extracted fields. |
| [`DecodeCrossrefResponse`](../src/enrich/crossref.go#L30) | function | 30-36 | `func DecodeCrossrefResponse(body []byte, doi string) *ArticleEnrichment` | DecodeCrossrefResponse converts a raw Crossref work response into the workspace enrichment representation. The workspace owns cache policy and persistence; this package remains responsible only for decoding. |
| [`crossrefEntryToArticle`](../src/enrich/crossref.go#L40) | function | 40-66 | `func crossrefEntryToArticle(entry map[string]any, doi string) *ArticleEnrichment` | crossrefEntryToArticle converts a Crossref API message map to an ArticleEnrichment. |
| [`extractCrossrefAuthors`](../src/enrich/crossref.go#L69) | function | 69-112 | `func extractCrossrefAuthors(authors []any) []EnrichedAuthor` | extractCrossrefAuthors parses the Crossref author array. |
| [`extractCrossrefReferences`](../src/enrich/crossref.go#L115) | function | 115-154 | `func extractCrossrefReferences(refs []any) []EnrichedReference` | extractCrossrefReferences parses the Crossref reference array. |

### [`src/enrich/crossref_unit_test.go`](../src/enrich/crossref_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestDecodeCrossrefResponse`](../src/enrich/crossref_unit_test.go#L9) | test | 9-26 | `func TestDecodeCrossrefResponse(t *testing.T)` | TestDecodeCrossrefResponse verifies decode crossref response. |

### [`src/enrich/enrich.go`](../src/enrich/enrich.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`SourceConfig`](../src/enrich/enrich.go#L12) | struct | 12-25 | `type SourceConfig struct { Name string BaseURL string UserAgent string ContactEmail string RatePerSecond int Concurrency int TimeoutSecs int MaxRetries int Fields []string ExtraURLs map[string]string BatchSize int FillMissingOnly bool }` | SourceConfig describes an enrichment source as declared in a SOMETHING configuration. |
| [`Config`](../src/enrich/enrich.go#L28) | struct | 28-30 | `type Config struct { Sources map[string]SourceConfig }` | Config holds all enrichment sources keyed by their configuration name. |
| [`EnrichedAuthor`](../src/enrich/enrich.go#L33) | struct | 33-46 | `type EnrichedAuthor struct { ORCID string FirstName string LastName string CitationName string Affiliation string DisplayName string WorksCount int CitedByCnt int HIndex int I10Index int Institution string Source string }` | EnrichedAuthor is the per-author result of enrichment. |
| [`EnrichedReference`](../src/enrich/enrich.go#L49) | struct | 49-55 | ``type EnrichedReference struct { DOI string `json:"doi"` Title string `json:"title"` Author string `json:"author"` Year int `json:"year"` Source string `json:"source"` }`` | EnrichedReference is enriched metadata for one cited reference. |
| [`ArticleEnrichment`](../src/enrich/enrich.go#L58) | struct | 58-67 | `type ArticleEnrichment struct { DOI string Title string Abstract string Authors []EnrichedAuthor Publisher string References []EnrichedReference CitationCount int ReferenceCount int }` | ArticleEnrichment holds gathered data for one article from one source. |
| [`GatherResult`](../src/enrich/enrich.go#L70) | struct | 70-77 | `type GatherResult struct { Source string FillMissingOnly bool Articles map[string]*ArticleEnrichment Authors map[string]*EnrichedAuthor AuthorMatches map[string]string DOINotFound []string }` | GatherResult is the pure output of one gather step. |

### [`src/enrich/openalex.go`](../src/enrich/openalex.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`DecodeOpenAlexResponse`](../src/enrich/openalex.go#L16) | function | 16-22 | `func DecodeOpenAlexResponse(body []byte, doi string) (*ArticleEnrichment, []string)` | DecodeOpenAlexResponse converts one raw work response. Reference metadata is populated separately by DecodeOpenAlexReferenceResponse so the workspace can cache work and reference lookups independently. |
| [`DecodeOpenAlexReferenceResponse`](../src/enrich/openalex.go#L26) | function | 26-50 | `func DecodeOpenAlexReferenceResponse(body []byte) map[string]EnrichedReference` | DecodeOpenAlexReferenceResponse converts an OpenAlex batch response to references keyed by OpenAlex work ID (for example, W123). |
| [`extractOpenAlexEntry`](../src/enrich/openalex.go#L53) | function | 53-59 | `func extractOpenAlexEntry(body []byte) map[string]any` | extractOpenAlexEntry decodes an OpenAlex work payload, returning nil for malformed JSON. |
| [`openalexEntryToArticle`](../src/enrich/openalex.go#L62) | function | 62-84 | `func openalexEntryToArticle(entry map[string]any, doi string) *ArticleEnrichment` | openalexEntryToArticle converts an OpenAlex work object to article enrichment fields. |
| [`reconstructAbstract`](../src/enrich/openalex.go#L87) | function | 87-113 | `func reconstructAbstract(index map[string]any) string` | reconstructAbstract rebuilds abstract text from OpenAlex's word-position index. |
| [`extractOpenAlexPublisher`](../src/enrich/openalex.go#L116) | function | 116-131 | `func extractOpenAlexPublisher(entry map[string]any) string` | extractOpenAlexPublisher returns the direct or primary-source publisher name from a work object. |
| [`extractOpenAlexAuthors`](../src/enrich/openalex.go#L134) | function | 134-169 | `func extractOpenAlexAuthors(authorships []any) []EnrichedAuthor` | extractOpenAlexAuthors converts usable OpenAlex authorships to enriched authors in source order. |
| [`openAlexReferenceIDs`](../src/enrich/openalex.go#L172) | function | 172-188 | `func openAlexReferenceIDs(entry map[string]any) []string` | openAlexReferenceIDs returns unique OpenAlex work identifiers from referenced-work URLs. |
| [`extractOpenAlexReferences`](../src/enrich/openalex.go#L191) | function | 191-203 | `func extractOpenAlexReferences(references []any) []EnrichedReference` | extractOpenAlexReferences converts referenced-work DOI URLs to enriched references. |
| [`normalizeOpenAlexDOI`](../src/enrich/openalex.go#L206) | function | 206-212 | `func normalizeOpenAlexDOI(value string) string` | normalizeOpenAlexDOI returns a lowercase DOI extracted from a URL or prefixed value. |
| [`extractOpenAlexID`](../src/enrich/openalex.go#L220) | function | 220-229 | `func extractOpenAlexID(url string) string` | extractOpenAlexID returns a work identifier from an OpenAlex URL or bare identifier. |
| [`extractDOIFromURL`](../src/enrich/openalex.go#L232) | function | 232-238 | `func extractDOIFromURL(url string) string` | extractDOIFromURL returns a lowercase DOI embedded in a DOI URL. |

### [`src/enrich/openalex_unit_test.go`](../src/enrich/openalex_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestDecodeOpenAlexResponses`](../src/enrich/openalex_unit_test.go#L9) | test | 9-26 | `func TestDecodeOpenAlexResponses(t *testing.T)` | TestDecodeOpenAlexResponses verifies decode open alex responses. |
| [`TestDecodeOpenAlexAuthorResponse`](../src/enrich/openalex_unit_test.go#L29) | test | 29-119 | `func TestDecodeOpenAlexAuthorResponse(t *testing.T)` | TestDecodeOpenAlexAuthorResponse verifies decode open alex author response. |

### [`src/enrich/orcid.go`](../src/enrich/orcid.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`DecodeOpenAlexAuthorResponse`](../src/enrich/orcid.go#L15) | function | 15-22 | `func DecodeOpenAlexAuthorResponse(body []byte, orcid string) *EnrichedAuthor` | DecodeOpenAlexAuthorResponse converts an OpenAlex author response for an exact ORCID lookup. |
| [`decodeOpenAlexAuthor`](../src/enrich/orcid.go#L25) | function | 25-31 | `func decodeOpenAlexAuthor(body []byte) map[string]any` | decodeOpenAlexAuthor decodes open alex author from the supplied payload. |
| [`DecodeORCIDRecordResponse`](../src/enrich/orcid.go#L35) | function | 35-42 | `func DecodeORCIDRecordResponse(body []byte, orcid string) *EnrichedAuthor` | DecodeORCIDRecordResponse converts an ORCID person record for an exact ORCID lookup. |
| [`decodeORCIDRecord`](../src/enrich/orcid.go#L45) | function | 45-81 | `func decodeORCIDRecord(body []byte) map[string]any` | decodeORCIDRecord decodes orcid record from the supplied payload. |
| [`ORCIDNameSearchURLs`](../src/enrich/orcid.go#L86) | function | 86-114 | `func ORCIDNameSearchURLs(source SourceConfig, name string) []string` | ORCIDNameSearchURLs returns the ordered exact-name search requests used by the workspace pipeline. Candidates from every successful query remain evidence for review and are never treated as identity proof. |
| [`ORCIDNameSearchCandidate`](../src/enrich/orcid.go#L118) | struct | 118-120 | `type ORCIDNameSearchCandidate struct { ORCID string }` | ORCIDNameSearchCandidate is provider evidence only, never proof that the citation author is the person identified by the returned ORCID. |
| [`DecodeORCIDNameSearchCandidates`](../src/enrich/orcid.go#L124) | function | 124-142 | `func DecodeORCIDNameSearchCandidates(body []byte) []ORCIDNameSearchCandidate` | DecodeORCIDNameSearchCandidates returns every usable ORCID in provider order. Workspace callers retain this ambiguity for human review. |
| [`orcidEntryToAuthor`](../src/enrich/orcid.go#L145) | function | 145-193 | `func orcidEntryToAuthor(entry map[string]any, orcid string) *EnrichedAuthor` | orcidEntryToAuthor converts a validated ORCID record to author enrichment fields. |
| [`queryEscape`](../src/enrich/orcid.go#L196) | function | 196-202 | `func queryEscape(value string) string` | queryEscape removes embedded quotes and quotes multi-word ORCID search values. |

### [`src/enrich/orcid_unit_test.go`](../src/enrich/orcid_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestDecodeORCIDResponsesAndSearchCandidates`](../src/enrich/orcid_unit_test.go#L12) | test | 12-28 | `func TestDecodeORCIDResponsesAndSearchCandidates(t *testing.T)` | TestDecodeORCIDResponsesAndSearchCandidates verifies decode orcid responses and search candidates. |

### [`src/enrich/payload_validation.go`](../src/enrich/payload_validation.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`ValidateProviderPayload`](../src/enrich/payload_validation.go#L15) | function | 15-51 | `func ValidateProviderPayload(provider, namespace string, body []byte) error` | ValidateProviderPayload verifies that a successful provider response has the envelope expected by the workspace decoder. It deliberately validates only transport envelopes, not optional bibliographic fields: a valid provider record may legitimately omit a requested field. |
| [`requireObject`](../src/enrich/payload_validation.go#L54) | function | 54-64 | `func requireObject(object map[string]json.RawMessage, key string) error` | requireObject requires a valid object value. |
| [`requireArray`](../src/enrich/payload_validation.go#L67) | function | 67-77 | `func requireArray(object map[string]json.RawMessage, key string) error` | requireArray requires a valid array value. |
| [`requireArrayOrNull`](../src/enrich/payload_validation.go#L80) | function | 80-93 | `func requireArrayOrNull(object map[string]json.RawMessage, key string) error` | requireArrayOrNull requires a valid array or null value. |
| [`requireString`](../src/enrich/payload_validation.go#L96) | function | 96-106 | `func requireString(object map[string]json.RawMessage, key string) error` | requireString requires a valid string value. |

### [`src/enrich/payload_validation_unit_test.go`](../src/enrich/payload_validation_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestValidateProviderPayload`](../src/enrich/payload_validation_unit_test.go#L9) | test | 9-71 | `func TestValidateProviderPayload(t *testing.T)` | TestValidateProviderPayload verifies validate provider payload. |
| [`errorContains`](../src/enrich/payload_validation_unit_test.go#L74) | function | 74-76 | `func errorContains(err error, substr string) bool` | errorContains supports the package test suite's error contains setup or assertions. |

### [`src/logging/logging.go`](../src/logging/logging.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`init`](../src/logging/logging.go#L20) | function | 20-23 | `func init()` | init installs the package's process-wide runtime configuration. |
| [`Logger`](../src/logging/logging.go#L26) | function | 26-31 | `func Logger(component string) *slog.Logger` | Logger returns a logger backed by the shared process handler. |

### [`src/logging/logging_unit_test.go`](../src/logging/logging_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestLoggerCreatedAtInitUsesMinLevel`](../src/logging/logging_unit_test.go#L15) | test | 15-26 | `func TestLoggerCreatedAtInitUsesMinLevel(t *testing.T)` | TestLoggerCreatedAtInitUsesMinLevel verifies logger created at init uses min level. |
| [`TestLoggerReturnsSharedHandler`](../src/logging/logging_unit_test.go#L29) | test | 29-40 | `func TestLoggerReturnsSharedHandler(t *testing.T)` | TestLoggerReturnsSharedHandler verifies logger returns shared handler. |
| [`TestLoggerEmptyComponent`](../src/logging/logging_unit_test.go#L43) | test | 43-48 | `func TestLoggerEmptyComponent(t *testing.T)` | TestLoggerEmptyComponent verifies logger empty component. |

### [`src/main.go`](../src/main.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`version`](../src/main.go#L28) | function | 28-34 | `func version() string` | version returns the semantic version string, appending "-development" for development builds so release and dev binaries are distinguishable. |
| [`usage`](../src/main.go#L37) | function | 37-88 | `func usage()` | usage writes the supported command syntax to standard error. |
| [`main`](../src/main.go#L91) | function | 91-111 | `func main()` | main dispatches the analysis command selected by process arguments and exits on command failure. |
| [`serveMain`](../src/main.go#L114) | function | 114-157 | `func serveMain()` | serveMain serves main. |
| [`frontendAssets`](../src/main.go#L160) | function | 160-172 | `func frontendAssets(dir string) (fs.FS, error)` | frontendAssets returns either explicit filesystem assets or the embedded production frontend. |
| [`runPipelineMain`](../src/main.go#L175) | function | 175-205 | `func runPipelineMain()` | runPipelineMain parses run flags, resolves workspace selections, and executes each pipeline workspace. |
| [`changeToRepositoryRoot`](../src/main.go#L208) | function | 208-214 | `func changeToRepositoryRoot()` | changeToRepositoryRoot moves one directory upward only when execution starts inside the module directory. |

### [`src/main_test.go`](../src/main_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestPipelineEndToEndWorkspace`](../src/main_test.go#L22) | test | 22-323 | `func TestPipelineEndToEndWorkspace(t *testing.T)` | TestPipelineEndToEndWorkspace verifies pipeline end to end workspace. |
| [`TestWorkspaceAttemptRetryAndSourceHash`](../src/main_test.go#L326) | test | 326-386 | `func TestWorkspaceAttemptRetryAndSourceHash(t *testing.T)` | TestWorkspaceAttemptRetryAndSourceHash verifies workspace attempt retry and source hash. |
| [`TestWorkspaceRevisionPlanAndAttemptGrouping`](../src/main_test.go#L389) | test | 389-473 | `func TestWorkspaceRevisionPlanAndAttemptGrouping(t *testing.T)` | TestWorkspaceRevisionPlanAndAttemptGrouping verifies workspace revision plan and attempt grouping. |
| [`TestWorkspacePipelineNormalizesOnlyValidArticles`](../src/main_test.go#L476) | test | 476-536 | `func TestWorkspacePipelineNormalizesOnlyValidArticles(t *testing.T)` | TestWorkspacePipelineNormalizesOnlyValidArticles verifies workspace pipeline normalizes only valid articles. |
| [`TestWorkspaceAttemptRejectsConcurrentPlan`](../src/main_test.go#L539) | test | 539-558 | `func TestWorkspaceAttemptRejectsConcurrentPlan(t *testing.T)` | TestWorkspaceAttemptRejectsConcurrentPlan verifies workspace attempt rejects concurrent plan. |
| [`TestWorkspaceAttemptRecordsDeclaredEnrichmentPolicy`](../src/main_test.go#L561) | test | 561-606 | `func TestWorkspaceAttemptRecordsDeclaredEnrichmentPolicy(t *testing.T)` | TestWorkspaceAttemptRecordsDeclaredEnrichmentPolicy verifies workspace attempt records declared enrichment policy. |
| [`TestWorkspacePipelineRecordsUnreadableSourcePreflightFailure`](../src/main_test.go#L609) | test | 609-689 | `func TestWorkspacePipelineRecordsUnreadableSourcePreflightFailure(t *testing.T)` | TestWorkspacePipelineRecordsUnreadableSourcePreflightFailure verifies workspace pipeline records unreadable source preflight failure. |
| [`TestWorkspacePipelineFailsForEmptyOrMalformedSource`](../src/main_test.go#L692) | test | 692-779 | `func TestWorkspacePipelineFailsForEmptyOrMalformedSource(t *testing.T)` | TestWorkspacePipelineFailsForEmptyOrMalformedSource verifies workspace pipeline fails for empty or malformed source. |
| [`TestWorkspacePipelineRecordsInformationalExpectedResultCounts`](../src/main_test.go#L782) | test | 782-833 | `func TestWorkspacePipelineRecordsInformationalExpectedResultCounts(t *testing.T)` | TestWorkspacePipelineRecordsInformationalExpectedResultCounts verifies workspace pipeline records informational expected result counts. |
| [`TestWorkspacePipelineRetainsSourceRecordsRejectedDuringCanonicalConversion`](../src/main_test.go#L836) | test | 836-871 | `func TestWorkspacePipelineRetainsSourceRecordsRejectedDuringCanonicalConversion(t *testing.T)` | TestWorkspacePipelineRetainsSourceRecordsRejectedDuringCanonicalConversion verifies workspace pipeline retains source records rejected during canonical conversion. |
| [`TestFrontendAssets`](../src/main_test.go#L874) | test | 874-903 | `func TestFrontendAssets(t *testing.T)` | TestFrontendAssets verifies frontend assets. |
| [`TestVersion`](../src/main_test.go#L906) | test | 906-914 | `func TestVersion(t *testing.T)` | TestVersion verifies the version command output format and current value. |
| [`chdirToRepositoryRoot`](../src/main_test.go#L917) | function | 917-932 | `func chdirToRepositoryRoot(t *testing.T)` | chdirToRepositoryRoot supports the package test suite's chdir to repository root setup or assertions. |
| [`testWorkspaceRun`](../src/main_test.go#L935) | function | 935-953 | `func testWorkspaceRun(sourcePath string) *workspace.Run` | testWorkspaceRun supports the package test suite's test workspace run setup or assertions. |

### [`src/manifest/helpers_test.go`](../src/manifest/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`baseResolvedManifest`](../src/manifest/helpers_test.go#L5) | function | 5-63 | `func baseResolvedManifest() *ResolvedManifest` | baseResolvedManifest supports the package test suite's base resolved manifest setup or assertions. |
| [`baseInputManifest`](../src/manifest/helpers_test.go#L66) | function | 66-83 | `func baseInputManifest(rm *ResolvedManifest) *InputManifest` | baseInputManifest supports the package test suite's base input manifest setup or assertions. |
| [`cloneMap`](../src/manifest/helpers_test.go#L86) | function | 86-92 | `func cloneMap[V any](m map[string]V) map[string]V` | cloneMap returns a shallow copy of a string-keyed map. |
| [`cloneStrings`](../src/manifest/helpers_test.go#L95) | function | 95-99 | `func cloneStrings(s []string) []string` | cloneStrings returns a copy of a string slice. |

### [`src/manifest/lifecycle.go`](../src/manifest/lifecycle.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`AttemptStatus`](../src/manifest/lifecycle.go#L14) | type | 14 | `type AttemptStatus string` | AttemptStatus represents the lifecycle state of a pipeline run attempt. |
| [`ValidAttemptStatuses`](../src/manifest/lifecycle.go#L26) | function | 26-28 | `func ValidAttemptStatuses() []AttemptStatus` | ValidAttemptStatuses returns all valid attempt status values. |
| [`ValidateAttemptStatus`](../src/manifest/lifecycle.go#L31) | function | 31-38 | `func ValidateAttemptStatus(s string) error` | ValidateAttemptStatus returns an error if s is not a valid attempt status. |
| [`StageOutcome`](../src/manifest/lifecycle.go#L41) | type | 41 | `type StageOutcome string` | StageOutcome represents the result of a single pipeline stage. |
| [`ValidStageOutcomes`](../src/manifest/lifecycle.go#L60) | function | 60-62 | `func ValidStageOutcomes() []StageOutcome` | ValidStageOutcomes returns all valid stage outcome values. |
| [`ValidateStageOutcome`](../src/manifest/lifecycle.go#L65) | function | 65-72 | `func ValidateStageOutcome(s string) error` | ValidateStageOutcome returns an error if s is not a valid stage outcome. |
| [`CacheOutcome`](../src/manifest/lifecycle.go#L78) | type | 78 | `type CacheOutcome string` | CacheOutcome represents the result of a cache lookup operation. It describes what the cache layer returned, not where the data was ultimately resolved. The resolution source (network, prior-run snapshot, etc.) is tracked separately as a fetch/resolution event. |
| [`ValidCacheOutcomes`](../src/manifest/lifecycle.go#L92) | function | 92-94 | `func ValidCacheOutcomes() []CacheOutcome` | ValidCacheOutcomes returns all valid cache outcome values. |
| [`ValidateCacheOutcome`](../src/manifest/lifecycle.go#L97) | function | 97-104 | `func ValidateCacheOutcome(s string) error` | ValidateCacheOutcome returns an error if s is not a valid cache outcome. |
| [`RunVisibility`](../src/manifest/lifecycle.go#L107) | type | 107 | `type RunVisibility string` | RunVisibility represents the visibility state of a pipeline run. |
| [`ValidRunVisibilities`](../src/manifest/lifecycle.go#L121) | function | 121-123 | `func ValidRunVisibilities() []RunVisibility` | ValidRunVisibilities returns all valid run visibility values. |
| [`ValidateRunVisibility`](../src/manifest/lifecycle.go#L126) | function | 126-133 | `func ValidateRunVisibility(s string) error` | ValidateRunVisibility returns an error if s is not a valid run visibility. |
| [`AuditAction`](../src/manifest/lifecycle.go#L136) | type | 136 | `type AuditAction string` | AuditAction identifies one kind of append-only audit event. |
| [`ValidAuditActions`](../src/manifest/lifecycle.go#L179) | function | 179-188 | `func ValidAuditActions() []AuditAction` | ValidAuditActions returns all valid audit action values. |
| [`ValidateAuditAction`](../src/manifest/lifecycle.go#L191) | function | 191-203 | `func ValidateAuditAction(s string) error` | ValidateAuditAction returns an error if s is not a valid audit action. |
| [`AuditEvent`](../src/manifest/lifecycle.go#L209) | struct | 209-244 | ``type AuditEvent struct { // OccurredAt is the ISO 8601 UTC timestamp of the event. OccurredAt string `json:"occurred_at"` // Actor identifies the system component or user that produced the event // (e.g. "pipeline", "viewer", "admin"). Actor string `json:"actor"` // PipelineRunID links this event to a specific pipeline run attempt. // Zero means the event is not associated with a specific run. PipelineRunID int64 `json:"pipeline_run_id,omitempty"` // EntityType identifies the kind of entity affected (e.g. "article", // "author", "plan", "run", "source"). EntityType string `json:"entity_type"` // EntityID is the identifier of the affected entity. EntityID string `json:"entity_id"` // Action is the audit action name. Action AuditAction `json:"action"` // BeforeJSON is the JSON representation of the entity state before the // action, if applicable. BeforeJSON string `json:"before_json,omitempty"` // AfterJSON is the JSON representation of the entity state after the // action, if applicable. AfterJSON string `json:"after_json,omitempty"` // MetadataJSON is additional event-specific context as JSON. MetadataJSON string `json:"metadata_json,omitempty"` // CorrelationID is an optional idempotency key for deduplication. CorrelationID string `json:"correlation_id,omitempty"` }`` | AuditEvent is a single append-only audit record. Every event records when it occurred, which actor produced it, which pipeline run it belongs to, the affected entity, the action performed, optional before/after state, and a correlation id for deduplication. |
| [`RetentionPolicy`](../src/manifest/lifecycle.go#L249) | struct | 249-253 | ``type RetentionPolicy struct { // TrashRetentionDays is the minimum number of days a trashed run is kept // before it becomes eligible for purge. Zero means indefinite retention. TrashRetentionDays int `json:"trash_retention_days"` }`` | RetentionPolicy describes how long trashed run data is retained before it becomes eligible for purge. Audit events are never deleted; RetentionPolicy does not govern audit records. |
| [`PurgePolicy`](../src/manifest/lifecycle.go#L257) | struct | 257-266 | ``type PurgePolicy struct { // RequireVerification, when true, requires that purging a run first // verifies no shared artifacts or cache entries are referenced by other // runs. RequireVerification bool `json:"require_verification"` // KeepTombstone, when true, retains a lightweight purge event or tombstone // record instead of removing all evidence of the purged run. KeepTombstone bool `json:"keep_tombstone"` }`` | PurgePolicy describes the authorization and safety checks required before data is permanently removed. |
| [`DefaultRetentionPolicy`](../src/manifest/lifecycle.go#L269) | function | 269-273 | `func DefaultRetentionPolicy() RetentionPolicy` | DefaultRetentionPolicy returns the default retention policy. |
| [`DefaultPurgePolicy`](../src/manifest/lifecycle.go#L276) | function | 276-281 | `func DefaultPurgePolicy() PurgePolicy` | DefaultPurgePolicy returns the default purge policy. |

### [`src/manifest/lifecycle_unit_test.go`](../src/manifest/lifecycle_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestValidAttemptStatuses`](../src/manifest/lifecycle_unit_test.go#L11) | test | 11-22 | `func TestValidAttemptStatuses(t *testing.T)` | TestValidAttemptStatuses verifies valid attempt statuses. |
| [`TestValidateAttemptStatusValid`](../src/manifest/lifecycle_unit_test.go#L25) | test | 25-31 | `func TestValidateAttemptStatusValid(t *testing.T)` | TestValidateAttemptStatusValid verifies validate attempt status valid. |
| [`TestValidateAttemptStatusInvalid`](../src/manifest/lifecycle_unit_test.go#L34) | test | 34-38 | `func TestValidateAttemptStatusInvalid(t *testing.T)` | TestValidateAttemptStatusInvalid verifies validate attempt status invalid. |
| [`TestValidateAttemptStatusEmpty`](../src/manifest/lifecycle_unit_test.go#L41) | test | 41-45 | `func TestValidateAttemptStatusEmpty(t *testing.T)` | TestValidateAttemptStatusEmpty verifies validate attempt status empty. |
| [`TestValidStageOutcomes`](../src/manifest/lifecycle_unit_test.go#L48) | test | 48-59 | `func TestValidStageOutcomes(t *testing.T)` | TestValidStageOutcomes verifies valid stage outcomes. |
| [`TestValidateStageOutcomeValid`](../src/manifest/lifecycle_unit_test.go#L62) | test | 62-68 | `func TestValidateStageOutcomeValid(t *testing.T)` | TestValidateStageOutcomeValid verifies validate stage outcome valid. |
| [`TestValidateStageOutcomeInvalid`](../src/manifest/lifecycle_unit_test.go#L71) | test | 71-75 | `func TestValidateStageOutcomeInvalid(t *testing.T)` | TestValidateStageOutcomeInvalid verifies validate stage outcome invalid. |
| [`TestValidateStageOutcomeEmpty`](../src/manifest/lifecycle_unit_test.go#L78) | test | 78-82 | `func TestValidateStageOutcomeEmpty(t *testing.T)` | TestValidateStageOutcomeEmpty verifies validate stage outcome empty. |
| [`TestValidCacheOutcomes`](../src/manifest/lifecycle_unit_test.go#L85) | test | 85-96 | `func TestValidCacheOutcomes(t *testing.T)` | TestValidCacheOutcomes verifies valid cache outcomes. |
| [`TestValidateCacheOutcomeValid`](../src/manifest/lifecycle_unit_test.go#L99) | test | 99-105 | `func TestValidateCacheOutcomeValid(t *testing.T)` | TestValidateCacheOutcomeValid verifies validate cache outcome valid. |
| [`TestValidateCacheOutcomeInvalid`](../src/manifest/lifecycle_unit_test.go#L108) | test | 108-112 | `func TestValidateCacheOutcomeInvalid(t *testing.T)` | TestValidateCacheOutcomeInvalid verifies validate cache outcome invalid. |
| [`TestValidateCacheOutcomeEmpty`](../src/manifest/lifecycle_unit_test.go#L115) | test | 115-119 | `func TestValidateCacheOutcomeEmpty(t *testing.T)` | TestValidateCacheOutcomeEmpty verifies validate cache outcome empty. |
| [`TestCacheOutcomeConstants`](../src/manifest/lifecycle_unit_test.go#L122) | test | 122-135 | `func TestCacheOutcomeConstants(t *testing.T)` | TestCacheOutcomeConstants verifies cache outcome constants. |
| [`TestValidRunVisibilities`](../src/manifest/lifecycle_unit_test.go#L138) | test | 138-149 | `func TestValidRunVisibilities(t *testing.T)` | TestValidRunVisibilities verifies valid run visibilities. |
| [`TestValidateRunVisibilityValid`](../src/manifest/lifecycle_unit_test.go#L152) | test | 152-158 | `func TestValidateRunVisibilityValid(t *testing.T)` | TestValidateRunVisibilityValid verifies validate run visibility valid. |
| [`TestValidateRunVisibilityInvalid`](../src/manifest/lifecycle_unit_test.go#L161) | test | 161-165 | `func TestValidateRunVisibilityInvalid(t *testing.T)` | TestValidateRunVisibilityInvalid verifies validate run visibility invalid. |
| [`TestValidateRunVisibilityEmpty`](../src/manifest/lifecycle_unit_test.go#L168) | test | 168-172 | `func TestValidateRunVisibilityEmpty(t *testing.T)` | TestValidateRunVisibilityEmpty verifies validate run visibility empty. |
| [`TestRunVisibilityConstants`](../src/manifest/lifecycle_unit_test.go#L175) | test | 175-185 | `func TestRunVisibilityConstants(t *testing.T)` | TestRunVisibilityConstants verifies run visibility constants. |
| [`TestValidAuditActions`](../src/manifest/lifecycle_unit_test.go#L188) | test | 188-206 | `func TestValidAuditActions(t *testing.T)` | TestValidAuditActions verifies valid audit actions. |
| [`TestValidateAuditActionValid`](../src/manifest/lifecycle_unit_test.go#L209) | test | 209-215 | `func TestValidateAuditActionValid(t *testing.T)` | TestValidateAuditActionValid verifies validate audit action valid. |
| [`TestValidateAuditActionInvalid`](../src/manifest/lifecycle_unit_test.go#L218) | test | 218-222 | `func TestValidateAuditActionInvalid(t *testing.T)` | TestValidateAuditActionInvalid verifies validate audit action invalid. |
| [`TestValidateAuditActionEmpty`](../src/manifest/lifecycle_unit_test.go#L225) | test | 225-229 | `func TestValidateAuditActionEmpty(t *testing.T)` | TestValidateAuditActionEmpty verifies validate audit action empty. |
| [`TestAuditActionConstants`](../src/manifest/lifecycle_unit_test.go#L232) | test | 232-259 | `func TestAuditActionConstants(t *testing.T)` | TestAuditActionConstants verifies audit action constants. |
| [`TestAuditEventFields`](../src/manifest/lifecycle_unit_test.go#L262) | test | 262-294 | `func TestAuditEventFields(t *testing.T)` | TestAuditEventFields verifies audit event fields. |
| [`TestDefaultRetentionPolicy`](../src/manifest/lifecycle_unit_test.go#L297) | test | 297-302 | `func TestDefaultRetentionPolicy(t *testing.T)` | TestDefaultRetentionPolicy verifies default retention policy. |
| [`TestDefaultPurgePolicy`](../src/manifest/lifecycle_unit_test.go#L305) | test | 305-313 | `func TestDefaultPurgePolicy(t *testing.T)` | TestDefaultPurgePolicy verifies default purge policy. |
| [`TestRetentionPolicyCustom`](../src/manifest/lifecycle_unit_test.go#L316) | test | 316-323 | `func TestRetentionPolicyCustom(t *testing.T)` | TestRetentionPolicyCustom verifies retention policy custom. |
| [`TestPurgePolicyCustom`](../src/manifest/lifecycle_unit_test.go#L326) | test | 326-337 | `func TestPurgePolicyCustom(t *testing.T)` | TestPurgePolicyCustom verifies purge policy custom. |

### [`src/manifest/manifest.go`](../src/manifest/manifest.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`ResolvedManifest`](../src/manifest/manifest.go#L24) | struct | 24-57 | ``type ResolvedManifest struct { // FormatVersion is the workspace config format version (e.g. 2). FormatVersion int `json:"format_version"` // SearchID is the stable, human-chosen identifier for one research question. SearchID string `json:"search_id"` // SearchRevision is the intentional revision of query, filters, source // selection, or field policy. SearchRevision string `json:"search_revision"` // EnrichmentEnabled is required and determines whether this declared run // performs enrichment. It is part of the run manifest, not a CLI override. EnrichmentEnabled bool `json:"enrichment_enabled"` // ReusePolicy declares whether a matching completed plan may be reused. ReusePolicy string `json:"reuse_policy"` // CachePolicy declares the ordered cache read layers and explicit write // destinations. CachePolicy CachePolicy `json:"cache_policy"` // Sources is the ordered list of resolved source declarations. Sources []SourceManifest `json:"sources"` // EnrichmentProviders is the list of enrichment API configurations // including provider name, base URL, requested fields, and fill-missing // policy. Changes to provider settings alter the execution fingerprint. EnrichmentProviders []EnrichmentProvider `json:"enrichment_providers,omitempty"` // SchemaVersion is the database schema version (latest migration filename) // that this manifest was built against. SchemaVersion string `json:"schema_version"` }`` | ResolvedManifest is the canonical resolved configuration snapshot built from the SOMETHING evaluation result. It contains all interpolated values, expanded paths, resolved source settings, field lists, run policy, and pipeline-affecting configuration that determines the execution fingerprint. The pipeline builds this before any source file is read, persists it, and uses the in-memory copy for the rest of the attempt. |
| [`CachePolicy`](../src/manifest/manifest.go#L63) | struct | 63-77 | ``type CachePolicy struct { // Reads is the ordered list of cache layers to consult when reading. Reads []string `json:"reads"` // Writes is the list of cache layers to write to. Writes []string `json:"writes"` // ReadRunID specifies a specific run to read from when Reads contains // "run_specific". Ignored otherwise. Default -1; an error is raised when // "run_specific" is used without setting this to a valid run ID. ReadRunID int `json:"read_run_id,omitempty"` // NegativeTTLDays is the TTL in days for negative cache entries (404s, etc.). NegativeTTLDays int `json:"negative_ttl_days"` }`` | CachePolicy declares cache read layers, write destinations, optional read_run_id for run-specific reads, and negative-entry TTL. Valid read layer names: "active_run", "global", "network", "run_specific", "run:N" (prior-run snapshot). Valid write layer names: "active_run", "global". |
| [`EnrichmentProvider`](../src/manifest/manifest.go#L84) | struct | 84-117 | ``type EnrichmentProvider struct { // Name is the provider key (e.g. "crossref", "openalex", "orcid"). Name string `json:"name"` // BaseURL is the resolved API endpoint URL. BaseURL string `json:"base_url"` // ExtraURLs is a map of additional named URL endpoints for this provider // (e.g. ORCID's author search endpoint). ExtraURLs map[string]string `json:"extra_urls,omitempty"` // Fields is the list of enrichment fields requested from this provider. Fields []string `json:"fields"` // FillMissingOnly indicates whether this provider stores only fields // that are still missing after prior providers. FillMissingOnly bool `json:"fill_missing_only"` // RatePerSecond is the maximum number of requests per second. RatePerSecond int `json:"rate_per_second"` // Concurrency is the maximum number of concurrent requests. Concurrency int `json:"concurrency"` // TimeoutSeconds is the HTTP request timeout in seconds. TimeoutSeconds int `json:"timeout_seconds"` // MaxRetries is the maximum number of retry attempts for failed requests. MaxRetries int `json:"max_retries"` // BatchSize is the number of items per batch request (e.g. OpenAlex // reference resolution). BatchSize int `json:"batch_size"` }`` | EnrichmentProvider describes one enrichment API configuration that affects the execution fingerprint. Each provider has a name, endpoint URL, requested fields, and whether it fills only missing values. The full set of configurable provider settings is captured so that any behavior-changing edit produces a distinct execution fingerprint. |
| [`RawDataFilter`](../src/manifest/manifest.go#L122) | struct | 122-128 | ``type RawDataFilter struct { // Filters is the ordered list of filter names applied at this stage. Filters []string `json:"filters"` // Count is the number of articles that pass this filter stage. Count int `json:"count"` }`` | RawDataFilter records one stage of source-level filtering with its cumulative article count. Filters are ordered from least to most restrictive within a source declaration. |
| [`SourceManifest`](../src/manifest/manifest.go#L132) | struct | 132-168 | ``type SourceManifest struct { // Name is the source identifier (e.g. "scopus", "ieeexplore", "wos"). Name string `json:"name"` // ExpectedFile is the resolved path to the raw source file. ExpectedFile string `json:"expected_file"` // FileType is the source file type (e.g. "csv", "bib"). Determines which // parser is used during ingestion. FileType string `json:"file_type"` // Query is the search query used to obtain this source. Query string `json:"query"` // Filters records the ordered source-level filter stages and their // cumulative article counts. The first entry (NO_FILTER) is the raw // total; subsequent entries represent progressive filter application. Filters []RawDataFilter `json:"filters,omitempty"` // ExpectedResultCount records the declared count for the selected filters. // It is an expectation, not a count measured while parsing the export. ExpectedResultCount int `json:"expected_result_count,omitempty"` // Date records when the source export was downloaded (provenance). Date string `json:"date,omitempty"` // RequestedFields is the list of enrichment fields requested for this source. RequestedFields []string `json:"requested_fields"` // PatchFields maps source-specific field names to canonical field names. // Changes to rename mappings alter the execution fingerprint. PatchFields map[string]string `json:"patch_fields,omitempty"` // KeepFields is the ordered whitelist of fields to retain after renaming. // Changes to the keep list alter the execution fingerprint. KeepFields []string `json:"keep_fields,omitempty"` }`` | SourceManifest is a resolved source declaration with all interpolation expanded and paths resolved. |
| [`InputManifest`](../src/manifest/manifest.go#L173) | struct | 173-180 | ``type InputManifest struct { // ResolvedManifestHash is the SHA-256 hex digest of the canonical JSON // representation of the ResolvedManifest this input manifest belongs to. ResolvedManifestHash string `json:"resolved_manifest_hash"` // SourceFiles maps source name to its file information. SourceFiles map[string]SourceFileInfo `json:"source_files"` }`` | InputManifest records the state of declared source files before parsing. It links to the resolved-config manifest via ResolvedManifestHash and supplies the source hashes for the execution-plan fingerprint. |
| [`SourceFileInfo`](../src/manifest/manifest.go#L183) | struct | 183-197 | ``type SourceFileInfo struct { // Path is the resolved absolute or relative path to the source file. Path string `json:"path"` // SHA256 is the hex-encoded SHA-256 digest of the file content. SHA256 string `json:"sha256"` // Size is the file size in bytes. Size int64 `json:"size"` // ReadError records why the configured source could not be read during // preflight. It is present only on a failed input manifest; SHA256 and Size // are then unavailable and must not be interpreted as source content data. ReadError string `json:"read_error,omitempty"` }`` | SourceFileInfo holds the identity and content hash of a single source file. |
| [`ArtifactLayout`](../src/manifest/manifest.go#L200) | struct | 200-213 | ``type ArtifactLayout struct { // Root is the root directory for artifact storage. Root string `json:"root"` // ContentHashAlgo is the hash algorithm used for content-addressed naming // (e.g. "sha256"). ContentHashAlgo string `json:"content_hash_algo"` // RetentionPolicy describes how artifacts are retained: // "per_run" — artifacts are scoped to and removed with the owning run. // "shared" — artifacts are content-addressed and retained while any run // references them. RetentionPolicy string `json:"retention_policy"` }`` | ArtifactLayout defines where and how pipeline artifacts are stored. |
| [`CacheStoreLayout`](../src/manifest/manifest.go#L216) | struct | 216-219 | ``type CacheStoreLayout struct { // Path is the filesystem path to the global cache store. Path string `json:"path"` }`` | CacheStoreLayout defines the global cache store location. |
| [`ExecutionFingerprint`](../src/manifest/manifest.go#L224) | type | 224 | `type ExecutionFingerprint string` | ExecutionFingerprint is a SHA-256 hex digest that uniquely identifies an execution plan. It is deterministic: semantically equivalent resolved and input manifests produce the same fingerprint. |
| [`StageFingerprint`](../src/manifest/manifest.go#L229) | struct | 229-241 | ``type StageFingerprint struct { // Stage is the stage name (e.g. "parse", "deduplicate", "enrich"). Stage string `json:"stage"` // InputFingerprint is the execution fingerprint of the upstream input. InputFingerprint ExecutionFingerprint `json:"input_fingerprint"` // ConfigHash is the SHA-256 hex digest of the stage-specific configuration. ConfigHash string `json:"config_hash"` // OutputHash is the SHA-256 hex digest of the stage output, if available. OutputHash string `json:"output_hash,omitempty"` }`` | StageFingerprint identifies a single pipeline stage's input/output for reuse detection. A changed source-file hash, config, or upstream stage output produces a different stage fingerprint. |
| [`canonicalJSON`](../src/manifest/manifest.go#L248) | function | 248-254 | `func canonicalJSON(v any) ([]byte, error)` | canonicalJSON returns the canonical JSON representation of v. The output is deterministic: struct fields are serialized in declaration order, map keys are sorted alphabetically, and no extra whitespace is added. Go's encoding/json performs default HTML escaping of <, >, &, which is deterministic and does not affect fingerprint stability. |
| [`fingerprintInput`](../src/manifest/manifest.go#L260) | struct | 260-263 | ``type fingerprintInput struct { Resolved ResolvedManifest `json:"resolved"` Input inputManifestForFingerprint `json:"input"` }`` | fingerprintInput combines the resolved and input manifests for fingerprint computation. The ResolvedManifestHash in InputManifest is excluded from the fingerprint to avoid circular dependency; the full ResolvedManifest is included directly. |
| [`inputManifestForFingerprint`](../src/manifest/manifest.go#L266) | struct | 266-268 | ``type inputManifestForFingerprint struct { SourceFiles map[string]SourceFileInfo `json:"source_files"` }`` | inputManifestForFingerprint restricts input fingerprinting to authoritative source-file evidence. |
| [`ComputeFingerprint`](../src/manifest/manifest.go#L277) | function | 277-299 | `func ComputeFingerprint(rm *ResolvedManifest, im *InputManifest) (ExecutionFingerprint, error)` | ComputeFingerprint computes a deterministic execution fingerprint from the resolved manifest and input manifest pair. The fingerprint is the SHA-256 hex digest of the canonical JSON representation of both manifests combined. The input manifest's ResolvedManifestHash is a persistence link, not a fingerprint input. It is excluded from the hash to avoid circular dependency. Callers should use NewInputManifest to create correctly linked pairs. |
| [`ComputeStageFingerprint`](../src/manifest/manifest.go#L305) | function | 305-326 | `func ComputeStageFingerprint(stage string, inputFP ExecutionFingerprint, stageConfig any, outputHash string) (StageFingerprint, error)` | ComputeStageFingerprint computes a deterministic fingerprint for a single pipeline stage from its identity, upstream input fingerprint, stage-specific configuration, and stage output hash. A non-empty outputHash is recorded for reuse detection; pass "" when the output is not yet computed. |
| [`(*ResolvedManifest).Hash`](../src/manifest/manifest.go#L330) | method | 330-340 | `func (*ResolvedManifest).Hash() (string, error)` | Hash computes the SHA-256 hex digest of the canonical JSON representation of the ResolvedManifest. |
| [`NewInputManifest`](../src/manifest/manifest.go#L344) | function | 344-363 | `func NewInputManifest(rm *ResolvedManifest, sourceFiles map[string]SourceFileInfo) (*InputManifest, error)` | NewInputManifest creates a new InputManifest linked to the given resolved manifest. It computes the ResolvedManifestHash automatically. |
| [`(*InputManifest).SourceNames`](../src/manifest/manifest.go#L366) | method | 366-373 | `func (*InputManifest).SourceNames() []string` | SourceNames returns the source names in sorted order. |

### [`src/manifest/manifest_unit_test.go`](../src/manifest/manifest_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestCanonicalJSONDeterministic`](../src/manifest/manifest_unit_test.go#L16) | test | 16-29 | `func TestCanonicalJSONDeterministic(t *testing.T)` | TestCanonicalJSONDeterministic verifies canonical json deterministic. |
| [`TestCanonicalJSONSemanticEquivalence`](../src/manifest/manifest_unit_test.go#L32) | test | 32-53 | `func TestCanonicalJSONSemanticEquivalence(t *testing.T)` | TestCanonicalJSONSemanticEquivalence verifies canonical json semantic equivalence. |
| [`TestFingerprintStableForSameManifest`](../src/manifest/manifest_unit_test.go#L56) | test | 56-73 | `func TestFingerprintStableForSameManifest(t *testing.T)` | TestFingerprintStableForSameManifest verifies fingerprint stable for same manifest. |
| [`TestFingerprintStableForSemanticEquivalence`](../src/manifest/manifest_unit_test.go#L76) | test | 76-97 | `func TestFingerprintStableForSemanticEquivalence(t *testing.T)` | TestFingerprintStableForSemanticEquivalence verifies fingerprint stable for semantic equivalence. |
| [`TestFingerprintIsSHA256Hex`](../src/manifest/manifest_unit_test.go#L100) | test | 100-120 | `func TestFingerprintIsSHA256Hex(t *testing.T)` | TestFingerprintIsSHA256Hex verifies fingerprint is sha256 hex. |
| [`TestFingerprintChangesOnFormatVersion`](../src/manifest/manifest_unit_test.go#L123) | test | 123-144 | `func TestFingerprintChangesOnFormatVersion(t *testing.T)` | TestFingerprintChangesOnFormatVersion verifies fingerprint changes on format version. |
| [`TestFingerprintChangesOnSearchID`](../src/manifest/manifest_unit_test.go#L147) | test | 147-166 | `func TestFingerprintChangesOnSearchID(t *testing.T)` | TestFingerprintChangesOnSearchID verifies fingerprint changes on search id. |
| [`TestFingerprintChangesOnSearchRevision`](../src/manifest/manifest_unit_test.go#L169) | test | 169-188 | `func TestFingerprintChangesOnSearchRevision(t *testing.T)` | TestFingerprintChangesOnSearchRevision verifies fingerprint changes on search revision. |
| [`TestFingerprintChangesOnEnrichmentEnabled`](../src/manifest/manifest_unit_test.go#L191) | test | 191-210 | `func TestFingerprintChangesOnEnrichmentEnabled(t *testing.T)` | TestFingerprintChangesOnEnrichmentEnabled verifies fingerprint changes on enrichment enabled. |
| [`TestFingerprintChangesOnReusePolicy`](../src/manifest/manifest_unit_test.go#L213) | test | 213-232 | `func TestFingerprintChangesOnReusePolicy(t *testing.T)` | TestFingerprintChangesOnReusePolicy verifies fingerprint changes on reuse policy. |
| [`TestFingerprintChangesOnCacheReads`](../src/manifest/manifest_unit_test.go#L235) | test | 235-254 | `func TestFingerprintChangesOnCacheReads(t *testing.T)` | TestFingerprintChangesOnCacheReads verifies fingerprint changes on cache reads. |
| [`TestFingerprintChangesOnCacheWrites`](../src/manifest/manifest_unit_test.go#L257) | test | 257-276 | `func TestFingerprintChangesOnCacheWrites(t *testing.T)` | TestFingerprintChangesOnCacheWrites verifies fingerprint changes on cache writes. |
| [`TestFingerprintChangesOnNegativeTTL`](../src/manifest/manifest_unit_test.go#L279) | test | 279-298 | `func TestFingerprintChangesOnNegativeTTL(t *testing.T)` | TestFingerprintChangesOnNegativeTTL verifies fingerprint changes on negative ttl. |
| [`TestFingerprintChangesOnSourceQuery`](../src/manifest/manifest_unit_test.go#L301) | test | 301-320 | `func TestFingerprintChangesOnSourceQuery(t *testing.T)` | TestFingerprintChangesOnSourceQuery verifies fingerprint changes on source query. |
| [`TestFingerprintChangesOnSourceFile`](../src/manifest/manifest_unit_test.go#L323) | test | 323-345 | `func TestFingerprintChangesOnSourceFile(t *testing.T)` | TestFingerprintChangesOnSourceFile verifies fingerprint changes on source file. |
| [`TestFingerprintChangesOnSourceFileOrder`](../src/manifest/manifest_unit_test.go#L348) | test | 348-371 | `func TestFingerprintChangesOnSourceFileOrder(t *testing.T)` | TestFingerprintChangesOnSourceFileOrder verifies fingerprint changes on source file order. |
| [`TestFingerprintChangesOnRequestedFields`](../src/manifest/manifest_unit_test.go#L374) | test | 374-393 | `func TestFingerprintChangesOnRequestedFields(t *testing.T)` | TestFingerprintChangesOnRequestedFields verifies fingerprint changes on requested fields. |
| [`TestFingerprintChangesOnFileType`](../src/manifest/manifest_unit_test.go#L396) | test | 396-415 | `func TestFingerprintChangesOnFileType(t *testing.T)` | TestFingerprintChangesOnFileType verifies fingerprint changes on file type. |
| [`TestFingerprintChangesOnPatchFields`](../src/manifest/manifest_unit_test.go#L418) | test | 418-437 | `func TestFingerprintChangesOnPatchFields(t *testing.T)` | TestFingerprintChangesOnPatchFields verifies fingerprint changes on patch fields. |
| [`TestFingerprintChangesOnKeepFields`](../src/manifest/manifest_unit_test.go#L440) | test | 440-459 | `func TestFingerprintChangesOnKeepFields(t *testing.T)` | TestFingerprintChangesOnKeepFields verifies fingerprint changes on keep fields. |
| [`TestFingerprintChangesOnEnrichmentProviders`](../src/manifest/manifest_unit_test.go#L462) | test | 462-483 | `func TestFingerprintChangesOnEnrichmentProviders(t *testing.T)` | TestFingerprintChangesOnEnrichmentProviders verifies fingerprint changes on enrichment providers. |
| [`TestFingerprintChangesOnSchemaVersion`](../src/manifest/manifest_unit_test.go#L486) | test | 486-505 | `func TestFingerprintChangesOnSchemaVersion(t *testing.T)` | TestFingerprintChangesOnSchemaVersion verifies fingerprint changes on schema version. |
| [`TestFingerprintChangesOnAllModifications`](../src/manifest/manifest_unit_test.go#L508) | test | 508-545 | `func TestFingerprintChangesOnAllModifications(t *testing.T)` | TestFingerprintChangesOnAllModifications verifies fingerprint changes on all modifications. |
| [`TestNewInputManifestLinksToResolved`](../src/manifest/manifest_unit_test.go#L548) | test | 548-565 | `func TestNewInputManifestLinksToResolved(t *testing.T)` | TestNewInputManifestLinksToResolved verifies new input manifest links to resolved. |
| [`TestNewInputManifestNilResolved`](../src/manifest/manifest_unit_test.go#L568) | test | 568-576 | `func TestNewInputManifestNilResolved(t *testing.T)` | TestNewInputManifestNilResolved verifies new input manifest nil resolved. |
| [`TestInputManifestSourceNamesSorted`](../src/manifest/manifest_unit_test.go#L579) | test | 579-592 | `func TestInputManifestSourceNamesSorted(t *testing.T)` | TestInputManifestSourceNamesSorted verifies input manifest source names sorted. |
| [`TestResolvedManifestHashDeterministic`](../src/manifest/manifest_unit_test.go#L595) | test | 595-609 | `func TestResolvedManifestHashDeterministic(t *testing.T)` | TestResolvedManifestHashDeterministic verifies resolved manifest hash deterministic. |
| [`TestResolvedManifestHashNil`](../src/manifest/manifest_unit_test.go#L612) | test | 612-618 | `func TestResolvedManifestHashNil(t *testing.T)` | TestResolvedManifestHashNil verifies resolved manifest hash nil. |
| [`TestComputeStageFingerprint`](../src/manifest/manifest_unit_test.go#L621) | test | 621-646 | `func TestComputeStageFingerprint(t *testing.T)` | TestComputeStageFingerprint verifies compute stage fingerprint. |
| [`TestComputeStageFingerprintEmptyStage`](../src/manifest/manifest_unit_test.go#L649) | test | 649-654 | `func TestComputeStageFingerprintEmptyStage(t *testing.T)` | TestComputeStageFingerprintEmptyStage verifies compute stage fingerprint empty stage. |
| [`TestComputeStageFingerprintNoConfig`](../src/manifest/manifest_unit_test.go#L657) | test | 657-665 | `func TestComputeStageFingerprintNoConfig(t *testing.T)` | TestComputeStageFingerprintNoConfig verifies compute stage fingerprint no config. |
| [`TestStageFingerprintOutputHash`](../src/manifest/manifest_unit_test.go#L668) | test | 668-677 | `func TestStageFingerprintOutputHash(t *testing.T)` | TestStageFingerprintOutputHash verifies stage fingerprint output hash. |
| [`TestStageFingerprintEmptyOutputHash`](../src/manifest/manifest_unit_test.go#L680) | test | 680-689 | `func TestStageFingerprintEmptyOutputHash(t *testing.T)` | TestStageFingerprintEmptyOutputHash verifies stage fingerprint empty output hash. |
| [`TestStageFingerprintChangesOnInput`](../src/manifest/manifest_unit_test.go#L692) | test | 692-708 | `func TestStageFingerprintChangesOnInput(t *testing.T)` | TestStageFingerprintChangesOnInput verifies stage fingerprint changes on input. |
| [`TestStageFingerprintChangesOnConfig`](../src/manifest/manifest_unit_test.go#L711) | test | 711-724 | `func TestStageFingerprintChangesOnConfig(t *testing.T)` | TestStageFingerprintChangesOnConfig verifies stage fingerprint changes on config. |
| [`TestStageFingerprintChangesOnValidationRules`](../src/manifest/manifest_unit_test.go#L727) | test | 727-746 | `func TestStageFingerprintChangesOnValidationRules(t *testing.T)` | TestStageFingerprintChangesOnValidationRules verifies stage fingerprint changes on validation rules. |
| [`TestComputeFingerprintNilResolved`](../src/manifest/manifest_unit_test.go#L749) | test | 749-754 | `func TestComputeFingerprintNilResolved(t *testing.T)` | TestComputeFingerprintNilResolved verifies compute fingerprint nil resolved. |
| [`TestComputeFingerprintNilInput`](../src/manifest/manifest_unit_test.go#L757) | test | 757-762 | `func TestComputeFingerprintNilInput(t *testing.T)` | TestComputeFingerprintNilInput verifies compute fingerprint nil input. |
| [`TestComputeFingerprintHashMismatchIgnored`](../src/manifest/manifest_unit_test.go#L765) | test | 765-778 | `func TestComputeFingerprintHashMismatchIgnored(t *testing.T)` | TestComputeFingerprintHashMismatchIgnored verifies compute fingerprint hash mismatch ignored. |
| [`TestComputeFingerprintEmptyHashAccepted`](../src/manifest/manifest_unit_test.go#L781) | test | 781-789 | `func TestComputeFingerprintEmptyHashAccepted(t *testing.T)` | TestComputeFingerprintEmptyHashAccepted verifies compute fingerprint empty hash accepted. |
| [`TestArtifactLayoutDefaults`](../src/manifest/manifest_unit_test.go#L792) | test | 792-808 | `func TestArtifactLayoutDefaults(t *testing.T)` | TestArtifactLayoutDefaults verifies artifact layout defaults. |
| [`TestCacheStoreLayout`](../src/manifest/manifest_unit_test.go#L811) | test | 811-818 | `func TestCacheStoreLayout(t *testing.T)` | TestCacheStoreLayout verifies cache store layout. |
| [`TestFingerprintChangesTable`](../src/manifest/manifest_unit_test.go#L821) | test | 821-1050 | `func TestFingerprintChangesTable(t *testing.T)` | TestFingerprintChangesTable verifies fingerprint changes table. |
| [`TestFingerprintEmptyFields`](../src/manifest/manifest_unit_test.go#L1053) | test | 1053-1074 | `func TestFingerprintEmptyFields(t *testing.T)` | TestFingerprintEmptyFields verifies fingerprint empty fields. |
| [`BenchmarkComputeFingerprint`](../src/manifest/manifest_unit_test.go#L1077) | benchmark | 1077-1088 | `func BenchmarkComputeFingerprint(b *testing.B)` | BenchmarkComputeFingerprint measures compute fingerprint. |
| [`BenchmarkResolvedManifestHash`](../src/manifest/manifest_unit_test.go#L1091) | benchmark | 1091-1101 | `func BenchmarkResolvedManifestHash(b *testing.B)` | BenchmarkResolvedManifestHash measures resolved manifest hash. |
| [`ExampleComputeFingerprint`](../src/manifest/manifest_unit_test.go#L1104) | function | 1104-1147 | `func ExampleComputeFingerprint()` | ExampleComputeFingerprint supports the package test suite's example compute fingerprint setup or assertions. |

### [`src/normalization/normalization.go`](../src/normalization/normalization.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`titleFirstRune`](../src/normalization/normalization.go#L20) | function | 20-27 | `func titleFirstRune(value string) string` | titleFirstRune lowercases a string and uppercases its first Unicode rune. |
| [`firstRune`](../src/normalization/normalization.go#L30) | function | 30-35 | `func firstRune(value string) string` | firstRune returns the first Unicode rune as a string, or an empty string for empty input. |
| [`firstLetterIsNonASCII`](../src/normalization/normalization.go#L38) | function | 38-45 | `func firstLetterIsNonASCII(value string) bool` | firstLetterIsNonASCII reports whether the first Unicode letter is outside ASCII. |
| [`isInitials`](../src/normalization/normalization.go#L62) | function | 62-77 | `func isInitials(word string) bool` | isInitials reports whether a word is a supported compact or punctuated initial sequence. |
| [`isUpperAlpha`](../src/normalization/normalization.go#L80) | function | 80-87 | `func isUpperAlpha(s string) bool` | isUpperAlpha reports whether a non-empty string contains only ASCII uppercase letters. |
| [`smartTitle`](../src/normalization/normalization.go#L90) | function | 90-117 | `func smartTitle(word string) string` | smartTitle applies author-name casing rules to one word while preserving particles and compounds. |
| [`splitGivenFamily`](../src/normalization/normalization.go#L120) | function | 120-137 | `func splitGivenFamily(words []string) (given, family []string)` | splitGivenFamily splits given family into its component values. |
| [`wordToInitial`](../src/normalization/normalization.go#L140) | function | 140-158 | `func wordToInitial(word string) string` | wordToInitial converts a given-name word or existing initial sequence to dotted uppercase initials. |
| [`toInitials`](../src/normalization/normalization.go#L161) | function | 161-173 | `func toInitials(givenStr string) string` | toInitials converts whitespace-separated given names to canonical dotted initials. |
| [`normalizeFamilyWords`](../src/normalization/normalization.go#L176) | function | 176-203 | `func normalizeFamilyWords(words []string) []string` | normalizeFamilyWords normalizes family words. |
| [`NormalizeAuthorName`](../src/normalization/normalization.go#L208) | function | 208-266 | `func NormalizeAuthorName(name string) string` | NormalizeAuthorName normalizes a single author name to 'Lastname, F. M.' format. |
| [`splitOnComma`](../src/normalization/normalization.go#L269) | function | 269-283 | `func splitOnComma(name string) []string` | splitOnComma splits on comma into its component values. |
| [`SplitFirstLast`](../src/normalization/normalization.go#L286) | function | 286-292 | `func SplitFirstLast(normalized string) (first, last string)` | SplitFirstLast splits 'Lastname, F. M.' into (first_name, last_name). |
| [`expandAbbreviations`](../src/normalization/normalization.go#L330) | function | 330-337 | `func expandAbbreviations(text string) string` | expandAbbreviations expands recognized affiliation abbreviations case-insensitively. |
| [`affTitleWord`](../src/normalization/normalization.go#L340) | function | 340-382 | `func affTitleWord(word string, firstWord bool) string` | affTitleWord applies affiliation-specific casing while preserving punctuation and acronyms. |
| [`normalizeAffiliationCase`](../src/normalization/normalization.go#L387) | function | 387-421 | `func normalizeAffiliationCase(text string) string` | normalizeAffiliationCase normalizes affiliation case. |
| [`NormalizeAffiliation`](../src/normalization/normalization.go#L424) | function | 424-435 | `func NormalizeAffiliation(aff string) string` | NormalizeAffiliation normalizes an affiliation string. |
| [`isAcronym`](../src/normalization/normalization.go#L477) | function | 477-480 | `func isAcronym(word string) bool` | isAcronym reports whether a word is a short all-uppercase publisher acronym. |
| [`pubTitleCore`](../src/normalization/normalization.go#L483) | function | 483-505 | `func pubTitleCore(core string) string` | pubTitleCore applies publisher-specific casing to an unpunctuated word core. |
| [`pubTitleWord`](../src/normalization/normalization.go#L508) | function | 508-526 | `func pubTitleWord(word string) string` | pubTitleWord applies publisher casing while preserving surrounding punctuation. |
| [`detachLegalSuffix`](../src/normalization/normalization.go#L529) | function | 529-535 | `func detachLegalSuffix(text string) (string, string)` | detachLegalSuffix separates a recognized publisher legal suffix from the preceding name. |
| [`titleCasePublisher`](../src/normalization/normalization.go#L538) | function | 538-627 | `func titleCasePublisher(name string) string` | titleCasePublisher applies publisher-specific title casing across words and parenthesized text. |
| [`NormalizePublisher`](../src/normalization/normalization.go#L630) | function | 630-656 | `func NormalizePublisher(publisher string) string` | NormalizePublisher normalizes a publisher name. |
| [`journalIsAcronym`](../src/normalization/normalization.go#L708) | function | 708-710 | `func journalIsAcronym(word string) bool` | journalIsAcronym reports whether a word is in the recognized journal-acronym set. |
| [`stripJournalSubtitle`](../src/normalization/normalization.go#L715) | function | 715-717 | `func stripJournalSubtitle(name string) string` | stripJournalSubtitle removes a dash-delimited generic article subtitle from a journal name. |
| [`titleCaseJournal`](../src/normalization/normalization.go#L720) | function | 720-809 | `func titleCaseJournal(name string) string` | titleCaseJournal applies journal-specific casing across words, punctuation, and parenthesized text. |
| [`NormalizeJournal`](../src/normalization/normalization.go#L812) | function | 812-829 | `func NormalizeJournal(journal string) string` | NormalizeJournal normalizes a journal name. |

### [`src/normalization/normalization_unit_test.go`](../src/normalization/normalization_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestIsInitials`](../src/normalization/normalization_unit_test.go#L14) | test | 14-27 | `func TestIsInitials(t *testing.T)` | TestIsInitials verifies is initials. |
| [`TestSmartTitle`](../src/normalization/normalization_unit_test.go#L34) | test | 34-62 | `func TestSmartTitle(t *testing.T)` | TestSmartTitle verifies smart title. |
| [`TestWordToInitial`](../src/normalization/normalization_unit_test.go#L69) | test | 69-79 | `func TestWordToInitial(t *testing.T)` | TestWordToInitial verifies word to initial. |
| [`TestToInitials`](../src/normalization/normalization_unit_test.go#L86) | test | 86-93 | `func TestToInitials(t *testing.T)` | TestToInitials verifies to initials. |
| [`TestNormalizeFamilyWords`](../src/normalization/normalization_unit_test.go#L100) | test | 100-125 | `func TestNormalizeFamilyWords(t *testing.T)` | TestNormalizeFamilyWords verifies normalize family words. |
| [`TestSplitGivenFamily`](../src/normalization/normalization_unit_test.go#L132) | test | 132-145 | `func TestSplitGivenFamily(t *testing.T)` | TestSplitGivenFamily verifies split given family. |
| [`TestSplitOnComma`](../src/normalization/normalization_unit_test.go#L152) | test | 152-162 | `func TestSplitOnComma(t *testing.T)` | TestSplitOnComma verifies split on comma. |
| [`TestNormalizeAuthorName`](../src/normalization/normalization_unit_test.go#L169) | test | 169-208 | `func TestNormalizeAuthorName(t *testing.T)` | TestNormalizeAuthorName verifies normalize author name. |
| [`TestSplitFirstLast`](../src/normalization/normalization_unit_test.go#L215) | test | 215-225 | `func TestSplitFirstLast(t *testing.T)` | TestSplitFirstLast verifies split first last. |
| [`TestNormalizeAffiliation`](../src/normalization/normalization_unit_test.go#L232) | test | 232-293 | `func TestNormalizeAffiliation(t *testing.T)` | TestNormalizeAffiliation verifies normalize affiliation. |
| [`contains`](../src/normalization/normalization_unit_test.go#L296) | function | 296-298 | `func contains(s, substr string) bool` | contains supports the package test suite's contains setup or assertions. |
| [`containsStr`](../src/normalization/normalization_unit_test.go#L301) | function | 301-308 | `func containsStr(s, substr string) bool` | containsStr supports the package test suite's contains str setup or assertions. |
| [`TestNormalizePublisher`](../src/normalization/normalization_unit_test.go#L315) | test | 315-356 | `func TestNormalizePublisher(t *testing.T)` | TestNormalizePublisher verifies normalize publisher. |
| [`TestNormalizeJournal`](../src/normalization/normalization_unit_test.go#L363) | test | 363-404 | `func TestNormalizeJournal(t *testing.T)` | TestNormalizeJournal verifies normalize journal. |
| [`hasPrefix`](../src/normalization/normalization_unit_test.go#L407) | function | 407-409 | `func hasPrefix(s, prefix string) bool` | hasPrefix supports the package test suite's has prefix setup or assertions. |
| [`TestFirstRune`](../src/normalization/normalization_unit_test.go#L416) | test | 416-429 | `func TestFirstRune(t *testing.T)` | TestFirstRune verifies first rune. |
| [`TestFirstLetterIsNonASCII`](../src/normalization/normalization_unit_test.go#L436) | test | 436-452 | `func TestFirstLetterIsNonASCII(t *testing.T)` | TestFirstLetterIsNonASCII verifies first letter is non ascii. |
| [`TestTitleFirstRune`](../src/normalization/normalization_unit_test.go#L459) | test | 459-475 | `func TestTitleFirstRune(t *testing.T)` | TestTitleFirstRune verifies title first rune. |
| [`TestIsUpperAlpha`](../src/normalization/normalization_unit_test.go#L482) | test | 482-504 | `func TestIsUpperAlpha(t *testing.T)` | TestIsUpperAlpha verifies is upper alpha. |
| [`TestExpandAbbreviations`](../src/normalization/normalization_unit_test.go#L511) | test | 511-527 | `func TestExpandAbbreviations(t *testing.T)` | TestExpandAbbreviations verifies expand abbreviations. |
| [`TestAffTitleWord`](../src/normalization/normalization_unit_test.go#L534) | test | 534-562 | `func TestAffTitleWord(t *testing.T)` | TestAffTitleWord verifies aff title word. |
| [`TestNormalizeAffiliationCase`](../src/normalization/normalization_unit_test.go#L569) | test | 569-581 | `func TestNormalizeAffiliationCase(t *testing.T)` | TestNormalizeAffiliationCase verifies normalize affiliation case. |
| [`TestIsAcronym`](../src/normalization/normalization_unit_test.go#L588) | test | 588-613 | `func TestIsAcronym(t *testing.T)` | TestIsAcronym verifies is acronym. |
| [`TestPubTitleCore`](../src/normalization/normalization_unit_test.go#L620) | test | 620-645 | `func TestPubTitleCore(t *testing.T)` | TestPubTitleCore verifies pub title core. |
| [`TestPubTitleWord`](../src/normalization/normalization_unit_test.go#L652) | test | 652-677 | `func TestPubTitleWord(t *testing.T)` | TestPubTitleWord verifies pub title word. |
| [`TestDetachLegalSuffix`](../src/normalization/normalization_unit_test.go#L684) | test | 684-710 | `func TestDetachLegalSuffix(t *testing.T)` | TestDetachLegalSuffix verifies detach legal suffix. |
| [`TestTitleCasePublisher`](../src/normalization/normalization_unit_test.go#L717) | test | 717-742 | `func TestTitleCasePublisher(t *testing.T)` | TestTitleCasePublisher verifies title case publisher. |
| [`TestJournalIsAcronym`](../src/normalization/normalization_unit_test.go#L749) | test | 749-768 | `func TestJournalIsAcronym(t *testing.T)` | TestJournalIsAcronym verifies journal is acronym. |
| [`TestStripJournalSubtitle`](../src/normalization/normalization_unit_test.go#L775) | test | 775-796 | `func TestStripJournalSubtitle(t *testing.T)` | TestStripJournalSubtitle verifies strip journal subtitle. |
| [`TestTitleCaseJournal`](../src/normalization/normalization_unit_test.go#L803) | test | 803-834 | `func TestTitleCaseJournal(t *testing.T)` | TestTitleCaseJournal verifies title case journal. |

### [`src/pdfstore/audit.go`](../src/pdfstore/audit.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`OutboxEvent`](../src/pdfstore/audit.go#L14) | struct | 14-24 | `type OutboxEvent struct { EventKey string OccurredAt string Actor string PipelineRunID int64 EntityType string EntityID string Action string MetadataJSON string CorrelationID string }` | OutboxEvent carries metadata audit evidence awaiting cross-database delivery. |
| [`insertOutbox`](../src/pdfstore/audit.go#L27) | function | 27-50 | `func insertOutbox(ctx context.Context, tx *sql.Tx, event OutboxEvent, occurredAt string) error` | insertOutbox inserts outbox. |
| [`(*Store).FlushAuditOutbox`](../src/pdfstore/audit.go#L56) | method | 56-131 | `func (*Store).FlushAuditOutbox(ctx context.Context, metadata *sql.DB) (int, error)` | FlushAuditOutbox mirrors undelivered PDF events into the metadata database. The metadata audit row and delivery link commit together. Marking the PDF event delivered is a separate idempotent step so crashes cannot duplicate an append-only audit row. |

### [`src/pdfstore/helpers_test.go`](../src/pdfstore/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`openTestStore`](../src/pdfstore/helpers_test.go#L12) | function | 12-20 | `func openTestStore(t *testing.T) *Store` | openTestStore supports the package test suite's open test store setup or assertions. |

### [`src/pdfstore/store.go`](../src/pdfstore/store.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Store`](../src/pdfstore/store.go#L29) | struct | 29-32 | `type Store struct { DB *sql.DB now func() time.Time }` | Store is the writable companion PDF database. |
| [`Document`](../src/pdfstore/store.go#L35) | struct | 35-41 | `type Document struct { DOI string Status string ContentHash string InventoriedAt string UpdatedAt string }` | Document describes one normalized article's PDF inventory state. |
| [`AddResult`](../src/pdfstore/store.go#L44) | struct | 44-48 | `type AddResult struct { ContentHash string ByteSize int Added bool }` | AddResult reports the content identity, byte size, and insertion outcome of a manual PDF add. |
| [`Open`](../src/pdfstore/store.go#L52) | function | 52-58 | `func Open(path, registryPath string) (*Store, error)` | Open creates or opens the PDF store and applies its independent migration chain selected by the database registry. |
| [`(*Store).Close`](../src/pdfstore/store.go#L61) | method | 61 | `func (*Store).Close() error` | Close releases resources owned by the receiver. |
| [`timestamp`](../src/pdfstore/store.go#L64) | function | 64 | `func timestamp(t time.Time) string` | timestamp formats a UTC time for persisted PDF metadata. |
| [`newCorrelationID`](../src/pdfstore/store.go#L67) | function | 67-76 | `func newCorrelationID() (string, error)` | newCorrelationID returns a cryptographically random hexadecimal audit correlation identifier. |
| [`(*Store).Document`](../src/pdfstore/store.go#L79) | method | 79-98 | `func (*Store).Document(ctx context.Context, doi string) (*Document, error)` | Document returns PDF inventory metadata for a normalized DOI, or nil when it is unregistered. |
| [`(*Store).Register`](../src/pdfstore/store.go#L103) | method | 103-156 | `func (*Store).Register(ctx context.Context, doi string, workID, pipelineRunID int64) (bool, error)` | Register creates the not-available inventory row for one normalized work. Re-registering a DOI preserves its current state and emits no duplicate audit event. |
| [`(*Store).Add`](../src/pdfstore/store.go#L161) | method | 161-251 | `func (*Store).Add(ctx context.Context, doi string, workID int64, data []byte) (AddResult, error)` | Add inventories a validated local PDF for a normalized work. The DOI must already have been registered by the pipeline. An available record is immutable; adding the same DOI again reports it as unchanged. |
| [`BindStore`](../src/pdfstore/store.go#L255) | function | 255-279 | `func BindStore(ctx context.Context, metadata *sql.DB, relativePath string) error` | BindStore records a portable bundle-relative companion path. Existing bindings are preserved so older corpus bundles remain usable. |
| [`BoundStorePath`](../src/pdfstore/store.go#L283) | function | 283-295 | `func BoundStorePath(ctx context.Context, metadata *sql.DB, metadataPath string) (string, error)` | BoundStorePath returns the existing companion path or binds the default corpus.pdf.db beside the metadata database on first inventory use. |
| [`resolveStorePath`](../src/pdfstore/store.go#L298) | function | 298-315 | `func resolveStorePath(metadataPath, relativePath string) (string, error)` | resolveStorePath resolves store path from the supplied context. |
| [`validateRelativeStorePath`](../src/pdfstore/store.go#L318) | function | 318-327 | `func validateRelativeStorePath(relativePath string) (string, error)` | validateRelativeStorePath rejects absolute or escaping companion-store paths and returns a clean relative path. |

### [`src/pdfstore/store_integration_test.go`](../src/pdfstore/store_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAddNormalizesDOIDeduplicatesBlobsAndPreservesExistingDocument`](../src/pdfstore/store_integration_test.go#L17) | test | 17-59 | `func TestAddNormalizesDOIDeduplicatesBlobsAndPreservesExistingDocument(t *testing.T)` | TestAddNormalizesDOIDeduplicatesBlobsAndPreservesExistingDocument verifies add normalizes doi deduplicates blobs and preserves existing document. |
| [`TestAddAndAuditOutboxAreTransactionalAndIdempotent`](../src/pdfstore/store_integration_test.go#L62) | test | 62-117 | `func TestAddAndAuditOutboxAreTransactionalAndIdempotent(t *testing.T)` | TestAddAndAuditOutboxAreTransactionalAndIdempotent verifies add and audit outbox are transactional and idempotent. |
| [`TestAddRollsBackWhenAuditOutboxWriteFails`](../src/pdfstore/store_integration_test.go#L120) | test | 120-149 | `func TestAddRollsBackWhenAuditOutboxWriteFails(t *testing.T)` | TestAddRollsBackWhenAuditOutboxWriteFails verifies add rolls back when audit outbox write fails. |
| [`TestBoundStorePathUsesDefaultAndPreservesExistingBinding`](../src/pdfstore/store_integration_test.go#L152) | test | 152-172 | `func TestBoundStorePathUsesDefaultAndPreservesExistingBinding(t *testing.T)` | TestBoundStorePathUsesDefaultAndPreservesExistingBinding verifies bound store path uses default and preserves existing binding. |
| [`TestStoreIntegrationInvalidInputs`](../src/pdfstore/store_integration_test.go#L175) | test | 175-183 | `func TestStoreIntegrationInvalidInputs(t *testing.T)` | TestStoreIntegrationInvalidInputs verifies store integration invalid inputs. |

### [`src/pdfstore/store_unit_test.go`](../src/pdfstore/store_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestNewCorrelationIDProducesUniqueNonEmptyValues`](../src/pdfstore/store_unit_test.go#L14) | test | 14-29 | `func TestNewCorrelationIDProducesUniqueNonEmptyValues(t *testing.T)` | TestNewCorrelationIDProducesUniqueNonEmptyValues verifies new correlation id produces unique non empty values. |
| [`TestNewCorrelationIDFormatMatchesUUID`](../src/pdfstore/store_unit_test.go#L32) | test | 32-44 | `func TestNewCorrelationIDFormatMatchesUUID(t *testing.T)` | TestNewCorrelationIDFormatMatchesUUID verifies new correlation id format matches uuid. |
| [`TestTimestampReturnsNonEmptyRFC3339NanoFormat`](../src/pdfstore/store_unit_test.go#L47) | test | 47-65 | `func TestTimestampReturnsNonEmptyRFC3339NanoFormat(t *testing.T)` | TestTimestampReturnsNonEmptyRFC3339NanoFormat verifies timestamp returns non empty rfc3339 nano format. |
| [`TestValidateRelativeStorePathAcceptsValidPath`](../src/pdfstore/store_unit_test.go#L68) | test | 68-76 | `func TestValidateRelativeStorePathAcceptsValidPath(t *testing.T)` | TestValidateRelativeStorePathAcceptsValidPath verifies validate relative store path accepts valid path. |
| [`TestValidateRelativeStorePathRejectsUnsafePaths`](../src/pdfstore/store_unit_test.go#L79) | test | 79-85 | `func TestValidateRelativeStorePathRejectsUnsafePaths(t *testing.T)` | TestValidateRelativeStorePathRejectsUnsafePaths verifies validate relative store path rejects unsafe paths. |
| [`TestValidateRelativeStorePathRejectsAbsolutePath`](../src/pdfstore/store_unit_test.go#L88) | test | 88-92 | `func TestValidateRelativeStorePathRejectsAbsolutePath(t *testing.T)` | TestValidateRelativeStorePathRejectsAbsolutePath verifies validate relative store path rejects absolute path. |
| [`TestStorePathValidationRejectsUnsafePaths`](../src/pdfstore/store_unit_test.go#L95) | test | 95-108 | `func TestStorePathValidationRejectsUnsafePaths(t *testing.T)` | TestStorePathValidationRejectsUnsafePaths verifies store path validation rejects unsafe paths. |
| [`TestResolveStorePathResolvesRelativePath`](../src/pdfstore/store_unit_test.go#L111) | test | 111-122 | `func TestResolveStorePathResolvesRelativePath(t *testing.T)` | TestResolveStorePathResolvesRelativePath verifies resolve store path resolves relative path. |
| [`TestResolveStorePathRejectsMetadataPathEqualToStorePath`](../src/pdfstore/store_unit_test.go#L125) | test | 125-130 | `func TestResolveStorePathRejectsMetadataPathEqualToStorePath(t *testing.T)` | TestResolveStorePathRejectsMetadataPathEqualToStorePath verifies resolve store path rejects metadata path equal to store path. |

### [`src/pdfstore/validation.go`](../src/pdfstore/validation.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`ValidatePDF`](../src/pdfstore/validation.go#L16) | function | 16-28 | `func ValidatePDF(data []byte, maxBytes int) (string, error)` | ValidatePDF enforces the byte bound and the PDF signature, then returns the lowercase SHA-256 content hash. |

### [`src/pdfstore/validation_unit_test.go`](../src/pdfstore/validation_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestValidatePDFBoundariesAndHash`](../src/pdfstore/validation_unit_test.go#L12) | test | 12-27 | `func TestValidatePDFBoundariesAndHash(t *testing.T)` | TestValidatePDFBoundariesAndHash verifies validate pdf boundaries and hash. |

### [`src/server/audit.go`](../src/server/audit.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).runAudit`](../src/server/audit.go#L20) | method | 20-38 | `func (*Server).runAudit(w http.ResponseWriter, r *http.Request)` | runAudit returns run-scoped and eligible global audit events with filters and facets. |
| [`(*Server).audit`](../src/server/audit.go#L41) | method | 41-201 | `func (*Server).audit(w http.ResponseWriter, r *http.Request)` | audit validates filters and returns a cursor-paginated audit timeline with summary and facets. |
| [`auditMultiValues`](../src/server/audit.go#L204) | function | 204-224 | `func auditMultiValues(raw, parameter string) ([]string, error)` | auditMultiValues parses, deduplicates, and bounds a comma-separated audit facet filter. |
| [`auditInClause`](../src/server/audit.go#L227) | function | 227-235 | `func auditInClause(column string, values []string) (string, []any)` | auditInClause builds a parameterized SQL IN clause for validated audit facet values. |
| [`auditWhere`](../src/server/audit.go#L238) | function | 238-243 | `func auditWhere(clauses []string) string` | auditWhere joins audit predicates into an optional SQL WHERE clause. |
| [`(*Server).auditSummary`](../src/server/audit.go#L246) | method | 246-261 | `func (*Server).auditSummary(ctx context.Context, where string, args []any) (map[string]any, error)` | auditSummary counts filtered audit events by presentation category. |
| [`(*Server).auditFacet`](../src/server/audit.go#L264) | method | 264-291 | `func (*Server).auditFacet(ctx context.Context, column string, runID *int64, includeGlobalPDF bool) ([]string, error)` | auditFacet returns distinct non-empty values for an allowlisted audit column and run scope. |
| [`(*Server).auditRows`](../src/server/audit.go#L294) | method | 294-301 | `func (*Server).auditRows(ctx context.Context, condition string, args ...any) ([]map[string]any, error)` | auditRows returns audit event rows matching a caller-supplied parameterized condition. |
| [`(*Server).trash`](../src/server/audit.go#L304) | method | 304-320 | `func (*Server).trash(w http.ResponseWriter, r *http.Request)` | trash returns runs whose visibility state is trashed. |
| [`(*Server).runArtifacts`](../src/server/audit.go#L323) | method | 323-401 | `func (*Server).runArtifacts(w http.ResponseWriter, r *http.Request)` | runArtifacts returns artifact metadata linked to the selected run. |
| [`(*Server).artifactContent`](../src/server/audit.go#L404) | method | 404-424 | `func (*Server).artifactContent(w http.ResponseWriter, r *http.Request)` | artifactContent streams one stored artifact blob with a safe content disposition. |
| [`(*Server).artifactInspection`](../src/server/audit.go#L427) | method | 427-479 | `func (*Server).artifactInspection(w http.ResponseWriter, r *http.Request)` | artifactInspection returns bounded metadata and preview content for one artifact. |
| [`(*Server).artifactPreviewBlob`](../src/server/audit.go#L482) | method | 482-502 | `func (*Server).artifactPreviewBlob(ctx context.Context, artifactID int64, previewBytes int) (string, int64, int64, []byte, error)` | artifactPreviewBlob reads a bounded artifact prefix together with its media type and total size. |
| [`(*Server).artifactBlob`](../src/server/audit.go#L505) | method | 505-525 | `func (*Server).artifactBlob(ctx context.Context, artifactID int64) (string, int64, []byte, error)` | artifactBlob returns the complete content-addressed blob for one run artifact. |
| [`normalizedArtifactContentType`](../src/server/audit.go#L528) | function | 528-534 | `func normalizedArtifactContentType(contentType string) string` | normalizedArtifactContentType parses and lowercases an artifact media type without parameters. |
| [`jsonArtifactContentType`](../src/server/audit.go#L537) | function | 537-540 | `func jsonArtifactContentType(contentType string) bool` | jsonArtifactContentType reports whether a normalized media type carries JSON. |
| [`inlineArtifactContentType`](../src/server/audit.go#L543) | function | 543-549 | `func inlineArtifactContentType(contentType string) bool` | inlineArtifactContentType reports whether a normalized media type is safe for inline display. |
| [`(*Server).artifactFilename`](../src/server/audit.go#L552) | method | 552-576 | `func (*Server).artifactFilename(ctx context.Context, artifactID int64, contentType string) string` | artifactFilename derives a safe download filename from artifact metadata and media type. |
| [`(*Server).runCacheUses`](../src/server/audit.go#L579) | method | 579-624 | `func (*Server).runCacheUses(w http.ResponseWriter, r *http.Request)` | runCacheUses returns cache-use evidence recorded for the selected run. |

### [`src/server/audit_integration_test.go`](../src/server/audit_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAPIDetailsArtifactsAndAudit`](../src/server/audit_integration_test.go#L15) | test | 15-106 | `func TestAPIDetailsArtifactsAndAudit(t *testing.T)` | TestAPIDetailsArtifactsAndAudit verifies api details artifacts and audit. |

### [`src/server/corpus.go`](../src/server/corpus.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`scopedRowsDefinition`](../src/server/corpus.go#L16) | struct | 16-23 | `type scopedRowsDefinition struct { columns []string from string where string groupBy string search string sortFields map[string]string }` | scopedRowsDefinition defines the safe projection, joins, filters, and sorting for one corpus section. |
| [`(*Server).runCorpus`](../src/server/corpus.go#L72) | method | 72-134 | `func (*Server).runCorpus(w http.ResponseWriter, r *http.Request)` | runCorpus returns one context-scoped corpus section for the selected run. |
| [`corpusSelectColumns`](../src/server/corpus.go#L137) | function | 137-150 | `func corpusSelectColumns(kind string) string` | corpusSelectColumns returns the fixed safe projection for a browsable corpus section. |
| [`(*Server).runStages`](../src/server/corpus.go#L153) | method | 153-209 | `func (*Server).runStages(w http.ResponseWriter, r *http.Request)` | runStages returns detailed work-stage outcomes for the selected run. |
| [`(*Server).runStageSummaries`](../src/server/corpus.go#L212) | method | 212-268 | `func (*Server).runStageSummaries(ctx context.Context, runID int64) ([]map[string]any, error)` | runStageSummaries returns aggregate outcome counts by pipeline stage. |
| [`scopedRowsRequest`](../src/server/corpus.go#L271) | function | 271-305 | `func scopedRowsRequest(r *http.Request, fields map[string]string, fallback string) (int, int, string, string, string, error)` | scopedRowsRequest parses and validates the context, filters, sorting, and pagination for a corpus request. |
| [`scopedWhere`](../src/server/corpus.go#L308) | function | 308-321 | `func scopedWhere(base, searchable string, runID int64, query string) (string, []any)` | scopedWhere builds the SQL predicate and arguments for a scoped corpus request. |
| [`scopedPagination`](../src/server/corpus.go#L324) | function | 324-330 | `func scopedPagination(page, perPage int, total int64, sort, order string) map[string]any` | scopedPagination returns validated page, page-size, offset, and limit values. |
| [`(*Server).requireRun`](../src/server/corpus.go#L333) | method | 333-342 | `func (*Server).requireRun(ctx context.Context, runID int64) error` | requireRun requires a valid run value. |

### [`src/server/corpus_integration_test.go`](../src/server/corpus_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestRunScopedCorpusAndStages`](../src/server/corpus_integration_test.go#L15) | test | 15-65 | `func TestRunScopedCorpusAndStages(t *testing.T)` | TestRunScopedCorpusAndStages verifies run scoped corpus and stages. |

### [`src/server/details.go`](../src/server/details.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).articleDetail`](../src/server/details.go#L19) | method | 19-98 | `func (*Server).articleDetail(w http.ResponseWriter, r *http.Request)` | articleDetail treats the numeric route identifier as an immutable work revision ID. It intentionally does not expose the retired mutable articles projection. |
| [`(*Server).authorDetail`](../src/server/details.go#L101) | method | 101-131 | `func (*Server).authorDetail(w http.ResponseWriter, r *http.Request)` | authorDetail returns one author occurrence with its articles and audit evidence. |
| [`(*Server).referenceDetail`](../src/server/details.go#L134) | method | 134-170 | `func (*Server).referenceDetail(w http.ResponseWriter, r *http.Request)` | referenceDetail returns one reference mention with its citing and resolved-work context. |
| [`(*Server).rows`](../src/server/details.go#L173) | method | 173-180 | `func (*Server).rows(ctx context.Context, query string, args ...any) ([]map[string]any, error)` | rows executes a read-only query and converts every result row to a field map. |
| [`(*Server).oneRow`](../src/server/details.go#L183) | method | 183-192 | `func (*Server).oneRow(ctx context.Context, query string, args ...any) (map[string]any, error)` | oneRow returns the first mapped query row, or nil when the query returns no rows. |
| [`stringID`](../src/server/details.go#L195) | function | 195 | `func stringID(id int64) string` | stringID formats a numeric database identifier in base 10. |

### [`src/server/details_integration_test.go`](../src/server/details_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAPIDetailsArticleAuthorReference`](../src/server/details_integration_test.go#L14) | test | 14-40 | `func TestAPIDetailsArticleAuthorReference(t *testing.T)` | TestAPIDetailsArticleAuthorReference verifies api details article author reference. |
| [`TestAPIReferenceResolutionUsesFinalRevision`](../src/server/details_integration_test.go#L43) | test | 43-90 | `func TestAPIReferenceResolutionUsesFinalRevision(t *testing.T)` | TestAPIReferenceResolutionUsesFinalRevision verifies api reference resolution uses final revision. |
| [`TestAPIArticleDetailIncludesRunScopedActivityHistory`](../src/server/details_integration_test.go#L93) | test | 93-170 | `func TestAPIArticleDetailIncludesRunScopedActivityHistory(t *testing.T)` | TestAPIArticleDetailIncludesRunScopedActivityHistory verifies api article detail includes run scoped activity history. |

### [`src/server/evaluation.go`](../src/server/evaluation.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).runEvaluation`](../src/server/evaluation.go#L20) | method | 20-73 | `func (*Server).runEvaluation(w http.ResponseWriter, r *http.Request)` | runEvaluation lists the selected run's normalized articles and overlays their state from the independently bound PDF inventory. |
| [`(*Server).overlayPDFInventory`](../src/server/evaluation.go#L76) | method | 76-122 | `func (*Server).overlayPDFInventory(ctx context.Context, items []map[string]any) error` | overlayPDFInventory overlays companion PDF availability onto evaluation rows by normalized DOI. |

### [`src/server/evaluation_integration_test.go`](../src/server/evaluation_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvaluationListsOnlyNormalizedArticlesWithInventoryState`](../src/server/evaluation_integration_test.go#L15) | test | 15-45 | `func TestEvaluationListsOnlyNormalizedArticlesWithInventoryState(t *testing.T)` | TestEvaluationListsOnlyNormalizedArticlesWithInventoryState verifies evaluation lists only normalized articles with inventory state. |

### [`src/server/fixture_integration_test.go`](../src/server/fixture_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestGenerateFixture`](../src/server/fixture_integration_test.go#L28) | test | 28-540 | `func TestGenerateFixture(t *testing.T)` | TestGenerateFixture creates a workspace fixture database at src/server/testdata/workspace.fixture.db. It is used by the dev server and by Playwright tests. Run it with: cd src && FORCE_FIXTURE=1 go test ./server -run TestGenerateFixture -count=1 The FORCE_FIXTURE variable is required when the fixture already exists; the test skips automatically when the output file is present. |
| [`sprintf`](../src/server/fixture_integration_test.go#L543) | function | 543-546 | `func sprintf(format string, args ...any) string` | sprintf is a convenience wrapper for fmt.Sprintf used in the fixture generator. |

### [`src/server/graph.go`](../src/server/graph.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).graph`](../src/server/graph.go#L24) | method | 24-93 | `func (*Server).graph(w http.ResponseWriter, r *http.Request)` | graph validates graph filters and returns a bounded relationship graph for one run. |
| [`(*Server).graphArticles`](../src/server/graph.go#L96) | method | 96-158 | `func (*Server).graphArticles(ctx context.Context, r *http.Request, runID int64, limit int) ([]map[string]any, int, error)` | graphArticles selects normalized, valid article nodes matching the request filters and limit. |
| [`(*Server).graphEdges`](../src/server/graph.go#L161) | method | 161-240 | `func (*Server).graphEdges(ctx context.Context, mode string, articles []map[string]any) ([]map[string]any, []map[string]any, bool, error)` | graphEdges builds bounded nodes and edges for one supported relationship mode. |
| [`(*Server).graphResearchNetwork`](../src/server/graph.go#L243) | method | 243-415 | `func (*Server).graphResearchNetwork(ctx context.Context, articles []map[string]any) ([]map[string]any, []map[string]any, bool, error)` | graphResearchNetwork combines authorship, reference, citation, coauthor, and bibliographic-coupling relationships. |
| [`placeholders`](../src/server/graph.go#L418) | function | 418-425 | `func placeholders(ids []int64) (string, []any)` | placeholders returns a comma-separated SQL placeholder list and matching identifier arguments. |
| [`graphFilters`](../src/server/graph.go#L428) | function | 428-436 | `func graphFilters(r *http.Request, runID int64, mode string, limit int) map[string]any` | graphFilters returns the effective non-empty graph filters for response metadata. |

### [`src/server/graph_integration_test.go`](../src/server/graph_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAPIDetailsGraphModes`](../src/server/graph_integration_test.go#L19) | test | 19-55 | `func TestAPIDetailsGraphModes(t *testing.T)` | TestAPIDetailsGraphModes verifies api details graph modes. |
| [`TestAPIGraphFiltersAndTruncation`](../src/server/graph_integration_test.go#L58) | test | 58-145 | `func TestAPIGraphFiltersAndTruncation(t *testing.T)` | TestAPIGraphFiltersAndTruncation verifies api graph filters and truncation. |

### [`src/server/helpers_test.go`](../src/server/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`pdfViewerFixture`](../src/server/helpers_test.go#L19) | struct | 19-26 | `type pdfViewerFixture struct { server *Server runID int64 availableID int64 notAvailableID int64 unavailableID int64 revisionID int64 }` | pdfViewerFixture is a fixture type used by the package test suite. |
| [`referenceResolutionFixture`](../src/server/helpers_test.go#L29) | struct | 29-36 | `type referenceResolutionFixture struct { path string citingRevisionID int64 externalMentionID int64 resolvedMentionID int64 normalizedTargetID int64 normalizedTargetTitle string }` | referenceResolutionFixture is a fixture type used by the package test suite. |
| [`articleActivityFixture`](../src/server/helpers_test.go#L39) | struct | 39-44 | `type articleActivityFixture struct { path string normalizedRevisionID int64 discardedRevisionID int64 discardedReason string }` | articleActivityFixture is a fixture type used by the package test suite. |
| [`viewerFixture`](../src/server/helpers_test.go#L47) | function | 47-137 | `func viewerFixture(t *testing.T) (string, int64, int64, int64)` | viewerFixture supports the package test suite's viewer fixture setup or assertions. |
| [`viewerReferenceResolutionFixture`](../src/server/helpers_test.go#L140) | function | 140-225 | `func viewerReferenceResolutionFixture(t *testing.T) referenceResolutionFixture` | viewerReferenceResolutionFixture supports the package test suite's viewer reference resolution fixture setup or assertions. |
| [`viewerArticleActivityFixture`](../src/server/helpers_test.go#L228) | function | 228-324 | `func viewerArticleActivityFixture(t *testing.T) articleActivityFixture` | viewerArticleActivityFixture supports the package test suite's viewer article activity fixture setup or assertions. |
| [`viewerRequest`](../src/server/helpers_test.go#L327) | function | 327-332 | `func viewerRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder` | viewerRequest supports the package test suite's viewer request setup or assertions. |
| [`requestJSON`](../src/server/helpers_test.go#L335) | function | 335-344 | `func requestJSON(t *testing.T, handler http.Handler, path string) (int, map[string]any)` | requestJSON supports the package test suite's request json setup or assertions. |
| [`newPDFViewerFixture`](../src/server/helpers_test.go#L347) | function | 347-426 | `func newPDFViewerFixture(t *testing.T) pdfViewerFixture` | newPDFViewerFixture supports the package test suite's new pdf viewer fixture setup or assertions. |

### [`src/server/identity_evidence.go`](../src/server/identity_evidence.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).runIdentityEvidence`](../src/server/identity_evidence.go#L15) | method | 15-81 | `func (*Server).runIdentityEvidence(w http.ResponseWriter, r *http.Request)` | runIdentityEvidence exposes name-derived ORCID evidence without presenting it as an author identity. The endpoint is unavailable for databases created before the evidence migration, preserving the viewer's read-only behavior. |
| [`(*Server).identityEvidenceStats`](../src/server/identity_evidence.go#L84) | method | 84-100 | `func (*Server).identityEvidenceStats(ctx context.Context, runID int64) (map[string]int64, error)` | identityEvidenceStats counts candidate and resolution states for the selected context. |
| [`(*Server).attachIdentityCandidates`](../src/server/identity_evidence.go#L103) | method | 103-122 | `func (*Server).attachIdentityCandidates(ctx context.Context, resolutions []map[string]any) error` | attachIdentityCandidates groups stored identity candidates under their resolution records. |

### [`src/server/identity_evidence_integration_test.go`](../src/server/identity_evidence_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestRunScopedIdentityEvidence`](../src/server/identity_evidence_integration_test.go#L14) | test | 14-27 | `func TestRunScopedIdentityEvidence(t *testing.T)` | TestRunScopedIdentityEvidence verifies run scoped identity evidence. |

### [`src/server/overview.go`](../src/server/overview.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).sourceResultCounts`](../src/server/overview.go#L52) | method | 52-69 | `func (*Server).sourceResultCounts(ctx context.Context, runID int64) ([]map[string]any, error)` | sourceResultCounts returns the stored source inventory and result-count evidence for a run. |
| [`(*Server).sourceFilterCounts`](../src/server/overview.go#L72) | method | 72-104 | `func (*Server).sourceFilterCounts(ctx context.Context, runID int64) ([]map[string]any, error)` | sourceFilterCounts decodes the stored per-source filter-stage counts for a run. |
| [`(*Server).health`](../src/server/overview.go#L107) | method | 107-112 | `func (*Server).health(w http.ResponseWriter, r *http.Request)` | health reports database readability and the discovered table inventory. |
| [`(*Server).tableNames`](../src/server/overview.go#L115) | method | 115-128 | `func (*Server).tableNames() []string` | tableNames returns discovered table names in deterministic order. |
| [`(*Server).searches`](../src/server/overview.go#L131) | method | 131-184 | `func (*Server).searches(w http.ResponseWriter, r *http.Request)` | searches returns searches with their immutable revisions. |
| [`(*Server).plans`](../src/server/overview.go#L187) | method | 187-205 | `func (*Server).plans(w http.ResponseWriter, r *http.Request)` | plans returns execution plans for the required search revision. |
| [`(*Server).runs`](../src/server/overview.go#L208) | method | 208-259 | `func (*Server).runs(w http.ResponseWriter, r *http.Request)` | runs returns pipeline attempts filtered by research context and visibility. |
| [`(*Server).overview`](../src/server/overview.go#L262) | method | 262-364 | `func (*Server).overview(w http.ResponseWriter, r *http.Request)` | overview returns captured metrics, coverage, relationships, and source evidence for a run. |
| [`metricGroup`](../src/server/overview.go#L367) | function | 367-377 | `func metricGroup(metrics map[string]map[string]any, names ...string) map[string]any` | metricGroup selects named metrics and marks absent captures as unavailable. |
| [`sourceBreakdown`](../src/server/overview.go#L380) | function | 380-396 | `func sourceBreakdown(metrics []map[string]any, totals map[string]int64) map[string]any` | sourceBreakdown calculates each source's share of captured input records. |
| [`enrichmentFieldBreakdown`](../src/server/overview.go#L401) | function | 401-415 | `func enrichmentFieldBreakdown(byName map[string]map[string]any) map[string]any` | enrichmentFieldBreakdown extracts per-field enrichment counts from the byName metric map. Metrics named "enriched_fields_<field>" with no source are included. |
| [`enrichmentProviderBreakdown`](../src/server/overview.go#L420) | function | 420-431 | `func enrichmentProviderBreakdown(metrics []map[string]any) map[string]any` | enrichmentProviderBreakdown extracts per-provider enrichment counts from the raw metrics list. Metrics named "enriched_fields" with a non-empty source are included. |
| [`normalizationFieldBreakdown`](../src/server/overview.go#L434) | function | 434-476 | `func normalizationFieldBreakdown(metrics []map[string]any) map[string]map[string]any` | normalizationFieldBreakdown groups normalization outcome metrics by field and derives percentages. |
| [`metricDenominator`](../src/server/overview.go#L479) | function | 479-491 | `func metricDenominator(metric string, values map[string]int64) (int64, bool)` | metricDenominator returns the captured population against which a metric is measured. |
| [`(*Server).currentCoverage`](../src/server/overview.go#L494) | method | 494-503 | `func (*Server).currentCoverage(ctx context.Context, runID int64) (map[string]any, error)` | currentCoverage returns work-revision and journal coverage for a run. |
| [`(*Server).relationshipTotals`](../src/server/overview.go#L506) | method | 506-522 | `func (*Server).relationshipTotals(ctx context.Context, runID int64) (map[string]int64, error)` | relationshipTotals counts canonical works, authorships, references, and resolved citations for a run. |
| [`percent`](../src/server/overview.go#L525) | function | 525-531 | `func percent(value, denominator int64) *float64` | percent returns value as a percentage of denominator, or nil when denominator is zero. |
| [`requiredQueryID`](../src/server/overview.go#L534) | function | 534-543 | `func requiredQueryID(r *http.Request, name string) (int64, error)` | requiredQueryID validates the endpoint query allowlist and returns one required positive identifier. |
| [`validateKnownQuery`](../src/server/overview.go#L546) | function | 546-560 | `func validateKnownQuery(r *http.Request, allowed ...string) error` | validateKnownQuery rejects semicolon syntax and query parameters outside the endpoint allowlist. |
| [`parseOptionalInt`](../src/server/overview.go#L563) | function | 563-572 | `func parseOptionalInt(raw, name string) (int64, error)` | parseOptionalInt parses a named decimal query value for an endpoint diagnostic. |

### [`src/server/overview_integration_test.go`](../src/server/overview_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestOverviewReportsUnavailableMetricsAndFrontendContract`](../src/server/overview_integration_test.go#L18) | test | 18-104 | `func TestOverviewReportsUnavailableMetricsAndFrontendContract(t *testing.T)` | TestOverviewReportsUnavailableMetricsAndFrontendContract verifies overview reports unavailable metrics and frontend contract. |
| [`TestOverviewSupportsPreResultCountRunSources`](../src/server/overview_integration_test.go#L107) | test | 107-141 | `func TestOverviewSupportsPreResultCountRunSources(t *testing.T)` | TestOverviewSupportsPreResultCountRunSources verifies overview supports pre result count run sources. |

### [`src/server/pdf.go`](../src/server/pdf.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).workPDFStatus`](../src/server/pdf.go#L17) | method | 17-27 | `func (*Server).workPDFStatus(w http.ResponseWriter, r *http.Request)` | workPDFStatus returns normalized DOI inventory status for the requested work. |
| [`(*Server).pdfStatusForWork`](../src/server/pdf.go#L30) | method | 30-68 | `func (*Server).pdfStatusForWork(ctx context.Context, workID int64) (map[string]any, error)` | pdfStatusForWork reads companion PDF availability metadata for one work revision. |
| [`nullableValue`](../src/server/pdf.go#L71) | function | 71-76 | `func nullableValue(value sql.NullString) any` | nullableValue converts a nullable SQL string to either its value or nil. |
| [`(*Server).workPDF`](../src/server/pdf.go#L79) | method | 79-117 | `func (*Server).workPDF(w http.ResponseWriter, r *http.Request)` | workPDF streams the validated PDF associated with the requested work revision. |

### [`src/server/pdf_integration_test.go`](../src/server/pdf_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestPDFStatusEndpointsCoverEveryState`](../src/server/pdf_integration_test.go#L18) | test | 18-39 | `func TestPDFStatusEndpointsCoverEveryState(t *testing.T)` | TestPDFStatusEndpointsCoverEveryState verifies pdf status endpoints cover every state. |
| [`TestPDFContentSupportsInlineRanges`](../src/server/pdf_integration_test.go#L42) | test | 42-63 | `func TestPDFContentSupportsInlineRanges(t *testing.T)` | TestPDFContentSupportsInlineRanges verifies pdf content supports inline ranges. |
| [`TestArticleIncludesManualPDFAuditEvent`](../src/server/pdf_integration_test.go#L66) | test | 66-90 | `func TestArticleIncludesManualPDFAuditEvent(t *testing.T)` | TestArticleIncludesManualPDFAuditEvent verifies article includes manual pdf audit event. |
| [`TestViewerPDFConnectionIsReadOnly`](../src/server/pdf_integration_test.go#L93) | test | 93-99 | `func TestViewerPDFConnectionIsReadOnly(t *testing.T)` | TestViewerPDFConnectionIsReadOnly verifies viewer pdf connection is read only. |

### [`src/server/server.go`](../src/server/server.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Server`](../src/server/server.go#L36) | struct | 36-42 | `type Server struct { db *sql.DB pdfDB *sql.DB pdfPath string tables map[string]tableInfo AssetsFS fs.FS // if non-nil, serves frontend assets from this filesystem }` | Server serves one existing workspace database. db is opened in SQLite's read-only mode and query_only is set for every connection. |
| [`tableInfo`](../src/server/server.go#L45) | struct | 45-49 | ``type tableInfo struct { Name string `json:"name"` Columns []columnInfo `json:"columns"` Count int64 `json:"row_count"` }`` | tableInfo stores the discovered columns for one browsable SQLite table. |
| [`columnInfo`](../src/server/server.go#L52) | struct | 52-56 | ``type columnInfo struct { Name string `json:"name"` Type string `json:"type"` PrimaryKey bool `json:"primary_key"` }`` | columnInfo records a SQLite column's name, declared type, and primary-key position. |
| [`(*Server).tableHasColumns`](../src/server/server.go#L59) | method | 59-74 | `func (*Server).tableHasColumns(table string, required ...string) bool` | tableHasColumns reports whether a discovered table contains every requested column. |
| [`Open`](../src/server/server.go#L77) | function | 77-118 | `func Open(path string) (*Server, error)` | Open opens an existing database without creating it or modifying it. |
| [`(*Server).Close`](../src/server/server.go#L121) | method | 121-129 | `func (*Server).Close() error` | Close releases resources owned by the receiver. |
| [`(*Server).PDFStoreBound`](../src/server/server.go#L132) | method | 132 | `func (*Server).PDFStoreBound() bool` | PDFStoreBound reports whether a readable companion PDF database is attached. |
| [`(*Server).Handler`](../src/server/server.go#L135) | method | 135-170 | `func (*Server).Handler() http.Handler` | Handler returns the local API and embedded frontend handler. |
| [`(*Server).openBoundPDFStore`](../src/server/server.go#L173) | method | 173-243 | `func (*Server).openBoundPDFStore(ctx context.Context, metadataDir string) error` | openBoundPDFStore resolves and opens the companion PDF database declared by metadata. |
| [`pdfTableColumns`](../src/server/server.go#L246) | function | 246-266 | `func pdfTableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error)` | pdfTableColumns returns the discovered columns for a companion PDF table. |
| [`(*Server).HTTPServer`](../src/server/server.go#L269) | method | 269-278 | `func (*Server).HTTPServer(addr string) *http.Server` | HTTPServer returns a conservatively configured local HTTP server. |
| [`(*Server).discoverTables`](../src/server/server.go#L281) | method | 281-316 | `func (*Server).discoverTables(ctx context.Context) error` | discoverTables reads the SQLite schema and returns tables eligible for read-only browsing. |
| [`(*Server).columns`](../src/server/server.go#L319) | method | 319-337 | `func (*Server).columns(ctx context.Context, table string) ([]columnInfo, error)` | columns returns ordered metadata for the requested table's columns. |
| [`quoteIdentifier`](../src/server/server.go#L340) | function | 340-342 | `func quoteIdentifier(identifier string) string` | quoteIdentifier quotes a validated SQLite identifier and escapes embedded quotes. |
| [`(*Server).hasTable`](../src/server/server.go#L345) | method | 345 | `func (*Server).hasTable(name string) bool` | hasTable reports whether a table was discovered as browsable. |
| [`(*Server).hasColumn`](../src/server/server.go#L348) | method | 348-359 | `func (*Server).hasColumn(table, column string) bool` | hasColumn reports whether a discovered table contains a named column. |
| [`apiError`](../src/server/server.go#L362) | struct | 362-367 | ``type apiError struct { Error struct { Code string `json:"code"` Message string `json:"message"` } `json:"error"` }`` | apiError is the stable JSON envelope returned for client-visible failures. |
| [`apiProblem`](../src/server/server.go#L370) | struct | 370-373 | `type apiProblem struct { Code, Message string Status int }` | apiProblem carries an HTTP status and safe client-facing error message. |
| [`(*apiProblem).Error`](../src/server/server.go#L376) | method | 376 | `func (*apiProblem).Error() string` | Error returns the receiver's diagnostic message. |
| [`badRequest`](../src/server/server.go#L379) | function | 379-381 | `func badRequest(message string) error` | badRequest constructs an API problem with HTTP status 400. |
| [`notFound`](../src/server/server.go#L384) | function | 384-386 | `func notFound(message string) error` | notFound constructs an API problem with HTTP status 404. |
| [`withJSONErrors`](../src/server/server.go#L389) | function | 389-397 | `func withJSONErrors(next http.Handler) http.Handler` | withJSONErrors converts handler-returned errors into the server's JSON error response. |
| [`writeJSON`](../src/server/server.go#L400) | function | 400-404 | `func writeJSON(w http.ResponseWriter, status int, value any)` | writeJSON writes a JSON response with the supplied HTTP status. |
| [`writeError`](../src/server/server.go#L407) | function | 407-412 | `func writeError(w http.ResponseWriter, status int, code, message string)` | writeError writes the stable JSON error envelope. |
| [`(*Server).respond`](../src/server/server.go#L415) | method | 415-427 | `func (*Server).respond(w http.ResponseWriter, r *http.Request, value any, err error)` | respond maps successful values, client problems, and internal failures to safe JSON responses. |
| [`queryContext`](../src/server/server.go#L430) | function | 430-432 | `func queryContext(r *http.Request) (context.Context, context.CancelFunc)` | queryContext derives a request context bounded by the server query timeout. |
| [`positiveID`](../src/server/server.go#L435) | function | 435-441 | `func positiveID(raw string) (int64, error)` | positiveID parses a strictly positive decimal identifier or returns a bad-request problem. |
| [`rowsAsMaps`](../src/server/server.go#L444) | function | 444-470 | `func rowsAsMaps(rows *sql.Rows) ([]map[string]any, error)` | rowsAsMaps scans SQL rows into maps keyed by result-column name. |

### [`src/server/server_integration_test.go`](../src/server/server_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestOpenIsReadOnlyAndDoesNotCreateMissingDatabase`](../src/server/server_integration_test.go#L18) | test | 18-39 | `func TestOpenIsReadOnlyAndDoesNotCreateMissingDatabase(t *testing.T)` | TestOpenIsReadOnlyAndDoesNotCreateMissingDatabase verifies open is read only and does not create missing database. |
| [`TestAPIWorkspaceDiscoveryAndSafePagination`](../src/server/server_integration_test.go#L42) | test | 42-65 | `func TestAPIWorkspaceDiscoveryAndSafePagination(t *testing.T)` | TestAPIWorkspaceDiscoveryAndSafePagination verifies api workspace discovery and safe pagination. |
| [`TestEmbeddedFrontendResearchWorkspaceContract`](../src/server/server_integration_test.go#L68) | test | 68-116 | `func TestEmbeddedFrontendResearchWorkspaceContract(t *testing.T)` | TestEmbeddedFrontendResearchWorkspaceContract verifies embedded frontend research workspace contract. |
| [`TestHandlerServesFilesystemAssets`](../src/server/server_integration_test.go#L119) | test | 119-136 | `func TestHandlerServesFilesystemAssets(t *testing.T)` | TestHandlerServesFilesystemAssets verifies handler serves filesystem assets. |

### [`src/server/server_unit_test.go`](../src/server/server_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestHelper_quoteIdentifier`](../src/server/server_unit_test.go#L16) | test | 16-38 | `func TestHelper_quoteIdentifier(t *testing.T)` | TestHelper_quoteIdentifier verifies helper quote identifier. |
| [`TestHelper_positiveID`](../src/server/server_unit_test.go#L41) | test | 41-85 | `func TestHelper_positiveID(t *testing.T)` | TestHelper_positiveID verifies helper positive id. |
| [`TestHelper_stringID`](../src/server/server_unit_test.go#L88) | test | 88-105 | `func TestHelper_stringID(t *testing.T)` | TestHelper_stringID verifies helper string id. |
| [`TestHelper_nullableValue`](../src/server/server_unit_test.go#L108) | test | 108-128 | `func TestHelper_nullableValue(t *testing.T)` | TestHelper_nullableValue verifies helper nullable value. |
| [`TestHelper_badRequest`](../src/server/server_unit_test.go#L131) | test | 131-149 | `func TestHelper_badRequest(t *testing.T)` | TestHelper_badRequest verifies helper bad request. |
| [`TestHelper_notFound`](../src/server/server_unit_test.go#L152) | test | 152-167 | `func TestHelper_notFound(t *testing.T)` | TestHelper_notFound verifies helper not found. |
| [`TestHelper_metricGroup`](../src/server/server_unit_test.go#L170) | test | 170-192 | `func TestHelper_metricGroup(t *testing.T)` | TestHelper_metricGroup verifies helper metric group. |
| [`TestHelper_sourceBreakdown`](../src/server/server_unit_test.go#L195) | test | 195-221 | `func TestHelper_sourceBreakdown(t *testing.T)` | TestHelper_sourceBreakdown verifies helper source breakdown. |
| [`TestHelper_sourceBreakdown_noDenominator`](../src/server/server_unit_test.go#L224) | test | 224-236 | `func TestHelper_sourceBreakdown_noDenominator(t *testing.T)` | TestHelper_sourceBreakdown_noDenominator verifies helper source breakdown no denominator. |
| [`TestHelper_enrichmentFieldBreakdown`](../src/server/server_unit_test.go#L239) | test | 239-266 | `func TestHelper_enrichmentFieldBreakdown(t *testing.T)` | TestHelper_enrichmentFieldBreakdown verifies helper enrichment field breakdown. |
| [`TestHelper_enrichmentProviderBreakdown`](../src/server/server_unit_test.go#L269) | test | 269-291 | `func TestHelper_enrichmentProviderBreakdown(t *testing.T)` | TestHelper_enrichmentProviderBreakdown verifies helper enrichment provider breakdown. |
| [`TestHelper_normalizationFieldBreakdown`](../src/server/server_unit_test.go#L294) | test | 294-333 | `func TestHelper_normalizationFieldBreakdown(t *testing.T)` | TestHelper_normalizationFieldBreakdown verifies helper normalization field breakdown. |
| [`TestHelper_normalizationFieldBreakdown_emptyProcessed`](../src/server/server_unit_test.go#L336) | test | 336-355 | `func TestHelper_normalizationFieldBreakdown_emptyProcessed(t *testing.T)` | TestHelper_normalizationFieldBreakdown_emptyProcessed verifies helper normalization field breakdown empty processed. |
| [`TestHelper_normalizationFieldBreakdown_skipsMissingStatuses`](../src/server/server_unit_test.go#L358) | test | 358-369 | `func TestHelper_normalizationFieldBreakdown_skipsMissingStatuses(t *testing.T)` | TestHelper_normalizationFieldBreakdown_skipsMissingStatuses verifies helper normalization field breakdown skips missing statuses. |
| [`TestHelper_metricDenominator`](../src/server/server_unit_test.go#L372) | test | 372-406 | `func TestHelper_metricDenominator(t *testing.T)` | TestHelper_metricDenominator verifies helper metric denominator. |
| [`TestHelper_metricDenominator_zeroThreshold`](../src/server/server_unit_test.go#L409) | test | 409-421 | `func TestHelper_metricDenominator_zeroThreshold(t *testing.T)` | TestHelper_metricDenominator_zeroThreshold verifies helper metric denominator zero threshold. |
| [`TestHelper_percent`](../src/server/server_unit_test.go#L424) | test | 424-452 | `func TestHelper_percent(t *testing.T)` | TestHelper_percent verifies helper percent. |
| [`TestHelper_normalizedArtifactContentType`](../src/server/server_unit_test.go#L455) | test | 455-475 | `func TestHelper_normalizedArtifactContentType(t *testing.T)` | TestHelper_normalizedArtifactContentType verifies helper normalized artifact content type. |
| [`TestHelper_jsonArtifactContentType`](../src/server/server_unit_test.go#L478) | test | 478-496 | `func TestHelper_jsonArtifactContentType(t *testing.T)` | TestHelper_jsonArtifactContentType verifies helper json artifact content type. |
| [`TestHelper_inlineArtifactContentType`](../src/server/server_unit_test.go#L499) | test | 499-522 | `func TestHelper_inlineArtifactContentType(t *testing.T)` | TestHelper_inlineArtifactContentType verifies helper inline artifact content type. |
| [`TestHelper_auditMultiValues`](../src/server/server_unit_test.go#L525) | test | 525-609 | `func TestHelper_auditMultiValues(t *testing.T)` | TestHelper_auditMultiValues verifies helper audit multi values. |
| [`TestHelper_auditInClause`](../src/server/server_unit_test.go#L612) | test | 612-645 | `func TestHelper_auditInClause(t *testing.T)` | TestHelper_auditInClause verifies helper audit in clause. |
| [`TestHelper_auditWhere`](../src/server/server_unit_test.go#L648) | test | 648-665 | `func TestHelper_auditWhere(t *testing.T)` | TestHelper_auditWhere verifies helper audit where. |
| [`TestHelper_corpusSelectColumns`](../src/server/server_unit_test.go#L668) | test | 668-686 | `func TestHelper_corpusSelectColumns(t *testing.T)` | TestHelper_corpusSelectColumns verifies helper corpus select columns. |
| [`TestHelper_scopedPagination`](../src/server/server_unit_test.go#L689) | test | 689-732 | `func TestHelper_scopedPagination(t *testing.T)` | TestHelper_scopedPagination verifies helper scoped pagination. |
| [`TestHelper_placeholders`](../src/server/server_unit_test.go#L735) | test | 735-768 | `func TestHelper_placeholders(t *testing.T)` | TestHelper_placeholders verifies helper placeholders. |
| [`TestHelper_parseOptionalInt`](../src/server/server_unit_test.go#L771) | test | 771-823 | `func TestHelper_parseOptionalInt(t *testing.T)` | TestHelper_parseOptionalInt verifies helper parse optional int. |
| [`ptr`](../src/server/server_unit_test.go#L826) | function | 826-828 | `func ptr(v float64) *float64` | ptr supports the package test suite's ptr setup or assertions. |

### [`src/server/tables.go`](../src/server/tables.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`(*Server).tablesHandler`](../src/server/tables.go#L16) | method | 16-26 | `func (*Server).tablesHandler(w http.ResponseWriter, r *http.Request)` | tablesHandler returns metadata for every discovered browsable table. |
| [`(*Server).tableRows`](../src/server/tables.go#L29) | method | 29-61 | `func (*Server).tableRows(w http.ResponseWriter, r *http.Request)` | tableRows returns a bounded page from one validated browsable table. |
| [`tableRequest`](../src/server/tables.go#L64) | function | 64-102 | `func tableRequest(r *http.Request, info tableInfo) (int, int, string, string, error)` | tableRequest parses the requested table name from the route path. |

### [`src/server/tables_integration_test.go`](../src/server/tables_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAPITableBrowserErrors`](../src/server/tables_integration_test.go#L15) | test | 15-32 | `func TestAPITableBrowserErrors(t *testing.T)` | TestAPITableBrowserErrors verifies api table browser errors. |

### [`src/something/api.go`](../src/something/api.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`LoadSomethingFile`](../src/something/api.go#L16) | function | 16-22 | `func LoadSomethingFile(filepath string) (result map[string]any, err error)` | LoadSomethingFile loads, parses, and evaluates a .something file. Returns the evaluated config map. Only variables not marked with `#priv` are included in the result. |
| [`LoadSomethingBytes`](../src/something/api.go#L27) | function | 27-48 | `func LoadSomethingBytes(data []byte, filepath string) (result map[string]any, err error)` | LoadSomethingBytes compiles and evaluates one already-read SOMETHING file. It allows callers that must retain an immutable source snapshot to evaluate exactly those bytes rather than reading a mutable path a second time. |
| [`Pprint`](../src/something/api.go#L51) | function | 51-89 | `func Pprint(v any, indent int) string` | Pprint pretty-prints a resolved SOMETHING value. |

### [`src/something/api_functional_test.go`](../src/something/api_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestParseIncludeValue_Functional`](../src/something/api_functional_test.go#L16) | test | 16-36 | `func TestParseIncludeValue_Functional(t *testing.T)` | TestParseIncludeValue_Functional verifies parse include value functional. |
| [`TestLoadSomethingFileError_Functional`](../src/something/api_functional_test.go#L39) | test | 39-51 | `func TestLoadSomethingFileError_Functional(t *testing.T)` | TestLoadSomethingFileError_Functional verifies load something file error functional. |
| [`TestEvalIncludeFromFile_Functional`](../src/something/api_functional_test.go#L54) | test | 54-67 | `func TestEvalIncludeFromFile_Functional(t *testing.T)` | TestEvalIncludeFromFile_Functional verifies eval include from file functional. |
| [`TestEvalIncludeDedup_Functional`](../src/something/api_functional_test.go#L70) | test | 70-83 | `func TestEvalIncludeDedup_Functional(t *testing.T)` | TestEvalIncludeDedup_Functional verifies eval include dedup functional. |
| [`TestEvalIncludeNotFound_Functional`](../src/something/api_functional_test.go#L86) | test | 86-94 | `func TestEvalIncludeNotFound_Functional(t *testing.T)` | TestEvalIncludeNotFound_Functional verifies eval include not found functional. |
| [`TestEvalIncludeInScope_Functional`](../src/something/api_functional_test.go#L97) | test | 97-116 | `func TestEvalIncludeInScope_Functional(t *testing.T)` | TestEvalIncludeInScope_Functional verifies eval include in scope functional. |
| [`TestEvalScopeBodyIncludeNamespace_Functional`](../src/something/api_functional_test.go#L119) | test | 119-133 | `func TestEvalScopeBodyIncludeNamespace_Functional(t *testing.T)` | TestEvalScopeBodyIncludeNamespace_Functional verifies eval scope body include namespace functional. |

### [`src/something/api_unit_test.go`](../src/something/api_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvalPprint`](../src/something/api_unit_test.go#L14) | test | 14-37 | `func TestEvalPprint(t *testing.T)` | TestEvalPprint verifies eval pprint. |
| [`TestEvalPprintMapWithItems`](../src/something/api_unit_test.go#L40) | test | 40-45 | `func TestEvalPprintMapWithItems(t *testing.T)` | TestEvalPprintMapWithItems verifies eval pprint map with items. |
| [`TestEvalPprintArrayWithItems`](../src/something/api_unit_test.go#L48) | test | 48-53 | `func TestEvalPprintArrayWithItems(t *testing.T)` | TestEvalPprintArrayWithItems verifies eval pprint array with items. |
| [`TestEvalPprintDefault`](../src/something/api_unit_test.go#L56) | test | 56-62 | `func TestEvalPprintDefault(t *testing.T)` | TestEvalPprintDefault verifies eval pprint default. |

### [`src/something/ast.go`](../src/something/ast.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`PrimitiveKind`](../src/something/ast.go#L6) | type | 6 | `type PrimitiveKind int` | PrimitiveKind represents a built-in value type. |
| [`(PrimitiveKind).String`](../src/something/ast.go#L19) | method | 19-38 | `func (PrimitiveKind).String() string` | String returns the receiver's textual representation. |
| [`TypeRef`](../src/something/ast.go#L41) | interface | 41-43 | `type TypeRef interface { typeRefMarker() }` | TypeRef is a syntactic or resolved SOMETHING type. |
| [`(PrimitiveKind).typeRefMarker`](../src/something/ast.go#L46) | method | 46 | `func (PrimitiveKind).typeRefMarker()` | typeRefMarker marks PrimitiveKind as a TypeRef implementation. |
| [`(TypeName).typeRefMarker`](../src/something/ast.go#L49) | method | 49 | `func (TypeName).typeRefMarker()` | typeRefMarker marks TypeName as a TypeRef implementation. |
| [`(*MappingType).typeRefMarker`](../src/something/ast.go#L52) | method | 52 | `func (*MappingType).typeRefMarker()` | typeRefMarker marks MappingType as a TypeRef implementation. |
| [`(*ArrayType).typeRefMarker`](../src/something/ast.go#L55) | method | 55 | `func (*ArrayType).typeRefMarker()` | typeRefMarker marks ArrayType as a TypeRef implementation. |
| [`(*EnumKeyType).typeRefMarker`](../src/something/ast.go#L58) | method | 58 | `func (*EnumKeyType).typeRefMarker()` | typeRefMarker marks EnumKeyType as a TypeRef implementation. |
| [`TypeName`](../src/something/ast.go#L61) | type | 61 | `type TypeName string` | TypeName is an unresolved named type reference. |
| [`MappingType`](../src/something/ast.go#L64) | struct | 64-67 | `type MappingType struct { KeyType TypeRef ValueType TypeRef }` | MappingType defines mapping key and value types. |
| [`ArrayType`](../src/something/ast.go#L70) | struct | 70-72 | `type ArrayType struct { ElementType TypeRef }` | ArrayType defines an integer-indexed array. |
| [`EnumKeyType`](../src/something/ast.go#L75) | struct | 75-78 | `type EnumKeyType struct { EnumName string ElementType TypeRef }` | EnumKeyType defines an array indexed by members of a named enum. |
| [`Program`](../src/something/ast.go#L81) | struct | 81-84 | `type Program struct { Statements []Statement Filepath string }` | Program is an ordered syntax or expanded AST. Statement order is semantic. |
| [`Statement`](../src/something/ast.go#L87) | interface | 87-90 | `type Statement interface { statementMarker() statementBase() *StatementBase }` | Statement is a source-ordered AST statement. |
| [`StatementBase`](../src/something/ast.go#L93) | struct | 93-96 | `type StatementBase struct { Private bool Location *SourceLocation }` | StatementBase contains metadata shared by every statement. |
| [`AssignmentMode`](../src/something/ast.go#L99) | type | 99 | `type AssignmentMode int` | AssignmentMode distinguishes declaration, inference, and reassignment. |
| [`Assignment`](../src/something/ast.go#L109) | struct | 109-115 | `type Assignment struct { StatementBase Target LValue Mode AssignmentMode DeclaredType TypeRef Value AssignmentValue }` | Assignment is the common representation for every value or type binding. Directives may appear in its target or value before directive expansion. |
| [`(*Assignment).statementMarker`](../src/something/ast.go#L118) | method | 118 | `func (*Assignment).statementMarker()` | statementMarker marks Assignment as a Statement implementation. |
| [`(*Assignment).statementBase`](../src/something/ast.go#L121) | method | 121 | `func (*Assignment).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`LValue`](../src/something/ast.go#L124) | interface | 124-126 | `type LValue interface { lvalueMarker() }` | LValue is a syntactic assignment destination. |
| [`IdentifierLValue`](../src/something/ast.go#L129) | struct | 129-131 | `type IdentifierLValue struct { Name string }` | IdentifierLValue names a binding in the current scope. |
| [`MemberLValue`](../src/something/ast.go#L134) | struct | 134-137 | `type MemberLValue struct { Root string Accesses []Access }` | MemberLValue identifies a field or indexed member below a root binding. |
| [`IterationLValue`](../src/something/ast.go#L140) | struct | 140-142 | `type IterationLValue struct { Label Expression }` | IterationLValue is replaced with an IdentifierLValue during expansion. |
| [`AsLValue`](../src/something/ast.go#L145) | struct | 145-147 | `type AsLValue struct { Name Expression }` | AsLValue is replaced with an identifier or member target during expansion. |
| [`(*IdentifierLValue).lvalueMarker`](../src/something/ast.go#L150) | method | 150 | `func (*IdentifierLValue).lvalueMarker()` | lvalueMarker marks IdentifierLValue as an LValue implementation. |
| [`(*MemberLValue).lvalueMarker`](../src/something/ast.go#L153) | method | 153 | `func (*MemberLValue).lvalueMarker()` | lvalueMarker marks MemberLValue as an LValue implementation. |
| [`(*IterationLValue).lvalueMarker`](../src/something/ast.go#L156) | method | 156 | `func (*IterationLValue).lvalueMarker()` | lvalueMarker marks IterationLValue as an LValue implementation. |
| [`(*AsLValue).lvalueMarker`](../src/something/ast.go#L159) | method | 159 | `func (*AsLValue).lvalueMarker()` | lvalueMarker marks AsLValue as an LValue implementation. |
| [`Access`](../src/something/ast.go#L162) | interface | 162-164 | `type Access interface { accessMarker() }` | Access is one field or index operation in a reference or lvalue. |
| [`FieldAccess`](../src/something/ast.go#L167) | struct | 167-170 | `type FieldAccess struct { Name string Location *SourceLocation }` | FieldAccess represents a field access AST node. |
| [`IndexAccess`](../src/something/ast.go#L173) | struct | 173-176 | `type IndexAccess struct { Index Expression Location *SourceLocation }` | IndexAccess represents an index access AST node. |
| [`(*FieldAccess).accessMarker`](../src/something/ast.go#L179) | method | 179 | `func (*FieldAccess).accessMarker()` | accessMarker marks FieldAccess as an Access implementation. |
| [`(*IndexAccess).accessMarker`](../src/something/ast.go#L182) | method | 182 | `func (*IndexAccess).accessMarker()` | accessMarker marks IndexAccess as an Access implementation. |
| [`AssignmentValue`](../src/something/ast.go#L185) | interface | 185-187 | `type AssignmentValue interface { assignmentValueMarker() }` | AssignmentValue is an expression, scope, or type definition on an assignment. |
| [`Expression`](../src/something/ast.go#L190) | interface | 190-194 | `type Expression interface { AssignmentValue expressionMarker() expressionLocation() *SourceLocation }` | Expression is an unevaluated value expression. |
| [`StringExpression`](../src/something/ast.go#L197) | struct | 197-201 | `type StringExpression struct { Literal *StringLiteral Multiline string Location *SourceLocation }` | StringExpression represents a string expression AST node. |
| [`IntegerExpression`](../src/something/ast.go#L204) | struct | 204-207 | `type IntegerExpression struct { Value int Location *SourceLocation }` | IntegerExpression represents an integer expression AST node. |
| [`FloatExpression`](../src/something/ast.go#L210) | struct | 210-213 | `type FloatExpression struct { Value float64 Location *SourceLocation }` | FloatExpression represents a float expression AST node. |
| [`BooleanExpression`](../src/something/ast.go#L216) | struct | 216-219 | `type BooleanExpression struct { Value bool Location *SourceLocation }` | BooleanExpression represents a boolean expression AST node. |
| [`ReferenceExpression`](../src/something/ast.go#L222) | struct | 222-226 | `type ReferenceExpression struct { Root string Accesses []Access Location *SourceLocation }` | ReferenceExpression represents a reference expression AST node. |
| [`ArrayExpression`](../src/something/ast.go#L229) | struct | 229-233 | `type ArrayExpression struct { DeclaredType TypeRef Elements []Expression Location *SourceLocation }` | ArrayExpression represents an array expression AST node. |
| [`MappingExpression`](../src/something/ast.go#L236) | struct | 236-240 | `type MappingExpression struct { DeclaredType *MappingType Entries []*MappingEntry Location *SourceLocation }` | MappingExpression represents a mapping expression AST node. |
| [`StructExpression`](../src/something/ast.go#L243) | struct | 243-247 | `type StructExpression struct { TypeName string Fields []*FieldAssignment Location *SourceLocation }` | StructExpression represents a struct expression AST node. |
| [`IncludeExpression`](../src/something/ast.go#L250) | struct | 250-253 | `type IncludeExpression struct { Filepath string Location *SourceLocation }` | IncludeExpression represents an include expression AST node. |
| [`IterationExpression`](../src/something/ast.go#L256) | struct | 256-259 | `type IterationExpression struct { Label Expression Location *SourceLocation }` | IterationExpression represents an iteration expression AST node. |
| [`MacroCallExpression`](../src/something/ast.go#L262) | struct | 262-266 | `type MacroCallExpression struct { Name string Arguments []Expression Location *SourceLocation }` | MacroCallExpression represents a macro call expression AST node. |
| [`NamespaceExpression`](../src/something/ast.go#L269) | struct | 269-272 | `type NamespaceExpression struct { Statements []Statement Location *SourceLocation }` | NamespaceExpression represents a namespace expression AST node. |
| [`TypedExpression`](../src/something/ast.go#L276) | struct | 276-280 | `type TypedExpression struct { Value Expression Type TypeRef Location *SourceLocation }` | TypedExpression preserves a directive's checked result type after its compile-time value has been converted back into an AST expression. |
| [`(*StringExpression).assignmentValueMarker`](../src/something/ast.go#L283) | method | 283 | `func (*StringExpression).assignmentValueMarker()` | assignmentValueMarker marks StringExpression as an AssignmentValue implementation. |
| [`(*IntegerExpression).assignmentValueMarker`](../src/something/ast.go#L286) | method | 286 | `func (*IntegerExpression).assignmentValueMarker()` | assignmentValueMarker marks IntegerExpression as an AssignmentValue implementation. |
| [`(*FloatExpression).assignmentValueMarker`](../src/something/ast.go#L289) | method | 289 | `func (*FloatExpression).assignmentValueMarker()` | assignmentValueMarker marks FloatExpression as an AssignmentValue implementation. |
| [`(*BooleanExpression).assignmentValueMarker`](../src/something/ast.go#L292) | method | 292 | `func (*BooleanExpression).assignmentValueMarker()` | assignmentValueMarker marks BooleanExpression as an AssignmentValue implementation. |
| [`(*ReferenceExpression).assignmentValueMarker`](../src/something/ast.go#L295) | method | 295 | `func (*ReferenceExpression).assignmentValueMarker()` | assignmentValueMarker marks ReferenceExpression as an AssignmentValue implementation. |
| [`(*ArrayExpression).assignmentValueMarker`](../src/something/ast.go#L298) | method | 298 | `func (*ArrayExpression).assignmentValueMarker()` | assignmentValueMarker marks ArrayExpression as an AssignmentValue implementation. |
| [`(*MappingExpression).assignmentValueMarker`](../src/something/ast.go#L301) | method | 301 | `func (*MappingExpression).assignmentValueMarker()` | assignmentValueMarker marks MappingExpression as an AssignmentValue implementation. |
| [`(*StructExpression).assignmentValueMarker`](../src/something/ast.go#L304) | method | 304 | `func (*StructExpression).assignmentValueMarker()` | assignmentValueMarker marks StructExpression as an AssignmentValue implementation. |
| [`(*IncludeExpression).assignmentValueMarker`](../src/something/ast.go#L307) | method | 307 | `func (*IncludeExpression).assignmentValueMarker()` | assignmentValueMarker marks IncludeExpression as an AssignmentValue implementation. |
| [`(*IterationExpression).assignmentValueMarker`](../src/something/ast.go#L310) | method | 310 | `func (*IterationExpression).assignmentValueMarker()` | assignmentValueMarker marks IterationExpression as an AssignmentValue implementation. |
| [`(*MacroCallExpression).assignmentValueMarker`](../src/something/ast.go#L313) | method | 313 | `func (*MacroCallExpression).assignmentValueMarker()` | assignmentValueMarker marks MacroCallExpression as an AssignmentValue implementation. |
| [`(*NamespaceExpression).assignmentValueMarker`](../src/something/ast.go#L316) | method | 316 | `func (*NamespaceExpression).assignmentValueMarker()` | assignmentValueMarker marks NamespaceExpression as an AssignmentValue implementation. |
| [`(*TypedExpression).assignmentValueMarker`](../src/something/ast.go#L319) | method | 319 | `func (*TypedExpression).assignmentValueMarker()` | assignmentValueMarker marks TypedExpression as an AssignmentValue implementation. |
| [`(*StringExpression).expressionMarker`](../src/something/ast.go#L322) | method | 322 | `func (*StringExpression).expressionMarker()` | expressionMarker marks StringExpression as an Expression implementation. |
| [`(*IntegerExpression).expressionMarker`](../src/something/ast.go#L325) | method | 325 | `func (*IntegerExpression).expressionMarker()` | expressionMarker marks IntegerExpression as an Expression implementation. |
| [`(*FloatExpression).expressionMarker`](../src/something/ast.go#L328) | method | 328 | `func (*FloatExpression).expressionMarker()` | expressionMarker marks FloatExpression as an Expression implementation. |
| [`(*BooleanExpression).expressionMarker`](../src/something/ast.go#L331) | method | 331 | `func (*BooleanExpression).expressionMarker()` | expressionMarker marks BooleanExpression as an Expression implementation. |
| [`(*ReferenceExpression).expressionMarker`](../src/something/ast.go#L334) | method | 334 | `func (*ReferenceExpression).expressionMarker()` | expressionMarker marks ReferenceExpression as an Expression implementation. |
| [`(*ArrayExpression).expressionMarker`](../src/something/ast.go#L337) | method | 337 | `func (*ArrayExpression).expressionMarker()` | expressionMarker marks ArrayExpression as an Expression implementation. |
| [`(*MappingExpression).expressionMarker`](../src/something/ast.go#L340) | method | 340 | `func (*MappingExpression).expressionMarker()` | expressionMarker marks MappingExpression as an Expression implementation. |
| [`(*StructExpression).expressionMarker`](../src/something/ast.go#L343) | method | 343 | `func (*StructExpression).expressionMarker()` | expressionMarker marks StructExpression as an Expression implementation. |
| [`(*IncludeExpression).expressionMarker`](../src/something/ast.go#L346) | method | 346 | `func (*IncludeExpression).expressionMarker()` | expressionMarker marks IncludeExpression as an Expression implementation. |
| [`(*IterationExpression).expressionMarker`](../src/something/ast.go#L349) | method | 349 | `func (*IterationExpression).expressionMarker()` | expressionMarker marks IterationExpression as an Expression implementation. |
| [`(*MacroCallExpression).expressionMarker`](../src/something/ast.go#L352) | method | 352 | `func (*MacroCallExpression).expressionMarker()` | expressionMarker marks MacroCallExpression as an Expression implementation. |
| [`(*NamespaceExpression).expressionMarker`](../src/something/ast.go#L355) | method | 355 | `func (*NamespaceExpression).expressionMarker()` | expressionMarker marks NamespaceExpression as an Expression implementation. |
| [`(*TypedExpression).expressionMarker`](../src/something/ast.go#L358) | method | 358 | `func (*TypedExpression).expressionMarker()` | expressionMarker marks TypedExpression as an Expression implementation. |
| [`(*StringExpression).expressionLocation`](../src/something/ast.go#L361) | method | 361 | `func (*StringExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*IntegerExpression).expressionLocation`](../src/something/ast.go#L364) | method | 364 | `func (*IntegerExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*FloatExpression).expressionLocation`](../src/something/ast.go#L367) | method | 367 | `func (*FloatExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*BooleanExpression).expressionLocation`](../src/something/ast.go#L370) | method | 370 | `func (*BooleanExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*ReferenceExpression).expressionLocation`](../src/something/ast.go#L373) | method | 373 | `func (*ReferenceExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*ArrayExpression).expressionLocation`](../src/something/ast.go#L376) | method | 376 | `func (*ArrayExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*MappingExpression).expressionLocation`](../src/something/ast.go#L379) | method | 379 | `func (*MappingExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*StructExpression).expressionLocation`](../src/something/ast.go#L382) | method | 382 | `func (*StructExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*IncludeExpression).expressionLocation`](../src/something/ast.go#L385) | method | 385 | `func (*IncludeExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*IterationExpression).expressionLocation`](../src/something/ast.go#L388) | method | 388 | `func (*IterationExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*MacroCallExpression).expressionLocation`](../src/something/ast.go#L391) | method | 391 | `func (*MacroCallExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*NamespaceExpression).expressionLocation`](../src/something/ast.go#L394) | method | 394 | `func (*NamespaceExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*TypedExpression).expressionLocation`](../src/something/ast.go#L397) | method | 397 | `func (*TypedExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`MappingEntry`](../src/something/ast.go#L400) | struct | 400-405 | `type MappingEntry struct { Keys []Expression Value Expression IsComposite bool Location *SourceLocation }` | MappingEntry is one mapping literal entry. Composite keys have multiple keys. |
| [`FieldAssignment`](../src/something/ast.go#L408) | struct | 408-412 | `type FieldAssignment struct { Name string Value Expression Location *SourceLocation }` | FieldAssignment is one field initializer in a struct literal. |
| [`ScopeExpression`](../src/something/ast.go#L415) | struct | 415-418 | `type ScopeExpression struct { Statements []Statement Location *SourceLocation }` | ScopeExpression is a block whose declarations form a scope value. |
| [`(*ScopeExpression).assignmentValueMarker`](../src/something/ast.go#L421) | method | 421 | `func (*ScopeExpression).assignmentValueMarker()` | assignmentValueMarker marks ScopeExpression as an AssignmentValue implementation. |
| [`FieldDefinition`](../src/something/ast.go#L424) | struct | 424-431 | `type FieldDefinition struct { Name string DeclaredType TypeRef InferType bool Optional bool DefaultValue Expression Location *SourceLocation }` | FieldDefinition is one field in a setup definition. |
| [`SetupDefinition`](../src/something/ast.go#L434) | struct | 434-437 | `type SetupDefinition struct { Fields []*FieldDefinition Location *SourceLocation }` | SetupDefinition is the right-hand side of a `name: setup = { ... }` assignment. |
| [`(*SetupDefinition).assignmentValueMarker`](../src/something/ast.go#L440) | method | 440 | `func (*SetupDefinition).assignmentValueMarker()` | assignmentValueMarker marks SetupDefinition as an AssignmentValue implementation. |
| [`EnumMember`](../src/something/ast.go#L443) | struct | 443-447 | `type EnumMember struct { Name string Value Expression Location *SourceLocation }` | EnumMember is one ordered member of an enum definition. |
| [`EnumDefinition`](../src/something/ast.go#L450) | struct | 450-454 | `type EnumDefinition struct { ValueType TypeRef Members []*EnumMember Location *SourceLocation }` | EnumDefinition is the right-hand side of a `name: enum = { ... }` assignment. |
| [`(*EnumDefinition).assignmentValueMarker`](../src/something/ast.go#L457) | method | 457 | `func (*EnumDefinition).assignmentValueMarker()` | assignmentValueMarker marks EnumDefinition as an AssignmentValue implementation. |
| [`IncludeDirective`](../src/something/ast.go#L460) | struct | 460-463 | `type IncludeDirective struct { StatementBase Filepath string }` | IncludeDirective inserts another file's statements at its source position. |
| [`(*IncludeDirective).statementMarker`](../src/something/ast.go#L466) | method | 466 | `func (*IncludeDirective).statementMarker()` | statementMarker marks IncludeDirective as a Statement implementation. |
| [`(*IncludeDirective).statementBase`](../src/something/ast.go#L469) | method | 469 | `func (*IncludeDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`ForDirective`](../src/something/ast.go#L472) | struct | 472-478 | `type ForDirective struct { StatementBase ElementName string KeyName string Source Expression Body []Statement }` | ForDirective expands its body once for each source element. |
| [`(*ForDirective).statementMarker`](../src/something/ast.go#L481) | method | 481 | `func (*ForDirective).statementMarker()` | statementMarker marks ForDirective as a Statement implementation. |
| [`(*ForDirective).statementBase`](../src/something/ast.go#L484) | method | 484 | `func (*ForDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`InsertDirective`](../src/something/ast.go#L487) | struct | 487-490 | `type InsertDirective struct { StatementBase Contents []Expression }` | InsertDirective parses evaluated strings as statements at its source position. |
| [`(*InsertDirective).statementMarker`](../src/something/ast.go#L493) | method | 493 | `func (*InsertDirective).statementMarker()` | statementMarker marks InsertDirective as a Statement implementation. |
| [`(*InsertDirective).statementBase`](../src/something/ast.go#L496) | method | 496 | `func (*InsertDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`MacroParam`](../src/something/ast.go#L499) | struct | 499-503 | `type MacroParam struct { Name string Type TypeRef Location *SourceLocation }` | MacroParam is one typed macro input. |
| [`MacroDirective`](../src/something/ast.go#L506) | struct | 506-513 | `type MacroDirective struct { StatementBase Name string Params []MacroParam ReturnType TypeRef Body []Statement Return Expression }` | MacroDirective is a compile-time expression template. It is removed by expansion. |
| [`(*MacroDirective).statementMarker`](../src/something/ast.go#L516) | method | 516 | `func (*MacroDirective).statementMarker()` | statementMarker marks MacroDirective as a Statement implementation. |
| [`(*MacroDirective).statementBase`](../src/something/ast.go#L519) | method | 519 | `func (*MacroDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`AssertDirective`](../src/something/ast.go#L523) | struct | 523-527 | `type AssertDirective struct { StatementBase TypeName string Body []Statement }` | AssertDirective validates a setup type definition with a body of assertion statements evaluated in a scope that inherits the type's fields. |
| [`(*AssertDirective).statementMarker`](../src/something/ast.go#L530) | method | 530 | `func (*AssertDirective).statementMarker()` | statementMarker marks AssertDirective as a Statement implementation. |
| [`(*AssertDirective).statementBase`](../src/something/ast.go#L533) | method | 533 | `func (*AssertDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`IfDirective`](../src/something/ast.go#L538) | struct | 538-542 | `type IfDirective struct { StatementBase Condition Expression Body []Statement }` | IfDirective conditionally evaluates its body when the condition is true. Body is always non-nil, either from the block form (#if cond { ... }) or the single-statement form (#if cond stmt;). |
| [`(*IfDirective).statementMarker`](../src/something/ast.go#L545) | method | 545 | `func (*IfDirective).statementMarker()` | statementMarker marks IfDirective as a Statement implementation. |
| [`(*IfDirective).statementBase`](../src/something/ast.go#L548) | method | 548 | `func (*IfDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`ErrorDirective`](../src/something/ast.go#L551) | struct | 551-554 | `type ErrorDirective struct { StatementBase Message Expression // string expression, may use interpolation }` | ErrorDirective terminates evaluation with a user-defined error message. |
| [`(*ErrorDirective).statementMarker`](../src/something/ast.go#L557) | method | 557 | `func (*ErrorDirective).statementMarker()` | statementMarker marks ErrorDirective as a Statement implementation. |
| [`(*ErrorDirective).statementBase`](../src/something/ast.go#L560) | method | 560 | `func (*ErrorDirective).statementBase() *StatementBase` | statementBase returns the receiver's shared statement metadata. |
| [`BinaryOpKind`](../src/something/ast.go#L563) | type | 563 | `type BinaryOpKind int` | BinaryOpKind identifies a binary comparison or logical operator. |
| [`BinaryOpExpression`](../src/something/ast.go#L577) | struct | 577-582 | `type BinaryOpExpression struct { Left Expression Op BinaryOpKind Right Expression Location *SourceLocation }` | BinaryOpExpression is a binary comparison or logical expression. |
| [`UnaryOpKind`](../src/something/ast.go#L585) | type | 585 | `type UnaryOpKind int` | UnaryOpKind identifies a unary operator. |
| [`UnaryOpExpression`](../src/something/ast.go#L592) | struct | 592-596 | `type UnaryOpExpression struct { Op UnaryOpKind Operand Expression Location *SourceLocation }` | UnaryOpExpression is a unary expression (e.g., #not). |
| [`MatchExpression`](../src/something/ast.go#L599) | struct | 599-603 | `type MatchExpression struct { Value Expression Pattern Expression Location *SourceLocation }` | MatchExpression evaluates a regex match. |
| [`LenExpression`](../src/something/ast.go#L606) | struct | 606-609 | `type LenExpression struct { Value Expression Location *SourceLocation }` | LenExpression evaluates the length of an array or mapping. |
| [`(*BinaryOpExpression).assignmentValueMarker`](../src/something/ast.go#L612) | method | 612 | `func (*BinaryOpExpression).assignmentValueMarker()` | assignmentValueMarker marks BinaryOpExpression as an AssignmentValue implementation. |
| [`(*UnaryOpExpression).assignmentValueMarker`](../src/something/ast.go#L615) | method | 615 | `func (*UnaryOpExpression).assignmentValueMarker()` | assignmentValueMarker marks UnaryOpExpression as an AssignmentValue implementation. |
| [`(*MatchExpression).assignmentValueMarker`](../src/something/ast.go#L618) | method | 618 | `func (*MatchExpression).assignmentValueMarker()` | assignmentValueMarker marks MatchExpression as an AssignmentValue implementation. |
| [`(*LenExpression).assignmentValueMarker`](../src/something/ast.go#L621) | method | 621 | `func (*LenExpression).assignmentValueMarker()` | assignmentValueMarker marks LenExpression as an AssignmentValue implementation. |
| [`(*BinaryOpExpression).expressionMarker`](../src/something/ast.go#L624) | method | 624 | `func (*BinaryOpExpression).expressionMarker()` | expressionMarker marks BinaryOpExpression as an Expression implementation. |
| [`(*UnaryOpExpression).expressionMarker`](../src/something/ast.go#L627) | method | 627 | `func (*UnaryOpExpression).expressionMarker()` | expressionMarker marks UnaryOpExpression as an Expression implementation. |
| [`(*MatchExpression).expressionMarker`](../src/something/ast.go#L630) | method | 630 | `func (*MatchExpression).expressionMarker()` | expressionMarker marks MatchExpression as an Expression implementation. |
| [`(*LenExpression).expressionMarker`](../src/something/ast.go#L633) | method | 633 | `func (*LenExpression).expressionMarker()` | expressionMarker marks LenExpression as an Expression implementation. |
| [`(*BinaryOpExpression).expressionLocation`](../src/something/ast.go#L636) | method | 636 | `func (*BinaryOpExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*UnaryOpExpression).expressionLocation`](../src/something/ast.go#L639) | method | 639 | `func (*UnaryOpExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*MatchExpression).expressionLocation`](../src/something/ast.go#L642) | method | 642 | `func (*MatchExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |
| [`(*LenExpression).expressionLocation`](../src/something/ast.go#L645) | method | 645 | `func (*LenExpression).expressionLocation() *SourceLocation` | expressionLocation returns the receiver's source location. |

### [`src/something/ast_functional_test.go`](../src/something/ast_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestStringLiteralViaPipeline`](../src/something/ast_functional_test.go#L16) | test | 16-21 | `func TestStringLiteralViaPipeline(t *testing.T)` | TestStringLiteralViaPipeline verifies string literal via pipeline. |
| [`TestIntegerLiteralViaPipeline`](../src/something/ast_functional_test.go#L24) | test | 24-29 | `func TestIntegerLiteralViaPipeline(t *testing.T)` | TestIntegerLiteralViaPipeline verifies integer literal via pipeline. |
| [`TestFloatLiteralViaPipeline`](../src/something/ast_functional_test.go#L32) | test | 32-37 | `func TestFloatLiteralViaPipeline(t *testing.T)` | TestFloatLiteralViaPipeline verifies float literal via pipeline. |
| [`TestBooleanLiteralViaPipeline`](../src/something/ast_functional_test.go#L40) | test | 40-45 | `func TestBooleanLiteralViaPipeline(t *testing.T)` | TestBooleanLiteralViaPipeline verifies boolean literal via pipeline. |
| [`TestTypedStringDeclaration`](../src/something/ast_functional_test.go#L50) | test | 50-55 | `func TestTypedStringDeclaration(t *testing.T)` | TestTypedStringDeclaration verifies typed string declaration. |
| [`TestTypedIntegerDeclaration`](../src/something/ast_functional_test.go#L58) | test | 58-63 | `func TestTypedIntegerDeclaration(t *testing.T)` | TestTypedIntegerDeclaration verifies typed integer declaration. |
| [`TestInterpolationBasic`](../src/something/ast_functional_test.go#L68) | test | 68-73 | `func TestInterpolationBasic(t *testing.T)` | TestInterpolationBasic verifies interpolation basic. |
| [`TestInterpolationMultiple`](../src/something/ast_functional_test.go#L76) | test | 76-81 | `func TestInterpolationMultiple(t *testing.T)` | TestInterpolationMultiple verifies interpolation multiple. |
| [`TestArrayTypeParsing`](../src/something/ast_functional_test.go#L86) | test | 86-98 | `func TestArrayTypeParsing(t *testing.T)` | TestArrayTypeParsing verifies array type parsing. |
| [`TestMappingTypeParsing`](../src/something/ast_functional_test.go#L103) | test | 103-112 | `func TestMappingTypeParsing(t *testing.T)` | TestMappingTypeParsing verifies mapping type parsing. |
| [`TestEnumParsing`](../src/something/ast_functional_test.go#L117) | test | 117-122 | `func TestEnumParsing(t *testing.T)` | TestEnumParsing verifies enum parsing. |
| [`TestSetupParsing`](../src/something/ast_functional_test.go#L127) | test | 127-136 | `func TestSetupParsing(t *testing.T)` | TestSetupParsing verifies setup parsing. |
| [`TestScopeParsing`](../src/something/ast_functional_test.go#L141) | test | 141-146 | `func TestScopeParsing(t *testing.T)` | TestScopeParsing verifies scope parsing. |
| [`TestIncludeExpressionParsing`](../src/something/ast_functional_test.go#L151) | test | 151-160 | `func TestIncludeExpressionParsing(t *testing.T)` | TestIncludeExpressionParsing verifies include expression parsing. |
| [`TestMultilineParsing`](../src/something/ast_functional_test.go#L165) | test | 165-170 | `func TestMultilineParsing(t *testing.T)` | TestMultilineParsing verifies multiline parsing. |
| [`TestEmptyProgram`](../src/something/ast_functional_test.go#L175) | test | 175-180 | `func TestEmptyProgram(t *testing.T)` | TestEmptyProgram verifies empty program. |
| [`TestMultipleDeclarations`](../src/something/ast_functional_test.go#L185) | test | 185-196 | `func TestMultipleDeclarations(t *testing.T)` | TestMultipleDeclarations verifies multiple declarations. |
| [`TestReferenceRootIdentity`](../src/something/ast_functional_test.go#L201) | test | 201-206 | `func TestReferenceRootIdentity(t *testing.T)` | TestReferenceRootIdentity verifies reference root identity. |
| [`TestForDirectiveParsing`](../src/something/ast_functional_test.go#L211) | test | 211-216 | `func TestForDirectiveParsing(t *testing.T)` | TestForDirectiveParsing verifies for directive parsing. |
| [`TestInsertDirectiveParsing`](../src/something/ast_functional_test.go#L221) | test | 221-226 | `func TestInsertDirectiveParsing(t *testing.T)` | TestInsertDirectiveParsing verifies insert directive parsing. |
| [`TestIterationDirectiveParsing`](../src/something/ast_functional_test.go#L231) | test | 231-242 | `func TestIterationDirectiveParsing(t *testing.T)` | TestIterationDirectiveParsing verifies iteration directive parsing. |
| [`TestAsLvalueDirectiveParsing`](../src/something/ast_functional_test.go#L247) | test | 247-252 | `func TestAsLvalueDirectiveParsing(t *testing.T)` | TestAsLvalueDirectiveParsing verifies as lvalue directive parsing. |
| [`TestOptionalFieldParsing`](../src/something/ast_functional_test.go#L257) | test | 257-263 | `func TestOptionalFieldParsing(t *testing.T)` | TestOptionalFieldParsing verifies optional field parsing. |
| [`TestEnumTaggedValueParsing`](../src/something/ast_functional_test.go#L268) | test | 268-273 | `func TestEnumTaggedValueParsing(t *testing.T)` | TestEnumTaggedValueParsing verifies enum tagged value parsing. |
| [`TestNegativeNumberParsing`](../src/something/ast_functional_test.go#L278) | test | 278-283 | `func TestNegativeNumberParsing(t *testing.T)` | TestNegativeNumberParsing verifies negative number parsing. |
| [`TestCompositeKeyParsing`](../src/something/ast_functional_test.go#L288) | test | 288-294 | `func TestCompositeKeyParsing(t *testing.T)` | TestCompositeKeyParsing verifies composite key parsing. |
| [`TestIndexAccessParsing`](../src/something/ast_functional_test.go#L299) | test | 299-304 | `func TestIndexAccessParsing(t *testing.T)` | TestIndexAccessParsing verifies index access parsing. |
| [`TestFieldAccessParsing`](../src/something/ast_functional_test.go#L309) | test | 309-314 | `func TestFieldAccessParsing(t *testing.T)` | TestFieldAccessParsing verifies field access parsing. |
| [`TestTypedExpressionViaStruct`](../src/something/ast_functional_test.go#L319) | test | 319-325 | `func TestTypedExpressionViaStruct(t *testing.T)` | TestTypedExpressionViaStruct verifies typed expression via struct. |
| [`TestNamespaceExpressionParsing`](../src/something/ast_functional_test.go#L330) | test | 330-335 | `func TestNamespaceExpressionParsing(t *testing.T)` | TestNamespaceExpressionParsing verifies namespace expression parsing. |
| [`TestEnumKeyTypeParsing`](../src/something/ast_functional_test.go#L340) | test | 340-345 | `func TestEnumKeyTypeParsing(t *testing.T)` | TestEnumKeyTypeParsing verifies enum key type parsing. |

### [`src/something/ast_unit_test.go`](../src/something/ast_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestTypeRefInterfaceMarkers`](../src/something/ast_unit_test.go#L13) | test | 13-22 | `func TestTypeRefInterfaceMarkers(t *testing.T)` | TestTypeRefInterfaceMarkers verifies type ref interface markers. |

### [`src/something/directive_generator.go`](../src/something/directive_generator.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`macroEnvironment`](../src/something/directive_generator.go#L15) | struct | 15-18 | `type macroEnvironment struct { parent *macroEnvironment definitions map[string]*MacroDirective }` | macroEnvironment stores lexically nested macro definitions for directive expansion. |
| [`newMacroEnvironment`](../src/something/directive_generator.go#L21) | function | 21-23 | `func newMacroEnvironment(parent *macroEnvironment) *macroEnvironment` | newMacroEnvironment creates an empty macro scope linked to its parent. |
| [`(*macroEnvironment).lookup`](../src/something/directive_generator.go#L26) | method | 26-33 | `func (*macroEnvironment).lookup(name string) (*MacroDirective, bool)` | lookup searches the current and enclosing macro scopes for a definition. |
| [`DirectiveGenerator`](../src/something/directive_generator.go#L36) | struct | 36-47 | `type DirectiveGenerator struct { filepath string runtime *runtimeState macros *macroEnvironment includeStack []string parsedIncludes map[string]*Program bareIncludes map[string]bool macroStack []string iterationCount int iterationCounters map[string]int constantValues []map[string]any }` | DirectiveGenerator expands directives from one parsed syntax tree. |
| [`NewDirectiveGenerator`](../src/something/directive_generator.go#L50) | function | 50-59 | `func NewDirectiveGenerator(filepath string) *DirectiveGenerator` | NewDirectiveGenerator constructs directive generator. |
| [`(*DirectiveGenerator).err`](../src/something/directive_generator.go#L62) | method | 62-64 | `func (*DirectiveGenerator).err(message string, location *SourceLocation, suggestion string)` | err panics with a source-located SomethingError for a directive failure. |
| [`(*DirectiveGenerator).Expand`](../src/something/directive_generator.go#L67) | method | 67-81 | `func (*DirectiveGenerator).Expand(program *Program) *Program` | Expand removes every directive and preserves the generated assignment order. |
| [`(*DirectiveGenerator).expandStatements`](../src/something/directive_generator.go#L84) | method | 84-110 | `func (*DirectiveGenerator).expandStatements(statements []Statement, inheritedPrivate bool) []Statement` | expandStatements expands statements into its structural AST form. |
| [`(*DirectiveGenerator).expandAssignment`](../src/something/directive_generator.go#L113) | method | 113-136 | `func (*DirectiveGenerator).expandAssignment(source *Assignment, private bool) *Assignment` | expandAssignment expands assignment into its structural AST form. |
| [`(*DirectiveGenerator).expandScopeValue`](../src/something/directive_generator.go#L139) | method | 139-159 | `func (*DirectiveGenerator).expandScopeValue(assignment *Assignment, source *ScopeExpression, kind PrimitiveKind) *ScopeExpression` | expandScopeValue expands scope value into its structural AST form. |
| [`(*DirectiveGenerator).expandSetupDefinition`](../src/something/directive_generator.go#L162) | method | 162-172 | `func (*DirectiveGenerator).expandSetupDefinition(source *SetupDefinition) *SetupDefinition` | expandSetupDefinition expands setup definition into its structural AST form. |
| [`(*DirectiveGenerator).expandEnumDefinition`](../src/something/directive_generator.go#L175) | method | 175-185 | `func (*DirectiveGenerator).expandEnumDefinition(source *EnumDefinition) *EnumDefinition` | expandEnumDefinition expands enum definition into its structural AST form. |
| [`(*DirectiveGenerator).expandLValue`](../src/something/directive_generator.go#L188) | method | 188-212 | `func (*DirectiveGenerator).expandLValue(target LValue, location *SourceLocation) LValue` | expandLValue expands l value into its structural AST form. |
| [`(*DirectiveGenerator).expandAccesses`](../src/something/directive_generator.go#L215) | method | 215-226 | `func (*DirectiveGenerator).expandAccesses(accesses []Access) []Access` | expandAccesses expands accesses into its structural AST form. |
| [`(*DirectiveGenerator).parseGeneratedLValue`](../src/something/directive_generator.go#L229) | method | 229-240 | `func (*DirectiveGenerator).parseGeneratedLValue(name string, location *SourceLocation) LValue` | parseGeneratedLValue parses generated l value from the supplied input. |
| [`(*DirectiveGenerator).expandExpression`](../src/something/directive_generator.go#L243) | method | 243-316 | `func (*DirectiveGenerator).expandExpression(expression Expression) Expression` | expandExpression expands expression into its structural AST form. |
| [`(*DirectiveGenerator).expandString`](../src/something/directive_generator.go#L319) | method | 319-368 | `func (*DirectiveGenerator).expandString(source *StringExpression) Expression` | expandString expands string into its structural AST form. |
| [`(*DirectiveGenerator).directiveLabel`](../src/something/directive_generator.go#L371) | method | 371-382 | `func (*DirectiveGenerator).directiveLabel(expression Expression, directive string, location *SourceLocation) string` | directiveLabel expands and evaluates an optional directive label as a string. |
| [`(*DirectiveGenerator).evaluateConstantReference`](../src/something/directive_generator.go#L385) | method | 385-395 | `func (*DirectiveGenerator).evaluateConstantReference(path string, location *SourceLocation) any` | evaluateConstantReference evaluates constant reference against the current evaluator state. |
| [`(*DirectiveGenerator).nextIterationKey`](../src/something/directive_generator.go#L398) | method | 398-407 | `func (*DirectiveGenerator).nextIterationKey(label string) string` | nextIterationKey returns and advances the deterministic counter for an iteration label. |
| [`(*DirectiveGenerator).peekIterationKey`](../src/something/directive_generator.go#L410) | method | 410-416 | `func (*DirectiveGenerator).peekIterationKey(label string) string` | peekIterationKey returns the next deterministic key for an iteration label without advancing it. |
| [`(*DirectiveGenerator).validateAndApply`](../src/something/directive_generator.go#L419) | method | 419-443 | `func (*DirectiveGenerator).validateAndApply(assignment *Assignment)` | validateAndApply type-checks an expanded assignment against the temporary runtime and applies it. |
| [`(*DirectiveGenerator).runtimeTargetType`](../src/something/directive_generator.go#L446) | method | 446-472 | `func (*DirectiveGenerator).runtimeTargetType(target LValue, location *SourceLocation) TypeRef` | runtimeTargetType resolves the current type of an assignment target during expansion. |
| [`(*DirectiveGenerator).runtimeValueAssignable`](../src/something/directive_generator.go#L475) | method | 475-548 | `func (*DirectiveGenerator).runtimeValueAssignable(expected TypeRef, value any) bool` | runtimeValueAssignable reports whether an evaluated directive value conforms to an expected type. |
| [`(*DirectiveGenerator).expandBareInclude`](../src/something/directive_generator.go#L551) | method | 551-565 | `func (*DirectiveGenerator).expandBareInclude(include *IncludeDirective, private bool) []Statement` | expandBareInclude expands bare include into its structural AST form. |
| [`(*DirectiveGenerator).expandNamespaceInclude`](../src/something/directive_generator.go#L568) | method | 568-585 | `func (*DirectiveGenerator).expandNamespaceInclude(include *IncludeExpression) Expression` | expandNamespaceInclude expands namespace include into its structural AST form. |
| [`(*DirectiveGenerator).resolveIncludePath`](../src/something/directive_generator.go#L588) | method | 588-597 | `func (*DirectiveGenerator).resolveIncludePath(path string, location *SourceLocation) string` | resolveIncludePath resolves include path from the supplied context. |
| [`cleanKnownPath`](../src/something/directive_generator.go#L600) | function | 600-606 | `func cleanKnownPath(path string) string` | cleanKnownPath cleans a path and makes it absolute when the runtime can resolve it. |
| [`(*DirectiveGenerator).loadInclude`](../src/something/directive_generator.go#L609) | method | 609-621 | `func (*DirectiveGenerator).loadInclude(path string, location *SourceLocation) *Program` | loadInclude loads include from the supplied source. |
| [`(*DirectiveGenerator).pathInIncludeStack`](../src/something/directive_generator.go#L624) | method | 624-631 | `func (*DirectiveGenerator).pathInIncludeStack(path string) bool` | pathInIncludeStack reports whether an include path is already active. |
| [`(*DirectiveGenerator).includeCycleError`](../src/something/directive_generator.go#L634) | method | 634-641 | `func (*DirectiveGenerator).includeCycleError(path string, location *SourceLocation)` | includeCycleError raises a diagnostic containing the active recursive include chain. |
| [`(*DirectiveGenerator).expandFor`](../src/something/directive_generator.go#L644) | method | 644-675 | `func (*DirectiveGenerator).expandFor(loop *ForDirective, private bool) []Statement` | expandFor expands for into its structural AST form. |
| [`(*DirectiveGenerator).expandInsert`](../src/something/directive_generator.go#L678) | method | 678-696 | `func (*DirectiveGenerator).expandInsert(insert *InsertDirective, private bool) []Statement` | expandInsert expands insert into its structural AST form. |
| [`(*DirectiveGenerator).registerMacro`](../src/something/directive_generator.go#L699) | method | 699-712 | `func (*DirectiveGenerator).registerMacro(macro *MacroDirective)` | registerMacro registers macro. |
| [`(*DirectiveGenerator).expandMacroCall`](../src/something/directive_generator.go#L715) | method | 715-764 | `func (*DirectiveGenerator).expandMacroCall(call *MacroCallExpression) Expression` | expandMacroCall expands macro call into its structural AST form. |
| [`(*DirectiveGenerator).checkExpandedMacro`](../src/something/directive_generator.go#L767) | method | 767-803 | `func (*DirectiveGenerator).checkExpandedMacro(macro *MacroDirective, arguments []any, body []Statement, returnExpression Expression, returnType TypeRef, outerRuntime *runtimeEnvironment)` | checkExpandedMacro checks expanded macro against the current invariants. |
| [`(*DirectiveGenerator).expressionFromRuntime`](../src/something/directive_generator.go#L806) | method | 806-850 | `func (*DirectiveGenerator).expressionFromRuntime(value any, location *SourceLocation) Expression` | expressionFromRuntime converts a compile-time runtime value back into a syntax expression. |
| [`stringExpression`](../src/something/directive_generator.go#L853) | function | 853-859 | `func stringExpression(value string, location *SourceLocation) *StringExpression` | stringExpression wraps plain text in a string-literal expression at a source location. |
| [`(*DirectiveGenerator).pushConstants`](../src/something/directive_generator.go#L862) | method | 862-864 | `func (*DirectiveGenerator).pushConstants(values map[string]any)` | pushConstants adds one lexical constant scope for directive expansion. |
| [`(*DirectiveGenerator).popConstants`](../src/something/directive_generator.go#L867) | method | 867-869 | `func (*DirectiveGenerator).popConstants()` | popConstants removes the innermost directive constant scope. |
| [`(*DirectiveGenerator).lookupConstant`](../src/something/directive_generator.go#L872) | method | 872-879 | `func (*DirectiveGenerator).lookupConstant(name string) (any, bool)` | lookupConstant searches the innermost-to-outermost directive constant scopes. |

### [`src/something/directive_generator_functional_test.go`](../src/something/directive_generator_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvalForArray_Functional`](../src/something/directive_generator_functional_test.go#L14) | test | 14-20 | `func TestEvalForArray_Functional(t *testing.T)` | TestEvalForArray_Functional verifies eval for array functional. |
| [`TestEvalInsert_Functional`](../src/something/directive_generator_functional_test.go#L23) | test | 23-28 | `func TestEvalInsert_Functional(t *testing.T)` | TestEvalInsert_Functional verifies eval insert functional. |
| [`TestEvalInsertInterpolation_Functional`](../src/something/directive_generator_functional_test.go#L31) | test | 31-36 | `func TestEvalInsertInterpolation_Functional(t *testing.T)` | TestEvalInsertInterpolation_Functional verifies eval insert interpolation functional. |
| [`TestEvalInsertMultipleValues_Functional`](../src/something/directive_generator_functional_test.go#L39) | test | 39-44 | `func TestEvalInsertMultipleValues_Functional(t *testing.T)` | TestEvalInsertMultipleValues_Functional verifies eval insert multiple values functional. |
| [`TestEvalIteration_Functional`](../src/something/directive_generator_functional_test.go#L47) | test | 47-59 | `func TestEvalIteration_Functional(t *testing.T)` | TestEvalIteration_Functional verifies eval iteration functional. |
| [`TestEvalIterationPeek_Functional`](../src/something/directive_generator_functional_test.go#L62) | test | 62-68 | `func TestEvalIterationPeek_Functional(t *testing.T)` | TestEvalIterationPeek_Functional verifies eval iteration peek functional. |
| [`TestEvalAsLvalue_Functional`](../src/something/directive_generator_functional_test.go#L71) | test | 71-76 | `func TestEvalAsLvalue_Functional(t *testing.T)` | TestEvalAsLvalue_Functional verifies eval as lvalue functional. |
| [`TestEvalAsLvalueFromVar_Functional`](../src/something/directive_generator_functional_test.go#L79) | test | 79-84 | `func TestEvalAsLvalueFromVar_Functional(t *testing.T)` | TestEvalAsLvalueFromVar_Functional verifies eval as lvalue from var functional. |
| [`TestEvalScopeFromIteration_Functional`](../src/something/directive_generator_functional_test.go#L87) | test | 87-101 | `func TestEvalScopeFromIteration_Functional(t *testing.T)` | TestEvalScopeFromIteration_Functional verifies eval scope from iteration functional. |
| [`TestEvalNestedIterationScope_Functional`](../src/something/directive_generator_functional_test.go#L104) | test | 104-124 | `func TestEvalNestedIterationScope_Functional(t *testing.T)` | TestEvalNestedIterationScope_Functional verifies eval nested iteration scope functional. |
| [`TestEvalIterationWithLabel_Functional`](../src/something/directive_generator_functional_test.go#L127) | test | 127-138 | `func TestEvalIterationWithLabel_Functional(t *testing.T)` | TestEvalIterationWithLabel_Functional verifies eval iteration with label functional. |
| [`TestEvalIterationLabelsHaveIndependentCounters_Functional`](../src/something/directive_generator_functional_test.go#L141) | test | 141-161 | `func TestEvalIterationLabelsHaveIndependentCounters_Functional(t *testing.T)` | TestEvalIterationLabelsHaveIndependentCounters_Functional verifies eval iteration labels have independent counters functional. |
| [`TestEvalBareScopeDoesNotDoubleConsumeIterationCounters_Functional`](../src/something/directive_generator_functional_test.go#L164) | test | 164-170 | `func TestEvalBareScopeDoesNotDoubleConsumeIterationCounters_Functional(t *testing.T)` | TestEvalBareScopeDoesNotDoubleConsumeIterationCounters_Functional verifies eval bare scope does not double consume iteration counters functional. |
| [`TestEvalForMappingIteration_Functional`](../src/something/directive_generator_functional_test.go#L173) | test | 173-180 | `func TestEvalForMappingIteration_Functional(t *testing.T)` | TestEvalForMappingIteration_Functional verifies eval for mapping iteration functional. |
| [`TestEvalForMapIntKey_Functional`](../src/something/directive_generator_functional_test.go#L183) | test | 183-189 | `func TestEvalForMapIntKey_Functional(t *testing.T)` | TestEvalForMapIntKey_Functional verifies eval for map int key functional. |
| [`TestEvalForSourceError_Functional`](../src/something/directive_generator_functional_test.go#L192) | test | 192-196 | `func TestEvalForSourceError_Functional(t *testing.T)` | TestEvalForSourceError_Functional verifies eval for source error functional. |
| [`TestEvalForMappingError_Functional`](../src/something/directive_generator_functional_test.go#L199) | test | 199-203 | `func TestEvalForMappingError_Functional(t *testing.T)` | TestEvalForMappingError_Functional verifies eval for mapping error functional. |
| [`TestEvalForSourceArray_Functional`](../src/something/directive_generator_functional_test.go#L206) | test | 206-212 | `func TestEvalForSourceArray_Functional(t *testing.T)` | TestEvalForSourceArray_Functional verifies eval for source array functional. |
| [`TestEvalScopeBodyForDecl_Functional`](../src/something/directive_generator_functional_test.go#L215) | test | 215-222 | `func TestEvalScopeBodyForDecl_Functional(t *testing.T)` | TestEvalScopeBodyForDecl_Functional verifies eval scope body for decl functional. |
| [`TestEvalScopeBodyInsertDecl_Functional`](../src/something/directive_generator_functional_test.go#L225) | test | 225-232 | `func TestEvalScopeBodyInsertDecl_Functional(t *testing.T)` | TestEvalScopeBodyInsertDecl_Functional verifies eval scope body insert decl functional. |
| [`TestEvalInsertErrorNotString_Functional`](../src/something/directive_generator_functional_test.go#L235) | test | 235-239 | `func TestEvalInsertErrorNotString_Functional(t *testing.T)` | TestEvalInsertErrorNotString_Functional verifies eval insert error not string functional. |
| [`TestEvalScopeBodyIterationTyped_Functional`](../src/something/directive_generator_functional_test.go#L242) | test | 242-257 | `func TestEvalScopeBodyIterationTyped_Functional(t *testing.T)` | TestEvalScopeBodyIterationTyped_Functional verifies eval scope body iteration typed functional. |
| [`TestEvalScopeBodyAsLvalueTyped_Functional`](../src/something/directive_generator_functional_test.go#L260) | test | 260-269 | `func TestEvalScopeBodyAsLvalueTyped_Functional(t *testing.T)` | TestEvalScopeBodyAsLvalueTyped_Functional verifies eval scope body as lvalue typed functional. |
| [`TestParseAsLvalueError_Functional`](../src/something/directive_generator_functional_test.go#L272) | test | 272-276 | `func TestParseAsLvalueError_Functional(t *testing.T)` | TestParseAsLvalueError_Functional verifies parse as lvalue error functional. |
| [`TestEvalResolveAsLvalueFromString_Functional`](../src/something/directive_generator_functional_test.go#L279) | test | 279-285 | `func TestEvalResolveAsLvalueFromString_Functional(t *testing.T)` | TestEvalResolveAsLvalueFromString_Functional verifies eval resolve as lvalue from string functional. |
| [`TestEvalPrivIteration_Functional`](../src/something/directive_generator_functional_test.go#L288) | test | 288-296 | `func TestEvalPrivIteration_Functional(t *testing.T)` | TestEvalPrivIteration_Functional verifies eval priv iteration functional. |
| [`TestEvalPrivAsLvalue_Functional`](../src/something/directive_generator_functional_test.go#L299) | test | 299-305 | `func TestEvalPrivAsLvalue_Functional(t *testing.T)` | TestEvalPrivAsLvalue_Functional verifies eval priv as lvalue functional. |
| [`TestEvalPrivIterationInScope_Functional`](../src/something/directive_generator_functional_test.go#L308) | test | 308-317 | `func TestEvalPrivIterationInScope_Functional(t *testing.T)` | TestEvalPrivIterationInScope_Functional verifies eval priv iteration in scope functional. |
| [`TestEvalPrivAsLvalueInScope_Functional`](../src/something/directive_generator_functional_test.go#L320) | test | 320-327 | `func TestEvalPrivAsLvalueInScope_Functional(t *testing.T)` | TestEvalPrivAsLvalueInScope_Functional verifies eval priv as lvalue in scope functional. |
| [`TestResolveAsLvalueNameAll_Functional`](../src/something/directive_generator_functional_test.go#L330) | test | 330-337 | `func TestResolveAsLvalueNameAll_Functional(t *testing.T)` | TestResolveAsLvalueNameAll_Functional verifies resolve as lvalue name all functional. |
| [`TestResolveAsLvalueNameError_Functional`](../src/something/directive_generator_functional_test.go#L340) | test | 340-344 | `func TestResolveAsLvalueNameError_Functional(t *testing.T)` | TestResolveAsLvalueNameError_Functional verifies resolve as lvalue name error functional. |

### [`src/something/directive_generator_unit_test.go`](../src/something/directive_generator_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestNewDirectiveGenerator`](../src/something/directive_generator_unit_test.go#L11) | test | 11-31 | `func TestNewDirectiveGenerator(t *testing.T)` | TestNewDirectiveGenerator verifies new directive generator. |
| [`TestNextIterationKeyDefault`](../src/something/directive_generator_unit_test.go#L34) | test | 34-44 | `func TestNextIterationKeyDefault(t *testing.T)` | TestNextIterationKeyDefault verifies next iteration key default. |
| [`TestNextIterationKeyLabeled`](../src/something/directive_generator_unit_test.go#L47) | test | 47-57 | `func TestNextIterationKeyLabeled(t *testing.T)` | TestNextIterationKeyLabeled verifies next iteration key labeled. |
| [`TestNextIterationKeyIndependentCounters`](../src/something/directive_generator_unit_test.go#L60) | test | 60-75 | `func TestNextIterationKeyIndependentCounters(t *testing.T)` | TestNextIterationKeyIndependentCounters verifies next iteration key independent counters. |
| [`TestPeekIterationKey`](../src/something/directive_generator_unit_test.go#L78) | test | 78-97 | `func TestPeekIterationKey(t *testing.T)` | TestPeekIterationKey verifies peek iteration key. |
| [`TestPeekIterationKeyLabeled`](../src/something/directive_generator_unit_test.go#L100) | test | 100-107 | `func TestPeekIterationKeyLabeled(t *testing.T)` | TestPeekIterationKeyLabeled verifies peek iteration key labeled. |
| [`TestPathInIncludeStackEmpty`](../src/something/directive_generator_unit_test.go#L110) | test | 110-115 | `func TestPathInIncludeStackEmpty(t *testing.T)` | TestPathInIncludeStackEmpty verifies path in include stack empty. |
| [`TestPathInIncludeStackFound`](../src/something/directive_generator_unit_test.go#L118) | test | 118-127 | `func TestPathInIncludeStackFound(t *testing.T)` | TestPathInIncludeStackFound verifies path in include stack found. |
| [`TestPathInIncludeStackNotFound`](../src/something/directive_generator_unit_test.go#L130) | test | 130-136 | `func TestPathInIncludeStackNotFound(t *testing.T)` | TestPathInIncludeStackNotFound verifies path in include stack not found. |
| [`TestCleanKnownPathAlreadyClean`](../src/something/directive_generator_unit_test.go#L139) | test | 139-144 | `func TestCleanKnownPathAlreadyClean(t *testing.T)` | TestCleanKnownPathAlreadyClean verifies clean known path already clean. |
| [`TestCleanKnownPathWithRelative`](../src/something/directive_generator_unit_test.go#L147) | test | 147-152 | `func TestCleanKnownPathWithRelative(t *testing.T)` | TestCleanKnownPathWithRelative verifies clean known path with relative. |
| [`TestNewMacroEnvironmentNoParent`](../src/something/directive_generator_unit_test.go#L155) | test | 155-169 | `func TestNewMacroEnvironmentNoParent(t *testing.T)` | TestNewMacroEnvironmentNoParent verifies new macro environment no parent. |
| [`TestNewMacroEnvironmentWithParent`](../src/something/directive_generator_unit_test.go#L172) | test | 172-178 | `func TestNewMacroEnvironmentWithParent(t *testing.T)` | TestNewMacroEnvironmentWithParent verifies new macro environment with parent. |
| [`TestStringExpressionBasic`](../src/something/directive_generator_unit_test.go#L181) | test | 181-198 | `func TestStringExpressionBasic(t *testing.T)` | TestStringExpressionBasic verifies string expression basic. |
| [`TestExpressionFromRuntimeString`](../src/something/directive_generator_unit_test.go#L201) | test | 201-211 | `func TestExpressionFromRuntimeString(t *testing.T)` | TestExpressionFromRuntimeString verifies expression from runtime string. |
| [`TestExpressionFromRuntimeInteger`](../src/something/directive_generator_unit_test.go#L214) | test | 214-224 | `func TestExpressionFromRuntimeInteger(t *testing.T)` | TestExpressionFromRuntimeInteger verifies expression from runtime integer. |
| [`TestExpressionFromRuntimeFloat`](../src/something/directive_generator_unit_test.go#L227) | test | 227-237 | `func TestExpressionFromRuntimeFloat(t *testing.T)` | TestExpressionFromRuntimeFloat verifies expression from runtime float. |
| [`TestExpressionFromRuntimeBool`](../src/something/directive_generator_unit_test.go#L240) | test | 240-250 | `func TestExpressionFromRuntimeBool(t *testing.T)` | TestExpressionFromRuntimeBool verifies expression from runtime bool. |
| [`TestExpressionFromRuntimeArray`](../src/something/directive_generator_unit_test.go#L253) | test | 253-263 | `func TestExpressionFromRuntimeArray(t *testing.T)` | TestExpressionFromRuntimeArray verifies expression from runtime array. |
| [`TestConstantStackPushPopLookup`](../src/something/directive_generator_unit_test.go#L266) | test | 266-305 | `func TestConstantStackPushPopLookup(t *testing.T)` | TestConstantStackPushPopLookup verifies constant stack push pop lookup. |
| [`TestExpandSetupDefinitionWithoutDefaults`](../src/something/directive_generator_unit_test.go#L308) | test | 308-329 | `func TestExpandSetupDefinitionWithoutDefaults(t *testing.T)` | TestExpandSetupDefinitionWithoutDefaults verifies expand setup definition without defaults. |
| [`TestExpandSetupDefinitionWithDefaults`](../src/something/directive_generator_unit_test.go#L332) | test | 332-354 | `func TestExpandSetupDefinitionWithDefaults(t *testing.T)` | TestExpandSetupDefinitionWithDefaults verifies expand setup definition with defaults. |
| [`TestExpandEnumDefinitionWithoutTaggedValues`](../src/something/directive_generator_unit_test.go#L357) | test | 357-375 | `func TestExpandEnumDefinitionWithoutTaggedValues(t *testing.T)` | TestExpandEnumDefinitionWithoutTaggedValues verifies expand enum definition without tagged values. |
| [`TestExpandEnumDefinitionWithTaggedValues`](../src/something/directive_generator_unit_test.go#L378) | test | 378-393 | `func TestExpandEnumDefinitionWithTaggedValues(t *testing.T)` | TestExpandEnumDefinitionWithTaggedValues verifies expand enum definition with tagged values. |
| [`TestExpandAccessesFieldAccess`](../src/something/directive_generator_unit_test.go#L396) | test | 396-410 | `func TestExpandAccessesFieldAccess(t *testing.T)` | TestExpandAccessesFieldAccess verifies expand accesses field access. |
| [`TestExpandAccessesIndexAccess`](../src/something/directive_generator_unit_test.go#L413) | test | 413-431 | `func TestExpandAccessesIndexAccess(t *testing.T)` | TestExpandAccessesIndexAccess verifies expand accesses index access. |
| [`TestRuntimeValueAssignableString`](../src/something/directive_generator_unit_test.go#L434) | test | 434-445 | `func TestRuntimeValueAssignableString(t *testing.T)` | TestRuntimeValueAssignableString verifies runtime value assignable string. |
| [`TestRuntimeValueAssignableInteger`](../src/something/directive_generator_unit_test.go#L448) | test | 448-456 | `func TestRuntimeValueAssignableInteger(t *testing.T)` | TestRuntimeValueAssignableInteger verifies runtime value assignable integer. |
| [`TestRuntimeValueAssignableBoolean`](../src/something/directive_generator_unit_test.go#L459) | test | 459-467 | `func TestRuntimeValueAssignableBoolean(t *testing.T)` | TestRuntimeValueAssignableBoolean verifies runtime value assignable boolean. |
| [`TestRuntimeValueAssignableFloat`](../src/something/directive_generator_unit_test.go#L470) | test | 470-481 | `func TestRuntimeValueAssignableFloat(t *testing.T)` | TestRuntimeValueAssignableFloat verifies runtime value assignable float. |
| [`TestExpandLValueIdentifier`](../src/something/directive_generator_unit_test.go#L484) | test | 484-495 | `func TestExpandLValueIdentifier(t *testing.T)` | TestExpandLValueIdentifier verifies expand l value identifier. |
| [`TestExpandLValueIteration`](../src/something/directive_generator_unit_test.go#L498) | test | 498-509 | `func TestExpandLValueIteration(t *testing.T)` | TestExpandLValueIteration verifies expand l value iteration. |
| [`TestExpandLValueIterationWithLabel`](../src/something/directive_generator_unit_test.go#L512) | test | 512-524 | `func TestExpandLValueIterationWithLabel(t *testing.T)` | TestExpandLValueIterationWithLabel verifies expand l value iteration with label. |
| [`TestDirectiveGeneratorErrPanic`](../src/something/directive_generator_unit_test.go#L527) | test | 527-532 | `func TestDirectiveGeneratorErrPanic(t *testing.T)` | TestDirectiveGeneratorErrPanic verifies directive generator err panic. |
| [`TestMacroEnvironmentLookupWithParent`](../src/something/directive_generator_unit_test.go#L535) | test | 535-555 | `func TestMacroEnvironmentLookupWithParent(t *testing.T)` | TestMacroEnvironmentLookupWithParent verifies macro environment lookup with parent. |
| [`TestIncludeCycleError`](../src/something/directive_generator_unit_test.go#L558) | test | 558-565 | `func TestIncludeCycleError(t *testing.T)` | TestIncludeCycleError verifies include cycle error. |
| [`TestExpandAccessesEmpty`](../src/something/directive_generator_unit_test.go#L568) | test | 568-574 | `func TestExpandAccessesEmpty(t *testing.T)` | TestExpandAccessesEmpty verifies expand accesses empty. |
| [`TestRuntimeValueAssignableNilType`](../src/something/directive_generator_unit_test.go#L577) | test | 577-583 | `func TestRuntimeValueAssignableNilType(t *testing.T)` | TestRuntimeValueAssignableNilType verifies runtime value assignable nil type. |
| [`TestExpandAccessesPreservesLocation`](../src/something/directive_generator_unit_test.go#L586) | test | 586-598 | `func TestExpandAccessesPreservesLocation(t *testing.T)` | TestExpandAccessesPreservesLocation verifies expand accesses preserves location. |
| [`TestExpandEnumDefinitionCopySemantics`](../src/something/directive_generator_unit_test.go#L601) | test | 601-617 | `func TestExpandEnumDefinitionCopySemantics(t *testing.T)` | TestExpandEnumDefinitionCopySemantics verifies enum expansion does not share member pointers with its input. |
| [`TestExpandSetupDefinitionCopySemantics`](../src/something/directive_generator_unit_test.go#L620) | test | 620-631 | `func TestExpandSetupDefinitionCopySemantics(t *testing.T)` | TestExpandSetupDefinitionCopySemantics verifies setup expansion copies field definitions. |
| [`TestPrimitiveKindIdentity`](../src/something/directive_generator_unit_test.go#L634) | test | 634-641 | `func TestPrimitiveKindIdentity(t *testing.T)` | TestPrimitiveKindIdentity verifies primitive kinds compare by value. |
| [`TestPrimitiveKindTypeAssertion`](../src/something/directive_generator_unit_test.go#L644) | test | 644-654 | `func TestPrimitiveKindTypeAssertion(t *testing.T)` | TestPrimitiveKindTypeAssertion verifies primitive kinds retain their concrete TypeRef identity. |
| [`TestLookupConstantReturnsFalseOnEmptyStack`](../src/something/directive_generator_unit_test.go#L657) | test | 657-667 | `func TestLookupConstantReturnsFalseOnEmptyStack(t *testing.T)` | TestLookupConstantReturnsFalseOnEmptyStack verifies lookup constant returns false on empty stack. |
| [`TestLookupConstantScansTopToBottom`](../src/something/directive_generator_unit_test.go#L670) | test | 670-682 | `func TestLookupConstantScansTopToBottom(t *testing.T)` | TestLookupConstantScansTopToBottom verifies lookup constant scans top to bottom. |

### [`src/something/errors.go`](../src/something/errors.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`SourceLocation`](../src/something/errors.go#L12) | struct | 12-16 | `type SourceLocation struct { Line int Col int Filepath string }` | SourceLocation identifies a source position in a .something file. |
| [`SomethingError`](../src/something/errors.go#L19) | struct | 19-25 | `type SomethingError struct { Message string Line int Col int Filepath string Suggestion string }` | SomethingError is a language error with source location and an optional fix. |
| [`(*SomethingError).Error`](../src/something/errors.go#L28) | method | 28-44 | `func (*SomethingError).Error() string` | Error returns the receiver's diagnostic message. |
| [`errAt`](../src/something/errors.go#L47) | function | 47-49 | `func errAt(msg string, tok Token, filepath string, suggestion string) *SomethingError` | errAt constructs a SomethingError from token coordinates. |
| [`errLoc`](../src/something/errors.go#L52) | function | 52-60 | `func errLoc(msg string, loc *SourceLocation, filepath string, suggestion string) *SomethingError` | errLoc constructs a SomethingError from an optional source location. |

### [`src/something/errors_functional_test.go`](../src/something/errors_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestUndefinedVariableError`](../src/something/errors_functional_test.go#L14) | test | 14-18 | `func TestUndefinedVariableError(t *testing.T)` | TestUndefinedVariableError verifies undefined variable error. |
| [`TestTypeMismatchStringToInteger`](../src/something/errors_functional_test.go#L21) | test | 21-25 | `func TestTypeMismatchStringToInteger(t *testing.T)` | TestTypeMismatchStringToInteger verifies type mismatch string to integer. |
| [`TestTypeMismatchIntegerToString`](../src/something/errors_functional_test.go#L28) | test | 28-32 | `func TestTypeMismatchIntegerToString(t *testing.T)` | TestTypeMismatchIntegerToString verifies type mismatch integer to string. |
| [`TestTypeMismatchBooleanToString`](../src/something/errors_functional_test.go#L35) | test | 35-39 | `func TestTypeMismatchBooleanToString(t *testing.T)` | TestTypeMismatchBooleanToString verifies type mismatch boolean to string. |
| [`TestStructMissingRequiredField`](../src/something/errors_functional_test.go#L42) | test | 42-46 | `func TestStructMissingRequiredField(t *testing.T)` | TestStructMissingRequiredField verifies struct missing required field. |
| [`TestStructUnknownField`](../src/something/errors_functional_test.go#L49) | test | 49-53 | `func TestStructUnknownField(t *testing.T)` | TestStructUnknownField verifies struct unknown field. |
| [`TestStructTypeMismatch`](../src/something/errors_functional_test.go#L56) | test | 56-60 | `func TestStructTypeMismatch(t *testing.T)` | TestStructTypeMismatch verifies struct type mismatch. |
| [`TestUnknownSetupType`](../src/something/errors_functional_test.go#L63) | test | 63-67 | `func TestUnknownSetupType(t *testing.T)` | TestUnknownSetupType verifies unknown setup type. |
| [`TestArrayOutOfBounds`](../src/something/errors_functional_test.go#L70) | test | 70-74 | `func TestArrayOutOfBounds(t *testing.T)` | TestArrayOutOfBounds verifies array out of bounds. |
| [`TestInvalidTimestampFormat`](../src/something/errors_functional_test.go#L77) | test | 77-81 | `func TestInvalidTimestampFormat(t *testing.T)` | TestInvalidTimestampFormat verifies invalid timestamp format. |
| [`TestAsLvalueRequiresNonEmptyString`](../src/something/errors_functional_test.go#L84) | test | 84-88 | `func TestAsLvalueRequiresNonEmptyString(t *testing.T)` | TestAsLvalueRequiresNonEmptyString verifies as lvalue requires non empty string. |
| [`TestAsLvalueFromNonStringVariable`](../src/something/errors_functional_test.go#L91) | test | 91-95 | `func TestAsLvalueFromNonStringVariable(t *testing.T)` | TestAsLvalueFromNonStringVariable verifies as lvalue from non string variable. |
| [`TestForSourceMustBeArray`](../src/something/errors_functional_test.go#L98) | test | 98-102 | `func TestForSourceMustBeArray(t *testing.T)` | TestForSourceMustBeArray verifies for source must be array. |
| [`TestInsertContentMustBeString`](../src/something/errors_functional_test.go#L105) | test | 105-109 | `func TestInsertContentMustBeString(t *testing.T)` | TestInsertContentMustBeString verifies insert content must be string. |
| [`TestEnumValueAccessOnPlainEnum`](../src/something/errors_functional_test.go#L112) | test | 112-116 | `func TestEnumValueAccessOnPlainEnum(t *testing.T)` | TestEnumValueAccessOnPlainEnum verifies enum value access on plain enum. |
| [`TestUndefinedFieldInDotPath`](../src/something/errors_functional_test.go#L119) | test | 119-123 | `func TestUndefinedFieldInDotPath(t *testing.T)` | TestUndefinedFieldInDotPath verifies undefined field in dot path. |
| [`TestDotPathOnNonDict`](../src/something/errors_functional_test.go#L126) | test | 126-130 | `func TestDotPathOnNonDict(t *testing.T)` | TestDotPathOnNonDict verifies dot path on non dict. |
| [`TestUndefinedVariableInMappingKey`](../src/something/errors_functional_test.go#L133) | test | 133-137 | `func TestUndefinedVariableInMappingKey(t *testing.T)` | TestUndefinedVariableInMappingKey verifies undefined variable in mapping key. |
| [`TestMappingKeyNotFound`](../src/something/errors_functional_test.go#L140) | test | 140-144 | `func TestMappingKeyNotFound(t *testing.T)` | TestMappingKeyNotFound verifies mapping key not found. |
| [`TestArrayTypeMismatchInElement`](../src/something/errors_functional_test.go#L147) | test | 147-151 | `func TestArrayTypeMismatchInElement(t *testing.T)` | TestArrayTypeMismatchInElement verifies array type mismatch in element. |
| [`TestMappingValueTypeMismatch`](../src/something/errors_functional_test.go#L154) | test | 154-158 | `func TestMappingValueTypeMismatch(t *testing.T)` | TestMappingValueTypeMismatch verifies mapping value type mismatch. |
| [`TestUndefinedTypeReference`](../src/something/errors_functional_test.go#L161) | test | 161-165 | `func TestUndefinedTypeReference(t *testing.T)` | TestUndefinedTypeReference verifies undefined type reference. |
| [`TestSuccessfulEvaluationNoPanic`](../src/something/errors_functional_test.go#L168) | test | 168-174 | `func TestSuccessfulEvaluationNoPanic(t *testing.T)` | TestSuccessfulEvaluationNoPanic verifies successful evaluation no panic. |
| [`TestParseErrorHasLocation`](../src/something/errors_functional_test.go#L177) | test | 177-192 | `func TestParseErrorHasLocation(t *testing.T)` | TestParseErrorHasLocation verifies parse error has location. |
| [`TestScopeTypeMismatch`](../src/something/errors_functional_test.go#L195) | test | 195-199 | `func TestScopeTypeMismatch(t *testing.T)` | TestScopeTypeMismatch verifies scope type mismatch. |
| [`TestNamespaceTypeMismatch`](../src/something/errors_functional_test.go#L202) | test | 202-206 | `func TestNamespaceTypeMismatch(t *testing.T)` | TestNamespaceTypeMismatch verifies namespace type mismatch. |
| [`TestLoadFileNotFound`](../src/something/errors_functional_test.go#L209) | test | 209-217 | `func TestLoadFileNotFound(t *testing.T)` | TestLoadFileNotFound verifies load file not found. |
| [`TestIterationTypeMismatch`](../src/something/errors_functional_test.go#L220) | test | 220-224 | `func TestIterationTypeMismatch(t *testing.T)` | TestIterationTypeMismatch verifies iteration type mismatch. |
| [`TestAsLvalueTypeMismatch`](../src/something/errors_functional_test.go#L227) | test | 227-231 | `func TestAsLvalueTypeMismatch(t *testing.T)` | TestAsLvalueTypeMismatch verifies as lvalue type mismatch. |
| [`TestInsertLexError`](../src/something/errors_functional_test.go#L234) | test | 234-238 | `func TestInsertLexError(t *testing.T)` | TestInsertLexError verifies insert lex error. |
| [`TestForSourceMustBeArrayOrMapping`](../src/something/errors_functional_test.go#L241) | test | 241-245 | `func TestForSourceMustBeArrayOrMapping(t *testing.T)` | TestForSourceMustBeArrayOrMapping verifies for source must be array or mapping. |

### [`src/something/errors_unit_test.go`](../src/something/errors_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvalErrorLocation`](../src/something/errors_unit_test.go#L14) | test | 14-33 | `func TestEvalErrorLocation(t *testing.T)` | TestEvalErrorLocation verifies eval error location. |
| [`TestErrAtHelper`](../src/something/errors_unit_test.go#L36) | test | 36-52 | `func TestErrAtHelper(t *testing.T)` | TestErrAtHelper verifies err at helper. |

### [`src/something/evaluator.go`](../src/something/evaluator.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`EnumValue`](../src/something/evaluator.go#L14) | struct | 14-17 | `type EnumValue struct { Ordinal int EnumName string }` | EnumValue preserves enum identity internally while expressions are evaluated. |
| [`runtimeBinding`](../src/something/evaluator.go#L20) | struct | 20-24 | `type runtimeBinding struct { value any typeRef TypeRef private bool }` | runtimeBinding stores one evaluated value with its type and visibility. |
| [`runtimeEnvironment`](../src/something/evaluator.go#L27) | struct | 27-31 | `type runtimeEnvironment struct { parent *runtimeEnvironment bindings map[string]*runtimeBinding types map[string]TypeRef }` | runtimeEnvironment stores lexically nested runtime bindings and named types. |
| [`newRuntimeEnvironment`](../src/something/evaluator.go#L34) | function | 34-40 | `func newRuntimeEnvironment(parent *runtimeEnvironment) *runtimeEnvironment` | newRuntimeEnvironment creates an empty runtime scope linked to its parent. |
| [`(*runtimeEnvironment).lookupBinding`](../src/something/evaluator.go#L43) | method | 43-50 | `func (*runtimeEnvironment).lookupBinding(name string) (*runtimeBinding, bool)` | lookupBinding searches the current and enclosing scopes for a value binding. |
| [`(*runtimeEnvironment).lookupType`](../src/something/evaluator.go#L53) | method | 53-60 | `func (*runtimeEnvironment).lookupType(name string) (TypeRef, bool)` | lookupType searches the current and enclosing scopes for a named type. |
| [`runtimeObject`](../src/something/evaluator.go#L63) | struct | 63-67 | `type runtimeObject struct { environment *runtimeEnvironment kind PrimitiveKind typeName string }` | runtimeObject carries scope, namespace, or setup fields in a nested runtime environment. |
| [`runtimeMapping`](../src/something/evaluator.go#L70) | struct | 70-74 | `type runtimeMapping struct { entries []runtimeMappingEntry keyType TypeRef valueType TypeRef }` | runtimeMapping preserves ordered key-value entries and their resolved types. |
| [`runtimeMappingEntry`](../src/something/evaluator.go#L77) | struct | 77-80 | `type runtimeMappingEntry struct { key any value any }` | runtimeMappingEntry stores one evaluated mapping key and value. |
| [`(*runtimeMapping).lookup`](../src/something/evaluator.go#L83) | method | 83-90 | `func (*runtimeMapping).lookup(key any) (any, bool)` | lookup returns the value associated with a runtime-equivalent mapping key. |
| [`(*runtimeMapping).update`](../src/something/evaluator.go#L93) | method | 93-101 | `func (*runtimeMapping).update(key, value any) bool` | update replaces the value for a runtime-equivalent mapping key and reports whether it existed. |
| [`runtimeKeysEqual`](../src/something/evaluator.go#L104) | function | 104-119 | `func runtimeKeysEqual(left, right any) bool` | runtimeKeysEqual compares evaluated mapping keys while preserving enum ordinal compatibility. |
| [`Evaluator`](../src/something/evaluator.go#L122) | struct | 122-126 | `type Evaluator struct { checked *CheckedProgram filepath string state *runtimeState }` | Evaluator evaluates a checked program into its public configuration map. |
| [`runtimeState`](../src/something/evaluator.go#L129) | struct | 129-136 | `type runtimeState struct { filepath string root *runtimeEnvironment current *runtimeEnvironment types map[*Assignment]TypeRef assertions map[string][]Statement // type name -> assertion bodies instanceLocation *SourceLocation // set when evaluating assertions for a specific instance }` | runtimeState owns evaluator scopes, checked assignment types, assertions, and source context. |
| [`newRuntimeState`](../src/something/evaluator.go#L139) | function | 139-142 | `func newRuntimeState(filepath string, types map[*Assignment]TypeRef) *runtimeState` | newRuntimeState creates evaluator state with an empty root environment. |
| [`NewEvaluator`](../src/something/evaluator.go#L145) | function | 145-150 | `func NewEvaluator(checked *CheckedProgram, filepath string) *Evaluator` | NewEvaluator constructs evaluator. |
| [`(*Evaluator).evaluate`](../src/something/evaluator.go#L153) | method | 153-159 | `func (*Evaluator).evaluate() map[string]any` | evaluate executes a checked program and returns its public configuration map. |
| [`(*runtimeState).err`](../src/something/evaluator.go#L162) | method | 162-164 | `func (*runtimeState).err(message string, location *SourceLocation, suggestion string)` | err panics with a source-located SomethingError for a runtime semantic failure. |
| [`(*runtimeState).evaluateStatements`](../src/something/evaluator.go#L167) | method | 167-182 | `func (*runtimeState).evaluateStatements(statements []Statement)` | evaluateStatements evaluates statements against the current evaluator state. |
| [`(*runtimeState).evaluateAssertDirective`](../src/something/evaluator.go#L185) | method | 185-197 | `func (*runtimeState).evaluateAssertDirective(assertion *AssertDirective)` | evaluateAssertDirective evaluates assert directive against the current evaluator state. |
| [`(*runtimeState).evaluateIfDirective`](../src/something/evaluator.go#L200) | method | 200-209 | `func (*runtimeState).evaluateIfDirective(ifDir *IfDirective)` | evaluateIfDirective evaluates if directive against the current evaluator state. |
| [`(*runtimeState).evaluateErrorDirective`](../src/something/evaluator.go#L212) | method | 212-224 | `func (*runtimeState).evaluateErrorDirective(errDir *ErrorDirective)` | evaluateErrorDirective evaluates error directive against the current evaluator state. |
| [`(*runtimeState).evaluateAssignment`](../src/something/evaluator.go#L227) | method | 227-248 | `func (*runtimeState).evaluateAssignment(assignment *Assignment)` | evaluateAssignment evaluates assignment against the current evaluator state. |
| [`(*runtimeState).evaluateEnumDefinition`](../src/something/evaluator.go#L251) | method | 251-266 | `func (*runtimeState).evaluateEnumDefinition(assignment *Assignment, definition *EnumDefinition)` | evaluateEnumDefinition evaluates enum definition against the current evaluator state. |
| [`(*runtimeState).evaluateSetupDefinition`](../src/something/evaluator.go#L269) | method | 269-280 | `func (*runtimeState).evaluateSetupDefinition(assignment *Assignment, definition *SetupDefinition)` | evaluateSetupDefinition evaluates setup definition against the current evaluator state. |
| [`requireIdentifierTarget`](../src/something/evaluator.go#L283) | function | 283-289 | `func requireIdentifierTarget(state *runtimeState, target LValue, location *SourceLocation) string` | requireIdentifierTarget requires a valid identifier target value. |
| [`(*runtimeState).evaluateObjectAssignment`](../src/something/evaluator.go#L292) | method | 292-304 | `func (*runtimeState).evaluateObjectAssignment(assignment *Assignment, statements []Statement, kind PrimitiveKind)` | evaluateObjectAssignment evaluates object assignment against the current evaluator state. |
| [`(*runtimeState).assignValue`](../src/something/evaluator.go#L307) | method | 307-326 | `func (*runtimeState).assignValue(assignment *Assignment, value any, typeRef TypeRef)` | assignValue declares or reassigns an evaluated assignment target. |
| [`(*runtimeState).declareMember`](../src/something/evaluator.go#L329) | method | 329-344 | `func (*runtimeState).declareMember(target *MemberLValue, value any, typeRef TypeRef, private bool, location *SourceLocation)` | declareMember adds a previously undeclared field to a scope or namespace value. |
| [`(*runtimeState).reassign`](../src/something/evaluator.go#L347) | method | 347-390 | `func (*runtimeState).reassign(target LValue, value any, location *SourceLocation)` | reassign replaces an existing variable, field, array element, or mapping entry. |
| [`(*runtimeState).resolveLValueContainer`](../src/something/evaluator.go#L393) | method | 393-407 | `func (*runtimeState).resolveLValueContainer(root string, accesses []Access, location *SourceLocation) (any, TypeRef, Access)` | resolveLValueContainer resolves l value container from the supplied context. |
| [`(*runtimeState).evaluateExpression`](../src/something/evaluator.go#L410) | method | 410-455 | `func (*runtimeState).evaluateExpression(expression Expression, expected TypeRef) any` | evaluateExpression evaluates expression against the current evaluator state. |
| [`(*runtimeState).evaluateString`](../src/something/evaluator.go#L458) | method | 458-506 | `func (*runtimeState).evaluateString(expression *StringExpression) string` | evaluateString evaluates string against the current evaluator state. |
| [`(*runtimeState).evaluateInterpolationReference`](../src/something/evaluator.go#L509) | method | 509-512 | `func (*runtimeState).evaluateInterpolationReference(name string, location *SourceLocation) any` | evaluateInterpolationReference evaluates interpolation reference against the current evaluator state. |
| [`(*runtimeState).evaluateBinaryOp`](../src/something/evaluator.go#L515) | method | 515-567 | `func (*runtimeState).evaluateBinaryOp(expression *BinaryOpExpression) any` | evaluateBinaryOp evaluates binary op against the current evaluator state. |
| [`(*runtimeState).evaluateUnaryOp`](../src/something/evaluator.go#L570) | method | 570-577 | `func (*runtimeState).evaluateUnaryOp(expression *UnaryOpExpression) any` | evaluateUnaryOp evaluates unary op against the current evaluator state. |
| [`(*runtimeState).evaluateMatch`](../src/something/evaluator.go#L580) | method | 580-596 | `func (*runtimeState).evaluateMatch(expression *MatchExpression) any` | evaluateMatch evaluates match against the current evaluator state. |
| [`(*runtimeState).evaluateLen`](../src/something/evaluator.go#L599) | method | 599-610 | `func (*runtimeState).evaluateLen(expression *LenExpression) any` | evaluateLen evaluates len against the current evaluator state. |
| [`runtimeValuesEqual`](../src/something/evaluator.go#L613) | function | 613-640 | `func runtimeValuesEqual(left, right any) bool` | runtimeValuesEqual compares two runtime values for equality. |
| [`compareValues`](../src/something/evaluator.go#L643) | function | 643-695 | `func compareValues(left, right any, location *SourceLocation, state *runtimeState) int` | compareValues performs ordered comparison returning -1/0/1. |
| [`dottedReference`](../src/something/evaluator.go#L698) | function | 698-705 | `func dottedReference(path string, location *SourceLocation) (string, []Access)` | dottedReference converts a dotted path into a root name and field-access sequence. |
| [`(*runtimeState).evaluateReference`](../src/something/evaluator.go#L708) | method | 708-751 | `func (*runtimeState).evaluateReference(reference *ReferenceExpression, expected TypeRef) any` | evaluateReference evaluates reference against the current evaluator state. |
| [`(*runtimeState).enumMember`](../src/something/evaluator.go#L754) | method | 754-762 | `func (*runtimeState).enumMember(enumType *EnumType, member string, location *SourceLocation) *EnumValue` | enumMember returns the typed ordinal for a named enum member or raises a diagnostic. |
| [`(*runtimeState).expectedEnum`](../src/something/evaluator.go#L765) | method | 765-769 | `func (*runtimeState).expectedEnum(expected TypeRef) *EnumType` | expectedEnum resolves an expected type and returns it only when it is an enum. |
| [`(*runtimeState).resolveAccess`](../src/something/evaluator.go#L772) | method | 772-775 | `func (*runtimeState).resolveAccess(current any, access Access, location *SourceLocation) any` | resolveAccess resolves access from the supplied context. |
| [`(*runtimeState).resolveTypedAccess`](../src/something/evaluator.go#L778) | method | 778-833 | `func (*runtimeState).resolveTypedAccess(current any, currentType TypeRef, access Access, location *SourceLocation) (any, TypeRef)` | resolveTypedAccess resolves typed access from the supplied context. |
| [`(*runtimeState).collectionIndexType`](../src/something/evaluator.go#L836) | method | 836-850 | `func (*runtimeState).collectionIndexType(typeRef TypeRef, location *SourceLocation) TypeRef` | collectionIndexType returns the accepted index or key type of a collection. |
| [`(*runtimeState).collectionElementType`](../src/something/evaluator.go#L853) | method | 853-863 | `func (*runtimeState).collectionElementType(typeRef TypeRef, location *SourceLocation) TypeRef` | collectionElementType returns the element or value type of an array, enum-keyed array, or mapping. |
| [`(*runtimeState).integerIndex`](../src/something/evaluator.go#L866) | method | 866-876 | `func (*runtimeState).integerIndex(index any, location *SourceLocation) int` | integerIndex converts an integer or enum value to an array index. |
| [`(*runtimeState).evaluateArray`](../src/something/evaluator.go#L879) | method | 879-897 | `func (*runtimeState).evaluateArray(expression *ArrayExpression, expected TypeRef) any` | evaluateArray evaluates array against the current evaluator state. |
| [`(*runtimeState).evaluateMapping`](../src/something/evaluator.go#L900) | method | 900-932 | `func (*runtimeState).evaluateMapping(expression *MappingExpression, expected TypeRef) any` | evaluateMapping evaluates mapping against the current evaluator state. |
| [`(*runtimeState).evaluateStruct`](../src/something/evaluator.go#L935) | method | 935-997 | `func (*runtimeState).evaluateStruct(expression *StructExpression, expected TypeRef) any` | evaluateStruct evaluates struct against the current evaluator state. |
| [`(*runtimeState).resolveType`](../src/something/evaluator.go#L1000) | method | 1000-1020 | `func (*runtimeState).resolveType(typeRef TypeRef, location *SourceLocation) TypeRef` | resolveType resolves type from the supplied context. |
| [`(*runtimeState).runtimeType`](../src/something/evaluator.go#L1023) | method | 1023-1070 | `func (*runtimeState).runtimeType(value any) TypeRef` | runtimeType derives a SOMETHING type reference from an evaluated value. |
| [`(*runtimeState).publicMap`](../src/something/evaluator.go#L1073) | method | 1073-1082 | `func (*runtimeState).publicMap(environment *runtimeEnvironment) map[string]any` | publicMap materializes the non-private bindings in a runtime environment. |
| [`materializeValue`](../src/something/evaluator.go#L1085) | function | 1085-1138 | `func materializeValue(value any, includePrivate bool) any` | materializeValue recursively converts internal runtime values to public Go values. |
| [`runtimeTypeName`](../src/something/evaluator.go#L1141) | function | 1141-1162 | `func runtimeTypeName(value any) string` | runtimeTypeName returns the user-facing type name of an evaluated value. |
| [`sortedRuntimeBindingKeys`](../src/something/evaluator.go#L1165) | function | 1165-1172 | `func sortedRuntimeBindingKeys(values map[string]*runtimeBinding) []string` | sortedRuntimeBindingKeys returns runtime binding keys in deterministic order. |
| [`sortedMapKeys`](../src/something/evaluator.go#L1175) | function | 1175-1179 | `func sortedMapKeys(values map[string]any) []string` | sortedMapKeys returns map keys in deterministic order. |
| [`mapKeys`](../src/something/evaluator.go#L1182) | function | 1182-1188 | `func mapKeys(values map[string]any) []string` | mapKeys returns all keys from a string-keyed runtime map. |
| [`sortedFieldDefinitionKeys`](../src/something/evaluator.go#L1191) | function | 1191-1198 | `func sortedFieldDefinitionKeys(values map[string]*FieldDefinition) []string` | sortedFieldDefinitionKeys returns field definition keys in deterministic order. |
| [`isValidTimestamp`](../src/something/evaluator.go#L1201) | function | 1201-1224 | `func isValidTimestamp(value string) bool` | isValidTimestamp reports whether a string matches the supported timestamp shape. |
| [`typeNameOf`](../src/something/evaluator.go#L1227) | function | 1227-1229 | `func typeNameOf(value any) string` | typeNameOf returns the SOMETHING runtime type name for a value. |
| [`typeRefDisplayName`](../src/something/evaluator.go#L1232) | function | 1232-1237 | `func typeRefDisplayName(typeRef TypeRef) string` | typeRefDisplayName returns a diagnostic name for a type reference. |
| [`isAlphaNum`](../src/something/evaluator.go#L1240) | function | 1240-1242 | `func isAlphaNum(value byte) bool` | isAlphaNum reports whether a byte is an ASCII letter or digit. |
| [`indexOf`](../src/something/evaluator.go#L1245) | function | 1245-1252 | `func indexOf(values []string, expected string) int` | indexOf returns the first matching string index, or minus one when absent. |

### [`src/something/evaluator_functional_test.go`](../src/something/evaluator_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvalStringLiteral`](../src/something/evaluator_functional_test.go#L17) | test | 17-22 | `func TestEvalStringLiteral(t *testing.T)` | TestEvalStringLiteral verifies eval string literal. |
| [`TestEvalIntegerLiteral`](../src/something/evaluator_functional_test.go#L25) | test | 25-30 | `func TestEvalIntegerLiteral(t *testing.T)` | TestEvalIntegerLiteral verifies eval integer literal. |
| [`TestEvalFloatLiteral`](../src/something/evaluator_functional_test.go#L33) | test | 33-38 | `func TestEvalFloatLiteral(t *testing.T)` | TestEvalFloatLiteral verifies eval float literal. |
| [`TestEvalBooleanTrue`](../src/something/evaluator_functional_test.go#L41) | test | 41-46 | `func TestEvalBooleanTrue(t *testing.T)` | TestEvalBooleanTrue verifies eval boolean true. |
| [`TestEvalInterpolation`](../src/something/evaluator_functional_test.go#L51) | test | 51-56 | `func TestEvalInterpolation(t *testing.T)` | TestEvalInterpolation verifies eval interpolation. |
| [`TestEvalInterpolationInteger`](../src/something/evaluator_functional_test.go#L59) | test | 59-64 | `func TestEvalInterpolationInteger(t *testing.T)` | TestEvalInterpolationInteger verifies eval interpolation integer. |
| [`TestEvalInterpolationFloat`](../src/something/evaluator_functional_test.go#L67) | test | 67-72 | `func TestEvalInterpolationFloat(t *testing.T)` | TestEvalInterpolationFloat verifies eval interpolation float. |
| [`TestEvalInterpolationBoolean`](../src/something/evaluator_functional_test.go#L75) | test | 75-80 | `func TestEvalInterpolationBoolean(t *testing.T)` | TestEvalInterpolationBoolean verifies eval interpolation boolean. |
| [`TestEvalMultiline`](../src/something/evaluator_functional_test.go#L85) | test | 85-90 | `func TestEvalMultiline(t *testing.T)` | TestEvalMultiline verifies eval multiline. |
| [`TestEvalMultilineNoNewline`](../src/something/evaluator_functional_test.go#L93) | test | 93-98 | `func TestEvalMultilineNoNewline(t *testing.T)` | TestEvalMultilineNoNewline verifies eval multiline no newline. |
| [`TestEvalMultilineNoIndent`](../src/something/evaluator_functional_test.go#L101) | test | 101-106 | `func TestEvalMultilineNoIndent(t *testing.T)` | TestEvalMultilineNoIndent verifies eval multiline no indent. |
| [`TestEvalMultilineStripSpaces`](../src/something/evaluator_functional_test.go#L109) | test | 109-114 | `func TestEvalMultilineStripSpaces(t *testing.T)` | TestEvalMultilineStripSpaces verifies eval multiline strip spaces. |
| [`TestEvalMultilineCombined`](../src/something/evaluator_functional_test.go#L117) | test | 117-122 | `func TestEvalMultilineCombined(t *testing.T)` | TestEvalMultilineCombined verifies eval multiline combined. |
| [`TestEvalMultilineInterpolation`](../src/something/evaluator_functional_test.go#L125) | test | 125-130 | `func TestEvalMultilineInterpolation(t *testing.T)` | TestEvalMultilineInterpolation verifies eval multiline interpolation. |
| [`TestEvalMultilineInterpolationDotPath`](../src/something/evaluator_functional_test.go#L133) | test | 133-138 | `func TestEvalMultilineInterpolationDotPath(t *testing.T)` | TestEvalMultilineInterpolationDotPath verifies eval multiline interpolation dot path. |
| [`TestEvalMultilineWithAllParams`](../src/something/evaluator_functional_test.go#L141) | test | 141-146 | `func TestEvalMultilineWithAllParams(t *testing.T)` | TestEvalMultilineWithAllParams verifies eval multiline with all params. |
| [`TestEvalArray`](../src/something/evaluator_functional_test.go#L151) | test | 151-163 | `func TestEvalArray(t *testing.T)` | TestEvalArray verifies eval array. |
| [`TestEvalArrayIndexAccess`](../src/something/evaluator_functional_test.go#L166) | test | 166-171 | `func TestEvalArrayIndexAccess(t *testing.T)` | TestEvalArrayIndexAccess verifies eval array index access. |
| [`TestEvalArrayOutOfBounds`](../src/something/evaluator_functional_test.go#L174) | test | 174-178 | `func TestEvalArrayOutOfBounds(t *testing.T)` | TestEvalArrayOutOfBounds verifies eval array out of bounds. |
| [`TestEvalMapping`](../src/something/evaluator_functional_test.go#L183) | test | 183-192 | `func TestEvalMapping(t *testing.T)` | TestEvalMapping verifies eval mapping. |
| [`TestEvalMappingKeyAccess`](../src/something/evaluator_functional_test.go#L195) | test | 195-200 | `func TestEvalMappingKeyAccess(t *testing.T)` | TestEvalMappingKeyAccess verifies eval mapping key access. |
| [`TestEvalCombinedFieldAndIndexAccess`](../src/something/evaluator_functional_test.go#L203) | test | 203-214 | `func TestEvalCombinedFieldAndIndexAccess(t *testing.T)` | TestEvalCombinedFieldAndIndexAccess verifies eval combined field and index access. |
| [`TestEvalQualifiedEnumMappingKey`](../src/something/evaluator_functional_test.go#L217) | test | 217-222 | `func TestEvalQualifiedEnumMappingKey(t *testing.T)` | TestEvalQualifiedEnumMappingKey verifies eval qualified enum mapping key. |
| [`TestEvalEmptyMappingUsesStringKeys`](../src/something/evaluator_functional_test.go#L225) | test | 225-230 | `func TestEvalEmptyMappingUsesStringKeys(t *testing.T)` | TestEvalEmptyMappingUsesStringKeys verifies eval empty mapping uses string keys. |
| [`TestEvalMappingEnumKeys`](../src/something/evaluator_functional_test.go#L233) | test | 233-242 | `func TestEvalMappingEnumKeys(t *testing.T)` | TestEvalMappingEnumKeys verifies eval mapping enum keys. |
| [`TestEvalMappingEnumKeyWithDotPrefix`](../src/something/evaluator_functional_test.go#L245) | test | 245-255 | `func TestEvalMappingEnumKeyWithDotPrefix(t *testing.T)` | TestEvalMappingEnumKeyWithDotPrefix verifies eval mapping enum key with dot prefix. |
| [`TestEvalMappingKeyAccessEnumIndex`](../src/something/evaluator_functional_test.go#L258) | test | 258-263 | `func TestEvalMappingKeyAccessEnumIndex(t *testing.T)` | TestEvalMappingKeyAccessEnumIndex verifies eval mapping key access enum index. |
| [`TestEvalMappingCompositeKeys`](../src/something/evaluator_functional_test.go#L266) | test | 266-272 | `func TestEvalMappingCompositeKeys(t *testing.T)` | TestEvalMappingCompositeKeys verifies eval mapping composite keys. |
| [`TestEvalMappingCompositeStringKeys`](../src/something/evaluator_functional_test.go#L275) | test | 275-282 | `func TestEvalMappingCompositeStringKeys(t *testing.T)` | TestEvalMappingCompositeStringKeys verifies eval mapping composite string keys. |
| [`TestEvalMappingStringKeyWithDot`](../src/something/evaluator_functional_test.go#L285) | test | 285-292 | `func TestEvalMappingStringKeyWithDot(t *testing.T)` | TestEvalMappingStringKeyWithDot verifies eval mapping string key with dot. |
| [`TestEvalEnumPlain`](../src/something/evaluator_functional_test.go#L297) | test | 297-302 | `func TestEvalEnumPlain(t *testing.T)` | TestEvalEnumPlain verifies eval enum plain. |
| [`TestEvalEnumShorthand`](../src/something/evaluator_functional_test.go#L305) | test | 305-310 | `func TestEvalEnumShorthand(t *testing.T)` | TestEvalEnumShorthand verifies eval enum shorthand. |
| [`TestEvalEnumTaggedValue`](../src/something/evaluator_functional_test.go#L313) | test | 313-318 | `func TestEvalEnumTaggedValue(t *testing.T)` | TestEvalEnumTaggedValue verifies eval enum tagged value. |
| [`TestEvalEnumTaggedStructValue`](../src/something/evaluator_functional_test.go#L321) | test | 321-330 | `func TestEvalEnumTaggedStructValue(t *testing.T)` | TestEvalEnumTaggedStructValue verifies eval enum tagged struct value. |
| [`TestEvalEnumIndexedVariable`](../src/something/evaluator_functional_test.go#L333) | test | 333-339 | `func TestEvalEnumIndexedVariable(t *testing.T)` | TestEvalEnumIndexedVariable verifies eval enum indexed variable. |
| [`TestEvalEnumQualifiedAccess`](../src/something/evaluator_functional_test.go#L342) | test | 342-347 | `func TestEvalEnumQualifiedAccess(t *testing.T)` | TestEvalEnumQualifiedAccess verifies eval enum qualified access. |
| [`TestEvalEnumValueViaStructField`](../src/something/evaluator_functional_test.go#L350) | test | 350-359 | `func TestEvalEnumValueViaStructField(t *testing.T)` | TestEvalEnumValueViaStructField verifies eval enum value via struct field. |
| [`TestEvalEnumValueNoValueType`](../src/something/evaluator_functional_test.go#L362) | test | 362-366 | `func TestEvalEnumValueNoValueType(t *testing.T)` | TestEvalEnumValueNoValueType verifies eval enum value no value type. |
| [`TestEvalEnumValueOrdinalInResult`](../src/something/evaluator_functional_test.go#L369) | test | 369-375 | `func TestEvalEnumValueOrdinalInResult(t *testing.T)` | TestEvalEnumValueOrdinalInResult verifies eval enum value ordinal in result. |
| [`TestEvalOptionalEnumDefaultSupportsTaggedValueAccess`](../src/something/evaluator_functional_test.go#L378) | test | 378-388 | `func TestEvalOptionalEnumDefaultSupportsTaggedValueAccess(t *testing.T)` | TestEvalOptionalEnumDefaultSupportsTaggedValueAccess verifies eval optional enum default supports tagged value access. |
| [`TestEvalStruct`](../src/something/evaluator_functional_test.go#L393) | test | 393-402 | `func TestEvalStruct(t *testing.T)` | TestEvalStruct verifies eval struct. |
| [`TestEvalNestedStruct`](../src/something/evaluator_functional_test.go#L405) | test | 405-412 | `func TestEvalNestedStruct(t *testing.T)` | TestEvalNestedStruct verifies eval nested struct. |
| [`TestEvalScope`](../src/something/evaluator_functional_test.go#L417) | test | 417-422 | `func TestEvalScope(t *testing.T)` | TestEvalScope verifies eval scope. |
| [`TestEvalScopePrivateVar`](../src/something/evaluator_functional_test.go#L425) | test | 425-433 | `func TestEvalScopePrivateVar(t *testing.T)` | TestEvalScopePrivateVar verifies eval scope private var. |
| [`TestEvalPrivateVar`](../src/something/evaluator_functional_test.go#L436) | test | 436-444 | `func TestEvalPrivateVar(t *testing.T)` | TestEvalPrivateVar verifies eval private var. |
| [`TestEvalScopeTwoPass`](../src/something/evaluator_functional_test.go#L447) | test | 447-453 | `func TestEvalScopeTwoPass(t *testing.T)` | TestEvalScopeTwoPass verifies eval scope two pass. |
| [`TestEvalScopeVarDeclNamedType`](../src/something/evaluator_functional_test.go#L456) | test | 456-462 | `func TestEvalScopeVarDeclNamedType(t *testing.T)` | TestEvalScopeVarDeclNamedType verifies eval scope var decl named type. |
| [`TestEvalScopeBodyNestedScope`](../src/something/evaluator_functional_test.go#L465) | test | 465-471 | `func TestEvalScopeBodyNestedScope(t *testing.T)` | TestEvalScopeBodyNestedScope verifies eval scope body nested scope. |
| [`TestEvalDotPath`](../src/something/evaluator_functional_test.go#L476) | test | 476-481 | `func TestEvalDotPath(t *testing.T)` | TestEvalDotPath verifies eval dot path. |
| [`TestEvalDotAccessEnumField`](../src/something/evaluator_functional_test.go#L484) | test | 484-489 | `func TestEvalDotAccessEnumField(t *testing.T)` | TestEvalDotAccessEnumField verifies eval dot access enum field. |
| [`TestEvalResolveDotPathUndefinedField`](../src/something/evaluator_functional_test.go#L492) | test | 492-496 | `func TestEvalResolveDotPathUndefinedField(t *testing.T)` | TestEvalResolveDotPathUndefinedField verifies eval resolve dot path undefined field. |
| [`TestEvalResolveDotPathNonDict`](../src/something/evaluator_functional_test.go#L499) | test | 499-503 | `func TestEvalResolveDotPathNonDict(t *testing.T)` | TestEvalResolveDotPathNonDict verifies eval resolve dot path non dict. |
| [`TestEvalResolveDotPathVariableKey`](../src/something/evaluator_functional_test.go#L506) | test | 506-512 | `func TestEvalResolveDotPathVariableKey(t *testing.T)` | TestEvalResolveDotPathVariableKey verifies eval resolve dot path variable key. |
| [`TestEvalResolveDotPathVariableKeyUndefined`](../src/something/evaluator_functional_test.go#L515) | test | 515-519 | `func TestEvalResolveDotPathVariableKeyUndefined(t *testing.T)` | TestEvalResolveDotPathVariableKeyUndefined verifies eval resolve dot path variable key undefined. |
| [`TestEvalUndefinedVar`](../src/something/evaluator_functional_test.go#L524) | test | 524-528 | `func TestEvalUndefinedVar(t *testing.T)` | TestEvalUndefinedVar verifies eval undefined var. |
| [`TestEvalVariableWithSemicolons`](../src/something/evaluator_functional_test.go#L531) | test | 531-536 | `func TestEvalVariableWithSemicolons(t *testing.T)` | TestEvalVariableWithSemicolons verifies eval variable with semicolons. |
| [`TestEvalNegativeInteger`](../src/something/evaluator_functional_test.go#L541) | test | 541-546 | `func TestEvalNegativeInteger(t *testing.T)` | TestEvalNegativeInteger verifies eval negative integer. |
| [`TestEvalNegativeFloat`](../src/something/evaluator_functional_test.go#L549) | test | 549-554 | `func TestEvalNegativeFloat(t *testing.T)` | TestEvalNegativeFloat verifies eval negative float. |
| [`TestParseIntLiteral`](../src/something/evaluator_functional_test.go#L559) | test | 559-566 | `func TestParseIntLiteral(t *testing.T)` | TestParseIntLiteral verifies parse int literal. |
| [`TestParseFloatLiteral`](../src/something/evaluator_functional_test.go#L569) | test | 569-576 | `func TestParseFloatLiteral(t *testing.T)` | TestParseFloatLiteral verifies parse float literal. |
| [`TestIsInt`](../src/something/evaluator_functional_test.go#L579) | test | 579-589 | `func TestIsInt(t *testing.T)` | TestIsInt verifies is int. |
| [`TestIsValidTimestampWithMicros`](../src/something/evaluator_functional_test.go#L592) | test | 592-597 | `func TestIsValidTimestampWithMicros(t *testing.T)` | TestIsValidTimestampWithMicros verifies is valid timestamp with micros. |
| [`TestMapKeysErrorPath`](../src/something/evaluator_functional_test.go#L600) | test | 600-605 | `func TestMapKeysErrorPath(t *testing.T)` | TestMapKeysErrorPath verifies map keys error path. |
| [`TestTypeNameOfAll`](../src/something/evaluator_functional_test.go#L610) | test | 610-624 | `func TestTypeNameOfAll(t *testing.T)` | TestTypeNameOfAll verifies type name of all. |
| [`TestTypeNameOfArray`](../src/something/evaluator_functional_test.go#L627) | test | 627-632 | `func TestTypeNameOfArray(t *testing.T)` | TestTypeNameOfArray verifies type name of array. |
| [`TestTypeNameOfMap`](../src/something/evaluator_functional_test.go#L635) | test | 635-640 | `func TestTypeNameOfMap(t *testing.T)` | TestTypeNameOfMap verifies type name of map. |
| [`TestTypeNameOfFloat`](../src/something/evaluator_functional_test.go#L643) | test | 643-648 | `func TestTypeNameOfFloat(t *testing.T)` | TestTypeNameOfFloat verifies type name of float. |
| [`TestEvalTypeNameOf`](../src/something/evaluator_functional_test.go#L651) | test | 651-656 | `func TestEvalTypeNameOf(t *testing.T)` | TestEvalTypeNameOf verifies eval type name of. |
| [`TestIsValidTimestampAll`](../src/something/evaluator_functional_test.go#L661) | test | 661-675 | `func TestIsValidTimestampAll(t *testing.T)` | TestIsValidTimestampAll verifies is valid timestamp all. |
| [`TestGetLocationIterationDecl`](../src/something/evaluator_functional_test.go#L680) | test | 680-685 | `func TestGetLocationIterationDecl(t *testing.T)` | TestGetLocationIterationDecl verifies get location iteration decl. |
| [`TestGetLocationAsLvalueDecl`](../src/something/evaluator_functional_test.go#L688) | test | 688-693 | `func TestGetLocationAsLvalueDecl(t *testing.T)` | TestGetLocationAsLvalueDecl verifies get location as lvalue decl. |
| [`TestValidExprKindsDefault`](../src/something/evaluator_functional_test.go#L698) | test | 698-705 | `func TestValidExprKindsDefault(t *testing.T)` | TestValidExprKindsDefault verifies valid expr kinds default. |
| [`TestValidExprKindsForTypeReturnNil`](../src/something/evaluator_functional_test.go#L708) | test | 708-715 | `func TestValidExprKindsForTypeReturnNil(t *testing.T)` | TestValidExprKindsForTypeReturnNil verifies valid expr kinds for type return nil. |
| [`TestEvalIterationPeekInScope`](../src/something/evaluator_functional_test.go#L720) | test | 720-731 | `func TestEvalIterationPeekInScope(t *testing.T)` | TestEvalIterationPeekInScope verifies eval iteration peek in scope. |

### [`src/something/evaluator_unit_test.go`](../src/something/evaluator_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestTypeRefDisplayNameDefault`](../src/something/evaluator_unit_test.go#L14) | test | 14-21 | `func TestTypeRefDisplayNameDefault(t *testing.T)` | TestTypeRefDisplayNameDefault verifies type ref display name default. |
| [`TestTypeRefDisplayNameTypeName`](../src/something/evaluator_unit_test.go#L24) | test | 24-31 | `func TestTypeRefDisplayNameTypeName(t *testing.T)` | TestTypeRefDisplayNameTypeName verifies type ref display name type name. |
| [`TestTypeNameOfDefault`](../src/something/evaluator_unit_test.go#L34) | test | 34-40 | `func TestTypeNameOfDefault(t *testing.T)` | TestTypeNameOfDefault verifies type name of default. |
| [`TestIndexOfNotFound`](../src/something/evaluator_unit_test.go#L43) | test | 43-49 | `func TestIndexOfNotFound(t *testing.T)` | TestIndexOfNotFound verifies index of not found. |

### [`src/something/helpers_functional_test.go`](../src/something/helpers_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestKindNameAll`](../src/something/helpers_functional_test.go#L12) | test | 12-25 | `func TestKindNameAll(t *testing.T)` | TestKindNameAll verifies kind name all. |
| [`TestKindNameFloat`](../src/something/helpers_functional_test.go#L28) | test | 28-32 | `func TestKindNameFloat(t *testing.T)` | TestKindNameFloat verifies kind name float. |

### [`src/something/helpers_test.go`](../src/something/helpers_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`testData`](../src/something/helpers_test.go#L14) | function | 14-41 | `func testData() map[string]any` | testData supports the package test suite's test data setup or assertions. |
| [`evalText`](../src/something/helpers_test.go#L44) | function | 44-52 | `func evalText(t *testing.T, text string) map[string]any` | evalText supports the package test suite's eval text setup or assertions. |
| [`evalTextErr`](../src/something/helpers_test.go#L55) | function | 55-71 | `func evalTextErr(t *testing.T, text string) string` | evalTextErr supports the package test suite's eval text err setup or assertions. |
| [`NodeKind`](../src/something/helpers_test.go#L74) | type | 74 | `type NodeKind int` | NodeKind identifies the compact test-only AST view used by parser assertions while production retains one source-ordered representation. |
| [`ValueNode`](../src/something/helpers_test.go#L91) | struct | 91-96 | `type ValueNode struct { Kind NodeKind Raw any Resolved any Location *SourceLocation }` | ValueNode is a fixture type used by the package test suite. |
| [`VarDecl`](../src/something/helpers_test.go#L99) | struct | 99-107 | `type VarDecl struct { Name string InferType bool ExplicitType string DeclaredType TypeRef Value *ValueNode Priv bool Location *SourceLocation }` | VarDecl is a fixture type used by the package test suite. |
| [`IterationDecl`](../src/something/helpers_test.go#L110) | struct | 110-118 | `type IterationDecl struct { IterationLabel Expression Value *ValueNode InferType bool ExplicitType string DeclaredType TypeRef Priv bool Location *SourceLocation }` | IterationDecl is a fixture type used by the package test suite. |
| [`AsLvalueDecl`](../src/something/helpers_test.go#L121) | struct | 121-129 | `type AsLvalueDecl struct { NameExpr *ValueNode Value *ValueNode InferType bool ExplicitType string DeclaredType TypeRef Priv bool Location *SourceLocation }` | AsLvalueDecl is a fixture type used by the package test suite. |
| [`ForDecl`](../src/something/helpers_test.go#L132) | struct | 132-137 | `type ForDecl struct { ElementName string KeyName string Source *ValueNode Body []any }` | ForDecl is a fixture type used by the package test suite. |
| [`InsertDecl`](../src/something/helpers_test.go#L140) | struct | 140-142 | `type InsertDecl struct { Contents []*ValueNode }` | InsertDecl is a fixture type used by the package test suite. |
| [`IncludeDecl`](../src/something/helpers_test.go#L145) | struct | 145-147 | `type IncludeDecl struct { Filepath string }` | IncludeDecl is a fixture type used by the package test suite. |
| [`ScopeDecl`](../src/something/helpers_test.go#L150) | struct | 150-152 | `type ScopeDecl struct { Body []any }` | ScopeDecl is a fixture type used by the package test suite. |
| [`parsedAssignmentView`](../src/something/helpers_test.go#L155) | struct | 155-158 | `type parsedAssignmentView struct { LValue any RValue any }` | parsedAssignmentView is a fixture type used by the package test suite. |
| [`MemberPair`](../src/something/helpers_test.go#L161) | struct | 161-164 | `type MemberPair struct { Name string Value any }` | MemberPair is a fixture type used by the package test suite. |
| [`EnumDecl`](../src/something/helpers_test.go#L167) | struct | 167-171 | `type EnumDecl struct { Name string ValueType TypeRef Members []MemberPair }` | EnumDecl is a fixture type used by the package test suite. |
| [`SetupDecl`](../src/something/helpers_test.go#L174) | struct | 174-177 | `type SetupDecl struct { Name string Fields []*FieldDefinition }` | SetupDecl is a fixture type used by the package test suite. |
| [`MacroDecl`](../src/something/helpers_test.go#L180) | struct | 180-184 | `type MacroDecl struct { Name string Params []MacroParam SetExpr *ValueNode }` | MacroDecl is a fixture type used by the package test suite. |
| [`(*IncludeDecl).Priv`](../src/something/helpers_test.go#L187) | method | 187 | `func (*IncludeDecl).Priv() bool` | Priv supplies IncludeDecl test-fixture behavior. |
| [`(*VarDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L190) | method | 190 | `func (*VarDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies VarDecl test-fixture behavior. |
| [`(*ForDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L193) | method | 193 | `func (*ForDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies ForDecl test-fixture behavior. |
| [`(*InsertDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L196) | method | 196 | `func (*InsertDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies InsertDecl test-fixture behavior. |
| [`(*IncludeDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L199) | method | 199 | `func (*IncludeDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies IncludeDecl test-fixture behavior. |
| [`(*IterationDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L202) | method | 202 | `func (*IterationDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies IterationDecl test-fixture behavior. |
| [`(*AsLvalueDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L205) | method | 205 | `func (*AsLvalueDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies AsLvalueDecl test-fixture behavior. |
| [`(*ScopeDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L208) | method | 208 | `func (*ScopeDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies ScopeDecl test-fixture behavior. |
| [`(*MacroDecl).scopeBodyItemMarker`](../src/something/helpers_test.go#L211) | method | 211 | `func (*MacroDecl).scopeBodyItemMarker()` | scopeBodyItemMarker supplies MacroDecl test-fixture behavior. |
| [`getLocation`](../src/something/helpers_test.go#L214) | function | 214-224 | `func getLocation(value any) *SourceLocation` | getLocation supports the package test suite's get location setup or assertions. |
| [`validExprKindsForType`](../src/something/helpers_test.go#L227) | function | 227 | `func validExprKindsForType(TypeRef) []NodeKind` | validExprKindsForType supports the package test suite's valid expr kinds for type setup or assertions. |
| [`kindName`](../src/something/helpers_test.go#L230) | function | 230-257 | `func kindName(kind NodeKind) string` | kindName supports the package test suite's kind name setup or assertions. |
| [`parsedProgramView`](../src/something/helpers_test.go#L260) | struct | 260-272 | `type parsedProgramView struct { Enums []*EnumDecl Setups []*SetupDecl TopLevelVars []*VarDecl TopLevelFors []*ForDecl TopLevelInserts []*InsertDecl TopLevelIncludes []*IncludeDecl TopLevelIterations []*IterationDecl TopLevelAsLvalues []*AsLvalueDecl TopLevelBareScopes []*ScopeDecl Scopes []*parsedAssignmentView Macros []*MacroDecl }` | parsedProgramView is a fixture type used by the package test suite. |
| [`parsedValue`](../src/something/helpers_test.go#L275) | function | 275-329 | `func parsedValue(expression Expression) *ValueNode` | parsedValue supports the package test suite's parsed value setup or assertions. |
| [`parsedReferencePath`](../src/something/helpers_test.go#L332) | function | 332-354 | `func parsedReferencePath(reference *ReferenceExpression) string` | parsedReferencePath supports the package test suite's parsed reference path setup or assertions. |
| [`stringLiteralToStringValue`](../src/something/helpers_test.go#L357) | function | 357-368 | `func stringLiteralToStringValue(expression *StringExpression) string` | stringLiteralToStringValue supports the package test suite's string literal to string value setup or assertions. |
| [`assignmentName`](../src/something/helpers_test.go#L371) | function | 371-376 | `func assignmentName(target LValue) string` | assignmentName supports the package test suite's assignment name setup or assertions. |
| [`explicitType`](../src/something/helpers_test.go#L379) | function | 379-393 | `func explicitType(assignment *Assignment) string` | explicitType supports the package test suite's explicit type setup or assertions. |
| [`parsedBody`](../src/something/helpers_test.go#L396) | function | 396-426 | `func parsedBody(statements []Statement) []any` | parsedBody supports the package test suite's parsed body setup or assertions. |
| [`parsedView`](../src/something/helpers_test.go#L429) | function | 429-476 | `func parsedView(program *Program) *parsedProgramView` | parsedView supports the package test suite's parsed view setup or assertions. |
| [`tokenize`](../src/something/helpers_test.go#L479) | function | 479-482 | `func tokenize(t *testing.T, text string) []Token` | tokenize supports the package test suite's tokenize setup or assertions. |
| [`tokenKinds`](../src/something/helpers_test.go#L485) | function | 485-493 | `func tokenKinds(t *testing.T, text string) []TokenKind` | tokenKinds supports the package test suite's token kinds setup or assertions. |
| [`assertKind`](../src/something/helpers_test.go#L496) | function | 496-501 | `func assertKind(t *testing.T, tok Token, expected TokenKind)` | assertKind supports the package test suite's assert kind setup or assertions. |
| [`assertPanic`](../src/something/helpers_test.go#L504) | function | 504-522 | `func assertPanic(t *testing.T, fn func(), msgContains string)` | assertPanic supports the package test suite's assert panic setup or assertions. |
| [`parseText`](../src/something/helpers_test.go#L525) | function | 525-530 | `func parseText(t *testing.T, text string) *parsedProgramView` | parseText builds a parsed program view from source text. |

### [`src/something/helpers_unit_test.go`](../src/something/helpers_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestIncludeDeclPrivStub`](../src/something/helpers_unit_test.go#L11) | test | 11-16 | `func TestIncludeDeclPrivStub(t *testing.T)` | TestIncludeDeclPrivStub verifies include decl priv stub. |
| [`TestScopeBodyItemInterfaceMarkers`](../src/something/helpers_unit_test.go#L19) | test | 19-27 | `func TestScopeBodyItemInterfaceMarkers(t *testing.T)` | TestScopeBodyItemInterfaceMarkers verifies scope body item interface markers. |
| [`TestGetLocationDefault`](../src/something/helpers_unit_test.go#L30) | test | 30-35 | `func TestGetLocationDefault(t *testing.T)` | TestGetLocationDefault verifies get location default. |
| [`TestKindNameInclude`](../src/something/helpers_unit_test.go#L38) | test | 38-43 | `func TestKindNameInclude(t *testing.T)` | TestKindNameInclude verifies kind name include. |
| [`TestKindNameUnknown`](../src/something/helpers_unit_test.go#L46) | test | 46-51 | `func TestKindNameUnknown(t *testing.T)` | TestKindNameUnknown verifies kind name unknown. |
| [`TestKindNameReference`](../src/something/helpers_unit_test.go#L54) | test | 54-59 | `func TestKindNameReference(t *testing.T)` | TestKindNameReference verifies kind name reference. |

### [`src/something/lexer.go`](../src/something/lexer.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TokenKind`](../src/something/lexer.go#L14) | type | 14 | `type TokenKind int` | TokenKind identifies a lexical category in the SOMETHING language. |
| [`(TokenKind).String`](../src/something/lexer.go#L139) | method | 139-144 | `func (TokenKind).String() string` | String returns the receiver's textual representation. |
| [`Token`](../src/something/lexer.go#L147) | struct | 147-152 | `type Token struct { Kind TokenKind Value any Line int Col int }` | Token represents a single lexeme. |
| [`StringPart`](../src/something/lexer.go#L155) | interface | 155-157 | `type StringPart interface { stringPartMarker() }` | StringPart is one literal or interpolated part of a string token. |
| [`StringText`](../src/something/lexer.go#L160) | type | 160 | `type StringText string` | StringText is literal text within a string. |
| [`(StringText).stringPartMarker`](../src/something/lexer.go#L163) | method | 163 | `func (StringText).stringPartMarker()` | stringPartMarker marks StringText as a StringPart implementation. |
| [`(*InterpolationRef).stringPartMarker`](../src/something/lexer.go#L166) | method | 166 | `func (*InterpolationRef).stringPartMarker()` | stringPartMarker marks InterpolationRef as a StringPart implementation. |
| [`InterpolationRef`](../src/something/lexer.go#L169) | struct | 169-171 | `type InterpolationRef struct { Name string }` | InterpolationRef is a dotted reference inside a string literal. |
| [`StringLiteral`](../src/something/lexer.go#L174) | struct | 174-176 | `type StringLiteral struct { Parts []StringPart }` | StringLiteral is the lexer's structured representation of a string. |
| [`(Token).StrValue`](../src/something/lexer.go#L179) | method | 179-184 | `func (Token).StrValue() string` | StrValue returns the token's string payload, or an empty string for another payload type. |
| [`Lexer`](../src/something/lexer.go#L240) | struct | 240-247 | `type Lexer struct { text string pos int line int col int length int filepath string }` | Lexer tracks source text, coordinates, and file identity while producing tokens. |
| [`NewLexer`](../src/something/lexer.go#L250) | function | 250-259 | `func NewLexer(text string, filepath string) *Lexer` | NewLexer constructs lexer. |
| [`(*Lexer).err`](../src/something/lexer.go#L262) | method | 262-264 | `func (*Lexer).err(msg string)` | err panics with a SomethingError at the lexer's current source position. |
| [`(*Lexer).advance`](../src/something/lexer.go#L267) | method | 267-279 | `func (*Lexer).advance()` | advance consumes one byte and updates line and column coordinates. |
| [`(*Lexer).peek`](../src/something/lexer.go#L282) | method | 282-288 | `func (*Lexer).peek(offset int) byte` | peek returns a byte relative to the current position, or zero outside the source. |
| [`(*Lexer).skipWhitespaceAndComments`](../src/something/lexer.go#L291) | method | 291-328 | `func (*Lexer).skipWhitespaceAndComments()` | skipWhitespaceAndComments consumes whitespace and nested line or block comments. |
| [`(*Lexer).readWord`](../src/something/lexer.go#L331) | method | 331-347 | `func (*Lexer).readWord() string` | readWord reads word from the supplied source. |
| [`(*Lexer).readDigitsWithUnderscore`](../src/something/lexer.go#L350) | method | 350-363 | `func (*Lexer).readDigitsWithUnderscore() string` | readDigitsWithUnderscore reads digits with underscore from the supplied source. |
| [`(*Lexer).readExponent`](../src/something/lexer.go#L366) | method | 366-380 | `func (*Lexer).readExponent() string` | readExponent reads exponent from the supplied source. |
| [`(*Lexer).readNumber`](../src/something/lexer.go#L383) | method | 383-428 | `func (*Lexer).readNumber() Token` | readNumber reads number from the supplied source. |
| [`(*Lexer).fallbackChar`](../src/something/lexer.go#L431) | method | 431-467 | `func (*Lexer).fallbackChar() Token` | fallbackChar tokenizes punctuation that is not handled by a longer lexical form. |
| [`(*Lexer).readString`](../src/something/lexer.go#L470) | method | 470-551 | `func (*Lexer).readString(quote byte, tokLine, tokCol int) Token` | readString reads string from the supplied source. |
| [`(*Lexer).readMultiline`](../src/something/lexer.go#L554) | method | 554-635 | `func (*Lexer).readMultiline() Token` | readMultiline reads multiline from the supplied source. |
| [`(*Lexer).Tokenize`](../src/something/lexer.go#L638) | method | 638-796 | `func (*Lexer).Tokenize() []Token` | Tokenize returns the token list for the lexer's text. |
| [`ParseIntLiteral`](../src/something/lexer.go#L799) | function | 799-803 | `func ParseIntLiteral(s string) int` | ParseIntLiteral removes digit separators and parses a validated integer literal. |
| [`ParseFloatLiteral`](../src/something/lexer.go#L806) | function | 806-810 | `func ParseFloatLiteral(s string) float64` | ParseFloatLiteral removes digit separators and parses a validated floating-point literal. |
| [`IsInt`](../src/something/lexer.go#L813) | function | 813-817 | `func IsInt(s string) bool` | IsInt reports whether a possibly underscore-separated string parses as an integer. |

### [`src/something/lexer_unit_test.go`](../src/something/lexer_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestTokenizeEmpty`](../src/something/lexer_unit_test.go#L11) | test | 11-16 | `func TestTokenizeEmpty(t *testing.T)` | TestTokenizeEmpty verifies tokenize empty. |
| [`TestTokenizeWhitespace`](../src/something/lexer_unit_test.go#L19) | test | 19-24 | `func TestTokenizeWhitespace(t *testing.T)` | TestTokenizeWhitespace verifies tokenize whitespace. |
| [`TestTokenizeStringDouble`](../src/something/lexer_unit_test.go#L27) | test | 27-43 | `func TestTokenizeStringDouble(t *testing.T)` | TestTokenizeStringDouble verifies tokenize string double. |
| [`TestTokenizeStringSingle`](../src/something/lexer_unit_test.go#L46) | test | 46-52 | `func TestTokenizeStringSingle(t *testing.T)` | TestTokenizeStringSingle verifies tokenize string single. |
| [`TestTokenizeStringEscape`](../src/something/lexer_unit_test.go#L55) | test | 55-62 | `func TestTokenizeStringEscape(t *testing.T)` | TestTokenizeStringEscape verifies tokenize string escape. |
| [`TestTokenizeStringInterpolation`](../src/something/lexer_unit_test.go#L65) | test | 65-84 | `func TestTokenizeStringInterpolation(t *testing.T)` | TestTokenizeStringInterpolation verifies tokenize string interpolation. |
| [`TestTokenizeStringInterpolationDotPath`](../src/something/lexer_unit_test.go#L87) | test | 87-94 | `func TestTokenizeStringInterpolationDotPath(t *testing.T)` | TestTokenizeStringInterpolationDotPath verifies tokenize string interpolation dot path. |
| [`TestTokenizeUnterminatedString`](../src/something/lexer_unit_test.go#L97) | test | 97-101 | `func TestTokenizeUnterminatedString(t *testing.T)` | TestTokenizeUnterminatedString verifies tokenize unterminated string. |
| [`TestTokenizeBadInterpolation`](../src/something/lexer_unit_test.go#L104) | test | 104-108 | `func TestTokenizeBadInterpolation(t *testing.T)` | TestTokenizeBadInterpolation verifies tokenize bad interpolation. |
| [`TestTokenizeUnclosedInterpolation`](../src/something/lexer_unit_test.go#L111) | test | 111-115 | `func TestTokenizeUnclosedInterpolation(t *testing.T)` | TestTokenizeUnclosedInterpolation verifies tokenize unclosed interpolation. |
| [`TestTokenizeInteger`](../src/something/lexer_unit_test.go#L118) | test | 118-124 | `func TestTokenizeInteger(t *testing.T)` | TestTokenizeInteger verifies tokenize integer. |
| [`TestTokenizeIntegerNegative`](../src/something/lexer_unit_test.go#L127) | test | 127-133 | `func TestTokenizeIntegerNegative(t *testing.T)` | TestTokenizeIntegerNegative verifies tokenize integer negative. |
| [`TestTokenizeIntegerUnderscore`](../src/something/lexer_unit_test.go#L136) | test | 136-142 | `func TestTokenizeIntegerUnderscore(t *testing.T)` | TestTokenizeIntegerUnderscore verifies tokenize integer underscore. |
| [`TestTokenizeFloat`](../src/something/lexer_unit_test.go#L145) | test | 145-151 | `func TestTokenizeFloat(t *testing.T)` | TestTokenizeFloat verifies tokenize float. |
| [`TestTokenizeFloatExponent`](../src/something/lexer_unit_test.go#L154) | test | 154-160 | `func TestTokenizeFloatExponent(t *testing.T)` | TestTokenizeFloatExponent verifies tokenize float exponent. |
| [`TestTokenizeFloatNegativeExponent`](../src/something/lexer_unit_test.go#L163) | test | 163-169 | `func TestTokenizeFloatNegativeExponent(t *testing.T)` | TestTokenizeFloatNegativeExponent verifies tokenize float negative exponent. |
| [`TestTokenizeFloatNegative`](../src/something/lexer_unit_test.go#L172) | test | 172-178 | `func TestTokenizeFloatNegative(t *testing.T)` | TestTokenizeFloatNegative verifies tokenize float negative. |
| [`TestTokenizeFloatUnderscore`](../src/something/lexer_unit_test.go#L181) | test | 181-184 | `func TestTokenizeFloatUnderscore(t *testing.T)` | TestTokenizeFloatUnderscore verifies tokenize float underscore. |
| [`TestTokenizeKeywords`](../src/something/lexer_unit_test.go#L187) | test | 187-216 | `func TestTokenizeKeywords(t *testing.T)` | TestTokenizeKeywords verifies tokenize keywords. |
| [`TestTokenizeIdentifier`](../src/something/lexer_unit_test.go#L219) | test | 219-225 | `func TestTokenizeIdentifier(t *testing.T)` | TestTokenizeIdentifier verifies tokenize identifier. |
| [`TestTokenizeIdentifierWithUnderscore`](../src/something/lexer_unit_test.go#L228) | test | 228-231 | `func TestTokenizeIdentifierWithUnderscore(t *testing.T)` | TestTokenizeIdentifierWithUnderscore verifies tokenize identifier with underscore. |
| [`TestTokenizePunctuation`](../src/something/lexer_unit_test.go#L234) | test | 234-260 | `func TestTokenizePunctuation(t *testing.T)` | TestTokenizePunctuation verifies tokenize punctuation. |
| [`TestTokenizeHash`](../src/something/lexer_unit_test.go#L263) | test | 263-266 | `func TestTokenizeHash(t *testing.T)` | TestTokenizeHash verifies tokenize hash. |
| [`TestTokenizeLineComment`](../src/something/lexer_unit_test.go#L269) | test | 269-275 | `func TestTokenizeLineComment(t *testing.T)` | TestTokenizeLineComment verifies tokenize line comment. |
| [`TestTokenizeBlockComment`](../src/something/lexer_unit_test.go#L278) | test | 278-281 | `func TestTokenizeBlockComment(t *testing.T)` | TestTokenizeBlockComment verifies tokenize block comment. |
| [`TestTokenizeNestedBlockComment`](../src/something/lexer_unit_test.go#L284) | test | 284-287 | `func TestTokenizeNestedBlockComment(t *testing.T)` | TestTokenizeNestedBlockComment verifies tokenize nested block comment. |
| [`TestTokenizeCRLFWhitespace`](../src/something/lexer_unit_test.go#L290) | test | 290-295 | `func TestTokenizeCRLFWhitespace(t *testing.T)` | TestTokenizeCRLFWhitespace verifies tokenize crlf whitespace. |
| [`TestTokenizeUnterminatedBlockComment`](../src/something/lexer_unit_test.go#L298) | test | 298-302 | `func TestTokenizeUnterminatedBlockComment(t *testing.T)` | TestTokenizeUnterminatedBlockComment verifies tokenize unterminated block comment. |
| [`TestTokenizeMultiline`](../src/something/lexer_unit_test.go#L305) | test | 305-312 | `func TestTokenizeMultiline(t *testing.T)` | TestTokenizeMultiline verifies tokenize multiline. |
| [`TestTokenizeMultilineWithParams`](../src/something/lexer_unit_test.go#L315) | test | 315-322 | `func TestTokenizeMultilineWithParams(t *testing.T)` | TestTokenizeMultilineWithParams verifies tokenize multiline with params. |
| [`TestTokenizeMultilineWithMultipleParams`](../src/something/lexer_unit_test.go#L325) | test | 325-332 | `func TestTokenizeMultilineWithMultipleParams(t *testing.T)` | TestTokenizeMultilineWithMultipleParams verifies tokenize multiline with multiple params. |
| [`TestTokenizeMultilineStripSpaces`](../src/something/lexer_unit_test.go#L335) | test | 335-338 | `func TestTokenizeMultilineStripSpaces(t *testing.T)` | TestTokenizeMultilineStripSpaces verifies tokenize multiline strip spaces. |
| [`TestTokenizeUnexpectedChar`](../src/something/lexer_unit_test.go#L341) | test | 341-345 | `func TestTokenizeUnexpectedChar(t *testing.T)` | TestTokenizeUnexpectedChar verifies tokenize unexpected char. |
| [`TestParseFallbackChar`](../src/something/lexer_unit_test.go#L348) | test | 348-351 | `func TestParseFallbackChar(t *testing.T)` | TestParseFallbackChar verifies parse fallback char. |
| [`TestEvalStringLiteralRef`](../src/something/lexer_unit_test.go#L354) | test | 354-361 | `func TestEvalStringLiteralRef(t *testing.T)` | TestEvalStringLiteralRef verifies eval string literal ref. |
| [`TestLexerFallbackChar`](../src/something/lexer_unit_test.go#L364) | test | 364-395 | `func TestLexerFallbackChar(t *testing.T)` | TestLexerFallbackChar verifies lexer fallback char. |

### [`src/something/macro_functional_test.go`](../src/something/macro_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvalMacroNoParams`](../src/something/macro_functional_test.go#L13) | test | 13-23 | `func TestEvalMacroNoParams(t *testing.T)` | TestEvalMacroNoParams verifies eval macro no params. |
| [`TestEvalMacroWithParams`](../src/something/macro_functional_test.go#L26) | test | 26-36 | `func TestEvalMacroWithParams(t *testing.T)` | TestEvalMacroWithParams verifies eval macro with params. |
| [`TestEvalMacroWithBodyVars`](../src/something/macro_functional_test.go#L39) | test | 39-51 | `func TestEvalMacroWithBodyVars(t *testing.T)` | TestEvalMacroWithBodyVars verifies eval macro with body vars. |
| [`TestEvalMacroReturningArray`](../src/something/macro_functional_test.go#L54) | test | 54-98 | `func TestEvalMacroReturningArray(t *testing.T)` | TestEvalMacroReturningArray verifies eval macro returning array. |
| [`TestEvalMacroReturningInteger`](../src/something/macro_functional_test.go#L101) | test | 101-111 | `func TestEvalMacroReturningInteger(t *testing.T)` | TestEvalMacroReturningInteger verifies eval macro returning integer. |
| [`TestEvalMacroReturningStruct`](../src/something/macro_functional_test.go#L114) | test | 114-129 | `func TestEvalMacroReturningStruct(t *testing.T)` | TestEvalMacroReturningStruct verifies eval macro returning struct. |
| [`TestEvalMacroTypeMismatch`](../src/something/macro_functional_test.go#L132) | test | 132-141 | `func TestEvalMacroTypeMismatch(t *testing.T)` | TestEvalMacroTypeMismatch verifies eval macro type mismatch. |
| [`TestEvalMacroUndefined`](../src/something/macro_functional_test.go#L144) | test | 144-150 | `func TestEvalMacroUndefined(t *testing.T)` | TestEvalMacroUndefined verifies eval macro undefined. |
| [`TestEvalMacroArgCountMismatch`](../src/something/macro_functional_test.go#L153) | test | 153-162 | `func TestEvalMacroArgCountMismatch(t *testing.T)` | TestEvalMacroArgCountMismatch verifies eval macro arg count mismatch. |
| [`TestEvalMacroParamOrder`](../src/something/macro_functional_test.go#L165) | test | 165-179 | `func TestEvalMacroParamOrder(t *testing.T)` | TestEvalMacroParamOrder verifies eval macro param order. |
| [`TestEvalMacroBodyVarsNotExposed`](../src/something/macro_functional_test.go#L182) | test | 182-196 | `func TestEvalMacroBodyVarsNotExposed(t *testing.T)` | TestEvalMacroBodyVarsNotExposed verifies eval macro body vars not exposed. |
| [`TestEvalMacroMultipleCalls`](../src/something/macro_functional_test.go#L199) | test | 199-210 | `func TestEvalMacroMultipleCalls(t *testing.T)` | TestEvalMacroMultipleCalls verifies eval macro multiple calls. |
| [`TestEvalMacroInStructField`](../src/something/macro_functional_test.go#L213) | test | 213-228 | `func TestEvalMacroInStructField(t *testing.T)` | TestEvalMacroInStructField verifies eval macro in struct field. |
| [`TestEvalMacroWithForDirective`](../src/something/macro_functional_test.go#L231) | test | 231-254 | `func TestEvalMacroWithForDirective(t *testing.T)` | TestEvalMacroWithForDirective verifies eval macro with for directive. |
| [`TestEvalMacroParamTypeString`](../src/something/macro_functional_test.go#L257) | test | 257-267 | `func TestEvalMacroParamTypeString(t *testing.T)` | TestEvalMacroParamTypeString verifies eval macro param type string. |
| [`TestEvalMacroParamTypeStringWrong`](../src/something/macro_functional_test.go#L270) | test | 270-279 | `func TestEvalMacroParamTypeStringWrong(t *testing.T)` | TestEvalMacroParamTypeStringWrong verifies eval macro param type string wrong. |
| [`TestEvalMacroParamTypeInteger`](../src/something/macro_functional_test.go#L282) | test | 282-292 | `func TestEvalMacroParamTypeInteger(t *testing.T)` | TestEvalMacroParamTypeInteger verifies eval macro param type integer. |
| [`TestEvalMacroParamTypeIntegerWrong`](../src/something/macro_functional_test.go#L295) | test | 295-304 | `func TestEvalMacroParamTypeIntegerWrong(t *testing.T)` | TestEvalMacroParamTypeIntegerWrong verifies eval macro param type integer wrong. |
| [`TestEvalMacroParamTypeFloat`](../src/something/macro_functional_test.go#L307) | test | 307-317 | `func TestEvalMacroParamTypeFloat(t *testing.T)` | TestEvalMacroParamTypeFloat verifies eval macro param type float. |
| [`TestEvalMacroParamTypeBoolean`](../src/something/macro_functional_test.go#L320) | test | 320-330 | `func TestEvalMacroParamTypeBoolean(t *testing.T)` | TestEvalMacroParamTypeBoolean verifies eval macro param type boolean. |
| [`TestEvalMacroParamTypeEnum`](../src/something/macro_functional_test.go#L333) | test | 333-344 | `func TestEvalMacroParamTypeEnum(t *testing.T)` | TestEvalMacroParamTypeEnum verifies eval macro param type enum. |
| [`TestEvalMacroParamTypeEnumWrong`](../src/something/macro_functional_test.go#L347) | test | 347-357 | `func TestEvalMacroParamTypeEnumWrong(t *testing.T)` | TestEvalMacroParamTypeEnumWrong verifies eval macro param type enum wrong. |
| [`TestEvalMacroParamTypeSetup`](../src/something/macro_functional_test.go#L360) | test | 360-371 | `func TestEvalMacroParamTypeSetup(t *testing.T)` | TestEvalMacroParamTypeSetup verifies eval macro param type setup. |
| [`TestEvalMacroParamTypeSetupWrong`](../src/something/macro_functional_test.go#L374) | test | 374-384 | `func TestEvalMacroParamTypeSetupWrong(t *testing.T)` | TestEvalMacroParamTypeSetupWrong verifies eval macro param type setup wrong. |
| [`TestEvalMacroParamTypeSetupAnonymous`](../src/something/macro_functional_test.go#L387) | test | 387-398 | `func TestEvalMacroParamTypeSetupAnonymous(t *testing.T)` | TestEvalMacroParamTypeSetupAnonymous verifies eval macro param type setup anonymous. |
| [`TestEvalMacroParamTypeArray`](../src/something/macro_functional_test.go#L401) | test | 401-411 | `func TestEvalMacroParamTypeArray(t *testing.T)` | TestEvalMacroParamTypeArray verifies eval macro param type array. |
| [`TestEvalMacroParamTypeArrayWrong`](../src/something/macro_functional_test.go#L414) | test | 414-423 | `func TestEvalMacroParamTypeArrayWrong(t *testing.T)` | TestEvalMacroParamTypeArrayWrong verifies eval macro param type array wrong. |
| [`TestEvalMacroParamTypeMapping`](../src/something/macro_functional_test.go#L426) | test | 426-436 | `func TestEvalMacroParamTypeMapping(t *testing.T)` | TestEvalMacroParamTypeMapping verifies eval macro param type mapping. |
| [`TestEvalMacroParamTypeMappingWrong`](../src/something/macro_functional_test.go#L439) | test | 439-448 | `func TestEvalMacroParamTypeMappingWrong(t *testing.T)` | TestEvalMacroParamTypeMappingWrong verifies eval macro param type mapping wrong. |
| [`TestEvalMacroParamTypeTimestamp`](../src/something/macro_functional_test.go#L451) | test | 451-461 | `func TestEvalMacroParamTypeTimestamp(t *testing.T)` | TestEvalMacroParamTypeTimestamp verifies eval macro param type timestamp. |
| [`TestEvalMacroParamTypeTimestampWrong`](../src/something/macro_functional_test.go#L464) | test | 464-473 | `func TestEvalMacroParamTypeTimestampWrong(t *testing.T)` | TestEvalMacroParamTypeTimestampWrong verifies eval macro param type timestamp wrong. |
| [`TestEvalMacroParamTypeMultipleCorrect`](../src/something/macro_functional_test.go#L476) | test | 476-486 | `func TestEvalMacroParamTypeMultipleCorrect(t *testing.T)` | TestEvalMacroParamTypeMultipleCorrect verifies eval macro param type multiple correct. |
| [`TestEvalMacroParamTypeMultipleWrong`](../src/something/macro_functional_test.go#L489) | test | 489-498 | `func TestEvalMacroParamTypeMultipleWrong(t *testing.T)` | TestEvalMacroParamTypeMultipleWrong verifies eval macro param type multiple wrong. |
| [`TestEvalMacroReturnTypeString`](../src/something/macro_functional_test.go#L501) | test | 501-510 | `func TestEvalMacroReturnTypeString(t *testing.T)` | TestEvalMacroReturnTypeString verifies eval macro return type string. |
| [`TestEvalMacroReturnTypeArray`](../src/something/macro_functional_test.go#L513) | test | 513-522 | `func TestEvalMacroReturnTypeArray(t *testing.T)` | TestEvalMacroReturnTypeArray verifies eval macro return type array. |
| [`TestEvalMacroReturnTypeSetup`](../src/something/macro_functional_test.go#L525) | test | 525-535 | `func TestEvalMacroReturnTypeSetup(t *testing.T)` | TestEvalMacroReturnTypeSetup verifies eval macro return type setup. |
| [`TestEvalMacroReturnTypeEnum`](../src/something/macro_functional_test.go#L538) | test | 538-548 | `func TestEvalMacroReturnTypeEnum(t *testing.T)` | TestEvalMacroReturnTypeEnum verifies eval macro return type enum. |
| [`TestEvalMacroReturnTypeMapping`](../src/something/macro_functional_test.go#L551) | test | 551-560 | `func TestEvalMacroReturnTypeMapping(t *testing.T)` | TestEvalMacroReturnTypeMapping verifies eval macro return type mapping. |
| [`TestEvalMacroReturnTypeBoolean`](../src/something/macro_functional_test.go#L563) | test | 563-572 | `func TestEvalMacroReturnTypeBoolean(t *testing.T)` | TestEvalMacroReturnTypeBoolean verifies eval macro return type boolean. |
| [`TestEvalMacroReturnTypeFloat`](../src/something/macro_functional_test.go#L575) | test | 575-584 | `func TestEvalMacroReturnTypeFloat(t *testing.T)` | TestEvalMacroReturnTypeFloat verifies eval macro return type float. |
| [`TestEvalMacroParamTypeScope`](../src/something/macro_functional_test.go#L587) | test | 587-598 | `func TestEvalMacroParamTypeScope(t *testing.T)` | TestEvalMacroParamTypeScope verifies eval macro param type scope. |
| [`TestEvalMacroParamTypeScopeWrong`](../src/something/macro_functional_test.go#L601) | test | 601-610 | `func TestEvalMacroParamTypeScopeWrong(t *testing.T)` | TestEvalMacroParamTypeScopeWrong verifies eval macro param type scope wrong. |

### [`src/something/macro_unit_test.go`](../src/something/macro_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestTokenizeMacroAndSet`](../src/something/macro_unit_test.go#L12) | test | 12-31 | `func TestTokenizeMacroAndSet(t *testing.T)` | TestTokenizeMacroAndSet verifies tokenize macro and set. |
| [`TestTokenizeRarrow`](../src/something/macro_unit_test.go#L34) | test | 34-39 | `func TestTokenizeRarrow(t *testing.T)` | TestTokenizeRarrow verifies tokenize rarrow. |
| [`TestTokenizeBang`](../src/something/macro_unit_test.go#L42) | test | 42-47 | `func TestTokenizeBang(t *testing.T)` | TestTokenizeBang verifies tokenize bang. |
| [`TestParseMacroDecl`](../src/something/macro_unit_test.go#L50) | test | 50-71 | `func TestParseMacroDecl(t *testing.T)` | TestParseMacroDecl verifies parse macro decl. |
| [`TestParseMacroDeclWithParams`](../src/something/macro_unit_test.go#L74) | test | 74-92 | `func TestParseMacroDeclWithParams(t *testing.T)` | TestParseMacroDeclWithParams verifies parse macro decl with params. |
| [`TestParseMacroCall`](../src/something/macro_unit_test.go#L95) | test | 95-108 | `func TestParseMacroCall(t *testing.T)` | TestParseMacroCall verifies parse macro call. |
| [`TestParseMacroCallWithArgs`](../src/something/macro_unit_test.go#L111) | test | 111-125 | `func TestParseMacroCallWithArgs(t *testing.T)` | TestParseMacroCallWithArgs verifies parse macro call with args. |
| [`TestScopeBodyItemMacroMarker`](../src/something/macro_unit_test.go#L128) | test | 128-131 | `func TestScopeBodyItemMacroMarker(t *testing.T)` | TestScopeBodyItemMacroMarker verifies scope body item macro marker. |
| [`TestKindNameMacroCall`](../src/something/macro_unit_test.go#L134) | test | 134-139 | `func TestKindNameMacroCall(t *testing.T)` | TestKindNameMacroCall verifies kind name macro call. |

### [`src/something/parser.go`](../src/something/parser.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Parser`](../src/something/parser.go#L12) | struct | 12-16 | `type Parser struct { tokens []Token pos int filepath string }` | Parser tracks a token stream and file identity while building an ordered syntax tree. |
| [`NewParser`](../src/something/parser.go#L19) | function | 19-21 | `func NewParser(tokens []Token, filepath string) *Parser` | NewParser constructs parser. |
| [`(*Parser).peekPrecedence`](../src/something/parser.go#L35) | method | 35-55 | `func (*Parser).peekPrecedence() int` | peekPrecedence returns the precedence of the next infix operator, or precLowest. |
| [`(*Parser).infixBinding`](../src/something/parser.go#L58) | method | 58-84 | `func (*Parser).infixBinding() (BinaryOpKind, bool)` | infixBinding returns the BinaryOpKind for the next infix operator, if any. |
| [`(*Parser).peek`](../src/something/parser.go#L87) | method | 87-93 | `func (*Parser).peek(offset int) Token` | peek returns a token relative to the current parser position, using EOF outside the stream. |
| [`(*Parser).advance`](../src/something/parser.go#L96) | method | 96-102 | `func (*Parser).advance() Token` | advance consumes and returns the current token. |
| [`(*Parser).location`](../src/something/parser.go#L105) | method | 105-107 | `func (*Parser).location(token Token) *SourceLocation` | location converts token coordinates to a file-aware source location. |
| [`(*Parser).err`](../src/something/parser.go#L110) | method | 110-115 | `func (*Parser).err(message, suggestion string, location *SourceLocation)` | err panics with a source-located SomethingError at the supplied or current location. |
| [`(*Parser).expect`](../src/something/parser.go#L118) | method | 118-127 | `func (*Parser).expect(kind TokenKind, message string) Token` | expect consumes the required token kind or raises the requested syntax error. |
| [`(*Parser).expectStatementTerminator`](../src/something/parser.go#L130) | method | 130-132 | `func (*Parser).expectStatementTerminator(context string)` | expectStatementTerminator consumes the semicolon required after a statement context. |
| [`(*Parser).ParseProgram`](../src/something/parser.go#L135) | method | 135-137 | `func (*Parser).ParseProgram() *Program` | ParseProgram parses one complete file. Directives remain in the syntax AST. |
| [`(*Parser).parseStatements`](../src/something/parser.go#L140) | method | 140-149 | `func (*Parser).parseStatements(end TokenKind) []Statement` | parseStatements parses statements from the supplied input. |
| [`(*Parser).parseStatement`](../src/something/parser.go#L152) | method | 152-174 | `func (*Parser).parseStatement() Statement` | parseStatement parses statement from the supplied input. |
| [`(*Parser).parseDirectiveStatement`](../src/something/parser.go#L177) | method | 177-224 | `func (*Parser).parseDirectiveStatement(hash Token) Statement` | parseDirectiveStatement parses directive statement from the supplied input. |
| [`(*Parser).parseAssertDirective`](../src/something/parser.go#L227) | method | 227-237 | `func (*Parser).parseAssertDirective(location *SourceLocation) *AssertDirective` | parseAssertDirective parses assert directive from the supplied input. |
| [`(*Parser).parseIfDirective`](../src/something/parser.go#L240) | method | 240-259 | `func (*Parser).parseIfDirective(location *SourceLocation) *IfDirective` | parseIfDirective parses if directive from the supplied input. |
| [`(*Parser).parseErrorDirective`](../src/something/parser.go#L262) | method | 262-271 | `func (*Parser).parseErrorDirective(location *SourceLocation) *ErrorDirective` | parseErrorDirective parses error directive from the supplied input. |
| [`(*Parser).parseOptionalDirectiveArgument`](../src/something/parser.go#L274) | method | 274-285 | `func (*Parser).parseOptionalDirectiveArgument(name string) Expression` | parseOptionalDirectiveArgument parses optional directive argument from the supplied input. |
| [`(*Parser).parseAssignment`](../src/something/parser.go#L288) | method | 288-331 | `func (*Parser).parseAssignment(target LValue, location *SourceLocation) *Assignment` | parseAssignment parses assignment from the supplied input. |
| [`(*Parser).parseNamedLValue`](../src/something/parser.go#L334) | method | 334-341 | `func (*Parser).parseNamedLValue() LValue` | parseNamedLValue parses named l value from the supplied input. |
| [`(*Parser).parseAccesses`](../src/something/parser.go#L344) | method | 344-361 | `func (*Parser).parseAccesses() []Access` | parseAccesses parses accesses from the supplied input. |
| [`(*Parser).parseIncludePath`](../src/something/parser.go#L364) | method | 364-369 | `func (*Parser).parseIncludePath() string` | parseIncludePath parses include path from the supplied input. |
| [`(*Parser).parseForDirective`](../src/something/parser.go#L372) | method | 372-393 | `func (*Parser).parseForDirective(location *SourceLocation) *ForDirective` | parseForDirective parses for directive from the supplied input. |
| [`(*Parser).parseForSource`](../src/something/parser.go#L398) | method | 398-408 | `func (*Parser).parseForSource() Expression` | parseForSource keeps the loop body delimiter from being interpreted as the opening brace of a typed struct literal. Macro calls still use the general expression parser because they may produce an iterable value. |
| [`(*Parser).parseInsertDirective`](../src/something/parser.go#L411) | method | 411-424 | `func (*Parser).parseInsertDirective(location *SourceLocation) *InsertDirective` | parseInsertDirective parses insert directive from the supplied input. |
| [`(*Parser).parseMacroDirective`](../src/something/parser.go#L427) | method | 427-474 | `func (*Parser).parseMacroDirective(location *SourceLocation) *MacroDirective` | parseMacroDirective parses macro directive from the supplied input. |
| [`(*Parser).parseEnumDefinition`](../src/something/parser.go#L477) | method | 477-499 | `func (*Parser).parseEnumDefinition(location *SourceLocation) *EnumDefinition` | parseEnumDefinition parses enum definition from the supplied input. |
| [`(*Parser).parseSetupDefinition`](../src/something/parser.go#L502) | method | 502-512 | `func (*Parser).parseSetupDefinition(location *SourceLocation) *SetupDefinition` | parseSetupDefinition parses setup definition from the supplied input. |
| [`(*Parser).parseFieldDefinition`](../src/something/parser.go#L515) | method | 515-541 | `func (*Parser).parseFieldDefinition() *FieldDefinition` | parseFieldDefinition parses field definition from the supplied input. |
| [`(*Parser).parseScopeExpression`](../src/something/parser.go#L544) | method | 544-549 | `func (*Parser).parseScopeExpression() *ScopeExpression` | parseScopeExpression parses scope expression from the supplied input. |
| [`(*Parser).parseTypeRef`](../src/something/parser.go#L552) | method | 552-608 | `func (*Parser).parseTypeRef() TypeRef` | parseTypeRef parses type ref from the supplied input. |
| [`(*Parser).parseExpression`](../src/something/parser.go#L611) | method | 611-613 | `func (*Parser).parseExpression() Expression` | parseExpression parses expression from the supplied input. |
| [`(*Parser).parseExpressionPrecedence`](../src/something/parser.go#L616) | method | 616-631 | `func (*Parser).parseExpressionPrecedence(minPrec int) Expression` | parseExpressionPrecedence parses expression precedence from the supplied input. |
| [`(*Parser).parsePrefix`](../src/something/parser.go#L634) | method | 634-746 | `func (*Parser).parsePrefix() Expression` | parsePrefix parses prefix from the supplied input. |
| [`(*Parser).parseInfix`](../src/something/parser.go#L749) | method | 749-763 | `func (*Parser).parseInfix(left Expression, op BinaryOpKind) Expression` | parseInfix parses infix from the supplied input. |
| [`tokenStartsType`](../src/something/parser.go#L766) | function | 766-768 | `func tokenStartsType(kind TokenKind) bool` | tokenStartsType reports whether a token can begin a SOMETHING type reference. |
| [`isTypeToken`](../src/something/parser.go#L771) | function | 771-773 | `func isTypeToken(kind TokenKind) bool` | isTypeToken reports whether a token is a primitive type keyword. |
| [`(*Parser).parseArrayExpression`](../src/something/parser.go#L776) | method | 776-789 | `func (*Parser).parseArrayExpression() Expression` | parseArrayExpression parses array expression from the supplied input. |
| [`(*Parser).parseTypedArrayExpression`](../src/something/parser.go#L792) | method | 792-808 | `func (*Parser).parseTypedArrayExpression() Expression` | parseTypedArrayExpression parses typed array expression from the supplied input. |
| [`(*Parser).parseMappingExpression`](../src/something/parser.go#L811) | method | 811-823 | `func (*Parser).parseMappingExpression() Expression` | parseMappingExpression parses mapping expression from the supplied input. |
| [`(*Parser).parseMappingBody`](../src/something/parser.go#L826) | method | 826-828 | `func (*Parser).parseMappingBody(declaredType *MappingType) Expression` | parseMappingBody parses mapping body from the supplied input. |
| [`(*Parser).parseMappingBodyAt`](../src/something/parser.go#L831) | method | 831-867 | `func (*Parser).parseMappingBodyAt(declaredType *MappingType, location *SourceLocation) Expression` | parseMappingBodyAt parses mapping body at from the supplied input. |
| [`(*Parser).parseStructExpression`](../src/something/parser.go#L870) | method | 870-885 | `func (*Parser).parseStructExpression(typeName string, location *SourceLocation) Expression` | parseStructExpression parses struct expression from the supplied input. |
| [`(*Parser).parseMacroCall`](../src/something/parser.go#L888) | method | 888-901 | `func (*Parser).parseMacroCall(name string, location *SourceLocation) Expression` | parseMacroCall parses macro call from the supplied input. |
| [`processMultilineContent`](../src/something/parser.go#L904) | function | 904-933 | `func processMultilineContent(raw, params string) string` | processMultilineContent applies declared indentation, newline, and whitespace transformations to multiline text. |
| [`stringLiteralToString`](../src/something/parser.go#L936) | function | 936-953 | `func stringLiteralToString(token Token) string` | stringLiteralToString reconstructs a string token while preserving interpolation placeholders. |
| [`typeRefString`](../src/something/parser.go#L956) | function | 956-979 | `func typeRefString(ref TypeRef) string` | typeRefString returns the canonical diagnostic representation of a type reference. |

### [`src/something/parser_unit_test.go`](../src/something/parser_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestParseEmpty`](../src/something/parser_unit_test.go#L11) | test | 11-16 | `func TestParseEmpty(t *testing.T)` | TestParseEmpty verifies parse empty. |
| [`TestParseVarInfer`](../src/something/parser_unit_test.go#L19) | test | 19-31 | `func TestParseVarInfer(t *testing.T)` | TestParseVarInfer verifies parse var infer. |
| [`TestParseVarTyped`](../src/something/parser_unit_test.go#L34) | test | 34-46 | `func TestParseVarTyped(t *testing.T)` | TestParseVarTyped verifies parse var typed. |
| [`TestParseVarInteger`](../src/something/parser_unit_test.go#L49) | test | 49-55 | `func TestParseVarInteger(t *testing.T)` | TestParseVarInteger verifies parse var integer. |
| [`TestParseVarBoolean`](../src/something/parser_unit_test.go#L58) | test | 58-64 | `func TestParseVarBoolean(t *testing.T)` | TestParseVarBoolean verifies parse var boolean. |
| [`TestParseVarFloat`](../src/something/parser_unit_test.go#L67) | test | 67-73 | `func TestParseVarFloat(t *testing.T)` | TestParseVarFloat verifies parse var float. |
| [`TestParseVarTimestamp`](../src/something/parser_unit_test.go#L76) | test | 76-82 | `func TestParseVarTimestamp(t *testing.T)` | TestParseVarTimestamp verifies parse var timestamp. |
| [`TestParseEnumPlain`](../src/something/parser_unit_test.go#L85) | test | 85-103 | `func TestParseEnumPlain(t *testing.T)` | TestParseEnumPlain verifies parse enum plain. |
| [`TestParseEnumTagged`](../src/something/parser_unit_test.go#L106) | test | 106-118 | `func TestParseEnumTagged(t *testing.T)` | TestParseEnumTagged verifies parse enum tagged. |
| [`TestParseSetup`](../src/something/parser_unit_test.go#L121) | test | 121-136 | `func TestParseSetup(t *testing.T)` | TestParseSetup verifies parse setup. |
| [`TestParseSetupOptional`](../src/something/parser_unit_test.go#L139) | test | 139-151 | `func TestParseSetupOptional(t *testing.T)` | TestParseSetupOptional verifies parse setup optional. |
| [`TestParseBareScope`](../src/something/parser_unit_test.go#L154) | test | 154-163 | `func TestParseBareScope(t *testing.T)` | TestParseBareScope verifies parse bare scope. |
| [`TestParseArrayType`](../src/something/parser_unit_test.go#L166) | test | 166-172 | `func TestParseArrayType(t *testing.T)` | TestParseArrayType verifies parse array type. |
| [`TestParseMappingType`](../src/something/parser_unit_test.go#L175) | test | 175-181 | `func TestParseMappingType(t *testing.T)` | TestParseMappingType verifies parse mapping type. |
| [`TestParseEnumKeyType`](../src/something/parser_unit_test.go#L184) | test | 184-197 | `func TestParseEnumKeyType(t *testing.T)` | TestParseEnumKeyType verifies parse enum key type. |
| [`TestParseIterationDirective`](../src/something/parser_unit_test.go#L200) | test | 200-212 | `func TestParseIterationDirective(t *testing.T)` | TestParseIterationDirective verifies parse iteration directive. |
| [`TestParseIterationInfer`](../src/something/parser_unit_test.go#L215) | test | 215-224 | `func TestParseIterationInfer(t *testing.T)` | TestParseIterationInfer verifies parse iteration infer. |
| [`TestParseIterationWithLabel`](../src/something/parser_unit_test.go#L227) | test | 227-236 | `func TestParseIterationWithLabel(t *testing.T)` | TestParseIterationWithLabel verifies parse iteration with label. |
| [`TestParseIterationScope`](../src/something/parser_unit_test.go#L239) | test | 239-255 | `func TestParseIterationScope(t *testing.T)` | TestParseIterationScope verifies parse iteration scope. |
| [`TestParseForDirective`](../src/something/parser_unit_test.go#L258) | test | 258-270 | `func TestParseForDirective(t *testing.T)` | TestParseForDirective verifies parse for directive. |
| [`TestParseForMapping`](../src/something/parser_unit_test.go#L273) | test | 273-286 | `func TestParseForMapping(t *testing.T)` | TestParseForMapping verifies parse for mapping. |
| [`TestParseInsertDirective`](../src/something/parser_unit_test.go#L289) | test | 289-294 | `func TestParseInsertDirective(t *testing.T)` | TestParseInsertDirective verifies parse insert directive. |
| [`TestParseInsertDirectiveMultipleValues`](../src/something/parser_unit_test.go#L297) | test | 297-302 | `func TestParseInsertDirectiveMultipleValues(t *testing.T)` | TestParseInsertDirectiveMultipleValues verifies parse insert directive multiple values. |
| [`TestParseIncludeDirective`](../src/something/parser_unit_test.go#L305) | test | 305-314 | `func TestParseIncludeDirective(t *testing.T)` | TestParseIncludeDirective verifies parse include directive. |
| [`TestParseAsLvalueDirective`](../src/something/parser_unit_test.go#L317) | test | 317-322 | `func TestParseAsLvalueDirective(t *testing.T)` | TestParseAsLvalueDirective verifies parse as lvalue directive. |
| [`TestParsePrivModifier`](../src/something/parser_unit_test.go#L325) | test | 325-333 | `func TestParsePrivModifier(t *testing.T)` | TestParsePrivModifier verifies parse priv modifier. |
| [`TestParseStructValue`](../src/something/parser_unit_test.go#L336) | test | 336-348 | `func TestParseStructValue(t *testing.T)` | TestParseStructValue verifies parse struct value. |
| [`TestParseStructAnonymous`](../src/something/parser_unit_test.go#L351) | test | 351-361 | `func TestParseStructAnonymous(t *testing.T)` | TestParseStructAnonymous verifies parse struct anonymous. |
| [`TestParseMappingLiteral`](../src/something/parser_unit_test.go#L364) | test | 364-370 | `func TestParseMappingLiteral(t *testing.T)` | TestParseMappingLiteral verifies parse mapping literal. |
| [`TestParseArrayLiteral`](../src/something/parser_unit_test.go#L373) | test | 373-379 | `func TestParseArrayLiteral(t *testing.T)` | TestParseArrayLiteral verifies parse array literal. |
| [`TestParseEnumMemberShorthand`](../src/something/parser_unit_test.go#L382) | test | 382-394 | `func TestParseEnumMemberShorthand(t *testing.T)` | TestParseEnumMemberShorthand verifies parse enum member shorthand. |
| [`TestParseReferenceWithDots`](../src/something/parser_unit_test.go#L397) | test | 397-406 | `func TestParseReferenceWithDots(t *testing.T)` | TestParseReferenceWithDots verifies parse reference with dots. |
| [`TestParseReferenceWithIndex`](../src/something/parser_unit_test.go#L409) | test | 409-418 | `func TestParseReferenceWithIndex(t *testing.T)` | TestParseReferenceWithIndex verifies parse reference with index. |
| [`TestParseReferenceWithCombinedAccess`](../src/something/parser_unit_test.go#L421) | test | 421-426 | `func TestParseReferenceWithCombinedAccess(t *testing.T)` | TestParseReferenceWithCombinedAccess verifies parse reference with combined access. |
| [`TestParseMultilineValue`](../src/something/parser_unit_test.go#L429) | test | 429-435 | `func TestParseMultilineValue(t *testing.T)` | TestParseMultilineValue verifies parse multiline value. |
| [`TestParseErrorMissingBrace`](../src/something/parser_unit_test.go#L438) | test | 438-442 | `func TestParseErrorMissingBrace(t *testing.T)` | TestParseErrorMissingBrace verifies parse error missing brace. |
| [`TestParseErrorUnknownDirective`](../src/something/parser_unit_test.go#L445) | test | 445-449 | `func TestParseErrorUnknownDirective(t *testing.T)` | TestParseErrorUnknownDirective verifies parse error unknown directive. |
| [`TestParseArrayTypeWithExplicitIndex`](../src/something/parser_unit_test.go#L452) | test | 452-461 | `func TestParseArrayTypeWithExplicitIndex(t *testing.T)` | TestParseArrayTypeWithExplicitIndex verifies parse array type with explicit index. |
| [`TestParseScopeBodyItemPrivIteration`](../src/something/parser_unit_test.go#L464) | test | 464-481 | `func TestParseScopeBodyItemPrivIteration(t *testing.T)` | TestParseScopeBodyItemPrivIteration verifies parse scope body item priv iteration. |
| [`TestParseScopeBodyUnknownDirective`](../src/something/parser_unit_test.go#L484) | test | 484-488 | `func TestParseScopeBodyUnknownDirective(t *testing.T)` | TestParseScopeBodyUnknownDirective verifies parse scope body unknown directive. |
| [`TestParseScopeBodyNonIdentifier`](../src/something/parser_unit_test.go#L491) | test | 491-495 | `func TestParseScopeBodyNonIdentifier(t *testing.T)` | TestParseScopeBodyNonIdentifier verifies parse scope body non identifier. |
| [`TestParseForSourceDottedPath`](../src/something/parser_unit_test.go#L498) | test | 498-503 | `func TestParseForSourceDottedPath(t *testing.T)` | TestParseForSourceDottedPath verifies parse for source dotted path. |
| [`TestParseIndexAccessAllBranches`](../src/something/parser_unit_test.go#L506) | test | 506-519 | `func TestParseIndexAccessAllBranches(t *testing.T)` | TestParseIndexAccessAllBranches verifies parse index access all branches. |
| [`TestParseIndexAccessError`](../src/something/parser_unit_test.go#L522) | test | 522-526 | `func TestParseIndexAccessError(t *testing.T)` | TestParseIndexAccessError verifies parse index access error. |
| [`TestParseScopeBodyItemTypedVar`](../src/something/parser_unit_test.go#L529) | test | 529-542 | `func TestParseScopeBodyItemTypedVar(t *testing.T)` | TestParseScopeBodyItemTypedVar verifies parse scope body item typed var. |
| [`TestParseScopeBodyItemIncludeVar`](../src/something/parser_unit_test.go#L545) | test | 545-560 | `func TestParseScopeBodyItemIncludeVar(t *testing.T)` | TestParseScopeBodyItemIncludeVar verifies parse scope body item include var. |
| [`TestParserExpectError`](../src/something/parser_unit_test.go#L563) | test | 563-568 | `func TestParserExpectError(t *testing.T)` | TestParserExpectError verifies parser expect error. |
| [`TestParserExpectEmptyMsg`](../src/something/parser_unit_test.go#L571) | test | 571-576 | `func TestParserExpectEmptyMsg(t *testing.T)` | TestParserExpectEmptyMsg verifies parser expect empty msg. |
| [`TestExpectErrorPath`](../src/something/parser_unit_test.go#L579) | test | 579-584 | `func TestExpectErrorPath(t *testing.T)` | TestExpectErrorPath verifies expect error path. |

### [`src/something/pipeline_functional_test.go`](../src/something/pipeline_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`compileText`](../src/something/pipeline_functional_test.go#L14) | function | 14-20 | `func compileText(t *testing.T, text string) (*Program, *Program, *CheckedProgram)` | compileText supports the package test suite's compile text setup or assertions. |
| [`assertExpandedAssignments`](../src/something/pipeline_functional_test.go#L23) | function | 23-37 | `func assertExpandedAssignments(t *testing.T, statements []Statement)` | assertExpandedAssignments supports the package test suite's assert expanded assignments setup or assertions. |
| [`TestOrderedASTInterfaceMarkers`](../src/something/pipeline_functional_test.go#L40) | test | 40-90 | `func TestOrderedASTInterfaceMarkers(t *testing.T)` | TestOrderedASTInterfaceMarkers verifies ordered ast interface markers. |
| [`TestPipelinePreservesSyntaxOrderThenRemovesDirectives`](../src/something/pipeline_functional_test.go#L93) | test | 93-126 | `func TestPipelinePreservesSyntaxOrderThenRemovesDirectives(t *testing.T)` | TestPipelinePreservesSyntaxOrderThenRemovesDirectives verifies pipeline preserves syntax order then removes directives. |
| [`TestDirectiveExpansionResolvesLoopElementMembers`](../src/something/pipeline_functional_test.go#L129) | test | 129-146 | `func TestDirectiveExpansionResolvesLoopElementMembers(t *testing.T)` | TestDirectiveExpansionResolvesLoopElementMembers verifies directive expansion resolves loop element members. |
| [`TestSourceOrderRejectsReferencesBeforeDeclaration`](../src/something/pipeline_functional_test.go#L149) | test | 149-156 | `func TestSourceOrderRejectsReferencesBeforeDeclaration(t *testing.T)` | TestSourceOrderRejectsReferencesBeforeDeclaration verifies source order rejects references before declaration. |
| [`TestScopesAllowIndependentNamesAndQualifiedOuterAccess`](../src/something/pipeline_functional_test.go#L159) | test | 159-175 | `func TestScopesAllowIndependentNamesAndQualifiedOuterAccess(t *testing.T)` | TestScopesAllowIndependentNamesAndQualifiedOuterAccess verifies scopes allow independent names and qualified outer access. |
| [`TestPrivateSourcesRemainAccessibleAndDestinationControlsPublication`](../src/something/pipeline_functional_test.go#L178) | test | 178-198 | `func TestPrivateSourcesRemainAccessibleAndDestinationControlsPublication(t *testing.T)` | TestPrivateSourcesRemainAccessibleAndDestinationControlsPublication verifies private sources remain accessible and destination controls publication. |
| [`TestReassignmentSupportsBindingsAndMembers`](../src/something/pipeline_functional_test.go#L201) | test | 201-224 | `func TestReassignmentSupportsBindingsAndMembers(t *testing.T)` | TestReassignmentSupportsBindingsAndMembers verifies reassignment supports bindings and members. |
| [`TestReassignmentEnforcesExistingDestinationAndType`](../src/something/pipeline_functional_test.go#L227) | test | 227-237 | `func TestReassignmentEnforcesExistingDestinationAndType(t *testing.T)` | TestReassignmentEnforcesExistingDestinationAndType verifies reassignment enforces existing destination and type. |
| [`TestDeclarationsRejectNameAndDestinationConflicts`](../src/something/pipeline_functional_test.go#L240) | test | 240-256 | `func TestDeclarationsRejectNameAndDestinationConflicts(t *testing.T)` | TestDeclarationsRejectNameAndDestinationConflicts verifies declarations reject name and destination conflicts. |
| [`TestEnumIndexedCollectionsRetainIndexTypesDuringEvaluation`](../src/something/pipeline_functional_test.go#L259) | test | 259-272 | `func TestEnumIndexedCollectionsRetainIndexTypesDuringEvaluation(t *testing.T)` | TestEnumIndexedCollectionsRetainIndexTypesDuringEvaluation verifies enum indexed collections retain index types during evaluation. |
| [`TestIterationAndAsLvalueExpandToConcreteDestinations`](../src/something/pipeline_functional_test.go#L275) | test | 275-293 | `func TestIterationAndAsLvalueExpandToConcreteDestinations(t *testing.T)` | TestIterationAndAsLvalueExpandToConcreteDestinations verifies iteration and as lvalue expand to concrete destinations. |
| [`TestIncludeExpansionUsesSourcePosition`](../src/something/pipeline_functional_test.go#L296) | test | 296-314 | `func TestIncludeExpansionUsesSourcePosition(t *testing.T)` | TestIncludeExpansionUsesSourcePosition verifies include expansion uses source position. |
| [`TestIncludeCyclesReportTheDependencyChain`](../src/something/pipeline_functional_test.go#L317) | test | 317-334 | `func TestIncludeCyclesReportTheDependencyChain(t *testing.T)` | TestIncludeCyclesReportTheDependencyChain verifies include cycles report the dependency chain. |
| [`TestMacroExpansionChecksBodyInputsAndOutput`](../src/something/pipeline_functional_test.go#L337) | test | 337-367 | `func TestMacroExpansionChecksBodyInputsAndOutput(t *testing.T)` | TestMacroExpansionChecksBodyInputsAndOutput verifies macro expansion checks body inputs and output. |
| [`TestMacroExpansionPreservesDeclaredResultTypes`](../src/something/pipeline_functional_test.go#L370) | test | 370-396 | `func TestMacroExpansionPreservesDeclaredResultTypes(t *testing.T)` | TestMacroExpansionPreservesDeclaredResultTypes verifies macro expansion preserves declared result types. |
| [`TestRuntimeDiagnosticTypeNamesAndKeyEquality`](../src/something/pipeline_functional_test.go#L399) | test | 399-434 | `func TestRuntimeDiagnosticTypeNamesAndKeyEquality(t *testing.T)` | TestRuntimeDiagnosticTypeNamesAndKeyEquality verifies runtime diagnostic type names and key equality. |
| [`TestMacroRecursionReportsDirectAndIndirectChains`](../src/something/pipeline_functional_test.go#L437) | test | 437-451 | `func TestMacroRecursionReportsDirectAndIndirectChains(t *testing.T)` | TestMacroRecursionReportsDirectAndIndirectChains verifies macro recursion reports direct and indirect chains. |
| [`TestValueAndTypeCyclesReportDependencyChains`](../src/something/pipeline_functional_test.go#L454) | test | 454-479 | `func TestValueAndTypeCyclesReportDependencyChains(t *testing.T)` | TestValueAndTypeCyclesReportDependencyChains verifies value and type cycles report dependency chains. |
| [`TestRequiredStatementTerminators`](../src/something/pipeline_functional_test.go#L482) | test | 482-521 | `func TestRequiredStatementTerminators(t *testing.T)` | TestRequiredStatementTerminators verifies required statement terminators. |
| [`TestCompoundDeclarationsRejectTrailingSemicolons`](../src/something/pipeline_functional_test.go#L524) | test | 524-541 | `func TestCompoundDeclarationsRejectTrailingSemicolons(t *testing.T)` | TestCompoundDeclarationsRejectTrailingSemicolons verifies compound declarations reject trailing semicolons. |
| [`TestDefinitionMembersAndLiteralElementsRequireTheirSeparators`](../src/something/pipeline_functional_test.go#L544) | test | 544-582 | `func TestDefinitionMembersAndLiteralElementsRequireTheirSeparators(t *testing.T)` | TestDefinitionMembersAndLiteralElementsRequireTheirSeparators verifies definition members and literal elements require their separators. |
| [`TestRequiredColons`](../src/something/pipeline_functional_test.go#L585) | test | 585-602 | `func TestRequiredColons(t *testing.T)` | TestRequiredColons verifies required colons. |
| [`TestEmptyInsertIsANoOp`](../src/something/pipeline_functional_test.go#L605) | test | 605-610 | `func TestEmptyInsertIsANoOp(t *testing.T)` | TestEmptyInsertIsANoOp verifies empty insert is a no op. |
| [`TestAssertValidatesSetupInstance`](../src/something/pipeline_functional_test.go#L613) | test | 613-631 | `func TestAssertValidatesSetupInstance(t *testing.T)` | TestAssertValidatesSetupInstance verifies assert validates setup instance. |
| [`TestAssertPanicsOnNonSetup`](../src/something/pipeline_functional_test.go#L634) | test | 634-641 | `func TestAssertPanicsOnNonSetup(t *testing.T)` | TestAssertPanicsOnNonSetup verifies assert panics on non setup. |
| [`TestAssertPanicsOnUndefinedTarget`](../src/something/pipeline_functional_test.go#L644) | test | 644-648 | `func TestAssertPanicsOnUndefinedTarget(t *testing.T)` | TestAssertPanicsOnUndefinedTarget verifies assert panics on undefined target. |
| [`TestAssertTriggersErrorOnInvalidInstance`](../src/something/pipeline_functional_test.go#L651) | test | 651-664 | `func TestAssertTriggersErrorOnInvalidInstance(t *testing.T)` | TestAssertTriggersErrorOnInvalidInstance verifies assert triggers error on invalid instance. |
| [`TestAssertErrorIncludesInstanceLocation`](../src/something/pipeline_functional_test.go#L667) | test | 667-691 | `func TestAssertErrorIncludesInstanceLocation(t *testing.T)` | TestAssertErrorIncludesInstanceLocation verifies assert error includes instance location. |
| [`TestAssertPassesOnValidInstance`](../src/something/pipeline_functional_test.go#L694) | test | 694-708 | `func TestAssertPassesOnValidInstance(t *testing.T)` | TestAssertPassesOnValidInstance verifies assert passes on valid instance. |
| [`TestIfTrueExecutesBody`](../src/something/pipeline_functional_test.go#L711) | test | 711-721 | `func TestIfTrueExecutesBody(t *testing.T)` | TestIfTrueExecutesBody verifies if true executes body. |
| [`TestIfFalseSkipsBody`](../src/something/pipeline_functional_test.go#L724) | test | 724-734 | `func TestIfFalseSkipsBody(t *testing.T)` | TestIfFalseSkipsBody verifies if false skips body. |
| [`TestIfWithSingleStatement`](../src/something/pipeline_functional_test.go#L737) | test | 737-745 | `func TestIfWithSingleStatement(t *testing.T)` | TestIfWithSingleStatement verifies if with single statement. |
| [`TestIfWithFalseSingleStatement`](../src/something/pipeline_functional_test.go#L748) | test | 748-756 | `func TestIfWithFalseSingleStatement(t *testing.T)` | TestIfWithFalseSingleStatement verifies if with false single statement. |
| [`TestErrorDirectivePanics`](../src/something/pipeline_functional_test.go#L759) | test | 759-763 | `func TestErrorDirectivePanics(t *testing.T)` | TestErrorDirectivePanics verifies error directive panics. |
| [`TestErrorDirectiveUsesInterpolation`](../src/something/pipeline_functional_test.go#L766) | test | 766-770 | `func TestErrorDirectiveUsesInterpolation(t *testing.T)` | TestErrorDirectiveUsesInterpolation verifies error directive uses interpolation. |
| [`TestComparisonOperators`](../src/something/pipeline_functional_test.go#L773) | test | 773-793 | `func TestComparisonOperators(t *testing.T)` | TestComparisonOperators verifies comparison operators. |
| [`TestBooleanOperators`](../src/something/pipeline_functional_test.go#L796) | test | 796-814 | `func TestBooleanOperators(t *testing.T)` | TestBooleanOperators verifies boolean operators. |
| [`TestShortCircuitAnd`](../src/something/pipeline_functional_test.go#L817) | test | 817-834 | `func TestShortCircuitAnd(t *testing.T)` | TestShortCircuitAnd verifies short circuit and. |
| [`TestShortCircuitOr`](../src/something/pipeline_functional_test.go#L837) | test | 837-854 | `func TestShortCircuitOr(t *testing.T)` | TestShortCircuitOr verifies short circuit or. |
| [`TestParenthesizedGrouping`](../src/something/pipeline_functional_test.go#L857) | test | 857-864 | `func TestParenthesizedGrouping(t *testing.T)` | TestParenthesizedGrouping verifies parenthesized grouping. |
| [`TestMatchExpression`](../src/something/pipeline_functional_test.go#L867) | test | 867-875 | `func TestMatchExpression(t *testing.T)` | TestMatchExpression verifies match expression. |
| [`TestMatchInvalidRegex`](../src/something/pipeline_functional_test.go#L878) | test | 878-882 | `func TestMatchInvalidRegex(t *testing.T)` | TestMatchInvalidRegex verifies match invalid regex. |
| [`TestLenOnArray`](../src/something/pipeline_functional_test.go#L885) | test | 885-893 | `func TestLenOnArray(t *testing.T)` | TestLenOnArray verifies len on array. |
| [`TestLenOnEmptyArray`](../src/something/pipeline_functional_test.go#L896) | test | 896-904 | `func TestLenOnEmptyArray(t *testing.T)` | TestLenOnEmptyArray verifies len on empty array. |
| [`TestLenOnMapping`](../src/something/pipeline_functional_test.go#L907) | test | 907-915 | `func TestLenOnMapping(t *testing.T)` | TestLenOnMapping verifies len on mapping. |
| [`TestOperatorPrecedence`](../src/something/pipeline_functional_test.go#L918) | test | 918-926 | `func TestOperatorPrecedence(t *testing.T)` | TestOperatorPrecedence verifies operator precedence. |
| [`TestStringEscapingDoubleBrace`](../src/something/pipeline_functional_test.go#L929) | test | 929-937 | `func TestStringEscapingDoubleBrace(t *testing.T)` | TestStringEscapingDoubleBrace verifies string escaping double brace. |
| [`TestNestedIfInAssert`](../src/something/pipeline_functional_test.go#L940) | test | 940-959 | `func TestNestedIfInAssert(t *testing.T)` | TestNestedIfInAssert verifies nested if in assert. |
| [`TestUnknownDirectiveErrorMessage`](../src/something/pipeline_functional_test.go#L962) | test | 962-966 | `func TestUnknownDirectiveErrorMessage(t *testing.T)` | TestUnknownDirectiveErrorMessage verifies unknown directive error message. |

### [`src/something/typechecker.go`](../src/something/typechecker.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`BindingType`](../src/something/typechecker.go#L12) | struct | 12-15 | `type BindingType struct { Type TypeRef Private bool }` | BindingType is the checked type and visibility of a scope member. |
| [`ScopeType`](../src/something/typechecker.go#L18) | struct | 18-20 | `type ScopeType struct { Fields map[string]*BindingType }` | ScopeType is the checked structural type of a scope value. |
| [`NamespaceType`](../src/something/typechecker.go#L23) | struct | 23-26 | `type NamespaceType struct { Fields map[string]*BindingType Types map[string]TypeRef }` | NamespaceType is the checked structural type of an included namespace. |
| [`EnumType`](../src/something/typechecker.go#L29) | struct | 29-34 | `type EnumType struct { Name string ValueType TypeRef Members map[string]Expression MemberList []string }` | EnumType is a resolved enum definition. |
| [`SetupType`](../src/something/typechecker.go#L37) | struct | 37-40 | `type SetupType struct { Name string Fields map[string]*FieldDefinition }` | SetupType is a resolved setup definition. |
| [`(*EnumType).typeRefMarker`](../src/something/typechecker.go#L43) | method | 43 | `func (*EnumType).typeRefMarker()` | typeRefMarker marks EnumType as a TypeRef implementation. |
| [`(*SetupType).typeRefMarker`](../src/something/typechecker.go#L46) | method | 46 | `func (*SetupType).typeRefMarker()` | typeRefMarker marks SetupType as a TypeRef implementation. |
| [`(*ScopeType).typeRefMarker`](../src/something/typechecker.go#L49) | method | 49 | `func (*ScopeType).typeRefMarker()` | typeRefMarker marks ScopeType as a TypeRef implementation. |
| [`(*NamespaceType).typeRefMarker`](../src/something/typechecker.go#L52) | method | 52 | `func (*NamespaceType).typeRefMarker()` | typeRefMarker marks NamespaceType as a TypeRef implementation. |
| [`CheckedProgram`](../src/something/typechecker.go#L55) | struct | 55-58 | `type CheckedProgram struct { Program *Program AssignmentTypes map[*Assignment]TypeRef }` | CheckedProgram is the result of the standalone type-checking phase. |
| [`staticBinding`](../src/something/typechecker.go#L61) | struct | 61-64 | `type staticBinding struct { typeRef TypeRef private bool }` | staticBinding records the inferred or declared type and assignment state of one name. |
| [`staticEnvironment`](../src/something/typechecker.go#L67) | struct | 67-71 | `type staticEnvironment struct { parent *staticEnvironment bindings map[string]*staticBinding types map[string]TypeRef }` | staticEnvironment stores lexical type bindings and named type definitions. |
| [`newStaticEnvironment`](../src/something/typechecker.go#L74) | function | 74-80 | `func newStaticEnvironment(parent *staticEnvironment) *staticEnvironment` | newStaticEnvironment creates an empty lexical type-checking scope linked to its parent. |
| [`(*staticEnvironment).lookupBinding`](../src/something/typechecker.go#L83) | method | 83-90 | `func (*staticEnvironment).lookupBinding(name string) (*staticBinding, bool)` | lookupBinding searches the current and enclosing static scopes for a binding. |
| [`(*staticEnvironment).lookupType`](../src/something/typechecker.go#L93) | method | 93-100 | `func (*staticEnvironment).lookupType(name string) (TypeRef, bool)` | lookupType searches the current and enclosing static scopes for a named type. |
| [`TypeChecker`](../src/something/typechecker.go#L103) | struct | 103-108 | `type TypeChecker struct { program *Program filepath string current *staticEnvironment assignmentTypes map[*Assignment]TypeRef }` | TypeChecker resolves and validates one fully expanded program. |
| [`NewTypeChecker`](../src/something/typechecker.go#L111) | function | 111-116 | `func NewTypeChecker(program *Program, filepath string) *TypeChecker` | NewTypeChecker constructs type checker. |
| [`(*TypeChecker).err`](../src/something/typechecker.go#L119) | method | 119-121 | `func (*TypeChecker).err(message string, location *SourceLocation, suggestion string)` | err panics with a source-located SomethingError for a semantic failure. |
| [`(*TypeChecker).Check`](../src/something/typechecker.go#L124) | method | 124-128 | `func (*TypeChecker).Check() *CheckedProgram` | Check returns semantic annotations used by evaluation. |
| [`(*TypeChecker).checkStatements`](../src/something/typechecker.go#L131) | method | 131-146 | `func (*TypeChecker).checkStatements(statements []Statement)` | checkStatements checks statements against the current invariants. |
| [`(*TypeChecker).checkAssignment`](../src/something/typechecker.go#L149) | method | 149-188 | `func (*TypeChecker).checkAssignment(assignment *Assignment)` | checkAssignment checks assignment against the current invariants. |
| [`(*TypeChecker).checkEnumDefinition`](../src/something/typechecker.go#L191) | method | 191-208 | `func (*TypeChecker).checkEnumDefinition(assignment *Assignment, definition *EnumDefinition)` | checkEnumDefinition checks enum definition against the current invariants. |
| [`(*TypeChecker).checkAssertDirective`](../src/something/typechecker.go#L211) | method | 211-230 | `func (*TypeChecker).checkAssertDirective(assertion *AssertDirective)` | checkAssertDirective checks assert directive against the current invariants. |
| [`(*TypeChecker).checkIfDirective`](../src/something/typechecker.go#L233) | method | 233-237 | `func (*TypeChecker).checkIfDirective(ifDir *IfDirective)` | checkIfDirective checks if directive against the current invariants. |
| [`(*TypeChecker).checkErrorDirective`](../src/something/typechecker.go#L240) | method | 240-243 | `func (*TypeChecker).checkErrorDirective(errDir *ErrorDirective)` | checkErrorDirective checks error directive against the current invariants. |
| [`(*TypeChecker).checkSetupDefinition`](../src/something/typechecker.go#L246) | method | 246-270 | `func (*TypeChecker).checkSetupDefinition(assignment *Assignment, definition *SetupDefinition)` | checkSetupDefinition checks setup definition against the current invariants. |
| [`(*TypeChecker).checkScopeAssignment`](../src/something/typechecker.go#L273) | method | 273-320 | `func (*TypeChecker).checkScopeAssignment(assignment *Assignment, scope *ScopeExpression, namespace bool)` | checkScopeAssignment checks scope assignment against the current invariants. |
| [`(*TypeChecker).checkNamespaceAssignment`](../src/something/typechecker.go#L323) | method | 323-326 | `func (*TypeChecker).checkNamespaceAssignment(assignment *Assignment, namespace *NamespaceExpression)` | checkNamespaceAssignment checks namespace assignment against the current invariants. |
| [`(*TypeChecker).requireTypeTarget`](../src/something/typechecker.go#L329) | method | 329-335 | `func (*TypeChecker).requireTypeTarget(assignment *Assignment) string` | requireTypeTarget requires a valid type target value. |
| [`(*TypeChecker).ensureTypeNameAvailable`](../src/something/typechecker.go#L338) | method | 338-345 | `func (*TypeChecker).ensureTypeNameAvailable(name string, location *SourceLocation)` | ensureTypeNameAvailable rejects duplicate or shadowed named type declarations. |
| [`(*TypeChecker).declareTarget`](../src/something/typechecker.go#L348) | method | 348-381 | `func (*TypeChecker).declareTarget(assignment *Assignment, typeRef TypeRef)` | declareTarget records a new assignment target and enforces declaration rules. |
| [`(*TypeChecker).resolveExistingTarget`](../src/something/typechecker.go#L384) | method | 384-399 | `func (*TypeChecker).resolveExistingTarget(target LValue, location *SourceLocation) TypeRef` | resolveExistingTarget resolves existing target from the supplied context. |
| [`(*TypeChecker).resolveStaticContainer`](../src/something/typechecker.go#L402) | method | 402-415 | `func (*TypeChecker).resolveStaticContainer(root string, accesses []Access, location *SourceLocation) (TypeRef, Access)` | resolveStaticContainer resolves static container from the supplied context. |
| [`(*TypeChecker).expressionType`](../src/something/typechecker.go#L418) | method | 418-478 | `func (*TypeChecker).expressionType(expression Expression, expected TypeRef) TypeRef` | expressionType resolves and validates the static type of an expression. |
| [`(*TypeChecker).checkBinaryOpType`](../src/something/typechecker.go#L481) | method | 481-508 | `func (*TypeChecker).checkBinaryOpType(expression *BinaryOpExpression) TypeRef` | checkBinaryOpType checks binary op type against the current invariants. |
| [`(*TypeChecker).isComparableType`](../src/something/typechecker.go#L511) | method | 511-517 | `func (*TypeChecker).isComparableType(typeRef TypeRef) bool` | isComparableType reports whether a resolved type supports ordered comparison. |
| [`(*TypeChecker).checkStringReferences`](../src/something/typechecker.go#L520) | method | 520-545 | `func (*TypeChecker).checkStringReferences(expression *StringExpression)` | checkStringReferences checks string references against the current invariants. |
| [`(*TypeChecker).constantString`](../src/something/typechecker.go#L548) | method | 548-564 | `func (*TypeChecker).constantString(expression *StringExpression) (string, bool)` | constantString returns a statically known string expression when one is available. |
| [`(*TypeChecker).referenceType`](../src/something/typechecker.go#L567) | method | 567-606 | `func (*TypeChecker).referenceType(reference *ReferenceExpression, expected TypeRef) TypeRef` | referenceType resolves the type reached by a root binding and its member accesses. |
| [`(*TypeChecker).accessType`](../src/something/typechecker.go#L609) | method | 609-669 | `func (*TypeChecker).accessType(base TypeRef, access Access, location *SourceLocation, assignment bool) TypeRef` | accessType validates one field or index access and returns the resulting type. |
| [`(*TypeChecker).arrayExpressionType`](../src/something/typechecker.go#L672) | method | 672-711 | `func (*TypeChecker).arrayExpressionType(expression *ArrayExpression, expected TypeRef) TypeRef` | arrayExpressionType checks array elements and returns the resolved array type. |
| [`(*TypeChecker).mappingExpressionType`](../src/something/typechecker.go#L714) | method | 714-740 | `func (*TypeChecker).mappingExpressionType(expression *MappingExpression, expected TypeRef) TypeRef` | mappingExpressionType checks mapping keys and values and returns the resolved mapping type. |
| [`(*TypeChecker).requireMappingKeyType`](../src/something/typechecker.go#L743) | method | 743-753 | `func (*TypeChecker).requireMappingKeyType(typeRef TypeRef, location *SourceLocation)` | requireMappingKeyType requires a valid mapping key type value. |
| [`(*TypeChecker).structExpressionType`](../src/something/typechecker.go#L756) | method | 756-801 | `func (*TypeChecker).structExpressionType(expression *StructExpression, expected TypeRef) TypeRef` | structExpressionType checks named struct fields and returns the resolved setup type. |
| [`(*TypeChecker).resolveType`](../src/something/typechecker.go#L804) | method | 804-831 | `func (*TypeChecker).resolveType(typeRef TypeRef, location *SourceLocation) TypeRef` | resolveType resolves type from the supplied context. |
| [`(*TypeChecker).requireAssignable`](../src/something/typechecker.go#L834) | method | 834-839 | `func (*TypeChecker).requireAssignable(expected, actual TypeRef, location *SourceLocation, context string)` | requireAssignable requires a valid assignable value. |
| [`(*TypeChecker).assignable`](../src/something/typechecker.go#L842) | method | 842-889 | `func (*TypeChecker).assignable(expected, actual TypeRef) bool` | assignable reports whether an actual type may be assigned to an expected type. |
| [`(*TypeChecker).structurallyAssignable`](../src/something/typechecker.go#L892) | method | 892-903 | `func (*TypeChecker).structurallyAssignable(expected, actual map[string]*BindingType) bool` | structurallyAssignable reports whether two compound types have compatible structure. |
| [`enumHasMember`](../src/something/typechecker.go#L906) | function | 906-913 | `func enumHasMember(enumType *EnumType, name string) bool` | enumHasMember reports whether a resolved enum contains a named member. |
| [`sortedBindingTypeKeys`](../src/something/typechecker.go#L916) | function | 916-923 | `func sortedBindingTypeKeys(values map[string]*BindingType) []string` | sortedBindingTypeKeys returns binding type keys in deterministic order. |
| [`(*TypeChecker).detectDependencyCycles`](../src/something/typechecker.go#L927) | method | 927-962 | `func (*TypeChecker).detectDependencyCycles()` | detectDependencyCycles reports direct and indirect value or type cycles before source-order checking reports a less useful forward-reference error. |
| [`(*TypeChecker).collectCycleNodes`](../src/something/typechecker.go#L965) | method | 965-1000 | `func (*TypeChecker).collectCycleNodes(statements []Statement, prefix string, graph map[string][]string, locations map[string]*SourceLocation)` | collectCycleNodes collects cycle nodes from the supplied inputs. |
| [`(*TypeChecker).collectCycleEdges`](../src/something/typechecker.go#L1003) | method | 1003-1078 | `func (*TypeChecker).collectCycleEdges(statements []Statement, prefix string, graph map[string][]string, visibleValues, visibleTypes map[string]string)` | collectCycleEdges collects cycle edges from the supplied inputs. |
| [`copyCycleNames`](../src/something/typechecker.go#L1081) | function | 1081-1087 | `func copyCycleNames(source map[string]string) map[string]string` | copyCycleNames copies cycle names into an independent value. |
| [`cycleTargetName`](../src/something/typechecker.go#L1090) | function | 1090-1108 | `func cycleTargetName(target LValue, prefix string) string` | cycleTargetName returns the declared root name participating in dependency analysis. |
| [`typeDependencies`](../src/something/typechecker.go#L1111) | function | 1111-1124 | `func typeDependencies(typeRef TypeRef) []string` | typeDependencies collects named types referenced by a type expression. |
| [`expressionDependencies`](../src/something/typechecker.go#L1127) | function | 1127-1195 | `func expressionDependencies(expression Expression) []string` | expressionDependencies collects root bindings referenced by an expression. |
| [`referencePath`](../src/something/typechecker.go#L1198) | function | 1198-1210 | `func referencePath(reference *ReferenceExpression) string` | referencePath renders a reference root and accesses for dependency diagnostics. |

### [`src/something/typechecker_functional_test.go`](../src/something/typechecker_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestEvalTypedVar_Functional`](../src/something/typechecker_functional_test.go#L13) | test | 13-18 | `func TestEvalTypedVar_Functional(t *testing.T)` | TestEvalTypedVar_Functional verifies eval typed var functional. |
| [`TestEvalTypedVarTypeMismatch_Functional`](../src/something/typechecker_functional_test.go#L21) | test | 21-25 | `func TestEvalTypedVarTypeMismatch_Functional(t *testing.T)` | TestEvalTypedVarTypeMismatch_Functional verifies eval typed var type mismatch functional. |
| [`TestEvalTypeMismatchFloat_Functional`](../src/something/typechecker_functional_test.go#L28) | test | 28-32 | `func TestEvalTypeMismatchFloat_Functional(t *testing.T)` | TestEvalTypeMismatchFloat_Functional verifies eval type mismatch float functional. |
| [`TestEvalTypeMismatchBoolean_Functional`](../src/something/typechecker_functional_test.go#L35) | test | 35-39 | `func TestEvalTypeMismatchBoolean_Functional(t *testing.T)` | TestEvalTypeMismatchBoolean_Functional verifies eval type mismatch boolean functional. |
| [`TestEvalTypeMismatchTimestamp_Functional`](../src/something/typechecker_functional_test.go#L42) | test | 42-46 | `func TestEvalTypeMismatchTimestamp_Functional(t *testing.T)` | TestEvalTypeMismatchTimestamp_Functional verifies eval type mismatch timestamp functional. |
| [`TestEvalValidTimestamp_Functional`](../src/something/typechecker_functional_test.go#L49) | test | 49-54 | `func TestEvalValidTimestamp_Functional(t *testing.T)` | TestEvalValidTimestamp_Functional verifies eval valid timestamp functional. |
| [`TestEvalTypeRefResolution_Functional`](../src/something/typechecker_functional_test.go#L57) | test | 57-65 | `func TestEvalTypeRefResolution_Functional(t *testing.T)` | TestEvalTypeRefResolution_Functional verifies eval type ref resolution functional. |
| [`TestEvalTypeRefUnknown_Functional`](../src/something/typechecker_functional_test.go#L68) | test | 68-72 | `func TestEvalTypeRefUnknown_Functional(t *testing.T)` | TestEvalTypeRefUnknown_Functional verifies eval type ref unknown functional. |
| [`TestEvalFormatUnknownTypeSuggestion_Functional`](../src/something/typechecker_functional_test.go#L75) | test | 75-79 | `func TestEvalFormatUnknownTypeSuggestion_Functional(t *testing.T)` | TestEvalFormatUnknownTypeSuggestion_Functional verifies eval format unknown type suggestion functional. |
| [`TestEvalResolveTypeRefArray_Functional`](../src/something/typechecker_functional_test.go#L82) | test | 82-88 | `func TestEvalResolveTypeRefArray_Functional(t *testing.T)` | TestEvalResolveTypeRefArray_Functional verifies eval resolve type ref array functional. |
| [`TestEvalTypedArrayRejectsWrongElementType_Functional`](../src/something/typechecker_functional_test.go#L91) | test | 91-98 | `func TestEvalTypedArrayRejectsWrongElementType_Functional(t *testing.T)` | TestEvalTypedArrayRejectsWrongElementType_Functional verifies eval typed array rejects wrong element type functional. |
| [`TestEvalResolveTypeRefMapping_Functional`](../src/something/typechecker_functional_test.go#L101) | test | 101-107 | `func TestEvalResolveTypeRefMapping_Functional(t *testing.T)` | TestEvalResolveTypeRefMapping_Functional verifies eval resolve type ref mapping functional. |
| [`TestEvalTypedMappingRejectsWrongValueType_Functional`](../src/something/typechecker_functional_test.go#L110) | test | 110-117 | `func TestEvalTypedMappingRejectsWrongValueType_Functional(t *testing.T)` | TestEvalTypedMappingRejectsWrongValueType_Functional verifies eval typed mapping rejects wrong value type functional. |
| [`TestEvalTypeCheckScope_Functional`](../src/something/typechecker_functional_test.go#L120) | test | 120-124 | `func TestEvalTypeCheckScope_Functional(t *testing.T)` | TestEvalTypeCheckScope_Functional verifies eval type check scope functional. |
| [`TestEvalTypeCheckNamespace_Functional`](../src/something/typechecker_functional_test.go#L127) | test | 127-132 | `func TestEvalTypeCheckNamespace_Functional(t *testing.T)` | TestEvalTypeCheckNamespace_Functional verifies eval type check namespace functional. |
| [`TestEvalParseExplicitTypeScope_Functional`](../src/something/typechecker_functional_test.go#L135) | test | 135-141 | `func TestEvalParseExplicitTypeScope_Functional(t *testing.T)` | TestEvalParseExplicitTypeScope_Functional verifies eval parse explicit type scope functional. |
| [`TestEvalStructTypeCheckNested_Functional`](../src/something/typechecker_functional_test.go#L144) | test | 144-148 | `func TestEvalStructTypeCheckNested_Functional(t *testing.T)` | TestEvalStructTypeCheckNested_Functional verifies eval struct type check nested functional. |
| [`TestEvalStructMissingRequired_Functional`](../src/something/typechecker_functional_test.go#L151) | test | 151-155 | `func TestEvalStructMissingRequired_Functional(t *testing.T)` | TestEvalStructMissingRequired_Functional verifies eval struct missing required functional. |
| [`TestEvalStructUnknownField_Functional`](../src/something/typechecker_functional_test.go#L158) | test | 158-162 | `func TestEvalStructUnknownField_Functional(t *testing.T)` | TestEvalStructUnknownField_Functional verifies eval struct unknown field functional. |
| [`TestEvalStructTypeMismatch_Functional`](../src/something/typechecker_functional_test.go#L165) | test | 165-169 | `func TestEvalStructTypeMismatch_Functional(t *testing.T)` | TestEvalStructTypeMismatch_Functional verifies eval struct type mismatch functional. |
| [`TestEvalStructDefault_Functional`](../src/something/typechecker_functional_test.go#L172) | test | 172-178 | `func TestEvalStructDefault_Functional(t *testing.T)` | TestEvalStructDefault_Functional verifies eval struct default functional. |
| [`TestResolveTypeRefArray_Functional`](../src/something/typechecker_functional_test.go#L181) | test | 181-189 | `func TestResolveTypeRefArray_Functional(t *testing.T)` | TestResolveTypeRefArray_Functional verifies resolve type ref array functional. |
| [`TestResolveTypeRefMapping_Functional`](../src/something/typechecker_functional_test.go#L192) | test | 192-201 | `func TestResolveTypeRefMapping_Functional(t *testing.T)` | TestResolveTypeRefMapping_Functional verifies resolve type ref mapping functional. |
| [`TestResolveTypeRefEnumKey_Functional`](../src/something/typechecker_functional_test.go#L204) | test | 204-215 | `func TestResolveTypeRefEnumKey_Functional(t *testing.T)` | TestResolveTypeRefEnumKey_Functional verifies resolve type ref enum key functional. |
| [`TestTypeRefDisplayNameAll_Functional`](../src/something/typechecker_functional_test.go#L218) | test | 218-228 | `func TestTypeRefDisplayNameAll_Functional(t *testing.T)` | TestTypeRefDisplayNameAll_Functional verifies type ref display name all functional. |
| [`TestTypeRefDisplayNameMapping_Functional`](../src/something/typechecker_functional_test.go#L231) | test | 231-236 | `func TestTypeRefDisplayNameMapping_Functional(t *testing.T)` | TestTypeRefDisplayNameMapping_Functional verifies type ref display name mapping functional. |
| [`TestTypeRefDisplayNameArray_Functional`](../src/something/typechecker_functional_test.go#L239) | test | 239-244 | `func TestTypeRefDisplayNameArray_Functional(t *testing.T)` | TestTypeRefDisplayNameArray_Functional verifies type ref display name array functional. |
| [`TestTypeRefDisplayNameEnumKey_Functional`](../src/something/typechecker_functional_test.go#L247) | test | 247-252 | `func TestTypeRefDisplayNameEnumKey_Functional(t *testing.T)` | TestTypeRefDisplayNameEnumKey_Functional verifies type ref display name enum key functional. |
| [`TestValidExprKindsForTypeAll_Functional`](../src/something/typechecker_functional_test.go#L255) | test | 255-265 | `func TestValidExprKindsForTypeAll_Functional(t *testing.T)` | TestValidExprKindsForTypeAll_Functional verifies valid expr kinds for type all functional. |
| [`TestValidExprKindsPrimString_Functional`](../src/something/typechecker_functional_test.go#L268) | test | 268-273 | `func TestValidExprKindsPrimString_Functional(t *testing.T)` | TestValidExprKindsPrimString_Functional verifies valid expr kinds prim string functional. |
| [`TestValidExprKindsPrimFloat_Functional`](../src/something/typechecker_functional_test.go#L276) | test | 276-281 | `func TestValidExprKindsPrimFloat_Functional(t *testing.T)` | TestValidExprKindsPrimFloat_Functional verifies valid expr kinds prim float functional. |
| [`TestValidExprKindsPrimTimestamp_Functional`](../src/something/typechecker_functional_test.go#L284) | test | 284-289 | `func TestValidExprKindsPrimTimestamp_Functional(t *testing.T)` | TestValidExprKindsPrimTimestamp_Functional verifies valid expr kinds prim timestamp functional. |
| [`TestValidExprKindsPrimBoolean_Functional`](../src/something/typechecker_functional_test.go#L292) | test | 292-297 | `func TestValidExprKindsPrimBoolean_Functional(t *testing.T)` | TestValidExprKindsPrimBoolean_Functional verifies valid expr kinds prim boolean functional. |
| [`TestFormatUnknownTypeSuggestionNoSetups_Functional`](../src/something/typechecker_functional_test.go#L300) | test | 300-304 | `func TestFormatUnknownTypeSuggestionNoSetups_Functional(t *testing.T)` | TestFormatUnknownTypeSuggestionNoSetups_Functional verifies format unknown type suggestion no setups functional. |
| [`TestResolveTypeRefUnknown_Functional`](../src/something/typechecker_functional_test.go#L307) | test | 307-312 | `func TestResolveTypeRefUnknown_Functional(t *testing.T)` | TestResolveTypeRefUnknown_Functional verifies resolve type ref unknown functional. |

### [`src/something/typechecker_unit_test.go`](../src/something/typechecker_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`newTestTypeChecker`](../src/something/typechecker_unit_test.go#L16) | function | 16-18 | `func newTestTypeChecker() *TypeChecker` | newTestTypeChecker supports the package test suite's new test type checker setup or assertions. |
| [`TestAssignablePrimitiveMatching`](../src/something/typechecker_unit_test.go#L21) | test | 21-39 | `func TestAssignablePrimitiveMatching(t *testing.T)` | TestAssignablePrimitiveMatching verifies assignable primitive matching. |
| [`TestAssignablePrimitiveMismatched`](../src/something/typechecker_unit_test.go#L42) | test | 42-61 | `func TestAssignablePrimitiveMismatched(t *testing.T)` | TestAssignablePrimitiveMismatched verifies assignable primitive mismatched. |
| [`TestAssignableFloatFromInt`](../src/something/typechecker_unit_test.go#L64) | test | 64-74 | `func TestAssignableFloatFromInt(t *testing.T)` | TestAssignableFloatFromInt verifies assignable float from int. |
| [`TestAssignableTimestampFromString`](../src/something/typechecker_unit_test.go#L77) | test | 77-87 | `func TestAssignableTimestampFromString(t *testing.T)` | TestAssignableTimestampFromString verifies assignable timestamp from string. |
| [`TestAssignableScopeTypes`](../src/something/typechecker_unit_test.go#L90) | test | 90-104 | `func TestAssignableScopeTypes(t *testing.T)` | TestAssignableScopeTypes verifies assignable scope types. |
| [`TestAssignableNamespaceTypes`](../src/something/typechecker_unit_test.go#L107) | test | 107-116 | `func TestAssignableNamespaceTypes(t *testing.T)` | TestAssignableNamespaceTypes verifies assignable namespace types. |
| [`TestAssignableEnumTypes`](../src/something/typechecker_unit_test.go#L119) | test | 119-134 | `func TestAssignableEnumTypes(t *testing.T)` | TestAssignableEnumTypes verifies assignable enum types. |
| [`TestAssignableSetupTypes`](../src/something/typechecker_unit_test.go#L137) | test | 137-150 | `func TestAssignableSetupTypes(t *testing.T)` | TestAssignableSetupTypes verifies assignable setup types. |
| [`TestAssignableNilTypes`](../src/something/typechecker_unit_test.go#L153) | test | 153-159 | `func TestAssignableNilTypes(t *testing.T)` | TestAssignableNilTypes verifies assignable nil types. |
| [`TestAssignableArrayTypes`](../src/something/typechecker_unit_test.go#L162) | test | 162-178 | `func TestAssignableArrayTypes(t *testing.T)` | TestAssignableArrayTypes verifies assignable array types. |
| [`TestAssignableMappingTypes`](../src/something/typechecker_unit_test.go#L181) | test | 181-198 | `func TestAssignableMappingTypes(t *testing.T)` | TestAssignableMappingTypes verifies assignable mapping types. |
| [`TestAssignableEnumKeyTypes`](../src/something/typechecker_unit_test.go#L201) | test | 201-224 | `func TestAssignableEnumKeyTypes(t *testing.T)` | TestAssignableEnumKeyTypes verifies assignable enum key types. |
| [`TestAssignableScopeStructurally`](../src/something/typechecker_unit_test.go#L227) | test | 227-248 | `func TestAssignableScopeStructurally(t *testing.T)` | TestAssignableScopeStructurally verifies assignable scope structurally. |
| [`TestStructurallyAssignable`](../src/something/typechecker_unit_test.go#L251) | test | 251-275 | `func TestStructurallyAssignable(t *testing.T)` | TestStructurallyAssignable verifies structurally assignable. |
| [`TestEnumHasMemberFound`](../src/something/typechecker_unit_test.go#L278) | test | 278-292 | `func TestEnumHasMemberFound(t *testing.T)` | TestEnumHasMemberFound verifies enum has member found. |
| [`TestEnumHasMemberNotFound`](../src/something/typechecker_unit_test.go#L295) | test | 295-306 | `func TestEnumHasMemberNotFound(t *testing.T)` | TestEnumHasMemberNotFound verifies enum has member not found. |
| [`TestEnumHasMemberEmptyEnum`](../src/something/typechecker_unit_test.go#L309) | test | 309-314 | `func TestEnumHasMemberEmptyEnum(t *testing.T)` | TestEnumHasMemberEmptyEnum verifies enum has member empty enum. |
| [`TestSortedBindingTypeKeys`](../src/something/typechecker_unit_test.go#L317) | test | 317-331 | `func TestSortedBindingTypeKeys(t *testing.T)` | TestSortedBindingTypeKeys verifies sorted binding type keys. |
| [`TestSortedBindingTypeKeysEmpty`](../src/something/typechecker_unit_test.go#L334) | test | 334-339 | `func TestSortedBindingTypeKeysEmpty(t *testing.T)` | TestSortedBindingTypeKeysEmpty verifies sorted binding type keys empty. |
| [`TestSortedBindingTypeKeysNil`](../src/something/typechecker_unit_test.go#L342) | test | 342-347 | `func TestSortedBindingTypeKeysNil(t *testing.T)` | TestSortedBindingTypeKeysNil verifies sorted binding type keys nil. |
| [`TestTypeDependenciesTypeName`](../src/something/typechecker_unit_test.go#L350) | test | 350-356 | `func TestTypeDependenciesTypeName(t *testing.T)` | TestTypeDependenciesTypeName verifies type dependencies type name. |
| [`TestTypeDependenciesArray`](../src/something/typechecker_unit_test.go#L359) | test | 359-365 | `func TestTypeDependenciesArray(t *testing.T)` | TestTypeDependenciesArray verifies type dependencies array. |
| [`TestTypeDependenciesMapping`](../src/something/typechecker_unit_test.go#L368) | test | 368-377 | `func TestTypeDependenciesMapping(t *testing.T)` | TestTypeDependenciesMapping verifies type dependencies mapping. |
| [`TestTypeDependenciesEnumKey`](../src/something/typechecker_unit_test.go#L380) | test | 380-391 | `func TestTypeDependenciesEnumKey(t *testing.T)` | TestTypeDependenciesEnumKey verifies type dependencies enum key. |
| [`TestTypeDependenciesPrimitive`](../src/something/typechecker_unit_test.go#L394) | test | 394-404 | `func TestTypeDependenciesPrimitive(t *testing.T)` | TestTypeDependenciesPrimitive verifies type dependencies primitive. |
| [`TestExpressionDependenciesReference`](../src/something/typechecker_unit_test.go#L407) | test | 407-414 | `func TestExpressionDependenciesReference(t *testing.T)` | TestExpressionDependenciesReference verifies expression dependencies reference. |
| [`TestExpressionDependenciesArray`](../src/something/typechecker_unit_test.go#L417) | test | 417-429 | `func TestExpressionDependenciesArray(t *testing.T)` | TestExpressionDependenciesArray verifies expression dependencies array. |
| [`TestExpressionDependenciesNoDependencies`](../src/something/typechecker_unit_test.go#L432) | test | 432-438 | `func TestExpressionDependenciesNoDependencies(t *testing.T)` | TestExpressionDependenciesNoDependencies verifies expression dependencies no dependencies. |
| [`TestReferencePathSimple`](../src/something/typechecker_unit_test.go#L441) | test | 441-447 | `func TestReferencePathSimple(t *testing.T)` | TestReferencePathSimple verifies reference path simple. |
| [`TestReferencePathWithAccesses`](../src/something/typechecker_unit_test.go#L450) | test | 450-462 | `func TestReferencePathWithAccesses(t *testing.T)` | TestReferencePathWithAccesses verifies reference path with accesses. |
| [`TestReferencePathStopsAtIndex`](../src/something/typechecker_unit_test.go#L465) | test | 465-479 | `func TestReferencePathStopsAtIndex(t *testing.T)` | TestReferencePathStopsAtIndex verifies reference path stops at index. |
| [`TestReferencePathEmptyRoot`](../src/something/typechecker_unit_test.go#L482) | test | 482-488 | `func TestReferencePathEmptyRoot(t *testing.T)` | TestReferencePathEmptyRoot verifies reference path empty root. |
| [`TestCycleTargetNameIdentifier`](../src/something/typechecker_unit_test.go#L491) | test | 491-497 | `func TestCycleTargetNameIdentifier(t *testing.T)` | TestCycleTargetNameIdentifier verifies cycle target name identifier. |
| [`TestCycleTargetNameMember`](../src/something/typechecker_unit_test.go#L500) | test | 500-512 | `func TestCycleTargetNameMember(t *testing.T)` | TestCycleTargetNameMember verifies cycle target name member. |
| [`TestCycleTargetNameMemberWithIndexAccess`](../src/something/typechecker_unit_test.go#L515) | test | 515-528 | `func TestCycleTargetNameMemberWithIndexAccess(t *testing.T)` | TestCycleTargetNameMemberWithIndexAccess verifies cycle target name member with index access. |
| [`TestCycleTargetNameDefault`](../src/something/typechecker_unit_test.go#L531) | test | 531-537 | `func TestCycleTargetNameDefault(t *testing.T)` | TestCycleTargetNameDefault verifies cycle target name default. |
| [`TestCopyCycleNames`](../src/something/typechecker_unit_test.go#L540) | test | 540-554 | `func TestCopyCycleNames(t *testing.T)` | TestCopyCycleNames verifies copy cycle names. |
| [`TestCopyCycleNamesEmpty`](../src/something/typechecker_unit_test.go#L557) | test | 557-562 | `func TestCopyCycleNamesEmpty(t *testing.T)` | TestCopyCycleNamesEmpty verifies copy cycle names empty. |
| [`TestCopyCycleNamesNil`](../src/something/typechecker_unit_test.go#L565) | test | 565-570 | `func TestCopyCycleNamesNil(t *testing.T)` | TestCopyCycleNamesNil verifies copy cycle names nil. |
| [`TestConstantStringNoInterpolation`](../src/something/typechecker_unit_test.go#L573) | test | 573-583 | `func TestConstantStringNoInterpolation(t *testing.T)` | TestConstantStringNoInterpolation verifies constant string no interpolation. |
| [`TestConstantStringLiteralNoInterpolation`](../src/something/typechecker_unit_test.go#L586) | test | 586-600 | `func TestConstantStringLiteralNoInterpolation(t *testing.T)` | TestConstantStringLiteralNoInterpolation verifies constant string literal no interpolation. |
| [`TestConstantStringWithInterpolation`](../src/something/typechecker_unit_test.go#L603) | test | 603-617 | `func TestConstantStringWithInterpolation(t *testing.T)` | TestConstantStringWithInterpolation verifies constant string with interpolation. |
| [`TestConstantStringMultilineWithInterpolation`](../src/something/typechecker_unit_test.go#L620) | test | 620-627 | `func TestConstantStringMultilineWithInterpolation(t *testing.T)` | TestConstantStringMultilineWithInterpolation verifies constant string multiline with interpolation. |
| [`TestConstantStringNilLiteralEmptyMultiline`](../src/something/typechecker_unit_test.go#L630) | test | 630-640 | `func TestConstantStringNilLiteralEmptyMultiline(t *testing.T)` | TestConstantStringNilLiteralEmptyMultiline verifies constant string nil literal empty multiline. |
| [`TestNewStaticEnvironmentNoParent`](../src/something/typechecker_unit_test.go#L643) | test | 643-654 | `func TestNewStaticEnvironmentNoParent(t *testing.T)` | TestNewStaticEnvironmentNoParent verifies new static environment no parent. |
| [`TestNewStaticEnvironmentWithParent`](../src/something/typechecker_unit_test.go#L657) | test | 657-663 | `func TestNewStaticEnvironmentWithParent(t *testing.T)` | TestNewStaticEnvironmentWithParent verifies new static environment with parent. |
| [`TestAssignableExpectedIsPrimitiveKindSwitch`](../src/something/typechecker_unit_test.go#L666) | test | 666-676 | `func TestAssignableExpectedIsPrimitiveKindSwitch(t *testing.T)` | TestAssignableExpectedIsPrimitiveKindSwitch verifies assignable expected is primitive kind switch. |
| [`TestAssignableNotAssignable`](../src/something/typechecker_unit_test.go#L679) | test | 679-685 | `func TestAssignableNotAssignable(t *testing.T)` | TestAssignableNotAssignable verifies assignable not assignable. |
| [`TestAssignableBothPrimitiveDifferent`](../src/something/typechecker_unit_test.go#L688) | test | 688-705 | `func TestAssignableBothPrimitiveDifferent(t *testing.T)` | TestAssignableBothPrimitiveDifferent verifies assignable both primitive different. |
| [`TestExpressionDependenciesStructExpression`](../src/something/typechecker_unit_test.go#L708) | test | 708-719 | `func TestExpressionDependenciesStructExpression(t *testing.T)` | TestExpressionDependenciesStructExpression verifies expression dependencies struct expression. |
| [`TestExpressionDependenciesMappingExpression`](../src/something/typechecker_unit_test.go#L722) | test | 722-735 | `func TestExpressionDependenciesMappingExpression(t *testing.T)` | TestExpressionDependenciesMappingExpression verifies expression dependencies mapping expression. |
| [`TestExpressionDependenciesTypedExpression`](../src/something/typechecker_unit_test.go#L738) | test | 738-746 | `func TestExpressionDependenciesTypedExpression(t *testing.T)` | TestExpressionDependenciesTypedExpression verifies expression dependencies typed expression. |

### [`src/something/utils.go`](../src/something/utils.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`PathNotValidError`](../src/something/utils.go#L16) | struct | 16-19 | `type PathNotValidError struct { Segment string Keys []string }` | PathNotValidError is raised when a path segment cannot be found in the current dict. |
| [`(*PathNotValidError).Error`](../src/something/utils.go#L22) | method | 22-24 | `func (*PathNotValidError).Error() string` | Error returns the receiver's diagnostic message. |
| [`PathCannotBeReachedError`](../src/something/utils.go#L27) | struct | 27-30 | `type PathCannotBeReachedError struct { Segment string CurrentType string }` | PathCannotBeReachedError is raised when an intermediate value is not a dict. |
| [`(*PathCannotBeReachedError).Error`](../src/something/utils.go#L33) | method | 33-35 | `func (*PathCannotBeReachedError).Error() string` | Error returns the receiver's diagnostic message. |
| [`OutOfBoundsError`](../src/something/utils.go#L38) | struct | 38-42 | `type OutOfBoundsError struct { Index int Count int Segment string }` | OutOfBoundsError is raised when an index is out of bounds for an iteration segment. |
| [`(*OutOfBoundsError).Error`](../src/something/utils.go#L45) | method | 45-47 | `func (*OutOfBoundsError).Error() string` | Error returns the receiver's diagnostic message. |
| [`matchIterationKeys`](../src/something/utils.go#L52) | function | 52-108 | `func matchIterationKeys(data map[string]any, segment string) []string` | matchIterationKeys returns all keys in data whose name matches segment with "[iteration]" expanded to "iteration_" + 10 digits, sorted by counter ascending. When segment does not contain "[iteration]" the list contains at most one key. |
| [`checkDict`](../src/something/utils.go#L111) | function | 111-120 | `func checkDict(current any, segment string) (map[string]any, error)` | checkDict checks dict against the current invariants. |
| [`walkOnce`](../src/something/utils.go#L124) | function | 124-141 | `func walkOnce(data map[string]any, segments []string) (any, error)` | walkOnce walks segments through data. Every segment containing "[iteration]" must match exactly one key. Returns the final value. |
| [`walkIndex`](../src/something/utils.go#L145) | function | 145-166 | `func walkIndex(data map[string]any, index int, segments []string) (any, error)` | walkIndex walks segments through data, using index to select among matching iteration keys. The index is only applied to segments containing "[iteration]". |
| [`walkAll`](../src/something/utils.go#L170) | function | 170-210 | `func walkAll(data map[string]any, segments []string) ([]any, error)` | walkAll walks segments through data, branching on every "[iteration]" segment that matches multiple keys. Returns all terminal values reachable. |
| [`checkType`](../src/something/utils.go#L213) | function | 213-248 | `func checkType(val any, expected string, path []string) error` | checkType checks type against the current invariants. |
| [`GetStringOnce`](../src/something/utils.go#L251) | function | 251-260 | `func GetStringOnce(data map[string]any, path ...string) (string, error)` | GetStringOnce returns a string value at path. |
| [`GetIntegerOnce`](../src/something/utils.go#L263) | function | 263-272 | `func GetIntegerOnce(data map[string]any, path ...string) (int, error)` | GetIntegerOnce returns an integer value at path. |
| [`GetFloatOnce`](../src/something/utils.go#L275) | function | 275-287 | `func GetFloatOnce(data map[string]any, path ...string) (float64, error)` | GetFloatOnce returns a float value at path. |
| [`GetBoolOnce`](../src/something/utils.go#L290) | function | 290-299 | `func GetBoolOnce(data map[string]any, path ...string) (bool, error)` | GetBoolOnce returns a boolean value at path. |
| [`GetTimestampOnce`](../src/something/utils.go#L302) | function | 302-311 | `func GetTimestampOnce(data map[string]any, path ...string) (string, error)` | GetTimestampOnce returns a timestamp (string) value at path. |
| [`GetListOnce`](../src/something/utils.go#L314) | function | 314-323 | `func GetListOnce(data map[string]any, path ...string) ([]any, error)` | GetListOnce returns a list value at path. |
| [`GetMappingOnce`](../src/something/utils.go#L326) | function | 326-335 | `func GetMappingOnce(data map[string]any, path ...string) (map[string]any, error)` | GetMappingOnce returns a mapping (dict) value at path. |
| [`GetStructOnce`](../src/something/utils.go#L338) | function | 338-347 | `func GetStructOnce(data map[string]any, path ...string) (map[string]any, error)` | GetStructOnce returns a struct (dict) value at path. |
| [`GetScopeOnce`](../src/something/utils.go#L350) | function | 350-359 | `func GetScopeOnce(data map[string]any, path ...string) (map[string]any, error)` | GetScopeOnce returns a scope (dict) value at path. |
| [`GetEnumOnce`](../src/something/utils.go#L362) | function | 362-371 | `func GetEnumOnce(data map[string]any, path ...string) (int, error)` | GetEnumOnce returns an enum (integer ordinal) value at path. |
| [`GetStringIndex`](../src/something/utils.go#L374) | function | 374-383 | `func GetStringIndex(data map[string]any, index int, path ...string) (string, error)` | GetStringIndex returns a string at path, using index to select among iterations. |
| [`GetIntegerIndex`](../src/something/utils.go#L386) | function | 386-395 | `func GetIntegerIndex(data map[string]any, index int, path ...string) (int, error)` | GetIntegerIndex returns an integer at path, using index to select among iterations. |
| [`GetFloatIndex`](../src/something/utils.go#L398) | function | 398-410 | `func GetFloatIndex(data map[string]any, index int, path ...string) (float64, error)` | GetFloatIndex returns a float at path, using index to select among iterations. |
| [`GetBoolIndex`](../src/something/utils.go#L413) | function | 413-422 | `func GetBoolIndex(data map[string]any, index int, path ...string) (bool, error)` | GetBoolIndex returns a boolean at path, using index to select among iterations. |
| [`GetTimestampIndex`](../src/something/utils.go#L425) | function | 425-434 | `func GetTimestampIndex(data map[string]any, index int, path ...string) (string, error)` | GetTimestampIndex returns a timestamp at path, using index to select among iterations. |
| [`GetListIndex`](../src/something/utils.go#L437) | function | 437-446 | `func GetListIndex(data map[string]any, index int, path ...string) ([]any, error)` | GetListIndex returns a list at path, using index to select among iterations. |
| [`GetMappingIndex`](../src/something/utils.go#L449) | function | 449-458 | `func GetMappingIndex(data map[string]any, index int, path ...string) (map[string]any, error)` | GetMappingIndex returns a mapping at path, using index to select among iterations. |
| [`GetStructIndex`](../src/something/utils.go#L461) | function | 461-470 | `func GetStructIndex(data map[string]any, index int, path ...string) (map[string]any, error)` | GetStructIndex returns a struct at path, using index to select among iterations. |
| [`GetScopeIndex`](../src/something/utils.go#L473) | function | 473-482 | `func GetScopeIndex(data map[string]any, index int, path ...string) (map[string]any, error)` | GetScopeIndex returns a scope at path, using index to select among iterations. |
| [`GetEnumIndex`](../src/something/utils.go#L485) | function | 485-494 | `func GetEnumIndex(data map[string]any, index int, path ...string) (int, error)` | GetEnumIndex returns an enum at path, using index to select among iterations. |
| [`GetStringAll`](../src/something/utils.go#L497) | function | 497-510 | `func GetStringAll(data map[string]any, path ...string) ([]string, error)` | GetStringAll returns all string values reachable at path. |
| [`GetIntegerAll`](../src/something/utils.go#L513) | function | 513-526 | `func GetIntegerAll(data map[string]any, path ...string) ([]int, error)` | GetIntegerAll returns all integer values reachable at path. |
| [`GetFloatAll`](../src/something/utils.go#L529) | function | 529-546 | `func GetFloatAll(data map[string]any, path ...string) ([]float64, error)` | GetFloatAll returns all float values reachable at path. |
| [`GetBoolAll`](../src/something/utils.go#L549) | function | 549-562 | `func GetBoolAll(data map[string]any, path ...string) ([]bool, error)` | GetBoolAll returns all boolean values reachable at path. |
| [`GetTimestampAll`](../src/something/utils.go#L565) | function | 565-578 | `func GetTimestampAll(data map[string]any, path ...string) ([]string, error)` | GetTimestampAll returns all timestamp values reachable at path. |
| [`GetListAll`](../src/something/utils.go#L581) | function | 581-595 | `func GetListAll(data map[string]any, path ...string) ([]any, error)` | GetListAll returns all list values reachable at path. |
| [`GetMappingAll`](../src/something/utils.go#L598) | function | 598-611 | `func GetMappingAll(data map[string]any, path ...string) ([]map[string]any, error)` | GetMappingAll returns all mapping values reachable at path. |
| [`GetStructAll`](../src/something/utils.go#L614) | function | 614-627 | `func GetStructAll(data map[string]any, path ...string) ([]map[string]any, error)` | GetStructAll returns all struct values reachable at path. |
| [`GetScopeAll`](../src/something/utils.go#L630) | function | 630-643 | `func GetScopeAll(data map[string]any, path ...string) ([]map[string]any, error)` | GetScopeAll returns all scope values reachable at path. |
| [`GetEnumAll`](../src/something/utils.go#L646) | function | 646-659 | `func GetEnumAll(data map[string]any, path ...string) ([]int, error)` | GetEnumAll returns all enum (int) values reachable at path. |

### [`src/something/utils_functional_test.go`](../src/something/utils_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestUtilsWithEvaluatedConfig`](../src/something/utils_functional_test.go#L12) | test | 12-78 | `func TestUtilsWithEvaluatedConfig(t *testing.T)` | TestUtilsWithEvaluatedConfig verifies utils with evaluated config. |

### [`src/something/utils_unit_test.go`](../src/something/utils_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestMatchIterationKeysPlain`](../src/something/utils_unit_test.go#L13) | test | 13-19 | `func TestMatchIterationKeysPlain(t *testing.T)` | TestMatchIterationKeysPlain verifies match iteration keys plain. |
| [`TestMatchIterationKeysPlainMissing`](../src/something/utils_unit_test.go#L22) | test | 22-28 | `func TestMatchIterationKeysPlainMissing(t *testing.T)` | TestMatchIterationKeysPlainMissing verifies match iteration keys plain missing. |
| [`TestMatchIterationKeysWildcard`](../src/something/utils_unit_test.go#L31) | test | 31-43 | `func TestMatchIterationKeysWildcard(t *testing.T)` | TestMatchIterationKeysWildcard verifies match iteration keys wildcard. |
| [`TestMatchIterationKeysWildcardWithSuffix`](../src/something/utils_unit_test.go#L46) | test | 46-55 | `func TestMatchIterationKeysWildcardWithSuffix(t *testing.T)` | TestMatchIterationKeysWildcardWithSuffix verifies match iteration keys wildcard with suffix. |
| [`TestMatchIterationKeysWildcardNoMatch`](../src/something/utils_unit_test.go#L58) | test | 58-64 | `func TestMatchIterationKeysWildcardNoMatch(t *testing.T)` | TestMatchIterationKeysWildcardNoMatch verifies match iteration keys wildcard no match. |
| [`TestWalkOnceSimple`](../src/something/utils_unit_test.go#L67) | test | 67-76 | `func TestWalkOnceSimple(t *testing.T)` | TestWalkOnceSimple verifies walk once simple. |
| [`TestWalkOnceNested`](../src/something/utils_unit_test.go#L79) | test | 79-88 | `func TestWalkOnceNested(t *testing.T)` | TestWalkOnceNested verifies walk once nested. |
| [`TestWalkOnceIteration`](../src/something/utils_unit_test.go#L91) | test | 91-101 | `func TestWalkOnceIteration(t *testing.T)` | TestWalkOnceIteration verifies walk once iteration. |
| [`TestWalkOnceIterationWithSuffix`](../src/something/utils_unit_test.go#L104) | test | 104-113 | `func TestWalkOnceIterationWithSuffix(t *testing.T)` | TestWalkOnceIterationWithSuffix verifies walk once iteration with suffix. |
| [`TestWalkOncePathNotValid`](../src/something/utils_unit_test.go#L116) | test | 116-125 | `func TestWalkOncePathNotValid(t *testing.T)` | TestWalkOncePathNotValid verifies walk once path not valid. |
| [`TestWalkOncePathCannotBeReached`](../src/something/utils_unit_test.go#L128) | test | 128-137 | `func TestWalkOncePathCannotBeReached(t *testing.T)` | TestWalkOncePathCannotBeReached verifies walk once path cannot be reached. |
| [`TestWalkOnceMultipleMatches`](../src/something/utils_unit_test.go#L140) | test | 140-149 | `func TestWalkOnceMultipleMatches(t *testing.T)` | TestWalkOnceMultipleMatches verifies walk once multiple matches. |
| [`TestWalkIndexSimple`](../src/something/utils_unit_test.go#L152) | test | 152-161 | `func TestWalkIndexSimple(t *testing.T)` | TestWalkIndexSimple verifies walk index simple. |
| [`TestWalkIndexIteration`](../src/something/utils_unit_test.go#L164) | test | 164-173 | `func TestWalkIndexIteration(t *testing.T)` | TestWalkIndexIteration verifies walk index iteration. |
| [`TestWalkIndexIterationLast`](../src/something/utils_unit_test.go#L176) | test | 176-185 | `func TestWalkIndexIterationLast(t *testing.T)` | TestWalkIndexIterationLast verifies walk index iteration last. |
| [`TestWalkIndexOutOfBounds`](../src/something/utils_unit_test.go#L188) | test | 188-197 | `func TestWalkIndexOutOfBounds(t *testing.T)` | TestWalkIndexOutOfBounds verifies walk index out of bounds. |
| [`TestWalkIndexNegative`](../src/something/utils_unit_test.go#L200) | test | 200-209 | `func TestWalkIndexNegative(t *testing.T)` | TestWalkIndexNegative verifies walk index negative. |
| [`TestWalkIndexPathNotValid`](../src/something/utils_unit_test.go#L212) | test | 212-221 | `func TestWalkIndexPathNotValid(t *testing.T)` | TestWalkIndexPathNotValid verifies walk index path not valid. |
| [`TestWalkAllSimple`](../src/something/utils_unit_test.go#L224) | test | 224-233 | `func TestWalkAllSimple(t *testing.T)` | TestWalkAllSimple verifies walk all simple. |
| [`TestWalkAllIteration`](../src/something/utils_unit_test.go#L236) | test | 236-248 | `func TestWalkAllIteration(t *testing.T)` | TestWalkAllIteration verifies walk all iteration. |
| [`TestWalkAllIterationWithSuffix`](../src/something/utils_unit_test.go#L251) | test | 251-260 | `func TestWalkAllIterationWithSuffix(t *testing.T)` | TestWalkAllIterationWithSuffix verifies walk all iteration with suffix. |
| [`TestWalkAllEmptyResult`](../src/something/utils_unit_test.go#L263) | test | 263-272 | `func TestWalkAllEmptyResult(t *testing.T)` | TestWalkAllEmptyResult verifies walk all empty result. |
| [`TestWalkAllPathNotValid`](../src/something/utils_unit_test.go#L275) | test | 275-284 | `func TestWalkAllPathNotValid(t *testing.T)` | TestWalkAllPathNotValid verifies walk all path not valid. |
| [`TestGetStringOnce`](../src/something/utils_unit_test.go#L287) | test | 287-296 | `func TestGetStringOnce(t *testing.T)` | TestGetStringOnce verifies get string once. |
| [`TestGetStringOnceTypeError`](../src/something/utils_unit_test.go#L299) | test | 299-308 | `func TestGetStringOnceTypeError(t *testing.T)` | TestGetStringOnceTypeError verifies get string once type error. |
| [`TestGetIntegerOnce`](../src/something/utils_unit_test.go#L311) | test | 311-320 | `func TestGetIntegerOnce(t *testing.T)` | TestGetIntegerOnce verifies get integer once. |
| [`TestGetFloatOnce`](../src/something/utils_unit_test.go#L323) | test | 323-332 | `func TestGetFloatOnce(t *testing.T)` | TestGetFloatOnce verifies get float once. |
| [`TestGetFloatOnceFromInt`](../src/something/utils_unit_test.go#L335) | test | 335-344 | `func TestGetFloatOnceFromInt(t *testing.T)` | TestGetFloatOnceFromInt verifies get float once from int. |
| [`TestGetBoolOnce`](../src/something/utils_unit_test.go#L347) | test | 347-356 | `func TestGetBoolOnce(t *testing.T)` | TestGetBoolOnce verifies get bool once. |
| [`TestGetTimestampOnce`](../src/something/utils_unit_test.go#L359) | test | 359-368 | `func TestGetTimestampOnce(t *testing.T)` | TestGetTimestampOnce verifies get timestamp once. |
| [`TestGetListOnce`](../src/something/utils_unit_test.go#L371) | test | 371-380 | `func TestGetListOnce(t *testing.T)` | TestGetListOnce verifies get list once. |
| [`TestGetMappingOnce`](../src/something/utils_unit_test.go#L383) | test | 383-392 | `func TestGetMappingOnce(t *testing.T)` | TestGetMappingOnce verifies get mapping once. |
| [`TestGetStructOnce`](../src/something/utils_unit_test.go#L395) | test | 395-404 | `func TestGetStructOnce(t *testing.T)` | TestGetStructOnce verifies get struct once. |
| [`TestGetScopeOnce`](../src/something/utils_unit_test.go#L407) | test | 407-418 | `func TestGetScopeOnce(t *testing.T)` | TestGetScopeOnce verifies get scope once. |
| [`TestGetEnumOnce`](../src/something/utils_unit_test.go#L421) | test | 421-430 | `func TestGetEnumOnce(t *testing.T)` | TestGetEnumOnce verifies get enum once. |
| [`TestGetEnumOnceTypeError`](../src/something/utils_unit_test.go#L433) | test | 433-439 | `func TestGetEnumOnceTypeError(t *testing.T)` | TestGetEnumOnceTypeError verifies get enum once type error. |
| [`TestGetStringIndex`](../src/something/utils_unit_test.go#L442) | test | 442-451 | `func TestGetStringIndex(t *testing.T)` | TestGetStringIndex verifies get string index. |
| [`TestGetIntegerIndex`](../src/something/utils_unit_test.go#L454) | test | 454-463 | `func TestGetIntegerIndex(t *testing.T)` | TestGetIntegerIndex verifies get integer index. |
| [`TestGetFloatIndex`](../src/something/utils_unit_test.go#L466) | test | 466-478 | `func TestGetFloatIndex(t *testing.T)` | TestGetFloatIndex verifies get float index. |
| [`TestGetBoolIndex`](../src/something/utils_unit_test.go#L481) | test | 481-492 | `func TestGetBoolIndex(t *testing.T)` | TestGetBoolIndex verifies get bool index. |
| [`TestGetTimestampIndex`](../src/something/utils_unit_test.go#L495) | test | 495-506 | `func TestGetTimestampIndex(t *testing.T)` | TestGetTimestampIndex verifies get timestamp index. |
| [`TestGetListIndex`](../src/something/utils_unit_test.go#L509) | test | 509-520 | `func TestGetListIndex(t *testing.T)` | TestGetListIndex verifies get list index. |
| [`TestGetMappingIndex`](../src/something/utils_unit_test.go#L523) | test | 523-534 | `func TestGetMappingIndex(t *testing.T)` | TestGetMappingIndex verifies get mapping index. |
| [`TestGetStructIndex`](../src/something/utils_unit_test.go#L537) | test | 537-548 | `func TestGetStructIndex(t *testing.T)` | TestGetStructIndex verifies get struct index. |
| [`TestGetScopeIndex`](../src/something/utils_unit_test.go#L551) | test | 551-562 | `func TestGetScopeIndex(t *testing.T)` | TestGetScopeIndex verifies get scope index. |
| [`TestGetEnumIndex`](../src/something/utils_unit_test.go#L565) | test | 565-576 | `func TestGetEnumIndex(t *testing.T)` | TestGetEnumIndex verifies get enum index. |
| [`TestGetStringIndexOutOfBounds`](../src/something/utils_unit_test.go#L579) | test | 579-588 | `func TestGetStringIndexOutOfBounds(t *testing.T)` | TestGetStringIndexOutOfBounds verifies get string index out of bounds. |
| [`TestGetStringAll`](../src/something/utils_unit_test.go#L591) | test | 591-603 | `func TestGetStringAll(t *testing.T)` | TestGetStringAll verifies get string all. |
| [`TestGetIntegerAll`](../src/something/utils_unit_test.go#L606) | test | 606-618 | `func TestGetIntegerAll(t *testing.T)` | TestGetIntegerAll verifies get integer all. |
| [`TestGetFloatAll`](../src/something/utils_unit_test.go#L621) | test | 621-633 | `func TestGetFloatAll(t *testing.T)` | TestGetFloatAll verifies get float all. |
| [`TestGetBoolAll`](../src/something/utils_unit_test.go#L636) | test | 636-651 | `func TestGetBoolAll(t *testing.T)` | TestGetBoolAll verifies get bool all. |
| [`TestGetTimestampAll`](../src/something/utils_unit_test.go#L654) | test | 654-666 | `func TestGetTimestampAll(t *testing.T)` | TestGetTimestampAll verifies get timestamp all. |
| [`TestGetListAll`](../src/something/utils_unit_test.go#L669) | test | 669-681 | `func TestGetListAll(t *testing.T)` | TestGetListAll verifies get list all. |
| [`TestGetMappingAll`](../src/something/utils_unit_test.go#L684) | test | 684-696 | `func TestGetMappingAll(t *testing.T)` | TestGetMappingAll verifies get mapping all. |
| [`TestGetStructAll`](../src/something/utils_unit_test.go#L699) | test | 699-711 | `func TestGetStructAll(t *testing.T)` | TestGetStructAll verifies get struct all. |
| [`TestGetScopeAll`](../src/something/utils_unit_test.go#L714) | test | 714-726 | `func TestGetScopeAll(t *testing.T)` | TestGetScopeAll verifies get scope all. |
| [`TestGetEnumAll`](../src/something/utils_unit_test.go#L729) | test | 729-744 | `func TestGetEnumAll(t *testing.T)` | TestGetEnumAll verifies get enum all. |
| [`TestGetStringAllEmpty`](../src/something/utils_unit_test.go#L747) | test | 747-756 | `func TestGetStringAllEmpty(t *testing.T)` | TestGetStringAllEmpty verifies get string all empty. |
| [`TestPathNotValidError`](../src/something/utils_unit_test.go#L759) | test | 759-765 | `func TestPathNotValidError(t *testing.T)` | TestPathNotValidError verifies path not valid error. |
| [`TestPathCannotBeReachedError`](../src/something/utils_unit_test.go#L768) | test | 768-774 | `func TestPathCannotBeReachedError(t *testing.T)` | TestPathCannotBeReachedError verifies path cannot be reached error. |
| [`TestOutOfBoundsError`](../src/something/utils_unit_test.go#L777) | test | 777-783 | `func TestOutOfBoundsError(t *testing.T)` | TestOutOfBoundsError verifies out of bounds error. |

### [`src/something/workspace_config_integration_test.go`](../src/something/workspace_config_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestWorkspaceConfigValidDeclarations`](../src/something/workspace_config_integration_test.go#L14) | test | 14-124 | `func TestWorkspaceConfigValidDeclarations(t *testing.T)` | TestWorkspaceConfigValidDeclarations verifies workspace config valid declarations. |
| [`TestWorkspaceConfigMissingFields`](../src/something/workspace_config_integration_test.go#L127) | test | 127-142 | `func TestWorkspaceConfigMissingFields(t *testing.T)` | TestWorkspaceConfigMissingFields verifies workspace config missing fields. |
| [`TestWorkspaceConfigFormatVersionOne`](../src/something/workspace_config_integration_test.go#L145) | test | 145-168 | `func TestWorkspaceConfigFormatVersionOne(t *testing.T)` | TestWorkspaceConfigFormatVersionOne verifies workspace config format version one. |
| [`TestWorkspaceConfigEmptyCachePolicy`](../src/something/workspace_config_integration_test.go#L171) | test | 171-207 | `func TestWorkspaceConfigEmptyCachePolicy(t *testing.T)` | TestWorkspaceConfigEmptyCachePolicy verifies workspace config empty cache policy. |

### [`src/tools/coveragecheck/main.go`](../src/tools/coveragecheck/main.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`policy`](../src/tools/coveragecheck/main.go#L20) | struct | 20-26 | `type policy struct { Version int CoverageMode string ExcludePackages []string PackageMinimums map[string]float64 FileMinimums map[string]fileRule }` | policy defines repository-wide and per-file coverage thresholds. |
| [`fileRule`](../src/tools/coveragecheck/main.go#L29) | struct | 29-32 | `type fileRule struct { Minimum float64 Rationale string }` | fileRule defines a path pattern and minimum coverage threshold. |
| [`coverage`](../src/tools/coveragecheck/main.go#L35) | struct | 35-38 | `type coverage struct { Covered int Statements int }` | coverage accumulates covered and total statements for one profile scope. |
| [`(coverage).percent`](../src/tools/coveragecheck/main.go#L41) | method | 41-46 | `func (coverage).percent() float64` | percent returns a covered-line percentage, treating an empty total as complete. |
| [`main`](../src/tools/coveragecheck/main.go#L49) | function | 49-61 | `func main()` | main dispatches the analysis command selected by process arguments and exits on command failure. |
| [`check`](../src/tools/coveragecheck/main.go#L64) | function | 64-113 | `func check(profilePath, policyPath string, output io.Writer) error` | check checks check against the current invariants. |
| [`readPolicy`](../src/tools/coveragecheck/main.go#L116) | function | 116-197 | `func readPolicy(policyPath string) (policy, error)` | readPolicy reads policy from the supplied source. |
| [`readProfile`](../src/tools/coveragecheck/main.go#L200) | function | 200-269 | `func readProfile(input io.Reader) (map[string]coverage, map[string]coverage, string, error)` | readProfile reads profile from the supplied source. |
| [`writeCoverageTable`](../src/tools/coveragecheck/main.go#L272) | function | 272-294 | `func writeCoverageTable(output io.Writer, title string, minimums map[string]float64, actual map[string]coverage, failures *[]string)` | writeCoverageTable writes coverage table to the supplied destination. |

### [`src/tools/coveragecheck/main_functional_test.go`](../src/tools/coveragecheck/main_functional_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestCheckReportsTrackedCoverageAndRejectsRegression`](../src/tools/coveragecheck/main_functional_test.go#L16) | test | 16-53 | `func TestCheckReportsTrackedCoverageAndRejectsRegression(t *testing.T)` | TestCheckReportsTrackedCoverageAndRejectsRegression verifies check reports tracked coverage and rejects regression. |

### [`src/tools/coveragecheck/main_unit_test.go`](../src/tools/coveragecheck/main_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestReadProfileMergesRepeatedBlocksByHighestHitCount`](../src/tools/coveragecheck/main_unit_test.go#L13) | test | 13-31 | `func TestReadProfileMergesRepeatedBlocksByHighestHitCount(t *testing.T)` | TestReadProfileMergesRepeatedBlocksByHighestHitCount verifies read profile merges repeated blocks by highest hit count. |
| [`TestReadProfileRejectsMalformedInput`](../src/tools/coveragecheck/main_unit_test.go#L34) | test | 34-45 | `func TestReadProfileRejectsMalformedInput(t *testing.T)` | TestReadProfileRejectsMalformedInput verifies read profile rejects malformed input. |

### [`src/tools/doccheck/catalog.go`](../src/tools/doccheck/catalog.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`catalogEntry`](../src/tools/doccheck/catalog.go#L25) | struct | 25-33 | `type catalogEntry struct { file string kind string name string signature string description string start int end int }` | catalogEntry is one source-derived declaration row in the project catalog. |
| [`markdownCell`](../src/tools/doccheck/catalog.go#L36) | function | 36-39 | `func markdownCell(value string) string` | markdownCell collapses whitespace and escapes Markdown table delimiters. |
| [`markdownCode`](../src/tools/doccheck/catalog.go#L42) | function | 42-49 | `func markdownCode(value string) string` | markdownCode wraps one table-safe value in a code span that tolerates struct tags. |
| [`nodeText`](../src/tools/doccheck/catalog.go#L52) | function | 52-58 | `func nodeText(files *token.FileSet, node ast.Node) string` | nodeText formats one Go AST node as compact source text. |
| [`sourceDescription`](../src/tools/doccheck/catalog.go#L61) | function | 61-66 | `func sourceDescription(group *ast.CommentGroup) string` | sourceDescription returns only an attached source comment or the explicit fallback. |
| [`goDeclarationKind`](../src/tools/doccheck/catalog.go#L69) | function | 69-81 | `func goDeclarationKind(spec *ast.TypeSpec) string` | goDeclarationKind identifies the catalog kind of a Go type declaration. |
| [`collectGoDeclarations`](../src/tools/doccheck/catalog.go#L84) | function | 84-149 | `func collectGoDeclarations(root string) ([]catalogEntry, error)` | collectGoDeclarations walks src and returns named functions, methods, types, and tests. |
| [`renderCatalogEntries`](../src/tools/doccheck/catalog.go#L152) | function | 152-173 | `func renderCatalogEntries(entries []catalogEntry) string` | renderCatalogEntries renders declarations in per-file Markdown tables. |
| [`generatedCatalog`](../src/tools/doccheck/catalog.go#L176) | function | 176-194 | `func generatedCatalog(root string) (string, error)` | generatedCatalog composes the source-derived catalog sections. |
| [`checkCatalogDescriptions`](../src/tools/doccheck/catalog.go#L197) | function | 197-240 | `func checkCatalogDescriptions(root string) error` | checkCatalogDescriptions rejects maintained declarations without attached source documentation. |
| [`replaceGeneratedRegion`](../src/tools/doccheck/catalog.go#L243) | function | 243-250 | `func replaceGeneratedRegion(document, begin, end, generated, label string) (string, error)` | replaceGeneratedRegion replaces exactly one marked block without changing maintained prose. |
| [`catalogContent`](../src/tools/doccheck/catalog.go#L253) | function | 253-268 | `func catalogContent(root string) (string, string, string, error)` | catalogContent returns current and generated catalog document content. |
| [`checkCatalog`](../src/tools/doccheck/catalog.go#L271) | function | 271-280 | `func checkCatalog(root string) error` | checkCatalog verifies that PROJECT_CATALOG.md matches source without writing it. |
| [`runCatalogCommand`](../src/tools/doccheck/catalog.go#L283) | function | 283-331 | `func runCatalogCommand(args []string, stdout, stderr io.Writer) int` | runCatalogCommand checks or updates the source catalog. |

### [`src/tools/doccheck/catalog_unit_test.go`](../src/tools/doccheck/catalog_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestCollectGoDeclarationsIncludesFunctionsMethodsTypesAndSourceDescriptions`](../src/tools/doccheck/catalog_unit_test.go#L13) | test | 13-39 | `func TestCollectGoDeclarationsIncludesFunctionsMethodsTypesAndSourceDescriptions(t *testing.T)` | TestCollectGoDeclarationsIncludesFunctionsMethodsTypesAndSourceDescriptions verifies collect go declarations includes functions methods types and source descriptions. |
| [`TestCollectGoDeclarationsClassifiesTests`](../src/tools/doccheck/catalog_unit_test.go#L42) | test | 42-53 | `func TestCollectGoDeclarationsClassifiesTests(t *testing.T)` | TestCollectGoDeclarationsClassifiesTests verifies collect go declarations classifies tests. |
| [`TestCollectJavaScriptDeclarationsIncludesJSDocClassesMethodsAndTests`](../src/tools/doccheck/catalog_unit_test.go#L56) | test | 56-86 | `func TestCollectJavaScriptDeclarationsIncludesJSDocClassesMethodsAndTests(t *testing.T)` | TestCollectJavaScriptDeclarationsIncludesJSDocClassesMethodsAndTests verifies collect java script declarations includes js doc classes methods and tests. |
| [`TestCollectJavaScriptDeclarationsExcludesVendorAndRejectsUnsupportedSyntax`](../src/tools/doccheck/catalog_unit_test.go#L89) | test | 89-110 | `func TestCollectJavaScriptDeclarationsExcludesVendorAndRejectsUnsupportedSyntax(t *testing.T)` | TestCollectJavaScriptDeclarationsExcludesVendorAndRejectsUnsupportedSyntax verifies collect java script declarations excludes vendor and rejects unsupported syntax. |
| [`TestCatalogCheckIsNonMutatingAndUpdateChangesOnlyMarkers`](../src/tools/doccheck/catalog_unit_test.go#L113) | test | 113-137 | `func TestCatalogCheckIsNonMutatingAndUpdateChangesOnlyMarkers(t *testing.T)` | TestCatalogCheckIsNonMutatingAndUpdateChangesOnlyMarkers verifies catalog check is non mutating and update changes only markers. |
| [`TestCheckCatalogDescriptionsRejectsMissingComments`](../src/tools/doccheck/catalog_unit_test.go#L140) | test | 140-155 | `func TestCheckCatalogDescriptionsRejectsMissingComments(t *testing.T)` | TestCheckCatalogDescriptionsRejectsMissingComments verifies check catalog descriptions rejects missing comments. |
| [`TestCheckCatalogDescriptionsAcceptsMaintainedComments`](../src/tools/doccheck/catalog_unit_test.go#L158) | test | 158-166 | `func TestCheckCatalogDescriptionsAcceptsMaintainedComments(t *testing.T)` | TestCheckCatalogDescriptionsAcceptsMaintainedComments verifies check catalog descriptions accepts maintained comments. |
| [`TestCheckCatalogDescriptionsRejectsMisnamedGoComments`](../src/tools/doccheck/catalog_unit_test.go#L169) | test | 169-178 | `func TestCheckCatalogDescriptionsRejectsMisnamedGoComments(t *testing.T)` | TestCheckCatalogDescriptionsRejectsMisnamedGoComments verifies check catalog descriptions rejects misnamed go comments. |

### [`src/tools/doccheck/javascript_catalog.go`](../src/tools/doccheck/javascript_catalog.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`hasPathPart`](../src/tools/doccheck/javascript_catalog.go#L24) | function | 24-31 | `func hasPathPart(path, part string) bool` | hasPathPart reports whether a path contains an exact part. |
| [`projectJavaScriptFiles`](../src/tools/doccheck/javascript_catalog.go#L34) | function | 34-69 | `func projectJavaScriptFiles(root string) ([]string, error)` | projectJavaScriptFiles returns maintained project-authored JavaScript paths. |
| [`javascriptDoc`](../src/tools/doccheck/javascript_catalog.go#L72) | function | 72-99 | `func javascriptDoc(lines []string, declaration int) string` | javascriptDoc returns an adjacent JSDoc description and tags without inferring behavior. |
| [`collectJavaScriptDeclarations`](../src/tools/doccheck/javascript_catalog.go#L102) | function | 102-168 | `func collectJavaScriptDeclarations(root string) ([]catalogEntry, []catalogEntry, error)` | collectJavaScriptDeclarations returns named declarations and test-title entries. |

### [`src/tools/doccheck/main.go`](../src/tools/doccheck/main.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`chainRegistry`](../src/tools/doccheck/main.go#L40) | struct | 40-43 | `type chainRegistry struct { label string setting string }` | chainRegistry maps a documented migration chain to its expected directory and boundary. |
| [`migrationBoundary`](../src/tools/doccheck/main.go#L46) | struct | 46-49 | `type migrationBoundary struct { first int last int }` | migrationBoundary records the first and last migration filenames in a configured chain. |
| [`assignmentValue`](../src/tools/doccheck/main.go#L52) | function | 52-59 | `func assignmentValue(text, name, source string) (string, error)` | assignmentValue returns one quoted SOMETHING assignment or a descriptive error. |
| [`maintainedMarkdownFiles`](../src/tools/doccheck/main.go#L62) | function | 62-97 | `func maintainedMarkdownFiles(root string) ([]string, error)` | maintainedMarkdownFiles returns root guides and docs Markdown in deterministic order. |
| [`destinationPath`](../src/tools/doccheck/main.go#L100) | function | 100-125 | `func destinationPath(raw string) (string, bool)` | destinationPath returns the local file portion of a Markdown destination. |
| [`withoutInlineCode`](../src/tools/doccheck/main.go#L128) | function | 128-152 | `func withoutInlineCode(line string) string` | withoutInlineCode replaces complete backtick-delimited code spans while preserving surrounding Markdown. |
| [`markdownDestinations`](../src/tools/doccheck/main.go#L155) | function | 155-185 | `func markdownDestinations(text string) map[int][]string` | markdownDestinations returns inline and reference-style destinations outside fenced code blocks. |
| [`pathWithin`](../src/tools/doccheck/main.go#L188) | function | 188-194 | `func pathWithin(root, target string) bool` | pathWithin reports whether target is contained by root after path cleaning. |
| [`checkMarkdownLinks`](../src/tools/doccheck/main.go#L197) | function | 197-243 | `func checkMarkdownLinks(root string, markdownFiles []string) []string` | checkMarkdownLinks reports missing, unreadable, or repository-escaping local Markdown links. |
| [`checkObsoleteReferences`](../src/tools/doccheck/main.go#L246) | function | 246-265 | `func checkObsoleteReferences(root string, markdownFiles []string) []string` | checkObsoleteReferences reports exact historical references that are invalid in current documentation. |
| [`migrationVersions`](../src/tools/doccheck/main.go#L268) | function | 268-297 | `func migrationVersions(filenames []string, source string) ([]int, []string)` | migrationVersions parses ordered migration versions and reports malformed or duplicate declarations. |
| [`checkMigrationChains`](../src/tools/doccheck/main.go#L300) | function | 300-398 | `func checkMigrationChains(root string) ([]string, map[string]migrationBoundary)` | checkMigrationChains validates configured migration files and returns version boundaries by chain. |
| [`sortedDifference`](../src/tools/doccheck/main.go#L401) | function | 401-410 | `func sortedDifference(left, right map[string]bool) []string` | sortedDifference returns deterministic keys present in left and absent from right. |
| [`checkDocumentedMigrationBoundaries`](../src/tools/doccheck/main.go#L413) | function | 413-437 | `func checkDocumentedMigrationBoundaries(root string, boundaries map[string]migrationBoundary) []string` | checkDocumentedMigrationBoundaries requires designated guides to state each derived boundary. |
| [`checkRepository`](../src/tools/doccheck/main.go#L440) | function | 440-462 | `func checkRepository(root string) ([]string, int, error)` | checkRepository runs every documentation check and returns failures plus the Markdown count. |
| [`runCheck`](../src/tools/doccheck/main.go#L465) | function | 465-490 | `func runCheck(args []string, stdout, stderr io.Writer) int` | runCheck parses check arguments, executes every non-mutating check, and returns a process exit status. |
| [`run`](../src/tools/doccheck/main.go#L493) | function | 493-511 | `func run(args []string, stdout, stderr io.Writer) int` | run parses the command hierarchy and returns a process exit status. |
| [`main`](../src/tools/doccheck/main.go#L514) | function | 514-516 | `func main()` | main runs the documentation consistency command. |

### [`src/tools/doccheck/main_unit_test.go`](../src/tools/doccheck/main_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`writeTestFile`](../src/tools/doccheck/main_unit_test.go#L14) | function | 14-23 | `func writeTestFile(t *testing.T, root, relativePath, content string)` | writeTestFile creates one documentation-checker fixture file. |
| [`writeMigrationRepository`](../src/tools/doccheck/main_unit_test.go#L26) | function | 26-52 | `func writeMigrationRepository(t *testing.T, root string, metadataLast, pdfLast int)` | writeMigrationRepository creates matching metadata, PDF, and documentation fixtures. |
| [`TestDestinationPathIgnoresRemoteAndAnchorLinks`](../src/tools/doccheck/main_unit_test.go#L55) | test | 55-65 | `func TestDestinationPathIgnoresRemoteAndAnchorLinks(t *testing.T)` | TestDestinationPathIgnoresRemoteAndAnchorLinks verifies destination path ignores remote and anchor links. |
| [`TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples`](../src/tools/doccheck/main_unit_test.go#L68) | test | 68-75 | `func TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples(t *testing.T)` | TestMarkdownLinkCheckAcceptsExistingFilesAndIgnoresCodeExamples verifies markdown link check accepts existing files and ignores code examples. |
| [`TestMarkdownLinkCheckReportsMissingAndEscapingTargets`](../src/tools/doccheck/main_unit_test.go#L78) | test | 78-85 | `func TestMarkdownLinkCheckReportsMissingAndEscapingTargets(t *testing.T)` | TestMarkdownLinkCheckReportsMissingAndEscapingTargets verifies markdown link check reports missing and escaping targets. |
| [`TestMarkdownLinkCheckReportsSymbolicLinkEscape`](../src/tools/doccheck/main_unit_test.go#L88) | test | 88-103 | `func TestMarkdownLinkCheckReportsSymbolicLinkEscape(t *testing.T)` | TestMarkdownLinkCheckReportsSymbolicLinkEscape verifies markdown link check reports symbolic link escape. |
| [`TestObsoleteReferenceCheckReportsExactHistoricalNames`](../src/tools/doccheck/main_unit_test.go#L106) | test | 106-113 | `func TestObsoleteReferenceCheckReportsExactHistoricalNames(t *testing.T)` | TestObsoleteReferenceCheckReportsExactHistoricalNames verifies obsolete reference check reports exact historical names. |
| [`TestMigrationChecksAcceptMatchingFilesAndDocumentation`](../src/tools/doccheck/main_unit_test.go#L116) | test | 116-127 | `func TestMigrationChecksAcceptMatchingFilesAndDocumentation(t *testing.T)` | TestMigrationChecksAcceptMatchingFilesAndDocumentation verifies migration checks accept matching files and documentation. |
| [`TestMigrationChecksReportMissingUnconfiguredAndStaleDocumentation`](../src/tools/doccheck/main_unit_test.go#L130) | test | 130-146 | `func TestMigrationChecksReportMissingUnconfiguredAndStaleDocumentation(t *testing.T)` | TestMigrationChecksReportMissingUnconfiguredAndStaleDocumentation verifies migration checks report missing unconfigured and stale documentation. |

### [`src/tools/doccheck/markdown_format.go`](../src/tools/doccheck/markdown_format.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`markdownLineKind`](../src/tools/doccheck/markdown_format.go#L18) | function | 18-38 | `func markdownLineKind(line string) string` | markdownLineKind classifies lines relevant to the one-physical-line contract. |
| [`checkSingleLineMarkdown`](../src/tools/doccheck/markdown_format.go#L41) | function | 41-76 | `func checkSingleLineMarkdown(root string, markdownFiles []string) []string` | checkSingleLineMarkdown reports prose or list continuations outside fenced blocks. |

### [`src/tools/doccheck/markdown_format_unit_test.go`](../src/tools/doccheck/markdown_format_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestSingleLineMarkdownAcceptsOneLineContentAndMultilineFences`](../src/tools/doccheck/markdown_format_unit_test.go#L12) | test | 12-19 | `func TestSingleLineMarkdownAcceptsOneLineContentAndMultilineFences(t *testing.T)` | TestSingleLineMarkdownAcceptsOneLineContentAndMultilineFences verifies single line markdown accepts one line content and multiline fences. |
| [`TestSingleLineMarkdownReportsWrappedParagraphListAndTableContent`](../src/tools/doccheck/markdown_format_unit_test.go#L22) | test | 22-32 | `func TestSingleLineMarkdownReportsWrappedParagraphListAndTableContent(t *testing.T)` | TestSingleLineMarkdownReportsWrappedParagraphListAndTableContent verifies single line markdown reports wrapped paragraph list and table content. |

### [`src/tools/doccheck/state.go`](../src/tools/doccheck/state.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`documentState`](../src/tools/doccheck/state.go#L27) | struct | 27-30 | `type documentState struct { path string hash string }` | documentState stores one tracked documentation path, digest, and review dependents. |
| [`documentationFiles`](../src/tools/doccheck/state.go#L33) | function | 33-62 | `func documentationFiles(root string) ([]string, error)` | documentationFiles returns every maintained regular file under docs. |
| [`hashDocumentation`](../src/tools/doccheck/state.go#L65) | function | 65-80 | `func hashDocumentation(root string) ([]documentState, error)` | hashDocumentation hashes maintained documentation in deterministic path order. |
| [`markedContent`](../src/tools/doccheck/state.go#L83) | function | 83-90 | `func markedContent(document, begin, end, label string) (string, error)` | markedContent returns content strictly between one marker pair. |
| [`linkedPaths`](../src/tools/doccheck/state.go#L93) | function | 93-116 | `func linkedPaths(cell string) ([]string, error)` | linkedPaths parses path labels from one Markdown table cell. |
| [`parseDependencies`](../src/tools/doccheck/state.go#L119) | function | 119-153 | `func parseDependencies(document string) (map[string][]string, []string)` | parseDependencies reads the manually maintained review-dependency table. |
| [`parseStoredState`](../src/tools/doccheck/state.go#L156) | function | 156-204 | `func parseStoredState(document string) (map[string]string, []string)` | parseStoredState reads and validates the generated state table. |
| [`dependentGuidance`](../src/tools/doccheck/state.go#L207) | function | 207-213 | `func dependentGuidance(path string, dependencies map[string][]string) string` | dependentGuidance formats the documents that require review after one source changes. |
| [`documentStateContent`](../src/tools/doccheck/state.go#L216) | function | 216-231 | `func documentStateContent(root string) (string, []documentState, map[string][]string, map[string]string, []string, error)` | documentStateContent compares exact documentation bytes with the acknowledged state. |
| [`checkDocumentState`](../src/tools/doccheck/state.go#L234) | function | 234-269 | `func checkDocumentState(root string) ([]string, error)` | checkDocumentState reports changed, added, removed, and malformed documentation state. |
| [`renderStateTable`](../src/tools/doccheck/state.go#L272) | function | 272-288 | `func renderStateTable(states []documentState, dependencies map[string][]string) string` | renderStateTable renders current hashes and review dependents. |
| [`runStateCommand`](../src/tools/doccheck/state.go#L291) | function | 291-358 | `func runStateCommand(args []string, stdout, stderr io.Writer) int` | runStateCommand checks or explicitly acknowledges the documentation state. |

### [`src/tools/doccheck/state_unit_test.go`](../src/tools/doccheck/state_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`stateFixture`](../src/tools/doccheck/state_unit_test.go#L13) | function | 13-15 | `func stateFixture() string` | stateFixture supports the package test suite's state fixture setup or assertions. |
| [`TestDocumentationFilesExcludeStateAndReferenceTree`](../src/tools/doccheck/state_unit_test.go#L18) | test | 18-30 | `func TestDocumentationFilesExcludeStateAndReferenceTree(t *testing.T)` | TestDocumentationFilesExcludeStateAndReferenceTree verifies documentation files exclude state and reference tree. |
| [`TestDocumentStateReportsChangedAddedAndRemovedFilesWithDependents`](../src/tools/doccheck/state_unit_test.go#L33) | test | 33-56 | `func TestDocumentStateReportsChangedAddedAndRemovedFilesWithDependents(t *testing.T)` | TestDocumentStateReportsChangedAddedAndRemovedFilesWithDependents verifies document state reports changed added and removed files with dependents. |
| [`TestDocumentStateRejectsMalformedDuplicateEscapingAndExcludedRows`](../src/tools/doccheck/state_unit_test.go#L59) | test | 59-87 | `func TestDocumentStateRejectsMalformedDuplicateEscapingAndExcludedRows(t *testing.T)` | TestDocumentStateRejectsMalformedDuplicateEscapingAndExcludedRows verifies document state rejects malformed duplicate escaping and excluded rows. |
| [`TestStateCheckIsNonMutatingAndUpdatePreservesDependencies`](../src/tools/doccheck/state_unit_test.go#L90) | test | 90-113 | `func TestStateCheckIsNonMutatingAndUpdatePreservesDependencies(t *testing.T)` | TestStateCheckIsNonMutatingAndUpdatePreservesDependencies verifies state check is non mutating and update preserves dependencies. |
| [`TestStateCheckDetectsStaleGeneratedDependentsAndUpdateRepairsGeneratedRows`](../src/tools/doccheck/state_unit_test.go#L116) | test | 116-150 | `func TestStateCheckDetectsStaleGeneratedDependentsAndUpdateRepairsGeneratedRows(t *testing.T)` | TestStateCheckDetectsStaleGeneratedDependentsAndUpdateRepairsGeneratedRows verifies state check detects stale generated dependents and update repairs generated rows. |

### [`src/tools/pdf-store/main.go`](../src/tools/pdf-store/main.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`main`](../src/tools/pdf-store/main.go#L18) | function | 18-36 | `func main()` | main dispatches the analysis command selected by process arguments and exits on command failure. |
| [`add`](../src/tools/pdf-store/main.go#L39) | function | 39-41 | `func add(metadataPath, doi, filePath string) error` | add validates command arguments and adds one manual PDF through the configured database registry. |
| [`addWithRegistry`](../src/tools/pdf-store/main.go#L44) | function | 44-113 | `func addWithRegistry(metadataPath, doi, filePath, registryPath string) error` | addWithRegistry opens the metadata and PDF stores, inserts content, and drains the audit outbox. |

### [`src/tools/pdf-store/main_integration_test.go`](../src/tools/pdf-store/main_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestAddRequiresCorpusDOIAndPreservesExistingDownload`](../src/tools/pdf-store/main_integration_test.go#L19) | test | 19-171 | `func TestAddRequiresCorpusDOIAndPreservesExistingDownload(t *testing.T)` | TestAddRequiresCorpusDOIAndPreservesExistingDownload verifies add requires corpus doi and preserves existing download. |
| [`TestAddRejectsInvalidAndOversizedPDFs`](../src/tools/pdf-store/main_integration_test.go#L174) | test | 174-201 | `func TestAddRejectsInvalidAndOversizedPDFs(t *testing.T)` | TestAddRejectsInvalidAndOversizedPDFs verifies add rejects invalid and oversized pd fs. |

### [`src/tools/something-json/main.go`](../src/tools/something-json/main.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`main`](../src/tools/something-json/main.go#L15) | function | 15-35 | `func main()` | main dispatches the analysis command selected by process arguments and exits on command failure. |

### [`src/tools/something-json/main_integration_test.go`](../src/tools/something-json/main_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestLoadRealConfig`](../src/tools/something-json/main_integration_test.go#L17) | test | 17-48 | `func TestLoadRealConfig(t *testing.T)` | TestLoadRealConfig verifies load real config. |

### [`src/tools/something-json/main_unit_test.go`](../src/tools/something-json/main_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestLoadSimpleConfig`](../src/tools/something-json/main_unit_test.go#L16) | test | 16-34 | `func TestLoadSimpleConfig(t *testing.T)` | TestLoadSimpleConfig verifies load simple config. |
| [`TestLoadConfigWithValues`](../src/tools/something-json/main_unit_test.go#L37) | test | 37-57 | `func TestLoadConfigWithValues(t *testing.T)` | TestLoadConfigWithValues verifies load config with values. |
| [`TestLoadMissingFile`](../src/tools/something-json/main_unit_test.go#L60) | test | 60-68 | `func TestLoadMissingFile(t *testing.T)` | TestLoadMissingFile verifies load missing file. |

### [`src/validation/validation.go`](../src/validation/validation.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Fields`](../src/validation/validation.go#L13) | struct | 13-19 | `type Fields struct { DOI string Title string Year int Publisher string ReferenceCount int }` | Fields is the workspace-facing validation input. It contains no mutable database identity or persistence state. |
| [`IsRealDOI`](../src/validation/validation.go#L24) | function | 24-27 | `func IsRealDOI(value string) bool` | IsRealDOI checks whether a string looks like a DOI. |
| [`ValidateFields`](../src/validation/validation.go#L30) | function | 30-55 | `func ValidateFields(a Fields, authorCount int) []string` | ValidateFields runs the workspace validation rules. |
| [`sortedReasons`](../src/validation/validation.go#L58) | function | 58-78 | `func sortedReasons(m map[string]int) []string` | sortedReasons returns reasons in deterministic order. |

### [`src/validation/validation_unit_test.go`](../src/validation/validation_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestIsRealDOI`](../src/validation/validation_unit_test.go#L13) | test | 13-30 | `func TestIsRealDOI(t *testing.T)` | TestIsRealDOI verifies is real doi. |
| [`TestValidateFields`](../src/validation/validation_unit_test.go#L33) | test | 33-43 | `func TestValidateFields(t *testing.T)` | TestValidateFields verifies validate fields. |
| [`TestSortedReasons`](../src/validation/validation_unit_test.go#L46) | test | 46-52 | `func TestSortedReasons(t *testing.T)` | TestSortedReasons verifies sorted reasons. |

### [`src/workspace/cache.go`](../src/workspace/cache.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`cacheRequest`](../src/workspace/cache.go#L22) | struct | 22-27 | `type cacheRequest struct { Provider string Namespace string Identity string URL string }` | cacheRequest identifies one provider request independently of its storage layer. |
| [`cacheResponse`](../src/workspace/cache.go#L30) | struct | 30-36 | `type cacheResponse struct { Body []byte Status int Layer string Outcome manifest.CacheOutcome PayloadArtifactID int64 }` | cacheResponse records resolved payload bytes, status, layer, outcome, and artifact identity. |
| [`workspaceCache`](../src/workspace/cache.go#L41) | struct | 41-45 | `type workspaceCache struct { db *database.Database runID int64 policy manifest.CachePolicy }` | workspaceCache applies one resolved cache policy without making enrichment packages depend on SQLite. The same policy covers provider work, author, and name-search requests. |
| [`(*workspaceCache).resolve`](../src/workspace/cache.go#L48) | method | 48-127 | `func (*workspaceCache).resolve(ctx context.Context, request cacheRequest, fetch func(context.Context) *enrich.FetchResult, negative func([]byte) bool) (*cacheResponse, error)` | resolve follows the declared cache read order, validates reusable payloads, and records outcome evidence. |
| [`(*workspaceCache).fetchAndRecord`](../src/workspace/cache.go#L130) | method | 130-191 | `func (*workspaceCache).fetchAndRecord(ctx context.Context, request cacheRequest, fingerprint, layer string, fetch func(context.Context) *enrich.FetchResult, negative func([]byte) bool) (*cacheResponse, error)` | fetchAndRecord validates a network result, persists cacheable evidence, and records cache metrics and audit. |
| [`dereferenceInt64`](../src/workspace/cache.go#L194) | function | 194-199 | `func dereferenceInt64(value *int64) int64` | dereferenceInt64 returns a pointed integer or zero for nil. |
| [`cacheableNegative`](../src/workspace/cache.go#L204) | function | 204-215 | `func cacheableNegative(request cacheRequest) bool` | cacheableNegative limits negative TTLs to provider responses whose 404 or validated empty-result semantics mean "not found" rather than a malformed endpoint, authorization issue, or transient provider failure. |
| [`(*workspaceCache).extractorVersion`](../src/workspace/cache.go#L218) | method | 218-223 | `func (*workspaceCache).extractorVersion(layer string) string` | extractorVersion returns the layer-specific extractor key used to isolate active-run entries. |
| [`(*workspaceCache).incrementMetric`](../src/workspace/cache.go#L226) | method | 226-241 | `func (*workspaceCache).incrementMetric(metric, provider string) error` | incrementMetric increments a cache metric at both run-wide and provider scope. |
| [`(*workspaceCache).recordUse`](../src/workspace/cache.go#L244) | method | 244-247 | `func (*workspaceCache).recordUse(entryID int64, layer string, outcome manifest.CacheOutcome) error` | recordUse persists one run-to-cache-entry lookup outcome. |
| [`(*workspaceCache).readPayload`](../src/workspace/cache.go#L250) | method | 250-259 | `func (*workspaceCache).readPayload(artifactID int64) ([]byte, error)` | readPayload reads payload from the supplied source. |
| [`(*workspaceCache).recordAudit`](../src/workspace/cache.go#L262) | method | 262-276 | `func (*workspaceCache).recordAudit(action manifest.AuditAction, request cacheRequest, layer string, outcome manifest.CacheOutcome, entryID int64) error` | recordAudit appends cache decision evidence for one provider request. |
| [`cacheFingerprint`](../src/workspace/cache.go#L279) | function | 279-284 | `func cacheFingerprint(request cacheRequest) string` | cacheFingerprint hashes every request-affecting field deterministically. |
| [`cacheEntryExpired`](../src/workspace/cache.go#L287) | function | 287-298 | `func cacheEntryExpired(entry *database.CacheEntry, now time.Time) bool` | cacheEntryExpired reports whether a cache entry is past its parsed expiry, treating malformed expiry as stale. |

### [`src/workspace/cache_integration_test.go`](../src/workspace/cache_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`openWorkspaceCacheTest`](../src/workspace/cache_integration_test.go#L26) | function | 26-38 | `func openWorkspaceCacheTest(t *testing.T, policy manifest.CachePolicy) (*database.Database, *workspaceCache, int64)` | openWorkspaceCacheTest supports the package test suite's open workspace cache test setup or assertions. |
| [`testCacheRequest`](../src/workspace/cache_integration_test.go#L41) | function | 41-43 | `func testCacheRequest() cacheRequest` | testCacheRequest supports the package test suite's test cache request setup or assertions. |
| [`testCachePayload`](../src/workspace/cache_integration_test.go#L46) | function | 46-48 | `func testCachePayload(title string) []byte` | testCachePayload supports the package test suite's test cache payload setup or assertions. |
| [`TestWorkspaceCacheNamedPriorRunReproduction`](../src/workspace/cache_integration_test.go#L51) | test | 51-79 | `func TestWorkspaceCacheNamedPriorRunReproduction(t *testing.T)` | TestWorkspaceCacheNamedPriorRunReproduction verifies workspace cache named prior run reproduction. |
| [`TestWorkspaceCacheGlobalReuseAcrossRuns`](../src/workspace/cache_integration_test.go#L82) | test | 82-101 | `func TestWorkspaceCacheGlobalReuseAcrossRuns(t *testing.T)` | TestWorkspaceCacheGlobalReuseAcrossRuns verifies workspace cache global reuse across runs. |
| [`TestWorkspaceCacheOfflineMiss`](../src/workspace/cache_integration_test.go#L104) | test | 104-114 | `func TestWorkspaceCacheOfflineMiss(t *testing.T)` | TestWorkspaceCacheOfflineMiss verifies workspace cache offline miss. |
| [`TestWorkspaceCacheFreshBypassesGlobal`](../src/workspace/cache_integration_test.go#L117) | test | 117-144 | `func TestWorkspaceCacheFreshBypassesGlobal(t *testing.T)` | TestWorkspaceCacheFreshBypassesGlobal verifies workspace cache fresh bypasses global. |
| [`TestWorkspaceCacheStaleNegativeRelooksUp`](../src/workspace/cache_integration_test.go#L147) | test | 147-174 | `func TestWorkspaceCacheStaleNegativeRelooksUp(t *testing.T)` | TestWorkspaceCacheStaleNegativeRelooksUp verifies workspace cache stale negative relooks up. |
| [`TestWorkspaceCacheNetworkFallback`](../src/workspace/cache_integration_test.go#L177) | test | 177-209 | `func TestWorkspaceCacheNetworkFallback(t *testing.T)` | TestWorkspaceCacheNetworkFallback verifies workspace cache network fallback. |
| [`TestWorkspaceCacheRejectsMalformedSuccessAndRecovers`](../src/workspace/cache_integration_test.go#L212) | test | 212-241 | `func TestWorkspaceCacheRejectsMalformedSuccessAndRecovers(t *testing.T)` | TestWorkspaceCacheRejectsMalformedSuccessAndRecovers verifies workspace cache rejects malformed success and recovers. |
| [`TestWorkspaceCacheSkipsStoredMalformedPayload`](../src/workspace/cache_integration_test.go#L244) | test | 244-275 | `func TestWorkspaceCacheSkipsStoredMalformedPayload(t *testing.T)` | TestWorkspaceCacheSkipsStoredMalformedPayload verifies workspace cache skips stored malformed payload. |
| [`TestWorkspaceCacheORCIDNameFailureAndEmptyMatchPolicies`](../src/workspace/cache_integration_test.go#L278) | test | 278-311 | `func TestWorkspaceCacheORCIDNameFailureAndEmptyMatchPolicies(t *testing.T)` | TestWorkspaceCacheORCIDNameFailureAndEmptyMatchPolicies verifies workspace cache orcid name failure and empty match policies. |
| [`TestWorkspaceMetricUnavailableForOlderRun`](../src/workspace/cache_integration_test.go#L314) | test | 314-321 | `func TestWorkspaceMetricUnavailableForOlderRun(t *testing.T)` | TestWorkspaceMetricUnavailableForOlderRun verifies workspace metric unavailable for older run. |
| [`TestWorkspaceCrossrefCacheUsesSQLitePolicy`](../src/workspace/cache_integration_test.go#L324) | test | 324-347 | `func TestWorkspaceCrossrefCacheUsesSQLitePolicy(t *testing.T)` | TestWorkspaceCrossrefCacheUsesSQLitePolicy verifies workspace crossref cache uses sq lite policy. |
| [`TestWorkspaceOpenAlexReferenceCacheUsesSQLitePolicy`](../src/workspace/cache_integration_test.go#L350) | test | 350-379 | `func TestWorkspaceOpenAlexReferenceCacheUsesSQLitePolicy(t *testing.T)` | TestWorkspaceOpenAlexReferenceCacheUsesSQLitePolicy verifies the OpenAlex reference cache uses SQLite policy. |
| [`openWorkspaceCacheTestForDB`](../src/workspace/cache_integration_test.go#L382) | function | 382-389 | `func openWorkspaceCacheTestForDB(t *testing.T, db *database.Database, policy manifest.CachePolicy) (*database.Database, *workspaceCache, int64)` | openWorkspaceCacheTestForDB supports the package test suite's open workspace cache test for db setup or assertions. |

### [`src/workspace/config.go`](../src/workspace/config.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`Config`](../src/workspace/config.go#L23) | struct | 23-26 | `type Config struct { OriginalBytes []byte Runs []*Run }` | Config is one immutable evaluation of a workspace configuration file. OriginalBytes are retained so a run can persist exactly what was evaluated. |
| [`Run`](../src/workspace/config.go#L29) | struct | 29-32 | `type Run struct { Manifest *manifest.ResolvedManifest Enrichment *enrich.Config }` | Run is one declared workspace iteration. |
| [`Selector`](../src/workspace/config.go#L36) | function | 36-38 | `func Selector(searchID, revision string) string` | Selector identifies one workspace iteration by its stable search ID and revision label. Its CLI form is search_id@search_revision. |
| [`Load`](../src/workspace/config.go#L42) | function | 42-86 | `func Load(path string) (*Config, error)` | Load reads a workspace configuration exactly once, evaluates those bytes, and converts every workspace iteration to typed run configuration. |
| [`(*Config).Select`](../src/workspace/config.go#L90) | method | 90-121 | `func (*Config).Select(selectors []string) ([]*Run, error)` | Select returns all runs when selectors is empty. Otherwise it returns the requested iterations in declaration order and rejects unknown selectors. |
| [`parseRun`](../src/workspace/config.go#L124) | function | 124-183 | `func parseRun(entry map[string]any, configDir string) (*Run, error)` | parseRun parses run from the supplied input. |
| [`parseCachePolicy`](../src/workspace/config.go#L190) | function | 190-263 | `func parseCachePolicy(entry map[string]any, reusePolicy string) (manifest.CachePolicy, error)` | parseCachePolicy parses cache policy from the supplied input. |
| [`parseSources`](../src/workspace/config.go#L266) | function | 266-341 | `func parseSources(entry map[string]any, configDir string) ([]manifest.SourceManifest, error)` | parseSources parses sources from the supplied input. |
| [`parseProviders`](../src/workspace/config.go#L344) | function | 344-414 | `func parseProviders(entry map[string]any) ([]manifest.EnrichmentProvider, *enrich.Config, error)` | parseProviders parses providers from the supplied input. |
| [`requiredString`](../src/workspace/config.go#L417) | function | 417-423 | `func requiredString(values map[string]any, name string) (string, error)` | requiredString returns a required non-empty string from evaluated configuration. |
| [`optionalString`](../src/workspace/config.go#L426) | function | 426-432 | `func optionalString(values map[string]any, name string) string` | optionalString reads the optional string value. |
| [`nestedString`](../src/workspace/config.go#L435) | function | 435-441 | `func nestedString(values map[string]any, parent, name string) (string, error)` | nestedString reads a required string from a required nested mapping. |
| [`requiredBool`](../src/workspace/config.go#L444) | function | 444-450 | `func requiredBool(values map[string]any, name string) (bool, error)` | requiredBool returns a required Boolean from evaluated configuration. |
| [`optionalBool`](../src/workspace/config.go#L453) | function | 453-463 | `func optionalBool(values map[string]any, name string, fallback bool) (bool, error)` | optionalBool reads the optional bool value. |
| [`requiredInt`](../src/workspace/config.go#L466) | function | 466-472 | `func requiredInt(values map[string]any, name string) (int, error)` | requiredInt returns a required integer from evaluated configuration. |
| [`optionalInt`](../src/workspace/config.go#L475) | function | 475-485 | `func optionalInt(values map[string]any, name string, fallback int) (int, error)` | optionalInt reads the optional int value. |
| [`requiredPath`](../src/workspace/config.go#L488) | function | 488-497 | `func requiredPath(values map[string]any, name, configDir string) (string, error)` | requiredPath returns a required path resolved relative to the configuration directory. |
| [`stringList`](../src/workspace/config.go#L500) | function | 500-514 | `func stringList(values map[string]any, name string) ([]string, error)` | stringList reads a required list of non-empty strings. |
| [`stringMap`](../src/workspace/config.go#L517) | function | 517-519 | `func stringMap(values map[string]any, name string) (map[string]string, error)` | stringMap reads a required mapping whose keys and values are strings. |
| [`optionalStringMap`](../src/workspace/config.go#L522) | function | 522-524 | `func optionalStringMap(values map[string]any, name string) (map[string]string, error)` | optionalStringMap reads the optional string map value. |
| [`requiredStringMap`](../src/workspace/config.go#L527) | function | 527-545 | `func requiredStringMap(values map[string]any, name string, required bool) (map[string]string, error)` | requiredStringMap validates a required or optional mapping of non-empty strings. |
| [`validateCacheLayers`](../src/workspace/config.go#L548) | function | 548-572 | `func validateCacheLayers(layers []string, write bool) error` | validateCacheLayers validates cache-layer names, uniqueness, and read or write eligibility. |
| [`validateRunLayer`](../src/workspace/config.go#L575) | function | 575-586 | `func validateRunLayer(reads []string) error` | validateRunLayer checks that a resolved run:N layer has a valid run ID. |
| [`enumIntList`](../src/workspace/config.go#L590) | function | 590-607 | `func enumIntList(values map[string]any, name string, memberNames []string) ([]int, error)` | enumIntList reads a list of int enum ordinals from a map and validates they are within range of the provided member names list. |
| [`parseRawDataFilters`](../src/workspace/config.go#L612) | function | 612-642 | `func parseRawDataFilters(source map[string]any, name string) ([]manifest.RawDataFilter, error)` | parseRawDataFilters reads the "filters" field as a list of raw_data_filters structs, each containing a "filters" array of available_filters enum ordinals and a "count" integer. |
| [`contains`](../src/workspace/config.go#L645) | function | 645-652 | `func contains(values []string, target string) bool` | contains reports whether a string slice contains an exact target. |

### [`src/workspace/config_unit_test.go`](../src/workspace/config_unit_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestLoadBuildsResolvedWorkspaceRuns`](../src/workspace/config_unit_test.go#L16) | test | 16-41 | `func TestLoadBuildsResolvedWorkspaceRuns(t *testing.T)` | TestLoadBuildsResolvedWorkspaceRuns verifies load builds resolved workspace runs. |
| [`TestProductionWorkspaceConfigLoads`](../src/workspace/config_unit_test.go#L44) | test | 44-86 | `func TestProductionWorkspaceConfigLoads(t *testing.T)` | TestProductionWorkspaceConfigLoads verifies production workspace config loads. |
| [`TestSelectUsesDeclaredOrderAndRejectsUnknown`](../src/workspace/config_unit_test.go#L89) | test | 89-104 | `func TestSelectUsesDeclaredOrderAndRejectsUnknown(t *testing.T)` | TestSelectUsesDeclaredOrderAndRejectsUnknown verifies select uses declared order and rejects unknown. |
| [`TestLoadRetainsEvaluatedBytesAfterConfigChanges`](../src/workspace/config_unit_test.go#L107) | test | 107-122 | `func TestLoadRetainsEvaluatedBytesAfterConfigChanges(t *testing.T)` | TestLoadRetainsEvaluatedBytesAfterConfigChanges verifies load retains evaluated bytes after config changes. |
| [`TestLoadRejectsUnsupportedWorkspacePolicies`](../src/workspace/config_unit_test.go#L125) | test | 125-144 | `func TestLoadRejectsUnsupportedWorkspacePolicies(t *testing.T)` | TestLoadRejectsUnsupportedWorkspacePolicies verifies load rejects unsupported workspace policies. |
| [`writeConfig`](../src/workspace/config_unit_test.go#L147) | function | 147-154 | `func writeConfig(t *testing.T, text string) string` | writeConfig supports the package test suite's write config setup or assertions. |
| [`testConfig`](../src/workspace/config_unit_test.go#L157) | function | 157-286 | `func testConfig() string` | testConfig supports the package test suite's test config setup or assertions. |
| [`TestConfigHelpers`](../src/workspace/config_unit_test.go#L289) | test | 289-472 | `func TestConfigHelpers(t *testing.T)` | TestConfigHelpers verifies config helpers. |

### [`src/workspace/orcid_candidates_integration_test.go`](../src/workspace/orcid_candidates_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestORCIDNameSearchRetainsCandidatesWithoutCreatingIdentity`](../src/workspace/orcid_candidates_integration_test.go#L24) | test | 24-82 | `func TestORCIDNameSearchRetainsCandidatesWithoutCreatingIdentity(t *testing.T)` | TestORCIDNameSearchRetainsCandidatesWithoutCreatingIdentity verifies orcid name search retains candidates without creating identity. |
| [`TestWorkspacePipelinePreservesMetadataAndProviderFailureEvidence`](../src/workspace/orcid_candidates_integration_test.go#L85) | test | 85-162 | `func TestWorkspacePipelinePreservesMetadataAndProviderFailureEvidence(t *testing.T)` | TestWorkspacePipelinePreservesMetadataAndProviderFailureEvidence verifies workspace pipeline preserves metadata and provider failure evidence. |
| [`testWorkspaceRun`](../src/workspace/orcid_candidates_integration_test.go#L165) | function | 165-183 | `func testWorkspaceRun(sourcePath string) *Run` | testWorkspaceRun supports the package test suite's test workspace run setup or assertions. |

### [`src/workspace/store.go`](../src/workspace/store.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`databaseRegistryPath`](../src/workspace/store.go#L30) | function | 30-45 | `func databaseRegistryPath() string` | databaseRegistryPath returns the path to the database registry config at <repo-root>/config/database.something. It walks up from CWD looking for go.mod (src/go.mod), then goes one level up to find the config dir. |
| [`RunPipeline`](../src/workspace/store.go#L49) | function | 49-289 | `func RunPipeline(dbPath string, originalConfig []byte, run *Run, fresh bool) (runErr error)` | RunPipeline is the immutable workspace pipeline. It does not use the deprecated mutable corpus repositories. |
| [`normalizedInventoryWork`](../src/workspace/store.go#L292) | struct | 292-296 | `type normalizedInventoryWork struct { DOI string WorkID int64 PipelineRunID int64 }` | normalizedInventoryWork identifies a persisted normalized work for companion PDF registration. |
| [`syncNormalizedPDFInventory`](../src/workspace/store.go#L301) | function | 301-360 | `func syncNormalizedPDFInventory(ctx context.Context, db *database.Database, dbPath, registryPath string) (int, int, error)` | syncNormalizedPDFInventory reconciles the companion inventory from the authoritative normalized revisions. It is safe to call after a new run or when a completed execution plan is reused. |
| [`articlesWithFailedIdentityEvidence`](../src/workspace/store.go#L363) | function | 363-377 | `func articlesWithFailedIdentityEvidence(articles []*article.Article, evidence []uncertainORCIDSearchEvidence) []*article.Article` | articlesWithFailedIdentityEvidence selects articles whose identity provider search failed. |
| [`resultCountComparison`](../src/workspace/store.go#L380) | function | 380-389 | `func resultCountComparison(expected, observed int) string` | resultCountComparison classifies an observed source count as below, above, or matching its expectation. |
| [`loadWorkspaceEntries`](../src/workspace/store.go#L392) | function | 392-401 | `func loadWorkspaceEntries(fileType, path, source string) ([]map[string]string, error)` | loadWorkspaceEntries loads workspace entries from the supplied source. |
| [`cloneStringMap`](../src/workspace/store.go#L404) | function | 404-410 | `func cloneStringMap(in map[string]string) map[string]string` | cloneStringMap clones string map into an independent value. |
| [`persistWorkspaceStage`](../src/workspace/store.go#L413) | function | 413-437 | `func persistWorkspaceStage(db *database.Database, runID int64, articles []*article.Article, producerStage, stage, outcome string, reasons map[string][]string) (map[string]int64, map[string]int64, error)` | persistWorkspaceStage persists workspace stage through the owning repository. |
| [`setWorkspaceStageOutcome`](../src/workspace/store.go#L440) | function | 440-454 | `func setWorkspaceStageOutcome(db *database.Database, runID int64, articles []*article.Article, stage, outcome, reason string) error` | setWorkspaceStageOutcome sets workspace stage outcome using the supplied values. |
| [`setRunMetrics`](../src/workspace/store.go#L457) | function | 457-464 | `func setRunMetrics(db *database.Database, runID int64, metrics ...any) error` | setRunMetrics sets run metrics using the supplied values. |
| [`recordWorkspaceStage`](../src/workspace/store.go#L467) | function | 467-498 | `func recordWorkspaceStage(db *database.Database, run *Run, runID int64, name string, input any, output any) error` | recordWorkspaceStage records workspace stage. |
| [`persistWorkSnapshot`](../src/workspace/store.go#L501) | function | 501-542 | `func persistWorkSnapshot(db *database.Database, runID int64, a *article.Article, producerStage, extensionData string) (int64, int64, error)` | persistWorkSnapshot persists work snapshot through the owning repository. |
| [`workspaceRevisionExtension`](../src/workspace/store.go#L545) | struct | 545-549 | ``type workspaceRevisionExtension struct { NormalizedJournal string `json:"normalized_journal,omitempty"` ValidationReasons []string `json:"validation_reasons,omitempty"` NormalizedAuthors map[string]string `json:"normalized_authors,omitempty"` }`` | workspaceRevisionExtension stores normalized values and validation evidence outside canonical revision columns. |
| [`workspaceExtension`](../src/workspace/store.go#L552) | function | 552-561 | `func workspaceExtension(a *article.Article, reasons []string) string` | workspaceExtension serializes normalized journal, author names, and validation reasons for a revision. |
| [`validateWorkspaceArticles`](../src/workspace/store.go#L564) | function | 564-576 | `func validateWorkspaceArticles(articles []*article.Article) (valid, discarded []*article.Article, reasons map[string][]string)` | validateWorkspaceArticles partitions articles by validation outcome and records discard reasons by DOI. |
| [`normalizationFieldResult`](../src/workspace/store.go#L579) | struct | 579-583 | `type normalizationFieldResult struct { DOI string Field string Outcome string }` | normalizationFieldResult records one field's normalization outcome for an article DOI. |
| [`normalizeWorkspaceArticles`](../src/workspace/store.go#L602) | function | 602-624 | `func normalizeWorkspaceArticles(articles []*article.Article) []normalizationFieldResult` | normalizeWorkspaceArticles applies each normalizer and records one outcome for every checked field. Journal normalization is retained in revision extension data, so its result is measured here but not assigned to Journal. |
| [`normalizationResult`](../src/workspace/store.go#L627) | function | 627-635 | `func normalizationResult(doi, field, input, output string) normalizationFieldResult` | normalizationResult classifies a field as changed, already canonical, or unavailable. |
| [`recordNormalizationMetrics`](../src/workspace/store.go#L641) | function | 641-670 | `func recordNormalizationMetrics(db *database.Database, runID int64, processedArticles int, results []normalizationFieldResult) error` | recordNormalizationMetrics stores mutually exclusive outcomes for each checked field. No per-field audit events are emitted: the immutable normalized revision already records the output, and the prior revision is retained as the corresponding input evidence. |
| [`sumNormalizationOutcomes`](../src/workspace/store.go#L673) | function | 673-679 | `func sumNormalizationOutcomes(outcomes map[string]int) int` | sumNormalizationOutcomes totals the recognized mutually exclusive normalization outcomes. |
| [`enrichWorkspaceMetadata`](../src/workspace/store.go#L684) | function | 684-717 | `func enrichWorkspaceMetadata(db *database.Database, runID int64, run *Run, articles []*article.Article) (int, []fieldChange, error)` | enrichWorkspaceMetadata applies article-level providers. Its output is persisted before identity enrichment so an ORCID provider failure cannot erase the metadata and authorship evidence already gathered for the run. |
| [`enrichWorkspaceIdentity`](../src/workspace/store.go#L722) | function | 722-733 | `func enrichWorkspaceIdentity(db *database.Database, runID int64, run *Run, articles []*article.Article) (int, []fieldChange, []uncertainORCIDSearchEvidence, error)` | enrichWorkspaceIdentity applies exact observed-ORCID profiles and records name-search evidence. The caller persists this evidence against the prior enrich_metadata snapshot, including when this function returns an error. |
| [`gatherCachedCrossref`](../src/workspace/store.go#L736) | function | 736-759 | `func gatherCachedCrossref(ctx context.Context, cache *workspaceCache, source enrich.SourceConfig, articles []*article.Article) (*enrich.GatherResult, error)` | gatherCachedCrossref gathers cached crossref from the supplied inputs. |
| [`gatherCachedOpenAlex`](../src/workspace/store.go#L762) | function | 762-803 | `func gatherCachedOpenAlex(ctx context.Context, cache *workspaceCache, source enrich.SourceConfig, articles []*article.Article) (*enrich.GatherResult, error)` | gatherCachedOpenAlex gathers cached open alex from the supplied inputs. |
| [`resolveCachedOpenAlexReferences`](../src/workspace/store.go#L806) | function | 806-845 | `func resolveCachedOpenAlexReferences(ctx context.Context, cache *workspaceCache, client *enrich.Client, source enrich.SourceConfig, refIDsByDOI map[string][]string) (map[string]enrich.EnrichedReference, error)` | resolveCachedOpenAlexReferences resolves cached open alex references from the supplied context. |
| [`uncertainORCIDCandidate`](../src/workspace/store.go#L848) | struct | 848-854 | `type uncertainORCIDCandidate struct { ORCID string ProviderDisplay string QueryURL string PayloadArtifactID int64 ProviderRank int }` | uncertainORCIDCandidate stores one unconfirmed ORCID name-search result and its provenance. |
| [`uncertainORCIDSearchEvidence`](../src/workspace/store.go#L857) | struct | 857-864 | `type uncertainORCIDSearchEvidence struct { DOI string AuthorIndex int CitationName string Candidates []uncertainORCIDCandidate Status string ErrorMessage string }` | uncertainORCIDSearchEvidence records candidate-search results or provider failure for one author occurrence. |
| [`enrichCachedORCID`](../src/workspace/store.go#L867) | function | 867-927 | `func enrichCachedORCID(ctx context.Context, cache *workspaceCache, orcidSource, openAlexSource enrich.SourceConfig, hasOpenAlex bool, articles []*article.Article) (int, []fieldChange, []uncertainORCIDSearchEvidence, error)` | enrichCachedORCID gathers confirmed ORCID records and uncertain name-search evidence through the workspace cache. |
| [`recordAuthorFieldChanges`](../src/workspace/store.go#L932) | function | 932-956 | `func recordAuthorFieldChanges(authors []*article.Author, profile *enrich.EnrichedAuthor, authorDOI map[*article.Author]string) []fieldChange` | recordAuthorFieldChanges returns fieldChange entries for each author field that was filled by the given profile. It assumes applyAuthorProfile has already been called and the author fields are now populated. |
| [`resolveCachedORCIDProfile`](../src/workspace/store.go#L959) | function | 959-987 | `func resolveCachedORCIDProfile(ctx context.Context, cache *workspaceCache, orcidSource enrich.SourceConfig, orcidClient *enrich.Client, openAlexSource enrich.SourceConfig, openAlexClient *enrich.Client, hasOpenAlex bool, orcid string) (*enrich.EnrichedAuthor, error)` | resolveCachedORCIDProfile resolves cached orcid profile from the supplied context. |
| [`resolveCachedORCIDNameCandidates`](../src/workspace/store.go#L990) | function | 990-1020 | `func resolveCachedORCIDNameCandidates(ctx context.Context, cache *workspaceCache, source enrich.SourceConfig, client *enrich.Client, name string) ([]uncertainORCIDCandidate, error)` | resolveCachedORCIDNameCandidates resolves cached orcid name candidates from the supplied context. |
| [`persistUncertainORCIDEvidence`](../src/workspace/store.go#L1023) | function | 1023-1063 | `func persistUncertainORCIDEvidence(db *database.Database, runID int64, revisionIDs map[string]int64, evidence []uncertainORCIDSearchEvidence) error` | persistUncertainORCIDEvidence persists uncertain orcid evidence through the owning repository. |
| [`normalizeCacheName`](../src/workspace/store.go#L1066) | function | 1066-1068 | `func normalizeCacheName(name string) string` | normalizeCacheName normalizes cache name. |
| [`applyAuthorProfile`](../src/workspace/store.go#L1071) | function | 1071-1096 | `func applyAuthorProfile(authors []*article.Author, profile *enrich.EnrichedAuthor, matchedORCID string) bool` | applyAuthorProfile applies author profile to the supplied state. |
| [`fieldChange`](../src/workspace/store.go#L1099) | struct | 1099-1103 | `type fieldChange struct { DOI string Field string Provider string }` | fieldChange records one enriched field change for audit purposes. |
| [`applyArticleEnrichment`](../src/workspace/store.go#L1106) | function | 1106-1156 | `func applyArticleEnrichment(articles map[string]*article.Article, result *enrich.GatherResult) (int, []fieldChange)` | applyArticleEnrichment applies article enrichment to the supplied state. |
| [`usableEnrichedAuthors`](../src/workspace/store.go#L1159) | function | 1159-1172 | `func usableEnrichedAuthors(enriched []enrich.EnrichedAuthor) []article.Author` | usableEnrichedAuthors converts enriched authors that contain a non-empty citation name. |
| [`applyConfiguredArticleEnrichment`](../src/workspace/store.go#L1175) | function | 1175-1180 | `func applyConfiguredArticleEnrichment(articles map[string]*article.Article, source enrich.SourceConfig, result *enrich.GatherResult) (int, []fieldChange)` | applyConfiguredArticleEnrichment applies configured article enrichment to the supplied state. |
| [`emitFieldEnrichedAuditEvents`](../src/workspace/store.go#L1184) | function | 1184-1212 | `func emitFieldEnrichedAuditEvents(db *database.Database, runID int64, revisionIDs map[string]int64, changes []fieldChange) error` | emitFieldEnrichedAuditEvents records one field_enriched audit event per field change. revisionIDs maps DOI -> work_revision ID. |
| [`recordFieldEnrichmentMetrics`](../src/workspace/store.go#L1218) | function | 1218-1242 | `func recordFieldEnrichmentMetrics(db *database.Database, runID int64, changes []fieldChange) error` | recordFieldEnrichmentMetrics records per-field and per-provider enrichment metrics to pipeline_run_metrics. Per-author-index fields (e.g. author_orcid_0, author_first_name_1) are aggregated into a single count per field type (author_orcid, author_first_name, etc.). |
| [`completePipelineRun`](../src/workspace/store.go#L1245) | function | 1245-1251 | `func completePipelineRun(db *database.Database, runID int64) error` | completePipelineRun completes pipeline run and records its terminal state. |
| [`recordValidationAudit`](../src/workspace/store.go#L1254) | function | 1254-1278 | `func recordValidationAudit(db *database.Database, runID int64, articles []*article.Article, reasons map[string][]string) error` | recordValidationAudit records validation audit. |

### [`src/workspace/store_integration_test.go`](../src/workspace/store_integration_test.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`TestApplyConfiguredArticleEnrichmentHonorsFillMissingOnly`](../src/workspace/store_integration_test.go#L23) | test | 23-70 | `func TestApplyConfiguredArticleEnrichmentHonorsFillMissingOnly(t *testing.T)` | TestApplyConfiguredArticleEnrichmentHonorsFillMissingOnly verifies apply configured article enrichment honors fill missing only. |
| [`TestSyncNormalizedPDFInventoryRegistersOnceAndFlushesAudit`](../src/workspace/store_integration_test.go#L73) | test | 73-134 | `func TestSyncNormalizedPDFInventoryRegistersOnceAndFlushesAudit(t *testing.T)` | TestSyncNormalizedPDFInventoryRegistersOnceAndFlushesAudit verifies sync normalized pdf inventory registers once and flushes audit. |
| [`TestNormalizeWorkspaceArticlesRecordsExplicitFieldOutcomes`](../src/workspace/store_integration_test.go#L137) | test | 137-217 | `func TestNormalizeWorkspaceArticlesRecordsExplicitFieldOutcomes(t *testing.T)` | TestNormalizeWorkspaceArticlesRecordsExplicitFieldOutcomes verifies normalize workspace articles records explicit field outcomes. |
| [`TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName`](../src/workspace/store_integration_test.go#L220) | test | 220-250 | `func TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName(t *testing.T)` | TestPersistWorkspaceStageKeepsAuthorsWhenEnrichmentHasNoCitationName verifies persist workspace stage keeps authors when enrichment has no citation name. |
| [`TestApplyArticleEnrichmentReturnsFieldChanges`](../src/workspace/store_integration_test.go#L253) | test | 253-292 | `func TestApplyArticleEnrichmentReturnsFieldChanges(t *testing.T)` | TestApplyArticleEnrichmentReturnsFieldChanges verifies apply article enrichment returns field changes. |
| [`TestApplyArticleEnrichmentReturnsNoChangesWhenNoEnrichment`](../src/workspace/store_integration_test.go#L295) | test | 295-307 | `func TestApplyArticleEnrichmentReturnsNoChangesWhenNoEnrichment(t *testing.T)` | TestApplyArticleEnrichmentReturnsNoChangesWhenNoEnrichment verifies apply article enrichment returns no changes when no enrichment. |
| [`TestApplyAuthorProfileRecordsOnlyObservedFieldChanges`](../src/workspace/store_integration_test.go#L310) | test | 310-341 | `func TestApplyAuthorProfileRecordsOnlyObservedFieldChanges(t *testing.T)` | TestApplyAuthorProfileRecordsOnlyObservedFieldChanges verifies apply author profile records only observed field changes. |
| [`TestEmitFieldEnrichedAuditEvents`](../src/workspace/store_integration_test.go#L344) | test | 344-390 | `func TestEmitFieldEnrichedAuditEvents(t *testing.T)` | TestEmitFieldEnrichedAuditEvents verifies emit field enriched audit events. |
| [`TestRecordFieldEnrichmentMetrics`](../src/workspace/store_integration_test.go#L393) | test | 393-423 | `func TestRecordFieldEnrichmentMetrics(t *testing.T)` | TestRecordFieldEnrichmentMetrics verifies record field enrichment metrics. |
| [`TestEmitFieldEnrichedSkipsUnknownDOI`](../src/workspace/store_integration_test.go#L426) | test | 426-454 | `func TestEmitFieldEnrichedSkipsUnknownDOI(t *testing.T)` | TestEmitFieldEnrichedSkipsUnknownDOI verifies emit field enriched skips unknown doi. |

### [`src/workspace/support.go`](../src/workspace/support.go)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`finishPipelineRun`](../src/workspace/support.go#L28) | function | 28-43 | `func finishPipelineRun(db *database.Database, runID int64, status, summary string)` | finishPipelineRun finishes pipeline run and records its terminal state. |
| [`StringListFlag`](../src/workspace/support.go#L46) | type | 46 | `type StringListFlag []string` | StringListFlag accumulates repeated command-line flag values in declaration order. |
| [`(*StringListFlag).String`](../src/workspace/support.go#L49) | method | 49-51 | `func (*StringListFlag).String() string` | String returns the receiver's textual representation. |
| [`(*StringListFlag).Set`](../src/workspace/support.go#L54) | method | 54-57 | `func (*StringListFlag).Set(value string) error` | Set appends one command-line value to the receiver. |
| [`emptySourceError`](../src/workspace/support.go#L62) | struct | 62-67 | `type emptySourceError struct { fileType string source string path string detail string }` | emptySourceError means parsing completed successfully but yielded no raw records. The pipeline records its known observed count before failing the attempt because an empty configured export remains an input error. |
| [`(*emptySourceError).Error`](../src/workspace/support.go#L70) | method | 70-72 | `func (*emptySourceError).Error() string` | Error returns the receiver's diagnostic message. |
| [`observedCountFromLoadError`](../src/workspace/support.go#L75) | function | 75-81 | `func observedCountFromLoadError(err error) (int, bool)` | observedCountFromLoadError extracts the known zero record count from an empty-source failure. |
| [`StartWorkspaceAttempt`](../src/workspace/support.go#L84) | function | 84-236 | `func StartWorkspaceAttempt(db *database.Database, originalConfig []byte, run *Run, fresh bool) (int64, error)` | StartWorkspaceAttempt snapshots configuration and inputs, reuses or creates a plan, and starts an eligible pipeline attempt. |
| [`recordPreflightStep`](../src/workspace/support.go#L239) | function | 239-286 | `func recordPreflightStep(db *database.Database, runID, configArtifactID, inputManifestArtifactID int64, inputFingerprint, outputFingerprint string, reusedFromRunID *int64, failureSummary string) error` | recordPreflightStep records preflight step. |
| [`buildInputManifest`](../src/workspace/support.go#L289) | function | 289-309 | `func buildInputManifest(resolved *manifest.ResolvedManifest) (*manifest.InputManifest, error)` | buildInputManifest hashes configured source files and returns both captured evidence and aggregate read failure. |
| [`persistArtifact`](../src/workspace/support.go#L312) | function | 312-322 | `func persistArtifact(db *database.Database, runID int64, data []byte, contentType string) (int64, error)` | persistArtifact stores content-addressed artifact metadata and bytes for a run. |
| [`contentHash`](../src/workspace/support.go#L325) | function | 325-328 | `func contentHash(data []byte) string` | contentHash returns the lowercase hexadecimal SHA-256 digest of data. |
| [`BoolMetric`](../src/workspace/support.go#L331) | function | 331-336 | `func BoolMetric(value bool) int` | BoolMetric converts a Boolean value to the metric convention of one or zero. |
| [`freshReason`](../src/workspace/support.go#L339) | function | 339-347 | `func freshReason(explicitFresh bool, reusePolicy string) string` | freshReason returns the persisted reason for starting a non-reused attempt. |
| [`loadCSVEntries`](../src/workspace/support.go#L350) | function | 350-385 | `func loadCSVEntries(path, source string) ([]map[string]string, error)` | loadCSVEntries loads csv entries from the supplied source. |
| [`loadBibEntries`](../src/workspace/support.go#L388) | function | 388-410 | `func loadBibEntries(path, source string) ([]map[string]string, error)` | loadBibEntries loads bib entries from the supplied source. |

## JavaScript declarations

### [`frontend/scripts/run-playwright.mjs`](../frontend/scripts/run-playwright.mjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`mustExist`](../frontend/scripts/run-playwright.mjs#L50) | function | 50 | `async function mustExist(target, name, hint)` | Asynchronously implements must exist for the viewer. |
| [`startServer`](../frontend/scripts/run-playwright.mjs#L59) | function | 59 | `function startServer()` | Starts the fixture-backed viewer on an operating-system-assigned loopback port. |
| [`fail`](../frontend/scripts/run-playwright.mjs#L89) | function | 89 | `function fail(error)` | Stops startup and rejects with the server process failure. |
| [`waitForHealth`](../frontend/scripts/run-playwright.mjs#L99) | function | 99 | `async function waitForHealth(baseURL)` | Asynchronously implements wait for health for the viewer. |
| [`stopServer`](../frontend/scripts/run-playwright.mjs#L116) | function | 116 | `async function stopServer()` | Asynchronously implements stop server for the viewer. |
| [`exitCode`](../frontend/scripts/run-playwright.mjs#L128) | function | 128 | `function exitCode(child, name)` | Normalizes a child-process exit result. |
| [`delay`](../frontend/scripts/run-playwright.mjs#L136) | function | 136 | `function delay(milliseconds)` | Returns a promise that resolves after the requested interval. |
| [`npmCommand`](../frontend/scripts/run-playwright.mjs#L141) | function | 141 | `function npmCommand()` | Returns the platform-appropriate npm command. |

### [`frontend/tests/e2e.spec.cjs`](../frontend/tests/e2e.spec.cjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`generatedContext`](../frontend/tests/e2e.spec.cjs#L7) | function | 7 | `async function generatedContext(request)` | Resolves the generated database's persistent URL-state identifiers through public APIs. |
| [`generatedURL`](../frontend/tests/e2e.spec.cjs#L37) | function | 37 | `function generatedURL(context, updates)` | Builds a context-preserving application URL for one generated viewer route. |

### [`frontend/tests/ui-quality.spec.cjs`](../frontend/tests/ui-quality.spec.cjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`url`](../frontend/tests/ui-quality.spec.cjs#L13) | function | 13 | `function url(overrides = {})` | Builds a fixture viewer URL. |
| [`visit`](../frontend/tests/ui-quality.spec.cjs#L18) | function | 18 | `async function visit(page, overrides = {})` | Navigates to a UI-quality fixture state. |
| [`expectNoPageOverflow`](../frontend/tests/ui-quality.spec.cjs#L24) | function | 24 | `async function expectNoPageOverflow(page)` | Asynchronously implements expect no page overflow for the viewer. |

### [`frontend/tests/unit/views/evaluation.test.js`](../frontend/tests/unit/views/evaluation.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`setLocation`](../frontend/tests/unit/views/evaluation.test.js#L9) | function | 9 | `function setLocation(values)` | Sets location. |
| [`response`](../frontend/tests/unit/views/evaluation.test.js#L21) | function | 21 | `function response(data)` | Builds a mock fetch response. |

### [`frontend/tests/unit/views/provenance.test.js`](../frontend/tests/unit/views/provenance.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`setLocation`](../frontend/tests/unit/views/provenance.test.js#L9) | function | 9 | `function setLocation(values)` | Sets location. |
| [`response`](../frontend/tests/unit/views/provenance.test.js#L17) | function | 17 | `function response(data)` | Builds a mock fetch response. |

### [`frontend/tests/viewer.spec.cjs`](../frontend/tests/viewer.spec.cjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`goto`](../frontend/tests/viewer.spec.cjs#L46) | function | 46 | `async function goto(page, url)` | Navigate to a URL and wait for network idle. |
| [`contextURL`](../frontend/tests/viewer.spec.cjs#L54) | function | 54 | `function contextURL(overrides = {})` | Build a context URL with search, revision, plan, and run IDs. |
| [`selectRun`](../frontend/tests/viewer.spec.cjs#L68) | function | 68 | `async function selectRun(page, searchId, revisionId, planId, runId)` | Navigate to a fully selected context URL. |

### [`src/server/frontend/api.js`](../src/server/frontend/api.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`endpoint`](../src/server/frontend/api.js#L5) | function | 5 | `function endpoint(path, query)` | Builds an API path and query string from supplied values. |
| [`api`](../src/server/frontend/api.js#L19) | function | 19 | `async function api(path, query)` | Fetches and decodes one JSON API response. |
| [`tables`](../src/server/frontend/api.js#L46) | function | 46 | `async function tables()` | Loads and caches the discovered database table list. |

### [`src/server/frontend/components/audit-events.js`](../src/server/frontend/components/audit-events.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`auditCategory`](../src/server/frontend/components/audit-events.js#L7) | function | 7 | `function auditCategory(event)` | Classifies an audit event into its presentation category. |
| [`eventMetadata`](../src/server/frontend/components/audit-events.js#L22) | function | 22 | `function eventMetadata(event)` | Parses an audit event's stored metadata object. |
| [`auditOutcome`](../src/server/frontend/components/audit-events.js#L27) | function | 27 | `function auditOutcome(event, metadata)` | Derives the display outcome from recorded metadata and action semantics. |
| [`auditEntity`](../src/server/frontend/components/audit-events.js#L46) | function | 46 | `function auditEntity(event)` | Returns a context-preserving link or label for the affected audit entity. |
| [`eventSummary`](../src/server/frontend/components/audit-events.js#L67) | function | 67 | `function eventSummary(event, metadata)` | Returns a concise human-readable summary of an audit event. |
| [`additionalMetadata`](../src/server/frontend/components/audit-events.js#L88) | function | 88 | `function additionalMetadata(metadata)` | Returns metadata fields not already represented in the primary event presentation. |
| [`eventDetails`](../src/server/frontend/components/audit-events.js#L100) | function | 100 | `function eventDetails(event, metadata)` | Returns expandable facts and JSON payloads for an audit event. |
| [`auditEventMarkup`](../src/server/frontend/components/audit-events.js#L129) | function | 129 | `function auditEventMarkup(event)` | Returns the complete escaped markup for one audit event. |
| [`auditStream`](../src/server/frontend/components/audit-events.js#L152) | function | 152 | `function auditStream(events, emptyMessage)` | Groups audit events by local date and returns timeline markup. |
| [`recordAuditInvestigation`](../src/server/frontend/components/audit-events.js#L174) | function | 174 | `function recordAuditInvestigation(events)` | Records audit investigation. |
| [`bindRecordAuditInvestigation`](../src/server/frontend/components/audit-events.js#L192) | function | 192 | `function bindRecordAuditInvestigation(events)` | Binds DOM behavior for record audit investigation. |
| [`apply`](../src/server/frontend/components/audit-events.js#L203) | function | 203 | `function apply()` | Applies the associated state. |
| [`resetAndApply`](../src/server/frontend/components/audit-events.js#L224) | function | 224 | `function resetAndApply()` | Resets and apply. |

### [`src/server/frontend/components/context-selector.js`](../src/server/frontend/components/context-selector.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`showSkeleton`](../src/server/frontend/components/context-selector.js#L28) | function | 28 | `function showSkeleton(key)` | Show a loading skeleton placeholder in a dropdown's field area. @param {string} key - The select key ('search', 'revision', 'plan', 'run') |
| [`hideSkeleton`](../src/server/frontend/components/context-selector.js#L52) | function | 52 | `function hideSkeleton(key)` | Remove a loading skeleton from a dropdown's field area. @param {string} key - The select key |
| [`showDropdownError`](../src/server/frontend/components/context-selector.js#L66) | function | 66 | `function showDropdownError(key, message)` | Show an inline error message below a dropdown. @param {string} key - The select key @param {string} message - Error message text |
| [`hideDropdownError`](../src/server/frontend/components/context-selector.js#L84) | function | 84 | `function hideDropdownError(key)` | Remove an error message from a dropdown's field area. @param {string} key - The select key |
| [`addClearButton`](../src/server/frontend/components/context-selector.js#L98) | function | 98 | `function addClearButton(key)` | Add a clear button (×) to a dropdown field. Clicking it clears the select value and resets dependent URL params. @param {string} key - The select key |
| [`removeClearButton`](../src/server/frontend/components/context-selector.js#L129) | function | 129 | `function removeClearButton(key)` | Remove a clear button from a dropdown field. @param {string} key - The select key |
| [`autoSelectSingle`](../src/server/frontend/components/context-selector.js#L145) | function | 145 | `function autoSelectSingle(key)` | Auto-select a single option if the dropdown has exactly one. Updates the URL parameter directly without triggering a render cycle. Returns true if auto-selected. @param {string} key - The select key @returns {boolean} |
| [`initDropdownEnhancements`](../src/server/frontend/components/context-selector.js#L173) | function | 173 | `function initDropdownEnhancements()` | Initialize per-dropdown enhancements. Adds clear buttons to each dropdown. |
| [`selectOptions`](../src/server/frontend/components/context-selector.js#L190) | function | 190 | `function selectOptions(select, items, selected, label, labelFn)` | Selects options. |
| [`hydrateSelectors`](../src/server/frontend/components/context-selector.js#L213) | function | 213 | `async function hydrateSelectors()` | Asynchronously implements hydrate selectors for the viewer. |

### [`src/server/frontend/components/data-table.js`](../src/server/frontend/components/data-table.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`rowFilter`](../src/server/frontend/components/data-table.js#L7) | function | 7 | `function rowFilter(rows, query)` | Returns whether a row contains the case-insensitive filter text. |
| [`scrollTableIntoView`](../src/server/frontend/components/data-table.js#L20) | function | 20 | `function scrollTableIntoView()` | Moves focus and scroll position to the table region when available. |
| [`dataTable`](../src/server/frontend/components/data-table.js#L28) | function | 28 | `function dataTable(tableName, result, context)` | Renders and binds a filterable, sortable, paginated in-memory data table. |
| [`bindTableControls`](../src/server/frontend/components/data-table.js#L186) | function | 186 | `function bindTableControls(tableName, page)` | Binds DOM behavior for table controls. |
| [`updates`](../src/server/frontend/components/data-table.js#L197) | function | 197 | `function updates(values)` | Updates s. |
| [`handleExpandToggle`](../src/server/frontend/components/data-table.js#L269) | function | 269 | `function handleExpandToggle(event)` | Handles expand toggle. |

### [`src/server/frontend/components/graph.js`](../src/server/frontend/components/graph.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`graphField`](../src/server/frontend/components/graph.js#L9) | function | 9 | `function graphField(name, label, type)` | Returns an escaped graph-filter input with its current URL value. |
| [`graphQuery`](../src/server/frontend/components/graph.js#L19) | function | 19 | `function graphQuery()` | Returns the current graph-filter values keyed by query parameter. |
| [`graphLink`](../src/server/frontend/components/graph.js#L28) | function | 28 | `function graphLink(node)` | Returns a context-preserving detail link for a graph node when one exists. |
| [`endpointID`](../src/server/frontend/components/graph.js#L42) | function | 42 | `function endpointID(endpoint)` | Returns an edge endpoint identifier from either an identifier or resolved node object. |
| [`graphClusters`](../src/server/frontend/components/graph.js#L50) | function | 50 | `function graphClusters(sourceNodes, sourceEdges)` | Finds deterministic connected components and maps graph nodes to cluster identifiers. |
| [`graphResult`](../src/server/frontend/components/graph.js#L88) | function | 88 | `function graphResult(data)` | Returns bounded graph controls, legend, canvas, selection, and relationship-table markup. |
| [`nodeSize`](../src/server/frontend/components/graph.js#L167) | function | 167 | `function nodeSize(node, degree, maxDegree)` | Calculates a node radius from entity type and visible degree. |
| [`hash`](../src/server/frontend/components/graph.js#L184) | function | 184 | `function hash(value)` | Returns a deterministic unsigned hash for stable graph placement. |
| [`palette`](../src/server/frontend/components/graph.js#L193) | function | 193 | `function palette()` | Reads graph colors from active CSS custom properties with safe fallbacks. |
| [`get`](../src/server/frontend/components/graph.js#L196) | function | 196 | `function get(name, fallback)` | Returns the associated state. |
| [`drawDiamond`](../src/server/frontend/components/graph.js#L217) | function | 217 | `function drawDiamond(context, x, y, radius)` | Draws diamond. |
| [`drawTriangle`](../src/server/frontend/components/graph.js#L227) | function | 227 | `function drawTriangle(context, x, y, radius)` | Draws triangle. |
| [`relationshipLabel`](../src/server/frontend/components/graph.js#L236) | function | 236 | `function relationshipLabel(edge)` | Returns the user-facing label for a graph edge type and its relevant metadata. |
| [`destroyGraph`](../src/server/frontend/components/graph.js#L263) | function | 263 | `function destroyGraph()` | Destroys graph. |
| [`mountGraph`](../src/server/frontend/components/graph.js#L280) | function | 280 | `function mountGraph(data)` | Mounts graph. |
| [`resize`](../src/server/frontend/components/graph.js#L405) | function | 405 | `function resize()` | Resizes the backing canvas for its layout size and device pixel ratio. |
| [`graphBounds`](../src/server/frontend/components/graph.js#L425) | function | 425 | `function graphBounds(nodes)` | Returns the radius-aware world-coordinate bounds of graph nodes. |
| [`clusterOverview`](../src/server/frontend/components/graph.js#L440) | function | 440 | `function clusterOverview(nodes)` | Returns connected-cluster sizes ordered from largest to smallest. |
| [`drawClusterOverview`](../src/server/frontend/components/graph.js#L453) | function | 453 | `function drawClusterOverview(context, clusters, colors, width, height, offset, legendInset)` | Draws cluster overview. |
| [`fitGraph`](../src/server/frontend/components/graph.js#L494) | function | 494 | `function fitGraph(graph)` | Adjusts the graph transform to fit all node bounds in the canvas. |
| [`runLayout`](../src/server/frontend/components/graph.js#L511) | function | 511 | `function runLayout(graph, status)` | Advances the force simulation in animation-frame batches and finalizes spatial state. |
| [`next`](../src/server/frontend/components/graph.js#L534) | function | 534 | `function next()` | Advances and redraws the next batch of force-layout ticks. |
| [`draw`](../src/server/frontend/components/graph.js#L564) | function | 564 | `function draw(graph)` | Draws the associated state. |
| [`drawArrow`](../src/server/frontend/components/graph.js#L751) | function | 751 | `function drawArrow(context, source, target, radius, color)` | Draws arrow. |
| [`graphCoordinates`](../src/server/frontend/components/graph.js#L765) | function | 765 | `function graphCoordinates(graph, event)` | Converts a pointer event from canvas coordinates to graph world coordinates. |
| [`zoomViewAt`](../src/server/frontend/components/graph.js#L774) | function | 774 | `function zoomViewAt(view, screenPoint, nextScale)` | Returns a zoom transform that keeps the selected screen point stationary. |
| [`nearestOverviewCluster`](../src/server/frontend/components/graph.js#L785) | function | 785 | `function nearestOverviewCluster(graph, event)` | Returns the overview cluster hit by a pointer event, when any. |
| [`focusCluster`](../src/server/frontend/components/graph.js#L795) | function | 795 | `function focusCluster(graph, clusterID)` | Focuses cluster. |
| [`buildSpatialIndex`](../src/server/frontend/components/graph.js#L811) | function | 811 | `function buildSpatialIndex(nodes)` | Builds spatial index. |
| [`nearbyNodes`](../src/server/frontend/components/graph.js#L825) | function | 825 | `function nearbyNodes(index, point)` | Returns nodes in the spatial-index cell surrounding a graph point. |
| [`nearestNode`](../src/server/frontend/components/graph.js#L841) | function | 841 | `function nearestNode(graph, point)` | Returns the closest selectable node within its hit radius. |
| [`bindInteractions`](../src/server/frontend/components/graph.js#L865) | function | 865 | `function bindInteractions(graph, status, selectionPanel, zoomIndicator)` | Binds DOM behavior for interactions. |
| [`setSelection`](../src/server/frontend/components/graph.js#L870) | function | 870 | `function setSelection(id)` | Sets selection. |
| [`updateZoomDisplay`](../src/server/frontend/components/graph.js#L896) | function | 896 | `function updateZoomDisplay()` | Updates zoom display. |
| [`bindGraphSearch`](../src/server/frontend/components/graph.js#L1063) | function | 1063 | `function bindGraphSearch(graph)` | Bind graph node search — highlights matching nodes by name/DOI. |
| [`bindGraphExport`](../src/server/frontend/components/graph.js#L1076) | function | 1076 | `function bindGraphExport(graph, data)` | Bind graph export as PNG — downloads the canvas as a PNG image. |
| [`bindGraphExpand`](../src/server/frontend/components/graph.js#L1097) | function | 1097 | `function bindGraphExpand(graph)` | Binds DOM behavior for graph expand. |
| [`updateLabel`](../src/server/frontend/components/graph.js#L1104) | function | 1104 | `function updateLabel()` | Updates label. |
| [`selectionMarkup`](../src/server/frontend/components/graph.js#L1136) | function | 1136 | `function selectionMarkup(node, neighbours)` | Selects ion markup. |
| [`nodeMarkup`](../src/server/frontend/components/graph.js#L1171) | function | 1171 | `function nodeMarkup(node)` | Returns escaped linked or plain label markup for a graph node. |
| [`renderEdgePage`](../src/server/frontend/components/graph.js#L1181) | function | 1181 | `function renderEdgePage(graph)` | Renders edge page. |
| [`edgeDetails`](../src/server/frontend/components/graph.js#L1234) | function | 1234 | `function edgeDetails(edge)` | Returns relationship-specific details for a graph edge row. |

### [`src/server/frontend/components/pagination.js`](../src/server/frontend/components/pagination.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`paginationPages`](../src/server/frontend/components/pagination.js#L5) | function | 5 | `function paginationPages(currentPage, totalPages, visibleCount)` | Returns the bounded sequence of page numbers surrounding the current page. |
| [`pagination`](../src/server/frontend/components/pagination.js#L22) | function | 22 | `function pagination(result, options)` | Returns accessible pagination markup for server-backed or in-memory results. |
| [`control`](../src/server/frontend/components/pagination.js#L46) | function | 46 | `function control(label, target, disabled, relation)` | Returns one pagination navigation control. |

### [`src/server/frontend/components/shell.js`](../src/server/frontend/components/shell.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`initHealthCheck`](../src/server/frontend/components/shell.js#L7) | function | 7 | `function initHealthCheck()` | Initializes health check. |
| [`initMobileNavToggle`](../src/server/frontend/components/shell.js#L31) | function | 31 | `function initMobileNavToggle()` | Initialize mobile nav toggle. Shows/hides the primary navigation on small screens. |
| [`handleToggle`](../src/server/frontend/components/shell.js#L39) | function | 39 | `function handleToggle()` | Handles toggle. |

### [`src/server/frontend/router.js`](../src/server/frontend/router.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`setURL`](../src/server/frontend/router.js#L15) | function | 15 | `function setURL(updates, replace)` | Replaces the current URL state without triggering a navigation reload. |
| [`bindFocusContext`](../src/server/frontend/router.js#L29) | function | 29 | `function bindFocusContext()` | Binds DOM behavior for focus context. |
| [`syncPrimaryNavigation`](../src/server/frontend/router.js#L39) | function | 39 | `function syncPrimaryNavigation(current)` | Synchronizes primary navigation. |
| [`renderView`](../src/server/frontend/router.js#L60) | function | 60 | `async function renderView()` | Asynchronously renders view. |
| [`render`](../src/server/frontend/router.js#L88) | function | 88 | `async function render()` | Asynchronously renders the associated state. |

### [`src/server/frontend/state.js`](../src/server/frontend/state.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`params`](../src/server/frontend/state.js#L56) | function | 56 | `function params()` | Returns the current URL search parameters. |
| [`value`](../src/server/frontend/state.js#L61) | function | 61 | `function value(name)` | Returns a named URL parameter or an empty string. |
| [`view`](../src/server/frontend/state.js#L66) | function | 66 | `function view()` | Returns the selected viewer view. |
| [`section`](../src/server/frontend/state.js#L71) | function | 71 | `function section(name, fallback)` | Returns a named section parameter or its fallback. |
| [`esc`](../src/server/frontend/state.js#L76) | function | 76 | `function esc(raw)` | Escapes a value for safe HTML text insertion. |
| [`asJSON`](../src/server/frontend/state.js#L83) | function | 83 | `function asJSON(item)` | Formats a value for JSON-oriented display. |
| [`list`](../src/server/frontend/state.js#L91) | function | 91 | `function list(data, keys)` | Returns the first matching array in an API response. |
| [`pickID`](../src/server/frontend/state.js#L106) | function | 106 | `function pickID(item)` | Returns the first supported identifier present on an item. |
| [`text`](../src/server/frontend/state.js#L123) | function | 123 | `function text(item, fields)` | Returns the first non-empty display field on an item. |
| [`number`](../src/server/frontend/state.js#L133) | function | 133 | `function number(raw)` | Converts a value to a finite number or zero. |
| [`formatNumber`](../src/server/frontend/state.js#L142) | function | 142 | `function formatNumber(raw)` | Formats number. |
| [`percent`](../src/server/frontend/state.js#L147) | function | 147 | `function percent(raw, denominator)` | Formats a count as a percentage of its denominator. |
| [`formatTime`](../src/server/frontend/state.js#L157) | function | 157 | `function formatTime(raw)` | Formats time. |
| [`formatBytes`](../src/server/frontend/state.js#L169) | function | 169 | `function formatBytes(raw)` | Formats bytes. |
| [`humanLabel`](../src/server/frontend/state.js#L185) | function | 185 | `function humanLabel(raw)` | Converts a machine-oriented identifier to a title-cased display label. |
| [`parseObject`](../src/server/frontend/state.js#L192) | function | 192 | `function parseObject(raw)` | Parses object. |
| [`statusClass`](../src/server/frontend/state.js#L211) | function | 211 | `function statusClass(raw)` | Maps a recorded status to its semantic color class. |
| [`statusChip`](../src/server/frontend/state.js#L232) | function | 232 | `function statusChip(raw)` | Returns escaped label markup for a recorded status. |
| [`metricEntries`](../src/server/frontend/state.js#L239) | function | 239 | `function metricEntries(group)` | Normalizes array- or object-backed metrics to display-name and value pairs. |
| [`selectedRun`](../src/server/frontend/state.js#L250) | function | 250 | `function selectedRun()` | Returns the pipeline run selected by the current URL context. |
| [`showError`](../src/server/frontend/state.js#L258) | function | 258 | `function showError(error)` | Shows error. |
| [`clearError`](../src/server/frontend/state.js#L264) | function | 264 | `function clearError()` | Clears error. |
| [`busy`](../src/server/frontend/state.js#L270) | function | 270 | `function busy(isBusy)` | Shows or hides the global loading indicator. |
| [`link`](../src/server/frontend/state.js#L275) | function | 275 | `function link(updates)` | Builds an internal URL that preserves current research context and applies supplied updates. |
| [`pageHeader`](../src/server/frontend/state.js#L294) | function | 294 | `function pageHeader(kicker, title, description, extra)` | Returns the standard page header with escaped copy and optional actions. |
| [`breadcrumb`](../src/server/frontend/state.js#L304) | function | 304 | `function breadcrumb(options)` | Returns research-context breadcrumb markup for the current or supplied parent record. |
| [`emptyState`](../src/server/frontend/state.js#L359) | function | 359 | `function emptyState(title, detail, action)` | Returns a complete empty-view state with the standard page header. |
| [`emptyPanel`](../src/server/frontend/state.js#L367) | function | 367 | `function emptyPanel(title, detail, action)` | Returns compact empty-state panel markup. |
| [`panel`](../src/server/frontend/state.js#L375) | function | 375 | `function panel(title, description, body, classes)` | Returns the standard titled content-panel markup. |
| [`table`](../src/server/frontend/state.js#L387) | function | 387 | `function table(title, description, columns, rows, classes)` | Returns an escaped data table inside the standard panel wrapper. |
| [`subnav`](../src/server/frontend/state.js#L417) | function | 417 | `function subnav(items, current, key)` | Returns context-preserving tab navigation for a keyed section. |
| [`filterChips`](../src/server/frontend/state.js#L428) | function | 428 | `function filterChips(filters, labels, options)` | Filters chips. |
| [`metricCard`](../src/server/frontend/state.js#L452) | function | 452 | `function metricCard(name, metric, href)` | Returns a metric card with availability, denominator, and optional navigation. |
| [`flowStage`](../src/server/frontend/state.js#L485) | function | 485 | `function flowStage(label, raw, base, previous, extraClass, stageKey, options)` | Returns one retention-flow stage with counts, percentages, and optional links. |
| [`sourceFilterStageSummary`](../src/server/frontend/state.js#L578) | function | 578 | `function sourceFilterStageSummary(items)` | Combines cumulative source filter counts into ordered cross-source stages. |
| [`retentionPhase`](../src/server/frontend/state.js#L625) | function | 625 | `function retentionPhase(title, description, summary, content, className)` | Returns one titled phase in the retention-flow presentation. |
| [`flowGroup`](../src/server/frontend/state.js#L633) | function | 633 | `function flowGroup(title, cards, className)` | Returns a labeled group of retention-flow cards. |
| [`retentionFlow`](../src/server/frontend/state.js#L640) | function | 640 | `function retentionFlow(overview)` | Returns the source-selection and pipeline-retention flow for an overview payload. |
| [`breakdown`](../src/server/frontend/state.js#L727) | function | 727 | `function breakdown(title, source, valueLabel, useTotal)` | Returns a metric breakdown table with relative bars and optional total percentages. |
| [`valueRender`](../src/server/frontend/state.js#L750) | function | 750 | `function valueRender(row)` | Formats one breakdown value with availability and optional percentage. |
| [`barRender`](../src/server/frontend/state.js#L767) | function | 767 | `function barRender(row)` | Returns an accessible relative-volume bar for one breakdown row. |
| [`sourceResultCountSummary`](../src/server/frontend/state.js#L788) | function | 788 | `function sourceResultCountSummary(items, classes)` | Returns the expected-versus-observed source export count table. |
| [`count`](../src/server/frontend/state.js#L794) | function | 794 | `function count(raw)` | Formats a source count or its unavailable state. |
| [`comparison`](../src/server/frontend/state.js#L802) | function | 802 | `function comparison(raw)` | Returns a status chip for a source-count comparison. |
| [`date`](../src/server/frontend/state.js#L810) | function | 810 | `function date(raw)` | Escapes an export date or returns its unavailable state. |
| [`sourceSearchQueries`](../src/server/frontend/state.js#L839) | function | 839 | `function sourceSearchQueries(items, classes)` | Returns expandable exact-query markup for source exports. |
| [`timeline`](../src/server/frontend/state.js#L863) | function | 863 | `function timeline(rows)` | Returns chronological audit feed markup for generic event rows. |
| [`detailTable`](../src/server/frontend/state.js#L937) | function | 937 | `function detailTable(title, rows)` | Builds a table whose columns are derived from the supplied detail records. |
| [`cell`](../src/server/frontend/state.js#L954) | function | 954 | `function cell(item, column, tableName, options)` | Formats and links a table cell according to its column and table context. |
| [`bindCopyButtons`](../src/server/frontend/state.js#L998) | function | 998 | `function bindCopyButtons()` | Bind copy-to-clipboard behavior for [data-copy-text] buttons. Shows "Copied!" feedback for 2 seconds, falls back to prompt(). |
| [`bindDismissibleMessages`](../src/server/frontend/state.js#L1017) | function | 1017 | `function bindDismissibleMessages()` | Bind dismissible behavior for .ui.message elements with a .close child. Clicking the close button fades out and removes the message. |
| [`bindLoadingButtons`](../src/server/frontend/state.js#L1033) | function | 1033 | `function bindLoadingButtons()` | Bind loading state for buttons with [data-loading]. On click, the button shows a spinner and disables itself. |

### [`src/server/frontend/views/advanced.js`](../src/server/frontend/views/advanced.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`advancedView`](../src/server/frontend/views/advanced.js#L8) | function | 8 | `async function advancedView()` | Asynchronously implements advanced view for the viewer. |

### [`src/server/frontend/views/corpus.js`](../src/server/frontend/views/corpus.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`columnNames`](../src/server/frontend/views/corpus.js#L74) | function | 74 | `function columnNames(table)` | Returns the ordered union of column names present in result rows. |
| [`identityEvidenceTable`](../src/server/frontend/views/corpus.js#L87) | function | 87 | `function identityEvidenceTable(data, context)` | Returns the column definition used for identity evidence rows. |
| [`candidateDetails`](../src/server/frontend/views/corpus.js#L107) | function | 107 | `function candidateDetails(row)` | Returns expandable candidate details for an identity evidence row. |
| [`clippedRecordLink`](../src/server/frontend/views/corpus.js#L180) | function | 180 | `function clippedRecordLink(kind, idKey, id, title)` | Returns a context-preserving record link with a clipped label. |
| [`clippedRecordText`](../src/server/frontend/views/corpus.js#L188) | function | 188 | `function clippedRecordText(title)` | Returns escaped record text clipped to the requested length. |
| [`corpusColumnConfig`](../src/server/frontend/views/corpus.js#L194) | function | 194 | `function corpusColumnConfig(current)` | Returns section-specific labels and renderers for corpus columns. |
| [`corpusView`](../src/server/frontend/views/corpus.js#L249) | function | 249 | `async function corpusView()` | Asynchronously implements corpus view for the viewer. |

### [`src/server/frontend/views/detail.js`](../src/server/frontend/views/detail.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`detailLink`](../src/server/frontend/views/detail.js#L13) | function | 13 | `function detailLink(kind, id)` | Returns a context-preserving link to a related detail record. |
| [`backToCorpus`](../src/server/frontend/views/detail.js#L26) | function | 26 | `function backToCorpus(kind)` | Returns the context-preserving corpus return URL for a detail view. |
| [`detailReturn`](../src/server/frontend/views/detail.js#L39) | function | 39 | `function detailReturn(kind)` | Returns detail-view navigation markup back to the originating corpus section. |
| [`recorded`](../src/server/frontend/views/detail.js#L54) | function | 54 | `function recorded(raw, fallback)` | Records ed. |
| [`propertyGrid`](../src/server/frontend/views/detail.js#L62) | function | 62 | `function propertyGrid(entries, classes)` | Returns definition-list markup for labeled record properties. |
| [`summaryStrip`](../src/server/frontend/views/detail.js#L71) | function | 71 | `function summaryStrip(entries)` | Returns compact summary-fact markup for a detail record. |
| [`mappingValue`](../src/server/frontend/views/detail.js#L79) | function | 79 | `function mappingValue(raw)` | Converts a stored mapping representation to a displayable object. |
| [`extensionMapping`](../src/server/frontend/views/detail.js#L97) | function | 97 | `function extensionMapping(raw)` | Returns the parsed extension mapping stored on a work revision. |
| [`keywordValues`](../src/server/frontend/views/detail.js#L106) | function | 106 | `function keywordValues(raw)` | Returns normalized keyword values from stored array or delimited input. |
| [`keywordMarkup`](../src/server/frontend/views/detail.js#L129) | function | 129 | `function keywordMarkup(raw)` | Returns label markup for normalized keyword values. |
| [`rawRecord`](../src/server/frontend/views/detail.js#L142) | function | 142 | `function rawRecord(record, excluded)` | Returns expandable JSON markup for a raw record. |
| [`collectionMarkup`](../src/server/frontend/views/detail.js#L161) | function | 161 | `function collectionMarkup(key, title, description, columns, rows, page)` | Returns expandable markup for a related-record collection. |
| [`mountCollection`](../src/server/frontend/views/detail.js#L192) | function | 192 | `function mountCollection(key, title, description, columns, rows)` | Mounts collection. |
| [`renderCollection`](../src/server/frontend/views/detail.js#L198) | function | 198 | `function renderCollection(key)` | Renders collection. |
| [`stageReasonMarkup`](../src/server/frontend/views/detail.js#L216) | function | 216 | `function stageReasonMarkup(raw)` | Returns escaped validation or failure reason markup for a stage outcome. |
| [`articleView`](../src/server/frontend/views/detail.js#L233) | function | 233 | `function articleView(record, data)` | Returns the article detail view from its immutable revision payload. |
| [`pdfStatusPanel`](../src/server/frontend/views/detail.js#L298) | function | 298 | `function pdfStatusPanel(record, pdf)` | Returns PDF inventory and download-status markup for an article. |
| [`authorView`](../src/server/frontend/views/detail.js#L319) | function | 319 | `function authorView(record, data)` | Returns the author occurrence detail view with related articles and audit evidence. |
| [`referenceView`](../src/server/frontend/views/detail.js#L345) | function | 345 | `function referenceView(record)` | Returns the reference mention detail view with citation context. |
| [`detailView`](../src/server/frontend/views/detail.js#L376) | function | 376 | `async function detailView(kind)` | Asynchronously implements detail view for the viewer. |

### [`src/server/frontend/views/evaluation.js`](../src/server/frontend/views/evaluation.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`titleLink`](../src/server/frontend/views/evaluation.js#L13) | function | 13 | `function titleLink(row)` | Returns a context-preserving article link for an evaluation row. |
| [`inventoriedTime`](../src/server/frontend/views/evaluation.js#L20) | function | 20 | `function inventoriedTime(row)` | Returns the recorded PDF inventory time or an unavailable label. |
| [`evaluationView`](../src/server/frontend/views/evaluation.js#L28) | function | 28 | `async function evaluationView()` | Asynchronously implements evaluation view for the viewer. |

### [`src/server/frontend/views/overview.js`](../src/server/frontend/views/overview.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`normalizationValue`](../src/server/frontend/views/overview.js#L7) | function | 7 | `function normalizationValue(metric)` | Returns a normalization metric value or its unavailable presentation. |
| [`capturedMetricValue`](../src/server/frontend/views/overview.js#L30) | function | 30 | `function capturedMetricValue(item)` | Returns the numeric value of a captured metric, or null when unavailable. |
| [`capturedMetricsByStage`](../src/server/frontend/views/overview.js#L38) | function | 38 | `function capturedMetricsByStage(metrics)` | Groups captured metrics by pipeline stage. |
| [`capturedMetricsMarkup`](../src/server/frontend/views/overview.js#L55) | function | 55 | `function capturedMetricsMarkup(metrics)` | Returns table markup for captured pipeline metrics. |
| [`fixedPercentageMetric`](../src/server/frontend/views/overview.js#L68) | function | 68 | `function fixedPercentageMetric(metric)` | Returns a metric copy with a percentage derived from its value and denominator. |
| [`overviewView`](../src/server/frontend/views/overview.js#L77) | function | 77 | `async function overviewView()` | Asynchronously implements overview view for the viewer. |

### [`src/server/frontend/views/provenance.js`](../src/server/frontend/views/provenance.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`selectedValues`](../src/server/frontend/views/provenance.js#L29) | function | 29 | `function selectedValues(raw)` | Returns the selected comma-separated values for an audit facet. |
| [`auditMultiSelect`](../src/server/frontend/views/provenance.js#L34) | function | 34 | `function auditMultiSelect(name, label, options, selectedRaw)` | Returns a multi-select control for one audit facet. |
| [`auditQuery`](../src/server/frontend/views/provenance.js#L48) | function | 48 | `function auditQuery(cursor)` | Builds API query parameters from the active audit filters. |
| [`auditFilterSummary`](../src/server/frontend/views/provenance.js#L64) | function | 64 | `function auditFilterSummary()` | Returns markup summarizing active audit filters and their removal links. |
| [`auditFilters`](../src/server/frontend/views/provenance.js#L77) | function | 77 | `function auditFilters(facets)` | Returns the complete audit filter form. |
| [`auditSummary`](../src/server/frontend/views/provenance.js#L105) | function | 105 | `function auditSummary(data)` | Returns summary cards for the filtered audit result. |
| [`auditView`](../src/server/frontend/views/provenance.js#L117) | function | 117 | `function auditView(data)` | Returns the audit timeline and pagination markup. |
| [`artifactContext`](../src/server/frontend/views/provenance.js#L133) | function | 133 | `function artifactContext(context)` | Returns the research-context fields displayed for an artifact. |
| [`artifactActions`](../src/server/frontend/views/provenance.js#L146) | function | 146 | `function artifactActions(row)` | Returns safe inspect and download actions for an artifact. |
| [`artifactsView`](../src/server/frontend/views/provenance.js#L158) | function | 158 | `function artifactsView(data)` | Returns the run artifact inventory markup. |
| [`pageSizeOptions`](../src/server/frontend/views/provenance.js#L192) | function | 192 | `function pageSizeOptions(current)` | Returns page-size option markup with the current value selected. |
| [`cacheView`](../src/server/frontend/views/provenance.js#L199) | function | 199 | `function cacheView(data)` | Returns cache-use evidence and pagination markup. |
| [`stageStatus`](../src/server/frontend/views/provenance.js#L235) | function | 235 | `function stageStatus(summary, step)` | Returns the effective display status for a work-stage record. |
| [`stageFlow`](../src/server/frontend/views/provenance.js#L255) | function | 255 | `function stageFlow(summaries, steps)` | Returns ordered stage-flow markup for one work. |
| [`stagesView`](../src/server/frontend/views/provenance.js#L299) | function | 299 | `function stagesView(data)` | Returns work-stage evidence and pagination markup. |
| [`runView`](../src/server/frontend/views/provenance.js#L333) | function | 333 | `function runView(artifactData)` | Returns stored run details and exact configuration links. |
| [`provenanceView`](../src/server/frontend/views/provenance.js#L376) | function | 376 | `async function provenanceView()` | Asynchronously implements provenance view for the viewer. |
| [`bindAuditControls`](../src/server/frontend/views/provenance.js#L431) | function | 431 | `function bindAuditControls()` | Binds DOM behavior for audit controls. |
| [`bindArtifactInspection`](../src/server/frontend/views/provenance.js#L495) | function | 495 | `function bindArtifactInspection()` | Binds DOM behavior for artifact inspection. |
| [`renderArtifactInspector`](../src/server/frontend/views/provenance.js#L533) | function | 533 | `function renderArtifactInspector()` | Renders artifact inspector. |
| [`copyArtifactText`](../src/server/frontend/views/provenance.js#L586) | function | 586 | `async function copyArtifactText(text)` | Asynchronously copies artifact text. |

### [`src/server/frontend/views/relationships.js`](../src/server/frontend/views/relationships.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`modeControl`](../src/server/frontend/views/relationships.js#L33) | function | 33 | `function modeControl(current)` | Returns markup for selecting a relationship graph mode. |
| [`appliedFilters`](../src/server/frontend/views/relationships.js#L45) | function | 45 | `function appliedFilters()` | Returns markup summarizing the active relationship filters. |
| [`clusterSummary`](../src/server/frontend/views/relationships.js#L58) | function | 58 | `function clusterSummary(data)` | Returns markup summarizing connected graph clusters. |
| [`relationshipsView`](../src/server/frontend/views/relationships.js#L74) | function | 74 | `async function relationshipsView()` | Asynchronously implements relationships view for the viewer. |

### [`src/server/frontend/views/trash.js`](../src/server/frontend/views/trash.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`trashView`](../src/server/frontend/views/trash.js#L6) | function | 6 | `async function trashView()` | Asynchronously implements trash view for the viewer. |

## JavaScript test cases

### [`frontend/tests/e2e.spec.cjs`](../frontend/tests/e2e.spec.cjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`pipeline evidence is consistent across Corpus, Provenance, and Evaluation`](../frontend/tests/e2e.spec.cjs#L48) | test | 48 | `test('pipeline evidence is consistent across Corpus, Provenance, and Evaluation', callback)` | pipeline evidence is consistent across Corpus, Provenance, and Evaluation |

### [`frontend/tests/ui-quality.spec.cjs`](../frontend/tests/ui-quality.spec.cjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`detail breadcrumbs remain concise and identify the parent collection`](../frontend/tests/ui-quality.spec.cjs#L35) | test | 35 | `test('detail breadcrumbs remain concise and identify the parent collection', callback)` | detail breadcrumbs remain concise and identify the parent collection |
| [`mobile and medium layouts fit the viewport while tables retain their own scroller`](../frontend/tests/ui-quality.spec.cjs#L44) | test | 44 | `test('mobile and medium layouts fit the viewport while tables retain their own scroller', callback)` | mobile and medium layouts fit the viewport while tables retain their own scroller |
| [`skip link, errors, and reduced motion are announced`](../frontend/tests/ui-quality.spec.cjs#L68) | test | 68 | `test('skip link, errors, and reduced motion are announced', callback)` | skip link, errors, and reduced motion are announced |
| [`${name} has no axe violations`](../frontend/tests/ui-quality.spec.cjs#L103) | test | 103 | ``test(`${name} has no axe violations`, callback)`` | ${name} has no axe violations |
| [`${name} light`](../frontend/tests/ui-quality.spec.cjs#L124) | test | 124 | ``test(`${name} light`, callback)`` | ${name} light |
| [`overview dark`](../frontend/tests/ui-quality.spec.cjs#L130) | test | 130 | `test('overview dark', callback)` | overview dark |
| [`artifact preview light`](../frontend/tests/ui-quality.spec.cjs#L136) | test | 136 | `test('artifact preview light', callback)` | artifact preview light |

### [`frontend/tests/unit/api.test.js`](../frontend/tests/unit/api.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`builds a path with query parameters`](../frontend/tests/unit/api.test.js#L11) | test | 11 | `it('builds a path with query parameters', callback)` | builds a path with query parameters |
| [`omits empty and null parameters`](../frontend/tests/unit/api.test.js#L16) | test | 16 | `it('omits empty and null parameters', callback)` | omits empty and null parameters |
| [`handles no query`](../frontend/tests/unit/api.test.js#L24) | test | 24 | `it('handles no query', callback)` | handles no query |
| [`fetches and returns data from a successful response`](../frontend/tests/unit/api.test.js#L36) | test | 36 | `it('fetches and returns data from a successful response', callback)` | fetches and returns data from a successful response |
| [`returns the full body when no data key`](../frontend/tests/unit/api.test.js#L52) | test | 52 | `it('returns the full body when no data key', callback)` | returns the full body when no data key |
| [`throws on non-ok response with error message`](../frontend/tests/unit/api.test.js#L68) | test | 68 | `it('throws on non-ok response with error message', callback)` | throws on non-ok response with error message |
| [`throws on non-ok response without error message`](../frontend/tests/unit/api.test.js#L85) | test | 85 | `it('throws on non-ok response without error message', callback)` | throws on non-ok response without error message |
| [`throws on invalid JSON response`](../frontend/tests/unit/api.test.js#L102) | test | 102 | `it('throws on invalid JSON response', callback)` | throws on invalid JSON response |
| [`aborts when state.controller is aborted`](../frontend/tests/unit/api.test.js#L119) | test | 119 | `it('aborts when state.controller is aborted', callback)` | aborts when state.controller is aborted |
| [`fetches tables on first call and caches them`](../frontend/tests/unit/api.test.js#L146) | test | 146 | `it('fetches tables on first call and caches them', callback)` | fetches tables on first call and caches them |

### [`frontend/tests/unit/components/context-selector.test.js`](../frontend/tests/unit/components/context-selector.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`selects.search is a DOM element`](../frontend/tests/unit/components/context-selector.test.js#L11) | test | 11 | `it('selects.search is a DOM element', callback)` | selects.search is a DOM element |
| [`selects.revision is a DOM element`](../frontend/tests/unit/components/context-selector.test.js#L16) | test | 16 | `it('selects.revision is a DOM element', callback)` | selects.revision is a DOM element |
| [`selects.plan is a DOM element`](../frontend/tests/unit/components/context-selector.test.js#L21) | test | 21 | `it('selects.plan is a DOM element', callback)` | selects.plan is a DOM element |
| [`selects.run is a DOM element`](../frontend/tests/unit/components/context-selector.test.js#L26) | test | 26 | `it('selects.run is a DOM element', callback)` | selects.run is a DOM element |
| [`clearContext is a DOM element`](../frontend/tests/unit/components/context-selector.test.js#L31) | test | 31 | `it('clearContext is a DOM element', callback)` | clearContext is a DOM element |
| [`fetches searches and populates the search selector`](../frontend/tests/unit/components/context-selector.test.js#L46) | test | 46 | `it('fetches searches and populates the search selector', callback)` | fetches searches and populates the search selector |
| [`shows selection summary when no revision is selected`](../frontend/tests/unit/components/context-selector.test.js#L71) | test | 71 | `it('shows selection summary when no revision is selected', callback)` | shows selection summary when no revision is selected |

### [`frontend/tests/unit/components/data-table.test.js`](../frontend/tests/unit/components/data-table.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`returns all rows when query is empty`](../frontend/tests/unit/components/data-table.test.js#L17) | test | 17 | `it('returns all rows when query is empty', callback)` | returns all rows when query is empty |
| [`filters rows by matching any field`](../frontend/tests/unit/components/data-table.test.js#L23) | test | 23 | `it('filters rows by matching any field', callback)` | filters rows by matching any field |
| [`is case-insensitive`](../frontend/tests/unit/components/data-table.test.js#L29) | test | 29 | `it('is case-insensitive', callback)` | is case-insensitive |
| [`matches across multiple fields`](../frontend/tests/unit/components/data-table.test.js#L34) | test | 34 | `it('matches across multiple fields', callback)` | matches across multiple fields |
| [`returns empty array when no match`](../frontend/tests/unit/components/data-table.test.js#L39) | test | 39 | `it('returns empty array when no match', callback)` | returns empty array when no match |
| [`renders a table with header and rows`](../frontend/tests/unit/components/data-table.test.js#L47) | test | 47 | `it('renders a table with header and rows', callback)` | renders a table with header and rows |
| [`renders empty state when no rows`](../frontend/tests/unit/components/data-table.test.js#L65) | test | 65 | `it('renders empty state when no rows', callback)` | renders empty state when no rows |
| [`renders empty state with query message when query is set`](../frontend/tests/unit/components/data-table.test.js#L75) | test | 75 | `it('renders empty state with query message when query is set', callback)` | renders empty state with query message when query is set |
| [`renders expandable rows when expandableFields provided`](../frontend/tests/unit/components/data-table.test.js#L85) | test | 85 | `it('renders expandable rows when expandableFields provided', callback)` | renders expandable rows when expandableFields provided |
| [`renders sort buttons for sortable columns`](../frontend/tests/unit/components/data-table.test.js#L103) | test | 103 | `it('renders sort buttons for sortable columns', callback)` | renders sort buttons for sortable columns |
| [`disables previous button on page 1`](../frontend/tests/unit/components/data-table.test.js#L114) | test | 114 | `it('disables previous button on page 1', callback)` | disables previous button on page 1 |
| [`disables next button when has_next is false`](../frontend/tests/unit/components/data-table.test.js#L124) | test | 124 | `it('disables next button when has_next is false', callback)` | disables next button when has_next is false |
| [`handles column objects with name property`](../frontend/tests/unit/components/data-table.test.js#L134) | test | 134 | `it('handles column objects with name property', callback)` | handles column objects with name property |
| [`filters columns by whitelist`](../frontend/tests/unit/components/data-table.test.js#L146) | test | 146 | `it('filters columns by whitelist', callback)` | filters columns by whitelist |
| [`binds click handlers to sort buttons`](../frontend/tests/unit/components/data-table.test.js#L165) | test | 165 | `it('binds click handlers to sort buttons', callback)` | binds click handlers to sort buttons |
| [`binds click handlers to page buttons`](../frontend/tests/unit/components/data-table.test.js#L179) | test | 179 | `it('binds click handlers to page buttons', callback)` | binds click handlers to page buttons |

### [`frontend/tests/unit/components/graph.test.js`](../frontend/tests/unit/components/graph.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`renders a labeled input field`](../frontend/tests/unit/components/graph.test.js#L11) | test | 11 | `it('renders a labeled input field', callback)` | renders a labeled input field |
| [`uses provided type`](../frontend/tests/unit/components/graph.test.js#L19) | test | 19 | `it('uses provided type', callback)` | uses provided type |
| [`escapes HTML in label`](../frontend/tests/unit/components/graph.test.js#L24) | test | 24 | `it('escapes HTML in label', callback)` | escapes HTML in label |
| [`returns an object with all graph filter values`](../frontend/tests/unit/components/graph.test.js#L34) | test | 34 | `it('returns an object with all graph filter values', callback)` | returns an object with all graph filter values |
| [`links to article view for article nodes`](../frontend/tests/unit/components/graph.test.js#L47) | test | 47 | `it('links to article view for article nodes', callback)` | links to article view for article nodes |
| [`links to author view for author nodes`](../frontend/tests/unit/components/graph.test.js#L52) | test | 52 | `it('links to author view for author nodes', callback)` | links to author view for author nodes |
| [`links to reference view for other nodes`](../frontend/tests/unit/components/graph.test.js#L57) | test | 57 | `it('links to reference view for other nodes', callback)` | links to reference view for other nodes |
| [`does not fabricate a detail route for a raw referenced-author string`](../frontend/tests/unit/components/graph.test.js#L62) | test | 62 | `it('does not fabricate a detail route for a raw referenced-author string', callback)` | does not fabricate a detail route for a raw referenced-author string |
| [`renders graph result with nodes and edges`](../frontend/tests/unit/components/graph.test.js#L70) | test | 70 | `it('renders graph result with nodes and edges', callback)` | renders graph result with nodes and edges |
| [`includes truncation warning when data is truncated`](../frontend/tests/unit/components/graph.test.js#L84) | test | 84 | `it('includes truncation warning when data is truncated', callback)` | includes truncation warning when data is truncated |
| [`handles missing counts`](../frontend/tests/unit/components/graph.test.js#L96) | test | 96 | `it('handles missing counts', callback)` | handles missing counts |
| [`includes node search input in toolbar`](../frontend/tests/unit/components/graph.test.js#L103) | test | 103 | `it('includes node search input in toolbar', callback)` | includes node search input in toolbar |
| [`includes zoom indicator in toolbar`](../frontend/tests/unit/components/graph.test.js#L110) | test | 110 | `it('includes zoom indicator in toolbar', callback)` | includes zoom indicator in toolbar |
| [`includes export PNG button in toolbar`](../frontend/tests/unit/components/graph.test.js#L117) | test | 117 | `it('includes export PNG button in toolbar', callback)` | includes export PNG button in toolbar |
| [`assigns deterministic connected components`](../frontend/tests/unit/components/graph.test.js#L127) | test | 127 | `it('assigns deterministic connected components', callback)` | assigns deterministic connected components |
| [`keeps the world position beneath the pointer fixed while zooming`](../frontend/tests/unit/components/graph.test.js#L139) | test | 139 | `it('keeps the world position beneath the pointer fixed while zooming', callback)` | keeps the world position beneath the pointer fixed while zooming |
| [`does nothing when no active graph`](../frontend/tests/unit/components/graph.test.js#L154) | test | 154 | `it('does nothing when no active graph', callback)` | does nothing when no active graph |
| [`does nothing when canvas is missing`](../frontend/tests/unit/components/graph.test.js#L163) | test | 163 | `it('does nothing when canvas is missing', callback)` | does nothing when canvas is missing |

### [`frontend/tests/unit/components/pagination.test.js`](../frontend/tests/unit/components/pagination.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`keeps a bounded page window around the current page`](../frontend/tests/unit/components/pagination.test.js#L8) | test | 8 | `it('keeps a bounded page window around the current page', callback)` | keeps a bounded page window around the current page |
| [`renders range, page count, numbered pages, and boundary controls`](../frontend/tests/unit/components/pagination.test.js#L14) | test | 14 | `it('renders range, page count, numbered pages, and boundary controls', callback)` | renders range, page count, numbered pages, and boundary controls |
| [`handles an empty result without inventing a visible row`](../frontend/tests/unit/components/pagination.test.js#L24) | test | 24 | `it('handles an empty result without inventing a visible row', callback)` | handles an empty result without inventing a visible row |

### [`frontend/tests/unit/components/shell.test.js`](../frontend/tests/unit/components/shell.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`is a DOM element`](../frontend/tests/unit/components/shell.test.js#L10) | test | 10 | `it('is a DOM element', callback)` | is a DOM element |
| [`sets healthy status on successful response`](../frontend/tests/unit/components/shell.test.js#L24) | test | 24 | `it('sets healthy status on successful response', callback)` | sets healthy status on successful response |
| [`sets unavailable status when readable is false`](../frontend/tests/unit/components/shell.test.js#L42) | test | 42 | `it('sets unavailable status when readable is false', callback)` | sets unavailable status when readable is false |
| [`sets unavailable status on fetch failure`](../frontend/tests/unit/components/shell.test.js#L59) | test | 59 | `it('sets unavailable status on fetch failure', callback)` | sets unavailable status on fetch failure |
| [`toggles rw-mobile-nav-open on click`](../frontend/tests/unit/components/shell.test.js#L96) | test | 96 | `it('toggles rw-mobile-nav-open on click', callback)` | toggles rw-mobile-nav-open on click |
| [`closes nav when a nav link is clicked`](../frontend/tests/unit/components/shell.test.js#L108) | test | 108 | `it('closes nav when a nav link is clicked', callback)` | closes nav when a nav link is clicked |
| [`is a no-op when toggle or nav is missing`](../frontend/tests/unit/components/shell.test.js#L118) | test | 118 | `it('is a no-op when toggle or nav is missing', callback)` | is a no-op when toggle or nav is missing |

### [`frontend/tests/unit/router.test.js`](../frontend/tests/unit/router.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`updates URL and triggers render`](../frontend/tests/unit/router.test.js#L11) | test | 11 | `it('updates URL and triggers render', callback)` | updates URL and triggers render |
| [`replaces state when replace is true`](../frontend/tests/unit/router.test.js#L19) | test | 19 | `it('replaces state when replace is true', callback)` | replaces state when replace is true |
| [`binds click handler to focus-context button`](../frontend/tests/unit/router.test.js#L31) | test | 31 | `it('binds click handler to focus-context button', callback)` | binds click handler to focus-context button |
| [`does nothing when button is missing`](../frontend/tests/unit/router.test.js#L44) | test | 44 | `it('does nothing when button is missing', callback)` | does nothing when button is missing |
| [`renders overview when no view is set`](../frontend/tests/unit/router.test.js#L58) | test | 58 | `it('renders overview when no view is set', callback)` | renders overview when no view is set |

### [`frontend/tests/unit/state.test.js`](../frontend/tests/unit/state.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`app is a DOM element`](../frontend/tests/unit/state.test.js#L18) | test | 18 | `it('app is a DOM element', callback)` | app is a DOM element |
| [`notice is a DOM element`](../frontend/tests/unit/state.test.js#L23) | test | 23 | `it('notice is a DOM element', callback)` | notice is a DOM element |
| [`loading is a DOM element`](../frontend/tests/unit/state.test.js#L28) | test | 28 | `it('loading is a DOM element', callback)` | loading is a DOM element |
| [`state has expected keys`](../frontend/tests/unit/state.test.js#L33) | test | 33 | `it('state has expected keys', callback)` | state has expected keys |
| [`pageSizes contains expected values`](../frontend/tests/unit/state.test.js#L42) | test | 42 | `it('pageSizes contains expected values', callback)` | pageSizes contains expected values |
| [`corpusSections has expected keys`](../frontend/tests/unit/state.test.js#L46) | test | 46 | `it('corpusSections has expected keys', callback)` | corpusSections has expected keys |
| [`provenanceSections has expected keys`](../frontend/tests/unit/state.test.js#L55) | test | 55 | `it('provenanceSections has expected keys', callback)` | provenanceSections has expected keys |
| [`graphFilters contains expected filters`](../frontend/tests/unit/state.test.js#L64) | test | 64 | `it('graphFilters contains expected filters', callback)` | graphFilters contains expected filters |
| [`params returns URLSearchParams from location.search`](../frontend/tests/unit/state.test.js#L75) | test | 75 | `it('params returns URLSearchParams from location.search', callback)` | params returns URLSearchParams from location.search |
| [`value reads a query parameter`](../frontend/tests/unit/state.test.js#L81) | test | 81 | `it('value reads a query parameter', callback)` | value reads a query parameter |
| [`view returns the current view or overview default`](../frontend/tests/unit/state.test.js#L86) | test | 86 | `it('view returns the current view or overview default', callback)` | view returns the current view or overview default |
| [`section returns the named parameter or fallback`](../frontend/tests/unit/state.test.js#L90) | test | 90 | `it('section returns the named parameter or fallback', callback)` | section returns the named parameter or fallback |
| [`escapes HTML special characters`](../frontend/tests/unit/state.test.js#L99) | test | 99 | `it('escapes HTML special characters', callback)` | escapes HTML special characters |
| [`handles null and undefined`](../frontend/tests/unit/state.test.js#L106) | test | 106 | `it('handles null and undefined', callback)` | handles null and undefined |
| [`converts numbers to strings`](../frontend/tests/unit/state.test.js#L111) | test | 111 | `it('converts numbers to strings', callback)` | converts numbers to strings |
| [`escapes ampersands`](../frontend/tests/unit/state.test.js#L116) | test | 116 | `it('escapes ampersands', callback)` | escapes ampersands |
| [`returns strings unchanged`](../frontend/tests/unit/state.test.js#L124) | test | 124 | `it('returns strings unchanged', callback)` | returns strings unchanged |
| [`formats objects as pretty JSON`](../frontend/tests/unit/state.test.js#L128) | test | 128 | `it('formats objects as pretty JSON', callback)` | formats objects as pretty JSON |
| [`formats arrays as pretty JSON`](../frontend/tests/unit/state.test.js#L134) | test | 134 | `it('formats arrays as pretty JSON', callback)` | formats arrays as pretty JSON |
| [`handles null`](../frontend/tests/unit/state.test.js#L139) | test | 139 | `it('handles null', callback)` | handles null |
| [`returns the first matching key from data`](../frontend/tests/unit/state.test.js#L147) | test | 147 | `it('returns the first matching key from data', callback)` | returns the first matching key from data |
| [`returns data if it is an array`](../frontend/tests/unit/state.test.js#L152) | test | 152 | `it('returns data if it is an array', callback)` | returns data if it is an array |
| [`returns empty array for non-array data`](../frontend/tests/unit/state.test.js#L156) | test | 156 | `it('returns empty array for non-array data', callback)` | returns empty array for non-array data |
| [`returns empty array when no keys match`](../frontend/tests/unit/state.test.js#L162) | test | 162 | `it('returns empty array when no keys match', callback)` | returns empty array when no keys match |
| [`prefers .id`](../frontend/tests/unit/state.test.js#L170) | test | 170 | `it('prefers .id', callback)` | prefers .id |
| [`falls back to search_id`](../frontend/tests/unit/state.test.js#L174) | test | 174 | `it('falls back to search_id', callback)` | falls back to search_id |
| [`falls back to run_id`](../frontend/tests/unit/state.test.js#L178) | test | 178 | `it('falls back to run_id', callback)` | falls back to run_id |
| [`falls back to plan_id`](../frontend/tests/unit/state.test.js#L182) | test | 182 | `it('falls back to plan_id', callback)` | falls back to plan_id |
| [`returns empty string when no id found`](../frontend/tests/unit/state.test.js#L186) | test | 186 | `it('returns empty string when no id found', callback)` | returns empty string when no id found |
| [`returns the first non-empty field`](../frontend/tests/unit/state.test.js#L196) | test | 196 | `it('returns the first non-empty field', callback)` | returns the first non-empty field |
| [`returns Unnamed when all fields are empty`](../frontend/tests/unit/state.test.js#L200) | test | 200 | `it('returns Unnamed when all fields are empty', callback)` | returns Unnamed when all fields are empty |
| [`returns Unnamed for null/undefined item`](../frontend/tests/unit/state.test.js#L204) | test | 204 | `it('returns Unnamed for null/undefined item', callback)` | returns Unnamed for null/undefined item |
| [`parses numeric values`](../frontend/tests/unit/state.test.js#L213) | test | 213 | `it('parses numeric values', callback)` | parses numeric values |
| [`returns 0 for non-numeric values`](../frontend/tests/unit/state.test.js#L218) | test | 218 | `it('returns 0 for non-numeric values', callback)` | returns 0 for non-numeric values |
| [`handles { value } objects`](../frontend/tests/unit/state.test.js#L224) | test | 224 | `it('handles { value } objects', callback)` | handles { value } objects |
| [`returns 0 for NaN and Infinity`](../frontend/tests/unit/state.test.js#L229) | test | 229 | `it('returns 0 for NaN and Infinity', callback)` | returns 0 for NaN and Infinity |
| [`formats numbers with locale separators`](../frontend/tests/unit/state.test.js#L238) | test | 238 | `it('formats numbers with locale separators', callback)` | formats numbers with locale separators |
| [`handles zero`](../frontend/tests/unit/state.test.js#L244) | test | 244 | `it('handles zero', callback)` | handles zero |
| [`handles object values`](../frontend/tests/unit/state.test.js#L248) | test | 248 | `it('handles object values', callback)` | handles object values |
| [`calculates percentage`](../frontend/tests/unit/state.test.js#L257) | test | 257 | `it('calculates percentage', callback)` | calculates percentage |
| [`returns em dash for zero denominator`](../frontend/tests/unit/state.test.js#L261) | test | 261 | `it('returns em dash for zero denominator', callback)` | returns em dash for zero denominator |
| [`handles object values`](../frontend/tests/unit/state.test.js#L265) | test | 265 | `it('handles object values', callback)` | handles object values |
| [`returns em dash for falsy input`](../frontend/tests/unit/state.test.js#L273) | test | 273 | `it('returns em dash for falsy input', callback)` | returns em dash for falsy input |
| [`returns the raw string for invalid dates`](../frontend/tests/unit/state.test.js#L279) | test | 279 | `it('returns the raw string for invalid dates', callback)` | returns the raw string for invalid dates |
| [`formats valid date strings`](../frontend/tests/unit/state.test.js#L283) | test | 283 | `it('formats valid date strings', callback)` | formats valid date strings |
| [`formats byte counts without treating kilobytes as decimal units`](../frontend/tests/unit/state.test.js#L294) | test | 294 | `it('formats byte counts without treating kilobytes as decimal units', callback)` | formats byte counts without treating kilobytes as decimal units |
| [`turns stored field names into readable labels`](../frontend/tests/unit/state.test.js#L299) | test | 299 | `it('turns stored field names into readable labels', callback)` | turns stored field names into readable labels |
| [`parses object payloads and rejects scalar or malformed JSON`](../frontend/tests/unit/state.test.js#L303) | test | 303 | `it('parses object payloads and rejects scalar or malformed JSON', callback)` | parses object payloads and rejects scalar or malformed JSON |
| [`uses namespaced pagination updates when a filter chip is removed`](../frontend/tests/unit/state.test.js#L309) | test | 309 | `it('uses namespaced pagination updates when a filter chip is removed', callback)` | uses namespaced pagination updates when a filter chip is removed |
| [`returns red for failure-related statuses`](../frontend/tests/unit/state.test.js#L324) | test | 324 | `it('returns red for failure-related statuses', callback)` | returns red for failure-related statuses |
| [`returns green for completion-related statuses`](../frontend/tests/unit/state.test.js#L332) | test | 332 | `it('returns green for completion-related statuses', callback)` | returns green for completion-related statuses |
| [`returns orange for warning-like statuses`](../frontend/tests/unit/state.test.js#L340) | test | 340 | `it('returns orange for warning-like statuses', callback)` | returns orange for warning-like statuses |
| [`returns blue for informational and unrecorded statuses`](../frontend/tests/unit/state.test.js#L349) | test | 349 | `it('returns blue for informational and unrecorded statuses', callback)` | returns blue for informational and unrecorded statuses |
| [`wraps status in a span with class`](../frontend/tests/unit/state.test.js#L361) | test | 361 | `it('wraps status in a span with class', callback)` | wraps status in a span with class |
| [`uses Not recorded for null`](../frontend/tests/unit/state.test.js#L367) | test | 367 | `it('uses Not recorded for null', callback)` | uses Not recorded for null |
| [`converts arrays of objects to entries`](../frontend/tests/unit/state.test.js#L376) | test | 376 | `it('converts arrays of objects to entries', callback)` | converts arrays of objects to entries |
| [`converts object to entries`](../frontend/tests/unit/state.test.js#L387) | test | 387 | `it('converts object to entries', callback)` | converts object to entries |
| [`returns empty array for null/undefined`](../frontend/tests/unit/state.test.js#L392) | test | 392 | `it('returns empty array for null/undefined', callback)` | returns empty array for null/undefined |
| [`returns undefined when no run_id is set`](../frontend/tests/unit/state.test.js#L401) | test | 401 | `it('returns undefined when no run_id is set', callback)` | returns undefined when no run_id is set |
| [`finds a run by run_id`](../frontend/tests/unit/state.test.js#L405) | test | 405 | `it('finds a run by run_id', callback)` | finds a run by run_id |
| [`showError sets notice text and removes hidden`](../frontend/tests/unit/state.test.js#L426) | test | 426 | `it('showError sets notice text and removes hidden', callback)` | showError sets notice text and removes hidden |
| [`showError handles string errors`](../frontend/tests/unit/state.test.js#L433) | test | 433 | `it('showError handles string errors', callback)` | showError handles string errors |
| [`clearError hides notice and clears text`](../frontend/tests/unit/state.test.js#L438) | test | 438 | `it('clearError hides notice and clears text', callback)` | clearError hides notice and clears text |
| [`busy toggles loading visibility`](../frontend/tests/unit/state.test.js#L444) | test | 444 | `it('busy toggles loading visibility', callback)` | busy toggles loading visibility |
| [`builds a query string from updates`](../frontend/tests/unit/state.test.js#L455) | test | 455 | `it('builds a query string from updates', callback)` | builds a query string from updates |
| [`removes keys with empty values but keeps view default`](../frontend/tests/unit/state.test.js#L462) | test | 462 | `it('removes keys with empty values but keeps view default', callback)` | removes keys with empty values but keeps view default |
| [`removes keys with null values but keeps view default`](../frontend/tests/unit/state.test.js#L468) | test | 468 | `it('removes keys with null values but keeps view default', callback)` | removes keys with null values but keeps view default |
| [`ensures view defaults to overview`](../frontend/tests/unit/state.test.js#L473) | test | 473 | `it('ensures view defaults to overview', callback)` | ensures view defaults to overview |
| [`returns empty query for no updates`](../frontend/tests/unit/state.test.js#L478) | test | 478 | `it('returns empty query for no updates', callback)` | returns empty query for no updates |
| [`renders a page header with kicker, title, description`](../frontend/tests/unit/state.test.js#L487) | test | 487 | `it('renders a page header with kicker, title, description', callback)` | renders a page header with kicker, title, description |
| [`includes extra content when provided`](../frontend/tests/unit/state.test.js#L497) | test | 497 | `it('includes extra content when provided', callback)` | includes extra content when provided |
| [`escapes HTML in inputs`](../frontend/tests/unit/state.test.js#L502) | test | 502 | `it('escapes HTML in inputs', callback)` | escapes HTML in inputs |
| [`returns empty string when no context is set`](../frontend/tests/unit/state.test.js#L512) | test | 512 | `it('returns empty string when no context is set', callback)` | returns empty string when no context is set |
| [`includes search_id when set`](../frontend/tests/unit/state.test.js#L516) | test | 516 | `it('includes search_id when set', callback)` | includes search_id when set |
| [`uses only the immediate parent and record type for detail context`](../frontend/tests/unit/state.test.js#L531) | test | 531 | `it('uses only the immediate parent and record type for detail context', callback)` | uses only the immediate parent and record type for detail context |
| [`renders an empty state with page header and panel`](../frontend/tests/unit/state.test.js#L551) | test | 551 | `it('renders an empty state with page header and panel', callback)` | renders an empty state with page header and panel |
| [`includes action content when provided`](../frontend/tests/unit/state.test.js#L558) | test | 558 | `it('includes action content when provided', callback)` | includes action content when provided |
| [`renders a panel section`](../frontend/tests/unit/state.test.js#L567) | test | 567 | `it('renders a panel section', callback)` | renders a panel section |
| [`omits description paragraph when empty`](../frontend/tests/unit/state.test.js#L576) | test | 576 | `it('omits description paragraph when empty', callback)` | omits description paragraph when empty |
| [`includes classes when provided`](../frontend/tests/unit/state.test.js#L581) | test | 581 | `it('includes classes when provided', callback)` | includes classes when provided |
| [`renders a table with header and rows`](../frontend/tests/unit/state.test.js#L590) | test | 590 | `it('renders a table with header and rows', callback)` | renders a table with header and rows |
| [`renders empty state when no rows`](../frontend/tests/unit/state.test.js#L602) | test | 602 | `it('renders empty state when no rows', callback)` | renders empty state when no rows |
| [`handles column objects with render functions`](../frontend/tests/unit/state.test.js#L607) | test | 607 | `it('handles column objects with render functions', callback)` | handles column objects with render functions |
| [`handles column objects with label property`](../frontend/tests/unit/state.test.js#L614) | test | 614 | `it('handles column objects with label property', callback)` | handles column objects with label property |
| [`renders subnavigation links`](../frontend/tests/unit/state.test.js#L625) | test | 625 | `it('renders subnavigation links', callback)` | renders subnavigation links |
| [`renders a metric card with value`](../frontend/tests/unit/state.test.js#L639) | test | 639 | `it('renders a metric card with value', callback)` | renders a metric card with value |
| [`renders unavailable state`](../frontend/tests/unit/state.test.js#L646) | test | 646 | `it('renders unavailable state', callback)` | renders unavailable state |
| [`includes href when provided`](../frontend/tests/unit/state.test.js#L651) | test | 651 | `it('includes href when provided', callback)` | includes href when provided |
| [`shows percentage when denominator is present`](../frontend/tests/unit/state.test.js#L656) | test | 656 | `it('shows percentage when denominator is present', callback)` | shows percentage when denominator is present |
| [`renders a flow stage with count`](../frontend/tests/unit/state.test.js#L666) | test | 666 | `it('renders a flow stage with count', callback)` | renders a flow stage with count |
| [`shows input baseline for null previous`](../frontend/tests/unit/state.test.js#L674) | test | 674 | `it('shows input baseline for null previous', callback)` | shows input baseline for null previous |
| [`shows diff from prior`](../frontend/tests/unit/state.test.js#L679) | test | 679 | `it('shows diff from prior', callback)` | shows diff from prior |
| [`handles unavailable state`](../frontend/tests/unit/state.test.js#L684) | test | 684 | `it('handles unavailable state', callback)` | handles unavailable state |
| [`handles null raw`](../frontend/tests/unit/state.test.js#L689) | test | 689 | `it('handles null raw', callback)` | handles null raw |
| [`renders not recorded when input is missing`](../frontend/tests/unit/state.test.js#L698) | test | 698 | `it('renders not recorded when input is missing', callback)` | renders not recorded when input is missing |
| [`renders not recorded when input is unavailable`](../frontend/tests/unit/state.test.js#L703) | test | 703 | `it('renders not recorded when input is unavailable', callback)` | renders not recorded when input is unavailable |
| [`renders full retention flow with valid data`](../frontend/tests/unit/state.test.js#L708) | test | 708 | `it('renders full retention flow with valid data', callback)` | renders full retention flow with valid data |
| [`uses the initial unfiltered source total for every retention percentage`](../frontend/tests/unit/state.test.js#L726) | test | 726 | `it('uses the initial unfiltered source total for every retention percentage', callback)` | uses the initial unfiltered source total for every retention percentage |
| [`renders not recorded for empty entries`](../frontend/tests/unit/state.test.js#L773) | test | 773 | `it('renders not recorded for empty entries', callback)` | renders not recorded for empty entries |
| [`renders a breakdown table with entries`](../frontend/tests/unit/state.test.js#L778) | test | 778 | `it('renders a breakdown table with entries', callback)` | renders a breakdown table with entries |
| [`renders with total when useTotal is true`](../frontend/tests/unit/state.test.js#L786) | test | 786 | `it('renders with total when useTotal is true', callback)` | renders with total when useTotal is true |
| [`renders a table with source counts`](../frontend/tests/unit/state.test.js#L797) | test | 797 | `it('renders a table with source counts', callback)` | renders a table with source counts |
| [`handles empty items`](../frontend/tests/unit/state.test.js#L808) | test | 808 | `it('handles empty items', callback)` | handles empty items |
| [`renders empty state for no rows`](../frontend/tests/unit/state.test.js#L817) | test | 817 | `it('renders empty state for no rows', callback)` | renders empty state for no rows |
| [`renders timeline items`](../frontend/tests/unit/state.test.js#L821) | test | 821 | `it('renders timeline items', callback)` | renders timeline items |
| [`renders enrichment detail`](../frontend/tests/unit/state.test.js#L831) | test | 831 | `it('renders enrichment detail', callback)` | renders enrichment detail |
| [`renders validation detail`](../frontend/tests/unit/state.test.js#L842) | test | 842 | `it('renders validation detail', callback)` | renders validation detail |
| [`renders error detail`](../frontend/tests/unit/state.test.js#L852) | test | 852 | `it('renders error detail', callback)` | renders error detail |
| [`renders status detail`](../frontend/tests/unit/state.test.js#L861) | test | 861 | `it('renders status detail', callback)` | renders status detail |
| [`renders identity detail`](../frontend/tests/unit/state.test.js#L869) | test | 869 | `it('renders identity detail', callback)` | renders identity detail |
| [`renders search detail with revision`](../frontend/tests/unit/state.test.js#L877) | test | 877 | `it('renders search detail with revision', callback)` | renders search detail with revision |
| [`includes actor when present`](../frontend/tests/unit/state.test.js#L886) | test | 886 | `it('includes actor when present', callback)` | includes actor when present |
| [`includes entity when present`](../frontend/tests/unit/state.test.js#L895) | test | 895 | `it('includes entity when present', callback)` | includes entity when present |
| [`renders a table from an array of records`](../frontend/tests/unit/state.test.js#L909) | test | 909 | `it('renders a table from an array of records', callback)` | renders a table from an array of records |
| [`handles empty rows`](../frontend/tests/unit/state.test.js#L919) | test | 919 | `it('handles empty rows', callback)` | handles empty rows |
| [`renders NULL for null/undefined`](../frontend/tests/unit/state.test.js#L928) | test | 928 | `it('renders NULL for null/undefined', callback)` | renders NULL for null/undefined |
| [`renders short values inline`](../frontend/tests/unit/state.test.js#L934) | test | 934 | `it('renders short values inline', callback)` | renders short values inline |
| [`truncates long values with details`](../frontend/tests/unit/state.test.js#L940) | test | 940 | `it('truncates long values with details', callback)` | truncates long values with details |
| [`creates article link for article_id column`](../frontend/tests/unit/state.test.js#L947) | test | 947 | `it('creates article link for article_id column', callback)` | creates article link for article_id column |
| [`creates author link for author_id column`](../frontend/tests/unit/state.test.js#L953) | test | 953 | `it('creates author link for author_id column', callback)` | creates author link for author_id column |
| [`creates reference link for reference_id column`](../frontend/tests/unit/state.test.js#L959) | test | 959 | `it('creates reference link for reference_id column', callback)` | creates reference link for reference_id column |
| [`creates article link for work_revision id with tableName`](../frontend/tests/unit/state.test.js#L965) | test | 965 | `it('creates article link for work_revision id with tableName', callback)` | creates article link for work_revision id with tableName |
| [`creates author link for author_occurrences id with tableName`](../frontend/tests/unit/state.test.js#L970) | test | 970 | `it('creates author link for author_occurrences id with tableName', callback)` | creates author link for author_occurrences id with tableName |
| [`creates reference link for reference_mentions id with tableName`](../frontend/tests/unit/state.test.js#L975) | test | 975 | `it('creates reference link for reference_mentions id with tableName', callback)` | creates reference link for reference_mentions id with tableName |
| [`copies text and shows Copied! feedback`](../frontend/tests/unit/state.test.js#L988) | test | 988 | `it('copies text and shows Copied! feedback', callback)` | copies text and shows Copied! feedback |
| [`falls back to prompt when clipboard fails`](../frontend/tests/unit/state.test.js#L1010) | test | 1010 | `it('falls back to prompt when clipboard fails', callback)` | falls back to prompt when clipboard fails |
| [`fades out and hides message on close click`](../frontend/tests/unit/state.test.js#L1041) | test | 1041 | `it('fades out and hides message on close click', callback)` | fades out and hides message on close click |
| [`adds loading class and disables button on click`](../frontend/tests/unit/state.test.js#L1066) | test | 1066 | `it('adds loading class and disables button on click', callback)` | adds loading class and disables button on click |

### [`frontend/tests/unit/views/advanced.test.js`](../frontend/tests/unit/views/advanced.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`renders advanced view with table browser`](../frontend/tests/unit/views/advanced.test.js#L18) | test | 18 | `it('renders advanced view with table browser', callback)` | renders advanced view with table browser |

### [`frontend/tests/unit/views/corpus.test.js`](../frontend/tests/unit/views/corpus.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`renders corpus view with articles section`](../frontend/tests/unit/views/corpus.test.js#L18) | test | 18 | `it('renders corpus view with articles section', callback)` | renders corpus view with articles section |

### [`frontend/tests/unit/views/detail.test.js`](../frontend/tests/unit/views/detail.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`shows empty state when no id is set`](../frontend/tests/unit/views/detail.test.js#L17) | test | 17 | `it('shows empty state when no id is set', callback)` | shows empty state when no id is set |
| [`renders article detail when article_id is set`](../frontend/tests/unit/views/detail.test.js#L26) | test | 26 | `it('renders article detail when article_id is set', callback)` | renders article detail when article_id is set |
| [`renders discarded validation reasons as individual entries`](../frontend/tests/unit/views/detail.test.js#L111) | test | 111 | `it('renders discarded validation reasons as individual entries', callback)` | renders discarded validation reasons as individual entries |
| [`renders author detail when author_id is set`](../frontend/tests/unit/views/detail.test.js#L153) | test | 153 | `it('renders author detail when author_id is set', callback)` | renders author detail when author_id is set |
| [`renders reference detail when reference_id is set`](../frontend/tests/unit/views/detail.test.js#L191) | test | 191 | `it('renders reference detail when reference_id is set', callback)` | renders reference detail when reference_id is set |

### [`frontend/tests/unit/views/evaluation.test.js`](../frontend/tests/unit/views/evaluation.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`requires a selected run without requesting evaluation data`](../frontend/tests/unit/views/evaluation.test.js#L36) | test | 36 | `it('requires a selected run without requesting evaluation data', callback)` | requires a selected run without requesting evaluation data |
| [`renders normalized articles with red and green inventory tags`](../frontend/tests/unit/views/evaluation.test.js#L52) | test | 52 | `it('renders normalized articles with red and green inventory tags', callback)` | renders normalized articles with red and green inventory tags |

### [`frontend/tests/unit/views/overview.test.js`](../frontend/tests/unit/views/overview.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`shows empty state when no run_id is set`](../frontend/tests/unit/views/overview.test.js#L17) | test | 17 | `it('shows empty state when no run_id is set', callback)` | shows empty state when no run_id is set |
| [`renders overview content when run_id is set`](../frontend/tests/unit/views/overview.test.js#L26) | test | 26 | `it('renders overview content when run_id is set', callback)` | renders overview content when run_id is set |

### [`frontend/tests/unit/views/provenance.test.js`](../frontend/tests/unit/views/provenance.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`shows one contextual empty panel for run-scoped sections without a run`](../frontend/tests/unit/views/provenance.test.js#L28) | test | 28 | `it('shows one contextual empty panel for run-scoped sections without a run', callback)` | shows one contextual empty panel for run-scoped sections without a run |
| [`renders server-filtered audit evidence with visible active filters`](../frontend/tests/unit/views/provenance.test.js#L35) | test | 35 | `it('renders server-filtered audit evidence with visible active filters', callback)` | renders server-filtered audit evidence with visible active filters |
| [`updates URL state when audit filters are submitted`](../frontend/tests/unit/views/provenance.test.js#L60) | test | 60 | `it('updates URL state when audit filters are submitted', callback)` | updates URL state when audit filters are submitted |
| [`renders bounded artifact preview metadata and truncation guidance`](../frontend/tests/unit/views/provenance.test.js#L78) | test | 78 | `it('renders bounded artifact preview metadata and truncation guidance', callback)` | renders bounded artifact preview metadata and truncation guidance |
| [`renders cache search and complete pagination controls`](../frontend/tests/unit/views/provenance.test.js#L102) | test | 102 | `it('renders cache search and complete pagination controls', callback)` | renders cache search and complete pagination controls |
| [`renders stage progression before paginated details`](../frontend/tests/unit/views/provenance.test.js#L123) | test | 123 | `it('renders stage progression before paginated details', callback)` | renders stage progression before paginated details |
| [`renders run identity and configuration snapshots`](../frontend/tests/unit/views/provenance.test.js#L146) | test | 146 | `it('renders run identity and configuration snapshots', callback)` | renders run identity and configuration snapshots |

### [`frontend/tests/unit/views/relationships.test.js`](../frontend/tests/unit/views/relationships.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`shows empty state when no run_id is set`](../frontend/tests/unit/views/relationships.test.js#L17) | test | 17 | `it('shows empty state when no run_id is set', callback)` | shows empty state when no run_id is set |
| [`renders relationships view when run_id is set`](../frontend/tests/unit/views/relationships.test.js#L26) | test | 26 | `it('renders relationships view when run_id is set', callback)` | renders relationships view when run_id is set |

### [`frontend/tests/unit/views/trash.test.js`](../frontend/tests/unit/views/trash.test.js)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`renders trash view with trashed runs and restore history`](../frontend/tests/unit/views/trash.test.js#L17) | test | 17 | `it('renders trash view with trashed runs and restore history', callback)` | renders trash view with trashed runs and restore history |

### [`frontend/tests/viewer.spec.cjs`](../frontend/tests/viewer.spec.cjs)

| Symbol | Kind | Lines | Declaration, inputs, and outputs | Source description |
|---|---|---:|---|---|
| [`health endpoint returns readable status`](../frontend/tests/viewer.spec.cjs#L82) | test | 82 | `test('health endpoint returns readable status', callback)` | health endpoint returns readable status |
| [`page loads and renders navigation`](../frontend/tests/viewer.spec.cjs#L91) | test | 91 | `test('page loads and renders navigation', callback)` | page loads and renders navigation |
| [`primary navigation links are present`](../frontend/tests/viewer.spec.cjs#L97) | test | 97 | `test('primary navigation links are present', callback)` | primary navigation links are present |
| [`primary navigation preserves the selected research context`](../frontend/tests/viewer.spec.cjs#L106) | test | 106 | `test('primary navigation preserves the selected research context', callback)` | primary navigation preserves the selected research context |
| [`research context is displayed after selecting a run`](../frontend/tests/viewer.spec.cjs#L117) | test | 117 | `test('research context is displayed after selecting a run', callback)` | research context is displayed after selecting a run |
| [`displays run metrics for a completed run`](../frontend/tests/viewer.spec.cjs#L129) | test | 129 | `test('displays run metrics for a completed run', callback)` | displays run metrics for a completed run |
| [`displays retention funnel for completed run`](../frontend/tests/viewer.spec.cjs#L136) | test | 136 | `test('displays retention funnel for completed run', callback)` | displays retention funnel for completed run |
| [`uses one initial raw-result baseline across source and pipeline retention stages`](../frontend/tests/viewer.spec.cjs#L143) | test | 143 | `test('uses one initial raw-result baseline across source and pipeline retention stages', callback)` | uses one initial raw-result baseline across source and pipeline retention stages |
| [`displays informational source export-count comparisons`](../frontend/tests/viewer.spec.cjs#L189) | test | 189 | `test('displays informational source export-count comparisons', callback)` | displays informational source export-count comparisons |
| [`shows no enrichment badge for enrichment-disabled run`](../frontend/tests/viewer.spec.cjs#L197) | test | 197 | `test('shows no enrichment badge for enrichment-disabled run', callback)` | shows no enrichment badge for enrichment-disabled run |
| [`displays enrichment field and provider breakdowns`](../frontend/tests/viewer.spec.cjs#L204) | test | 204 | `test('displays enrichment field and provider breakdowns', callback)` | displays enrichment field and provider breakdowns |
| [`articles section loads and shows article rows`](../frontend/tests/viewer.spec.cjs#L217) | test | 217 | `test('articles section loads and shows article rows', callback)` | articles section loads and shows article rows |
| [`articles section supports search filtering`](../frontend/tests/viewer.spec.cjs#L224) | test | 224 | `test('articles section supports search filtering', callback)` | articles section supports search filtering |
| [`authors section loads and shows author rows`](../frontend/tests/viewer.spec.cjs#L232) | test | 232 | `test('authors section loads and shows author rows', callback)` | authors section loads and shows author rows |
| [`references section loads and shows reference rows`](../frontend/tests/viewer.spec.cjs#L239) | test | 239 | `test('references section loads and shows reference rows', callback)` | references section loads and shows reference rows |
| [`sources section loads and shows source records`](../frontend/tests/viewer.spec.cjs#L249) | test | 249 | `test('sources section loads and shows source records', callback)` | sources section loads and shows source records |
| [`identity evidence shows uncertain ORCID candidates without a confirmed link`](../frontend/tests/viewer.spec.cjs#L265) | test | 265 | `test('identity evidence shows uncertain ORCID candidates without a confirmed link', callback)` | identity evidence shows uncertain ORCID candidates without a confirmed link |
| [`corpus supports pagination`](../frontend/tests/viewer.spec.cjs#L275) | test | 275 | `test('corpus supports pagination', callback)` | corpus supports pagination |
| [`corpus ignores a sort field that is unsupported by its selected section`](../frontend/tests/viewer.spec.cjs#L286) | test | 286 | `test('corpus ignores a sort field that is unsupported by its selected section', callback)` | corpus ignores a sort field that is unsupported by its selected section |
| [`graph loads and renders for a completed run`](../frontend/tests/viewer.spec.cjs#L297) | test | 297 | `test('graph loads and renders for a completed run', callback)` | graph loads and renders for a completed run |
| [`graph supports mode switching to citation`](../frontend/tests/viewer.spec.cjs#L306) | test | 306 | `test('graph supports mode switching to citation', callback)` | graph supports mode switching to citation |
| [`graph supports text search filter`](../frontend/tests/viewer.spec.cjs#L313) | test | 313 | `test('graph supports text search filter', callback)` | graph supports text search filter |
| [`graph filters are applied explicitly and remain visible in the URL`](../frontend/tests/viewer.spec.cjs#L320) | test | 320 | `test('graph filters are applied explicitly and remain visible in the URL', callback)` | graph filters are applied explicitly and remain visible in the URL |
| [`graph node search input is present and functional`](../frontend/tests/viewer.spec.cjs#L328) | test | 328 | `test('graph node search input is present and functional', callback)` | graph node search input is present and functional |
| [`graph export PNG button is present`](../frontend/tests/viewer.spec.cjs#L339) | test | 339 | `test('graph export PNG button is present', callback)` | graph export PNG button is present |
| [`graph legend is anchored at the bottom left of the canvas`](../frontend/tests/viewer.spec.cjs#L346) | test | 346 | `test('graph legend is anchored at the bottom left of the canvas', callback)` | graph legend is anchored at the bottom left of the canvas |
| [`background click clears a selected graph node`](../frontend/tests/viewer.spec.cjs#L358) | test | 358 | `test('background click clears a selected graph node', callback)` | background click clears a selected graph node |
| [`clicking the selected graph node again clears it`](../frontend/tests/viewer.spec.cjs#L367) | test | 367 | `test('clicking the selected graph node again clears it', callback)` | clicking the selected graph node again clears it |
| [`secondary-button drag pans without changing graph selection`](../frontend/tests/viewer.spec.cjs#L393) | test | 393 | `test('secondary-button drag pans without changing graph selection', callback)` | secondary-button drag pans without changing graph selection |
| [`audit section loads and shows events`](../frontend/tests/viewer.spec.cjs#L410) | test | 410 | `test('audit section loads and shows events', callback)` | audit section loads and shows events |
| [`audit stream filters events by category on the server`](../frontend/tests/viewer.spec.cjs#L417) | test | 417 | `test('audit stream filters events by category on the server', callback)` | audit stream filters events by category on the server |
| [`audit stream exposes mirrored PDF events`](../frontend/tests/viewer.spec.cjs#L436) | test | 436 | `test('audit stream exposes mirrored PDF events', callback)` | audit stream exposes mirrored PDF events |
| [`artifacts section identifies configuration snapshots and offers downloads`](../frontend/tests/viewer.spec.cjs#L444) | test | 444 | `test('artifacts section identifies configuration snapshots and offers downloads', callback)` | artifacts section identifies configuration snapshots and offers downloads |
| [`cache uses section supports shared filtering and pagination controls`](../frontend/tests/viewer.spec.cjs#L457) | test | 457 | `test('cache uses section supports shared filtering and pagination controls', callback)` | cache uses section supports shared filtering and pagination controls |
| [`stages section loads`](../frontend/tests/viewer.spec.cjs#L470) | test | 470 | `test('stages section loads', callback)` | stages section loads |
| [`run details section loads`](../frontend/tests/viewer.spec.cjs#L478) | test | 478 | `test('run details section loads', callback)` | run details section loads |
| [`lists only normalized articles with manual inventory status`](../frontend/tests/viewer.spec.cjs#L490) | test | 490 | `test('lists only normalized articles with manual inventory status', callback)` | lists only normalized articles with manual inventory status |
| [`preserves research context in the Evaluation navigation link`](../frontend/tests/viewer.spec.cjs#L503) | test | 503 | `test('preserves research context in the Evaluation navigation link', callback)` | preserves research context in the Evaluation navigation link |
| [`lists available tables`](../frontend/tests/viewer.spec.cjs#L516) | test | 516 | `test('lists available tables', callback)` | lists available tables |
| [`displays rows from a selected table`](../frontend/tests/viewer.spec.cjs#L523) | test | 523 | `test('displays rows from a selected table', callback)` | displays rows from a selected table |
| [`table browser supports pagination`](../frontend/tests/viewer.spec.cjs#L530) | test | 530 | `test('table browser supports pagination', callback)` | table browser supports pagination |
| [`ignores a sort field that does not belong to the selected table`](../frontend/tests/viewer.spec.cjs#L539) | test | 539 | `test('ignores a sort field that does not belong to the selected table', callback)` | ignores a sort field that does not belong to the selected table |
| [`trash view shows trashed runs`](../frontend/tests/viewer.spec.cjs#L549) | test | 549 | `test('trash view shows trashed runs', callback)` | trash view shows trashed runs |
| [`article detail shows revision metadata`](../frontend/tests/viewer.spec.cjs#L560) | test | 560 | `test('article detail shows revision metadata', callback)` | article detail shows revision metadata |
| [`article detail opens the bound PDF without discarding research context`](../frontend/tests/viewer.spec.cjs#L572) | test | 572 | `test('article detail opens the bound PDF without discarding research context', callback)` | article detail opens the bound PDF without discarding research context |
| [`article detail shows an absent PDF without an open action`](../frontend/tests/viewer.spec.cjs#L586) | test | 586 | `test('article detail shows an absent PDF without an open action', callback)` | article detail shows an absent PDF without an open action |
| [`article detail preserves the originating corpus state`](../frontend/tests/viewer.spec.cjs#L592) | test | 592 | `test('article detail preserves the originating corpus state', callback)` | article detail preserves the originating corpus state |
| [`article detail shows authors and references`](../frontend/tests/viewer.spec.cjs#L604) | test | 604 | `test('article detail shows authors and references', callback)` | article detail shows authors and references |
| [`author detail shows author information`](../frontend/tests/viewer.spec.cjs#L612) | test | 612 | `test('author detail shows author information', callback)` | author detail shows author information |
| [`reference detail shows reference information`](../frontend/tests/viewer.spec.cjs#L623) | test | 623 | `test('reference detail shows reference information', callback)` | reference detail shows reference information |
| [`shows error for invalid API route`](../frontend/tests/viewer.spec.cjs#L634) | test | 634 | `test('shows error for invalid API route', callback)` | shows error for invalid API route |
| [`shows error for invalid per_page value`](../frontend/tests/viewer.spec.cjs#L642) | test | 642 | `test('shows error for invalid per_page value', callback)` | shows error for invalid per_page value |
| [`rejects SQL injection attempts in sort parameter`](../frontend/tests/viewer.spec.cjs#L649) | test | 649 | `test('rejects SQL injection attempts in sort parameter', callback)` | rejects SQL injection attempts in sort parameter |
| [`rejects invalid order parameter`](../frontend/tests/viewer.spec.cjs#L656) | test | 656 | `test('rejects invalid order parameter', callback)` | rejects invalid order parameter |
| [`rejects unknown query parameters`](../frontend/tests/viewer.spec.cjs#L663) | test | 663 | `test('rejects unknown query parameters', callback)` | rejects unknown query parameters |
| [`returns 404 for nonexistent table`](../frontend/tests/viewer.spec.cjs#L670) | test | 670 | `test('returns 404 for nonexistent table', callback)` | returns 404 for nonexistent table |
| [`handles article detail with nonexistent ID gracefully`](../frontend/tests/viewer.spec.cjs#L677) | test | 677 | `test('handles article detail with nonexistent ID gracefully', callback)` | handles article detail with nonexistent ID gracefully |
| [`frontend renders error state for invalid view`](../frontend/tests/viewer.spec.cjs#L682) | test | 682 | `test('frontend renders error state for invalid view', callback)` | frontend renders error state for invalid view |
| [`renders on mobile viewport (375px)`](../frontend/tests/viewer.spec.cjs#L692) | test | 692 | `test('renders on mobile viewport (375px)', callback)` | renders on mobile viewport (375px) |
| [`renders on tablet viewport (768px)`](../frontend/tests/viewer.spec.cjs#L700) | test | 700 | `test('renders on tablet viewport (768px)', callback)` | renders on tablet viewport (768px) |
| [`renders on desktop viewport (1280px)`](../frontend/tests/viewer.spec.cjs#L706) | test | 706 | `test('renders on desktop viewport (1280px)', callback)` | renders on desktop viewport (1280px) |
| [`respects prefers-color-scheme: dark`](../frontend/tests/viewer.spec.cjs#L716) | test | 716 | `test('respects prefers-color-scheme: dark', callback)` | respects prefers-color-scheme: dark |
| [`respects prefers-color-scheme: light`](../frontend/tests/viewer.spec.cjs#L727) | test | 727 | `test('respects prefers-color-scheme: light', callback)` | respects prefers-color-scheme: light |
| [`page has a skip-to-content link`](../frontend/tests/viewer.spec.cjs#L737) | test | 737 | `test('page has a skip-to-content link', callback)` | page has a skip-to-content link |
| [`main content area has a landmark role or id`](../frontend/tests/viewer.spec.cjs#L743) | test | 743 | `test('main content area has a landmark role or id', callback)` | main content area has a landmark role or id |
| [`navigation is a landmark`](../frontend/tests/viewer.spec.cjs#L749) | test | 749 | `test('navigation is a landmark', callback)` | navigation is a landmark |
| [`images have alt text`](../frontend/tests/viewer.spec.cjs#L755) | test | 755 | `test('images have alt text', callback)` | images have alt text |
| [`selecting a search revision shows its plans`](../frontend/tests/viewer.spec.cjs#L769) | test | 769 | `test('selecting a search revision shows its plans', callback)` | selecting a search revision shows its plans |
| [`viewing a failed run shows failure indicators`](../frontend/tests/viewer.spec.cjs#L776) | test | 776 | `test('viewing a failed run shows failure indicators', callback)` | viewing a failed run shows failure indicators |
| [`uses an unlabeled, plain disclosure column`](../frontend/tests/viewer.spec.cjs#L787) | test | 787 | `test('uses an unlabeled, plain disclosure column', callback)` | uses an unlabeled, plain disclosure column |
| [`toggle arrow expands a row showing property grid`](../frontend/tests/viewer.spec.cjs#L795) | test | 795 | `test('toggle arrow expands a row showing property grid', callback)` | toggle arrow expands a row showing property grid |
| [`clicking anywhere on the row expands it`](../frontend/tests/viewer.spec.cjs#L806) | test | 806 | `test('clicking anywhere on the row expands it', callback)` | clicking anywhere on the row expands it |
| [`clicking toggle again collapses the row`](../frontend/tests/viewer.spec.cjs#L815) | test | 815 | `test('clicking toggle again collapses the row', callback)` | clicking toggle again collapses the row |
| [`toggle arrow and aria-expanded update on click`](../frontend/tests/viewer.spec.cjs#L825) | test | 825 | `test('toggle arrow and aria-expanded update on click', callback)` | toggle arrow and aria-expanded update on click |
| [`multiple rows can be expanded simultaneously`](../frontend/tests/viewer.spec.cjs#L839) | test | 839 | `test('multiple rows can be expanded simultaneously', callback)` | multiple rows can be expanded simultaneously |
| [`clicking close button hides the message`](../frontend/tests/viewer.spec.cjs#L853) | test | 853 | `test('clicking close button hides the message', callback)` | clicking close button hides the message |
| [`mobile nav toggle shows and hides navigation links`](../frontend/tests/viewer.spec.cjs#L872) | test | 872 | `test('mobile nav toggle shows and hides navigation links', callback)` | mobile nav toggle shows and hides navigation links |

<!-- END GENERATED PROJECT CATALOG -->
