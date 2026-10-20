//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockPipelineDatabase holds a nonblocking process lock on the database inode until the descriptor closes.
func lockPipelineDatabase(path string, create bool) (*os.File, error) {
	flags := os.O_RDWR
	if create {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, fmt.Errorf("open pipeline lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("database is locked by an active pipeline or recovery process: %w", err)
	}
	return file, nil
}
