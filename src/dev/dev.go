// Package dev controls whether the binary serves frontend assets from the
// filesystem (development, hot-reload) or from the embedded FS (production).
// The mode is a compile-time constant, mirroring the pattern in logging.MinLevel.
// Changing it requires rebuilding the binary. There is no runtime mode switch.
package dev

// Mode controls frontend asset serving.
//
//	false = production: serve from //go:embed (the default)
//	true  = development: serve from the filesystem (src/server/frontend/)
const Mode = false
