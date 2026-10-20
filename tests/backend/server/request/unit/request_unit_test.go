// request_unit_test.go verifies the shared viewer request contract through its production API.
package request_test

import (
	"analysis/backend/server/request"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHelper_positiveID verifies helper positive id.
func TestHelper_positiveID(t *testing.T) {
	tests := []struct {
		raw     string
		wantID  int64
		wantErr bool
	}{
		{"1", 1, false},
		{"42", 42, false},
		{"9223372036854775807", math.MaxInt64, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"-42", 0, true},
		{"abc", 0, true},
		{"", 0, true},
		{"1.0", 0, true},
		{" 1", 0, true},
	}
	for _, tc := range tests {
		id, err := request.PositiveID(tc.raw)
		if tc.wantErr {
			var problem *request.Problem
			if !errors.As(err, &problem) {
				t.Fatalf("PositiveID(%q) error=%v, want Problem", tc.raw, err)
			}
			if problem.Code != "invalid_request" || problem.Status != http.StatusBadRequest {
				t.Errorf("PositiveID(%q) problem=%+v", tc.raw, problem)
			}
			continue
		}
		if err != nil || id != tc.wantID {
			t.Errorf("PositiveID(%q)=%d err=%v, want %d", tc.raw, id, err, tc.wantID)
		}
	}
}

// TestHelper_badRequest verifies helper bad request.
func TestHelper_badRequest(t *testing.T) {
	err := request.BadRequest("test message")
	var problem *request.Problem
	if !errors.As(err, &problem) {
		t.Fatal("badRequest did not return *apiProblem")
	}
	if problem.Code != "invalid_request" {
		t.Errorf("code = %q, want %q", problem.Code, "invalid_request")
	}
	if problem.Message != "test message" {
		t.Errorf("message = %q, want %q", problem.Message, "test message")
	}
	if problem.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", problem.Status, http.StatusBadRequest)
	}
	if problem.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", problem.Error(), "test message")
	}
}

// TestHelper_notFound verifies helper not found.
func TestHelper_notFound(t *testing.T) {
	err := request.NotFound("missing")
	var problem *request.Problem
	if !errors.As(err, &problem) {
		t.Fatal("notFound did not return *apiProblem")
	}
	if problem.Code != "not_found" {
		t.Errorf("code = %q, want %q", problem.Code, "not_found")
	}
	if problem.Message != "missing" {
		t.Errorf("message = %q, want %q", problem.Message, "missing")
	}
	if problem.Status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", problem.Status, http.StatusNotFound)
	}
}

// TestHelper_auditMultiValues verifies helper audit multi values.
func TestHelper_auditMultiValues(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		vals, err := request.MultiValues("", "param")
		if err != nil {
			t.Fatal(err)
		}
		if vals != nil {
			t.Errorf("got %v, want nil", vals)
		}
	})
	t.Run("single value", func(t *testing.T) {
		vals, err := request.MultiValues("abc", "param")
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 1 || vals[0] != "abc" {
			t.Errorf("got %v, want [abc]", vals)
		}
	})
	t.Run("multiple values", func(t *testing.T) {
		vals, err := request.MultiValues("a, b, c", "param")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"a", "b", "c"}
		if len(vals) != len(want) {
			t.Fatalf("got %v, want %v", vals, want)
		}
		for i := range want {
			if vals[i] != want[i] {
				t.Errorf("vals[%d] = %q, want %q", i, vals[i], want[i])
			}
		}
	})
	t.Run("deduplicates values", func(t *testing.T) {
		vals, err := request.MultiValues("x, x, y, x", "param")
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 2 || vals[0] != "x" || vals[1] != "y" {
			t.Errorf("got %v, want [x y]", vals)
		}
	})
	t.Run("whitespace trimmed", func(t *testing.T) {
		vals, err := request.MultiValues("  hello , world ", "param")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"hello", "world"}
		if len(vals) != len(want) {
			t.Fatalf("got %v, want %v", vals, want)
		}
		for i := range want {
			if vals[i] != want[i] {
				t.Errorf("vals[%d] = %q, want %q", i, vals[i], want[i])
			}
		}
	})
	t.Run("rejects empty item", func(t *testing.T) {
		_, err := request.MultiValues("a,,b", "param")
		if err == nil {
			t.Error("expected error for empty item")
		}
	})
	t.Run("rejects long value", func(t *testing.T) {
		long := make([]byte, 201)
		for i := range long {
			long[i] = 'x'
		}
		_, err := request.MultiValues("a,"+string(long), "param")
		if err == nil {
			t.Error("expected error for value >200 chars")
		}
	})
	t.Run("rejects more than 100 values", func(t *testing.T) {
		parts := make([]string, 101)
		for i := 0; i < 101; i++ {
			parts[i] = fmt.Sprintf("v%03d", i)
		}
		_, err := request.MultiValues(strings.Join(parts, ","), "param")
		if err == nil {
			t.Error("expected error for >100 values")
		}
	})
}

// TestHelper_scopedPagination verifies helper scoped pagination.
func TestHelper_scopedPagination(t *testing.T) {
	tests := []struct {
		page, perPage int
		total         int64
		sort, order   string
		want          map[string]any
	}{
		{1, 50, 100, "id", "asc", map[string]any{"page": 1, "per_page": 50, "total_rows": int64(100), "total_pages": int64(2), "has_next": true, "sort": "id", "order": "asc"}},
		{2, 50, 100, "id", "DESC", map[string]any{"page": 2, "per_page": 50, "total_rows": int64(100), "total_pages": int64(2), "has_next": false, "sort": "id", "order": "desc"}},
		{1, 20, 0, "name", "ASC", map[string]any{"page": 1, "per_page": 20, "total_rows": int64(0), "total_pages": int64(0), "has_next": false, "sort": "name", "order": "asc"}},
		{1, 50, 1, "id", "asc", map[string]any{"page": 1, "per_page": 50, "total_rows": int64(1), "total_pages": int64(1), "has_next": false, "sort": "id", "order": "asc"}},
		{1, 100, 250, "year", "DeSc", map[string]any{"page": 1, "per_page": 100, "total_rows": int64(250), "total_pages": int64(3), "has_next": true, "sort": "year", "order": "desc"}},
	}
	for _, tc := range tests {
		got := request.Pagination(tc.page, tc.perPage, tc.total, tc.sort, tc.order)
		for key, wantVal := range tc.want {
			gotVal, ok := got[key]
			if !ok {
				t.Errorf("scopedPagination(%d,%d,%d,%q,%q) missing key %q", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key)
				continue
			}
			switch w := wantVal.(type) {
			case int:
				if g, ok2 := gotVal.(int); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %d", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			case int64:
				if g, ok2 := gotVal.(int64); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %d", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			case bool:
				if g, ok2 := gotVal.(bool); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %v", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			case string:
				if g, ok2 := gotVal.(string); !ok2 || g != w {
					t.Errorf("scopedPagination(%d,%d,%d,%q,%q)[%q] = %v (type %T), want %q", tc.page, tc.perPage, tc.total, tc.sort, tc.order, key, gotVal, gotVal, w)
				}
			default:
				t.Errorf("unexpected type for key %q: %T", key, wantVal)
			}
		}
	}
}

// TestHelper_parseOptionalInt verifies helper parse optional int.
func TestHelper_parseOptionalInt(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		v, err := request.OptionalInt("", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != 0 {
			t.Errorf("got %d, want 0", v)
		}
	})
	t.Run("valid int", func(t *testing.T) {
		v, err := request.OptionalInt("42", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != 42 {
			t.Errorf("got %d, want 42", v)
		}
	})
	t.Run("negative int allowed", func(t *testing.T) {
		v, err := request.OptionalInt("-5", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != -5 {
			t.Errorf("got %d, want -5", v)
		}
	})
	t.Run("zero valid", func(t *testing.T) {
		v, err := request.OptionalInt("0", "param")
		if err != nil {
			t.Fatal(err)
		}
		if v != 0 {
			t.Errorf("got %d, want 0", v)
		}
	})
	t.Run("invalid string", func(t *testing.T) {
		_, err := request.OptionalInt("abc", "param")
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("max int64", func(t *testing.T) {
		v, err := request.OptionalInt("9223372036854775807", "big")
		if err != nil {
			t.Fatal(err)
		}
		if v != math.MaxInt64 {
			t.Errorf("got %d, want %d", v, math.MaxInt64)
		}
	})
}

// TestScopedRowsRequestCapsSearchQuery verifies the shared scoped-rows parser
// rejects an oversized search string before it reaches a family builder.
func TestScopedRowsRequestCapsSearchQuery(t *testing.T) {
	fields := map[string]string{"id": "id"}
	oversized := httptest.NewRequest(http.MethodGet, "/?q="+strings.Repeat("x", 201), nil)
	if _, _, _, _, _, err := request.Rows(oversized, fields, "id"); err == nil {
		t.Fatal("expected an oversized search query to be rejected")
	}
	valid := httptest.NewRequest(http.MethodGet, "/?q=short", nil)
	page, perPage, sort, order, query, err := request.Rows(valid, fields, "id")
	if err != nil || page != 1 || perPage != 50 || sort != "id" || order != "ASC" || query != "short" {
		t.Fatalf("valid request = (%d,%d,%q,%q,%q,%v)", page, perPage, sort, order, query, err)
	}
}
