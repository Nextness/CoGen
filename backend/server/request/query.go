// Package request validates viewer identifiers, query allowlists, filters, and pagination while preserving safe HTTP diagnostics.
package request

import (
	"net/http"
	"strconv"
	"strings"
)

// SearchLimit bounds search strings before they reach a database-family filter builder.
const SearchLimit = 200

// permittedPageSizes defines the supported corpus page sizes.
var permittedPageSizes = map[int]bool{20: true, 50: true, 100: true, 200: true, 500: true}

// Problem carries an HTTP status and safe client-facing error message.
type Problem struct {
	Code, Message string
	Status        int
	Details       any
}

// Error returns the receiver's diagnostic message.
func (e *Problem) Error() string { return e.Message }

// BadRequest constructs an API problem with HTTP status 400.
func BadRequest(message string) error {
	return &Problem{Code: "invalid_request", Message: message, Status: http.StatusBadRequest}
}

// NotFound constructs an API problem with HTTP status 404.
func NotFound(message string) error {
	return &Problem{Code: "not_found", Message: message, Status: http.StatusNotFound}
}

// PositiveID parses a strictly positive decimal identifier or returns a bad-request problem.
func PositiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, BadRequest("id must be a positive integer")
	}
	return id, nil
}

// RequiredID parses a required positive query identifier and returns a safe validation problem.
func RequiredID(r *http.Request, name string) (int64, error) {
	if err := KnownQuery(r, name); err != nil {
		return 0, err
	}
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, BadRequest(name + " is required")
	}
	return PositiveID(raw)
}

// KnownQuery rejects query keys outside the endpoint allowlist.
func KnownQuery(r *http.Request, allowed ...string) error {
	if strings.Contains(r.URL.RawQuery, ";") {
		return BadRequest("query parameters must not contain semicolons")
	}
	known := map[string]bool{}
	for _, name := range allowed {
		known[name] = true
	}
	for name := range r.URL.Query() {
		if !known[name] {
			return BadRequest("unknown query parameter: " + name)
		}
	}
	return nil
}

// OptionalInt parses an optional decimal integer and uses zero for an omitted value.
func OptionalInt(raw, name string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, BadRequest(name + " must be an integer")
	}
	return value, nil
}

// Rows validates page, ordering, and search parameters for a bounded collection.
func Rows(r *http.Request, fields map[string]string, fallback string, additionalQueryKeys ...string) (int, int, string, string, string, error) {
	queryKeys := []string{"page", "per_page", "sort", "order", "q"}
	queryKeys = append(queryKeys, additionalQueryKeys...)
	if err := KnownQuery(r, queryKeys...); err != nil {
		return 0, 0, "", "", "", err
	}
	page, perPage := 1, 50
	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, "", "", "", BadRequest("page must be a positive integer")
		}
		page = parsed
	}
	if raw := r.URL.Query().Get("per_page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || !permittedPageSizes[parsed] {
			return 0, 0, "", "", "", BadRequest("per_page must be one of 20, 50, 100, 200, 500")
		}
		perPage = parsed
	}
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = fallback
	}
	if _, ok := fields[sort]; !ok {
		return 0, 0, "", "", "", BadRequest("sort must be a supported field")
	}
	order := strings.ToUpper(r.URL.Query().Get("order"))
	if order == "" {
		order = "ASC"
	}
	if order != "ASC" && order != "DESC" {
		return 0, 0, "", "", "", BadRequest("order must be asc or desc")
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > SearchLimit {
		return 0, 0, "", "", "", BadRequest("q must be at most 200 characters")
	}
	return page, perPage, sort, order, query, nil
}

// Pagination returns validated page, page-size, offset, and limit values.
func Pagination(page, perPage int, total int64, sort, order string) map[string]any {
	return map[string]any{
		"page":        page,
		"per_page":    perPage,
		"total_rows":  total,
		"total_pages": (total + int64(perPage) - 1) / int64(perPage),
		"has_next":    int64(page*perPage) < total,
		"sort":        sort,
		"order":       strings.ToLower(order),
	}
}

// MultiValues parses, deduplicates, and bounds a comma-separated audit facet filter.
func MultiValues(raw, parameter string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	seen := make(map[string]bool)
	values := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		value := strings.TrimSpace(item)
		if value == "" || len(value) > 200 {
			return nil, BadRequest(parameter + " contains an invalid value")
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
		if len(values) > 100 {
			return nil, BadRequest(parameter + " accepts at most 100 values")
		}
	}
	return values, nil
}
