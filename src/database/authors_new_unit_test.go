// Unit tests for ORCID validation functions.
//go:build unit

package database

import (
	"testing"
)

// TestORCIDValidFormat verifies orcid valid format.
func TestORCIDValidFormat(t *testing.T) {
	valid := []string{
		"0000-0001-2345-6789", // checksum = 9
		"0000-0002-1694-233X", // checksum = X
		"0000-0003-1234-5674", // checksum = 4
		"0000-0001-5109-3700", // checksum = 0
	}
	for _, orcid := range valid {
		if !isValidORCID(orcid) {
			t.Errorf("expected ORCID %q to be valid", orcid)
		}
	}
}

// TestORCIDInvalidFormat verifies orcid invalid format.
func TestORCIDInvalidFormat(t *testing.T) {
	invalid := []string{
		"",                      // empty
		"0000-0001-2345-678",    // too short
		"0000-0001-2345-67890",  // too long
		"0000-0001-2345-678a",   // non-digit/X check char
		"0000-0001-2345-678",    // missing group
		"0000-0001-2345-67890",  // extra digit
		"0000-0001-2345",        // truncated
		"0000-0001-2345-6789-0", // extra group
		"0000-0001-2345-678",    // wrong length
		"abcd-efgh-ijkl-mnop",   // non-digit characters
	}
	for _, orcid := range invalid {
		if isValidORCID(orcid) {
			t.Errorf("expected ORCID %q to be invalid", orcid)
		}
	}
}

// TestORCIDInvalidChecksum verifies orcid invalid checksum.
func TestORCIDInvalidChecksum(t *testing.T) {
	invalid := []string{
		"0000-0001-2345-6780", // correct checksum is 9, this has 0
		"0000-0001-2345-6781", // checksum 1
		"0000-0001-2345-6782", // checksum 2
		"0000-0001-2345-6783", // checksum 3
		"0000-0001-2345-6784", // checksum 4
		"0000-0001-2345-6785", // checksum 5
		"0000-0001-2345-6786", // checksum 6
		"0000-0001-2345-6787", // checksum 7
		"0000-0001-2345-6788", // checksum 8
		"0000-0001-2345-678X", // checksum should be 9, not X
	}
	for _, orcid := range invalid {
		if isValidORCID(orcid) {
			t.Errorf("expected ORCID %q to have invalid checksum", orcid)
		}
	}
}

// TestORCIDNormalizedStillValid verifies orcid normalized still valid.
func TestORCIDNormalizedStillValid(t *testing.T) {
	if !isValidORCID("0000-0002-1694-233x") {
		t.Error("expected lowercase X check digit to be accepted")
	}
	if !isValidORCID("0000-0001-2345-6789") {
		t.Error("expected numeric check digit to be accepted")
	}
}
