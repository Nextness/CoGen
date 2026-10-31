//go:build !unix

// worker_other.go keeps worker cancellation bounded on non-Unix platforms.
package pdfannotate

import "os/exec"

// configureWorkerProcess uses the platform default process cancellation.
func configureWorkerProcess(_ *exec.Cmd) {}
