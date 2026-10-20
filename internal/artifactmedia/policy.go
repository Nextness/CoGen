// Package artifactmedia classifies artifact media for bounded previews and safe downloads.
package artifactmedia

import (
	"mime"
	"strings"
)

// Policy describes a normalized media type and its JSON and inline-preview capabilities.
type Policy struct {
	MediaType    string
	JSON, Inline bool
}

// Inspect strips media parameters and applies the closed active-content exclusions for artifact previews.
func Inspect(value string) Policy {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		mediaType = "application/octet-stream"
	}
	json := mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
	inline := strings.HasPrefix(mediaType, "text/") || json || mediaType == "application/x-something-config"
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		inline = false
	}
	return Policy{MediaType: mediaType, JSON: json, Inline: inline}
}
