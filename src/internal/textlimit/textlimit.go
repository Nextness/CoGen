// Package textlimit provides byte-bounded valid UTF-8 prefixes for persisted text evidence.
package textlimit

import "unicode/utf8"

// UTF8Prefix returns a valid UTF-8 prefix of value that fits within limit bytes.
func UTF8Prefix(value string, limit int) string {
	return string(UTF8PrefixBytes([]byte(value), limit))
}

// UTF8PrefixBytes returns a valid UTF-8 prefix of value that fits within limit bytes.
func UTF8PrefixBytes(value []byte, limit int) []byte {
	if limit < 1 {
		return nil
	}
	data := value
	if len(data) <= limit && utf8.Valid(data) {
		return value
	}
	if len(data) > limit {
		data = data[:limit]
	}
	for len(data) > 0 && !utf8.Valid(data) {
		data = data[:len(data)-1]
	}
	return data
}
