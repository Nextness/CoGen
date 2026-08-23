// Command prepare-osf creates a sanitized, self-contained copy of a corpus bundle.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"analysis/something"
	"analysis/workspace"

	_ "modernc.org/sqlite"
)

// options contains source and new-output paths accepted by the export command.
type options struct{ DB, Config, Out string }

// hashMapping records a reviewer-bearing raw configuration hash and its sanitized replacement.
type hashMapping struct {
	Original  string `json:"original"`
	Sanitized string `json:"sanitized"`
}

// exportManifest records schema identity, copied file hashes, sanitization mappings, and limitations.
type exportManifest struct {
	CreatedAt               string            `json:"created_at"`
	SourceSchemaVersions    map[string]string `json:"source_schema_versions"`
	SanitizedSchemaVersions map[string]string `json:"sanitized_schema_versions"`
	Files                   map[string]string `json:"files"`
	ConfigurationMappings   []hashMapping     `json:"configuration_hash_mappings"`
	BrowserDraftDisclaimer  string            `json:"browser_draft_disclaimer"`
}

// main parses the copy-only export command and reports failures without publishing partial output.
func main() {
	var input options
	flag.StringVar(&input.DB, "db", "", "existing metadata database")
	flag.StringVar(&input.Config, "config", "", "optional SOMETHING configuration to sanitize and include")
	flag.StringVar(&input.Out, "out", "", "new output directory")
	flag.Parse()
	if err := prepare(context.Background(), input); err != nil {
		fmt.Fprintln(os.Stderr, "prepare-osf:", err)
		os.Exit(1)
	}
}

// prepare validates source ownership, sanitizes temporary snapshots, and atomically publishes a new bundle.
func prepare(ctx context.Context, input options) error {
	if strings.TrimSpace(input.DB) == "" || strings.TrimSpace(input.Out) == "" {
		return fmt.Errorf("--db and --out are required")
	}
	dbPath, err := existingFile(input.DB)
	if err != nil {
		return fmt.Errorf("metadata database: %w", err)
	}
	outPath, err := filepath.Abs(input.Out)
	if err != nil {
		return fmt.Errorf("resolve output: %w", err)
	}
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("output already exists: %s", outPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output: %w", err)
	}
	configPath := ""
	if input.Config != "" {
		configPath, err = existingFile(input.Config)
		if err != nil {
			return fmt.Errorf("configuration: %w", err)
		}
	}
	for _, source := range []string{dbPath, configPath} {
		if source != "" && samePath(source, outPath) {
			return fmt.Errorf("output aliases input %s", source)
		}
	}

	source, err := sql.Open("sqlite", sqliteReadURI(dbPath))
	if err != nil {
		return err
	}
	defer source.Close()
	sourceMetadataVersion, err := schemaVersionDB(ctx, source)
	if err != nil {
		return fmt.Errorf("read source metadata schema version: %w", err)
	}
	var relativePDF string
	if err := source.QueryRowContext(ctx, "SELECT relative_path FROM pdf_store_binding WHERE id=1").Scan(&relativePDF); err != nil {
		return fmt.Errorf("read PDF binding: %w", err)
	}
	pdfPath, err := safeCompanionPath(dbPath, relativePDF)
	if err != nil {
		return err
	}
	if _, err := existingFile(pdfPath); err != nil {
		return fmt.Errorf("bound PDF database: %w", err)
	}
	if samePath(pdfPath, outPath) {
		return fmt.Errorf("output aliases bound PDF database")
	}
	sourcePDFVersion, err := schemaVersion(ctx, pdfPath)
	if err != nil {
		return fmt.Errorf("read source PDF schema version: %w", err)
	}

	parent := filepath.Dir(outPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create output parent: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, "."+filepath.Base(outPath)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary export: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(temporary)
		}
	}()

	metadataCopy := filepath.Join(temporary, filepath.Base(dbPath))
	if err := snapshotSQLite(ctx, source, metadataCopy); err != nil {
		return fmt.Errorf("snapshot metadata database: %w", err)
	}
	pdfSource, err := sql.Open("sqlite", sqliteReadURI(pdfPath))
	if err != nil {
		return fmt.Errorf("open bound PDF database: %w", err)
	}
	pdfCopy := filepath.Join(temporary, filepath.Clean(relativePDF))
	if err := os.MkdirAll(filepath.Dir(pdfCopy), 0o755); err != nil {
		pdfSource.Close()
		return err
	}
	if err := snapshotSQLite(ctx, pdfSource, pdfCopy); err != nil {
		pdfSource.Close()
		return fmt.Errorf("snapshot PDF database: %w", err)
	}
	if err := pdfSource.Close(); err != nil {
		return err
	}

	var copiedConfig string
	if configPath != "" {
		copiedConfig, err = copyAndSanitizeConfiguration(configPath, temporary)
		if err != nil {
			return err
		}
		loaded, err := workspace.Load(copiedConfig)
		if err != nil {
			return fmt.Errorf("evaluate sanitized configuration: %w", err)
		}
		for _, run := range loaded.Runs {
			if run.Reviewer.Username != "" || run.Reviewer.Email != "" {
				return fmt.Errorf("sanitized configuration still evaluates to a nonempty reviewer")
			}
		}
	}

	copyDB, err := sql.Open("sqlite", metadataCopy+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	mappings, metadataVersion, err := sanitizeMetadata(ctx, copyDB)
	if err != nil {
		copyDB.Close()
		return err
	}
	if err := copyDB.Close(); err != nil {
		return err
	}
	pdfVersion, err := schemaVersion(ctx, pdfCopy)
	if err != nil {
		return err
	}
	files, err := exportedFileHashes(temporary)
	if err != nil {
		return err
	}
	manifest := exportManifest{
		CreatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
		SourceSchemaVersions:    map[string]string{"metadata": sourceMetadataVersion, "pdf": sourcePDFVersion},
		SanitizedSchemaVersions: map[string]string{"metadata": metadataVersion, "pdf": pdfVersion},
		Files:                   files, ConfigurationMappings: mappings,
		BrowserDraftDisclaimer: "Browser-local drafts and arbitrary note or provider text were not scanned or copied by this export.",
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(temporary, "export-manifest.json"), encoded, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, outPath); err != nil {
		return fmt.Errorf("publish export: %w", err)
	}
	published = true
	return nil
}

// sanitizeMetadata redacts reviewer identity and rewires copied content-addressed configuration artifacts.
func sanitizeMetadata(ctx context.Context, db *sql.DB) ([]hashMapping, string, error) {
	if _, err := db.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
		return nil, "", fmt.Errorf("enable secure deletion in export copy: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE pipeline_run_reviewers SET username='', email=''"); err != nil {
		return nil, "", fmt.Errorf("redact reviewers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE review_settings SET corpus_id=lower(hex(randomblob(16))) WHERE id=1"); err != nil {
		return nil, "", fmt.Errorf("regenerate review corpus ID: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT artifact.id, artifact.content_hash, artifact.content_type,
		blob.pipeline_run_id, blob.data FROM run_artifacts link JOIN artifacts artifact ON artifact.id=link.artifact_id
		JOIN artifact_blobs blob ON blob.artifact_id=artifact.id WHERE link.artifact_role='workspace_config' ORDER BY artifact.id`)
	if err != nil {
		return nil, "", err
	}
	type rawArtifact struct {
		id, runID         int64
		hash, contentType string
		data              []byte
	}
	items := []rawArtifact{}
	for rows.Next() {
		var item rawArtifact
		if err := rows.Scan(&item.id, &item.hash, &item.contentType, &item.runID, &item.data); err != nil {
			rows.Close()
			return nil, "", err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, "", err
	}
	mappings := []hashMapping{}
	for _, item := range items {
		sanitized, changed, err := sanitizeReviewerAssignments(item.data)
		if err != nil {
			return nil, "", fmt.Errorf("sanitize workspace configuration artifact %s: %w", item.hash, err)
		}
		if !changed {
			continue
		}
		digest := sha256.Sum256(sanitized)
		newHash := hex.EncodeToString(digest[:])
		var newID int64
		err = tx.QueryRowContext(ctx, "SELECT id FROM artifacts WHERE content_hash=?", newHash).Scan(&newID)
		if err == sql.ErrNoRows {
			result, err := tx.ExecContext(ctx, `INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES (?, ?, ?)`, newHash, len(sanitized), item.contentType)
			if err != nil {
				return nil, "", err
			}
			newID, err = result.LastInsertId()
			if err != nil {
				return nil, "", err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (?, ?, ?)`, newID, item.runID, sanitized); err != nil {
				return nil, "", err
			}
		} else if err != nil {
			return nil, "", err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE run_artifacts SET artifact_id=? WHERE artifact_id=? AND artifact_role='workspace_config'", newID, item.id); err != nil {
			return nil, "", err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE search_revisions SET config_artifact_hash=? WHERE config_artifact_hash=?", newHash, item.hash); err != nil {
			return nil, "", err
		}
		var references int
		if err := tx.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM run_artifacts WHERE artifact_id=?)+
			(SELECT COUNT(*) FROM run_steps WHERE input_artifact_id=? OR output_artifact_id=?)+
			(SELECT COUNT(*) FROM cache_entries WHERE payload_artifact_id=?)+
			(SELECT COUNT(*) FROM author_identity_candidates WHERE payload_artifact_id=?)`, item.id, item.id, item.id, item.id, item.id).Scan(&references); err != nil {
			return nil, "", err
		}
		if references == 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM artifact_blobs WHERE artifact_id=?", item.id); err != nil {
				return nil, "", err
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM artifacts WHERE id=?", item.id); err != nil {
				return nil, "", err
			}
		}
		mappings = append(mappings, hashMapping{Original: item.hash, Sanitized: newHash})
	}
	var foreignKeyFailures int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyFailures); err != nil {
		return nil, "", err
	}
	if foreignKeyFailures != 0 {
		return nil, "", fmt.Errorf("sanitized metadata has %d foreign key failures", foreignKeyFailures)
	}
	var version string
	if err := tx.QueryRowContext(ctx, "SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1").Scan(&version); err != nil {
		return nil, "", err
	}
	if err := validateArtifactBlobs(ctx, tx); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Original < mappings[j].Original })
	return mappings, version, nil
}

// validateArtifactBlobs recomputes every copied artifact size and SHA-256 identity.
func validateArtifactBlobs(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT artifact.content_hash, artifact.byte_size, blob.data
		FROM artifacts artifact JOIN artifact_blobs blob ON blob.artifact_id=artifact.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var expected string
		var size int
		var data []byte
		if err := rows.Scan(&expected, &size, &data); err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		if size != len(data) || expected != hex.EncodeToString(digest[:]) {
			return fmt.Errorf("artifact blob hash or size validation failed for %s", expected)
		}
	}
	return rows.Err()
}

// sanitizeReviewerAssignments replaces only provable inline reviewer values and fails closed otherwise.
func sanitizeReviewerAssignments(source []byte) ([]byte, bool, error) {
	text := string(source)
	type replacement struct{ start, end int }
	replacements := []replacement{}
	for index := 0; index < len(text); {
		next, kind := nextCodeToken(text, index)
		if kind == tokenEnd {
			break
		}
		if kind != tokenIdentifier {
			// Strings and multiline blocks report one position past their end,
			// while punctuation reports its own position. Advance to the start
			// of the next token without consuming a following identifier's
			// first byte.
			if next > index {
				index = next
			} else {
				index = next + 1
			}
			continue
		}
		index = next
		if !hasWordAt(text, index, "reviewer") {
			index++
			continue
		}
		cursor := skipTrivia(text, index+len("reviewer"))
		if cursor >= len(text) || text[cursor] != '=' {
			index += len("reviewer")
			continue
		}
		start := skipTrivia(text, cursor+1)
		if !hasWordAt(text, start, "reviewer_config") {
			return nil, false, fmt.Errorf("reviewer assignment must use an inline reviewer_config value")
		}
		open := skipTrivia(text, start+len("reviewer_config"))
		if open >= len(text) || text[open] != '{' {
			return nil, false, fmt.Errorf("reviewer assignment uses an unsupported macro or reference")
		}
		end, err := matchingBrace(text, open)
		if err != nil {
			return nil, false, err
		}
		replacements = append(replacements, replacement{start: start, end: end + 1})
		index = end + 1
	}
	if len(replacements) == 0 {
		return append([]byte(nil), source...), false, nil
	}
	var output strings.Builder
	previous := 0
	for _, item := range replacements {
		output.WriteString(text[previous:item.start])
		output.WriteString(`reviewer_config { username = "", email = "", }`)
		previous = item.end
	}
	output.WriteString(text[previous:])
	return []byte(output.String()), true, nil
}

// tokenKind distinguishes source identifiers from skipped strings, comments, multiline data, and punctuation.
type tokenKind int

const (
	tokenEnd tokenKind = iota
	tokenIdentifier
	tokenOther
)

// nextCodeToken returns the next executable SOMETHING source position without inspecting data tokens.
func nextCodeToken(text string, start int) (int, tokenKind) {
	index := skipTrivia(text, start)
	if index >= len(text) {
		return index, tokenEnd
	}
	if text[index] == '"' || text[index] == '\'' {
		end, _ := quotedEnd(text, index)
		return end, tokenOther
	}
	if end, ok := somethingMultilineEnd(text, index); ok {
		return end, tokenOther
	}
	if unicode.IsLetter(rune(text[index])) || text[index] == '_' {
		return index, tokenIdentifier
	}
	return index, tokenOther
}

// somethingMultilineEnd skips one SOMETHING multiline literal including its named closing delimiter.
func somethingMultilineEnd(text string, start int) (int, bool) {
	if !strings.HasPrefix(text[start:], "#multiline") || start+len("#multiline") < len(text) && isIdentifierByte(text[start+len("#multiline")]) {
		return 0, false
	}
	headerEnd := strings.IndexByte(text[start:], '\n')
	if headerEnd < 0 {
		return len(text), true
	}
	headerEnd += start
	fields := strings.Fields(text[start:headerEnd])
	if len(fields) < 2 {
		return headerEnd + 1, true
	}
	delimiter := fields[len(fields)-1]
	for lineStart := headerEnd + 1; lineStart <= len(text); {
		lineEnd := strings.IndexByte(text[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += lineStart
		}
		line := strings.TrimSpace(something.StripMultilineComment(text[lineStart:lineEnd]))
		if line == delimiter || line == delimiter+"," || line == delimiter+";" {
			if lineEnd < len(text) {
				return lineEnd + 1, true
			}
			return lineEnd, true
		}
		if lineEnd == len(text) {
			break
		}
		lineStart = lineEnd + 1
	}
	return len(text), true
}

// skipTrivia advances over whitespace and line comments.
func skipTrivia(text string, start int) int {
	for start < len(text) {
		if unicode.IsSpace(rune(text[start])) {
			start++
			continue
		}
		if strings.HasPrefix(text[start:], "//") {
			if newline := strings.IndexByte(text[start:], '\n'); newline >= 0 {
				start += newline + 1
				continue
			}
			return len(text)
		}
		break
	}
	return start
}

// matchingBrace finds an inline reviewer value boundary while ignoring quoted data and comments.
func matchingBrace(text string, open int) (int, error) {
	depth := 0
	for index := open; index < len(text); index++ {
		if strings.HasPrefix(text[index:], "//") {
			newline := strings.IndexByte(text[index:], '\n')
			if newline < 0 {
				return 0, fmt.Errorf("unclosed reviewer value")
			}
			index += newline
			continue
		}
		if text[index] == '"' || text[index] == '\'' {
			end, err := quotedEnd(text, index)
			if err != nil {
				return 0, err
			}
			index = end - 1
			continue
		}
		switch text[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}
	return 0, fmt.Errorf("unclosed reviewer_config value")
}

// quotedEnd returns the position after an escaped single or double quoted token.
func quotedEnd(text string, start int) (int, error) {
	quote := text[start]
	for index := start + 1; index < len(text); index++ {
		if text[index] == '\\' {
			index++
			continue
		}
		if text[index] == quote {
			return index + 1, nil
		}
	}
	return len(text), fmt.Errorf("unclosed quoted value")
}

// hasWordAt matches one identifier without accepting a longer identifier prefix.
func hasWordAt(text string, start int, word string) bool {
	if start < 0 || start+len(word) > len(text) || text[start:start+len(word)] != word {
		return false
	}
	end := start + len(word)
	return (start == 0 || !isIdentifierByte(text[start-1])) && (end == len(text) || !isIdentifierByte(text[end]))
}

// isIdentifierByte reports whether a byte may continue the identifiers used by the sanitizer.
func isIdentifierByte(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

// copyAndSanitizeConfiguration copies one include tree without permitting lexical or symbolic-link escape.
func copyAndSanitizeConfiguration(source, destinationRoot string) (string, error) {
	root, err := filepath.Abs(filepath.Dir(source))
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	visited := map[string]bool{}
	var copyOne func(string) (string, error)
	copyOne = func(path string) (string, error) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(root, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("configuration include escapes copied root: %s", path)
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", err
		}
		resolvedRelative, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("configuration include escapes copied root through a symbolic link: %s", path)
		}
		destination := filepath.Join(destinationRoot, "config", relative)
		if visited[resolved] {
			return destination, nil
		}
		visited[resolved] = true
		data, err := os.ReadFile(resolved)
		if err != nil {
			return "", err
		}
		sanitized, _, err := sanitizeReviewerAssignments(data)
		if err != nil {
			return "", fmt.Errorf("sanitize configuration %s: %w", absolute, err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(destination, sanitized, 0o644); err != nil {
			return "", err
		}
		includes, err := localIncludes(string(data))
		if err != nil {
			return "", fmt.Errorf("inspect configuration includes in %s: %w", absolute, err)
		}
		for _, include := range includes {
			if _, err := copyOne(filepath.Join(filepath.Dir(absolute), include)); err != nil {
				return "", err
			}
		}
		return destination, nil
	}
	return copyOne(source)
}

// localIncludes extracts literal local include paths outside comments, strings, and multiline data.
func localIncludes(source string) ([]string, error) {
	includes := []string{}
	for offset := 0; offset < len(source); {
		index, kind := nextCodeToken(source, offset)
		if kind == tokenEnd {
			break
		}
		if kind != tokenOther || !strings.HasPrefix(source[index:], "#include") {
			offset = index + 1
			continue
		}
		index = skipTrivia(source, index+len("#include"))
		if index >= len(source) || source[index] != '(' {
			return nil, fmt.Errorf("#include must use a local quoted path")
		}
		index = skipTrivia(source, index+1)
		if index >= len(source) || source[index] != '"' {
			return nil, fmt.Errorf("#include must use a local quoted path")
		}
		end, err := quotedEnd(source, index)
		if err != nil {
			return nil, err
		}
		includes = append(includes, source[index+1:end-1])
		offset = end
	}
	return includes, nil
}

// snapshotSQLite uses VACUUM INTO to capture a WAL-consistent source without overwriting a destination.
func snapshotSQLite(ctx context.Context, source *sql.DB, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("snapshot destination exists")
	}
	_, err := source.ExecContext(ctx, "VACUUM INTO ?", destination)
	return err
}

// schemaVersion opens one SQLite file read-only and returns its latest recorded migration filename.
func schemaVersion(ctx context.Context, path string) (string, error) {
	db, err := sql.Open("sqlite", sqliteReadURI(path))
	if err != nil {
		return "", err
	}
	defer db.Close()
	return schemaVersionDB(ctx, db)
}

// schemaVersionDB returns the latest recorded migration filename from an open SQLite connection.
func schemaVersionDB(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

// exportedFileHashes hashes every regular copied file and rejects unsupported file types.
func exportedFileHashes(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "export-manifest.json" {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("export contains unsupported file type: %s", path)
		}
		hash, err := fileHash(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(mustRelative(root, path))] = hash
		return nil
	})
	return files, err
}

// safeCompanionPath resolves only a clean bundle-relative PDF binding.
func safeCompanionPath(metadataPath, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe companion binding %q", relative)
	}
	return filepath.Join(filepath.Dir(metadataPath), relative), nil
}

// existingFile returns an absolute path only for an existing regular file.
func existingFile(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", absolute)
	}
	return absolute, nil
}

// samePath detects aliases using filesystem identity when both targets exist.
func samePath(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

// sqliteReadURI returns an existing-only read URI with the project busy timeout.
func sqliteReadURI(path string) string {
	return "file:" + filepath.ToSlash(path) + "?mode=ro&_pragma=busy_timeout(5000)"
}

// fileHash streams one file into a lowercase SHA-256 digest.
func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// mustRelative returns the previously validated path relative to its export root.
func mustRelative(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		panic(err)
	}
	return value
}
