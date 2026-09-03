// Package pathpolicy provides filesystem containment checks for corpus-bundle paths.
package pathpolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveExistingWithin resolves an existing candidate and requires both its lexical and resolved paths to remain below root.
func ResolveExistingWithin(root, candidate string) (string, error) {
	absRoot, absCandidate, resolvedRoot, err := resolveRootAndCandidate(root, candidate)
	if err != nil {
		return "", err
	}
	if !Within(absRoot, absCandidate) {
		return "", fmt.Errorf("path escapes its allowed root")
	}
	resolvedCandidate, err := filepath.EvalSymlinks(absCandidate)
	if err != nil {
		return "", err
	}
	if !Within(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("path escapes its allowed root through a symbolic link")
	}
	return resolvedCandidate, nil
}

// ResolveExistingComponentsWithin resolves every existing candidate component while allowing a missing final file.
func ResolveExistingComponentsWithin(root, candidate string) (string, error) {
	absRoot, absCandidate, resolvedRoot, err := resolveRootAndCandidate(root, candidate)
	if err != nil {
		return "", err
	}
	if !Within(absRoot, absCandidate) {
		return "", fmt.Errorf("path escapes its allowed root")
	}
	resolvedCandidate, err := resolveExistingComponents(absCandidate)
	if err != nil {
		return "", err
	}
	if !Within(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("path escapes its allowed root through a symbolic link")
	}
	return resolvedCandidate, nil
}

// Within reports whether path is root itself or is contained below root.
func Within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// resolveRootAndCandidate returns absolute lexical and resolved root paths for one candidate.
func resolveRootAndCandidate(root, candidate string) (string, string, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", "", err
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(absRoot, candidate)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", "", "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", "", err
	}
	return absRoot, absCandidate, resolvedRoot, nil
}

// resolveExistingComponents resolves all existing components while retaining a not-yet-created final component.
func resolveExistingComponents(path string) (string, error) {
	path = filepath.Clean(path)
	missing := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if _, lstatErr := os.Lstat(path); lstatErr == nil {
			return "", fmt.Errorf("resolve symbolic link %q: %w", path, err)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}
