// Package migrations parses and applies configured SQLite migration chains for independently owned stores.
package migrations

import (
	"analysis/backend/something"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var migrationFilenamePattern = regexp.MustCompile(`^V[0-9]{5}_[A-Za-z0-9][A-Za-z0-9._-]*\.sql$`)

// ParseChain reads #iteration("_db_migration") entries from the config.
// Returns them in iteration-counter order (as they appear in the file).
func ParseChain(cfg map[string]any) ([]Entry, error) {
	structs, err := something.GetStructAll(cfg, "[iteration]_db_migration")
	if err != nil {
		lg.Debug("migration configuration query failed", "error", err)
		return nil, fmt.Errorf("get migration structs: %w", err)
	}

	var entries []Entry
	for _, s := range structs {
		filename, ok := s["filename"].(string)
		if !ok || !ValidFilename(filename) {
			return nil, fmt.Errorf("migration filename must be in V00001_description.sql form")
		}
		e := Entry{Filename: filename}
		if raw, ok := s["supersedes"].([]any); ok {
			for _, value := range raw {
				filename, ok := value.(string)
				if !ok || !ValidFilename(filename) {
					return nil, fmt.Errorf("migration %s supersedes must contain filenames in V00001_description.sql form", e.Filename)
				}
				if filename == e.Filename {
					return nil, fmt.Errorf("migration %s cannot supersede itself", e.Filename)
				}
				e.Supersedes = append(e.Supersedes, filename)
			}
		}

		entries = append(entries, e)
	}
	lg.Debug("migration configuration query successful", "entries", len(entries))
	return entries, nil
}

// ValidFilename reports whether filename follows the configured VNNNNN_description.sql migration identity form.
func ValidFilename(filename string) bool {
	return filepath.Base(filename) == filename && !strings.ContainsAny(filename, `/\\`) && !strings.Contains(filename, "..") && migrationFilenamePattern.MatchString(filename)
}

// Entry stores one configured migration filename and compatible prior identities.
type Entry struct {
	Filename   string
	Supersedes []string
}

// LoadChain evaluates the database registry and returns its migrations in declaration order.
func LoadChain(configPath string) ([]Entry, error) {
	cfg, err := something.LoadSomethingFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load something config %s: %w", configPath, err)
	}

	entries, err := ParseChain(cfg)
	if err != nil {
		return nil, err
	}

	// Sort by iteration counter (already in order from ParseChain)
	return entries, nil
}

const upMarker = "-- ==UP=="
const downMarker = "-- ==DOWN=="

// ReadUpSQL returns the SQL between a migration's required UP and DOWN markers.
func ReadUpSQL(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	content := string(data)

	if strings.Count(content, upMarker) != 1 || strings.Count(content, downMarker) != 1 {
		return "", fmt.Errorf("migration must contain exactly one %s and one %s marker", upMarker, downMarker)
	}
	upStart := strings.Index(content, upMarker)
	downStart := strings.Index(content, downMarker)
	if downStart <= upStart {
		return "", fmt.Errorf("migration %s marker must precede %s marker", upMarker, downMarker)
	}
	upSQL := strings.TrimSpace(content[upStart+len(upMarker) : downStart])
	if upSQL == "" {
		return "", fmt.Errorf("migration %s section must not be empty", upMarker)
	}
	return upSQL, nil
}
