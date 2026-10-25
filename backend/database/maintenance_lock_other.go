//go:build !(linux || darwin || freebsd || openbsd || netbsd || dragonfly)

package database

import (
	"context"
	"fmt"
	"os"
)

// acquireFileLock rejects maintenance ownership when the platform cannot enforce it.
func acquireFileLock(ctx context.Context, file *os.File, mode MaintenanceMode) error {
	return fmt.Errorf("maintenance ownership requires a platform with flock support")
}

// releaseFileLock has no lock to release on unsupported platforms.
func releaseFileLock(file *os.File) error {
	return nil
}
