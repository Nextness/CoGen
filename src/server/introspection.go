// introspection.go owns the retained schema-discovery and advanced table
// browser adapter. Table and column identifiers are selected dynamically, so
// these statements stay outside sqlc and are recognized as one explicit
// raw-SQL exception by the ownership check. Discovery allowlists every
// browsable table, quoting escapes identifiers, and the browser enforces row,
// cell, projection, and response-byte limits.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"analysis/internal/textlimit"
)

// tableInfo stores the discovered columns for one browsable SQLite table.
type tableInfo struct {
	Name    string       `json:"name"`
	Columns []columnInfo `json:"columns"`
}

// columnInfo records a SQLite column's name, declared type, and primary-key position.
type columnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	PrimaryKey bool   `json:"primary_key"`
}

// tableHasColumns reports whether a discovered table contains every requested column.
func (s *Server) tableHasColumns(table string, required ...string) bool {
	info, ok := s.tables[table]
	if !ok {
		return false
	}
	available := make(map[string]struct{}, len(info.Columns))
	for _, column := range info.Columns {
		available[column.Name] = struct{}{}
	}
	for _, name := range required {
		if _, ok := available[name]; !ok {
			return false
		}
	}
	return true
}

// tableNames returns the discovered browsable table names in deterministic order.
func (s *Server) tableNames() []string {
	names := make([]string, 0, len(s.tables))
	for name := range s.tables {
		names = append(names, name)
	}
	// Table discovery orders its query. This fallback is intentionally tiny to
	// avoid exposing map iteration order through the API.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// hasTable reports whether a table was discovered as browsable.
func (s *Server) hasTable(name string) bool { _, ok := s.tables[name]; return ok }

// hasColumn reports whether a discovered table contains a named column.
func (s *Server) hasColumn(table, column string) bool {
	t, ok := s.tables[table]
	if !ok {
		return false
	}
	for _, c := range t.Columns {
		if c.Name == column {
			return true
		}
	}
	return false
}

// discoverTables reads the SQLite schema and returns tables eligible for read-only browsing.
func (s *Server) discoverTables(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM sqlite_master
        WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return fmt.Errorf("discover workspace tables: %w", err)
	}
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	s.tables = make(map[string]tableInfo)
	for _, name := range names {
		columns, err := s.columns(ctx, name)
		if err != nil {
			return err
		}
		s.tables[name] = tableInfo{Name: name, Columns: columns}
	}
	return nil
}

// columns returns ordered metadata for the requested table's columns.
func (s *Server) columns(ctx context.Context, table string) ([]columnInfo, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+quoteIdentifier(table)+")")
	if err != nil {
		return nil, fmt.Errorf("read schema for %q: %w", table, err)
	}
	defer rows.Close()
	var result []columnInfo
	for rows.Next() {
		var cid, pk int
		var name, typ string
		var notNull int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		result = append(result, columnInfo{Name: name, Type: typ, PrimaryKey: pk > 0})
	}
	return result, rows.Err()
}

// quoteIdentifier quotes a validated SQLite identifier and escapes embedded quotes.
func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// verifyReviewSchema rejects an unmigrated metadata database before writable controls are served.
func (s *Server) verifyReviewSchema(ctx context.Context) error {
	requiredTables := []string{
		"pipeline_run_reviewers", "review_settings", "review_contexts", "work_review_versions",
		"work_review_version_substatuses", "review_context_work_heads", "review_notes",
		"review_note_versions", "review_context_note_heads", "review_note_links", "review_anchors",
		"review_anchor_versions", "review_context_anchor_heads",
	}
	for _, table := range requiredTables {
		if !s.hasTable(table) {
			return fmt.Errorf("metadata database is missing review migration table %q; run analysis migrate --db <metadata.db>", table)
		}
	}
	requiredTriggers := []string{
		"review_contexts_abort_update", "review_contexts_abort_delete", "work_review_versions_abort_update",
		"work_review_versions_abort_delete", "review_note_versions_abort_update", "review_note_versions_abort_delete",
		"review_anchor_versions_abort_update", "review_anchor_versions_abort_delete",
	}
	for _, trigger := range requiredTriggers {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?`, trigger).Scan(&count); err != nil {
			return fmt.Errorf("inspect review migration triggers: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("metadata database is missing review migration trigger %q; run analysis migrate --db <metadata.db>", trigger)
		}
	}
	return nil
}

// pdfTableColumns returns the discovered columns for a companion PDF table.
func pdfTableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("inspect PDF store table %q: %w", table, err)
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("inspect PDF store table %q: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inspect PDF store table %q: %w", table, err)
	}
	return columns, nil
}

var permittedPageSizes = map[int]bool{20: true, 50: true, 100: true, 200: true, 500: true}
var permittedAdvancedPageSizes = map[int]bool{20: true, 50: true, 100: true}

const (
	advancedCellBytes       = 1024
	advancedResponseBytes   = 256 * 1024
	advancedProjectionLimit = 32
)

// tableProjection describes the columns that Advanced may return without exposing raw binary or sensitive evidence.
type tableProjection struct {
	Columns        []columnInfo      `json:"columns"`
	OmittedColumns map[string]string `json:"omitted_columns,omitempty"`
	RedactedFields []string          `json:"redacted_fields,omitempty"`
}

// tableSummary is the schema-only discovery shape; row counts are computed only for the selected table.
type tableSummary struct {
	Name string `json:"name"`
	tableProjection
}

// tablesHandler returns metadata for every discovered browsable table.
func (s *Server) tablesHandler(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]tableInfo, 0, len(s.tables))
	for _, name := range s.tableNames() {
		info := s.tables[name]
		items = append(items, tableInfo{Name: info.Name, Columns: safeTableProjection(info).Columns})
	}
	s.respond(w, r, map[string]any{"tables": items}, nil)
}

// tableRows returns a bounded page from one validated browsable table.
func (s *Server) tableRows(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r, "page", "per_page", "sort", "order"); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	table := r.PathValue("table")
	info, ok := s.tables[table]
	if !ok {
		s.respond(w, r, nil, notFound("table not found"))
		return
	}
	projection := safeTableProjection(info)
	if len(projection.Columns) == 0 {
		s.respond(w, r, nil, badRequest("selected table has no safely browsable columns"))
		return
	}
	projectedInfo := tableInfo{Name: info.Name, Columns: projection.Columns}
	page, perPage, sort, order, err := tableRequest(r, projectedInfo)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	var totalRows int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdentifier(table)).Scan(&totalRows); err != nil {
		s.respond(w, r, nil, fmt.Errorf("count selected table %q: %w", table, err))
		return
	}
	totalPages := (totalRows + int64(perPage) - 1) / int64(perPage)
	if totalPages == 0 {
		page = 1
	} else if int64(page) > totalPages {
		page = int(totalPages)
	}
	offset := (page - 1) * perPage
	selectColumns := make([]string, 0, len(projection.Columns))
	truncationColumns := make([]string, 0, len(projection.Columns))
	for index, column := range projection.Columns {
		quoted := quoteIdentifier(column.Name)
		if advancedSensitiveColumn(column.Name) {
			selectColumns = append(selectColumns, fmt.Sprintf("CASE WHEN %s IS NULL THEN NULL ELSE '[redacted]' END AS %s", quoted, quoted))
			continue
		}
		selectColumns = append(selectColumns, fmt.Sprintf("CASE WHEN typeof(%s)='text' AND length(CAST(%s AS BLOB))>? THEN substr(CAST(%s AS BLOB),1,?) ELSE %s END AS %s", quoted, quoted, quoted, quoted, quoted))
		truncationColumns = append(truncationColumns, fmt.Sprintf("CASE WHEN typeof(%s)='text' AND length(CAST(%s AS BLOB))>? THEN 1 ELSE 0 END AS %s", quoted, quoted, quoteIdentifier(fmt.Sprintf("__truncated_%d", index))))
	}
	selectColumns = append(selectColumns, truncationColumns...)
	orderColumns := []string{quoteIdentifier(sort) + " " + sqlOrderKeyword(order)}
	for _, column := range projection.Columns {
		if column.PrimaryKey && column.Name != sort {
			orderColumns = append(orderColumns, quoteIdentifier(column.Name)+" "+sqlOrderKeyword(order))
		}
	}
	if len(orderColumns) == 1 {
		for _, column := range projection.Columns {
			if column.Name != sort {
				orderColumns = append(orderColumns, quoteIdentifier(column.Name)+" "+sqlOrderKeyword(order))
			}
		}
	}
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s LIMIT ? OFFSET ?", strings.Join(selectColumns, ", "), quoteIdentifier(table), strings.Join(orderColumns, ", "))
	args := make([]any, 0, len(projection.Columns)*3+2)
	for _, column := range projection.Columns {
		if advancedSensitiveColumn(column.Name) {
			continue
		}
		args = append(args, advancedCellBytes, advancedCellBytes)
	}
	for _, column := range projection.Columns {
		if advancedSensitiveColumn(column.Name) {
			continue
		}
		args = append(args, advancedCellBytes)
	}
	args = append(args, perPage, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer rows.Close()
	items, err := tableRowsAsMaps(rows)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	truncatedFields := make(map[string][]string)
	for _, column := range projection.RedactedFields {
		truncatedFields[column] = []string{"sensitive_value_redacted"}
	}
	for rowIndex, item := range items {
		for columnIndex, column := range projection.Columns {
			marker := fmt.Sprintf("__truncated_%d", columnIndex)
			if value, ok := item[marker]; ok {
				delete(item, marker)
				if value != nil && value != int64(0) {
					switch text := item[column.Name].(type) {
					case string:
						item[column.Name] = textlimit.UTF8Prefix(text, advancedCellBytes)
					case []byte:
						item[column.Name] = textlimit.UTF8Prefix(string(text), advancedCellBytes)
					}
					truncatedFields[column.Name] = appendUnique(truncatedFields[column.Name], "cell_byte_limit")
				}
			}
		}
		items[rowIndex] = item
	}
	boundAdvancedRows(items, projection.Columns, truncatedFields)
	tableMetadata := tableSummary{Name: info.Name, tableProjection: projection}
	s.respond(w, r, map[string]any{
		"table":            tableMetadata,
		"rows":             items,
		"truncated_fields": truncatedFields,
		"limits": map[string]any{
			"cell_bytes": advancedCellBytes, "response_value_bytes": advancedResponseBytes,
		},
		"pagination": map[string]any{
			"page": page, "per_page": perPage, "total_rows": totalRows, "total_pages": totalPages,
			"sort": sort, "order": strings.ToLower(order),
		},
	}, nil)
}

// tableRowsAsMaps scans table-browser rows without converting byte prefixes before UTF-8 truncation.
func tableRowsAsMaps(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(columns))
		for index, value := range values {
			item[columns[index]] = value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// safeTableProjection excludes binary values, redacts sensitive evidence, and caps overly wide schemas.
func safeTableProjection(info tableInfo) tableProjection {
	projection := tableProjection{Columns: make([]columnInfo, 0, len(info.Columns)), OmittedColumns: make(map[string]string)}
	for _, column := range info.Columns {
		if strings.Contains(strings.ToUpper(column.Type), "BLOB") {
			projection.OmittedColumns[column.Name] = "binary_value"
			continue
		}
		if len(projection.Columns) >= advancedProjectionLimit && !column.PrimaryKey {
			projection.OmittedColumns[column.Name] = "projection_column_limit"
			continue
		}
		projection.Columns = append(projection.Columns, column)
		if advancedSensitiveColumn(column.Name) {
			projection.RedactedFields = append(projection.RedactedFields, column.Name)
		}
	}
	if len(projection.OmittedColumns) == 0 {
		projection.OmittedColumns = nil
	}
	return projection
}

// advancedSensitiveColumn reports whether a generic cell may contain private or large research evidence.
func advancedSensitiveColumn(name string) bool {
	normalized := strings.ToLower(name)
	for _, fragment := range []string{"body", "selected_text", "email", "payload", "before_json", "after_json", "metadata_json", "config_text", "config_json", "manifest_json"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

// boundAdvancedRows enforces a deterministic total value budget without dropping page rows.
func boundAdvancedRows(rows []map[string]any, columns []columnInfo, truncated map[string][]string) {
	used := 0
	for _, row := range rows {
		for _, column := range columns {
			value, ok := row[column.Name]
			if !ok || value == nil {
				continue
			}
			encoded, err := json.Marshal(value)
			if err != nil || used+len(encoded) > advancedResponseBytes {
				row[column.Name] = "[omitted: response byte budget]"
				truncated[column.Name] = appendUnique(truncated[column.Name], "response_byte_limit")
				continue
			}
			used += len(encoded)
		}
	}
}

// appendUnique adds one truncation reason at most once per projected field.
func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// tableRequest parses the requested table name from the route path.
func tableRequest(r *http.Request, info tableInfo) (int, int, string, string, error) {
	page, perPage := 1, 50
	if raw := r.URL.Query().Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return 0, 0, "", "", badRequest("page must be a positive integer")
		}
		page = value
	}
	if raw := r.URL.Query().Get("per_page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || !permittedAdvancedPageSizes[value] {
			return 0, 0, "", "", badRequest("per_page must be one of 20, 50, 100")
		}
		perPage = value
	}
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = info.Columns[0].Name
	}
	order := strings.ToUpper(r.URL.Query().Get("order"))
	if order == "" {
		order = "ASC"
	}
	if order != "ASC" && order != "DESC" {
		return 0, 0, "", "", badRequest("order must be asc or desc")
	}
	for _, column := range info.Columns {
		if column.Name == sort {
			return page, perPage, column.Name, order, nil
		}
	}
	return 0, 0, "", "", badRequest("sort must be a column in the selected table")
}
