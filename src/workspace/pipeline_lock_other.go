//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package workspace

import (
	"fmt"
	"os"
)

// lockPipelineDatabase rejects execution when the platform cannot enforce the pipeline ownership lock.
func lockPipelineDatabase(path string, create bool) (*os.File, error) {
	return nil, fmt.Errorf("pipeline execution and recovery require an operating system with flock support")
}
