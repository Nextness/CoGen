// Package codefacts exports deterministic portable repository facts without
// requiring CGO, native libraries, agents, or models.
package codefacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Policy selects eligible source bytes for one portable fact snapshot.
type Policy struct {
	Roots              []string
	ExcludeDirs        []string
	ExcludeInternalSQL bool
}

// DefaultPolicy selects maintained source, optional production source, tests, documentation, and configuration.
var DefaultPolicy = Policy{
	Roots:              []string{"backend", "tooling/go", "tooling/agentgraph", "internal", "tests", "docs", "config"},
	ExcludeDirs:        []string{"testdata", "node_modules", "ref"},
	ExcludeInternalSQL: true,
}

// FileFact records one eligible file and its SHA-256 digest.
type FileFact struct {
	Path string
	Hash string
}

// Snapshot is a frozen in-memory view of captured source bytes.
// It implements fs.FS, fs.ReadFileFS, and fs.ReadDirFS so owners can read the
// same bytes without touching the live tree.
type Snapshot struct {
	files map[string][]byte
	dirs  map[string][]fs.DirEntry
	facts []FileFact
}

// Capture reads eligible files under root according to policy and returns an
// immutable snapshot plus their deterministic file facts.
func Capture(root string, policy Policy) (*Snapshot, error) {
	if len(policy.Roots) == 0 {
		policy = DefaultPolicy
	}
	files := make(map[string][]byte)
	for _, relativeRoot := range policy.Roots {
		base := filepath.Join(root, filepath.FromSlash(relativeRoot))
		if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("stat %s: %w", relativeRoot, err)
		}
		err := filepath.WalkDir(base, func(filePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, filePath)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if entry.IsDir() {
				if relative == "docs/ref" || excludedDir(entry.Name(), policy.ExcludeDirs) {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			if policy.ExcludeInternalSQL && generatedInternalSQL(relative) {
				return nil
			}
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read %s: %w", relative, err)
			}
			files[relative] = data
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		files["go.mod"] = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}
	return NewSnapshot(files)
}

// FileFacts reads eligible files from fsys according to policy and returns their ordered facts.
func FileFacts(fsys fs.FS, policy Policy) ([]FileFact, error) {
	if len(policy.Roots) == 0 {
		policy = DefaultPolicy
	}
	facts := make([]FileFact, 0)
	addFile := func(filePath string) error {
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}
		facts = append(facts, FileFact{Path: filePath, Hash: hashBytes(data)})
		return nil
	}
	for _, relativeRoot := range policy.Roots {
		if _, err := fs.Stat(fsys, relativeRoot); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := fs.WalkDir(fsys, relativeRoot, func(filePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if filePath == "docs/ref" || excludedDir(entry.Name(), policy.ExcludeDirs) {
					return fs.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			if policy.ExcludeInternalSQL && generatedInternalSQL(filePath) {
				return nil
			}
			return addFile(filePath)
		})
		if err != nil {
			return nil, err
		}
	}
	if _, err := fs.Stat(fsys, "go.mod"); err == nil {
		if err := addFile("go.mod"); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	sort.Slice(facts, func(left, right int) bool { return facts[left].Path < facts[right].Path })
	return facts, nil
}

// NewSnapshot builds an immutable filesystem and deterministic file facts from captured bytes.
func NewSnapshot(files map[string][]byte) (*Snapshot, error) {
	snapshot := &Snapshot{
		files: make(map[string][]byte, len(files)),
		dirs:  make(map[string][]fs.DirEntry),
	}
	paths := make([]string, 0, len(files))
	for filePath, data := range files {
		if !fs.ValidPath(filePath) || filePath == "." {
			return nil, fmt.Errorf("invalid captured path %q", filePath)
		}
		snapshot.files[filePath] = append([]byte(nil), data...)
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	snapshot.facts = make([]FileFact, 0, len(paths))
	children := make(map[string]map[string]fs.DirEntry)
	for _, filePath := range paths {
		snapshot.facts = append(snapshot.facts, FileFact{Path: filePath, Hash: hashBytes(snapshot.files[filePath])})
		child := fs.FileInfoToDirEntry(fileInfo{name: path.Base(filePath)})
		for parent := path.Dir(filePath); parent != "." && parent != "/"; parent = path.Dir(parent) {
			if children[parent] == nil {
				children[parent] = make(map[string]fs.DirEntry)
			}
			children[parent][child.Name()] = child
			child = fs.FileInfoToDirEntry(fileInfo{name: path.Base(parent), mode: fs.ModeDir})
		}
	}
	for dir, entries := range children {
		list := make([]fs.DirEntry, 0, len(entries))
		for _, entry := range entries {
			list = append(list, entry)
		}
		sort.Slice(list, func(left, right int) bool { return list[left].Name() < list[right].Name() })
		snapshot.dirs[dir] = list
	}
	for filePath := range snapshot.files {
		parent := path.Dir(filePath)
		if parent == "." || parent == "/" {
			continue
		}
		if _, exists := snapshot.dirs[parent]; !exists {
			snapshot.dirs[parent] = []fs.DirEntry{}
		}
	}
	return snapshot, nil
}

// Facts returns the deterministic file facts captured by this snapshot.
func (s *Snapshot) Facts() []FileFact {
	return append([]FileFact(nil), s.facts...)
}

// InputFingerprint hashes the ordered captured path and digest pairs.
func (s *Snapshot) InputFingerprint() string {
	return Fingerprint(s.facts)
}

// Fingerprint hashes ordered path and digest pairs exactly as the envelope contract requires.
func Fingerprint(facts []FileFact) string {
	var builder strings.Builder
	for _, fact := range facts {
		builder.WriteString(fact.Path)
		builder.WriteByte(0)
		builder.WriteString(fact.Hash)
		builder.WriteByte('\n')
	}
	return hashBytes([]byte(builder.String()))
}

// Open implements fs.FS for captured files and directories.
func (s *Snapshot) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if data, ok := s.files[name]; ok {
		return &snapshotFile{name: name, data: data, info: fileInfo{name: path.Base(name), size: int64(len(data))}}, nil
	}
	if entries, ok := s.dirs[name]; ok {
		return &snapshotDir{name: name, entries: entries}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// ReadFile implements fs.ReadFileFS for captured files.
func (s *Snapshot) ReadFile(name string) ([]byte, error) {
	data, ok := s.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), data...), nil
}

// ReadDir implements fs.ReadDirFS for captured directories.
func (s *Snapshot) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, ok := s.dirs[name]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return append([]fs.DirEntry(nil), entries...), nil
}

// excludedDir reports whether a directory name is excluded by policy.
func excludedDir(name string, excludes []string) bool {
	for _, exclude := range excludes {
		if name == exclude {
			return true
		}
	}
	return strings.HasSuffix(name, "-snapshots")
}

// generatedInternalSQL reports whether a path lives under a generated internal/sql directory.
func generatedInternalSQL(relativePath string) bool {
	segments := strings.Split(relativePath, "/")
	for index := 0; index+1 < len(segments); index++ {
		if segments[index] == "internal" && segments[index+1] == "sql" {
			return true
		}
	}
	return false
}

// hashBytes returns the prefixed SHA-256 digest of data.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// fileInfo is a minimal fs.FileInfo for captured files and directories.
type fileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

// Name returns the base name.
func (i fileInfo) Name() string { return i.name }

// Size returns the captured byte length.
func (i fileInfo) Size() int64 { return i.size }

// Mode returns the file mode.
func (i fileInfo) Mode() fs.FileMode { return i.mode }

// ModTime returns a zero time for deterministic facts.
func (i fileInfo) ModTime() time.Time { return time.Time{} }

// IsDir reports whether the entry is a directory.
func (i fileInfo) IsDir() bool { return i.mode.IsDir() }

// Sys returns no host-specific metadata.
func (i fileInfo) Sys() any { return nil }

// snapshotFile is an immutable captured regular file.
type snapshotFile struct {
	name   string
	data   []byte
	offset int
	info   fileInfo
}

// Stat returns the captured file info.
func (f *snapshotFile) Stat() (fs.FileInfo, error) { return f.info, nil }

// Read returns captured bytes from the current offset.
func (f *snapshotFile) Read(buffer []byte) (int, error) {
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(buffer, f.data[f.offset:])
	f.offset += n
	return n, nil
}

// Close releases the file handle.
func (f *snapshotFile) Close() error { return nil }

// snapshotDir is an immutable captured directory.
type snapshotDir struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

// Stat returns directory info.
func (d *snapshotDir) Stat() (fs.FileInfo, error) {
	return fileInfo{name: path.Base(d.name), mode: fs.ModeDir}, nil
}

// Read rejects byte reads on directories.
func (d *snapshotDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}

// Close releases the directory handle.
func (d *snapshotDir) Close() error { return nil }

// ReadDir returns directory entries, honoring the n limit.
func (d *snapshotDir) ReadDir(n int) ([]fs.DirEntry, error) {
	remaining := d.entries[d.offset:]
	if n <= 0 || n > len(remaining) {
		d.offset = len(d.entries)
		return append([]fs.DirEntry(nil), remaining...), nil
	}
	page := append([]fs.DirEntry(nil), remaining[:n]...)
	d.offset += n
	return page, nil
}
