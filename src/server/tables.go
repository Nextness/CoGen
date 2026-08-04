// tables.go provides the schema-discovered table browser endpoint
// that lists all user-accessible tables and their columns for the
// selected database store.
package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var permittedPageSizes = map[int]bool{20: true, 50: true, 100: true, 200: true, 500: true}

// tablesHandler returns metadata for every discovered browsable table.
func (s *Server) tablesHandler(w http.ResponseWriter, r *http.Request) {
	if err := validateKnownQuery(r); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	items := make([]tableInfo, 0, len(s.tables))
	for _, name := range s.tableNames() {
		items = append(items, s.tables[name])
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
	page, perPage, sort, order, err := tableRequest(r, info)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	ctx, cancel := queryContext(r)
	defer cancel()
	offset := (page - 1) * perPage
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY %s %s LIMIT ? OFFSET ?", quoteIdentifier(table), quoteIdentifier(sort), order)
	rows, err := s.db.QueryContext(ctx, query, perPage, offset)
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
	s.respond(w, r, map[string]any{"table": info, "rows": items, "pagination": map[string]any{"page": page, "per_page": perPage, "total_rows": info.Count, "total_pages": (info.Count + int64(perPage) - 1) / int64(perPage), "sort": sort, "order": strings.ToLower(order)}}, nil)
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
		if err != nil || !permittedPageSizes[value] {
			return 0, 0, "", "", badRequest("per_page must be one of 20, 50, 100, 200, 500")
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
