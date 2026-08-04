// payload_validation.go validates the transport envelope of provider
// HTTP responses before they reach the workspace decoder. It checks
// for valid JSON and the expected top-level structure.
package enrich

import (
	"encoding/json"
	"fmt"
)

// ValidateProviderPayload verifies that a successful provider response has the
// envelope expected by the workspace decoder. It deliberately validates only
// transport envelopes, not optional bibliographic fields: a valid provider
// record may legitimately omit a requested field.
func ValidateProviderPayload(provider, namespace string, body []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(object) == 0 {
		return fmt.Errorf("missing provider response envelope")
	}

	switch provider {
	case "crossref":
		if namespace != "work_by_doi" {
			return fmt.Errorf("unsupported Crossref cache namespace %q", namespace)
		}
		return requireObject(object, "message")
	case "openalex":
		switch namespace {
		case "work_by_doi", "author_by_orcid":
			return requireString(object, "id") // A single OpenAlex record is its own envelope.
		case "work_references":
			return requireArray(object, "results")
		default:
			return fmt.Errorf("unsupported OpenAlex cache namespace %q", namespace)
		}
	case "orcid":
		switch namespace {
		case "author_by_orcid":
			return requireObject(object, "person")
		case "author_name_search":
			return requireArrayOrNull(object, "result")
		default:
			return fmt.Errorf("unsupported ORCID cache namespace %q", namespace)
		}
	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
}

// requireObject requires a valid object value.
func requireObject(object map[string]json.RawMessage, key string) error {
	raw, ok := object[key]
	if !ok {
		return fmt.Errorf("missing %q envelope", key)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return fmt.Errorf("%q envelope is not an object", key)
	}
	return nil
}

// requireArray requires a valid array value.
func requireArray(object map[string]json.RawMessage, key string) error {
	raw, ok := object[key]
	if !ok {
		return fmt.Errorf("missing %q envelope", key)
	}
	var value []json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return fmt.Errorf("%q envelope is not an array", key)
	}
	return nil
}

// requireArrayOrNull requires a valid array or null value.
func requireArrayOrNull(object map[string]json.RawMessage, key string) error {
	raw, ok := object[key]
	if !ok {
		return fmt.Errorf("missing %q envelope", key)
	}
	if string(raw) == "null" {
		return nil
	}
	var value []json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return fmt.Errorf("%q envelope is not an array or null", key)
	}
	return nil
}

// requireString requires a valid string value.
func requireString(object map[string]json.RawMessage, key string) error {
	raw, ok := object[key]
	if !ok {
		return fmt.Errorf("missing %q envelope", key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return fmt.Errorf("%q envelope is not a non-empty string", key)
	}
	return nil
}
