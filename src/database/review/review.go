// review.go provides the shared review-family store, transaction boundary, list
// bounds, and scanning helpers used by the context, decision, note, anchor,
// relationship, and audit files. Shared lifecycle and error definitions live in
// errors.go.
package review

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	generated "analysis/database/review/internal/sql"
	"analysis/internal/textlimit"
)

const (
	reviewListLimit        = 101
	reviewTextPreviewBytes = 1024
	anchorTextPreviewBytes = 512
)

// Store binds the generated review queries to one already configured connection.
type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// New returns a review family store over an already configured connection. It
// only binds the generated queries to db; it does not build a SQLite URI,
// alter pragmas, open or close the connection, load migration configuration,
// or run migrations.
func New(db *sql.DB) *Store {
	return &Store{db: db, queries: generated.New(db)}
}

// withTx runs fn inside one transaction bound to the generated queries and
// rolls back every partial review mutation on error.
func (s *Store) withTx(ctx context.Context, fn func(*generated.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		// Rollback on panic; if fn returned an error the tx is already rolled back.
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// timestamp returns the current UTC time in the review repository's persisted format.
func timestamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

// CorpusID returns the opaque corpus identity used to namespace browser-local drafts.
func (s *Store) CorpusID(ctx context.Context) (string, error) {
	id, err := s.queries.GetReviewCorpusID(ctx)
	if err != nil {
		return "", fmt.Errorf("read review corpus ID: %w", err)
	}
	return id, nil
}

// boundedText decodes a bounded SQL text projection with its original byte length.
type boundedText struct {
	text  sql.NullString
	bytes int
}

// boundedPreview builds a bounded text projection from generated preview,
// original byte-length, and presence fields.
func boundedPreview(preview string, bytes int64, present bool) boundedText {
	return boundedText{text: sql.NullString{String: preview, Valid: present}, bytes: int(bytes)}
}

// optional returns the bounded value and whether SQL truncated its original byte sequence.
func (value boundedText) optional() (*string, bool) {
	if !value.text.Valid {
		return nil, false
	}
	text := textlimit.UTF8Prefix(value.text.String, len([]byte(value.text.String)))
	return &text, value.bytes > len([]byte(text))
}

// optionalInt64 converts an optional application value into a typed nullable parameter.
func optionalInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

// optionalString converts an optional application value into a typed nullable parameter.
func optionalString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

// nullInt64Pointer converts a scanned nullable integer into an optional value.
func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

// optionalStringEqual compares nullable normalized text.
func optionalStringEqual(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

// stringSlicesEqual compares canonical ordered string sets.
func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// sameNullableID compares optional immutable identifiers by value.
func sameNullableID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// reviewerDisplay exposes only the optional username and never places reviewer email in portable API responses.
func reviewerDisplay(username, _ string) string {
	username = strings.TrimSpace(username)
	if username != "" {
		return username
	}
	return "Anonymous or redacted"
}
