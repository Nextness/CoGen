//go:build e2e && e2e_live

package live_test

import (
	flow "analysis/tests/support/go/e2eflow"

	"os"

	"testing"
)

// TestE2ELive verifies the explicitly enabled real-provider path with structural assertions.
func TestE2ELive(t *testing.T) {
	if os.Getenv("E2E_LIVE") != "1" {
		t.Skip("set E2E_LIVE=1 to contact real enrichment providers")
	}
	root := flow.E2ERepositoryRoot(t)
	mode := flow.Mode{
		Name: "live", Enrichment: true, Live: true,
		Sources:        []flow.Source{{Name: "live_csv", Filename: "live.csv", FileType: "csv", Count: 2}},
		ExpectedParsed: 2, ExpectedUnique: 2, ExpectedValid: 2, ExpectedDiscarded: 0,
		ExpectedTitles: []string{"Live ORCID Name Search", "Live Provider Enrichment Candidate"},
	}
	result := flow.RunE2EVariant(t, root, mode, nil)
	flow.AssertE2EAPI(t, result)
}
