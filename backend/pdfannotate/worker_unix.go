//go:build unix

// worker_unix.go configures bounded worker processes on Unix platforms.
package pdfannotate

import (
	"errors"
	"os/exec"
	"syscall"
)

// configureWorkerProcess isolates the worker in its own process group so
// cancellation kills and reaps any child processes it may have started.
func configureWorkerProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
}
