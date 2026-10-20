// Package jsonvalue_test verifies the JSON boundary for absent and recorded zero values.
package jsonvalue_test

import (
	"analysis/internal/jsonvalue"
	"encoding/json"
	"testing"
)

// TestOptionalPreservesNullAndZeroValues keeps absence distinct from empty strings and zero identifiers.
func TestOptionalPreservesNullAndZeroValues(t *testing.T) {
	empty, text := "", "hello"
	zero, id := int64(0), int64(9223372036854775807)
	for name, test := range map[string]struct {
		value any
		want  string
	}{
		"absent text": {jsonvalue.Optional[string](nil), "null"},
		"empty text":  {jsonvalue.Optional(&empty), `""`},
		"text":        {jsonvalue.Optional(&text), `"hello"`},
		"absent id":   {jsonvalue.Optional[int64](nil), "null"},
		"zero id":     {jsonvalue.Optional(&zero), "0"},
		"id":          {jsonvalue.Optional(&id), "9223372036854775807"},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(test.value)
			if err != nil || string(encoded) != test.want {
				t.Fatalf("JSON=%s err=%v, want %s", encoded, err, test.want)
			}
		})
	}
}
