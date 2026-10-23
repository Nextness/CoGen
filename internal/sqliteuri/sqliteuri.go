// Package sqliteuri constructs encoded SQLite file URIs.
package sqliteuri

import "net/url"

// File returns a SQLite file URI for path with the supplied query parameters.
// Repeated parameters, including modernc SQLite pragmas, retain their values.
func File(path string, parameters map[string][]string) string {
	return (&url.URL{Scheme: "file", Path: path, RawQuery: url.Values(parameters).Encode()}).String()
}
