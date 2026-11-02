// maintenance.go provides cross-process ownership of one metadata bundle's
// schema so preparation cannot run beneath active viewers or pipeline attempts.
package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaintenanceMode selects shared operation ownership or exclusive schema maintenance.
type MaintenanceMode int

const (
	// MaintenanceShared allows multiple current-schema viewers and one pipeline attempt to coexist.
	MaintenanceShared MaintenanceMode = iota
	// MaintenanceExclusive excludes every other maintenance, viewer, and pipeline operation.
	MaintenanceExclusive
)

// MaintenanceLock holds cross-process ownership of one metadata bundle's schema.
type MaintenanceLock struct {
	file *os.File
}

// AcquireMaintenanceLock acquires shared or exclusive bundle maintenance
// ownership for metadataPath. It waits for conflicting owners until ctx is
// canceled. Closing the returned lock releases ownership.
func AcquireMaintenanceLock(ctx context.Context, metadataPath string, mode MaintenanceMode) (*MaintenanceLock, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if strings.TrimSpace(metadataPath) == "" {
		return nil, fmt.Errorf("metadata database path is required")
	}
	if mode != MaintenanceShared && mode != MaintenanceExclusive {
		return nil, fmt.Errorf("unknown maintenance mode %d", mode)
	}
	lockPath, err := maintenanceLockPath(metadataPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("create maintenance lock directory: %w", err)
	}
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("open maintenance lock: %w", err)
	}
	if err := acquireFileLock(ctx, file, mode); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &MaintenanceLock{file: file}, nil
}

// Close releases maintenance ownership.
func (l *MaintenanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := releaseFileLock(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}

// maintenanceLockPath resolves the lock file beside the metadata database.
func maintenanceLockPath(metadataPath string) (string, error) {
	absolute, err := filepath.Abs(metadataPath)
	if err != nil {
		return "", fmt.Errorf("resolve metadata database path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	} else if resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(absolute)); err == nil {
		absolute = filepath.Join(resolvedDir, filepath.Base(absolute))
	}
	return absolute + ".maintenance.lock", nil
}
