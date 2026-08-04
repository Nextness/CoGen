// manifest_unit_test.go tests the resolved-manifest and input-manifest types,
// deterministic fingerprint computation, and canonical config
// serialisation helpers.
//go:build unit

package manifest

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestCanonicalJSONDeterministic verifies canonical json deterministic.
func TestCanonicalJSONDeterministic(t *testing.T) {
	rm := baseResolvedManifest()
	b1, err := canonicalJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := canonicalJSON(rm)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Error("canonical JSON is not deterministic: same input produced different output")
	}
}

// TestCanonicalJSONSemanticEquivalence verifies canonical json semantic equivalence.
func TestCanonicalJSONSemanticEquivalence(t *testing.T) {
	// Two ResolvedManifests with the same semantic content must produce
	// identical canonical JSON.
	rm1 := baseResolvedManifest()

	rm2 := baseResolvedManifest()
	// Copy fields to ensure the structs are independent but equivalent.
	// (They are already independent since baseResolvedManifest returns a new
	// value each call.)

	b1, err := canonicalJSON(rm1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := canonicalJSON(rm2)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Error("semantically equivalent manifests produced different canonical JSON")
	}
}

// TestFingerprintStableForSameManifest verifies fingerprint stable for same manifest.
func TestFingerprintStableForSameManifest(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)

	fp1, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	fp2, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	if fp1 != fp2 {
		t.Error("same manifest pair produced different fingerprints")
	}
}

// TestFingerprintStableForSemanticEquivalence verifies fingerprint stable for semantic equivalence.
func TestFingerprintStableForSemanticEquivalence(t *testing.T) {
	// Two independent manifest pairs with the same content must produce the
	// same fingerprint.
	rm1 := baseResolvedManifest()
	im1 := baseInputManifest(rm1)

	rm2 := baseResolvedManifest()
	im2 := baseInputManifest(rm2)

	fp1, err := ComputeFingerprint(rm1, im1)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if fp1 != fp2 {
		t.Error("semantically equivalent manifest pairs produced different fingerprints")
	}
}

// TestFingerprintIsSHA256Hex verifies fingerprint is sha256 hex.
func TestFingerprintIsSHA256Hex(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)

	fp, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	if len(fp) != sha256.Size*2 {
		t.Errorf("fingerprint length = %d, want %d (SHA-256 hex)", len(fp), sha256.Size*2)
	}

	// Verify it's valid hex
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("fingerprint contains non-hex character: %c", c)
			break
		}
	}
}

// TestFingerprintChangesOnFormatVersion verifies fingerprint changes on format version.
func TestFingerprintChangesOnFormatVersion(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)

	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.FormatVersion = 3
	im2 := baseInputManifest(rm2)

	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when format_version changed")
	}
}

// TestFingerprintChangesOnSearchID verifies fingerprint changes on search id.
func TestFingerprintChangesOnSearchID(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.SearchID = "different-search"
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when search_id changed")
	}
}

// TestFingerprintChangesOnSearchRevision verifies fingerprint changes on search revision.
func TestFingerprintChangesOnSearchRevision(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.SearchRevision = "2026-08-new-query"
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when search_revision changed")
	}
}

// TestFingerprintChangesOnEnrichmentEnabled verifies fingerprint changes on enrichment enabled.
func TestFingerprintChangesOnEnrichmentEnabled(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.EnrichmentEnabled = false
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when enrichment_enabled changed")
	}
}

// TestFingerprintChangesOnReusePolicy verifies fingerprint changes on reuse policy.
func TestFingerprintChangesOnReusePolicy(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.ReusePolicy = "fresh"
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when reuse_policy changed")
	}
}

// TestFingerprintChangesOnCacheReads verifies fingerprint changes on cache reads.
func TestFingerprintChangesOnCacheReads(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.CachePolicy.Reads = []string{"run:31", "global", "network"}
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when cache reads changed")
	}
}

// TestFingerprintChangesOnCacheWrites verifies fingerprint changes on cache writes.
func TestFingerprintChangesOnCacheWrites(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.CachePolicy.Writes = []string{"active_run"}
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when cache writes changed")
	}
}

// TestFingerprintChangesOnNegativeTTL verifies fingerprint changes on negative ttl.
func TestFingerprintChangesOnNegativeTTL(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.CachePolicy.NegativeTTLDays = 30
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when negative_ttl_days changed")
	}
}

// TestFingerprintChangesOnSourceQuery verifies fingerprint changes on source query.
func TestFingerprintChangesOnSourceQuery(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.Sources[0].Query = "TITLE-ABS-KEY(BPMN AND optimisation AND 2026)"
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when source query changed")
	}
}

// TestFingerprintChangesOnSourceFile verifies fingerprint changes on source file.
func TestFingerprintChangesOnSourceFile(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	im2 := baseInputManifest(rm)
	im2.SourceFiles["scopus"] = SourceFileInfo{
		Path:   "corpus/scopus.raw.csv",
		SHA256: "newhash123",
		Size:   2048,
	}
	changedFP, err := ComputeFingerprint(rm, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when source file hash changed")
	}
}

// TestFingerprintChangesOnSourceFileOrder verifies fingerprint changes on source file order.
func TestFingerprintChangesOnSourceFileOrder(t *testing.T) {
	// Adding a source file -> new fingerprint
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	im2 := baseInputManifest(rm)
	im2.SourceFiles["wos"] = SourceFileInfo{
		Path:   "corpus/wos.raw.bib",
		SHA256: "ghi789",
		Size:   768,
	}
	changedFP, err := ComputeFingerprint(rm, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when a source file was added")
	}
}

// TestFingerprintChangesOnRequestedFields verifies fingerprint changes on requested fields.
func TestFingerprintChangesOnRequestedFields(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.Sources[0].RequestedFields = []string{"title", "doi", "abstract", "authors", "references"}
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when requested_fields changed")
	}
}

// TestFingerprintChangesOnFileType verifies fingerprint changes on file type.
func TestFingerprintChangesOnFileType(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.Sources[0].FileType = "bib"
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when file_type changed")
	}
}

// TestFingerprintChangesOnPatchFields verifies fingerprint changes on patch fields.
func TestFingerprintChangesOnPatchFields(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.Sources[0].PatchFields = map[string]string{"different": "mapping"}
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when patch_fields changed")
	}
}

// TestFingerprintChangesOnKeepFields verifies fingerprint changes on keep fields.
func TestFingerprintChangesOnKeepFields(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.Sources[0].KeepFields = []string{"title", "doi"}
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when keep_fields changed")
	}
}

// TestFingerprintChangesOnEnrichmentProviders verifies fingerprint changes on enrichment providers.
func TestFingerprintChangesOnEnrichmentProviders(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.EnrichmentProviders = append(rm2.EnrichmentProviders, EnrichmentProvider{
		Name: "orcid", BaseURL: "https://pub.orcid.org/", Fields: []string{"name"},
	})
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when enrichment providers changed")
	}
}

// TestFingerprintChangesOnSchemaVersion verifies fingerprint changes on schema version.
func TestFingerprintChangesOnSchemaVersion(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	rm2 := baseResolvedManifest()
	rm2.SchemaVersion = "V00005"
	im2 := baseInputManifest(rm2)
	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when schema_version changed")
	}
}

// TestFingerprintChangesOnAllModifications verifies fingerprint changes on all modifications.
func TestFingerprintChangesOnAllModifications(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	// Modify every fingerprint-affecting field
	rm2 := baseResolvedManifest()
	rm2.FormatVersion = 3
	rm2.SearchID = "different-search"
	rm2.SearchRevision = "different-revision"
	rm2.EnrichmentEnabled = false
	rm2.ReusePolicy = "fresh"
	rm2.CachePolicy.Reads = []string{"network"}
	rm2.CachePolicy.Writes = []string{"active_run"}
	rm2.CachePolicy.NegativeTTLDays = 7
	rm2.Sources[0].Query = "different-query"
	rm2.Sources[0].RequestedFields = []string{"title"}
	rm2.SchemaVersion = "V00099"

	im2 := baseInputManifest(rm2)
	im2.SourceFiles["scopus"] = SourceFileInfo{
		Path:   "corpus/scopus.raw.csv",
		SHA256: "allnew",
		Size:   9999,
	}

	changedFP, err := ComputeFingerprint(rm2, im2)
	if err != nil {
		t.Fatal(err)
	}

	if baseFP == changedFP {
		t.Error("fingerprint did not change when all fields were modified")
	}
}

// TestNewInputManifestLinksToResolved verifies new input manifest links to resolved.
func TestNewInputManifestLinksToResolved(t *testing.T) {
	rm := baseResolvedManifest()
	im, err := NewInputManifest(rm, map[string]SourceFileInfo{
		"scopus": {Path: "corpus/scopus.raw.csv", SHA256: "abc", Size: 100},
	})
	if err != nil {
		t.Fatal(err)
	}

	expectedHash, err := rm.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if im.ResolvedManifestHash != expectedHash {
		t.Errorf("ResolvedManifestHash = %q, want %q", im.ResolvedManifestHash, expectedHash)
	}
}

// TestNewInputManifestNilResolved verifies new input manifest nil resolved.
func TestNewInputManifestNilResolved(t *testing.T) {
	_, err := NewInputManifest(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil resolved manifest")
	}
	if !strings.Contains(err.Error(), "resolved manifest is nil") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInputManifestSourceNamesSorted verifies input manifest source names sorted.
func TestInputManifestSourceNamesSorted(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)

	names := im.SourceNames()
	if len(names) != 2 {
		t.Fatalf("SourceNames length = %d, want 2", len(names))
	}

	// Must be sorted alphabetically
	if names[0] != "ieeexplore" || names[1] != "scopus" {
		t.Errorf("SourceNames = %v, want [ieeexplore, scopus]", names)
	}
}

// TestResolvedManifestHashDeterministic verifies resolved manifest hash deterministic.
func TestResolvedManifestHashDeterministic(t *testing.T) {
	rm := baseResolvedManifest()
	h1, err := rm.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := rm.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if h1 != h2 {
		t.Error("ResolvedManifest.Hash is not deterministic")
	}
}

// TestResolvedManifestHashNil verifies resolved manifest hash nil.
func TestResolvedManifestHashNil(t *testing.T) {
	var rm *ResolvedManifest
	_, err := rm.Hash()
	if err == nil {
		t.Fatal("expected error for nil ResolvedManifest")
	}
}

// TestComputeStageFingerprint verifies compute stage fingerprint.
func TestComputeStageFingerprint(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	execFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	stageFP, err := ComputeStageFingerprint("parse", execFP, map[string]any{
		"parser_version": "1.0",
		"strict_mode":    true,
	}, "output_hash_abc")
	if err != nil {
		t.Fatal(err)
	}

	if stageFP.Stage != "parse" {
		t.Errorf("Stage = %q, want %q", stageFP.Stage, "parse")
	}
	if stageFP.InputFingerprint != execFP {
		t.Error("InputFingerprint does not match the execution fingerprint")
	}
	if stageFP.ConfigHash == "" {
		t.Error("ConfigHash should not be empty when stage config is provided")
	}
}

// TestComputeStageFingerprintEmptyStage verifies compute stage fingerprint empty stage.
func TestComputeStageFingerprintEmptyStage(t *testing.T) {
	_, err := ComputeStageFingerprint("", "", nil, "")
	if err == nil {
		t.Fatal("expected error for empty stage name")
	}
}

// TestComputeStageFingerprintNoConfig verifies compute stage fingerprint no config.
func TestComputeStageFingerprintNoConfig(t *testing.T) {
	stageFP, err := ComputeStageFingerprint("enrich", "fp123", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stageFP.ConfigHash != "" {
		t.Error("ConfigHash should be empty when no stage config is provided")
	}
}

// TestStageFingerprintOutputHash verifies stage fingerprint output hash.
func TestStageFingerprintOutputHash(t *testing.T) {
	// Output hash must be propagated through the stage fingerprint.
	stageFP, err := ComputeStageFingerprint("export", "fp_in", nil, "out_hash_xyz")
	if err != nil {
		t.Fatal(err)
	}
	if stageFP.OutputHash != "out_hash_xyz" {
		t.Errorf("OutputHash = %q, want %q", stageFP.OutputHash, "out_hash_xyz")
	}
}

// TestStageFingerprintEmptyOutputHash verifies stage fingerprint empty output hash.
func TestStageFingerprintEmptyOutputHash(t *testing.T) {
	// Empty output hash is valid (output not yet computed).
	stageFP, err := ComputeStageFingerprint("parse", "fp_in", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stageFP.OutputHash != "" {
		t.Errorf("OutputHash = %q, want empty", stageFP.OutputHash)
	}
}

// TestStageFingerprintChangesOnInput verifies stage fingerprint changes on input.
func TestStageFingerprintChangesOnInput(t *testing.T) {
	sfp1, err := ComputeStageFingerprint("parse", "fp_base", map[string]any{"v": "1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	sfp2, err := ComputeStageFingerprint("parse", "fp_different", map[string]any{"v": "1"}, "")
	if err != nil {
		t.Fatal(err)
	}

	if sfp1.ConfigHash != sfp2.ConfigHash {
		t.Error("same stage config should produce same ConfigHash")
	}
	if sfp1.InputFingerprint == sfp2.InputFingerprint {
		t.Error("different input fingerprints should produce different StageFingerprints")
	}
}

// TestStageFingerprintChangesOnConfig verifies stage fingerprint changes on config.
func TestStageFingerprintChangesOnConfig(t *testing.T) {
	sfp1, err := ComputeStageFingerprint("parse", "fp1", map[string]any{"v": "1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	sfp2, err := ComputeStageFingerprint("parse", "fp1", map[string]any{"v": "2"}, "")
	if err != nil {
		t.Fatal(err)
	}

	if sfp1.ConfigHash == sfp2.ConfigHash {
		t.Error("different stage config should produce different ConfigHash")
	}
}

// TestStageFingerprintChangesOnValidationRules verifies stage fingerprint changes on validation rules.
func TestStageFingerprintChangesOnValidationRules(t *testing.T) {
	first, err := ComputeStageFingerprint("validate", "enriched-output", map[string]any{
		"required_fields": []string{"title", "doi"},
	}, "validation-output")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ComputeStageFingerprint("validate", "enriched-output", map[string]any{
		"required_fields": []string{"title", "doi", "publisher"},
	}, "validation-output")
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfigHash == second.ConfigHash {
		t.Fatal("changed validation rules produced the same stage configuration hash")
	}
	if first.InputFingerprint != second.InputFingerprint {
		t.Fatal("validation-rule change unexpectedly changed the upstream input fingerprint")
	}
}

// TestComputeFingerprintNilResolved verifies compute fingerprint nil resolved.
func TestComputeFingerprintNilResolved(t *testing.T) {
	_, err := ComputeFingerprint(nil, &InputManifest{})
	if err == nil {
		t.Fatal("expected error for nil resolved manifest")
	}
}

// TestComputeFingerprintNilInput verifies compute fingerprint nil input.
func TestComputeFingerprintNilInput(t *testing.T) {
	_, err := ComputeFingerprint(&ResolvedManifest{}, nil)
	if err == nil {
		t.Fatal("expected error for nil input manifest")
	}
}

// TestComputeFingerprintHashMismatchIgnored verifies compute fingerprint hash mismatch ignored.
func TestComputeFingerprintHashMismatchIgnored(t *testing.T) {
	// ResolvedManifestHash is a persistence link, not a fingerprint constraint.
	// ComputeFingerprint does not validate it; the fingerprint is computed
	// from the data regardless.
	rm := baseResolvedManifest()
	im := &InputManifest{
		ResolvedManifestHash: "wronghash",
		SourceFiles:          nil,
	}
	_, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatalf("unexpected error: hash mismatch should be ignored by ComputeFingerprint: %v", err)
	}
}

// TestComputeFingerprintEmptyHashAccepted verifies compute fingerprint empty hash accepted.
func TestComputeFingerprintEmptyHashAccepted(t *testing.T) {
	// An empty ResolvedManifestHash is accepted (e.g. zero-value manifests).
	rm := &ResolvedManifest{}
	im := &InputManifest{}
	_, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatalf("unexpected error for empty hash: %v", err)
	}
}

// TestArtifactLayoutDefaults verifies artifact layout defaults.
func TestArtifactLayoutDefaults(t *testing.T) {
	al := ArtifactLayout{
		Root:            "artifacts",
		ContentHashAlgo: "sha256",
		RetentionPolicy: "shared",
	}

	if al.Root != "artifacts" {
		t.Errorf("Root = %q, want %q", al.Root, "artifacts")
	}
	if al.ContentHashAlgo != "sha256" {
		t.Errorf("ContentHashAlgo = %q, want %q", al.ContentHashAlgo, "sha256")
	}
	if al.RetentionPolicy != "shared" {
		t.Errorf("RetentionPolicy = %q, want %q", al.RetentionPolicy, "shared")
	}
}

// TestCacheStoreLayout verifies cache store layout.
func TestCacheStoreLayout(t *testing.T) {
	csl := CacheStoreLayout{
		Path: "cache/global_store",
	}
	if csl.Path != "cache/global_store" {
		t.Errorf("Path = %q, want %q", csl.Path, "cache/global_store")
	}
}

// TestFingerprintChangesTable verifies fingerprint changes table.
func TestFingerprintChangesTable(t *testing.T) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)
	baseFP, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		modifyRM func(*ResolvedManifest)
		modifyIM func(*InputManifest, *ResolvedManifest)
	}{
		{
			name: "format_version",
			modifyRM: func(rm *ResolvedManifest) {
				rm.FormatVersion = 99
			},
		},
		{
			name: "search_id",
			modifyRM: func(rm *ResolvedManifest) {
				rm.SearchID = "other"
			},
		},
		{
			name: "search_revision",
			modifyRM: func(rm *ResolvedManifest) {
				rm.SearchRevision = "other"
			},
		},
		{
			name: "enrichment_enabled",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentEnabled = false
			},
		},
		{
			name: "reuse_policy",
			modifyRM: func(rm *ResolvedManifest) {
				rm.ReusePolicy = "retry"
			},
		},
		{
			name: "cache_reads",
			modifyRM: func(rm *ResolvedManifest) {
				rm.CachePolicy.Reads = []string{"network"}
			},
		},
		{
			name: "cache_writes",
			modifyRM: func(rm *ResolvedManifest) {
				rm.CachePolicy.Writes = []string{"active_run"}
			},
		},
		{
			name: "negative_ttl",
			modifyRM: func(rm *ResolvedManifest) {
				rm.CachePolicy.NegativeTTLDays = 0
			},
		},
		{
			name: "source_query",
			modifyRM: func(rm *ResolvedManifest) {
				rm.Sources[0].Query = "changed"
			},
		},
		{
			name: "source_expected_file",
			modifyRM: func(rm *ResolvedManifest) {
				rm.Sources[0].ExpectedFile = "corpus/other.raw.csv"
			},
		},
		{
			name: "source_requested_fields",
			modifyRM: func(rm *ResolvedManifest) {
				rm.Sources[0].RequestedFields = []string{"title"}
			},
		},
		{
			name: "source_file_hash",
			modifyIM: func(im *InputManifest, rm *ResolvedManifest) {
				sf := im.SourceFiles["scopus"]
				sf.SHA256 = "different"
				im.SourceFiles["scopus"] = sf
			},
		},
		{
			name: "source_file_size",
			modifyIM: func(im *InputManifest, rm *ResolvedManifest) {
				sf := im.SourceFiles["scopus"]
				sf.Size = 99999
				im.SourceFiles["scopus"] = sf
			},
		},
		{
			name: "source_file_read_error",
			modifyIM: func(im *InputManifest, rm *ResolvedManifest) {
				sf := im.SourceFiles["scopus"]
				sf.SHA256 = ""
				sf.Size = 0
				sf.ReadError = "permission denied"
				im.SourceFiles["scopus"] = sf
			},
		},
		{
			name: "source_file_added",
			modifyIM: func(im *InputManifest, rm *ResolvedManifest) {
				im.SourceFiles["wos"] = SourceFileInfo{
					Path: "corpus/wos.raw.bib", SHA256: "new", Size: 100,
				}
			},
		},
		{
			name: "schema_version",
			modifyRM: func(rm *ResolvedManifest) {
				rm.SchemaVersion = "V00010"
			},
		},
		{
			name: "file_type",
			modifyRM: func(rm *ResolvedManifest) {
				rm.Sources[0].FileType = "bib"
			},
		},
		{
			name: "patch_fields",
			modifyRM: func(rm *ResolvedManifest) {
				rm.Sources[0].PatchFields = map[string]string{"new": "mapping"}
			},
		},
		{
			name: "keep_fields",
			modifyRM: func(rm *ResolvedManifest) {
				rm.Sources[0].KeepFields = []string{"title"}
			},
		},
		{
			name: "enrichment_provider_name",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].Name = "different-provider"
			},
		},
		{
			name: "enrichment_provider_url",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].BaseURL = "https://other.example.com/"
			},
		},
		{
			name: "enrichment_provider_fields",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].Fields = []string{"title"}
			},
		},
		{
			name: "enrichment_provider_fill_missing",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].FillMissingOnly = !rm.EnrichmentProviders[0].FillMissingOnly
			},
		},
		{
			name: "provider_added",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders = append(rm.EnrichmentProviders, EnrichmentProvider{
					Name: "orcid", BaseURL: "https://pub.orcid.org/", Fields: []string{"name"},
				})
			},
		},
		{
			name: "provider_rate_per_second",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].RatePerSecond = 50
			},
		},
		{
			name: "provider_concurrency",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].Concurrency = 1
			},
		},
		{
			name: "provider_timeout",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].TimeoutSeconds = 5
			},
		},
		{
			name: "provider_max_retries",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].MaxRetries = 0
			},
		},
		{
			name: "provider_batch_size",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].BatchSize = 100
			},
		},
		{
			name: "provider_extra_urls",
			modifyRM: func(rm *ResolvedManifest) {
				rm.EnrichmentProviders[0].ExtraURLs = map[string]string{"author": "https://api.example.com/author/"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm2 := baseResolvedManifest()
			im2 := baseInputManifest(rm2)

			if tt.modifyRM != nil {
				tt.modifyRM(rm2)
			}
			if tt.modifyIM != nil {
				tt.modifyIM(im2, rm2)
			}

			changedFP, err := ComputeFingerprint(rm2, im2)
			if err != nil {
				t.Fatal(err)
			}

			if baseFP == changedFP {
				t.Error("fingerprint did not change")
			}
		})
	}
}

// TestFingerprintEmptyFields verifies fingerprint empty fields.
func TestFingerprintEmptyFields(t *testing.T) {
	// Empty fields should still produce a valid fingerprint, and the same
	// empty-resolved + empty-input should be stable.
	rm := &ResolvedManifest{}
	im := &InputManifest{}

	fp1, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := ComputeFingerprint(rm, im)
	if err != nil {
		t.Fatal(err)
	}

	if fp1 != fp2 {
		t.Error("fingerprint of empty manifests is not stable")
	}
	if len(fp1) != sha256.Size*2 {
		t.Errorf("fingerprint length = %d, want %d", len(fp1), sha256.Size*2)
	}
}

// BenchmarkComputeFingerprint measures compute fingerprint.
func BenchmarkComputeFingerprint(b *testing.B) {
	rm := baseResolvedManifest()
	im := baseInputManifest(rm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ComputeFingerprint(rm, im)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolvedManifestHash measures resolved manifest hash.
func BenchmarkResolvedManifestHash(b *testing.B) {
	rm := baseResolvedManifest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rm.Hash()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ExampleComputeFingerprint supports the package test suite's example compute fingerprint setup or assertions.
func ExampleComputeFingerprint() {
	rm := &ResolvedManifest{
		FormatVersion:     2,
		SearchID:          "example-search",
		SearchRevision:    "v1",
		EnrichmentEnabled: true,
		ReusePolicy:       "reuse_completed",
		CachePolicy: CachePolicy{
			Reads:           []string{"global", "network"},
			Writes:          []string{"active_run", "global"},
			NegativeTTLDays: 14,
		},
		Sources: []SourceManifest{
			{
				Name:            "scopus",
				ExpectedFile:    "corpus/scopus.raw.csv",
				FileType:        "csv",
				Query:           "TITLE-ABS-KEY(test)",
				RequestedFields: []string{"title", "doi"},
			},
		},
		SchemaVersion: "V00004",
	}

	im, err := NewInputManifest(rm, map[string]SourceFileInfo{
		"scopus": {
			Path:   "corpus/scopus.raw.csv",
			SHA256: "abcdef",
			Size:   1024,
		},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fp, err := ComputeFingerprint(rm, im)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Fingerprint (hex): %x\n", []byte(fp))
}
