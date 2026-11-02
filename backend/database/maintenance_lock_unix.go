//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

const maintenanceLockRetryInterval = 10 * time.Millisecond

// acquireFileLock takes a nonblocking shared or exclusive flock and retries
// until ownership is available or the context is canceled.
func acquireFileLock(ctx context.Context, file *os.File, mode MaintenanceMode) error {
	how := syscall.LOCK_SH
	if mode == MaintenanceExclusive {
		how = syscall.LOCK_EX
	}
	for {
		err := syscall.Flock(int(file.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return fmt.Errorf("acquire maintenance lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire maintenance lock: %w", ctx.Err())
		case <-time.After(maintenanceLockRetryInterval):
		}
	}
}

// releaseFileLock releases one maintenance flock.
func releaseFileLock(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("release maintenance lock: %w", err)
	}
	return nil
}
