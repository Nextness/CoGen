package doccheck

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
)

// ReviewMetadata is a narrow read-only DTO for one acknowledged document.
// It carries only the stored digest and its reviewed dependents.
type ReviewMetadata struct {
	Document   string
	Hash       string
	Dependents []string
}

// SnapshotReviewMetadata reads the acknowledged documentation state from a
// read-only source snapshot. A missing DOC-STATE.md yields no metadata.
func SnapshotReviewMetadata(fsys fs.FS) ([]ReviewMetadata, error) {
	data, err := fs.ReadFile(fsys, stateDocument)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: cannot read documentation state: %w", stateDocument, err)
	}
	document := string(data)
	dependencies, dependencyFailures := parseDependencies(document)
	stored, storedFailures := parseStoredState(document)
	failures := append(dependencyFailures, storedFailures...)
	if len(failures) > 0 {
		return nil, fmt.Errorf("%s: %s", stateDocument, failures[0])
	}
	metadata := make([]ReviewMetadata, 0, len(stored))
	for path, hash := range stored {
		metadata = append(metadata, ReviewMetadata{
			Document:   path,
			Hash:       hash,
			Dependents: append([]string(nil), dependencies[path]...),
		})
	}
	sort.Slice(metadata, func(left, right int) bool {
		return metadata[left].Document < metadata[right].Document
	})
	return metadata, nil
}
