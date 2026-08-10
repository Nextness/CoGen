// Package dev marks development builds through the version suffix. The binary
// never embeds frontend assets; asset serving is always filesystem-based and
// selected by the serve --assets-dir flag.
package dev

// Mode marks a development build so version() appends "-development".
//
//	false = release build (the default)
//	true  = development build
const Mode = false
