// repository_input_integration_test.go verifies public repository boundaries reject nil values.
//go:build integration

package database

import (
	"testing"

	"analysis/manifest"
)

// TestRepositoriesRejectNilInputs verifies invalid pointer inputs return errors instead of panicking.
func TestRepositoriesRejectNilInputs(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	for name, create := range map[string]func() error{
		"author occurrence": func() error { _, err := db.AuthorOccs.Create(nil); return err },
		"authorship":        func() error { _, err := db.Authorships.Create(nil); return err },
		"cache entry":       func() error { _, err := db.CacheEntries.Upsert(nil); return err },
		"cache use":         func() error { _, err := db.RunCacheUses.Create(nil); return err },
		"reference mention": func() error { _, err := db.ReferenceMentions.Create(nil); return err },
		"work revision":     func() error { _, err := db.WorkRevisions.Create(nil); return err },
		"audit event":       func() error { _, err := db.AuditEvents.Insert((*manifest.AuditEvent)(nil)); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := create(); err == nil {
				t.Fatal("expected nil input error")
			}
		})
	}
}
