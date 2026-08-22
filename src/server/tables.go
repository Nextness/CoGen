// tables.go provides the schema-discovered table browser endpoint
// that lists all user-accessible tables and their columns for the
// selected database store.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

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
		selectColumns = append(selectColumns, fmt.Sprintf("CASE WHEN typeof(%s)='text' AND length(CAST(%s AS BLOB))>? THEN substr(%s,1,?) ELSE %s END AS %s", quoted, quoted, quoted, quoted, quoted))
		truncationColumns = append(truncationColumns, fmt.Sprintf("CASE WHEN typeof(%s)='text' AND length(CAST(%s AS BLOB))>? THEN 1 ELSE 0 END AS %s", quoted, quoted, quoteIdentifier(fmt.Sprintf("__truncated_%d", index))))
	}
	selectColumns = append(selectColumns, truncationColumns...)
	orderColumns := []string{quoteIdentifier(sort) + " " + order}
	for _, column := range projection.Columns {
		if column.PrimaryKey && column.Name != sort {
			orderColumns = append(orderColumns, quoteIdentifier(column.Name)+" "+order)
		}
	}
	if len(orderColumns) == 1 {
		for _, column := range projection.Columns {
			if column.Name != sort {
				orderColumns = append(orderColumns, quoteIdentifier(column.Name)+" "+order)
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
	items, err := rowsAsMaps(rows)
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
	valid := false
	for _, column := range info.Columns {
		if column.Name == sort {
			valid = true
			break
		}
	}
	if !valid {
		return 0, 0, "", "", badRequest("sort must be a column in the selected table")
	}
	order := strings.ToUpper(r.URL.Query().Get("order"))
	if order == "" {
		order = "ASC"
	}
	if order != "ASC" && order != "DESC" {
		return 0, 0, "", "", badRequest("order must be asc or desc")
	}
	return page, perPage, sort, order, nil
}
